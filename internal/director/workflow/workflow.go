// Package workflow is what one demonstration taught, and what it would take to do it again.
//
// # The missing middle
//
// Marco has watched a person navigate and worked out what happened:
// [[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]] gives the screens,
// [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]] the learning unit,
// [[ADR-174-three-kinds-of-transition-and-what-a-brief-screen-was]] why each change happened, and
// [[ADR-180-finding-a-demonstrated-target-again]] whether a demonstrated control can be found
// again. What none of them produce is a thing a person can look at and say "yes, that is what I
// showed you — save it".
//
// The owner put the product back in the middle of it:
//
//	Marco is supposed to observe a user demonstrating a workflow, remember it, and reproduce it
//	later. […] The next milestone is not another observation benchmark. It is a workflow Marco
//	can remember and eventually perform without making me demonstrate it again.
//
// So: a Draft is the ordered record of a demonstration, with its evidence and its unresolved
// ambiguities attached to the steps they belong to. A Plan is what the Draft would do against the
// screen in front of it right now. Neither can press anything.
//
// # A draft is not a Play, and geometry is not identity
//
// Two separate refusals, and conflating them is the failure this package is shaped to avoid.
//
// The first is about authority. A Draft is a record of an observation; a Play is a program that
// runs. The owner:
//
//	Distinguish a learned draft from an executable Play. Do not silently promote uncertain screen
//	or target identities into durable authority.
//
// The second is about meaning, and it is the sharper one:
//
//	Marco can recognize a geometric target again. It has not yet proven that it recognizes the
//	same semantic control. […] I would retain the current geometric matcher, but keep its result
//	explicitly classified as geometric identity evidence, not semantic identity.
//
// A `matched` reacquisition says *a control of this shape is in this place*. If a list scrolls,
// the row that moves into the demonstrated row's place satisfies every geometric test there is —
// ADR-180 records that as a test asserting the wrong answer on purpose. So no step in this package
// ever becomes executable on its own evidence. `Support` names what stands behind a step, the best
// available value is `SupportGeometric`, and turning that into permission takes a human
// confirming that particular step. There is no code path that confirms a step without being told
// to, and `TestNoStepConfirmsItself` is what keeps it that way.
//
// # It cannot act
//
// Nothing here injects, and nothing here may. A planned action carries the point it resolved and
// hands it to `marcoexec`, which lowers it into a Marco program a host runs under the existing
// execution authorisation. Director does not touch a desktop; that boundary is older than this
// package and is not relaxed by it.
package workflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Support is what kind of evidence stands behind a step.
//
// Deliberately short. There is no `SupportSemantic`: nothing in Marco can currently establish that
// two readings of a rectangle are the same CONTROL rather than the same place, and inventing the
// word before the evidence exists is how a confident wrong answer gets a vocabulary.
type Support string

const (
	// SupportNone is a step that cannot be planned from what was observed.
	SupportNone Support = "none"
	// SupportGeometric is a control of this shape, in this place, on a screen whose
	// arrangement matches.
	//
	// The strongest thing this subsystem can currently say, and it is NOT a claim that the
	// control means what it meant. See the package note.
	SupportGeometric Support = "geometric"
)

// Step is one thing a demonstration taught.
//
// It carries its own evidence rather than pointing at a session that will be gone. The reason is
// the one `execute.ReplayPolicy` already states for deictic actions: a replay re-establishes its
// object from a snapshot and refuses when it cannot, because reading "this" again would resolve to
// whatever is there now.
type Step struct {
	// From is the screen the demonstration was on, To where it arrived.
	From observe.VisualScreenID `json:"from"`
	To   observe.VisualScreenID `json:"to"`
	// Cause is why the screen changed. Only a click is something Marco could repeat.
	Cause observe.TransitionCause `json:"cause"`
	// Target is the control that was pressed, empty for every other cause.
	Target observe.VisualTargetID `json:"target,omitempty"`
	// Seen is how many times the demonstration did this.
	Seen int `json:"seen"`

	// Readings is what the control was seen as, and Containers the larger regions that
	// also contained the press.
	//
	// The snapshot. Containers are kept because a reacquisition has to be able to say "the
	// panel is still here and the button is not" rather than matching the panel — see
	// ADR-180, where mutual centre containment turned out not to be a size test.
	Readings   []observe.Region `json:"readings,omitempty"`
	Containers []observe.Region `json:"containers,omitempty"`
	// look is the look this step's own press happened on, for ordering only.
	//
	// Unexported so it can never serialise: it is a session-local index and the saved
	// artifact carries `Position` instead, assigned at the save from the order here.
	look int
	// Outcome is the words read SOMEWHERE OTHER THAN this control that appeared when
	// it was pressed and stayed.
	//
	// What a replay has to see again where the screen itself does not change. Empty
	// when the session could not establish one, which is honestly different from an
	// outcome of no words. See observe/outcome.go.
	Outcome []string `json:"outcome,omitempty"`
	// Text is what the demonstration read ON the control, and TextLooks how many
	// looks of its screen contributed a reading of it.
	//
	// The one piece of evidence in a step a person would recognise. See
	// observe/controllabel.go for what may become one, and
	// [[Experiment-049-the-control-it-could-see-and-could-not-name]] for the live step
	// that refused with the word on the screen in front of it.
	Text      []string `json:"text,omitempty"`
	TextLooks int      `json:"text_looks,omitempty"`

	// Uncertain says the session could not tell this target from another on the same
	// screen: two identities that led to the same destination.
	Uncertain bool `json:"uncertain,omitempty"`
	// Unresolved is what a person would have to settle, in words.
	Unresolved []string `json:"unresolved,omitempty"`

	// confirmed is a human having agreed to THIS step. Unexported on purpose: it is set
	// by Confirm and by nothing else, so no amount of evidence can set it.
	confirmed bool
	by        string
}

// Confirmed reports whether a person agreed to this step.
func (s Step) Confirmed() bool { return s.confirmed }

// ConfirmedBy names who agreed, empty when nobody has.
func (s Step) ConfirmedBy() string { return s.by }

// Support is what stands behind this step.
func (s Step) Support() Support {
	switch {
	case s.Cause != observe.CauseClick:
		// NOT A THING TO REPEAT. Entering Free Play is a load completing: the person
		// clicked, a loading screen appeared, and the destination arrived with no
		// further input. A planner that pressed something here would be pressing at a
		// screen that was going to change anyway. ADR-174.
		return SupportNone
	case s.Target == "":
		return SupportNone
	case len(s.Readings) == 0:
		return SupportNone
	case s.To == "" || s.To == observe.VisualScreenUnknown:
		// NOWHERE TO VERIFY AGAINST. A step whose destination is unknown cannot be
		// checked after the fact, and an action nobody can verify is one nobody should
		// take.
		return SupportNone
	default:
		return SupportGeometric
	}
}

// Repeatable reports whether this step is something Marco could attempt at all.
//
// Separate from Confirmed, and both are required before anything is lowered. This one is about
// the evidence; the other is about permission.
func (s Step) Repeatable() bool { return s.Support() != SupportNone }

// Draft is the ordered record of one demonstration.
//
// Session-local, like the screens and targets it names. A draft that outlived its session would
// refer to screen identities that mean nothing in the next one — promoting those into durable
// Places is explicitly not authorised, so persisting a draft is the next decision rather than
// part of this one. See the package note and ADR-181.
type Draft struct {
	// Name is what a person would call it. "enter Free Play".
	Name string `json:"name"`
	// Steps in the order the demonstration walked them.
	Steps []Step `json:"steps"`
	// Notes is what is true of the draft as a whole rather than of one step.
	Notes []string `json:"notes,omitempty"`
}

// Executable reports whether every repeatable step has been confirmed.
//
// A draft with nothing repeatable is not executable, which is deliberate: `every step of none is
// confirmed` is true and useless, and a planner that read it as permission would run an empty
// workflow and report success.
func (d Draft) Executable() bool {
	any := false
	for _, s := range d.Steps {
		if !s.Repeatable() {
			continue
		}
		any = true
		if !s.confirmed {
			return false
		}
	}
	return any
}

// Confirm records that a person agreed to one step, by index.
//
// The only way a step becomes confirmed. It takes a name because an unattributed confirmation is
// indistinguishable from a default, and it refuses a step that is not repeatable — agreeing to a
// step whose evidence cannot support it would produce a confirmed thing that still cannot run,
// which reads as a defect in the planner rather than in the agreement.
func (d *Draft) Confirm(i int, by string) error {
	if i < 0 || i >= len(d.Steps) {
		return fmt.Errorf("workflow: step %d is not in a draft of %d", i, len(d.Steps))
	}
	if by == "" {
		return fmt.Errorf("workflow: a confirmation needs somebody to have given it")
	}
	if !d.Steps[i].Repeatable() {
		return fmt.Errorf("workflow: step %d is %s, so there is nothing to confirm: %s",
			i, d.Steps[i].Support(), d.Steps[i].why())
	}
	d.Steps[i].confirmed = true
	d.Steps[i].by = by
	return nil
}

// Confirmation says who agreed to this step, if anybody has.
//
// # Why this is readable at all
//
// So the agreement can be WRITTEN DOWN, and for no other reason. A durable workflow carries an
// audit record of who confirmed which step — that is a requirement of
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — and a store outside this
// package cannot read an unexported field.
//
// What it must never become is a way to restore one. `Confirm` is still the only thing that can
// make a step confirmed, it still needs a person's name, and a step read back out of a file
// comes back UNCONFIRMED however much audit it carries:
//
//	Stored confirmation history is audit metadata only. It is never reusable authorization.
//
// There is deliberately no setter. A reader can learn that somebody agreed; nothing can replay
// their agreement.
func (s Step) Confirmation() (by string, ok bool) { return s.by, s.confirmed }

// why explains in words what a step's support rests on or lacks.
func (s Step) why() string {
	switch {
	case s.Cause != observe.CauseClick:
		return fmt.Sprintf("the screen changed by %s rather than by a click, so there "+
			"is nothing to press — the response to this step is to wait and verify",
			s.Cause)
	case s.Target == "":
		return "the press landed on nothing the proposer found, so there is no control " +
			"to look for again"
	case len(s.Readings) == 0:
		return "no reading of the control was kept, so nothing could be matched"
	case s.To == "" || s.To == observe.VisualScreenUnknown:
		return "the destination is unknown, so the action could not be verified afterwards"
	default:
		return "a control of this shape was clicked here and the screen changed; that is " +
			"geometric evidence and not proof that the control means what it meant"
	}
}

// Learn builds a draft from what a session observed.
//
// `walk` is the order the demonstration visited its screens — `VisualScreens.Walk` — and `ev` the
// click evidence. The walk rather than the step list, because the steps are reported most-repeated
// first and a workflow is an ORDER; a draft assembled from the sorted steps would be a set of
// edges in popularity order, which is not a thing anybody demonstrated.
func Learn(name string, walk []observe.VisualScreenID, ev observe.ClickEvidence,
	reg []observe.TargetRecord) Draft {

	d := Draft{Name: name}
	byPair := map[[2]observe.VisualScreenID][]observe.RouteStep{}
	for _, st := range ev.Steps {
		k := [2]observe.VisualScreenID{st.From, st.To}
		byPair[k] = append(byPair[k], st)
	}
	records := map[observe.VisualTargetID]observe.TargetRecord{}
	for _, r := range reg {
		records[r.ID] = r
	}

	used := map[[2]observe.VisualScreenID]int{}
	for i := 0; i+1 < len(walk); i++ {
		k := [2]observe.VisualScreenID{walk[i], walk[i+1]}
		cands := byPair[k]
		if len(cands) == 0 {
			// THE WALK WENT SOMEWHERE THE EVIDENCE DOES NOT EXPLAIN. Recorded as
			// itself rather than skipped: a draft that silently dropped a step
			// would describe a shorter route than the one that was demonstrated,
			// and the person would be shown a workflow missing a move they made.
			d.Steps = append(d.Steps, Step{
				From: k[0], To: k[1], Cause: observe.CauseUnknown, Seen: 1,
				Unresolved: []string{"the demonstration went this way and no " +
					"recorded change explains it"},
			})
			continue
		}
		// TWO OCCURRENCES OF ONE PAIR CAN DIFFER IN CAUSE — ADR-174 measured a step
		// that was a click once and automatic the other time. So each visit takes the
		// next occurrence rather than the first, and the draft records what happened
		// on THAT pass.
		st := cands[min(used[k], len(cands)-1)]
		used[k]++
		step := Step{
			From: st.From, To: st.To, Cause: st.Cause,
			Target: st.Target, Seen: st.Count,
		}
		// AND WHAT THE PRESS DID, where the screen did not change enough to say.
		//
		// The words a demonstration read somewhere other than the control it
		// pressed, which appeared and stayed. For a step whose screens are the
		// SAME label at both ends this is the only evidence that anything
		// happened at all. See observe/outcome.go.
		step.Outcome = outcomeOf(ev, st.Target)
		// AND WHEN IT HAPPENED, for ordering against the panel steps below.
		if len(st.Looks) > 0 {
			step.look = st.Looks[0]
		}
		if rec, ok := records[st.Target]; ok {
			step.Readings = append([]observe.Region{}, rec.Where...)
			step.Containers = append([]observe.Region{}, rec.Containers...)
			step.Text = append([]string{}, rec.Text...)
			step.TextLooks = rec.TextLooks
			// A CONTROL NOBODY READ IS SAID SO, because the re-grounder's only
			// other source is the workflow's own title and a reader has to be
			// able to tell "unlabelled" from "not looked at".
			if len(step.Text) == 0 && st.Cause == observe.CauseClick {
				step.Unresolved = append(step.Unresolved,
					"nothing was read on this control, so a repair has "+
						"only the workflow's own name to go on")
			}
			step.Uncertain = len(rec.MaybeOneControlWith) > 0
			if step.Uncertain {
				step.Unresolved = append(step.Unresolved, fmt.Sprintf(
					"this session could not tell %s from %v on %s: they led to "+
						"the same place, so they may be one control read "+
						"two ways or two entries that open the same thing",
					st.Target, rec.MaybeOneControlWith, st.From))
			}
			if rec.Presses < 2 {
				step.Unresolved = append(step.Unresolved,
					"clicked once, so nothing independent confirms that this "+
						"control can be found again")
			}
		}
		if st.Cause != observe.CauseClick {
			step.Unresolved = append(step.Unresolved, step.why())
		}
		d.Steps = append(d.Steps, step)
	}
	// ── AND THE PRESSES THAT CHANGED SOMETHING WITHOUT CHANGING THE SCREEN ──
	//
	// A step has always needed a TRANSITION, which is why Ark's GRAPHICS click produced
	// none: it changes the panel under a persistent tab bar, the screen model records no
	// change, and the press is filed as inert. So the one demonstrated click in this
	// project with a corroborated button-level target was not in the procedure at all,
	// and no verifier could have helped — there was nothing to verify.
	//
	// Admitted here on ONE condition: the press has a VERIFIED local outcome. That is
	// positive evidence, persistent across the settled window, read outside the control
	// itself, on a screen that holds still — see observe/outcome.go for the four terms and
	// [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]] for the
	// measurement. An inert press WITHOUT one stays out: a click that did nothing
	// observable is not a step, and that is most of them.
	//
	// Both ends are the same screen, which is true and is what the replay path now
	// handles: such a step can never arrive and is finished by its own outcome.
	if panels := panelSteps(ev, records); len(panels) > 0 {
		d.Steps = mergeByLook(d.Steps, panels)
	}
	if len(d.Steps) == 0 {
		d.Notes = append(d.Notes, "the demonstration never settled on two screens in a "+
			"row, and no press changed anything that could be verified, so there is "+
			"no step in it")
	}
	return d
}

// Action is what a plan would do, with the evidence that resolved it.
//
// `At` is computed from the look in front of the planner and exists only inside one plan. The
// demonstration's own press position is not in a Step and is not reachable from here, which is the
// owner's instruction and also the only way a workflow survives a window that moved:
//
//	Do not turn original click coordinates into permanent replay targets.
type Action struct {
	Step int `json:"step"`
	// At is where the control is NOW, normalised to the window.
	At observe.PointerAt `json:"at"`
	// Where is the region it resolved to.
	Where observe.Region `json:"where"`
	// Expect is the screen that must follow, or the action was not what it looked like.
	Expect observe.VisualScreenID `json:"expect"`
	// Support names the evidence. Never more than geometric.
	Support Support `json:"support"`
	// Because says how it resolved.
	Because string `json:"because"`
}

// Refusal is a step that could not be planned, and why.
type Refusal struct {
	Step    int                     `json:"step"`
	Result  observe.ReacquireResult `json:"result,omitempty"`
	Because string                  `json:"because"`
}

// Plan works out what the draft would do next, given where it is and what it can see.
//
// `at` is the index of the step to attempt, `screen` the screen standing now and `regions` what
// the proposer found now. Exactly one of the two returns is set.
//
// It refuses more often than it acts, and every refusal names itself:
//
//   - the draft is not where it thought it was
//   - the step was never confirmed by a person
//   - the control cannot be found, or could be more than one thing
//   - the identity is one the session itself could not pin down
//
// `got` is the reacquisition outcomes for the look in front of the caller. Taking them rather than
// the correlator keeps this package unable to reach the press evidence at all: a planner that
// could read the original observations could read the original COORDINATES, and the owner's
// instruction is that those must never become permanent replay targets. Here they are not merely
// unused — they are out of scope.
func Plan(d Draft, at int, screen observe.VisualScreenID,
	got []observe.Reacquisition) (*Action, *Refusal) {

	if at < 0 || at >= len(d.Steps) {
		return nil, &Refusal{Step: at, Because: fmt.Sprintf(
			"step %d is not in a draft of %d", at, len(d.Steps))}
	}
	s := d.Steps[at]
	if !s.Repeatable() {
		return nil, &Refusal{Step: at, Because: s.why()}
	}
	if !s.confirmed {
		// PERMISSION IS NOT EVIDENCE. This is the refusal that makes a draft a draft,
		// and it comes before the matching so that an unconfirmed step cannot even be
		// shown a resolved point — which would read as "all it needs is a go-ahead".
		return nil, &Refusal{Step: at, Because: fmt.Sprintf(
			"step %d has not been confirmed by anybody. Its evidence is %s, which "+
				"says a control of this shape is in this place and does not say "+
				"the control means what it meant", at, s.Support())}
	}
	if screen != s.From {
		return nil, &Refusal{Step: at, Because: fmt.Sprintf(
			"this step was demonstrated on %s and the screen standing now is %s",
			s.From, screen)}
	}
	for _, r := range got {
		if r.Target != s.Target {
			continue
		}
		if r.Screen != s.From {
			// AN OUTCOME FROM A DIFFERENT SCREEN IS NOT EVIDENCE ABOUT THIS STEP.
			// The outcomes are computed by the caller, so this is the one thing
			// the caller could get wrong in a way that looks right: a target
			// identity is scoped to a screen, and the same id on another screen
			// is another control.
			return nil, &Refusal{Step: at, Because: fmt.Sprintf(
				"the outcome offered for %s was computed on %s and this step "+
					"is on %s", s.Target, r.Screen, s.From)}
		}
		if !r.Actionable() {
			return nil, &Refusal{Step: at, Result: r.Result, Because: r.Because}
		}
		return &Action{
			Step: at, Where: r.Where, Expect: s.To, Support: SupportGeometric,
			At: observe.PointerAt{
				X: r.Where.X + r.Where.Width/2,
				Y: r.Where.Y + r.Where.Height/2,
			},
			Because: r.Because,
		}, nil
	}
	return nil, &Refusal{Step: at, Result: observe.ReacquireUnobservable, Because: fmt.Sprintf(
		"%s was demonstrated on this screen and the matcher offered no outcome for "+
			"it at all", s.Target)}
}

// Verdict is what happened after an action.
type Verdict string

const (
	// VerdictArrived is the screen the step expected.
	VerdictArrived Verdict = "arrived"
	// VerdictElsewhere is a different screen: the action did something, and not this.
	VerdictElsewhere Verdict = "elsewhere"
	// VerdictWaiting is the screen has not changed yet, which is ordinary for a step
	// whose destination arrives by a load completing.
	VerdictWaiting Verdict = "waiting"
	// VerdictUnobserved is nobody could see. Not progress and not failure.
	VerdictUnobserved Verdict = "unobserved"
)

// Verify says whether an action went where it said it would.
//
// Nothing here is allowed to conclude success from having acted. The owner's older rule, preserved
// across this whole subsystem: delivery is not success, and self-report is not proof. The screen
// says whether the screen changed.
func Verify(a Action, from, now observe.VisualScreenID) Verdict {
	switch {
	case now == "" || now == observe.VisualScreenUnobserved || now == observe.VisualScreenUnknown:
		return VerdictUnobserved
	case now == a.Expect:
		return VerdictArrived
	case now == from:
		return VerdictWaiting
	default:
		return VerdictElsewhere
	}
}

// Unresolved is everything a person would have to settle, with the step it belongs to.
//
// Collected rather than counted: "three ambiguities" is not something anybody can act on, and the
// owner asked for the workflow to be shown "including what Marco can confidently reproduce and
// what still requires clarification".
//
// # One fact about one control is one line
//
// It used to be one line per step, so a control pressed on two visits had the same sentence
// printed about it twice — and the sentence is about the CONTROL, not about either step. A
// person reading two identical lines counts two problems and has one. The owner's rule:
//
//	Do not ask several small questions when one targeted question would resolve the meaningful
//	distinction.
//
// `workflowstore.Question` already does this on the other side of the same idea: it carries
// every step that presses the control "so a person can see that answering once settles all of
// them". So facts that name a target merge across the steps that share it, and facts about a
// transition — a screen that changed for no reason anybody pressed — stay keyed to the pair,
// because those really are different occurrences.
func (d Draft) Unresolved() []string {
	type subject struct {
		target   observe.VisualTargetID
		from, to observe.VisualScreenID
		msg      string
	}
	steps := map[subject][]int{}
	var order []subject
	for i, s := range d.Steps {
		for _, u := range s.Unresolved {
			k := subject{msg: u}
			if s.Target != "" {
				k.target = s.Target
			} else {
				k.from, k.to = s.From, s.To
			}
			if _, seen := steps[k]; !seen {
				order = append(order, k)
			}
			steps[k] = append(steps[k], i)
		}
	}
	var out []string
	for _, k := range order {
		at := steps[k]
		if len(at) == 1 {
			s := d.Steps[at[0]]
			out = append(out, fmt.Sprintf("step %d (%s -> %s): %s",
				at[0], s.From, s.To, k.msg))
			continue
		}
		where := make([]string, len(at))
		for i, n := range at {
			where[i] = strconv.Itoa(n)
		}
		out = append(out, fmt.Sprintf("steps %s (%s): %s",
			strings.Join(where, ", "), k.target, k.msg))
	}
	out = append(out, d.Notes...)
	sort.Strings(out)
	return out
}

// outcomeOf is the demonstrated local outcome for one target, when the session verified one.
//
// The BEST of the occurrences rather than the first or the union: a control pressed twice may
// have been read well once and badly once, and a union would make a replay look for words that
// only one visit saw. `Verified` is the four terms holding; an unverified outcome contributes
// nothing, which is how a step whose outcome could not be established comes out empty rather
// than coming out weak.
func outcomeOf(ev observe.ClickEvidence, target observe.VisualTargetID) []string {
	if target == "" {
		return nil
	}
	var best []string
	for _, o := range ev.Outcomes {
		if o.Target != target || !o.Verified() {
			continue
		}
		if len(o.Appeared) > len(best) {
			best = o.Appeared
		}
	}
	if len(best) == 0 {
		return nil
	}
	return append([]string{}, best...)
}

// panelSteps is the presses that changed something without changing the screen.
//
// One step per INERT press whose local outcome verified. Both ends are the same screen, which
// is what happened: the person pressed a control, the panel under it changed, and the screen
// model recorded no transition. See `Learn` for why this is admitted and on what condition.
//
// A press with no target is skipped whatever its outcome: a control with no identity has no
// boundary to find again, so there is nothing a replay could press. That is
// [[ADR-234-the-person-does-not-have-to-prove-the-click]] unchanged — the press is still kept
// as evidence, in the raw demonstration, and what it cannot be is a step.
func panelSteps(ev observe.ClickEvidence,
	records map[observe.VisualTargetID]observe.TargetRecord) []Step {

	verified := map[observe.VisualTargetID]observe.LocalOutcome{}
	for _, o := range ev.Outcomes {
		if o.Target == "" || !o.Verified() {
			continue
		}
		// THE BEST OF THE OCCURRENCES, as `outcomeOf` takes it: a control pressed
		// twice may have been read well once and badly once.
		if len(o.Appeared) > len(verified[o.Target].Appeared) {
			verified[o.Target] = o
		}
	}
	var out []Step
	for _, p := range ev.Inert {
		o, ok := verified[p.Target]
		if !ok || p.Target == "" || p.Screen == "" {
			continue
		}
		step := Step{
			From: p.Screen, To: p.Screen, Cause: observe.CauseClick,
			Target: p.Target, Seen: 1,
			Outcome: append([]string{}, o.Appeared...),
			look:    p.Look,
		}
		if rec, ok := records[p.Target]; ok {
			step.Readings = append([]observe.Region{}, rec.Where...)
			step.Containers = append([]observe.Region{}, rec.Containers...)
			step.Text = append([]string{}, rec.Text...)
			step.TextLooks = rec.TextLooks
		}
		// AND WHAT A READER HAS TO KNOW ABOUT IT. This step cannot be finished by
		// arriving anywhere, so a run that could not check its outcome can never
		// complete it — and that is worth saying in the file rather than discovering
		// at replay.
		step.Unresolved = append(step.Unresolved, fmt.Sprintf("this press changed "+
			"the window without changing the screen, so it is finished by its "+
			"own outcome (%d word(s)) and never by arriving somewhere",
			len(o.Appeared)))
		out = append(out, step)
	}
	return out
}

// mergeByLook puts the panel steps into the walk at the moment they were performed.
//
// A MERGE and not a sort. The walk's own order is not touched: `byPair` already walked it in
// the order the demonstration visited its screens, and an earlier version that re-sorted
// everything by look moved two legs of a twice-visited walk past each other — because a step
// whose transition recorded no look sorted as though it happened first.
//
// So each walk step keeps its place, a look is inherited forward across the ones that recorded
// none, and a panel step goes after every walk step that happened at or before it. A panel step
// whose screen the walk has already left is still placed in time rather than dropped: a replay
// that cannot reach it refuses it, and refusing a step somebody demonstrated is honest where
// silently omitting it is not.
func mergeByLook(walk, panels []Step) []Step {
	sort.SliceStable(panels, func(i, j int) bool { return panels[i].look < panels[j].look })
	looks := make([]int, len(walk))
	last := 0
	for i, s := range walk {
		if s.look > 0 {
			last = s.look
		}
		looks[i] = last
	}
	out := make([]Step, 0, len(walk)+len(panels))
	next := 0
	for i, s := range walk {
		for next < len(panels) && panels[next].look < looks[i] {
			out = append(out, panels[next])
			next++
		}
		out = append(out, s)
	}
	return append(out, panels[next:]...)
}
