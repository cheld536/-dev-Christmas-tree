package main

import (
	"christmas/internal/buildinfo"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed app.exe
var payload []byte
var version = buildinfo.Version()
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
	c := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; $w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut($env:DCT_LINK); $s.TargetPath=$env:DCT_TARGET; $s.Arguments=$env:DCT_ARGS; $s.IconLocation=$env:DCT_TARGET+',0'; $s.WorkingDirectory=[IO.Path]::GetDirectoryName($env:DCT_TARGET); $s.Save()`)
	c.Env = append(os.Environ(), "DCT_LINK="+path, "DCT_TARGET="+target, "DCT_ARGS="+args)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return c.Run()
}

type prior struct {
	Path, Active, Timeout                          string
	HasPath, HasActive, HasTimeout, ChangedTimeout bool
	SystemTimeout                                  uint32
	SystemActive, HasSystemActive                  bool
}

func install(dir, menu string, registerSaver, desktopLink bool, progress func(string)) (failure error) {
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	export, e := os.MkdirTemp("", "dct-uninstall-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(export)
	app := filepath.Join(dir, "DeveloperChristmasTree.exe")
	scr := filepath.Join(dir, "DeveloperChristmasTree.scr")
	un := filepath.Join(dir, "Uninstall.exe")
	shortcut := filepath.Join(menu, "Developer Christmas Tree.lnk")
	saverBackup := filepath.Join(dir, "previous-saver.json")
	journal := installJournal{}
	defer func() {
		if failure != nil {
			if rollbackErr := journal.rollback(); rollbackErr != nil {
				failure = errors.Join(failure, fmt.Errorf("기존 설치 상태를 복원하지 못했습니다. %w", rollbackErr))
			}
		}
	}()
	progress("앱과 화면 보호기 파일을 복사하는 중…")
	if e := journal.write(app, payload); e != nil {
		return fmt.Errorf("실행 중인 Christmas Tree 창을 닫고 다시 시도하세요.\n%w", e)
	}
	if e := journal.write(scr, payload); e != nil {
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
		if e = journal.write(un, b); e != nil {
			return e
		}
	}
	progress("시작 메뉴와 설치된 앱에 등록하는 중…")
	if e := journal.changeFile(shortcut, func() error { return installShortcut(shortcut, app, true) }); e != nil {
		return fmt.Errorf("시작 메뉴 바로가기를 만들 수 없습니다.\n%w", e)
	}
	var desktopPath string
	if desktopLink {
		folder, e := desktopDirectory()
		if e != nil {
			return fmt.Errorf("Windows 바탕 화면 폴더를 확인할 수 없습니다.\n%w", e)
		}
		desktopPath = filepath.Join(folder, "Developer Christmas Tree.lnk")
		if e := journal.desktopShortcut(desktopPath, app); e != nil {
			return e
		}
	}
	if e := journal.saveUninstallKey(filepath.Join(export, "previous.reg")); e != nil {
		return e
	}
	for n, v := range map[string]string{"DisplayName": "Developer Christmas Tree", "DisplayVersion": version, "Publisher": "Developer Christmas Tree", "InstallLocation": dir, "DisplayIcon": app, "UninstallString": "\"" + un + "\" /uninstall"} {
		if e := regSet(uninstallKey, n, v); e != nil {
			return e
		}
	}
	for _, name := range []string{"NoModify", "NoRepair"} {
		if e := reg("add", uninstallKey, "/v", name, "/t", "REG_DWORD", "/d", "1", "/f"); e != nil {
			return e
		}
	}
	if desktopPath != "" {
		if e := regSet(uninstallKey, "DesktopShortcut", desktopPath); e != nil {
			return e
		}
	}
	if registerSaver {
		progress("Windows 화면 보호기로 등록하는 중…")
		previous := readSaverSettings()
		if !strings.EqualFold(strings.Trim(previous.Path, "\""), scr) {
			b, _ := json.Marshal(previous)
			if e := journal.write(saverBackup, b); e != nil {
				return e
			}
		}
		journal = append(journal, func() error { return restoreSaver(previous, true) })
		if e := regSet(desktop, "SCRNSAVE.EXE", scr); e != nil {
			return e
		}
		if e := regSet(desktop, "ScreenSaveActive", "1"); e != nil {
			return e
		}
		if previous.ChangedTimeout {
			if e := regSet(desktop, "ScreenSaveTimeOut", "600"); e != nil {
				return e
			}
			u.NewProc("SystemParametersInfoW").Call(15, 600, 0, 3)
		}
		activate(true)
		v, ok := regRead(desktop, "SCRNSAVE.EXE")
		if !ok || !strings.EqualFold(strings.Trim(v, "\""), scr) {
			return fmt.Errorf("화면 보호기 등록을 확인할 수 없습니다. Windows 정책을 확인하세요.")
		}
	}
	return nil
}
func uninstall(dir, menu string) {
	if msg("Christmas Tree를 삭제할까요?\n\n설치된 앱과 화면 보호기를 제거합니다.", 0x24) != 6 {
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
		if err := restoreSaver(p, false); err != nil {
			msg("기존 화면 보호기 설정을 복원하지 못했어요.\n"+err.Error(), 0x10)
			return
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
	if path, ok := regRead(uninstallKey, "DesktopShortcut"); ok {
		if folder, err := desktopDirectory(); err == nil && strings.EqualFold(path, filepath.Join(folder, "Developer Christmas Tree.lnk")) {
			if target, err := shortcutTarget(path); err == nil && strings.EqualFold(target, filepath.Join(dir, "DeveloperChristmasTree.exe")) {
				os.Remove(path)
			}
		}
	}
	reg("delete", uninstallKey, "/f")
	if msg("저장한 설정도 지울까요?\n\n직접 내보낸 프리셋 파일은 그대로 남습니다.", 0x24) == 6 {
		if e := os.Remove(filepath.Join(dir, "settings.json")); e != nil && !os.IsNotExist(e) {
			msg("설정 파일을 지우지 못했어요.\n"+e.Error(), 0x30)
		}
	}
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
	lock, _, err := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(str("Local\\DeveloperChristmasTree-Setup"))))
	if lock == 0 {
		msg("설치 동시 실행을 확인할 수 없습니다.", 0x10)
		return
	}
	defer kernel.NewProc("CloseHandle").Call(lock)
	if err == syscall.Errno(183) {
		msg("이미 설치 또는 제거가 진행 중입니다.", 0x30)
		return
	}
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
	setupWindow(dir, menu)
}
