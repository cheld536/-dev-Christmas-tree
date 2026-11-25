package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var u = syscall.NewLazyDLL("user32.dll")
var g = syscall.NewLazyDLL("gdi32.dll")
var k = syscall.NewLazyDLL("kernel32.dll")
var gl = syscall.NewLazyDLL("opengl32.dll")
var comdlg = syscall.NewLazyDLL("comdlg32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")
var procCache = map[string]*syscall.LazyProc{}
var stringsKeep [128]*uint16
var keepIndex int
var uiUpdating bool
var toastText string
var toastUntil time.Time
var footerEvent captionEvent
var footerUntil time.Time
var settingsPreview uintptr

var pointersKeep [2048]any
var pointerIndex int
var panelBrush, editBrush, cardBrush, titleFont, sectionFont uintptr
var version = "2.3.0"
var uiScale = 1.0
var buildPage, currentPage int
var pageChildren = map[int][]uintptr{}
var toggleState = map[int]bool{}
var toggleIDs = map[int]bool{}
var fullScreen, paused, overlayVisible bool
var audioState AudioFrame
var monoFace = "Consolas"
var audioStatus uintptr
var lastAudioStatus time.Time

func sc(v int) int { return int(math.Round(float64(v) * uiScale)) }
func track(h uintptr) {
	if buildPage > 0 {
		pageChildren[buildPage] = append(pageChildren[buildPage], h)
	}
}
func chooseFont(face, fallback string, size int, weight uintptr) uintptr {
	h := call(g, "CreateFontW", signed(-sc(size)), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, w(face))
	dc := call(u, "GetDC", 0)
	old := call(g, "SelectObject", dc, h)
	var b [128]uint16
	call(g, "GetTextFaceW", dc, 128, ptr(&b[0]))
	call(g, "SelectObject", dc, old)
	call(u, "ReleaseDC", 0, dc)
	if !strings.EqualFold(syscall.UTF16ToString(b[:]), face) {
		call(g, "DeleteObject", h)
		return call(g, "CreateFontW", signed(-sc(size)), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, w(fallback))
	}
	return h
}
func roundRegion(h uintptr, ww, hh, r int) {
	if layout := settingControls[h]; layout != nil {
		layout.radius = r
	}
	region := call(g, "CreateRoundRectRgn", 0, 0, uintptr(sc(ww)+1), uintptr(sc(hh)+1), uintptr(sc(r)), uintptr(sc(r)))
	if call(u, "SetWindowRgn", h, region, 1) == 0 {
		call(g, "DeleteObject", region)
	}
}
func page(n int) {
	currentPage = n
	for i, hs := range pageChildren {
		for _, h := range hs {
			v := uintptr(0)
			if i == n {
				v = 5
			}
			call(u, "ShowWindow", h, v)
		}
	}
	for _, id := range []int{idVisualPage, idAudioPage} {
		call(u, "InvalidateRect", controls[id], 0, 0)
	}
}

//go:uintptrescapes
func call(d *syscall.LazyDLL, n string, a ...uintptr) uintptr {
	key := d.Name + ":" + n
	p := procCache[key]
	if p == nil {
		p = d.NewProc(n)
		procCache[key] = p
	}
	r, _, _ := p.Call(a...)
	return r
}
func w(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", ""))
	stringsKeep[keepIndex%len(stringsKeep)] = p
	keepIndex++
	return uintptr(unsafe.Pointer(p))
}
func ptr[T any](p *T) uintptr {
	pointersKeep[pointerIndex%len(pointersKeep)] = p
	pointerIndex++
	return uintptr(unsafe.Pointer(p))
}
func f32(f float64) uintptr { return uintptr(math.Float32bits(float32(f))) }
func f64(f float64) uintptr { return uintptr(math.Float64bits(f)) }
func cr(c uint32) uintptr   { return uintptr((c&255)<<16 | c&0xff00 | (c>>16)&255) }
func low(v uintptr) int     { return int(v & 0xffff) }
func high(v uintptr) int    { return int(v >> 16 & 0xffff) }
func signed(v int) uintptr  { return uintptr(int64(v)) }

type rect struct{ L, T, R, B int32 }
type pt struct{ X, Y int32 }
type msg struct {
	H       uintptr
	M       uint32
	W, L    uintptr
	T       uint32
	P       pt
	Private uint32
}
type wc struct {
	Size, Style                                       uint32
	Proc                                              uintptr
	Extra, WndExtra                                   int32
	Instance, Icon, Cursor, Brush, Menu, Class, Small uintptr
}
type paint struct {
	DC                 uintptr
	Erase              int32
	R                  rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}

const (
	wmDestroy = 2
	wmSize    = 5
	wmPaint   = 15
	wmClose   = 16
	wmErase   = 20
	wmCommand = 0x111
	wmTimer   = 0x113
	wmKey     = 0x100
	wmSysKey  = 0x104
	wmMouse   = 0x200
	wsChild   = 0x40000000
	wsVisible = 0x10000000
	wsPopup   = 0x80000000
	wsTab     = 0x10000
	wsVScroll = 0x200000
	wsBorder  = 0x800000
	wmSetFont = 0x30
)

var inst uintptr
var mainwin uintptr
var font uintptr
var mode string
var controls = map[int]uintptr{}
var surfaces = map[uintptr]*surface{}
var cfg = defaults()
var started = time.Now()
var exitAt time.Time
var mouseStart pt
var previewOnly bool
var saverWindows []uintptr
var currentStats = "CPU --  GPU --  RAM --"
var lastStats time.Time

func configDir() string {
	p := os.Getenv("LOCALAPPDATA")
	if p == "" {
		p = os.TempDir()
	}
	return filepath.Join(p, "DeveloperChristmasTree")
}
func load(path string) (Settings, error) {
	s := defaults()
	b, e := os.ReadFile(path)
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return defaults(), e
	}
	return normalize(s), nil
}
func save(path string, s Settings) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(normalize(s), "", "  ")
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func configPath() string { return filepath.Join(configDir(), "settings.json") }
func box(h uintptr, s string, flags uintptr) uintptr {
	return call(u, "MessageBoxW", h, w(s), w("Developer Christmas Tree"), flags)
}
func send(h uintptr, m uint32, a, b uintptr) uintptr {
	return call(u, "SendMessageW", h, uintptr(m), a, b)
}
func txt(h uintptr) string {
	n := call(u, "GetWindowTextLengthW", h)
	b := make([]uint16, n+1)
	call(u, "GetWindowTextW", h, ptr(&b[0]), n+1)
	return syscall.UTF16ToString(b)
}
func set(h uintptr, s string) { call(u, "SetWindowTextW", h, w(s)) }
func client(h uintptr) rect   { var r rect; call(u, "GetClientRect", h, ptr(&r)); return r }
func window(cls, title string, style, ex uintptr, x, y, ww, hh int, parent, id uintptr) uintptr {
	return call(u, "CreateWindowExW", ex, w(cls), w(title), style, signed(x), signed(y), signed(ww), signed(hh), parent, id, inst, 0)
}
func child(cls, title string, style uintptr, x, y, ww, hh, id int) uintptr {
	h := window(cls, title, wsChild|wsVisible|style, 0, sc(x), sc(y), sc(ww), sc(hh), mainwin, uintptr(id))
	if h != 0 {
		settingControls[h] = &settingControl{x: x, y: y, width: ww, height: hh}
	}
	track(h)
	if id > 0 {
		controls[id] = h
	}
	send(h, wmSetFont, font, 1)
	if cls == "COMBOBOX" || cls == "EDIT" {
		call(syscall.NewLazyDLL("uxtheme.dll"), "SetWindowTheme", h, w("DarkMode_Explorer"), 0)
	}
	return h
}
func label(s string, x, y, ww, hh int) { child("STATIC", s, 0, x, y, ww, hh, 0) }
func heading(s string, x, y, ww, hh int, f uintptr) {
	h := child("STATIC", s, 0, x, y, ww, hh, 0)
	if layout := settingControls[h]; layout != nil {
		layout.fontRole = 2
		if f == titleFont {
			layout.fontRole = 1
		}
	}
	send(h, wmSetFont, f, 1)
}
func combo(id, x, y int, items []string) {
	h := child("COMBOBOX", "", wsTab|wsVScroll|3, x, y, 230, 250, id)
	for i, s := range items {
		if id == idTheme {
			s = []string{"무지개", "클래식 크리스마스", "아이스 블루", "사이버펑크", "골드 & 화이트", "매트릭스 그린", "직접 선택"}[i]
		}
		if id == idDir {
			s = []string{"시계 방향", "반시계 방향", "정지"}[i]
		}
		if id == idRenderer {
			s = []string{"OpenGL (자동)", "선명한 화면 (GDI)"}[i]
		}
		send(h, 0x143, 0, w(s))
	}
}
func slider(id, x, y, min, max int) {
	h := child("DCTSlider", "", wsTab, x, y, 188, 28, id)
	send(h, 0x407, 1, uintptr(min))
	send(h, 0x408, 1, uintptr(max))
	child("STATIC", "", 0, x+190, y+4, 65, 23, id+100)
}
func check(id, x, y int, s string) {
	toggleIDs[id] = true
	child("BUTTON", s, wsTab|0xB, x, y, 200, 31, id)
}
func button(id, x, y, ww int, s string) { child("BUTTON", s, wsTab|0xB, x, y, ww, 36, id) }
func setToggle(id int, v bool) {
	if toggleState[id] == v {
		return
	}
	toggleState[id] = v
	call(u, "InvalidateRect", controls[id], 0, 0)
}

const (
	idTheme       = 101
	idDir         = 102
	idRenderer    = 103
	idSpeed       = 110
	idLight       = 111
	idFPS         = 112
	idSize        = 113
	idSnow        = 120
	idOrn         = 121
	idMsg         = 122
	idBloom       = 123
	idMatrix      = 124
	idCount       = 125
	idMonitor     = 126
	idMusic       = 127
	idMulti       = 128
	idPhysics     = 129
	idCustom      = 130
	idBG          = 131
	idMessage     = 132
	idSave        = 140
	idReset       = 141
	idTest        = 142
	idExport      = 143
	idImport      = 144
	idWindows     = 145
	idFull        = 146
	idVisualPage  = 147
	idAudioPage   = 148
	idAudioPreset = 150
	idAudioGain   = 151
	idBassGain    = 152
	idMidGain     = 153
	idHighGain    = 154
	idSpectrum    = 155
	idBPM         = 156
	idSafe        = 157
	idLowPower    = 158
	idCodeStyle   = 159
	idResolution  = 160
)

func selectCombo(id int, items []string, s string) {
	for i, v := range items {
		if v == s {
			send(controls[id], 0x14e, uintptr(i), 0)
			return
		}
	}
}
func comboVal(id int, items []string) string {
	i := int(send(controls[id], 0x147, 0, 0))
	if i < 0 || i >= len(items) {
		return items[0]
	}
	return items[i]
}
func uiSet() {
	uiUpdating = true
	defer func() { uiUpdating = false }()
	selectCombo(idTheme, themes, cfg.Theme)
	selectCombo(idDir, directions, cfg.Direction)
	selectCombo(idCodeStyle, codeThemes, codeThemes[cfg.CodeStyle])
	selectCombo(idResolution, fullscreenResolutions, cfg.FullscreenResolution)
	selectCombo(idAudioPreset, audioPresets, cfg.AudioPreset)
	for id, v := range map[int]int{idSpeed: cfg.Speed, idLight: cfg.LightSpeed, idFPS: cfg.FPS, idSize: cfg.Size, idAudioGain: cfg.AudioGain, idBassGain: cfg.BassGain, idMidGain: cfg.MidGain, idHighGain: cfg.HighGain} {
		send(controls[id], 0x405, 1, uintptr(v))
	}
	for id, v := range map[int]bool{idSnow: cfg.Snow, idOrn: cfg.Ornaments, idMsg: cfg.Messages, idBloom: cfg.Bloom, idMatrix: cfg.Matrix, idCount: cfg.Countdown, idMonitor: cfg.Monitor, idMusic: cfg.Music, idMulti: cfg.Multi, idPhysics: cfg.SnowPhysics, idSpectrum: cfg.Spectrum, idBPM: cfg.ShowBPM, idSafe: cfg.SafeLights, idLowPower: cfg.LowPower} {
		setToggle(id, v)
	}
	set(controls[idCustom], "트리 색상  "+cfg.Custom)
	set(controls[idBG], "배경 색상  "+cfg.Background)
	set(controls[idMessage], cfg.Message)
	uiRead()
}
func uiRead() {
	if mainwin == 0 {
		return
	}
	cfg.Theme = comboVal(idTheme, themes)
	cfg.Direction = comboVal(idDir, directions)
	cfg.Renderer = renderers[1]
	cfg.CodeStyle = clamp(int(send(controls[idCodeStyle], 0x147, 0, 0)), 0, len(codeThemes)-1)
	cfg.FullscreenResolution = comboVal(idResolution, fullscreenResolutions)
	cfg.AudioPreset = comboVal(idAudioPreset, audioPresets)
	for id, p := range map[int]*int{idSpeed: &cfg.Speed, idLight: &cfg.LightSpeed, idFPS: &cfg.FPS, idSize: &cfg.Size, idAudioGain: &cfg.AudioGain, idBassGain: &cfg.BassGain, idMidGain: &cfg.MidGain, idHighGain: &cfg.HighGain} {
		*p = int(send(controls[id], 0x400, 0, 0))
		s := fmt.Sprint(*p)
		if id == idSpeed || id == idLight {
			s = fmt.Sprintf("%.2fx", float64(*p)/100)
		} else if id == idSize || (id >= idAudioGain && id <= idHighGain) {
			s += "%"
		}
		if txt(controls[id+100]) != s {
			set(controls[id+100], s)
		}
	}
	for id, p := range map[int]*bool{idSnow: &cfg.Snow, idOrn: &cfg.Ornaments, idMsg: &cfg.Messages, idBloom: &cfg.Bloom, idMatrix: &cfg.Matrix, idCount: &cfg.Countdown, idMonitor: &cfg.Monitor, idMusic: &cfg.Music, idMulti: &cfg.Multi, idPhysics: &cfg.SnowPhysics, idSpectrum: &cfg.Spectrum, idBPM: &cfg.ShowBPM, idSafe: &cfg.SafeLights, idLowPower: &cfg.LowPower} {
		*p = toggleState[id]
	}
	cfg.Message = txt(controls[idMessage])
	cfg = normalize(cfg)
}

type choose struct {
	Size                 uint32
	Owner, Instance      uintptr
	RGB                  uint32
	Custom               uintptr
	Flags                uint32
	Data, Hook, Template uintptr
}

func chooseColor(id int) {
	s := cfg.Custom
	if id == idBG {
		s = cfg.Background
	}
	c, _ := hexColor(s)
	var custom [16]uint32
	cc := choose{Owner: mainwin, RGB: uint32(cr(c)), Custom: ptr(&custom[0]), Flags: 3}
	cc.Size = uint32(unsafe.Sizeof(cc))
	res := call(comdlg, "ChooseColorW", ptr(&cc))
	runtime.KeepAlive(&custom)
	runtime.KeepAlive(&cc)
	if res != 0 {
		v := fmt.Sprintf("#%06X", cr(cc.RGB))
		if id == idBG {
			cfg.Background = v
			set(controls[id], "배경 색상  "+v)
		} else {
			cfg.Custom = v
			set(controls[id], "트리 색상  "+v)
			selectCombo(idTheme, themes, "Custom")
		}
	}
}

type ofn struct {
	Size                                   uint32
	Owner, Instance, Filter, CustomFilter  uintptr
	MaxCust, FilterIndex                   uint32
	File                                   uintptr
	MaxFile                                uint32
	FileTitle                              uintptr
	MaxTitle                               uint32
	InitialDir, Title                      uintptr
	Flags                                  uint32
	FileOffset, FileExt                    uint16
	DefExt, Data, Hook, Template, Reserved uintptr
	Reserved2, FlagsEx                     uint32
}

func fileDialog(export bool) string {
	var buf [2048]uint16
	f := utf16.Encode([]rune("Christmas preset (*.json)\x00*.json\x00All files (*.*)\x00*.*\x00\x00"))
	if export {
		copy(buf[:], syscall.StringToUTF16("Christmas-preset.json"))
	}
	o := ofn{Owner: mainwin, Filter: ptr(&f[0]), File: ptr(&buf[0]), MaxFile: uint32(len(buf)), FilterIndex: 1, Flags: 0x800 | 0x80000, DefExt: w("json")}
	o.Size = uint32(unsafe.Sizeof(o))
	fn := "GetOpenFileNameW"
	if export {
		fn = "GetSaveFileNameW"
		o.Flags |= 2
	} else {
		o.Flags |= 0x1000
	}
	res := call(comdlg, fn, ptr(&o))
	runtime.KeepAlive(f)
	runtime.KeepAlive(&buf)
	runtime.KeepAlive(&o)
	if res != 0 {
		return syscall.UTF16ToString(buf[:])
	}
	return ""
}
func command(id int) {
	if toggleIDs[id] {
		setToggle(id, !toggleState[id])
		uiRead()
		return
	}
	uiRead()
	switch id {
	case idCustom, idBG:
		chooseColor(id)
	case idSave:
		if e := save(configPath(), cfg); e != nil {
			box(mainwin, e.Error(), 0x10)
		} else {
			set(controls[idSave], "저장됨")
			lastSaved = time.Now()
		}
	case idReset:
		cfg = defaults()
		uiSet()
	case idTest:
		fullScreen = false
		startSaver(false, 0)
	case idFull:
		openFull()
	case idVisualPage:
		page(1)
	case idAudioPage:
		page(2)
	case idExport:
		if p := fileDialog(true); p != "" {
			if e := save(p, cfg); e != nil {
				box(mainwin, e.Error(), 0x10)
			}
		}
	case idImport:
		if p := fileDialog(false); p != "" {
			s, e := load(p)
			if e != nil {
				box(mainwin, e.Error(), 0x10)
			} else {
				cfg = s
				uiSet()
			}
		}
	case idWindows:
		call(shell, "ShellExecuteW", mainwin, w("open"), w("control.exe"), w("desk.cpl,,@screensaver"), 0, 1)
	}
}

var lastSaved time.Time

const previewX, previewY, previewW, previewH = 30, 112, 580, 502

func setupUI() {
	uiUpdating = true
	defer func() { uiUpdating = false }()
	work := settingsWorkArea(0)
	uiScale = settingsScale(work, int(call(u, "GetDpiForSystem")))
	x := int(work.L) + (int(work.R-work.L)-sc(1120))/2
	y := int(work.T) + (int(work.B-work.T)-sc(770))/2
	versionLabel := strings.TrimSuffix(version, ".0")
	mainwin = window("DCTMain", "Christmas Tree  •  "+versionLabel, 0x06CA0000, 0, x, y, sc(1120), sc(770), 0, 0)
	if mainwin == 0 {
		box(0, "설정 창을 열 수 없어요.", 0x10)
		return
	}
	dark := int32(1)
	dwm := syscall.NewLazyDLL("dwmapi.dll")
	call(dwm, "DwmSetWindowAttribute", mainwin, 20, ptr(&dark), 4)
	corner := int32(2)
	call(dwm, "DwmSetWindowAttribute", mainwin, 33, ptr(&corner), 4)
	makeSettingsFonts()
	heading("Christmas Tree", 25, 13, 285, 37, titleFont)
	label(versionLabel, 312, 25, 100, 23)
	heading("미리보기", 30, 78, 210, 28, sectionFont)
	audioStatus = child("STATIC", "", 0, 290, 82, 320, 22, 0)
	p := window("DCTRender", "", wsChild|wsVisible, 0, sc(previewX), sc(previewY), sc(previewW), sc(previewH), mainwin, 0)
	if p != 0 {
		settingControls[p] = &settingControl{x: previewX, y: previewY, width: previewW, height: previewH}
	}
	roundRegion(p, previewW, previewH, 20)
	addSurface(p, false)
	settingsPreview = p

	button(idFull, 30, 628, 180, "전체 화면   F11")
	button(idTest, 220, 628, 180, "화면 보호기 미리보기")
	button(idWindows, 410, 628, 200, "Windows 화면 보호기")
	label("전체 화면 해상도", 30, 675, 135, 24)
	combo(idResolution, 166, 672, fullscreenResolutions)
	label("H  ·  전체 화면 단축키", 412, 675, 195, 24)
	button(idVisualPage, 649, 78, 210, "트리와 조명")
	button(idAudioPage, 869, 78, 207, "음악과 효과")
	buildPage = 1
	label("테마", 649, 128, 140, 22)
	combo(idTheme, 805, 125, themes)
	label("회전 방향", 649, 165, 140, 22)
	combo(idDir, 805, 162, directions)
	label("코드 스타일", 649, 202, 140, 22)
	combo(idCodeStyle, 805, 199, codeThemes)
	label("회전 속도", 649, 244, 140, 22)
	slider(idSpeed, 805, 237, 0, 400)
	label("조명 속도", 649, 279, 140, 22)
	slider(idLight, 805, 272, 0, 400)
	label("프레임", 649, 314, 140, 22)
	slider(idFPS, 805, 307, 10, 144)
	label("트리 크기", 649, 349, 140, 22)
	slider(idSize, 805, 342, 45, 100)
	button(idCustom, 649, 379, 210, "트리 색상")
	button(idBG, 869, 379, 207, "배경 색상")
	heading("분위기", 649, 432, 420, 26, sectionFont)
	check(idSnow, 649, 466, "눈 내리기")
	check(idOrn, 869, 466, "코드 장식")
	check(idMsg, 649, 502, "인사말 표시")
	check(idBloom, 869, 502, "빛 번짐")
	check(idMatrix, 649, 538, "코드 비")
	check(idCount, 869, 538, "크리스마스 D-Day")
	label("나만의 문구 (선택)", 649, 580, 400, 22)
	h := child("EDIT", "", wsTab|0x80, 649, 608, 427, 32, idMessage)
	send(h, 0xc5, 100, 0)
	send(h, 0xd3, 1, uintptr(sc(10)))
	roundRegion(h, 427, 32, 12)
	buildPage = 2
	check(idMusic, 649, 125, "음악 반응")
	label("PC에서 재생되는 소리에 반응해요.", 649, 163, 427, 24)
	label("음악 스타일", 649, 202, 145, 22)
	combo(idAudioPreset, 805, 199, audioPresets)
	label("반응 강도", 649, 244, 140, 22)
	slider(idAudioGain, 805, 237, 0, 200)
	label("저음", 649, 279, 140, 22)
	slider(idBassGain, 805, 272, 0, 200)
	label("중음", 649, 314, 140, 22)
	slider(idMidGain, 805, 307, 0, 200)
	label("고음", 649, 349, 140, 22)
	slider(idHighGain, 805, 342, 0, 200)
	label("저음·중음·고음은 '직접 설정'에서 적용돼요.", 649, 381, 427, 24)
	heading("화면 옵션", 649, 425, 400, 26, sectionFont)
	check(idSpectrum, 649, 459, "이퀄라이저")
	check(idBPM, 869, 459, "BPM 표시")
	check(idSafe, 649, 495, "빛 변화 줄이기")
	check(idLowPower, 869, 495, "저전력 모드")
	check(idMonitor, 649, 531, "시스템 사용량")
	check(idMulti, 869, 531, "모든 모니터")
	check(idPhysics, 649, 567, "바람과 눈 쌓임")
	label("무음 상태에서는 기본 애니메이션이 재생돼요.", 649, 611, 427, 24)
	buildPage = 0
	button(idSave, 649, 655, 101, "저장")
	button(idReset, 757, 655, 100, "초기화")
	button(idExport, 864, 655, 102, "내보내기")
	button(idImport, 973, 655, 103, "불러오기")
	uiSet()
	page(1)
	call(u, "ShowWindow", mainwin, 5)
	call(u, "UpdateWindow", mainwin)
}
func wantsGL() bool {
	if cfg.Renderer != renderers[0] {
		return false
	}
	for _, c := range cfg.Message {
		if c > 127 {
			return false
		}
	}
	return true
}
func addSurface(h uintptr, saver bool) {
	if h == 0 {
		return
	}
	surfaces[h] = &surface{hwnd: h, dc: call(u, "GetDC", h), saver: saver, last: time.Now(), begin: time.Now()}

}
func openFull() {
	if len(saverWindows) > 0 {
		return
	}
	uiRead()
	fullScreen = true
	paused = false
	overlayVisible = false
	footerEvent = captionNone
	toastUntil = time.Time{}
	startSaver(false, 0)
	if len(saverWindows) == 0 {
		fullScreen = false
	}
}

// Explicit user shortcuts send standard Windows media keys to the active media session.
func mediaKey(v uintptr) { call(u, "keybd_event", v, 0, 0, 0); call(u, "keybd_event", v, 0, 2, 0) }
func onOff(name string, enabled bool) string {
	if enabled {
		return name + " 켜짐"
	}
	return name + " 꺼짐"
}
func fullKey(key uintptr) {
	switch key {
	case 0x1B, 0x7A:
		stopSaver()
		return
	case 0x20:
		paused = !paused
		footerEvent = captionResumed
		footerUntil = time.Now().Add(2300 * time.Millisecond)
	case 0x26:
		cfg.Speed = clamp(cfg.Speed+25, 0, 400)
	case 0x28:
		cfg.Speed = clamp(cfg.Speed-25, 0, 400)
	case 0x25:
		cfg.Direction = directions[1]
	case 0x27:
		cfg.Direction = directions[0]
	case 'C':
		for i, t := range themes {
			if t == cfg.Theme {
				cfg.Theme = themes[(i+1)%len(themes)]
				break
			}
		}
	case 'S':
		cfg.Snow = !cfg.Snow
		footerEvent = captionNone
		if cfg.Snow {
			footerEvent = captionSnow
			footerUntil = time.Now().Add(2300 * time.Millisecond)
		}
	case 'M':
		cfg.Matrix = !cfg.Matrix
	case 'B':
		cfg.Bloom = !cfg.Bloom
	case 'H':
		overlayVisible = !overlayVisible
	case 'D':
		cfg.CodeStyle = (cfg.CodeStyle + 1) % len(codeThemes)
	case 'A':
		cfg.Music = !cfg.Music
	case 9:
		cfg.Monitor = !cfg.Monitor
	case 'P':
		for i, t := range audioPresets {
			if t == cfg.AudioPreset {
				cfg.AudioPreset = audioPresets[(i+1)%len(audioPresets)]
				break
			}
		}
	case 'V':
		cfg.Spectrum = !cfg.Spectrum
	case 'Z', 'X', 'N':
		if key == 'Z' {
			mediaKey(0xB1)
			toastText = "이전 곡"
		} else if key == 'X' {
			mediaKey(0xB3)
			toastText = "재생 / 일시 정지"
		} else {
			mediaKey(0xB0)
			toastText = "다음 곡"
		}
		toastUntil = time.Now().Add(1500 * time.Millisecond)
		return
	case 'G':
		cfg.AudioGain = clamp(cfg.AudioGain+25, 0, 200)
	case 'J':
		cfg.AudioGain = clamp(cfg.AudioGain-25, 0, 200)
	}
	if mainwin != 0 {
		uiSet()
	}
	switch key {
	case 0x20:
		if paused {
			toastText = "일시 정지"
		} else {
			toastText = "다시 재생"
		}
	case 0x26, 0x28:
		toastText = fmt.Sprintf("회전 %.2f×", float64(cfg.Speed)/100)
	case 'C':
		toastText = cfg.Theme
	case 'P':
		toastText = cfg.AudioPreset
	case 'G', 'J':
		toastText = fmt.Sprintf("음악 반응 %d%%", cfg.AudioGain)
	case 0x25, 0x27:
		toastText = cfg.Direction
	case 'D':
		toastText = codeThemes[cfg.CodeStyle]
	case 'S':
		toastText = onOff("눈", cfg.Snow)
	case 'M':
		toastText = onOff("코드 비", cfg.Matrix)
	case 'B':
		toastText = onOff("빛 번짐", cfg.Bloom)
	case 'A':
		toastText = onOff("음악 반응", cfg.Music)
	case 'V':
		toastText = onOff("이퀄라이저", cfg.Spectrum)
	case 9:
		toastText = onOff("시스템 사용량", cfg.Monitor)
	default:
		return
	}
	toastUntil = time.Now().Add(1500 * time.Millisecond)
}
func stopSaver() {
	for _, h := range saverWindows {
		call(u, "DestroyWindow", h)
	}
	saverWindows = nil
	fullScreen = false
	paused = false
	overlayVisible = false
	if !previewOnly {
		call(u, "ShowCursor", 1)
	}
	if mainwin == 0 {
		call(u, "PostQuitMessage", 0)
	}
	if mainwin != 0 {
		call(u, "SetForegroundWindow", mainwin)
	}
}
func monitorRects() []rect {
	var rs []rect
	cb := syscall.NewCallback(func(h, dc, r, data uintptr) uintptr { rs = append(rs, *(*rect)(unsafe.Pointer(r))); return 1 })
	call(u, "EnumDisplayMonitors", 0, 0, cb, 0)
	return rs
}
func startSaver(preview bool, parent uintptr) {
	if len(saverWindows) > 0 {
		return
	}
	previewOnly = preview
	exitAt = time.Now().Add(650 * time.Millisecond)
	if preview {
		r := client(parent)
		h := window("DCTRender", "", wsChild|wsVisible, 0, 0, 0, int(r.R), int(r.B), parent, 0)
		if h != 0 {
			saverWindows = append(saverWindows, h)
			addSurface(h, true)
		}
		return
	}
	rs := monitorRects()
	if !cfg.Multi || len(rs) == 0 {
		rs = []rect{{0, 0, int32(call(u, "GetSystemMetrics", 0)), int32(call(u, "GetSystemMetrics", 1))}}
	}
	call(u, "GetCursorPos", ptr(&mouseStart))
	call(u, "ShowCursor", 0)
	for _, r := range rs {
		h := window("DCTRender", "Developer Christmas Tree", wsPopup|wsVisible, 0x88, int(r.L), int(r.T), int(r.R-r.L), int(r.B-r.T), 0, 0)
		if h != 0 {
			saverWindows = append(saverWindows, h)
			addSurface(h, true)
			call(u, "ShowWindow", h, 5)
		}
	}
	if len(saverWindows) > 0 {
		call(u, "SetForegroundWindow", saverWindows[0])
		call(u, "SetFocus", saverWindows[0])
	} else {
		call(u, "ShowCursor", 1)
	}
}
func renderProc(h, m, a, b uintptr) uintptr {
	sf := surfaces[h]
	switch m {
	case wmErase:
		return 1
	case wmPaint:
		var p paint
		call(u, "BeginPaint", h, ptr(&p))
		call(u, "EndPaint", h, ptr(&p))
		if sf != nil {
			sf.draw(true)
		}
		return 0
	case wmTimer:
		if sf != nil {
			if sf.saver && previewOnly {
				parent := call(u, "GetParent", h)
				if parent == 0 || call(u, "IsWindow", parent) == 0 {
					stopSaver()
					return 0
				}
				r := client(parent)
				own := client(h)
				if r.R != own.R || r.B != own.B {
					call(u, "MoveWindow", h, 0, 0, uintptr(r.R), uintptr(r.B), 1)
				}
			}

			updateExtras()
			sf.draw(false)
			if sf.saver && !previewOnly && !fullScreen && time.Now().After(exitAt) {
				var p pt
				call(u, "GetCursorPos", ptr(&p))
				if math.Abs(float64(p.X-mouseStart.X))+math.Abs(float64(p.Y-mouseStart.Y)) > 7 {
					stopSaver()
				}
			}
		}
		return 0
	case wmKey, wmSysKey, 0x201, 0x204, 0x207:
		if sf != nil && sf.saver && fullScreen {
			if m == wmKey || m == wmSysKey {
				if b&(1<<30) == 0 || (a >= 0x25 && a <= 0x28) || a == 'G' || a == 'J' {
					fullKey(a)
				}
			}
			return 0
		}
		if sf != nil && sf.saver && !previewOnly && time.Now().After(exitAt) {
			stopSaver()
			return 0
		}
	case wmClose:
		if sf != nil && sf.saver {
			stopSaver()
			return 0
		}
	case wmDestroy:
		if sf != nil {
			sf.free()
			delete(surfaces, h)
			if previewOnly && mainwin == 0 {
				call(u, "PostQuitMessage", 0)
			}
		}
		return 0
	}
	return call(u, "DefWindowProcW", h, m, a, b)
}
func mainProc(h, m, a, b uintptr) uintptr {
	switch m {
	case 0x02E0:
		if h == mainwin && b != 0 {
			changeSettingsDPI(h, low(a), *(*rect)(unsafe.Pointer(b)))
		}
		return 0
	case wmErase:
		return 1
	case wmPaint:
		var p paint
		call(u, "BeginPaint", h, ptr(&p))
		r := client(h)
		call(u, "FillRect", p.DC, ptr(&r), panelBrush)
		rounded(p.DC, rect{int32(sc(16)), int32(sc(62)), int32(sc(624)), int32(sc(706))}, 24, 0x19252A)
		rounded(p.DC, rect{int32(sc(634)), int32(sc(62)), int32(sc(1090)), int32(sc(706))}, 24, 0x19252A)
		call(u, "EndPaint", h, ptr(&p))
		return 0
	case 0x2B:
		di := (*drawItem)(unsafe.Pointer(b))
		drawControl(di)
		return 1
	case 0x0138, 0x0135, 0x0134: // static / dialog / list box
		call(g, "SetTextColor", a, cr(0xE9F0ED))
		var r rect
		call(u, "GetWindowRect", b, ptr(&r))
		p := pt{r.L, r.T}
		call(u, "ScreenToClient", h, ptr(&p))
		if p.Y < int32(sc(60)) {
			call(g, "SetBkColor", a, cr(0x101A1E))
			return panelBrush
		}
		call(g, "SetBkColor", a, cr(0x19252A))
		return cardBrush
	case 0x0133: // edit
		call(g, "SetTextColor", a, cr(0xE9F0ED))
		call(g, "SetBkColor", a, cr(0x28373D))
		return editBrush
	case wmCommand:
		if uiUpdating {
			return 0
		}
		if (high(a) == 1 && (low(a) == idTheme || low(a) == idDir || low(a) == idAudioPreset || low(a) == idCodeStyle || low(a) == idResolution)) || (high(a) == 0x300 && low(a) == idMessage) {
			uiRead()
			return 0
		}
		if high(a) == 0 && (toggleIDs[low(a)] || (low(a) >= 130 && low(a) <= 148)) {
			command(low(a))
		}
		return 0
	case wmClose:
		if len(saverWindows) > 0 {
			stopSaver()
		}
		call(u, "DestroyWindow", h)
		return 0
	case wmDestroy:
		call(u, "PostQuitMessage", 0)
		return 0
	}
	return call(u, "DefWindowProcW", h, m, a, b)
}
func register(name string, cb uintptr, style uint32, brush uintptr) {
	c := wc{Style: style, Proc: cb, Instance: inst, Cursor: call(u, "LoadCursorW", 0, 32512), Brush: brush, Class: w(name)}
	if name == "DCTMain" {
		c.Icon = call(u, "LoadIconW", inst, 1)
		c.Small = call(u, "LoadImageW", inst, 1, 1, 16, 16, 0x8000)
	}
	c.Size = uint32(unsafe.Sizeof(c))
	call(u, "RegisterClassExW", ptr(&c))
}
func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if proc := u.NewProc("SetProcessDpiAwarenessContext"); proc.Find() == nil {
		if r, _, _ := proc.Call(signed(-4)); r == 0 {
			call(u, "SetProcessDPIAware")
		}
	} else {
		call(u, "SetProcessDPIAware")
	}
	inst = call(k, "GetModuleHandleW", 0)
	syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControls").Call()
	panelBrush = call(g, "CreateSolidBrush", cr(0x101A1E))
	cardBrush = call(g, "CreateSolidBrush", cr(0x19252A))
	probeMono()
	editBrush = call(g, "CreateSolidBrush", cr(0x28373D))
	register("DCTMain", syscall.NewCallback(mainProc), 3, panelBrush)
	register("DCTRender", syscall.NewCallback(renderProc), 0x23, 0)
	register("DCTSlider", syscall.NewCallback(sliderProc), 3, cardBrush)
	cfg, _ = load(configPath())
	cfg.Renderer = renderers[1]
	arg := ""
	if len(os.Args) > 1 {
		arg = strings.ToLower(os.Args[1])
	}
	if strings.HasPrefix(arg, "/p") || strings.HasPrefix(arg, "-p") {
		mode = "preview"
		var parent uintptr
		if len(os.Args) > 2 {
			fmt.Sscan(os.Args[2], &parent)
		} else if len(arg) > 3 {
			fmt.Sscan(arg[3:], &parent)
		}
		if parent == 0 {
			return
		}
		startSaver(true, parent)
		if len(saverWindows) == 0 {
			return
		}
	} else if arg == "/s" || arg == "-s" {
		mode = "saver"
		startSaver(false, 0)
		if len(saverWindows) == 0 {
			return
		}
	} else {
		mode = "config"
		setupUI()
		if mainwin == 0 {
			return
		}
	}
	go audioWorker()
	ctx, cancelMedia := context.WithCancel(context.Background())
	defer cancelMedia()
	go mediaWorker(ctx)
	var m msg
	mm := syscall.NewLazyDLL("winmm.dll")
	call(mm, "timeBeginPeriod", 1)
	defer call(mm, "timeEndPeriod", 1)
loop:
	for {
		for call(u, "PeekMessageW", ptr(&m), 0, 0, 0, 1) != 0 {
			if m.M == 0x12 {
				break loop
			}
			if mainwin != 0 && len(saverWindows) == 0 && m.M == wmKey && m.W == 0x7A {
				openFull()
				continue
			}
			if mainwin != 0 && len(saverWindows) == 0 && call(u, "IsDialogMessageW", mainwin, ptr(&m)) != 0 {
				continue
			}
			call(u, "TranslateMessage", ptr(&m))
			call(u, "DispatchMessageW", ptr(&m))
		}

		if mainwin != 0 && time.Since(lastAudioStatus) > 250*time.Millisecond {
			status := "음악 반응 꺼짐"
			if cfg.Music {
				status = "오디오 연결 대기"
				if audioState.Connected {
					status = "소리 대기 중"
					if audioState.Active {
						status = "음악 반응 중"
					}
				}
			}
			if m := mediaSnapshot.Load(); cfg.Music && m != nil && m.Title != "" && m.Status == "Playing" {
				status = "음악 재생 중"
			}
			if txt(audioStatus) != status {
				set(audioStatus, status)
			}
			if !lastSaved.IsZero() && time.Since(lastSaved) > 2*time.Second {
				set(controls[idSave], "저장")
				lastSaved = time.Time{}
			}
			lastAudioStatus = time.Now()
		}
		for h := range surfaces {
			if len(saverWindows) > 0 && h == settingsPreview {
				continue
			}
			renderProc(h, wmTimer, 0, 0)
		}
		call(u, "MsgWaitForMultipleObjectsEx", 0, 0, 3, 0x04FF, 4)
	}
	close(audioStop)
	<-audioDone
}

type drawItem struct {
	CtlType, CtlID, ItemID, Action, State uint32
	Hwnd, DC                              uintptr
	R                                     rect
	Data                                  uintptr
}

func rounded(dc uintptr, r rect, radius int, c uint32) {
	brush := call(g, "CreateSolidBrush", cr(c))
	old := call(g, "SelectObject", dc, brush)
	pen := call(g, "SelectObject", dc, call(g, "GetStockObject", 8))
	call(g, "RoundRect", dc, signed(int(r.L)), signed(int(r.T)), signed(int(r.R)), signed(int(r.B)), uintptr(sc(radius)), uintptr(sc(radius)))
	call(g, "SelectObject", dc, pen)
	call(g, "SelectObject", dc, old)
	call(g, "DeleteObject", brush)
}
func drawControl(d *drawItem) {
	id := int(d.CtlID)
	dc := d.DC
	r := d.R
	call(u, "FillRect", dc, ptr(&r), cardBrush)
	old := call(g, "SelectObject", dc, font)
	call(g, "SetBkMode", dc, 1)
	fg := uint32(0xE9F0ED)
	bg := uint32(0x2B3B41)
	active := id == idFull || id == idSave || (id == idVisualPage && currentPage == 1) || (id == idAudioPage && currentPage == 2)
	if toggleIDs[id] {
		on := toggleState[id]
		tr := rect{r.L + int32(sc(2)), r.T + int32(sc(7)), r.L + int32(sc(38)), r.B - int32(sc(7))}
		if on {
			bg = 0xA1E6C5
		}
		rounded(dc, tr, 20, bg)
		knob := tr
		knob.L += int32(sc(3))
		knob.R = knob.L + int32(sc(11))
		knob.T += int32(sc(3))
		knob.B -= int32(sc(3))
		if on {
			knob.R = tr.R - int32(sc(3))
			knob.L = knob.R - int32(sc(11))
		}
		rounded(dc, knob, 14, 0xF5FAF7)
		r.L += int32(sc(46))
		call(g, "SetTextColor", dc, cr(fg))
		call(u, "DrawTextW", dc, w(txt(d.Hwnd)), signed(-1), ptr(&r), 0x824)
		if d.State&0x10 != 0 {
			call(u, "DrawFocusRect", dc, ptr(&r))
		}
	} else {
		if active {
			bg = 0xA1E6C5
			fg = 0x12251F
		}
		if d.State&1 != 0 {
			bg = shade(bg, .82)
		}
		r.R--
		r.B--
		rounded(dc, r, 16, bg)
		call(g, "SetTextColor", dc, cr(fg))
		call(u, "DrawTextW", dc, w(txt(d.Hwnd)), signed(-1), ptr(&r), 0x825)
		if d.State&0x10 != 0 {
			r.L += int32(sc(5))
			r.T += int32(sc(4))
			r.R -= int32(sc(5))
			r.B -= int32(sc(4))
			call(u, "DrawFocusRect", dc, ptr(&r))
		}
	}
	call(g, "SelectObject", dc, old)
}
func probeMono() {
	for _, face := range []string{"JetBrains Mono", "Cascadia Mono"} {
		dc := call(u, "GetDC", 0)
		f := call(g, "CreateFontW", signed(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0x31, w(face))
		old := call(g, "SelectObject", dc, f)
		var b [128]uint16
		call(g, "GetTextFaceW", dc, 128, ptr(&b[0]))
		if strings.EqualFold(syscall.UTF16ToString(b[:]), face) {
			monoFace = face
		}
		call(g, "SelectObject", dc, old)
		call(g, "DeleteObject", f)
		call(u, "ReleaseDC", 0, dc)
		if monoFace == face {
			return
		}
	}
}

type sliderState struct {
	min, max, pos int
	drag          bool
}

var sliders = map[uintptr]*sliderState{}

func sliderProc(h, m, a, b uintptr) uintptr {
	s := sliders[h]
	if s == nil {
		s = &sliderState{min: 0, max: 100}
		sliders[h] = s
	}
	redraw := func() { call(u, "InvalidateRect", h, 0, 0) }
	change := func(v int) {
		v = clamp(v, s.min, s.max)
		if v != s.pos {
			s.pos = v
			redraw()
			if !uiUpdating && mainwin != 0 {
				uiRead()
			}
		}
	}
	fromX := func(x int) {
		r := client(h)
		pad := sc(10)
		span := int(r.R) - 2*pad
		if span > 0 {
			change(s.min + int(math.Round(float64(x-pad)/float64(span)*float64(s.max-s.min))))
		}
	}
	switch m {
	case 0x400:
		return uintptr(s.pos)
	case 0x405:
		change(int(b))
		return 0
	case 0x407:
		s.min = int(b)
		if s.max < s.min {
			s.max = s.min
		}
		change(s.pos)
		redraw()
		return 0
	case 0x408:
		s.max = int(b)
		if s.max < s.min {
			s.max = s.min
		}
		change(s.pos)
		redraw()
		return 0
	case 0x87:
		return 1
	case 0x7, 0x8:
		redraw()
		return 0
	case 0x201:
		call(u, "SetFocus", h)
		call(u, "SetCapture", h)
		s.drag = true
		fromX(int(int16(low(b))))
		return 0
	case wmMouse:
		if s.drag {
			fromX(int(int16(low(b))))
		}
		return 0
	case 0x202:
		if s.drag {
			fromX(int(int16(low(b))))
			s.drag = false
			call(u, "ReleaseCapture")
		}
		return 0
	case 0x215:
		s.drag = false
		return 0
	case wmKey:
		step := 1
		if s.max-s.min > 100 {
			step = 5
		}
		switch a {
		case 0x25, 0x28:
			change(s.pos - step)
		case 0x27, 0x26:
			change(s.pos + step)
		case 0x24:
			change(s.min)
		case 0x23:
			change(s.max)
		case 0x21:
			change(s.pos + step*5)
		case 0x22:
			change(s.pos - step*5)
		}
		return 0
	case wmErase:
		return 1
	case wmPaint:
		var p paint
		call(u, "BeginPaint", h, ptr(&p))
		r := client(h)
		call(u, "FillRect", p.DC, ptr(&r), cardBrush)
		pad := int32(sc(10))
		cy := r.B / 2
		end := r.R - pad
		f := 0.
		if s.max > s.min {
			f = float64(s.pos-s.min) / float64(s.max-s.min)
		}
		x := pad + int32(float64(end-pad)*f)
		rail := rect{pad, cy - int32(sc(3)), end, cy + int32(sc(3))}
		rounded(p.DC, rail, 7, 0x405057)
		fill := rail
		fill.R = x
		if fill.R > fill.L {
			rounded(p.DC, fill, 7, 0xA1E6C5)
		}
		knob := rect{x - int32(sc(7)), cy - int32(sc(7)), x + int32(sc(7)), cy + int32(sc(7))}
		rounded(p.DC, knob, 14, 0xE7FAEF)
		if call(u, "GetFocus") == h {
			r.L += 1
			r.R -= 1
			r.T += 1
			r.B -= 1
			call(u, "DrawFocusRect", p.DC, ptr(&r))
		}
		call(u, "EndPaint", h, ptr(&p))
		return 0
	case wmDestroy:
		delete(sliders, h)
		return 0
	}
	return call(u, "DefWindowProcW", h, m, a, b)
}
