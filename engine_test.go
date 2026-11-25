package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestThemesAndDimensions(t *testing.T) {
	for _, theme := range themes {
		for _, wh := range [][2]int{{16, 12}, {40, 32}, {120, 52}, {220, 100}} {
			s := defaults()
			s.Theme = theme
			f := frame(wh[0], wh[1], 4.1, .51, .5, s, "CPU 10% GPU* 2% RAM 45%")
			if len(f.Cells) == 0 {
				t.Fatalf("empty %v %v", theme, wh)
			}
			seen := map[[2]int]bool{}
			for _, c := range f.Cells {
				k := [2]int{c.X, c.Y}
				if c.X < 0 || c.X >= wh[0] || c.Y < 0 || c.Y >= wh[1] || c.Color > 0xffffff || seen[k] {
					t.Fatalf("invalid cell %v", c)
				}
				seen[k] = true
			}
		}
	}
}
func TestRotationAndAudio(t *testing.T) {
	s := defaults()
	f := frame(120, 52, 1, 0, 0, s, "")
	if reflect.DeepEqual(f, frame(120, 52, 1, 1, 0, s, "")) {
		t.Fatal("rotation did not change frame")
	}
	if reflect.DeepEqual(f, frame(120, 52, 1, -1, 0, s, "")) {
		t.Fatal("reverse rotation did not change frame")
	}
	if reflect.DeepEqual(f, frame(120, 52, 1, 0, .9, s, "")) {
		t.Fatal("audio did not change frame")
	}
	if !reflect.DeepEqual(f, frame(120, 52, 1, 0, 0, s, "")) {
		t.Fatal("non-deterministic frame")
	}
}
func TestEffects(t *testing.T) {
	s := defaults()
	f := frame(120, 52, 3, 0, 0, s, "")
	for _, change := range []func(*Settings){func(s *Settings) { s.Snow = false }, func(s *Settings) { s.Matrix = false }, func(s *Settings) { s.Ornaments = false }, func(s *Settings) { s.Messages = false }, func(s *Settings) { s.Countdown = false }, func(s *Settings) { s.SnowPhysics = false }, func(s *Settings) { s.Size = 45 }, func(s *Settings) { s.LightSpeed = 400 }, func(s *Settings) { s.Theme = "Custom"; s.Custom = "#FF00AA" }} {
		v := s
		change(&v)
		if reflect.DeepEqual(f, frame(120, 52, 3, 0, 0, v, "")) {
			t.Fatalf("effect not changed: %+v", v)
		}
	}
	s.Monitor = true
	g := frame(120, 52, 3, 0, 0, s, "CPU 10% GPU* 20% RAM 40%")
	if reflect.DeepEqual(f, g) {
		t.Fatal("stats not rendered")
	}
	s.Message = "메리 크리스마스"
	g = frame(120, 52, 3, 0, 0, s, "")
	found := false
	for _, c := range g.Cells {
		if c.Ch == '메' {
			found = true
		}
	}
	if !found {
		t.Fatal("Unicode message not rendered")
	}
}
func TestSettings(t *testing.T) {
	s := defaults()
	s.FPS = 200
	s.Speed = -1
	s.Size = 1
	s.Background = "invalid"
	s.Theme = "bad"
	s.Message = strings.Repeat("가", 110)
	s = normalize(s)
	if s.FPS != 144 || s.Speed != 0 || s.Size != 45 || s.Background != defaults().Background || s.Theme != themes[0] || len([]rune(s.Message)) != 100 {
		t.Fatalf("bad normalization %+v", s)
	}
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var n Settings
	if e = json.Unmarshal(b, &n); e != nil || !reflect.DeepEqual(s, n) {
		t.Fatal("settings round trip failed", e)
	}
}
func TestCountdown(t *testing.T) {
	for _, c := range []struct {
		date time.Time
		want string
	}{{time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC), "CHRISTMAS  D-30"}, {time.Date(2026, 12, 25, 12, 0, 0, 0, time.UTC), "CHRISTMAS DAY"}, {time.Date(2026, 12, 26, 12, 0, 0, 0, time.UTC), "CHRISTMAS  D-364"}} {
		if got := countdown(c.date); got != c.want {
			t.Fatalf("got %q want %q", got, c.want)
		}
	}
}
func TestTinyFrame(t *testing.T) {
	if len(frame(5, 4, 0, 0, 0, defaults(), "").Cells) != 0 {
		t.Fatal("tiny frame not handled")
	}
}
func BenchmarkFrame(b *testing.B) {
	s := defaults()
	for i := 0; i < b.N; i++ {
		frame(120, 52, float64(i)*.01, float64(i)*.01, .3, s, "")
	}
}

func TestV22RendererAndCodeStyleMigration(t *testing.T) {
	s := defaults()
	s.Renderer = renderers[0]
	s.CodeStyle = 500
	s = normalize(s)
	if s.Renderer != renderers[1] || s.CodeStyle != len(codeThemes)-1 {
		t.Fatalf("migration: %+v", s)
	}
	s.CodeStyle = -1
	if normalize(s).CodeStyle != 0 {
		t.Fatal("code style range")
	}
}
func TestV22StableTreeGeometry(t *testing.T) {
	s := defaults()
	s.Snow = false
	s.Matrix = false
	s.Spectrum = false
	s.Countdown = false
	s.Messages = false
	a := AudioFrame{Level: .8, Active: true}
	a.Bands[0] = .9
	a.Bands[1] = .7
	f := audioFrame(120, 52, 1, .4, AudioFrame{}, s, "", false)
	q := audioFrame(120, 52, 1, .4, a, s, "", false)
	if len(f.Cells) != len(q.Cells) {
		t.Fatalf("geometry count changed: %d %d", len(f.Cells), len(q.Cells))
	}
	for i, c := range f.Cells {
		d := q.Cells[i]
		if c.X != d.X || c.Y != d.Y || c.Depth != d.Depth {
			t.Fatal("music moved tree geometry")
		}
	}
	if reflect.DeepEqual(f, q) {
		t.Fatal("lights did not react")
	}
}
