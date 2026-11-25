package main

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	calendarGetDate = 0x1001
	calendarSetDate = 0x1002
	calendarGetSize = 0x1009
	calendarRange   = 0x1012
	calendarSelect  = -746
)

type calendarDate struct {
	year, month, weekday, day, hour, minute, second, millis uint16
}

type calendarNotice struct {
	window, id uintptr
	code       int32
}

var coupleCalendarOpen bool

func setupCoupleCalendar() {
	classes := struct{ size, flags uint32 }{8, 0x100}
	call(syscall.NewLazyDLL("comctl32.dll"), "InitCommonControlsEx", ptr(&classes))
	h := child("SysMonthCal32", "", wsTab|wsBorder|0x10, 649, 398, 1, 1, idCoupleCalendar)
	if h == 0 {
		call(u, "EnableWindow", controls[idCoupleCalendarButton], 0)
	} else {
		call(u, "ShowWindow", h, 0)
	}
}

func positionCoupleCalendar() {
	h := controls[idCoupleCalendar]
	var size rect
	if send(h, calendarGetSize, 0, ptr(&size)) == 0 {
		return
	}
	width := max(int(size.R-size.L)+2, sc(210))
	height := int(size.B-size.T) + 2
	top := min(sc(398), sc(648)-height)
	call(u, "SetWindowPos", h, 0, signed(sc(1076)-width), signed(top), uintptr(width), uintptr(height), 0x10)
}

func setCoupleCalendarVisible(show bool) {
	h := controls[idCoupleCalendar]
	if h == 0 || coupleCalendarOpen == show {
		return
	}
	coupleCalendarOpen = show
	if show {
		today := time.Now()
		bounds := [2]calendarDate{{}, toCalendarDate(today)}
		send(h, calendarRange, 2, ptr(&bounds))
		selected, err := time.Parse(time.DateOnly, txt(controls[idCoupleDate]))
		if err != nil || selected.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)) {
			selected = today
		}
		selectedDate := toCalendarDate(selected)
		if send(h, calendarSetDate, 0, ptr(&selectedDate)) == 0 {
			todayDate := toCalendarDate(today)
			send(h, calendarSetDate, 0, ptr(&todayDate))
		}
		positionCoupleCalendar()
		showCoupleDateHints(0)
		set(controls[idCoupleCalendarButton], "달력 닫기")
		call(u, "ShowWindow", h, 5)
		call(u, "SetFocus", h)
		return
	}
	call(u, "ShowWindow", h, 0)
	if currentPage == 3 {
		showCoupleDateHints(5)
	}
	set(controls[idCoupleCalendarButton], "달력에서 선택")
}

func showCoupleDateHints(visibility uintptr) {
	for _, id := range []int{idCoupleDayHint, idCoupleStatus, idCouplePrivacy} {
		call(u, "ShowWindow", controls[id], visibility)
	}
}

func toCalendarDate(date time.Time) calendarDate {
	return calendarDate{year: uint16(date.Year()), month: uint16(date.Month()), day: uint16(date.Day())}
}

func closeCoupleCalendar() {
	setCoupleCalendarVisible(false)
	call(u, "SetFocus", controls[idCoupleCalendarButton])
}

func chooseCoupleCalendarDate() {
	h := controls[idCoupleCalendar]
	var date calendarDate
	if send(h, calendarGetDate, 0, ptr(&date)) != 0 {
		selected := time.Date(int(date.year), time.Month(date.month), int(date.day), 0, 0, 0, 0, time.UTC)
		set(controls[idCoupleDate], selected.Format(time.DateOnly))
	}
	closeCoupleCalendar()
}

func coupleCalendarNotify(lParam uintptr) bool {
	if !coupleCalendarOpen || lParam == 0 {
		return false
	}
	event := (*calendarNotice)(unsafe.Pointer(lParam))
	if event.window != controls[idCoupleCalendar] || event.id != idCoupleCalendar || event.code != calendarSelect {
		return false
	}
	chooseCoupleCalendarDate()
	return true
}

func coupleCalendarKey(message msg) bool {
	if !coupleCalendarOpen || message.M != wmKey {
		return false
	}
	if message.W == 0x1B {
		closeCoupleCalendar()
		return true
	}
	if message.H == controls[idCoupleCalendar] && message.W == 0x0D {
		chooseCoupleCalendarDate()
		return true
	}
	return false
}
