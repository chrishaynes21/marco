package demo

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The recorder.
//
//	Recorder subscribes to existing Director events. Do not add duplicate observation
//	paths.
//	Recording never bypasses verification.
//
// Both rules fall out of WHERE this sits. It is handed the Outcome of a request that has
// already been observed, planned, policy-checked, executed, re-observed, verified and
// recorded to the action graph. It observes nothing, decides nothing and touches nothing:
// it reads what the Director concluded and keeps the semantic part.
//
// That is also why an unverified action cannot slip into a demonstration as a step. There
// is no path from "something happened on the desktop" to this file — only from "an action
// was verified and became a node".

// Recorder holds the open demonstration, if there is one.
//
// One at a time, deliberately. Two concurrent demonstrations would each see the other's
// steps: the Director acts on one desktop, and there is no way to attribute an action to
// one session rather than the other.
type Recorder struct {
	mu     sync.Mutex
	active *Demonstration
	// Now is injectable so recorded sessions are deterministic under test.
	Now func() time.Time
	// OnComplete, when set, is called with each finished demonstration — the seam the
	// service uses to persist it. Called OUTSIDE the lock.
	OnComplete func(*Demonstration)
}

// NewRecorder returns a recorder with no session open.
func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// ErrRecording is returned when a second session is started over an open one.
var ErrRecording = fmt.Errorf("a demonstration is already being recorded")

// ErrNotRecording is returned when a session is ended and none is open.
var ErrNotRecording = fmt.Errorf("no demonstration is being recorded")

// Start opens a session.
func (r *Recorder) Start(id ID) (*Demonstration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		return nil, ErrRecording
	}
	r.active = &Demonstration{ID: id, Started: r.now(), Status: Recording}
	return r.snapshotLocked(), nil
}

// Recording reports whether a session is open.
func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active != nil
}

// Active returns a copy of the open session, nil when none is.
func (r *Recorder) Active() *Demonstration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// Stop ends the session and returns it.
//
// The status is decided HERE rather than at extraction time: a demonstration that
// contained a credential entry is REFUSED as it is closed, so the refusal is a durable
// fact about the session rather than a verdict something could be asked to re-reach with
// different rules later.
func (r *Recorder) Stop() (*Demonstration, error) {
	r.mu.Lock()
	if r.active == nil {
		r.mu.Unlock()
		return nil, ErrNotRecording
	}
	d := r.active
	d.Completed = r.now()
	d.Status = Completed
	if reason, unsafe := Unsafe(d); unsafe {
		d.Status, d.Refusal = Refused, reason
	}
	r.active = nil
	out := copyOf(d)
	r.mu.Unlock()

	if r.OnComplete != nil {
		r.OnComplete(out)
	}
	return out, nil
}

// Abandon ends the session without keeping it as learnable.
func (r *Recorder) Abandon(reason string) (*Demonstration, error) {
	r.mu.Lock()
	if r.active == nil {
		r.mu.Unlock()
		return nil, ErrNotRecording
	}
	d := r.active
	d.Completed = r.now()
	d.Status = Abandoned
	d.Refusal = reason
	r.active = nil
	out := copyOf(d)
	r.mu.Unlock()

	if r.OnComplete != nil {
		r.OnComplete(out)
	}
	return out, nil
}

// Observe records one completed request.
//
// The single subscription point, and it takes the whole Outcome rather than a stream of
// smaller events for a reason: a request is the unit the Director verifies and records,
// and half a request is not a thing that happened.
func (r *Recorder) Observe(out execute.Outcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return
	}
	r.observeLocked(out)
}

func (r *Recorder) observeLocked(out execute.Outcome) {
	d := r.active
	if req := strings.TrimSpace(out.Request); req != "" && !lastEquals(d.Requests, req) {
		d.Requests = append(d.Requests, req)
	}

	// A PROGRAM is its steps. Recorded individually because that is what they were: each
	// step observed, executed and verified on its own, and a procedure learned from the
	// program as a whole would be learned from something the Director never treated as a
	// unit.
	if out.Program != nil {
		for _, step := range out.Program.Steps {
			r.observeLocked(step)
		}
		r.noteProgram(d, out.Program)
		return
	}
	// A COLLECTION iterated over a bounded set. Each member's turn is an ordinary
	// outcome; the iteration itself is noted, because a procedure learned from one would
	// be a bulk operation the user demonstrated once — see Unsafe.
	if out.Collection != nil {
		note(d, fmt.Sprintf("a collection of %d member(s) was iterated (%q)",
			len(out.Collection.Iterations), out.Request))
	}
	if out.Replay != nil {
		note(d, fmt.Sprintf("an action was replayed %d time(s)", out.Replay.Completed))
	}
	if out.Status == directorapi.ResultNeedsClarification {
		note(d, "a step needed the user to say which control was meant: "+out.Message)
	}
	// What the confirmation gate concluded, from the gate itself. A request that had to
	// be agreed to cannot become a procedure — see Unsafe — and the fact is recorded here
	// rather than re-derived later from the words in a step's phrase.
	if b := out.Binding; b != nil && b.Confirmation != "" &&
		b.Confirmation != execute.ConfirmationNotRequired {
		d.Confirmed = appendUnique(d.Confirmed,
			fmt.Sprintf("%q was %s", out.Request, b.Confirmation))
	}

	step, ok := stepFrom(out)
	if !ok {
		// Nothing executed. Recorded as a NOTE rather than dropped: a request that failed
		// to resolve is exactly the kind of thing that makes a demonstration unlearnable,
		// and a reader needs to see it.
		if out.Status != directorapi.ResultDone && out.Request != "" {
			note(d, fmt.Sprintf("%q did not produce a verified action (%s)",
				out.Request, out.Status))
		}
		return
	}
	step.Index = len(d.Steps) + 1
	step.Clarified = out.Status == directorapi.ResultNeedsClarification
	d.Steps = append(d.Steps, step)
	d.Nodes = append(d.Nodes, step.Node)
	r.noteApplication(d, step.Application)
}

// noteProgram records what the program as a whole did that its steps do not say.
func (r *Recorder) noteProgram(d *Demonstration, p *execute.ProgramOutcome) {
	if p.Status != directorapi.ResultDone {
		note(d, fmt.Sprintf("a program ended %s: %s", p.Status, p.Message))
	}
	if p.Confirmation != "" && p.Confirmation != execute.ConfirmationNotRequired {
		d.Confirmed = appendUnique(d.Confirmed,
			fmt.Sprintf("the goal %q was %s", p.Program.Goal, p.Confirmation))
	}
	if p.Goal != nil {
		// Provenance only. Goal recovery reads the ACTIONS — see RecoverGoal — and this
		// exists so a reader can see whether the two agreed.
		note(d, fmt.Sprintf("the user asked for %s, carried out by %q",
			p.Goal.Goal.Kind.Describe(), p.Goal.Procedure))
	}
}

// noteApplication tracks whether the whole demonstration stayed in one application.
//
// An empty Application on the demonstration means it did NOT, which the extractor refuses:
// a procedure is registered for an application, and one that spanned two is either two
// procedures or a workflow, neither of which this milestone learns.
func (r *Recorder) noteApplication(d *Demonstration, app string) {
	if app == "" {
		return
	}
	switch {
	case len(d.Steps) == 1 || d.Application == "":
		if !d.mixedApps {
			d.Application = app
		}
	case !strings.EqualFold(d.Application, app):
		note(d, fmt.Sprintf("the demonstration moved from %s to %s", d.Application, app))
		d.Application, d.mixedApps = "", true
	}
}

// stepFrom turns one executed request into the semantic step a demonstration keeps.
//
// Returns false for anything that did not become a verified action: there is no such thing
// as a recorded step that did not happen.
func stepFrom(out execute.Outcome) (Step, bool) {
	if out.Node == nil || out.Record == nil {
		return Step{}, false
	}
	rec := out.Record
	s := Step{
		Node:        out.Node.ID,
		Kind:        kindOf(out.Intent),
		Semantic:    semanticOf(out.Intent),
		Target:      targetOf(out.Intent, rec),
		Verified:    rec.Success,
		Status:      rec.Status,
		Evidence:    evidenceKinds(rec.Verification),
		Application: out.Node.ResolvedTarget.App,
		Phrase:      firstNonEmpty(out.Intent.Raw, out.Request),
	}
	if p := out.Node.GoalProvenance; p != nil {
		s.Goal, s.Procedure = p.Goal, p.Procedure
	}
	if out.Plan != nil && len(out.Plan.Steps) > 0 {
		s.Preconditions = out.Plan.Steps[0].Expect
	}
	if s.Kind == StepEdit {
		s.Text, s.Sensitive, s.ValueRef = editedText(out)
	}
	return s, true
}

// kindOf reads what a request DID from its intent.
func kindOf(in directorapi.Intent) StepKind {
	switch in.Verb {
	case "edit":
		return StepEdit
	case "focus":
		return StepFocus
	case "remember", "capture":
		return StepCapture
	}
	return StepSemantic
}

// semanticOf reads the verb a step carried.
//
//	Extract intent, not clicks.
//
// A semantic request carries its verb explicitly. A CLICK does not — and is recorded as an
// INVOKE, because that is what clicking a control means. It is not a reinterpretation: the
// capability ladder's lowest rung for invoke is a click, so the two are the same act
// described at two levels, and the level a procedure is made of is this one.
//
// The alternative would be to record "click" and let a learned procedure lower it back to
// a click every time — which would throw away the ladder, and with it the control's own
// Invoke pattern on every machine where one exists.
func semanticOf(in directorapi.Intent) directorapi.SemanticActionKind {
	if kind, ok := in.Parameters[intent.SemanticKindParam].(string); ok && kind != "" {
		return directorapi.SemanticActionKind(kind)
	}
	if in.Verb == "click" {
		return directorapi.SemanticInvoke
	}
	return ""
}

// targetOf builds the SEMANTIC description of what a step acted on.
//
// From the reference the request carried and the control that was resolved — never from
// the resolved target's identity, which is the handle half and is deliberately left in the
// action graph where it belongs.
func targetOf(in directorapi.Intent, rec *directorapi.ActionRecord) Target {
	t := Target{
		Label:       rec.Target.Label,
		ElementRole: rec.Target.Role,
	}
	if len(in.Targets) > 0 {
		ref := in.Targets[0]
		t.Phrase = ref.Phrase
		t.Deictic = ref.Kind == directorapi.ReferenceDeictic || ref.RequiresBinding
		t.Anaphoric = ref.Kind == directorapi.ReferenceAnaphoric
		t.DerivedEditor = ref.RequiresEditor
	}
	// The semantic role of the control, recovered from its label. This is the part that
	// makes a learned procedure portable: "the control that renames" replays on a machine
	// in another language, and "Umbenennen" does not.
	if !t.DerivedEditor && !t.Deictic {
		t.Role = goal.RoleForLabel(rec.Target.Label)
	}
	return t
}

// editedText reports what an edit step wrote, and whether it may be kept.
//
//	Never capture plaintext sensitive values.
//
// A value the value layer classified as anything but normal is recorded as HAVING BEEN
// sensitive and never as its content — and a step that read a program-local value records
// the reference, because a procedure that consumed one cannot be learned at all.
func editedText(out execute.Outcome) (text string, sensitive bool, ref string) {
	for _, st := range out.Stages {
		if st.Name != "value" {
			continue
		}
		// The value layer renders its own description, which is already redacted for
		// anything not public — see describeValueUse. What is needed here is only whether
		// a value was involved and under what name.
		ref = valueName(st.Detail)
		if strings.Contains(st.Detail, "sensitive") || strings.Contains(st.Detail, "secret") {
			return "", true, ref
		}
	}
	if isSensitiveField(out) {
		return "", true, ref
	}
	return out.Intent.Text, false, ref
}

// valueName pulls a ${name} reference out of a value stage's description.
func valueName(detail string) string {
	i := strings.Index(detail, "${")
	if i < 0 {
		return ""
	}
	rest := detail[i+2:]
	j := strings.Index(rest, "}")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// sensitiveFieldWords are the control labels that mean "do not keep what was typed here".
//
// A closed, reviewable list rather than a heuristic. It errs toward refusing: a field
// called "PIN code" that was not a credential costs the user one refused demonstration,
// and the reverse costs them a password in a file on disk.
var sensitiveFieldWords = []string{
	"password", "passcode", "passphrase", "pass phrase", "pin", "secret", "token",
	"api key", "apikey", "credential", "security code", "cvv", "cvc", "card number",
	"cardnumber", "account number", "sort code", "iban", "ssn", "social security",
	"one-time", "one time code", "otp", "verification code", "auth code", "2fa",
	"mfa code",
}

// isSensitiveField reports whether a step typed into something that should not be kept.
//
// From the control's LABEL and the phrase that named it, which is all a verified action
// record carries: the Director's element model has no password-field role, and a provider
// that reports one would be the better signal the day there is one. Until then this errs
// toward refusing.
func isSensitiveField(out execute.Outcome) bool {
	if out.Record == nil {
		return false
	}
	label := strings.ToLower(out.Record.Target.Label)
	phrase := ""
	if len(out.Intent.Targets) > 0 {
		phrase = strings.ToLower(out.Intent.Targets[0].Phrase)
	}
	for _, w := range sensitiveFieldWords {
		if strings.Contains(label, w) || strings.Contains(phrase, w) {
			return true
		}
	}
	return false
}

// evidenceKinds is what verification used, by KIND only.
//
// The details carry labels, values and window titles — the user's own content — and a
// demonstration is a durable file. The kinds are what an extraction decision needs
// ("focus_changed", "inline_editor_value"), and they carry nothing of the desktop.
func evidenceKinds(v directorapi.VerificationResult) []string {
	out := make([]string, 0, len(v.Evidence))
	for _, e := range v.Evidence {
		if e.Observed {
			out = append(out, e.Kind)
		}
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func note(d *Demonstration, s string) {
	if !lastEquals(d.Notes, s) {
		d.Notes = append(d.Notes, s)
	}
}

func lastEquals(list []string, s string) bool {
	return len(list) > 0 && list[len(list)-1] == s
}

// appendUnique adds a line once. A program and its steps both report the confirmation
// that covered them, and the same fact twice reads as two facts.
func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// snapshotLocked returns a copy of the open session.
func (r *Recorder) snapshotLocked() *Demonstration {
	if r.active == nil {
		return nil
	}
	return copyOf(r.active)
}

// copyOf deep-copies a demonstration, so a caller holding one cannot see it change.
func copyOf(d *Demonstration) *Demonstration {
	if d == nil {
		return nil
	}
	out := *d
	out.Requests = append([]string{}, d.Requests...)
	out.Confirmed = append([]string{}, d.Confirmed...)
	out.Notes = append([]string{}, d.Notes...)
	out.Nodes = append([]actiongraph.NodeID{}, d.Nodes...)
	out.Steps = make([]Step, len(d.Steps))
	copy(out.Steps, d.Steps)
	for i := range out.Steps {
		out.Steps[i].Evidence = append([]string{}, d.Steps[i].Evidence...)
		out.Steps[i].Preconditions = append([]directorapi.Condition{},
			d.Steps[i].Preconditions...)
	}
	return &out
}
