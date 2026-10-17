package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Sequential semantic execution.
//
//	A program is not a batch of actions. It is a sequence of independently
//	observed, independently compiled, independently verified semantic operations.
//
// The loop below is that sentence as code. Every iteration observes, resolves against
// what it just observed, lowers ONE operation into Marco, compiles it, runs it,
// observes again, verifies, and only then advances. The Director holds control at
// every boundary, which is what makes "open File, then click Save" possible: Save does
// not exist in the world until File has been clicked, so it cannot be resolved before.
//
// The alternative — plan everything, resolve everything, then execute — is what makes
// desktop automation brittle. It resolves Save against a world that has no Save in it,
// and either fails at planning time or clicks a stale coordinate.

// ProgramOutcome is the result of running a whole program.
type ProgramOutcome struct {
	Program program.Program `json:"program"`
	// Steps are the per-step outcomes, in order, for the steps that ran.
	Steps []Outcome `json:"steps"`
	// Completed is how many steps verified.
	Completed int `json:"completed"`
	// Status is the headline result.
	Status directorapi.ResultStatus `json:"status"`
	// Message is one sentence for a person.
	Message string `json:"message"`
	// Goal is the expansion this program came from, nil when the request was an
	// ordinary sequence. Carried so the action graph and `explain goal` can say WHICH
	// procedure produced these steps — the steps are the answer, not the working.
	Goal *goal.Expansion `json:"goal,omitempty"`
	// Missing names the requirements a recognised goal could not satisfy — "name",
	// "destination" — when the request was understood and under-specified.
	//
	// Carried so the service can ASK and then resume: the Message is the English
	// question, and this is what an answer supplies. See goal.Supplying.
	Missing []string `json:"missing,omitempty"`
	// Confirmation is what happened at the goal-level confirmation gate. Empty for a
	// request that was not a goal.
	Confirmation ConfirmationOutcome `json:"confirmation,omitempty"`
	// Correlation is the whole-goal verification: whether the object the user pointed
	// at is the object the result belongs to. Nil when the goal had no such check —
	// which is every goal but a rename of a bound file, and every Director with nothing
	// able to read the result back.
	Correlation *verify.Correlation `json:"correlation,omitempty"`
	// StoppedAt is the 1-based step the program stopped at, 0 if it finished.
	StoppedAt int `json:"stopped_at,omitempty"`
	// StartedAt is the 0-based index the run began at, non-zero only on a resume.
	//
	// Recorded rather than inferred. Inferring it from len(Program.Steps)-len(Steps)
	// is only correct when the steps that ran are the TAIL, which holds for a resume
	// and is wrong for a program that stopped early — it labelled step 1 of a
	// cancelled three-step program as "3/3".
	StartedAt int `json:"started_at,omitempty"`
	// Pending is the step awaiting clarification, when Status is NeedsClarification.
	Pending *program.StepID `json:"pending_step,omitempty"`
	// Resumed is what the completed steps learned, kept so a clarification can
	// continue with a back-reference still meaning what it meant.
	Resumed program.Context `json:"-"`
}

// ProgramProgress reports a step boundary to the caller.
type ProgramProgress struct {
	ProgramID program.ProgramID
	StepID    program.StepID
	Index     int // 1-based
	Total     int
	Phrase    string
	Verb      string
	Status    string
	Detail    string
}

// HandleProgram decomposes a request into a program and runs it.
//
// Validation happens on the WHOLE program before the first step, so a request whose
// fourth step is unsupported does nothing at all rather than doing three quarters of
// something the user did not ask for.
func (p *Pipeline) HandleProgram(ctx context.Context, request string) ProgramOutcome {
	ctx = ensureBindings(ctx)
	// A GOAL first. "Rename this file to Budget" is one request however many steps it
	// becomes, and the clause splitter below would cut it at " to " into two clauses
	// that each mean nothing. What comes back is an ordinary program, so everything
	// after this line is unchanged.
	if ex, bound, isGoal, err := p.expandGoal(ctx, request); isGoal {
		if err != nil {
			return goalRefusal(request, err)
		}
		// What the whole goal will have to be shown to have done, captured BEFORE it
		// runs: the content that must survive is only readable while the file still
		// exists under its old name, and the files that must not move are only listable
		// while they are all still there. Nil for anything that is not a rename of a
		// bound file, or when nothing can read the filesystem.
		check := p.prepareRenameCheck(ex, bound)
		// The gate. Expansion has happened — it read a registry, observed the screen and
		// built structs — and nothing OBSERVABLE has: no focus change, no input, no
		// capability call. This is the last point at which that is still true.
		outcome, message := p.confirmGoal(ctx, request, ex)
		if !outcome.Confirmed() {
			return confirmationRefusal(request, ex, outcome, message)
		}
		if outcome == ConfirmationAccepted {
			// Scoped to THIS execution, so the per-step gate does not ask again for
			// something the user has already agreed to. What it covers is decided by
			// coveredByGoalConfirmation, which needs the concrete object as well as the
			// risk: a yes for one file is not a yes for another.
			cov := coverage{
				Risk: ex.Safety.Risk, Goal: ex.Goal.Kind, Procedure: ex.Procedure,
				Destructive: ex.Safety.Destructive || ex.Safety.Irreversible,
			}
			if b := firstBinding(ex); b != nil {
				cov.Resource, cov.Label = b.Resource, b.Label
			}
			ctx = withCoverage(ctx, cov)
		}
		// Stamped on every node the program produces, and cleared afterwards so the
		// next ordinary request does not inherit a goal it had nothing to do with.
		p.goalProvenance = goalProvenanceOf(request, ex)
		defer func() { p.goalProvenance = nil }()

		out := p.RunProgram(ctx, ex.Program, program.Context{}, 0)
		out.Goal = &ex
		out.Confirmation = outcome
		// The whole-goal verification, run automatically after real execution and before
		// the outcome is reported. A mismatch turns a program the per-step checks called
		// successful into a FAILURE, because "the right kind of change happened" is
		// satisfied by the right thing happening to the wrong file.
		p.correlateGoal(&out, check)
		return out
	}

	prog, err := program.Decompose(request, p.parseWith(ctx))
	if err != nil {
		return ProgramOutcome{
			Program: prog,
			Status:  directorapi.ResultFailed,
			Message: err.Error(),
		}
	}
	return p.RunProgram(ctx, prog, program.Context{}, 0)
}

// RunProgram executes a validated program from a given step.
//
// from is the 0-based index to start at, which is how a clarification resumes:
// completed steps stay completed and are not re-run. Re-running them would repeat
// input that already landed — the double-application hazard the single-step layer
// spends so much care avoiding.
func (p *Pipeline) RunProgram(ctx context.Context, prog program.Program,
	pctx program.Context, from int) ProgramOutcome {

	ctx = ensureBindings(ctx)
	out := ProgramOutcome{Program: prog, Completed: from, StartedAt: from}

	// The execution-side validator, and the reason it is here rather than only in the
	// expander: RunProgram is the single door every program goes through — a goal
	// expansion, a clause split, a resumed clarification — and a deictic step that
	// reached it without a binding must stop before step 1 rather than at step 3 with two
	// steps already performed.
	if err := program.ValidateBound(prog); err != nil {
		out.Program.Status = program.StatusRejected
		out.Status = directorapi.ResultBlocked
		out.Message = err.Error()
		return out
	}

	out.Program.Status = program.StatusRunning
	total := len(prog.Steps)

	// ONE environment per program execution, created before step 1 and shared by every
	// step after it. EnsureValues is idempotent, which is what makes a resume correct:
	// the paused program brings its environment back, and replacing it with an empty one
	// would silently unbind everything the completed steps captured.
	env := pctx.EnsureValues()
	env.SetProgram(string(prog.ID))
	colls := pctx.EnsureCollections()
	colls.SetProgram(string(prog.ID))
	if p.OnCollections != nil {
		p.OnCollections(colls)
	}
	if p.OnEnvironment != nil {
		// The service learns which environment belongs to the running program, so
		// `director status` and `director explain value` can answer while it runs. The
		// environment is still OWNED by the program — this hands over a reference for
		// reading, not custody.
		p.OnEnvironment(env)
	}

	// The lifetime ends here, on EVERY path out of this function — an early return, a
	// cancellation, a phase deadline, a planner refusing step 4, a panic. A deferred
	// clear is the only construction that covers all of them, and "values disappear
	// after every terminal outcome" is not a claim worth making if it depends on
	// remembering to call something at fourteen return sites.
	//
	// The single exception is a clarification PAUSE, which is not terminal: the program
	// is still alive, waiting for an answer that may arrive from an entirely different
	// client process, and its captured values must still be there when it does.
	paused := false
	defer func() {
		if paused {
			return
		}
		// Clear FIRST, then report what it did. Clear is idempotent on the environment
		// but this defer is the single place the event is emitted, which is what makes
		// "exactly once" true however many deferred paths lead here: there is only one
		// of them.
		// The collections go with the values: both are program-local, and both would
		// otherwise be re-run against a screen nobody was looking at.
		cleared := colls.Clear()
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventCollectionCleared, ProgramID: string(prog.ID),
			MatchedCount: cleared, TerminalState: string(out.Status),
			Reason: "the program reached a terminal state",
		})
		n := env.Clear()
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventEnvironmentCleared, ProgramID: string(prog.ID),
			ValueCount: n, TerminalState: string(out.Status),
			Reason: "the program reached a terminal state",
		})
		if n > 0 {
			p.markPhase(trace.PhaseProgramEnd,
				fmt.Sprintf("discarded %d program-local value(s)", n))
		}
		if p.OnEnvironment != nil {
			p.OnEnvironment(nil)
		}
		if p.OnCollections != nil {
			p.OnCollections(nil)
		}
	}()
	if from > 0 {
		// A resumed program is a continuation, not a new one. Marking it says so in the
		// trace, which is how a reader tells "step 1 ran again" from "step 1 was
		// already done".
		p.markPhase(trace.PhaseProgramResume,
			fmt.Sprintf("resuming at step %d of %d; %d already verified", from+1, total, from))
	}

	for i := from; i < total; i++ {
		step := prog.Steps[i]

		// Cancellation between steps. Never mid-program: a Marco program that has
		// started runs to its end, because abandoning it halfway leaves the desktop in
		// a state nobody planned — a key held down, half a string typed. Cancellation
		// prevents the NEXT one.
		if err := ctx.Err(); err != nil {
			out.Program.Status = program.StatusStopped
			out.Status = directorapi.ResultCancelled
			out.StoppedAt = i + 1
			out.Message = fmt.Sprintf("stopped before step %d of %d (%s)", i+1, total, step.Phrase)
			return out
		}

		// The step position rides on the pipeline so every phase record inside this
		// step says which step it belonged to.
		p.stepIndex, p.stepCount, p.stepID = i+1, total, string(step.ID)
		// What the PROCEDURE asked of this step, recorded per step: a step that was
		// allowed to be inconclusive and one that had to prove itself are different
		// claims about the same outcome, and a reader of the graph cannot tell them
		// apart without this.
		if p.goalProvenance != nil {
			p.goalProvenance.Verification = string(step.Verification)
			p.goalProvenance.Guarded = len(step.Preconditions) > 0
		}
		p.reportStep(prog, step, i+1, total, "running", "")

		// A back-reference inherits the previous step's target. Applied HERE, to the
		// intent, so the step still resolves against a fresh world — inheriting the
		// resolved handle itself would be exactly the stale-handle bug this design
		// exists to prevent.
		step.Operation = inherit(step.Operation, pctx)

		stepOut := p.runStep(ctx, step.Phrase, step.Operation, pctx)
		out.Steps = append(out.Steps, stepOut)

		// A STOP THAT ARRIVED MID-STEP IS THE REASON FOR WHATEVER THE STEP SAID.
		//
		// The check at the top of this loop catches a stop that lands BETWEEN steps. This
		// one catches the window inside one: the operation was in flight when the context
		// died, it came back with an error its own cancellation produced, and reading that
		// error as the step's verdict tells somebody who just pressed stop that their
		// request failed. Measured on a real desktop: `type Marco into the Name field and
		// click Save`, stopped during the typing — nothing typed, Save never pressed,
		// reported as `failed`. That is the one word that invites a retry, offered to the
		// one person who should not be nudged to run a half-done sequence again.
		//
		// Narrow on purpose, three ways. A step that SUCCEEDED is left alone, because the
		// next iteration's check reports it as stopped before the FOLLOWING step, which is
		// the more precise sentence. A clarification is left alone because it is not an
		// ending. And the step's own account is kept in the message, so a genuine failure
		// that happened to coincide with a stop is relabelled, never erased.
		//
		// The same rule as [[ADR-070]]-era cancellation handling elsewhere in Marco: when
		// the context is gone, the cancellation is the cause, and reporting the downstream
		// symptom sends a person to fix something that was never wrong. Enforced by
		// TestAStopDuringAStepIsCancelledNotFailed, with
		// TestAnUncancelledStepFailureIsStillAFailure holding the other side.
		if ctx.Err() != nil && stepOut.Status != directorapi.ResultDone &&
			stepOut.Status != directorapi.ResultNeedsClarification {
			out.Program.Status = program.StatusStopped
			out.Status = directorapi.ResultCancelled
			out.StoppedAt = i + 1
			out.Message = fmt.Sprintf("stopped during step %d of %d (%s): %s",
				i+1, total, step.Phrase, stepOut.Message)
			p.reportStep(prog, step, i+1, total, "stopped", stepOut.Message)
			return out
		}

		switch stepOut.Status {
		case directorapi.ResultDone:
			out.Completed++
			pctx = advance(pctx, stepOut)
			p.reportStep(prog, step, i+1, total, "verified", stepOut.Message)

		case directorapi.ResultNeedsClarification:
			// Pause, keeping everything. The program id, the step id and the completed
			// history all survive so that answering resumes rather than restarts.
			out.Program.Status = program.StatusNeedsClarification
			out.Status = directorapi.ResultNeedsClarification
			out.StoppedAt = i + 1
			id := step.ID
			out.Pending = &id
			out.Message = stepOut.Message
			out.Resumed = pctx
			// Not terminal: the values stay bound for the answer.
			paused = true
			p.markPhase(trace.PhaseProgramPause,
				fmt.Sprintf("paused at step %d of %d awaiting a clarification", i+1, total))
			p.reportStep(prog, step, i+1, total, "needs clarification", stepOut.Message)
			return out

		case directorapi.ResultNothingToDo:
			// A PRECONDITION ALREADY MET IS NOT A FAILED STEP.
			//
			// Measured live. "Rename this file to summary.txt" with the file already
			// selected: step one selected it again, nothing changed, and
			// verification reported `inconclusive: "notes.txt" reports itself
			// selected and nothing else changed to confirm it`. A four-step rename
			// abandoned at step one because step one had nothing to do.
			//
			// The step is counted and the context advances, exactly as a verified
			// one — because the state the next step needs is the state that was
			// already there. What does NOT happen is a record, and that is right:
			// nothing was performed, and the account says so rather than inventing
			// an action to describe.
			out.Completed++
			pctx = advance(pctx, stepOut)
			p.reportStep(prog, step, i+1, total, "nothing needed doing", stepOut.Message)
			continue

		case directorapi.ResultCancelled:
			out.Program.Status = program.StatusStopped
			out.Status = directorapi.ResultCancelled
			out.StoppedAt = i + 1
			out.Message = stepOut.Message
			return out

		case directorapi.ResultPartial:
			// The step acted and could not be PROVED. For an operation the World Model
			// genuinely cannot observe — a selection, a copy, an Enter that submitted a
			// search without changing the field — that is the expected answer, and the
			// planner marked it best-effort for exactly this case.
			//
			// Anything else stops. Best-effort is a closed list in the planner, not a
			// judgement made here, so a step cannot talk its way past verification by
			// being hard to check.
			if step.Verification == program.VerifyBestEffort {
				out.Completed++
				pctx = advance(pctx, stepOut)
				p.reportStep(prog, step, i+1, total, "unverified (not observable)", stepOut.Message)
				continue
			}
			out.Program.Status = program.StatusFailed
			out.Status = stepOut.Status
			out.StoppedAt = i + 1
			out.Message = fmt.Sprintf("step %d of %d (%s) could not be verified: %s",
				i+1, total, step.Phrase, stepOut.Message)
			p.reportStep(prog, step, i+1, total, "unverified", stepOut.Message)
			return out

		default:
			// FAILED, UNSAFE, AMBIGUOUS, UNOBSERVABLE — the program stops immediately
			// and later steps do not run. Continuing would execute step i+1 against a
			// world step i failed to produce.
			out.Program.Status = program.StatusFailed
			out.Status = stepOut.Status
			out.StoppedAt = i + 1
			out.Message = fmt.Sprintf("step %d of %d (%s) did not complete: %s",
				i+1, total, step.Phrase, stepOut.Message)
			p.reportStep(prog, step, i+1, total, "failed", stepOut.Message)
			return out
		}
	}

	// ── Did the person get what they asked for? ───────────────────────────────
	//
	// "N of N steps verified" is a claim about Marco's own mechanism. A procedure whose
	// last step typed into the wrong control and pressed Enter completes every step too,
	// and reports it.
	//
	// So where the goal declared what it was FOR, that is what decides the outcome — and
	// it is checked against a world observed now, after everything has settled, rather
	// than against the snapshot the last step happened to verify against.
	//
	// A program whose goal declared nothing is judged exactly as before. That is
	// deliberate: adding the field must not make every existing procedure assert
	// something nobody wrote down. See [[ADR-224-a-procedure-is-finished-when-the-person-has-what-they-asked-for]].
	if len(prog.Outcome) > 0 {
		return p.concludeByOutcome(ctx, out, prog, total)
	}

	out.Program.Status = program.StatusCompleted
	out.Status = directorapi.ResultDone
	out.Message = fmt.Sprintf("%d of %d steps verified", out.Completed, total)
	return out
}

// concludeByOutcome decides a program by what the person asked for rather than by its steps.
//
// Three answers, and the third is the one that keeps this honest:
//
//	MET        the outcome is there. Done, and the sentence says what is there.
//	UNMET      the steps ran and the thing is not there. NOT done — this is the case a
//	           step count would have reported as success.
//	UNCHECKED  the outcome could not be evaluated. Also not done. "I could not look" is
//	           not "it worked", and treating it as one would make every condition type
//	           Marco does not yet understand a silent pass.
func (p *Pipeline) concludeByOutcome(ctx context.Context, out ProgramOutcome,
	prog program.Program, total int) ProgramOutcome {

	world, err := p.observeTraced(ctx)
	if err != nil {
		out.Program.Status = program.StatusFailed
		out.Status = directorapi.ResultPartial
		out.Message = fmt.Sprintf("%d of %d steps ran, and the screen could not be read "+
			"to check whether %s: %v", out.Completed, total,
			outcomeAsked(prog), err)
		return out
	}
	got := verify.CheckOutcome(prog.Outcome, &world, touchedBy(out)...)
	switch {
	case got.Satisfied():
		out.Program.Status = program.StatusCompleted
		out.Status = directorapi.ResultDone
		out.Message = got.Describe()
	case len(got.Unmet) > 0:
		out.Program.Status = program.StatusFailed
		out.Status = directorapi.ResultFailed
		out.Message = fmt.Sprintf("%d of %d steps ran, but %s", out.Completed, total,
			got.Describe())
	default:
		out.Program.Status = program.StatusFailed
		out.Status = directorapi.ResultPartial
		out.Message = fmt.Sprintf("%d of %d steps ran, and %s", out.Completed, total,
			got.Describe())
	}
	return out
}

// touchedBy is every control this program's steps acted on.
//
// Handed to the outcome check so none of them can be the evidence. Measured: without it, "make
// a new folder called Reports" reported `COMPLETED: text_field "Reports" is there` about an
// edit box holding Marco's own keystrokes, while the folder on disk was still "New folder". See
// verify.CheckOutcome.
func touchedBy(out ProgramOutcome) []directorapi.ElementID {
	var ids []directorapi.ElementID
	for _, step := range out.Steps {
		if step.Record != nil && step.Record.Target.ElementID != "" {
			ids = append(ids, step.Record.Target.ElementID)
		}
		if step.Resolution != nil && step.Resolution.Target != nil {
			ids = append(ids, step.Resolution.Target.ElementID)
		}
	}
	return ids
}

// outcomeAsked is the goal's own description of what it was for, for a message about not having
// been able to check it.
func outcomeAsked(prog program.Program) string {
	var parts []string
	for _, c := range prog.Outcome {
		parts = append(parts, c.Describe())
	}
	return strings.Join(parts, " and ")
}

// runStep executes ONE semantic operation, whole: observe, resolve, plan, policy,
// lower to Marco, compile, execute, observe, verify, record.
//
// It is the existing single-step pipeline, entered with an already-parsed intent. That
// reuse is deliberate and load-bearing: a program step gets exactly the verification,
// the retry guard, the settle-by-condition and the Marco lowering that a single
// request gets, because it IS one. A separate "sequence executor" with its own
// simplified version of all that is how the two paths drift.
func (p *Pipeline) runStep(ctx context.Context, phrase string, in directorapi.Intent,
	pctx program.Context) Outcome {
	// AND IT IS A STEP, which the single-step pipeline has one reason to know: a
	// precondition already met means a procedure CONTINUES, where a person's request
	// already satisfied means nothing needs doing. See program.Context.WithinProgram.
	pctx.WithinProgram = true
	return p.handleParsed(ctx, phrase, in, pctx)
}

// inherit fills a back-reference from the program context.
//
// It rewrites the QUERY, not the target: the step still looks the element up in a
// fresh world. If the previous step's control is gone, resolution fails honestly
// rather than acting on a handle that no longer means anything.
func inherit(in directorapi.Intent, pctx program.Context) directorapi.Intent {
	if len(in.Targets) == 0 || pctx.LastResolvedTarget == nil {
		return in
	}
	t := in.Targets[0]
	if !program.IsBackReference(t.Phrase) {
		return in
	}
	last := pctx.LastResolvedTarget
	q := &directorapi.ElementQuery{}
	switch {
	case last.Label != "":
		// By LABEL rather than by id. A label survives a repaint that reissues
		// element ids, and re-finding by label is what "the same control" means.
		q.Label = last.Label
		q.Role = last.Role
	case last.Role != "":
		q.Role = last.Role
	default:
		return in
	}
	if last.WindowID != "" {
		w := last.WindowID
		q.Window = &w
	}
	in.Targets = []directorapi.ReferenceExpression{{
		Phrase: t.Phrase,
		Kind:   directorapi.ReferenceAnaphoric,
		Query:  q,
	}}
	return in
}

// advance carries what a completed step learned into the next one.
func advance(pctx program.Context, out Outcome) program.Context {
	if out.Resolution != nil && out.Resolution.Target != nil {
		t := *out.Resolution.Target
		pctx.LastResolvedTarget = &t
	}
	if out.World != nil {
		if w, ok := out.World.FocusedWindow(); ok {
			win := *w
			pctx.LastWindow = &win
		}
	}
	if out.Node != nil {
		pctx.LastActionNode = string(out.Node.ID)
	}
	return pctx
}

// reportStep emits a step boundary through the pipeline's progress hook.
func (p *Pipeline) reportStep(prog program.Program, step program.Step,
	index, total int, status, detail string) {

	if p.OnProgram == nil {
		return
	}
	p.OnProgram(ProgramProgress{
		ProgramID: prog.ID, StepID: step.ID,
		Index: index, Total: total,
		Phrase: step.Phrase, Verb: step.Operation.Verb,
		Status: status, Detail: detail,
	})
}

// Render draws the per-step trace a person sees.
func (o ProgramOutcome) Render() string {
	var b strings.Builder
	total := len(o.Program.Steps)
	for i, s := range o.Steps {
		verb := ""
		if i < total {
			verb = o.Program.Steps[i].Verb() + " " + o.Program.Steps[i].Phrase
		}
		mark := "verified"
		switch s.Status {
		case directorapi.ResultDone:
		case directorapi.ResultNeedsClarification:
			mark = "needs clarification"
		case directorapi.ResultPartial:
			mark = "UNVERIFIED"
		case directorapi.ResultCancelled:
			mark = "cancelled"
		default:
			mark = "FAILED"
		}
		fmt.Fprintf(&b, "[%d/%d] %s\n        %s\n", i+1, total, verb, mark)
		if s.Status != directorapi.ResultDone && s.Message != "" {
			fmt.Fprintf(&b, "        %s\n", s.Message)
		}
	}
	fmt.Fprintf(&b, "%s\n", strings.ToUpper(string(o.Status)))
	return b.String()
}

// parentForStep links a step's node to the previous step's, so a program reads as a
// chain in the action graph rather than as unrelated events.
func parentForStep(pctx program.Context) *actiongraph.NodeID {
	if pctx.LastActionNode == "" {
		return nil
	}
	id := actiongraph.NodeID(pctx.LastActionNode)
	return &id
}

// HandleRequest is the entry point for any request.
//
// The order of the three routes is the whole of it:
//
//  1. A GOAL first. "Rename this file to Budget" is one request however many steps it
//     becomes, and it is a single clause — so a split-first router never reaches the goal
//     layer at all, and the phrase falls through to a parser that reads "rename X to Y"
//     as a request to rename a VARIABLE. That is not a hypothetical: it is what the
//     daemon did the first time this scenario ran live, because the goal layer lived
//     behind HandleProgram and production called this function.
//  2. A multi-clause request becomes a program.
//  3. Everything else takes the single-step path unchanged — byte for byte the behaviour
//     every earlier milestone was built and validated against.
//
// Routing on the goal parser rather than on the split, because a goal is defined by what
// it MEANS and a clause count says nothing about that.
func (p *Pipeline) HandleRequest(ctx context.Context, request string) Outcome {
	ctx = ensureBindings(ctx)

	if p.IsGoal(request) {
		return Collapse(request, p.HandleProgram(ctx, request))
	}

	clauses := program.Split(request)
	if len(clauses) <= 1 {
		return p.Handle(ctx, request)
	}

	prog, err := program.Decompose(request, p.parseWith(ctx))
	if err != nil {
		// The whole request is refused, and nothing ran. Reported as a failure with
		// the reason rather than as a clarification, because the problem is not that
		// the Director needs more information — it is that it will not do this.
		out := Outcome{Request: request, Status: directorapi.ResultFailed, Message: err.Error()}
		out.Stages = append(out.Stages, Stage{Name: "plan", Detail: err.Error()})
		out.Program = &ProgramOutcome{Program: prog, Status: directorapi.ResultFailed, Message: err.Error()}
		return out
	}
	prog.ID = program.ProgramID(p.nextID())
	return Collapse(request, p.RunProgram(ctx, prog, program.Context{}, 0))
}

// Collapse renders a program's result as a single Outcome.
//
// The service, the action graph and every client speak Outcome, and a program is one
// request from the user's point of view. The per-step detail is not lost — it is in
// Program.Steps, and the trace shows every step — but the headline is one status and
// one sentence, which is what "did that work?" wants.
func Collapse(request string, po ProgramOutcome) Outcome {
	out := Outcome{
		Request: request,
		Status:  po.Status,
		Message: po.Message,
		Program: &po,
	}
	// The last step's detail is what a caller needs most: for a completed program it
	// is the final verification, and for a stopped one it is the reason.
	if n := len(po.Steps); n > 0 {
		last := po.Steps[n-1]
		out.Intent = last.Intent
		out.World = last.World
		out.Resolution = last.Resolution
		out.Plan = last.Plan
		out.Policy = last.Policy
		out.Record = last.Record
		out.Node = last.Node
		// Numbered by the step.s REAL position in the program, not by its index in the
		// steps that happened to run. A resumed program runs only the tail, and
		// labelling the first of those "1/2" would tell the reader step 1 ran again.
		firstRan := po.StartedAt
		for i, s := range po.Steps {
			for _, st := range s.Stages {
				out.Stages = append(out.Stages, Stage{
					Name:   fmt.Sprintf("%d/%d %s", firstRan+i+1, len(po.Program.Steps), st.Name),
					Detail: st.Detail, OK: st.OK,
				})
			}
		}
	}
	return out
}
