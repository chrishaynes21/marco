// Package workflowstore persists what somebody demonstrated, and nothing else.
//
// # The three claims this exists to keep apart
//
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] names them, and the owner's
// sentence is the whole design:
//
//	Persistence shouldn't imply recognition, and recognition shouldn't imply authority.
//
//	Workflow        lives durably     "this is what you showed me"      licenses nothing
//	Binding         one session       "this is where I think I am"      licenses nothing
//	Authorisation   one action        "this press, now, here"           licenses a press
//
// Only the first is here. [[Experiment-029-binding-a-screen-across-two-independent-folds]]
// measured the second and found it not ready — five screens of eight bound correctly, two bound
// confidently to screens the other fold had never seen, and no threshold separates them — so
// nothing in this package compares an arrangement to anything or decides that two screens are
// the same. A caller that has made that decision can state it; this package will not make it.
//
// # Why the refusal is structural rather than a flag
//
// A saved workflow's screens and controls are labelled `screen_a`, `control_1` — scoped to the
// artifact. Those are not session identities and cannot accidentally be ones: no session ever
// mints a screen called `screen_a`, so a loaded workflow handed to `workflow.Plan` is refused
// because the screen standing now is not the screen the step names. It is not refused because a
// boolean says so. Deleting a guard cannot make a saved workflow runnable; the identities would
// still not match anything.
//
// Binding is therefore a rewrite of those labels into the current session's own identities, and
// it is the caller's decision, recorded as the caller's.
//
// # What a saved workflow may not contain
//
// Each for a reason already settled, and each asserted by a test:
//
//   - A PRESS COORDINATE. ADR-181 keeps the correlator out of the planner's reach so the
//     coordinates are out of scope rather than merely unused; a store that wrote them back
//     would undo that in one field.
//   - A SESSION SCREEN OR TARGET ID. `visual_3` means nothing tomorrow, and an id that escaped
//     into a durable file would be exactly the geometry-only promotion that is not authorised.
//   - A DURABLE PLACE, an entry identity, or any node in the durable graph. Saving a workflow
//     is not a claim about the world.
//   - REUSABLE AUTHORISATION. Who confirmed which step is written down, because an audit record
//     is a requirement; a step read back comes out UNCONFIRMED regardless.
package workflowstore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// Version is the artifact format.
//
// Checked on load and refused when unknown, in both directions. A file from a newer Marco
// describes evidence this build may not understand, and reading it optimistically would produce
// a workflow missing whatever the new version added — which is worse than refusing, because the
// shortfall is invisible.
// Version 2 added Clarifications. A version-1 file is refused rather than read without them:
// the field is where a person's answer about a control's identity lives, and a reader that
// ignored it would quietly match a control the person had said was something else.
// Version 3 added the SEMANTIC BODY — a step's verb and value, and a control's role and the
// three ways it can be addressed without a name. A version-2 reader given one of these files
// would see a walk between screens that do not exist and press nothing, which is why the
// refusal is on the version rather than on the fields.
const Version = 3

// Artifact is one durable workflow: inert, evidence-bearing, and unable to act.
type Artifact struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	// SavedAt is when, in UTC. Audit, never logic: nothing here expires or is preferred
	// for being newer.
	SavedAt time.Time `json:"saved_at"`
	// Screens are the artifact's own labels and the evidence behind each.
	Screens []Screen `json:"screens,omitempty"`
	// Controls are the artifact's own labels and the readings that might find each again.
	Controls []Control `json:"controls,omitempty"`
	Steps    []Step    `json:"steps"`
	Notes    []string  `json:"notes,omitempty"`
	// Clarifications are what people have said about controls the demonstration
	// could not settle. Evidence with provenance, never a rule: an answer is about
	// one control of this workflow and reaches nothing else.
	Clarifications []Answer `json:"clarifications,omitempty"`
	// Corrections are the steps a person took OUT of this procedure, and why.
	//
	// Provenance, not instruction: a corrected step is already absent from `Steps`, so a
	// reader that knows nothing about this field performs exactly the corrected
	// procedure. That is why it needed no version bump, and it is asserted rather than
	// assumed — see correction.go, which is also where the one way to make one lives.
	Corrections []Correction `json:"corrections,omitempty"`
	// CorrectedAt is when the procedure was last changed by a person, zero when it never
	// was. SEPARATE from SavedAt, which stays the instant the demonstration was saved
	// because it is the link to the raw record beside it.
	CorrectedAt time.Time `json:"corrected_at,omitzero"`
}

// Screen is a label and what the demonstration saw, never a name from the session.
type Screen struct {
	Label string `json:"label"`
	// Arrangement is the evidence: a count of rectangles in the segmenter's cells.
	//
	// Stored so a later session has something to compare, and explicitly NOT stored as a
	// claim that comparing it will work. Experiment-029 measured that it does not,
	// reliably, and the honest consequence is that this field is evidence waiting for a
	// mechanism rather than a mechanism.
	Arrangement observe.VisualArrangement `json:"arrangement"`
	// Text is what the screen was READ to say while it was being demonstrated:
	// the words that held still across readings, with the source that read them.
	//
	// Saved because a screen is not only a collection of rectangles, and because
	// the arrangement alone has been measured not to have the margin it needs.
	// On the Rocket League artifact three of 36 screen pairs sit within 0.05 of
	// joining and the closest two are separated by 0.010, while the words on those
	// same two agree at 0.231. See observe/screentext.go.
	//
	// A POINTER, so an artifact nobody read carries no `text` key at all rather
	// than an empty one. `omitempty` does nothing to a struct, and every artifact
	// on disk today was saved without this field — a file that suddenly grew
	// `"text":{}` on every screen would be noise in the one document the owner
	// reads to see what Marco thinks it is keeping.
	//
	// Nil is NOT the same as a screen with no words on it. Read through `Said`,
	// which answers unread text for nil: a reader that could not tell them apart
	// would treat every old route as a chain of silent screens, and a silent
	// screen is one that can never disagree with anything.
	Text *observe.ScreenText `json:"text,omitempty"`
}

// Said is what this screen was read to say, and unread text when nobody read it.
//
// The only way to ask. Reaching for `Text` directly means handling nil at every call
// site, and the one that forgets turns a missing reading into a panic or, worse, into a
// screen that said nothing.
func (s Screen) Said() observe.ScreenText {
	if s.Text == nil {
		return observe.ScreenText{}
	}
	return *s.Text
}

// Control is a label and every reading of the control, with the containers that are not it.
type Control struct {
	Label string `json:"label"`
	// Readings is every reading of the CONTROL, Containers the larger regions that also
	// contained the press.
	//
	// Containers are kept because the control/container ambiguity is what makes a click
	// target hard, and discarded evidence cannot later contradict a match.
	Readings   []observe.Region `json:"readings,omitempty"`
	Containers []observe.Region `json:"containers,omitempty"`
	// Text is what the demonstration read ON this control, and it is the strongest
	// evidence of identity the artifact can carry.
	//
	// # Why it is empty in every artifact recorded so far
	//
	// Because no demonstration writes it yet. `Label` is a generated name — `control_1`
	// — and the measured artifact records no text, no label and no container, so the one
	// human-supplied meaning in it is the workflow's own NAME. That is what
	// [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] matches against, and
	// it is weak on purpose: a workflow agreeing with itself is not somebody pointing.
	//
	// The field exists ahead of the recorder because the PREFERENCE is a rule, not a
	// convenience — asked for in `ExpectedWords`, reported with its provenance wherever
	// it decides anything, and held by a test on both branches. A fallback that nothing
	// could ever prefer over would quietly become the only rule there is.
	Text []string `json:"text,omitempty"`
	// Role names this control by what it MEANS — the rename command, the confirm
	// button — and is the strongest identity evidence of all, stronger than Text,
	// because it is the only one that survives a change of language.
	//
	// Empty wherever nothing named the control by meaning, which is most of a settings
	// pane and all of a game menu. Then Text is what there is, and ADR-227 says why
	// that is the honest answer rather than a shortfall.
	Role string `json:"role,omitempty"`
	// Deictic marks the control the DEMONSTRATION pointed at rather than named — "this
	// file". Carried so that replay refuses to guess: a saved step that acted on
	// whatever the person was looking at cannot be replayed from the artifact alone,
	// and saying so is the whole of what this field does.
	Deictic bool `json:"deictic,omitempty"`
	// Editor marks the inline editor a previous step opened on the object a previous
	// step acted on — the in-place rename box, resolved at run time and never searched
	// for by name.
	Editor bool `json:"editor,omitempty"`
	// Focused marks a step addressed to the focused context, which is what a verb that
	// names no control — confirm, undo — is aimed at.
	Focused bool `json:"focused,omitempty"`
}

// Step is one demonstrated move, at its position in the walk.
type Step struct {
	// Position in the walk. Stored rather than implied by array order, because the same
	// control at two positions carries different expectations and a reader comparing two
	// artifacts should not have to count.
	Position int    `json:"position"`
	From     string `json:"from"`
	To       string `json:"to"`
	// Control is the artifact's label for what was pressed, empty for every other cause.
	Control string                  `json:"control,omitempty"`
	Cause   observe.TransitionCause `json:"cause"`
	Seen    int                     `json:"seen"`
	// Verb is WHAT THIS STEP DID, as a semantic action: open, select, invoke, confirm,
	// focus, edit.
	//
	// # Why a visual walk never had one
	//
	// Because it could not. A recorded demonstration over frames observes a press and a
	// screen change, and `Cause` is the strongest statement available about why the
	// screen changed — not about what the person meant by pressing. A Director
	// demonstration knows, because the step WAS a semantic action that Marco performed
	// and verified, so the verb is recorded rather than inferred.
	//
	// Empty for every artifact a frame recording produced, and those keep working: a
	// step with no verb is a transition, which is what they always were.
	Verb string `json:"verb,omitempty"`
	// Value is the text an edit step set, and is empty for every other verb.
	//
	// Never a secret. `Unsafe` refuses a demonstration containing one before extraction
	// begins, and `FromDemonstration` refuses a sensitive step again here rather than
	// trusting that — a password written into a durable file is the one mistake in this
	// package that cannot be taken back.
	Value string `json:"value,omitempty"`
	// Says is the step's own sentence, for the plan line and the trace.
	//
	// Provenance, not instruction: nothing parses it back. It exists because a person
	// reading this file has to be able to see what they showed Marco without decoding
	// `control_2` and a verb.
	Says string `json:"says,omitempty"`
	// Uncertain says the session could not tell this control from another on the same
	// screen, and Unresolved says what in words a person can answer.
	Uncertain  bool     `json:"uncertain,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
	// ConfirmedBy is AUDIT. It records that somebody agreed, and it is never read back as
	// agreement — see Draft.
	ConfirmedBy string `json:"confirmed_by,omitempty"`
}

// screenLabel and controlLabel are the artifact's own vocabulary.
//
// Deliberately unlike anything a session mints. `visual_3` and `screen_a` must not be mistakable
// for one another by a human reading a file or by code comparing two strings.
func screenLabel(i int) string  { return fmt.Sprintf("screen_%c", 'a'+rune(i%26)) }
func controlLabel(i int) string { return fmt.Sprintf("control_%d", i+1) }

// Labels is which artifact label each session screen was given.
//
// SAVE-TIME ONLY, and never stored. A caller that still has the session it saved from sometimes
// needs to know which label a screen became — an offline experiment binding a saved workflow
// against a second, independent fold needs exactly this to work out which screen of the second
// fold corresponds to which label of the first.
//
// It is a function of the draft alone, and it is the same function `Save` uses, so there is no
// way for the two to disagree about which screen became `screen_c`. Labels are allocated in the
// order the walk meets them, so reading the file top to bottom is reading the route in the order
// it was demonstrated.
func Labels(d workflow.Draft) map[observe.VisualScreenID]string {
	out := map[observe.VisualScreenID]string{}
	for _, s := range d.Steps {
		for _, id := range []observe.VisualScreenID{s.From, s.To} {
			if id == "" {
				continue
			}
			if _, ok := out[id]; !ok {
				out[id] = screenLabel(len(out))
			}
		}
	}
	return out
}

// ControlLabels is which artifact label each session control was given.
//
// The companion to `Labels`, for the same reason and with the same rule: SAVE-TIME ONLY, a
// function of the draft alone, and the same allocation `SaveWith` performs — so the two cannot
// disagree about which target became `control_2`.
//
// It exists because the RAW DEMONSTRATION beside a saved routine records each press by the
// routine's own label rather than by a session identity, which is what keeps
// `TestAVisualScreenCannotReachDurableMemory` true of a file that nevertheless records every
// press. See evidence.go.
//
// Allocated in step order over the steps that HAVE a target, which is exactly what `SaveWith`
// does: an automatic transition mints no control, so it must not consume a number either.
func ControlLabels(d workflow.Draft) map[observe.VisualTargetID]string {
	out := map[observe.VisualTargetID]string{}
	for _, s := range d.Steps {
		if s.Target == "" {
			continue
		}
		if _, ok := out[s.Target]; !ok {
			out[s.Target] = controlLabel(len(out))
		}
	}
	return out
}

// Save turns a session's draft into a durable artifact.
//
// `arrangements` is the evidence for each screen the draft names. A screen with no arrangement
// is still saved — the workflow is what somebody demonstrated and dropping a step because its
// evidence is thin would silently shorten the route they taught — but it is saved as a screen
// nothing can ever bind, which the artifact says plainly rather than hiding.
//
// # Nothing in the tree calls this any more, and it is kept on purpose
//
// Every save path goes through `SaveWith`, because
// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured what an artifact
// without screen text cannot do: a destination nobody read cannot be corroborated by a reading,
// and the arrangement alone establishes the wrong page often enough to matter. A mode that has
// the readings in hand and saves them away was the defect, not a convenience.
//
// This remains as the statement of one contract — an artifact with no text is byte-identical to
// one saved before the field existed — which `TestSaveWithoutTextWritesNoTextAtAll` holds by
// comparing the two. A caller with no readings to offer should still use `SaveWith` with nil,
// which says so explicitly.
func Save(d workflow.Draft, arrangements map[observe.VisualScreenID]observe.VisualArrangement,
	at time.Time) (Artifact, error) {

	return SaveWith(d, arrangements, nil, at)
}

// SaveWith is Save, with what each screen was read to say.
//
// A separate entry point rather than a fourth parameter on Save, because most callers
// have no text to offer and a nil map threaded through all of them would say nothing
// about whether this evidence was collected. A screen missing from `texts` is saved
// with unread text, which is the honest record: nobody read it.
func SaveWith(d workflow.Draft,
	arrangements map[observe.VisualScreenID]observe.VisualArrangement,
	texts map[observe.VisualScreenID]observe.ScreenText,
	at time.Time) (Artifact, error) {

	if d.Name == "" {
		return Artifact{}, fmt.Errorf("workflowstore: a workflow needs a name, " +
			"because the person has to be able to ask for it by one")
	}
	a := Artifact{Version: Version, Name: d.Name, SavedAt: at.UTC(), Notes: d.Notes}

	// ── THE LABELS, allocated in the order the walk meets them ──
	//
	// In walk order rather than sorted, so reading the file top to bottom is reading the
	// route in the order it was demonstrated.
	screens := Labels(d)
	controls := map[observe.VisualTargetID]string{}
	// The screens in label order, so the file reads as the route was walked.
	byLabel := make([]observe.VisualScreenID, 0, len(screens))
	for id := range screens {
		byLabel = append(byLabel, id)
	}
	sort.Slice(byLabel, func(i, j int) bool { return screens[byLabel[i]] < screens[byLabel[j]] })
	for _, id := range byLabel {
		sc := Screen{Label: screens[id], Arrangement: arrangements[id]}
		// ONLY WHEN SOMEBODY READ IT. An unread screen carries no key, so an
		// artifact saved with no text source is byte-identical to one saved
		// before this field existed.
		if t, ok := texts[id]; ok && t.Read() {
			said := t
			sc.Text = &said
		}
		a.Screens = append(a.Screens, sc)
	}
	label := func(id observe.VisualScreenID) string { return screens[id] }

	// BY INDEX, NOT BY POINTER. Holding a *Control into a slice that is still being
	// appended to is a pointer into an array that `append` may replace: the merge
	// for the first control then writes into the old backing array and the saved
	// artifact keeps one reading of a control that was pressed twice. It does not
	// fail — it silently saves less evidence than the demonstration produced.
	slot := map[observe.VisualTargetID]int{}
	for i, s := range d.Steps {
		st := Step{
			Position: i, From: label(s.From), To: label(s.To),
			Cause: s.Cause, Seen: s.Seen,
			Uncertain: s.Uncertain, Unresolved: s.Unresolved,
		}
		if by, ok := s.Confirmation(); ok {
			st.ConfirmedBy = by
		}
		if s.Target != "" {
			cl, ok := controls[s.Target]
			if !ok {
				cl = controlLabel(len(controls))
				controls[s.Target] = cl
				a.Controls = append(a.Controls, Control{Label: cl})
				slot[s.Target] = len(a.Controls) - 1
			}
			st.Control = cl
			// THE EVIDENCE, MERGED ACROSS EVERY STEP THAT USED THIS CONTROL.
			// One control pressed at two positions was read twice, and both
			// readings are evidence about the same thing.
			c := &a.Controls[slot[s.Target]]
			c.Readings = mergeRegions(c.Readings, s.Readings)
			c.Containers = mergeRegions(c.Containers, s.Containers)
			// AND WHAT IT WAS READ TO SAY. The draft already holds only what
			// the presses AGREED on -- see observe/controllabel.go -- so this
			// copies rather than merges: two steps using one control carry the
			// same accumulated label, and a union here would quietly widen it.
			if len(s.Text) > 0 {
				c.Text = append([]string{}, s.Text...)
			}
		}
		a.Steps = append(a.Steps, st)
	}

	// THE PROSE LAST, once every label exists.
	//
	// A sentence can mention a screen the walk reaches later, so scrubbing as each
	// step was built would have left the early ones naming identities that had no
	// label yet — and those would have been scrubbed to the neutral phrase instead
	// of to the label they deserved. Cheap, and the ordering is the whole of it.
	a.Notes = scrubAll(a.Notes, screens, controls)
	for i := range a.Steps {
		a.Steps[i].Unresolved = scrubAll(a.Steps[i].Unresolved, screens, controls)
	}
	return a, nil
}

// sessionID matches a session-minted screen or target identity.
//
// `visual_3`, `target_12`. Anchored on the prefix and the digits because those are exactly what
// a session allocates, and nothing else in this subsystem is named that way.
var sessionID = regexp.MustCompile(`\b(visual|target)_[0-9]+\b`)

// scrub rewrites a sentence written for a session into one that survives the session.
//
// # The leak this exists to close, which a synthetic fixture could not show
//
// `Unresolved` is prose for a person: "this session could not tell target_2 from [target_10] on
// visual_2: they led to the same place". Written straight to a file that is meant to outlive the
// session, those identities are exactly what ADR-183 forbids — they mean nothing tomorrow and
// they look like they mean something. The first artifact saved from the real recording carried
// two of them, inside sentences, while every structural field was clean.
//
// Known identities become the artifact's own labels. An identity the artifact does NOT describe
// — a control the session thought this one might be confused with, which was never itself
// pressed — becomes a neutral phrase: the ambiguity is the point of the sentence and the dead
// name is not. A reader loses nothing they could have used.
func scrub(text string, screens map[observe.VisualScreenID]string,
	controls map[observe.VisualTargetID]string) string {

	return sessionID.ReplaceAllStringFunc(text, func(id string) string {
		if l, ok := screens[observe.VisualScreenID(id)]; ok {
			return l
		}
		if l, ok := controls[observe.VisualTargetID(id)]; ok {
			return l
		}
		if strings.HasPrefix(id, "visual_") {
			return "another screen this workflow does not name"
		}
		return "another control on that screen"
	})
}

// scrubAll rewrites every sentence in a list.
func scrubAll(text []string, screens map[observe.VisualScreenID]string,
	controls map[observe.VisualTargetID]string) []string {

	if len(text) == 0 {
		return nil
	}
	out := make([]string, 0, len(text))
	for _, t := range text {
		out = append(out, scrub(t, screens, controls))
	}
	return out
}

// mergeRegions appends the rectangles of `add` that are not already present.
func mergeRegions(into, add []observe.Region) []observe.Region {
	for _, r := range add {
		dup := false
		for _, have := range into {
			if have == r {
				dup = true
				break
			}
		}
		if !dup {
			into = append(into, r)
		}
	}
	return into
}

// Marshal writes the artifact.
func (a Artifact) Marshal() ([]byte, error) {
	if a.Version != Version {
		return nil, fmt.Errorf("workflowstore: refusing to write version %d; this "+
			"build writes %d", a.Version, Version)
	}
	return json.MarshalIndent(a, "", "  ")
}

// Unmarshal reads one, refusing anything it cannot vouch for.
//
// Corruption is refused rather than repaired. A half-read workflow is a route missing moves the
// person made, and a planner handed one would walk a shorter path confidently — the failure this
// whole subsystem is built to avoid, arriving through the file system.
func Unmarshal(b []byte) (Artifact, error) {
	var a Artifact
	if err := json.Unmarshal(b, &a); err != nil {
		return Artifact{}, fmt.Errorf("workflowstore: unreadable: %w", err)
	}
	switch {
	case a.Version == 0:
		return Artifact{}, fmt.Errorf("workflowstore: no version, so this is not a " +
			"workflow artifact this build can vouch for")
	case a.Version != Version:
		// ONE DECISION, TWO MESSAGES. Written as two cases first, and a
		// mutation run found the `older` one unreachable: while Version is 1,
		// the only smaller value is 0 and the case above takes it. Deleting it
		// outright would mean a future build accepting an older file through
		// the default branch, so the directions share one reachable test.
		which := "is older than this build's"
		why := "and no migration exists"
		if a.Version > Version {
			which = "was written by a newer Marco; this build reads"
			why = "and reading it anyway would drop whatever that " +
				"version added without saying so"
		}
		return Artifact{}, fmt.Errorf("workflowstore: version %d %s %d, %s",
			a.Version, which, Version, why)
	case a.Name == "":
		return Artifact{}, fmt.Errorf("workflowstore: the artifact has no name")
	case len(a.Steps) == 0:
		return Artifact{}, fmt.Errorf("workflowstore: %q has no steps, so there is "+
			"nothing it could be a record of", a.Name)
	}
	// EVERY LABEL A STEP NAMES MUST EXIST. A step pointing at a screen the file does not
	// describe is evidence that went missing in transit, and binding it later would bind
	// to nothing.
	known := map[string]bool{}
	for _, s := range a.Screens {
		known[s.Label] = true
	}
	ctrl := map[string]bool{}
	for _, c := range a.Controls {
		ctrl[c.Label] = true
	}
	for _, st := range a.Steps {
		for _, l := range []string{st.From, st.To} {
			if l != "" && !known[l] {
				return Artifact{}, fmt.Errorf("workflowstore: step %d names screen "+
					"%q, which this artifact does not describe", st.Position, l)
			}
		}
		if st.Control != "" && !ctrl[st.Control] {
			return Artifact{}, fmt.Errorf("workflowstore: step %d names control %q, "+
				"which this artifact does not describe", st.Position, st.Control)
		}
	}
	for _, cl := range a.Clarifications {
		if !ctrl[cl.Control] {
			return Artifact{}, fmt.Errorf("workflowstore: a clarification "+
				"names control %q, which this artifact does not describe",
				cl.Control)
		}
		if cl.By == "" {
			return Artifact{}, fmt.Errorf("workflowstore: a clarification "+
				"for %q has nobody attached to it, so it is "+
				"indistinguishable from a default", cl.Control)
		}
	}
	return a, nil
}

// ── binding ───────────────────────────────────────────────────────────────────

// BindResult is what became of one proposed correspondence.
type BindResult string

const (
	// BindBound: exactly one proposed candidate is supported by the stored evidence.
	BindBound BindResult = "bound"
	// BindAmbiguous: more than one is. A REFUSAL, and not a tie to be broken.
	BindAmbiguous BindResult = "ambiguous"
	// BindNoCounterpart: none is. Also a refusal, and a different one — "I cannot tell
	// which of these" and "none of these" send a reader to different places.
	BindNoCounterpart BindResult = "no-counterpart"
)

// ScreenCandidate is one screen a caller believes a saved screen might be now, with the
// evidence for it as observed NOW.
type ScreenCandidate struct {
	ID          observe.VisualScreenID
	Arrangement observe.VisualArrangement
}

// Proposal is a caller's claim about one saved screen, with every current screen it thinks
// could be the one.
//
// Offering several is the honest case and the one the outcome rules are written for: a caller
// that narrowed to its favourite before asking would have already made the decision this is
// supposed to examine.
type Proposal struct {
	Label      string
	Candidates []ScreenCandidate
}

// ScreenOutcome is one saved screen's verdict, with the numbers behind it.
type ScreenOutcome struct {
	Label  string
	Result BindResult
	// ID is the screen this is bound to, and is set only when bound.
	ID observe.VisualScreenID
	// Scores is every candidate's similarity to the stored arrangement, including the
	// ones that did not qualify. Reported because a binding nobody can see the losing
	// scores for is one nobody can argue with.
	Scores  map[observe.VisualScreenID]float64
	Because string
}

// BindScreens validates a caller's proposed correspondences against the stored evidence.
//
// # What this does, and the much smaller thing it can actually prove
//
// The caller says which current screen it believes a saved one is. This checks that claim
// against the arrangement the demonstration recorded, and answers one of three ways.
//
// **Agreement is necessary and not sufficient**, and that sentence is the whole design.
// [[Experiment-029-binding-a-screen-across-two-independent-folds]] measured this relation
// binding the wrong screen at 0.886 while binding right ones at 0.556 — so a candidate that
// agrees may still be the wrong screen, and this cannot tell. What it CAN do is reject a
// correspondence the evidence contradicts, and refuse when more than one candidate qualifies.
// That is validation, not recognition, and calling it recognition is the error this whole
// subsystem exists to avoid.
//
// # Why more than one qualifying candidate is a refusal rather than a ranking
//
// Because ranking is exactly what the measurement broke. The best wrong binding in
// Experiment-029 outscored four of the five correct ones, so "the highest score above the
// floor" is a rule with a measured error rate and no way to tell when it is wrong. Two
// qualifying candidates is therefore `ambiguous` however far apart they score. A caller that
// knows which is which can say so by proposing one.
func (a Artifact) BindScreens(props []Proposal) []ScreenOutcome {
	offered := map[string][]ScreenCandidate{}
	for _, p := range props {
		offered[p.Label] = append(offered[p.Label], p.Candidates...)
	}

	out := make([]ScreenOutcome, 0, len(a.Screens))
	for _, sc := range a.Screens {
		o := ScreenOutcome{Label: sc.Label, Scores: map[observe.VisualScreenID]float64{}}
		var qualified []ScreenCandidate
		for _, c := range offered[sc.Label] {
			sim := observe.ArrangementSimilarity(sc.Arrangement, c.Arrangement)
			o.Scores[c.ID] = sim
			if sim >= observe.VisualScreenMatch {
				qualified = append(qualified, c)
			}
		}
		switch {
		case len(offered[sc.Label]) == 0:
			o.Result = BindNoCounterpart
			o.Because = "nothing was proposed for this screen"
		case len(qualified) == 0:
			o.Result = BindNoCounterpart
			o.Because = fmt.Sprintf("%d candidate(s) were proposed and the stored "+
				"arrangement supports none of them at or above %.2f",
				len(offered[sc.Label]), observe.VisualScreenMatch)
		case len(qualified) > 1:
			// NOT A TIE TO BREAK. See the note above: the ordering of these
			// scores is the thing that was measured and found unreliable.
			o.Result = BindAmbiguous
			o.Because = fmt.Sprintf("%d candidates are supported by the stored "+
				"arrangement and nothing here chooses between them; the best "+
				"wrong binding ever measured outscored four correct ones",
				len(qualified))
		default:
			o.Result = BindBound
			o.ID = qualified[0].ID
			o.Because = fmt.Sprintf("the only candidate the stored arrangement "+
				"supports, at %.3f. Agreement is necessary and not sufficient: "+
				"this validates the caller's correspondence and does not "+
				"establish it", o.Scores[qualified[0].ID])
		}
		out = append(out, o)
	}
	return out
}

// Binding is which current screen each saved screen corresponds to.
//
// CONTROLS ARE NOT IN IT, and that is ADR-183's rule rather than an omission:
//
//	targets are not bound. Not at save time and not at bind time. A control is re-acquired
//	from the artifact's stored readings against the proposals of the look in front of
//	Marco, at the moment of acting.
//
// So a saved control keeps the artifact's own label in every session, and `Targets` hands its
// readings to the matcher. A bound coordinate would be a stale answer that still looks fresh.
type Binding struct {
	Screens map[string]observe.VisualScreenID
}

// BindingFrom collects the bound outcomes, and only those.
//
// An ambiguous or unsupported screen contributes nothing, so a binding assembled from a run of
// outcomes is as incomplete as the run was — and `Bind` then refuses it, naming what is
// missing. The refusal arrives at the end rather than being skipped at the start.
func BindingFrom(outcomes []ScreenOutcome) Binding {
	b := Binding{Screens: map[string]observe.VisualScreenID{}}
	for _, o := range outcomes {
		if o.Result == BindBound {
			b.Screens[o.Label] = o.ID
		}
	}
	return b
}

// Draft returns the workflow as the planner's own type, with the artifact's labels intact.
//
// UNBOUND AND UNCONFIRMED, both structurally. The screens are called `screen_a`, which no
// session ever mints, so `workflow.Plan` refuses every step because the screen standing now is
// not the screen the step names. And no step is confirmed, however much audit the file carries,
// so `Executable` is false and planning would refuse even if the names matched.
//
// Two independent reasons a loaded workflow cannot run. That is deliberate: either alone would
// be a single guard somebody could remove.
func (a Artifact) Draft() workflow.Draft {
	return a.draft(Binding{})
}

// Bind applies a caller's decision and returns a draft in the session's own screen identities.
//
// Refuses while any screen is unbound, naming what is missing. A partial binding is the
// dangerous case — a workflow that could plan its first three steps and then discover it has no
// idea where step four goes is one that acts before it finds out.
//
// The returned draft is STILL UNCONFIRMED. Binding says where things are; it does not say
// anybody agreed to press them.
func (a Artifact) Bind(b Binding) (workflow.Draft, error) {
	var missing []string
	for _, sc := range a.Screens {
		if b.Screens[sc.Label] == "" {
			missing = append(missing, sc.Label)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return workflow.Draft{}, fmt.Errorf("workflowstore: %q cannot be bound: "+
			"nothing has been established about %v. A workflow that bound what it "+
			"could and left the rest would act before discovering it was lost",
			a.Name, missing)
	}
	return a.draft(b), nil
}

// Targets is the stored evidence for every control, scoped to the screen it was demonstrated
// on, ready for the matcher.
//
// The control keeps the ARTIFACT's label in every session, so an outcome and the step that
// wants it name the same thing without anything having been bound. Only the screen is
// translated, and a control whose screen is unbound is left out: matching it would produce an
// outcome scoped to a screen nobody has established, and `workflow.Plan` would then refuse it
// for a reason that reads like a matcher failure.
func (a Artifact) Targets(b Binding) []observe.StoredTarget {
	byLabel := map[string]Control{}
	for _, c := range a.Controls {
		byLabel[c.Label] = c
	}
	seen := map[string]bool{}
	var out []observe.StoredTarget
	for _, st := range a.Steps {
		if st.Control == "" || seen[st.Control] {
			continue
		}
		id, ok := b.Screens[st.From]
		if !ok || id == "" {
			continue
		}
		seen[st.Control] = true
		// THE CLARIFIED EVIDENCE, which is the same evidence when nobody has
		// answered. A clarification changes which rectangle is the control and
		// which are the regions it must not be confused with; it changes
		// nothing else, and it cannot make a control exist.
		c, _ := a.clarified(byLabel[st.Control])
		out = append(out, observe.StoredTarget{
			ID:     observe.VisualTargetID(c.Label),
			Screen: id,
			// UNCERTAIN FROM THE STEP, because it is the step that recorded the
			// session's doubt about this control.
			Uncertain:  st.Uncertain,
			Readings:   append([]observe.Region{}, c.Readings...),
			Containers: append([]observe.Region{}, c.Containers...),
		})
	}
	return out
}

// draft builds the planner's type, translating SCREEN labels through whatever binding exists.
//
// A control's label is left exactly as it is, in every session. That is not an oversight: the
// matcher is handed the same label through `Targets`, so a step and the outcome for it name one
// another without anything having been bound, and there is no moment at which a control's
// identity could be decided by this package.
func (a Artifact) draft(b Binding) workflow.Draft {
	readings := map[string]Control{}
	for _, c := range a.Controls {
		readings[c.Label] = c
	}
	screen := func(label string) observe.VisualScreenID {
		if label == "" {
			return ""
		}
		if id, ok := b.Screens[label]; ok && id != "" {
			return id
		}
		return observe.VisualScreenID(label)
	}

	d := workflow.Draft{Name: a.Name, Notes: a.Notes}
	for _, st := range a.Steps {
		s := workflow.Step{
			From: screen(st.From), To: screen(st.To), Cause: st.Cause,
			Target: observe.VisualTargetID(st.Control), Seen: st.Seen,
			Uncertain: st.Uncertain, Unresolved: st.Unresolved,
		}
		if c, ok := readings[st.Control]; ok {
			cl, _ := a.clarified(c)
			s.Readings = append([]observe.Region{}, cl.Readings...)
			s.Containers = append([]observe.Region{}, cl.Containers...)
		}
		d.Steps = append(d.Steps, s)
	}
	return d
}

// Audit is who agreed to what, for a person to read. Never for a planner.
func (a Artifact) Audit() map[int]string {
	out := map[int]string{}
	for _, s := range a.Steps {
		if s.ConfirmedBy != "" {
			out[s.Position] = s.ConfirmedBy
		}
	}
	return out
}

// VisualTargets is every demonstrated control of this artifact, WITHOUT a screen binding.
//
// # Why there are two of these
//
// `Targets` answers "which controls are on the screens I have bound", and it skips a step whose
// source screen nothing bound — correctly, because the question it serves is scoped to a screen.
// This answers a different question:
//
//	I'd shift the question from "which previously seen screen does this frame resemble?" to
//	"can I find the particular control I was shown, in the expected local context, with enough
//	evidence to act?"
//
// [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] is why. Seven
// measures now fail to separate a correct screen binding from an incorrect one, so a path that
// must bind a screen before it may look for a control inherits an error rate nothing can detect.
// `observe.ReacquireStored` never needed the screen — it matches a control's readings against
// the proposals in front of it — and this is the view that lets it be asked.
//
// # What it does NOT claim
//
// Dropping the screen drops evidence, and that is the whole cost: a control found "somewhere"
// is a weaker claim than a control found on the screen it was demonstrated on, and this returns
// targets whose `Screen` is empty so nothing downstream can mistake one for the other.
// `Reacquisition.Screen` comes back empty too, and a caller that wants the screen as supporting
// context must bind it and say so.
//
// Enforced by TestVisualTargetsCarryNoScreen and TestVisualTargetsSurviveAnArtifactWithNoScreens.
func (a Artifact) VisualTargets() []observe.StoredTarget {
	byLabel := map[string]Control{}
	for _, c := range a.Controls {
		byLabel[c.Label] = c
	}
	seen := map[string]bool{}
	var out []observe.StoredTarget
	for _, st := range a.Steps {
		if st.Control == "" || seen[st.Control] {
			continue
		}
		c, ok := byLabel[st.Control]
		if !ok {
			continue
		}
		// A CONTROL WITH NO VISUAL EVIDENCE IS NOT A VISUAL TARGET. A desktop
		// demonstration's controls carry a role or a label and no readings at all, and
		// offering one here would be offering a target that can never match anything —
		// which reads as a reacquisition failure rather than as a control that was
		// never seen by the proposer.
		cl, _ := a.clarified(c)
		if len(cl.Readings) == 0 {
			continue
		}
		seen[st.Control] = true
		out = append(out, observe.StoredTarget{
			ID: observe.VisualTargetID(cl.Label),
			// NO SCREEN. See above: this is the screen-independent view, and an
			// empty Screen is how a reader can tell which question was asked.
			Uncertain:  st.Uncertain,
			Readings:   append([]observe.Region{}, cl.Readings...),
			Containers: append([]observe.Region{}, cl.Containers...),
		})
	}
	return out
}
