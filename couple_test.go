package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFitCoupleNamesSharesTheAvailableSpace(t *testing.T) {
	for _, tc := range []struct {
		first, second, budget int32
		left, right           int32
	}{
		{20, 25, 46, 20, 25},
		{70, 80, 41, 20, 21},
		{70, 10, 43, 33, 10},
		{9, 80, 42, 9, 33},
		{10, 10, -1, 0, 0},
	} {
		left, right := fitCoupleNames(tc.first, tc.second, tc.budget)
		if left != tc.left || right != tc.right {
			t.Fatalf("너비 %d, %d를 %d에 맞춘 결과 = (%d, %d), 기댓값 = (%d, %d)", tc.first, tc.second, tc.budget, left, right, tc.left, tc.right)
		}
	}
}

func TestCoupleCaptionCalendarDays(t *testing.T) {
	tests := []struct {
		name, start string
		now         time.Time
		want        string
	}{
		{"첫날", "2026-12-01", time.Date(2026, 12, 1, 23, 59, 59, 0, time.FixedZone("KST", 9*60*60)), "D+1일"},
		{"자정", "2026-12-01", time.Date(2026, 12, 2, 0, 0, 0, 0, time.FixedZone("KST", 9*60*60)), "D+2일"},
		{"윤년", "2024-02-28", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), "D+3일"},
		{"연도 변경", "2025-12-31", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "D+2일"},
		{"서머타임 날짜", "2026-03-08", time.Date(2026, 3, 9, 0, 30, 0, 0, time.FixedZone("EDT", -4*60*60)), "D+2일"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			display, err := coupleCaption(CoupleSettings{true, "지우", "하늘", tc.start}, tc.now)
			if err != nil || display.text() != "지우 ♥️ 하늘  "+tc.want {
				t.Fatalf("기념일 표시 = %q, 오류 = %v", display.text(), err)
			}
		})
	}
}

func TestCoupleCaptionValidationAndSettings(t *testing.T) {
	now := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	for _, settings := range []CoupleSettings{
		{true, "", "하늘", "2026-11-24"},
		{true, "지우", "하늘", "2026-11-26"},
		{true, "지우", "하늘", "2026-02-29"},
		{true, "지우", "하늘", "0000-01-01"},
	} {
		if display, err := coupleCaption(settings, now); err == nil || display.text() != "" {
			t.Fatalf("유효하지 않은 설정을 표시했습니다. %v, %v", display, err)
		}
	}
	old := defaults()
	if err := json.Unmarshal([]byte(`{"Theme":"Ice Blue"}`), &old); err != nil {
		t.Fatal(err)
	}
	if display, err := coupleCaption(old.Couple, now); err != nil || display.text() != "" {
		t.Fatal("기존 설정의 기념일 기본값은 꺼져 있어야 합니다.")
	}
	s := defaults()
	s.Couple = CoupleSettings{true, "  " + strings.Repeat("별", maxCoupleName+2), " 하늘 ", " 2026-11-24 "}
	s = normalize(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var saved Settings
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	display, err := coupleCaption(saved.Couple, now)
	if err != nil || display.text() != strings.Repeat("별", maxCoupleName)+" ♥️ 하늘  D+2일" {
		t.Fatalf("저장된 이름과 기념일 불일치. %q, %v", display.text(), err)
	}
}
