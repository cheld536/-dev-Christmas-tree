package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxCoupleName = 16

type CoupleSettings struct {
	Enabled               bool
	FirstName, SecondName string
	StartDate             string
}

type coupleDisplay struct {
	firstName, secondName, counter string
}

func (c coupleDisplay) text() string {
	if c.firstName == "" {
		return ""
	}
	return c.firstName + " ♥️ " + c.secondName + "  " + c.counter
}

func fitCoupleNames(first, second, budget int32) (int32, int32) {
	budget = max(0, budget)
	left := min(first, max(budget/2, budget-second))
	return left, min(second, budget-left)
}

func normalizeCouple(c CoupleSettings) CoupleSettings {
	normalizeName := func(s string) string {
		s = strings.Join(strings.Fields(s), " ")
		n := 0
		for i := range s {
			if n == maxCoupleName {
				return s[:i]
			}
			n++
		}
		return s
	}
	c.FirstName = normalizeName(c.FirstName)
	c.SecondName = normalizeName(c.SecondName)
	c.StartDate = strings.TrimSpace(c.StartDate)
	return c
}

func coupleCaption(c CoupleSettings, now time.Time) (coupleDisplay, error) {
	if !c.Enabled {
		return coupleDisplay{}, nil
	}
	if c.FirstName == "" || c.SecondName == "" {
		return coupleDisplay{}, errors.New("두 이름을 모두 입력하세요.")
	}
	start, err := time.Parse(time.DateOnly, c.StartDate)
	if err != nil || start.Year() < 1 {
		return coupleDisplay{}, errors.New("시작일을 YYYY-MM-DD 형식으로 입력하세요.")
	}
	year, month, day := now.Date()
	today := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if start.After(today) {
		return coupleDisplay{}, errors.New("시작일은 오늘이나 이전 날짜를 선택하세요.")
	}
	const secondsPerDay = 24 * 60 * 60
	days := (today.Unix()-start.Unix())/secondsPerDay + 1
	return coupleDisplay{firstName: c.FirstName, secondName: c.SecondName, counter: fmt.Sprintf("D+%d일", days)}, nil
}
