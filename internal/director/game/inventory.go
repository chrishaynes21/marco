package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Reading an inventory out of a World State.
//
//	Plugins should model slots, stacks, containers, equipment, hotbar, weight,
//	capacity. These become semantic collections. Collections already exist. The plugin
//	provides the meaning.
//
// So this file adds no model. It READS the entities a pack's observers contributed and
// answers the questions a request implies — "everything except food", "every damaged tool",
// "is this full?" — over the ordinary world, in terms of the ordinary EntityIdentity.
//
// Nothing here acts. An inventory view is a description of what was observed, and the thing
// that moves an item is an ordinary semantic action on an ordinary element, planned,
// confirmed, lowered to Marco and verified like every other.
//
// # Why this is not a collection type
//
// The collections layer already answers "the bounded set matching this query, re-run every
// iteration". An inventory is a place to look, not a second kind of set — so what this
// provides is the QUERY, and the iteration, the ordinal-drift protection and the
// verify-before-advance all stay where they are.

// Item is one thing in an inventory, as observed.
type Item struct {
	// Element is which element carries it, so a caller can act on it through the
	// ordinary path. An id, valid within this world and never stored.
	Element directorapi.ElementID `json:"element"`
	// Entity is the application's own account of it.
	Entity directorapi.EntityIdentity `json:"entity"`
	// Label is the control's caption, kept because it is what a user reads.
	Label string `json:"label,omitempty"`
}

// Describe renders an item in one phrase.
func (i Item) Describe() string { return i.Entity.Describe() }

// Inventory is what a container holds, as one observation saw it.
type Inventory struct {
	// Container is the name of the container this describes, empty for everything the
	// world could see.
	Container string `json:"container,omitempty"`
	// Items are the things in it, in slot order where slots are known.
	Items []Item `json:"items,omitempty"`
	// Slots is how many positions were seen, including empty ones.
	Slots int `json:"slots"`
	// Filled is how many hold something established.
	Filled int `json:"filled"`
	// Unknown is how many could not be read. The number that stops "deposit
	// everything" from being a safe request when it is not zero.
	Unknown int `json:"unknown"`
	// Capacity is how many fit, when a meter or a container said so.
	Capacity int `json:"capacity,omitempty"`
}

// Full reports whether the container is established to have no room.
//
// A THREE-way answer collapsed carefully: full when every readable slot holds something
// and nothing was unreadable; not full when a slot is established empty; and not full —
// with known=false — when the answer rests on slots nobody could read.
func (inv Inventory) Full() (full, known bool) {
	switch {
	case inv.Slots == 0:
		return false, false
	case inv.Unknown > 0:
		// Some slots could not be read. "Full" would be a guess, and the guess that
		// stops a deposit is the one that leaves a user wondering why nothing happened.
		return false, false
	case inv.Capacity > 0:
		return inv.Filled >= inv.Capacity, true
	}
	return inv.Filled >= inv.Slots, true
}

// Describe renders an inventory for a person.
func (inv Inventory) Describe() string {
	where := inv.Container
	if where == "" {
		where = "the inventory"
	}
	out := fmt.Sprintf("%s: %d of %d slot(s) filled", where, inv.Filled, inv.Slots)
	if inv.Capacity > 0 {
		out += fmt.Sprintf(", capacity %d", inv.Capacity)
	}
	if inv.Unknown > 0 {
		out += fmt.Sprintf(", %d unreadable", inv.Unknown)
	}
	return out
}

// ReadInventory gathers what a world says about one container.
//
// Container empty means everything the world could see, which is what "sort my inventory"
// asks about when the player has one. A name narrows it to that container's contents.
func ReadInventory(w directorapi.WorldState, container string) Inventory {
	inv := Inventory{Container: container}
	for _, el := range w.Elements {
		e := el.Entity
		if !e.Known() {
			continue
		}
		if container != "" && !strings.EqualFold(e.Container, container) &&
			!strings.EqualFold(e.Name, container) {
			continue
		}
		switch e.Kind {
		case directorapi.EntityContainer:
			// The container itself carries the capacity, when it knows one.
			if c := e.Capacity; c != nil && *c > inv.Capacity {
				inv.Capacity = *c
			}
		case directorapi.EntitySlot, directorapi.EntityItem, directorapi.EntityEquipment:
			inv.Items = append(inv.Items, Item{
				Element: el.ID, Entity: *e.Clone(), Label: el.Label,
			})
			inv.Slots++
			switch {
			case !e.Countable():
				inv.Unknown++
			case !e.Empty():
				inv.Filled++
			}
		}
	}
	// Slot order where it is known, then by name, so two reads of an unchanged screen
	// produce the same list — which is what a collection iterating it depends on.
	sort.SliceStable(inv.Items, func(i, j int) bool {
		a, b := inv.Items[i].Entity, inv.Items[j].Entity
		if a.Slot != b.Slot {
			if a.Slot == 0 || b.Slot == 0 {
				return b.Slot == 0
			}
			return a.Slot < b.Slot
		}
		return a.Name < b.Name
	})
	return inv
}

// ── the selections a request implies ──────────────────────────────────────────

// Selection is a set of items chosen by a rule, with the rule written down.
type Selection struct {
	// Items are what was chosen.
	Items []Item `json:"items,omitempty"`
	// Rule is what chose them, for the explanation and the confirmation prompt.
	Rule string `json:"rule"`
	// Skipped is how many items were left out because they could not be read. Reported
	// rather than hidden: "deposit everything" that silently skipped four unreadable
	// slots did not deposit everything.
	Skipped int `json:"skipped,omitempty"`
}

// Describe renders a selection for a prompt.
func (s Selection) Describe() string {
	out := fmt.Sprintf("%d item(s) — %s", len(s.Items), s.Rule)
	if s.Skipped > 0 {
		out += fmt.Sprintf("; %d could not be read and were left alone", s.Skipped)
	}
	return out
}

// Everything selects every item that holds something.
func (inv Inventory) Everything() Selection {
	sel := Selection{Rule: "everything that could be read"}
	for _, it := range inv.Items {
		switch {
		case !it.Entity.Countable():
			sel.Skipped++
		case !it.Entity.Empty():
			sel.Items = append(sel.Items, it)
		}
	}
	return sel
}

// Except selects everything but the named categories.
//
//	Deposit everything except food.
//
// The categories are the PACK's vocabulary. The framework compares strings and has no
// opinion about what "food" is — which is the division of labour the whole milestone rests
// on.
func (inv Inventory) Except(categories ...string) Selection {
	sel := Selection{Rule: "everything except " + strings.Join(categories, ", ")}
	for _, it := range inv.Items {
		if !it.Entity.Countable() {
			sel.Skipped++
			continue
		}
		if it.Entity.Empty() || inCategories(it.Entity, categories) {
			continue
		}
		sel.Items = append(sel.Items, it)
	}
	return sel
}

// OfCategory selects the items in one category.
func (inv Inventory) OfCategory(category string) Selection {
	sel := Selection{Rule: "everything in " + category}
	for _, it := range inv.Items {
		if !it.Entity.Countable() {
			sel.Skipped++
			continue
		}
		if it.Entity.InCategory(category) {
			sel.Items = append(sel.Items, it)
		}
	}
	return sel
}

// Named selects the items whose name matches, case-insensitively.
func (inv Inventory) Named(name string) Selection {
	sel := Selection{Rule: fmt.Sprintf("everything called %q", name)}
	for _, it := range inv.Items {
		if strings.EqualFold(it.Entity.Name, name) {
			sel.Items = append(sel.Items, it)
		}
	}
	return sel
}

// Below selects the items whose meter is under a threshold.
//
//	Repair every damaged tool.
//
// Items with no readable level are SKIPPED rather than assumed intact: a tool whose
// durability could not be read is not a tool known to be fine.
func (inv Inventory) Below(fraction float64) Selection {
	sel := Selection{Rule: fmt.Sprintf("everything below %.0f%%", fraction*100)}
	for _, it := range inv.Items {
		level, ok := it.Entity.Level()
		if !ok {
			sel.Skipped++
			continue
		}
		if level < fraction {
			sel.Items = append(sel.Items, it)
		}
	}
	return sel
}

func inCategories(e directorapi.EntityIdentity, categories []string) bool {
	for _, c := range categories {
		if e.InCategory(c) {
			return true
		}
	}
	return false
}

// ── meters ────────────────────────────────────────────────────────────────────

// Meter is a named quantity with a maximum — health, weight, durability, progress.
type Meter struct {
	Name  string  `json:"name"`
	Level float64 `json:"level"`
	// Element is which element reported it.
	Element directorapi.ElementID `json:"element"`
}

// Meters reads every meter a world reports.
func Meters(w directorapi.WorldState) []Meter {
	var out []Meter
	for _, el := range w.Elements {
		e := el.Entity
		if !e.Known() || e.Kind != directorapi.EntityMeter {
			continue
		}
		level, ok := e.Level()
		if !ok {
			continue
		}
		out = append(out, Meter{Name: e.Name, Level: level, Element: el.ID})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// MeterLevel is one meter's level, and whether it was found.
func MeterLevel(w directorapi.WorldState, name string) (float64, bool) {
	for _, m := range Meters(w) {
		if strings.EqualFold(m.Name, name) {
			return m.Level, true
		}
	}
	return 0, false
}

// ── stations and queues ───────────────────────────────────────────────────────

// Station is a place where work happens — a workbench, a furnace, an incubator.
type Station struct {
	Name    string                `json:"name"`
	State   string                `json:"state,omitempty"`
	Element directorapi.ElementID `json:"element"`
	// Progress is how far along it is, when it says.
	Progress *float64 `json:"progress,omitempty"`
}

// Stations reads every station a world reports.
func Stations(w directorapi.WorldState) []Station {
	var out []Station
	for _, el := range w.Elements {
		e := el.Entity
		if !e.Known() || (e.Kind != directorapi.EntityStation && e.Kind != directorapi.EntityQueue) {
			continue
		}
		s := Station{Name: e.Name, State: e.State, Element: el.ID}
		if f, ok := e.Level(); ok {
			s.Progress = &f
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
