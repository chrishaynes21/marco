// Package game is the Director's extension point for applications with their own
// vocabulary — starting with games, and not limited to them.
//
// The governing rules:
//
//	The Director understands semantics.
//	Game plugins contribute semantics.
//	Marco executes deterministic mechanics.
//	No plugin may bypass the Director.
//	Every mutation still lowers through legal Marco.
//
// # What this package is, and what it refuses to become
//
// It is a REGISTRY and a set of declarations. There is no execution here: nothing in this
// package or reachable from it can click, type, observe, or run Marco. A capability
// contributes five kinds of thing, and every one of them plugs into an extension point
// that already existed:
//
//	Observers   → observation.Provider, run by the ordinary collector, fused by the
//	              ordinary engine. They produce EVIDENCE and nothing else.
//	Procedures  → goal.Procedure, in the ordinary registry, expanded by the ordinary
//	              expander into an ordinary program.
//	Conditions  → conditions.WorldCondition, answered by the ordinary wait engine.
//	Verifiers   → verify.EvidenceSource, weighed alongside the Director's own evidence
//	              and capped so it cannot outvote it.
//	Policies    → policy.Rule, which may only NARROW a decision.
//
// That list is the design. A capability that wanted to do something not on it would be
// asking for a second execution path, and the answer is no — the Director's whole value is
// that one pipeline observes, plans, confirms, lowers to Marco, executes and verifies, and
// a plugin that stepped around any of that would take the guarantees with it.
//
// # The Director stays game-agnostic
//
// Nothing in internal/director imports a capability pack. Packs live in internal/gamepacks
// and are registered by the composition root — the same shape as the platform adapters,
// enforced the same way, by a test. Grep for "palworld" inside internal/director and the
// answer must stay empty.
package game

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"

	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Capability is one application's contribution to the Director's semantics.
//
// Every method may return nothing. A pack that contributes only procedures is a legitimate
// pack, and a pack that contributes only a detection rule is a way of saying "this is
// Palworld and I know nothing else about it" — which is more useful than silence.
type Capability interface {
	// Name identifies the pack ("palworld"). Used in diagnostics, in contributed role
	// names, and as the owner of everything it registers.
	Name() string
	// Describe is what to call it for a person ("Palworld").
	Describe() string
	// Version is the pack's own version, not the game's. What game version a pack
	// believes it is looking at belongs in Detection.
	Version() string

	// Detect reports whether this pack serves what is on screen, and how sure it is.
	Detect(w directorapi.WorldState, p Process) Detection

	// Interpreters say what this pack makes of the controls the Director perceived.
	//
	// NOT perception providers, and the difference is a boundary the Director enforces:
	// observations are EVIDENCE, and only internal/director/perception may see them —
	// which is what lets a new source be added without touching anything that reasons.
	// A pack reasons over BELIEF, like everything else outside perception: it is handed
	// the fused elements and says which of them are inventory slots.
	//
	// The practical consequence is the right one. A pack cannot capture a screen, run a
	// model, or contribute evidence nobody weighed; it annotates what the ordinary
	// pipeline already established. A pack that genuinely needs a new SOURCE — a vision
	// model reading a full-screen game that exposes no accessibility tree — contributes
	// it as an ordinary perception provider wired at the composition root, and this
	// interface is what turns its output into entities.
	Interpreters() []Interpreter
	// Procedures are the semantic procedures it contributes — ordinary goal procedures,
	// each DECLARING which kind of automation it is.
	//
	// The declaration is checked against Safety.Permitted at registration, so a pack
	// cannot ship a crafting procedure in an application where it permitted only
	// reminders. That check is what makes Part 11's distinction structural instead of a
	// promise: an unpermitted procedure stops the Director from starting rather than
	// being discovered when a user asks for it on a live game.
	Procedures() []Procedure
	// ControlRoles are the controls it names by meaning, so a procedure can ask for
	// "the craft button" rather than for an English string.
	ControlRoles() []goal.ContributedRole
	// Conditions are the states it can wait for, as ordinary world conditions.
	Conditions() []conditions.WorldCondition
	// Verifiers contribute evidence the Director cannot produce itself.
	Verifiers() []verify.EvidenceSource
	// Policies are the restrictions this application places on being automated.
	Policies() []policy.Rule

	// Safety is what the pack declares about the application: whether it is online,
	// competitive, protected, and what kinds of automation it permits.
	Safety() Safety
}

// Procedure is a contributed procedure with the automation it declares itself to be.
//
// The pairing is the point. A goal.Procedure says what it DOES; this says what KIND of
// thing doing it is, in the vocabulary a pack's Safety declaration is written in — so the
// two can be checked against each other by something other than a reader's judgement.
type Procedure struct {
	// Procedure is the ordinary procedure, unchanged and unwrapped before it reaches
	// the registry.
	Procedure goal.Procedure
	// Automation is what this procedure is, and must be one the pack permits.
	Automation Automation
}

// Process is what the operating system knows about the foreground application.
//
// Supplied by the composition root, because knowing it requires platform code the Director
// may not import. Every field is optional: a Director with no process source detects from
// the window title and the observations alone, which is weaker and still works.
type Process struct {
	// Name is the process name ("Palworld-Win64-Shipping.exe").
	Name string `json:"name,omitempty"`
	// Executable is the full path, when it is knowable.
	Executable string `json:"executable,omitempty"`
	// PID is the process id, for diagnostics only. Never used for identity: it is
	// reassigned, and a pack that keyed on one would follow the wrong program.
	PID int `json:"pid,omitempty"`
	// Version is the executable's reported version, when the platform can read one.
	Version string `json:"version,omitempty"`
}

// Detection is what a pack concluded about what is on screen.
type Detection struct {
	// Matched reports whether this pack serves what it was shown.
	Matched bool `json:"matched"`
	// Confidence is 0..1. A pack that matched on the executable alone should say so
	// with a lower number than one that also recognised the interface.
	Confidence float64 `json:"confidence"`
	// GameVersion is the version of the APPLICATION, when the pack can tell. Empty is
	// the honest answer for most.
	GameVersion string `json:"game_version,omitempty"`
	// Mode is which part of the application is in front — "inventory", "crafting",
	// "map", "menu". A pack's own vocabulary, reported and never interpreted here.
	Mode string `json:"mode,omitempty"`
	// Evidence is what the conclusion rests on, one clause each. Required for a match:
	// a detection a reader cannot check is a detection they have to trust.
	Evidence []string `json:"evidence,omitempty"`
}

// Safety is what a pack declares about the application it serves.
//
//	Plugins declare: online game, competitive, protected, automation restrictions.
//	The framework should make these policies explicit rather than hiding them.
//
// Declared by the pack and enforced by the framework — see policy.go. A pack that declared
// nothing gets the strictest reading rather than the loosest: an application nobody has
// said anything about is one nothing is known about.
type Safety struct {
	// Online reports that the application talks to a server other people share.
	Online bool `json:"online"`
	// Competitive reports that players are ranked against each other. The strongest
	// signal there is: automation that would be unremarkable alone is not, here.
	Competitive bool `json:"competitive"`
	// Protected reports that the application ships anti-automation measures. Declared
	// so the framework can REFUSE, never so anything can route around one — see the
	// non-goals in docs/director-games.md.
	Protected bool `json:"protected"`
	// Permitted is what this pack says may be automated here. A closed vocabulary; see
	// Automation.
	Permitted []Automation `json:"permitted,omitempty"`
	// Note is the pack's own sentence about what it permits and why, shown in
	// diagnostics and in a refusal.
	Note string `json:"note,omitempty"`
}

// Automation names a kind of thing automation can do, so a pack can permit some and not
// others.
//
// A CLOSED vocabulary, and the closure is the point. A free-form permission list would let
// a pack write "combat: yes" and have the framework carry it, which is precisely what this
// milestone's non-goals rule out.
type Automation string

const (
	// AutomationInventory moves, sorts, deposits and withdraws things the user owns.
	AutomationInventory Automation = "inventory"
	// AutomationCrafting queues and collects work at a station.
	AutomationCrafting Automation = "crafting"
	// AutomationMenus navigates the application's own interface.
	AutomationMenus Automation = "menus"
	// AutomationOrganization renames, sorts, tidies and labels.
	AutomationOrganization Automation = "organization"
	// AutomationAccessibility substitutes for input a player cannot comfortably make.
	AutomationAccessibility Automation = "accessibility"
	// AutomationReminders observes and reports without acting.
	AutomationReminders Automation = "reminders"
)

// supportive is every automation the framework will consider.
//
// The list is exhaustive on purpose. There is no Automation value for combat, aiming,
// movement or trading with other players, so a pack cannot permit one — not because the
// framework would refuse it, but because there is no way to write it down.
var supportive = []Automation{
	AutomationInventory, AutomationCrafting, AutomationMenus,
	AutomationOrganization, AutomationAccessibility, AutomationReminders,
}

// Automations returns the vocabulary.
func Automations() []Automation { return append([]Automation{}, supportive...) }

// Supportive reports whether an automation is one the framework recognises.
func (a Automation) Supportive() bool {
	for _, s := range supportive {
		if s == a {
			return true
		}
	}
	return false
}

// Permits reports whether this pack allows a kind of automation.
func (s Safety) Permits(a Automation) bool {
	for _, p := range s.Permitted {
		if p == a {
			return true
		}
	}
	return false
}

// Describe renders a safety declaration for a person.
func (s Safety) Describe() string {
	var parts []string
	if s.Online {
		parts = append(parts, "online")
	}
	if s.Competitive {
		parts = append(parts, "competitive")
	}
	if s.Protected {
		parts = append(parts, "protected against automation")
	}
	if len(parts) == 0 {
		parts = append(parts, "offline, single-player")
	}
	out := strings.Join(parts, ", ")
	if len(s.Permitted) == 0 {
		return out + "; nothing is permitted"
	}
	names := make([]string, 0, len(s.Permitted))
	for _, p := range s.Permitted {
		names = append(names, string(p))
	}
	return fmt.Sprintf("%s; permits %s", out, strings.Join(names, ", "))
}

// ── the registry ──────────────────────────────────────────────────────────────

// Registry holds the capability packs.
//
// One per Director. Packs are added at composition time and never removed: a capability
// that appeared or vanished mid-session would make one request resolve what the next could
// not, for reasons nothing could show a user.
type Registry struct {
	mu    sync.RWMutex
	packs []Capability
}

// NewRegistry returns an empty registry.
//
// EMPTY, and that is the Director's ordinary state. A Director with no packs behaves in
// every respect as it did before this package existed, which is what makes games an
// extension rather than a mode.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a pack and everything it contributes that is registered globally.
//
// The control roles are registered here rather than by the caller, because a pack whose
// procedures name roles nothing knows about would expand into steps that resolve nothing —
// discovered by the first user to ask, mid-request. Registering them together means a pack
// is either usable or refused.
func (r *Registry) Register(c Capability) error {
	if c == nil {
		return fmt.Errorf("game: a nil capability cannot be registered")
	}
	name := strings.TrimSpace(c.Name())
	if name == "" {
		return fmt.Errorf("game: a capability pack needs a name")
	}

	r.mu.Lock()
	for _, existing := range r.packs {
		if strings.EqualFold(existing.Name(), name) {
			r.mu.Unlock()
			return fmt.Errorf("game: %q is already registered", name)
		}
	}
	r.mu.Unlock()

	if err := validateSafety(c); err != nil {
		return err
	}
	for _, role := range c.ControlRoles() {
		if role.Owner == "" {
			role.Owner = name
		}
		if role.Owner != name {
			return fmt.Errorf("game: %q contributes a control role owned by %q",
				name, role.Owner)
		}
		if err := goal.RegisterControlRole(role); err != nil {
			return fmt.Errorf("game: %q: %w", name, err)
		}
	}

	r.mu.Lock()
	r.packs = append(r.packs, c)
	r.mu.Unlock()
	return nil
}

// validateSafety refuses a pack whose declaration the framework cannot honour.
//
// Checked at REGISTRATION, so a pack that permits something the vocabulary does not
// contain stops the Director from starting rather than being discovered when a user asks
// for it on a live game.
func validateSafety(c Capability) error {
	s := c.Safety()
	for _, p := range s.Permitted {
		if !p.Supportive() {
			return fmt.Errorf("game: %q permits %q, which is not a kind of automation "+
				"this framework recognises", c.Name(), p)
		}
	}
	if s.Protected && len(s.Permitted) > 0 {
		// A pack cannot declare an application protected and then permit automation in
		// it. The two statements contradict each other, and the safe reading of a
		// contradiction is the strict one — so it is refused rather than resolved.
		return fmt.Errorf("game: %q declares %s protected against automation and also "+
			"permits automation in it; those cannot both be true",
			c.Name(), c.Describe())
	}

	// Every procedure must fall under something the pack permitted. This is where Part
	// 11 stops being a promise: a pack that permits inventory and crafting cannot ship a
	// procedure declaring itself anything else, and one that permits nothing cannot ship
	// a procedure at all.
	for _, p := range c.Procedures() {
		switch {
		case p.Automation == "":
			return fmt.Errorf("game: %q contributes %q without saying what kind of "+
				"automation it is", c.Name(), p.Procedure.Name)
		case !p.Automation.Supportive():
			return fmt.Errorf("game: %q contributes %q as %q, which is not a kind of "+
				"automation this framework recognises",
				c.Name(), p.Procedure.Name, p.Automation)
		case !s.Permits(p.Automation):
			return fmt.Errorf("game: %q contributes %q as %s automation, which it does "+
				"not permit in %s", c.Name(), p.Procedure.Name, p.Automation, c.Describe())
		}
	}
	return nil
}

// Packs lists what is registered, by name.
func (r *Registry) Packs() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]Capability{}, r.packs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Find returns a pack by name.
func (r *Registry) Find(name string) (Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.packs {
		if strings.EqualFold(p.Name(), name) {
			return p, true
		}
	}
	return nil, false
}

// Len is how many packs are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.packs)
}

// Interpreter says what a capability pack makes of one perceived control.
//
// Handed an Element the fusion engine produced, and returns the entity it stands for in
// the application's own model — or false, which is the answer for most of a window.
//
// It cannot observe, cannot act, and cannot see where the element came from. That last
// point is the boundary: an interpreter that could ask "was this OCR or accessibility?"
// would make packs sensitive to how a thing was seen, and adding a source would then mean
// revisiting every pack.
type Interpreter interface {
	// Name identifies the pack that contributed it, and becomes the entity's Source.
	Name() string
	// Interpret reports what this element is, and false when the pack does not
	// recognise it.
	Interpret(el directorapi.Element) (directorapi.EntityIdentity, bool)
}

// Interpreters is every contributed interpreter.
//
// From ALL packs rather than only the detected one, deliberately: an interpreter decides
// for itself whether it recognises an element, and a Director that ran only the detected
// pack's interpreters could never detect a game BY its interface — the recognition and the
// detection would each be waiting for the other.
func (r *Registry) Interpreters() []Interpreter {
	var out []Interpreter
	for _, p := range r.Packs() {
		out = append(out, p.Interpreters()...)
	}
	return out
}

// Enrich annotates a world with what the packs make of it.
//
//	Plugins may enrich World State. These become ordinary semantic entities.
//
// Called once per observation, after fusion. It adds nothing to the world but ENTITIES: no
// element is created, removed, relabelled or re-identified, and an element a pack does not
// recognise is returned exactly as it arrived.
//
// The FIRST pack to recognise an element wins, and a later one cannot overwrite it — the
// same rule fusion applies to resources, for the same reason: two packs disagreeing about
// what a control is would be a disagreement to refuse rather than to resolve by order.
func (r *Registry) Enrich(w *directorapi.WorldState) {
	if w == nil {
		return
	}
	interpreters := r.Interpreters()
	if len(interpreters) == 0 {
		return
	}
	for _, el := range w.Elements {
		if el == nil || el.Entity != nil {
			continue
		}
		for _, in := range interpreters {
			entity, ok := in.Interpret(*el)
			if !ok {
				continue
			}
			if entity.Source == "" {
				entity.Source = in.Name()
			}
			el.Entity = entity.Clone()
			break
		}
	}
}

// Procedures is every contributed procedure, unwrapped for the goal registry.
//
// Unwrapped, so what reaches the registry is an ordinary goal.Procedure and nothing
// downstream can tell it came from a pack. The automation declaration has already done its
// work by this point — it was checked at registration.
func (r *Registry) Procedures() []goal.Procedure {
	var out []goal.Procedure
	for _, p := range r.Packs() {
		for _, proc := range p.Procedures() {
			out = append(out, proc.Procedure)
		}
	}
	return out
}

// DeclaredProcedures is every contributed procedure WITH its declaration, for diagnostics.
func (r *Registry) DeclaredProcedures() []Procedure {
	var out []Procedure
	for _, p := range r.Packs() {
		out = append(out, p.Procedures()...)
	}
	return out
}

// Conditions is every contributed condition.
func (r *Registry) Conditions() []conditions.WorldCondition {
	var out []conditions.WorldCondition
	for _, p := range r.Packs() {
		out = append(out, p.Conditions()...)
	}
	return out
}

// Verifiers is every contributed evidence source.
func (r *Registry) Verifiers() []verify.EvidenceSource {
	var out []verify.EvidenceSource
	for _, p := range r.Packs() {
		out = append(out, p.Verifiers()...)
	}
	return out
}

// Policies is every contributed rule, plus the framework's own.
//
// The framework's rule goes FIRST, so an application declared protected or competitive is
// refused before a pack's own more permissive rule is consulted. A pack cannot widen what
// the framework allows — see policy.go — but ordering it first means the reason the user
// sees is the strongest one.
func (r *Registry) Policies() []policy.Rule {
	out := []policy.Rule{r.SafetyRule()}
	for _, p := range r.Packs() {
		out = append(out, p.Policies()...)
	}
	return out
}

// RegisterProcedures adds every contributed procedure to a goal registry.
//
// Validated afterwards by the caller: a pack that shadowed another pack's procedure is a
// build-time fault, and NewValidatedRegistry is where that is caught.
func (r *Registry) RegisterProcedures(reg *goal.Registry) {
	for _, p := range r.Procedures() {
		reg.Register(p)
	}
}

// ── detection ─────────────────────────────────────────────────────────────────

// Active is what the Director currently believes it is looking at.
type Active struct {
	// Pack is the capability serving the foreground application, empty when none does.
	Pack string `json:"pack,omitempty"`
	// Describe is the application's name for a person.
	Application string `json:"application,omitempty"`
	// Detection is what the pack concluded.
	Detection Detection `json:"detection"`
	// Safety is what that pack declares.
	Safety Safety `json:"safety"`
	// Considered lists every pack that was asked and what it said, so "why did it not
	// detect my game?" has an answer.
	Considered []Considered `json:"considered,omitempty"`
}

// Considered is one pack's answer during detection.
type Considered struct {
	Pack       string   `json:"pack"`
	Matched    bool     `json:"matched"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}

// Detected reports whether a pack serves what is in front.
func (a Active) Detected() bool { return a.Pack != "" }

// Detect asks every pack what it makes of the foreground, and picks the most confident.
//
// A tie REFUSES rather than choosing: two packs equally sure they serve the same window is
// a question about the packs, and picking one by registration order is how a user's
// Minecraft gets Palworld's procedures.
func (r *Registry) Detect(w directorapi.WorldState, p Process) Active {
	out := Active{Application: applicationOf(w)}

	best, ties := -1.0, 0
	var winner Capability
	for _, pack := range r.Packs() {
		d := pack.Detect(w, p)
		out.Considered = append(out.Considered, Considered{
			Pack: pack.Name(), Matched: d.Matched,
			Confidence: d.Confidence, Evidence: d.Evidence,
		})
		if !d.Matched {
			continue
		}
		switch {
		case d.Confidence > best:
			best, ties, winner = d.Confidence, 1, pack
			out.Detection = d
		case d.Confidence == best:
			ties++
		}
	}
	if winner == nil || ties > 1 {
		if ties > 1 {
			out.Detection = Detection{
				Evidence: []string{fmt.Sprintf(
					"%d packs are equally sure they serve this window, so none was chosen",
					ties)},
			}
		}
		return out
	}
	out.Pack = winner.Name()
	out.Application = winner.Describe()
	out.Safety = winner.Safety()
	return out
}

// applicationOf is the app in front, empty when nothing has been observed.
func applicationOf(w directorapi.WorldState) string {
	if w.ActiveApp == nil {
		return ""
	}
	return w.ActiveApp.ID
}
