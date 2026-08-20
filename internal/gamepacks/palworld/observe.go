package palworld

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Turning observed controls into entities.
//
//	A capability pack may contribute OCR providers, vision providers, HUD providers,
//	inventory providers, map providers, crafting providers. They produce Observations
//	only. Never semantic actions. Never execution.
//
// This one is an INTERPRETER. It observes nothing of its own: it is a refiner over the
// evidence another provider already produced, and everything it emits is an observation
// carrying an EntityIdentity. That is deliberate and worth being clear about, because the
// alternative shapes are both worse:
//
//   - A provider that captured the screen itself would be a second observation path, and
//     the milestone forbids one.
//   - A provider that reached into the game would be reading memory, which is a non-goal
//     and has no field in EntityIdentity that could carry the result.
//
// So the interpretation is: this control's caption reads "Wood 43", the accessibility tree
// says it is a list item inside a panel called Inventory, and this pack knows that means a
// slot holding 43 Wood. The Director then reasons about a slot holding 43 Wood, which is
// something it understands without knowing what Wood is.
//
// # The honest status
//
// The patterns below are written from Palworld's English interface as documented. Whether a
// real Palworld window exposes an accessibility tree at all is UNKNOWN and untested — most
// full-screen games expose nothing, which is why the vision and OCR providers exist and why
// this pack's next piece of work is a provider backed by one of them. See
// docs/director-games.md, Known gaps.

// interpreter says what this pack makes of one perceived control.
type interpreter struct{}

func newInterpreter() *interpreter { return &interpreter{} }

var _ game.Interpreter = (*interpreter)(nil)

func (*interpreter) Name() string { return Name }

// Interpret reads one ELEMENT and reports what this pack makes of it.
//
// An element, not an observation: only internal/director/perception may see evidence, and
// a pack reasons over belief like everything else outside it. The practical effect is that
// this cannot ask how a control was seen — it is handed what the Director established, and
// says which of it is an inventory slot.
func (*interpreter) Interpret(el directorapi.Element) (directorapi.EntityIdentity, bool) {
	// A GRID CELL first. A cell that a detector found and the geometry pass positioned
	// carries a durable identity the ordinary caption rules cannot produce, and reading it
	// as an anonymous list item would throw that away.
	if e, ok := interpretGridCell(el); ok {
		return e, true
	}
	return interpret(el)
}

// ── the interpretation itself ─────────────────────────────────────────────────

// stackPattern matches a slot caption: "Wood 43", "Wood x43", "Wood ×43".
//
// Anchored at the end, so an item whose NAME contains a number ("Pal Sphere 2") keeps it.
var stackPattern = regexp.MustCompile(`^(.*?)[\s]*[x×]?[\s]*(\d+)$`)

// meterPattern matches a meter caption: "Weight 143/300", "Health 87 / 100".
var meterPattern = regexp.MustCompile(`^(.*?)[\s]*(\d+)[\s]*/[\s]*(\d+)$`)

// percentPattern matches "Incubating 64%".
var percentPattern = regexp.MustCompile(`^(.*?)[\s]*(\d+)[\s]*%$`)

// interpret reads one control and reports what this pack makes of it.
//
// Returns false for everything it does not recognise, which is most of a window. A pack
// that guessed would fill the world with entities nothing could act on correctly.
func interpret(el directorapi.Element) (directorapi.EntityIdentity, bool) {
	label := strings.TrimSpace(el.Label)
	if label == "" {
		label = strings.TrimSpace(el.Value)
	}
	if label == "" {
		return directorapi.EntityIdentity{}, false
	}
	base := directorapi.EntityIdentity{Source: Name, Confidence: 0.8}

	// A meter: "Weight 143/300".
	if m := meterPattern.FindStringSubmatch(label); m != nil {
		name := strings.TrimSpace(m[1])
		now, _ := strconv.Atoi(m[2])
		max, _ := strconv.Atoi(m[3])
		if max > 0 && isMeter(name) {
			base.Kind = directorapi.EntityMeter
			base.Name = canonicalMeter(name)
			base.Quantity = directorapi.Quantity(now)
			base.Capacity = directorapi.Quantity(max)
			base.Fraction = directorapi.Fraction(float64(now) / float64(max))
			base.Evidence = []string{"the control reads " + label}
			return base, true
		}
	}

	// A station reporting progress: "Incubating 64%".
	if m := percentPattern.FindStringSubmatch(label); m != nil {
		pct, _ := strconv.Atoi(m[2])
		base.Kind = directorapi.EntityStation
		base.Name = canonicalStation(strings.TrimSpace(m[1]))
		base.Fraction = directorapi.Fraction(float64(pct) / 100)
		base.State = StateWorking
		if pct >= 100 {
			base.State = StateReady
		}
		base.Evidence = []string{"the control reads " + label}
		return base, true
	}

	// A named station or container, by exact caption.
	if kind, name, ok := namedEntity(label); ok {
		base.Kind = kind
		base.Name = name
		base.Evidence = []string{"the control is captioned " + label}
		return base, true
	}

	// A slot holding a stack: "Wood 43". Only inside something that is a container,
	// which is what stops a quest counter in a HUD from being read as an inventory slot.
	if m := stackPattern.FindStringSubmatch(label); m != nil && el.Role == directorapi.RoleListItem {
		name := strings.TrimSpace(m[1])
		count, _ := strconv.Atoi(m[2])
		if name != "" {
			base.Kind = directorapi.EntitySlot
			base.Name = name
			base.Category = categoryOf(name)
			base.Quantity = directorapi.Quantity(count)
			base.Container = containerOf(el)
			base.Evidence = []string{"a list item reading " + label}
			return base, true
		}
	}

	// A list item with no count, inside a container: an empty slot, or an item whose
	// count this pack could not read. Those are DIFFERENT, and the difference is the
	// Quantity pointer — left nil here, which is what makes "deposit everything" report
	// that it skipped something.
	if el.Role == directorapi.RoleListItem && containerOf(el) != "" {
		base.Kind = directorapi.EntitySlot
		base.Name = label
		base.Category = categoryOf(label)
		base.Container = containerOf(el)
		base.Confidence = 0.5
		base.Evidence = []string{
			"a list item in a container, with no readable count — its contents are " +
				"reported as unknown rather than as empty",
		}
		return base, true
	}
	return directorapi.EntityIdentity{}, false
}

// interpretGridCell reads a cell of a detected GRID as an inventory slot.
//
//	Vision → Inventory Grid → Interpreter → Inventory Collection.
//	The interpreter still works only on World State. Never pixels.
//
// And it does: what arrives here is an ELEMENT that the vision provider produced, the
// fusion engine believed, and the enrichment pass handed over. This function has no idea a
// camera was involved. What it recognises is a grid position — an ordinary attribute on an
// ordinary element — and what it concludes is that a regular arrangement of same-sized
// cells inside a Palworld window is the inventory.
//
// # Why the position and not the picture
//
// A vision cell has no label, no automation id and nothing durable but where it is. Its
// grid POSITION is the exception: cell (3,4) is cell (3,4) in the next frame and at a
// different window position, which is what lets a slot be talked about at all. So the slot
// this produces is named by its position, and its contents are UNKNOWN — the pack can see
// that there is a slot and cannot see what is in it, and saying so is what stops "deposit
// everything" from believing it has covered a grid it cannot read.
func interpretGridCell(el directorapi.Element) (directorapi.EntityIdentity, bool) {
	row, hasRow := attrInt(el, "grid_row")
	col, hasCol := attrInt(el, "grid_column")
	index, hasIndex := attrInt(el, "grid_index")
	if !hasRow || !hasCol || !hasIndex {
		return directorapi.EntityIdentity{}, false
	}

	e := directorapi.EntityIdentity{
		Kind: directorapi.EntitySlot,
		// Named by POSITION, because that is the only durable thing about it. A slot
		// called "row 3, column 4" is one a request can refer to and a later frame can
		// recognise; a slot named after a caption nobody read would be neither.
		Name:      fmt.Sprintf("slot %d", index),
		Container: ContainerInventory,
		Slot:      index,
		Source:    Name,
		// Below what a read caption earns. The pack is confident there is a cell here
		// and knows nothing about its contents, and the number says so.
		Confidence: 0.5,
		Evidence: []string{
			fmt.Sprintf("cell (%d,%d) of a %s grid the Director perceived",
				row, col, gridShape(el)),
			"its contents were not readable, so they are reported as unknown rather " +
				"than as empty",
		},
	}
	// Quantity is deliberately left nil. See EntityIdentity.Quantity: an empty slot and
	// a slot nobody could read are different states, and only one of them is safe to act
	// on with "everything".
	if label := strings.TrimSpace(el.Label); label != "" {
		// A label DID arrive — OCR read the caption and fusion merged it into this cell.
		// Now the ordinary stack interpretation applies, and the slot gains a name, a
		// category and a count.
		if stack, ok := interpretStack(label); ok {
			stack.Container = ContainerInventory
			stack.Slot = index
			stack.Source = Name
			stack.Evidence = append(stack.Evidence,
				fmt.Sprintf("in cell (%d,%d) of a perceived grid", row, col))
			return stack, true
		}
		e.Name = label
		e.Category = categoryOf(label)
	}
	return e, true
}

// interpretStack reads a "Wood 43" caption, without needing an element around it.
//
// Split out of interpret so a grid cell can reuse it: a caption means the same thing
// whether it arrived on a list item the accessibility tree reported or on a cell a detector
// found and OCR read.
func interpretStack(label string) (directorapi.EntityIdentity, bool) {
	m := stackPattern.FindStringSubmatch(label)
	if m == nil {
		return directorapi.EntityIdentity{}, false
	}
	name := strings.TrimSpace(m[1])
	if name == "" {
		return directorapi.EntityIdentity{}, false
	}
	count, _ := strconv.Atoi(m[2])
	return directorapi.EntityIdentity{
		Kind: directorapi.EntitySlot, Name: name, Category: categoryOf(name),
		Quantity: directorapi.Quantity(count), Confidence: 0.8,
		Evidence: []string{"a cell reading " + label},
	}, true
}

// gridShape describes the arrangement a cell belongs to, for the evidence line.
func gridShape(el directorapi.Element) string {
	rows, hasRows := attrInt(el, "grid_rows")
	cols, hasCols := attrInt(el, "grid_columns")
	if !hasRows || !hasCols {
		return "perceived"
	}
	return fmt.Sprintf("%d×%d", rows, cols)
}

// attrInt reads an integer attribute, tolerating the shapes JSON round-tripping produces.
//
// An attribute crosses a process boundary as JSON when a world is stored or shipped, and
// comes back as float64. A reader that only accepted int would work in-process and fail
// silently across one — which is the kind of bug that shows up once, in a diagnostic
// nobody trusts afterwards.
func attrInt(el directorapi.Element, key string) (int, bool) {
	switch v := el.Attributes[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}

// containerOf reads which container a control belongs to.
//
// From the element's own parent label, which the accessibility tree supplies. Empty when
// the tree said nothing, and an empty container is what stops this pack from claiming a
// HUD counter belongs to the inventory.
func containerOf(el directorapi.Element) string {
	parent, _ := el.Attributes["parent_label"].(string)
	switch {
	case equalFold(parent, ContainerInventory), equalFold(parent, "Player Inventory"):
		return ContainerInventory
	case equalFold(parent, ContainerStorage), equalFold(parent, "Storage Container"),
		equalFold(parent, "Chest"):
		return ContainerStorage
	case equalFold(parent, ContainerPalbox):
		return ContainerPalbox
	}
	return ""
}

// namedEntity recognises a control that IS a station, container or queue.
func namedEntity(label string) (directorapi.EntityKind, string, bool) {
	switch {
	case equalFold(label, "Workbench"), equalFold(label, "Primitive Workbench"):
		return directorapi.EntityStation, StationWorkbench, true
	case equalFold(label, "Egg Incubator"), equalFold(label, "Incubating Egg"):
		return directorapi.EntityStation, StationIncubator, true
	case equalFold(label, "Feed Box"):
		return directorapi.EntityStation, StationFeedBox, true
	case equalFold(label, "Storage Container"), equalFold(label, "Chest"):
		return directorapi.EntityContainer, ContainerStorage, true
	case equalFold(label, "Palbox"):
		return directorapi.EntityContainer, ContainerPalbox, true
	case equalFold(label, "Crafting Queue"), equalFold(label, "Production Queue"):
		return directorapi.EntityQueue, QueueCrafting, true
	}
	return "", "", false
}

// isMeter reports whether a name is one of the meters this pack models.
func isMeter(name string) bool { return canonicalMeter(name) != "" }

// canonicalMeter maps an observed caption to the pack's own meter name.
func canonicalMeter(name string) string {
	switch {
	case equalFold(name, "Weight"), equalFold(name, "Carry Weight"):
		return MeterWeight
	case equalFold(name, "HP"), equalFold(name, "Health"):
		return MeterHealth
	case equalFold(name, "Hunger"), equalFold(name, "Food"):
		return MeterHunger
	}
	return ""
}

// canonicalStation maps a progress caption to a station name.
func canonicalStation(name string) string {
	switch {
	case strings.Contains(strings.ToLower(name), "incubat"):
		return StationIncubator
	case strings.Contains(strings.ToLower(name), "craft"),
		strings.Contains(strings.ToLower(name), "produc"):
		return QueueCrafting
	}
	return strings.TrimSpace(name)
}

// categoryOf groups an item the way "deposit everything except food" needs.
//
// A TABLE, not a heuristic, and deliberately incomplete: an item this pack has not
// classified gets no category, which means "everything except food" includes it. That is
// the right failure — the alternative is guessing that an unknown item is food and leaving
// it behind.
var categories = map[string]string{
	"berries": CategoryFood, "red berries": CategoryFood, "bread": CategoryFood,
	"baked berries": CategoryFood, "salad": CategoryFood, "pizza": CategoryFood,
	"cake": CategoryFood, "jam-filled bun": CategoryFood, "grilled": CategoryFood,
	"honey": CategoryFood, "milk": CategoryFood, "egg": CategoryFood,
	"cotton candy": CategoryFood, "mushroom": CategoryFood,

	"wood": CategoryMaterial, "stone": CategoryMaterial, "fiber": CategoryMaterial,
	"paldium fragment": CategoryMaterial, "ore": CategoryMaterial, "ingot": CategoryMaterial,
	"coal": CategoryMaterial, "sulfur": CategoryMaterial, "quartz": CategoryMaterial,
	"leather": CategoryMaterial, "wool": CategoryMaterial, "bone": CategoryMaterial,
	"cloth": CategoryMaterial, "nail": CategoryMaterial, "gunpowder": CategoryMaterial,

	"stone pickaxe": CategoryTool, "stone axe": CategoryTool, "pickaxe": CategoryTool,
	"axe": CategoryTool, "hammer": CategoryTool, "torch": CategoryTool,

	"pal sphere": CategorySphere, "mega sphere": CategorySphere,
	"giga sphere": CategorySphere, "hyper sphere": CategorySphere,

	"arrow": CategoryAmmo, "handgun ammo": CategoryAmmo, "rifle ammo": CategoryAmmo,
	"shotgun shells": CategoryAmmo,
}

func categoryOf(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if c, ok := categories[lower]; ok {
		return c
	}
	// A suffix match, so "Red Berries" and "Grilled Lamball" reach the same answer as
	// their base word. Exact first, so a specific entry always wins.
	for key, c := range categories {
		if strings.HasSuffix(lower, key) {
			return c
		}
	}
	return ""
}
