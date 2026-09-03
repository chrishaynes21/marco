package observe

import "strings"

// The THEATER: the durable semantic world Marco performs in.
//
// # What this file is, and what it deliberately is not
//
// It is a SEMANTIC BOUNDARY, not a second store. `semanticmemory.Store` remains the storage
// engine underneath — one file, one match-or-append, one subject bound — and every operation here
// delegates to it. What this adds is a vocabulary: consumers ask for meaning rather than
// assembling subject records, so a caller cannot accidentally hard-code a provider assumption on
// the way past.
//
// There is no `theater` package. The concept names the boundary; the implementation keeps the
// names that already describe what it does. See
// [[ADR-068-the-theater-is-the-durable-semantic-world]].
//
// # Where Theater sits
//
//	Director   decides what should be accomplished
//	Theater    knows the world it must be accomplished in, and mounts the production
//	Fusion     reconciles live observation toward those identities; persists nothing
//	Providers  supply observations and the capabilities that can act
//	Audience   supplies intent, names, corrections and consent — the only semantic authority
//
// This file is the durable half: the repertoire. What is on stage right now is live evidence and
// belongs to perception, not here.

// TargetKind is the small semantic vocabulary a durable target may be described by.
//
// # Why this is closed, and why it is not a control-type list
//
// Because a durable target that recorded "ListItem" would be remembering how one provider on one
// operating system described a thing, and a play resolved through it would quietly require that
// provider's vocabulary forever. What survives is what the person would say: it was a button, or
// a row in a list, or a box you type in.
//
// Small on purpose. Every word here has to mean something to a reader with no idea what UI
// Automation is, and a vocabulary that grew to cover every provider's taxonomy would be that
// taxonomy wearing a different hat.
type TargetKind string

const (
	KindButton   TargetKind = "button"
	KindItem     TargetKind = "item"
	KindField    TargetKind = "field"
	KindMenu     TargetKind = "menu"
	KindLink     TargetKind = "link"
	KindTab      TargetKind = "tab"
	KindCheckbox TargetKind = "checkbox"
	KindWindow   TargetKind = "window"
)

// targetKinds maps what a PROVIDER called something onto what Marco calls it.
//
// The one place a provider's vocabulary is allowed to touch a durable one, and it is a narrowing:
// anything not listed becomes empty, which means "Marco does not know what sort of thing this
// is". Unknown is not false — a target with no kind is still a target, and the matcher treats a
// missing kind as saying nothing rather than as disagreeing.
var targetKinds = map[string]TargetKind{
	"button":     KindButton,
	"menu_item":  KindItem,
	"list_item":  KindItem,
	"tree_item":  KindItem,
	"tab":        KindTab,
	"tab_item":   KindTab,
	"checkbox":   KindCheckbox,
	"radio":      KindCheckbox,
	"link":       KindLink,
	"hyperlink":  KindLink,
	"menu":       KindMenu,
	"text_field": KindField,
	"edit":       KindField,
	"combo_box":  KindField,
	"window":     KindWindow,
}

// TargetKindOf is what Marco calls a control a provider described this way.
//
// Empty when nothing in the vocabulary fits, which is the honest answer and not a failure.
func TargetKindOf(role string) TargetKind {
	return targetKinds[strings.ToLower(strings.TrimSpace(role))]
}

// TargetSignature is the durable identity of one semantic target.
//
// Built here rather than by callers so there is one answer to "what makes a target that target",
// and so the fields a target must NEVER carry have nowhere to be set from: there is no parameter
// on this function for a runtime id, a node handle, a rectangle or a coordinate.
func TargetSignature(place string, label string, kind TargetKind) StructureSignature {
	return StructureSignature{
		Subject: SubjectTarget,
		Label:   strings.TrimSpace(label),
		Kind:    string(kind),
		Place:   place,
	}
}

// Theater is the durable semantic world, as the Director and a running play may ask about it.
//
// Narrow on purpose. Every method is about MEANING — what things are, where they are, what they
// are called — and none of them can reach a pass, a sample, a provider or an effect. A caller
// holding one cannot perceive and cannot act.
type Theater interface {
	// RememberTarget makes a semantic target durable, asserting nothing about what it means.
	//
	// The same licence discipline a place has: the caller has to have one, and the label must
	// already have passed AdmittedTargetLabel. Idempotent — a target already known is
	// returned unchanged, with anything the Audience has said about it untouched.
	RememberTarget(application string, sig StructureSignature) (string, error)
	// ResolveTarget is which durable target this description is, if Marco knows one.
	ResolveTarget(application string, sig StructureSignature) Recollection
	// TargetsIn is every target known to be grounded in a place, in a stable order.
	//
	// What lets a surface say what Marco knows about a screen, and what lets a play be
	// checked against a world before it is run.
	TargetsIn(application, place string) []RememberedSubject
}

// TargetsGroundedIn selects the targets belonging to a place out of a subject list.
//
// A helper rather than a query, because the store's own listing is the canonical enumeration and
// a second one would be a second answer. Deterministic order: by label, so two readings of one
// world produce the same account.
func TargetsGroundedIn(subjects []RememberedSubject, application, place string) []RememberedSubject {
	var out []RememberedSubject
	for _, s := range subjects {
		if s.Structure.Subject != SubjectTarget || s.Structure.Place != place {
			continue
		}
		if application != "" && !strings.EqualFold(s.Application, application) {
			continue
		}
		out = append(out, s)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Structure.Label < out[j-1].Structure.Label; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Authoritative reports whether a source may CORRECT what Marco believes.
//
// Only the person. Perception supplies evidence and evidence can be wrong; a person saying "no,
// that is the other one" is a decision. Weighing the two as though they were the same kind of
// claim is how an observed label comes to outrank the human who corrected it — which is the one
// failure this distinction exists to make impossible.
//
// A method on the EXISTING vocabulary rather than a second one. `EvidenceSource` already means
// "an independent way of knowing" and already separates the person from the providers; what was
// missing was only the word for what that separation is FOR.
func (s EvidenceSource) Authoritative() bool { return s == FromUser }

// TargetsDemonstrated is every semantic target a demonstration shows the person acting on.
//
// # Which place each one belongs to
//
// The place the click happened ON, which is the checkpoint the step STARTED from — not the one it
// arrived at. A person on the Bluetooth screen presses "Mouse" and ends up on the Mouse screen;
// the target belongs to Bluetooth, because that is where it can be found again. Getting this
// backwards would ground every target on the screen it navigates away to, and no play could ever
// resolve one.
//
// Only labelled targets survive. An unnamed one has nothing to be found by — see Discriminating —
// and admitting it would store a record nothing could ever match.
func TargetsDemonstrated(c ProcedureCandidate) []StructureSignature {
	place := c.Start.Subject
	var out []StructureSignature
	for _, step := range c.Steps {
		for i, in := range step.Intents {
			if in != NavPoint || place == "" {
				continue
			}
			t := targetAt(step.Targets, i)
			if t == "" {
				continue
			}
			var kind TargetKind
			if i < len(step.Targets) {
				kind = TargetKindOf(step.Targets[i].Role)
			}
			out = append(out, TargetSignature(place, t, kind))
		}
		// The next step's actions happen wherever this one ended up.
		place = step.Arrived.Subject
	}
	return out
}

// TargetStore is the durable half of the Repertoire: where a semantic target is kept.
//
// Narrower than Theater on purpose, and narrower than Memory in the direction that matters: it
// can write a target's identity and nothing else. There is no SemanticKnowledge here, no
// relationship, no goal and no authority, so a caller holding one cannot record a judgement about
// a target, cannot connect it to anything, and cannot act on it.
type TargetStore interface {
	// RememberTarget makes a target durable, asserting nothing about what it means.
	//
	// `learned` is provenance — which way of knowing produced the evidence — and is never a
	// statement about how the target must be reached later.
	RememberTarget(application string, sig StructureSignature, learned EvidenceSource) (
		string, error)
}

// TargetSweepStore is the batch half of TargetStore: everything one settled screen offers, written
// together.
//
// # Why a batch rather than a loop over RememberTarget
//
// Two reasons, and the second is the load-bearing one.
//
// One save instead of one per control, which matters when a screen offers thirty.
//
// And ONE announcement. A learning feed reports what changed to a person, and thirty events saying
// "noticed a control" would bury the one saying "learned a way" under a screen's worth of
// furniture — the exact failure Strengthened exists to prevent, arriving from a new direction. A
// batch is what lets the store say "six things you can do here" as one commit, and the store is
// the only honest place to say it from: it is what knows how many of the six were actually new.
type TargetSweepStore interface {
	// RememberTargetsSeen makes several targets durable together, returning how many were new.
	//
	// The same discipline RememberTarget follows for each: idempotent by signature, no
	// judgement written, an existing record returned untouched.
	RememberTargetsSeen(application string, sigs []StructureSignature,
		learned EvidenceSource) (int, error)
}
