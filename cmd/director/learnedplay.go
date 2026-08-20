package main

import (
	"fmt"
	"os"
	"time"

	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Writing down what Marco learned — and stopping there.
//
// # What this produces
//
// Text. A `.marco` program a person can read, held to Core v1 by the real compiler, handed back
// in a response. It is not written to `routes/`, not registered, not resolvable, and not runnable
// by anything here.
//
// **Generating Marco is not authorization to run Marco.** The one thing that could reach a
// computer in this repository is a claimed rehearsal grant, and nothing on this path has one.
//
// See [[ADR-027-what-marco-learned-becomes-marco]].

// LearnedPlay writes a verified procedure down as ordinary Marco.
//
// THE lowering entry point. It recomputes the judgement from live memory — verification is derived
// and re-derived, never read back — generates Core v1, and compiles it before showing anybody, so
// a program that Marco could not accept is reported rather than offered.
func (r *Runtime) LearnedPlay(q service.LearnedQuery) (service.LearnedView, error) {
	if r.observations == nil {
		return service.LearnedView{}, fmt.Errorf("this Director has no observation registry")
	}
	g := r.observations
	g.mu.RLock()
	memory := g.memory
	application := q.Application
	if application == "" {
		for i := len(g.finished) - 1; i >= 0; i-- {
			if application = g.finished[i].Session.Application; application != "" {
				break
			}
		}
	}
	g.mu.RUnlock()
	if memory == nil || application == "" {
		return service.LearnedView{}, fmt.Errorf("nothing has been observed yet")
	}
	store, ok := memory.(observe.CandidateStore)
	if !ok {
		return service.LearnedView{}, fmt.Errorf("this Director keeps no demonstrations")
	}
	rehearsals, _ := memory.(observe.RehearsalStore)

	top := memory.Topology(application)
	var out service.LearnedView
	var internal []lowered
	out.Application = application

	for _, c := range store.Candidates(application) {
		// RECOMPUTED, never read back. A candidate that was lowerable last week may not be
		// now — see [[ADR-021-a-judgement-is-recomputed-not-recorded]].
		j, known := g.judgeNow(application, c.Relationship)
		if !known {
			continue
		}
		a := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
			corroborationFor(store, application, c))
		if rehearsals != nil {
			a = a.WithRehearsal(c, j.Digest, top, rehearsals.Rehearsals(application))
		}
		// THE lowering recompute, and the naming demand that comes out of it.
		lowering := observe.JudgeLowering(c, a, top, application)
		// A verified procedure that is ready to be written down and cannot say where it
		// begins is the ONLY thing in this system that makes Marco ask what a screen is
		// called. Not a sweep over unnamed subjects, not passive observation, not a tidier
		// memory: a concrete artifact is blocked, and the user's word is what unblocks it.
		//
		// Idempotent by durable subject, because this whole loop runs on every read.
		//
		// Deleting this call must fail TestAVerifiedPlayThatCannotSayWhereItStartsAsks.
		//
		// ONE question at a time, and the judgement decides which. `Unnamed` is source first,
		// then destination — so naming the source, recomputing, and finding the destination
		// still missing is the ordinary shape. There is no queue and no remembered question:
		// the next need is discovered by asking the judgement again.
		if refusedFor(lowering, observe.RefusalScreenUnnamed) && len(lowering.Unnamed) > 0 {
			g.ProposeScreenName(application, lowering.Unnamed[0])
		}

		play := service.LearnedPlayView{
			From: c.Relationship.From, To: c.Relationship.To,
			Eligible: lowering.Eligible, Lines: observe.DescribeLowering(lowering),
			// The judgement's OWN list, in its own order, so a caller following the
			// lifecycle asks the judgement which screen is blocking rather than keeping a
			// queue that could disagree with it.
			Unnamed: append([]string{}, lowering.Unnamed...),
		}
		for _, r := range lowering.Refusals {
			play.Refusals = append(play.Refusals, string(r))
		}
		if lowering.Eligible {
			src, err := lowerAndCompile(lowering)
			if err != nil {
				// A LANGUAGE-EXPRESSION GAP, reported rather than worked around. The
				// alternative is widening Marco to make lowering convenient, which is
				// how a language stops being one somebody reads.
				play.Eligible = false
				play.Refusals = append(play.Refusals, string(observe.RefusalInexpressible))
				play.Detail = err.Error()
			} else {
				play.Source = src
			}
		}
		out.Plays = append(out.Plays, play)
		internal = append(internal, lowered{view: play, steps: meaningsOf(lowering),
			sequence: c.Sequence, evidence: j.Digest,
			startsOn: lowering.StartsOn.String(), endsOn: lowering.EndsOn.String()})
	}
	if q.Save || q.Register || q.Forget {
		if err := r.lifecycle(q, &out, internal); err != nil {
			return out, err
		}
	}
	return out, nil
}

// refusedFor reports whether a lowering judgement carries one particular refusal.
func refusedFor(j observe.LoweringJudgement, want observe.LoweringRefusal) bool {
	for _, r := range j.Refusals {
		if r == want {
			return true
		}
	}
	return false
}

// meaningsOf is the judgement's steps as play actions, for regenerating under a chosen name.
//
// A translation and nothing else: the judgement decided what the play does, and this carries that
// across a package boundary the domain type may not cross. A step that presses a control keeps its
// LABEL, which is the whole reason the two types have the shape they do.
func meaningsOf(j observe.LoweringJudgement) [][]marcoexec.PlayAction {
	out := make([][]marcoexec.PlayAction, 0, len(j.Steps))
	for _, run := range j.Steps {
		acts := make([]marcoexec.PlayAction, 0, len(run))
		for _, a := range run {
			if a.Invokes() {
				acts = append(acts, marcoexec.Press(a.Called, a.Kind))
				continue
			}
			acts = append(acts, marcoexec.Navigate(string(a.Intent)))
		}
		out = append(out, acts)
	}
	return out
}

// lowerAndCompile writes the play and then asks Marco whether it is Marco.
//
// The compiler is the authority, and it runs against the canonical `os.marco` surface rather than
// any convenient subset: a capability Director emits that the real act does not export must fail
// HERE, at compile time, which is the whole of [[ADR-005-legal-marco-only]].
//
// Nothing is executed. Compilation produces a program and the program is dropped.
func lowerAndCompile(j observe.LoweringJudgement) (string, error) {
	// The play carries its own entry condition, from the name the USER gave the starting
	// screen. `JudgeLowering` resolved it from durable memory; nothing here may substitute.
	src, err := marcoexec.LowerProvisionalActionsBetween(
		j.StartsOn.String(), j.EndsOn.String(), meaningsOf(j))
	if err != nil {
		return "", err
	}
	if err := compileGenerated(src); err != nil {
		return "", err
	}
	return src, nil
}

// compileGenerated asks Marco whether what the Director wrote is Marco.
//
// Against the canonical act surfaces, not a convenient subset: a capability the Director emits
// that the real act does not export must fail HERE, which is the whole of
// [[ADR-005-legal-marco-only]]. Nothing is executed — compilation produces a program and the
// program is dropped.
//
// # Why it does not assemble the modules itself
//
// It used to, by concatenating the two module sources it knew about and deleting the two `use`
// lines it knew about. That is a hand-maintained copy of a fact the runtime already owns, and it
// went stale the moment a play could press a control by name: such a play imports the Theater act,
// which was not in the Director's list, so a route the Audience had named, rehearsed and verified
// ended at `core_cannot_express` — `unknown type "Target"` — while the spec test asserting that
// same play compiles stayed green off its own, longer list.
//
// `driver.CheckSource` resolves `use` exactly as running the play would. There is one list, in the
// resolver, and a module the Director imports either exists there or the play is refused.
//
// Deleting this call must fail TestThePreflightAcceptsAPlayThatPressesAControlByName.
func compileGenerated(src string) error {
	if err := driver.CheckSource(src); err != nil {
		return fmt.Errorf("the generated play does not compile: %w", err)
	}
	return nil
}

// corroborationFor is what the other demonstration of the same route said.
func corroborationFor(store observe.CandidateStore, application string,
	c observe.ProcedureCandidate) observe.Corroboration {

	for _, other := range store.Candidates(application) {
		if other.Relationship == c.Relationship && other.Sequence != c.Sequence {
			return observe.Corroboration{Compared: true,
				Agreement: observe.CompareCandidates(c, other)}
		}
	}
	return observe.Corroboration{}
}

// ── the lifecycle: name, save, register, forget ───────────────────────────────

// learnedRegistry is where saved plays live.
//
// The SAME registry ordinary taught and authored routes use, honouring `$MARCO_ROUTES` exactly as
// `marco` does. A learned play is a `.marco` file in the routes tree; that is the point.
func learnedRegistry() routes.Registry {
	dir := os.Getenv("MARCO_ROUTES")
	if dir == "" {
		dir = "routes"
	}
	return routes.Registry{Dir: dir}
}

// lifecycle carries out whichever of save / register / forget the request asked for.
//
// # Four boundaries, and nothing crosses one it was not asked to
//
//	naming      the user says what the play and its verb are called
//	saving      it becomes a file, where the resolver cannot see it
//	registering it moves somewhere the resolver looks
//	forgetting  it and its provenance go away — and nothing Director OBSERVED does
//
// None of them runs anything, and none of them creates any authority to run anything.
func (r *Runtime) lifecycle(q service.LearnedQuery, v *service.LearnedView,
	plays []lowered) error {

	reg := learnedRegistry()
	// THE SLUG IS WHAT THE AUDIENCE SAYS, not the actor the Marco happens to declare.
	//
	// `Name`/`Verb` are the play.s identity inside the language; the slug is how a person
	// asks for it. Taking the slug from Name meant the taught phrase "Open Mouse Settings"
	// registered as `mousesettings` and then resolved to a suggestion rather than a route.
	//
	// Deleting the Phrase branch must fail TestTheTaughtPhraseIsWhatResolves.
	slug := routes.Slug(q.Name)
	if p := strings.TrimSpace(q.Phrase); p != "" {
		slug = routes.Slug(p)
	}
	rt := routes.Route{App: v.Application, Slug: slug}
	out := &service.LearnedSaved{Name: slug}

	if q.Forget {
		if err := reg.Unregister(rt); err != nil {
			return err
		}
		out.Forgotten = true
		out.Lines = []string{
			"Marco no longer has that play.",
			"What it observed is untouched — forgetting a play is not forgetting what it saw.",
		}
		v.Saved = out
		return nil
	}

	if q.Save {
		if slug == "" {
			return fmt.Errorf("a play needs a name before it can be saved")
		}
		// THE WHOLE ROUTE when one was asked for, the single edge otherwise.
		play, ok := joinWalk(plays, q.Walk)
		if !ok {
			play, ok = pickPlay(plays, q)
		}
		if !ok {
			return fmt.Errorf("no route is ready to be written down")
		}
		// REGENERATED with the chosen names, never string-replaced. A rename by substitution
		// can change a procedure that happens to share a word; building the file again from
		// the same ordered meanings cannot.
		src, err := marcoexec.LowerActionsBetween(q.Name, q.Verb, play.startsOn, play.endsOn,
			play.steps)
		if err != nil {
			return err
		}
		if err := compileGenerated(src); err != nil {
			return err
		}
		if err := reg.SaveStaged(rt, src, routes.Origin{
			Kind: routes.KindLearned, Application: v.Application,
			From: play.view.From, To: play.view.To, Sequence: play.sequence,
			Evidence: play.evidence, SavedAt: sessionClock.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		out.Saved, out.Source = true, src
		out.Lines = append(out.Lines, "Saved as "+slug+".",
			"Nothing can ask for it yet — register it when you want to be able to.")
	}

	if q.Register {
		if err := reg.Register(rt); err != nil {
			return err
		}
		out.Registered = true
		out.Lines = append(out.Lines, "Registered. You can ask for "+slug+" now.",
			"Asking for it is not the same as Marco performing it.")
	}
	if q.Save || q.Register {
		v.Saved = out
	}
	return nil
}

// lowered is one route's play plus the provenance a sidecar needs.
type lowered struct {
	view     service.LearnedPlayView
	steps    [][]marcoexec.PlayAction
	sequence int
	evidence string
	// startsOn and endsOn are what the user calls the screens this route begins on and is
	// expected to arrive at. Both come from the judgement, which read them out of durable
	// memory; nothing here may substitute one.
	startsOn string
	endsOn   string
}

// pickPlay chooses which route the request is about.
func pickPlay(plays []lowered, q service.LearnedQuery) (lowered, bool) {
	for _, p := range plays {
		if !p.view.Eligible {
			continue
		}
		if q.From != "" && p.view.From != q.From {
			continue
		}
		if q.To != "" && p.view.To != q.To {
			continue
		}
		return p, true
	}
	return lowered{}, false
}

// joinWalk builds the play for a whole ordered route out of the plays for its edges.
//
// # Why this exists
//
// A demonstration of A → B → C is kept as two reusable edges, and each lowers to its own play.
// That decomposition is right — each leg is route knowledge in its own right — and it is not the
// behaviour the Audience asked for. Saving the terminal edge alone produced:
//
//	do Screen's Showing with "Bluetooth & devices"…     ← the play begins in the middle
//	the target1 is a Target with Name "Mouse"…
//
// Asked from Home, that play refuses its own entry condition. The first verified edge was never
// lost; it was never joined, because nothing had ever asked for a route rather than an edge.
//
// The join is ordinary: the steps of each edge in walk order, the FIRST edge's starting screen,
// the LAST edge's finishing screen. `LowerActionsBetween` already emits one local per activation,
// so the Marco needs nothing new.
//
// Every edge must be eligible. A route is not partly writable — half a procedure is a different
// procedure — and an edge missing from the walk is a refusal rather than a shorter play.
//
// Deleting this must fail TestAWholeRouteLowersEveryEdgeInOrder.
func joinWalk(plays []lowered, walk []service.LearnedStep) (lowered, bool) {
	if len(walk) == 0 {
		return lowered{}, false
	}
	byEdge := map[service.LearnedStep]lowered{}
	for _, p := range plays {
		byEdge[service.LearnedStep{From: p.view.From, To: p.view.To}] = p
	}
	var out lowered
	for i, step := range walk {
		p, ok := byEdge[step]
		if !ok || !p.view.Eligible {
			return lowered{}, false
		}
		if i == 0 {
			// WHERE THE AUDIENCE STARTED, which is the whole point of joining.
			out.startsOn, out.view.From = p.startsOn, p.view.From
			out.sequence, out.evidence = p.sequence, p.evidence
		}
		out.steps = append(out.steps, p.steps...)
		out.endsOn, out.view.To = p.endsOn, p.view.To
	}
	out.view.Eligible = true
	return out, true
}
