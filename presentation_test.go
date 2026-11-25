package main

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCodeStyleChangesTreeOrnaments(t *testing.T) {
	settings := defaults()
	settings.Snow, settings.Matrix, settings.Countdown, settings.Messages = false, false, false, false
	python := frame(100, 48, 1, .4, 0, settings, "")
	for style := 1; style < len(codeThemes); style++ {
		settings.CodeStyle = style
		styled := frame(100, 48, 1, .4, 0, settings, "")
		if slices.Equal(python.Cells, styled.Cells) {
			t.Fatalf("%s 테마에서 코드 장식이 바뀌지 않았습니다.", codeThemes[style])
		}
	}
}

func TestFullscreenResolution(t *testing.T) {
	for _, tc := range []struct {
		choice        string
		width, height int
	}{
		{fullscreenResolutions[0], 2560, 1600},
		{"1080p", 1920, 1080},
		{"4K", 3840, 2160},
		{"알 수 없는 값", 2560, 1600},
	} {
		w, h := fullscreenSize(tc.choice, 2560, 1600)
		if w != tc.width || h != tc.height {
			t.Fatalf("%q 렌더링 크기 = %d×%d, 기댓값 = %d×%d", tc.choice, w, h, tc.width, tc.height)
		}
	}
	for _, raw := range []string{`{"Theme":"Classic Christmas"}`, `{"FullscreenResolution":"기타"}`} {
		s := defaults()
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatal(err)
		}
		if normalize(s).FullscreenResolution != fullscreenResolutions[0] {
			t.Fatal("이전 설정은 모니터의 기본 해상도를 사용해야 합니다.")
		}
	}
	if w, h := fitFrame(3840, 2160, 2560, 1600); w != 2560 || h != 1440 {
		t.Fatalf("16:10 모니터에 맞춘 4K 화면 = %d×%d", w, h)
	}
	if h := footerHeight(260, 170, 19, true, true, false); h != 38 {
		t.Fatalf("좁은 전체 화면의 하단 높이 = %d", h)
	}
	if h := footerHeight(580, 432, 19, true, true, true); h != 38 {
		t.Fatalf("기념일과 종료 안내용 하단 높이 = %d", h)
	}
}
