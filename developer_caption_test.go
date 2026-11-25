package main

import (
	"testing"
	"time"
)

func TestDeveloperCaption(t *testing.T) {
	christmas := time.Date(2026, time.December, 25, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name                       string
		now                        time.Time
		enabled, countdown, paused bool
		event                      captionEvent
		code, comment              string
	}{
		{"paused", christmas, true, true, true, captionSnow, "breakpoint()", "코코아 충전 중"},
		{"resumed", christmas, true, true, false, captionResumed, "continue", "코코아 충전 완료"},
		{"snow", christmas, true, true, false, captionSnow, "snow += 1", "누적되는 건 눈만"},
		{"expired", christmas.Add(time.Second), true, true, false, captionSnow, `assert presents, "선물 누락"`, ""},
		{"next day", christmas.AddDate(0, 0, 1), true, true, false, captionNone, "while True: spread_joy()", ""},
		{"countdown off", christmas, true, false, false, captionNone, "while True: spread_joy()", ""},
		{"hidden", christmas, false, true, true, captionSnow, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := Settings{Messages: tc.enabled, Countdown: tc.countdown}
			code, comment := developerCaption(tc.now, 0, settings, tc.paused, tc.event, christmas.Add(time.Second))
			if code != tc.code || comment != tc.comment {
				t.Fatalf("문구 = (%q, %q), 기댓값 = (%q, %q)", code, comment, tc.code, tc.comment)
			}
		})
	}
}

func TestDefaultAndCustomDeveloperCaptions(t *testing.T) {
	now := time.Date(2026, time.November, 26, 0, 0, 0, 0, time.Local)
	settings := Settings{Messages: true, Message: " \t "}
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "while True: spread_joy()"},
		{3*time.Second - time.Millisecond, "while True: spread_joy()"},
		{3 * time.Second, `git commit -m "Christmas"`},
		{6 * time.Second, `print("Hello, Christmas!")`},
		{9 * time.Second, "404: bugs not found"},
		{12 * time.Second, "while True: spread_joy()"},
	} {
		if got, _ := developerCaption(now, tc.elapsed, settings, false, captionNone, time.Time{}); got != tc.want {
			t.Fatalf("%v 이후 기본 문구 = %q, 기댓값 = %q", tc.elapsed, got, tc.want)
		}
	}
	settings.Message = "  우리만의 크리스마스  "
	if got, _ := developerCaption(now, 6*time.Second, settings, true, captionSnow, now.Add(time.Second)); got != "우리만의 크리스마스" {
		t.Fatalf("사용자 문구 = %q", got)
	}
	settings.Messages = false
	if got, _ := developerCaption(now, 6*time.Second, settings, false, captionNone, time.Time{}); got != "" {
		t.Fatalf("인사말을 끈 상태에서 문구 표시 = %q", got)
	}
}
