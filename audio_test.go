package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestFFTBandSeparation(t *testing.T) {
	for _, rate := range []int{44100, 48000} {
		for _, v := range []struct {
			hz   float64
			band int
		}{{40, 0}, {120, 1}, {360, 2}, {1100, 3}, {3000, 4}, {5200, 5}, {9500, 6}} {
			a := analyzer{rate: rate}
			for i := 0; i < rate/2; i++ {
				a.push(.16 * math.Sin(2*math.Pi*v.hz*float64(i)/float64(rate)))
			}
			max := 0
			for i, x := range a.smooth.Bands {
				if x > a.smooth.Bands[max] {
					max = i
				}
			}
			if max != v.band {
				t.Fatalf("%d Hz %v expected %d got %d: %v", rate, v.hz, v.band, max, a.smooth.Bands)
			}
			if !a.smooth.Active || a.smooth.Level <= .1 {
				t.Fatal("no signal")
			}
		}
	}
}
func TestSilenceAndGain(t *testing.T) {
	a := analyzer{rate: 48000}
	for i := 0; i < 24000; i++ {
		a.push(.01 * math.Sin(2*math.Pi*120*float64(i)/48000))
	}
	if a.smooth.Bands[1] < .2 {
		t.Fatal("quiet tone not normalized")
	}
	for i := 0; i < 48000; i++ {
		a.push(0)
	}
	if a.smooth.Active || a.smooth.Level > .01 || a.smooth.Bands[1] > .01 {
		t.Fatalf("silence %+v", a.smooth)
	}
	for i := 0; i < fftSize; i++ {
		a.push(math.NaN())
	}
	if math.IsNaN(a.smooth.Level) {
		t.Fatal("NaN propagation")
	}
}
func TestBeatAndTempo(t *testing.T) {
	a := analyzer{rate: 48000}
	hits := 0
	last := 0.
	for i := 0; i < 48000*7; i++ {
		sec := float64(i) / 48000
		phase := math.Mod(sec, .5)
		x := 0.
		if phase < .065 {
			x = .65 * math.Exp(-phase*42) * math.Sin(2*math.Pi*110*sec)
		}
		a.push(x)
		if a.lastBeat != last {
			hits++
			last = a.lastBeat
		}
	}
	if hits < 8 || hits > 16 {
		t.Fatalf("beat count %d", hits)
	}
	if math.Abs(a.smooth.BPM-120) > 12 {
		t.Fatalf("tempo %v", a.smooth.BPM)
	}
}
func TestAudioReactiveRendering(t *testing.T) {
	s := defaults()
	af := AudioFrame{Level: .8, Beat: .9, Active: true, BPM: 120}
	af.Bands = [7]float64{.9, .7, .3, .5, .4, .8, .9}
	for i := range af.Spectrum {
		af.Spectrum[i] = float64(i) / 64
	}
	a := reactive(s, af)
	if a.Beat >= af.Beat {
		t.Fatal("safe mode not applied")
	}
	bar := audioFrame(120, 52, 1, .4, a, s, "", false)
	if reflect.DeepEqual(bar, audioFrame(120, 52, 1, .4, AudioFrame{}, s, "", false)) {
		t.Fatal("no music reaction")
	}
	s.Spectrum = false
	off := audioFrame(120, 52, 1, .4, a, s, "", false)
	if !reflect.DeepEqual(bar, off) {
		t.Fatal("하단 막대 설정이 트리 장면을 변경합니다.")
	}
	s.Spectrum, s.RingSpectrum = true, true
	s = normalize(s)
	if s.Spectrum || !s.RingSpectrum {
		t.Fatal("트리 둘레 이퀄라이저를 켜면 하단 막대가 꺼져야 합니다.")
	}
	if reflect.DeepEqual(off, audioFrame(120, 52, 1, .4, a, s, "", false)) {
		t.Fatal("음악에 반응하는 트리 둘레 막대가 그려지지 않습니다.")
	}
	if !reflect.DeepEqual(audioFrame(120, 52, 1, .4, AudioFrame{}, s, "", false),
		audioFrame(120, 52, 1, .4, AudioFrame{}, defaults(), "", false)) {
		t.Fatal("무음일 때 트리 둘레 막대가 표시됩니다.")
	}
	s.Music = false
	if reactive(s, af) != (AudioFrame{}) {
		t.Fatal("music disabled")
	}
	if len(audioPresets) != 10 {
		t.Fatal("presets")
	}
}
func TestUpgradeSettings(t *testing.T) {
	s := defaults()
	if e := json.Unmarshal([]byte(`{"Theme":"Ice Blue","Speed":180,"Music":false,"Message":"snow"}`), &s); e != nil {
		t.Fatal(e)
	}
	s = normalize(s)
	if s.Theme != "Ice Blue" || s.Speed != 180 || s.Music || s.AudioGain != 100 || s.AudioPreset != "자동" || !s.SafeLights {
		t.Fatalf("migration %+v", s)
	}
}
