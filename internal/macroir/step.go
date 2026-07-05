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
	StepKeyDown  StepKind = "keydown"  // Key — press and HOLD (released by a later StepKeyUp)
	StepKeyUp    StepKind = "keyup"    // Key — release a held key
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
	// Color, on a StepClick with a Template, is the "0xRRGGBB" pixel under the click.
	// codegen emits it as the anchor's secondary resolver: confirm the screen by that
	// one pixel at the click point when the template match is marginal. Empty → none.
	Color string `json:"color,omitempty"`
	// Window, on a StepClick, is the title of the foreground window at click time.
	// codegen emits it as the anchor's CONTEXT check: a strong match in the WRONG window
	// (an app like Steam with several windows) can't produce a confident click. Empty → none.
	Window string `json:"window,omitempty"`
	// AnchorText, on a StepClick, is on-screen text to locate by OCR (the Text
	// resolver). Unlike Template/Color (gates → click the recorded coordinate), text is
	// a LOCATOR for a moved target: codegen emits a `Text's Find` fallback that clicks
	// where the word IS. Empty → no text resolver. Needs a `--host Text=bridge:ocr`.
	AnchorText string `json:"anchorText,omitempty"`
	// AnchorVision, on a StepClick, is the CLASS of the clicked control as named by the
	// learned detector ("button", "icon", "menu item", "prompt"), captured at teach time.
	// Like AnchorText it's a LOCATOR, not a gate: codegen emits a `Vision's Locate`
	// fallback that finds the nearest element of this class — the resolver for a control
	// with no text and no clean edge (an icon on a gradient). Empty → no vision resolver.
	// Needs a `--host Vision=bridge:vision` with a model.
	AnchorVision string `json:"anchorVision,omitempty"`
	// AnchorClickX, AnchorClickY are the click's position WITHIN the Template image (its
	// own pixel space, 0-origin), captured at teach time. Teach-time OCR labelling uses
	// it to read only the button UNDER the click — not every word in the crop — so a
	// captured menu yields the label you clicked, not a jumble of its neighbours. (0,0)
	// means unknown (the OCR step falls back to the template centre).
	AnchorClickX int `json:"anchorClickX,omitempty"`
	AnchorClickY int `json:"anchorClickY,omitempty"`
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
