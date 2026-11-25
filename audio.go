package main

import (
	"math"
	"math/cmplx"
)

const fftSize = 2048
const hopSize = 512

var audioPresets = []string{"자동", "캐럴", "EDM", "사이버펑크", "재즈", "로파이", "클래식", "매트릭스", "갤럭시", "직접 설정"}

type AudioFrame struct {
	Bands             [7]float64
	Spectrum          [64]float64
	Level, Beat, BPM  float64
	Active, Connected bool
}
type analyzer struct {
	ring                          [fftSize]float64
	pos, count, hop, rate         int
	prev                          [fftSize / 2]float64
	smooth                        AudioFrame
	floor, gain, fluxAvg, fluxDev float64
	lastBeat                      float64
	clock                         float64
	intervals                     []float64
}

func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		w := cmplx.Rect(1, -2*math.Pi/float64(l))
		for i := 0; i < n; i += l {
			z := complex(1, 0)
			for j := 0; j < l/2; j++ {
				u, v := a[i+j], a[i+j+l/2]*z
				a[i+j], a[i+j+l/2] = u+v, u-v
				z *= w
			}
		}
	}
}
func limit(x float64) float64 { return math.Max(0, math.Min(1, x)) }
func (a *analyzer) push(x float64) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		x = 0
	}
	x = math.Max(-1, math.Min(1, x))
	a.ring[a.pos] = x
	a.pos = (a.pos + 1) % fftSize
	a.count++
	a.hop++
	if a.count >= fftSize && a.hop >= hopSize {
		a.hop = 0
		a.analyze()
	}
}
func (a *analyzer) analyze() {
	if a.rate <= 0 {
		return
	}
	dt := float64(hopSize) / float64(a.rate)
	a.clock += dt
	var bins [fftSize]complex128
	var rms float64
	for i := 0; i < fftSize; i++ {
		x := a.ring[(a.pos+i)%fftSize]
		rms += x * x
		bins[i] = complex(x*(.5-.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))), 0)
	}
	rms = math.Sqrt(rms / fftSize)
	a.smooth.Active = rms > 0.00035
	if a.gain == 0 {
		a.gain = .03
	}
	a.gain = math.Max(.003, math.Max(rms, a.gain*math.Exp(-dt/1.8)))
	fft(bins[:])
	var energy [7]float64
	var spec [64]float64
	var flux float64
	edges := [8]float64{20, 60, 250, 500, 2000, 4000, 6000, 20000}
	for i := 1; i < fftSize/2; i++ {
		hz := float64(i*a.rate) / fftSize
		if hz < 20 || hz > 20000 {
			continue
		}
		m := cmplx.Abs(bins[i]) * 4 / fftSize
		for b := 0; b < 7; b++ {
			if hz >= edges[b] && hz < edges[b+1] {
				energy[b] += m * m
				break
			}
		}
		j := clamp(int(64*math.Log(hz/20)/math.Log(1000)), 0, 63)
		spec[j] = math.Max(spec[j], m)
		v := math.Log1p(m * 100)
		flux += math.Max(0, v-a.prev[i])
		a.prev[i] = v
	}
	alpha := 1 - math.Exp(-dt/.075)
	gain := math.Max(.003, a.gain)
	for i, v := range energy {
		target := limit(math.Sqrt(v) / gain * .85)
		if !a.smooth.Active {
			target = 0
		}
		a.smooth.Bands[i] += (target - a.smooth.Bands[i]) * alpha
	}
	for i, v := range spec {
		target := limit(v / gain * .9)
		if !a.smooth.Active {
			target = 0
		}
		a.smooth.Spectrum[i] += (target - a.smooth.Spectrum[i]) * alpha
	}
	target := limit(rms / gain)
	if !a.smooth.Active {
		target = 0
	}
	a.smooth.Level += (target - a.smooth.Level) * alpha
	a.smooth.Beat *= math.Exp(-dt / .22)
	threshold := a.fluxAvg + math.Max(.15, 2.1*a.fluxDev)
	if a.smooth.Active && a.clock > .22 && flux > threshold && a.clock-a.lastBeat > .34 {
		if a.lastBeat > 0 {
			d := a.clock - a.lastBeat
			if d >= .34 && d <= 1.5 {
				a.intervals = append(a.intervals, d)
				if len(a.intervals) > 8 {
					a.intervals = a.intervals[1:]
				}
				if len(a.intervals) >= 3 {
					var sum float64
					for _, x := range a.intervals {
						sum += x
					}
					bpm := 60 * float64(len(a.intervals)) / sum
					if bpm < 70 {
						bpm *= 2
					}
					a.smooth.BPM = bpm
				}
			}
		}
		a.lastBeat = a.clock
		a.smooth.Beat = 1
	}
	a.fluxAvg += (flux - a.fluxAvg) * .035
	a.fluxDev += (math.Abs(flux-a.fluxAvg) - a.fluxDev) * .035
	if a.clock-a.lastBeat > 3 {
		a.smooth.BPM = 0
		a.intervals = nil
	}
	a.smooth.Connected = true
}
func (a *analyzer) silence() {
	for i := range a.smooth.Bands {
		a.smooth.Bands[i] *= .85
	}
	for i := range a.smooth.Spectrum {
		a.smooth.Spectrum[i] *= .85
	}
	a.smooth.Level *= .85
	a.smooth.Beat *= .75
	a.smooth.Active = false
	a.smooth.BPM = 0
}
func audioStyle(s Settings) (bass, mid, high, beat float64) {
	switch s.AudioPreset {
	case "캐럴":
		return .55, .65, .45, .4
	case "EDM":
		return 1.35, .85, 1.15, 1.3
	case "사이버펑크":
		return .95, 1.2, .9, 1
	case "재즈":
		return .45, .8, .5, .35
	case "로파이":
		return .35, .45, .3, .25
	case "클래식":
		return .55, .9, .8, .4
	case "매트릭스":
		return .7, .7, 1.3, .7
	case "갤럭시":
		return .8, 1.1, 1.3, .9
	case "직접 설정":
		return float64(s.BassGain) / 100, float64(s.MidGain) / 100, float64(s.HighGain) / 100, 1
	default:
		return .85, .85, .85, .8
	}
}
func reactive(s Settings, a AudioFrame) AudioFrame {
	if !s.Music {
		return AudioFrame{}
	}
	b, m, h, bt := audioStyle(s)
	g := float64(s.AudioGain) / 100
	for i := range a.Bands {
		k := m
		if i < 2 {
			k = b
		}
		if i >= 4 {
			k = h
		}
		a.Bands[i] = limit(a.Bands[i] * g * k)
	}
	for i := range a.Spectrum {
		a.Spectrum[i] = limit(a.Spectrum[i] * g)
	}
	a.Level = limit(a.Level * g)
	a.Beat = limit(a.Beat * g * bt)
	if s.SafeLights {
		a.Beat *= .35
	}
	return a
}
