//go:build windows

// marco-show is the surface Marco points with.
//
// # What it is
//
// A transient, click-through, non-activating window that outlines rectangles somebody else
// computed. It is the visual half of "this is what I mean", and it is deliberately the dumbest
// process in the system: it reads a role, a sentence and a list of rectangles from stdin, draws
// them, and exits.
//
// # What it must never become
//
// It holds no model, resolves no subject and reads nothing from the screen. Every semantic decision
// — WHICH thing is meant, whether it can be located at all, which of its parts are present — was
// made in the Director, where the evidence is. A surface that could work any of that out for itself
// would be a second place deciding what Marco means, and it would be the one with the least
// evidence and the most incentive to always produce something.
//
// # The four invariants, and where each is enforced
//
//	never takes focus          WS_EX_NOACTIVATE + InitUnfocused + SetRunnableOnUnfocused
//	never intercepts a click   WS_EX_LAYERED|WS_EX_TRANSPARENT, PROBED with WindowFromPoint
//	never sees a keystroke     follows from never having focus, counted below
//	never becomes perception   the ownership property, set on its own handle
//	never draws where it isn't the window it was placed as — see hold
//
// An invariant nobody measures is a hope. An invariant measured by the wrong question is worse,
// because it reports success: passthrough was originally read off a counter of clicks Ebiten
// delivered to this game, which stays at zero whether the surface is transparent to the mouse or
// swallowing every click before anyone downstream hears about it. Found live — text could not be
// selected anywhere beneath the surface — with the counter reporting NO the whole time.
//
// So passthrough is now asked of the OS hit test, and a surface that finds itself in the way exits
// rather than reporting it. Standing between somebody and their computer is not a thing to log.
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"sync"
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
	procSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	procSetPropW            = user32.NewProc("SetPropW")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procWindowFromPoint     = user32.NewProc("WindowFromPoint")
)

// windowAt is who Windows says owns a desktop pixel.
//
// THE probe for click-through, and the reason it is the right one: WindowFromPoint performs the
// same hit test the mouse does, and it skips windows marked WS_EX_TRANSPARENT. If it hands back
// this surface's own handle, this surface is standing in front of the user's application — whatever
// style bits were asked for and whatever the input counters say.
func windowAt(x, y int) uintptr {
	// A POINT is two int32s and is passed by value, which on amd64 is one register.
	h, _, _ := procWindowFromPoint.Call(uintptr(uint32(int32(x))) | uintptr(uint32(int32(y)))<<32)
	return h
}

// windowTitle names THIS instance's window, uniquely.
//
// The base is what a person sees; the pid suffix is what `adopt` finds the window BY. Two
// surfaces may overlap on purpose — a learn highlight while a `sight --show` runs — and with
// one shared title FindWindowW resolved whichever was created first, so the second instance
// adopted the first one's window: set its owned-surface property, rewrote its styles, and
// fought its per-frame position reassert forever. A title only this process uses makes the
// window it finds structurally its own.
var windowTitle = fmt.Sprintf("marco is pointing (%d)", os.Getpid())

type winRect struct{ left, top, right, bottom int32 }

// Request is what the Director sends. Rectangles and words; nothing else crosses.
type Request struct {
	Boxes   []Box  `json:"boxes"`
	Say     string `json:"say"`
	Role    string `json:"role"`
	Seconds int    `json:"seconds"`
}

// Box is one desktop rectangle, in the pixels GetWindowRect reports.
type Box struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func foreground() uintptr { h, _, _ := procGetForegroundWindow.Call(); return h }

func metric(i int) int { v, _, _ := procGetSystemMetrics.Call(uintptr(i)); return int(int32(v)) }

// virtualDesktop is the union of every monitor, in the same coordinates the boxes are in.
//
// The clamp target. A window asked for a position outside it is silently moved by the window
// manager, and a surface that then drew at the position it ASKED for would put every outline a few
// pixels off — which is exactly the near-miss a person reads as Marco meaning something else.
func virtualDesktop() (x, y, w, h int) {
	const (
		smXVirtualScreen  = 76
		smYVirtualScreen  = 77
		smCXVirtualScreen = 78
		smCYVirtualScreen = 79
	)
	return metric(smXVirtualScreen), metric(smYVirtualScreen),
		metric(smCXVirtualScreen), metric(smCYVirtualScreen)
}

// pointer is the drawing.
type pointer struct {
	boxes []Box
	// originX/originY translate desktop pixels into this window's own space. Set from the
	// window's ACTUAL rectangle once it exists — see adopt.
	originX, originY int
	w, h             int

	// wantX..wantH is the rectangle this surface asked for, kept so adopt can insist on it.
	wantX, wantY, wantW, wantH int

	frames int
	life   int // frames to live
	fade   int // frames of fade in and out
	clicks int
	keys   int

	// mu guards hwnd only, which adopt writes off the game thread and hold reads on it.
	mu   sync.Mutex
	hwnd uintptr
	// moved counts how many times the window had to be put back, and lost says it could not
	// be. Reported at exit: a surface that silently wandered is worse than one that admits it.
	moved int
	lost  bool
	// blocking is the surface discovering it is standing in front of the user's application.
	blocking bool
}

func (p *pointer) Update() error {
	p.frames++
	p.hold()
	// WHAT THE SURFACE RECEIVED. WS_EX_TRANSPARENT says Windows should not hit-test this
	// window and WS_EX_NOACTIVATE says it should never take focus; only an actual event
	// arriving here can prove neither happened. Both must still be zero at exit.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		p.clicks++
	}
	p.keys += len(inpututil.AppendJustPressedKeys(nil))
	if p.blocking || p.frames > p.life {
		return ebiten.Termination
	}
	return nil
}

// alpha is the transience: it arrives, holds, and leaves.
//
// A highlight that snapped on and off would read as a flash — an alert. Reference is quieter than
// that, and the fade is most of what carries the difference.
func (p *pointer) alpha() float64 {
	switch {
	case p.frames < p.fade:
		return float64(p.frames) / float64(p.fade)
	case p.frames > p.life-p.fade:
		return math.Max(0, float64(p.life-p.frames)/float64(p.fade))
	}
	return 1
}

// The palette: a calm, cool accent. Deliberately NOT the system selection colour, not red, and
// never a solid fill — those all say something Marco is not saying. Selection, error and focus are
// three claims this surface must never make.
var (
	accent = color.NRGBA{0x7c, 0xa9, 0xf0, 0xff}
	wash   = color.NRGBA{0x7c, 0xa9, 0xf0, 0x1c}
)

func fade(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * a))
	return c
}

func (p *pointer) Draw(screen *ebiten.Image) {
	// Nothing is drawn before the window's true origin is known, and nothing is drawn once the
	// surface can no longer cover what it was made to show. Both are the same rule: this is a
	// claim about specific pixels, and a claim that cannot be placed is not made.
	p.mu.Lock()
	ready := p.hwnd != 0
	p.mu.Unlock()
	if !ready || p.lost {
		return
	}
	a := p.alpha()
	if a <= 0 {
		return
	}
	for _, b := range p.boxes {
		x := float32(b.X - p.originX)
		y := float32(b.Y - p.originY)
		w, h := float32(b.Width), float32(b.Height)
		// A whisper of wash so a thin control still reads, then the outline that does the
		// actual pointing. Each region keeps its own outline: a set of choices is several
		// controls, and one rectangle around all of them would claim the space between.
		vector.DrawFilledRect(screen, x, y, w, h, fade(wash, a), false)
		vector.StrokeRect(screen, x, y, w, h, 1.5, fade(accent, a), true)
	}
	if len(p.boxes) > 1 {
		p.drawBelonging(screen, a)
	}
}

// drawBelonging says "these are one thing" without saying "and so is everything between them".
//
// Corner ticks at the extremes of the group, never a closed rectangle and never a fill. A complete
// outline around several controls is read as one control; four short marks are read as a bracket,
// which is the relationship that is actually being claimed.
func (p *pointer) drawBelonging(screen *ebiten.Image, a float64) {
	minX, minY := math.MaxInt32, math.MaxInt32
	maxX, maxY := math.MinInt32, math.MinInt32
	for _, b := range p.boxes {
		minX, minY = min(minX, b.X), min(minY, b.Y)
		maxX, maxY = max(maxX, b.X+b.Width), max(maxY, b.Y+b.Height)
	}
	const gap = 6
	l := float32(minX-p.originX) - gap
	t := float32(minY-p.originY) - gap
	r := float32(maxX-p.originX) + gap
	btm := float32(maxY-p.originY) + gap
	// Tick length scales with the group but stays short: a tick that grew into a side would
	// become the closed outline this exists to avoid.
	tick := float32(math.Min(14, math.Min(float64(r-l), float64(btm-t))/4))
	c := fade(accent, a*0.75)
	for _, seg := range [][4]float32{
		{l, t, l + tick, t}, {l, t, l, t + tick},
		{r - tick, t, r, t}, {r, t, r, t + tick},
		{l, btm - tick, l, btm}, {l, btm, l + tick, btm},
		{r, btm - tick, r, btm}, {r - tick, btm, r, btm},
	} {
		vector.StrokeLine(screen, seg[0], seg[1], seg[2], seg[3], 1.5, c, true)
	}
}

func (p *pointer) Layout(int, int) (int, int) { return p.w, p.h }

func main() {
	var req Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "marco-show: reading the request:", err)
		os.Exit(2)
	}
	if len(req.Boxes) == 0 {
		fmt.Fprintln(os.Stderr, "marco-show: nothing to point at")
		os.Exit(2)
	}
	if req.Seconds <= 0 {
		req.Seconds = 5
	}

	x, y, w, h := surfaceFor(req.Boxes)

	before := foreground()
	fmt.Printf("FOREGROUND before hwnd=%#x\n", before)

	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowMousePassthrough(true)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowPosition(x, y)

	const fps = 60
	g := &pointer{
		boxes: req.Boxes, originX: x, originY: y, w: w, h: h,
		wantX: x, wantY: y, wantW: w, wantH: h,
		life: req.Seconds * fps, fade: fps / 3,
	}
	go g.adopt()

	op := &ebiten.RunGameOptions{ScreenTransparent: true, SkipTaskbar: true, InitUnfocused: true}
	if err := ebiten.RunGameWithOptions(g, op); err != nil && err != ebiten.Termination {
		fmt.Fprintln(os.Stderr, "marco-show:", err)
	}

	after := foreground()
	fmt.Printf("FOREGROUND after hwnd=%#x\n", after)
	fmt.Printf("FOREGROUND_HWND_UNCHANGED: %s\n", yesNo(before == after))
	fmt.Printf("COMPANION_RECEIVED_POINTER: %s (%d)\n", yesNo(g.clicks > 0), g.clicks)
	fmt.Printf("COMPANION_RECEIVED_KEYBOARD: %s (%d)\n", yesNo(g.keys > 0), g.keys)
	// A surface that moved and was put back is fine and is still worth counting. One that could
	// no longer cover its regions stopped drawing, and that must be visible rather than looking
	// like a highlight nobody noticed.
	fmt.Printf("SURFACE_HELD_ITS_PLACE: %s (corrected %d)\n", yesNo(!g.lost), g.moved)
}

func yesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

// surfaceFor is the window rectangle: everything to be drawn, plus room for the outline, clamped
// to the desktop that actually exists.
//
// # The padding is PRESENTATION, and it shrinks rather than pushing
//
// The rectangles arriving here are exact — they came from `pkg/referent`, which is arithmetic over
// measured geometry and knows nothing about outlines. An outline drawn ON the boundary of a region
// is half outside it, so the surface needs a few pixels of room. That room is this file's business
// and nobody else's: compensating for it upstream would corrupt the canonical mapping for every
// other consumer, including ones that do not draw at all.
//
// At a monitor edge the padding is simply not taken. Asking for a window past the left edge of the
// leftmost monitor gets it moved back by the window manager, and the drawing — which is positioned
// relative to where the window WAS asked to be — then lands several pixels off. Giving up the
// padding costs a sliver of an outline; taking it costs the accuracy of every rectangle.
func surfaceFor(boxes []Box) (x, y, w, h int) {
	minX, minY := math.MaxInt32, math.MaxInt32
	maxX, maxY := math.MinInt32, math.MinInt32
	for _, b := range boxes {
		minX, minY = min(minX, b.X), min(minY, b.Y)
		maxX, maxY = max(maxX, b.X+b.Width), max(maxY, b.Y+b.Height)
	}
	// Enough for the outline, the corner ticks and their gap.
	const pad = 12
	minX, minY, maxX, maxY = minX-pad, minY-pad, maxX+pad, maxY+pad

	vx, vy, vw, vh := virtualDesktop()
	if vw > 0 && vh > 0 {
		minX, minY = max(minX, vx), max(minY, vy)
		maxX, maxY = min(maxX, vx+vw), min(maxY, vy+vh)
	}
	return minX, minY, maxX - minX, maxY - minY
}

// adopt claims the window as Marco's and reconciles where it actually ended up.
//
// Runs once the window exists — Ebiten creates it inside RunGame, so there is nothing to mark
// before that.
//
// Three things, in this order, and the order matters:
//
//  1. the ownership property, FIRST, so the window is excluded from perception as early as
//     possible. A surface visible to Director for even one cycle is contamination.
//  2. the extended style bits, asserted rather than assumed. Passthrough and no-activate are asked
//     for through Ebiten; this makes sure of them, because "the library probably did" is not a
//     basis for an invariant.
//  3. the true origin. Whatever the window manager did with the requested position, the drawing is
//     translated by where the window IS. This is the belt to the clamp's braces: the clamp keeps
//     the surface covering the regions, and this keeps the regions on the right pixels even if
//     something moved it anyway.
//
// hold keeps the surface where it was put, every frame, and stops drawing when it cannot.
//
// # The failure this exists for
//
// Found live on a two-monitor desktop: the highlight was correct, the user alt-tabbed to a browser
// on the other screen, and the outlines relocated a monitor over. Nothing upstream moved — the
// rectangles Director computed were right and stayed right. Ebiten recomputes its monitor and
// device scale factor when focus changes and repositions its window, and the origin every box is
// drawn relative to was read ONCE, at startup, so from that moment on the whole surface was
// translated by however far the window had travelled.
//
// # Two defences, because the first one cannot be trusted alone
//
//  1. Put it back. The rectangle was chosen to cover the regions and clamped to the desktop that
//     exists; anything else moving it is something this surface did not ask for.
//  2. Believe the rectangle, not the request. The origin is re-read every frame, so even during
//     the frame a move is being corrected the outlines land on the pixels they were measured at.
//
// And when the window no longer covers what it was made to show, drawing STOPS. A highlight in the
// wrong place is worse than no highlight — that rule is applied everywhere else in this chain, in
// pkg/referent and in the referent resolver, and this surface was the one place it was missing.
//
// Deleting either the re-assert or the coverage check must fail the marco-show hold tests.
func (p *pointer) hold() {
	p.mu.Lock()
	hwnd := p.hwnd
	p.mu.Unlock()
	if hwnd == 0 {
		return
	}
	r, ok := rectOf(hwnd)
	if !ok {
		return
	}
	if !sameRect(r, p.wantX, p.wantY, p.wantW, p.wantH) {
		p.moved++
		const (
			swpNoZOrder   = 0x0004
			swpNoActivate = 0x0010
		)
		procSetWindowPos.Call(hwnd, 0,
			uintptr(int32(p.wantX)), uintptr(int32(p.wantY)),
			uintptr(int32(p.wantW)), uintptr(int32(p.wantH)),
			swpNoZOrder|swpNoActivate)
		if again, ok := rectOf(hwnd); ok {
			r = again
		}
	}
	p.originX, p.originY = int(r.left), int(r.top)
	// The check that actually matters. Not "is it where I asked" — a window manager is
	// entitled to shave a few pixels at a monitor edge, and the origin above already absorbs
	// that. This asks whether the thing can still be drawn at all where it belongs.
	if !p.covers(int(r.left), int(r.top), int(r.right-r.left), int(r.bottom-r.top)) {
		p.lost = true
	}
	p.checkPassthrough(hwnd)
}

// checkPassthrough asks Windows who owns the pixels this surface is sitting on.
//
// # Why a counter was not enough
//
// The exit report counted clicks EBITEN delivered to this game, and reported passthrough from
// that. Those are different questions. A window can swallow every click at the hit-test stage —
// the user's drag never starts, their selection never happens — while delivering nothing to the
// game, so the counter reads a confident NO and the invariant is broken anyway. Found live: text
// could not be selected anywhere under the surface until it was stopped.
//
// WindowFromPoint performs the same hit test the mouse does and skips WS_EX_TRANSPARENT windows.
// If it returns this surface's own handle, this surface is in the way. That is the question.
//
// # And it acts, rather than only reporting
//
// A highlight that is blocking the desktop has already failed at the only thing it was for, and
// the person is left prising a decoration off their screen. It leaves immediately. Being wrong
// about what Marco meant is recoverable; standing between somebody and their computer is not.
func (p *pointer) checkPassthrough(hwnd uintptr) {
	// Every half second at 60fps, and once early. Cheap, but not free, and nothing about the
	// answer changes between frames unless a style did.
	if p.frames != 2 && p.frames%30 != 0 {
		return
	}
	for _, b := range p.boxes {
		if windowAt(b.X+b.Width/2, b.Y+b.Height/2) == hwnd {
			p.blocking = true
			return
		}
	}
}

// covers reports whether the surface still contains every region it was made to show.
func (p *pointer) covers(x, y, w, h int) bool {
	for _, b := range p.boxes {
		if b.X < x || b.Y < y || b.X+b.Width > x+w || b.Y+b.Height > y+h {
			return false
		}
	}
	return true
}

func rectOf(hwnd uintptr) (winRect, bool) {
	var r winRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r, r.right > r.left && r.bottom > r.top
}

func sameRect(r winRect, x, y, w, h int) bool {
	return int(r.left) == x && int(r.top) == y &&
		int(r.right-r.left) == w && int(r.bottom-r.top) == h
}

func (p *pointer) adopt() {
	name, _ := syscall.UTF16PtrFromString(windowTitle)
	var hwnd uintptr
	for i := 0; i < 200 && hwnd == 0; i++ {
		hwnd, _, _ = procFindWindowW.Call(0, uintptr(unsafe.Pointer(name)))
		if hwnd == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if hwnd == 0 {
		fmt.Fprintln(os.Stderr, "marco-show: could not find my own window")
		return
	}

	if prop, err := syscall.UTF16PtrFromString(directorapi.OwnedSurfaceProperty); err == nil {
		procSetPropW.Call(hwnd, uintptr(unsafe.Pointer(prop)), 1)
	}

	const (
		gwlExStyle      = ^uintptr(19) // -20
		wsExTransparent = 0x00000020
		wsExNoActivate  = 0x08000000
		wsExLayered     = 0x00080000
		wsExToolWindow  = 0x00000080
	)
	// LAYERED goes on WITH transparent, and that pairing is the fix for a live failure: the user
	// could not select text anywhere under the surface until it went away. WS_EX_TRANSPARENT
	// alone is documented for paint ordering; the combination with WS_EX_LAYERED is what makes
	// Windows skip the window when it decides which one a click belongs to.
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle,
		ex|wsExTransparent|wsExNoActivate|wsExToolWindow|wsExLayered)
	ex, _, _ = procGetWindowLongPtrW.Call(hwnd, gwlExStyle)

	// PLACED HERE, not by the graphics library. Measured on a two-monitor desktop with the
	// left screen at x=-1920: a window asked for x=-1897 was created at x=-1920, a silent
	// 23-pixel shift. The size was honoured exactly, so nothing about the surface looked
	// wrong — every outline inside it was simply drawn 23 pixels from where it was measured,
	// which is the near-miss a person reads as Marco meaning a different control.
	//
	// SWP_NOACTIVATE, because moving a window must not be a way of focusing one.
	// SWP_FRAMECHANGED, because the style bits above were written after the window existed and
	// Windows caches the frame it computed from the old ones. Without it the styles are set and
	// not yet in effect, which is indistinguishable from not having set them.
	const (
		swpNoZOrder     = 0x0004
		swpNoActivate   = 0x0010
		swpFrameChanged = 0x0020
	)
	procSetWindowPos.Call(hwnd, 0,
		uintptr(int32(p.wantX)), uintptr(int32(p.wantY)),
		uintptr(int32(p.wantW)), uintptr(int32(p.wantH)),
		swpNoZOrder|swpNoActivate|swpFrameChanged)

	// The window is now known, and [hold] owns where it is from here on — every frame, on the
	// game thread. Publishing the handle is the LAST thing this does, because it is also what
	// lets drawing begin: a frame drawn before the true origin is known would be drawn relative
	// to nothing, which is the whole desktop's worth of offset.
	r, _ := rectOf(hwnd)
	p.mu.Lock()
	p.hwnd = hwnd
	p.mu.Unlock()

	fmt.Printf("SURFACE hwnd=%#x rect=%dx%d+%d+%d exstyle=%#x transparent=%v noactivate=%v "+
		"layered=%v toolwindow=%v owned=yes\n",
		hwnd, r.right-r.left, r.bottom-r.top, r.left, r.top, ex,
		ex&wsExTransparent != 0, ex&wsExNoActivate != 0,
		ex&wsExLayered != 0, ex&wsExToolWindow != 0)
}
