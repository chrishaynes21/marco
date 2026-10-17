package directorapi

import (
	"fmt"
	"time"
)

// ResolutionStatus is the outcome of trying to find what the user meant.
//
// The four values exist because live inspection showed a two-value answer
// (found / not found) to be dangerous. Searching a Discord window for a button
// returns nothing, and searching a fully-readable dialog for a button that genuinely
// is not there also returns nothing — but the first means "I cannot see into this
// application" and the second means "it does not exist". Reporting them the same way
// makes the Director confidently wrong about the first, and it makes the correct
// recovery (reach for another source, or open the menu it is hidden in) impossible
// to choose.
type ResolutionStatus string

const (
	// ResolutionResolved is one clear winner. Acting is appropriate.
	ResolutionResolved ResolutionStatus = "resolved"

	// ResolutionAmbiguous is several viable candidates with no clear best. The
	// Director should ASK. Picking the top of a near-tie is how an agent clicks the
	// wrong "Apply" and reports success.
	ResolutionAmbiguous ResolutionStatus = "ambiguous"

	// ResolutionUnobservable means the question could not be answered, because the
	// world is too poor to answer it: the application exposed only containers, a
	// source that was needed did not report, or the snapshot was truncated.
	//
	// Crucially it is NOT evidence of absence. "I could not find Save in Discord" is
	// a statement about the Director, not about Discord.
	ResolutionUnobservable ResolutionStatus = "unobservable"

	// ResolutionAbsent means the world was observed well enough to answer, and the
	// target is not in it. This is a real finding, and it is what licenses a
	// discovery attempt or an honest "there is no Save button here".
	ResolutionAbsent ResolutionStatus = "absent"
)

// Actionable reports whether this outcome permits proceeding without asking.
func (s ResolutionStatus) Actionable() bool { return s == ResolutionResolved }

// Resolution is the full result of resolving one reference, including why.
//
// It always carries its reasoning. A target the Director cannot justify is a target
// it should not act on, and after the fact the reasoning is the only thing that
// makes a wrong choice diagnosable.
type Resolution struct {
	Status ResolutionStatus `json:"status"`

	// Target is the chosen element, set only when Status is resolved.
	Target *ResolvedTarget `json:"target,omitempty"`

	// Candidates are everything considered, best first — including the rejected, with
	// the reason for rejection.
	Candidates []TargetCandidate `json:"candidates,omitempty"`

	// Contenders is the AUTHORITATIVE ORDERED SET of choices to offer the user, set
	// when Status is ambiguous. Position 1 in this slice is what "the first one"
	// means, position 2 what "the second one" means, and so on.
	//
	// A slice rather than a count, and the difference is not cosmetic. It was a count,
	// alongside Candidates holding EVERY candidate including the rejected ones — which
	// left every consumer to re-derive "which of these may I offer?" from Rejected and
	// Score. Two definitions of viable candidate, in two packages, which is a contract
	// waiting to drift. It did: the resolver could report AMBIGUOUS while the
	// clarification layer found nothing it was willing to offer, and the user got an
	// empty question.
	//
	// So there is one definition now and it lives here. Consumers offer exactly this
	// list, in this order, and never filter it again. Candidates remains the full
	// contest for diagnostics — the losers are the explanation, not the choice.
	Contenders []TargetCandidate `json:"contenders,omitempty"`

	// Explanation is one sentence a person can read.
	Explanation string `json:"explanation,omitempty"`

	// Discovery is a bounded plan to look somewhere not currently visible — a menu
	// that must be opened before its items exist. Offered when the target is absent
	// from the observed world but the world contains unopened places it could be.
	Discovery *DiscoveryPlan `json:"discovery,omitempty"`

	// Blocker names what made the answer unobservable, for the explanation log:
	// "coverage", "actionability", "degraded_source", "stale".
	Blocker string `json:"blocker,omitempty"`

	// Reworded says this answer was reached by asking the request again in words the
	// screen uses, and what was set aside to do it — "setting aside \"the\" and
	// \"panel\"", "reading personalisation→personalization from the words on screen".
	//
	// Empty for every answer reached by the words the person actually used, which is
	// almost all of them: rewording is reached only where the literal reading found
	// nothing at all.
	//
	// It exists so a reworded answer is VISIBLY reworded. An answer that set a word aside
	// is a weaker claim than one that matched, and a person reading a wrong answer needs
	// to be able to see which step was wrong — the words set aside, the spelling repaired,
	// or the kind of control inferred from their own noun. A confidence alone cannot say
	// that, and a reworded answer that read like an exact match would be the resolver
	// overstating what it knows.
	//
	// Set on AMBIGUOUS as well as on RESOLVED, because a question raised about a reworded
	// reading has to be able to say so: "I can see two things that might be what you
	// called the Apps panel" is a different question from "I can see two things called
	// Apps".
	Reworded string `json:"reworded,omitempty"`
}

// DiscoveryPlan is a bounded search for something that is not on screen yet.
//
// Most desktop targets do not exist as elements until something is opened: Notepad
// exposes File, Edit and View but no Save, because Save only comes into being once
// the File menu is open. Treating that as "absent" makes a large share of ordinary
// requests impossible.
//
// Every part of this is bounded on purpose. Discovery means acting on the UI in
// order to look at it, which is the point where an agent can start wandering. So the
// probes are enumerated up front rather than generated as it goes, the count is
// capped, each one is individually reversible, and Cleanup restores the UI whether
// or not the target was found.
type DiscoveryPlan struct {
	// Reason explains why discovery is being proposed.
	Reason string `json:"reason"`
	// Query is what is being looked for.
	Query *ElementQuery `json:"query,omitempty"`
	// Probes are the places to look, best first, already truncated to MaxProbes.
	Probes []DiscoveryProbe `json:"probes"`
	// MaxProbes is the ceiling that was applied.
	MaxProbes int `json:"max_probes"`
	// Risk is the risk of the probing itself. Opening a menu is low risk and
	// reversible; that is precisely why menus are the only thing probed by default.
	Risk RiskLevel `json:"risk"`
}

// DiscoveryProbe is one place to look.
type DiscoveryProbe struct {
	// Container is the element to open.
	Container ElementID `json:"container"`
	// Label is its name, for the explanation ("open the File menu").
	Label string `json:"label"`
	// Open is the action that reveals its contents.
	Open Action `json:"open"`
	// Cleanup returns the UI to how it was, run whether or not the probe succeeded.
	// Without it, a failed discovery leaves a menu hanging open over the application
	// and every subsequent observation describes the menu instead of the window.
	Cleanup Action `json:"cleanup"`
	// Score is how likely this container is to hold the target.
	Score float64 `json:"score"`
}

// Empty reports whether there is nothing to try.
func (p *DiscoveryPlan) Empty() bool { return p == nil || len(p.Probes) == 0 }

// ContenderCount is how many choices are on offer.
func (r Resolution) ContenderCount() int { return len(r.Contenders) }

// MinContenders is the fewest choices an ambiguous resolution may carry.
//
// Two, because "ambiguous" with one choice is not ambiguous — it is resolved — and
// with none it is absent. Either would be a lie to the user in a different direction.
const MinContenders = 2

// Consistent reports whether a Resolution keeps the promise its Status makes.
//
//	AMBIGUOUS     => at least two ordered contenders to offer
//	RESOLVED      => exactly one selected target
//	ABSENT        => no contenders (nothing to choose between)
//	UNOBSERVABLE  => no claim about absence is made
//
// AMBIGUOUS is not merely a score verdict. It is a PROMISE that the Director has at
// least two concrete, safe, ordered choices it can put in front of a person. A
// resolution that cannot keep that promise is an internal inconsistency, and saying so
// is far more useful than silently rendering an empty question.
func (r Resolution) Consistent() error {
	switch r.Status {
	case ResolutionAmbiguous:
		if len(r.Contenders) < MinContenders {
			return fmt.Errorf(
				"AMBIGUOUS resolution produced %d clarification contenders, and at least %d are required",
				len(r.Contenders), MinContenders)
		}
	case ResolutionResolved:
		if r.Target == nil {
			return fmt.Errorf("RESOLVED resolution has no target")
		}
	case ResolutionAbsent:
		if len(r.Contenders) > 0 {
			return fmt.Errorf(
				"ABSENT resolution carries %d contenders, so something was offerable after all",
				len(r.Contenders))
		}
	}
	return nil
}

// GroundedIn reports whether this resolution established a SPECIFIC target in THIS world, and
// says what was missing when it did not.
//
// # The question it exists to answer, and the one it must not be read as answering
//
// Policy's world-level gates ask "could this Stage answer a question like the one being asked".
// The resolver asks "which Prop did this request mean, and on what evidence", and it already
// states the principle the gates contradict:
//
//	A match found despite a poor world is still a match: the world being weak overall does not
//	unfind a control we can plainly see and use.
//
// So a grounded target settles the WORLD-LEVEL evidence question for this one action, and
// settles nothing else. It does not establish that the action is actionable, authorized,
// executable or safe. Risk, destructiveness, staleness, confirmation, the capability ladder's
// terminal refusal and an application's own rules all still apply, are all evaluated elsewhere,
// and none of them consults this. See [[Experiment-055-what-coverage-and-blind-actually-measure]].
//
// # Why a non-nil target is not grounding
//
// Every clause below is a way a target pointer can exist and mean nothing. An AMBIGUOUS
// resolution carries contenders and no choice; a resolution can name an element that is not in
// the world being judged, because it was resolved against an earlier snapshot; a zero-confidence
// match is not a match; and a target in a stale world is a target that may have moved. Reading
// `Target != nil` as grounding would turn each of those into a licence to skip a gate.
func (r *Resolution) GroundedIn(w *WorldState, now time.Time) (bool, string) {
	switch {
	case r == nil:
		return false, "nothing was resolved for this request"
	case r.Status != ResolutionResolved:
		return false, "the request resolved to " + string(r.Status) + ", not to one target"
	case r.Consistent() != nil:
		// The type's own promise-keeping check: RESOLVED means exactly one target.
		return false, "the resolution does not keep the promise its status makes"
	case r.Target == nil:
		return false, "the resolution carries no target"
	case r.Target.Confidence <= 0:
		return false, "the target was identified with no confidence at all"
	case w == nil:
		return false, "there is no world to have grounded it in"
	}

	// AND IT IS IN THE WORLD BEING JUDGED, with somewhere real to aim.
	//
	// Geometry only — whether anything may OPERATE it is a different question, asked at
	// grounding by `Element.OperableUnder` and never here. A gate that asked it twice would
	// be a second permission decision in the place that must not make one.
	if id := r.Target.ElementID; id != "" {
		el, ok := w.Element(id)
		if !ok {
			return false, "the target is not in this reading of the screen"
		}
		acts := el.Actions()
		if !acts.ClickableByBounds {
			return false, "the target has no on-screen rectangle in this reading"
		}
		// AND THE SURFACE, when the press belongs somewhere other than this element's
		// own middle. Geometry again, and the same question asked of the same world: a
		// resolution naming a surface that is not in this reading has nowhere to aim,
		// and a grounded target with nowhere to aim is exactly what this gate is for.
		if s := r.Target.Surface; s != "" && s != id {
			sel, ok := w.Element(s)
			if !ok {
				return false, "the surface the press was aimed at is not in this " +
					"reading of the screen"
			}
			if !sel.Actions().ClickableByBounds {
				return false, "the surface the press was aimed at has no " +
					"on-screen rectangle in this reading"
			}
		}
		if !acts.Enabled {
			// The resolver requires this too, and keeps a disabled match as a
			// candidate so "Save is greyed out" can be said. Such a match is a real
			// finding and not a reason to stop asking whether this Stage could
			// answer the question at all.
			return false, "the target will not respond: it is disabled"
		}
	} else if id := r.Target.WindowID; id != "" {
		if _, ok := w.Window(id); !ok {
			return false, "the target window is not in this reading of the screen"
		}
	} else {
		return false, "the target names neither an element nor a window"
	}

	// AND THE READING STILL DESCRIBES THE PRESENT. Belt and braces: policy's own staleness
	// gate is deliberately NOT conditional on grounding, and this makes it impossible for a
	// grounded target to be the thing that carries a stale world past any gate.
	if w.ConfidenceAt(now).Stale() {
		return false, "this reading of the screen is out of date"
	}
	return true, ""
}
