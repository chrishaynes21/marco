package main

// THE AUTHORISATION A PERSON GIVES BY PRESSING PLAY, ONCE.
//
// # What was wrong with the thing this replaces
//
// Three questions per press, and a person had already answered all three. They composed the Act
// out of interactions they picked and named, they chose which Act to run, and they pressed Play.
// Then the run asked them to say they were ready, asked which saved screen they were looking at
// — reading out an arrangement score they have no way to check — and then, per interaction,
// asked whether they agreed to the step and whether Marco might press it. The owner's verdict on
// operating that: it is an engineering acceptance checklist handed over as a product.
//
// The ceremony was never the safety. Every guard that makes a supervised press safe is in the
// runner and asks nobody anything: the occlusion refusal on each look, the world re-check at the
// instant of the press, the pointer-arrival measurement after the dwell, the outcome gate between
// interactions, the stop signal read at the last moment it could matter, the deadline and the
// action cap. Those all stay. What goes is the keypress ceremony in front of them.
//
// # What a grant is, and what it is not
//
// It is a record of one person's decision, made at one instant, over named interactions:
//
//	the Act, by the name they gave it
//	the revision of the demonstration its interactions came from
//	the application it is pinned to
//	the interactions it performs, by the names the person gave THEM
//	who gave it, and when
//
// It is NOT a simulated keypress. Nothing here presses F7 on somebody's behalf, and nothing here
// touches `navsource`: human-input provenance is unchanged, and a run performed under a grant
// says in its record that it was performed under a grant rather than that somebody answered a
// question. That distinction is the whole reason this is a type and not an `--i-agree` flag.
//
// It is also NOT a standing permission. ADR-183 refuses a session flag, and this is narrower
// than one in four ways at once: one Act, one revision, one application, one run. The file is
// written by whoever starts the run, read once, and spent on exactly the interactions it names —
// an attempt to press anything else is refused by `grantedGranter.Authorise`, which is the term
// that makes "clicking Play authorises the selected Act" a claim with an edge.
//
// # Why a file
//
// Because the grant has to travel from the surface a person pressed to the process that performs
// the work, and it has to be auditable afterwards. A composite flag string would be the same
// information with nowhere to put the next field and no shape for a reviewer to read. The run
// record keeps it, so "what authorised this press" is answerable from the reference alone.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// actGrantVersion is the shape of the file, so an older surface cannot be read as a newer grant.
const actGrantVersion = 1

// actGrant is one person's decision to perform one Act, once.
type actGrant struct {
	Version int    `json:"version"`
	Act     string `json:"act"`
	// Revision is the demonstration revision the Act's bindings are pinned to, as the
	// surface read it when it offered the Act.
	//
	// Checked against the Act on disk, because a grant is given for the thing the person
	// was shown. Re-composing an Act between the press and the run would otherwise spend
	// their decision on interactions they never saw.
	Revision string `json:"revision,omitempty"`
	// Application is the executable the Act is pinned to, and the only one this licenses.
	Application string `json:"application,omitempty"`
	// Interactions are the names the person gave the things they picked, in order. The
	// grant covers these and nothing else.
	Interactions []string `json:"interactions"`
	// Source is the digest of the play the Act's bindings were written with.
	Source string `json:"source,omitempty"`
	// Bound is WHAT EACH NAME MEANS: the demonstrated interaction each one resolves to.
	//
	// # Why the names and the revision are not enough
	//
	// Because an Act can be composed again over the SAME demonstration, with the SAME
	// names, in the SAME order, bound to DIFFERENT positions. Every other term here is
	// unchanged by that — the revision is the demonstration's and the demonstration did
	// not move, the names and their order are what the person typed, and the generated
	// play is byte-identical because its sentences are those names. What changed is the
	// only thing that decides which control gets pressed.
	//
	// So the grant pins the mapping. A person authorised "the graphics tab" meaning
	// interaction 0 of that demonstration; if it now means interaction 1, their decision
	// was about something else and the run is refused before anything is observed.
	Bound []grantBound `json:"bound,omitempty"`
	// By is who gave it, in words, and At the instant they did.
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// grantBound is one name and the demonstrated interaction it was authorised to mean.
type grantBound struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// readActGrant reads a grant and refuses one that does not say enough to be a decision.
//
// Every refusal is fatal to the run rather than a reason to fall back to asking: a surface that
// meant to hand over an authorisation and handed over something unreadable has lost the person's
// decision, and performing anyway under a guess about what they meant is the one thing this type
// exists to prevent.
func readActGrant(path string) (actGrant, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return actGrant{}, fmt.Errorf("the authorisation could not be read: %w", err)
	}
	var g actGrant
	if err := json.Unmarshal(raw, &g); err != nil {
		return actGrant{}, fmt.Errorf("the authorisation is not readable: %w", err)
	}
	switch {
	case g.Version != actGrantVersion:
		return actGrant{}, fmt.Errorf("the authorisation is version %d and this "+
			"version of Marco performs version %d", g.Version, actGrantVersion)
	case strings.TrimSpace(g.Act) == "":
		return actGrant{}, fmt.Errorf("the authorisation names no Act, so there is " +
			"nothing it authorises")
	case len(g.Interactions) == 0:
		return actGrant{}, fmt.Errorf("the authorisation names no interactions, so " +
			"there is nothing it authorises")
	case strings.TrimSpace(g.By) == "":
		return actGrant{}, fmt.Errorf("the authorisation does not say who gave it")
	case g.At.IsZero():
		return actGrant{}, fmt.Errorf("the authorisation does not say when it was given")
	// ── AND IT HAS TO PIN WHICH ACT, NOT JUST WHICH NAMES ──
	//
	// Required rather than optional, because an optional check is a check that can be
	// skipped by leaving a field out. These three are the Act's identity: the
	// demonstration its interactions came from, the play they were bound to, and what
	// each name MEANS. Without the last one, the same names over the same demonstration
	// bound to different interactions are indistinguishable from the thing somebody
	// authorised — see `Bound`.
	case strings.TrimSpace(g.Revision) == "":
		return actGrant{}, fmt.Errorf("the authorisation names no demonstration " +
			"revision, so it does not say which version of the Act was authorised")
	case strings.TrimSpace(g.Source) == "":
		return actGrant{}, fmt.Errorf("the authorisation names no play digest, so " +
			"it does not say which version of the Act was authorised")
	case len(g.Bound) != len(g.Interactions):
		return actGrant{}, fmt.Errorf("the authorisation names %d interaction(s) and "+
			"says what %d of them mean; it cannot authorise a name whose "+
			"interaction it does not pin", len(g.Interactions), len(g.Bound))
	}
	// AND THE TWO LISTS ARE THE SAME LIST, in the same order. Two lists of one fact is
	// the shape this repository has a memory note about: they agree when they are
	// written and disagree the first time one is changed.
	for i, name := range g.Interactions {
		if g.Bound[i].Name != name {
			return actGrant{}, fmt.Errorf("the authorisation performs %q as "+
				"interaction %d and says what %q means there; one of the two "+
				"lists is wrong and nothing here can say which",
				name, i+1, g.Bound[i].Name)
		}
	}
	return g, nil
}

// agrees asks whether this grant is for the Act that is about to be performed.
//
// The name is not enough. An Act is a play plus bindings pinned to a demonstration revision, and
// all three can change between the instant somebody pressed Play and the instant a run loads
// them. What the person authorised is the thing they were shown.
// Checked against the PREFLIGHTED plan rather than against the files, because the plan is what
// will be performed: preflight has already established that the play is the one the bindings
// resolve to, pinned to a revision the routine still has, and that every position is real.
func (g actGrant) agrees(play string, p demonstrated.Plan) error {
	if !strings.EqualFold(routes.Slug(g.Act), routes.Slug(play)) {
		return fmt.Errorf("the authorisation is for %q and this run performs %q",
			g.Act, play)
	}
	if g.Revision != p.Revision {
		return fmt.Errorf("the authorisation was given for revision %s of the "+
			"demonstration and %q now performs revision %s — it has been composed "+
			"again since, so this would authorise interactions that may not be "+
			"the ones somebody chose", g.Revision, play, p.Revision)
	}
	if g.Application != "" && p.Routine.Application != "" &&
		g.Application != string(p.Routine.Application) {
		return fmt.Errorf("the authorisation is for %s and %q is pinned to %s",
			g.Application, play, p.Routine.Application)
	}
	// ── AND IT IS THE SAME INTERACTIONS, IN THE SAME ORDER ──
	//
	// Order and not just membership. A grant is given over a sequence somebody read — "the
	// graphics tab, then apply changes" — and the same two interactions the other way
	// round are a different thing to have agreed to: the second press is made on the
	// screen the first one produced.
	if len(g.Interactions) != len(p.Order) {
		return fmt.Errorf("the authorisation names %d interaction(s) and %q performs "+
			"%d", len(g.Interactions), play, len(p.Order))
	}
	for i, want := range g.Interactions {
		if p.Order[i] != want {
			return fmt.Errorf("the authorisation names %q as interaction %d and %q "+
				"performs %q there, so the order somebody authorised is not "+
				"the order this would perform", want, i+1, play, p.Order[i])
		}
	}
	// ── AND THE SAME PLAY, BY ITS OWN DIGEST ──
	//
	// The bindings record which play they were written with, and preflight has already
	// refused a play that does not hash to it. This is the other end of that: the
	// authorisation records which one the person was shown.
	if g.Source != p.Source {
		return fmt.Errorf("the authorisation was given for a different version of "+
			"%q (%s, now %s) — it has been composed again since", play,
			digestHead(g.Source), digestHead(p.Source))
	}
	// ── AND EVERY NAME STILL MEANS THE INTERACTION IT MEANT ──
	//
	// The term the other four cannot carry: recomposing the same Act over the same
	// demonstration with the same names in the same order, bound to different positions,
	// leaves the revision, the names, the order and the play's own digest all unchanged
	// and changes every control that gets pressed.
	//
	// Checked only when the grant carries the mapping, so a grant written by an older
	// surface is refused by the four terms above rather than by a field it never had —
	// and `Studio` has always written it. A grant with no mapping and a play that needs
	// one is the case `readActGrant` cannot see and this cannot either; what closes it is
	// that nothing else writes grants.
	for _, b := range g.Bound {
		at, ok := p.Steps[b.Name]
		if !ok {
			return fmt.Errorf("the authorisation names the interaction %q and %q "+
				"does not perform it", b.Name, play)
		}
		if at != b.Position {
			return fmt.Errorf("the authorisation was given for %q meaning "+
				"interaction %d of the demonstration, and in %q it now means "+
				"interaction %d — it has been composed again since, so this "+
				"would press something nobody authorised",
				b.Name, b.Position, play, at)
		}
	}
	return nil
}

// digestHead is as much of a digest as a person can compare by eye, for a sentence.
func digestHead(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

// covers asks whether one interaction is one of the ones this grant was given for.
func (g actGrant) covers(name string) bool {
	for _, n := range g.Interactions {
		if n == name {
			return true
		}
	}
	return false
}

// describe is the grant in one sentence, for a transcript, a binding and a record.
//
// It says WHAT the authority was rather than naming a key, because that is the difference the
// audit trail has to keep: "the person who pressed Play" and "the person at the keyboard (F7)"
// are two different claims about how a press came to be allowed.
func (g actGrant) describe() string {
	return fmt.Sprintf("%s at %s, authorising %s", g.By,
		g.At.Local().Format("15:04:05"), g.names())
}

// names is the interactions in the person's own words, for a sentence.
func (g actGrant) names() string {
	quoted := make([]string, 0, len(g.Interactions))
	for _, n := range g.Interactions {
		quoted = append(quoted, fmt.Sprintf("%q", n))
	}
	switch len(quoted) {
	case 1:
		return quoted[0]
	case 2:
		return quoted[0] + " and " + quoted[1]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
}

// ── the granter ───────────────────────────────────────────────────────────────

// grantedGranter authorises a press from a decision already made, and refuses anything else.
//
// It is a `workflowrun.Granter` and nothing else — the same interface the keyboard desk
// implements, so the runner cannot tell them apart and every check it makes on an authorisation
// is unchanged. It cannot see a screen, cannot press anything, and what it returns is still
// checked by `Spendable` against the step, the window and the observation before it buys
// anything.
//
// There is no desk here and no key source: nothing waits, because nobody is being asked.
type grantedGranter struct {
	grant actGrant
	out   io.Writer
	live  bool
	// life is how long the authorisation it returns lasts. As short as the desk's, and
	// for the same reason: the press happens in the same instant the grant is returned,
	// and one that outlived the moment would be the session flag ADR-183 refuses — which
	// a grant given in advance makes it MORE important to keep short, not less.
	life time.Duration
	now  func() time.Time
	// events is the structured account, and scenes is which interaction a step is. Both
	// as the desk holds them; see interactionidentity.go for why a step is not an
	// identity.
	events *runstate.Writer
	scenes *performedInteractions
	// granted and refused are presses this authorised and presses it would not. The
	// second number is the one worth having: a run that refused a press under a grant
	// tried to do something the person did not authorise, and the report must not be
	// silent about it.
	granted  int
	refused  int
	outside  []string
	proposed int
}

func (g *grantedGranter) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

// Authorise permits one press, if it is one of the presses the grant was given for.
func (g *grantedGranter) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	what := "write down"
	if g.live {
		what = "SEND"
	}
	// WHICH INTERACTION THIS IS, asked of the thing that selected it. A step is not an
	// identity: for an Act composed by picking it is neither the position the person is
	// watching nor the name they gave it.
	at := g.scenes.at(in.Step, "")
	// ── THE SCOPE CHECK, WHICH IS WHAT MAKES THE GRANT NARROW ──
	//
	// An aim that names nothing — index 0 — is a press the play did not select, and a
	// name the grant does not carry is a press the person did not authorise. Either way
	// nothing is permitted, and the sentence says which it was: this is the one refusal
	// in the product that means Marco tried to do something nobody agreed to.
	if at == nil || at.Index == 0 || !g.grant.covers(at.Name) {
		g.refused++
		named := "an interaction this Act does not name"
		if at != nil && at.Name != "" {
			named = fmt.Sprintf("%q", at.Name)
		}
		g.outside = append(g.outside, named)
		fmt.Fprintf(g.out, "\n  REFUSED: the run asked to press %s, and the "+
			"authorisation given was for %s. Nothing was sent.\n",
			named, g.grant.names())
		return workflowrun.Authorisation{}, false
	}

	g.granted++
	fmt.Fprintf(g.out, "\n  %d of %d · %s\n", at.Index, at.Of, at.Name)
	if in.Because != "" {
		fmt.Fprintf(g.out, "    found: %s\n", in.Because)
	}
	fmt.Fprintf(g.out, "    target: %s\n", in.Target(1920, 1080))
	fmt.Fprintf(g.out, "    Marco will %s a left click there, once, under the "+
		"authorisation of %s.\n", what, g.grant.describe())
	// SAID AS A PRESS GOING OUT, not as a question. There is no question: a front end
	// renders this as progress, and the phase is the same `acting` the desk emits once
	// somebody has answered.
	g.events.Say(runstate.Event{Phase: runstate.Acting, Interaction: at})
	return workflowrun.Authorisation{
		Step: in.Step,
		By:   g.grant.describe(),
		// THE OBSERVATION AND THE WINDOW THIS WAS ASKED ABOUT, copied from the
		// intent exactly as the desk copies them. Copying is not evidence that the
		// world stood still — the runner re-observes it — and it is what makes an
		// authorisation from an earlier screen or another window unspendable. A
		// grant given before the run does not reach past this: it still cannot
		// authorise a press in an observation it was not made in.
		Epoch: in.Epoch, Generation: in.Generation, Window: in.Window,
		Expires: g.clock().Add(g.life),
	}, true
}

// agreed records that the grant covers this step, in place of the agreement question.
//
// The step-agreement question asked whether a step belongs in the run at all. For an Act that is
// what composing it answered: the person picked these interactions, named them, and pressed
// Play. So the confirmation is recorded with the grant as its authority rather than put to
// somebody a second time — and it is recorded, rather than skipped, because the session's own
// account of why each step was performed is what the report and the record read.
func (g *grantedGranter) agreed(at int, s stepConfirmer) bool {
	g.proposed++
	return s.ConfirmStep(at, g.grant.describe()) == nil
}

// stepConfirmer is the session's step-confirmation half, named so this file does not need the
// whole binding to say what it uses of it.
type stepConfirmer interface {
	ConfirmStep(at int, by string) error
}
