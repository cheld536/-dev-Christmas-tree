package main

import "time"

func setupCouplePage() {
	heading("커플 기념일", 649, 127, 420, 28, sectionFont)
	check(idCoupleEnabled, 649, 167, "커플 기념일 표시")
	edit := func(id, top, length int, title string) {
		label(title, 649, top, 400, 23)
		h := child("EDIT", "", wsTab|0x80, 649, top+26, 427, 34, id)
		send(h, 0xc5, uintptr(length), 0)
		send(h, 0xd3, 1, uintptr(sc(10)))
		roundRegion(h, 427, 34, 12)
	}
	edit(idCoupleFirst, 222, 2*maxCoupleName, "첫 번째 이름")
	edit(idCoupleSecond, 303, 2*maxCoupleName, "두 번째 이름")
	edit(idCoupleDate, 384, len(time.DateOnly), "시작일 (YYYY-MM-DD)")
	label("시작일을 D+1일로 계산합니다.", 649, 455, 427, 24)
	child("STATIC", "", 0, 649, 494, 427, 48, idCoupleStatus)
	label("이름과 시작일은 이 PC의 설정에 저장됩니다.", 649, 590, 427, 27)
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
	} else if display.names != "" {
		status = display.text()
	}
	if txt(h) != status {
		set(h, status)
	}
}
