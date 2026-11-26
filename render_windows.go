package main

import (
	"math"
	"time"
	"unsafe"
)

type pfd struct {
	Size, Version                                                                                                                                                                                                               uint16
	Flags                                                                                                                                                                                                                       uint32
	PixelType, ColorBits, RedBits, RedShift, GreenBits, GreenShift, BlueBits, BlueShift, AlphaBits, AlphaShift, AccumBits, AccumRed, AccumGreen, AccumBlue, AccumAlpha, DepthBits, StencilBits, AuxBuffers, LayerType, Reserved byte
	LayerMask, VisibleMask, DamageMask                                                                                                                                                                                          uint32
}
type surface struct {
	hwnd, dc, rc, list, glfont, gdifont, mem, bmp, oldbmp uintptr
	width, height, fontsize, bw, bh                       int
	failed, pixelSet, saver                               bool
	last, begin                                           time.Time
	angle                                                 float64
	uiFont, uiBold, uiCode                                uintptr
	uiFontSize                                            int
	next                                                  time.Time
	previewStory                                          storyKind
	previewStoryStart                                     time.Time
}

func (s *surface) free() {
	for _, f := range []uintptr{s.uiFont, s.uiBold, s.uiCode} {
		if f != 0 {
			call(g, "DeleteObject", f)
		}
	}
	if s.rc != 0 {
		call(gl, "wglMakeCurrent", s.dc, s.rc)
		if s.list != 0 {
			call(gl, "glDeleteLists", s.list, 96)
		}
		call(gl, "wglMakeCurrent", 0, 0)
		call(gl, "wglDeleteContext", s.rc)
	}
	if s.glfont != 0 {
		call(g, "DeleteObject", s.glfont)
	}
	if s.mem != 0 {
		call(g, "SelectObject", s.mem, s.oldbmp)
		call(g, "DeleteObject", s.bmp)
		call(g, "DeleteDC", s.mem)
	}
	if s.gdifont != 0 {
		call(g, "DeleteObject", s.gdifont)
	}
	call(u, "KillTimer", s.hwnd, 1)
	call(u, "ReleaseDC", s.hwnd, s.dc)
}
func (s *surface) initGL() bool {
	if s.failed {
		return false
	}
	if s.rc != 0 {
		return call(gl, "wglMakeCurrent", s.dc, s.rc) != 0
	}
	if !s.pixelSet {
		p := pfd{Size: uint16(unsafe.Sizeof(pfd{})), Version: 1, Flags: 0x25, ColorBits: 32, DepthBits: 0}
		fmt := call(g, "ChoosePixelFormat", s.dc, ptr(&p))
		if fmt == 0 || call(g, "SetPixelFormat", s.dc, fmt, ptr(&p)) == 0 {
			s.failed = true
			return false
		}
		s.pixelSet = true
	}
	s.rc = call(gl, "wglCreateContext", s.dc)
	if s.rc == 0 || call(gl, "wglMakeCurrent", s.dc, s.rc) == 0 {
		s.failed = true
		return false
	}
	return true
}
func mono(size int) uintptr {
	return call(g, "CreateFontW", signed(-size), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0x31, w(monoFace))
}
func (s *surface) glFont(size int) bool {
	if s.list != 0 && s.fontsize == size {
		return true
	}
	if s.list != 0 {
		call(gl, "glDeleteLists", s.list, 96)
		s.list = 0
	}
	if s.glfont != 0 {
		call(g, "DeleteObject", s.glfont)
	}
	s.glfont = mono(size)
	old := call(g, "SelectObject", s.dc, s.glfont)
	s.list = call(gl, "glGenLists", 96)
	ok := s.list != 0 && call(gl, "wglUseFontBitmapsW", s.dc, 32, 96, s.list) != 0
	call(g, "SelectObject", s.dc, old)
	s.fontsize = size
	if !ok {
		s.failed = true
	}
	return ok
}
func glColor(c uint32, a float64) {
	call(gl, "glColor4ub", uintptr(c>>16&255), uintptr(c>>8&255), uintptr(c&255), uintptr(math.Round(a*255)))
}
func (s *surface) draw(force bool) {
	now := time.Now()
	dt := now.Sub(s.last).Seconds()
	if !force && now.Before(s.next) {
		return
	}
	period := time.Duration(float64(time.Second) / float64(effectiveFPS()))
	if s.next.IsZero() || now.Sub(s.next) > period {
		s.next = now.Add(period)
	} else {
		s.next = s.next.Add(period)
	}
	s.last = now
	if dt > .15 {
		dt = .15
	}
	sign := 1.
	if cfg.Direction == "Counterclockwise" {
		sign = -1
	}
	if cfg.Direction == "Static" {
		sign = 0
	}
	if !paused || !s.saver {
		s.angle += dt * .85 * float64(cfg.Speed) / 100 * sign
	}
	r := client(s.hwnd)
	dw, dh := int(r.R), int(r.B)
	if dw < 10 || dh < 10 {
		return
	}
	ww, hh := dw, dh
	if s.saver && fullScreen {
		ww, hh = fullscreenSize(cfg.FullscreenResolution, dw, dh)
	}
	sceneH := hh
	if cfg.Music && ww >= 280 && hh >= 190 {
		sceneH = hh - clamp(hh/6, 104, 168)
	}
	line := clamp(hh/45, 13, 38) + 6
	var couple coupleDisplay
	if !s.saver || fullScreen {
		couple, _ = coupleCaption(cfg.Couple, now)
	}
	treeH := sceneH - footerHeight(ww, sceneH, line, cfg.Messages, s.saver && fullScreen, couple.firstName != "")
	size := clamp(treeH/52, 9, 46)
	cw := int(math.Round(float64(size) * .62))
	ch := int(math.Round(float64(size) * 1.18))
	if cw < 5 {
		cw = 5
	}
	cols, rows := ww/cw, treeH/ch
	cols = clamp(cols, 1, 220)
	rows = clamp(rows, 1, 100)
	sceneCfg := cfg
	sceneCfg.Message = ""
	elapsed := now.Sub(s.begin)
	story := storyDuring(s.previewStory, now.Sub(s.previewStoryStart))
	if story.kind == noStory && !s.previewStoryStart.IsZero() {
		s.previewStoryStart = time.Time{}
		set(controls[idStoryStatus], noStory.title())
	}
	if story.kind == noStory && cfg.Stories {
		story = scheduledStory(elapsed)
	}
	f := composeFrame(cols, rows, elapsed.Seconds(), s.angle, audioState, sceneCfg, currentStats, false, story)
	ox := (ww - cols*cw) / 2
	oy := (treeH - rows*ch) / 2
	s.drawGDI(f, ww, hh, cw, ch, ox, oy, size, sceneH, treeH, couple)
	if s.mem == 0 {
		return
	}
	if ww == dw && hh == dh {
		call(g, "BitBlt", s.dc, 0, 0, uintptr(dw), uintptr(dh), s.mem, 0, 0, 0xCC0020)
		return
	}
	w, h := fitFrame(ww, hh, dw, dh)
	if w != dw || h != dh {
		call(g, "PatBlt", s.dc, 0, 0, uintptr(dw), uintptr(dh), 0x42)
	}
	// ASCII 글자의 가장자리를 보존하고 큰 화면을 매 프레임 보간하는 비용을 줄입니다.
	call(g, "SetStretchBltMode", s.dc, 3)
	call(g, "StretchBlt", s.dc, uintptr((dw-w)/2), uintptr((dh-h)/2), uintptr(w), uintptr(h), s.mem, 0, 0, uintptr(ww), uintptr(hh), 0xCC0020)
}
func (s *surface) drawGL(f Frame, ww, hh, cw, ch, ox, oy, size int) {
	call(gl, "glViewport", 0, 0, uintptr(ww), uintptr(hh))
	call(gl, "glMatrixMode", 0x1701)
	call(gl, "glLoadIdentity")
	call(gl, "glOrtho", f64(0), f64(float64(ww)), f64(float64(hh)), f64(0), f64(-1), f64(1))
	call(gl, "glMatrixMode", 0x1700)
	call(gl, "glLoadIdentity")
	c := f.BG
	call(gl, "glClearColor", f32(float64(c>>16&255)/255), f32(float64(c>>8&255)/255), f32(float64(c&255)/255), f32(1))
	call(gl, "glClear", 0x4000)
	if cfg.Bloom && !cfg.LowPower {
		call(gl, "glEnable", 0x0BE2)
		call(gl, "glBlendFunc", 0x0302, 1)
		call(gl, "glEnable", 0x0B10)
		for _, layer := range []struct{ sz, a float64 }{{2.2, .035}, {1.4, .06}, {.75, .12}} {
			call(gl, "glPointSize", f32(float64(size)*layer.sz))
			call(gl, "glBegin", 0)
			for _, v := range f.Cells {
				if !v.Glow {
					continue
				}
				glColor(v.Color, math.Min(1, layer.a*(1+.45*audioState.Level)))
				call(gl, "glVertex2i", uintptr(ox+v.X*cw+cw/2), uintptr(oy+v.Y*ch+ch/2))
			}
			call(gl, "glEnd")
		}
		call(gl, "glDisable", 0x0B10)
		call(gl, "glDisable", 0x0BE2)
	}
	for _, v := range f.Cells {
		if v.Ch < 32 || v.Ch > 127 {
			continue
		}
		glColor(v.Color, 1)
		call(gl, "glRasterPos2i", uintptr(ox+v.X*cw), uintptr(oy+v.Y*ch+size))
		call(gl, "glCallList", s.list+uintptr(v.Ch-32))
	}
	call(g, "SwapBuffers", s.dc)
}
func (s *surface) drawGDI(f Frame, ww, hh, cw, ch, ox, oy, size, sceneH, treeH int, couple coupleDisplay) {
	if s.mem == 0 || s.bw != ww || s.bh != hh {
		if s.mem != 0 {
			call(g, "SelectObject", s.mem, s.oldbmp)
			call(g, "DeleteObject", s.bmp)
			call(g, "DeleteDC", s.mem)
		}
		s.mem = call(g, "CreateCompatibleDC", s.dc)
		s.bmp = call(g, "CreateCompatibleBitmap", s.dc, uintptr(ww), uintptr(hh))
		if s.mem == 0 || s.bmp == 0 {
			if s.mem != 0 {
				call(g, "DeleteDC", s.mem)
			}
			if s.bmp != 0 {
				call(g, "DeleteObject", s.bmp)
			}
			s.mem = 0
			s.bmp = 0
			return
		}
		s.oldbmp = call(g, "SelectObject", s.mem, s.bmp)
		s.bw, s.bh = ww, hh
	}
	if s.gdifont == 0 || s.height != size {
		if s.gdifont != 0 {
			call(g, "DeleteObject", s.gdifont)
		}
		s.gdifont = mono(size)
		s.height = size
	}
	r := rect{0, 0, int32(ww), int32(hh)}
	brush := call(g, "CreateSolidBrush", cr(f.BG))
	call(u, "FillRect", s.mem, ptr(&r), brush)
	call(g, "DeleteObject", brush)
	old := call(g, "SelectObject", s.mem, s.gdifont)
	call(g, "SetBkMode", s.mem, 1)
	if cfg.Bloom && !cfg.LowPower {
		for _, v := range f.Cells {
			if !v.Glow {
				continue
			}
			x, y := ox+v.X*cw, oy+v.Y*ch
			rr := rect{int32(x), int32(y + 2), int32(x + cw), int32(y + ch - 2)}
			rounded(s.mem, rr, 6, shade(v.Color, .10+.06*audioState.Level))
		}
	}
	for _, v := range f.Cells {
		call(g, "SetTextColor", s.mem, cr(v.Color))
		chv := uint16(v.Ch)
		if v.Ch > 0xffff {
			chv = '?'
		}
		call(g, "TextOutW", s.mem, uintptr(ox+v.X*cw), uintptr(oy+v.Y*ch), ptr(&chv), 1)
	}
	call(g, "SelectObject", s.mem, old)
	s.drawOverlay(ww, hh, sceneH, treeH, couple)
}

func effectiveFPS() int {
	if cfg.LowPower && cfg.FPS > 30 {
		return 30
	}
	return cfg.FPS
}
