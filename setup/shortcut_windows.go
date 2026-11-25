package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var ole = syscall.NewLazyDLL("ole32.dll")

var (
	shellLinkClass = syscall.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	shellLinkW     = syscall.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	persistFileID  = syscall.GUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

const (
	queryInterface = 0
	releaseObject  = 2
	linkGetPath    = 3
	linkWorkingDir = 9
	linkIcon       = 17
	linkSetPath    = 20
	persistLoad    = 5
	persistSave    = 6
)

type comObject struct{ methods *[21]uintptr }

func comError(operation string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(hr))
	}
	return nil
}

//go:uintptrescapes
func (obj *comObject) call(slot int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(obj.methods[slot], append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return r
}

func useShellLink(action func(link, persist *comObject) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr := invoke(ole, "CoInitializeEx", 0, 2)
	if uint32(hr) != 0x80010106 {
		if err := comError("CoInitializeEx", hr); err != nil {
			return err
		}
		defer invoke(ole, "CoUninitialize")
	}
	var link *comObject
	if err := comError("CoCreateInstance", invoke(ole, "CoCreateInstance",
		uintptr(unsafe.Pointer(&shellLinkClass)), 0, 1, uintptr(unsafe.Pointer(&shellLinkW)), uintptr(unsafe.Pointer(&link)))); err != nil {
		return err
	}
	defer link.call(releaseObject)
	var persist *comObject
	if err := comError("QueryInterface", link.call(queryInterface, uintptr(unsafe.Pointer(&persistFileID)), uintptr(unsafe.Pointer(&persist)))); err != nil {
		return err
	}
	defer persist.call(releaseObject)
	return action(link, persist)
}

func link(path, target string) error {
	return useShellLink(func(link, persist *comObject) error {
		if err := comError("SetPath", link.call(linkSetPath, uintptr(unsafe.Pointer(str(target))))); err != nil {
			return err
		}
		if err := comError("SetWorkingDirectory", link.call(linkWorkingDir, uintptr(unsafe.Pointer(str(filepath.Dir(target)))))); err != nil {
			return err
		}
		if err := comError("SetIconLocation", link.call(linkIcon, uintptr(unsafe.Pointer(str(target))), 0)); err != nil {
			return err
		}
		return comError("Save", persist.call(persistSave, uintptr(unsafe.Pointer(str(path))), 1))
	})
}

func shortcutTarget(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	var target string
	err := useShellLink(func(link, persist *comObject) error {
		if err := comError("Load", persist.call(persistLoad, uintptr(unsafe.Pointer(str(path))), 0)); err != nil {
			return err
		}
		var buffer [1024]uint16
		hr := link.call(linkGetPath, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 4)
		target = syscall.UTF16ToString(buffer[:])
		return comError("GetPath", hr)
	})
	return target, err
}
