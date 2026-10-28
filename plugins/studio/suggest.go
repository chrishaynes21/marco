package main

// SAY WHAT YOU WANT, AND STUDIO OFFERS THE ROUTINE IT THINKS YOU MEANT.
//
// # What this is, and what it deliberately is not
//
// It is a SUGGESTION. Nothing here starts a process, raises a window or presses anything: it
// answers with one routine's identity and a sentence, and the thing that runs it is the Play
// button that was already there — `PlayRoutine`, unchanged, with every guard it already has. The
// model proposes and the person decides, which is the rule `cmd/marco`'s own Advisor arm learnt
// the hard way: an earlier version of that ran the play a model named with no question asked.
//
// # And the resolver it asks was already here, and nothing was calling it
//
// `internal/dispatch` has had the whole of this for a long time — a Dispatcher, an Advisor
// interface, a plugin client that speaks to a local Ollama through `plugins/llama`, and a
// `validate` that accepts a run only when the route is one the caller offered. `setup.ps1` pins
// `$MARCO_RESOLVER` and `$MARCO_ASSISTANT` at that plugin, so on a set-up machine it is
// configured and ready.
//
// It was reached by `marco dispatch` and by the `marco assistant` REPL, and by nothing a person
// uses. The overlay — the widget — never called it: its one door is `marco do --source=typed`,
// whose play lookup is exact and deliberately not fuzzy. So this reuses an existing resolver THAT
// THE WIDGET WAS NOT ACTUALLY CALLING, and that correction is part of the record rather than a
// detail: the code was complete, tested, configured by the installer, and dead.
//
// Which is why a decision now says where it came from. See the Source fields below.
//
// # Scopes are preserved, and the target application is STATED
//
// A route IS its scope (see internal/routes), and the three mean different things:
//
//	GLOBAL                    anywhere. Always eligible.
//	ANYWHERE-IN-APPLICATION   brings its own application forward, so it is eligible from
//	                          wherever you are — across applications, which is its point.
//	APPLICATION/CONTEXT       works only while its application is in front. Eligible ONLY
//	                          when that application is the stated target.
//
// The target is CHOSEN, never inferred. Two inferences were available and both are wrong. Studio
// owns the foreground whenever somebody types in it, so reading the foreground would answer
// "Studio" — and treating "this application has a window" as eligibility would make every running
// application's in-place commands reachable from anywhere, which erases the distinction between
// the two app scopes entirely. With no target stated, no context routine is offered, and the
// answer says so.
//
// See [[ADR-265-say-what-you-want-and-studio-offers-the-routine]].

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Target is one application a request may be aimed at, as the page offers it.
type Target struct {
	// App is the application as the library names it, and is the term the page hands back.
	App string `json:"app"`
	// Open is whether it has a window right now. A HINT AND NOT A FILTER: it is shown so
	// somebody can see that the application their in-place command needs is not running,
	// and it does not decide eligibility, because the choice is theirs to make.
	Open bool `json:"open"`
	// Context is how many in-place routines it has, which is what choosing it unlocks.
	Context int `json:"context"`
}

// Suggestion is what Studio made of one request.
type Suggestion struct {
	// Kind is what to do about it, in dispatch's own four words plus "none":
	// `run` (ID names the routine), `learn`, `chat`, `clarify`, `none`.
	Kind string `json:"kind"`
	// ID is the routine's WHOLE identity, exactly as a card carries it, and is what the
	// page passes to PlayRoutine. Empty for anything but `run`.
	//
	// It comes out of the label→identity map built below and is NEVER re-resolved from
	// the label, the name or the slug: a name is not unique and neither is a slug, and a
	// second lookup is how a surface comes to display one routine and perform another.
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Scope string `json:"scope,omitempty"`
	// Reply is the sentence to show: the resolver's own words where it gave some, this
	// window's where it did not.
	Reply string `json:"reply,omitempty"`
	// Target is the application this request was read against, echoed back so the page
	// renders one fact from one source rather than its own copy of what it sent.
	Target string `json:"target"`
	// Offered is how many routines were eligible, which is the honest denominator for
	// everything above: a suggestion out of two candidates and out of forty are different
	// claims.
	Offered int `json:"offered"`
	// Source is which arm decided — `exact`, `advisor`, `fallback`, `none` — and Resolver
	// is a sentence about the configured resolver's part in it.
	//
	// SHOWN, NOT USED. Nothing here branches on them. They exist because the three arms
	// are indistinguishable in their answers and two of them need no model at all, so a
	// window built on this could work, look right, and never once reach Ollama. That is a
	// shape this product has shipped before.
	Source   string `json:"source"`
	Resolver string `json:"resolver"`
}

// suggestAdvisor is the Advisor this window asks, or a test's stand-in.
//
// A seam of the same kind as `listing` and `raising`: the real one shells out to a plugin that
// talks to a model server, and the behaviour worth holding — which candidates are offered, which
// identity comes back, what is said when the resolver is silent — must be exercisable without
// one. Nil means ask the configured plugin, which is every production path.
//
// A LIVE TEST ASKS THE REAL ONE. A stand-in establishes the policy and nothing about the
// connection; see suggestlive_test.go, which runs the actual plugin against the actual model and
// asserts the identity that comes back.
func (a *Acts) suggestAdvisor() dispatch.Advisor {
	if a.advisor != nil {
		return a.advisor
	}
	return dispatch.Default()
}

// SuggestTargets is the applications a request can be aimed at.
//
// Every application in the library that has at least one in-place routine, because those are the
// ones a target CHANGES anything for: a global routine and an anywhere-in-application one are
// eligible whatever is chosen. Sorted, so the page is stable between polls.
func (a *Acts) SuggestTargets() []Target {
	open := map[string]bool{}
	for _, c := range a.windowsNow() {
		if c.Application != "" {
			open[strings.ToLower(c.Application)] = true
		}
	}
	count := map[string]int{}
	for _, r := range a.Routines() {
		if r.App != "" && !r.Focus {
			count[r.App]++
		}
	}
	out := []Target{}
	for app, n := range count {
		out = append(out, Target{App: app, Open: open[strings.ToLower(app)], Context: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].App < out[j].App })
	return out
}

// sharedNames counts how many eligible routines answer to each name.
//
// By the matcher's key rather than by the slug, for the same reason uniqueness is checked that
// way: two slugs it cannot tell apart are ONE name as far as a request is concerned, whatever
// they look like on disk. One place, because two counts of the same fact could disagree about
// which requests are ambiguous — and the whole point is that an ambiguous one is not answered
// silently.
func sharedNames(rs []Routine) map[string]int {
	out := map[string]int{}
	for _, r := range rs {
		out[nlu.Key(r.Slug)]++
	}
	return out
}

// whichOnesAnswerTo is the clarification for a name more than one routine has.
//
// In the product's OWN words for a scope (`plays.Scope.Says`) rather than a second set written
// here: the difference between the three is the thing being asked about, so it has to be put the
// way every other surface puts it.
func whichOnesAnswerTo(request string, rs []Routine) string {
	key := nlu.Key(request)
	var says []string
	for _, r := range rs {
		if nlu.Key(r.Slug) != key {
			continue
		}
		says = append(says, fmt.Sprintf("%s (%s)", r.Name,
			plays.ScopeOf(r.route()).Says(r.App)))
	}
	sort.Strings(says)
	return fmt.Sprintf("%q is the name of %d routines, and nothing has said which you "+
		"mean: %s", request, len(says), strings.Join(says, "; "))
}

// eligible is the routines a request may be answered with, given the stated target.
//
// Three rules, one per scope, and the whole of what keeps them apart. See the file comment.
func eligible(all []Routine, target string) []Routine {
	out := []Routine{}
	for _, r := range all {
		switch {
		case !r.Runnable:
			// NOT OFFERED AT ALL. A routine that would refuse is not an answer to
			// anything, and offering it would make the model's best case a dead end.
		case r.App == "":
			out = append(out, r) // global
		case r.Focus:
			out = append(out, r) // anywhere-in-application: it raises its own
		case target != "" && r.App == target:
			out = append(out, r) // in-place, and this is the application
		}
	}
	return out
}

// labelled gives every eligible routine a label the matcher can tell apart, and answers the map
// from label back to the routine.
//
// # Why a label and not the identity
//
// Because the identity is path-shaped — `ark/context/settings` — and the deterministic matcher
// strips anything that is not alphanumeric or a space WITHOUT inserting one, so that reduces to
// "arkcontextsettings" and matches nothing anybody would type. The resolver prompt asks for a
// slug, too. So what is offered is slug-shaped and the identity travels beside it.
//
// # Why uniqueness is checked on the matcher's own key
//
// Because two labels that differ as strings can be ONE candidate to the matcher: a hyphen is a
// space to it and "in" is a stop word, so `settings-in-ark` and a routine genuinely slugged
// `settings-ark` both reduce to "settings ark". Appending scope text can therefore collide with
// another routine's actual slug — at which point an answer naming either is an answer naming
// both. The check is `nlu.Key`, which is the matcher's own reduction rather than a copy of it, so
// this cannot come to a different conclusion from the thing doing the comparing.
//
// The ladder is: the slug; then the slug with its scope spelled out; then a counter. The counter
// is never pretty and always works — digits survive the normalization — and it is reached only
// when a library genuinely holds two routines this cannot otherwise tell apart.
//
// # And it is two passes, because order was deciding who kept their own name
//
// Every routine claims its OWN SLUG first, and only then does anything derive a label. One pass
// gave the first routine in the listing whatever it asked for: a routine genuinely slugged
// `settings-in-ark` would lose its own name to a derived label for `settings` saved in Ark, if
// the listing happened to put that one first — and then get a label derived from a name that was
// never its. Both stayed distinct and both mapped correctly, so nothing was WRONG; it was
// arbitrary, and arbitrary on the listing's order is the kind of thing that is right in the test
// and wrong on somebody's machine.
//
// # And a SHARED name belongs to nobody, because unique labels are not an answer to ambiguity
//
// The first version of the two passes still handed the bare slug to one of the namesakes — the
// first eligible `settings` got `settings` and the others were qualified. Every label was then
// distinct and every one mapped to the right identity, so the collision problem was solved; the
// PERSON's problem was not. A request for "settings" is an exact match on that label, which means
// the deterministic arm answers it offline, selects whichever routine the listing happened to put
// first, and never asks the resolver or the person which of the three was meant. An ambiguity
// resolved by listing order, reported as an exact match.
//
// So when eligible routines share a normalized name, EVERY ONE of them is qualified and the bare
// name is left belonging to nobody. A request for it then matches no label exactly: the resolver
// is asked — with the target stated, which is what it is for — and if nothing answers, the
// deterministic arm has only near misses, which clarify and select nothing.
func labelled(rs []Routine) ([]string, map[string]Routine) {
	byLabel := map[string]Routine{}
	keys := map[string]bool{}
	labels := []string{}
	take := func(label string, r Routine) bool {
		key := nlu.Key(label)
		if label == "" || keys[key] || byLabel[label].ID != "" {
			return false
		}
		keys[key] = true
		byLabel[label] = r
		labels = append(labels, label)
		return true
	}
	// WHICH NAMES ARE SHARED, asked before anything claims one.
	shared := sharedNames(rs)
	// PASS ONE: ITS OWN NAME, for everything that can have it AND is the only one asking.
	rest := []Routine{}
	for _, r := range rs {
		if shared[nlu.Key(r.Slug)] > 1 || !take(r.Slug, r) {
			rest = append(rest, r)
		}
	}
	// PASS TWO: a derived label for whoever is left.
	for _, r := range rest {
		if take(scopedLabel(r), r) {
			continue
		}
		// AND A COUNTER, because the scoped label is allowed to fail too: it can be
		// another routine's actual slug, and that routine has already been given it
		// above. Bounded by the number of routines, since every attempt that fails
		// means another routine already holds that label.
		for n := 2; n <= len(rs)+2; n++ {
			if take(fmt.Sprintf("%s-%d", scopedLabel(r), n), r) {
				break
			}
		}
		// A ROUTINE THAT STILL HAS NO LABEL IS LEFT OUT OF THE OFFER, which cannot
		// happen — each attempt above consumes a distinct label, so one of them is
		// free — and is the right answer if it ever does: not offering a routine costs
		// somebody a click, and offering it under a label that names another one would
		// select the wrong routine.
	}
	return labels, byLabel
}

// scopedLabel is a routine's slug with its scope spelled out in words a person would use.
//
// Slug-shaped, because that is what the resolver prompt asks for and what the matcher reads: the
// hyphens become spaces on both sides of the boundary, so `settings-anywhere-in-ark` is "settings
// anywhere ark" to the matcher and an exact string to the plugin.
func scopedLabel(r Routine) string {
	switch {
	case r.App == "":
		return r.Slug + "-anywhere"
	case r.Focus:
		return r.Slug + "-anywhere-in-" + routes.Slug(r.App)
	default:
		return r.Slug + "-in-" + routes.Slug(r.App)
	}
}

// SuggestRoutine reads one request against the library and answers which routine it meant.
//
// IT PERFORMS NOTHING. The page shows what comes back and the person presses Play, which is the
// button and the guards that were already there.
func (a *Acts) SuggestRoutine(request, target string) Suggestion {
	request = strings.TrimSpace(request)
	target = strings.TrimSpace(target)
	said := Suggestion{Kind: dispatch.IntentNone, Target: target,
		Source: dispatch.SourceNone, Resolver: resolverSentence(a.suggestAdvisor(), "")}
	if request == "" {
		said.Reply = "say what you want, in your own words"
		return said
	}

	rs := eligible(a.Routines(), target)
	shared := sharedNames(rs)
	labels, byLabel := labelled(rs)
	said.Offered = len(labels)
	if len(labels) == 0 {
		said.Reply = "nothing in this library is eligible: " + whyNothing(target)
		return said
	}

	dec := dispatch.New(a.suggestAdvisor()).
		Decide(context.Background(), request, labels, "")
	said.Kind, said.Source = dec.Intent, dec.Source
	said.Resolver = resolverSentence(a.suggestAdvisor(), dec.Advisor)
	said.Reply = dec.Reply

	switch dec.Intent {
	case dispatch.IntentRun:
		// THE LABEL IS A KEY INTO THE MAP AND IS NOT RESOLVED AGAIN.
		//
		// `dispatch.validate` has already refused anything that is not one of the
		// labels offered, so this lookup cannot miss for a reason anybody can arrange
		// — and if it ever did, the answer is no suggestion rather than a routine
		// found by name. A name is not unique and neither is a slug.
		r, ok := byLabel[dec.Route]
		if !ok {
			said.Kind, said.Reply = dispatch.IntentClarify,
				"something was suggested that is not one of the routines offered, "+
					"so nothing has been selected"
			return said
		}
		said.ID, said.Name, said.Scope = r.ID, r.Name, r.Scope
		if said.Reply == "" {
			said.Reply = fmt.Sprintf("%s — %s", r.Name, r.Scope)
		}
	case dispatch.IntentClarify:
		// THE NEAR MISS NAMES NOTHING TO PRESS. `dispatch` puts its candidate in
		// `Route` for a caller that wants to hint at it; this one does not select it,
		// because a question with the answer already filled in is not a question.
		said.ID = ""
	}

	// ── AND A NAME MORE THAN ONE ROUTINE HAS IS A QUESTION, NOT A NEAR MISS ──
	//
	// Nothing above can have answered it exactly, because a shared name belongs to no
	// label (see labelled). What is left is a resolver that resolved it — which is kept,
	// and says `advisor`, and is the resolver doing its job with the target it was given
	// — or a deterministic near miss on one of the qualified labels, which would read as
	// "did you mean settings anywhere?" and name an arbitrary one of them. Or an offer to
	// RECORD a routine called `settings` when three already answer to it.
	//
	// Both of those last two are the ambiguity being answered rather than asked, so this
	// replaces them with the question and the alternatives.
	if said.Kind != dispatch.IntentRun && shared[nlu.Key(request)] > 1 {
		said.Kind, said.ID = dispatch.IntentClarify, ""
		said.Name, said.Scope = "", ""
		said.Reply = whichOnesAnswerTo(request, rs)
	}
	return said
}

// whyNothing says which scopes were and were not available, when none were.
func whyNothing(target string) string {
	if target == "" {
		return "no application is targeted, so no in-place routine is offered — " +
			"choose one, or save what you want as a global routine"
	}
	return fmt.Sprintf("there is no runnable global routine, no routine that brings an "+
		"application forward, and none in place for %s", target)
}

// resolverSentence says what part the configured resolver had in this.
//
// # Why this is a sentence and not a flag
//
// Because "it is configured" and "it answered" are different facts and both matter. A person who
// installed Ollama wants to know their resolver is being reached; a person who has not wants to
// know that is why only exact names work. And the third state is the one that used to be
// invisible: configured, asked, and silent — which looks exactly like never having configured one.
//
// It does not claim WHICH kind of silence. `dispatch.Advisor` reports "I have nothing" for a
// plugin that declined, a plugin that is missing, a model server that is down and a reply that
// would not parse, and inventing a distinction here would be a guess about somebody's machine.
func resolverSentence(a dispatch.Advisor, state string) string {
	if a == nil {
		return "no resolver is configured, so only an exact name or a near match can be " +
			"found — set $MARCO_RESOLVER (setup.ps1 points it at plugins\\llama)"
	}
	switch state {
	case dispatch.AdvisorAnswered:
		return "the configured resolver answered this: " + dispatch.PluginPath()
	case dispatch.AdvisorNotAsked:
		// AND IT SAYS IT WAS NOT ASKED, which is not the same as having answered.
		// This used to read "answered this" for an exact name — a free offline match
		// crediting a resolver that was never consulted, which is the false
		// impression this whole sentence exists to prevent.
		return "a resolver is configured and was NOT asked, because this was " +
			"answered offline: " + dispatch.PluginPath()
	case dispatch.AdvisorSilent:
		return "the configured resolver was asked and did not answer, so this is a " +
			"deterministic match: " + dispatch.PluginPath() +
			" — check that the model server is running"
	}
	return "a resolver is configured: " + dispatch.PluginPath()
}
