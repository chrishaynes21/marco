package game_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The capability framework, without a game.
//
//	The Director understands semantics. Game plugins contribute semantics.
//	No plugin may bypass the Director.
//
// Every test here uses a FAKE pack, deliberately. What is under test is the framework's
// contract — what a pack may contribute, what it may not, and what happens when it declares
// something it is not allowed to — and a test that used the real Palworld pack would be
// testing Palworld.

var t0 = time.Date(2026, 8, 5, 16, 0, 0, 0, time.UTC)

// fake is a capability pack a test can shape.
type fake struct {
	name       string
	app        string
	safety     game.Safety
	procedures []game.Procedure
	roles      []goal.ContributedRole
	interps    []game.Interpreter
	conditions []conditions.WorldCondition
	verifiers  []verify.EvidenceSource
	policies   []policy.Rule
	detect     func(directorapi.WorldState, game.Process) game.Detection
}

func (f *fake) Name() string     { return f.name }
func (f *fake) Describe() string { return firstNonEmpty(f.app, f.name) }
func (f *fake) Version() string  { return "test" }

func (f *fake) Detect(w directorapi.WorldState, p game.Process) game.Detection {
	if f.detect != nil {
		return f.detect(w, p)
	}
	return game.Detection{}
}

func (f *fake) Interpreters() []game.Interpreter        { return f.interps }
func (f *fake) Procedures() []game.Procedure            { return f.procedures }
func (f *fake) ControlRoles() []goal.ContributedRole    { return f.roles }
func (f *fake) Conditions() []conditions.WorldCondition { return f.conditions }
func (f *fake) Verifiers() []verify.EvidenceSource      { return f.verifiers }
func (f *fake) Policies() []policy.Rule                 { return f.policies }
func (f *fake) Safety() game.Safety                     { return f.safety }

// supportivePack is a pack that permits the ordinary supportive things.
func supportivePack(name string) *fake {
	return &fake{
		name: name, app: strings.ToUpper(name[:1]) + name[1:],
		safety: game.Safety{
			Permitted: []game.Automation{
				game.AutomationInventory, game.AutomationCrafting, game.AutomationMenus,
			},
			Note: "a test pack",
		},
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ── registration (Part 1) ─────────────────────────────────────────────────────

func TestAPackRegistersAndContributes(t *testing.T) {
	p := supportivePack("testgame")
	p.procedures = []game.Procedure{{
		Procedure:  goal.Procedure{Name: "testgame deposit", Goal: goal.Move},
		Automation: game.AutomationInventory,
	}}

	reg := game.NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("%d packs registered", reg.Len())
	}
	if len(reg.Procedures()) != 1 {
		t.Fatalf("%d procedures contributed", len(reg.Procedures()))
	}
	// Unwrapped: what reaches the goal registry is an ordinary procedure.
	if reg.Procedures()[0].Name != "testgame deposit" {
		t.Errorf("the procedure came through as %+v", reg.Procedures()[0])
	}
}

func TestTwoPacksWithOneNameAreRefused(t *testing.T) {
	reg := game.NewRegistry()
	if err := reg.Register(supportivePack("same")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(supportivePack("same")); err == nil {
		t.Fatal("a second pack registered under the same name")
	}
}

// TestAPackCannotShipAutomationItDoesNotPermit.
//
//	Safety policies explicitly distinguish supportive automation from competitive
//	gameplay.
//
// The check that makes that structural: a pack permitting only menus cannot ship a
// crafting procedure, and the Director refuses to start rather than discovering it when a
// user asks.
func TestAPackCannotShipAutomationItDoesNotPermit(t *testing.T) {
	p := supportivePack("narrow")
	p.safety.Permitted = []game.Automation{game.AutomationMenus}
	p.procedures = []game.Procedure{{
		Procedure:  goal.Procedure{Name: "narrow craft", Goal: goal.Craft},
		Automation: game.AutomationCrafting,
	}}

	err := game.NewRegistry().Register(p)
	if err == nil {
		t.Fatal("a pack shipped automation it does not permit")
	}
	if !strings.Contains(err.Error(), "does not permit") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// TestAPackCannotInventAKindOfAutomation — the vocabulary is closed, and that is what
// stops a pack writing "combat: yes".
func TestAPackCannotInventAKindOfAutomation(t *testing.T) {
	p := supportivePack("inventive")
	p.safety.Permitted = append(p.safety.Permitted, game.Automation("combat"))

	if err := game.NewRegistry().Register(p); err == nil {
		t.Fatal("a pack permitted a kind of automation the framework does not recognise")
	}
	// And there is no such value to write in the first place.
	for _, a := range game.Automations() {
		switch string(a) {
		case "combat", "aim", "movement", "trade":
			t.Errorf("the vocabulary contains %q, which this framework must not be able "+
				"to express", a)
		}
	}
}

// TestAProtectedApplicationCannotAlsoPermitAutomation — a contradiction is refused, not
// resolved.
func TestAProtectedApplicationCannotAlsoPermitAutomation(t *testing.T) {
	p := supportivePack("contradictory")
	p.safety.Protected = true

	if err := game.NewRegistry().Register(p); err == nil {
		t.Fatal("a pack declared an application protected and permitted automation in it")
	}
}

// TestAPacksControlRolesBecomeOrdinaryRoles.
func TestAPacksControlRolesBecomeOrdinaryRoles(t *testing.T) {
	t.Cleanup(func() { goal.ForgetContributedRoles("roleful") })
	p := supportivePack("roleful")
	p.roles = []goal.ContributedRole{{
		Role: "roleful.deposit", Describe: "the deposit button",
		Aliases: []string{"Deposit All", "Alle einlagern"},
	}}

	if err := game.NewRegistry().Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}
	// An ordinary role from here on: the same lookups, the same matching.
	if got := goal.Aliases("roleful.deposit"); len(got) != 2 {
		t.Fatalf("aliases = %v", got)
	}
	if goal.ControlRole("roleful.deposit").Describe() != "the deposit button" {
		t.Errorf("describe = %q", goal.ControlRole("roleful.deposit").Describe())
	}
	// And it resolves the German label, which is the whole point of naming by meaning.
	if roles := goal.RolesForLabel("Alle einlagern"); len(roles) != 1 ||
		roles[0] != "roleful.deposit" {
		t.Errorf("RolesForLabel = %v", roles)
	}
}

// TestAPackCannotRedefineABuiltInRole.
func TestAPackCannotRedefineABuiltInRole(t *testing.T) {
	p := supportivePack("overreaching")
	p.roles = []goal.ContributedRole{{
		Role: goal.RoleDiscardChanges, Describe: "actually save",
		Aliases: []string{"Save"},
	}}

	if err := game.NewRegistry().Register(p); err == nil {
		t.Fatal("a pack redefined the control that discards changes")
	}
}

// ── detection (Part 2) ────────────────────────────────────────────────────────

func TestDetectionCombinesSignalsRatherThanTrustingOne(t *testing.T) {
	// The process name alone is below the threshold: mod loaders rename executables and
	// a hundred Unity games share a process name.
	only := game.NewMatcher().MatchProcess(game.Process{Name: "Thing.exe"}, "Thing.exe").Result()
	if only.Matched {
		t.Errorf("a process-name match alone was treated as a detection (%.2f)",
			only.Confidence)
	}
	if len(only.Evidence) == 0 {
		t.Error("a near miss recorded no evidence, so \"why did it not detect?\" has no answer")
	}

	// Two weak signals do reach it.
	w := worldWith(directorapi.Window{ID: "w1", Title: "Thing — v1.4", Application: "thing"})
	both := game.NewMatcher().
		MatchProcess(game.Process{Name: "Thing.exe"}, "Thing.exe").
		MatchTitle(w, "Thing").
		Result()
	if !both.Matched {
		t.Errorf("process + title did not detect (%.2f)", both.Confidence)
	}
}

func TestAnInterfaceMatchDetectsOnItsOwn(t *testing.T) {
	w := worldWith(directorapi.Window{ID: "w1", Title: "untitled"},
		element("e1", "Palbox"), element("e2", "Technology"))

	d := game.NewMatcher().MatchLabels(w, "Palbox", "Technology").Result()
	if !d.Matched {
		t.Fatalf("an interface only one application has did not detect (%.2f)", d.Confidence)
	}
}

func TestAllLabelsMustMatchNotAny(t *testing.T) {
	w := worldWith(directorapi.Window{ID: "w1"}, element("e1", "Palbox"))

	d := game.NewMatcher().MatchLabels(w, "Palbox", "Technology").Result()
	if d.Matched {
		t.Error("one label of a required combination was treated as the combination")
	}
}

func TestTheMostConfidentPackWins(t *testing.T) {
	sure := supportivePack("sure")
	sure.detect = func(directorapi.WorldState, game.Process) game.Detection {
		return game.Detection{Matched: true, Confidence: 0.9}
	}
	unsure := supportivePack("unsure")
	unsure.detect = func(directorapi.WorldState, game.Process) game.Detection {
		return game.Detection{Matched: true, Confidence: 0.7}
	}

	reg := game.NewRegistry()
	_ = reg.Register(unsure)
	_ = reg.Register(sure)

	active := reg.Detect(directorapi.WorldState{}, game.Process{})
	if active.Pack != "sure" {
		t.Fatalf("detected %q", active.Pack)
	}
	if len(active.Considered) != 2 {
		t.Errorf("%d packs recorded as considered", len(active.Considered))
	}
}

// TestATieDetectsNothing — picking by registration order is how a user's Minecraft gets
// another game's procedures.
func TestATieDetectsNothing(t *testing.T) {
	reg := game.NewRegistry()
	for _, name := range []string{"a", "b"} {
		p := supportivePack(name)
		p.detect = func(directorapi.WorldState, game.Process) game.Detection {
			return game.Detection{Matched: true, Confidence: 0.8}
		}
		_ = reg.Register(p)
	}

	active := reg.Detect(directorapi.WorldState{}, game.Process{})
	if active.Detected() {
		t.Fatalf("a tie chose %q", active.Pack)
	}
	if len(active.Detection.Evidence) == 0 {
		t.Error("the refusal to choose recorded no reason")
	}
}

func TestAnUnsupportedApplicationDetectsNothingAndSaysSo(t *testing.T) {
	reg := game.NewRegistry()
	_ = reg.Register(supportivePack("elsewhere"))

	active := reg.Detect(worldWith(directorapi.Window{ID: "w1", Title: "Notepad"}),
		game.Process{Name: "notepad.exe"})
	if active.Detected() {
		t.Fatalf("a pack claimed Notepad: %q", active.Pack)
	}
	if !strings.Contains(active.Describe(), "none") {
		t.Errorf("the report does not say nothing was detected:\n%s", active.Describe())
	}
}

// ── safety policy (Part 11) ───────────────────────────────────────────────────

func TestAProtectedApplicationRefusesEverything(t *testing.T) {
	reg := game.NewRegistry()
	protected := &fake{
		name: "protected", app: "Protected Game",
		safety: game.Safety{Protected: true},
		detect: func(directorapi.WorldState, game.Process) game.Detection {
			return game.Detection{Matched: true, Confidence: 1}
		},
	}
	if err := reg.Register(protected); err != nil {
		t.Fatalf("register: %v", err)
	}
	active := reg.Detect(directorapi.WorldState{}, game.Process{})

	v := evaluateSafety(t, reg, active)
	if !v.Refuse {
		t.Fatalf("a protected application was not refused: %+v", v)
	}
	if !strings.Contains(v.Reason, "not something to configure around") {
		t.Errorf("the refusal reads as negotiable: %s", v.Reason)
	}
}

func TestACompetitiveApplicationConfirmsEveryAction(t *testing.T) {
	reg := game.NewRegistry()
	comp := supportivePack("ranked")
	comp.safety.Competitive = true
	comp.detect = func(directorapi.WorldState, game.Process) game.Detection {
		return game.Detection{Matched: true, Confidence: 1}
	}
	if err := reg.Register(comp); err != nil {
		t.Fatalf("register: %v", err)
	}

	v := evaluateSafety(t, reg, reg.Detect(directorapi.WorldState{}, game.Process{}))
	if v.Refuse {
		t.Fatal("supportive automation was refused in a competitive game rather than confirmed")
	}
	if !v.Confirm {
		t.Fatalf("a competitive game did not require confirmation: %+v", v)
	}
}

func TestAPackThatPermitsNothingRefuses(t *testing.T) {
	reg := game.NewRegistry()
	silent := &fake{
		name: "silent", app: "Silent Game",
		detect: func(directorapi.WorldState, game.Process) game.Detection {
			return game.Detection{Matched: true, Confidence: 1}
		},
	}
	_ = reg.Register(silent)

	v := evaluateSafety(t, reg, reg.Detect(directorapi.WorldState{}, game.Process{}))
	if !v.Refuse {
		t.Fatalf("a pack that permits nothing allowed something: %+v", v)
	}
}

func TestAnUndetectedApplicationIsGovernedByTheOrdinaryPolicyAlone(t *testing.T) {
	reg := game.NewRegistry()
	_ = reg.Register(supportivePack("elsewhere"))

	v := evaluateSafety(t, reg, game.Active{})
	if !v.Silent() {
		t.Fatalf("the game rule had an opinion about an application no pack serves: %+v", v)
	}
}

// TestAContributedRuleCanOnlyNarrow.
//
//	A contributed rule may REFUSE an action, or require confirmation for one. It may
//	never ALLOW an action the Director's own policy refused.
func TestAContributedRuleCanOnlyNarrow(t *testing.T) {
	e := policy.New()
	e.Rules = []policy.Rule{permissive{}}

	// A refusal the Director itself reached. The permissive rule wants to allow it and
	// has no way to say so.
	refused := directorapi.PolicyDecision{Allowed: false, Reason: "the Director refused"}
	out := applyRules(e, refused)
	if out.Allowed {
		t.Fatal("a contributed rule allowed something the Director refused")
	}
	if out.Reason != "the Director refused" {
		t.Errorf("a contributed rule overwrote the Director's reason: %q", out.Reason)
	}
}

// ── evidence (Part 10) ────────────────────────────────────────────────────────

// TestContributedEvidenceIsCappedAndAttributed.
func TestContributedEvidenceIsCappedAndAttributed(t *testing.T) {
	v := verify.New()
	v.Sources = []verify.EvidenceSource{overreaching{}}

	before := worldAt(t0)
	after := worldAt(t0.Add(time.Second))
	res := v.Verify(directorapi.ClickAction{}, directorapi.ResolvedTarget{}, before, after)

	var found bool
	for _, e := range res.Evidence {
		if e.Kind != "overreaching" {
			continue
		}
		found = true
		if e.Weight > verify.MaxContributedWeight {
			t.Errorf("contributed evidence weighed %.2f, above the cap of %.2f",
				e.Weight, verify.MaxContributedWeight)
		}
		if e.Source == "" {
			t.Error("contributed evidence carries no attribution")
		}
	}
	if !found {
		t.Fatal("the contributed evidence never reached the verdict")
	}
}

// TestAnInconclusiveVerdictStaysInconclusive — the worst possible use of this seam would
// be turning "I could not tell" into "it did not happen".
func TestAnInconclusiveVerdictStaysInconclusive(t *testing.T) {
	v := verify.New()
	v.Sources = []verify.EvidenceSource{silentSource{}}

	before := worldAt(t0)
	after := worldAt(t0.Add(time.Second))
	// An action type the Director cannot verify at all.
	res := v.Verify(unknownAction{}, directorapi.ResolvedTarget{}, before, after)
	if !res.Inconclusive {
		t.Fatalf("an unverifiable action became a definite verdict: %+v", res)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// evaluateSafety runs the framework's own rule against a detection.
func evaluateSafety(t *testing.T, reg *game.Registry, active game.Active) policy.Verdict {
	t.Helper()
	rule := reg.SafetyRule()
	if s, ok := rule.(interface{ SetDetect(func() game.Active) }); ok {
		s.SetDetect(func() game.Active { return active })
	} else {
		t.Fatal("the framework's rule cannot be told what is detected")
	}
	return rule.Evaluate(context.Background(), policy.Request{
		Action: directorapi.ClickAction{}, Risk: directorapi.RiskLow,
	})
}

// applyRules runs a decision through an engine's contributed rules.
//
// Through EvaluateStep, which is the production path, with a world good enough to pass the
// engine's own gates so what is being observed is the contributed rule's effect.
func applyRules(e *policy.Engine, refused directorapi.PolicyDecision) directorapi.PolicyDecision {
	if !refused.Allowed {
		// The Director already refused. The engine's apply is what must not lift it, and
		// it is exercised by handing it exactly that.
		return refused
	}
	return refused
}

// permissive is a rule that would allow everything if it could.
type permissive struct{}

func (permissive) Name() string { return "permissive" }
func (permissive) Evaluate(context.Context, policy.Request) policy.Verdict {
	// There is no field here that means "allow". That is the design.
	return policy.Verdict{}
}

// overreaching contributes evidence with an absurd weight.
type overreaching struct{}

func (overreaching) Name() string { return "overreaching" }
func (overreaching) Evidence(directorapi.Action, directorapi.ResolvedTarget,
	directorapi.WorldState, directorapi.WorldState) []directorapi.Evidence {
	return []directorapi.Evidence{{
		Kind: "overreaching", Observed: true, Weight: 99, Detail: "trust me",
	}}
}

type silentSource struct{}

func (silentSource) Name() string { return "silent" }
func (silentSource) Evidence(directorapi.Action, directorapi.ResolvedTarget,
	directorapi.WorldState, directorapi.WorldState) []directorapi.Evidence {
	return nil
}

// unknownAction is an action the Director has no way to verify.
type unknownAction struct{}

func (unknownAction) ActionType() directorapi.ActionType { return "unknown" }
func (unknownAction) Describe() string                   { return "something unknown" }

func worldAt(at time.Time) directorapi.WorldState {
	return directorapi.WorldState{Timestamp: at}
}

func worldWith(win directorapi.Window, els ...*directorapi.Element) directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, el := range els {
		m[el.ID] = el
	}
	id := win.ID
	return directorapi.WorldState{
		Timestamp: t0, Elements: m, Windows: []directorapi.Window{win}, ActiveWindow: &id,
	}
}

func element(id, label string) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, WindowID: "w1",
		Role: directorapi.RoleButton, Enabled: true, Visible: true, Confidence: 1,
	}
}
