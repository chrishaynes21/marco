package marcoexec

import (
	"fmt"
	"strings"
)

// Writing down what Marco learned, as a play somebody reads.
//
// # Why this lives here and not in a new package
//
// Because this package already owns the whole lowering: a typed intention in, legal Marco out,
// one encoder, one set of escaping rules. `internal/codegen` is the LEARN-side generator and is
// forbidden to the Director for good reasons; a third generator would be a third set of escaping
// bugs and a third opinion about what legal Marco looks like.
//
// # What a learned play is allowed to contain
//
// Navigation meanings, in order, and nothing else. No subject ids, no screen states, no digests,
// no window handles, no counts, no verdicts, no comments about how Director came to believe any
// of it. The whole promise of the language is that the audience reads the script and the
// machinery stays backstage — see [[Core#rule]] and
// [[ADR-027-what-marco-learned-becomes-marco]].
//
// # And it is inert
//
// This produces TEXT. It writes no file, registers nothing, and hands back nothing that can run.
// Generating Marco is not authorization to run Marco.

// ProvisionalActor is the name a learned play carries before a person has named it.
//
// Deliberately unmistakable. Core v1 needs a capitalised declaration name and Director has no
// business inventing a meaningful one — guessing "OpenAudioSettings" from a screen's text would
// be exactly the leak this milestone exists to prevent, and guessing from the application would
// name the play after the wrong thing.
//
// `UnnamedShortcut` says the true thing twice over: it is a shortcut, and nobody has named it.
// It also reads as a noun in the sentence Core actually writes — `the UnnamedShortcut is an
// actor.` — which a bare adjective would not. A person renaming it is a text edit in a file they
// can read, which is the point of the file being Marco.
const ProvisionalActor = "UnnamedShortcut"

// ProvisionalVerb is what a learned play's one capability is called until somebody says otherwise.
//
// `Run` is Core's own example verb and reads correctly in the sentence that calls it:
// `do UnnamedShortcut's Run...`.
const ProvisionalVerb = "Run"

// LowerPlay writes a verified procedure as an ordinary Marco program.
//
// `steps` is one ordered run of navigation meanings per step of the procedure. Order is preserved
// exactly and repeats are preserved exactly: `down, down, confirm` is three presses and two of
// them are the same word, and a set or a dedup would leave the selection one row short of where
// the demonstration went.
//
// Every meaning is checked against the closed vocabulary before anything is written. A meaning
// Core has no sentence for is a LANGUAGE-EXPRESSION GAP reported as an error, never a sentence
// invented to make lowering convenient.
//
// The result is deterministic: the same procedure produces byte-identical source, because nothing
// session-shaped, numbered or map-ordered reaches this function.
func LowerPlay(steps [][]string) (string, error) {
	return lowerPlay(ProvisionalActor, ProvisionalVerb, "", "", Navigations(steps))
}

// StartsOn is the sentence a play uses to say where it begins.
//
// # Why this is not new syntax
//
// Because Marco already says it. `do <Act>'s <Cap> with "…"... when ok? … or? …` is Core v1
// composition — the same shape `OS's Focus` has always used to mean "ok if the active window
// matches". What was missing was not a language construct but a CAPABILITY: nothing could answer
// "is the screen the user named the one in front?".
//
// So Core v1 is unchanged and `screen.marco` declares one read-only export. See
// [[ADR-030-a-play-says-where-it-begins]].
const StartsOn = "Screen's Showing"

// EndsOn is the sentence a play uses to say where it expects to finish.
//
// The SAME capability, asked a second time. That is the point: nothing new was needed in the
// language or in the act to check a screen after the effects rather than before them, because
// "is the screen the user named the one in front?" is the same question whenever it is asked.
// See [[ADR-032-a-play-says-where-it-ends]].
const EndsOn = "Screen's Showing"

// LowerNamedPlay writes the same play with the names a person chose.
//
// # Why naming REGENERATES rather than edits
//
// Because a rename by string replacement is a rename that can change the procedure. `Run` appears
// in a declaration, in a body header and in a call sentence; a careless replacement would also hit
// a navigation meaning that happened to share the word, and the play would still compile. Building
// the file again from the same ordered meanings cannot do that — the steps are the input, and a
// name is not.
//
// Both names exist for a reason a novice can hold. The ACTOR is the thing in the play: what it is.
// The VERB is what it does. `do Volume's Mute...` reads as a sentence because it is one.
func LowerNamedPlay(actor, verb string, steps [][]string) (string, error) {
	return LowerPlayBetween(actor, verb, "", "", steps)
}

// LowerProvisionalPlayBetween writes the guarded, postconditioned play under placeholder names.
//
// For the PREVIEW: `director learned` shows what would be written before anybody has named the
// play, and the placeholder is legitimate there. Saving goes through LowerPlayBetween, which
// refuses the placeholder — naming the behaviour can wait, and naming it durably cannot.
func LowerProvisionalPlayBetween(startsOn, endsOn string, steps [][]string) (string, error) {
	return lowerPlay(ProvisionalActor, ProvisionalVerb, startsOn, endsOn, Navigations(steps))
}

// LowerPlayBetween writes a named play that begins only on `startsOn` and succeeds only on `endsOn`.
//
// Both are what the USER calls those screens — the same kind of name they gave the play, resolved
// from durable memory by the judgement. An empty pair writes a play with no entry condition and no
// postcondition, which is what an unguarded play is; a LEARNED play is never lowered that way,
// because Director always knows where its route began and where the rehearsal ended up.
func LowerPlayBetween(actor, verb, startsOn, endsOn string, steps [][]string) (string, error) {
	if err := CheckPlayName(actor); err != nil {
		return "", fmt.Errorf("the name for the thing itself: %w", err)
	}
	if err := CheckPlayName(verb); err != nil {
		return "", fmt.Errorf("the name for what it does: %w", err)
	}
	return lowerPlay(actor, verb, startsOn, endsOn, Navigations(steps))
}

func lowerPlay(actor, verb, startsOn, endsOn string, steps [][]PlayAction) (string, error) {
	if len(steps) == 0 {
		return "", fmt.Errorf("marcoexec: a play needs something to do")
	}
	invokes := false
	for _, run := range steps {
		if len(run) == 0 {
			return "", fmt.Errorf("marcoexec: a step needs at least one action")
		}
		for _, a := range run {
			if a.Invokes() {
				invokes = true
				continue
			}
			if !NavigationMeanings[a.Intent] {
				return "", fmt.Errorf(
					"marcoexec: Marco has no sentence for %q, so this cannot be written down",
					a.Intent)
			}
		}
	}

	var b strings.Builder
	// The only comment, and it is about the READER's options rather than about Director.
	// Somebody opening this file should learn two things immediately: where it came from in
	// one plain phrase, and that it is theirs to change.
	b.WriteString("// Marco learned this by watching. Rename it and change it however you like.\n")
	b.WriteString("use os.\n")
	// Either claim needs the Screen act. A learned play always has both, but a play with only
	// one still has to compile rather than emit a sentence for an act it never imported.
	if startsOn != "" || endsOn != "" {
		b.WriteString("use screen.\n")
	}
	// The Accessibility act, imported only when the play actually presses a control by name.
	// A play that never does must not import an act it never uses.
	if invokes {
		b.WriteString("use theater.\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "the %s is an actor.\n\n", actor)
	fmt.Fprintf(&b, "this can %s.\n", verb)
	fmt.Fprintf(&b, "this's %s does...\n", verb)
	// THE entry condition, and the steps live INSIDE its `when ok?` arm.
	//
	// # Why nested and not an early return
	//
	// An earlier shape asked the question first and let the `or?` arm return, with the steps
	// after the block. It compiled — and it did not guard: a return inside an arm ends the
	// arm, not the capability, so execution walked on to the steps anyway. Compiling is not
	// behaving, and the test that only read the source would have shipped it.
	//
	// Nesting makes it structural. There is no line after the block for control to fall
	// through to, so the only path to an effect is through an `ok`.
	step := "    "
	var why string
	if startsOn != "" {
		where, err := Quote(startsOn)
		if err != nil {
			return "", err
		}
		if why, err = Quote("this play starts on " + startsOn); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "    do %s with %s...\n", StartsOn, where)
		b.WriteString("        when ok?\n")
		step = "            "
	}
	ctl := 0
	for _, run := range steps {
		for _, a := range run {
			if a.Invokes() {
				// A target needs a LOCAL to carry it: an act takes one input and a
				// set is built before it is passed. Numbered, so a play that
				// activates two targets declares two locals rather than redeclaring
				// one — which would be a different program that happened to compile.
				ctl++
				called, err := Quote(a.Called)
				if err != nil {
					return "", err
				}
				// KIND is written only when the demonstration knew one. An empty
				// Kind means "whatever it is", and writing `Kind ""` would say
				// something different and wrong — the same rule the Control set
				// follows for an omitted Window.
				if a.Kind != "" {
					kind, err := Quote(a.Kind)
					if err != nil {
						return "", err
					}
					fmt.Fprintf(&b, "%sthe target%d is a Target with Name %s, Kind %s.\n",
						step, ctl, called, kind)
				} else {
					fmt.Fprintf(&b, "%sthe target%d is a Target with Name %s.\n",
						step, ctl, called)
				}
				fmt.Fprintf(&b, "%sdo Theater's Activate with target%d.\n", step, ctl)
				continue
			}
			name, err := Quote(a.Intent)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "%sdo OS's Navigate with %s.\n", step, name)
		}
	}
	// THE postcondition, and the only route to `ok!` when there is one.
	//
	// # Why success moves inside it
	//
	// Because every key going out successfully is not the same thing as the application having
	// arrived. A host can send `down` and `confirm` perfectly while a dialog eats them, a
	// frame drops, or the menu had one more row than the demonstration did — and a play that
	// reported success there would be lying about the only thing anybody cares about.
	//
	// It is nested for the same structural reason the entry condition is: there is no line
	// after the block for control to fall through to, so there is no path to `ok!` that has
	// not positively identified the screen the play said it would finish on.
	if endsOn != "" {
		where, err := Quote(endsOn)
		if err != nil {
			return "", err
		}
		wrong, err := Quote("this play expected to finish on " + endsOn)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%sdo %s with %s...\n", step, EndsOn, where)
		fmt.Fprintf(&b, "%s    when ok?\n", step)
		fmt.Fprintf(&b, "%s        this is ok!\n", step)
		fmt.Fprintf(&b, "%s    or?\n", step)
		fmt.Fprintf(&b, "%s        this is failed with error %s!\n", step, wrong)
	} else {
		fmt.Fprintf(&b, "%sthis is ok!\n", step)
	}
	if startsOn != "" {
		b.WriteString("        or?\n")
		// `!` and not `.`: the four endings are exclamations in Core, and a period leaves
		// the arm without an ending at all.
		fmt.Fprintf(&b, "            this is failed with error %s!\n", why)
	}
	b.WriteString("\n")
	b.WriteString("the App is a script.\n\n")
	fmt.Fprintf(&b, "do %s's %s...\n", actor, verb)
	b.WriteString("    when ok?\n")
	b.WriteString("        log \"done\".\n")
	b.WriteString("    or?\n")
	b.WriteString("        log that's error.\n")
	return b.String(), nil
}

// CheckPlayName holds a chosen name to Marco's own rules.
//
// # Why this is not a second identifier grammar
//
// It is not one. Marco's rule is written down in one sentence in Core: *a lowercase name is a
// local; a capitalised one is a declaration.* A play's actor and verb are declarations, so they are
// capitalised words — and the compiler is still the authority, because a name that gets past this
// and produces something Marco will not accept fails at compile time like anything else.
//
// What this adds is a READABLE refusal before that happens. "a name is one word, starting with a
// capital letter" is something a person can act on; a parse error at line 5, column 12 is not.
func CheckPlayName(name string) error {
	if name == "" {
		return fmt.Errorf("it needs a name")
	}
	if r := rune(name[0]); r < 'A' || r > 'Z' {
		return fmt.Errorf("%q must start with a capital letter, like Volume or Mute", name)
	}
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		default:
			return fmt.Errorf("%q must be one word of letters and numbers, "+
				"like Volume or MuteEveryone", name)
		}
	}
	if name == ProvisionalActor || name == ProvisionalVerb+"AndForget" {
		return fmt.Errorf("%q is the placeholder Marco uses before anybody has named it", name)
	}
	return nil
}

// PlayAction is one thing a generated play does.
//
// # Two kinds, because a demonstration contains two kinds
//
// A keypress is a MEANING. The person confirmed; which key that is belongs to the host, which is
// the acting half of [[ADR-013-navigation-is-meaning-not-keys]].
//
// A click is aimed at a THING, and until now a play could not say which. That was not a gap in
// what Marco saw — the control's label is resolved while watching and kept with the demonstration
// — but in what Marco could WRITE: the only way to name a control was the accessibility source's
// own id, which identifies it in the tree as it stands now and means nothing after a redraw. A
// play holds the label instead, and the host resolves it against what is on screen when it runs.
//
// Exactly one field is set.
type PlayAction struct {
	// Intent is a navigation meaning, from NavigationMeanings.
	Intent string
	// Called is a control's label.
	Called string
	// Kind is what sort of thing it is, from Marco's small semantic vocabulary. Empty when
	// the demonstration could not tell, which is honest and not a failure.
	Kind string
}

// Invokes reports whether this action presses a named control rather than sending a meaning.
func (a PlayAction) Invokes() bool { return a.Called != "" }

// Navigate is one navigation meaning, as a play action.
func Navigate(intent string) PlayAction { return PlayAction{Intent: intent} }

// Press is one named target, as a play action.
func Press(called, kind string) PlayAction { return PlayAction{Called: called, Kind: kind} }

// Navigations is a run of navigation meanings as play actions.
//
// The bridge for every caller whose steps are meanings and only meanings, which is most of them:
// a play that presses a control by name comes from Learn and is built as actions from the start.
func Navigations(steps [][]string) [][]PlayAction {
	out := make([][]PlayAction, 0, len(steps))
	for _, run := range steps {
		acts := make([]PlayAction, 0, len(run))
		for _, in := range run {
			acts = append(acts, Navigate(in))
		}
		out = append(out, acts)
	}
	return out
}

// LowerProvisionalActionsBetween is LowerProvisionalPlayBetween for steps that may press controls.
//
// Separate from the meanings-only entry point rather than replacing it, because most callers have
// meanings and converting them at every call site would put the same loop in a dozen places.
func LowerProvisionalActionsBetween(startsOn, endsOn string, steps [][]PlayAction) (
	string, error) {

	return lowerPlay(ProvisionalActor, ProvisionalVerb, startsOn, endsOn, steps)
}

// LowerActionsBetween is LowerPlayBetween for steps that may press controls.
func LowerActionsBetween(actor, verb, startsOn, endsOn string, steps [][]PlayAction) (
	string, error) {

	if err := CheckPlayName(actor); err != nil {
		return "", fmt.Errorf("the name for the thing itself: %w", err)
	}
	if err := CheckPlayName(verb); err != nil {
		return "", fmt.Errorf("the name for what it does: %w", err)
	}
	return lowerPlay(actor, verb, startsOn, endsOn, steps)
}
