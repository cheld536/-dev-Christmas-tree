package main

import "time"

func setupCouplePage() {
	heading("커플 기념일", 649, 127, 420, 28, sectionFont)
	check(idCoupleEnabled, 649, 167, "커플 기념일 표시")
	edit := func(id, top, width, length int, title string) {
		label(title, 649, top, 400, 23)
		h := child("EDIT", "", wsTab|0x80, 649, top+26, width, 34, id)
		send(h, 0xc5, uintptr(length), 0)
		send(h, 0xd3, 1, uintptr(sc(10)))
		roundRegion(h, width, 34, 12)
	}
	edit(idCoupleFirst, 200, 427, 2*maxCoupleName, "첫 번째 이름")
	edit(idCoupleSecond, 264, 427, 2*maxCoupleName, "두 번째 이름")
	edit(idCoupleDate, 328, 271, len(time.DateOnly), "시작일 (YYYY-MM-DD)")
	button(idCoupleCalendarButton, 929, 353, 147, "달력에서 선택")
	child("STATIC", "시작일을 D+1일로 계산합니다.", 0, 649, 410, 427, 24, idCoupleDayHint)
	child("STATIC", "", 0, 649, 445, 427, 48, idCoupleStatus)
	child("STATIC", "이름과 시작일은 이 PC의 설정에 저장됩니다.", 0, 649, 590, 427, 27, idCouplePrivacy)
	setupCoupleCalendar()
}

func setCoupleControls(c CoupleSettings) {
	setToggle(idCoupleEnabled, c.Enabled)
	set(controls[idCoupleFirst], c.FirstName)
	set(controls[idCoupleSecond], c.SecondName)
	set(controls[idCoupleDate], c.StartDate)
}

func readCoupleControls() CoupleSettings {
	return CoupleSettings{
		Enabled:   toggleState[idCoupleEnabled],
		FirstName: txt(controls[idCoupleFirst]), SecondName: txt(controls[idCoupleSecond]),
		StartDate: txt(controls[idCoupleDate]),
	}
}

func updateCoupleStatus() {
	h := controls[idCoupleStatus]
	if h == 0 {
		return
	}
	status := "기념일을 켜면 트리 아래와 F11 화면에 표시됩니다."
	if display, err := coupleCaption(cfg.Couple, time.Now()); err != nil {
		status = err.Error()
	} else if display.firstName != "" {
		status = display.text()
	}
	if txt(h) != status {
		set(h, status)
	}
}
