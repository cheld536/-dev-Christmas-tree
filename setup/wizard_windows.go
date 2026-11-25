package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"path/filepath"
	"syscall"
	"unsafe"
)

//go:embed tree.png
var setupArtwork []byte

var painter = syscall.NewLazyDLL("gdi32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var currentWizard *wizard

const (
	setupInstall = 101
	setupSaver   = 102
	setupClose   = 103
	setupVersion = 104
	setupDesktop = 105
	setupUpdate  = 0x8001
)

type winRect struct{ left, top, right, bottom int32 }
type winPoint struct{ x, y int32 }
type winMessage struct {
	window  uintptr
	message uint32
	w, l    uintptr
	time    uint32
	point   winPoint
	private uint32
}
type windowClass struct {
	size, style                                       uint32
	callback                                          uintptr
	extra, windowExtra                                int32
	instance, icon, cursor, brush, menu, class, small uintptr
}
type paintState struct {
	dc              uintptr
	erase           int32
	rect            winRect
	restore, update int32
	reserved        [32]byte
}
type bitmapInfo struct {
	size          uint32
	width, height int32
	planes, bits  uint16
	extra         [6]uint32
}

type drawButton struct {
	controlType, controlID, itemID, action, state uint32
	window, dc                                    uintptr
	bounds                                        winRect
	data                                          uintptr
}

type setupResult struct {
	progress string
	err      error
}

type wizard struct {
	hwnd, instance, background, panel uintptr
	kicker, versionLabel              uintptr
	heading, detail, status, option   uintptr
	desktopOption                     uintptr
	primary, close                    uintptr
	font, title                       uintptr
	dir, menu                         string
	active, complete                  bool
	updates                           chan setupResult
	art                               []byte
	artWidth, artHeight               int
}

//go:uintptrescapes
func invoke(dll *syscall.LazyDLL, name string, args ...uintptr) uintptr {
	r, _, _ := dll.NewProc(name).Call(args...)
	return r
}

func str(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func writeWindow(h uintptr, s string) {
	invoke(u, "SetWindowTextW", h, uintptr(unsafe.Pointer(str(s))))
}

func (w *wizard) readArtwork() {
	img, err := png.Decode(bytes.NewReader(setupArtwork))
	if err != nil {
		return
	}
	bounds := img.Bounds()
	w.artWidth, w.artHeight = bounds.Dx(), bounds.Dy()
	w.art = make([]byte, 0, w.artWidth*w.artHeight*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			w.art = append(w.art, byte(b>>8), byte(g>>8), byte(r>>8), 0)
		}
	}
}

func setupWindow(dir, menu string) {
	if proc := u.NewProc("SetProcessDpiAwarenessContext"); proc.Find() == nil {
		proc.Call(^uintptr(3))
	}
	w := &wizard{dir: dir, menu: menu, updates: make(chan setupResult, 8)}
	w.instance = invoke(kernel, "GetModuleHandleW", 0)
	w.background = invoke(painter, "CreateSolidBrush", 0x201812)
	w.panel = invoke(painter, "CreateSolidBrush", 0x0D0D0D)
	defer invoke(painter, "DeleteObject", w.background)
	defer invoke(painter, "DeleteObject", w.panel)
	w.readArtwork()
	currentWizard = w
	name := str("DCTSetup")
	class := windowClass{callback: syscall.NewCallback(setupProc), instance: w.instance,
		icon: invoke(u, "LoadIconW", w.instance, 1), cursor: invoke(u, "LoadCursorW", 0, 32512),
		brush: w.background, class: uintptr(unsafe.Pointer(name))}
	class.size = uint32(unsafe.Sizeof(class))
	if invoke(u, "RegisterClassExW", uintptr(unsafe.Pointer(&class))) == 0 {
		msg("설치 창을 만들 수 없습니다.", 0x10)
		return
	}
	dpi := invoke(u, "GetDpiForSystem")
	if dpi == 0 {
		dpi = 96
	}
	width, height := 700*dpi/96, 412*dpi/96
	var work winRect
	invoke(u, "SystemParametersInfoW", 0x30, 0, uintptr(unsafe.Pointer(&work)), 0)
	x := work.left + (work.right-work.left-int32(width))/2
	y := work.top + (work.bottom-work.top-int32(height))/2
	w.hwnd = invoke(u, "CreateWindowExW", 0x10000, uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(str("Developer Christmas Tree 설치"))), 0x02CA0000,
		uintptr(x), uintptr(y), width, height, 0, 0, w.instance, 0)
	if w.hwnd == 0 {
		msg("설치 창을 열 수 없습니다.", 0x10)
		return
	}
	defer func() {
		invoke(painter, "DeleteObject", w.font)
		invoke(painter, "DeleteObject", w.title)
	}()
	dark := int32(1)
	invoke(syscall.NewLazyDLL("dwmapi.dll"), "DwmSetWindowAttribute", w.hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	w.kicker = w.control("STATIC", "SETUP  /  WINDOWS x64", 0, 0)
	w.versionLabel = w.control("STATIC", "v"+version, 2, setupVersion)
	w.heading = w.control("STATIC", "Developer Christmas Tree", 0, 0)
	w.detail = w.control("STATIC", fmt.Sprintf("설치 위치\r\n%s\r\n\r\n기존 트리 설정은 유지됩니다.", dir), 0, 0)
	w.option = w.control("BUTTON", "Windows 화면 보호기로 사용", 0x10003, setupSaver)
	w.desktopOption = w.control("BUTTON", "바탕 화면에 바로가기 만들기", 0x10003, setupDesktop)
	invoke(u, "SendMessageW", w.desktopOption, 0xF1, 1, 0)
	w.status = w.control("STATIC", "", 0, 0)
	w.primary = w.control("BUTTON", "설치", 0x1000B, setupInstall)
	w.close = w.control("BUTTON", "취소", 0x1000B, setupClose)
	w.layout()
	invoke(u, "ShowWindow", w.hwnd, 5)
	invoke(u, "SetFocus", w.primary)
	var message winMessage
	for int32(invoke(u, "GetMessageW", uintptr(unsafe.Pointer(&message)), 0, 0, 0)) > 0 {
		if message.message == 0x100 && message.w == 0x1B {
			invoke(u, "SendMessageW", w.hwnd, 0x10, 0, 0)
			continue
		}
		if message.message == 0x100 && message.w == 0x0D && !w.active {
			action := uintptr(setupInstall)
			if invoke(u, "GetFocus") == w.close {
				action = setupClose
			}
			invoke(u, "SendMessageW", w.hwnd, 0x111, action, 0)
			continue
		}
		if invoke(u, "IsDialogMessageW", w.hwnd, uintptr(unsafe.Pointer(&message))) == 0 {
			invoke(u, "TranslateMessage", uintptr(unsafe.Pointer(&message)))
			invoke(u, "DispatchMessageW", uintptr(unsafe.Pointer(&message)))
		}
	}
}

func (w *wizard) control(class, label string, style, id uintptr) uintptr {
	return invoke(u, "CreateWindowExW", 0, uintptr(unsafe.Pointer(str(class))), uintptr(unsafe.Pointer(str(label))),
		style|0x50000000, 0, 0, 0, 0, w.hwnd, id, w.instance, 0)
}

func (w *wizard) dimensions() (width, height int32) {
	var r winRect
	invoke(u, "GetClientRect", w.hwnd, uintptr(unsafe.Pointer(&r)))
	return r.right, r.bottom
}

func (w *wizard) layout() {
	width, height := w.dimensions()
	x := func(v int) uintptr { return uintptr(int(width) * v / 680) }
	y := func(v int) uintptr { return uintptr(int(height) * v / 370) }
	newFont := func(size int, weight uintptr) uintptr {
		return invoke(painter, "CreateFontW", uintptr(-int(y(size))), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(str("Segoe UI"))))
	}
	oldFont, oldTitle := w.font, w.title
	w.font, w.title = newFont(16, 400), newFont(27, 600)
	move := func(h, font uintptr, left, top, sw, sh int) {
		invoke(u, "MoveWindow", h, x(left), y(top), x(sw), y(sh), 1)
		invoke(u, "SendMessageW", h, 0x30, font, 1)
	}
	move(w.kicker, w.font, 257, 26, 290, 25)
	move(w.versionLabel, w.font, 551, 26, 99, 25)
	move(w.heading, w.title, 257, 57, 405, 43)
	move(w.detail, w.font, 257, 115, 395, 107)
	move(w.option, w.font, 255, 216, 395, 29)
	move(w.desktopOption, w.font, 255, 246, 395, 29)
	move(w.status, w.font, 257, 279, 395, 34)
	move(w.primary, w.font, 447, 315, 96, 36)
	move(w.close, w.font, 554, 315, 96, 36)
	invoke(painter, "DeleteObject", oldFont)
	invoke(painter, "DeleteObject", oldTitle)
	invoke(u, "InvalidateRect", w.hwnd, 0, 1)
}

func (w *wizard) paint(h uintptr) {
	var p paintState
	dc := invoke(u, "BeginPaint", h, uintptr(unsafe.Pointer(&p)))
	defer invoke(u, "EndPaint", h, uintptr(unsafe.Pointer(&p)))
	width, height := w.dimensions()
	art := winRect{width / 32, height / 16, width * 34 / 100, height * 15 / 16}
	invoke(u, "FillRect", dc, uintptr(unsafe.Pointer(&art)), w.panel)
	accent := invoke(painter, "CreateSolidBrush", 0x4B4331)
	invoke(u, "FrameRect", dc, uintptr(unsafe.Pointer(&art)), accent)
	invoke(painter, "DeleteObject", accent)
	if len(w.art) == 0 {
		return
	}
	dw := art.right - art.left - width/45
	dh := dw * int32(w.artHeight) / int32(w.artWidth)
	info := bitmapInfo{size: 40, width: int32(w.artWidth), height: -int32(w.artHeight), planes: 1, bits: 32}
	invoke(painter, "SetStretchBltMode", dc, 4)
	invoke(painter, "SetBrushOrgEx", dc, 0, 0, 0)
	invoke(painter, "StretchDIBits", dc, uintptr(art.left+(art.right-art.left-dw)/2), uintptr(art.top+(art.bottom-art.top-dh)/2),
		uintptr(dw), uintptr(dh), 0, 0, uintptr(w.artWidth), uintptr(w.artHeight), uintptr(unsafe.Pointer(&w.art[0])),
		uintptr(unsafe.Pointer(&info)), 0, 0xCC0020)
}

func (w *wizard) paintButton(item *drawButton) {
	background, foreground := uintptr(0x393029), uintptr(0xFAF5EB)
	if item.controlID == setupInstall {
		background, foreground = 0xBDE455, 0x1E2710
	}
	if item.state&4 != 0 {
		background, foreground = 0x2C2621, 0x998575
	} else if item.state&1 != 0 {
		background = 0x6F7061
	}
	brush := invoke(painter, "CreateSolidBrush", background)
	previousBrush := invoke(painter, "SelectObject", item.dc, brush)
	previousPen := invoke(painter, "SelectObject", item.dc, invoke(painter, "GetStockObject", 8))
	r := item.bounds
	invoke(painter, "RoundRect", item.dc, uintptr(r.left), uintptr(r.top), uintptr(r.right), uintptr(r.bottom), 12, 12)
	invoke(painter, "SelectObject", item.dc, previousPen)
	invoke(painter, "SelectObject", item.dc, previousBrush)
	invoke(painter, "DeleteObject", brush)
	oldFont := invoke(painter, "SelectObject", item.dc, w.font)
	invoke(painter, "SetBkMode", item.dc, 1)
	invoke(painter, "SetTextColor", item.dc, foreground)
	var label [80]uint16
	invoke(u, "GetWindowTextW", item.window, uintptr(unsafe.Pointer(&label[0])), uintptr(len(label)))
	invoke(u, "DrawTextW", item.dc, uintptr(unsafe.Pointer(&label[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x825)
	invoke(painter, "SelectObject", item.dc, oldFont)
	if item.state&0x10 != 0 {
		r.left, r.top, r.right, r.bottom = r.left+5, r.top+5, r.right-5, r.bottom-5
		invoke(u, "DrawFocusRect", item.dc, uintptr(unsafe.Pointer(&r)))
	}
}

func setupProc(h, message, a, b uintptr) uintptr {
	w := currentWizard
	switch message {
	case 0x0F:
		if w.hwnd != 0 {
			w.paint(h)
			return 0
		}
	case 0x0138, 0x0135:
		invoke(painter, "SetTextColor", a, 0xF3EAE2)
		invoke(painter, "SetBkColor", a, 0x201812)
		return w.background
	case 0x2B:
		if b != 0 {
			w.paintButton((*drawButton)(unsafe.Pointer(b)))
			return 1
		}
	case 0x02E0:
		r := *(*winRect)(unsafe.Pointer(b))
		invoke(u, "SetWindowPos", h, 0, uintptr(r.left), uintptr(r.top), uintptr(r.right-r.left), uintptr(r.bottom-r.top), 0x14)
		w.layout()
		return 0
	case 0x111:
		if a>>16 != 0 || w.active {
			return 0
		}
		switch a & 0xffff {
		case setupInstall:
			if w.complete {
				open(filepath.Join(w.dir, "DeveloperChristmasTree.exe"), "")
				invoke(u, "DestroyWindow", h)
			} else {
				w.start()
			}
		case setupClose, 2:
			invoke(u, "DestroyWindow", h)
		}
		return 0
	case setupUpdate:
		w.update(<-w.updates)
		return 0
	case 0x10:
		if !w.active {
			invoke(u, "DestroyWindow", h)
		}
		return 0
	case 2:
		invoke(u, "PostQuitMessage", 0)
		return 0
	}
	return invoke(u, "DefWindowProcW", h, message, a, b)
}

func (w *wizard) start() {
	w.active = true
	register := invoke(u, "SendMessageW", w.option, 0xF0, 0, 0) == 1
	desktopLink := invoke(u, "SendMessageW", w.desktopOption, 0xF0, 0, 0) == 1
	for _, h := range []uintptr{w.primary, w.close, w.option, w.desktopOption} {
		invoke(u, "EnableWindow", h, 0)
	}
	writeWindow(w.status, "설치를 준비하는 중…")
	go func() {
		send := func(result setupResult) {
			w.updates <- result
			invoke(u, "PostMessageW", w.hwnd, setupUpdate, 0, 0)
		}
		err := install(w.dir, w.menu, register, desktopLink, func(step string) { send(setupResult{progress: step}) })
		send(setupResult{err: err})
	}()
}

func (w *wizard) update(result setupResult) {
	if result.progress != "" {
		writeWindow(w.status, result.progress)
		return
	}
	w.active = false
	invoke(u, "EnableWindow", w.primary, 1)
	invoke(u, "EnableWindow", w.close, 1)
	writeWindow(w.close, "닫기")
	if result.err != nil {
		writeWindow(w.heading, "설치를 완료하지 못했어요")
		writeWindow(w.detail, result.err.Error())
		writeWindow(w.status, "")
		writeWindow(w.primary, "다시 시도")
		invoke(u, "EnableWindow", w.option, 1)
		invoke(u, "EnableWindow", w.desktopOption, 1)
	} else {
		w.complete = true
		writeWindow(w.heading, "설치가 끝났어요")
		writeWindow(w.detail, "시작 메뉴에서도 Developer Christmas Tree를 열 수 있습니다.")
		writeWindow(w.status, "")
		writeWindow(w.primary, "실행")
		invoke(u, "ShowWindow", w.option, 0)
		invoke(u, "ShowWindow", w.desktopOption, 0)
	}
	invoke(u, "SetFocus", w.primary)
}
