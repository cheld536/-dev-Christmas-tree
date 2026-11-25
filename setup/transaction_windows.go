package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"
)

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

func installShortcut(path, app string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(dir, ".shortcut-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	linkPath := filepath.Join(stage, "Developer Christmas Tree.lnk")
	if err := link(linkPath, app, ""); err != nil {
		return err
	}
	return os.Rename(linkPath, path)
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
