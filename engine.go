package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	Couple                                                                                  CoupleSettings
	Theme, Direction, Custom, Background, Message, Renderer                                 string
	Speed, LightSpeed, FPS, Size                                                            int
	AudioPreset                                                                             string
	FullscreenResolution                                                                    string
	CodeStyle                                                                               int
	AudioGain, BassGain, MidGain, HighGain                                                  int
	Spectrum, ShowBPM, SafeLights, LowPower, Stories                                        bool
	Snow, Ornaments, Messages, Bloom, Matrix, Countdown, Monitor, Music, Multi, SnowPhysics bool
}

var themes = []string{"Rainbow RGB", "Classic Christmas", "Ice Blue", "Cyberpunk", "Gold & White", "Matrix Green", "Custom"}
var codeThemes = []string{"Python", "C++", "Git", "Neural Network", "Minimal"}
var ornamentGlyphs = []string{"", "<>{}*&:;", "+-@#^~", "01[]*=^", "+."}
var directions = []string{"Clockwise", "Counterclockwise", "Static"}
var renderers = []string{"OpenGL (auto fallback)", "GDI (compatibility)"}
var fullscreenResolutions = []string{"자동 (모니터 해상도)", "1080p", "4K"}

func defaults() Settings {
	return Settings{Theme: themes[0], Direction: directions[0], Custom: "#5BD6FF", Background: "#0D0D0D", Renderer: renderers[1], FullscreenResolution: fullscreenResolutions[0], Speed: 100, LightSpeed: 100, FPS: 60, Size: 88, Snow: true, Ornaments: true, Messages: true, Bloom: true, Matrix: true, Countdown: true, Music: true, Multi: true, SnowPhysics: true, Stories: true, AudioPreset: audioPresets[0], AudioGain: 100, BassGain: 100, MidGain: 100, HighGain: 100, Spectrum: true, ShowBPM: false, SafeLights: true}
}
func clamp(x, a, b int) int {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if s == v {
			return true
		}
	}
	return false
}
func hexColor(s string) (uint32, bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, false
	}
	v, e := strconv.ParseUint(s[1:], 16, 32)
	return uint32(v), e == nil
}
func normalize(s Settings) Settings {
	d := defaults()
	if !contains(themes, s.Theme) {
		s.Theme = d.Theme
	}
	if !contains(directions, s.Direction) {
		s.Direction = d.Direction
	}
	s.Renderer = renderers[1]
	if !contains(fullscreenResolutions, s.FullscreenResolution) {
		s.FullscreenResolution = d.FullscreenResolution
	}
	if _, ok := hexColor(s.Custom); !ok {
		s.Custom = d.Custom
	}
	if _, ok := hexColor(s.Background); !ok {
		s.Background = d.Background
	}
	s.CodeStyle = clamp(s.CodeStyle, 0, len(codeThemes)-1)
	s.Speed = clamp(s.Speed, 0, 400)
	s.LightSpeed = clamp(s.LightSpeed, 0, 400)
	s.FPS = clamp(s.FPS, 10, 144)
	s.Size = clamp(s.Size, 45, 100)
	if !contains(audioPresets, s.AudioPreset) {
		s.AudioPreset = d.AudioPreset
	}
	s.AudioGain = clamp(s.AudioGain, 0, 200)
	s.BassGain = clamp(s.BassGain, 0, 200)
	s.MidGain = clamp(s.MidGain, 0, 200)
	s.HighGain = clamp(s.HighGain, 0, 200)
	r := []rune(s.Message)
	if len(r) > 100 {
		s.Message = string(r[:100])
	}
	s.Couple = normalizeCouple(s.Couple)
	return s
}
func rgb(r, g, b int) uint32 {
	return uint32(clamp(r, 0, 255)<<16 | clamp(g, 0, 255)<<8 | clamp(b, 0, 255))
}
func shade(c uint32, k float64) uint32 {
	return rgb(int(float64(c>>16&255)*k), int(float64(c>>8&255)*k), int(float64(c&255)*k))
}
func hsv(h, s, v float64) uint32 {
	h = h - math.Floor(h)
	f := h * 6
	i := int(f)
	a := f - float64(i)
	p, q, t := v*(1-s), v*(1-s*a), v*(1-s*(1-a))
	var r, g, b float64
	switch i {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return rgb(int(r*255), int(g*255), int(b*255))
}

type Point struct {
	x, y, z, u float64
	ch         byte
	kind       int
}

func radius(y float64) float64 {
	for _, v := range [][3]float64{{.34, 1.14, .46}, {-.25, .60, .68}, {-.91, .04, .92}} {
		if y >= v[0] && y <= v[1] {
			return .035 + v[2]*(v[1]-y)/(v[1]-v[0])
		}
	}
	return .025
}

const trunkBottom = -1.37

func tree() []Point {
	r := rand.New(rand.NewSource(1225))
	p := []Point{}
	add := func(y, a, rad float64, c byte, k int, u float64) {
		p = append(p, Point{rad * math.Cos(a), y, rad * math.Sin(a), u, c, k})
	}
	for i := 0; i < 103; i++ {
		y := -.91 + 2.05*float64(i)/102
		rad := radius(y)
		n := int(115 * rad)
		if n < 12 {
			n = 12
		}
		for j := 0; j < n; j++ {
			add(y, 2*math.Pi*float64(j)/float64(n)+float64(i)*.21, rad+(r.Float64()-.5)*.036, "^+*.:#"[r.Intn(6)], 0, r.Float64())
		}
	}
	for j := 0; j < 370; j++ {
		u := float64(j) / 369
		y := -.88 + 1.99*u
		a := 2 * math.Pi * 3.2 * u
		add(y, a, radius(y)+.055, 'o', 1, u)
		add(y, a+math.Pi, radius(y)+.035, '.', 2, u)
	}
	for j := 0; j < 95; j++ {
		y := -.86 + r.Float64()*1.83
		add(y, r.Float64()*2*math.Pi, radius(y)+.075, "{}<>01@$%&"[r.Intn(10)], 3, r.Float64())
	}
	for i := 0; i < 22; i++ {
		for j := 0; j < 22; j++ {
			add(trunkBottom+float64(i)*.022, 2*math.Pi*float64(j)/22, .105, '|', 4, 0)
		}
	}
	return p
}

var points = tree()

type Cell struct {
	X, Y  int
	Ch    rune
	Color uint32
	Depth float64
	Glow  bool
}
type Frame struct {
	Cells      []Cell
	Cols, Rows int
	BG         uint32
}

func color(p Point, z, t, a float64, s Settings) uint32 {
	u := p.u
	v := .42 + .47*(z/math.Max(.035, math.Hypot(p.x, p.z))+1)/2 + .10*u
	v = math.Min(1, math.Max(.25, v))
	if p.kind == 4 {
		return 0x995B34
	}
	if s.Theme == "Custom" {
		c, _ := hexColor(s.Custom)
		if p.kind == 0 {
			return shade(c, v)
		}
		return shade(c, .76+.24*math.Sin(t*3+u*16)*math.Sin(t*3+u*16)+a*.3)
	}
	if p.kind == 0 {
		h, sat := .36, .82
		switch s.Theme {
		case "Ice Blue":
			h, sat = .56, .74
		case "Cyberpunk":
			h, sat = .79, .79
		case "Gold & White":
			h, sat = .12, .52
		case "Matrix Green":
			h, sat = .32, .91
		}
		return hsv(h+.055*u, sat, v)
	}
	b := math.Min(1, .72+.22*math.Sin(t*5+u*40)+a*.60)
	switch s.Theme {
	case "Rainbow RGB":
		if p.kind == 2 {
			return hsv(.13, .55, b)
		}
		return hsv(u*1.9-t*.32, .78, b)
	case "Classic Christmas":
		return hsv([]float64{0, .12, .34}[int(u*100)%3], .82, b)
	case "Ice Blue":
		return hsv(.53+.11*u, .30+.55*u, b)
	case "Cyberpunk":
		return hsv([]float64{.86, .49}[int(u*9)%2], .85, b)
	case "Gold & White":
		return hsv(.12, .12+.70*u, b)
	default:
		return hsv(.30+.06*u, .85, b)
	}
}
func countdown(now time.Time) string {
	target := time.Date(now.Year(), 12, 25, 0, 0, 0, 0, now.Location())
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if day.Equal(target) {
		return "CHRISTMAS DAY"
	}
	if day.After(target) {
		target = target.AddDate(1, 0, 0)
	}
	days := 0
	for day.Before(target) {
		day = day.AddDate(0, 0, 1)
		days++
	}
	return fmt.Sprintf("CHRISTMAS  D-%d", days)
}
func frame(cols, rows int, t, angle, audio float64, s Settings, stats string) Frame {
	return audioFrame(cols, rows, t, angle, AudioFrame{Level: audio}, s, stats, false)
}
func audioFrame(cols, rows int, t, angle float64, af AudioFrame, s Settings, stats string, overlay bool) Frame {
	var story storyMoment
	if s.Stories {
		story = scheduledStory(time.Duration(t * float64(time.Second)))
	}
	return composeFrame(cols, rows, t, angle, af, s, stats, overlay, story)
}

func composeFrame(cols, rows int, t, angle float64, af AudioFrame, s Settings, stats string, overlay bool, story storyMoment) Frame {
	audio := af.Level
	bass := math.Max(af.Bands[0], af.Bands[1])
	mid := (af.Bands[2] + af.Bands[3] + af.Bands[4]) / 3
	high := (af.Bands[5] + af.Bands[6]) / 2
	bg, _ := hexColor(s.Background)
	f := Frame{nil, cols, rows, bg}
	if cols < 16 || rows < 12 {
		return f
	}
	grid := make([]Cell, cols*rows)
	for i := range grid {
		grid[i].Depth = -1e9
	}
	put := func(x, y int, c rune, col uint32, d float64, g bool) {
		if x >= 0 && x < cols && y >= 0 && y < rows && d >= grid[y*cols+x].Depth {
			grid[y*cols+x] = Cell{x, y, c, col, d, g}
		}
	}
	label := func(y int, str string, col uint32) {
		r := []rune(str)
		width := func(c rune) int {
			if c >= 0x1100 && (c <= 0x115f || (c >= 0x2e80 && c <= 0xa4cf) || (c >= 0xac00 && c <= 0xd7a3) || (c >= 0xf900 && c <= 0xfaff) || (c >= 0xff01 && c <= 0xff60)) {
				return 2
			}
			return 1
		}
		n := 0
		for _, c := range r {
			n += width(c)
		}
		x := (cols - n) / 2
		for _, c := range r {
			if c != ' ' {
				put(x, y, c, col, 1e6, false)
				if width(c) == 2 {
					put(x+1, y, ' ', col, 1e6, false)
				}
			}
			x += width(c)
		}
	}
	if s.Matrix {
		for i := 0; i < cols; i += 3 {
			head := int(math.Mod(t*float64(3+i%5)+8*high*math.Sin(t*.7)+float64(i*19), float64(rows+18)))
			for j := 0; j < 8; j++ {
				c := "01{}<>:;"[(i+j+int(t*4))%8]
				put(i, head-j, rune(c), shade(0x26B878, float64(8-j)/24), -300, false)
			}
		}
	}
	if s.Snow {
		for i := 0; i < 100+int(high*65); i++ {
			u := math.Mod(float64(i)*.6180339887, 1)
			v := math.Mod(float64(i)*.4142135623+t*(.035+float64(i%7)*.013), 1)
			if s.SnowPhysics {
				v = math.Mod(.28*v+.72*v*v+.012*math.Sin(t*.9+float64(i)), 1)
				if v < 0 {
					v++
				}
				u += .04 * math.Sin(t*.7+float64(i))
			}
			u = math.Mod(u+1, 1)
			ch := '.'
			if i%5 == 0 {
				ch = '*'
			}
			put(int(u*float64(cols)), int(v*float64(rows)), ch, 0x789CB8, -200, false)
		}
		if s.SnowPhysics {
			for x := 0; x < cols; x++ {
				h := int(math.Min(2, t/35)) + (x*17%3)/2
				for j := 0; j < h; j++ {
					put(x, rows-1-j, '.', 0x7694AF, -150, false)
				}
			}
		}
	}
	scale := math.Min(float64(rows-9)/3.1, float64(cols-4)/4.6) * float64(s.Size) / 100
	cx, cy := float64(cols-1)/2, float64(rows-1)/2+.2
	ca, sa := math.Cos(angle), math.Sin(angle)
	ct, st := math.Cos(.12), math.Sin(.12)
	project := func(x, y, z float64) (int, int, float64) {
		d := z*ct + y*st
		v := y*ct - z*st
		p := 5 / (5 - d)
		return int(math.Round(cx + x*scale*1.86*p)), int(math.Round(cy - v*scale*p)), d
	}
	lt := t * float64(s.LightSpeed) / 100
	glyphs := ornamentGlyphs[clamp(s.CodeStyle, 0, len(ornamentGlyphs)-1)]
	for _, p := range points {
		if p.kind == 3 && !s.Ornaments {
			continue
		}
		x, z := p.x*ca-p.z*sa, p.x*sa+p.z*ca
		pulse := 1.0
		px, py, d := project(x*pulse, p.y, z*pulse)
		ch := p.ch
		if p.kind == 3 && len(glyphs) > 0 {
			ch = glyphs[int(p.u*float64(len(glyphs)))]
		}
		if p.kind == 1 && math.Sin(lt*6+p.u*40) > .6 {
			ch = 'O'
		}
		put(px, py, rune(ch), color(p, z, lt+mid*.40*math.Sin(p.y*3-t*4), math.Min(1, audio*.45+bass*.20+af.Beat*.25), s), d, p.kind == 1 || p.kind == 3)
	}
	sx, sy, _ := project(0, 1.14, 0)
	for _, q := range []struct {
		x, y int
		c    rune
	}{{0, -2, '|'}, {-1, -1, '\\'}, {0, -1, '*'}, {1, -1, '/'}, {-2, 0, '-'}, {-1, 0, '-'}, {0, 0, '@'}, {1, 0, '-'}, {2, 0, '-'}, {-1, 1, '/'}, {1, 1, '\\'}} {
		put(sx+q.x, sy+q.y, q.c, shade(0xFFE78B, .76+.24*math.Max(high, af.Beat)), 1e5, true)
	}
	if s.Music && s.Spectrum && af.Level > .015 && rows > 22 {
		for i, v := range af.Spectrum {
			a := 2*math.Pi*float64(i)/64 + angle*.25
			x, y, d := project(math.Cos(a)*1.14, -1.28, math.Sin(a)*1.14)
			for j := 0; j <= int(v*4); j++ {
				put(x, y-j, '|', hsv(float64(i)/64, .65, .25+.65*v), d-2, false)
			}
		}
	}
	if af.Beat > .09 && !s.LowPower {
		for i := 0; i < 20; i++ {
			a := float64(i)*2.399 + t*.3
			rad := 1.05 + (1-af.Beat)*.9
			x, y, d := project(math.Cos(a)*rad, -.1+math.Sin(a*2)*.8, math.Sin(a)*rad)
			put(x, y, '.', hsv(float64(i)/20, .55, .65*af.Beat), d-2, true)
		}
	}
	if story.kind != noStory {
		_, trunkY, _ := project(0, trunkBottom, 0)
		drawStory(cols, rows, trunkY, story, put)
	}
	if s.ShowBPM && s.Music && af.BPM > 0 {
		label(rows-4, fmt.Sprintf("~ %.0f BPM", af.BPM), 0x9FB6CF)
	}
	if overlay {
		if rows > 24 {
			label(2, "S: snow  M: code rain  B: glow  G/J: gain  Tab: stats", 0xB3BFCB)
		}
		label(rows-1, "Esc: exit  Space: pause  C: theme  H: hide", 0xB3BFCB)
		label(rows-2, "Arrows: speed/direction  A: audio  P: preset  V: spectrum", 0xB3BFCB)
	}
	if s.Messages {
		label(0, "Merry Christmas", 0xF9D98A)
		msg := s.Message
		if strings.TrimSpace(msg) == "" {
			msg = ""
		}
		label(rows-3, msg, 0x82E8EB)
	}
	if s.Countdown {
		label(1, countdown(time.Now()), 0x9FB6CF)
	}
	if s.Monitor && !overlay {
		label(rows-2, stats, 0xB3BFCB)
	}
	for _, c := range grid {
		if c.Ch != 0 {
			f.Cells = append(f.Cells, c)
		}
	}
	return f
}
