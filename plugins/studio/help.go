package main

// WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT CAN EXPLAIN ABOUT ITSELF.
//
// Two answers, both read ONCE and both from the backend, for two different reasons.
//
// # Features: the alpha has one journey, and the other two ways in are not it
//
// `Acts` and `Scenes` are complete capabilities with their own views, and NOTHING IS REMOVED
// here — the code, the bindings and the sections stay exactly as they are. What is withheld is
// the way IN, because the alpha supports Routines and a rail offering three equal choices says
// otherwise. Somebody who follows `Acts` into the composer finds a surface the alpha cannot
// carry, and the cost of that is paid at the far end of it.
//
// Read from the environment ONCE, when this window's backend is made. Not per call: a rail that
// could change between two polls is a rail that moves under somebody's cursor, and a variable
// edited while the window is open would do exactly that. There is no settings control and no
// runtime toggle — that is a decision, not an omission, and it belongs to whoever ships the
// alpha rather than to whoever is using it.
//
// # Help: the sentences that name a key CANNOT be written into the page
//
// The stop gesture and the argument key are CONFIGURED (`$MARCO_STOP_KEY`, `$MARCO_ARG_KEY`),
// and this repository has already shipped the defect of a page that wrote one of them into
// itself: `Recording` reported the raw variable on one path and the parsed label on the other,
// so an unset override read as "there is no shortcut" in the default installation, which is
// every installation. See `stopLabel` and `argKeyLabel` in recording.go, which are the one
// resolution of each and are what this reuses.
//
// So Help is composed HERE, out of the same two labels and the same `everywhereSentence` the
// record card shows, and the page draws whatever it is handed. The day a key moves, the help
// moves with it.
//
// See [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]].

import (
	"os"
	"strings"
)

// Features is which optional parts of this window are offered, decided once when it opened.
type Features struct {
	// Views is the rail, in order, as the page should paint it. The page iterates THIS
	// rather than a list of its own.
	//
	// It is a list rather than two booleans because the page's own version was a
	// hardcoded `['routines','acts','scenes','history']` that it both painted the rail
	// from AND looked elements up by — so a button that was not there threw mid-paint and
	// stopped the window rendering altogether. The elements all stay in the document; the
	// LIST is what narrows.
	Views []string `json:"views"`
	// Acts and Scenes are the two that are off by default, named so the page can say so
	// without re-deriving it from the list.
	Acts   bool `json:"acts"`
	Scenes bool `json:"scenes"`
}

// alphaViews is the rail when nothing is turned on: the one journey, what it did, and help.
var alphaViews = []string{"routines", "history", "help"}

// featuresFromEnv reads the two flags, once.
//
// DEFAULT OFF, and off is the unset value as well as an explicitly false one: a flag whose
// absence turned a surface ON would be a surface nobody chose.
func featuresFromEnv() Features {
	f := Features{
		Acts:   flagOn(os.Getenv("MARCO_STUDIO_ACTS")),
		Scenes: flagOn(os.Getenv("MARCO_STUDIO_SCENES")),
	}
	// IN THE ORDER THE RAIL READS, with the journey first and help last. `history` stays
	// where it was relative to the two optional ones so that turning them on gives back
	// exactly the rail that shipped before this, plus Help.
	f.Views = []string{"routines"}
	if f.Acts {
		f.Views = append(f.Views, "acts")
	}
	if f.Scenes {
		f.Views = append(f.Views, "scenes")
	}
	f.Views = append(f.Views, "history", "help")
	return f
}

// flagOn reads one environment flag the way a person would write it.
//
// Several spellings because this is set by hand in a shell and not by a form, and a flag that
// silently ignores `true` is a flag somebody will believe they turned on.
func flagOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "on", "true", "yes", "y":
		return true
	}
	return false
}

// Features is which parts of this window are offered.
func (a *Acts) Features() Features { return a.features }

// ── WHAT THE WINDOW CAN EXPLAIN ABOUT ITSELF ──────────────────────────────────

// HelpSection is one thing a person might need explained, and what it says.
//
// Key is stable and is what an inline link beside a control names, so "what can be changed",
// written next to the step list, opens the section about editing rather than scrolling somebody
// to the top of a page of everything.
type HelpSection struct {
	Key   string   `json:"key"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

// Help is the one place this window explains itself.
type Help struct {
	Sections []HelpSection `json:"sections"`
}

// Help is what this window can explain, with the configured shortcuts in it.
//
// # Why these five and not a manual
//
// Because each one is an ability the audit found COMPLETE AND INVISIBLE. Arguments could be
// declared three ways and the window named none of them; the stop shortcut existed only in the
// band that sits behind the application being demonstrated in; a press recorded in a window is
// written down twice and nothing said which of the two numbers a person is looking at. A help
// section that explained the obvious parts and not these would be decoration.
//
// The recording section says what the recorder actually does, which is not what the first draft
// of this said. Marco records ONE window and THE WINDOW IS CHOSEN — `Deps.Begin` refuses a
// confined recording that was not told which one — so "the window you go to" is false and is
// the kind of false that teaches somebody to click somewhere else and wonder where it went. See
// the recording invariant in CLAUDE.md and [[ADR-268-record-only-the-chosen-window]].
func (a *Acts) Help() Help {
	stop := stopLabel()
	arg := argKeyLabel()

	recording := []string{
		"Press Record, give the routine a name, and choose the window to record " +
			"in — Marco asks which one before it starts, and records that " +
			"window only.",
		// FROM THE SAME SENTENCE THE RECORD CARD SHOWS, so the two cannot drift
		// and neither can go stale about the key.
		everywhereSentence(stop),
		"When you stop, you see what was recorded before anything is saved.",
	}

	asking := []string{
		"Name it “cloud pull stone with item” and the routine asks for item " +
			"each time you play it.",
	}
	if arg != "" {
		asking = append(asking, "Or press "+arg+" during the demonstration, at the "+
			"moment the value goes in.")
	}
	asking = append(asking,
		"Or afterwards: open its Steps tab, find the step that types the "+
			"value, name it and press Ask me for it.",
		"A routine that asks for a value shows a box on its card, and Play waits "+
			"until it is filled in.")

	editing := []string{
		"Change a wait, remove a step, or say a run of steps is text you typed.",
		// THE BOXES ARE THE CHANGE. There is no Set: what is typed is read when Review is
		// pressed, and an entry that is not a number stays as typed with the reason beside
		// it. See ADR-292.
		"Type a new wait or position into its box. Review collects every box you " +
			"changed; Reset puts a row back as it was.",
		"Make the text a step types something you supply each time — all of that " +
			"step’s text, not part of it.",
		"Nothing is written until you review the change and save it — the version " +
			"you would get is shown beside the one you have.",
		"The routine is written again from the list, so removing one half of a " +
			"held key removes the other half with it, and the waits around a " +
			"removed step stay as they are until you change them too.",
	}

	measured := []string{
		"A press recorded inside a window is written down twice: as an offset " +
			"inside that window, and as a point on the screen. Both were " +
			"measured at the same instant.",
		"The offset is the durable half — it still means the same place after the " +
			"window has moved — so it is the one you edit, and the point on " +
			"the screen moves with it from that same measurement.",
		"A press recorded with no window to measure against has only the point on " +
			"the screen, and that is what you edit instead.",
		"A press Marco finds by its picture is not moved at all: the coordinate is " +
			"a position inside that picture, so moving it would aim the " +
			"picture somewhere else.",
		// WHAT A CHANGE KEEPS, SAID ONCE. This was on every press row in the
		// editor, as the window's size and scale printed beside each pair of
		// numbers. It is the same promise for every row, so it is made here.
		"Changing a press keeps everything else about it: which button it uses, any " +
			"keys held down with it, and the window, size and scale it is " +
			"checked against.",
	}

	// WHAT A ROUTINE CHECKS, WHICH THE CARD USED TO ENUMERATE. The guarded sentence sat
	// above Play reading "Checks the program, window size and scale before each press.
	// Stops if a check fails." — true, and three measurements a person does not need
	// named at the moment they are deciding whether to press a button. The card is silent
	// for the ordinary case now and links here; what is left on the card is the two
	// contracts that are a warning. See `checksSentence` and ADR-251.
	checks := []string{
		"A routine recorded in a window checks three things before every press: that " +
			"the program is the one it was recorded in, that the window is the " +
			"size it was recorded at, and that it is drawn at the same scale.",
		"If any of them has changed it stops, rather than pressing somewhere it was " +
			"never shown.",
		"Beside Play you are told which window the run will bring forward, and whether " +
			"its presses are checked against it.",
		"A routine saved before Marco measured windows checks nothing: every press " +
			"happens at the point it was recorded at, whatever is there now. Its " +
			"card says so.",
		"A recording that measured only some of those terms refuses every press, and " +
			"its card says which measurement is missing. Record it again.",
	}

	// SIMPLIFY, SAID ONCE HERE AND AGAIN IN THE REVIEW WHERE IT IS APPROVED. The recording
	// review carried forty-one words of this beside the button; what it costs is now said
	// on the review itself, at the moment somebody decides, and this is the whole of it.
	simplify := []string{
		"Simplify… shortens the waiting: a run of typing becomes one step, and every " +
			"other pause drops to a quarter of a second.",
		"If the simplified routine then presses before a screen has caught up, use " +
			"the original.",
		"It builds the steps again from your demonstration, so a step you marked to " +
			"ask for its text types what you recorded again — mark it again afterwards.",
		"You see the simplified version before anything is saved.",
	}

	// OTHER NAMES, which are not a rename. The box that adds one is the first thing that
	// answered "where do I change what this is called" and was not the answer. See
	// ADR-289.
	names := []string{
		"A routine can answer to more than one phrase when you ask for it in your own " +
			"words. Add them on its Other names tab.",
		"Other names are extra ways to ask for it. To change what it is called, use " +
			"Rename beside its name.",
	}

	where := []string{
		"In one application only — it runs when that application is in front.",
		"In that application wherever it is — it brings the window forward first.",
		"Everywhere — no application is checked before it runs.",
	}

	return Help{Sections: []HelpSection{
		{Key: "recording", Title: "Recording a routine", Lines: recording},
		{Key: "arguments", Title: "Asking for a value each time", Lines: asking},
		{Key: "editing", Title: "Editing what a routine does", Lines: editing},
		{Key: "measured", Title: "Where a press is measured", Lines: measured},
		{Key: "checks", Title: "What a routine checks before it presses", Lines: checks},
		{Key: "simplify", Title: "Simplifying a routine", Lines: simplify},
		{Key: "names", Title: "Other names for a routine", Lines: names},
		{Key: "scope", Title: "Where a routine works", Lines: where},
	}}
}
