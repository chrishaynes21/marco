package observe_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// Pointing at the right thing, or saying plainly that it cannot.
//
// The rule under all of these: a highlight is a claim, and a wrong one is worse than none. A
// person shown a box believes Marco means what is inside it, so every path here either points at
// exactly the subject the question is about or points at nothing and says why.

// watching runs two screens past the tracker and returns what is on screen now.
//
// Built from the real fixtures through the real tracker rather than by hand, so the tracks, the
// states and the groups are the ones production would form.
func watching(t *testing.T) (observe.LiveGeometry, observe.StructuralGroup, observe.StructuralGroup) {
	t.Helper()
	var k observe.ShadowTracker
	show := func(regions []observe.ShadowRegion, times int) {
		for range times {
			k.Observe(&observe.ShadowSample{Ran: true, TargetProven: true, Regions: regions},
				observe.StructuralView{Source: observe.StructureFused, Regions: regions})
		}
	}
	for range 2 {
		show(screenfixture.Editor(), 6)
		show(screenfixture.Settings(), 10)
	}
	states, tracks := k.States(), k.Tracks()
	live := observe.LiveGeometry{
		Application: "testgame", Window: "hwnd:100", AtInference: 32,
		Tracks: tracks, States: states, Reliable: true,
	}
	// The session ends on the settings screen, so the editor.s group is a real subject that
	// is genuinely not visible any more -- which is the ordinary case for a question somebody
	// has navigated away from, and it costs this fixture nothing to produce it honestly.
	var onScreen, offScreen observe.StructuralGroup
	for _, g := range observe.Groups(tracks, states) {
		if len(regionsFor(t, g, live)) > 0 {
			onScreen = g
			continue
		}
		offScreen = g
	}
	if onScreen.ID == "" || offScreen.ID == "" {
		t.Skip("the fixture did not produce both a visible and a departed group")
	}
	return live, onScreen, offScreen
}

// asking is a question about one structural group.
func asking(group string) observe.Proposal {
	return observe.Proposal{
		ID: observe.ProposalID("q_" + group), Kind: observe.PossibleChoiceGroup,
		Status:  observe.ProposalOpen,
		Subject: observe.Subject{Kind: observe.SubjectGroup, Ref: group},
	}
}

// THE binding test. A question points at ITS subject, never at whatever is current.
func TestAQuestionPointsAtItsOwnSubjectAndNotAtWhateverIsCurrent(t *testing.T) {
	live, first, second := watching(t)

	got := observe.ReferentFor(asking(first.ID), observe.ReferentQuestion, live)
	if !got.CanPoint() {
		t.Fatalf("nothing to point at for %s: %q", first.ID, got.Unavailable)
	}
	allDrawnFrom(t, got.Regions, regionsFor(t, first, live), first.ID)
	// And it is not the other group's, which is what a "use whatever is current" mistake
	// would produce.
	if sharesAnything(got.Regions, regionsFor(t, second, live)) {
		t.Error("the question pointed at a different group. A question carries the identity " +
			"of what it is about, and that is the whole value of pointing at it")
	}
}

// A set of choices is several controls, and stays several.
func TestAReferentGivesEveryMemberItsOwnRegion(t *testing.T) {
	live, g, _ := watching(t)
	got := observe.ReferentFor(asking(g.ID), observe.ReferentQuestion, live)
	if !got.CanPoint() {
		t.Fatalf("nothing to point at: %q", got.Unavailable)
	}
	if len(got.Regions) < 2 {
		t.Fatalf("a group of %d members produced %d region(s)",
			len(g.Members), len(got.Regions))
	}
	// None of them is the group's envelope. A single box around the set would claim that
	// whatever sits between the members belongs to it.
	for _, r := range got.Regions {
		if r.Width >= g.Envelope.Width && r.Height >= g.Envelope.Height &&
			g.Envelope.Height > 0 {
			t.Error("a region as large as the whole group was drawn; that box contains " +
				"structures nobody said were part of the set")
		}
	}
}

// A subject that is not on screen is not pointed at, and the reason is a sentence.
func TestASubjectThatIsNotOnScreenCannotBePointedAt(t *testing.T) {
	live, _, _ := watching(t)
	got := observe.ReferentFor(asking("group_that_is_not_here"), observe.ReferentQuestion, live)
	if got.CanPoint() {
		t.Fatal("Marco offered to point at a group that is not on screen")
	}
	if len(got.Regions) != 0 {
		t.Errorf("an unavailable referent carries %d region(s)", len(got.Regions))
	}
	if got.Unavailable != observe.ReferentNotOnScreen {
		t.Errorf("the reason is %q", got.Unavailable)
	}
	if got.Unavailable.Say() == "" {
		t.Error("there is no sentence to show, so the surface has nothing to say")
	}
}

// Each way of being unable to point says its own thing.
func TestEveryReasonForNotPointingHasItsOwnSentence(t *testing.T) {
	live, on, _ := watching(t)
	q := asking(on.ID)

	nothing := observe.ReferentFor(q, observe.ReferentQuestion, observe.LiveGeometry{})
	if nothing.Unavailable != observe.ReferentNothingWatched {
		t.Errorf("with nothing being watched the reason is %q", nothing.Unavailable)
	}
	unreliable := live
	unreliable.Reliable = false
	if got := observe.ReferentFor(q, observe.ReferentQuestion, unreliable); got.Unavailable !=
		observe.ReferentCoordinatesUnreliable {
		t.Errorf("with unusable coordinates the reason is %q", got.Unavailable)
	} else if got.CanPoint() {
		t.Error("Marco drew anyway. A highlight that is nearly right is read as exactly right")
	}

	// Every reason is distinct prose, and every one of them says Marco still knows WHICH
	// structure it means — the inability is about the screen, not about the question.
	seen := map[string]bool{}
	for _, u := range []observe.ReferentUnavailable{
		observe.ReferentNothingWatched, observe.ReferentNotOnScreen, observe.ReferentNotAPart,
		observe.ReferentAnotherApplication, observe.ReferentCoordinatesUnreliable,
	} {
		s := u.Say()
		if s == "" {
			t.Errorf("%q has no sentence", u)
		}
		if seen[s] {
			t.Errorf("%q reuses another reason's sentence", u)
		}
		seen[s] = true
	}
	if observe.ReferentAvailable.Say() != "" {
		t.Error("being able to point produced an apology")
	}
}

// A whole screen is not a part of a screen, and is not outlined as though it were.
func TestAWholeScreenIsNotPointedAt(t *testing.T) {
	live, _, _ := watching(t)
	q := observe.Proposal{
		ID: "q_state", Kind: observe.PossibleSettingsLikeState,
		Subject: observe.Subject{Kind: observe.SubjectState, Ref: "state_1"},
	}
	got := observe.ReferentFor(q, observe.ReferentQuestion, live)
	if got.CanPoint() {
		t.Fatal("the whole window was outlined. That says nothing a person did not know " +
			"and implies a precision that is not there")
	}
	if got.Unavailable != observe.ReferentNotAPart {
		t.Errorf("the reason is %q", got.Unavailable)
	}
}

// Claiming to be able to point requires somewhere to point.
//
// The wording failure this whole mechanism exists to prevent: "the highlighted controls" in front
// of somebody with nothing highlighted.
func TestPointingRequiresBothAReasonAndSomewhereToPoint(t *testing.T) {
	if (observe.VisualReferent{}).CanPoint() {
		t.Error("an empty referent claims it can point")
	}
	empty := observe.VisualReferent{Role: observe.ReferentQuestion}
	if empty.CanPoint() {
		t.Error("a referent with no regions and no reason claims it can point")
	}
	withRegions := observe.VisualReferent{
		Regions:     []observe.Region{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05}},
		Unavailable: observe.ReferentNotOnScreen,
	}
	if withRegions.CanPoint() {
		t.Error("a referent that says it cannot point offered regions anyway")
	}
}

// The referent carries nothing captured.
//
// The same rule durable memory is held to, applied to a type that travels to a browser: counts,
// closed-vocabulary roles and window-relative geometry. No labels, no read text, no digests, no
// screenshots and no desktop coordinates.
func TestAReferentCarriesNothingCaptured(t *testing.T) {
	banned := map[string]bool{
		"SafeLabel": true, "Digest": true, "EntitySnapshot": true, "ShadowRegion": true,
		"ShadowSample": true, "StructureSignature": true, "InterfaceTerm": true,
		"ElementRole": true, "Rect": true,
	}
	var walk func(reflect.Type, string, int)
	walk = func(ty reflect.Type, path string, depth int) {
		if depth > 6 {
			return
		}
		if banned[ty.Name()] {
			t.Errorf("%s reaches %s, which carries captured content", path, ty.Name())
			return
		}
		switch ty.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array:
			walk(ty.Elem(), path+"[]", depth+1)
		case reflect.Map:
			walk(ty.Key(), path+"{key}", depth+1)
			walk(ty.Elem(), path+"{}", depth+1)
		case reflect.Struct:
			for i := range ty.NumField() {
				f := ty.Field(i)
				walk(f.Type, path+"."+f.Name, depth+1)
			}
		}
	}
	walk(reflect.TypeOf(observe.VisualReferent{}), "VisualReferent", 0)
}

// regionsFor is a group's RAW member geometry, deliberately unfiltered.
//
// Not a second copy of the resolver: it is the set the resolver draws FROM, so a test can assert
// that every rectangle Marco points at belongs to the subject without also asserting that nothing
// was left out. Those are different claims, and only the first one is about binding.
func regionsFor(t *testing.T, g observe.StructuralGroup,
	live observe.LiveGeometry) []observe.Region {

	t.Helper()
	by := map[string]observe.ShadowTrack{}
	for _, tr := range live.Tracks {
		by[tr.ID] = tr
	}
	out := make([]observe.Region, 0, len(g.Members))
	for _, id := range g.Members {
		tr, ok := by[id]
		if !ok || !tr.Present || tr.Reference.Width <= 0 || tr.Reference.Height <= 0 {
			continue
		}
		out = append(out, tr.Reference)
	}
	return out
}

// A question about a screen you have left stops pointing, and does not jump.
//
// The failure this prevents is specific and bad: the user navigates away, the highlight stays on
// screen, and it is now sitting on whatever happens to be there. They answer a question about one
// thing while looking at another.
func TestAQuestionAboutAScreenYouHaveLeftStopsPointing(t *testing.T) {
	live, onScreen, departed := watching(t)

	got := observe.ReferentFor(asking(departed.ID), observe.ReferentQuestion, live)
	if got.CanPoint() {
		t.Fatalf("a group that is no longer visible is still being pointed at: %+v", got.Regions)
	}
	if got.Unavailable != observe.ReferentNotOnScreen {
		t.Errorf("the reason is %q", got.Unavailable)
	}
	// And emphatically NOT the group that is on screen. Falling through to what is visible
	// is the mistake that puts a box round the wrong thing while the question says the right
	// one.
	here := observe.ReferentFor(asking(onScreen.ID), observe.ReferentQuestion, live)
	if !here.CanPoint() {
		t.Fatal("the visible group cannot be pointed at, so this proves nothing")
	}
	for _, r := range got.Regions {
		for _, o := range here.Regions {
			if r == o {
				t.Fatal("the departed question borrowed the visible group's geometry")
			}
		}
	}
}

// Never the durable envelope.
//
// A remembered subject carries geometry that can be months old and, as the Explorer record shows,
// can be unusable. Pointing with it would draw a box where something used to be — confidently
// indicating nothing.
func TestPointingNeverFallsBackToRememberedGeometry(t *testing.T) {
	live, _, departed := watching(t)
	q := asking(departed.ID)
	// A proposal that HAS been recognised against a durable record: the recall path fills
	// these in, and they are exactly the temptation.
	q.Recognised, q.RecognisedAs = true, "subj_remembered"
	q.Subject.Fingerprint.Envelope = &observe.Region{X: 0.04, Y: 0.1, Width: 0.11, Height: 0.8}

	got := observe.ReferentFor(q, observe.ReferentQuestion, live)
	if got.CanPoint() {
		t.Fatal("remembered geometry was used to point at something not currently visible")
	}
	if len(got.Regions) != 0 {
		t.Errorf("an unavailable referent carries %d region(s)", len(got.Regions))
	}
}

// A member that IS the window is not a part of it.
//
// Measured against the real thing: watching VS Code produced a group of 24 members, 12 of which
// were the entire frame — the window root, the workbench, and the panes that fill it, all of which
// recur exactly as reliably as the buttons inside them. Every one of those rectangles is correct.
// Drawing them is still wrong, because a person shown the whole window outlined concludes Marco
// means the whole window, and the eight controls it actually means stop meaning anything.
//
// The rule is not new. ReferentNotAPart already says a subject that is a whole screen cannot be
// pointed at; this is that same sentence applied to a member.
func TestPointingSkipsMembersThatAreTheWholeWindow(t *testing.T) {
	live, group := containedGroup(t)

	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		observe.ReferentQuestion, live)

	if !got.CanPoint() {
		t.Fatalf("every member was discarded: %q. Trading a useless highlight for no "+
			"highlight is not the fix", got.Unavailable)
	}
	for i, r := range got.Regions {
		if r.Width >= 0.9 && r.Height >= 0.9 {
			t.Fatalf("region %d covers the whole window (%v). Outlining it says nothing a "+
				"person did not already know, and it drowns every real member beside it",
				i, r)
		}
	}
	// The container really was a member — otherwise this passes for the wrong reason and
	// would keep passing after the rule was deleted.
	if len(got.Regions) != len(group.Members)-1 {
		t.Fatalf("%d member(s) produced %d region(s); exactly one of them is the window",
			len(group.Members), len(got.Regions))
	}
}

// A subject made ENTIRELY of containers says so, and does not read as "not on screen".
//
// Two silences that must not sound alike. "I can't see it" sends somebody to look for the thing;
// "there is nothing here to single out" tells them Marco found it and it has no parts. The second
// is the sentence ReferentNotAPart already carries for a whole-screen subject.
func TestAGroupOfNothingButContainersIsNotAPartRatherThanMissing(t *testing.T) {
	live := onScreenTracks([]observe.Region{
		{X: 0, Y: 0, Width: 1, Height: 1},
		{X: 0.004, Y: 0.01, Width: 0.99, Height: 0.98},
	})
	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)

	if got.CanPoint() {
		t.Fatal("outlined the window")
	}
	if got.Unavailable != observe.ReferentNotAPart {
		t.Errorf("refused with %q, want %q — every member was present, so \"not on screen\" "+
			"would send somebody looking for something that is right in front of them",
			got.Unavailable, observe.ReferentNotAPart)
	}
}

// A wide or tall control is a part of the screen however far it reaches.
//
// The failure mode of a careless test of this rule: a toolbar spanning the window is 100% of its
// width and nothing like the window, and losing it would remove exactly the kind of structure
// worth pointing at from every application that has one.
func TestAFullWidthControlIsStillAPartOfTheScreen(t *testing.T) {
	for _, r := range []observe.Region{
		{X: 0, Y: 0, Width: 1, Height: 0.04},          // a toolbar
		{X: 0, Y: 0.01, Width: 0.06, Height: 0.98},    // an activity bar down the side
		{X: 0, Y: 0.96, Width: 1, Height: 0.04},       // a status bar
		{X: 0.05, Y: 0.02, Width: 0.89, Height: 0.89}, // a large pane, still inset
	} {
		live := onScreenTracks([]observe.Region{
			{X: 0.1, Y: 0.0, Width: 0.05, Height: 0.02}, r,
		})
		got := observe.ReferentForSubject(
			observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
			observe.ReferentQuestion, live)
		if len(got.Regions) != 2 {
			t.Errorf("%v was treated as the whole window; a control is not the screen "+
				"merely for spanning one of its dimensions", r)
		}
	}
}

// onScreenTracks builds one group whose members are exactly these regions, present now.
//
// Hand-built rather than run through the tracker, because what is under test is which MEMBERS get
// a rectangle — and a fixture whose grouping rules quietly dropped the container would prove
// nothing about the rule that is supposed to drop it.
func onScreenTracks(regions []observe.Region) observe.LiveGeometry {
	const state = observe.ScreenStateID("state_1")
	tracks := make([]observe.ShadowTrack, 0, len(regions))
	for i, r := range regions {
		tracks = append(tracks, observe.ShadowTrack{
			ID: fmt.Sprintf("track_%d", i), Present: true, Reference: r,
			Seen: 10, Eligible: 10,
			States: []observe.TrackState{{State: state, Seen: 10, Eligible: 10}},
		})
	}
	return observe.LiveGeometry{
		Application: "testapp", Window: "hwnd:1", AtInference: 10, Reliable: true,
		Tracks: tracks,
		States: []observe.ScreenState{{ID: state, Inferences: 10, Episodes: 1}},
	}
}

// containedGroup is the shape VS Code actually produced: real controls in a row, with the window
// root sitting among them because it recurs exactly as reliably as they do.
func containedGroup(t *testing.T) (observe.LiveGeometry, observe.StructuralGroup) {
	t.Helper()
	live := onScreenTracks([]observe.Region{
		{X: 0, Y: 0, Width: 1, Height: 1},
		{X: 0.02, Y: 0.01, Width: 0.02, Height: 0.03},
		{X: 0.05, Y: 0.02, Width: 0.02, Height: 0.03},
		{X: 0.08, Y: 0.03, Width: 0.03, Height: 0.03},
	})
	groups := observe.Groups(live.Tracks, live.States)
	if len(groups) == 0 {
		t.Skip("the tracks did not form a group")
	}
	return live, groups[0]
}

// A group and its own members are not both outlined.
//
// From the first live acceptance run, on VS Code's menu bar. Nine outlines went up: one round each
// of eight menu items, and one round all eight at once — the menu bar, which the accessibility
// tree exposes beside its children and which recurs exactly as reliably as they do. Every
// rectangle was correct. What a person saw was an extra box next to the last item, pointing at
// nothing that was not already pointed at.
func TestPointingDoesNotOutlineAGroupAndItsMembersBoth(t *testing.T) {
	// The measured shape, normalised: the bar, then the items inside it. The bar shares its
	// left edge and its height with the first item, which is why a strict-containment rule
	// would miss it.
	bar := observe.Region{X: 0.0222, Y: 0.0076, Width: 0.2076, Height: 0.0334}
	items := []observe.Region{
		{X: 0.0222, Y: 0.0076, Width: 0.0186, Height: 0.0334},
		{X: 0.0403, Y: 0.0076, Width: 0.0201, Height: 0.0334},
		{X: 0.0599, Y: 0.0076, Width: 0.0356, Height: 0.0334},
		{X: 0.0950, Y: 0.0076, Width: 0.0227, Height: 0.0334},
	}
	live := onScreenTracks(append([]observe.Region{bar}, items...))

	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)

	if !got.CanPoint() {
		t.Fatalf("nothing to point at: %q", got.Unavailable)
	}
	for i, r := range got.Regions {
		if r == bar {
			t.Fatalf("region %d is the bar that contains the other %d. It adds no "+
				"information and reads as a box pointing at nothing", i, len(items))
		}
	}
	if len(got.Regions) != len(items) {
		t.Errorf("%d region(s), want the %d items themselves", len(got.Regions), len(items))
	}
}

// An ordinary pair is not an enclosure.
//
// A label inside its cell, a glyph inside its button: one member containing one other is real
// structure, and a rule that dropped it would start removing content to tidy the picture.
func TestAMemberContainingOneOtherIsKept(t *testing.T) {
	outer := observe.Region{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05}
	live := onScreenTracks([]observe.Region{
		outer,
		{X: 0.11, Y: 0.11, Width: 0.05, Height: 0.02}, // inside outer
		{X: 0.5, Y: 0.5, Width: 0.05, Height: 0.02},   // elsewhere
	})
	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)

	for _, r := range got.Regions {
		if r == outer {
			return
		}
	}
	t.Error("a member enclosing a single other was dropped as if it were a container")
}

// The smallest member always survives, so the filter cannot empty the referent.
//
// Not a guard — a property. The smallest-area member of any set encloses nothing, so it is never
// a candidate for removal. A defensive `if the result is empty, keep everything` was written here
// first and no mutation could kill it: nothing can reach it.
func TestTheSmallestMemberAlwaysSurvivesEnclosureFiltering(t *testing.T) {
	live := onScreenTracks([]observe.Region{
		{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.2},
		{X: 0.11, Y: 0.11, Width: 0.1, Height: 0.05},
		{X: 0.25, Y: 0.11, Width: 0.1, Height: 0.05},
	})
	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)
	if !got.CanPoint() {
		t.Fatalf("the referent was emptied: %q", got.Unavailable)
	}
}

// allDrawnFrom insists every rectangle came from this subject's own members.
//
// A SUBSET check, not an equality one. The resolver legitimately leaves members out — a control
// that is not present, one that is the whole window, one that merely encloses the others — and a
// test demanding equality would be asserting that no such rule may ever exist. What binding means
// is that nothing Marco points at came from somewhere else.
func allDrawnFrom(t *testing.T, got, members []observe.Region, subject string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("%s produced no regions", subject)
	}
	for i, r := range got {
		found := false
		for _, m := range members {
			if r == m {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("region %d (%v) is not one of %s's own members", i, r, subject)
		}
	}
}

// sharesAnything reports whether two sets have a rectangle in common.
func sharesAnything(a, b []observe.Region) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// One member Marco cannot place does not silence the ones it can.
//
// Found live, on Discord: an accessibility tree reported an element whose rectangle is not inside
// the window being watched, so its normalised region fell outside 0..1. `referent.Map` refused the
// whole batch — correctly, from where it stands, because one stray rectangle among many is exactly
// what a window that moved mid-measurement looks like. This layer knows better: it knows which
// member each region belongs to and that they were measured in one inference, so it declines the
// one it cannot place and keeps the rest.
func TestAMemberOutsideItsOwnWindowDoesNotSilenceTheRest(t *testing.T) {
	good := []observe.Region{
		{X: 0.10, Y: 0.10, Width: 0.05, Height: 0.02},
		{X: 0.20, Y: 0.10, Width: 0.05, Height: 0.02},
		{X: 0.30, Y: 0.10, Width: 0.05, Height: 0.02},
	}
	// The shapes a tree actually produces for something outside its own window.
	for _, stray := range []observe.Region{
		{X: -0.12, Y: 0.10, Width: 0.05, Height: 0.02}, // off to the left
		{X: 0.10, Y: -0.30, Width: 0.05, Height: 0.02}, // above the frame
		{X: 0.98, Y: 0.10, Width: 0.20, Height: 0.02},  // running off the right
	} {
		live := onScreenTracks(append(append([]observe.Region{}, good...), stray))
		got := observe.ReferentForSubject(
			observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
			observe.ReferentQuestion, live)

		if !got.CanPoint() {
			t.Fatalf("%v silenced the whole referent: %q. One member Marco cannot place is "+
				"not a reason to stop pointing at the three it can", stray, got.Unavailable)
		}
		if len(got.Regions) != len(good) {
			t.Errorf("%v produced %d region(s), want %d", stray, len(got.Regions), len(good))
		}
		for _, r := range got.Regions {
			if r == stray {
				t.Errorf("%v was kept; pkg/referent would then refuse the whole mapping "+
					"and nothing would be drawn at all", stray)
			}
		}
	}
}

// A subject Marco cannot place AT ALL says that, and does not read as absent.
func TestASubjectEntirelyOutsideItsWindowSaysTheCoordinatesAreUnreliable(t *testing.T) {
	live := onScreenTracks([]observe.Region{
		{X: -0.5, Y: 0.1, Width: 0.2, Height: 0.05},
		{X: -0.5, Y: 0.2, Width: 0.2, Height: 0.05},
	})
	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)

	if got.CanPoint() {
		t.Fatal("pointed at something outside the window it was measured against")
	}
	if got.Unavailable != observe.ReferentCoordinatesUnreliable {
		t.Errorf("refused with %q, want %q — the members are present, so \"not on screen\" "+
			"would be describing the wrong problem",
			got.Unavailable, observe.ReferentCoordinatesUnreliable)
	}
}

// Exactly two enclosed members is enough.
//
// The boundary, pinned from both sides: TestAMemberContainingOneOtherIsKept holds it at one, this
// holds it at two. Without this, raising the threshold to three passes every other test — and a
// three-item toolbar, which is most toolbars, keeps its redundant box.
func TestTwoEnclosedMembersIsEnoughToBeAnEnclosure(t *testing.T) {
	bar := observe.Region{X: 0.02, Y: 0.01, Width: 0.10, Height: 0.03}
	live := onScreenTracks([]observe.Region{
		bar,
		{X: 0.02, Y: 0.01, Width: 0.04, Height: 0.03},
		{X: 0.07, Y: 0.01, Width: 0.04, Height: 0.03},
	})
	got := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: "group_1"},
		observe.ReferentQuestion, live)

	for i, r := range got.Regions {
		if r == bar {
			t.Fatalf("region %d is the bar containing the other two; two is a set, and a "+
				"box round a set that is already outlined points at nothing new", i)
		}
	}
	if len(got.Regions) != 2 {
		t.Errorf("%d region(s), want the two items", len(got.Regions))
	}
}
