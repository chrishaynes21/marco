// Package resolveroute answers which saved routine a request in a person's own words meant.
//
// # Why this is one package and not one per surface
//
// Because it was one per surface, and there was only one surface. Studio grew a request box and
// the whole of this lived in `plugins/studio` — its own module, which the widget cannot import —
// so wiring the widget to it would have meant a second implementation of scope eligibility, a
// second way of building candidate labels, and a second place that decides which routine a
// sentence named. The product would then have had two answers to one question, which is the shape
// of defect this repository keeps finding.
//
// So the policy is here and DIRECTOR OWNS IT. Every surface asks Director, over the service
// protocol it already speaks, and gets back a route identity. Exact matching versus a model's
// opinion is an implementation detail inside this package rather than an ownership boundary: a
// surface does not hold half of a decision.
//
// Director rather than the engine's intake, because the intake is the AUTHORITY door and its
// play lookup is exact on purpose — "near enough" is a judgement, and judgement is Director's.
// Nothing here changes `internal/invoke`.
//
// # What it does not do
//
// It resolves. It does not run, raise, press, or write: what comes back is a route identity and a
// sentence, and the thing that performs a routine is the person pressing Play, which goes through
// the explicit-identity arm of the one intake with every guard it already has.
//
// It does consult a SUBPROCESS, because the configured resolver plugin is one — see
// `internal/dispatch` — and that is the mechanism this exists to reuse rather than something it
// smuggles past the no-execution rule.
//
// See [[ADR-267-director-owns-route-resolution]].
package resolveroute

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

// Candidate is one saved routine a request may be answered with.
//
// A route plus the two things a person needs in order to recognise it, and whether it would run.
// Deliberately NOT the surface's own card type: a card is a thing on a screen and this is a thing
// in a library, and the policy below must not depend on how either is drawn.
type Candidate struct {
	Route routes.Route
	// ID is the route's WHOLE identity — `global/<slug>`, `<app>/context/<slug>`,
	// `<app>/focus/<slug>` — and is the term that travels back to the surface and on to
	// Play. A name is not unique and neither is a slug.
	ID       string
	Name     string
	Scope    string
	Runnable bool
	// Answers is what this routine also answers to, besides its own name.
	//
	// OFFERED AS MORE LABELS FOR THE SAME CANDIDATE, which is the whole of how a phrase
	// is resolved here: `dispatch.Decide`'s first arm is an exact match over the offered
	// labels, so an alias is answered offline, reported as `exact` and `not-asked`, with
	// no plugin and no model server. A library whose routines all answer to several
	// phrases still resolves with nothing running.
	//
	// They are not a second kind of candidate. A label is a way of naming ONE routine,
	// and the identity that travels back is the routine's.
	Answers []string
}

// Slug, App and Focus are the route's, read through the candidate so the policy below reads as
// one thing rather than reaching through a field.
func (c Candidate) Slug() string { return c.Route.Slug }
func (c Candidate) App() string  { return c.Route.App }
func (c Candidate) Focus() bool  { return c.Route.Focus }

// Basis is HOW the application a request is read against came to be known.
//
// # Why this is a field and not a string
//
// Because "which application is in front" is the one fact a surface cannot answer honestly about
// itself. Studio owns the foreground whenever somebody types into it, and the widget is excluded
// from the window system's candidate set — so neither of them can say, and Director's own world
// model cannot simply be trusted either:
//
//   - The window-system provider takes its foreground claim from `winprovider.offered()`, which
//     excludes every Marco-owned surface. With Studio in front, NO application observation is
//     produced at all, so the last one stands — naming an application that is no longer in front,
//     with nothing in the value saying so.
//   - The accessibility provider takes the bridge's first window with no such exclusion, so it
//     can report a Marco window as the active application.
//
// Both were traced. So an observation's AGE is evidence and not proof, and admitting a
// context-only routine needs the foreground corroborated NOW, through the path that excludes
// Marco's own windows. What cannot be corroborated is reported as what it is.
type Basis string

const (
	// Stated is an application the person named. The authoritative answer, because it is
	// the only one that is a statement of intent rather than an observation.
	Stated Basis = "stated"
	// Observed is Director's world model, CORROBORATED against the live foreground.
	Observed Basis = "observed"
	// LastExternal is an application that was in front and is not known to be now: the
	// observation exists and the corroboration did not agree, or could not be made.
	LastExternal Basis = "last-external"
	// Unknown is no application at all.
	Unknown Basis = "unknown"
)

// Admits reports whether this basis may offer a routine that works only in place.
//
// Stated and Observed do. The other two do NOT: a routine that works only while its application
// is in front cannot be offered on the strength of a guess about what is in front, and the cost of
// being wrong is a routine pressing at coordinates in somebody else's window. Global and
// anywhere-in-application routines are eligible under every basis, because neither depends on it.
func (b Basis) Admits() bool { return b == Stated || b == Observed }

// Context is the application a request is read against, and how that was arrived at.
type Context struct {
	App   string `json:"app,omitempty"`
	Basis Basis  `json:"basis"`
	// Says is the sentence a surface shows beside the request.
	Says string `json:"says,omitempty"`
}

// Answer is what a resolution came to.
//
// The field names are the ones Studio's page already reads, so moving the policy here did not
// move the surface.
type Answer struct {
	// Kind is dispatch's own four words plus "none": run, learn, chat, clarify.
	Kind string `json:"kind"`
	// ID is the routine's whole identity, and App, Scope and Slug are the parts of it
	// carried separately so an invocation can be built without taking the string apart.
	// Empty for anything but a run.
	ID    string `json:"id,omitempty"`
	App   string `json:"app,omitempty"`
	Slug  string `json:"slug,omitempty"`
	Focus bool   `json:"focus,omitempty"`
	Name  string `json:"name,omitempty"`
	Scope string `json:"scope,omitempty"`
	Reply string `json:"reply,omitempty"`
	// Context is what this was read against, with its provenance.
	Context Context `json:"context"`
	// Offered is how many routines were eligible, which is the honest denominator.
	Offered int `json:"offered"`
	// Candidates is what a clarification narrowed to, by identity.
	Candidates []string `json:"candidates,omitempty"`
	// Source is which arm answered — exact, advisor, fallback, none — and Resolver is a
	// sentence about the configured resolver's part in it. SHOWN, NOT USED: two of the
	// three arms need no model, so a surface built on this could work and never once
	// reach the plugin.
	Source   string `json:"source"`
	Resolver string `json:"resolver"`
	// Library is the route library this was resolved against. A surface compares it with
	// its own and NAMES a mismatch rather than selecting from somewhere else.
	Library string `json:"library"`
}

// Resolver answers one request against one library.
type Resolver struct {
	// Reg is the library, which is Director's: `marcopaths.Routes()`.
	Reg routes.Registry
	// Advisor is the configured resolver plugin, or nil. A test substitutes it.
	Advisor dispatch.Advisor
}

// Resolve reads one request against the eligible routines and answers which one it meant.
func (r Resolver) Resolve(ctx context.Context, request string, cs []Candidate,
	where Context) Answer {

	request = strings.TrimSpace(request)
	said := Answer{Kind: dispatch.IntentNone, Source: dispatch.SourceNone,
		Context: where, Library: r.Reg.Dir,
		Resolver: ResolverSentence(r.Advisor, "")}
	if request == "" {
		said.Reply = "say what you want, in your own words"
		return said
	}

	eligible := Eligible(cs, where)
	names, phrases := answerCounts(eligible)
	labels, byLabel := labelled(eligible)
	said.Offered = len(labels)
	if len(labels) == 0 {
		said.Reply = "nothing in this library is eligible: " + whyNothing(where)
		return said
	}

	// THE RESOLVER IS TOLD WHICH APPLICATION, which is the same fact that filtered the
	// candidates. It is context for the question and not another filter on the answer.
	dec := dispatch.New(r.Advisor).Decide(ctx, request, labels, where.App)
	said.Kind, said.Source = dec.Intent, dec.Source
	said.Resolver = ResolverSentence(r.Advisor, dec.Advisor)
	said.Reply = dec.Reply

	switch dec.Intent {
	case dispatch.IntentRun:
		// THE LABEL IS A KEY INTO THE MAP AND IS NOT RESOLVED AGAIN. A name is not
		// unique and neither is a slug, and a second lookup is how a surface comes
		// to display one routine and perform another.
		c, ok := byLabel[dec.Route]
		if !ok {
			said.Kind, said.Reply = dispatch.IntentClarify,
				"something was suggested that is not one of the routines "+
					"offered, so nothing has been selected"
			return said
		}
		said.ID, said.Name, said.Scope = c.ID, c.Name, c.Scope
		said.App, said.Slug, said.Focus = c.App(), c.Slug(), c.Focus()
		if said.Reply == "" {
			said.Reply = fmt.Sprintf("%s — %s", c.Name, c.Scope)
		}
	case dispatch.IntentClarify:
		// A NEAR MISS NAMES NOTHING TO PRESS. A question with the answer already
		// filled in is not a question.
		said.ID = ""
	}

	// AND A NAME MORE THAN ONE ROUTINE HAS IS A QUESTION, NOT A NEAR MISS.
	//
	// Nothing above can have answered it exactly, because a shared name belongs to no
	// label. What is left is a resolver that resolved it — kept, and saying `advisor` —
	// or a deterministic near miss on one of the qualified labels, which would name an
	// arbitrary one of them, or an offer to RECORD a routine of a name three already
	// answer to. The last two are the ambiguity answered rather than asked.
	if said.Kind != dispatch.IntentRun && askedAbout(nlu.Key(request), names, phrases) {
		said.Kind, said.ID = dispatch.IntentClarify, ""
		said.Name, said.Scope, said.App, said.Slug, said.Focus = "", "", "", "", false
		said.Reply, said.Candidates = whichOnesAnswerTo(request, eligible)
	}
	return said
}

// Eligible is the routines a request may be answered with, given the context and its provenance.
//
// Three rules, one per scope, and the whole of what keeps them apart:
//
//	GLOBAL                    anywhere. Always eligible.
//	ANYWHERE-IN-APPLICATION   brings its own application forward, so it is eligible from
//	                          wherever you are — across applications, which is its point.
//	APPLICATION/CONTEXT       works only while its application is in front. Eligible ONLY
//	                          when that application is the context AND the context is one
//	                          that may be relied on. See Basis.Admits.
func Eligible(cs []Candidate, where Context) []Candidate {
	out := []Candidate{}
	for _, c := range cs {
		switch {
		case !c.Runnable:
			// NOT OFFERED AT ALL. A routine that would refuse is not an answer to
			// anything, and offering it would make the resolver's best case a
			// dead end.
		case c.App() == "":
			out = append(out, c) // global
		case c.Focus():
			out = append(out, c) // anywhere-in-application: it raises its own
		case where.Basis.Admits() && where.App != "" && c.App() == where.App:
			out = append(out, c) // in place, and this is the application
		}
	}
	return out
}

// answerCounts is how many eligible routines are CALLED each key, and how many ANSWER to it by
// phrase — counted apart, because a name outranks a phrase.
//
// By the matcher's key rather than by the slug, for the same reason uniqueness is checked that
// way: two slugs it cannot tell apart are ONE name as far as a request is concerned.
//
// # Why apart, and what counting them together cost
//
// Together, one routine's phrase could drag another routine's OWN NAME into ambiguity. A routine
// called `settings` and a second taught to answer to "settings" made the key shared, so the first
// lost its own name in the labelling pass and a request for `settings` became a question — about
// a routine that is actually called that. The product's own rule is that the slug wins; counting
// a phrase as equal to a name broke it inside the resolver while `routes` kept it everywhere
// else.
//
// Once each per routine, because a routine answering to two phrases that reduce to one key is
// still one routine and must not look like two.
func answerCounts(cs []Candidate) (names, phrases map[string]int) {
	names, phrases = map[string]int{}, map[string]int{}
	for _, c := range cs {
		if k := nlu.Key(c.Slug()); k != "" {
			names[k]++
		}
		seen := map[string]bool{}
		for _, a := range c.Answers {
			k := nlu.Key(a)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			phrases[k]++
		}
	}
	return names, phrases
}

// askedAbout reports whether a request names more than one routine, under that precedence.
//
// A key more than one routine is CALLED is a question, whatever any phrase says. A key no routine
// is called, that more than one answers to, is also a question. A key one routine is called is
// NOT a question — the name wins, and whatever phrases also reduce to it are shadowed by it.
//
// THAT LAST CASE IS DEFENSIVE AND NO TEST CAN SEE IT REMOVED, which is worth saying rather than
// leaving as an untested branch. A sole canonical name always claims its own label in `labelled`,
// so an exact request for it is answered by the offline arm and this function is never reached —
// the guard above it is `said.Kind != dispatch.IntentRun`. It is here because the precedence is a
// rule and not an accident of which arm happened to answer first: a resolver that stopped matching
// a canonical name exactly would otherwise start asking questions about routines that have the
// name somebody typed.
func askedAbout(key string, names, phrases map[string]int) bool {
	if names[key] > 0 {
		return names[key] > 1
	}
	return phrases[key] > 1
}

// whichOnesAnswerTo is the clarification for a name more than one routine has, and their
// identities.
//
// In the product's OWN words for a scope (`plays.Scope.Says`) rather than a second set written
// here: the difference between the three is the thing being asked about.
func whichOnesAnswerTo(request string, cs []Candidate) (string, []string) {
	key := nlu.Key(request)
	// UNDER THE SAME PRECEDENCE THE COUNTING USES. When routines are CALLED this, they
	// are the question and a phrase that also reduces to it is not part of it — listing
	// the phrase holder beside two namesakes would offer somebody a third routine the
	// name does not belong to.
	byName := false
	for _, c := range cs {
		if nlu.Key(c.Slug()) == key {
			byName = true
			break
		}
	}
	var says, ids []string
	for _, c := range cs {
		// BY ITS OWN NAME OR BY A PHRASE IT ANSWERS TO, because from the person's
		// side those are the same claim on the same words — and the sentence names
		// WHICH, because "settings and settings" tells somebody nothing while
		// "settings, and the one you taught to answer to settings" tells them where
		// to look.
		under := ""
		if nlu.Key(c.Slug()) == key {
			under = c.Name
		} else if !byName {
			for _, a := range c.Answers {
				if nlu.Key(a) == key {
					under = fmt.Sprintf("%s, which answers to %q",
						c.Name, a)
					break
				}
			}
		}
		if under == "" {
			continue
		}
		says = append(says, fmt.Sprintf("%s (%s)", under,
			plays.ScopeOf(c.Route).Says(c.App())))
		ids = append(ids, c.ID)
	}
	sort.Strings(says)
	sort.Strings(ids)
	return fmt.Sprintf("%q is the name of %d routines, and nothing has said which you "+
		"mean: %s", request, len(says), strings.Join(says, "; ")), ids
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
// `settings-ark` both reduce to "settings ark". The check is `nlu.Key`, which is the matcher's own
// reduction rather than a copy of it.
//
// # Two passes, because order was deciding who kept their own name
//
// Every routine claims its OWN SLUG first. One pass gave the first routine in the listing
// whatever it asked for, so a routine genuinely slugged `settings-in-ark` would lose its own name
// to a derived label for `settings` saved in Ark. Both stayed distinct and both mapped correctly,
// so nothing was WRONG; it was arbitrary on the listing's order, which is the kind of thing that
// is right in the test and wrong on somebody's machine.
//
// # And a SHARED name belongs to nobody
//
// Unique labels stop two routines being confused for one. They do not answer the PERSON's
// ambiguity: with the bare slug handed to one of the namesakes, a request for "settings" is an
// exact match on that label — answered offline, selecting whichever routine the listing happened
// to put first, with neither the resolver nor the person asked which of the three was meant.
func labelled(cs []Candidate) ([]string, map[string]Candidate) {
	byLabel := map[string]Candidate{}
	keys := map[string]bool{}
	labels := []string{}
	take := func(label string, c Candidate) bool {
		key := nlu.Key(label)
		if label == "" || keys[key] || byLabel[label].ID != "" {
			return false
		}
		keys[key] = true
		byLabel[label] = c
		labels = append(labels, label)
		return true
	}
	names, phrases := answerCounts(cs)
	// PASS ONE: ITS OWN NAME, for everything that can have it AND is the only one asking.
	//
	// Counted on NAMES alone, so a phrase somebody taught another routine cannot take a
	// routine's own name away from it. See answerCounts.
	rest := []Candidate{}
	for _, c := range cs {
		if names[nlu.Key(c.Slug())] > 1 || !take(c.Slug(), c) {
			rest = append(rest, c)
		}
	}
	// PASS TWO: THE PHRASES SOMEBODY CHOSE, before any label this package invents.
	//
	// A phrase a person taught a routine is what they are likely to type; `settings-in-ark`
	// is what is left when two routines want one name. So a phrase claims its key first,
	// and a derived label that wanted the same string takes the counter below.
	//
	// A SHARED PHRASE IS CLAIMED BY NOBODY, which is the same rule pass one applies to a
	// shared slug and it is load-bearing for the same reason: handed to one of the
	// routines that answers to it, a request for that phrase is an exact match on that
	// label — answered offline, selecting whichever the listing happened to put first,
	// with neither the resolver nor the person asked. `sharedNames` counts phrases, so a
	// phrase two routines answer to, or one that is another routine's actual name, has no
	// label at all and reaches the clarification below.
	for _, c := range cs {
		for _, a := range c.Answers {
			k := nlu.Key(a)
			// A NAME WINS, so a phrase reducing to one is not offered at all —
			// including the routine's own name, which it would add nothing to.
			// And a phrase two routines answer to is claimed by neither.
			if names[k] > 0 || phrases[k] > 1 {
				continue
			}
			take(a, c)
		}
	}
	// PASS THREE: a derived label for whoever is left.
	for _, c := range rest {
		if take(scopedLabel(c), c) {
			continue
		}
		// AND A COUNTER, because the scoped label is allowed to fail too: it can be
		// another routine's actual slug, and that routine has already been given it.
		for n := 2; n <= len(cs)+2; n++ {
			if take(fmt.Sprintf("%s-%d", scopedLabel(c), n), c) {
				break
			}
		}
		// A routine that still has no label is left out of the offer, which cannot
		// happen and is the right answer if it ever does: not offering one costs
		// somebody a click, and offering it under a label that names another one
		// would select the wrong routine.
	}
	return labels, byLabel
}

// scopedLabel is a routine's slug with its scope spelled out in words a person would use.
//
// Slug-shaped, because that is what the resolver prompt asks for and what the matcher reads.
func scopedLabel(c Candidate) string {
	switch {
	case c.App() == "":
		return c.Slug() + "-anywhere"
	case c.Focus():
		return c.Slug() + "-anywhere-in-" + routes.Slug(c.App())
	default:
		return c.Slug() + "-in-" + routes.Slug(c.App())
	}
}

// whyNothing says which scopes were and were not available, when none were.
func whyNothing(where Context) string {
	if !where.Basis.Admits() {
		return "nothing is known to be in front, so no in-place routine is offered — " +
			"name an application, or save what you want as a routine that works " +
			"anywhere"
	}
	return fmt.Sprintf("there is no runnable global routine, no routine that brings an "+
		"application forward, and none in place for %s", where.App)
}

// ResolverSentence says what part the configured resolver had in one answer.
//
// # Why this is a sentence and not a flag
//
// Because "it is configured" and "it answered" are different facts and both matter, and because
// two of the three arms need no model at all — so a surface built on this could work, look right,
// and never once reach the plugin. The state that used to be invisible is the third: configured,
// asked, and silent, which looks exactly like never having configured one.
//
// It does not claim WHICH kind of silence. `dispatch.Advisor` reports "I have nothing" for a
// plugin that declined, a plugin that is missing, a model server that is down and a reply that
// would not parse, and inventing a distinction here would be a guess about somebody's machine.
func ResolverSentence(a dispatch.Advisor, state string) string {
	if a == nil {
		return "no resolver is configured, so only an exact name or a near match can be " +
			"found — set $MARCO_RESOLVER (setup.ps1 points it at plugins\\llama)"
	}
	switch state {
	case dispatch.AdvisorAnswered:
		return "the configured resolver answered this: " + dispatch.PluginPath()
	case dispatch.AdvisorNotAsked:
		// AND IT SAYS IT WAS NOT ASKED, which is not the same as having answered: a
		// free offline match crediting a resolver that was never consulted is the
		// false impression this whole sentence exists to prevent.
		return "a resolver is configured and was NOT asked, because this was " +
			"answered offline: " + dispatch.PluginPath()
	case dispatch.AdvisorSilent:
		return "the configured resolver was asked and did not answer, so this is a " +
			"deterministic match: " + dispatch.PluginPath() +
			" — check that the model server is running"
	}
	return "a resolver is configured: " + dispatch.PluginPath()
}
