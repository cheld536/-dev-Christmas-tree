package main

import (
	"testing"
	"time"
)

func TestScheduledStory(t *testing.T) {
	for _, tt := range []struct {
		second int
		kind   storyKind
	}{
		{0, noStory}, {8, sleighStory}, {16, noStory},
		{28, giftStory}, {40, noStory}, {52, matchStory}, {64, noStory},
		{72, snowmanStory}, {84, noStory}, {90, starStory}, {98, noStory},
		{106, noStory}, {114, sleighStory},
	} {
		if got := scheduledStory(time.Duration(tt.second) * time.Second); got.kind != tt.kind || got.progress != 0 {
			t.Errorf("%d초 장면 = %+v, 예상 장면 = %v", tt.second, got, tt.kind)
		}
	}
	if got := storyDuring(giftStory, 6*time.Second); got.progress != .5 {
		t.Errorf("선물 장면 6초의 진행률 = %v", got.progress)
	}
}

func TestStoryFrames(t *testing.T) {
	find := func(kind storyKind, progress float64, color uint32, ch rune) (Cell, bool) {
		f := composeFrame(120, 45, 1, 0, AudioFrame{}, defaults(), "", false, storyMoment{kind, progress})
		for _, c := range f.Cells {
			if c.Color == color && c.Ch == ch && c.Depth == 2e5 {
				return c, true
			}
		}
		return Cell{}, false
	}
	left, leftOK := find(sleighStory, .25, rudolphRed, 'o')
	right, rightOK := find(sleighStory, .65, rudolphRed, 'o')
	if !leftOK || !rightOK || left.X >= right.X || right.Y > 6 {
		t.Fatalf("루돌프가 상단에서 오른쪽으로 이동하지 않음: %+v, %+v", left, right)
	}
	air, airOK := find(giftStory, .4, 0xFF5E6A, '[')
	ground, groundOK := find(giftStory, .9, 0xFF5E6A, '[')
	if !airOK || !groundOK || air.Y >= ground.Y || ground.Y < 35 || ground.Y > 41 {
		t.Fatalf("트리 아래에 선물이 놓이지 않음: %+v, %+v", air, ground)
	}
	for _, tt := range []struct {
		kind  storyKind
		color uint32
		ch    rune
	}{{matchStory, 0xFFCA70, '*'}, {snowmanStory, 0xFF5E6A, '~'}, {starStory, 0xFFF0A8, '*'}} {
		if _, ok := find(tt.kind, .50, tt.color, tt.ch); !ok {
			t.Errorf("%s 장면의 대표 문자가 없음", tt.kind.title())
		}
	}
	drawStory(24, 14, 11, storyMoment{snowmanStory, .5}, func(int, int, rune, uint32, float64, bool) {
		t.Error("작은 화면에 이야기 문자가 그려짐")
	})
}
