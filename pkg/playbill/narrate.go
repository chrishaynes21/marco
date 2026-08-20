package playbill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Turning ONE playbill into the three readings.
//
// # Why the sentences live here and not in each surface
//
// Because there are three surfaces and there will be more. The overlay renders this,
// the CLI renders this, and a future consumer window will render this. If each wrote
// its own wording, then "I couldn't tell what screen appeared" and "destination
// unverified" would eventually be two surfaces describing the same state, and the
// person testing Marco would have to know which one to believe.
//
// So the FACTS come from the Director, the WORDS come from here, and a presentation
// chooses only colour, order and layout.
//
// # The register
//
// Marco is a play and the machinery is backstage. Watch says "I've seen this screen
// before", not "subject fingerprint candidate matched durable identity". It says "I
// couldn't tell where that ended", not "destination RecognitionStatus = insufficient".
// The technical form is available underneath, in Deep, where somebody has asked for it.
//
// # Nothing here claims more than the Director does
//
// Every hedge in the vocabulary survives into the sentence. `Candidate` becomes "I've
// seen this before, but I'm not certain it's the same one" and never "I recognise
// this". A renderer that upgraded a hedge would be the most damaging kind of bug this
// milestone could ship, because it would look like progress.

// Line is one rendered row.
//
// Tone travels with the text so a presentation colours by meaning. The alternative —
// matching on the words — is what the first Director panel did, and it stops working
// silently the moment a sentence is reworded.
type Line struct {
	// Head marks a section title.
	Head bool
	// Indent is one level of nesting, for evidence under a claim.
	Indent int
	Text   string
	Tone   Tone
}

func line(text string, tone Tone) Line { return Line{Text: text, Tone: tone} }
func head(text string) Line            { return Line{Head: true, Text: text, Tone: Muted} }
func sub(text string, tone Tone) Line  { return Line{Indent: 1, Text: text, Tone: tone} }
func blank() Line                      { return Line{Text: "", Tone: Muted} }

func ifelse(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func plural(n int, one, many string) string { return ifelse(n == 1, one, many) }

// ── NORMAL ────────────────────────────────────────────────────────────────────

// Headline is the whole of the consumer reading: one word and one optional sentence.
//
// This exists in this milestone as PROOF rather than as product. The point being made
// is that the consumer surface reduces from the same value Watch expands from, so the
// two can never disagree about whether Marco has a question.
type Headline struct {
	// Word is what is happening, in one or two words.
	Word string
	// Detail is the one sentence a person might want under it. Often empty, which is
	// the correct consumer default — quiet unless there is something to say.
	Detail string
	Tone   Tone
	// Attention says a person is being waited on. The only thing that should ever pull
	// a consumer surface forward.
	Attention bool
}

// Normal reduces the playbill to what a person who is not debugging needs.
//
// Ordering is by what is owed to the person: a question first, because Marco is waiting
// on them; then what Marco is doing to their computer; then what it is learning; then
// what it is watching. The reduction is total — every state produces a headline — so a
// consumer surface never has to handle "none of the above".
func (v View) Normal() Headline {
	switch v.Reach {
	case Unreachable:
		return Headline{Word: "Not connected", Detail: v.Why, Tone: Alarm}
	case Absent:
		return Headline{Word: "Marco is asleep", Detail: v.Why, Tone: Muted}
	}

	if v.Question != nil {
		return Headline{Word: "Marco has a question", Detail: v.Question.Asks,
			Tone: Accent, Attention: true}
	}

	switch v.Doing.Phase {
	case AwaitingPermission:
		return Headline{Word: "Waiting for you", Detail: v.Doing.Because,
			Tone: Accent, Attention: true}
	case CheckingStart, Performing, CheckingResult:
		return Headline{Word: "Running", Detail: v.Doing.What, Tone: Good}
	case Succeeded:
		return Headline{Word: "Done", Detail: v.Doing.What, Tone: Good}
	case Unverified:
		return Headline{Word: "Couldn't verify", Detail: orWhy(v.Doing.Because,
			"I did it, but I couldn't tell where it ended."), Tone: Doubt}
	case Failed, Refused:
		return Headline{Word: ifelse(v.Doing.Phase == Refused, "Refused", "Didn't work"),
			Detail: v.Doing.Because, Tone: Alarm}
	case Cancelled:
		return Headline{Word: "Stopped", Detail: v.Doing.What, Tone: Muted}
	}

	// The Learn session outranks passive learning in the one-line reading. Somebody who
	// asked Marco to learn something is waiting for a cue; somebody being watched is not
	// waiting at all.
	if h, ok := v.learnSessionHeadline(); ok {
		return h
	}

	switch v.Learning.Stage {
	case RehearsalOffered:
		return Headline{Word: "Ready to try", Detail: v.Learning.Because,
			Tone: Accent, Attention: true}
	case Rehearsing:
		return Headline{Word: "Trying it", Detail: v.Learning.Because, Tone: Good}
	case Capturing:
		return Headline{Word: "Watching you", Detail: v.Learning.Because, Tone: Accent}
	case Comparing, AwaitingEvidence, Asking:
		return Headline{Word: "Learning", Detail: v.Learning.Because, Tone: Plain}
	case PlayAvailable:
		return Headline{Word: "Learned something", Detail: v.Learning.Because, Tone: Good}
	}

	if v.Current.Watching {
		return Headline{Word: "Watching", Detail: v.watchingWhat(), Tone: Plain}
	}
	return Headline{Word: "Ready", Tone: Muted}
}

func (v View) watchingWhat() string {
	switch {
	case v.Current.Screen != "" && v.Current.Recognition == Recognised:
		return "I'm on " + quote(v.Current.Screen) + "."
	case v.Current.Application != "":
		return "I'm watching " + v.Current.Application + "."
	}
	return ""
}

// ── WATCH ─────────────────────────────────────────────────────────────────────

// Watch is the human-readable account of what Marco sees, believes, learns and needs.
//
// The primary deliverable, and deliberately compact: somebody is meant to use another
// application while this is on screen. Sections that have nothing to say are omitted
// entirely rather than shown empty, because an empty section reads as an answer.
func (v View) Watch() []Line {
	if !v.Reach.Live() {
		return v.offline()
	}
	var out []Line

	out = append(out, v.currentLines()...)
	// WHAT MARCO CAN ACT ON sits directly under the place, above the counts.
	//
	// It is the section a person checks first — the names tell them at a glance whether
	// Marco is looking at the screen they think it is, which no count can. SEEING answers
	// "is perception working"; this answers "is it working on the right thing".
	if o := v.offerLines(); len(o) > 0 {
		out = append(out, blank(), head("I CAN ACT ON"))
		out = append(out, o...)
	}
	if s := v.seeingLines(); len(s) > 0 {
		out = append(out, blank(), head("SEEING"))
		out = append(out, s...)
	}
	if t := v.thinkingLines(); len(t) > 0 {
		out = append(out, blank(), head("THINKING"))
		out = append(out, t...)
	}
	if t := v.learnSessionLines(); len(t) > 0 {
		// LEARN SESSION sits ABOVE learning, and above everything except what is on screen
		// now. A person who asked Marco to learn something is waiting for a cue from Marco,
		// and the cue must not be below three sections they have to scroll past.
		out = append(out, blank(), head("LEARN SESSION"))
		out = append(out, t...)
	}
	if l := v.learningLines(); len(l) > 0 {
		out = append(out, blank(), head("LEARNING"))
		out = append(out, l...)
	}
	if d := v.doingLines(); len(d) > 0 {
		out = append(out, blank(), head("DOING"))
		out = append(out, d...)
	}
	if v.Question != nil {
		out = append(out, blank(), head("MARCO ASKS"))
		out = append(out, line(v.Question.Asks, Accent))
		if v.Question.About != "" {
			out = append(out, sub("about "+v.Question.About, Muted))
		}
		switch v.Question.Wants {
		case WantsName:
			out = append(out, sub("type a name for it", Muted))
		default:
			out = append(out, sub(strings.Join(v.Question.Answers, " / "), Muted))
		}
	}
	if v.Why != "" {
		out = append(out, blank(), head("WHY"))
		out = append(out, line(v.Why, Doubt))
	}
	if rows := momentLines(v.Recent); len(rows) > 0 {
		out = append(out, blank(), head("JUST NOW"))
		out = append(out, rows...)
	}
	return out
}

// momentLines renders the timeline, collapsing runs of the same sentence.
//
// The Director already publishes only MATERIAL changes, and one sample can still produce
// thirty of them: a first look at a busy application creates a hypothesis per candidate
// region, all in the same instant, all saying the same thing about different evidence.
// Thirty identical lines is one fact printed thirty times, and it buries the line above
// and below it.
//
// Collapsed at the RENDERING layer rather than in the log, deliberately. The events are
// genuinely distinct and a cursor over them has to stay exact; what is repetitive is the
// sentence, which is a property of how it reads and not of what happened.
func momentLines(moments []Moment) []Line {
	var out []Line
	for i := 0; i < len(moments); {
		m := moments[i]
		n := 1
		for i+n < len(moments) && moments[i+n].Says == m.Says {
			n++
		}
		text := m.At.Format("15:04:05") + "  " + m.Says
		if n > 1 {
			text += fmt.Sprintf("  ×%d", n)
		}
		out = append(out, line(text, m.Tone))
		i += n
	}
	return out
}

// offline is what Watch says when there is nothing behind it to ask.
//
// # Why no command appears here
//
// The Absent arm used to end "start it with:  director serve", which is a developer typing a
// developer's binary name. A person using Marco has no reason to know the Director exists, let
// alone that the way to make Marco see again is a subcommand of it — and the HUD is a
// click-through overlay, so the instruction was not even actionable where it was printed: there
// is no prompt under it to type into.
//
// The register to match is three lines away in currentLines: "I'm not watching anything right
// now." Marco says what is true about itself and offers the thing a person can actually reach.
// The control that starts watching belongs on a surface with controls; this sentence's whole job
// is to stop naming a command.
//
// Deleting the naming rule must fail TestWatchNeverTellsAPersonToRunADeveloperCommand.
func (v View) offline() []Line {
	switch v.Reach {
	case Absent:
		return []Line{
			line("I'm not watching anything right now.", Muted),
			blank(),
			sub("Turn watching on when you want me to follow what you're doing.", Muted),
		}
	default:
		return []Line{
			line("I can't reach Marco.", Alarm),
			sub(orWhy(v.Why, "the engine did not answer"), Muted),
		}
	}
}

// currentLines is the top of Watch: what Marco thinks it is looking at.
//
// The recognition vocabulary is spelled out in full here because it is the single
// thing this whole milestone exists to make legible. Each verdict gets its own
// sentence, and none of them is stronger than the verdict behind it.
func (v View) currentLines() []Line {
	c := v.Current
	where := c.Application
	if where == "" {
		where = "something"
	}

	var out []Line
	if !c.Watching {
		// A session that has ENDED still knows what it watched, and the difference
		// between the two matters: "I watched X" is a report, "I'm watching X" is a
		// claim about right now, and a surface that said the second while nothing was
		// running would be the stalest kind of certainty.
		if c.Application == "" {
			return append(out, line("I'm not watching anything right now.", Muted))
		}
		out = append(out, line("Not watching — last looked at "+where, Muted))
		if c.Screen != "" {
			out = append(out, sub("that was "+quote(c.Screen)+".", Muted))
		}
		return out
	}
	out = append(out, line("Watching "+where, Accent))

	switch c.Recognition {
	case Recognised:
		if c.Screen != "" {
			out = append(out, sub("I recognise this as "+quote(c.Screen)+".", Good))
		} else {
			out = append(out, sub("I've been here before — you haven't named it yet.", Good))
		}
	case Candidate:
		if c.Screen != "" {
			out = append(out, sub("This looks like "+quote(c.Screen)+
				", but I'm not certain it's the same one.", Doubt))
		} else {
			out = append(out, sub(
				"I've seen a screen like this before, but I'm not certain it's the same one.",
				Doubt))
		}
	case Ambiguous:
		out = append(out, sub(
			"This could be more than one screen I remember. I can't tell which.", Doubt))
	case Contested:
		out = append(out, sub(
			"This doesn't match what I remembered about it.", Alarm))
	case Unknown:
		out = append(out, sub("I don't recognise this screen.", Plain))
	case Unobservable:
		// Precisely this and not "I'm blind". Screen recognition runs on the structural
		// detector; accessibility can be feeding hundreds of elements while that produces
		// nothing, and "I can't see" would send somebody to check the wrong provider.
		out = append(out, sub("I can't tell one screen from another here.", Alarm))
	}

	if c.Interrupted {
		out = append(out, sub("The window went away and came back — "+
			"what I saw either side may not be the same window.", Doubt))
	}
	if c.Samples > 0 {
		age := ""
		if c.FreshnessMS > 0 {
			age = "  ·  last look " + shortMS(c.FreshnessMS) + " ago"
		}
		out = append(out, sub(strconv.Itoa(c.Samples)+" looks"+age, Muted))
	}
	return out
}

// offerLines is what Marco could act on, named where it may name it.
//
// The names are the point. A person watching a Learn attempt cannot tell from "41
// actionable" whether Marco is looking at the right screen; `System · Bluetooth & devices ·
// Network & internet` tells them in one glance, and tells them just as clearly when Marco is
// looking at the wrong window entirely.
//
// A withheld name is REPORTED rather than hidden. "12 more I'm not allowed to name" is the
// label gate showing its work; silence there would read as Marco having seen less than it
// did.
func (v View) offerLines() []Line {
	o := v.Offers
	if o.Actionable == 0 {
		return nil
	}
	var out []Line
	out = append(out, line(fmt.Sprintf("%d %s I could aim at", o.Actionable,
		plural(o.Actionable, "control", "controls")), Plain))
	for _, off := range o.Named {
		out = append(out, sub(off.Name, Plain))
	}
	if o.Withheld > 0 {
		// "more" only when some WERE named. Saying "3 more" after naming nothing implies
		// a list the reader missed, and sends them looking for it.
		word := "I'm not allowed to name here"
		if len(o.Named) > 0 {
			word = "more " + word
		}
		out = append(out, sub(fmt.Sprintf("%d %s", o.Withheld, word), Muted))
	}
	if o.Focused.Name != "" {
		out = append(out, line("focused: "+o.Focused.Name, Accent))
	} else if o.Focused.Role != "" {
		out = append(out, line("focused: a "+o.Focused.Role+" I'm not allowed to name",
			Muted))
	}
	return out
}

func (v View) seeingLines() []Line {
	s := v.Seeing
	if s.Structure == 0 && s.Unrecognised == 0 && len(s.Terms) == 0 && !s.Quiet {
		return nil
	}
	var out []Line
	if !v.Current.Watching {
		// A snapshot count, not a stability claim. Marco has looked once; it has not
		// watched anything hold still, and saying so would be the difference between a
		// count and a conclusion.
		//
		// AND IT IS SAID IN THE PAST TENSE, which is the other half and was missing.
		//
		// The gate was here and the wording was not: "I can make out 12 things in front of
		// me" reads as a live report, and this branch runs in exactly the two situations
		// where nothing is live. Either a session ENDED and these counts are its leftovers,
		// or nothing has ever been watched and the counts came from one world read that has
		// already finished. Both are things that happened; neither is happening.
		//
		// currentLines two functions up gets this right and says why — "I watched X" is a
		// report, "I'm watching X" is a claim about right now — and a count is no more
		// entitled to the present tense than a place name is. Past tense is true of a
		// finished session and true of a single look, so one wording covers both without
		// the section having to know which one it got.
		//
		// Deleting the past tense must fail TestAFinishedSessionCountsInThePastTense.
		if s.Structure > 0 {
			out = append(out, line(fmt.Sprintf("I could make out %d %s last time I looked.",
				s.Structure, plural(s.Structure, "thing", "things")), Plain))
		}
		if s.Quiet {
			out = append(out, sub("nothing usable in what I could see", Doubt))
		}
		return out
	}
	switch {
	case s.Structure == 0 && s.Unrecognised > 0:
		out = append(out, line(fmt.Sprintf(
			"Nothing is holding still — %d things I can't put a name to.",
			s.Unrecognised), Doubt))
	case s.Structure > 0:
		out = append(out, line(fmt.Sprintf("%d %s holding still here", s.Structure,
			plural(s.Structure, "thing is", "things are")), Plain))
		if len(s.Sources) > 0 {
			out = append(out, sub(strings.Join(s.Sources, ", "), Muted))
		}
		if s.Unrecognised > 0 {
			out = append(out, sub(fmt.Sprintf("%d more I can't put a name to",
				s.Unrecognised), Muted))
		}
	}
	if len(s.Terms) > 0 {
		out = append(out, sub("words I know here: "+strings.Join(s.Terms, ", "), Muted))
	}
	// Readable of Looks, said honestly. It is usually low, and a surface that hid it
	// would leave somebody wondering why Marco never classifies anything.
	if s.Looks > 0 {
		switch {
		case s.Readable == 0:
			out = append(out, sub(fmt.Sprintf(
				"I've looked %d times and haven't managed to read any of the writing",
				s.Looks), Doubt))
		case s.Readable < s.Looks:
			out = append(out, sub(fmt.Sprintf("I could read the writing on %d of %d looks",
				s.Readable, s.Looks), Muted))
		}
	}
	if s.Quiet {
		out = append(out, sub("nothing usable in what I can see", Doubt))
	}
	return out
}

func (v View) thinkingLines() []Line {
	t := v.Thinking
	if len(t.Readings) == 0 && len(t.Links) == 0 && t.Changes == 0 {
		return nil
	}
	var out []Line
	if t.Changes > 0 {
		// The two statements, in the order a person needs them: what changed, and what did
		// not. "Part of the screen changed" and "you went somewhere else" are different
		// events and a surface that reported them identically would be the defect this
		// wording exists to describe.
		what := "The screen changed"
		if t.SameSurface {
			what = "Part of the screen changed, in the same application"
		}
		out = append(out, line(fmt.Sprintf("%s %d %s.",
			what, t.Changes, plural(t.Changes, "time", "times")), Plain))
		switch {
		case t.Caused == 0:
			out = append(out, sub("I didn't see you do anything before any of them.", Muted))
		case t.Caused < t.Changes:
			out = append(out, sub(fmt.Sprintf(
				"I saw you do something before %d of them — I can't say it caused them.",
				t.Caused), Muted))
		default:
			out = append(out, sub(
				"I saw you do something before each one — I can't say it caused them.", Muted))
		}
	}
	for _, r := range t.Readings {
		out = append(out, line(r.Says, standingTone(r.Standing)))
		if hedge := standingHedge(r.Standing); hedge != "" {
			out = append(out, sub(hedge, Muted))
		}
		if r.Because != "" {
			out = append(out, sub(r.Because, Muted))
		}
		for _, b := range r.But {
			out = append(out, sub("but "+b, Doubt))
		}
		if r.Settles != "" {
			out = append(out, sub("it would help if "+r.Settles, Muted))
		}
	}
	if t.Total > len(t.Readings) {
		out = append(out, sub(fmt.Sprintf("+%d more", t.Total-len(t.Readings)), Muted))
	}
	if t.Retracted > 0 {
		out = append(out, sub(fmt.Sprintf("%d earlier %s I've taken back",
			t.Retracted, plural(t.Retracted, "idea", "ideas")), Muted))
	}
	for _, l := range t.Links {
		out = append(out, line(linkSentence(l), ifelseTone(l.Established, Good, Plain)))
		if l.Times > 0 {
			detail := fmt.Sprintf("seen %d %s", l.Times, plural(l.Times, "time", "times"))
			if l.Sessions > 1 {
				detail += fmt.Sprintf(" across %d sittings", l.Sessions)
			}
			if l.Attributed > 0 && l.Attributed < l.Times {
				detail += fmt.Sprintf("; %d of them after something you did", l.Attributed)
			}
			out = append(out, sub(detail, Muted))
		}
	}
	return out
}

func linkSentence(l Link) string {
	if l.Established {
		return "I know " + quote(l.From) + " leads to " + quote(l.To) + "."
	}
	return "I think " + quote(l.From) + " may lead to " + quote(l.To) + "."
}

func (v View) learningLines() []Line {
	l := v.Learning
	if l.Stage == NotLearning && len(l.Silence) == 0 {
		return nil
	}
	var out []Line
	if s := stageSentence(l); s != "" {
		out = append(out, line(s, stageTone(l.Stage)))
	}
	if l.Because != "" && l.Because != stageSentence(l) {
		out = append(out, sub(l.Because, Muted))
	}
	if l.Captured > 0 {
		detail := fmt.Sprintf("%d %s so far", l.Captured,
			plural(l.Captured, "thing you did", "things you did"))
		if l.Checkpoints > 0 {
			detail += fmt.Sprintf("; I could tell where I was %d %s",
				l.Checkpoints, plural(l.Checkpoints, "time", "times"))
		}
		out = append(out, sub(detail, Muted))
	}
	if l.Examples > 0 {
		out = append(out, sub(fmt.Sprintf("%d %s of this", l.Examples,
			plural(l.Examples, "example", "examples")), Muted))
	}
	for _, s := range l.Silence {
		out = append(out, sub(s, Muted))
	}
	if l.Remembered > 0 {
		out = append(out, sub(fmt.Sprintf("I have names for %d %s here",
			l.Remembered, plural(l.Remembered, "screen", "screens")), Muted))
	}
	return out
}

// stageSentence is the one sentence for each learning stage.
//
// Every one of them is in the first person and none of them names a mechanism. The
// stages that are waiting on a PERSON say so plainly, because that is the only thing on
// this list somebody watching can act on.
func stageSentence(l Learning) string {
	route := ""
	if l.From != "" && l.To != "" {
		route = " from " + quote(l.From) + " to " + quote(l.To)
	}
	switch l.Stage {
	case Observing:
		return "I'm watching how you use this."
	case AwaitingEvidence:
		return "I need another example before I'd guess."
	case Asking:
		return "I've asked you something and I'm waiting."
	case Capturing:
		return "You're showing me how to get there" + route + "."
	case Comparing:
		return "I'm comparing that with what you showed me before."
	case RehearsalOffered:
		return "I think I could do this" + route + " — I'm waiting for permission to try."
	case Rehearsing:
		return "I'm trying it" + route + "."
	case Rehearsed:
		return "I tried it" + route + "."
	case PlayAvailable:
		return "I could write this down as a play" + route + "."
	}
	return ""
}

func (v View) doingLines() []Line {
	d := v.Doing
	if d.Phase == NotDoing {
		return nil
	}
	var out []Line
	out = append(out, line(phraseSentence(d), phaseTone(d.Phase)))
	if d.What != "" && d.Phase != Succeeded {
		out = append(out, sub(quote(d.What), Muted))
	}
	if d.Steps > 0 {
		out = append(out, sub(fmt.Sprintf("step %d of %d", d.Step, d.Steps), Muted))
	}
	if d.Expected != "" {
		expect := "I expect to end up on " + quote(d.Expected)
		if d.Reached != "" {
			expect = "I expected " + quote(d.Expected) + " and reached " + quote(d.Reached)
		}
		out = append(out, sub(expect, Muted))
	}
	if d.Because != "" {
		out = append(out, sub(d.Because, phaseTone(d.Phase)))
	}
	if d.RunningMS > 1500 {
		out = append(out, sub("going for "+shortMS(d.RunningMS), Muted))
	}
	if !d.Live && (d.Phase == Performing || d.Phase == Succeeded) {
		out = append(out, sub("nothing reached your computer — this was a dry run", Muted))
	}
	return out
}

func phraseSentence(d Doing) string {
	switch d.Phase {
	case AwaitingPermission:
		return "I'm waiting for permission."
	case CheckingStart:
		return "I'm checking I'm in the right place to start."
	case Performing:
		return "I'm doing it."
	case CheckingResult:
		return "I'm looking to see where that ended."
	case Succeeded:
		return "That worked."
	case Unverified:
		return "I did it, but I couldn't tell where it ended."
	case Failed:
		return "That didn't work."
	case Refused:
		return "I stopped before doing anything."
	case Cancelled:
		return "You stopped me."
	}
	return ""
}

// ── DIAGNOSTICS ───────────────────────────────────────────────────────────────

// Deep is the developer reading: the evidence UNDER what Watch said.
//
// It begins with Watch, deliberately. The question a person asks of diagnostics is
// almost always "why does it say that", and answering it beside the claim rather than
// in a separate view is what stops the two from being compared by memory.
func (v View) Deep() []Line {
	out := v.Watch()
	d := v.Diagnostics
	if d == nil {
		return append(out, blank(), sub("diagnostics were not asked for", Muted))
	}

	out = append(out, blank(), head("PERCEPTION"))
	if len(d.Providers) == 0 {
		out = append(out, line("no providers registered", Alarm))
	}
	for _, p := range d.Providers {
		tone, note := Plain, ""
		switch {
		case !p.Available:
			tone, note = Alarm, "  unavailable: "+p.Why
		case p.Observations == 0:
			// A provider that is present and healthy and reaching none of the samples
			// is the exact shape of the unpinned-accessibility defect, and it is
			// invisible in every other view.
			tone, note = Alarm, "  <- contributed nothing"
		case p.Quarantined > 0:
			tone, note = Doubt, fmt.Sprintf("  %d refused (wrong window)", p.Quarantined)
		}
		row := fmt.Sprintf("%-14s %5d obs", trunc(p.Name, 14), p.Observations)
		if p.LatencyMS > 0 {
			row += fmt.Sprintf("  %dms", p.LatencyMS)
		}
		if p.Metric != "" {
			// A provider's own number, labelled as the provider's own. It is never a
			// Director confidence and is never shown outside this view.
			row += fmt.Sprintf("  %s=%.2f (%s's own)", p.Metric, p.Score, p.Name)
		}
		out = append(out, line(row+note, tone))
	}

	f := d.Fusion
	out = append(out, blank(),
		line(fmt.Sprintf("fusion   %d obs -> %d elements", f.Observations, f.Elements), Plain),
		sub(fmt.Sprintf("merged %d   rejected %d", f.Merged, f.Rejected), Muted))
	if len(f.Degraded) > 0 {
		out = append(out, sub("degraded: "+strings.Join(f.Degraded, ", "), Doubt))
	}
	if !f.ProvenanceOK {
		out = append(out, sub("PROVENANCE INCOMPLETE — sources may describe different windows",
			Alarm))
	}
	if f.CycleMS > 0 {
		out = append(out, sub(fmt.Sprintf("cycle %dms", f.CycleMS), Muted))
	}

	out = append(out, blank(), head("OBSERVATION"))
	out = append(out, line(fmt.Sprintf("%d screens   %d transitions",
		d.Screens, d.Transitions), Plain))
	out = append(out, sub(fmt.Sprintf("navigation: %d attributed  %d unattributed  %d context-admitted",
		d.Attributed, d.Unattributed, d.ContextAdmitted), Muted))
	if d.StructureSource != "" {
		out = append(out, sub("screens read from: "+d.StructureSource, Muted))
	}
	// THE identity measurement. Printed whenever anything was compared, because the
	// screen count on its own cannot say whether an application held still or whether the
	// comparison could not see it move.
	if d.MatchJoined > 0 || d.MatchSeparated > 0 {
		out = append(out, sub(fmt.Sprintf(
			"identity: %d same (weakest %.3f, mean %.3f)   %d other (strongest %.3f)   threshold %.2f",
			d.MatchJoined, d.MatchJoinedMin, d.MatchJoinedMean,
			d.MatchSeparated, d.MatchSeparatedMax, d.MatchThreshold), Muted))
		if d.MatchOverlap {
			out = append(out, sub("OVERLAP — a frame read as the same screen scored no "+
				"better than one read as another; no threshold separates them here", Alarm))
		}
	}
	// The second comparison, under the first. Without it "one screen" is ambiguous between
	// an application that never moved and one whose local changes were all beneath notice.
	if d.LocalSeen > 0 {
		out = append(out, sub(fmt.Sprintf(
			"within a screen: %d compared (weakest %.3f, mean %.3f)   %d part(s) replaced",
			d.LocalSeen, d.LocalMin, d.LocalMean, d.LocalReplaced), Muted))
	}
	if d.Structure != "" {
		out = append(out, sub("recognition: "+d.Structure, Doubt))
	}
	if d.SampleIntervalMS > 0 {
		out = append(out, sub(fmt.Sprintf("every %dms   %d label passes   %d skipped   %d late",
			d.SampleIntervalMS, d.LabelPasses, d.SamplesSkipped, d.SamplesLate), Muted))
	}
	if d.Verdict != "" {
		out = append(out, sub(fmt.Sprintf("recall verdict %q over %d candidates",
			d.Verdict, d.Candidates), Muted))
	}
	if d.Memory != "" {
		out = append(out, sub("memory: "+d.Memory, Doubt))
	}

	if d.Proposal != "" || d.Suppressed > 0 {
		out = append(out, blank(), head("QUESTIONS"))
		if d.Proposal != "" {
			out = append(out, line(fmt.Sprintf("%s  %s", d.Proposal, d.ProposalStatus), Plain))
		}
		if d.Suppressed > 0 {
			out = append(out, sub(fmt.Sprintf("%d withheld (already asked on this evidence)",
				d.Suppressed), Muted))
		}
	}

	if d.RehearsalPlanned > 0 || d.Authority != "" {
		out = append(out, blank(), head("REHEARSAL"))
		if d.RehearsalPlanned > 0 {
			out = append(out, line(fmt.Sprintf("step %d of %d   %s",
				d.RehearsalStep, d.RehearsalPlanned, d.RehearsalOutcome), Plain))
		}
		if d.Authority != "" {
			// The grant's STATE. Never the grant: authority that can be marshalled is
			// authority that can be replayed.
			out = append(out, sub("authority: "+d.Authority, Muted))
		}
	}

	out = append(out, blank(),
		sub(fmt.Sprintf("playbill v%d   epoch %s   composed in %dms",
			v.Version, trunc(v.Epoch, 12), d.ComposeMS), Muted),
		sub(fmt.Sprintf("cursor %d   oldest %d   digest %s",
			v.Cursor, v.Oldest, trunc(v.Digest, 8)), Muted))
	return out
}

// ── the coalescing digest ─────────────────────────────────────────────────────

// WithDigest returns the playbill with its Digest computed.
//
// The digest covers what a person would NOTICE changing and deliberately excludes
// clocks, sample counts and freshness. Without that exclusion every poll would produce
// a different digest and the coalescing it exists for would never fire once.
func (v View) WithDigest() View {
	h := sha256.New()
	w := func(parts ...string) {
		for _, p := range parts {
			_, _ = io.WriteString(h, p)
			_, _ = io.WriteString(h, "\x00")
		}
	}
	w(string(v.Reach), v.Why, v.Epoch)
	w(v.Current.Application, v.Current.Screen, string(v.Current.Recognition),
		boolKey(v.Current.Watching), boolKey(v.Current.Interrupted))
	w(strconv.Itoa(v.Seeing.Structure), strconv.Itoa(v.Seeing.Unrecognised),
		strings.Join(v.Seeing.Terms, ","), strings.Join(v.Seeing.Sources, ","))
	for _, r := range v.Thinking.Readings {
		w(r.Says, string(r.Standing), r.Because, strings.Join(r.But, "|"), r.Settles)
	}
	for _, l := range v.Thinking.Links {
		w(l.From, l.To, boolKey(l.Established), strconv.Itoa(l.Times))
	}
	w(strconv.Itoa(v.Thinking.Changes), strconv.Itoa(v.Thinking.Caused))
	w(string(v.Learning.Stage), v.Learning.From, v.Learning.To, v.Learning.Because,
		strings.Join(v.Learning.Silence, "|"))
	// The Learn section is IN the digest, so a surface that holds still while nothing
	// changes still moves the moment the cue appears. A cue nobody is told about is worse
	// than no cue.
	w(boolKey(v.LearnSession.Active), boolKey(v.LearnSession.Armed),
		boolKey(v.LearnSession.Waiting), v.LearnSession.Asked,
		v.LearnSession.Because, v.LearnSession.Learned, boolKey(v.LearnSession.Stopped),
		strings.Join(v.LearnSession.Did, "|"), strconv.Itoa(v.LearnSession.Examples))
	for _, s := range v.LearnSession.Progress {
		w(s.Name, string(s.State))
	}
	w(string(v.Doing.Phase), v.Doing.What, v.Doing.Because,
		strconv.Itoa(v.Doing.Step), v.Doing.Expected, v.Doing.Reached)
	if v.Question != nil {
		w(v.Question.ID, v.Question.Asks, string(v.Question.Wants))
	}
	v.Digest = hex.EncodeToString(h.Sum(nil))[:16]
	return v
}

// ── small helpers ─────────────────────────────────────────────────────────────

func standingTone(s Standing) Tone {
	switch s {
	case Confirmed, Recalled:
		return Good
	case Disputed, Withdrawn:
		return Doubt
	}
	return Plain
}

// standingHedge is the sentence that keeps a reading honest.
//
// Only the states that need one produce one. `Supported` says nothing extra because the
// claim already carries the Director's own hedging; `Confirmed` says who confirmed it,
// because "you told me" and "I worked it out" must never read the same.
func standingHedge(s Standing) string {
	switch s {
	case Tentative:
		return "I'm not sure about this one."
	case Disputed:
		return "the evidence points both ways."
	case Confirmed:
		return "you told me this."
	case Recalled:
		return "you told me this before."
	case Withdrawn:
		return "I've taken this back."
	}
	return ""
}

func stageTone(s Stage) Tone {
	switch s {
	case RehearsalOffered, Asking:
		return Accent
	case Rehearsing, Capturing, PlayAvailable:
		return Good
	case AwaitingEvidence:
		return Doubt
	}
	return Plain
}

func phaseTone(p Phase) Tone {
	switch p {
	case Succeeded:
		return Good
	case Unverified:
		return Doubt
	case Failed, Refused:
		return Alarm
	case AwaitingPermission:
		return Accent
	case Cancelled:
		return Muted
	}
	return Plain
}

func ifelseTone(c bool, a, b Tone) Tone {
	if c {
		return a
	}
	return b
}

func orWhy(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// quote wraps a name in typographic quotes, so a screen the user named reads as their
// word rather than as Marco's.
func quote(s string) string {
	if s == "" {
		return "somewhere I can't name"
	}
	return "“" + s + "”"
}

func shortMS(ms int64) string {
	switch {
	case ms < 1000:
		return strconv.FormatInt(ms, 10) + "ms"
	case ms < 60000:
		return strconv.FormatInt(ms/1000, 10) + "s"
	}
	return strconv.FormatInt(ms/60000, 10) + "m"
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func boolKey(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ── LEARN SESSION ─────────────────────────────────────────────────────────────

// learnSessionLines is the explicit learn session, for a person watching it happen.
//
// # What this has to make unmistakable
//
// Three things, and they are the three a person cannot recover from getting wrong:
//
//  1. whether Marco is watching THIS demonstration, as opposed to watching normally;
//  2. what Marco believes they just did — including that it could not tell;
//  3. whether a play actually exists, which is read from the artifact and never from
//     having reached the end of the flow.
//
// Everything else is progress and can be skimmed.
func (v View) learnSessionLines() []Line {
	t := v.LearnSession
	if !t.Active && t.Learned == "" && !t.Stopped {
		return nil
	}
	var out []Line
	if t.Asked != "" {
		out = append(out, line("Learning "+quote(t.Asked), Plain))
	}

	// THE cue. Accent is reserved in this package for things waiting on a person, and a
	// bounded demonstration window is exactly that: Marco is holding it open and can do
	// nothing until somebody acts.
	switch {
	case t.Armed:
		out = append(out, line("SHOW ME — I'm watching this example now.", Accent))
	case t.Active && !t.Waiting && t.Examples == 0 && firstStep(t.Progress):
		// ONLY before the first cue. An earlier draft showed this in every phase that was
		// not the cue, so "want me to try it once?" arrived under a line saying Marco was
		// getting ready — the opposite of true, and exactly the kind of stale reassurance
		// this surface exists to prevent.
		out = append(out, line("Getting ready…", Muted))
	}

	// What Marco thinks happened. The empty case is SPLIT, because "nothing yet" and "I
	// could not tell" are the two readings a person most needs kept apart.
	switch {
	case len(t.Did) > 0:
		out = append(out, sub("you did: "+strings.Join(t.Did, ", "), Plain))
	case t.Unattributed:
		out = append(out, sub("you did: ?", Doubt))
		out = append(out, sub("I saw the screen change, but I couldn't tell what you did.",
			Doubt))
	}

	if t.Examples > 0 {
		out = append(out, sub(fmt.Sprintf("%d %s so far", t.Examples,
			plural(t.Examples, "example", "examples")), Muted))
	}
	if t.Because != "" {
		out = append(out, sub(t.Because, learnTone(t)))
	}
	for _, s := range t.Progress {
		out = append(out, sub(stepMark(s.State)+" "+s.Name, stepTone(s.State)))
	}

	// The RESULT, from the artifact. A play with no name is not a play.
	switch {
	case t.Learned != "":
		out = append(out, line("Learned "+quote(t.Learned)+".", Good))
		if !t.Registered {
			out = append(out, sub("nothing can ask for it yet.", Muted))
		}
	case t.Stopped:
		out = append(out, line("Stopped. Nothing was written down.", Doubt))
	}
	return out
}

func learnTone(t LearnSession) Tone {
	switch {
	case t.Stopped:
		return Doubt
	case t.Learned != "":
		return Good
	case t.Armed:
		return Accent
	}
	return Muted
}

// stepMark is the checklist glyph. Plain characters: this renders in a terminal too.
func stepMark(s StepState) string {
	switch s {
	case StepDone:
		return "[x]"
	case StepCurrent:
		return "[>]"
	case StepSkipped:
		return "[-]"
	}
	return "[ ]"
}

func stepTone(s StepState) Tone {
	switch s {
	case StepDone:
		return Good
	case StepCurrent:
		return Accent
	}
	return Muted
}

// learnSessionHeadline is the one-line reading of an explicit learn session.
//
// Ordered by what the person has to DO, not by where the flow is: the cue first, the
// result second, progress last. `Attention` is set only for the cue, because that is the
// one state where Marco is holding still and cannot continue without them.
func (v View) learnSessionHeadline() (Headline, bool) {
	t := v.LearnSession
	switch {
	case t.Armed:
		return Headline{Word: "Show me", Detail: "I'm watching this example now.",
			Tone: Accent, Attention: true}, true
	case t.Learned != "":
		return Headline{Word: "Learned", Detail: quote(t.Learned) + ".", Tone: Good}, true
	case t.Stopped:
		return Headline{Word: "Stopped", Detail: t.Because, Tone: Doubt}, true
	case t.Waiting:
		// Their turn. Accent and Attention are reserved for exactly this, and collapsing
		// it into "Learning from you" would leave somebody watching a line that never changes while
		// Marco waits for them.
		return Headline{Word: "Waiting for you", Detail: t.Because,
			Tone: Accent, Attention: true}, true
	case t.Active:
		detail := t.Because
		if detail == "" {
			detail = "Hold still a moment."
		}
		return Headline{Word: "Learning from you", Detail: detail, Tone: Plain}, true
	}
	return Headline{}, false
}

// firstStep reports whether the checklist has not moved off its first entry.
//
// The checklist comes from the coordinator, so this asks the coordinator's own progress rather
// than guessing from a sentence.
func firstStep(p []Step) bool {
	if len(p) == 0 {
		return true
	}
	return p[0].State == StepCurrent
}
