package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

func fill(dc uintptr, r rect, c uint32) {
	if r.R <= r.L || r.B <= r.T {
		return
	}
	b := call(g, "CreateSolidBrush", cr(c))
	call(u, "FillRect", dc, ptr(&r), b)
	call(g, "DeleteObject", b)
}
func textAt(dc, face uintptr, r rect, str string, col uint32, flags uintptr) {
	if r.R <= r.L || r.B <= r.T {
		return
	}
	old := call(g, "SelectObject", dc, face)
	call(g, "SetBkMode", dc, 1)
	call(g, "SetTextColor", dc, cr(col))
	call(u, "DrawTextW", dc, w(strings.ReplaceAll(str, "\x00", "")), signed(-1), ptr(&r), flags|0x800|0x8000|0x20|4)
	call(g, "SelectObject", dc, old)
}
func timeLabel(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		v = 0
	}
	if v > 359999 {
		v = 359999
	}
	n := int(v)
	return fmt.Sprintf("%d:%02d", n/60, n%60)
}
func (s *surface) overlayFonts(scale int) {
	if s.uiFont != 0 && s.uiFontSize == scale {
		return
	}
	for _, f := range []uintptr{s.uiFont, s.uiBold, s.uiCode} {
		if f != 0 {
			call(g, "DeleteObject", f)
		}
	}
	s.uiFont = call(g, "CreateFontW", signed(-scale), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, w("Segoe UI"))
	s.uiBold = call(g, "CreateFontW", signed(-scale-2), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, w("Segoe UI"))
	s.uiCode = mono(scale)
	s.uiFontSize = scale
}

type bitmapHeader struct {
	Size                   uint32
	W, H                   int32
	Planes, Bits           uint16
	Compression, ImageSize uint32
	XPels, YPels           int32
	Used, Important        uint32
}

func artAt(dc uintptr, m *mediaInfo, x, y, size int) {
	if m == nil || len(m.Pixels) == 0 || size < 1 {
		return
	}
	info := bitmapHeader{Size: uint32(unsafe.Sizeof(bitmapHeader{})), W: int32(m.ArtW), H: -int32(m.ArtH), Planes: 1, Bits: 32}
	call(g, "SetStretchBltMode", dc, 4)
	call(g, "StretchDIBits", dc, signed(x), signed(y), uintptr(size), uintptr(size), 0, 0, uintptr(m.ArtW), uintptr(m.ArtH), uintptr(unsafe.Pointer(&m.Pixels[0])), ptr(&info), 0, 0xCC0020)
	runtime.KeepAlive(m)
}
func (s *surface) drawCoupleLine(bounds rect, couple coupleDisplay) {
	dc := s.mem
	old := call(g, "SelectObject", dc, s.uiFont)
	textWidth := func(text string) int32 {
		var size pt
		length := len(utf16.Encode([]rune(text)))
		call(g, "GetTextExtentPoint32W", dc, w(text), uintptr(length), ptr(&size))
		return size.X
	}
	const heart = "♥"
	gap := max(2, textWidth(" "))
	first, second := textWidth(couple.firstName)+1, textWidth(couple.secondName)+1
	heartWidth, counterWidth := textWidth(heart)+1, textWidth(couple.counter)+1
	call(g, "SelectObject", dc, old)

	available := bounds.R - bounds.L
	counterWidth = min(counterWidth, max(0, available-heartWidth-4*gap))
	first, second = fitCoupleNames(first, second, available-heartWidth-counterWidth-4*gap)
	left := bounds.L + max(0, (available-first-second-heartWidth-counterWidth-4*gap)/2)
	draw := func(text string, width int32, color uint32) {
		textAt(dc, s.uiFont, rect{left, bounds.T, min(left+width, bounds.R), bounds.B}, text, color, 0)
		left += width
	}
	const tint = 0xFFD4E7
	draw(couple.firstName, first, tint)
	left += gap
	draw(heart, heartWidth, 0xFF454F)
	left += gap
	draw(couple.secondName, second, tint)
	left += 2 * gap
	draw(couple.counter, counterWidth, tint)
}

func (s *surface) drawOverlay(ww, hh, sceneH, treeH int, couple coupleDisplay) {
	dc := s.mem
	if ww < 200 || hh < 120 {
		return
	}
	scale := clamp(hh/45, 13, 38)
	s.overlayFonts(scale)
	pad := clamp(hh/55, 8, 22)
	line := scale + 6
	if cfg.Music && sceneH < hh {
		x := pad
		y := sceneH + 4
		right := ww - pad
		h := hh - y - pad
		rounded(dc, rect{int32(x), int32(y), int32(right), int32(y + h)}, 16, 0x15232E)
		m := mediaSnapshot.Load()
		title, sub := "재생 정보 없음", "시스템 오디오 연결 대기"
		if audioState.Connected {
			sub = "재생을 기다리는 중"
			if audioState.Active {
				sub = "시스템 오디오 감지됨 · 곡 정보 미제공"
			}
		}
		if m != nil && m.Title != "" && time.Since(m.Updated) < 12*time.Second {
			title = m.Title
			sub = m.Artist
			if sub == "" {
				sub = m.Source
			}
			if m.Status != "Playing" {
				sub += "  ·  일시 정지 / 중지"
			}
		}
		textRight := right - pad
		textLeft := x + pad
		if m != nil && m.Title != "" && len(m.Pixels) > 0 && time.Since(m.Updated) < 12*time.Second && ww >= 430 {
			size := clamp(h-pad, 32, 104)
			artAt(dc, m, textLeft, y+(h-size)/2, size)
			textLeft += size + pad
		}
		barW := 0
		if cfg.Spectrum {
			barW = clamp((right-x)/3, 65, 280)
			textRight -= barW + pad
		}
		textAt(dc, s.uiBold, rect{int32(textLeft), int32(y + pad/2), int32(textRight), int32(y + pad/2 + line + 3)}, title, 0xF4F8FA, 0)
		textAt(dc, s.uiFont, rect{int32(textLeft), int32(y + pad/2 + line + 3), int32(textRight), int32(y + pad/2 + 2*line + 3)}, sub, 0xA8BFCC, 0)
		if m != nil && m.Title != "" && m.Duration > 0 && time.Since(m.Updated) < 12*time.Second {
			pos := m.Position
			if m.Status == "Playing" {
				pos += time.Since(m.Updated).Seconds()
			}
			pos = math.Max(0, math.Min(m.Duration, pos))
			yy := y + h - pad/2 - 5
			left := textLeft
			end := textRight - 2
			if yy > y+pad/2+2*line+3 && end > left {
				fill(dc, rect{int32(left), int32(yy), int32(end), int32(yy + 3)}, 0x40535F)
				fill(dc, rect{int32(left), int32(yy), int32(left + int(float64(end-left)*pos/m.Duration)), int32(yy + 3)}, 0x84E4C3)
				if yy-line >= y+pad/2+2*line+3 {
					textAt(dc, s.uiFont, rect{int32(left), int32(yy - line), int32(end), int32(yy)}, timeLabel(pos)+" / "+timeLabel(m.Duration), 0xA8BFCC, 0)
				}
			}
		}
		if barW > 0 {
			n := clamp(barW/5, 12, 48)
			gap := 2
			bw := barW / n
			baseline := y + h - pad
			maxH := h - 2*pad
			for i := 0; i < n; i++ {
				lo := i * 64 / n
				hi := (i + 1) * 64 / n
				v := 0.
				for j := lo; j < hi; j++ {
					v = math.Max(v, audioState.Spectrum[j])
				}
				v = math.Sqrt(math.Max(0, math.Min(1, v)))
				bh := 2 + int(v*float64(maxH-2))
				xx := right - pad - barW + i*bw
				fill(dc, rect{int32(xx), int32(baseline - bh), int32(xx + bw - gap), int32(baseline)}, hsv(.45+.17*float64(i)/float64(n), .55, .93))
			}
		}
	}
	if treeH < sceneH {
		now := time.Now()
		isFull := s.saver && fullScreen
		event := captionNone
		if isFull {
			event = footerEvent
		}
		code, comment := developerCaption(now, now.Sub(s.begin), cfg, isFull && paused, event, footerUntil)
		y := treeH
		drawLine := func(font uintptr, text string, color uint32) {
			textAt(dc, font, rect{int32(pad), int32(y), int32(ww - pad), int32(y + line)}, text, color, 1)
			y += line
		}
		if couple.firstName != "" {
			s.drawCoupleLine(rect{int32(pad), int32(y), int32(ww - pad), int32(y + line)}, couple)
			y += line
		} else if code != "" {
			budget := sceneH - treeH
			if isFull {
				budget -= line
			}
			if budget >= 2*line {
				drawLine(s.uiCode, "<<<  HAPPY CODING  >>>", 0xFFD66E)
			}
			if comment != "" && ww >= 25*scale {
				drawLine(s.uiFont, code+"  # "+comment, 0x69E6FF)
			} else if strings.TrimSpace(cfg.Message) != "" {
				drawLine(s.uiFont, code, 0x69E6FF)
			} else {
				drawLine(s.uiCode, code, 0x69E6FF)
			}
		}
		if isFull {
			drawLine(s.uiCode, "Esc / F11", 0x9BACBF)
		}
	}
	if s.saver && fullScreen && overlayVisible && ww >= 360 && sceneH >= 280 {
		keys := [][2]string{{"F11 / Esc", "돌아가기"}, {"Space", "회전 멈춤"}, {"↑ / ↓", "회전 속도"}, {"← / →", "회전 방향"}, {"C / D", "색상 / 코드 스타일"}, {"A / P", "음악 / 프리셋"}, {"S / M", "눈 / 코드 비"}, {"B / V", "빛 / 이퀄라이저"}, {"G / J", "음악 강도"}, {"Z / X / N", "이전 / 재생 / 다음"}, {"Tab", "시스템 사용량"}, {"H", "단축키 닫기"}}
		width := clamp(ww/3, 320, 490)
		x := ww - width - pad
		y := pad
		rowH := clamp((sceneH-4*pad-line*2)/len(keys), 17, line+9)
		h := rowH*len(keys) + line + 2*pad
		rounded(dc, rect{int32(x), int32(y), int32(x + width), int32(y + h)}, 20, 0x14232D)
		textAt(dc, s.uiBold, rect{int32(x + pad), int32(y + pad/2), int32(x + width - pad), int32(y + pad/2 + line)}, "단축키", 0xF5FAFD, 0)
		for i, k := range keys {
			yy := y + pad + line + i*rowH
			kw := clamp(width/3, 98, 145)
			r := rect{int32(x + pad), int32(yy), int32(x + pad + kw), int32(yy + rowH - 2)}
			rounded(dc, r, 8, 0x2A3D49)
			textAt(dc, s.uiFont, r, k[0], 0xDDF0F5, 1)
			textAt(dc, s.uiFont, rect{r.R + 8, r.T, int32(x + width - pad), r.B}, k[1], 0xC0CFD6, 0)
		}
	}
	if s.saver && fullScreen && !overlayVisible {
		textAt(dc, s.uiFont, rect{int32(ww - 200), int32(pad), int32(ww - pad), int32(pad + line)}, "H  ·  단축키", 0x93A8B4, 2)
	}
	if s.saver && fullScreen && time.Now().Before(toastUntil) {
		width := clamp(ww/4, 180, 330)
		x := (ww - width) / 2
		y := 2*line + pad
		r := rect{int32(x), int32(y), int32(x + width), int32(y + line + pad)}
		rounded(dc, r, 14, 0x223B44)
		textAt(dc, s.uiBold, r, toastText, 0xF1F9F7, 1)
	}
}
