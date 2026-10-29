package main

// BEFORE OFFERING TO LEARN A PHRASE, ASK WHETHER MARCO ALREADY HAS IT.
//
// # The moment this is
//
// The widget's one door is `marco do --source=… <phrase>`, and the engine's intake matches a play
// EXACTLY on purpose: "near enough" is a judgement, and judgement is Director's. So a phrase that
// is a routine's name runs, and everything else goes to Director to be read against the screen —
// and when nothing at all took it, the widget offers to record one.
//
// That offer is the moment. A person who typed "make the pictures nicer" may well have a routine
// for it under another name, and being offered a recording session is the one answer that is
// certainly wrong. So before the offer, Director is asked which saved routine the words named.
//
// # It asks DIRECTOR, through `marco`, and holds no policy
//
// Which routines are eligible depends on what is in front, and which library the answer is about
// is Director's. The widget holds neither fact and does not get a copy of either: it runs
// `marco resolve --json`, which is a client, and renders what comes back. No scope rule lives
// here, no exact matching, no resolver plugin, and no Director connection of its own.
//
// AND THE OFFER IS STILL AN OFFER. A resolution selects; it never runs. What a person confirms
// goes back through the same door with the whole identity — `--play`, `--app`, `--focus` — which
// is the intake's explicit-identity arm, where no phrase is looked up again and every guard is
// the one that was already there.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// resolved is `marco resolve --json`'s answer, as much of it as this surface renders.
//
// A subset on purpose: the widget shows a sentence and offers one routine. Everything else in the
// answer is for a surface with room to draw it.
type resolved struct {
	Answer struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		App     string `json:"app"`
		Slug    string `json:"slug"`
		Focus   bool   `json:"focus"`
		Name    string `json:"name"`
		Scope   string `json:"scope"`
		Reply   string `json:"reply"`
		Context struct {
			Says string `json:"says"`
		} `json:"context"`
		Source   string `json:"source"`
		Resolver string `json:"resolver"`
	} `json:"answer"`
	Mismatch string `json:"mismatch"`
	Why      string `json:"why"`
}

// resolveWithin bounds the ask.
//
// Short, because this sits between a person's request and an answer they are waiting for. The
// plugin's own cold-model bound is longer than this; a first call that pays it will miss, and the
// second will not.
// A VARIABLE rather than a constant purely so a test can shorten it, with the same justification
// `resolver.InterpretTimeout` carries: "a child that is abandoned keeps running" is a property
// that must hold before any of this is trusted, and at six seconds it is a property nobody would
// run. Production never reassigns it.
var resolveWithin = 6 * time.Second

// askWhichRoutine is a package variable for the same reason `spawnIntakeChild` is: this surface
// has to be exercisable without spawning `marco.exe`. Production never reassigns it.
//
// The second value is why there is NO answer, and "" when there is one. A bool would have made a
// timeout and an unreadable reply the same thing, and they are not: one of them is worth saying.
var askWhichRoutine = runResolveChild

// runResolveChild runs `marco resolve --json`, bounded, and reads the one object it prints.
//
// # The bound is HERE, and the child does not outlive it
//
// It used to be a `select` around a goroutine: the wait ended and the child kept running, holding
// a pipe nobody would read and, through it, whatever the resolver plugin had started. A surface
// that abandons a process it spawned is the shape of the eight `marco do` processes this
// repository spent a release gate on.
//
// `CommandContext` kills it when the context ends and `Output` waits for it, so the bound and the
// reaping are the same act. What a timeout produces is a REASON, said in the HUD, and never an
// offer to record something.
func runResolveChild(phrase string) (resolved, string) {
	ctx, cancel := context.WithTimeout(context.Background(), resolveWithin)
	defer cancel()
	cmd := exec.CommandContext(ctx, marcoBin(), "resolve", "--json", phrase)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		mlogI("overlay: resolve took too long", "phrase", phrase, "within",
			resolveWithin.String())
		return resolved{}, "the Director did not answer which routine that is within " +
			resolveWithin.String() + ", so nothing has been selected"
	}
	// AN EXIT CODE IS NOT THE ANSWER HERE. `resolve` exits 3 when there is no answer and
	// STILL prints why, which is the thing worth showing; so the output is read either
	// way and only unreadable output is a failure.
	if len(strings.TrimSpace(string(out))) == 0 {
		mlogI("overlay: resolve said nothing", "phrase", phrase, "err", errText(err))
		return resolved{}, "nothing could be asked which routine that is: " +
			marcoBin() + " answered nothing (" + errText(err) + ")"
	}
	var r resolved
	if jsonErr := json.Unmarshal(out, &r); jsonErr != nil {
		mlogW2("overlay: resolve answered something unreadable", "err", jsonErr.Error())
		return resolved{}, "what answered which routine that is could not be read: " +
			jsonErr.Error()
	}
	return r, ""
}

// errText is an error as a log value, and "" for none.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// disposition is what the widget did about a phrase, and what is left for the caller to do.
//
// # Why this is not a bool
//
// Because it was, and the caller read every false as permission to offer a recording. So "which
// of your two routines called settings did you mean?" became "shall I record another one", and a
// Director that could not be asked produced the same offer — which is a LOCAL FALLBACK by
// another route, in a design whose whole point is that there is not one.
//
// Three dispositions, and the middle one is the only thing that may become a learn offer:
//
//	offeredRun     a routine was named and the HUD is asking whether to run it
//	wouldLearn     DIRECTOR said this is something to record, so the learn offer is right
//	saidWhyNot     something was said and NOTHING further is offered — a clarification, a
//	               chat reply, a refusal, a mismatch, a timeout
type disposition int

const (
	offeredRun disposition = iota
	wouldLearn
	saidWhyNot
)

// offerWhatMarcoHas asks Director which routine a phrase named, and answers what it did.
//
// Exactly one offer reaches the HUD, because two prompts at once is a surface asking two
// questions with one answer line.
func offerWhatMarcoHas(h *model, phrase string) disposition {
	r, why := askWhichRoutine(phrase)
	if why != "" {
		// NOTHING COULD BE ASKED. Said, and not turned into an offer to record
		// something: a resolver that is unavailable is not evidence that Marco has
		// nothing, and recording is not the fallback for a service being down.
		h.log(why)
		return saidWhyNot
	}
	// AND A DIRECTOR THAT ANSWERED AND COULD NOT HELP IS THE SAME: its own sentence,
	// and no second offer.
	switch {
	case r.Why != "":
		h.log(r.Why)
		return saidWhyNot
	case r.Mismatch != "":
		h.log(r.Mismatch)
		return saidWhyNot
	}
	a := r.Answer
	switch {
	case a.Kind == "run" && a.ID != "":
		if a.Context.Says != "" {
			h.log(a.Context.Says)
		}
		h.log(a.Resolver)
		offerRoutine(h, a.Name, a.ID, a.App, a.Slug, a.Focus)
		return offeredRun
	case a.Kind == "learn":
		// THE ONE DECISION THAT IS A LEARN OFFER, and it is Director's rather than
		// this surface's reading of a silence.
		h.log(saidOr(a.Reply, "Marco has nothing like that saved."))
		return wouldLearn
	case a.Kind == "run":
		// A RUN WITH NOTHING TO RUN, which cannot be produced by a Director that
		// agrees with itself and is said rather than swallowed: an answer that names
		// a routine and carries no identity is the one thing this surface cannot act
		// on, and silence about it would read as Marco having nothing.
		mlogW2("overlay: a run with no identity", "reply", a.Reply)
		h.log(saidOr(a.Reply, "") + " (nothing was named to run, so nothing has been " +
			"selected)")
		return saidWhyNot
	default:
		// A CLARIFICATION OR A CHAT REPLY. Asking "shall I record one" on top of
		// "which of these did you mean" is the question nobody asked.
		h.log(saidOr(a.Reply, "nothing was said about which routine that is"))
		return saidWhyNot
	}
}

// saidOr is what Director said, or a sentence of this surface's when it said nothing.
//
// Because a HUD that shows an empty line has told somebody nothing happened, and something did.
func saidOr(said, instead string) string {
	if strings.TrimSpace(said) != "" {
		return said
	}
	return instead
}
