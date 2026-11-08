//go:build windows

package recorder

import (
	"testing"
	"unsafe"
)

// THE SIDE BUTTONS ARE RECORDED, BY NAME: back (XBUTTON1) and forward (XBUTTON2), down and up,
// through the mouse hook itself. The owner recorded ctrl+shift+forward and got nothing at all,
// because the hook read only left, right and middle.
//
// Not reading WM_XBUTTON*, or naming the two the other way round, must fail this.
func TestTheSideButtonsAreRecordedByName(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	wasActive := recActive.Load()
	t.Cleanup(func() { recActive.Store(wasActive) })
	recActive.Store(true)
	recMu.Lock()
	recBuf, recPub = nil, make(chan RecordedEvent, 16)
	recMu.Unlock()
	aDesktopWhere(t, chosenWindow, chosenProcess, true)

	hook := func(msg uintptr, xbutton uint32) {
		ms := msllHookStruct{pt: point{x: 640, y: 360}, mouseData: xbutton << 16}
		mouseProc(0, msg, uintptr(unsafe.Pointer(&ms)))
	}
	hook(wmXButtonDown, 2)
	hook(wmXButtonUp, 2)
	hook(wmXButtonDown, 1)
	hook(wmXButtonUp, 1)
	hook(wmXButtonDown, 7) // nothing Windows documents: not recorded as some other button

	recMu.Lock()
	var got []string
	for _, ev := range recBuf {
		if ev.Kind == EvClick {
			edge := "up"
			if ev.Down {
				edge = "down"
			}
			got = append(got, ev.Button+" "+edge)
		}
	}
	recMu.Unlock()
	want := []string{"forward down", "forward up", "back down", "back up"}
	if len(got) != len(want) {
		t.Fatalf("the hook recorded %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("the hook recorded %v, want %v", got, want)
			break
		}
	}
}
