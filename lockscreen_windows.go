package main

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const wmLockScreenResult = 0x800C

type lockScreenResult struct {
	path    string
	applied bool
	err     error
}

var lockScreenResults = make(chan lockScreenResult, 1)

func lockScreenImage(settings Settings) (*image.RGBA, error) {
	width, height := fullscreenSize(settings.FullscreenResolution, 1920, 1080)
	dc := call(g, "CreateCompatibleDC", 0)
	if dc == 0 {
		return nil, errors.New("잠금 화면 이미지의 그리기 영역을 만들 수 없습니다.")
	}
	defer call(g, "DeleteDC", dc)
	info := bitmapHeader{
		Size: uint32(unsafe.Sizeof(bitmapHeader{})), W: int32(width), H: -int32(height),
		Planes: 1, Bits: 32,
	}
	var pixels unsafe.Pointer
	bitmap := call(g, "CreateDIBSection", dc, ptr(&info), 0, ptr(&pixels), 0, 0)
	if bitmap == 0 || pixels == nil {
		return nil, errors.New("잠금 화면 이미지의 픽셀을 만들 수 없습니다.")
	}
	defer call(g, "DeleteObject", bitmap)
	previous := call(g, "SelectObject", dc, bitmap)
	defer call(g, "SelectObject", dc, previous)

	cellSize := height / 63
	cw, ch := cellSize*62/100, cellSize*118/100
	cols, rows := min(220, width/cw), height*4/5/ch
	settings.Messages, settings.Countdown, settings.Monitor = false, false, false
	frame := composeFrame(cols, rows, 5, .55, AudioFrame{}, settings, "", false, storyMoment{})
	fill(dc, rect{0, 0, int32(width), int32(height)}, frame.BG)
	font := mono(cellSize)
	if font == 0 {
		return nil, errors.New("잠금 화면 이미지의 글꼴을 만들 수 없습니다.")
	}
	defer call(g, "DeleteObject", font)
	var glow float64
	if settings.Bloom && !settings.LowPower {
		glow = .10
	}
	drawTreeCells(dc, font, frame, cw, ch, (width-cols*cw)/2, height/6, glow)
	call(g, "GdiFlush")

	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	bgra := unsafe.Slice((*byte)(pixels), len(picture.Pix))
	for i := 0; i < len(bgra); i += 4 {
		picture.Pix[i] = bgra[i+2]
		picture.Pix[i+1] = bgra[i+1]
		picture.Pix[i+2] = bgra[i]
		picture.Pix[i+3] = 255
	}
	return picture, nil
}

func saveLockScreenImage(directory string, picture image.Image) (string, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "Christmas-Tree-*.png")
	if err != nil {
		return "", err
	}
	encodeErr := png.Encode(file, picture)
	if err := errors.Join(encodeErr, file.Close()); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

const lockScreenPS = `
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null=[Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime]
$null=[Windows.System.UserProfile.UserProfilePersonalizationSettings,Windows.System.UserProfile,ContentType=WindowsRuntime]
if(-not [Windows.System.UserProfile.UserProfilePersonalizationSettings]::IsSupported()){exit 1}
$asTask=[System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {$_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name.StartsWith('IAsyncOperation')} | Select-Object -First 1
function Await($operation,$type) {
    $task=$asTask.MakeGenericMethod($type).Invoke($null,@($operation))
    if(-not $task.Wait(5000)){throw 'Lock screen operation timed out'}
    $task.Result
}
$file=Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($env:DCT_LOCK_SCREEN_IMAGE)) ([Windows.Storage.StorageFile])
$settings=[Windows.System.UserProfile.UserProfilePersonalizationSettings]::Current
if(-not (Await ($settings.TrySetLockScreenImageAsync($file)) ([bool]))){exit 1}
`

func setWindowsLockScreen(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPS(lockScreenPS))
	cmd.Env = append(os.Environ(), "DCT_LOCK_SCREEN_IMAGE="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Run() == nil
}

func applyLockScreen(hwnd uintptr, settings Settings) {
	control := controls[idLockScreen]
	if call(u, "IsWindowEnabled", control) == 0 {
		return
	}
	call(u, "EnableWindow", control, 0)
	set(control, "잠금 이미지 준비 중…")
	picture, err := lockScreenImage(settings)
	if err != nil {
		finishLockScreen(hwnd, lockScreenResult{err: err})
		return
	}
	directory := filepath.Join(configDir(), "lock-screen")
	postResult := u.NewProc("PostMessageW")
	go func() {
		var result lockScreenResult
		result.path, result.err = saveLockScreenImage(directory, picture)
		if result.err == nil {
			result.applied = setWindowsLockScreen(result.path)
		}
		lockScreenResults <- result
		postResult.Call(hwnd, wmLockScreenResult, 0, 0)
	}()
}

func finishLockScreen(hwnd uintptr, result lockScreenResult) {
	control := controls[idLockScreen]
	call(u, "EnableWindow", control, 1)
	if result.applied {
		set(control, "잠금 화면에 적용됨")
		return
	}
	set(control, "잠금 화면 이미지 적용")
	if result.err != nil {
		box(hwnd, "잠금 화면 이미지를 저장할 수 없습니다.\n"+result.err.Error(), 0x10)
	} else if box(hwnd, "이미지를 저장했습니다. Windows 설정의 잠금 화면에서 ‘사진’을 선택하고 아래 파일을 지정해 주세요.\n\n"+
		result.path+"\n\n확인을 누르면 잠금 화면 설정이 열립니다.", 1|0x40) == 1 {
		call(shell, "ShellExecuteW", hwnd, w("open"), w("ms-settings:lockscreen"), 0, 0, 1)
	}
}
