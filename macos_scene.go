package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"
)

type macFrameRequest struct {
	Columns, Rows  int
	Elapsed, Angle float64
	StoryPreview   int
	StoryElapsed   float64
	AudioPCM       []byte
	AudioRate      int
	AudioEnabled   bool
	Equalizer      string
	Settings       Settings
}

type macCell struct {
	X, Y  int
	Glyph string
	Color uint32
	Glow  bool
}

type macFrame struct {
	Columns, Rows int
	Background    uint32
	Cells         []macCell
	Greeting      string
	FirstName     string
	SecondName    string
	Counter       string
	CoupleError   string
	AudioActive   bool
	AudioBars     []float64
}

func makeMacFrame(request macFrameRequest, now time.Time, audio AudioFrame) macFrame {
	settings := normalize(request.Settings)
	cols, rows := clamp(request.Columns, 16, 220), clamp(request.Rows, 12, 100)
	elapsed := math.Mod(math.Max(0, request.Elapsed), 24*60*60)
	duration := time.Duration(elapsed * float64(time.Second))
	var story storyMoment
	if settings.Stories {
		story = scheduledStory(duration)
	}
	if request.StoryPreview >= int(sleighStory) && request.StoryPreview <= int(lastStory) {
		kind := storyKind(request.StoryPreview)
		if request.StoryElapsed >= 0 && request.StoryElapsed < kind.duration().Seconds() {
			story = storyDuring(kind, time.Duration(request.StoryElapsed*float64(time.Second)))
		}
	}
	sceneSettings := settings
	sceneSettings.Message = ""
	audio = reactive(settings, audio)
	frame := composeFrame(cols, rows, elapsed, math.Mod(request.Angle, 2*math.Pi), audio, sceneSettings, "", false, story)
	result := macFrame{Columns: cols, Rows: rows, Background: frame.BG, Cells: make([]macCell, 0, len(frame.Cells))}
	result.AudioActive = audio.Active
	if settings.Music && settings.Spectrum {
		result.AudioBars = audio.Spectrum[:]
	}
	for _, cell := range frame.Cells {
		if cell.Ch != ' ' {
			result.Cells = append(result.Cells, macCell{cell.X, cell.Y, string(cell.Ch), cell.Color, cell.Glow && settings.Bloom})
		}
	}
	if couple, err := coupleCaption(settings.Couple, now); err != nil {
		result.CoupleError = err.Error()
	} else if couple.firstName != "" {
		result.FirstName, result.SecondName, result.Counter = couple.firstName, couple.secondName, couple.counter
	}
	if result.FirstName == "" {
		result.Greeting, _ = developerCaption(now, duration, settings, false, captionNone, time.Time{})
	}
	return result
}

func serveMacFrames(input io.Reader, output io.Writer, now func() time.Time) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	var monitor analyzer
	idleFrames := 0
	for scanner.Scan() {
		request := macFrameRequest{Settings: defaults()}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			return fmt.Errorf("macOS 프레임 요청: %w", err)
		}
		request.Settings.Music = request.AudioEnabled
		request.Settings.Spectrum = request.Equalizer == "bars"
		request.Settings.RingSpectrum = request.Equalizer == "ring"
		if request.Settings.Music && request.AudioRate >= 8000 && request.AudioRate <= 96000 && len(request.AudioPCM) >= 4 {
			idleFrames = 0
			if monitor.rate != request.AudioRate {
				monitor = analyzer{rate: request.AudioRate}
			}
			samples := request.AudioPCM
			for len(samples) >= 4 {
				monitor.push(float64(math.Float32frombits(binary.LittleEndian.Uint32(samples[:4]))))
				samples = samples[4:]
			}
		} else if request.Settings.Music {
			idleFrames++
			if idleFrames >= 3 {
				monitor.silence()
			}
		} else {
			monitor = analyzer{}
			idleFrames = 0
		}
		if err := encoder.Encode(makeMacFrame(request, now(), monitor.smooth)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
