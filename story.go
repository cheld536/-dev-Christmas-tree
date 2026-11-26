package main

import (
	"math"
	"time"
)

type storyKind uint8

const (
	noStory storyKind = iota
	sleighStory
	giftStory
	matchStory
	snowmanStory
	starStory
)

const lastStory = starStory
const storyCycle = 106 * time.Second

func (kind storyKind) title() string {
	return [...]string{"누를 때마다 다음 장면", "산타와 루돌프", "트리 아래 선물", "성냥불과 작은 손님", "눈사람의 목도리", "겨울 하늘의 별똥별"}[kind]
}

type storyMoment struct {
	kind     storyKind
	progress float64
}

func (kind storyKind) duration() time.Duration {
	switch kind {
	case sleighStory, starStory:
		return 8 * time.Second
	case giftStory, matchStory, snowmanStory:
		return 12 * time.Second
	}
	return 0
}

func storyDuring(kind storyKind, elapsed time.Duration) storyMoment {
	if duration := kind.duration(); elapsed >= 0 && elapsed < duration {
		return storyMoment{kind, float64(elapsed) / float64(duration)}
	}
	return storyMoment{}
}

func scheduledStory(elapsed time.Duration) storyMoment {
	t := elapsed % storyCycle
	switch {
	case t >= 90*time.Second:
		return storyDuring(starStory, t-90*time.Second)
	case t >= 72*time.Second:
		return storyDuring(snowmanStory, t-72*time.Second)
	case t >= 52*time.Second:
		return storyDuring(matchStory, t-52*time.Second)
	case t >= 28*time.Second:
		return storyDuring(giftStory, t-28*time.Second)
	default:
		return storyDuring(sleighStory, t-8*time.Second)
	}
}

const rudolphRed uint32 = 0xFF424F

func sleighPosition(cols int, flight float64) (int, int) {
	return -27 + int(flight*float64(cols+27)), 2 + int(math.Round(1+math.Sin(flight*math.Pi*3)))
}

func drawStory(cols, rows, groundY int, moment storyMoment, put func(int, int, rune, uint32, float64, bool)) {
	if moment.kind == noStory || cols < 40 || rows < 19 {
		return
	}
	p := moment.progress
	paint := func(x, y int, line string, color uint32) {
		for i, ch := range line {
			if ch != ' ' {
				put(x+i, y, ch, color, 2e5, false)
			}
		}
	}
	spark := func(x, y int, ch rune, color uint32) {
		put(x, y, ch, color, 2e5, true)
	}
	const snow = 0xF1F6FF
	const gold = 0xFFCA70
	const deer = 0xE2A676
	const red = 0xFF5E6A

	const giftFlight = .72
	if moment.kind == sleighStory || moment.kind == giftStory {
		flight := p
		if moment.kind == giftStory {
			flight = p / giftFlight
		}
		x, y := sleighPosition(cols, flight)
		if x < cols {
			paint(x, y+2, "{#}", gold)
			paint(x+5, y, "/^*", red)
			paint(x+5, y+1, "(o)", snow)
			paint(x+4, y+2, "/[#]_", red)
			paint(x+1, y+3, `\________/--`, gold)
			paint(x+2, y+4, "o--------o", gold)
			paint(x+17, y, `\|/ \|/`, deer)
			paint(x+15, y+1, "     (o)>", deer)
			paint(x+13, y+2, "~(____/)", deer)
			legs := `/\  /\`
			if int(p*20)%2 == 0 {
				legs = ` /\  /`
			}
			paint(x+14, y+3, legs, deer)
			spark(x+24, y+1, 'o', rudolphRed)
			paint(x-6, y+2, ".  *", 0x9CD9EA)
		}
	}

	baseY := groundY - 4
	switch moment.kind {
	case giftStory:
		const dropStart, landed = .29, .66
		if p < dropStart {
			return
		}
		landingX := cols/2 - max(7, rows/5)
		fall := math.Min(1, (p-dropStart)/(landed-dropStart))
		releaseX, releaseY := sleighPosition(cols, dropStart/giftFlight)
		releaseX += 3
		releaseY += 5
		x := releaseX + int(float64(landingX-releaseX)*fall)
		y := releaseY + int(float64(baseY+1-releaseY)*fall*fall)
		paint(x+1, y, `\/`, gold)
		paint(x, y+1, "[++]", red)
		paint(x, y+2, "[__]", red)
		paint(x+1, y+1, "||", gold)
		if fall == 1 && int(p*18)%3 != 0 {
			spark(x+5, y, '*', snow)
		}
	case matchStory:
		x := cols/2 + max(5, rows/7)
		const coat = 0x79C5D1
		const scarf = 0xF38C92
		const face = 0xF1C79B
		if p < .18 {
			x += int((1 - p/.18) * 8)
			paint(x, baseY, `/^\`, scarf)
			paint(x, baseY+1, "(o)", face)
			paint(x, baseY+2, `/|\`, coat)
			paint(x, baseY+3, `/ \`, coat)
			return
		}
		paint(x, baseY, `/^\`, scarf)
		paint(x, baseY+1, "(o)", face)
		paint(x+2, baseY+1, "~", scarf)
		paint(x-2, baseY+2, "--/|_", coat)
		paint(x, baseY+3, "(_)_", coat)
		flame := [3]rune{'*', '+', '*'}[int(p*23)%3]
		spark(x-3, baseY+1, flame, gold)
		paint(x-3, baseY+2, "|", 0xDC9C65)
		if int(p*23)%3 == 0 {
			paint(x-4, baseY, ".", 0xB78358)
		}
	case snowmanStory:
		x, y := cols/2-max(11, rows/4), baseY-3
		if p < .20 {
			x -= int((1 - p/.20) * 8)
		}
		paint(x+1, y, "___", 0x95ACD2)
		paint(x, y+1, "[___]", 0x95ACD2)
		paint(x, y+2, "(o o)", snow)
		spark(x+3, y+3, '>', 0xFFA059)
		paint(x-1, y+4, `/ (_) \`, deer)
		paint(x, y+5, "( : )", snow)
		paint(x-1, y+6, "(__:__)", snow)
		if p >= .28 {
			paint(x, y+3, "==", red)
			paint(x+4, y+3, "=", red)
			length := min(4, 1+int((p-.28)*16))
			paint(x+5, y+4, "~~~~"[:length], red)
		}
	case starStory:
		x := cols/8 + int(p*float64(cols*3/4))
		y := 2 + int(p*float64(min(11, rows/3)))
		for i := 14; i >= 1; i-- {
			tail := max(0., p-float64(i)*.018)
			tx := cols/8 + int(tail*float64(cols*3/4))
			ty := 2 + int(tail*float64(min(11, rows/3)))
			glyph := "."
			if i < 6 {
				glyph = "-"
			}
			paint(tx, ty, glyph, shade(0xBFD1FF, 1-float64(i)*.055))
		}
		paint(x-1, y, "- -", 0xFFDA8F)
		paint(x, y-1, "|", 0xFFDA8F)
		paint(x, y+1, "|", 0xFFDA8F)
		spark(x, y, '*', 0xFFF0A8)
	}
}
