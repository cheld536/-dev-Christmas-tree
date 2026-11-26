package main

import (
	"image"
	imagecolor "image/color"
	"image/png"
	"os"
	"runtime"
	"testing"
)

func TestLockScreenImage(t *testing.T) {
	for _, item := range []struct {
		choice        string
		width, height int
	}{{"1080p", 1920, 1080}, {"4K", 3840, 2160}} {
		t.Run(item.choice, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			settings := defaults()
			settings.FullscreenResolution = item.choice
			settings.Background = "#112233"
			picture, err := lockScreenImage(settings)
			if err != nil {
				t.Fatal(err)
			}
			if picture.Bounds() != image.Rect(0, 0, item.width, item.height) {
				t.Fatalf("이미지 크기: %v", picture.Bounds())
			}
			background := imagecolor.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 255}
			if picture.RGBAAt(0, 0) != background {
				t.Fatal("선택한 배경색이 잠금 이미지에 반영되지 않았습니다.")
			}
			changed := 0
			for y := item.height / 3; y < item.height*3/4; y++ {
				for x := item.width / 3; x < item.width*2/3; x++ {
					if picture.RGBAAt(x, y) != background {
						changed++
					}
				}
			}
			if changed < 1000 {
				t.Fatal("잠금 이미지 중앙에 트리가 그려지지 않았습니다.")
			}
			path, err := saveLockScreenImage(t.TempDir(), picture)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			decoded, err := png.DecodeConfig(file)
			if err != nil || decoded.Width != item.width || decoded.Height != item.height {
				t.Fatalf("저장한 PNG에 문제가 있습니다: %+v, %v", decoded, err)
			}
		})
	}
}
