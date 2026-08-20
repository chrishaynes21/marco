package directorapi

import "testing"

// The fusion example from the Director design: an accessibility button, an OCR word
// inside it, and a vision detection of the same control. All three must be
// recognisable as ONE element, and the two overlap measures do different halves of
// that job — IoU for coincident boxes, Covers for a contained one.
func TestFusionOverlapSignals(t *testing.T) {
	access := Rect{X: 900, Y: 700, Width: 120, Height: 40}
	ocr := Rect{X: 920, Y: 710, Width: 70, Height: 20}
	vision := Rect{X: 900, Y: 700, Width: 120, Height: 40}

	if got := access.IoU(vision); got != 1.0 {
		t.Errorf("identical boxes should have IoU 1.0, got %v", got)
	}
	// The OCR word is wholly inside the button, so IoU is low — this is exactly the
	// case a naive IoU threshold would reject.
	if got := access.IoU(ocr); got > 0.35 {
		t.Errorf("a contained word should have low IoU, got %v", got)
	}
	// Covers sees it correctly: 100% of the word lies within the button.
	if got := access.Covers(ocr); got != 1.0 {
		t.Errorf("the button fully covers the word, want 1.0, got %v", got)
	}
}

func TestIoUDisjointIsZero(t *testing.T) {
	a := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	b := Rect{X: 100, Y: 100, Width: 10, Height: 10}
	if got := a.IoU(b); got != 0 {
		t.Errorf("disjoint boxes should have IoU 0, got %v", got)
	}
	if got := a.Intersect(b); got != (Rect{}) {
		t.Errorf("disjoint boxes should not intersect, got %+v", got)
	}
}

func TestIoUPartialOverlap(t *testing.T) {
	// Two 10x10 boxes overlapping in a 5x10 strip: intersection 50, union 150.
	a := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	b := Rect{X: 5, Y: 0, Width: 10, Height: 10}
	want := 50.0 / 150.0
	if got := a.IoU(b); got < want-1e-9 || got > want+1e-9 {
		t.Errorf("IoU = %v, want %v", got, want)
	}
}

// An empty rectangle is how a source says "this exists but I don't know where".
// It must never produce a click point that looks legitimate, and it must not
// contaminate geometry it takes part in.
func TestEmptyRectIsInert(t *testing.T) {
	empty := Rect{}
	real := Rect{X: 10, Y: 10, Width: 20, Height: 20}

	if !empty.Empty() {
		t.Error("the zero Rect must report empty")
	}
	if (Rect{X: 5, Y: 5, Width: 0, Height: 10}).Empty() != true {
		t.Error("a zero-width rect must report empty")
	}
	if got := empty.Area(); got != 0 {
		t.Errorf("an empty rect has no area, got %d", got)
	}
	if got := empty.IoU(real); got != 0 {
		t.Errorf("an empty rect overlaps nothing, got %v", got)
	}
	if got := real.Covers(empty); got != 0 {
		t.Errorf("covering an empty rect is meaningless, want 0, got %v", got)
	}
	// Union ignores an empty operand rather than stretching to the origin.
	if got := empty.Union(real); got != real {
		t.Errorf("union with empty should be the other rect, got %+v", got)
	}
	if got := real.Union(empty); got != real {
		t.Errorf("union with empty should be the other rect, got %+v", got)
	}
}

func TestCenterAndContains(t *testing.T) {
	r := Rect{X: 900, Y: 700, Width: 120, Height: 40}
	if got := r.Center(); got != (Point{X: 960, Y: 720}) {
		t.Errorf("centre = %+v, want (960,720)", got)
	}
	if !r.Contains(r.Center()) {
		t.Error("a rect must contain its own centre")
	}
	// Right and bottom edges are exclusive, so a neighbouring control that starts
	// exactly where this one ends isn't claimed by it.
	if r.Contains(Point{X: 1020, Y: 720}) {
		t.Error("the right edge must be exclusive")
	}
	if r.Contains(Point{X: 960, Y: 740}) {
		t.Error("the bottom edge must be exclusive")
	}
	if !r.Contains(Point{X: 900, Y: 700}) {
		t.Error("the top-left corner is inside")
	}
}

// Negative coordinates are normal on a multi-monitor desktop: a monitor placed left
// of or above the primary has a negative origin. Geometry must work there.
func TestNegativeCoordinates(t *testing.T) {
	left := Rect{X: -1920, Y: 0, Width: 1920, Height: 1080}
	el := Rect{X: -1000, Y: 500, Width: 100, Height: 30}

	if !left.ContainsRect(el) {
		t.Error("an element on the left monitor must be contained by it")
	}
	if got := el.Center(); got != (Point{X: -950, Y: 515}) {
		t.Errorf("centre on a negative-origin monitor = %+v, want (-950,515)", got)
	}
	if got := left.Covers(el); got != 1.0 {
		t.Errorf("the monitor fully covers the element, got %v", got)
	}
}

func TestContainsRect(t *testing.T) {
	outer := Rect{X: 0, Y: 0, Width: 100, Height: 100}
	if !outer.ContainsRect(Rect{X: 10, Y: 10, Width: 10, Height: 10}) {
		t.Error("an inner rect should be contained")
	}
	// Flush against the far edge still counts as inside.
	if !outer.ContainsRect(Rect{X: 90, Y: 90, Width: 10, Height: 10}) {
		t.Error("a rect flush with the far edge is still contained")
	}
	if outer.ContainsRect(Rect{X: 95, Y: 95, Width: 10, Height: 10}) {
		t.Error("a rect crossing the edge is not contained")
	}
}

// The source ladder is the Director's central perception rule, so it gets a test:
// structured sources must outrank pixel interpretation, every time.
func TestSourceLadderOrdering(t *testing.T) {
	ladder := []ObservationSource{
		SourceNative, SourceDOM, SourceAccessibility,
		SourceWindowSystem, SourcePlugin, SourceOCR, SourceVision, SourceModel,
	}
	for i := 1; i < len(ladder); i++ {
		if SourceRank(ladder[i-1]) <= SourceRank(ladder[i]) {
			t.Errorf("%s must outrank %s", ladder[i-1], ladder[i])
		}
	}
	if SourceRank(SourceAccessibility) <= SourceRank(SourceOCR) {
		t.Error("accessibility must be stronger evidence than OCR")
	}
	if SourceRank(SourceOCR) <= SourceRank(SourceVision) {
		t.Error("OCR must be stronger evidence than visual guessing")
	}
	if SourceRank("something-new") != 0 {
		t.Error("an unknown source must rank lowest, not accidentally high")
	}
}

func TestStructuredSources(t *testing.T) {
	for _, s := range []ObservationSource{SourceNative, SourceDOM, SourceAccessibility, SourceWindowSystem} {
		if !s.Structured() {
			t.Errorf("%s should be structured", s)
		}
	}
	for _, s := range []ObservationSource{SourceOCR, SourceVision, SourceModel} {
		if s.Structured() {
			t.Errorf("%s must not count as structured", s)
		}
	}
}

// An element that is disabled, invisible or has no bounds is a resolution RESULT,
// not a target. The Director should say "Save is greyed out" rather than clicking
// nothing and reporting success.
func TestActionableRejectsUnusableElements(t *testing.T) {
	bounds := Rect{X: 900, Y: 700, Width: 120, Height: 40}
	good := &Element{Role: RoleButton, Enabled: true, Visible: true, Bounds: bounds}
	if !good.Actionable() {
		t.Error("an enabled, visible, positioned button is actionable")
	}
	for name, e := range map[string]*Element{
		"disabled":  {Role: RoleButton, Enabled: false, Visible: true, Bounds: bounds},
		"invisible": {Role: RoleButton, Enabled: true, Visible: false, Bounds: bounds},
		"no bounds": {Role: RoleButton, Enabled: true, Visible: true},
		"offscreen": {Role: RoleButton, Enabled: true, Visible: true, Bounds: bounds, Offscreen: true},
		// Scenery. Treating a static label as actionable is exactly how an inert
		// exact-label match outranks the control the user meant.
		"inert text": {Role: RoleText, Enabled: true, Visible: true, Bounds: bounds},
		"a pane":     {Role: RolePane, Enabled: true, Visible: true, Bounds: bounds},
		"nil":        nil,
	} {
		if e.Actionable() {
			t.Errorf("a %s element must not be actionable", name)
		}
	}
}

// Different intents need different capabilities, which is why this is a set of flags
// rather than one boolean: a click needs Invokable, typing needs Settable, "the
// second tab" needs Selectable.
func TestActionabilityCapabilities(t *testing.T) {
	bounds := Rect{X: 10, Y: 10, Width: 80, Height: 24}
	el := func(r ElementRole) *Element {
		return &Element{Role: r, Enabled: true, Visible: true, Bounds: bounds}
	}

	button := el(RoleButton).Actions()
	if !button.Invokable || !button.Interactive || !button.Focusable {
		t.Errorf("a button should be invokable and focusable: %+v", button)
	}
	if button.Settable {
		t.Error("a button holds no value")
	}

	field := el(RoleTextField).Actions()
	if !field.Settable || !field.Focusable {
		t.Errorf("a text field should be settable and focusable: %+v", field)
	}
	if field.Invokable {
		t.Error("a text field is not invoked, it is typed into")
	}

	if !el(RoleTab).Actions().Selectable {
		t.Error("a tab is selectable")
	}

	check := el(RoleCheckbox).Actions()
	if !check.Invokable || !check.Settable {
		t.Errorf("a checkbox is both pressed and holds a value: %+v", check)
	}

	if text := el(RoleText).Actions(); text.Any() || text.Interactive {
		t.Errorf("static text affords nothing: %+v", text)
	}

	// An element can be perfectly interactive and still have nowhere to aim, which
	// is why reachability is separate from invokability rather than folded into it.
	noBounds := &Element{Role: RoleButton, Enabled: true, Visible: true}
	if a := noBounds.Actions(); a.ClickableByBounds {
		t.Error("an element with no bounds has nowhere to click")
	} else if !a.Invokable {
		t.Error("...but it is still an invokable kind of thing")
	}

	// A provider that genuinely knows about focus may say so.
	overridden := &Element{
		Role: RoleText, Enabled: true, Visible: true, Bounds: bounds,
		Attributes: map[string]any{"keyboard_focusable": true},
	}
	if !overridden.Actions().Focusable {
		t.Error("a provider's own focusability report should win")
	}
}

// Addressable is "can be named AND acted on" — what "click Save" could resolve to,
// and the basis of the coverage and actionability measures.
func TestAddressable(t *testing.T) {
	bounds := Rect{X: 10, Y: 10, Width: 80, Height: 24}
	if named := (&Element{Role: RoleButton, Label: "Save", Enabled: true, Visible: true, Bounds: bounds}); !named.Addressable() {
		t.Error("a named, usable button is addressable")
	}
	if anon := (&Element{Role: RoleButton, Enabled: true, Visible: true, Bounds: bounds}); anon.Addressable() {
		t.Error("an unlabelled control cannot be named, so it cannot be addressed")
	}
	if scenery := (&Element{Role: RoleText, Label: "Save", Enabled: true, Visible: true, Bounds: bounds}); scenery.Addressable() {
		t.Error("a label is not addressable however well it is named")
	}
}

// Container versus content is what distinguishes an application that exposed its
// interior from one that only exposed its shell — the Electron signature.
func TestContainerRoles(t *testing.T) {
	for _, r := range []ElementRole{RoleWindow, RolePane, RoleGroup, RoleToolbar, RoleUnknown, ""} {
		if !r.Container() || r.Content() {
			t.Errorf("%q should be a container, not content", r)
		}
	}
	for _, r := range []ElementRole{RoleButton, RoleText, RoleTextField, RoleMenuItem, RoleTab} {
		if r.Container() || !r.Content() {
			t.Errorf("%q should be content, not a container", r)
		}
	}
}

func TestRiskOrdering(t *testing.T) {
	if !RiskHigh.AtLeast(RiskMedium) || !RiskMedium.AtLeast(RiskLow) {
		t.Error("risk must be totally ordered high > medium > low")
	}
	if RiskLow.AtLeast(RiskMedium) {
		t.Error("low risk must not clear a medium bar")
	}
	if !RiskLow.AtLeast(RiskLow) {
		t.Error("AtLeast must be inclusive")
	}
}

// A reference with nothing in it is a planner bug. Validation depends on catching
// it here rather than letting execution improvise a target.
func TestElementReferenceResolvable(t *testing.T) {
	if (ElementReference{}).Resolvable() {
		t.Error("an empty reference is not resolvable")
	}
	if !(ElementReference{ID: "e1"}).Resolvable() {
		t.Error("an ID is resolvable")
	}
	if !(ElementReference{Query: &ElementQuery{Label: "Save"}}).Resolvable() {
		t.Error("a query is resolvable")
	}
	// A bare point is NOT resolvable: coordinate replay must be asked for explicitly.
	if (ElementReference{Point: &Point{X: 1, Y: 2}}).Resolvable() {
		t.Error("an unlocked coordinate must not be resolvable — no silent coordinate fallback")
	}
	if !(ElementReference{Point: &Point{X: 1, Y: 2}, CoordinateLocked: true}).Resolvable() {
		t.Error("an explicitly coordinate-locked point is resolvable")
	}
}

// An unconstrained query would match the whole desktop.
func TestElementQueryConstrained(t *testing.T) {
	if (&ElementQuery{}).Constrained() {
		t.Error("an empty query must not count as constrained")
	}
	var nilQuery *ElementQuery
	if nilQuery.Constrained() {
		t.Error("a nil query must not count as constrained")
	}
	// Scoping alone isn't a constraint — "everything in Notepad" is still everything.
	if (&ElementQuery{Application: "notepad"}).Constrained() {
		t.Error("an application scope alone is not a constraint")
	}
	if !(&ElementQuery{Label: "Save"}).Constrained() {
		t.Error("a label is a constraint")
	}
	if !(&ElementQuery{Ordinal: 2}).Constrained() {
		t.Error("an ordinal is a constraint")
	}
}

func TestRoleCapabilities(t *testing.T) {
	if !RoleButton.Clickable() || !RoleMenuItem.Clickable() || !RoleTab.Clickable() {
		t.Error("buttons, menu items and tabs are clickable")
	}
	if RoleText.Clickable() || RoleHeading.Clickable() {
		t.Error("inert text must not be treated as clickable")
	}
	if !RoleTextField.TextEditable() {
		t.Error("a text field accepts text")
	}
	if RoleButton.TextEditable() {
		t.Error("a button does not accept text")
	}
}

// Describe backs confirmations and logs, so it must prefer the user's own words and
// must never be empty.
func TestActionDescribe(t *testing.T) {
	click := ClickAction{Target: ElementReference{Description: "the submit button"}}
	if got := click.Describe(); got != "click the submit button" {
		t.Errorf("Describe should quote the user, got %q", got)
	}
	byLabel := ClickAction{Target: ElementReference{Query: &ElementQuery{Label: "Save"}}}
	if got := byLabel.Describe(); got != `click "Save"` {
		t.Errorf("Describe should fall back to the label, got %q", got)
	}
	if got := (ClickAction{}).Describe(); got == "" {
		t.Error("Describe must never be empty")
	}
	if got := (ClickAction{}).ActionType(); got != ActionClick {
		t.Errorf("ActionType = %q", got)
	}
}

// A secret action must describe itself by NAME. If Describe leaked the value, it
// would reach every confirmation prompt and log line.
func TestTypeSecretDescribeNamesNoValue(t *testing.T) {
	a := TypeAction{
		Target:    ElementReference{Description: "the password field"},
		SecretRef: "facebook:password",
	}
	got := a.Describe()
	if got == "" {
		t.Fatal("Describe must never be empty")
	}
	if a.Text != "" {
		t.Fatal("a secret action must not carry a plaintext value")
	}
	if want := "facebook:password"; !contains(got, want) {
		t.Errorf("Describe should name the credential, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Every concrete action must satisfy the interface. This is the list that fails when
// someone adds an action and forgets a method.
func TestAllActionsImplementTheInterface(t *testing.T) {
	actions := []Action{
		ClickAction{}, TypeAction{}, KeyAction{}, ScrollAction{}, DragAction{},
		FocusAction{}, ActivateAction{}, LaunchAction{}, MoveWindowAction{},
		WaitAction{}, RepeatAction{}, SequenceAction{},
	}
	seen := map[ActionType]bool{}
	for _, a := range actions {
		typ := a.ActionType()
		if typ == "" {
			t.Errorf("%T has an empty ActionType", a)
		}
		if seen[typ] {
			t.Errorf("%T reuses ActionType %q", a, typ)
		}
		seen[typ] = true
		if a.Describe() == "" {
			t.Errorf("%T has an empty Describe", a)
		}
	}
}

// WorldState lookups are used everywhere and must be total — a nil or empty snapshot
// is a normal early state, not a crash.
func TestWorldStateLookupsAreTotal(t *testing.T) {
	var nilWorld *WorldState
	if _, ok := nilWorld.Element("e1"); ok {
		t.Error("a nil world has no elements")
	}
	if _, ok := nilWorld.Window("w1"); ok {
		t.Error("a nil world has no windows")
	}
	if _, ok := nilWorld.FocusedWindow(); ok {
		t.Error("a nil world has no focused window")
	}

	empty := &WorldState{}
	if _, ok := empty.Element("e1"); ok {
		t.Error("an empty world has no elements")
	}
	if _, ok := empty.FocusedWindow(); ok {
		t.Error("a world with no active window reports none")
	}

	id := WindowID("w1")
	w := &WorldState{
		ActiveWindow: &id,
		Windows:      []Window{{ID: "w0"}, {ID: "w1", Title: "Save As"}},
		Elements:     map[ElementID]*Element{"e1": {ID: "e1", Label: "Save"}},
	}
	if got, ok := w.Element("e1"); !ok || got.Label != "Save" {
		t.Error("element lookup failed")
	}
	if got, ok := w.FocusedWindow(); !ok || got.Title != "Save As" {
		t.Error("focused window lookup failed")
	}
}

func TestElementHasSource(t *testing.T) {
	e := &Element{Sources: []ObservationSource{SourceAccessibility, SourceOCR}}
	if !e.HasSource(SourceOCR) {
		t.Error("OCR contributed")
	}
	if e.HasSource(SourceVision) {
		t.Error("vision did not contribute")
	}
}
