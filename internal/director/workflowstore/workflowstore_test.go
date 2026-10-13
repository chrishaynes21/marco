package workflowstore_test

// WHAT A SAVED WORKFLOW MUST STILL REFUSE TO DO.
//
// Persisting a demonstration is the first thing in this subsystem that outlives the session that
// learned it, and every safety property it has is one that could be lost quietly. A workflow
// that came back executable would not crash; it would work, once, on the wrong screen.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func rect(x, y, w, h float64) observe.Region {
	return observe.Region{X: x, Y: y, Width: w, Height: h}
}

var saved = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// demo is a two-route demonstration: the same control pressed at two positions, a wait, and a
// step whose cause nothing explains.
func demo(t *testing.T) (workflow.Draft, map[observe.VisualScreenID]observe.VisualArrangement) {
	t.Helper()
	d := workflow.Draft{Name: "enter free play", Notes: []string{"demonstrated once"}}
	click := func(from, to observe.VisualScreenID, target observe.VisualTargetID,
		readings, containers []observe.Region) workflow.Step {
		return workflow.Step{
			From: from, To: to, Target: target, Cause: observe.CauseClick, Seen: 1,
			Readings: readings, Containers: containers,
		}
	}
	d.Steps = []workflow.Step{
		// TWO READINGS OF ONE CONTROL, and workflow.Learn hands the SAME set to
		// every step that presses it — so the merge below is handed duplicates,
		// which is the case that matters and the one a fixture with two
		// different readings never exercises.
		click("visual_1", "visual_2", "target_1",
			[]observe.Region{rect(0.1, 0.1, 0.2, 0.05), rect(0.101, 0.1, 0.2, 0.05)},
			[]observe.Region{rect(0, 0, 0.9, 0.3)}),
		{From: "visual_2", To: "visual_3", Cause: observe.CauseAutomatic, Seen: 1,
			Unresolved: []string{"a load completed and there is nothing to press"}},
		withProse(click("visual_3", "visual_2", "target_2",
			[]observe.Region{rect(0.5, 0.5, 0.1, 0.04)}, nil),
			// EXACTLY WHAT workflow.Learn WRITES. The identities are inside a
			// sentence meant for a person, which is how two of them reached
			// the first artifact saved from the real recording while every
			// structural field was clean.
			"this session could not tell target_2 from [target_10] on visual_3: "+
				"they led to the same place"),
		// THE SAME CONTROL AGAIN, at a different position, read slightly differently.
		click("visual_1", "visual_2", "target_1",
			[]observe.Region{rect(0.1, 0.1, 0.2, 0.05), rect(0.101, 0.1, 0.2, 0.05)},
			[]observe.Region{rect(0, 0, 0.9, 0.3)}),
	}
	if err := d.Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	if err := d.Confirm(2, "chris"); err != nil {
		t.Fatal(err)
	}
	arr := map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"1,1": 9, "0,2": 3}, Total: 12},
		"visual_2": {Cells: map[string]int{"2,2": 4}, Total: 4},
		"visual_3": {Cells: map[string]int{"0,0": 7}, Total: 7},
	}
	return d, arr
}

// withProse attaches the kind of sentence workflow.Learn writes about an ambiguity.
func withProse(s workflow.Step, text string) workflow.Step {
	s.Uncertain = true
	s.Unresolved = append(s.Unresolved, text)
	return s
}

func roundTrip(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d, arr := demo(t)
	a, err := workflowstore.Save(d, arr, saved)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

// ROUND TRIP: the route, its order, and the evidence all survive.
func TestASavedWorkflowComesBackWithItsRouteAndItsEvidence(t *testing.T) {
	d, _ := demo(t)
	back := roundTrip(t)

	if back.Name != d.Name || len(back.Steps) != len(d.Steps) {
		t.Fatalf("came back as %q with %d step(s), want %q with %d",
			back.Name, len(back.Steps), d.Name, len(d.Steps))
	}
	for i, st := range back.Steps {
		if st.Position != i {
			t.Errorf("step %d says it is at position %d", i, st.Position)
		}
		if st.Cause != d.Steps[i].Cause {
			t.Errorf("step %d came back as %s, want %s", i, st.Cause, d.Steps[i].Cause)
		}
	}
	// ONE CONTROL, not two, for the control pressed at two positions — and both
	// readings kept, because both are evidence about the same thing.
	if len(back.Controls) != 2 {
		t.Fatalf("%d control(s), want 2", len(back.Controls))
	}
	var first workflowstore.Control
	for _, c := range back.Controls {
		if c.Label == back.Steps[0].Control {
			first = c
		}
	}
	if len(first.Readings) != 2 {
		t.Errorf("%s kept %d reading(s) of a control pressed twice, want 2",
			first.Label, len(first.Readings))
	}
	if len(first.Containers) != 1 {
		t.Errorf("%s kept %d container(s), want 1 — the containers are what let a "+
			"later look say the panel is here and the button is not",
			first.Label, len(first.Containers))
	}
	// The arrangement is the screen's evidence, and it survives.
	for _, s := range back.Screens {
		if s.Arrangement.Total == 0 {
			t.Errorf("screen %s came back with no arrangement, so nothing could "+
				"ever be compared against it", s.Label)
		}
	}
	// The steps that said something unresolved still say it.
	if len(back.Steps[1].Unresolved) == 0 {
		t.Error("an unresolved step came back with nothing unresolved")
	}
}

// PROVENANCE: no session identity, and no press coordinate, reaches the file.
//
// The two things ADR-183 forbids by name. A `visual_3` in a durable file is the geometry-only
// promotion that is not authorised, and a stored press position would undo ADR-181's whole
// reason for keeping the correlator out of the planner's reach.
func TestNoSessionIdentityOrPressCoordinateReachesTheFile(t *testing.T) {
	d, arr := demo(t)
	a, err := workflowstore.Save(d, arr, saved)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"visual_", "target_"} {
		if bytes.Contains(b, []byte(forbidden)) {
			t.Errorf("the artifact contains %q — a session identity has escaped into "+
				"a durable file, where it means nothing and looks like it means "+
				"something", forbidden)
		}
	}
	// A press position would be a point, and the artifact has no field for one. Asserted
	// against the encoded form rather than the type, so adding one later fails here.
	var generic map[string]any
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"at", "press", "point", "x", "y"} {
		if _, ok := generic[key]; ok {
			t.Errorf("the artifact has a top-level %q", key)
		}
	}
}

// A SESSION IDENTITY INSIDE A SENTENCE IS STILL A SESSION IDENTITY.
//
// The structural fields were never the hard part. `Unresolved` is prose for a person and it
// quotes the identities it is about, so an artifact with perfectly clean labels carried
// `target_2` and `target_10` in a sentence — found by saving the real recording, not by this
// package's own fixtures, which is why one of them now writes what Learn writes.
//
// The ambiguity is the point of the sentence; the dead name is not. So a control the artifact
// describes becomes its label, and one it does not becomes a phrase.
func TestAnIdentityInsideASentenceIsRewrittenToo(t *testing.T) {
	back := roundTrip(t)

	var found string
	for _, st := range back.Steps {
		for _, u := range st.Unresolved {
			if strings.Contains(u, "could not tell") {
				found = u
			}
		}
	}
	if found == "" {
		t.Fatal("the fixture's ambiguity sentence did not survive at all")
	}
	if strings.Contains(found, "target_") || strings.Contains(found, "visual_") {
		t.Errorf("a session identity survived inside prose: %q", found)
	}
	// The control the artifact DOES describe is named by its label.
	if !strings.Contains(found, "control_") {
		t.Errorf("the sentence lost the control it was about: %q", found)
	}
	// The SCREEN it was about is a screen this artifact describes, so it is named
	// by its label rather than flattened to the phrase for a stranger.
	if !strings.Contains(found, "screen_") {
		t.Errorf("the sentence lost the screen it was about: %q", found)
	}
	// And the one it does not describe is still reported as an ambiguity.
	if !strings.Contains(found, "another control") {
		t.Errorf("the sentence lost the fact that something was ambiguous: %q", found)
	}
	// The whole file, as the final word.
	b, err := back.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"visual_", "target_"} {
		if bytes.Contains(b, []byte(forbidden)) {
			t.Errorf("the artifact still contains %q somewhere", forbidden)
		}
	}
}

// AUTHORITY: a saved workflow comes back unable to run, for two independent reasons.
func TestALoadedWorkflowIsNotExecutableAuthority(t *testing.T) {
	back := roundTrip(t)

	// The audit says somebody agreed. That is a record, and it is all it is.
	if by := back.Audit(); len(by) != 2 || by[0] != "chris" {
		t.Fatalf("the audit record did not survive: %v", by)
	}

	d := back.Draft()
	if d.Executable() {
		t.Error("a workflow read from a file is executable. Stored confirmation is " +
			"audit metadata and is never reusable authorization")
	}
	for i := range d.Steps {
		if by, ok := d.Steps[i].Confirmation(); ok {
			t.Errorf("step %d came back confirmed by %q", i, by)
		}
	}
	// AND THE SECOND, INDEPENDENT REASON: its screens are not identities any session
	// has. Even confirmed, planning refuses, because the screen standing now is not the
	// screen the step names.
	if err := d.Confirm(0, "somebody"); err != nil {
		t.Fatal(err)
	}
	if _, ref := workflow.Plan(d, 0, "visual_1", nil); ref == nil {
		t.Fatal("a loaded workflow planned a step against a session screen; its " +
			"labels are being mistaken for identities")
	}
}

// UNSUPPORTED BINDING: a partial decision binds nothing.
func TestAPartiallyBoundWorkflowIsRefusedEntirely(t *testing.T) {
	back := roundTrip(t)

	if _, err := back.Bind(workflowstore.Binding{}); err == nil {
		t.Fatal("an empty binding was accepted")
	}
	// Everything but one control.
	b := workflowstore.Binding{
		Screens:  map[string]observe.VisualScreenID{},
		Controls: map[string]observe.VisualTargetID{},
	}
	for i, s := range back.Screens {
		b.Screens[s.Label] = observe.VisualScreenID(
			[]string{"visual_9", "visual_8", "visual_7"}[i%3])
	}
	for i, c := range back.Controls {
		if i == 0 {
			continue
		}
		b.Controls[c.Label] = "target_9"
	}
	_, err := back.Bind(b)
	if err == nil {
		t.Fatal("a workflow bound while one of its controls was unaccounted for")
	}
	if !strings.Contains(err.Error(), back.Controls[0].Label) {
		t.Errorf("the refusal does not name what is missing: %v", err)
	}
}

// AND THE SAME WHEN IT IS A SCREEN THAT IS UNACCOUNTED FOR.
//
// A separate case because it is a separate branch: a test that bound every screen and omitted a
// control passed whether the screen half existed or not, which a mutation run found.
func TestAWorkflowWithAnUnboundScreenIsRefused(t *testing.T) {
	back := roundTrip(t)
	b := workflowstore.Binding{
		Screens:  map[string]observe.VisualScreenID{},
		Controls: map[string]observe.VisualTargetID{},
	}
	for i, s := range back.Screens {
		if i == 0 {
			continue // the one nobody has said anything about
		}
		b.Screens[s.Label] = "visual_9"
	}
	for _, c := range back.Controls {
		b.Controls[c.Label] = "target_9"
	}
	_, err := back.Bind(b)
	if err == nil {
		t.Fatal("a workflow bound while one of its screens was unaccounted for")
	}
	if !strings.Contains(err.Error(), back.Screens[0].Label) {
		t.Errorf("the refusal does not name the missing screen: %v", err)
	}
}

// A COMPLETE BINDING PRODUCES A PLANNABLE — AND STILL UNCONFIRMED — WORKFLOW.
func TestBindingSaysWhereThingsAreAndNotThatAnybodyAgreed(t *testing.T) {
	back := roundTrip(t)
	b := workflowstore.Binding{
		Screens:  map[string]observe.VisualScreenID{},
		Controls: map[string]observe.VisualTargetID{},
	}
	for i, s := range back.Screens {
		b.Screens[s.Label] = observe.VisualScreenID("visual_" + string(rune('1'+i)))
	}
	for i, c := range back.Controls {
		b.Controls[c.Label] = observe.VisualTargetID("target_" + string(rune('1'+i)))
	}
	d, err := back.Bind(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.Steps[0].From != "visual_1" {
		t.Errorf("step 0 starts from %q after binding", d.Steps[0].From)
	}
	if d.Steps[0].Target != "target_1" {
		t.Errorf("step 0 presses %q after binding", d.Steps[0].Target)
	}
	// The readings came with it — a bound workflow that could not re-acquire its
	// controls would be a route with no way to find anything on it.
	if len(d.Steps[0].Readings) != 2 {
		t.Errorf("step 0 carries %d reading(s) after binding", len(d.Steps[0].Readings))
	}
	// AND STILL NOBODY HAS AGREED.
	if d.Executable() {
		t.Error("binding made a workflow executable. Knowing where a control is is " +
			"not permission to press it")
	}
}

// VERSIONING: a file this build cannot vouch for is refused, in both directions.
func TestAnUnknownVersionIsRefusedRatherThanRead(t *testing.T) {
	d, arr := demo(t)
	a, _ := workflowstore.Save(d, arr, saved)
	b, _ := a.Marshal()

	for _, c := range []struct {
		name string
		v    string
		want string
	}{
		{"newer", `"version": 99`, "newer Marco"},
		{"older", `"version": 0`, "not a workflow artifact"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bad := bytes.Replace(b, []byte(`"version": 1`), []byte(c.v), 1)
			if bytes.Equal(bad, b) {
				t.Fatal("the fixture did not change the version")
			}
			_, err := workflowstore.Unmarshal(bad)
			if err == nil {
				t.Fatal("a version this build cannot vouch for was read anyway")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("refusal says %q, want it to mention %q", err, c.want)
			}
		})
	}
	// And this build will not WRITE a version it does not own.
	a.Version = 2
	if _, err := a.Marshal(); err == nil {
		t.Error("an artifact claiming another version was written")
	}
}

// CORRUPTION: a damaged file is refused, never half-read.
//
// A half-read workflow is a route missing moves the person made, and a planner handed one walks
// a shorter path confidently. That is the failure this whole subsystem exists to avoid,
// arriving through the file system.
func TestADamagedArtifactIsRefusedRatherThanRepaired(t *testing.T) {
	d, arr := demo(t)
	a, _ := workflowstore.Save(d, arr, saved)
	good, _ := a.Marshal()

	cases := map[string][]byte{
		"empty":     {},
		"truncated": good[:len(good)/2],
		"not json":  []byte("this is not a workflow"),
		"no steps":  []byte(`{"version":1,"name":"x","steps":[]}`),
		"no name":   []byte(`{"version":1,"steps":[{"position":0}]}`),
		"screen not described": []byte(
			`{"version":1,"name":"x","steps":[{"position":0,"from":"screen_z"}]}`),
		"control not described": []byte(
			`{"version":1,"name":"x","screens":[{"label":"screen_a"}],` +
				`"steps":[{"position":0,"from":"screen_a","control":"control_9"}]}`),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := workflowstore.Unmarshal(body)
			if err == nil {
				t.Fatalf("read a damaged artifact as %q with %d step(s)",
					got.Name, len(got.Steps))
			}
			if got.Name != "" || len(got.Steps) > 0 {
				t.Errorf("a refused read still returned content: %+v", got)
			}
		})
	}
}

// A WORKFLOW WITH NO NAME IS NOT SAVED, because a person has to ask for it by one.
func TestAnUnnamedWorkflowIsNotSaved(t *testing.T) {
	if _, err := workflowstore.Save(workflow.Draft{}, nil, saved); err == nil {
		t.Error("an unnamed workflow was saved")
	}
}

// NOTHING HERE DECIDES THAT TWO SCREENS ARE THE SAME.
//
// Experiment-029 measured the available relation binding the wrong screen at 0.886 while binding
// right ones at 0.556, with no floor between them. Until something can abstain, a binder that
// chose would be choosing wrongly about a quarter of the time and saying nothing about it. So
// the artifact stores an arrangement as EVIDENCE and offers no way to act on it.
func TestTheStoreNeverDecidesThatTwoScreensAreTheSame(t *testing.T) {
	back := roundTrip(t)
	// Two screens with identical arrangements, which is exactly the case a matcher
	// would resolve silently. Binding still requires somebody to say which is which.
	if _, err := back.Bind(workflowstore.Binding{
		Screens: map[string]observe.VisualScreenID{back.Screens[0].Label: "visual_1"},
	}); err == nil {
		t.Fatal("the store bound a workflow from one screen and its own evidence")
	}
}
