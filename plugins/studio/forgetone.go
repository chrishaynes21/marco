package main

// FORGET ONE ROUTINE (R8 of docs/Surfaces-alignment-proposal-2026-11-02.md).
//
// Studio could forget EVERYTHING, and could not forget one routine. Now it can, through the one
// forget the CLI already uses (`orchestrator.ForgetRoutine`): the shortcuts first, then the
// provenance, then the routine with what is kept beside it, under the routine's lock.
//
// # Shown, then sealed
//
// The confirmation names exactly what goes: the routine and where it works, its demonstration,
// an edited step account, the phrases it also answers to, and each shortcut as it is pressed. That
// list is sealed (`PlanForget`), and the forget refuses, having removed nothing, if any of it has
// changed by the time it runs (`ForgetAsPlanned`). A shortcut assigned after somebody read the
// list is not removed without having been on it.
//
// # And what happened is READ, not assumed
//
// A forget that fails partway can leave any mix of its parts: its shortcuts gone and the routine
// still there, the demonstration removed and the source left. So the result is a reading of what
// is still on disk afterwards, part by part, whether or not the forget said it succeeded.

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// ForgetReview is what forgetting one routine would remove.
type ForgetReview struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Where string `json:"where"`
	// Parts is one line per thing that goes, in the order they are removed.
	Parts []string `json:"parts"`
	Token string   `json:"token,omitempty"`
	Why   string   `json:"why,omitempty"`
}

// ForgetPart is one part of a routine, after a forget: gone or still there.
type ForgetPart struct {
	What string `json:"what"`
	Gone bool   `json:"gone"`
}

// ForgetDone is what a forget did, part by part.
type ForgetDone struct {
	Removed bool         `json:"removed"`
	Parts   []ForgetPart `json:"parts,omitempty"`
	Says    string       `json:"says"`
	Why     string       `json:"why,omitempty"`
}

// ReviewForgetRoutine says what forgetting the routine id would remove. It removes nothing.
func (a *Acts) ReviewForgetRoutine(id string) ForgetReview {
	id = strings.TrimSpace(id)
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return ForgetReview{ID: id, Why: "Marco is recording a demonstration; one thing at a time"}
	}
	chosen, ok := a.routineWithID(id)
	if !ok {
		return ForgetReview{ID: id, Why: fmt.Sprintf("there is no play %q in this library "+
			"(%s) — press Refresh and choose it again", id, a.routesDir)}
	}
	rt := chosen.route()
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	p, err := d.PlanForget(rt)
	if err != nil {
		return ForgetReview{ID: id, Name: chosen.Name, Why: err.Error()}
	}
	a.keepForget(p)
	out := ForgetReview{ID: id, Name: chosen.Name, Where: whereItWorksWords(rt), Token: p.Token}
	leader := widgetconfig.Leader()
	for _, b := range p.Shortcuts {
		out.Parts = append(out.Parts, "the shortcut "+pressedAs(leader, b.Key)+" "+placeWords(b.App))
	}
	// IN THE ORDER THEY GO (orchestrator.ForgetRoutine, routes.Registry.Delete), which the card says.
	if p.Provenance {
		out.Parts = append(out.Parts, "the record of where it came from")
	}
	if p.Demonstration {
		out.Parts = append(out.Parts, "the demonstration it was made from")
	}
	if p.Steps {
		out.Parts = append(out.Parts, "the steps as you edited them")
	}
	if len(p.Phrases) > 0 {
		out.Parts = append(out.Parts, "what else it answers to: "+strings.Join(quoteAll(p.Phrases), ", "))
	}
	out.Parts = append(out.Parts, "the play itself ("+rt.ID()+")")
	return out
}

// ForgetRoutine forgets the routine whose review carried token, as it was shown, and reports
// each part as gone or still there.
//
// Forgetting through anything but ForgetAsPlanned must fail TestForgettingOneRoutineIsWhatWasShown.
func (a *Acts) ForgetRoutine(token string) ForgetDone {
	p, ok := a.takeForget(strings.TrimSpace(token))
	if !ok {
		return ForgetDone{Why: "the forget you are looking at is no longer the one this window " +
			"holds — review it again"}
	}
	a.mu.Lock()
	busy, recording := a.busy, a.recordingLocked()
	a.mu.Unlock()
	if busy || recording {
		return ForgetDone{Why: "something is in flight; stop it first"}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	f, err := d.ForgetAsPlanned(p)
	parts := a.partsLeft(p)
	out := ForgetDone{Removed: f.Removed && allGone(parts), Parts: parts}
	name := plays.Pretty(p.Route.Slug)
	switch {
	case err == nil && out.Removed:
		out.Says = fmt.Sprintf("Forgot %q and everything listed.", name)
	case anyGone(parts):
		out.Says = fmt.Sprintf("%q was only partly forgotten: %s", name, err)
		if err == nil {
			out.Says = fmt.Sprintf("%q was only partly forgotten.", name)
		}
	default:
		out.Says = fmt.Sprintf("Nothing was removed: %v", err)
	}
	return out
}

// partsLeft reads each part of a forgotten routine back off the disk.
func (a *Acts) partsLeft(p orchestrator.ForgetPlan) []ForgetPart {
	reg := a.reg()
	leader := widgetconfig.Leader()
	var out []ForgetPart
	for _, b := range p.Shortcuts {
		still := false
		if now, ok := reg.BindingAt(b.App, b.Key); ok {
			for _, id := range now.Routes {
				still = still || id == p.Route.ID()
			}
		}
		out = append(out, ForgetPart{What: "the shortcut " + pressedAs(leader, b.Key) +
			" " + placeWords(b.App), Gone: !still})
	}
	out = append(out, ForgetPart{What: "the play itself", Gone: !reg.Has(p.Route)})
	if p.Provenance {
		_, originErr := os.Stat(reg.OriginPath(p.Route))
		out = append(out, ForgetPart{What: "its provenance record", Gone: os.IsNotExist(originErr)})
	}
	if p.Demonstration {
		out = append(out, ForgetPart{What: "the demonstration", Gone: !reg.HasRecording(p.Route)})
	}
	if p.Steps {
		out = append(out, ForgetPart{What: "the edited steps", Gone: !reg.HasSteps(p.Route)})
	}
	if len(p.Phrases) > 0 {
		out = append(out, ForgetPart{What: "what else it answered to", Gone: !reg.HasAliases(p.Route)})
	}
	return out
}

func allGone(ps []ForgetPart) bool {
	for _, p := range ps {
		if !p.Gone {
			return false
		}
	}
	return true
}

func anyGone(ps []ForgetPart) bool {
	for _, p := range ps {
		if p.Gone {
			return true
		}
	}
	return false
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

func whereItWorksWords(rt routes.Route) string {
	switch {
	case rt.App == "":
		return "everywhere"
	case rt.Focus:
		return "from anywhere → " + rt.App
	}
	return "only in " + rt.App
}

// The reviews this window holds, by token, until one is used or the window has too many.
func (a *Acts) keepForget(p orchestrator.ForgetPlan) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.forgets == nil {
		a.forgets = map[string]orchestrator.ForgetPlan{}
	}
	if len(a.forgets) > 8 {
		a.forgets = map[string]orchestrator.ForgetPlan{}
	}
	a.forgets[p.Token] = p
}

func (a *Acts) takeForget(token string) (orchestrator.ForgetPlan, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.forgets[token]
	delete(a.forgets, token)
	return p, ok
}
