package execute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Value capture.
//
//	Objects always re-resolve. Values never re-resolve.
//
// A capture is the moment a fact leaves the screen and becomes data. Everything about
// how it is done follows from one property: it MUTATES NOTHING. Nothing is clicked,
// nothing is typed, and the only thing that briefly changes — the clipboard, when a
// selection has to be read through it — is put back.
//
// That is why a capture creates no Action Graph node. A node claims the computer was
// touched, and every later count of "what did the Director do" would be wrong. It is
// also why a failed capture is cheap: the program stops with the screen exactly as it
// was, which is the safest possible failure.
//
// The order is fixed:
//
//	observe → resolve → read → classify → verify → bind → continue
//
// Verify before bind, always. An unread value and an empty value produce the same empty
// string, and binding the first would let a later step type nothing into a field and
// report success.

// ControlReader reads a control's own value.
//
// The READ half of the value API and nothing else. Deliberately not providers.Values,
// which can also write: a component that only needs to look at a control should not
// acquire the ability to change it by accident, and this interface is what makes that
// structural rather than a matter of care.
type ControlReader interface {
	GetValue(ctx context.Context, window directorapi.WindowID, nativeID string) (string, bool, error)
}

// selectionProbe is written to the clipboard before a copy, so that "nothing was
// selected" is DISTINGUISHABLE from "the selection matched what was already there".
//
// Without it the two are identical: an unchanged clipboard after Ctrl+C could mean
// either, and guessing would either invent a value or discard a real one. Improbable
// enough that a user's own clipboard will never collide with it.
const selectionProbe = "[director-selection-probe-8f2a4c]"

// clipboardSettle bounds how long a copy is given to land.
//
// A copy is asynchronous: the keystroke returns long before the application has put
// anything on the clipboard. Polling for the probe to disappear is a WAIT for a
// condition rather than a sleep, and it stops the instant the answer is known.
const (
	clipboardSettle   = 600 * time.Millisecond
	clipboardInterval = 25 * time.Millisecond
)

// isCaptureCommand reports whether an intent captures a value.
func isCaptureCommand(in directorapi.Intent) (values.Capture, bool) {
	if in.Verb != intent.VerbCaptureValue {
		return values.Capture{}, false
	}
	c, ok := in.Parameters[values.ParamCapture].(values.Capture)
	return c, ok
}

// handleCapture executes one capture step.
//
// Returns the outcome and true when it handled the intent. It is answered here, ahead
// of the ordinary planning path, because there is no plan to build: nothing will be
// executed, so there is no action for policy to evaluate and no Marco to lower.
func (p *Pipeline) handleCapture(ctx context.Context, request string, in directorapi.Intent,
	pctx program.Context, add func(string, string, bool)) (Outcome, bool) {

	c, ok := isCaptureCommand(in)
	if !ok {
		return Outcome{}, false
	}
	out := Outcome{Request: request, Intent: in, Status: directorapi.ResultFailed}

	if err := c.Validate(); err != nil {
		out.Message = err.Error()
		add("capture", out.Message, false)
		return out, true
	}
	env := pctx.Values
	if env == nil || env.Cleared() {
		// Reachable only if a capture is run outside a program. Named rather than
		// silently creating an environment, which would bind a value nothing could
		// later read.
		out.Message = fmt.Sprintf("cannot capture %q: there is no running program to hold it, "+
			"and captured values do not outlive their program", c.Name)
		add("capture", out.Message, false)
		return out, true
	}
	if env.Has(c.Name) {
		out.Message = fmt.Sprintf("%q is already captured in this program, and values are "+
			"immutable; use a different name", c.Name)
		add("capture", out.Message, false)
		return out, true
	}

	// Before the source is read. No value data exists yet, which is exactly what makes
	// this event worth having: a capture that hangs or crashes leaves a started event
	// with no completion, and that gap is the diagnosis.
	p.emitValue(trace.ValueEvent{
		Kind: trace.EventCaptureStarted, Name: c.Name, CaptureKind: string(c.Kind),
	})

	v, status, msg := p.readValue(ctx, c, in, &out, add)
	if status != directorapi.ResultDone {
		// The capture is over and produced nothing. Reported honestly rather than
		// silently: "unknown" is a result, and a reader needs to see that the step ran.
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventCaptureCompleted, Name: c.Name, CaptureKind: string(c.Kind),
			Verified: false, Outcome: string(status), Reason: msg,
		})
		out.Status, out.Message = status, msg
		return out, true
	}
	p.emitValue(trace.ValueEvent{
		Kind: trace.EventCaptureCompleted, Name: c.Name, CaptureKind: string(c.Kind),
		ValueKind: string(v.Kind()), Visibility: string(v.Visibility()),
		ByteLength: v.Len(), SourceKind: string(v.Provenance().SourceKind),
		Verified: true, Outcome: string(directorapi.ResultDone),
	})

	if err := env.Bind(c.Name, v); err != nil {
		// No value_bound event. A refused duplicate did not bind, and emitting one
		// would put a binding in the record that never existed.
		out.Message = err.Error()
		add("capture", out.Message, false)
		return out, true
	}
	// Only after Bind SUCCEEDED.
	p.emitValue(trace.ValueEvent{
		Kind: trace.EventValueBound, Name: c.Name,
		ValueKind: string(v.Kind()), Visibility: string(v.Visibility()), ByteLength: v.Len(),
	})

	out.Status = directorapi.ResultDone
	// The SUMMARY is safe at any visibility: name, kind, visibility, length. The
	// content is not mentioned, so this line can be logged, traced and shown without a
	// per-call-site decision about whether it is safe.
	out.Message = fmt.Sprintf("Captured %s as %s (%s, %s, %d bytes, verified).",
		c.Kind.Describe(), c.Name, v.Kind(), v.Visibility(), v.Len())
	add("capture", out.Message, true)
	return out, true
}

// readValue performs the read for one capture kind.
//
// Returns the value and ResultDone, or a failure status and an explanation. Every
// failure path names WHY — absent, unreadable, unsafe, unsupported — because the user's
// next move differs for each and "capture failed" tells them nothing.
func (p *Pipeline) readValue(ctx context.Context, c values.Capture, in directorapi.Intent,
	out *Outcome, add func(string, string, bool)) (values.Value, directorapi.ResultStatus, string) {

	// A literal needs no world, no resolution and no desktop read. Running an
	// accessibility walk for `remember "John Smith" as customer` would be pure cost.
	if c.Kind == values.CaptureLiteral {
		add("capture", "bound literal text without observing", true)
		return bindRead(values.KindText, *c.Literal, "the request", values.Provenance{
			SourceKind: values.SourceLiteral,
			Method:     "taken from the request itself; nothing was observed or read",
			StepID:     p.stepID, StepIndex: p.stepIndex,
		})
	}

	world, err := p.observeTraced(ctx)
	if err != nil {
		add("observe", err.Error(), false)
		return values.Value{}, directorapi.ResultFailed,
			"could not observe the screen: " + err.Error()
	}
	out.World = &world
	add("observe", fmt.Sprintf("%d elements", len(world.Elements)), true)

	switch c.Kind {
	case values.CaptureWindowTitle:
		return p.captureWindowTitle(world, add)
	case values.CaptureClipboard:
		return p.captureClipboard(ctx, add)
	}

	// The two kinds that read a specific control. Resolution goes through the ordinary
	// path — same resolver, same ambiguity rules, same clarification — because a
	// capture-specific resolver would be a second definition of "which control did they
	// mean", and the two would drift until one read the wrong field.
	query := directorapi.ElementQuery{}
	if len(in.Targets) > 0 && in.Targets[0].Query != nil {
		query = *in.Targets[0].Query
	}
	res := p.Resolver.Resolve(&world, query)
	out.Resolution = &res
	add("resolve", string(res.Status)+": "+res.Explanation, res.Status == directorapi.ResolutionResolved)

	switch res.Status {
	case directorapi.ResolutionResolved:
	case directorapi.ResolutionAmbiguous:
		return values.Value{}, directorapi.ResultNeedsClarification, res.Explanation
	case directorapi.ResolutionUnobservable:
		return values.Value{}, directorapi.ResultFailed, fmt.Sprintf(
			"could not capture %q: the Director cannot currently observe this interface. "+
				"That is not evidence the value is absent.", c.Name)
	default:
		return values.Value{}, directorapi.ResultFailed, fmt.Sprintf(
			"could not capture %q: %s", c.Name, res.Explanation)
	}
	target := *res.Target

	// The refusal comes BEFORE the read, which is the whole point: a secret that has
	// been read has already been in memory as plaintext, and refusing afterwards would
	// be theatre.
	if why, refuse := refuseSecretRead(target); refuse {
		add("capture", why, false)
		return values.Value{}, directorapi.ResultBlocked, why
	}

	switch c.Kind {
	case values.CaptureControlValue:
		return p.captureControlValue(ctx, c, target, world, add)
	case values.CaptureSelectedText:
		return p.captureSelectedText(ctx, c, target, add)
	}
	return values.Value{}, directorapi.ResultFailed,
		fmt.Sprintf("%s is not a capture this Director performs", c.Kind)
}

// captureWindowTitle reads the title from the world already observed.
//
// No Marco execution and no provider call: the title is already in the World State, and
// asking the desktop again would be a second source of truth for something we just
// looked at.
//
// A window title is a VALUE, not a window reference. If the title changes a second
// later the captured value does not follow — that is what makes it usable as data.
func (p *Pipeline) captureWindowTitle(world directorapi.WorldState,
	add func(string, string, bool)) (values.Value, directorapi.ResultStatus, string) {

	win, ok := world.FocusedWindow()
	if !ok {
		return values.Value{}, directorapi.ResultFailed,
			"could not capture the window title: no window has focus"
	}
	// An untitled window is a real, verified fact — it genuinely has no title. Distinct
	// from not being able to see any window, which is the case above.
	add("capture", fmt.Sprintf("read the title of the focused window (%d bytes)", len(win.Title)), true)
	return bindRead(values.KindWindowTitle, win.Title, "the active window", values.Provenance{
		SourceKind: values.SourceWindowTitle, Application: win.Application,
		Method:   "read from the observed world, with no desktop call",
		Provider: "the World Model",
	})
}

// captureClipboard reads the clipboard without changing it.
//
// Nothing is borrowed and nothing is restored, because nothing is written: this is a
// read, and the clipboard is the thing being read. That is a different operation from
// the clipboard-ASSISTED selection read below, where the clipboard is a means rather
// than the subject.
func (p *Pipeline) captureClipboard(ctx context.Context,
	add func(string, string, bool)) (values.Value, directorapi.ResultStatus, string) {

	if p.Clipboard == nil {
		return values.Value{}, directorapi.ResultFailed,
			"this Director cannot read the clipboard"
	}
	contents, err := p.Clipboard.Read(ctx)
	if err != nil {
		return values.Value{}, directorapi.ResultFailed,
			"could not read the clipboard: " + err.Error()
	}
	switch {
	case contents.Empty:
		// Verified empty. The clipboard genuinely holds nothing, which is a fact and
		// binds normally.
		add("capture", "the clipboard is empty, and verifiably so", true)
	case !contents.IsText:
		// A picture, a file list, something this Director cannot represent. Returning ""
		// would invent an empty string for content that exists.
		return values.Value{}, directorapi.ResultFailed,
			"the clipboard holds something that is not text, and this Director captures " +
				"plain text only; nothing was stored, because unsupported is not empty"
	default:
		add("capture", fmt.Sprintf("read %d bytes of clipboard text", len(contents.Text)), true)
	}

	return bindRead(values.KindClipboard, contents.Text, "the clipboard", values.Provenance{
		SourceKind: values.SourceClipboard,
		Method:     "read the clipboard directly; nothing was written, so nothing was restored",
		Provider:   "OS's ClipboardGet",
		StepID:     p.stepID, StepIndex: p.stepIndex,
	})
}

// captureControlValue reads a control's own value.
//
// The ladder, strongest first:
//
//	the control's own value API  — authoritative, no keystrokes, no guessing
//	the observed World State     — what perception already read
//	Unknown                      — and the program stops
//
// OCR is deliberately NOT on this ladder. Reading pixels tells you what a control
// LOOKS like, which is not the same as what it holds: a masked field shows bullets, a
// truncated field shows an ellipsis, and a scrolled field shows a window onto its
// value. Any of those would be captured as though it were the value itself.
func (p *Pipeline) captureControlValue(ctx context.Context, c values.Capture,
	target directorapi.ResolvedTarget, world directorapi.WorldState,
	add func(string, string, bool)) (values.Value, directorapi.ResultStatus, string) {

	source := describeControl(target)

	if p.ControlValues != nil && target.NativeID != "" {
		text, known, err := p.ControlValues.GetValue(ctx, target.WindowID, target.NativeID)
		switch {
		case err != nil:
			add("capture", "the value API failed: "+err.Error(), false)
		case known:
			// Known INCLUDES the empty string. A field that was read and found empty is
			// a verified fact; that is the distinction the bool exists for.
			add("capture", fmt.Sprintf("read the value API (%d bytes)", len(text)), true)
			return bindRead(values.KindControlValue, text, source, p.controlProv(target,
				"the control's own value API", "Accessibility's GetValue"))
		default:
			add("capture", "the control has no readable value API", false)
		}
	}

	// What perception already saw. Weaker than the value API — it is a snapshot rather
	// than a live read — but it is real evidence rather than a guess.
	if el, ok := world.Element(target.ElementID); ok && el.Value != "" {
		add("capture", fmt.Sprintf("read the observed value (%d bytes)", len(el.Value)), true)
		return bindRead(values.KindControlValue, el.Value, source, p.controlProv(target,
			"the value perception had already observed", "the World Model"))
	}

	return values.Value{}, directorapi.ResultFailed, fmt.Sprintf(
		"could not capture %q: %s exists, but its value could not be read. "+
			"Nothing was stored, because an unreadable value is not an empty one.",
		c.Name, source)
}

// captureSelectedText reads what the user has selected.
//
// There is no selection API in this Director, so the selection is read through the
// clipboard — and the whole of the care below exists because that means touching
// something that belongs to the user.
//
// The sequence:
//
//  1. borrow the clipboard, leaving a PROBE on it
//  2. copy the selection through the ordinary editing path
//  3. wait, bounded, for the probe to be replaced
//  4. take the copied text
//  5. give the clipboard back, on every exit path
//  6. bind only what was copied
//
// The probe is what makes step 3 decidable. Without it, an unchanged clipboard after a
// copy could mean "nothing was selected" or "the selection was identical to what was
// already there", and guessing either way would invent a value or discard a real one.
func (p *Pipeline) captureSelectedText(ctx context.Context, c values.Capture,
	target directorapi.ResolvedTarget,
	add func(string, string, bool)) (values.Value, directorapi.ResultStatus, string) {

	if p.Clipboard == nil {
		return values.Value{}, directorapi.ResultFailed,
			"reading a selection needs the clipboard, which this Director cannot reach"
	}

	saved, err := p.Clipboard.Read(ctx)
	if err != nil {
		return values.Value{}, directorapi.ResultFailed,
			"could not read the clipboard before borrowing it: " + err.Error()
	}
	if !saved.IsText && !saved.Empty {
		// A picture cannot be saved and put back by anything here, so the only way to
		// preserve it is to leave it alone. Refusing the capture is the correct cost:
		// destroying a user's clipboard is not an acceptable price for reading a
		// selection.
		return values.Value{}, directorapi.ResultFailed,
			"the clipboard holds something that is not text and could not be given back " +
				"afterwards, so the selection was not read"
	}

	// Restoration is deferred FIRST, before anything can fail. Cancellation, a timeout
	// and a panic all take this path, which is what "the clipboard is restored on every
	// exit path" has to mean to be worth anything.
	restored := false
	defer func() {
		// A fresh context: the one we were given may already be cancelled, and giving
		// the user their clipboard back is not optional work that a cancellation should
		// skip.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if err := p.Clipboard.Write(rctx, saved.Text); err != nil {
			add("clipboard", "WARNING: the clipboard was borrowed and could not be restored: "+
				err.Error(), false)
			return
		}
		restored = true
		add("clipboard", "borrowed and restored", true)
	}()

	if err := p.Clipboard.Write(ctx, selectionProbe); err != nil {
		return values.Value{}, directorapi.ResultFailed,
			"could not borrow the clipboard: " + err.Error()
	}

	// The copy goes through the ordinary executor, as a legal editing operation against
	// the resolved control. Not a raw keystroke: the editor establishes and CONFIRMS
	// focus first, and input sent to an unfocused control is the one thing this Director
	// never does.
	copyAction := directorapi.EditAction{
		Target:    directorapi.ElementReference{ID: target.ElementID},
		Operation: "copy_selection",
	}
	if _, err := p.Executor.Execute(ctx, copyAction); err != nil {
		return values.Value{}, directorapi.ResultFailed,
			"could not copy the selection: " + err.Error()
	}

	text, copied, err := p.awaitCopy(ctx)
	if err != nil {
		return values.Value{}, directorapi.ResultFailed,
			"could not read the clipboard after copying: " + err.Error()
	}
	if !copied {
		// The probe survived: the copy put nothing on the clipboard. The honest reading
		// is that nothing was selected.
		//
		// NOT the field's whole value. Substituting it would answer a question the user
		// did not ask — they asked for the selection — and would do so most often
		// exactly when they had mis-selected and most needed to be told.
		return values.Value{}, directorapi.ResultFailed, fmt.Sprintf(
			"could not capture %q: nothing is selected. Nothing was stored, and the "+
				"field's full value was not substituted for the selection you asked for.",
			c.Name)
	}
	_ = restored // read by the deferred restore; named so its purpose is visible here

	add("capture", fmt.Sprintf("read %d bytes of selected text", len(text)), true)
	prov := p.controlProv(target, "clipboard probe with restoration", "OS's ClipboardGet")
	prov.SourceKind = values.SourceSelectedText
	prov.ClipboardRestored = &restored
	return bindRead(values.KindText, text, "the selection in "+describeControl(target), prov)
}

// awaitCopy waits, bounded, for the probe to be replaced by copied text.
//
// A wait for a CONDITION, not a sleep. A copy is asynchronous — the keystroke returns
// long before the application has written to the clipboard — and a fixed delay would be
// both slower than needed on a quick application and too short on a slow one.
func (p *Pipeline) awaitCopy(ctx context.Context) (string, bool, error) {
	deadline := time.Now().Add(clipboardSettle)
	for {
		contents, err := p.Clipboard.Read(ctx)
		if err != nil {
			return "", false, err
		}
		if contents.IsText && contents.Text != selectionProbe {
			return contents.Text, true, nil
		}
		if !contents.IsText && !contents.Empty {
			// The application copied something that is not text. Real content, and not
			// something this Director can represent.
			return "", false, fmt.Errorf(
				"the selection was copied as something other than text")
		}
		if time.Now().After(deadline) {
			return "", false, nil
		}
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case <-time.After(clipboardInterval):
		}
	}
}

// bindRead turns a successful read into a classified, verified value.
func bindRead(k values.Kind, text, source string, prov values.Provenance) (values.Value, directorapi.ResultStatus, string) {
	v, err := values.New(k, text, source, values.Classify(k, text))
	if err != nil {
		return values.Value{}, directorapi.ResultFailed, err.Error()
	}
	// Provenance is attached at the read, by the code that PERFORMED it. Reconstructing
	// it later — inferring "must have used the value API" from the kind — would be a
	// second account of what happened, and would be confidently wrong on exactly the
	// runs where the ladder fell back.
	prov.Source = source
	return v.WithProvenance(prov), directorapi.ResultDone, ""
}

// secretWords are the labels that mean "do not read this".
//
// A conservative NAME-based rule, and it is worth being honest about why: nothing in
// the World Model carries a protected-control flag today, so the accessibility tree
// cannot say "this is a password field". Until it can, the label is the only evidence
// available, and the failure that matters is reading a credential rather than refusing
// an innocent field — so this list errs towards refusing.
var secretWords = []string{
	"password", "passwd", "passphrase", "passcode", "pin", "secret",
	"credential", "token", "api key", "apikey", "private key", "cvv", "security code",
}

// refuseSecretRead refuses to read a control that looks like it holds a credential.
//
// Refused rather than captured-and-redacted. A secret that has been read into a
// program-local value exists as plaintext in the Director's memory, can be written into
// another control, and would be lowered into generated Marco source — which is exactly
// what Marco's named secret mechanism exists to prevent. The safest version of this
// feature is the one that does not read it at all.
func refuseSecretRead(t directorapi.ResolvedTarget) (string, bool) {
	hay := strings.ToLower(t.Label)
	for _, w := range secretWords {
		if strings.Contains(hay, w) {
			return fmt.Sprintf(
				"refusing to read %s into a program-local value: it looks like a credential, "+
					"and credentials are typed by name through Marco's secret mechanism rather "+
					"than copied into memory as text", describeControl(t)), true
		}
	}
	return "", false
}

// controlProv builds the provenance for a read from a resolved control.
//
// The method and provider are passed in by the CALLER, because only the caller knows
// which rung of the ladder actually answered. Inferring it here from the value's kind
// would produce an explanation that is confidently wrong on exactly the runs where the
// ladder fell back — which are the runs worth explaining.
func (p *Pipeline) controlProv(t directorapi.ResolvedTarget, method, provider string) values.Provenance {
	return values.Provenance{
		SourceKind: values.SourceControlValue,
		Role:       string(t.Role),
		Method:     method,
		Provider:   provider,
		Confidence: t.Confidence,
		StepID:     p.stepID,
		StepIndex:  p.stepIndex,
	}
}

// describeControl names a control for an explanation, without identity.
func describeControl(t directorapi.ResolvedTarget) string {
	label := t.Label
	if label == "" {
		label = "(unlabelled)"
	}
	role := string(t.Role)
	if role == "" {
		role = "control"
	}
	return fmt.Sprintf("the %s %q", role, label)
}
