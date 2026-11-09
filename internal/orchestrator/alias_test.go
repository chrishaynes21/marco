package orchestrator

// TEACHING A ROUTINE ANOTHER PHRASE, AND THE COLLISION THAT IS REFUSED RATHER THAN RESOLVED.
//
// The operation's own rules. What is NOT here is the resolution — a phrase reaching the routine
// it belongs to is `internal/routes`, and a phrase two routines answer to becoming a question is
// `internal/resolveroute`. This is the library-wide half: whether a phrase may be added at all.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

const phraseSrc = "use os.\n\nthe App is a script.\n\ndo OS's Key with \"b\".\n"

// aLibraryOf is a registry holding one routine per route given, each saved and runnable.
func aLibraryOf(t *testing.T, rts ...routes.Route) Deps {
	t.Helper()
	reg := routes.Registry{Dir: t.TempDir()}
	for _, rt := range rts {
		if err := reg.Save(rt, phraseSrc); err != nil {
			t.Fatal(err)
		}
	}
	return Deps{Reg: reg, Out: &strings.Builder{}}
}

// A ROUTINE COMES TO ANSWER TO A PHRASE, AND THE PHRASE REACHES IT.
//
// Through the registry's own resolution afterwards, because storing a phrase nothing resolves is
// the shape of defect this repository keeps finding: the write succeeded, the file is right, and
// the product does not behave differently.
func TestAddingAPhraseMakesTheRoutineAnswerToIt(t *testing.T) {
	rt := routes.Route{App: "trackmania", Focus: true, Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	said, err := d.AddAlias(rt, "Enter Free Play")
	if err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	if len(said.Answers) != 1 || said.Answers[0] != "Enter Free Play" {
		t.Errorf("it answers to %q", said.Answers)
	}
	if !strings.Contains(said.Says, "Enter Free Play") {
		t.Errorf("the sentence does not name the phrase: %q", said.Says)
	}
	// THROUGH THE READER DIRECTOR RESOLVES WITH, not the door: `Resolve` is
	// exact-or-nothing and does not consult phrases, because whether a phrase is
	// unambiguous is a judgement about the whole library.
	if back := d.Reg.AnsweringTo("enter free play"); len(back) != 1 || back[0] != rt {
		t.Errorf("the phrase is answered by %v", back)
	}
}

// AND A PHRASE ALREADY ANSWERING FOR SOMETHING ELSE IS REFUSED, NAMING IT.
//
// The courtesy at the one moment somebody can choose a different phrase. A collision can still
// appear later — a rename, a move, a routine saved afterwards — and `resolveroute` asks about
// those; what this prevents is deliberately creating a phrase whose only possible answer is a
// question.
func TestAPhraseAnotherRoutineIsCalledIsRefused(t *testing.T) {
	mine := routes.Route{App: "trackmania", Focus: true, Slug: "enter-freeplay"}
	theirs := routes.Route{Slug: "settings"}
	d := aLibraryOf(t, mine, theirs)
	_, err := d.AddAlias(mine, "settings")
	if err == nil {
		t.Fatal("a phrase that is another play's own name was accepted")
	}
	if !strings.Contains(err.Error(), "settings") ||
		!strings.Contains(err.Error(), theirs.ID()) {
		t.Errorf("the refusal does not say what it collides with: %v", err)
	}
	// AND NOTHING WAS WRITTEN.
	if d.Reg.HasAliases(mine) {
		t.Error("the refused phrase was stored anyway")
	}
}

// AND A PHRASE ANOTHER ROUTINE ALREADY ANSWERS TO IS REFUSED THE SAME WAY.
func TestAPhraseAnotherRoutineAlreadyAnswersToIsRefused(t *testing.T) {
	mine := routes.Route{Slug: "enter-freeplay"}
	theirs := routes.Route{Slug: "start-race"}
	d := aLibraryOf(t, mine, theirs)
	if _, err := d.AddAlias(theirs, "go racing"); err != nil {
		t.Fatal(err)
	}
	_, err := d.AddAlias(mine, "go racing")
	if err == nil {
		t.Fatal("two plays were allowed to answer to one phrase")
	}
	if !strings.Contains(err.Error(), theirs.ID()) {
		t.Errorf("the refusal does not name the other play: %v", err)
	}
}

// AND EITHER NORMALIZATION COLLIDING IS A COLLISION.
//
// The door folds with `routes.Slug` and Director's resolver compares on `nlu.Key`, which also
// drops hyphens and stop words — so `settings in ark` and `settings-ark` are two phrases to the
// first and ONE phrase to the second. A phrase that is distinct at the door and ambiguous in the
// resolver is a phrase whose meaning depends on which surface was asked.
func TestAPhraseThatCollidesInEitherNormalizationIsRefused(t *testing.T) {
	mine := routes.Route{Slug: "enter-freeplay"}
	theirs := routes.Route{Slug: "settings-ark"}
	d := aLibraryOf(t, mine, theirs)
	// DISTINCT AS SLUGS — `settings-in-ark` is not `settings-ark` — and one phrase to
	// the matcher, which drops the hyphens and the stop word.
	if routes.Slug("settings in ark") == routes.Slug("settings-ark") {
		t.Fatal("the fixture is wrong: these are one phrase at the door too, so " +
			"this would pass without the second check")
	}
	if _, err := d.AddAlias(mine, "settings in ark"); err == nil {
		t.Error("a phrase the resolver cannot tell from another play's name was " +
			"accepted, so what it means would depend on which surface asked")
	}
}

// A ROUTINE'S OWN NAME ADDS NOTHING, AND THAT IS NOT AN ERROR.
//
// The slug already answers before any phrase is consulted, so storing it would store a phrase
// that can never be reached. Said rather than written, and said rather than refused: nobody has
// done anything wrong.
func TestAddingARoutinesOwnNameIsSaidRatherThanStored(t *testing.T) {
	rt := routes.Route{Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	said, err := d.AddAlias(rt, "Enter Freeplay")
	if err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	if len(said.Answers) != 0 {
		t.Errorf("it now answers to %q", said.Answers)
	}
	if !strings.Contains(said.Says, "own name") {
		t.Errorf("the sentence does not say why: %q", said.Says)
	}
	if d.Reg.HasAliases(rt) {
		t.Error("a phrase that can never be reached was written down")
	}
}

// AND ADDING THE SAME PHRASE TWICE SAYS SO AND CHANGES NOTHING.
func TestAddingAPhraseTwiceChangesNothing(t *testing.T) {
	rt := routes.Route{Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	if _, err := d.AddAlias(rt, "go racing"); err != nil {
		t.Fatal(err)
	}
	said, err := d.AddAlias(rt, "GO RACING")
	if err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	if len(said.Answers) != 1 {
		t.Errorf("it answers to %q", said.Answers)
	}
	if !strings.Contains(said.Says, "already answers") {
		t.Errorf("the sentence is %q", said.Says)
	}
}

// A REMOVAL NAMES THE PHRASE, NOT A POSITION.
//
// The step-editing slice learnt what a row number stated against a list that has changed costs: an
// intention landing on a different thing, with nothing afterwards able to catch it. A phrase is
// its own identity, so there is no index to misapply — and removing one leaves the others exactly
// as they were.
func TestAPhraseIsRemovedByNamingIt(t *testing.T) {
	rt := routes.Route{Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	for _, p := range []string{"go racing", "free play", "start a race"} {
		if _, err := d.AddAlias(rt, p); err != nil {
			t.Fatal(err)
		}
	}
	said, err := d.RemoveAlias(rt, "free play")
	if err != nil {
		t.Fatalf("RemoveAlias: %v", err)
	}
	if strings.Join(said.Answers, "|") != "go racing|start a race" {
		t.Errorf("it answers to %q", said.Answers)
	}
	if len(d.Reg.AnsweringTo("free play")) != 0 {
		t.Error("the removed phrase still answers for the play")
	}
	if back := d.Reg.AnsweringTo("start a race"); len(back) != 1 || back[0] != rt {
		t.Error("removing one phrase took another with it")
	}
}

// AND REMOVING A PHRASE IT DOES NOT ANSWER TO IS NOT A FAILURE.
//
// The end state asked for is the end state. A surface whose list is one poll out of date would
// otherwise show somebody a failure for having removed something twice.
func TestRemovingAPhraseThatIsNotThereIsNotAnError(t *testing.T) {
	rt := routes.Route{Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	said, err := d.RemoveAlias(rt, "never added")
	if err != nil {
		t.Fatalf("RemoveAlias: %v", err)
	}
	if !strings.Contains(said.Says, "nothing to remove") {
		t.Errorf("the sentence is %q", said.Says)
	}
}

// A PHRASE WITH NO WORDS IN IT IS REFUSED.
//
// It folds to an empty slug, which would match every request that also folds to nothing.
func TestAPhraseWithNoWordsInItIsRefused(t *testing.T) {
	rt := routes.Route{Slug: "enter-freeplay"}
	d := aLibraryOf(t, rt)
	for _, said := range []string{"", "   ", "!!!", "-- --"} {
		if _, err := d.AddAlias(rt, said); err == nil {
			t.Errorf("%q was accepted as something a play answers to", said)
		}
	}
}

// AND PHRASES LEFT BY A HALF-FINISHED MOVE ARE NOT ADDED TO.
//
// The read every surface goes through refuses an unoffered file rather than returning an empty
// list, and an add that proceeded would write the stale file's phrases back out under this
// routine's binding — adopting phrases somebody attached to something else.
func TestPhrasesBoundElsewhereAreNotAddedTo(t *testing.T) {
	rt := routes.Route{App: "notepad", Slug: "press-b"}
	d := aLibraryOf(t, rt)
	if err := writeFileFor(t, d.Reg.AliasesPath(rt),
		`{"version":1,"route":"ark/focus/other","phrases":["tap bee"]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddAlias(rt, "something new"); err == nil {
		t.Fatal("a play with phrases belonging to something else was added to")
	}
	if _, why := d.AliasesOf(rt); !strings.Contains(why, "ark/focus/other") {
		t.Errorf("the reason does not say where they claim to belong: %q", why)
	}
}

// writeFileFor puts raw bytes where a sidecar goes, which is how a half-finished move is staged.
func writeFileFor(t *testing.T, path, body string) error {
	t.Helper()
	return os.WriteFile(path, []byte(body), 0o644)
}

// THE AUTHORISED VALUES ARE A COPY, SO WHAT RUNS IS WHAT WAS AUTHORISED.
//
// Sealing a map by reference seals nothing: a decision holding the caller's map changes when the
// caller does, and a digest taken at authorisation moves with it. That is the defect ADR-271
// records about a move's `Was` — a stored reading compared against a mutable field — and the
// values a routine types are the worst possible place to repeat it, because the whole point of
// confirming them is that somebody read them.
func TestTheAuthorisedValuesAreACopyOfWhatWasGiven(t *testing.T) {
	given := map[string]string{"message": "as it was shown"}
	r := Resolved{Route: routes.Route{Slug: "write-a-note"}}.WithValues(given)
	// THE CALLER CHANGES ITS MIND AFTER THE DOOR HAS ANSWERED.
	given["message"] = "something else entirely"
	given["smuggled"] = "and another"
	if got := r.Values["message"]; got != "as it was shown" {
		t.Errorf("the authorised value changed to %q when the caller's map did", got)
	}
	if _, ok := r.Values["smuggled"]; ok {
		t.Error("a value added after authorisation reached the authorised decision")
	}
	// AND NO VALUES IS STILL NO VALUES, rather than an empty map nobody can tell from one.
	if plain := (Resolved{}).WithValues(nil); plain.Values != nil {
		t.Errorf("an invocation with no values carries %v", plain.Values)
	}
}
