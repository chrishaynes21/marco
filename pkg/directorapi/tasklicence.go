package directorapi

import "fmt"

// A PERSON'S REQUEST, STANDING AS AUTHORIZATION FOR THE TASK THEY ASKED FOR.
//
// # What was measured
//
// Three completed workflows on a live desktop, and every one of them needed a separate
// `director confirm yes` partway through:
//
//	make a new folder called Reports
//	  [4/4] plan    confirm "Reports" (high risk)
//	        confirm the confirmation could not be put to you (no answer within 1m30s)
//	  BLOCKED: step 4 of 4 did not complete
//
// The person asked for a folder. Marco opened the New menu, chose Folder and typed the name
// without asking, and then stopped to ask whether it might press Enter — because `confirm` is
// classified high-risk by its verb, and its own comment says exactly why that classification
// is right in isolation: *"committing is the archetypal irreversible act: it sends, buys, posts
// or deletes, and which one is not knowable from the verb."*
//
// Not knowable from the verb. Knowable from the TASK.
//
// # What this is, and the one decision it reaches
//
// A licence is the person's request carried forward to the steps of the procedure that request
// expanded into. It reaches exactly one decision: whether a step's generic high-risk prompt is
// already answered by the authorization the request itself gave.
//
// Everything else is untouched — grounding, freshness, foreground, cancellation, the
// capability ladder, the refusal when nothing can honestly be done. This does not make a step
// safe; it records that somebody already said yes to the task the step belongs to.
//
// # Four things it cannot do
//
//  1. **It cannot waive a confirmation the PLANNER asked for.** Risk has two sources: the verb,
//     and the control it is aimed at. "Confirm" on a field holding a folder name and "confirm"
//     on a button labelled "Delete account" are the same verb; the planner raises the second
//     and sets Plan.RequiresConfirmation, and a licence may never remove it. That is the rule
//     Plan.RequiresConfirmation was always documented to have — *"policy may add one; it may
//     never remove one"* — and this is the first thing that relies on it.
//  2. **It cannot cover a consequential procedure unanswered.** A procedure that declares
//     itself destructive, irreversible or externally visible is covered only once the GOAL gate
//     has been answered — one question about the whole task, which is what a person can
//     meaningfully agree to, rather than one per keystroke.
//  3. **It cannot travel.** It names the request, the procedure, the step and the application,
//     and it is a value on one call. A step of another procedure, in another application, or an
//     action nobody declared is not covered by it — so a discovered action, a replay, or a
//     retry that re-aimed somewhere else all ask as they did before.
//  4. **It cannot be inferred.** Nothing derives a licence from a world or a plan; it is minted
//     by the code that knows this step is a declared step of a task the person asked for, and
//     is absent everywhere else.
//
// See [[ADR-225-a-request-authorizes-the-task-it-asked-for]].
type TaskLicence struct {
	// Request is the person's own words. The authorization is theirs, and an account of
	// why a step ran without asking has to be able to quote it.
	Request string `json:"request"`
	// Goal and Procedure are what the request expanded into.
	Goal      string `json:"goal"`
	Procedure string `json:"procedure"`
	// Step is the phrase the procedure DECLARED for the step being authorized.
	//
	// Minted per step rather than per task, so a step nobody declared cannot be covered
	// by a licence that happens to be lying around: the absence of a phrase here IS the
	// absence of authorization.
	Step string `json:"step"`
	// Application is what the task was authorized against.
	Application string `json:"application"`
	// Declared is what the procedure said it would do, before it did any of it.
	Declared TaskEffects `json:"declared"`
	// Confirmed records that the goal-level question was put and answered yes.
	Confirmed bool `json:"confirmed,omitempty"`
}

// TaskEffects is a procedure's own declaration of what it will do.
//
// Declared by a hand-written procedure before it runs, which is what makes it usable as an
// envelope: it is a promise made in advance rather than a guess made from the steps.
type TaskEffects struct {
	Risk      RiskLevel `json:"risk"`
	Mutations int       `json:"mutations"`
	// Destructive, External and Irreversible are the three that put a task outside the
	// routine envelope whatever its risk level says.
	Destructive  bool `json:"destructive,omitempty"`
	External     bool `json:"external,omitempty"`
	Irreversible bool `json:"irreversible,omitempty"`
}

// Routine reports whether these effects are the ordinary, recoverable kind a request can
// authorize without a separate question.
//
// Three flags and a risk level, all declared in advance. Not a judgement about the step: a task
// that overwrites a file declares Destructive and is asked about once, however gentle its
// individual steps look.
func (e TaskEffects) Routine() bool {
	return !e.Destructive && !e.External && !e.Irreversible && e.Risk != RiskHigh
}

// Authorizes reports whether this licence covers a step, and says why or why not.
//
// `plannerAsked` is Plan.RequiresConfirmation: the planner's own request for a prompt, which
// no licence may remove. It is passed in rather than read from a Plan so that the one place
// this decision is made cannot be given a plan and quietly ignore that field.
//
// # What is NOT consulted, and why
//
// `actionConfirmation.Destructive` — which is `!SemanticActionKind.Reversible()`, and whose
// doc says what that means: an action is reversible only when the Director can NAME the
// inverse. `confirm` has no nameable inverse, so it is "destructive" by that measure, as is
// every commit keystroke there is. Keying on it would refuse the entire case this exists for.
//
// The consequential check is therefore the planner's: a DESTRUCTIVE-LOOKING CONTROL raises the
// risk and sets Plan.RequiresConfirmation, which no authorization may waive. That is how
// deletion actually arises in a procedure — "invoke the Delete menu item" — and it is caught.
//
// What remains is trust in the procedure's declaration, and that is the trust `goal.Safety`
// was built on rather than something this adds: "declared rather than derived from the expanded
// steps... only the procedure's author knows what pressing it MEANS". A hand-written procedure
// that submits a payment and declares itself routine is a bug in that procedure, and there is
// no measurement of the steps that would catch it — which is why procedures are hand-written
// and never generated.
//
// # Read by the confirmation gate, and never by policy
//
// `internal/director/policy` must not see this, and a source guard holds that
// (`TestPolicyNeverConsultsALicence`). The reason is stated there and it is the right one: a
// permission layer that could read a licence would be deciding risk from what the person SAID
// rather than from what the action DOES. So policy classifies the action exactly as it always
// did — `confirm` stays high-risk — and this answers a different question one tier up, in the
// gate that already asks it: has the person authorized this?
//
// That gate is also where "you already agreed to this" lives (`coveredByGoalConfirmation`), and
// the two are deliberately separate. One is an answer somebody gave; the other is a request
// somebody made.
func (l *TaskLicence) Authorizes(application string, plannerAsked bool) (bool, string) {
	switch {
	case l == nil || l.Request == "" || l.Procedure == "" || l.Step == "":
		return false, "no task authorized this step"
	case plannerAsked:
		// The control, not the verb, is what made this high-risk. "Confirm" on a
		// button labelled "Delete account" is not a step anybody authorized by asking
		// for a folder.
		return false, "the control this acts on calls for its own confirmation"
	case application != "" && l.Application != "" && application != l.Application:
		return false, fmt.Sprintf("the task was authorized against %s and this acts in %s",
			l.Application, application)
	case !l.Declared.Routine() && !l.Confirmed:
		return false, fmt.Sprintf("%q declares itself %s and has not been agreed to",
			l.Procedure, l.Declared.describe())
	}
	return true, fmt.Sprintf("%q authorized this: %s", l.Request, l.Step)
}

// describe names why a procedure is outside the routine envelope.
func (e TaskEffects) describe() string {
	switch {
	case e.Destructive:
		return "destructive"
	case e.Irreversible:
		return "irreversible"
	case e.External:
		return "externally visible"
	default:
		return "high-risk"
	}
}
