package main

import (
	"fmt"
	"math"
	"syscall"
	"time"
	"unsafe"
)

var ole = syscall.NewLazyDLL("ole32.dll")
var pdh = syscall.NewLazyDLL("pdh.dll")

type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

//go:uintptrescapes
func com(p uintptr, index int, args ...uintptr) uintptr {
	v := *(*uintptr)(unsafe.Pointer(p))
	fn := *(*uintptr)(unsafe.Pointer(v + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	a := append([]uintptr{p}, args...)
	r, _, _ := syscall.SyscallN(fn, a...)
	return r
}
func failed(r uintptr) bool { return int32(r) < 0 }
func release(p uintptr) {
	if p != 0 {
		com(p, 2)
	}
}

type capture struct {
	client, reader             uintptr
	format                     uintptr
	channels, bits, align, tag int
}

var comInit bool

func newCapture() *capture {
	if !comInit {
		r := call(ole, "CoInitializeEx", 0, 2)
		if failed(r) {
			return nil
		}
		comInit = true
	}
	cls := guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidEnum := guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidClient := guid{0x1CB9AD4C, 0xDBFA, 0x4c32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidReader := guid{0xC8ADBD64, 0xE71E, 0x48a0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
	var en, dev uintptr
	if failed(call(ole, "CoCreateInstance", ptr(&cls), 0, 23, ptr(&iidEnum), ptr(&en))) || en == 0 {
		return nil
	}
	defer release(en)
	if failed(com(en, 4, 0, 1, ptr(&dev))) || dev == 0 {
		return nil
	}
	defer release(dev)
	c := &capture{}
	if failed(com(dev, 3, ptr(&iidClient), 23, 0, ptr(&c.client))) || c.client == 0 {
		return nil
	}
	if failed(com(c.client, 8, ptr(&c.format))) || c.format == 0 {
		c.close()
		return nil
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(c.format)), 18)
	c.tag = int(b[0]) | int(b[1])<<8
	c.channels = int(b[2]) | int(b[3])<<8
	c.align = int(b[12]) | int(b[13])<<8
	c.bits = int(b[14]) | int(b[15])<<8
	if c.tag == 0xfffe && (int(b[16])|int(b[17])<<8) >= 22 {
		c.tag = int(*(*uint32)(unsafe.Pointer(c.format + 24)))
	}
	if c.channels < 1 || c.align < 1 || c.bits < 8 || (c.tag != 1 && c.tag != 3) {
		c.close()
		return nil
	}
	if failed(com(c.client, 3, 0, 0x20000, 1000000, 0, c.format, 0)) || failed(com(c.client, 14, ptr(&iidReader), ptr(&c.reader))) || c.reader == 0 || failed(com(c.client, 10)) {
		c.close()
		return nil
	}
	return c
}
func (c *capture) close() {
	if c.client != 0 {
		com(c.client, 11)
	}
	release(c.reader)
	release(c.client)
	if c.format != 0 {
		call(ole, "CoTaskMemFree", c.format)
	}
}
func (c *capture) read() (float64, bool) {
	var sum float64
	count := 0
	for n := 0; n < 32; n++ {
		var packet uint32
		if failed(com(c.reader, 5, ptr(&packet))) {
			return 0, false
		}
		if packet == 0 {
			break
		}
		var data uintptr
		var frames, flags uint32
		if failed(com(c.reader, 3, ptr(&data), ptr(&frames), ptr(&flags), 0, 0)) {
			return 0, false
		}
		if data != 0 && flags&2 == 0 && frames < 100000 {
			b := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(frames)*c.align)
			step := c.bits / 8
			for i := 0; i < int(frames); i += 4 {
				for j := 0; j < c.channels; j++ {
					off := i*c.align + j*step
					if off+step > len(b) {
						continue
					}
					var x float64
					switch {
					case c.tag == 3 && c.bits == 32:
						x = float64(math.Float32frombits(uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24))
					case c.bits == 16:
						x = float64(int16(uint16(b[off])|uint16(b[off+1])<<8)) / 32768
					case c.bits == 24:
						v := int32(b[off]) | int32(b[off+1])<<8 | int32(b[off+2])<<16
						x = float64(v<<8) / 2147483648
					case c.bits == 32:
						x = float64(int32(uint32(b[off])|uint32(b[off+1])<<8|uint32(b[off+2])<<16|uint32(b[off+3])<<24)) / 2147483648
					case c.bits == 8:
						x = (float64(b[off]) - 128) / 128
					}
					if !math.IsNaN(x) && !math.IsInf(x, 0) {
						sum += x * x
						count++
					}
				}
			}
		}
		if failed(com(c.reader, 4, uintptr(frames))) {
			return 0, false
		}
	}
	if count == 0 {
		return 0, true
	}
	return math.Min(1, math.Sqrt(sum/float64(count))*3.5), true
}

type memStatus struct {
	Length, Load                                                                          uint32
	TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
}

var prevIdle, prevTotal uint64
var gpuQuery, gpuCounter uintptr
var gpuInit bool

func stats() string {
	cpu, gpu, ram := "--", "--", "--"
	var idle, kern, user uint64
	if call(k, "GetSystemTimes", ptr(&idle), ptr(&kern), ptr(&user)) != 0 {
		total := kern + user
		if prevTotal != 0 && total > prevTotal && idle >= prevIdle {
			v := 100 * (1 - float64(idle-prevIdle)/float64(total-prevTotal))
			cpu = fmt.Sprintf("%d%%", clamp(int(math.Round(v)), 0, 100))
		}
		prevIdle, prevTotal = idle, total
	}
	m := memStatus{Length: uint32(unsafe.Sizeof(memStatus{}))}
	if call(k, "GlobalMemoryStatusEx", ptr(&m)) != 0 {
		ram = fmt.Sprintf("%d%%", m.Load)
	}
	if !gpuInit {
		gpuInit = true
		if call(pdh, "PdhOpenQueryW", 0, 0, ptr(&gpuQuery)) == 0 {
			if call(pdh, "PdhAddEnglishCounterW", gpuQuery, w(`\GPU Engine(*)\Utilization Percentage`), 0, ptr(&gpuCounter)) != 0 {
				gpuCounter = 0
			}
		}
	}
	if gpuCounter != 0 && call(pdh, "PdhCollectQueryData", gpuQuery) == 0 {
		var bytes, n uint32
		call(pdh, "PdhGetFormattedCounterArrayW", gpuCounter, 0x200, ptr(&bytes), ptr(&n), 0)
		if bytes > 0 && bytes < 16*1024*1024 {
			b := make([]byte, bytes)
			if call(pdh, "PdhGetFormattedCounterArrayW", gpuCounter, 0x200, ptr(&bytes), ptr(&n), ptr(&b[0])) == 0 {
				best := -1.
				for i := 0; i < int(n) && i*24+24 <= len(b); i++ {
					status := *(*uint32)(unsafe.Pointer(&b[i*24+8]))
					v := *(*float64)(unsafe.Pointer(&b[i*24+16]))
					if status <= 1 && !math.IsNaN(v) && v > best {
						best = v
					}
				}
				if best >= 0 {
					gpu = fmt.Sprintf("%d%%", clamp(int(math.Round(best)), 0, 100))
				}
			}
		}
	}
	return "CPU " + cpu + "  GPU* " + gpu + "  RAM " + ram
}
func updateExtras() {
	now := time.Now()
	if cfg.Monitor && now.Sub(lastStats) > time.Second {
		currentStats = stats()
		lastStats = now
	}
	if !cfg.Music {
		audio = 0
		if audioCapture != nil {
			audioCapture.close()
			audioCapture = nil
		}
		return
	}
	if audioCapture == nil && now.After(audioRetry) {
		audioCapture = newCapture()
		audioRetry = now.Add(4 * time.Second)
	}
	if audioCapture != nil && now.Sub(lastAudio) > 25*time.Millisecond {
		v, ok := audioCapture.read()
		if !ok {
			audioCapture.close()
			audioCapture = nil
			audio = 0
		} else {
			audio = audio*.7 + v*.3
		}
		lastAudio = now
	}
}
