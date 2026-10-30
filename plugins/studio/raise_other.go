//go:build !windows

package main

// THE OTHER SIDE OF A WINDOWS SURFACE, KEPT COMPILING.
//
// Cross-platform by construction: every OS surface in this product sits behind a function with a
// Windows implementation and a stub, and the stub side has to build. This one answers no, which
// is the honest answer — nothing has been brought forward — and the run then relies on the
// application already leading the desktop, exactly as it does when Windows refuses the raise.
//
// `checked` is false, which is the difference that matters: "it is not in front" would refuse
// every run on this platform, and what is true is that nobody here can say where it is. The
// run's own foreground guard then answers it at the instant it matters.

func raiseWindow(uintptr, uint32) (leading, checked bool) { return false, false }

// raiseWindowTraced is the same answer with nothing observed, because there was nothing to ask.
func raiseWindowTraced(h uintptr, pid uint32) (bool, raiseTrace, bool) {
	return false, raiseTrace{Asked: h, AskedPID: pid,
		Why: "this platform cannot say where the foreground is"}, false
}

// raiseTrace and raiseLook are the Windows file's shapes, so the one caller compiles here too.
type raiseTrace struct {
	Asked                 uintptr     `json:"asked_hwnd"`
	AskedPID              uint32      `json:"asked_pid"`
	LeadingBefore         uintptr     `json:"leading_before_hwnd"`
	LeadingPIDBefore      uint32      `json:"leading_before_pid"`
	Restored              bool        `json:"restored,omitempty"`
	SetForegroundReturned bool        `json:"setforeground_returned"`
	SetForegroundError    string      `json:"setforeground_error,omitempty"`
	Looks                 []raiseLook `json:"looks"`
	LeadingAfter          uintptr     `json:"leading_after_hwnd"`
	LeadingPIDAfter       uint32      `json:"leading_after_pid"`
	Why                   string      `json:"why,omitempty"`
}

type raiseLook struct {
	At   int     `json:"look"`
	Hwnd uintptr `json:"hwnd"`
	PID  uint32  `json:"pid"`
}
