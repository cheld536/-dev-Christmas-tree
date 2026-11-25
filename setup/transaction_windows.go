package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func desktopDirectory() (string, error) {
	folder := syscall.GUID{Data1: 0xB4BFCC3A, Data2: 0xDB2C, Data3: 0x424C, Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}}
	var path *uint16
	hr := invoke(shell, "SHGetKnownFolderPath", uintptr(unsafe.Pointer(&folder)), 0, 0, uintptr(unsafe.Pointer(&path)))
	if path != nil {
		defer invoke(syscall.NewLazyDLL("ole32.dll"), "CoTaskMemFree", uintptr(unsafe.Pointer(path)))
	}
	if int32(hr) < 0 || path == nil {
		return "", fmt.Errorf("Windows 결과 코드 0x%08X", uint32(hr))
	}
	var length int
	for *(*uint16)(unsafe.Add(unsafe.Pointer(path), 2*length)) != 0 {
		length++
	}
	return syscall.UTF16ToString(unsafe.Slice(path, length)), nil
}

func shortcutTarget(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `
        $ErrorActionPreference = 'Stop'
        $shell = New-Object -ComObject WScript.Shell
        $target = [string]$shell.CreateShortcut($env:DCT_LINK).TargetPath
        [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($target))
    `)
	cmd.Env = append(os.Environ(), "DCT_LINK="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	value, err := cmd.Output()
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(string(value), "\uFEFF")))
	return string(decoded), err
}

func sameTarget(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
		return true
	}
	left, leftErr := os.Stat(a)
	right, rightErr := os.Stat(b)
	return leftErr == nil && rightErr == nil && os.SameFile(left, right)
}

func (j *installJournal) desktopShortcut(path, app string) error {
	target, err := shortcutTarget(path)
	if errors.Is(err, fs.ErrNotExist) {
		return j.changeFile(path, func() error { return installShortcut(path, app, false) })
	}
	if err != nil {
		return fmt.Errorf("기존 바탕 화면 바로가기 대상을 확인할 수 없습니다. %w", err)
	}
	if !sameTarget(target, app) {
		return errors.New("바탕 화면에 같은 이름의 다른 바로가기가 있습니다. 바로가기 옵션을 끄거나 기존 바로가기의 이름을 변경하세요.")
	}
	return nil
}

type installJournal []func() error

func (j installJournal) rollback() error {
	var err error
	for i := len(j) - 1; i >= 0; i-- {
		err = errors.Join(err, j[i]())
	}
	return err
}

func (j *installJournal) changeFile(path string, action func() error) error {
	before, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	existed := err == nil
	if err := action(); err != nil {
		return err
	}
	*j = append(*j, func() error {
		if existed {
			return copyFile(path, before)
		}
		err := os.Remove(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	})
	return nil
}

func (j *installJournal) write(path string, data []byte) error {
	return j.changeFile(path, func() error { return copyFile(path, data) })
}

func installShortcut(path, app string, overwrite bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	stage, err := os.CreateTemp(dir, ".shortcut-*.lnk")
	if err != nil {
		return err
	}
	linkPath := stage.Name()
	defer os.Remove(linkPath)
	if err := stage.Close(); err != nil {
		return err
	}
	if err := os.Remove(linkPath); err != nil {
		return err
	}
	if err := link(linkPath, app, ""); err != nil {
		return err
	}
	if overwrite {
		return os.Rename(linkPath, path)
	}
	result, _, err := kernel.NewProc("MoveFileW").Call(uintptr(unsafe.Pointer(str(linkPath))), uintptr(unsafe.Pointer(str(path))))
	if result == 0 {
		return err
	}
	return nil
}

func uninstallKeyExists() (bool, error) {
	var key syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, str(uninstallKey[5:]), 0, syscall.KEY_QUERY_VALUE, &key)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	syscall.RegCloseKey(key)
	return true, nil
}

func (j *installJournal) saveUninstallKey(path string) error {
	existed, err := uninstallKeyExists()
	if err != nil {
		return err
	}
	if existed {
		if err := reg("export", uninstallKey, path, "/y"); err != nil {
			return err
		}
	}
	*j = append(*j, func() error {
		current, err := uninstallKeyExists()
		if err != nil {
			return err
		}
		if current {
			if err := reg("delete", uninstallKey, "/f"); err != nil {
				return err
			}
		}
		if existed {
			return reg("import", path)
		}
		return nil
	})
	return nil
}

func readSaverSettings() prior {
	var p prior
	p.Path, p.HasPath = regRead(desktop, "SCRNSAVE.EXE")
	p.Active, p.HasActive = regRead(desktop, "ScreenSaveActive")
	p.Timeout, p.HasTimeout = regRead(desktop, "ScreenSaveTimeOut")
	timeout, err := strconv.Atoi(p.Timeout)
	p.ChangedTimeout = !p.HasTimeout || err != nil || timeout <= 0
	invoke(u, "SystemParametersInfoW", 14, 0, uintptr(unsafe.Pointer(&p.SystemTimeout)), 0)
	var active uint32
	p.HasSystemActive = invoke(u, "SystemParametersInfoW", 16, 0, uintptr(unsafe.Pointer(&active)), 0) != 0
	p.SystemActive = active != 0
	return p
}

func restoreSaver(p prior, force bool) error {
	restore := func(name, value string, exists bool) error {
		if exists {
			return regSet(desktop, name, value)
		}
		if _, ok := regRead(desktop, name); ok {
			return reg("delete", desktop, "/v", name, "/f")
		}
		return nil
	}
	currentTimeout, _ := regRead(desktop, "ScreenSaveTimeOut")
	var errs error
	if p.ChangedTimeout && (force || currentTimeout == "600") {
		n, err := strconv.Atoi(p.Timeout)
		if err == nil && n >= 0 {
			invoke(u, "SystemParametersInfoW", 15, uintptr(n), 0, 3)
		} else if p.SystemTimeout > 0 {
			invoke(u, "SystemParametersInfoW", 15, uintptr(p.SystemTimeout), 0, 3)
		}
		errs = restore("ScreenSaveTimeOut", p.Timeout, p.HasTimeout)
	}
	active := p.Active == "1"
	if p.HasSystemActive {
		active = p.SystemActive
	}
	activate(active)
	return errors.Join(errs, restore("ScreenSaveActive", p.Active, p.HasActive), restore("SCRNSAVE.EXE", p.Path, p.HasPath))
}
