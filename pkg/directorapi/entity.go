package directorapi

import (
	"fmt"
	"strings"
)

// The THING a control stands for inside an application's own model.
//
//	Plugins may enrich World State. These become ordinary semantic entities, not
//	game-specific hacks.
//
// ResourceIdentity answers "what object outside the application is this?" — a file, a
// folder, something with an existence the operating system can confirm. An entity answers a
// different question: "what is this WITHIN the application's own world?" A slot holding
// forty-three arrows is not a file and has no path; it is a real thing the user reasons
// about, and until this existed the Director could see only a control captioned "43".
//
// # Why this is not a game feature
//
// Nothing here says "game". An inventory slot, a container, a queue with progress, a
// resource meter — these are the shapes a mail client's folder list, a music player's
// queue and a build system's job list have too. What is game-specific is which controls
// carry which entities, and that is exactly what a capability pack contributes and the
// Director never learns.
//
// # What may not go in here
//
// The same rule ResourceIdentity follows, for the same reason: no coordinates, no handles,
// no process-local identifiers. An entity is written to the action graph and read by a
// later process, so a field meaningful only inside one observation would be a trap for the
// code that trusts this most.
//
// And no MEMORY. Every field here is something a provider read from the interface the
// player is looking at. Nothing in this design reads a game's memory, and there is no field
// that would carry the result if something did.
type EntityIdentity struct {
	// Kind is what sort of thing this is, from the closed vocabulary below.
	Kind EntityKind `json:"kind"`
	// Name is what the application calls it — the item in a slot, the label on a
	// container. The user's own word for the thing, and what a request names.
	Name string `json:"name,omitempty"`
	// Category groups entities the way the application does ("food", "tool", "ore").
	// Contributed by the pack, opaque to the Director, and what "deposit everything
	// except food" is answered from.
	Category string `json:"category,omitempty"`

	// Quantity is how many, when the entity is countable. Absent (nil) and zero are
	// different: an empty slot holds zero, and a slot whose count could not be read
	// holds an unknown number — and "deposit everything" must not treat the second as
	// the first.
	Quantity *int `json:"quantity,omitempty"`
	// Capacity is how many fit, when that is knowable.
	Capacity *int `json:"capacity,omitempty"`
	// Fraction is a proportion 0..1 for a meter — durability, health, progress.
	Fraction *float64 `json:"fraction,omitempty"`

	// Container is the entity this one is inside, by NAME rather than by id: a slot
	// belongs to "the chest", and which element the chest is is a question for the
	// world, answered fresh each time it is asked.
	Container string `json:"container,omitempty"`
	// Slot is the position within its container, 1-based, 0 when unpositioned.
	Slot int `json:"slot,omitempty"`

	// State is what the application says it is doing: "crafting", "ready", "empty".
	// A closed vocabulary per pack, reported and never interpreted by the Director.
	State string `json:"state,omitempty"`

	// Source names the pack and provider that established this, so a reader can weigh
	// it and two mechanisms can be told apart.
	Source string `json:"source,omitempty"`
	// Confidence is the source's certainty, 0..1.
	Confidence float64 `json:"confidence,omitempty"`
	// Evidence explains how it was established, one clause each.
	Evidence []string `json:"evidence,omitempty"`
}

// EntityKind is the closed vocabulary of what an entity can BE.
//
// Closed, and deliberately small. A pack that needed a kind absent from here would be
// describing something the Director has no way to reason about — and the honest answer to
// that is to add it here, with the semantics written down, rather than to let each pack
// invent nouns nothing else understands.
type EntityKind string

const (
	// EntityItem is a thing a user can hold, move, count or consume.
	EntityItem EntityKind = "item"
	// EntitySlot is a position that may hold an item, including an empty one. Distinct
	// from the item: "sort this inventory" acts on slots, "deposit the stone" on items.
	EntitySlot EntityKind = "slot"
	// EntityContainer holds slots — a chest, a bag, a stash, a mailbox.
	EntityContainer EntityKind = "container"
	// EntityEquipment is an item in use rather than in storage.
	EntityEquipment EntityKind = "equipment"
	// EntityQueue is an ordered set of work with progress — a craft queue, a build
	// list, a download list.
	EntityQueue EntityKind = "queue"
	// EntityRecipe is something that CAN be made, as opposed to something that is.
	EntityRecipe EntityKind = "recipe"
	// EntityMeter is a quantity with a maximum: health, weight, durability, progress.
	EntityMeter EntityKind = "meter"
	// EntityCreature is a living thing the user manages — a pal, a pet, a villager, a
	// crew member. Named for what it is rather than for any one game's word.
	EntityCreature EntityKind = "creature"
	// EntityStation is a place where work happens — a workbench, a furnace, an
	// incubator, a terminal.
	EntityStation EntityKind = "station"
	// EntityMarker is a point of interest — a map pin, a waypoint, a quest target.
	EntityMarker EntityKind = "marker"
)

// entityKinds is every kind, in a stable order, for listing and validation.
var entityKinds = []EntityKind{
	EntityItem, EntitySlot, EntityContainer, EntityEquipment, EntityQueue,
	EntityRecipe, EntityMeter, EntityCreature, EntityStation, EntityMarker,
}

// EntityKinds returns the vocabulary.
func EntityKinds() []EntityKind {
	return append([]EntityKind{}, entityKinds...)
}

// Known reports whether a kind is one the Director can reason about.
func (k EntityKind) Known() bool {
	for _, v := range entityKinds {
		if v == k {
			return true
		}
	}
	return false
}

// Describe renders a kind for a person.
func (k EntityKind) Describe() string { return string(k) }

// Known reports whether this is an entity anything can act on.
//
// A KIND the Director understands. Without one there is nothing to reason about, and a
// pack that reported a name with no kind would be contributing a caption — which the
// element already had.
func (e *EntityIdentity) Known() bool {
	return e != nil && e.Kind.Known()
}

// Countable reports whether the quantity is established.
//
// The distinction the nil pointer exists for: an empty slot and a slot nobody could read
// are different states, and only one of them is safe to act on with "everything".
func (e *EntityIdentity) Countable() bool {
	return e != nil && e.Quantity != nil
}

// Count is how many, and whether that is known.
func (e *EntityIdentity) Count() (int, bool) {
	if !e.Countable() {
		return 0, false
	}
	return *e.Quantity, true
}

// Empty reports whether a slot is established to hold nothing.
//
// False for a slot whose contents could not be read, which is the point: "is this empty?"
// and "do I know what is in this?" are different questions, and conflating them is how a
// sort silently discards something it could not see.
func (e *EntityIdentity) Empty() bool {
	n, ok := e.Count()
	return ok && n == 0
}

// Level is the meter's proportion, and whether it is known.
func (e *EntityIdentity) Level() (float64, bool) {
	if e == nil || e.Fraction == nil {
		return 0, false
	}
	return *e.Fraction, true
}

// InCategory reports whether the entity belongs to a category, case-insensitively.
func (e *EntityIdentity) InCategory(category string) bool {
	return e != nil && category != "" && strings.EqualFold(e.Category, category)
}

// Describe renders an entity in one phrase.
func (e *EntityIdentity) Describe() string {
	if !e.Known() {
		return "(nothing)"
	}
	out := string(e.Kind)
	if e.Name != "" {
		out = fmt.Sprintf("%s %q", e.Kind, e.Name)
	}
	if n, ok := e.Count(); ok {
		out += fmt.Sprintf(" ×%d", n)
	}
	if f, ok := e.Level(); ok {
		out += fmt.Sprintf(" at %.0f%%", f*100)
	}
	if e.Container != "" {
		out += " in " + e.Container
		if e.Slot > 0 {
			out += fmt.Sprintf(" slot %d", e.Slot)
		}
	}
	if e.State != "" {
		out += " (" + e.State + ")"
	}
	return out
}

// Clone returns a deep copy, nil for nil.
func (e *EntityIdentity) Clone() *EntityIdentity {
	if e == nil {
		return nil
	}
	out := *e
	out.Evidence = append([]string{}, e.Evidence...)
	if e.Quantity != nil {
		n := *e.Quantity
		out.Quantity = &n
	}
	if e.Capacity != nil {
		n := *e.Capacity
		out.Capacity = &n
	}
	if e.Fraction != nil {
		f := *e.Fraction
		out.Fraction = &f
	}
	return &out
}

// Quantity builds a countable quantity, for a provider filling one in.
func Quantity(n int) *int { return &n }

// Fraction builds a meter level, clamped to 0..1.
//
// Clamped rather than rejected: a provider that read 101% of durability has a rounding
// problem, not a reason to make the whole observation unusable.
func Fraction(f float64) *float64 {
	switch {
	case f < 0:
		f = 0
	case f > 1:
		f = 1
	}
	return &f
}
