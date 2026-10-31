package orchestrator

// WHAT A ROUTINE WILL TYPE, READ BACK BEFORE IT TYPES IT.
//
// A routine that types text somebody supplied is the first thing in this product where the
// invocation itself is worth reading back. `Authorize` asks only about a COMPOSED play — an
// authored or demonstrated routine is `Allowed` with `ReasonOrdinary` and nothing is put to
// anybody — and that is right for a routine whose every press was demonstrated. It is not enough
// for one whose text arrives at the moment of pressing: the text is the part nobody has seen
// before, and it is the part that can be wrong.
//
// So a routine with arguments gets a confirmation, and this is the thing being confirmed.
//
// # It is immutable, and that is the whole mechanism
//
// The values are COPIED in, the token digests them, and the confirmation is bound to three
// things: the route, the digest of the source as it then stood, and those values. A decision
// holding the caller's map would change when the caller did, and a digest taken at preparation
// would move with it — which is the defect
// [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] records about `Was`: WHAT WAS
// DISPLAYED and WHAT CURRENTLY EXISTS are two questions, and a stored reading compared against a
// mutable field only ever asks one.
//
// So `sealed` recomputes the token from whatever the struct NOW holds. The two are equal by
// construction and diverge exactly when something has changed what the confirmation says it
// showed.
//
// # It performs nothing
//
// What comes back is a routine, its values and a sentence. The thing that runs a routine is still
// `marco do` through the one door, with every guard it already has — see
// [[ADR-029-resolution-is-not-permission]], which this does not weaken: a confirmed invocation
// still crosses the authority door, and the values still reach `routes.CheckArgs` there.
//
// See [[Text-arguments-proposal-2026-10-30]].

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Invocation is a routine, the values it will run with, and what that will type.
type Invocation struct {
	// Token answers for this invocation and nothing else. Empty when it cannot be run.
	Token string
	Route routes.Route `json:"-"`
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	// Arguments is what the routine needs, in use order, with each one's kind — so a
	// surface can tell a value it must ask for from one the credential store answers.
	Arguments []routes.Argument `json:"arguments"`
	// Values is what will be typed, by argument name, and it is a COPY of what was
	// offered. Carried so the confirmation can show it and the run can be given it.
	Values map[string]string `json:"values,omitempty"`
	// From is how each carried value arrived, by argument name: typed, parsed, extracted
	// or edited.
	//
	// DERIVED HERE AND NEVER TAKEN ON TRUST. A surface says what was PROPOSED and what
	// will RUN, and this package compares them — so a value the person corrected cannot
	// be credited to the model by a surface that forgot to relabel its own field, which
	// is the one mislabelling that matters: it tells somebody a model read their sentence
	// correctly when what actually happened is that they fixed it.
	From map[string]string `json:"from,omitempty"`
	// Says is the confirmation, one line per argument, naming the argument and the text.
	//
	// THE ONE PLACE A VALUE IS REPEATED, and it is not a diagnostic: it is on a screen,
	// read by the person who typed it, and showing it is the whole point. Nothing else
	// this package produces carries a value — see Why.
	Says []string `json:"says,omitempty"`
	// Why is why this cannot be run as it stands: a value missing, a value this
	// substitution cannot write down, a routine that is not there.
	//
	// IT NAMES THE ARGUMENT AND NEVER ITS VALUE. A refusal is read in terminals and logs.
	Why string `json:"why,omitempty"`
	// SourceAt is a digest of the routine as it stood when this was prepared.
	//
	// EXPORTED BECAUSE IT HAS TO TRAVEL. A surface that confirmed this invocation is
	// about to start a child that reads the file AGAIN, and between those two moments a
	// save can land — so the version that was confirmed goes with the request and the
	// child compares it against the bytes it is about to run. Safe to carry because the
	// token seals it: a caller that altered it would fail `ConfirmInvocation`.
	SourceAt string `json:"source_at,omitempty"`
}

// Runnable is whether this invocation may be confirmed.
func (i Invocation) Runnable() bool { return i.Token != "" && i.Why == "" }

// Needs is whether the routine needs a value supplied at all.
//
// A routine with no text arguments is unchanged by any of this: Play presses it, and no
// confirmation is put to anybody about a routine whose every press was demonstrated.
func (i Invocation) Needs() bool {
	for _, a := range i.Arguments {
		if !a.Secret {
			return true
		}
	}
	return false
}

// PrepareInvocation reads a routine and says what running it with these values would type.
//
// Writes nothing and performs nothing. The values are copied; the token seals them with the
// source as it now stands.
func (d Deps) PrepareInvocation(rt routes.Route, values map[string]string) (Invocation, error) {
	return d.PrepareProposed(rt, values, nil, "")
}

// PrepareProposed is PrepareInvocation for values a REQUEST proposed, in whole or in part.
//
//	values     what will run, by argument name
//	proposed   what the request proposed, by argument name
//	proposedAt the digest of the routine those proposals were read from, or "" for none
//
// # The provenance is derived from the two maps
//
// A name in `proposed` whose value is unchanged is `extracted`; one that differs is `edited`,
// because the person corrected it and it is theirs now; a name that was never proposed is
// `typed`. A surface therefore cannot mislabel a value by forgetting to relabel a field it
// prefilled — it says what it offered and what came back, and the comparison is here.
//
// # And a proposal cannot outlive the revision it was made for
//
// The argument names a request was read against came from ONE revision of ONE routine. If the
// routine has been saved since, this REFUSES — **even when the new version declares the same
// names**, because the same name in a rewritten routine may be typed somewhere else entirely, and
// a value that crossed into a routine nobody proposed it for is text going to a place nobody
// proposed it for. Compared on the DIGEST and never on the names, which is the whole point: a
// comparison of names would pass exactly the case this exists to refuse.
//
// Asked BEFORE `CheckArgs`, because a proposal for an old revision must be reported as a routine
// that was saved again rather than as an argument nobody declared — the names may well have
// changed, and that refusal would send somebody looking for a typo.
func (d Deps) PrepareProposed(rt routes.Route, values, proposed map[string]string,
	proposedAt string) (Invocation, error) {

	if rt.Slug == "" {
		return Invocation{}, fmt.Errorf("an invocation needs a routine")
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return Invocation{}, fmt.Errorf("%q could not be read: %w",
			plays.Pretty(rt.Slug), err)
	}
	src := string(raw)
	i := Invocation{Route: rt, ID: rt.ID(), Name: NameIn(src, rt.Slug),
		Arguments: routes.Arguments(src), SourceAt: routes.DigestOf(src)}
	// ── WHETHER IT MAY RUN, ASKED OF THE NAMES AS THEY WERE SUPPLIED ──
	//
	// BEFORE ANY FILTERING, and the first version had it after. It filtered the values to
	// the names this routine takes and then asked `CheckArgs` about the FILTERED map — so
	// the unknown-name arm could never fire from here: `message` plus a misspelt `mesage`
	// dropped the second and ran happily with the first. A value somebody typed and a
	// routine that silently ignores it is the whole defect this check exists to prevent,
	// and the filter was doing the ignoring before the check could see it.
	//
	// Asked here so a surface is not offering a press the door would decline, and asked
	// again THERE because this is a review and that is the authority. Neither is the
	// other's substitute: the whole of ADR-273's surface lesson is that a dead button
	// proves nothing about a backend that would have obliged.
	if proposedAt != "" && i.SourceAt != proposedAt {
		i.Why = fmt.Sprintf("%q has been saved again since those values were read "+
			"out of what you said, so nothing has been proposed for what it now "+
			"does — say it again", i.Name)
		return i, nil
	}
	if why := routes.CheckArgs(src, values, nil); why != nil {
		i.Why = why.Error()
		return i, nil
	}
	// AND ONLY NOW ARE THEY CARRIED, TEXT ONLY.
	//
	// Nothing is dropped at this point — every supplied name passed the check above — so
	// this narrows the list to what will be TYPED. A secret-typed value is not carried,
	// because it would then be in the token, in the confirmation and on a command line,
	// and the credential store is where it comes from.
	wanted := map[string]bool{}
	for _, a := range i.Arguments {
		if !a.Secret {
			wanted[strings.ToLower(a.Name)] = true
		}
	}
	for k, v := range values {
		if wanted[strings.ToLower(k)] {
			if i.Values == nil {
				i.Values = map[string]string{}
			}
			name := strings.ToLower(k)
			i.Values[name] = v
			if i.From == nil {
				i.From = map[string]string{}
			}
			i.From[name] = howItArrived(name, v, proposed)
		}
	}
	i.Says = whatItWillType(i.Arguments, i.Values, i.From)
	return i.sealed(), nil
}

// howItArrived compares what will run with what was proposed.
//
// NOT A CLAIM A SURFACE MAKES. The three answers are the three things that can have happened to a
// prefilled box: nobody proposed anything for it, the proposal was left alone, or the person
// changed it. A changed proposal is the person's own value and saying otherwise would tell them a
// model read their sentence correctly when they are the one who fixed it.
//
// `routes.FromParsed` is not produced here: that is the deterministic grammar reading a phrase, one
// door along, and a value that reached this function had already been read by whatever read it.
func howItArrived(name, value string, proposed map[string]string) string {
	was, ok := proposed[name]
	switch {
	case !ok:
		return routes.FromTyped
	case was == value:
		return routes.FromExtracted
	default:
		return routes.FromEdited
	}
}

// ConfirmInvocation says why a confirmed invocation may no longer be run, or "" when it may.
//
// Three things are re-asked: that this is the invocation that was prepared (the token, recomputed
// from what the struct now holds), that the routine has not been written since, and that it still
// may run with these values. A surface holds the Invocation between the two calls, so all three
// are things that can have changed under it.
func (d Deps) ConfirmInvocation(i Invocation) string {
	switch {
	case i.Token == "":
		return "this invocation was never prepared, so there is nothing to confirm"
	case i.sealed().Token != i.Token:
		// WHAT IT SAYS IT SHOWED IS NOT WHAT IT NOW HOLDS. A value changed after the
		// confirmation was read is the case this exists for.
		return fmt.Sprintf("what %q would type is no longer what the confirmation "+
			"showed, so nothing has been run — look at it again", i.Name)
	}
	raw, err := os.ReadFile(d.Reg.Path(i.Route))
	if err != nil {
		return fmt.Sprintf("%q is no longer where this library keeps it, so nothing "+
			"has been run", i.Name)
	}
	src := string(raw)
	if routes.DigestOf(src) != i.SourceAt {
		return fmt.Sprintf("%q has been saved again since this was prepared, so what "+
			"it would do is not what the confirmation showed — look at it again",
			i.Name)
	}
	if why := routes.CheckArgs(src, i.Values, nil); why != nil {
		return fmt.Sprintf("%q: %v", i.Name, why)
	}
	return ""
}

// sealed recomputes the token from everything this invocation is about.
//
// THE VALUES THEMSELVES, not a count and not a stored digest of them: the struct is handed to a
// surface and handed back, and the one thing that must be caught is somebody changing what it
// says it showed. Sorted, because a map's order is not a fact about the invocation.
func (i Invocation) sealed() Invocation {
	keys := make([]string, 0, len(i.Values))
	for k := range i.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{i.ID, i.SourceAt}
	for _, k := range keys {
		// THE NAME, THE VALUE AND WHERE IT CAME FROM, LENGTH-DELIMITED. Joining them
		// plainly would let one pair of values digest the same as another — `a=bc`
		// and `ab=c` — which is a collision somebody could choose.
		//
		// The PROVENANCE is in here because a label changed after the confirmation
		// was read is the same class of thing as a value changed after it: what the
		// struct says it showed is not what it now holds. And it matters more than
		// it looks, because that label is the only thing telling somebody whether a
		// model or they themselves produced the text they are about to send.
		parts = append(parts, fmt.Sprintf("%d:%s=%d:%s@%d:%s", len(k), k,
			len(i.Values[k]), i.Values[k], len(i.From[k]), i.From[k]))
	}
	i.Token = routes.DigestOf(strings.Join(parts, "\x00"))
	return i
}

// whatItWillType is the confirmation: one line per argument, naming it and the text.
//
// Every argument, including the ones the credential store answers, because a person deciding
// whether to press is owed the whole list rather than the half that happens to be typed here.
func whatItWillType(args []routes.Argument, values, from map[string]string) []string {
	out := []string{}
	for _, a := range args {
		if a.Secret {
			out = append(out, fmt.Sprintf("%s — from the credential store", a.Name))
			continue
		}
		name := strings.ToLower(a.Name)
		v := values[name]
		if v == "" {
			// AN EMPTY VALUE SOMEBODY GAVE ON PURPOSE, said as what it is rather
			// than as a blank nobody can tell from a mistake.
			out = append(out, fmt.Sprintf("%s — nothing (it will type no text)%s",
				a.Name, saidWhere(from[name])))
			continue
		}
		out = append(out, fmt.Sprintf("%s — %q%s", a.Name, v, saidWhere(from[name])))
	}
	return out
}

// saidWhere is the clause naming where a value came from, or "" for one somebody typed.
//
// A VALUE SOMEBODY TYPED GETS NO CLAUSE, because they are reading their own text back and a line
// telling them they typed it is noise in the one place that must be easy to check. The two that
// DO get one are the two a person could otherwise mistake for their own: a value read out of
// their sentence by a model, and the same value after they corrected it.
func saidWhere(from string) string {
	switch from {
	case routes.FromExtracted:
		return " (read out of what you said)"
	case routes.FromEdited:
		return " (as you corrected it)"
	case routes.FromParsed:
		return " (read out of the words you used)"
	default:
		return ""
	}
}
