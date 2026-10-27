package codegen_test

// TYPED TEXT IS A LINK, AND A MIXED TEXT IS SEVERAL.
//
// `StepType` was in `emit` and in no arm of `propagating`, so typed text was never a link in a
// guarded routine: whatever followed it ran whether the text arrived or not. The keystroke's
// defect, one input along, and worse — a keystroke that did not arrive leaves the screen
// unchanged, and typing that stopped half way leaves a field holding something nobody chose. The
// press after it is a Save on a form that will be rejected, or accepted with the wrong value.
//
// # Why the expansion has to happen before the chain is built
//
// One `StepType` can emit SEVERAL sentences. `"user{{password}}"` is a Type, a Secret and
// possibly another Type, and a link is built around ONE `do` sentence — so a step emitting three
// put two of them where nothing could stop at them. Whichever one the chain wrapped, the other
// two ran regardless, which is the original defect surviving in the one case where it matters
// most: a password going into a form whose username never arrived.
//
// So `textBySegment` splits the step first, each segment becomes its own link, and these tests
// count the links rather than reading the sentences — a route with the right words in the wrong
// shape is the thing being ruled out.
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// typing is one demonstrated `Type` step.
func typing(text string) macroir.Step {
	return macroir.Step{Kind: macroir.StepType, Text: text}
}

// links counts the capabilities a chained route was written as.
//
// By the capability declarations and not by the `when ok?` arms: the script at the bottom of
// every route has an arm of its own, so counting arms says one more than there are links and says
// `1` about a legacy route that has none.
func links(src string) int {
	return strings.Count(src, "this can ")
}

// A TYPED TEXT IN A GUARDED ROUTINE IS THE GUARD OF A LINK.
//
// `do … ...` followed by `when ok?` is a link; `do … .` is an ordinary step. That is the whole
// difference, and asserting that the typing is "in the route" passes either way — which is how
// this went unnoticed: every test of typed text was about the words, and every test of the chain
// was about presses and keys.
func TestATypedTextInAGuardedRoutineIsALink(t *testing.T) {
	src := generated(t, aMeasuredPress(), typing("hello"))

	if !strings.Contains(src, "do OS's Type with \"hello\"...\n") {
		t.Errorf("the typing is not the guard of a link — it is written as an "+
			"ordinary step, so whatever follows it runs whether the text "+
			"arrived or not:\n%s", src)
	}
	if strings.Contains(src, "do OS's Type with \"hello\".\n") {
		t.Errorf("the typing is performed as a plain step:\n%s", src)
	}
}

// AND A MIXED TEXT IS ONE LINK PER SEGMENT, IN THE ORDER IT WAS DEMONSTRATED.
//
// The case the contract is about. Three sentences out of one step, and all three have to be
// stoppable: a username that did not arrive must not be followed by the password, and a password
// that did not arrive must not be followed by whatever submits the form.
func TestAMixedTextIsOneLinkPerSegment(t *testing.T) {
	src := generated(t, aMeasuredPress(), typing("user{{password}}tail"))

	// FIVE LINKS: the preparation, the press, then the literal, the secret and the
	// literal after it. Counted rather than read, because the sentences can all be
	// present in a route that wrapped one of them and left the other two in its success.
	if n := links(src); n != 5 {
		t.Errorf("a press and a three-segment text were written as %d link(s), "+
			"want 5 — one per thing that can fail to happen, the Activate "+
			"included:\n%s", n, src)
	}
	for _, want := range []string{
		"do OS's Type with \"user\"...\n",
		"do OS's Secret with \"password\"...\n",
		"do OS's Type with \"tail\"...\n",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the route does not contain %q as a link's guard:\n%s",
				want, src)
		}
	}
	// AND IN ORDER. A chain whose segments were reordered would type the password into
	// the field the username was going into.
	user := strings.Index(src, "OS's Type with \"user\"")
	secret := strings.Index(src, "OS's Secret with \"password\"")
	tail := strings.Index(src, "OS's Type with \"tail\"")
	if !(user < secret && secret < tail) {
		t.Errorf("the segments are written in the order %d, %d, %d:\n%s", user,
			secret, tail, src)
	}
}

// AND A DECLARED SECRET ARG KEEPS ITS ROUTE-QUALIFIED NAME IN A CHAIN.
//
// The one thing two places deciding this could get wrong in a way nobody notices until a run:
// `<route>:password` is where the value was stored, and an unqualified `password` is a lookup of
// a global secret nobody set. `emit` qualified it and a chain built from a second copy of that
// decision would not have — so there is one function now, and this is what holds it.
func TestADeclaredSecretKeepsItsRouteQualifiedNameInAChain(t *testing.T) {
	src, _, err := codegen.Route("ark graphics", "arkascended",
		[]macroir.Step{aMeasuredPress(), typing("{{password}}")}, t.TempDir(),
		"password")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "do OS's Secret with \"ark-graphics:password\"...\n") {
		t.Errorf("the declared secret is not looked up under the route's own name "+
			"as a link:\n%s", src)
	}
}

// AND A TYPING THAT DID NOT HAPPEN STOPS WHAT FOLLOWS IT.
//
// The behavioural half of being a link. Three cases because there are three places a mixed text
// can stop, and each leaves a different thing on the screen: before the username, between the
// username and the password, and after the password with the form still unsubmitted.
func TestATypingThatDidNotHappenStopsWhatFollowsIt(t *testing.T) {
	for _, c := range []struct {
		what   string
		refuse int
		did    []string
	}{
		{"the first literal", 1, []string{"Activate", "Click 1", "Type user"}},
		{"the secret", 2, []string{"Activate", "Click 1", "Type user",
			"Secret password"}},
		{"the last literal", 3, []string{"Activate", "Click 1", "Type user",
			"Secret password", "Type tail"}},
	} {
		t.Run(c.what, func(t *testing.T) {
			w := &watcher{refuseTyped: c.refuse}
			_, out := performedBy(t, w, aMeasuredPress(),
				typing("user{{password}}tail"), elsewhereInTheSameWindow())

			mustHaveDone(t, w, c.did...)
			if strings.Contains(out, "ark graphics: done") {
				t.Errorf("the routine reported that it finished after a "+
					"typing that did not happen:\n%s", out)
			}
		})
	}
}

// AND THE CONTROL: A TYPING THE MACHINE TOOK LETS THE ROUTINE GO ON.
//
// Without which every case above is satisfied by a route that stops at the first typing whatever
// happens — and that route would be useless in a different way.
func TestATypingThatHappenedLetsTheRoutineGoOn(t *testing.T) {
	w := &watcher{}
	_, out := performedBy(t, w, aMeasuredPress(), typing("user{{password}}tail"),
		elsewhereInTheSameWindow())

	mustHaveDone(t, w, "Activate", "Click 1", "Type user", "Secret password",
		"Type tail", "Click 2", "Sleep")
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine did not report that it finished:\n%s", out)
	}
}

// AND A ROUTE THAT ASKS TO BE CHECKED ABOUT NOTHING IS GENERATED AS IT ALWAYS WAS.
//
// The compatibility half, and the one that has to be asserted rather than assumed: every routine
// already on disk was saved under a contract that says a step which did not happen does not stop
// the run. Newly generated GUARDED routines gain propagation; a legacy one is the block it was,
// with its typing as a plain `do` and no arms around it.
func TestALegacyRoutinesTypingIsGeneratedUnchanged(t *testing.T) {
	src := generated(t, macroir.Step{Kind: macroir.StepClick, X: 10, Y: 20},
		typing("user{{password}}tail"))

	for _, want := range []string{
		"do OS's Type with \"user\".\n",
		"do OS's Secret with \"password\".\n",
		"do OS's Type with \"tail\".\n",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("a route that checks nothing lost %q:\n%s", want, src)
		}
	}
	if n := links(src); n != 1 {
		t.Errorf("a route that checks nothing was written as %d capabilities — a "+
			"chain — rather than the one block it has always been:\n%s", n, src)
	}
	if strings.Contains(src, "when ok?\n            do ") {
		t.Errorf("a route that checks nothing calls the next thing from inside an "+
			"arm:\n%s", src)
	}
}

// AND ITS CONTINUATION SEMANTICS ARE WHAT THEY WERE.
//
// The half that matters, and it is behavioural: a legacy route whose typing the machine refused
// goes ON to the next step. That is what every routine saved before any of this promises, and the
// one thing that must not happen to those routes is acquiring half of a guarantee — a route that
// stopped at a refused typing while nothing else about it was checked would be a third contract
// nobody chose.
func TestALegacyRoutineCarriesOnAfterATypingThatDidNotHappen(t *testing.T) {
	w := &watcher{refuseTyped: 1}
	_, out := performedBy(t, w, macroir.Step{Kind: macroir.StepClick, X: 10, Y: 20},
		typing("hello"), macroir.Step{Kind: macroir.StepClick, X: 30, Y: 40})

	// THE ACTIVATE IS THERE AND IS NOT A LINK. A legacy route brings its application
	// forward with a bare `do` whose status nobody reads, which is the shape it has
	// always had — so it is in the list, and so is everything after the refusal.
	mustHaveDone(t, w, "Activate", "Click 1", "Type hello", "Click 2", "Sleep")
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("a legacy routine whose typing was refused did not run to its "+
			"end:\n%s", out)
	}
}
