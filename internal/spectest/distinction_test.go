package spectest_test

import (
	"strings"
	"testing"
)

// Four words, four meanings, and a compiler that holds them apart.
//
// # Why this file exists
//
//	an act    organises what the play can do, and is the way in
//	a scene   describes where things happen, and holds what is there
//	an actor  is a thing in the play
//	a verb    is something one of them does
//
// The first three share one representation, because at run time they behave identically: each
// owns state, declares verbs, and hears and says. That sharing is deliberate and stays.
//
// What must not be shared is the MEANING. A reconciliation audit found that `this exports …`
// compiled on an actor, which meant "act" carried no obligation a reader could rely on — three
// words for one thing, discoverable only by reading the builder. These tests hold the line, and
// they drive the real lex → parse → build → compile pipeline rather than inspecting node types,
// because a distinction that only exists in an AST is a distinction no Marco author can see.

// ── the way in ────────────────────────────────────────────────────────────────

// An act offers capabilities outwards, and nothing else does.
func TestOnlyAnActOffersCapabilitiesOutwards(t *testing.T) {
	const act = `the Ahk is an act.
this exports Click.

the App is a script.

do Ahk's Click...
    when ok?
        log "clicked".
    or?
        log that's error.
`
	if err := compileSource(act); err != nil {
		t.Fatalf("an act that exports does not compile: %v", err)
	}

	for _, word := range []string{"actor", "scene"} {
		src := strings.Replace(act, "is an act.", "is "+articleFor(word)+" "+word+".", 1)
		err := compileSource(src)
		if err == nil {
			t.Errorf("%q exported a capability and compiled. `act` then means nothing: any "+
				"word can be the way in, and a reader has no way to tell which declarations "+
				"a host fulfils", word)
			continue
		}
		if !strings.Contains(err.Error(), "only an act exports") {
			t.Errorf("%q was rejected for the wrong reason: %v", word, err)
		}
		// The message has to teach, not just refuse.
		if !strings.Contains(err.Error(), "this can Click.") {
			t.Errorf("the error for %q does not say what to write instead: %v", word, err)
		}
	}
}

// ── where things happen ───────────────────────────────────────────────────────

// A scene holds what is there and knows verbs of its own.
//
// The containment needs no new syntax: `this's Hero is a Knight.` is the sentence Marco already
// uses to say that something has a thing, and a scene saying it about an actor is a scene
// holding an actor. That is the whole of the concept, and inventing a keyword for it would have
// been inventing a keyword for a sentence the language could already write.
func TestASceneHoldsActorsAndKnowsVerbs(t *testing.T) {
	const src = `the Knight is an actor.
this can Charge.
this's Charge does...
    log "charge".
    this is ok!

the Battlefield is a scene.
this's Hero is a Knight.
this can Begin.
this's Begin does...
    do Knight's Charge.
    this is ok!

the App is a script.

do Battlefield's Begin...
    when ok?
        log "begun".
    or?
        log that's error.
`
	if err := compileSource(src); err != nil {
		t.Fatalf("a scene holding an actor and declaring a verb does not compile: %v", err)
	}

	// Take the actor away and the scene is holding something that is not in the play.
	broken := strings.Replace(src, "the Knight is an actor.\n", "", 1)
	broken = strings.Replace(broken, "this can Charge.\n", "", 1)
	broken = strings.Replace(broken, "this's Charge does...\n    log \"charge\".\n    this is ok!\n", "", 1)
	if err := compileSource(broken); err == nil {
		t.Error("a scene held a Knight that is not in the play, and it compiled. Containment " +
			"that is never checked is decoration")
	}
}

// A scene's verb is a verb: it has a body, it finishes, and the compiler holds it to that.
func TestASceneVerbIsBehaviourAndMustFinish(t *testing.T) {
	const unfinished = `the Battlefield is a scene.
this can Begin.
this's Begin does...
    log "begun".

the App is a script.

do Battlefield's Begin...
    when ok?
        log "ok".
    or?
        log that's error.
`
	err := compileSource(unfinished)
	if err == nil {
		t.Fatal("a scene's verb ran off the end without finishing and compiled. A verb is " +
			"behaviour, and behaviour has an outcome")
	}
	if !strings.Contains(err.Error(), "falls off the end") &&
		!strings.Contains(err.Error(), "resolve") {
		t.Errorf("rejected for the wrong reason: %v", err)
	}
}

// ── things in the play ────────────────────────────────────────────────────────

// An actor is a thing: it holds what belongs to it and does what it can do.
func TestAnActorHoldsItsOwnThingsAndDoesItsOwnVerbs(t *testing.T) {
	const src = `the Sword is a set.
this's Name is a text.

the Knight is an actor.
this's Blade is a Sword.
this can Charge.
this's Charge does...
    log "charge".
    this is ok!

the App is a script.

do Knight's Charge...
    when ok?
        log "charged".
    or?
        log that's error.
`
	if err := compileSource(src); err != nil {
		t.Fatalf("an actor with a thing of its own and a verb of its own does not compile: %v",
			err)
	}

	// A verb it does not have is not a verb it can be asked for.
	missing := strings.Replace(src, "do Knight's Charge...", "do Knight's Retreat...", 1)
	if err := compileSource(missing); err == nil {
		t.Error("an actor was asked for a verb it does not have, and it compiled")
	}
}

// ── the distinction survives the whole pipeline ───────────────────────────────

// The authored word reaches the compiler, and swapping it changes what the program means.
//
// The point of the three-way check: these are not three spellings. Two of the three swaps must
// change the answer, or the words are interchangeable and one of them should go.
func TestSwappingTheWordChangesTheMeaning(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		compiles map[string]bool // declared word → does it compile
	}{{
		name: "offering a capability outwards",
		src: `the Surface is %s.
this exports Click.

the App is a script.

do Surface's Click...
    when ok?
        log "clicked".
    or?
        log that's error.
`,
		compiles: map[string]bool{"an act": true, "a scene": false, "an actor": false},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for word, want := range tc.compiles {
				err := compileSource(strings.Replace(tc.src, "%s", word, 1))
				if got := err == nil; got != want {
					t.Errorf("declared %q: compiles = %v, want %v (%v)", word, got, want, err)
				}
			}
		})
	}
}

func articleFor(word string) string {
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	}
	return "a"
}
