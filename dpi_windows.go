package main

import (
	"math"
	"unsafe"
)

type monitorInfo struct {
	size       uint32
	full, work rect
	flags      uint32
}

type settingControl struct {
	x, y, width, height int
	fontRole, radius    int
}

var settingControls = map[uintptr]*settingControl{}

func settingsWorkArea(monitor uintptr) rect {
	var info monitorInfo
	info.size = uint32(unsafe.Sizeof(info))
	if monitor != 0 && call(u, "GetMonitorInfoW", monitor, ptr(&info)) != 0 {
		return info.work
	}
	var work rect
	call(u, "SystemParametersInfoW", 0x30, 0, ptr(&work), 0)
	return work
}

func settingsScale(work rect, dpi int) float64 {
	if dpi <= 0 {
		dpi = 96
	}
	fit := math.Min(float64(work.R-work.L-8)/1120, float64(work.B-work.T-8)/770)
	return math.Max(.36, math.Min(float64(dpi)/96, fit))
}

func makeSettingsFonts() {
	font = chooseFont("Pretendard", "Segoe UI", 16, 400)
	titleFont = chooseFont("Segoe UI Variable Display", "Segoe UI", 28, 600)
	sectionFont = chooseFont("Pretendard", "Segoe UI", 19, 600)
}

func changeSettingsDPI(h uintptr, dpi int, suggested rect) {
	monitor := call(u, "MonitorFromRect", ptr(&suggested), 2)
	work := settingsWorkArea(monitor)
	scale := settingsScale(work, dpi)
	if uiScale != scale {
		uiScale = scale
		oldFonts := []uintptr{font, titleFont, sectionFont}
		makeSettingsFonts()
		faces := []uintptr{font, titleFont, sectionFont}
		for child, layout := range settingControls {
			call(u, "MoveWindow", child, signed(sc(layout.x)), signed(sc(layout.y)), uintptr(sc(layout.width)), uintptr(sc(layout.height)), 0)
			send(child, wmSetFont, faces[layout.fontRole], 0)
			if layout.radius > 0 {
				roundRegion(child, layout.width, layout.height, layout.radius)
			}
		}
		for _, id := range []int{idMessage, idCoupleFirst, idCoupleSecond, idCoupleDate} {
			send(controls[id], 0xd3, 1, uintptr(sc(10)))
		}
		for _, old := range oldFonts {
			call(g, "DeleteObject", old)
		}
	}
	width, height := sc(1120), sc(770)
	x := clamp(int(suggested.L), int(work.L), max(int(work.L), int(work.R)-width))
	y := clamp(int(suggested.T), int(work.T), max(int(work.T), int(work.B)-height))
	call(u, "SetWindowPos", h, 0, signed(x), signed(y), uintptr(width), uintptr(height), 0x14)
	if coupleCalendarOpen {
		positionCoupleCalendar()
	}
	call(u, "RedrawWindow", h, 0, 0, 0x185)
}
