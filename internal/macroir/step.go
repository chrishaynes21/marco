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
	// StepScroll turns the wheel at X,Y (window-relative like a click when measured): Wheel is the
	// direction, "up" | "down" | "left" | "right", and Count is how many notches.
	StepScroll StepKind = "scroll"
)

// Step is one instruction in a macro. Fields are interpreted per Kind; unused
// fields are zero. Nested kinds (loop, drag waypoints) use Steps.
type Step struct {
	Kind   StepKind `json:"kind"`
	X      int      `json:"x,omitempty"`
	Y      int      `json:"y,omitempty"`
	X2     int      `json:"x2,omitempty"`
	Y2     int      `json:"y2,omitempty"`
	Button string   `json:"button,omitempty"` // one of MouseButtons; empty is left
	Wheel  string   `json:"wheel,omitempty"`  // StepScroll: "up" | "down" | "left" | "right"
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
	// learned detector ("button", "icon", "menu item", "prompt"), captured at learn time.
	// Like AnchorText it's a LOCATOR, not a gate: codegen emits a `Vision's Locate`
	// fallback that finds the nearest element of this class — the resolver for a control
	// with no text and no clean edge (an icon on a gradient). Empty → no vision resolver.
	// Needs a `--host Vision=bridge:vision` with a model.
	AnchorVision string `json:"anchorVision,omitempty"`
	// AnchorClickX, AnchorClickY are the click's position WITHIN the Template image (its
	// own pixel space, 0-origin), captured at learn time. Learn-time OCR labelling uses
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
	RelX   int  `json:"relX,omitempty"`
	RelY   int  `json:"relY,omitempty"`
	WinRel bool `json:"winRel,omitempty"`
	// ToRelX, ToRelY (set with the terms below, on a StepDrag) are where the gesture
	// ENDED, as an offset into the same window — the destination of a drag.
	//
	// Its own pair rather than reusing RelX,RelY for whichever end is being talked about,
	// because a drag has two points and they are not interchangeable: one is where the
	// button goes down and the other is where it comes up, and a reader who had to work
	// out which end an offset belonged to would be reading a bug waiting to happen.
	//
	// MEASURED AT THE RELEASE AND NOT DERIVED FROM THE PRESS. The alternative was
	// subtracting the press's rectangle from the release's absolute coordinate, which is
	// an offset into the window as it WAS attributed to a point captured after — with
	// nothing able to tell whether the window moved in between, and no way for replay to
	// notice. So both edges of the gesture are measured where they happened and the two
	// have to agree about the window; see `internal/simplify`, which refuses a drag whose
	// edges describe two different rectangles.
	ToRelX int `json:"toRelX,omitempty"`
	ToRelY int `json:"toRelY,omitempty"`

	// ── WHAT THE WINDOW WAS LIKE WHEN THE PRESS WAS MADE ──
	//
	// Five terms, all measured at the instant of the press and all carried through to the
	// saved route, because a coordinate is only a claim about a place while the thing it
	// was measured against is still the same thing. A replayer that adds an offset to a
	// window without checking any of these is pressing a remembered pixel and calling it
	// a target.
	//
	// ONE SET OF THEM FOR A DRAG TOO, AND ONE IS THE POINT. A drag's two offsets are two
	// points in ONE window, so there is one rectangle, measured once — the start and the
	// destination resolve against the same live window at replay. Carrying a second set
	// would be inviting a drag to be checked against two windows, which is not a gesture.
	// What makes that honest is that both edges were measured and shown to agree before
	// these were written; a drag whose window changed mid-gesture never gets here.
	//
	// They are also the OPT-IN. A route whose points carry them asks for the guarded
	// contract — bind the window, then check it again immediately before the press — and
	// a route whose points do not keeps exactly the behaviour it was saved with. That is
	// why they are recorded rather than inferred: the presence of the evidence is the
	// request for it to be checked. See [[ADR-250-the-recorder-runs-in-the-window]].

	// RelTo names what RelX,RelY are measured FROM, and is the one term here that is
	// about meaning rather than measurement.
	//
	// Recorded rather than assumed, although there is exactly one value today, because
	// the alternative is that every route already saved gets silently reinterpreted the
	// day a recorder starts measuring from somewhere else. An offset from the outer frame
	// and an offset from the client area are different numbers for the same pixel, and
	// nothing in a file holding only `RelX 391` says which one it is.
	RelTo string `json:"relTo,omitempty"`
	// WinW, WinH are the OUTER dimensions of that window — the whole thing including its
	// frame, because that is what the origin is measured from too.
	WinW int `json:"winW,omitempty"`
	WinH int `json:"winH,omitempty"`
	// Dpi is the window's DPI, and it is its own term rather than something read off the
	// dimensions. An application that handles `WM_DPICHANGED` can relayout everything
	// inside a window whose outer rectangle never moves: same size, everything somewhere
	// else. 0 means it could not be read, which is not the same as 96.
	Dpi int `json:"dpi,omitempty"`
	// Process is the file name of the executable that owned the window, lowercased —
	// "arkascended.exe". The identity term, not the path: the path answers "is this the
	// same installation", which is a different question and makes a routine useless on
	// another machine.
	Process string `json:"process,omitempty"`

	Steps []Step `json:"steps,omitempty"` // loop body / drag waypoints
}

// RelToWindow is the one basis a demonstration is measured from today: the OUTER top-left of the
// window the press landed in, frame included, as `GetWindowRect` reports it.
//
// It is a named constant in the IR rather than a string in three packages because it is the
// vocabulary a saved route and the host that replays it have to agree on. A fourth spelling of
// it anywhere is a route that resolves its offsets against something nobody checked.
const RelToWindow = "window"
