package routes

// WHAT A ROUTINE ANSWERS TO, AND WHAT EVERY OPERATION ON A ROUTINE DOES TO IT.
//
// Narrow and load-bearing, in the same three places the account file's rules are: a phrase file is
// believed only while it still names the routine it sits beside, a file that cannot answer is
// never read as an absent one, and the slug reaches its routine before any phrase is consulted —
// because a name that runs something today keeping on running it is the whole compatibility
// guarantee.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const aliasSrc = "use os.\n\nthe App is a script.\n\ndo OS's Key with \"b\".\n"

// aRoutineAnsweringTo is a routine with a source and the phrases it also answers to.
func aRoutineAnsweringTo(t *testing.T, rt Route, phrases ...string) Registry {
	t.Helper()
	reg := Registry{Dir: t.TempDir()}
	if err := reg.Save(rt, aliasSrc); err != nil {
		t.Fatal(err)
	}
	if err := reg.SaveAliases(rt, phrases); err != nil {
		t.Fatal(err)
	}
	return reg
}

// A PHRASE IS KEPT AS SOMEBODY TYPED IT.
//
// The folding happens when phrases are COMPARED, not when they are stored, so the list read back
// is the list that was written. A person who typed "enter free play" is not shown
// `enter-free-play` and asked to recognise it.
func TestPhrasesAreStoredAsTheyWereTyped(t *testing.T) {
	rt := Route{App: "trackmania", Focus: true, Slug: "enter-freeplay"}
	reg := aRoutineAnsweringTo(t, rt, "Enter Free Play", "go racing")
	s, st := reg.Aliases(rt)
	if st != AliasesIntact {
		t.Fatalf("the phrases read as %q", st)
	}
	if strings.Join(s.Phrases, "|") != "Enter Free Play|go racing" {
		t.Errorf("the phrases came back as %q", s.Phrases)
	}
	// AND BOUND TO THE ROUTINE THEY SIT BESIDE, which is the only binding there is.
	if s.Route != rt.ID() {
		t.Errorf("the phrases say they belong to %q, not %q", s.Route, rt.ID())
	}
}

// AND A FILE THAT CANNOT ANSWER IS NOT AN ABSENT FILE.
//
// The correction the account file carries, in the place it matters here: absence means a routine
// answers to its own name and nothing else, which is what every routine already on disk is — so
// reading an unreadable file as absence would quietly drop somebody's phrases the next time
// anything wrote the file.
func TestAnUnreadablePhraseFileIsNotTreatedAsNone(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := os.WriteFile(reg.AliasesPath(rt), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Aliases(rt); st != AliasesUnreadable {
		t.Errorf("a file that cannot be parsed reads as %q", st)
	}
	// AND A VERSION THIS BUILD DOES NOT KNOW IS THE SAME ANSWER.
	if err := os.WriteFile(reg.AliasesPath(rt),
		[]byte(`{"version":99,"route":"notepad/context/press-b","phrases":["x"]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Aliases(rt); st != AliasesUnreadable {
		t.Errorf("a version from the future reads as %q", st)
	}
	// AND NEITHER OF THEM ANSWERS ANYTHING.
	if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
		t.Errorf("an unreadable file answered for %v", got)
	}
}

// AND A FILE THAT COULD NOT BE READ AT ALL IS THE SAME ANSWER.
//
// WRITTEN FOR A SURVIVOR. The case above is a file that WAS read and would not parse, which takes
// a different branch — so treating a read FAILURE as absence survived every assertion in it. The
// honest answer to a question that could not be asked is that it could not be asked: a permission,
// a handle something else is holding, or a directory standing where the file goes.
func TestAPhraseFileThatCannotBeReadIsNotTreatedAsNone(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := Registry{Dir: t.TempDir()}
	if err := reg.Save(rt, aliasSrc); err != nil {
		t.Fatal(err)
	}
	// A DIRECTORY WHERE THE FILE GOES, which is the one unreadable state a test can
	// stage portably.
	if err := os.MkdirAll(reg.AliasesPath(rt), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Aliases(rt); st != AliasesUnreadable {
		t.Errorf("a file that could not be read reads as %q, and absence is the one "+
			"answer that licenses writing over it", st)
	}
}

// AND A FILE BOUND TO ANOTHER IDENTITY IS NOT BELIEVED.
//
// The hazard this binding exists for: a phrase file left under a slug — by a move whose rewrite
// did not land, or by a routine that used to be saved there — would otherwise answer for whatever
// is saved there next. Fail-closed, because the cost of believing it is a phrase that runs a
// routine nobody attached it to.
func TestPhrasesBoundToAnotherRoutineAnswerForNothing(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := os.WriteFile(reg.AliasesPath(rt), []byte(
		`{"version":1,"route":"ark/focus/something-else","phrases":["tap bee"]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	s, st := reg.Aliases(rt)
	if st != AliasesElsewhere {
		t.Fatalf("a file naming another identity reads as %q", st)
	}
	// THE PHRASES ARE STILL READ BACK, because a surface refusing to offer them wants
	// to say what the file claims.
	if len(s.Phrases) != 1 || s.Route != "ark/focus/something-else" {
		t.Errorf("the file came back as %+v", s)
	}
	if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
		t.Errorf("it answered for %v anyway", got)
	}
}

// AND PHRASES WITH NO ROUTINE BESIDE THEM ANSWER FOR NOTHING EITHER.
func TestPhrasesWithNoRoutineAreOrphaned(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := os.Remove(reg.Path(rt)); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Aliases(rt); st != AliasesOrphaned {
		t.Errorf("phrases with no source beside them read as %q", st)
	}
}

// THE SLUG REACHES ITS ROUTINE BEFORE ANY PHRASE IS CONSULTED.
//
// The compatibility guarantee, and the only reason an alias is safe to add to a library people
// already depend on: a name that runs something today keeps running it, and no routine can lose
// its own name to another one's phrase. The same ordering, for the same reason, as the
// demonstration arm sitting after the play arm at the door.
func TestARoutinesOwnNameBeatsAnotherRoutinesPhrase(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	mine := Route{Slug: "settings"}
	thief := Route{Slug: "options"}
	for _, rt := range []Route{mine, thief} {
		if err := reg.Save(rt, aliasSrc); err != nil {
			t.Fatal(err)
		}
	}
	if err := reg.SaveAliases(thief, []string{"settings"}); err != nil {
		t.Fatal(err)
	}
	got, ok := reg.Resolve("", "settings")
	if !ok {
		t.Fatal("nothing answered to settings")
	}
	if got != mine {
		t.Errorf("settings resolved to %q, which is not the routine CALLED settings",
			got.ID())
	}
}

// AND THIS DOOR DOES NOT RESOLVE A PHRASE AT ALL.
//
// It did, by the same scope priority as a slug, and that put the decision in the one place that
// cannot ask a question: a phrase two routines answer to was selected by priority and performed,
// while Director — asked the same words against the same library — asks which was meant. One
// phrase, two answers, and the one that acted was the one that could not ask.
//
// So `Resolve` is exact-or-nothing, as it was before phrases existed, and `AnsweringTo` is what
// Director reads. The phrase is still there and still found; what has gone is this function
// choosing between the routines that answer to it.
func TestTheDoorDoesNotResolveAPhraseItself(t *testing.T) {
	rt := Route{Slug: "enter-freeplay"}
	reg := aRoutineAnsweringTo(t, rt, "go racing")
	if got, ok := reg.Resolve("", "go racing"); ok {
		t.Errorf("the door selected %q for a phrase, which is Director's judgement "+
			"to make", got.ID())
	}
	// AND THE PHRASE IS STILL FOUND, by the reader Director uses.
	if got := reg.AnsweringTo("go racing"); len(got) != 1 || got[0] != rt {
		t.Errorf("the phrase is answered by %v", got)
	}
	// AND THE ROUTINE'S OWN NAME IS UNTOUCHED, which is the half this must not cost.
	if got, ok := reg.Resolve("", "enter freeplay"); !ok || got != rt {
		t.Errorf("its own name resolved to %q", got.ID())
	}
}

// AND A PHRASE TWO ROUTINES ANSWER TO SELECTS NOTHING.
//
// The case that made the branch wrong, stated as its own claim: whatever else happens, this door
// does not pick one of them.
func TestASharedPhraseSelectsNothingAtTheDoor(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	one := Route{App: "arkascended", Focus: true, Slug: "ark-settings"}
	two := Route{App: "trackmania", Focus: true, Slug: "trackmania-settings"}
	for _, rt := range []Route{one, two} {
		if err := reg.Save(rt, aliasSrc); err != nil {
			t.Fatal(err)
		}
		if err := reg.SaveAliases(rt, []string{"settings"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, app := range []string{"", "arkascended", "trackmania", "notepad"} {
		if got, ok := reg.Resolve(app, "settings"); ok {
			t.Errorf("from %q a phrase two routines answer to selected %q",
				app, got.ID())
		}
	}
}

// EVERY ROUTINE THAT ANSWERS TO A PHRASE IS LISTED, WHATEVER ITS SCOPE.
//
// This reader does not filter and does not choose: eligibility is about the moment a request is
// made — which application is in front, and whether that may be relied on — and this package
// knows none of that. `resolveroute.Eligible` applies the scope rules and `resolveroute` asks
// which when more than one survives them.
func TestEveryRoutineAnsweringToAPhraseIsListed(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	here := Route{App: "trackmania", Slug: "trackmania-settings"}
	elsewhere := Route{App: "arkascended", Focus: true, Slug: "ark-settings"}
	global := Route{Slug: "any-settings"}
	for _, rt := range []Route{here, elsewhere, global} {
		if err := reg.Save(rt, aliasSrc); err != nil {
			t.Fatal(err)
		}
		if err := reg.SaveAliases(rt, []string{"open the settings"}); err != nil {
			t.Fatal(err)
		}
	}
	got := reg.AnsweringTo("open the settings")
	if len(got) != 3 {
		t.Fatalf("%d routines answer to the phrase, not the three saved: %v",
			len(got), got)
	}
	// SORTED, because `List` is: a listing whose order changes between reads is a
	// question whose candidates change between reads.
	if reg.AnsweringTo("open the settings")[0] != got[0] {
		t.Error("the listing is not stable between reads")
	}
}

// AND A PHRASE MORE THAN ONE ROUTINE ANSWERS TO IS REPORTED AS BOTH.
//
// This door answers exactly or not at all and has nowhere to ask a question, so it resolves by
// priority — but what it knows is that both answered, and `AnsweringTo` says so. Turning that
// into a question is `resolveroute`'s job, and it needs the whole list to do it.
func TestAPhraseTwoRoutinesAnswerToIsReportedAsBoth(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	one := Route{App: "arkascended", Focus: true, Slug: "ark-settings"}
	two := Route{App: "trackmania", Focus: true, Slug: "trackmania-settings"}
	for _, rt := range []Route{one, two} {
		if err := reg.Save(rt, aliasSrc); err != nil {
			t.Fatal(err)
		}
		if err := reg.SaveAliases(rt, []string{"settings"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := reg.AnsweringTo("settings"); len(got) != 2 {
		t.Errorf("%d routines answer to the phrase, not 2: %v", len(got), got)
	}
}

// AND THE FOLDING IS THE SAME ONE A NAME GETS.
//
// Which is what makes an alias EXACT rather than a guess, and is why it needs nothing running: the
// phrase and the request pass through `Slug` on both sides, so case, punctuation and runs of
// whitespace are already handled by the normalization the product has always had.
func TestAPhraseIsMatchedByTheSameFoldingANameIs(t *testing.T) {
	rt := Route{Slug: "enter-freeplay"}
	reg := aRoutineAnsweringTo(t, rt, "Enter Free Play")
	for _, said := range []string{"enter free play", "ENTER FREE PLAY",
		"  enter   free play  ", "enter-free-play", "\"enter free play\""} {
		got := reg.AnsweringTo(said)
		if len(got) != 1 || got[0] != rt {
			t.Errorf("%q answered %v", said, got)
		}
	}
}

// RENAMING A ROUTINE CARRIES THE PHRASES AND REWRITES WHAT THEY ARE BOUND TO.
//
// Both halves, because the file alone is not enough: it is bound to the identity it was written
// beside, so a file that was only MOVED reads as belonging somewhere else and the phrases silently
// stop answering. And a file left behind is one the next routine saved under the old slug would
// inherit.
func TestRenamingARoutineCarriesThePhrasesItAnswersTo(t *testing.T) {
	from := Route{App: "trackmania", Focus: true, Slug: "trackmania-settings"}
	to := Route{App: "trackmania", Focus: true, Slug: "tm-settings"}
	reg := aRoutineAnsweringTo(t, from, "open the settings")
	if err := reg.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	s, st := reg.Aliases(to)
	if st != AliasesIntact {
		t.Fatalf("after the rename the phrases read as %q (bound to %q)", st, s.Route)
	}
	if len(s.Phrases) != 1 || s.Phrases[0] != "open the settings" {
		t.Errorf("the phrases came back as %q", s.Phrases)
	}
	// AND THE PHRASE STILL ANSWERS FOR THE ROUTINE AT ITS NEW NAME.
	if got := reg.AnsweringTo("open the settings"); len(got) != 1 || got[0] != to {
		t.Errorf("after the rename the phrase answers %v", got)
	}
	// AND NOTHING IS LEFT UNDER THE OLD SLUG.
	if fileExists(reg.AliasesPath(from)) {
		t.Error("a phrase file was left behind for the next routine saved there")
	}
}

// DELETING A ROUTINE TAKES AWAY WHAT IT ANSWERED TO.
func TestDeletingARoutineTakesAwayItsPhrases(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := reg.Delete(rt); err != nil {
		t.Fatal(err)
	}
	if fileExists(reg.AliasesPath(rt)) {
		t.Error("the phrases outlived the routine they answered for")
	}
	if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
		t.Errorf("the phrase still answers, for %v", got)
	}
}

// AND A MOVE'S SECOND HALF SWEEPS THEM AND SAYS WHAT IT COULD NOT.
//
// `DeleteLeftovers` is not allowed to fail a move — by the time it runs the routine is gone — but
// litter that cannot be swept is still worth saying, because a phrase file left under a slug is
// one the next routine saved there would inherit.
func TestAMoveSweepsThePhrasesAndReportsWhatItCouldNot(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := reg.DeleteSource(rt); err != nil {
		t.Fatal(err)
	}
	if residue := reg.DeleteLeftovers(rt); len(residue) != 0 {
		t.Errorf("a clean sweep reported %q", residue)
	}
	if fileExists(reg.AliasesPath(rt)) {
		t.Error("the phrases were left behind")
	}
}

// AND A ROUTINE THAT ANSWERS TO NOTHING HAS NO FILE AT ALL.
//
// An empty list removes the file rather than writing one that claims nothing: a file with no
// phrases in it is unreadable by the rules above, so writing one would be writing a state every
// reader has to refuse.
func TestARoutineWithNoPhrasesHasNoFile(t *testing.T) {
	rt := Route{Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := reg.SaveAliases(rt, nil); err != nil {
		t.Fatal(err)
	}
	if reg.HasAliases(rt) {
		t.Error("a routine answering to nothing still has a phrase file")
	}
	if _, st := reg.Aliases(rt); st != AliasesNone {
		t.Errorf("it reads as %q", st)
	}
	// AND A LIST OF NOTHING BUT BLANKS IS THE SAME THING.
	if err := reg.SaveAliases(rt, []string{"", "   "}); err != nil {
		t.Fatal(err)
	}
	if reg.HasAliases(rt) {
		t.Error("blank phrases were written down")
	}
}

// A FRESH ROUTINE AT AN IDENTITY DOES NOT INHERIT WHAT THE LAST ONE ANSWERED TO.
//
// The binding is to a PLACE, not to a thing, and that is what makes this possible: delete a
// routine whose phrase file cannot be swept — `DeleteLeftovers` reports it and is not allowed to
// fail a move over it — and the file sits there bound to an identity with nothing in it. While
// there is no source it is orphaned and answers nothing. The moment a different routine is saved
// at that identity, the binding matches again.
//
// So creation clears it. The phrases belonged to a routine that no longer exists, and the person
// saving this one has never heard of them.
func TestAFreshRoutineDoesNotInheritLeftoverPhrases(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	// THE SOURCE GOES AND THE PHRASES DO NOT, which is the state a sweep that could not
	// remove the file leaves behind.
	if err := reg.DeleteSource(rt); err != nil {
		t.Fatal(err)
	}
	if !reg.HasAliases(rt) {
		t.Fatal("the fixture swept the file, so there is no residue to inherit")
	}
	if _, st := reg.Aliases(rt); st != AliasesOrphaned {
		t.Fatalf("with no source there, the file reads as %q", st)
	}
	// AND A DIFFERENT ROUTINE IS SAVED AT THE SAME IDENTITY.
	if err := reg.Create(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"z\".\n"); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Aliases(rt); st != AliasesNone {
		t.Errorf("the new routine's phrases read as %q, so it answers to what the "+
			"routine that used to be here was taught", st)
	}
	if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
		t.Errorf("the old phrase answers for %v", got)
	}
}

// AND A REPLACEMENT KEEPS THEM, which is the opposite answer and the same reason.
//
// `Replace` writes over a routine that IS there, so the phrases belong to the routine being
// replaced rather than to one that is gone — and `Keep`'s collision sentence says so, because
// nobody would guess it from a sentence about files being overwritten.
func TestAReplacementKeepsThePhrasesTheIdentityAnswersTo(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := reg.Replace(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"z\".\n"); err != nil {
		t.Fatal(err)
	}
	if got := reg.AnsweringTo("tap bee"); len(got) != 1 || got[0] != rt {
		t.Errorf("after a replacement the phrase answers %v", got)
	}
}

// NO INSTANT LETS A FRESH IDENTITY ANSWER TO AN ORPHAN.
//
// The ORDER of two writes, which neither end state can tell apart: before the creation there is no
// routine, after it there are no phrases, and both are true whichever way round the two steps
// happen. What differs is the middle. Clearing the orphan first means no instant has both; doing
// it afterwards means one does — the routine exists, the binding matches, and a resolver reading
// just then offers a phrase somebody attached to whatever used to be saved there.
//
// So this looks between them, through the one seam that can. At the instant before the routine
// exists the file is already gone, which is the claim: there is nothing left for the source to
// make match.
func TestNoInstantLetsAFreshIdentityAnswerToAnOrphan(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := reg.DeleteSource(rt); err != nil {
		t.Fatal(err)
	}
	if !reg.HasAliases(rt) {
		t.Fatal("the fixture left no residue, so there is nothing to inherit")
	}
	looked := false
	reg.beforeSource = func() {
		looked = true
		// THE ORPHAN IS ALREADY GONE, which is what clearing first means. With the
		// order the other way round this file is still here and the source is
		// about to appear beside it.
		if reg.HasAliases(rt) {
			t.Error("the phrase file is still there as the routine is about to " +
				"exist, so there is an instant in which it answers for it")
		}
		// AND A CONCURRENT RESOLVER IS OFFERED NOTHING, asked the way Director
		// asks rather than by reading the file.
		if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
			t.Errorf("a resolver reading at this instant is offered %v", got)
		}
		if _, st := reg.Aliases(rt); st != AliasesNone {
			t.Errorf("the identity's phrases read as %q at this instant", st)
		}
	}
	if err := reg.Create(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"z\".\n"); err != nil {
		t.Fatal(err)
	}
	if !looked {
		t.Fatal("the instant between the two writes was never reached, so this " +
			"asserted nothing")
	}
	// AND AFTERWARDS, which the end state already said and is cheap to keep.
	if got := reg.AnsweringTo("tap bee"); len(got) != 0 {
		t.Errorf("the new routine answers to %v", got)
	}
}

// A REFUSED CREATION CHANGES NOTHING, PHRASES INCLUDED.
//
// The control for the clear above, and the case that makes its ORDER a question of more than
// instants. `O_EXCL` is what makes a creation exclusive and it answers last, so a clear placed
// before it ran at an identity that already had a routine: that routine's own legitimate phrases
// were removed, and then the creation refused with "already exists". A refusal that has changed
// the library is not a refusal.
//
// And this is the path rather than a corner of it: `Keep` reaches `Create` exactly when its own
// collision check found the place free and something appeared since, which is the race `O_EXCL`
// exists for.
func TestARefusedCreationLeavesTheRoutineAndItsPhrasesAlone(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee", "hit the b key")
	srcWas, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	aliasWas, err := os.ReadFile(reg.AliasesPath(rt))
	if err != nil {
		t.Fatal(err)
	}

	err = reg.Create(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"z\".\n")
	if err == nil {
		t.Fatal("a routine was created over one that was already there")
	}
	// THE ANSWER CALLERS ALREADY READ. `Keep` and `Rescope` both ask `os.IsExist`, and
	// a different shape of error would turn a collision into a failure.
	if !os.IsExist(err) {
		t.Errorf("the refusal does not read as a collision: %v", err)
	}
	// AND BOTH FILES ARE BYTE FOR BYTE WHAT THEY WERE.
	if srcNow, _ := os.ReadFile(reg.Path(rt)); string(srcNow) != string(srcWas) {
		t.Error("the refused creation rewrote the routine that was there")
	}
	aliasNow, err := os.ReadFile(reg.AliasesPath(rt))
	if err != nil {
		t.Fatalf("the refused creation removed the phrases of the routine that was "+
			"there: %v", err)
	}
	if string(aliasNow) != string(aliasWas) {
		t.Errorf("the phrases were rewritten: %q, were %q", aliasNow, aliasWas)
	}
	// AND THE ROUTINE STILL ANSWERS TO BOTH OF THEM.
	for _, said := range []string{"tap bee", "hit the b key"} {
		if got := reg.AnsweringTo(said); len(got) != 1 || got[0] != rt {
			t.Errorf("%q is answered by %v", said, got)
		}
	}
}

// AND ANYTHING STANDING WHERE THE SOURCE GOES REFUSES THE CREATION, PHRASES INTACT.
//
// RENAMED AFTER A MUTATION SHOWED WHAT IT ACTUALLY REACHES. It was written for the third answer —
// a stat that says neither yes nor no — and a directory at the source path makes `os.Stat`
// SUCCEED, so it takes the first. That is worth a case of its own anyway: whatever is at that
// path, it is not this creation's to clear.
//
// The third answer is defensive and nothing here reaches it. A stat failing with something other
// than "not there" needs a path this platform cannot answer about, and `os.MkdirAll` above refuses
// the ones a test could build. It is kept because the DIRECTION of the guess is what matters:
// treating an unanswerable stat as absence is what clears somebody's phrases and then refuses.
func TestAnythingAtTheSourcePathRefusesTheCreation(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	if err := os.Remove(reg.Path(rt)); err != nil {
		t.Fatal(err)
	}
	// A DIRECTORY WHERE THE `.marco` GOES, with something in it so it cannot be removed
	// either.
	if err := os.MkdirAll(filepath.Join(reg.Path(rt), "in the way"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(rt, "use os.\n"); err == nil {
		t.Fatal("a creation went ahead with something already at the source path")
	}
	// AND THE PHRASES ARE UNTOUCHED, which is what the ordering is protecting.
	if !reg.HasAliases(rt) {
		t.Error("the phrases were cleared for a creation that then refused")
	}
}

// AND THE EXCLUSIVE CREATE IS STILL WHAT ANSWERS THE RACE.
//
// WRITTEN FOR A SURVIVOR, and the survivor was the whole point of `O_EXCL`: with the source
// checked first, every case a test could stage is refused before the open, so swapping `O_EXCL`
// for `O_TRUNC` changed nothing anywhere in this package. The check is not a substitute for the
// exclusive create — it removes the destructive clear for every writer that takes the route's
// write lock, and the create remains the answer for one that does NOT: a text editor, a sync,
// another tool, appearing between the two steps.
//
// That writer is what `beforeSource` stands in for here. It is the only instant it can happen in.
func TestARoutineThatAppearsDuringACreationIsNotOverwritten(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := Registry{Dir: t.TempDir()}
	const theirs = "use os.\n\nthe App is a script.\n\ndo OS's Key with \"someone else\".\n"
	// SOMETHING THAT DOES NOT TAKE THE LOCK SAVES A ROUTINE HERE, after the source was
	// found absent and before this creation writes.
	reg.beforeSource = func() {
		if err := os.MkdirAll(filepath.Dir(reg.Path(rt)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reg.Path(rt), []byte(theirs), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := reg.Create(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"mine\".\n")
	if err == nil {
		t.Fatal("a routine that appeared mid-creation was overwritten")
	}
	if !os.IsExist(err) {
		t.Errorf("the refusal does not read as a collision: %v", err)
	}
	// AND THEIRS IS WHAT IS ON DISK, byte for byte.
	got, readErr := os.ReadFile(reg.Path(rt))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != theirs {
		t.Errorf("the routine on disk is %q", got)
	}
}
