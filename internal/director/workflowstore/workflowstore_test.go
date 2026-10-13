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
//
// A workflow that bound what it could would plan its first steps and discover somewhere in the
// middle that it has no idea where it is. The refusal has to arrive before the first action, not
// at the first gap.
func TestAPartiallyBoundWorkflowIsRefusedEntirely(t *testing.T) {
	back := roundTrip(t)

	if _, err := back.Bind(workflowstore.Binding{}); err == nil {
		t.Fatal("an empty binding was accepted")
	}
	b := workflowstore.Binding{Screens: map[string]observe.VisualScreenID{}}
	for i, sc := range back.Screens {
		if i == 0 {
			continue // the one nobody has established
		}
		b.Screens[sc.Label] = "visual_9"
	}
	_, err := back.Bind(b)
	if err == nil {
		t.Fatal("a workflow bound while one of its screens was unaccounted for")
	}
	if !strings.Contains(err.Error(), back.Screens[0].Label) {
		t.Errorf("the refusal does not name what is missing: %v", err)
	}
}

// bindAll is a complete, arbitrary correspondence — what a caller who knows would supply.
func bindAll(a workflowstore.Artifact) workflowstore.Binding {
	b := workflowstore.Binding{Screens: map[string]observe.VisualScreenID{}}
	for i, sc := range a.Screens {
		b.Screens[sc.Label] = observe.VisualScreenID("visual_" + string(rune('1'+i)))
	}
	return b
}

// A COMPLETE BINDING PRODUCES A PLANNABLE — AND STILL UNCONFIRMED — WORKFLOW.
func TestBindingSaysWhereThingsAreAndNotThatAnybodyAgreed(t *testing.T) {
	back := roundTrip(t)
	d, err := back.Bind(bindAll(back))
	if err != nil {
		t.Fatal(err)
	}
	if d.Steps[0].From != "visual_1" {
		t.Errorf("step 0 starts from %q after binding", d.Steps[0].From)
	}
	// THE CONTROL KEEPS THE ARTIFACT'S LABEL, in this session and every other. Targets
	// are not bound: a control is re-acquired from its readings at the moment of acting,
	// and a bound identity would be a stale answer that still looks fresh.
	if d.Steps[0].Target != "control_1" {
		t.Errorf("step 0 presses %q after binding; a control's label is not a thing "+
			"binding translates", d.Steps[0].Target)
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

// THE STORED EVIDENCE REACHES THE MATCHER, SCOPED TO THE BOUND SCREEN.
func TestTargetsCarryTheirEvidenceToTheBoundScreen(t *testing.T) {
	back := roundTrip(t)
	got := back.Targets(bindAll(back))
	if len(got) != len(back.Controls) {
		t.Fatalf("%d stored target(s) for %d control(s)", len(got), len(back.Controls))
	}
	for _, st := range got {
		if !strings.HasPrefix(string(st.ID), "control_") {
			t.Errorf("a stored target is called %q", st.ID)
		}
		if !strings.HasPrefix(string(st.Screen), "visual_") {
			t.Errorf("%s is scoped to %q, which is not a screen of this session",
				st.ID, st.Screen)
		}
		if len(st.Readings) == 0 {
			t.Errorf("%s reached the matcher with no readings, so it could never "+
				"be found again", st.ID)
		}
	}
	// A CONTROL ON AN UNBOUND SCREEN IS LEFT OUT rather than scoped to a label. An
	// outcome on `screen_a` would be refused by the planner for a reason that reads
	// like the matcher failed.
	if n := len(back.Targets(workflowstore.Binding{})); n != 0 {
		t.Errorf("%d target(s) came back with nothing bound", n)
	}
}

// EVERY SCREEN GETS ITS OWN LABEL, IN THE ORDER THE WALK MET IT.
//
// Two screens sharing a label would collapse two places into one, and the artifact would
// describe a shorter route than the person walked — silently, because every field would still
// validate.
func TestEachScreenGetsItsOwnLabelInWalkOrder(t *testing.T) {
	d, _ := demo(t)
	got := workflowstore.Labels(d)
	seen := map[string]observe.VisualScreenID{}
	for id, label := range got {
		if other, clash := seen[label]; clash {
			t.Errorf("%s and %s were both called %s", id, other, label)
		}
		seen[label] = id
	}
	if len(got) != 3 {
		t.Fatalf("%d label(s) for 3 screens", len(got))
	}
	// visual_1 is the first screen the walk meets.
	if got["visual_1"] != "screen_a" {
		t.Errorf("the walk's first screen is %q, want screen_a", got["visual_1"])
	}
	// And Save agrees with Labels, because it IS Labels.
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Screens) != len(got) {
		t.Errorf("the artifact describes %d screen(s) for %d label(s)",
			len(a.Screens), len(got))
	}
	for i := 1; i < len(a.Screens); i++ {
		if a.Screens[i-1].Label >= a.Screens[i].Label {
			t.Errorf("the screens are not in label order: %s before %s",
				a.Screens[i-1].Label, a.Screens[i].Label)
		}
	}
}

// THE THREE OUTCOMES, AND WHY THE MIDDLE ONE IS NOT A TIE-BREAK.
func TestAProposedCorrespondenceIsValidatedOrRefused(t *testing.T) {
	back := roundTrip(t)
	// The artifact's own arrangements, so a candidate can be made to agree exactly.
	stored := map[string]observe.VisualArrangement{}
	for _, sc := range back.Screens {
		stored[sc.Label] = sc.Arrangement
	}
	labels := []string{back.Screens[0].Label, back.Screens[1].Label, back.Screens[2].Label}

	props := []workflowstore.Proposal{
		// ONE candidate that agrees -> bound.
		{Label: labels[0], Candidates: []workflowstore.ScreenCandidate{
			{ID: "visual_7", Arrangement: stored[labels[0]]},
		}},
		// TWO that agree -> ambiguous, however they score.
		{Label: labels[1], Candidates: []workflowstore.ScreenCandidate{
			{ID: "visual_8", Arrangement: stored[labels[1]]},
			{ID: "visual_9", Arrangement: stored[labels[1]]},
		}},
		// One that does not -> the caller's claim is REJECTED.
		{Label: labels[2], Candidates: []workflowstore.ScreenCandidate{
			{ID: "visual_6", Arrangement: observe.VisualArrangement{
				Cells: map[string]int{"8,8": 400}, Total: 400}},
		}},
	}
	want := map[string]workflowstore.BindResult{
		labels[0]: workflowstore.BindBound,
		labels[1]: workflowstore.BindAmbiguous,
		labels[2]: workflowstore.BindNoCounterpart,
	}
	got := back.BindScreens(props)
	if len(got) != len(back.Screens) {
		t.Fatalf("%d outcome(s) for %d screen(s)", len(got), len(back.Screens))
	}
	for _, o := range got {
		w, named := want[o.Label]
		if !named {
			// Every screen nobody proposed anything for.
			if o.Result != workflowstore.BindNoCounterpart {
				t.Errorf("%s: %q with nothing proposed", o.Label, o.Result)
			}
			continue
		}
		if o.Result != w {
			t.Errorf("%s: %q, want %q (scores %v, because %s)",
				o.Label, o.Result, w, o.Scores, o.Because)
		}
		if o.Result == workflowstore.BindBound && o.ID != "visual_7" {
			t.Errorf("%s bound to %q", o.Label, o.ID)
		}
		if o.Result != workflowstore.BindBound && o.ID != "" {
			t.Errorf("%s is %q and still names %q", o.Label, o.Result, o.ID)
		}
	}
	// AND A RUN OF OUTCOMES BINDS ONLY WHAT WAS BOUND.
	b := workflowstore.BindingFrom(got)
	if len(b.Screens) != 1 {
		t.Fatalf("%d screen(s) bound from one bound outcome", len(b.Screens))
	}
	if _, err := back.Bind(b); err == nil {
		t.Error("a workflow was bound from a run that refused most of its screens")
	}
}

// AN AMBIGUITY IS A REFUSAL EVEN WHEN ONE CANDIDATE IS PLAINLY BETTER.
//
// This is the measured rule and the one most likely to be "improved" away. Experiment-029's best
// WRONG binding scored 0.886 while correct ones scored as low as 0.556 — so a higher score is
// not evidence of being right, and taking the best of two qualifying candidates would be exactly
// the rule that produced two wrong bindings out of seven.
func TestABetterScoreIsNotATieBreak(t *testing.T) {
	back := roundTrip(t)
	exact := back.Screens[0].Arrangement
	// A second candidate that qualifies but agrees less well.
	weaker := observe.VisualArrangement{Cells: map[string]int{}, Total: 0}
	for k, v := range exact.Cells {
		weaker.Cells[k] = v
		weaker.Total += v
	}
	for k := range weaker.Cells {
		weaker.Cells[k] = weaker.Cells[k] * 3 / 4
		break
	}

	got := back.BindScreens([]workflowstore.Proposal{{
		Label: back.Screens[0].Label,
		Candidates: []workflowstore.ScreenCandidate{
			{ID: "visual_1", Arrangement: exact},
			{ID: "visual_2", Arrangement: weaker},
		},
	}})
	var o workflowstore.ScreenOutcome
	for _, c := range got {
		if c.Label == back.Screens[0].Label {
			o = c
		}
	}
	if o.Scores["visual_1"] <= o.Scores["visual_2"] {
		t.Fatalf("the fixture's candidates score %v; one must be plainly better",
			o.Scores)
	}
	if o.Result != workflowstore.BindAmbiguous {
		t.Errorf("%q with two qualifying candidates scoring %.3f and %.3f; a higher "+
			"score is not evidence of being right", o.Result,
			o.Scores["visual_1"], o.Scores["visual_2"])
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
		// A REAL OLDER VERSION, now that one exists. Version 1 had no
		// clarifications, and reading it as though it did would treat a control
		// nobody had answered about as one nobody needed to answer about.
		{"older", `"version": 1`, "older than this build"},
		{"none", `"version": 0`, "not a workflow artifact"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bad := bytes.Replace(b, []byte(`"version": 2`), []byte(c.v), 1)
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
	a.Version = 99
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
