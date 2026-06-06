// Package macroir is the OS-agnostic intermediate representation for a recorded
// or hand-written macro: an ordered list of Steps. The recorder lowers raw OS
// events into Steps, the simplify pass rewrites them, and codegen turns them
// into a Marco route. The shape mirrors the original MacroMarco macro JSON so
// recordings can be dumped, replayed, and re-simplified.
package macroir

// StepKind names the kinds of step a macro can contain.
type StepKind string

const (
	StepClick StepKind = "click" // X,Y,Button
	StepMove  StepKind = "move"  // X,Y — bare cursor move (rare; usually folded away)
	StepKey   StepKind = "key"   // Key, Count
	StepType  StepKind = "type"  // Text
	StepWait  StepKind = "wait"  // Ms
	StepDrag  StepKind = "drag"  // X,Y -> X2,Y2 with Button held (Steps = waypoints)
	StepLoop  StepKind = "loop"  // Count, Steps (nested body)
	StepFind  StepKind = "find"  // Image (reserved for Phase 6 image recognition)
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
	Steps  []Step   `json:"steps,omitempty"` // loop body / drag waypoints
}
