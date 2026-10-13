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
const Version = 1

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

// Save turns a session's draft into a durable artifact.
//
// `arrangements` is the evidence for each screen the draft names. A screen with no arrangement
// is still saved — the workflow is what somebody demonstrated and dropping a step because its
// evidence is thin would silently shorten the route they taught — but it is saved as a screen
// nothing can ever bind, which the artifact says plainly rather than hiding.
func Save(d workflow.Draft, arrangements map[observe.VisualScreenID]observe.VisualArrangement,
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
	screens := map[observe.VisualScreenID]string{}
	controls := map[observe.VisualTargetID]string{}
	label := func(id observe.VisualScreenID) string {
		if id == "" {
			return ""
		}
		if l, ok := screens[id]; ok {
			return l
		}
		l := screenLabel(len(screens))
		screens[id] = l
		a.Screens = append(a.Screens, Screen{Label: l, Arrangement: arrangements[id]})
		return l
	}

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
	return a, nil
}

// ── binding ───────────────────────────────────────────────────────────────────

// Binding is a caller's decision about what the artifact's labels correspond to NOW.
//
// A decision, and recorded as one. Nothing in this package compares an arrangement, scores a
// similarity or picks a best match: Experiment-029 measured that the available relation binds
// the wrong screen at 0.886 while binding right ones at 0.556, and that no floor separates them.
// Until something can abstain, the only honest binder is somebody who knows.
type Binding struct {
	Screens  map[string]observe.VisualScreenID
	Controls map[string]observe.VisualTargetID
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

// Bind applies a caller's decision and returns a draft in the session's own identities.
//
// Refuses while anything is unbound, naming what is missing. A partial binding is the dangerous
// case — a workflow that could plan its first three steps and then discover it has no idea where
// step four goes is one that acts before it finds out.
//
// The returned draft is STILL UNCONFIRMED. Binding says where things are; it does not say
// anybody agreed to press them.
func (a Artifact) Bind(b Binding) (workflow.Draft, error) {
	var missing []string
	for _, s := range a.Screens {
		if b.Screens[s.Label] == "" {
			missing = append(missing, s.Label)
		}
	}
	for _, c := range a.Controls {
		if b.Controls[c.Label] == "" {
			missing = append(missing, c.Label)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return workflow.Draft{}, fmt.Errorf("workflowstore: %q cannot be bound: "+
			"nothing has been said about %v. A workflow that bound what it could "+
			"and left the rest would act before discovering it was lost",
			a.Name, missing)
	}
	return a.draft(b), nil
}

// draft builds the planner's type, translating labels through whatever binding exists.
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
	control := func(label string) observe.VisualTargetID {
		if label == "" {
			return ""
		}
		if id, ok := b.Controls[label]; ok && id != "" {
			return id
		}
		return observe.VisualTargetID(label)
	}

	d := workflow.Draft{Name: a.Name, Notes: a.Notes}
	for _, st := range a.Steps {
		s := workflow.Step{
			From: screen(st.From), To: screen(st.To), Cause: st.Cause,
			Target: control(st.Control), Seen: st.Seen,
			Uncertain: st.Uncertain, Unresolved: st.Unresolved,
		}
		if c, ok := readings[st.Control]; ok {
			s.Readings = append([]observe.Region{}, c.Readings...)
			s.Containers = append([]observe.Region{}, c.Containers...)
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
