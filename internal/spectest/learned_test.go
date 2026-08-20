package spectest_test

import (
	"go/build"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
)

// What Marco learned is ordinary Marco, and the compiler is the authority on that.
//
// # Why the gate is here
//
// Because this package already owns the question "is generated Marco legal, inside Core, and free
// of backstage vocabulary", and it owns it with the REAL lexer, parser, graph builder and
// compiler. A learned play checked by string assertions or a second parser would be checked
// against this milestone's opinion of Marco rather than against Marco.

// learnedPlays are the shapes a verified procedure can take, as ordered navigation meanings.
//
// One step, two steps, a repeated direction, and a back/confirm mixture — the four things a
// learned route actually looks like.
var learnedPlays = []struct {
	name  string
	steps [][]string
}{
	{"one step", [][]string{{"confirm"}}},
	{"two steps", [][]string{{"confirm"}, {"confirm"}}},
	{"a repeated direction", [][]string{{"down", "down", "confirm"}}},
	{"back and confirm", [][]string{{"back"}, {"down", "confirm"}}},
	{"a longer route", [][]string{
		{"confirm"}, {"down", "down", "confirm"}, {"right", "confirm"}, {"back"},
	}},
}

// TestALearnedPlayCompiles is the round trip, through the real compiler.
//
// Not a string check and not a second parser: source in, tokens, tree, graph, compiled program.
// If the Director cannot write legal Marco, this is where it finds out.
func TestALearnedPlayCompiles(t *testing.T) {
	for _, tc := range learnedPlays {
		t.Run(tc.name, func(t *testing.T) {
			src, err := marcoexec.LowerPlay(tc.steps)
			if err != nil {
				t.Fatalf("lowering: %v", err)
			}
			if err := compileAgainstTheRealOS(src); err != nil {
				t.Fatalf("the generated play does not compile: %v\n\n%s", err, src)
			}
		})
	}
}

// The compiled program does what the procedure said, in order, with repeats intact.
//
// The semantic half of the round trip. It is not enough that the source compiles — the sentences
// that survive compilation have to be the navigation meanings that went in, in the same order,
// with `down, down` still two presses.
func TestALearnedPlaySaysExactlyWhatItWasGiven(t *testing.T) {
	for _, tc := range learnedPlays {
		t.Run(tc.name, func(t *testing.T) {
			// The expectation is built BEFORE lowering, on purpose. Built afterwards it
			// would be read from a slice the generator might have reordered in place, and
			// the test would agree with the bug.
			var want []string
			for _, run := range tc.steps {
				want = append(want, run...)
			}
			src, err := marcoexec.LowerPlay(tc.steps)
			if err != nil {
				t.Fatalf("lowering: %v", err)
			}

			// Read the meanings back out of the SOURCE the compiler accepted, in the
			// order they appear. Anything reordered, dropped or deduplicated shows here.
			var got []string
			for _, line := range strings.Split(src, "\n") {
				line = strings.TrimSpace(line)
				const prefix = `do OS's Navigate with "`
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				got = append(got, strings.TrimSuffix(
					strings.TrimPrefix(line, prefix), `".`))
			}
			if len(got) != len(want) {
				t.Fatalf("the play says %v; the procedure was %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("press %d is %q, want %q (whole play %v)",
						i+1, got[i], want[i], got)
				}
			}
		})
	}
}

// The same procedure produces byte-identical Marco, every time.
//
// Nothing session-shaped reaches the generator — no session number, no screen state, no track id,
// no window generation, no process id, no map iteration — so this is a property of the input
// rather than of luck. Repeated because a map ordering bug is intermittent by nature.
func TestALearnedPlayIsByteIdentical(t *testing.T) {
	for _, tc := range learnedPlays {
		first, err := marcoexec.LowerPlay(tc.steps)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for i := 0; i < 50; i++ {
			again, err := marcoexec.LowerPlay(tc.steps)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if again != first {
				t.Fatalf("%s generated different source on run %d", tc.name, i+2)
			}
		}
	}
}

// A learned play stays inside Core and names no backstage concept.
//
// The same two gates the checked-in routes are held to, applied to what Director writes when it
// writes down something it learned by watching — which is the most likely place for an
// implementation detail to escape, because that is where Director knows the most.
func TestALearnedPlayStaysInsideCoreAndOffTheBackstage(t *testing.T) {
	backstage := []string{
		"confidence", "hypothesis", "evidence", "candidate", "assessment",
		"checkpoint", "perception", "semanticmemory", "corroborat", "verdict",
		"screenparser", "shadow", "inference", "fingerprint", "signature",
		"topology", "recall", "provenance", "quarantine", "rehears", "digest",
		"subj_", "state_", "group_", "hwnd", "grant", "attempt", "session",
		"unobservable", "verified",
	}
	for _, tc := range learnedPlays {
		src, err := marcoexec.LowerPlay(tc.steps)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for i, line := range strings.Split(src, "\n") {
			w := firstWord(line)
			if w != "" && !coreVocabulary[w] {
				if area, ok := outsideCore[w]; ok {
					t.Errorf("%s:%d opens with %q (%s), outside Core",
						tc.name, i+1, w, area)
					continue
				}
				t.Errorf("%s:%d opens with %q, which spec/Core.md does not describe:\n    %s",
					tc.name, i+1, w, strings.TrimSpace(line))
			}
			lower := strings.ToLower(line)
			for _, word := range backstage {
				if strings.Contains(lower, word) {
					t.Errorf("%s:%d mentions %q:\n    %s\n"+
						"  Director may know WHY it learned this; the play says WHAT it "+
						"does — see Core.md#governance", tc.name, i+1, word,
						strings.TrimSpace(line))
				}
			}
		}
	}
}

// A meaning Core has no sentence for is reported, never invented around.
//
// The language-expression gap. Widening Marco to make lowering convenient is the one thing this
// milestone must not do, so an unknown meaning stops the lowering and says so.
func TestAMeaningCoreCannotSayStopsTheLowering(t *testing.T) {
	for _, bad := range [][]string{
		{"point"},                   // a pointer press needs a position, and that is not a meaning
		{"jump"},                    // not a navigation meaning at all
		{"confirm", "double-click"}, // one good, one impossible
	} {
		if src, err := marcoexec.LowerPlay([][]string{bad}); err == nil {
			t.Fatalf("%v was written down as Marco:\n%s", bad, src)
		}
	}
	if _, err := marcoexec.LowerPlay(nil); err == nil {
		t.Fatal("a play with nothing to do was written down")
	}
	if _, err := marcoexec.LowerPlay([][]string{{}}); err == nil {
		t.Fatal("a step with no presses was written down")
	}
}

// A novice can read it: the play names the meanings and nothing else.
//
// Deliberately a shape test rather than a taste test. What it holds is that the only sentences
// with content are the presses, so whatever a reader makes of the file, there is nothing in it
// standing between them and the four words `confirm`, `down`, `down`, `confirm`.
func TestALearnedPlayHasNothingInItButThePresses(t *testing.T) {
	src, err := marcoexec.LowerPlay([][]string{{"confirm"}, {"down", "down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if n := strings.Count(src, "//"); n != 1 {
		t.Errorf("%d comment(s); a learned play carries one line for the reader and no "+
			"explanation of Director", n)
	}
	if strings.Contains(src, "0.") || strings.Contains(src, "with 1") {
		t.Errorf("the play contains a number:\n%s", src)
	}
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `do OS's`) &&
			!strings.Contains(line, `do OS's Navigate with `) {
			t.Errorf("a learned play reaches a capability other than Navigate:\n    %s", line)
		}
	}
}

// compileAgainstTheRealOS compiles a learned play against the canonical OS act surface.
//
// Not the shared stub. That stub is a hand-maintained subset for the spec's examples, and a
// learned play will one day be run against the real `internal/osmod/os.marco` — so the real one is
// what decides whether Director wrote something the language accepts. A capability the Director
// emits that os.marco does not export fails here, which is exactly the failure ADR-005 exists to
// force at compile time rather than at a keyboard.
// # Why it no longer assembles the modules here
//
// It used to concatenate every module source a learned play might import and delete the matching
// `use` lines — a hand-maintained list, and the Director's pre-flight kept a SECOND, shorter one.
// Both claimed to answer "is this legal Marco". They disagreed the moment a play could press a
// control by name: this list had the Theater act, the Director's did not, so the suite was green
// while a verified, named, rehearsed route died live on `unknown type "Target"`.
//
// A test that assembles the world differently from the product is not testing the product. So this
// calls the same resolver the runtime uses, which is now also what the Director calls — one
// answer, and a drift like that becomes impossible rather than merely unlikely.
func compileAgainstTheRealOS(src string) error {
	return driver.CheckSource(src)
}

// The generator cannot reach anything that acts.
//
// Generating Marco is not authorization to run Marco. `LowerPlay` takes strings and returns a
// string; what it must not be able to do is acquire a host, a grant, an input adapter or an
// execution path along the way — because a generator that could would be one refactor from
// "generate and then run it".
func TestTheLearnedGeneratorCannotReachAnythingThatActs(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	reachable := map[string]bool{}
	if err := walkImportsFrom(pkg, reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	// Scoped to THIS module. Go's standard library has its own `internal/runtime`, and a
	// check that matched it would fail on every package in the repository.
	const mod = "github.com/chaynes-simpleclouds/marco/"
	for _, forbidden := range []struct{ fragment, why string }{
		{"internal/runtime", "the Marco runtime — can invoke hosts"},
		{"internal/oshost", "the OS host: keyboard, mouse, clipboard"},
		{"internal/driver", "drives input"},
		{"internal/recorder", "installs low-level input hooks"},
		{"internal/winctx", "window activation and focus"},
		{"internal/screen", "screen capture and input geometry"},
		{"internal/platform", "platform adapters, including every real host"},
		{"internal/director/rehearse", "rehearsal grants: the one thing that can authorize input"},
		{"internal/director/execute", "the execution pipeline"},
		{"os/exec", "starting processes"},
		// `os/exec` is stdlib and has no module prefix, so it is checked separately below.
	} {
		for path := range reachable {
			if strings.HasPrefix(path, mod) &&
				strings.Contains(strings.TrimPrefix(path, mod), forbidden.fragment) {
				t.Errorf("the lowering package reaches %s\n\treason it is forbidden: %s\n\t"+
					"writing a play down must not be able to become performing it",
					path, forbidden.why)
			}
		}
	}
}

func walkImportsFrom(path string, seen map[string]bool, depth int) error {
	if depth > 12 || seen[path] {
		return nil
	}
	seen[path] = true
	p, err := build.Import(path, ".", 0)
	if err != nil {
		return nil
	}
	for _, imp := range p.Imports {
		if err := walkImportsFrom(imp, seen, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// And it cannot start a process either. Checked apart from the module-scoped list above because
// `os/exec` is standard library and carries no module prefix.
func TestTheLearnedGeneratorCannotStartAProcess(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	reachable := map[string]bool{}
	if err := walkImportsFrom(pkg, reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	if reachable["os/exec"] {
		t.Error("the lowering package can start a process")
	}
}

// ── a play that says where it begins ──────────────────────────────────────────

// The entry condition is ordinary Core v1: `when ok?` over a read-only capability.
//
// No new syntax. `do <Act>'s <Cap> with "…"... when ok? … or? …` is the shape `OS's Focus` has
// always used to mean "ok if the active window matches" — what was missing was a capability that
// could answer "is the screen the user named the one in front?", and a capability is not a
// language change. See [[ADR-030-a-play-says-where-it-begins]].
func TestAGuardedPlayCompilesAndIsStillCoreMarco(t *testing.T) {
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu",
		"controller settings", [][]string{{"down", "down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if err := compileAgainstTheRealOS(src); err != nil {
		t.Fatalf("the guarded play does not compile: %v\n\n%s", err, src)
	}
	// It says where it begins, and what happens when it cannot.
	for _, want := range []string{
		`do Screen's Showing with "the pause menu"...`,
		"when ok?",
		"or?",
		`this is failed with error "this play starts on the pause menu"!`,
		// And where it expects to FINISH, in the same sentence shape, using the same
		// capability. No new syntax was needed to check a screen after the effects rather
		// than before them — see [[ADR-032-a-play-says-where-it-ends]].
		`do Screen's Showing with "controller settings"...`,
		`this is failed with error "this play expected to finish on controller settings"!`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the play does not contain %q:\n%s", want, src)
		}
	}
	// SUCCESS IS UNREACHABLE without the destination check. There is exactly one `this is
	// ok!` and it sits inside the postcondition's ok arm — so "every key went out" cannot
	// be mistaken for "the play worked".
	if n := strings.Count(src, "this is ok!"); n != 1 {
		t.Errorf("the play has %d success endings; there must be exactly one, inside the "+
			"destination check:\n%s", n, src)
	}
	okAt := strings.Index(src, "this is ok!")
	if destAt := strings.Index(src, `do Screen's Showing with "controller settings"`); destAt < 0 ||
		okAt < destAt {
		t.Errorf("the play can succeed before it has checked where it finished:\n%s", src)
	}
	// EVERY step is inside the `when ok?` arm. There is no path to a first effect that does
	// not go through the question — which is the whole of the guarantee.
	guard := strings.Index(src, `do Screen's Showing`)
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, `do OS's Navigate`) {
			continue
		}
		if strings.Index(src, line) < guard {
			t.Errorf("an effect appears before the entry condition:\n%s", src)
		}
		// And every effect sits INSIDE the `when ok?` arm. Nesting is what makes the guard
		// structural: there is no line after the block for control to fall through to, so
		// the only path to an effect is through an ok.
		//
		// An earlier shape asked first and let the `or?` arm return, with the steps after
		// the block. It compiled — and it did not guard, because a return inside an arm ends
		// the arm rather than the capability. Compiling is not behaving.
		if !strings.HasPrefix(line, "            ") {
			t.Errorf("an effect is not inside the `when ok?` arm:\n    %s", line)
		}
	}
	// And it is still Core vocabulary throughout, with no backstage word.
	for i, line := range strings.Split(src, "\n") {
		if w := firstWord(line); w != "" && !coreVocabulary[w] {
			t.Errorf("%d opens with %q, which spec/Core.md does not describe:\n    %s",
				i+1, w, strings.TrimSpace(line))
		}
		for _, word := range []string{"subj_", "state_", "shadow", "digest", "candidate",
			"evidence", "fingerprint", "confidence", "rehears"} {
			if strings.Contains(strings.ToLower(line), word) {
				t.Errorf("%d mentions %q:\n    %s", i+1, word, strings.TrimSpace(line))
			}
		}
	}
}

// A play with no entry condition is still lowerable, and says nothing about screens.
//
// The mechanism is a property of the PLAY, not of learned plays: an authored or taught play could
// carry the same sentence, and a play that does not need one carries nothing.
func TestAPlayWithoutAnEntryConditionSaysNothingAboutScreens(t *testing.T) {
	src, err := marcoexec.LowerPlay([][]string{{"confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if strings.Contains(src, "Screen's") || strings.Contains(src, "use screen.") {
		t.Errorf("an unguarded play talks about screens:\n%s", src)
	}
	if err := compileAgainstTheRealOS(src); err != nil {
		t.Fatalf("compiling: %v", err)
	}
}
