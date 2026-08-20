package game

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Conditions over the entities a pack contributed.
//
//	Health below 30%. Inventory full. Workbench open. Craft complete. Durability below
//	10%. These become ordinary Director conditions.
//
// Ordinary in the strongest sense: each implements conditions.WorldCondition and is
// answered by the ordinary wait engine, against an ordinary World State, with no code path
// of its own. What makes them possible is not machinery here — it is that a pack's
// observers put entities in the world, and these read them.
//
// They live in the FRAMEWORK rather than in a pack because none of them is game-specific.
// "A meter is below a threshold" is a shape, and which meter is called health is the pack's
// business. A pack that needs something genuinely its own implements the same interface and
// contributes it through Conditions().
//
// # The three-valued answer is the whole design
//
// Every condition here can report UNKNOWABLE, and every one of them uses it. A world where
// the inventory could not be read does not contain a not-full inventory; it contains an
// unanswered question, and a wait that treated the two alike would proceed as though it had
// checked.

// Condition ids, so a trace can name what was waited for.
const (
	IDMeterBelow       conditions.ID = "game.meter_below"
	IDMeterAbove       conditions.ID = "game.meter_above"
	IDInventoryFull    conditions.ID = "game.inventory_full"
	IDInventoryFree    conditions.ID = "game.inventory_has_room"
	IDEntityPresent    conditions.ID = "game.entity_present"
	IDEntityAbsent     conditions.ID = "game.entity_absent"
	IDEntityState      conditions.ID = "game.entity_state"
	IDItemCountAtLeast conditions.ID = "game.item_count_at_least"
)

// ── meters ────────────────────────────────────────────────────────────────────

// MeterBelow waits until a named meter drops under a fraction.
type MeterBelow struct {
	Meter    string
	Fraction float64
}

func (c MeterBelow) ID() conditions.ID { return IDMeterBelow }

func (c MeterBelow) Description() string {
	return fmt.Sprintf("%s is below %.0f%%", c.Meter, c.Fraction*100)
}

func (c MeterBelow) Evaluate(w directorapi.WorldState) evaluation.Result {
	level, ok := MeterLevel(w, c.Meter)
	if !ok {
		return evaluation.Unknowable(fmt.Sprintf(
			"nothing in this world reports %s, so whether it is below %.0f%% cannot be "+
				"answered", c.Meter, c.Fraction*100))
	}
	if level < c.Fraction {
		return evaluation.Satisfy(1, fmt.Sprintf("%s is at %.0f%%", c.Meter, level*100))
	}
	return evaluation.Deny(1, fmt.Sprintf("%s is at %.0f%%", c.Meter, level*100))
}

// MeterAbove is its complement.
type MeterAbove struct {
	Meter    string
	Fraction float64
}

func (c MeterAbove) ID() conditions.ID { return IDMeterAbove }

func (c MeterAbove) Description() string {
	return fmt.Sprintf("%s is above %.0f%%", c.Meter, c.Fraction*100)
}

func (c MeterAbove) Evaluate(w directorapi.WorldState) evaluation.Result {
	level, ok := MeterLevel(w, c.Meter)
	if !ok {
		return evaluation.Unknowable(fmt.Sprintf(
			"nothing in this world reports %s", c.Meter))
	}
	if level > c.Fraction {
		return evaluation.Satisfy(1, fmt.Sprintf("%s is at %.0f%%", c.Meter, level*100))
	}
	return evaluation.Deny(1, fmt.Sprintf("%s is at %.0f%%", c.Meter, level*100))
}

// ── inventory ─────────────────────────────────────────────────────────────────

// InventoryFull waits until a container has no room.
type InventoryFull struct{ Container string }

func (c InventoryFull) ID() conditions.ID { return IDInventoryFull }

func (c InventoryFull) Description() string {
	return describeContainer(c.Container) + " is full"
}

func (c InventoryFull) Evaluate(w directorapi.WorldState) evaluation.Result {
	inv := ReadInventory(w, c.Container)
	full, known := inv.Full()
	if !known {
		return evaluation.Unknowable(unreadable(inv))
	}
	if full {
		return evaluation.Satisfy(1, inv.Describe())
	}
	return evaluation.Deny(1, inv.Describe())
}

// InventoryHasRoom is its complement, and deliberately not "not full".
//
// The negation of an unknowable is still unknowable, and a caller who wrote Not(Full)
// would get that right — but "wait until there is room" is the request people make, and a
// condition that says so reads better in a trace than a negation of one that does not.
type InventoryHasRoom struct{ Container string }

func (c InventoryHasRoom) ID() conditions.ID { return IDInventoryFree }

func (c InventoryHasRoom) Description() string {
	return describeContainer(c.Container) + " has room"
}

func (c InventoryHasRoom) Evaluate(w directorapi.WorldState) evaluation.Result {
	inv := ReadInventory(w, c.Container)
	full, known := inv.Full()
	if !known {
		return evaluation.Unknowable(unreadable(inv))
	}
	if !full {
		return evaluation.Satisfy(1, inv.Describe())
	}
	return evaluation.Deny(1, inv.Describe())
}

// ItemCountAtLeast waits until a named item reaches a quantity.
//
//	Craft complete → item count increased.
type ItemCountAtLeast struct {
	Item  string
	Count int
}

func (c ItemCountAtLeast) ID() conditions.ID { return IDItemCountAtLeast }

func (c ItemCountAtLeast) Description() string {
	return fmt.Sprintf("there are at least %d %s", c.Count, c.Item)
}

func (c ItemCountAtLeast) Evaluate(w directorapi.WorldState) evaluation.Result {
	total, readable := 0, false
	for _, it := range ReadInventory(w, "").Named(c.Item).Items {
		if n, ok := it.Entity.Count(); ok {
			total += n
			readable = true
		}
	}
	if !readable {
		return evaluation.Unknowable(fmt.Sprintf(
			"no readable count of %s was found, so whether there are %d cannot be answered",
			c.Item, c.Count))
	}
	if total >= c.Count {
		return evaluation.Satisfy(1, fmt.Sprintf("there are %d %s", total, c.Item))
	}
	return evaluation.Deny(1, fmt.Sprintf("there are %d %s", total, c.Item))
}

// ── entities ──────────────────────────────────────────────────────────────────

// EntityPresent waits until an entity of a kind, and optionally a name, exists.
//
//	Workbench open. Chest open.
type EntityPresent struct {
	Kind directorapi.EntityKind
	Name string
}

func (c EntityPresent) ID() conditions.ID { return IDEntityPresent }

func (c EntityPresent) Description() string { return describeEntity(c.Kind, c.Name) + " is present" }

func (c EntityPresent) Evaluate(w directorapi.WorldState) evaluation.Result {
	if el, ok := findEntity(w, c.Kind, c.Name); ok {
		return evaluation.Satisfy(1, el.Entity.Describe()+" is present")
	}
	return evaluation.Deny(1, describeEntity(c.Kind, c.Name)+" is not present")
}

// EntityAbsent waits until it is gone.
type EntityAbsent struct {
	Kind directorapi.EntityKind
	Name string
}

func (c EntityAbsent) ID() conditions.ID { return IDEntityAbsent }

func (c EntityAbsent) Description() string { return describeEntity(c.Kind, c.Name) + " is gone" }

func (c EntityAbsent) Evaluate(w directorapi.WorldState) evaluation.Result {
	if el, ok := findEntity(w, c.Kind, c.Name); ok {
		return evaluation.Deny(1, el.Entity.Describe()+" is still present")
	}
	return evaluation.Satisfy(1, describeEntity(c.Kind, c.Name)+" is not present")
}

// EntityState waits until an entity reports a state.
//
//	Craft complete. Incubator ready.
type EntityState struct {
	Kind  directorapi.EntityKind
	Name  string
	State string
}

func (c EntityState) ID() conditions.ID { return IDEntityState }

func (c EntityState) Description() string {
	return fmt.Sprintf("%s is %s", describeEntity(c.Kind, c.Name), c.State)
}

func (c EntityState) Evaluate(w directorapi.WorldState) evaluation.Result {
	el, ok := findEntity(w, c.Kind, c.Name)
	if !ok {
		return evaluation.Unknowable(fmt.Sprintf(
			"%s is not in this world, so its state cannot be read",
			describeEntity(c.Kind, c.Name)))
	}
	if el.Entity.State == "" {
		return evaluation.Unknowable(fmt.Sprintf(
			"%s reports no state", el.Entity.Describe()))
	}
	if equalFold(el.Entity.State, c.State) {
		return evaluation.Satisfy(1, el.Entity.Describe())
	}
	return evaluation.Deny(1, el.Entity.Describe())
}

// ── helpers ───────────────────────────────────────────────────────────────────

// findEntity looks for one entity by kind and optional name.
func findEntity(w directorapi.WorldState, kind directorapi.EntityKind, name string) (
	*directorapi.Element, bool) {

	for _, el := range w.Elements {
		e := el.Entity
		if !e.Known() || e.Kind != kind {
			continue
		}
		if name != "" && !equalFold(e.Name, name) {
			continue
		}
		return el, true
	}
	return nil, false
}

func describeEntity(kind directorapi.EntityKind, name string) string {
	if name == "" {
		return "a " + string(kind)
	}
	return fmt.Sprintf("the %s %q", kind, name)
}

func describeContainer(name string) string {
	if name == "" {
		return "the inventory"
	}
	return name
}

// unreadable explains why a fullness question could not be answered.
func unreadable(inv Inventory) string {
	if inv.Slots == 0 {
		return "nothing in this world reports inventory slots, so whether it is full " +
			"cannot be answered"
	}
	return fmt.Sprintf("%d of %d slot(s) could not be read, so whether it is full cannot "+
		"be answered", inv.Unknown, inv.Slots)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lower(a[i]) != lower(b[i]) {
			return false
		}
	}
	return true
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// Every condition here is an ordinary world condition, asserted rather than assumed.
var (
	_ conditions.WorldCondition = MeterBelow{}
	_ conditions.WorldCondition = MeterAbove{}
	_ conditions.WorldCondition = InventoryFull{}
	_ conditions.WorldCondition = InventoryHasRoom{}
	_ conditions.WorldCondition = ItemCountAtLeast{}
	_ conditions.WorldCondition = EntityPresent{}
	_ conditions.WorldCondition = EntityAbsent{}
	_ conditions.WorldCondition = EntityState{}
)
