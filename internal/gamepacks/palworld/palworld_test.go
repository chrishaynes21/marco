package palworld_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/internal/gamepacks/palworld"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The first capability pack.
//
// What is under test is the PACK: that it registers, detects what it serves and nothing
// else, models an inventory out of what the Director perceived, and expands into ordinary
// programs. The framework's own contract is tested next door, without a game.
//
// Nothing here has been checked against a running Palworld. The fixtures are what the
// interface is documented to show; see docs/director-games.md, Known gaps.

var t0 = time.Date(2026, 8, 5, 17, 0, 0, 0, time.UTC)

// registered builds a registry with the pack in it, cleaning up the global control-role
// table afterwards so tests do not leak into each other.
func registered(t *testing.T) *game.Registry {
	t.Helper()
	t.Cleanup(func() { goal.ForgetContributedRoles(palworld.Name) })
	reg := game.NewRegistry()
	if err := reg.Register(palworld.New()); err != nil {
		t.Fatalf("register: %v", err)
	}
	return reg
}

// ── detection (Part 15: detection) ────────────────────────────────────────────

func TestThePackDetectsPalworld(t *testing.T) {
	reg := registered(t)
	w := worldWith("Palworld",
		el("e1", "Palbox", directorapi.RoleButton),
		el("e2", "Technology", directorapi.RoleButton))

	active := reg.Detect(w, game.Process{Name: "Palworld-Win64-Shipping.exe"})
	if !active.Detected() {
		t.Fatalf("Palworld was not detected: %+v", active)
	}
	if active.Pack != palworld.Name {
		t.Errorf("detected %q", active.Pack)
	}
	// Comfortably above the threshold, and short of certainty: three signals agreeing is
	// strong evidence, not proof, and a pack that reported 1.0 for it would be claiming
	// something it cannot know.
	if active.Detection.Confidence < 0.8 {
		t.Errorf("confidence = %.2f; process, title and interface all matched",
			active.Detection.Confidence)
	}
	if active.Detection.Confidence >= 1 {
		t.Errorf("confidence = %.2f; no combination of signals is certainty",
			active.Detection.Confidence)
	}
	if len(active.Detection.Evidence) < 3 {
		t.Errorf("the detection records %d evidence clause(s) for three signals: %v",
			len(active.Detection.Evidence), active.Detection.Evidence)
	}
}

func TestThePackDoesNotClaimAnotherApplication(t *testing.T) {
	reg := registered(t)
	w := worldWith("Untitled - Notepad", el("e1", "File", directorapi.RoleMenuItem))

	if active := reg.Detect(w, game.Process{Name: "notepad.exe"}); active.Detected() {
		t.Fatalf("the pack claimed Notepad: %+v", active)
	}
}

func TestThePackReportsWhichPartOfTheInterfaceIsInFront(t *testing.T) {
	reg := registered(t)
	w := worldWith("Palworld", el("e1", "Palbox", directorapi.RoleButton))

	active := reg.Detect(w, game.Process{Name: "Palworld-Win64-Shipping.exe"})
	if active.Detection.Mode != "palbox" {
		t.Errorf("mode = %q, want palbox", active.Detection.Mode)
	}
}

// ── the world (Part 15: world) ────────────────────────────────────────────────

// inventoryWorld is a player inventory as the accessibility tree would report it.
func inventoryWorld() directorapi.WorldState {
	return worldWith("Palworld",
		el("e1", "Inventory", directorapi.RoleList),
		slot("e2", "Wood 43"),
		slot("e3", "Stone 12"),
		slot("e4", "Red Berries 8"),
		slot("e5", "Stone Pickaxe 1"),
		el("e6", "Weight 143/300", directorapi.RoleText),
		el("e7", "Workbench", directorapi.RoleButton),
	)
}

func TestThePackModelsAnInventory(t *testing.T) {
	reg := registered(t)
	w := inventoryWorld()
	reg.Enrich(&w)

	inv := game.ReadInventory(w, palworld.ContainerInventory)
	if inv.Slots != 4 {
		t.Fatalf("%d slots modelled: %+v", inv.Slots, inv.Items)
	}
	if inv.Filled != 4 || inv.Unknown != 0 {
		t.Errorf("filled=%d unknown=%d", inv.Filled, inv.Unknown)
	}
	// Counts, read from the caption.
	if n, ok := find(inv, "Wood"); !ok || n != 43 {
		t.Errorf("Wood = %d (found=%v)", n, ok)
	}
	// Categories, which is what "everything except food" is answered from.
	if sel := inv.Except(palworld.CategoryFood); len(sel.Items) != 3 {
		t.Errorf("everything-except-food selected %d item(s): %+v",
			len(sel.Items), sel.Items)
	}
}

func TestThePackModelsAMeter(t *testing.T) {
	reg := registered(t)
	w := inventoryWorld()
	reg.Enrich(&w)

	level, ok := game.MeterLevel(w, palworld.MeterWeight)
	if !ok {
		t.Fatal("the weight meter was not modelled")
	}
	if level < 0.47 || level > 0.48 {
		t.Errorf("weight = %.3f, want 143/300", level)
	}
}

func TestThePackModelsAStation(t *testing.T) {
	reg := registered(t)
	w := inventoryWorld()
	reg.Enrich(&w)

	stations := game.Stations(w)
	if len(stations) != 1 || stations[0].Name != palworld.StationWorkbench {
		t.Fatalf("stations = %+v", stations)
	}
}

// TestAnUnreadableSlotIsUnknownRatherThanEmpty.
//
// The distinction the whole nil-Quantity design exists for: "deposit everything" that
// silently skipped a slot nobody could read did not deposit everything.
func TestAnUnreadableSlotIsUnknownRatherThanEmpty(t *testing.T) {
	reg := registered(t)
	w := worldWith("Palworld",
		el("e1", "Inventory", directorapi.RoleList),
		slot("e2", "Wood 43"),
		slot("e3", "???"), // a caption with no count
	)
	reg.Enrich(&w)

	inv := game.ReadInventory(w, palworld.ContainerInventory)
	if inv.Unknown != 1 {
		t.Fatalf("unknown=%d, want the unreadable slot counted: %+v", inv.Unknown, inv.Items)
	}
	sel := inv.Everything()
	if sel.Skipped != 1 {
		t.Errorf("everything skipped %d; the unreadable slot must be reported", sel.Skipped)
	}
	if !strings.Contains(sel.Describe(), "could not be read") {
		t.Errorf("the selection does not disclose what it skipped: %s", sel.Describe())
	}
	// And fullness cannot be established while something is unreadable.
	if _, known := inv.Full(); known {
		t.Error("fullness was established despite an unreadable slot")
	}
}

func TestThePackIgnoresControlsItDoesNotRecognise(t *testing.T) {
	reg := registered(t)
	w := worldWith("Palworld",
		el("e1", "Some Button", directorapi.RoleButton),
		el("e2", "A label", directorapi.RoleText),
	)
	reg.Enrich(&w)

	for _, e := range w.Elements {
		if e.Entity != nil {
			t.Errorf("%q was modelled as %s; the pack should not guess",
				e.Label, e.Entity.Describe())
		}
	}
}

// ── procedures (Part 15: procedures) ──────────────────────────────────────────

func TestThePacksProceduresExpandIntoOrdinaryPrograms(t *testing.T) {
	reg := registered(t)
	goals := goal.NewRegistry()
	reg.RegisterProcedures(goals)

	for _, tc := range []struct {
		name string
		g    goal.Goal
	}{
		{"open inventory", goal.Goal{Kind: goal.OpenSettings}},
		{"deposit", goal.Goal{Kind: goal.Move}},
		{"withdraw", goal.Goal{Kind: goal.Copy}},
		{"sort", goal.Goal{Kind: goal.Sort}},
		{"craft", goal.Goal{
			Kind: goal.Craft, Parameters: map[string]string{goal.ParamName: "Arrow"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.g
			g.Context.Application = "Palworld"
			ex, err := goal.Expand(goals, g, binder{})
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if !strings.HasPrefix(ex.Procedure, "palworld") {
				t.Fatalf("procedure = %q, want the pack's", ex.Procedure)
			}
			if len(ex.Program.Steps) == 0 {
				t.Fatal("expanded into no steps")
			}
			// The ordinary validator: if this passes, everything downstream works
			// because it is the same everything.
			if err := program.Validate(ex.Program); err != nil {
				t.Errorf("not an ordinary valid program: %v", err)
			}
		})
	}
}

// TestThePacksProceduresNameControlsByMeaning.
//
// The property that makes a pack work on a German machine: no step carries an English
// label as its identity.
func TestThePacksProceduresNameControlsByMeaning(t *testing.T) {
	reg := registered(t)
	goals := goal.NewRegistry()
	reg.RegisterProcedures(goals)

	g := goal.Goal{Kind: goal.Move, Context: goal.Context{Application: "Palworld"}}
	ex, err := goal.Expand(goals, g, binder{})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	step := ex.Program.Steps[0]
	q := step.Operation.Targets[0].Query
	if q == nil || len(q.AnyLabel) < 2 {
		t.Fatalf("the step names one label rather than a role's aliases: %+v", q)
	}
	// The German alias is among them, which is what naming by meaning buys.
	var german bool
	for _, l := range q.AnyLabel {
		if l == "Alle einlagern" {
			german = true
		}
	}
	if !german {
		t.Errorf("the aliases do not include the German label: %v", q.AnyLabel)
	}
}

// ── conditions (Part 15: conditions) ──────────────────────────────────────────

func TestInventoryFullIsAnsweredFromWhatWasModelled(t *testing.T) {
	reg := registered(t)
	w := inventoryWorld()
	reg.Enrich(&w)

	// Four filled slots and no capacity reported: full, because every slot seen holds
	// something and nothing was unreadable.
	res := game.InventoryFull{Container: palworld.ContainerInventory}.Evaluate(w)
	if res.State != evaluation.Satisfied {
		t.Errorf("state = %s (%s)", res.State, res.Explanation)
	}
}

func TestAConditionOverAWorldWithNoInventoryIsUnknowableNotFalse(t *testing.T) {
	w := worldWith("Palworld", el("e1", "Some Button", directorapi.RoleButton))

	res := game.InventoryFull{}.Evaluate(w)
	if res.State != evaluation.Unknown {
		t.Fatalf("state = %s; an unmodelled inventory is a question, not a no", res.State)
	}
	if !strings.Contains(res.Explanation, "cannot be answered") {
		t.Errorf("the explanation does not say it could not be answered: %s", res.Explanation)
	}
}

func TestCraftCompleteIsAStateNotAGuess(t *testing.T) {
	w := worldWith("Palworld", queue("e1", "Crafting Queue"))
	reg := registered(t)
	reg.Enrich(&w)

	// The queue exists but reports no state, so "is the craft complete?" is unanswerable
	// rather than no.
	res := game.EntityState{
		Kind: directorapi.EntityQueue, Name: palworld.QueueCrafting,
		State: palworld.StateComplete,
	}.Evaluate(w)
	if res.State != evaluation.Unknown {
		t.Errorf("state = %s (%s)", res.State, res.Explanation)
	}
}

// ── verification (Part 15: verification) ──────────────────────────────────────

func TestDepositingIsVerifiedByTheInventoryChanging(t *testing.T) {
	reg := registered(t)
	before := inventoryWorld()
	reg.Enrich(&before)

	after := worldWith("Palworld", el("e1", "Inventory", directorapi.RoleList))
	after.Timestamp = t0.Add(time.Second)
	reg.Enrich(&after)

	ev := packEvidence(t, reg, before, after)
	if !hasEvidence(ev, "inventory_changed") {
		t.Fatalf("the inventory emptying produced no evidence: %+v", ev)
	}
}

func TestCraftingIsVerifiedByTheCountGoingUp(t *testing.T) {
	reg := registered(t)
	before := worldWith("Palworld",
		el("e1", "Inventory", directorapi.RoleList), slot("e2", "Arrow 10"))
	reg.Enrich(&before)

	after := worldWith("Palworld",
		el("e1", "Inventory", directorapi.RoleList), slot("e2", "Arrow 30"))
	after.Timestamp = t0.Add(time.Second)
	reg.Enrich(&after)

	ev := packEvidence(t, reg, before, after)
	if !hasEvidence(ev, "item_count_increased") {
		t.Fatalf("crafting produced no count evidence: %+v", ev)
	}
	for _, e := range ev {
		if e.Kind == "item_count_increased" && !strings.Contains(e.Detail, "10 to 30") {
			t.Errorf("the evidence does not say what changed: %s", e.Detail)
		}
	}
}

// TestEvidenceIsSemanticRatherThanVisual — the milestone's own rule.
func TestEvidenceIsSemanticRatherThanVisual(t *testing.T) {
	reg := registered(t)
	before := inventoryWorld()
	reg.Enrich(&before)
	after := inventoryWorld()
	after.Timestamp = t0.Add(time.Second)
	reg.Enrich(&after)

	for _, e := range packEvidence(t, reg, before, after) {
		for _, forbidden := range []string{"pixel", "screenshot", "colour", "color", "image"} {
			if strings.Contains(strings.ToLower(e.Detail), forbidden) {
				t.Errorf("evidence %q rests on %q: %s", e.Kind, forbidden, e.Detail)
			}
		}
	}
}

func TestAnUnmodelledWorldProducesNoEvidence(t *testing.T) {
	reg := registered(t)
	before := worldWith("Notepad", el("e1", "File", directorapi.RoleMenuItem))
	after := worldWith("Notepad", el("e1", "File", directorapi.RoleMenuItem))
	after.Timestamp = t0.Add(time.Second)

	if ev := packEvidence(t, reg, before, after); len(ev) != 0 {
		t.Errorf("the pack had an opinion about Notepad: %+v", ev)
	}
}

// ── policy (Part 15: policy) ──────────────────────────────────────────────────

func TestThePackPermitsSupportiveAutomationOnly(t *testing.T) {
	s := palworld.New().Safety()
	for _, a := range s.Permitted {
		if !a.Supportive() {
			t.Errorf("the pack permits %q, which the framework does not recognise", a)
		}
	}
	if s.Protected {
		t.Error("the pack declares Palworld protected and also ships procedures")
	}
	// Every procedure it ships falls under something it permits — enforced at
	// registration, asserted here so the intent is visible.
	for _, p := range palworld.New().Procedures() {
		if !s.Permits(p.Automation) {
			t.Errorf("%q is %s automation, which the pack does not permit",
				p.Procedure.Name, p.Automation)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func packEvidence(t *testing.T, reg *game.Registry, before, after directorapi.WorldState) []directorapi.Evidence {
	t.Helper()
	var out []directorapi.Evidence
	for _, src := range reg.Verifiers() {
		out = append(out, src.Evidence(directorapi.ClickAction{},
			directorapi.ResolvedTarget{}, before, after)...)
	}
	return out
}

func hasEvidence(ev []directorapi.Evidence, kind string) bool {
	for _, e := range ev {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

func find(inv game.Inventory, name string) (int, bool) {
	for _, it := range inv.Items {
		if strings.EqualFold(it.Entity.Name, name) {
			return it.Entity.Count()
		}
	}
	return 0, false
}

func worldWith(title string, els ...*directorapi.Element) directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, el := range els {
		m[el.ID] = el
	}
	id := directorapi.WindowID("hwnd:1")
	return directorapi.WorldState{
		Timestamp: t0, Elements: m,
		Windows: []directorapi.Window{{
			ID: id, Title: title, Application: "palworld", Focused: true, Visible: true,
		}},
		ActiveWindow: &id,
		ActiveApp: &directorapi.Application{
			ID: "palworld", Name: title, Executable: "Palworld-Win64-Shipping.exe",
		},
	}
}

func el(id, label string, role directorapi.ElementRole) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: role, WindowID: "hwnd:1",
		Enabled: true, Visible: true, Confidence: 1,
	}
}

// slot is a list item inside the player's inventory.
func slot(id, label string) *directorapi.Element {
	e := el(id, label, directorapi.RoleListItem)
	e.Attributes = map[string]any{"parent_label": "Inventory"}
	return e
}

func queue(id, label string) *directorapi.Element {
	return el(id, label, directorapi.RoleList)
}

// binder resolves a deictic target without a desktop.
type binder struct{}

func (binder) Bind(req goal.BindRequest) (*binding.Binding, *binding.Problem) {
	return &binding.Binding{
		ID: "b1", Phrase: req.Phrase, Expected: req.Expected, Resolved: binding.KindFile,
		ElementID: "e1", NativeID: "uia:1", Resource: "thing", Label: "thing",
		WindowID: "hwnd:1", Application: req.Application, Confidence: 1,
	}, nil
}
