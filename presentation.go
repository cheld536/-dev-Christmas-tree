package main

func fullscreenSize(choice string, width, height int) (int, int) {
	switch choice {
	case "1080p":
		return 1920, 1080
	case "4K":
		return 3840, 2160
	default:
		return width, height
	}
}

func fitFrame(srcW, srcH, dstW, dstH int) (width, height int) {
	if dstW*srcH <= dstH*srcW {
		return dstW, dstW * srcH / srcW
	}
	return dstH * srcW / srcH, dstH
}

func footerHeight(width, sceneHeight, lineHeight int, messages, fullscreen, couple bool) int {
	if width < 200 || sceneHeight < 120 {
		return 0
	}
	rows := 0
	if couple {
		rows = 1
	} else if messages {
		rows = 1
		if sceneHeight >= 200 && width >= 320 {
			rows++
		}
	}
	if fullscreen {
		rows++
	}
	return rows * lineHeight
}
