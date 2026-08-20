// Package palworld is the first capability pack: what the Director needs to know to
// understand Palworld's inventory, crafting, storage and Pal management.
//
// It lives OUTSIDE internal/director on purpose. Nothing in the Director imports it — the
// composition root does, exactly as it does for the platform adapters — and
// internal/director/game/boundary_test.go asserts that. The Director does not know this
// package exists; it knows what a capability pack is.
//
// # What is in scope
//
//	Inventory · Crafting · Storage · Incubators · Base UI · Pal management
//
// Not combat. Not movement. Not anything a player does against another player. That is not
// a limitation of what this pack has got round to; it is the whole of what the framework's
// Automation vocabulary can express, so a future edit to this file cannot quietly widen it.
//
// # What this pack is made of
//
// Declarations and semantics. There is no code here that clicks, types, observes a screen
// or runs Marco: procedures are typed directives, conditions read the world, verifiers read
// two worlds, and the observer reads observations another provider already made. Everything
// that touches the game goes through the Director's one pipeline.
//
// # The honest status
//
// The label tables below are written from Palworld's English interface as documented. They
// have NOT been checked against a running game — see docs/director-games.md, Known gaps.
// What is proven is the framework: this pack registers, detects, contributes, and its
// procedures expand into ordinary programs.
package palworld

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Pack is the Palworld capability.
type Pack struct{}

// New returns the pack.
func New() *Pack { return &Pack{} }

var _ game.Capability = (*Pack)(nil)

// Name is the pack identifier, and the owner of everything it registers.
const Name = "palworld"

func (*Pack) Name() string     { return Name }
func (*Pack) Describe() string { return "Palworld" }
func (*Pack) Version() string  { return "v1" }

// Safety is what this pack declares about Palworld.
//
//	Online game · Competitive · Protected · Automation restrictions
//
// Palworld is played solo and on private servers a player runs or joins. It is NOT
// competitive: there is no ranking of players against each other, which is the signal that
// would make ordinary automation a fairness question. It ships no anti-automation measures
// this pack is aware of, so Protected is false — and if that ever changes, setting it true
// here refuses everything, which is the correct direction for that discovery.
//
// Online is TRUE, because a dedicated server is shared with other people, and what the
// framework does with that is make the declaration visible rather than silently assume a
// single-player game.
func (*Pack) Safety() game.Safety {
	return game.Safety{
		Online:      true,
		Competitive: false,
		Protected:   false,
		Permitted: []game.Automation{
			game.AutomationInventory,
			game.AutomationCrafting,
			game.AutomationMenus,
			game.AutomationOrganization,
			game.AutomationAccessibility,
			game.AutomationReminders,
		},
		Note: "Palworld is played solo or on servers a player chooses; this pack covers " +
			"inventory, crafting, storage, incubators and Pal management, and deliberately " +
			"nothing about combat, movement or capture",
	}
}

// Detect recognises Palworld from the process, the title and the interface.
//
// The interface signal is what makes this reliable. "Palbox" and "Technology" are controls
// no other application has, and a window carrying both is Palworld whatever the executable
// has been renamed to by a mod loader.
func (*Pack) Detect(w directorapi.WorldState, p game.Process) game.Detection {
	m := game.NewMatcher()
	m.MatchProcess(p, "Palworld-Win64-Shipping.exe", "Palworld.exe", "Palworld")
	m.MatchTitle(w, "Palworld")
	// The pack's own observer, when it has recognised something. The strongest signal
	// there is: it means the interface was modelled, not merely named.
	m.MatchEntities(w, Name)
	m.MatchLabels(w, "Palbox")
	m.Mode(modeOf(w))
	return m.Result()
}

// modeOf reports which part of the interface is in front.
//
// The pack's own vocabulary, reported and never interpreted by the Director. Read from
// controls the interface only shows in one place, in the order a reader would check.
func modeOf(w directorapi.WorldState) string {
	switch {
	case hasAny(w, "Palbox"):
		return "palbox"
	case hasAny(w, "Technology"):
		return "technology"
	case hasAny(w, "Incubating Egg", "Egg Incubator"):
		return "incubator"
	case hasAny(w, "Deposit All", "Storage Container"):
		return "storage"
	case hasAny(w, "Craft", "Crafting"):
		return "crafting"
	case hasAny(w, "Inventory", "Weight"):
		return "inventory"
	}
	return ""
}

// Interpreters is what this pack makes of the controls the Director perceived.
//
// One interpreter, and it observes NOTHING: it is handed the fused elements and says which
// of them are inventory slots, containers, meters and stations. A pack that needed a new
// SOURCE — a vision model for a game exposing no accessibility tree — would contribute an
// ordinary perception provider at the composition root, and this would turn its output into
// entities just the same.
func (p *Pack) Interpreters() []game.Interpreter {
	return []game.Interpreter{newInterpreter()}
}

// Procedures are the semantic procedures this pack contributes, each declaring what kind
// of automation it is.
func (p *Pack) Procedures() []game.Procedure { return procedures() }

// ControlRoles are the controls this pack names by meaning.
func (p *Pack) ControlRoles() []goal.ContributedRole { return controlRoles() }

// Conditions are the states it can wait for.
//
// Built from the framework's own condition types over this pack's vocabulary: which meter
// is called Weight and which state a completed craft reports are Palworld facts, and the
// shape "a meter is below a threshold" is not.
func (p *Pack) Conditions() []conditions.WorldCondition {
	return []conditions.WorldCondition{
		game.MeterBelow{Meter: MeterHealth, Fraction: 0.3},
		game.MeterAbove{Meter: MeterWeight, Fraction: 0.9},
		game.InventoryFull{},
		game.InventoryHasRoom{},
		game.EntityPresent{Kind: directorapi.EntityContainer},
		game.EntityPresent{Kind: directorapi.EntityStation, Name: StationWorkbench},
		game.EntityState{Kind: directorapi.EntityQueue, Name: QueueCrafting, State: StateComplete},
		game.EntityState{Kind: directorapi.EntityStation, Name: StationIncubator, State: StateReady},
	}
}

// Verifiers contribute evidence the Director cannot produce itself.
func (p *Pack) Verifiers() []verify.EvidenceSource {
	return []verify.EvidenceSource{&evidence{}}
}

// Policies is this pack's own restrictions, beyond the framework's.
//
// None. Everything this pack declares is expressed in its Safety, which the framework
// enforces — a pack rule here would be a second place the same question is answered.
func (p *Pack) Policies() []policy.Rule { return nil }

// ── the pack's vocabulary ─────────────────────────────────────────────────────

// Meters, stations, queues and states this pack models. Constants rather than bare
// strings because a procedure, a condition and an observer all name the same thing, and a
// typo in one of the three would produce a condition that waits forever.
const (
	MeterHealth = "Health"
	MeterWeight = "Weight"
	MeterHunger = "Hunger"

	StationWorkbench = "Workbench"
	StationIncubator = "Egg Incubator"
	StationFeedBox   = "Feed Box"

	QueueCrafting = "Crafting"

	StateComplete = "complete"
	StateReady    = "ready"
	StateWorking  = "working"

	ContainerInventory = "Inventory"
	ContainerStorage   = "Storage"
	ContainerPalbox    = "Palbox"
)

// Categories are the item groupings this pack recognises.
//
// The vocabulary "deposit everything except food" is answered from. Contributed by the
// pack because what counts as food is a Palworld fact; the framework compares strings.
const (
	CategoryFood      = "food"
	CategoryMaterial  = "material"
	CategoryTool      = "tool"
	CategoryWeapon    = "weapon"
	CategoryArmour    = "armour"
	CategoryAmmo      = "ammo"
	CategorySphere    = "sphere"
	CategoryAccessory = "accessory"
)

func hasAny(w directorapi.WorldState, labels ...string) bool {
	for _, el := range w.Elements {
		for _, want := range labels {
			if equalFold(el.Label, want) {
				return true
			}
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 32
		}
		if y >= 'A' && y <= 'Z' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}
