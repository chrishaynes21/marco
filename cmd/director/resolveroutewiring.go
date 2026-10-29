package main

// WHICH SAVED ROUTINE DID THEY MEAN.
//
// Director's answer to a request in a person's own words, and the only one in the product. Studio
// had its own, in its own module, where the widget could not reach it — so wiring the widget to
// it would have meant a second scope policy, a second way of building candidate labels and a
// second place deciding which routine a sentence named. The policy is `internal/resolveroute`
// now, and this is what gives it the three facts only Director has.
//
// # It grants nothing
//
// Everything here is a read. There is no branch that starts a performer, raises an application,
// presses anything or writes product data, and no argument a caller could pass that would produce
// one. It does consult the configured resolver plugin, which is a subprocess — that is the
// mechanism this reuses, and "zero execution" means zero route execution and zero input, not
// zero child processes.
//
// # The three facts
//
//	the library       Director's, through marcopaths, and NAMED in the answer so a surface
//	                  resolving against a different one is told rather than silently served
//	the candidates    every saved routine, with whether it would run as it stands
//	the context       which application this is read against, and HOW that was known
//
// The third is the one worth reading `resolveroute.Where` about: a world-model observation is
// evidence and not proof, because the window system's foreground claim comes through the path
// that drops Marco's own surfaces — so with a Marco window in front no observation is produced
// and the last one simply stands, naming an application that is no longer there. What grants a
// context is a foreground reading taken NOW through that same excluding path.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// resolveTimeout bounds one resolution, including the plugin call.
//
// The plugin has its own generous bound for a cold model load; this is the backstop on the whole
// question, so a surface asking as somebody types is never left holding a request that cannot
// end. Past it the answer is the deterministic one, which is what an unconfigured machine gets.
const resolveTimeout = 30 * time.Second

// ResolveRoute answers which saved routine a request named.
func (r *Runtime) ResolveRoute(p service.ResolveRoutePayload) service.ResolveRouteResponse {
	reg := learnedRegistry()
	resolver := resolveroute.Resolver{Reg: reg, Advisor: dispatch.Default()}
	where := r.whereToRead(p.Target)

	library := theLibrary(reg)
	// ── AND A TARGET NOBODY HAS ROUTINES FOR IS SAID, NOT SILENTLY HONOURED ──
	//
	// A stated target is authoritative about INTENT and is still a claim about this
	// library: somebody can name an application Director has nothing saved for, from a
	// dropdown built somewhere else or from a request assembled by hand. Honouring it
	// quietly produces an answer with no in-place routine in it and nothing saying why,
	// which reads as "Marco has nothing for this" when the truth is "not in the library
	// I am reading".
	if where.Basis == resolveroute.Stated && !anyRoutineFor(library, where.App) {
		where.Says += "; nothing in this library is saved for " + where.App
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	out := service.ResolveRouteResponse{
		Answer: resolver.Resolve(ctx, p.Request, library, where),
	}
	// ── AND A LIBRARY THAT IS NOT THE CALLER'S IS NAMED ──
	//
	// Both sides answer this through `marcopaths`, so they agree unless the two processes
	// were given different `$MARCO_ROUTES`. Named rather than resolved anyway: a surface
	// selecting a routine out of a library Director was not reading would be offering a
	// routine of that NAME and not that routine. The same reasoning `Prepared` already
	// carries for a review built in one library and replaced in another.
	if asked := strings.TrimSpace(p.Library); asked != "" && asked != reg.Dir {
		out.Mismatch = "this was resolved against the library at " + reg.Dir +
			" and you asked about " + asked + ", so a routine of that name here " +
			"is a different routine — nothing has been selected"
		// AND THE SELECTION IS CLEARED HERE, not left for a surface to remember.
		//
		// The first version carried the identity and a note saying a surface must
		// not use it, which is a rule in a comment. Director decides and the surface
		// renders: an answer that must not be acted on does not carry the thing to
		// act on. What is left is the question and the two library paths.
		out.Answer.ID, out.Answer.App, out.Answer.Slug = "", "", ""
		out.Answer.Focus, out.Answer.Name, out.Answer.Scope = false, "", ""
		out.Answer.Kind = "clarify"
		out.Answer.Reply = out.Mismatch
	}
	return out
}

// theLibrary is every saved routine, with whether it would run as it stands.
//
// Through the same two readers every other surface uses: the registry for what is there, and
// `Unrunnable` for the ones whose presses would all refuse. A routine that would refuse is not an
// answer to anything, so it is not offered — see resolveroute.Eligible.
func theLibrary(reg routes.Registry) []resolveroute.Candidate {
	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{}}
	cannot := map[string]bool{}
	for _, u := range d.Unrunnable() {
		cannot[orchestrator.RouteID(u.Route)] = true
	}
	out := []resolveroute.Candidate{}
	for _, rt := range reg.List() {
		raw, err := os.ReadFile(reg.Path(rt))
		if err != nil {
			continue
		}
		id := orchestrator.RouteID(rt)
		out = append(out, resolveroute.Candidate{
			Route:    rt,
			ID:       id,
			Name:     orchestrator.NameIn(string(raw), rt.Slug),
			Scope:    string(orchestrator.ScopeAlreadyOf(rt)),
			Runnable: !cannot[id],
		})
	}
	return out
}

// whereToRead decides which application a request is read against.
//
// Three readings, and the decision between them is `resolveroute.Where` rather than anything
// here: the rule is about evidence and is held without a desktop, and this is the part that needs
// one. See that function for why a fresh observation is not proof of a foreground.
func (r *Runtime) whereToRead(stated string) resolveroute.Context {
	return resolveroute.Where(stated, r.lastSeenApplication(), r.foregroundNow(), time.Now())
}

// lastSeenApplication is the world model's active application, with when it was observed.
func (r *Runtime) lastSeenApplication() resolveroute.Seen {
	r.mu.Lock()
	w := r.lastWorld
	r.mu.Unlock()
	if w == nil || w.ActiveApp == nil {
		return resolveroute.Seen{}
	}
	return resolveroute.Seen{App: w.ActiveApp.Name, At: w.Timestamp, OK: true}
}

// foregroundNow asks the window system which of the PERSON'S windows is in front, right now.
//
// Through `windowref.Foreground`, which reads the candidate set `winprovider.offered()` produces
// — and that set excludes every Marco-owned surface. So this answers nothing when Studio or the
// overlay has the foreground, which is exactly the case that must not unlock a routine working
// only in place. Answering nothing is the point of it, not a limitation of it.
func (r *Runtime) foregroundNow() resolveroute.Seen {
	if r == nil || r.winPlatform == nil {
		return resolveroute.Seen{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, res, _ := windowref.Foreground(ctx, r.winPlatform)
	if res != windowref.Resolved || strings.TrimSpace(c.Application) == "" {
		// NOTHING IDENTIFIABLE IN FRONT, which with a Marco window there is the
		// ordinary answer rather than a failure: the candidate set excludes our own
		// surfaces, so there is no candidate carrying the foreground at all.
		return resolveroute.Seen{}
	}
	return resolveroute.Seen{App: c.Application, At: time.Now(), OK: true}
}

// anyRoutineFor reports whether this library holds a routine saved for an application.
//
// Any scope of it, because the question is whether the library knows the application at all, not
// whether it has something in-place for it: a target naming an application with only
// anywhere-in-application routines is a target somebody can legitimately have meant.
func anyRoutineFor(cs []resolveroute.Candidate, app string) bool {
	if app == "" {
		return false
	}
	for _, c := range cs {
		if strings.EqualFold(c.App(), app) {
			return true
		}
	}
	return false
}
