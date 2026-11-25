package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
)

// WinRT media metadata is independent from WASAPI loopback samples.
// Windows PowerShell runs in a hidden child process, never on the UI thread.
type mediaInfo struct {
	Title, Artist, Source, Status string
	Position, Duration            float64
	Updated                       time.Time
	Art                           string `json:"Art"`
	Pixels                        []byte `json:"-"`
	ArtW, ArtH                    int    `json:"-"`
}

var mediaSnapshot atomic.Pointer[mediaInfo]

const mediaQuery = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null=[Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager,Windows.Media.Control,ContentType=WindowsRuntime]
$null=[Windows.Media.Control.GlobalSystemMediaTransportControlsSessionMediaProperties,Windows.Media.Control,ContentType=WindowsRuntime]
$asTask=[System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {$_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name.StartsWith('IAsyncOperation')} | Select-Object -First 1
function Await($op,$type) { $task=$asTask.MakeGenericMethod($type).Invoke($null,@($op)); if(-not $task.Wait(2000)){throw 'Media query timed out'}; $task.Result }
$mgr=Await ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync()) ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager])
$s=$mgr.GetCurrentSession()
if($null -eq $s){'{}';exit}
$p=Await ($s.TryGetMediaPropertiesAsync()) ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionMediaProperties])
$t=$s.GetTimelineProperties()
$pos=($t.Position-$t.StartTime).TotalSeconds
$duration=($t.EndTime-$t.StartTime).TotalSeconds
$status=$s.GetPlaybackInfo().PlaybackStatus.ToString()
if($status -eq 'Playing' -and $duration -gt 0){$elapsed=([DateTimeOffset]::UtcNow-$t.LastUpdatedTime).TotalSeconds; if($elapsed -ge 0 -and $elapsed -lt $duration){$pos=[Math]::Min($duration,$pos+$elapsed)}}
$art=''
try {
 if($null -ne $p.Thumbnail){
  $null=[Windows.Storage.Streams.IRandomAccessStreamWithContentType,Windows.Storage.Streams,ContentType=WindowsRuntime]
  $rs=Await ($p.Thumbnail.OpenReadAsync()) ([Windows.Storage.Streams.IRandomAccessStreamWithContentType])
  try {if($rs.Size -gt 0 -and $rs.Size -le 1048576){
    $stream=[System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($rs.GetInputStreamAt(0))
    try {$mem=[IO.MemoryStream]::new();try{$stream.CopyTo($mem);$art=[Convert]::ToBase64String($mem.ToArray())}finally{$mem.Dispose()}}finally{$stream.Dispose()}
  }} finally {$rs.Dispose()}
 }
} catch { $art='' }
@{Art=$art;Title=$p.Title;Artist=$p.Artist;Source=$s.SourceAppUserModelId;Status=$status;Position=$pos;Duration=$duration} | ConvertTo-Json -Compress
`

func encodedPS(s string) string {
	r := utf16.Encode([]rune(s))
	b := make([]byte, len(r)*2)
	for i, v := range r {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(b)
}
func decodeArt(m *mediaInfo) {
	b, err := base64.StdEncoding.DecodeString(m.Art)
	m.Art = ""
	if err != nil || len(b) == 0 || len(b) > 1048576 {
		return
	}
	c, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || c.Width < 1 || c.Height < 1 || c.Width > 2048 || c.Height > 2048 {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	const size = 96
	m.Pixels = make([]byte, size*size*4)
	m.ArtW = size
	m.ArtH = size
	side := w
	if h < side {
		side = h
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, _ := img.At(bounds.Min.X+(w-side)/2+x*side/size, bounds.Min.Y+(h-side)/2+y*side/size).RGBA()
			i := (y*size + x) * 4
			m.Pixels[i] = byte(b >> 8)
			m.Pixels[i+1] = byte(g >> 8)
			m.Pixels[i+2] = byte(r >> 8)
		}
	}
}
func mediaWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		if audioEnabled.Load() {
			q, cancel := context.WithTimeout(ctx, 8*time.Second)
			cmd := exec.CommandContext(q, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPS(mediaQuery))
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			b, err := cmd.Output()
			cancel()
			m := mediaInfo{}
			if err == nil && json.Unmarshal([]byte(strings.TrimSpace(string(b))), &m) == nil {
				decodeArt(&m)
				m.Updated = time.Now()
				mediaSnapshot.Store(&m)
			} else {
				mediaSnapshot.Store(nil)
			}
		} else {
			mediaSnapshot.Store(nil)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
