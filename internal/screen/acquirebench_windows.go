//go:build windows

package screen

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	procStretchBlt         = gdi32.NewProc("StretchBlt")
	procSetStretchBltMode  = gdi32.NewProc("SetStretchBltMode")
	procGetGuiResources    = user32.NewProc("GetGuiResources")
	procGetCurrentProcessW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcess")
)

const (
	// stretchColorOnColor is COLORONCOLOR: drop pixels rather than blend them.
	//
	// The cheap mode on purpose. HALFTONE averages, which costs more and blurs the very
	// edges the proposer looks for — and [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]]
	// already measured that a cheap nearest-neighbour reduction keeps every click target
	// on a proposed region. If the cheap mode turns out to lose them, the expensive one
	// is worth measuring; until then it is a cost with no evidence behind it.
	stretchColorOnColor = 3
	// grGDIObjects is GR_GDIOBJECTS: this process's GDI handle count.
	grGDIObjects = 0
)

// gdiObjects is how many GDI handles this process holds, or zero if it cannot be asked.
func gdiObjects() uint32 {
	h, _, _ := procGetCurrentProcessW.Call()
	n, _, _ := procGetGuiResources.Call(h, grGDIObjects)
	return uint32(n)
}

// acquireHandles is the device context and bitmap a capture needs.
type acquireHandles struct {
	screenDC, memDC, bmp uintptr
	w, h                 int
}

// release gives every handle back. Safe on a partially built set.
func (a *acquireHandles) release() {
	if a.bmp != 0 {
		procDeleteObject.Call(a.bmp)
		a.bmp = 0
	}
	if a.memDC != 0 {
		procDeleteDC.Call(a.memDC)
		a.memDC = 0
	}
	if a.screenDC != 0 {
		procReleaseDC.Call(0, a.screenDC)
		a.screenDC = 0
	}
}

// BenchmarkAcquire times the acquisition stages of a capture, for several configurations.
//
// The source rectangle is the same for every configuration; only the destination changes. It
// reads the screen and retains nothing: see the note on acquirebench.go.
//
// `frames` captures per configuration, `maxWall` a hard stop so a slow machine cannot turn a
// bounded benchmark into an unbounded one.
func BenchmarkAcquire(x, y, w, h int, cfgs []AcquireConfig, frames int,
	maxWall time.Duration) ([]AcquireResult, error) {

	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("screen: a %dx%d source rectangle is not a window", w, h)
	}
	if frames <= 0 {
		return nil, fmt.Errorf("screen: %d captures is not a measurement", frames)
	}
	deadline := time.Now().Add(maxWall)
	out := make([]AcquireResult, 0, len(cfgs))

	for _, cfg := range cfgs {
		by := cfg.Divide
		if by < 1 {
			by = 1
		}
		dw, dh := w/by, h/by
		if dw <= 0 || dh <= 0 {
			return nil, fmt.Errorf("screen: /%d of %dx%d is not a destination", by, w, h)
		}
		r := AcquireResult{
			Config: cfg, SrcW: w, SrcH: h, DstW: dw, DstH: dh,
			Bytes: dw * dh * 4, GDIBefore: gdiObjects(),
		}
		stage := map[string]*AcquireStage{}
		order := []string{"GetDC", "DC+bitmap setup", "blit", "GetDIBits", "cleanup",
			"total acquisition"}
		for _, n := range order {
			stage[n] = &AcquireStage{Name: n}
		}

		// The destination buffer is allocated ONCE and reused by every capture of this
		// configuration, so the measurement is of acquisition and not of the allocator
		// — and so no capture's pixels outlive the next one.
		buf := make([]byte, dw*dh*4)
		bmi := bitmapInfoHeader{
			Size: 40, Width: int32(dw), Height: -int32(dh),
			Planes: 1, BitCount: 32, Compression: biRGB,
		}

		var held acquireHandles
		if cfg.Reuse {
			t := time.Now()
			if err := openHandles(&held, dw, dh); err != nil {
				held.release()
				return nil, err
			}
			stage["DC+bitmap setup"].add(time.Since(t))
		}

		for i := 0; i < frames; i++ {
			if time.Now().After(deadline) {
				break
			}
			whole := time.Now()
			h2 := held
			if !cfg.Reuse {
				t := time.Now()
				sdc, _, _ := procGetDC.Call(0)
				stage["GetDC"].add(time.Since(t))
				if sdc == 0 {
					r.Failures++
					continue
				}
				h2 = acquireHandles{screenDC: sdc, w: dw, h: dh}
				t = time.Now()
				if err := fillHandles(&h2, dw, dh); err != nil {
					h2.release()
					r.Failures++
					continue
				}
				stage["DC+bitmap setup"].add(time.Since(t))
			}

			// ── THE BLIT ──
			t := time.Now()
			var ok uintptr
			if by == 1 {
				ok, _, _ = procBitBlt.Call(h2.memDC, 0, 0, uintptr(w), uintptr(h),
					h2.screenDC, uintptr(x), uintptr(y), srcCopy)
			} else {
				procSetStretchBltMode.Call(h2.memDC, stretchColorOnColor)
				ok, _, _ = procStretchBlt.Call(h2.memDC, 0, 0, uintptr(dw), uintptr(dh),
					h2.screenDC, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
					srcCopy)
			}
			stage["blit"].add(time.Since(t))
			if ok == 0 {
				r.Failures++
				if !cfg.Reuse {
					h2.release()
				}
				continue
			}

			// ── THE TRANSFER ──
			t = time.Now()
			got, _, _ := procGetDIBits.Call(h2.memDC, h2.bmp, 0, uintptr(dh),
				uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bmi)), dibRGB)
			stage["GetDIBits"].add(time.Since(t))
			if got == 0 {
				r.Failures++
				if !cfg.Reuse {
					h2.release()
				}
				continue
			}

			// ── THE VALIDITY CHECK: ONE BIT ──
			//
			// A blit that failed silently returns black, and a configuration that was
			// faster because it returned nothing would read as an improvement. This
			// asks whether ANY byte was non-zero and keeps nothing else: no sample,
			// no hash, no region, no count of colours.
			if anyNonZero(buf) {
				r.NonBlank++
			}

			if !cfg.Reuse {
				t = time.Now()
				h2.release()
				stage["cleanup"].add(time.Since(t))
			}
			stage["total acquisition"].add(time.Since(whole))
			r.Frames++
		}
		if cfg.Reuse {
			t := time.Now()
			held.release()
			stage["cleanup"].add(time.Since(t))
		}
		// THE BUFFER IS WIPED before it goes out of scope. Nothing reads it again, and
		// a measurement has no business leaving a window's pixels in a heap somebody
		// else will be handed.
		for i := range buf {
			buf[i] = 0
		}
		r.GDIAfter = gdiObjects()
		for _, n := range order {
			r.Stages = append(r.Stages, *stage[n])
		}
		out = append(out, r)
	}
	return out, nil
}

// openHandles takes a screen DC and builds a memory DC and bitmap on it.
func openHandles(a *acquireHandles, w, h int) error {
	sdc, _, _ := procGetDC.Call(0)
	if sdc == 0 {
		return fmt.Errorf("screen: GetDC failed")
	}
	a.screenDC = sdc
	return fillHandles(a, w, h)
}

// fillHandles builds the memory DC and bitmap for an already-taken screen DC.
func fillHandles(a *acquireHandles, w, h int) error {
	mem, _, _ := procCreateCompatibleDC.Call(a.screenDC)
	if mem == 0 {
		return fmt.Errorf("screen: CreateCompatibleDC failed")
	}
	a.memDC = mem
	bmp, _, _ := procCreateCompatibleBmp.Call(a.screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return fmt.Errorf("screen: CreateCompatibleBitmap failed")
	}
	a.bmp = bmp
	procSelectObject.Call(mem, bmp)
	a.w, a.h = w, h
	return nil
}

// anyNonZero reports whether a buffer holds anything at all.
//
// The whole validity check, and deliberately the weakest question that answers it. It stops at
// the first non-zero byte, so it is also the cheapest.
func anyNonZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return true
		}
	}
	return false
}
