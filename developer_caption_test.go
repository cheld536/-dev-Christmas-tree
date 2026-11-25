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
			code, comment := developerCaption(tc.now, tc.enabled, tc.countdown, tc.paused, tc.event, christmas.Add(time.Second))
			if code != tc.code || comment != tc.comment {
				t.Fatalf("문구 = (%q, %q), 기댓값 = (%q, %q)", code, comment, tc.code, tc.comment)
			}
		})
	}
}
