//go:build windows

// groundprobe is the Part 3 platform gate, and nothing else.
//
// It answers one question the audit could not answer by reading code: does a transparent,
// click-through, non-activating Ebiten window spanning the desktop take the foreground away from
// the application underneath it?
//
// It is a PROBE, not a product surface. It has no Director connection, no semantics, no controls
// and no state beyond a rectangle read from the environment. If the invariant it measures does not
// hold, the architecture it is probing is rejected rather than worked around — restoring focus
// after stealing it is not the same as never stealing it.
//
// It prints the foreground window handle at each step so a watcher can compare identities rather
// than titles.
package main

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procSetPropW            = user32.NewProc("SetPropW")
)

type rect struct{ left, top, right, bottom int32 }

func foreground() uintptr { h, _, _ := procGetForegroundWindow.Call(); return h }

// probe draws one outline+fill per box, in DESKTOP coordinates translated into the window's own
// space. The window itself is placed by the caller; the translation is the only arithmetic here,
// and it is a subtraction rather than a second coordinate model.
type probe struct {
	boxes    [][4]int // desktop x,y,w,h
	originX  int
	originY  int
	w, h     int
	step     int
	clicks   int
	keys     int
	reported map[string]bool
}

func (p *probe) Update() error {
	p.step++
	// What the companion RECEIVED. The whole point of the gate: WS_EX_TRANSPARENT says
	// Windows should not hit-test this window, and only an actual event can prove it did not.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		p.clicks++
	}
	p.keys += len(inpututil.AppendJustPressedKeys(nil))
	// Foreground is sampled every frame for the first few seconds; the watcher reads stdout.
	if p.step%60 == 0 {
		fmt.Printf("FOREGROUND step=%d hwnd=%#x\n", p.step, foreground())
	}
	if p.step > 12000 {
		return ebiten.Termination
	}
	return nil
}

func (p *probe) Draw(screen *ebiten.Image) {
	for _, b := range p.boxes {
		x := float32(b[0] - p.originX)
		y := float32(b[1] - p.originY)
		w, h := float32(b[2]), float32(b[3])
		// Subtle translucent fill, then a clear outline. No animation, no label.
		vector.DrawFilledRect(screen, x, y, w, h, color.NRGBA{0x6e, 0xa8, 0xfe, 0x28}, false)
		vector.StrokeRect(screen, x, y, w, h, 2, color.NRGBA{0x6e, 0xa8, 0xfe, 0xe0}, false)
	}
}

func (p *probe) Layout(int, int) (int, int) { return p.w, p.h }

func main() {
	// MARCO_PROBE_BOXES: "x,y,w,h;x,y,w,h" in desktop pixels.
	var boxes [][4]int
	for _, s := range strings.Split(os.Getenv("MARCO_PROBE_BOXES"), ";") {
		f := strings.Split(strings.TrimSpace(s), ",")
		if len(f) != 4 {
			continue
		}
		var b [4]int
		ok := true
		for i, v := range f {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				ok = false
				break
			}
			b[i] = n
		}
		if ok {
			boxes = append(boxes, b)
		}
	}
	if len(boxes) == 0 {
		fmt.Fprintln(os.Stderr, "usage: MARCO_PROBE_BOXES=x,y,w,h[;...] groundprobe")
		os.Exit(2)
	}

	// The window covers the bounding box of what is to be drawn, positioned at its origin.
	// Deliberately NOT the whole virtual desktop: the smallest surface that can show the
	// regions is the smallest surface that can interfere with anything.
	minX, minY := boxes[0][0], boxes[0][1]
	maxX, maxY := boxes[0][0]+boxes[0][2], boxes[0][1]+boxes[0][3]
	for _, b := range boxes[1:] {
		minX, minY = min(minX, b[0]), min(minY, b[1])
		maxX, maxY = max(maxX, b[0]+b[2]), max(maxY, b[1]+b[3])
	}
	const pad = 4
	minX, minY = minX-pad, minY-pad
	w, h := (maxX-minX)+pad, (maxY-minY)+pad

	before := foreground()
	fmt.Printf("FOREGROUND before hwnd=%#x\n", before)

	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowMousePassthrough(true)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowTitle("marco grounding probe")
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowPosition(minX, minY)

	go func() {
		time.Sleep(1500 * time.Millisecond)
		reportProbeWindow()
	}()

	op := &ebiten.RunGameOptions{ScreenTransparent: true, SkipTaskbar: true, InitUnfocused: true}
	g := &probe{boxes: boxes, originX: minX, originY: minY, w: w, h: h,
		reported: map[string]bool{}}
	if err := ebiten.RunGameWithOptions(g, op); err != nil && err != ebiten.Termination {
		fmt.Fprintln(os.Stderr, "groundprobe:", err)
	}
	fmt.Printf("FOREGROUND after hwnd=%#x\n", foreground())
}

// reportProbeWindow prints the probe's own HWND, its actual desktop rectangle and its extended
// style bits — so the caller can check WS_EX_TRANSPARENT / WS_EX_NOACTIVATE without guessing what
// Ebiten did, and can compare the rectangle Windows reports against the one that was asked for.
func reportProbeWindow() {
	name, _ := syscall.UTF16PtrFromString("marco grounding probe")
	h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		fmt.Println("PROBEWINDOW not-found")
		return
	}
	var r rect
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r)))
	const gwlExStyle = ^uintptr(19) // -20
	ex, _, _ := procGetWindowLongPtrW.Call(h, gwlExStyle)
	const (
		wsExTransparent = 0x00000020
		wsExNoActivate  = 0x08000000
		wsExLayered     = 0x00080000
		wsExToolWindow  = 0x00000080
	)
	// THE ownership marker, set on our own handle before anything can enumerate us as an
	// application. A property rather than a title: see directorapi.OwnedSurfaceProperty.
	if prop, err := syscall.UTF16PtrFromString(directorapi.OwnedSurfaceProperty); err == nil {
		procSetPropW.Call(h, uintptr(unsafe.Pointer(prop)), 1)
	}
	fmt.Printf("PROBEWINDOW hwnd=%#x rect=%dx%d+%d+%d exstyle=%#x transparent=%v "+
		"noactivate=%v layered=%v toolwindow=%v\n",
		h, r.right-r.left, r.bottom-r.top, r.left, r.top, ex,
		ex&wsExTransparent != 0, ex&wsExNoActivate != 0,
		ex&wsExLayered != 0, ex&wsExToolWindow != 0)
}
