package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed app.exe
var payload []byte
var u = syscall.NewLazyDLL("user32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")

const desktop = `HKCU\Control Panel\Desktop`
const uninstallKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\DeveloperChristmasTree`

func msg(text string, flags uintptr) uintptr {
	a, _ := syscall.UTF16PtrFromString(text)
	b, _ := syscall.UTF16PtrFromString("Developer Christmas Tree Setup")
	r, _, _ := u.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), flags)
	return r
}
func hidden(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	b, e := c.CombinedOutput()
	return string(b), e
}
func reg(args ...string) error { _, e := hidden("reg.exe", args...); return e }
func regRead(key, name string) (string, bool) {
	sub, _ := syscall.UTF16PtrFromString(strings.TrimPrefix(key, `HKCU\`))
	n, _ := syscall.UTF16PtrFromString(name)
	var h syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, sub, 0, syscall.KEY_QUERY_VALUE, &h) != nil {
		return "", false
	}
	defer syscall.RegCloseKey(h)
	var typ, size uint32
	if syscall.RegQueryValueEx(h, n, nil, &typ, nil, &size) != nil || size == 0 || size > 65536 || (typ != 1 && typ != 2) {
		return "", false
	}
	b := make([]uint16, (size+1)/2)
	if syscall.RegQueryValueEx(h, n, nil, &typ, (*byte)(unsafe.Pointer(&b[0])), &size) != nil {
		return "", false
	}
	return syscall.UTF16ToString(b), true
}
func regSet(key, name, val string) error {
	return reg("add", key, "/v", name, "/t", "REG_SZ", "/d", val, "/f")
}
func copyFile(dst string, data []byte) error {
	tmp := dst + ".new"
	if e := os.WriteFile(tmp, data, 0755); e != nil {
		return e
	}
	if e := os.Rename(tmp, dst); e != nil {
		os.Remove(tmp)
		return e
	}
	return nil
}
func open(path, args string) {
	a, _ := syscall.UTF16PtrFromString("open")
	b, _ := syscall.UTF16PtrFromString(path)
	c, _ := syscall.UTF16PtrFromString(args)
	shell.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(c)), 0, 1)
}
func activate(value bool) {
	v := uintptr(0)
	if value {
		v = 1
	}
	u.NewProc("SystemParametersInfoW").Call(17, v, 0, 3)
}
func link(path, target, args string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	c := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; $w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut($env:DCT_LINK); $s.TargetPath=$env:DCT_TARGET; $s.Arguments=$env:DCT_ARGS; $s.WorkingDirectory=[IO.Path]::GetDirectoryName($env:DCT_TARGET); $s.Save()`)
	c.Env = append(os.Environ(), "DCT_LINK="+path, "DCT_TARGET="+target, "DCT_ARGS="+args)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return c.Run()
}

type prior struct {
	Path, Active, Timeout                          string
	HasPath, HasActive, HasTimeout, ChangedTimeout bool
}

func install(dir, menu string) error {
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	app := filepath.Join(dir, "DeveloperChristmasTree.exe")
	scr := filepath.Join(dir, "DeveloperChristmasTree.scr")
	un := filepath.Join(dir, "Uninstall.exe")
	if e := copyFile(app, payload); e != nil {
		return fmt.Errorf("Close any running Christmas Tree windows, then try again.\n%w", e)
	}
	if e := copyFile(scr, payload); e != nil {
		return e
	}
	me, e := os.Executable()
	if e != nil {
		return e
	}
	if !strings.EqualFold(me, un) {
		b, e := os.ReadFile(me)
		if e != nil {
			return e
		}
		if e = copyFile(un, b); e != nil {
			return e
		}
	}
	for n, v := range map[string]string{"DisplayName": "Developer Christmas Tree", "DisplayVersion": "2.1.0", "Publisher": "Developer Christmas Tree", "InstallLocation": dir, "DisplayIcon": app, "UninstallString": "\"" + un + "\" /uninstall"} {
		if e := regSet(uninstallKey, n, v); e != nil {
			return e
		}
	}
	reg("add", uninstallKey, "/v", "NoModify", "/t", "REG_DWORD", "/d", "1", "/f")
	reg("add", uninstallKey, "/v", "NoRepair", "/t", "REG_DWORD", "/d", "1", "/f")
	if e := link(filepath.Join(menu, "Developer Christmas Tree.lnk"), app, ""); e != nil {
		msg("Installed, but Start menu shortcut could not be created. You can still open the settings app from the installation directory.", 0x30)
	}
	if msg("Windows 화면 보호기로 사용할까요?\n\n기존 대기 시간과 로그인 설정은 유지됩니다. 대기 시간이 없으면 10분으로 설정됩니다.", 0x24) == 6 {
		old, has := regRead(desktop, "SCRNSAVE.EXE")
		active, hasActive := regRead(desktop, "ScreenSaveActive")
		timeout, hasTimeout := regRead(desktop, "ScreenSaveTimeOut")
		seconds, errTimeout := strconv.Atoi(timeout)
		needsTimeout := !hasTimeout || errTimeout != nil || seconds <= 0
		if !strings.EqualFold(strings.Trim(old, "\""), scr) {
			b, _ := json.Marshal(prior{Path: old, Active: active, Timeout: timeout, HasPath: has, HasActive: hasActive, HasTimeout: hasTimeout, ChangedTimeout: needsTimeout})
			if e := os.WriteFile(filepath.Join(dir, "previous-saver.json"), b, 0600); e != nil {
				return e
			}
		}
		if e := regSet(desktop, "SCRNSAVE.EXE", scr); e != nil {
			return e
		}
		if e := regSet(desktop, "ScreenSaveActive", "1"); e != nil {
			return e
		}
		if needsTimeout {
			if e := regSet(desktop, "ScreenSaveTimeOut", "600"); e != nil {
				return e
			}
			u.NewProc("SystemParametersInfoW").Call(15, 600, 0, 3)
		}
		activate(true)
		v, ok := regRead(desktop, "SCRNSAVE.EXE")
		if !ok || !strings.EqualFold(strings.Trim(v, "\""), scr) {
			return fmt.Errorf("Screen saver registry verification failed. Your device may have a policy restriction.")
		}
	}
	return nil
}
func uninstall(dir, menu string) {
	if msg("Christmas Tree를 삭제할까요?\n\n저장한 설정과 프리셋은 남겨 둡니다.", 0x24) != 6 {
		return
	}
	scr := filepath.Join(dir, "DeveloperChristmasTree.scr")
	current, _ := regRead(desktop, "SCRNSAVE.EXE")
	if strings.EqualFold(strings.Trim(current, "\""), scr) {
		var p prior
		b, e := os.ReadFile(filepath.Join(dir, "previous-saver.json"))
		if e == nil {
			json.Unmarshal(b, &p)
		}
		if p.HasPath {
			regSet(desktop, "SCRNSAVE.EXE", p.Path)
		} else {
			reg("delete", desktop, "/v", "SCRNSAVE.EXE", "/f")
		}
		if p.ChangedTimeout {
			if timeout, _ := regRead(desktop, "ScreenSaveTimeOut"); timeout == "600" {
				if p.HasTimeout {
					regSet(desktop, "ScreenSaveTimeOut", p.Timeout)
					if n, e := strconv.Atoi(p.Timeout); e == nil && n >= 0 {
						u.NewProc("SystemParametersInfoW").Call(15, uintptr(n), 0, 3)
					}
				} else {
					reg("delete", desktop, "/v", "ScreenSaveTimeOut", "/f")
				}
			}
		}
		if p.HasActive {
			regSet(desktop, "ScreenSaveActive", p.Active)
			activate(p.Active == "1")
		} else {
			activate(false)
			reg("delete", desktop, "/v", "ScreenSaveActive", "/f")
		}
	}
	for _, n := range []string{"DeveloperChristmasTree.exe", "DeveloperChristmasTree.scr", "previous-saver.json"} {
		e := os.Remove(filepath.Join(dir, n))
		if e != nil && !os.IsNotExist(e) {
			msg("Close any running Christmas Tree app and try uninstalling again.\n"+e.Error(), 0x10)
			return
		}
	}
	os.Remove(filepath.Join(menu, "Developer Christmas Tree.lnk"))
	os.Remove(menu)
	reg("delete", uninstallKey, "/f")
	msg("Christmas Tree를 삭제했어요.", 0x40)
	me, _ := os.Executable()
	if strings.EqualFold(me, filepath.Join(dir, "Uninstall.exe")) { // remove the running uninstaller after exit, using the literal path in an environment variable.
		c := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `for($i=0;$i -lt 30;$i++){Start-Sleep -Seconds 1; try {Remove-Item -LiteralPath $env:DCT_REMOVE -Force -ErrorAction Stop; break} catch {}}`)
		c.Env = append(os.Environ(), "DCT_REMOVE="+me)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		c.Start()
	}
}
func main() {
	runtime.LockOSThread()
	local := os.Getenv("LOCALAPPDATA")
	roam := os.Getenv("APPDATA")
	if local == "" || roam == "" {
		msg("Windows user profile folders are unavailable.", 0x10)
		return
	}
	dir := filepath.Join(local, "DeveloperChristmasTree")
	menu := filepath.Join(roam, "Microsoft", "Windows", "Start Menu", "Programs", "Developer Christmas Tree")
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "/uninstall") {
		uninstall(dir, menu)
		return
	}
	if msg("Christmas Tree 2.1을 설치할까요?\n\n기존 설정은 그대로 유지됩니다.\n실행 중인 Christmas Tree 창을 먼저 닫아 주세요.", 0x24) != 6 {
		return
	}
	if e := install(dir, menu); e != nil {
		msg("Installation could not be completed:\n"+e.Error(), 0x10)
		return
	}
	if msg("설치가 끝났어요.\n\n지금 Christmas Tree를 열까요?", 0x24) == 6 {
		open(filepath.Join(dir, "DeveloperChristmasTree.exe"), "")
	}
}
