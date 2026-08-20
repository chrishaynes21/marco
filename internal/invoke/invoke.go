// Package invoke is the one semantic decision every entrance makes.
//
// # The rule, in one sentence
//
// If Marco already knows exactly what the Audience means, it uses the durable thing it knows;
// otherwise Director works out what they mean from the live world.
//
// # Why this is a package and not a habit
//
// Because there were two intakes and they disagreed. A phrase TYPED into the overlay went to Play
// lookup and, on a miss, became an offer to record a demonstration. The same words SPOKEN went
// straight to Director and were reinterpreted against the screen — so a Play the person had
// learned, registered and could run by typing was not found by saying its name, and a request
// Director could have satisfied became "shall I learn this?" the moment it was typed instead of
// said. Transport was deciding meaning, and the deciding happened in two files that shared
// nothing.
//
// So the decision is lifted out to somewhere it can be made once and proved. It is PURE: it reads
// a resolver and returns a verdict. It performs nothing, connects to nothing, and cannot be told
// what to decide by the surface that asks.
//
// # Source is recorded and never consulted
//
// `Request.Source` exists so a trace can say how intent arrived. Nothing in `Decide` reads it, and
// TestSourceCannotChangeTheDecision holds that line by putting every source through every case.
// Typing and speaking are transport differences, not semantic differences.
//
// # What this deliberately does not do
//
// It does not match fuzzily. Play lookup answers "does Marco already know this exactly"; the
// moment it starts guessing which Play somebody probably meant, it is a second semantic planner
// sitting in front of the real one, and the two will disagree. A near miss is a miss, and a miss
// belongs to Director — which can see, and can ask.
package invoke

import (
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Source is how an invocation arrived. Recorded for the trace; never read by Decide.
type Source string

const (
	// SourceTyped is the overlay command line.
	SourceTyped Source = "typed"
	// SourceSpoken is a final recognised phrase from voice.
	SourceSpoken Source = "spoken"
	// SourceHotkey is a leader-key binding.
	SourceHotkey Source = "hotkey"
	// SourceControlCentre is a Run pressed in the browser surface.
	SourceControlCentre Source = "control-centre"
	// SourceCLI is a person at a shell.
	SourceCLI Source = "cli"
	// SourceWeb is the web front end.
	SourceWeb Source = "web"
)

// Plays is the durable half of the decision: can Marco already answer this name?
//
// An interface, and a one-method one, for two reasons. It keeps `Decide` free of a filesystem so
// the decision can be proved without one — and it makes "an explicit identity is never looked up
// again" provable by handing in a resolver that records every call and asserting it was not
// asked. `routes.Registry` satisfies it as it stands.
//
// Note what is NOT here: no listing, no fuzzy match, no staging. `routes.Registry.Resolve` reads
// only the scopes discovery scans, so a staged play is structurally unable to answer — see
// [[ADR-028-a-learned-play-is-a-file-with-a-past]]. Nothing in this package needs to remember
// that, which is the point of it being structural.
type Plays interface {
	Resolve(app, name string) (routes.Route, bool)
}

// Request is one invocation, as the entrance received it.
type Request struct {
	// Text is what the Audience said or typed, unedited.
	Text string
	// Source is how it arrived. Recorded, never consulted.
	Source Source
	// Play is an identity the surface ALREADY holds — a clicked Run, a resolved binding.
	//
	// When it is set the words are not consulted at all. A surface that knows which Play it
	// means must not hand that back as a phrase to be guessed at again: the display name is
	// derived from a slug, the slug is not a general inverse of the name, and a second lookup
	// can land somewhere else entirely.
	Play *routes.Route
	// Named and Positional are arguments the entrance already peeled off, for a surface that
	// holds an identity rather than a phrase. A binding may carry them; a clicked Run does not.
	//
	// Decide never reads them — they are cargo. They live here so that one Request describes a
	// whole invocation and a caller is not tempted to keep a second channel beside it.
	Named      map[string]string
	Positional []string
	// App is the foreground application, when the entrance knows it. Scope resolution needs it.
	App string
	// Pending says Director is holding a question nobody has answered yet.
	//
	// When it is true the words belong to that question. "the second one" is not a Play and
	// must never be matched as one, and even a phrase that WOULD match a Play is an answer
	// while an answer is what was asked for.
	Pending bool
}

// Kind is what an invocation turned out to be.
type Kind string

const (
	// KindControl is stop or cancel: it acts on what is already running.
	KindControl Kind = "control"
	// KindPlay is a durable behaviour Marco already has.
	KindPlay Kind = "play"
	// KindDirector is intent Marco does not already have a Play for.
	KindDirector Kind = "director"
)

// Decision is the verdict, and everything a trace needs to explain it.
type Decision struct {
	// Kind is what to do.
	Kind Kind
	// Play is which durable behaviour, when Kind is KindPlay.
	Play routes.Route
	// Explicit says the Play came from the surface's own identity rather than from the words.
	Explicit bool
	// Phrase is what Director receives, when Kind is KindDirector. The Audience's words,
	// trimmed and no more — Director reads sentences, and rewriting one changes its meaning.
	Phrase string
	// Why is one line for the cross-surface trace: which rule fired, and on what.
	Why string
}

// Decide is the whole of it, and the order of the checks is the whole of the order.
//
//  1. A CONTROL PHRASE, always first and from every entrance. "stop" said while a play is running
//     has to reach the thing that is running before anything else looks at it — and a "stop" that
//     went to Play lookup would find nothing, miss, and be offered as something to learn. Which is
//     what typing it used to do.
//  2. A PENDING QUESTION captures the words. Director asked something; these are the answer.
//  3. AN EXPLICIT IDENTITY performs that Play. The words are not read.
//  4. AN EXACT DURABLE MATCH performs that Play.
//  5. Everything else is for Director.
//
// Deleting or reordering any arm must fail a test in invoke_test.go named for it.
func Decide(plays Plays, req Request) Decision {
	text := strings.TrimSpace(req.Text)

	// 1. CONTROL — before everything, including a pending question and an explicit identity.
	//
	// `intent.IsControlPhrase` is the ONE definition, shared with the Director service's own
	// phrase routing (internal/director/service/clarify.go). A second list here would be a
	// second answer to "did they say stop", and the two would drift the first time somebody
	// added a word to one of them.
	//
	// Deleting this arm must fail TestStopIsNeverOrdinaryText.
	if intent.IsControlPhrase(text) {
		return Decision{Kind: KindControl, Phrase: text,
			Why: "a control phrase — it acts on what is running"}
	}

	// 2. AN ANSWER BELONGS TO THE QUESTION.
	//
	// Ahead of Play matching on purpose: while Director is waiting to be told which one, a
	// phrase that happens to name a Play is still an answer. Claiming it here would silently
	// discard the question and start something else.
	//
	// Deleting this arm must fail TestAnAnswerToAPendingQuestionIsNotAPlay.
	if req.Pending {
		return Decision{Kind: KindDirector, Phrase: text,
			Why: "Director is waiting for an answer — these words are it"}
	}

	// 3. EXPLICIT IDENTITY. The surface already knows which Play; it is not asked to say so in
	// words that would then have to be guessed back.
	//
	// Deleting this arm must fail TestAnExplicitPlayIsNeverLookedUpAgain.
	if req.Play != nil {
		return Decision{Kind: KindPlay, Play: *req.Play, Explicit: true,
			Why: "the surface named the play itself"}
	}

	if text == "" {
		return Decision{Kind: KindDirector, Phrase: "", Why: "nothing was said"}
	}

	// 4. EXACT DURABLE MATCH.
	//
	// `Resolve` applies `routes.Slug` to the phrase, which already folds case, punctuation,
	// surrounding quotes and runs of whitespace onto one durable identity — the normalization
	// the product wants was here all along; nothing called it first. It is deliberately NOT
	// fuzzy: "near enough" is a judgement, and judgement is Director's job.
	//
	// The WHOLE phrase is tried before any invocation grammar is applied to it, so a play
	// called "wait then click" is reachable by its own name instead of being split into two
	// commands neither of which exists.
	//
	// Deleting this arm — or consulting Director first — must fail
	// TestAnExactlyKnownPlayIsNeverInterpreted.
	if rt, ok := plays.Resolve(req.App, text); ok {
		return Decision{Kind: KindPlay, Play: rt, Why: "an exact match for a play Marco has"}
	}

	// 5. DIRECTOR. Marco does not already know this one, so it is a question about the world.
	return Decision{Kind: KindDirector, Phrase: text,
		Why: "no play answers to this, so Director reads it against what is on screen"}
}

// Trace is one line saying how an invocation was routed, for the acceptance runs.
//
// Deliberately not a store and not a file: it is a string a caller may log. This phase was told
// not to create a durable telemetry store, and the words a person says are exactly the thing not
// to start keeping.
func (d Decision) Trace(req Request) string {
	var b strings.Builder
	b.WriteString("source=")
	b.WriteString(string(req.Source))
	b.WriteString(" decision=")
	b.WriteString(string(d.Kind))
	switch d.Kind {
	case KindPlay:
		b.WriteString(" play=")
		b.WriteString(d.Play.Slug)
		if d.Play.App != "" {
			b.WriteString(" app=")
			b.WriteString(d.Play.App)
		}
		if d.Explicit {
			b.WriteString(" explicit=yes")
		}
	case KindDirector, KindControl:
		if d.Phrase != "" {
			b.WriteString(" phrase=")
			b.WriteString(strconv.Quote(d.Phrase))
		}
	}
	b.WriteString(" why=")
	b.WriteString(strconv.Quote(d.Why))
	return b.String()
}
