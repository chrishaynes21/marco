package orchestrator

// TEACHING A ROUTINE ANOTHER PHRASE, AND REFUSING ONE THAT WOULD ONLY EVER BE A QUESTION.
//
// `routes` stores the phrases and matches them; this is the operation. The difference is the
// LIBRARY: storing a phrase is a fact about one routine, and deciding whether it may be added is a
// fact about every routine, because the cost of a phrase two routines answer to is paid by whoever
// says it.
//
// # Two moments, and they answer different questions
//
// A phrase more than one eligible routine answers to selects NOTHING and asks which — that is
// `resolveroute`'s rule and it holds whatever happens here, because a collision can appear long
// after a phrase was added: a rename, a move, or a routine saved later. What this adds is a
// refusal at the one moment somebody can choose a different phrase instead. It is a COURTESY and
// not the guarantee, and saying so matters: a product that refuses at the add tends to grow a
// resolution path that assumes the collision cannot happen.
//
// # And it refuses on BOTH normalizations
//
// The door folds with `routes.Slug` and Director's resolver compares on `nlu.Key`, which also
// drops hyphens and stop words — so `settings-in-ark` and `settings-ark` are two phrases to one
// and ONE phrase to the other. A phrase that is distinct at the door and ambiguous in the resolver
// is a phrase whose meaning depends on which surface was asked, so either reduction colliding is
// a refusal.
//
// # A removal names the PHRASE
//
// Not its position. The step-editing slice learnt what a row number stated against a list that has
// changed costs, and a phrase is its own identity — so there is no index to misapply and no basis
// token needed. What the write still needs is the ordinary version binding, which is the route
// lock and a re-read inside it.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"fmt"
	"slices"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Aliased is what a routine answers to after an add or a remove, and what it says about it.
type Aliased struct {
	// ID is the routine this is about, and Name its own name.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Answers is every phrase it now answers to, besides its own name.
	Answers []string `json:"answers"`
	// Says is the sentence a person reads.
	Says string `json:"says,omitempty"`
}

// AliasesOf is what a routine answers to, and why it cannot be said when it cannot.
//
// The read every surface listing phrases goes through, so that "there is no file", "the file
// belongs to something else" and "the file cannot be read" are three different sentences rather
// than one empty list. An unoffered state yields NO phrases, whatever the file holds: a surface
// that listed them would be offering phrases the door will not answer to.
func (d Deps) AliasesOf(rt routes.Route) (phrases []string, why string) {
	s, st := d.Reg.Aliases(rt)
	switch st {
	case routes.AliasesNone, routes.AliasesIntact:
		return s.Phrases, ""
	case routes.AliasesElsewhere:
		return nil, fmt.Sprintf("the phrases stored beside %q say they belong to %s, "+
			"so none of them answers for this routine — they were left by a "+
			"move that did not finish, or by a routine that used to be saved "+
			"here", rt.Slug, s.Route)
	case routes.AliasesOrphaned:
		return nil, fmt.Sprintf("there are phrases stored at %s and no routine there "+
			"to answer to them", rt.ID())
	}
	return nil, fmt.Sprintf("the phrases stored beside %q could not be read, so it is "+
		"not known what it answers to", rt.Slug)
}

// AddAlias teaches one routine one more phrase.
//
// Under the route's cross-process lock, with the list re-read inside it, for the reason every
// other writer in this package takes it: a check and a write are two instants, and two windows
// adding a phrase at once would each write the list they read.
func (d Deps) AddAlias(rt routes.Route, phrase string) (Aliased, error) {
	phrase = strings.TrimSpace(phrase)
	switch {
	case rt.Slug == "":
		return Aliased{}, fmt.Errorf("a phrase needs a routine to belong to")
	case phrase == "":
		return Aliased{}, fmt.Errorf("say what %q should also answer to", rt.Slug)
	case routes.Slug(phrase) == "":
		// NOTHING TO MATCH ON. A phrase of punctuation folds to an empty slug,
		// which would match every request that also folds to nothing.
		return Aliased{}, fmt.Errorf("%q has no words in it, so nothing could ever "+
			"match it", phrase)
	case !d.Reg.Has(rt):
		return Aliased{}, fmt.Errorf("there is no routine saved at %s", rt.ID())
	}
	hold, err := d.Reg.Lock(rt, SaveWithin)
	if err != nil {
		return Aliased{}, fmt.Errorf("%q is in use: %w", rt.Slug, err)
	}
	defer hold.Release()

	have, why := d.AliasesOf(rt)
	if why != "" {
		return Aliased{}, fmt.Errorf("%s, so nothing was added", why)
	}
	// ITS OWN NAME ADDS NOTHING, and that is not an error. The slug already answers
	// before any phrase is consulted, so storing it would store a phrase that can never
	// be reached — said rather than written.
	if same(phrase, rt.Slug) {
		return d.aliased(rt, have, fmt.Sprintf("%q is already this routine's own "+
			"name, so there was nothing to add", phrase)), nil
	}
	for _, p := range have {
		if same(phrase, p) {
			return d.aliased(rt, have, fmt.Sprintf("%q already answers to %q.",
				rt.Slug, p)), nil
		}
	}
	if clash := d.whatElseAnswersTo(phrase, rt); clash != "" {
		return Aliased{}, fmt.Errorf("%q is already how %s is reached, and a phrase "+
			"two routines answer to selects neither of them — it asks which. "+
			"Choose another phrase, or rename one of them", phrase, clash)
	}
	if err := d.Reg.SaveAliases(rt, append(slices.Clone(have), phrase)); err != nil {
		return Aliased{}, fmt.Errorf("%q was not written down: %w", phrase, err)
	}
	return d.aliased(rt, append(slices.Clone(have), phrase),
		fmt.Sprintf("%q now also answers to %q.", rt.Slug, phrase)), nil
}

// RemoveAlias takes one phrase away from one routine, BY NAMING IT.
//
// A phrase it does not answer to is not an error: the end state asked for is the end state, and a
// surface whose list was one poll out of date would otherwise show somebody a failure for having
// removed something twice.
func (d Deps) RemoveAlias(rt routes.Route, phrase string) (Aliased, error) {
	phrase = strings.TrimSpace(phrase)
	switch {
	case rt.Slug == "":
		return Aliased{}, fmt.Errorf("a phrase needs a routine to belong to")
	case phrase == "":
		return Aliased{}, fmt.Errorf("say which phrase %q should stop answering to",
			rt.Slug)
	}
	hold, err := d.Reg.Lock(rt, SaveWithin)
	if err != nil {
		return Aliased{}, fmt.Errorf("%q is in use: %w", rt.Slug, err)
	}
	defer hold.Release()

	have, why := d.AliasesOf(rt)
	if why != "" {
		return Aliased{}, fmt.Errorf("%s, so nothing was removed", why)
	}
	kept := make([]string, 0, len(have))
	gone := ""
	for _, p := range have {
		if same(p, phrase) {
			gone = p
			continue
		}
		kept = append(kept, p)
	}
	if gone == "" {
		return d.aliased(rt, have, fmt.Sprintf("%q does not answer to %q, so there "+
			"was nothing to remove.", rt.Slug, phrase)), nil
	}
	if err := d.Reg.SaveAliases(rt, kept); err != nil {
		return Aliased{}, fmt.Errorf("%q was not removed: %w", gone, err)
	}
	return d.aliased(rt, kept, fmt.Sprintf("%q no longer answers to %q.",
		rt.Slug, gone)), nil
}

// aliased is the answer, read back from where the write put it.
//
// From DISK rather than from the list just written, so what a surface shows is what the next
// request will be read against — the same reason a repair reads its digests back.
func (d Deps) aliased(rt routes.Route, fallback []string, says string) Aliased {
	out := Aliased{ID: rt.ID(), Name: plays.Pretty(rt.Slug), Says: says,
		Answers: fallback}
	if back, why := d.AliasesOf(rt); why == "" {
		out.Answers = back
	}
	return out
}

// whatElseAnswersTo names the routine a phrase would collide with, or "" when none would.
//
// Every routine in the library and not only the eligible ones, because eligibility is about the
// moment a request is made and a collision is about the library: two routines in different scopes
// answering to one phrase is a question somebody gets asked the first time both are eligible, and
// refusing it at the add is the point.
//
// BOTH REDUCTIONS, for the reason at the top of this file.
func (d Deps) whatElseAnswersTo(phrase string, except routes.Route) string {
	for _, rt := range d.Reg.List() {
		if rt == except {
			continue
		}
		if same(phrase, rt.Slug) {
			return fmt.Sprintf("%q at %s", plays.Pretty(rt.Slug), rt.ID())
		}
		s, st := d.Reg.Aliases(rt)
		if !st.Offered() {
			continue
		}
		for _, p := range s.Phrases {
			if same(phrase, p) {
				return fmt.Sprintf("%q at %s, which already answers to %q",
					plays.Pretty(rt.Slug), rt.ID(), p)
			}
		}
	}
	return ""
}

// same is whether two phrases are one phrase, under EITHER normalization.
//
// `routes.Slug` is the door's and `nlu.Key` is the resolver's, and they disagree: a hyphen is a
// space to the second and "in" is a stop word, so `settings-in-ark` and `settings-ark` are two
// phrases at the door and one in the resolver. Two that either reduction cannot tell apart are
// one phrase here.
func same(a, b string) bool {
	if routes.Slug(a) == routes.Slug(b) {
		return true
	}
	ka, kb := nlu.Key(a), nlu.Key(b)
	return ka != "" && ka == kb
}
