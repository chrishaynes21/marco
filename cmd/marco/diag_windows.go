//go:build windows

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	// Imported for its DPI-awareness init() — the same one oshost/the recorder run,
	// so `diag` reports and exercises the exact coordinate space the clicker uses.
	_ "github.com/chaynes-simpleclouds/marco/internal/screen"
)

// runDiag prints the process's DPI awareness, the monitor layout as Windows reports
// it, and the live cursor position; `diag move X Y` then SetCursorPos's there and
// reports where the cursor actually landed. It's the round-trip test for "clicks on
// the second monitor go to the wrong place": move your mouse onto monitor 2, run
// `marco diag` to read the physical coordinate, then `marco diag move <x> <y>` with
// those numbers — if it lands somewhere else, the coordinate space is the problem.
func runDiag(args []string) {
	var (
		user32                       = syscall.NewLazyDLL("user32.dll")
		procGetCursorPos             = user32.NewProc("GetCursorPos")
		procSetCursorPos             = user32.NewProc("SetCursorPos")
		procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
		procGetThreadDpiAwareness    = user32.NewProc("GetThreadDpiAwarenessContext")
		procAwarenessFromDpiAwareCtx = user32.NewProc("GetAwarenessFromDpiAwarenessContext")
	)
	type pt struct{ x, y int32 }
	metric := func(i int) int { r, _, _ := procGetSystemMetrics.Call(uintptr(i)); return int(int32(r)) }
	cursor := func() (int, int) {
		var p pt
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
		return int(p.x), int(p.y)
	}

	awareness := "unknown"
	if procGetThreadDpiAwareness.Find() == nil {
		ctx, _, _ := procGetThreadDpiAwareness.Call()
		aw, _, _ := procAwarenessFromDpiAwareCtx.Call(ctx)
		switch int(int32(aw)) {
		case 0:
			awareness = "UNAWARE  (⚠ multi-monitor/scaled coordinates will be virtualized — wrong)"
		case 1:
			awareness = "system   (⚠ wrong on a second monitor at a different scale)"
		case 2:
			awareness = "per-monitor-v2  (correct)"
		default:
			awareness = fmt.Sprintf("code %d", int(int32(aw)))
		}
	}

	fmt.Printf("DPI awareness:   %s\n", awareness)
	fmt.Printf("Primary monitor: %d x %d\n", metric(0), metric(1)) // SM_CXSCREEN / SM_CYSCREEN
	fmt.Printf("Virtual desktop: origin (%d, %d)  size %d x %d\n",
		metric(76), metric(77), metric(78), metric(79)) // SM_X/Y/CX/CY VIRTUALSCREEN
	cx, cy := cursor()
	fmt.Printf("Cursor now:      (%d, %d)\n", cx, cy)

	if len(args) == 3 && args[0] == "move" {
		x, errX := strconv.Atoi(args[1])
		y, errY := strconv.Atoi(args[2])
		if errX != nil || errY != nil {
			fmt.Fprintln(os.Stderr, "usage: marco diag move <x> <y>")
			os.Exit(2)
		}
		ret, _, _ := procSetCursorPos.Call(uintptr(x), uintptr(y))
		nx, ny := cursor()
		fmt.Printf("SetCursorPos(%d, %d): ret=%d -> cursor landed at (%d, %d)\n", x, y, ret, nx, ny)
		if nx != x || ny != y {
			fmt.Printf("  ✗ MISMATCH — requested (%d,%d), landed (%d,%d). The clicker can't reach that point.\n", x, y, nx, ny)
		} else {
			fmt.Println("  ✓ landed exactly — the coordinate path is fine for this point.")
		}
	} else {
		fmt.Println("\nTip: move your mouse onto the second monitor, run `marco diag` to read its")
		fmt.Println("coordinate, then `marco diag move <x> <y>` with those numbers to test the round-trip.")
	}
}
