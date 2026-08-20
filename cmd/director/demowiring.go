package main

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
)

// Demonstration recording in the daemon.
//
// The recorder lives HERE, in the long-lived service, because a demonstration spans
// several requests and the CLI is a fresh process each time. What it subscribes to is
// Handle's own outcome — the one place a completed, verified, recorded request exists —
// so recording adds no observation, no capture and no second path to the desktop.
//
// Every method below is CONTROL PLANE: none of them takes r.mu, so `director
// demonstrations` answers while a command is running. See lockrule_test.go. They take the
// recorder's lock and the store's, both of which are held only for the length of a map
// read or a file write.

// StartDemonstration opens a recording session.
func (r *Runtime) StartDemonstration() (*demo.Demonstration, error) {
	if r.demos == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	return r.demos.Start(demo.NewID(time.Now()))
}

// StopDemonstration closes the open session and stores it.
func (r *Runtime) StopDemonstration() (*demo.Demonstration, error) {
	if r.demos == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	return r.demos.Stop()
}

// AbandonDemonstration discards the open session.
func (r *Runtime) AbandonDemonstration(reason string) (*demo.Demonstration, error) {
	if r.demos == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	return r.demos.Abandon(reason)
}

// ActiveDemonstration is the open session, nil when none is.
func (r *Runtime) ActiveDemonstration() *demo.Demonstration {
	if r.demos == nil {
		return nil
	}
	return r.demos.Active()
}

// Demonstrations lists what has been recorded, newest first.
func (r *Runtime) Demonstrations() ([]*demo.Demonstration, error) {
	if r.demoStore == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	return r.demoStore.Demonstrations()
}

// Demonstration reads one back, preferring the session that is still open.
func (r *Runtime) Demonstration(id demo.ID) (*demo.Demonstration, error) {
	if active := r.ActiveDemonstration(); active != nil && active.ID == id {
		return active, nil
	}
	if r.demoStore == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	return r.demoStore.Demonstration(id)
}

// ExtractProcedure proposes a procedure from a recorded demonstration.
//
// It INSTALLS nothing. The proposal comes back for a person to read, and approval is a
// separate request — see ApproveProcedure.
func (r *Runtime) ExtractProcedure(id demo.ID) (demo.Extraction, error) {
	d, err := r.Demonstration(id)
	if err != nil {
		return demo.Extraction{}, err
	}
	return demo.Extract(d), nil
}

// ApproveProcedure installs an extracted procedure into the live registry.
//
//	Approved procedures enter the procedure registry. Exactly the same registry used by
//	built-in procedures.
//
// The extraction is re-run rather than taken from the caller. A client that could hand the
// service a candidate could hand it one it had edited, which would make approval a way to
// author procedures rather than a way to accept them — and user-written procedure code is
// a non-goal of this milestone for good reasons.
func (r *Runtime) ApproveProcedure(id demo.ID, by string) (*demo.Learned, error) {
	out, err := r.ExtractProcedure(id)
	if err != nil {
		return nil, err
	}
	l, err := demo.Approve(out, by, time.Now())
	if err != nil {
		return nil, err
	}
	if r.demoStore == nil {
		return nil, fmt.Errorf("this Director has no demonstration store")
	}
	if err := r.demoStore.SaveLearned(l); err != nil {
		return nil, err
	}
	// Into the LIVE registry, so the next request can use it. Without this the user would
	// have to restart the service to use something Marco just learned.
	if err := r.registerLearned(l); err != nil {
		return nil, err
	}
	return l, nil
}

// registerLearned adds one learned procedure to the running registry, refusing a
// registration that would make the registry unusable.
//
// Validated BEFORE it is kept, and by the registry's own rule. A learned procedure that
// permanently shadowed another would otherwise be discovered by the first user to ask for
// that goal, mid-request, on a live desktop — which is exactly what NewValidatedRegistry
// exists to prevent at startup.
func (r *Runtime) registerLearned(l *demo.Learned) error {
	if r.goals == nil {
		return fmt.Errorf("this Director has no procedure registry")
	}
	candidate := goal.NewRegistry()
	r.demoStore.Register(candidate)
	if shadowed := candidate.Validate(); len(shadowed) > 0 {
		return fmt.Errorf("%q cannot be installed: %s — %s",
			l.Name, shadowed[0].Procedure, shadowed[0].Reason)
	}
	r.goals.Register(l.AsProcedure())
	return nil
}

// ForgetProcedure removes a learned procedure.
//
// It does not un-register it from the running registry: the registry is append-only by
// design, and a procedure that vanished mid-session would make one request behave
// differently from the next for reasons nobody could see. The removal takes effect on the
// next start, and `director procedures` says so.
func (r *Runtime) ForgetProcedure(name string) error {
	if r.demoStore == nil {
		return fmt.Errorf("this Director has no demonstration store")
	}
	return r.demoStore.Forget(name)
}

// LearnedProcedures is what has been learned, for listing and explaining.
func (r *Runtime) LearnedProcedures() []*demo.Learned {
	if r.demoStore == nil {
		return nil
	}
	return r.demoStore.Learned()
}
