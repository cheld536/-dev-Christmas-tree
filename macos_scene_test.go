package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestMacFrameStream(t *testing.T) {
	now := time.Date(2026, time.November, 26, 12, 0, 0, 0, time.Local)
	input := bytes.NewBufferString(
		"{\"Columns\":120,\"Rows\":42}\n" +
			"{\"Columns\":120,\"Rows\":42,\"StoryPreview\":1,\"StoryElapsed\":4," +
			"\"Settings\":{\"Stories\":false,\"Bloom\":false,\"Couple\":{\"Enabled\":true," +
			"\"FirstName\":\"지우\",\"SecondName\":\"하늘\",\"StartDate\":\"2026-11-26\"}}}\n",
	)
	var output bytes.Buffer
	if err := serveMacFrames(input, &output, func() time.Time { return now }); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var first, second macFrame
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if first.Columns != 120 || first.Rows != 42 || len(first.Cells) == 0 || first.Greeting != "while True: spread_joy()" {
		t.Fatal("macOS 기본 프레임의 크기, 문자 또는 인사말이 잘못되었습니다.")
	}
	if second.FirstName != "지우" || second.SecondName != "하늘" || second.Counter != "D+1일" || second.Greeting != "" {
		t.Fatal("macOS에서 커플 기념일이 기본 인사말보다 우선하여 표시되지 않았습니다.")
	}
	hasSleigh := false
	for _, cell := range second.Cells {
		if cell.Glow {
			t.Fatal("빛 번짐을 끈 프레임에 빛 번짐 표시가 있습니다.")
		}
		hasSleigh = hasSleigh || (cell.Glyph == "o" && cell.Color == rudolphRed)
	}
	if !hasSleigh {
		t.Fatal("자동 재생을 꺼도 요청한 썰매 장면이 미리보기에 나와야 합니다.")
	}
}

func TestMacAudioStream(t *testing.T) {
	const rate = 48000
	input := new(bytes.Buffer)
	encoder := json.NewEncoder(input)
	note := make([]byte, 4096*4)
	for i := 0; i < len(note)/4; i++ {
		sample := float32(math.Sin(2*math.Pi*440*float64(i)/rate) * .2)
		binary.LittleEndian.PutUint32(note[4*i:], math.Float32bits(sample))
	}
	for _, style := range []string{"bars", "ring", "off"} {
		if err := encoder.Encode(map[string]any{
			"Columns": 120, "Rows": 42, "AudioEnabled": true,
			"AudioPCM": note, "AudioRate": rate, "Equalizer": style,
		}); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := serveMacFrames(input, &output, time.Now); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	for _, style := range []string{"bars", "ring", "off"} {
		var frame macFrame
		if err := decoder.Decode(&frame); err != nil {
			t.Fatal(err)
		}
		if !frame.AudioActive || (len(frame.AudioBars) == 64) != (style == "bars") {
			t.Fatalf("%s 모드의 시스템 오디오 또는 하단 막대 상태가 잘못되었습니다.", style)
		}
	}
}
