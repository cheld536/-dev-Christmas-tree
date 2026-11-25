package main

import (
	"strings"
	"time"
)

type captionEvent uint8

const (
	captionNone captionEvent = iota
	captionResumed
	captionSnow
)

var defaultDeveloperCaptions = [...]string{
	"while True: spread_joy()",
	`git commit -m "Christmas"`,
	`print("Hello, Christmas!")`,
	"404: bugs not found",
	"git switch -c holiday",
	"debug(santa.route)",
	"coffee = cocoa",
	"return cookies, 200",
	`deploy("snow")`,
	"CRON: Santa at midnight",
}

func developerCaption(now time.Time, elapsed time.Duration, settings Settings, paused bool, event captionEvent, until time.Time) (code, comment string) {
	custom := strings.TrimSpace(settings.Message)
	switch {
	case !settings.Messages:
		return "", ""
	case custom != "":
		return custom, ""
	case paused:
		return "breakpoint()", "코코아 충전 중"
	case now.Before(until) && event == captionResumed:
		return "continue", "코코아 충전 완료"
	case now.Before(until) && event == captionSnow:
		return "snow += 1", "누적되는 건 눈만"
	}
	index := int(max(0, elapsed) / (3 * time.Second))
	if settings.Countdown && now.Month() == time.December && now.Day() == 25 {
		index %= len(defaultDeveloperCaptions) + 1
		if index == 0 {
			return `assert presents, "선물 누락"`, ""
		}
		index--
	}
	return defaultDeveloperCaptions[index%len(defaultDeveloperCaptions)], ""
}
