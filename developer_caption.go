package main

import "time"

type captionEvent uint8

const (
	captionNone captionEvent = iota
	captionResumed
	captionSnow
)

func developerCaption(now time.Time, enabled, countdown, paused bool, event captionEvent, until time.Time) (code, comment string) {
	switch {
	case !enabled:
		return "", ""
	case paused:
		return "breakpoint()", "코코아 충전 중"
	case now.Before(until) && event == captionResumed:
		return "continue", "코코아 충전 완료"
	case now.Before(until) && event == captionSnow:
		return "snow += 1", "누적되는 건 눈만"
	case countdown && now.Month() == time.December && now.Day() == 25:
		return `assert presents, "선물 누락"`, ""
	default:
		return "while True: spread_joy()", ""
	}
}
