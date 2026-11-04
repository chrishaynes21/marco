package oshost

import (
	"strings"
	"testing"
)

// BindInFront binds the window the reading named, and only if it is still that window and still in
// front. It raises nothing, searches for nothing, and does not mark the run as activated, which
// would change where legacy window-relative presses and anchor probes resolve.
//
// Setting `activated`, or binding without the checks, must fail this.
func TestBindingTheWindowInFrontChecksItAndActivatesNothing(t *testing.T) {
	desk := &TestDesktop{Front: 0x10, Windows: map[uintptr]TestWindow{
		0x10: {PID: 5, Program: "notepad.exe", W: 800, H: 600, Dpi: 96},
		0x20: {PID: 6, Program: "chrome.exe", W: 800, H: 600, Dpi: 96},
	}}
	h := DesktopForTest(t, desk).(*Host)
	w := h.ReadInFront()
	if w.HWnd != 0x10 || w.PID != 5 || w.Program != "notepad.exe" {
		t.Fatalf("the reading is %+v", w)
	}
	if err := h.BindInFront(w); err != nil {
		t.Fatal(err)
	}
	if hw, app, pid := h.binding(); hw != 0x10 || pid != 5 || app != "notepad.exe" {
		t.Errorf("bound %#x %s %d", hw, app, pid)
	}
	if h.didActivate() {
		t.Error("binding the window in front marked the run as activated")
	}
	if _, _, raised, searched := desk.Read(); raised != 0 || searched != 0 {
		t.Errorf("raised %d, searched %d", raised, searched)
	}

	for _, c := range []struct {
		what   string
		change func(d *TestDesktop)
		says   string
	}{
		{"it left the front between the reading and the bind", func(d *TestDesktop) { d.Front = 0x20 }, "no longer in front"},
		{"it closed", func(d *TestDesktop) { delete(d.Windows, 0x10) }, "closed"},
		{"its handle has another owner", func(d *TestDesktop) { w := d.Windows[0x10]; w.PID = 9; d.Windows[0x10] = w }, "another process"},
	} {
		h := DesktopForTest(t, desk).(*Host)
		desk.Change(func(d *TestDesktop) {
			d.Front = 0x10
			d.Windows[0x10] = TestWindow{PID: 5, Program: "notepad.exe", W: 800, H: 600, Dpi: 96}
		})
		w := h.ReadInFront()
		desk.Change(c.change)
		err := h.BindInFront(w)
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", c.what, err)
		}
		if hw, _, _ := h.binding(); hw != 0 {
			t.Errorf("%s: a window was bound anyway", c.what)
		}
	}
	if err := h.BindInFront(InFront{}); err == nil {
		t.Error("an unreadable foreground was bound")
	}
}
