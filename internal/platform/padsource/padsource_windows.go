//go:build windows

package padsource

import (
	"syscall"
	"time"
	"unsafe"
)

// The Windows controller reader: XInputGetState, and nothing else.
//
// # Read-only by omission, not by discipline
//
// XInputSetState is the rumble call, and it is the only way this API can affect anything. It
// is not bound here. A future edit that wanted to actuate a controller would have to add a
// procedure binding, which is a visible act in a diff rather than a silent one — the same
// argument navsource makes about its raw key codes.
//
// # Why a poll is fine here when a hook would not be
//
// CLAUDE.md's hook rules are about low-level keyboard and mouse hooks, which share the
// installing thread and are dropped by Windows if a callback overruns. XInput has no callback
// and no hook: it is a device read on our own goroutine, so a slow consumer costs us a sample
// and costs the desktop nothing. None of the pump rules apply, and this file installs nothing.

var xinputDLLs = []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"}

const (
	padDPadUp    = 0x0001
	padDPadDown  = 0x0002
	padDPadLeft  = 0x0004
	padDPadRight = 0x0008
	padStart     = 0x0010
	padBack_     = 0x0020
	padA         = 0x1000
	padB         = 0x2000

	errorSuccess = 0

	// rescanInterval bounds how often disconnected slots are probed. XInputGetState on an
	// empty slot is documented as expensive, and probing four of them 125 times a second
	// would spend real CPU asking a question whose answer changes when somebody plugs in a
	// cable.
	rescanInterval = time.Second
)

// xinputGamepad mirrors XINPUT_GAMEPAD. The trigger bytes are read because the struct layout
// requires them, and are never looked at.
type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

type windowsBackend struct {
	getState *syscall.LazyProc
	why      string

	live       [4]bool
	lastRescan time.Time
}

func newBackend() backend {
	b := &windowsBackend{}
	for _, name := range xinputDLLs {
		dll := syscall.NewLazyDLL(name)
		if err := dll.Load(); err != nil {
			continue
		}
		proc := dll.NewProc("XInputGetState")
		if err := proc.Find(); err != nil {
			continue
		}
		b.getState = proc
		return b
	}
	b.why = "no XInput runtime is present on this machine, so a controller cannot be observed"
	return b
}

func (b *windowsBackend) unavailable() string { return b.why }

func (b *windowsBackend) poll() padSnapshot {
	if b.getState == nil {
		return padSnapshot{}
	}
	// Slots known to hold a controller are read every time; the empty ones are probed
	// occasionally, because XInputGetState on an empty slot is documented as expensive and
	// its answer only changes when somebody plugs in a cable.
	rescan := time.Since(b.lastRescan) >= rescanInterval
	if rescan {
		b.lastRescan = time.Now()
	}
	var found []padSnapshot
	for i := range b.live {
		if !b.live[i] && !rescan {
			continue
		}
		s, ok := b.read(i)
		b.live[i] = ok
		if ok {
			found = append(found, s)
		}
	}
	return merge(found)
}

// read polls one slot and translates it. XInput's button word dies in this function.
func (b *windowsBackend) read(index int) (padSnapshot, bool) {
	var st xinputState
	r, _, _ := b.getState.Call(uintptr(index), uintptr(unsafe.Pointer(&st)))
	if r != errorSuccess {
		return padSnapshot{}, false
	}
	raw := st.Gamepad.Buttons
	var buttons padButton
	for mask, meaning := range map[uint16]padButton{
		padDPadUp:    padUp,
		padDPadDown:  padDown,
		padDPadLeft:  padLeft,
		padDPadRight: padRight,
		padA:         padConfirm,
		padB:         padBack,
		padStart:     padPause,
		padBack_:     padPause,
	} {
		if raw&mask != 0 {
			buttons |= meaning
		}
	}
	return padSnapshot{
		connected: true,
		buttons:   buttons,
		stickX:    int(st.Gamepad.ThumbLX),
		stickY:    int(st.Gamepad.ThumbLY),
	}, true
}
