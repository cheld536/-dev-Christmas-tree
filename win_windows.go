package main

import (
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
var lastUI time.Time
var settingsPreview uintptr
var previewGL bool
var pointersKeep [2048]any
var pointerIndex int
var panelBrush, editBrush uintptr

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
var audio float64
var audioCapture *capture
var audioRetry time.Time
var lastAudio time.Time

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
	h := window(cls, title, wsChild|wsVisible|style, 0, x, y, ww, hh, mainwin, uintptr(id))
	if id > 0 {
		controls[id] = h
	}
	send(h, wmSetFont, font, 1)
	if cls == "BUTTON" || cls == "COMBOBOX" || cls == "EDIT" || cls == "msctls_trackbar32" {
		call(syscall.NewLazyDLL("uxtheme.dll"), "SetWindowTheme", h, w("DarkMode_Explorer"), 0)
	}
	return h
}
func label(s string, x, y, ww, hh int) { child("STATIC", s, 0, x, y, ww, hh, 0) }
func combo(id, x, y int, items []string) {
	h := child("COMBOBOX", "", wsTab|wsVScroll|3, x, y, 230, 250, id)
	for _, s := range items {
		send(h, 0x143, 0, w(s))
	}
}
func slider(id, x, y, min, max int) {
	h := child("msctls_trackbar32", "", wsTab|1, x, y, 188, 28, id)
	send(h, 0x407, 1, uintptr(min))
	send(h, 0x408, 1, uintptr(max))
	child("STATIC", "", 0, x+190, y+4, 65, 23, id+100)
}
func check(id, x, y int, s string)      { child("BUTTON", s, wsTab|3, x, y, 172, 23, id) }
func button(id, x, y, ww int, s string) { child("BUTTON", s, wsTab, x, y, ww, 31, id) }

const (
	idTheme    = 101
	idDir      = 102
	idRenderer = 103
	idSpeed    = 110
	idLight    = 111
	idFPS      = 112
	idSize     = 113
	idSnow     = 120
	idOrn      = 121
	idMsg      = 122
	idBloom    = 123
	idMatrix   = 124
	idCount    = 125
	idMonitor  = 126
	idMusic    = 127
	idMulti    = 128
	idPhysics  = 129
	idCustom   = 130
	idBG       = 131
	idMessage  = 132
	idSave     = 140
	idReset    = 141
	idTest     = 142
	idExport   = 143
	idImport   = 144
	idWindows  = 145
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
	selectCombo(idTheme, themes, cfg.Theme)
	selectCombo(idDir, directions, cfg.Direction)
	selectCombo(idRenderer, renderers, cfg.Renderer)
	for id, v := range map[int]int{idSpeed: cfg.Speed, idLight: cfg.LightSpeed, idFPS: cfg.FPS, idSize: cfg.Size} {
		send(controls[id], 0x405, 1, uintptr(v))
	}
	for id, v := range map[int]bool{idSnow: cfg.Snow, idOrn: cfg.Ornaments, idMsg: cfg.Messages, idBloom: cfg.Bloom, idMatrix: cfg.Matrix, idCount: cfg.Countdown, idMonitor: cfg.Monitor, idMusic: cfg.Music, idMulti: cfg.Multi, idPhysics: cfg.SnowPhysics} {
		val := uintptr(0)
		if v {
			val = 1
		}
		send(controls[id], 0xf1, val, 0)
	}
	set(controls[idCustom], "Tree: "+cfg.Custom)
	set(controls[idBG], "Background: "+cfg.Background)
	set(controls[idMessage], cfg.Message)
	uiRead()
}
func uiRead() {
	if mainwin == 0 {
		return
	}
	cfg.Theme = comboVal(idTheme, themes)
	cfg.Direction = comboVal(idDir, directions)
	cfg.Renderer = comboVal(idRenderer, renderers)
	for id, p := range map[int]*int{idSpeed: &cfg.Speed, idLight: &cfg.LightSpeed, idFPS: &cfg.FPS, idSize: &cfg.Size} {
		*p = int(send(controls[id], 0x400, 0, 0))
		s := fmt.Sprint(*p)
		if id == idSpeed || id == idLight {
			s = fmt.Sprintf("%.2fx", float64(*p)/100)
		} else if id == idSize {
			s += "%"
		}
		if txt(controls[id+100]) != s {
			set(controls[id+100], s)
		}
	}
	for id, p := range map[int]*bool{idSnow: &cfg.Snow, idOrn: &cfg.Ornaments, idMsg: &cfg.Messages, idBloom: &cfg.Bloom, idMatrix: &cfg.Matrix, idCount: &cfg.Countdown, idMonitor: &cfg.Monitor, idMusic: &cfg.Music, idMulti: &cfg.Multi, idPhysics: &cfg.SnowPhysics} {
		*p = send(controls[id], 0xf0, 0, 0) == 1
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
			set(controls[id], "Background: "+v)
		} else {
			cfg.Custom = v
			set(controls[id], "Tree: "+v)
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
	uiRead()
	switch id {
	case idCustom, idBG:
		chooseColor(id)
	case idSave:
		if e := save(configPath(), cfg); e != nil {
			box(mainwin, e.Error(), 0x10)
		} else {
			box(mainwin, "설정을 저장했습니다.", 0x40)
		}
	case idReset:
		cfg = defaults()
		uiSet()
	case idTest:
		startSaver(false, 0)
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
func setupUI() {
	var work rect
	call(u, "SystemParametersInfoW", 0x30, 0, ptr(&work), 0)
	x := int(work.L) + (int(work.R-work.L)-1120)/2
	y := int(work.T) + (int(work.B-work.T)-765)/2
	if x < int(work.L) {
		x = int(work.L)
	}
	if y < int(work.T) {
		y = int(work.T)
	}
	mainwin = window("DCTMain", "Developer Christmas Tree  |  Settings", 0x00CA0000, 0, x, y, 1120, 765, 0, 0)
	if mainwin == 0 {
		box(0, "Unable to create settings window.", 0x10)
		return
	}
	dark := int32(1)
	call(syscall.NewLazyDLL("dwmapi.dll"), "DwmSetWindowAttribute", mainwin, 20, ptr(&dark), 4)
	font = call(g, "CreateFontW", signed(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, w("Segoe UI"))
	label("실시간 미리보기  /  3D ASCII RENDERER", 20, 12, 620, 24)
	p := window("DCTRender", "", wsChild|wsVisible|wsBorder, 0, 20, 40, 600, 626, mainwin, 0)
	addSurface(p, false)
	settingsPreview = p
	previewGL = wantsGL()
	label("트리 설정", 640, 12, 430, 24)
	label("색상 테마", 640, 43, 150, 22)
	combo(idTheme, 790, 40, themes)
	label("회전 방향", 640, 81, 150, 22)
	combo(idDir, 790, 78, directions)
	label("렌더링 방식", 640, 119, 150, 22)
	combo(idRenderer, 790, 116, renderers)
	label("회전 속도", 640, 157, 150, 22)
	slider(idSpeed, 790, 151, 0, 400)
	label("조명 변화 속도", 640, 194, 150, 22)
	slider(idLight, 790, 188, 0, 400)
	label("목표 FPS", 640, 231, 150, 22)
	slider(idFPS, 790, 225, 10, 144)
	label("트리 크기", 640, 268, 150, 22)
	slider(idSize, 790, 262, 45, 100)
	button(idCustom, 640, 302, 195, "Tree color")
	button(idBG, 846, 302, 195, "Background")
	label("시각 효과", 640, 347, 400, 22)
	check(idSnow, 640, 374, "눈 내리기")
	check(idOrn, 837, 374, "코드 장식")
	check(idMsg, 640, 401, "메시지 표시")
	check(idBloom, 837, 401, "LED 빛 번짐")
	check(idMatrix, 640, 428, "매트릭스 레인")
	check(idCount, 837, 428, "크리스마스 D-Day")
	check(idMonitor, 640, 455, "CPU / GPU / RAM")
	check(idMusic, 837, 455, "시스템 음악 반응")
	check(idMulti, 640, 482, "다중 모니터")
	check(idPhysics, 837, 482, "바람 및 눈 쌓임")
	label("사용자 메시지 (비워 두면 개발자 문구 순환)", 640, 518, 440, 24)
	h := child("EDIT", "", wsTab|wsBorder|0x80, 640, 543, 401, 28, idMessage)
	send(h, 0xc5, 100, 0)
	button(idSave, 640, 588, 124, "설정 저장")
	button(idReset, 773, 588, 124, "기본값 복원")
	button(idTest, 906, 588, 135, "전체 화면 테스트")
	button(idExport, 640, 627, 124, "프리셋 내보내기")
	button(idImport, 773, 627, 124, "프리셋 불러오기")
	button(idWindows, 906, 627, 135, "Windows 설정")
	label("설정은 실시간 반영됩니다. 자동 화면 보호기에 적용하려면 저장하세요.  Esc 또는 마우스 이동으로 종료합니다.", 20, 680, 1050, 24)
	uiSet()
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
func refreshPreview() {
	if settingsPreview == 0 || mainwin == 0 {
		return
	}
	want := wantsGL()
	if want == previewGL {
		return
	}
	previewGL = want
	old := settingsPreview
	var a float64
	if sf := surfaces[old]; sf != nil {
		a = sf.angle
	}
	call(u, "DestroyWindow", old)
	settingsPreview = window("DCTRender", "", wsChild|wsVisible|wsBorder, 0, 20, 40, 600, 626, mainwin, 0)
	addSurface(settingsPreview, false)
	if sf := surfaces[settingsPreview]; sf != nil {
		sf.angle = a
	}
}
func addSurface(h uintptr, saver bool) {
	if h == 0 {
		return
	}
	surfaces[h] = &surface{hwnd: h, dc: call(u, "GetDC", h), saver: saver, last: time.Now(), begin: time.Now()}

}
func stopSaver() {
	for _, h := range saverWindows {
		call(u, "DestroyWindow", h)
	}
	saverWindows = nil
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
			if mainwin != 0 && time.Since(lastUI) > 30*time.Millisecond {
				uiRead()
				lastUI = time.Now()
			}
			updateExtras()
			sf.draw(false)
			if sf.saver && !previewOnly && time.Now().After(exitAt) {
				var p pt
				call(u, "GetCursorPos", ptr(&p))
				if math.Abs(float64(p.X-mouseStart.X))+math.Abs(float64(p.Y-mouseStart.Y)) > 7 {
					stopSaver()
				}
			}
		}
		return 0
	case wmKey, wmSysKey, 0x201, 0x204, 0x207:
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
	case 0x0138, 0x0135, 0x0134: // static / dialog / list box
		call(g, "SetTextColor", a, cr(0xE5EDF7))
		call(g, "SetBkColor", a, cr(0x111D2B))
		return panelBrush
	case 0x0133: // edit
		call(g, "SetTextColor", a, cr(0xE5EDF7))
		call(g, "SetBkColor", a, cr(0x203145))
		return editBrush
	case wmCommand:
		if high(a) == 0 && low(a) >= 130 && low(a) <= 145 {
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
	c.Size = uint32(unsafe.Sizeof(c))
	call(u, "RegisterClassExW", ptr(&c))
}
func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	call(u, "SetProcessDPIAware")
	inst = call(k, "GetModuleHandleW", 0)
	syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControls").Call()
	panelBrush = call(g, "CreateSolidBrush", cr(0x111D2B))
	editBrush = call(g, "CreateSolidBrush", cr(0x203145))
	register("DCTMain", syscall.NewCallback(mainProc), 3, panelBrush)
	register("DCTRender", syscall.NewCallback(renderProc), 0x23, 0)
	cfg, _ = load(configPath())
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
			if mainwin != 0 && len(saverWindows) == 0 && call(u, "IsDialogMessageW", mainwin, ptr(&m)) != 0 {
				continue
			}
			call(u, "TranslateMessage", ptr(&m))
			call(u, "DispatchMessageW", ptr(&m))
		}
		refreshPreview()
		for h := range surfaces {
			renderProc(h, wmTimer, 0, 0)
		}
		call(u, "MsgWaitForMultipleObjectsEx", 0, 0, 3, 0x04FF, 4)
	}
	if audioCapture != nil {
		audioCapture.close()
	}
}
