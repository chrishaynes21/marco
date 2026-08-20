package main

import (
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/internal/gamepacks/palworld"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Capability packs, wired at the composition root.
//
// This file is the ONLY place in the program that names a game. internal/director defines
// what a capability pack is; internal/gamepacks/palworld is one; and the two meet here,
// exactly as the Director and the platform adapters do. internal/director/boundary_test.go
// fails the build if that ever stops being true.
//
// Adding a game is adding a line to registeredPacks and nothing else. That is the whole
// claim of the milestone, and it is checkable: if adding a game requires editing anything
// under internal/director, the framework has not done its job.

// registeredPacks is every capability pack this build ships.
func registeredPacks() []game.Capability {
	return []game.Capability{
		palworld.New(),
	}
}

// newGameRegistry builds the registry and registers what this build ships.
//
// A registration failure STOPS the service, for the reason the procedure registry's
// validation does: a pack that contributes a role colliding with another's, or a procedure
// declaring automation it does not permit, is a build-time fault. Discovering it when a
// user asks for that procedure — mid-request, on a live game — is the outcome this
// prevents.
func newGameRegistry() (*game.Registry, error) {
	reg := game.NewRegistry()
	for _, pack := range registeredPacks() {
		if err := reg.Register(pack); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// gameState holds what the Director currently believes it is looking at.
//
// Cached with its own lock, not the command lock, because every policy decision reads it
// and `director game` must answer while a command runs. Refreshed from the world each
// observation, which is free: detection is pure computation over a world that was going to
// be built anyway.
type gameState struct {
	mu     sync.RWMutex
	active game.Active
	at     time.Time
}

// set records a fresh detection.
func (g *gameState) set(a game.Active, at time.Time) {
	g.mu.Lock()
	g.active, g.at = a, at
	g.mu.Unlock()
}

// get returns what is currently detected.
func (g *gameState) get() game.Active {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.active
}

// detect re-runs detection against a world.
//
// Called on every observation. A pack's Detect is a comparison over labels and a process
// record, so this costs nothing measurable — and detection that lagged behind the screen
// would apply one game's safety declaration to another's window.
func (r *Runtime) detect(w *directorapi.WorldState) {
	if r.games == nil || w == nil {
		return
	}
	r.gameState.set(r.games.Detect(*w, r.foregroundProcess(w)), w.Timestamp)
}

// foregroundProcess is what the platform knows about the application in front.
//
// From the ACTIVE APPLICATION the window-system provider reported, which is where the
// process identity lives. A Director whose provider reports none detects from the title
// and the interface alone, which is weaker and still works — see game.Matcher.
func (r *Runtime) foregroundProcess(w *directorapi.WorldState) game.Process {
	if w == nil || w.ActiveApp == nil {
		return game.Process{}
	}
	app := *w.ActiveApp
	return game.Process{
		Name:       baseName(firstNonEmptyStr(app.Executable, app.ID)),
		Executable: app.Executable,
		PID:        app.ProcessID,
	}
}

// baseName is the last path element, so a full executable path and a bare name compare
// alike.
func baseName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		return s[i+1:]
	}
	return s
}

// ── the control plane ─────────────────────────────────────────────────────────
//
// Every method below takes the game state's own lock and never r.mu, so `director game`
// and `director explain inventory` answer while a command is running. See lockrule_test.go.

// DetectedGame is what the Director believes it is looking at.
func (r *Runtime) DetectedGame() game.Active {
	if r.games == nil {
		return game.Active{}
	}
	return r.gameState.get()
}

// GameCapabilities is what every registered pack contributes.
func (r *Runtime) GameCapabilities() game.Report {
	if r.games == nil {
		return game.Report{}
	}
	return r.games.Report(r.gameState.get())
}

// GameInventory is what the Director can see of what the player holds.
//
// From the LAST OBSERVED world rather than by observing afresh, for the reason
// activeApplication reads the same field: a diagnostic that attached an observation cycle
// to every call would make asking a question change the thing being asked about.
func (r *Runtime) GameInventory(container string) game.InventoryReport {
	if r.games == nil {
		return game.InventoryReport{
			Unavailable: "this Director has no capability packs registered",
		}
	}
	r.diagMu.RLock()
	w := r.lastWorld
	r.diagMu.RUnlock()
	if w == nil {
		return game.InventoryReport{
			Unavailable: "the Director has not observed anything yet",
		}
	}
	return r.games.ReadInventoryReport(*w, r.gameState.get(), container)
}

// gamePolicy is the ordinary policy engine plus what the capability packs declare.
//
// The packs' rules go in at CONSTRUCTION rather than being consulted later, so there is one
// policy engine and one decision path. A second engine for games would be a second place
// the question "may this run?" is answered, and two answers to that question is exactly the
// failure this milestone's "no plugin may bypass the Director" rule exists to prevent.
func gamePolicy(reg *game.Registry, detect func() game.Active) *policy.Engine {
	e := policy.New()
	if reg == nil {
		return e
	}
	rules := reg.Policies()
	// The framework's own rule needs to know what is detected NOW. It is built by the
	// registry without that, because the registry has no idea where detection is cached —
	// so it is supplied here, at the one place that knows both.
	for _, r := range rules {
		if sr, ok := r.(interface{ SetDetect(func() game.Active) }); ok {
			sr.SetDetect(detect)
		}
	}
	e.Rules = rules
	return e
}

// gameVerifier is the ordinary verifier plus the packs' contributed evidence.
func gameVerifier(reg *game.Registry) *verify.Verifier {
	v := verify.New()
	if reg != nil {
		v.Sources = reg.Verifiers()
	}
	return v
}
