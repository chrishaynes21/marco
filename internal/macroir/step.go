// Package macroir is the OS-agnostic intermediate representation for a recorded
// or hand-written macro: an ordered list of Steps. The recorder lowers raw OS
// events into Steps, the simplify pass rewrites them, and codegen turns them
// into a Marco route. The shape mirrors the original MacroMarco macro JSON so
// recordings can be dumped, replayed, and re-simplified.
package macroir

// StepKind names the kinds of step a macro can contain.
type StepKind string

const (
	StepClick    StepKind = "click"    // X,Y,Button
	StepMove     StepKind = "move"     // X,Y — bare cursor move (rare; usually folded away)
	StepKey      StepKind = "key"      // Key, Count
	StepType     StepKind = "type"     // Text
	StepWait     StepKind = "wait"     // Ms
	StepDrag     StepKind = "drag"     // X,Y -> X2,Y2 with Button held (Steps = waypoints)
	StepLoop     StepKind = "loop"     // Count, Steps (nested body)
	StepActivate StepKind = "activate" // Text = app to focus/launch (a recorded app switch)
	StepFind     StepKind = "find"     // Image (reserved for Phase 6 image recognition)
)

// Step is one instruction in a macro. Fields are interpreted per Kind; unused
// fields are zero. Nested kinds (loop, drag waypoints) use Steps.
type Step struct {
	Kind   StepKind `json:"kind"`
	X      int      `json:"x,omitempty"`
	Y      int      `json:"y,omitempty"`
	X2     int      `json:"x2,omitempty"`
	Y2     int      `json:"y2,omitempty"`
	Button string   `json:"button,omitempty"` // "left" | "right" | "middle"
	Key    string   `json:"key,omitempty"`    // Marco key name, lowercased
	Text   string   `json:"text,omitempty"`
	Count  int      `json:"count,omitempty"` // key repeats / loop iterations
	Ms     int      `json:"ms,omitempty"`    // wait duration
	Image  string   `json:"image,omitempty"` // StepFind: template path
	// Template, when set on a StepClick, is a captured PNG of the distinctive
	// region the user clicked on. codegen turns such a click into a robust
	// "Find the image, click it; fall back to the recorded coordinate" so the
	// route survives the target moving. Tolerance is the per-channel color slack
	// for the match (0 → codegen's default).
	Template  []byte `json:"template,omitempty"`
	Tolerance int    `json:"tolerance,omitempty"`
	// RelX, RelY (set when WinRel is true, on a StepClick) are the click's offset
	// from the top-left of the app window it was made in. codegen emits a
	// window-relative click that resolves against the activated window at run time —
	// so it lands at the same spot inside the window on any monitor or position —
	// and falls back to the absolute X,Y if that window isn't found.
	RelX   int    `json:"relX,omitempty"`
	RelY   int    `json:"relY,omitempty"`
	WinRel bool   `json:"winRel,omitempty"`
	Steps  []Step `json:"steps,omitempty"` // loop body / drag waypoints
}
