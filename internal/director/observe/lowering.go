package observe

import (
	"fmt"
	"sort"
	"strings"
)

// May what Marco learned be written down as a play?
//
// # The last judgement before the language
//
//	CandidateAssessment   what does the demonstration evidence support?
//	RehearsalJudgement    is that enough to ASK for one controlled experiment?
//	RehearsalEvidence     a whole route ran and ended where it was meant to
//	LoweringJudgement     may that be written down as ordinary Marco?     ← here
//
// Nothing here writes anything. It answers a question and hands over the smallest semantic
// description a play needs: ordered navigation meanings, in order, with repeats intact.
//
// # What it deliberately does not hand over
//
// No subject ids, no screen states, no digests, no window handles, no counts and no verdicts. A
// generated play describes WHAT IT DOES, not how Director came to believe it — see
// [[ADR-027-what-marco-learned-becomes-marco]]. If any of that reached the generator it would
// eventually reach the file, because a field that exists gets printed.

// LoweringRefusal is the CLOSED vocabulary of why a procedure may not be written down.
//
// Discrete, like every other refusal in this system. There is no lowering score: a play either
// says what the route does or it does not exist.
type LoweringRefusal string

const (
	// RefusalNotVerified is the default and the important one. A demonstration Marco has
	// only WATCHED is not a procedure it knows works, however consistent it looked.
	RefusalNotVerified LoweringRefusal = "not_verified"
	// RefusalNoRehearsal is a candidate that has never been tried.
	RefusalNoRehearsal LoweringRefusal = "never_rehearsed"
	// RefusalRehearsalIncomplete is an attempt that stopped short. A prefix is not a route.
	RefusalRehearsalIncomplete LoweringRefusal = "rehearsal_incomplete"
	// RefusalEvidenceStale is a rehearsal of a demonstration that has since been revised.
	// The attempt really happened; it is no longer about this.
	RefusalEvidenceStale LoweringRefusal = "evidence_stale"
	// RefusalEndpointUnknown is a route whose start or destination memory no longer holds.
	RefusalEndpointUnknown LoweringRefusal = "endpoint_unrecognised"
	// RefusalWrongApplication is a candidate belonging to another application.
	RefusalWrongApplication LoweringRefusal = "application_mismatch"
	// RefusalNoSteps is a procedure with nothing to do.
	RefusalNothingToDo LoweringRefusal = "no_steps"
	// RefusalRequiresTextEntry is a route through a screen the user typed on. Nothing was
	// retained, so there is nothing to write down — and the fix is not to start retaining.
	RefusalCannotSayText LoweringRefusal = "requires_text_entry"
	// RefusalNoTargetToName is a click Marco could not attribute to a control.
	//
	// # Why this is no longer called unresolved_pointer_target
	//
	// Because that name described the wrong thing and cost a live diagnosis. It was reported
	// for a demonstration in which BOTH clicks resolved perfectly — the producer named the
	// controls, the labels reached the step, the rehearsal invoked one and verified it — and
	// it still read as "Marco could not work out what you clicked". The refusal was actually
	// about what Marco could SAY: a play had no way to name a control durably.
	//
	// Now that it has one, the refusal means what its name says. A click reaches here only
	// when nothing was on offer under the pointer, or the control's label was withheld by the
	// admission rule — and in either case there is genuinely no name to write down.
	RefusalNoTargetToName LoweringRefusal = "no_target_to_name"
	// RefusalInexpressible is a semantic action Marco Core v1 has no sentence for.
	//
	// A LANGUAGE-EXPRESSION GAP, reported rather than worked around. The alternative is
	// inventing syntax to make lowering convenient, which is how a language stops being one
	// somebody reads. See [[Core#Governance]].
	RefusalInexpressible LoweringRefusal = "core_cannot_express"
	// RefusalScreenUnnamed is a route whose starting screen nobody has named.
	//
	// NOT a placeholder case. A play's actor and verb may carry provisional names because
	// naming a behaviour can wait; the screen name cannot, because it is EXECUTABLE meaning —
	// `do Screen's Showing with "…"` has to resolve to something memory knows, and
	// `"UnnamedScreen"` resolves to nothing. A play that cannot say where it begins does not
	// become durable.
	RefusalScreenUnnamed LoweringRefusal = "screen_unnamed"
)

// LoweringJudgement is whether a verified procedure may be written down, and what it would say.
//
// Derived and recomputed, never stored — the same discipline
// [[ADR-021-a-judgement-is-recomputed-not-recorded]] applies to every judgement here. A candidate
// that was lowerable last week may not be now, because a screen was contradicted or the
// demonstration was revised.
type LoweringJudgement struct {
	Relationship RelationshipRef `json:"relationship"`
	Application  string          `json:"application"`
	Eligible     bool            `json:"eligible"`
	// Refusals is why not, in the closed vocabulary.
	Refusals []LoweringRefusal `json:"refusals,omitempty"`
	// StartsOn is what the USER calls the screen this route begins on.
	//
	// Resolved from durable memory for the ACTUAL source subject, never taken from a caller:
	// a play that could be handed any name would be a play whose first line means whatever
	// somebody passed in.
	StartsOn ScreenName `json:"starts_on,omitempty"`
	// EndsOn is what the USER calls the screen this route is expected to arrive at.
	//
	// The same rule as StartsOn and for the same reason: resolved from durable memory for the
	// VERIFIED destination subject, never taken from a caller. A play whose final claim could
	// be handed in from outside would be a play that says whatever somebody passed in.
	//
	// It is the postcondition. Emitting every key successfully is not the same as the
	// application having gone where the play said it would.
	EndsOn ScreenName `json:"ends_on,omitempty"`
	// Unnamed is the durable subjects this route needs named before it can be written down,
	// in the order they should be asked about.
	//
	// SOURCE FIRST, then destination. Not an arbitrary tie-break: a reader who is asked about
	// two screens at once has to hold both in their head, and the first line of the play is
	// the one that makes the second question make sense. Naming the source, recomputing, and
	// then discovering the destination is the natural shape — no queue, no remembered
	// question, just the judgement being asked again.
	Unnamed []string `json:"unnamed,omitempty"`
	// Steps is the whole of what a play needs: one ordered run of ACTIONS per step of the
	// procedure, in order, with repeats intact.
	//
	// `[[confirm] [down down confirm]]` is two steps and four presses, and the second `down`
	// is load-bearing — a set or a dedup would leave the selection one row short of where the
	// demonstration went.
	Steps [][]LoweredAction `json:"steps,omitempty"`
}

// LoweredAction is one thing a saved play will do.
//
// # Why this is not just a navigation meaning
//
// Because a demonstration contains two kinds of thing a play can say, and until now only one of
// them could be written down. A keypress is a MEANING — the person confirmed, and which key that
// is belongs to the host. A click is aimed at a THING, and the thing has a name: Marco resolves
// the control at capture time and keeps the label on DemonstrationStep.Targets.
//
// Lowering used to refuse every click, because a play could only say meanings and a coordinate is
// not something a play can say. That was right about the coordinate and wrong about the click:
// the label was there all along, and this is the shape that lets a play carry it.
//
// Exactly one field is set. An action is a meaning or a named control, never both.
type LoweredAction struct {
	// Intent is the navigation meaning, for a keypress.
	Intent NavIntent `json:"intent,omitempty"`
	// Called is the control's admitted label, for a click that resolved to one.
	//
	// The DURABLE way to say which control. The accessibility source's own id identifies it
	// in the tree as it stands now and means nothing after a redraw; the label is what the
	// person saw, and the host resolves it against what is on screen when the play runs.
	Called string `json:"called,omitempty"`
	// Kind is what sort of thing it is, from the small semantic vocabulary. Empty when the
	// demonstration could not tell.
	Kind string `json:"kind,omitempty"`
}

// Invokes reports whether this action presses a named control rather than sending a meaning.
func (a LoweredAction) Invokes() bool { return a.Called != "" }

// navigation is the shorthand for a meaning, so a caller building steps says what it means.
func navigation(in NavIntent) LoweredAction { return LoweredAction{Intent: in} }

// invoking is the shorthand for a named target.
func invoking(called, kind string) LoweredAction {
	return LoweredAction{Called: called, Kind: kind}
}

// Presses is how many navigation meanings the whole play would contain.
func (j LoweringJudgement) Presses() int {
	n := 0
	for _, s := range j.Steps {
		n += len(s)
	}
	return n
}

// JudgeLowering decides whether one verified procedure may be written down as Marco.
//
// # Why the input is an ASSESSMENT rather than a flag
//
// `ProcedureCandidate.Verified` is always false and deliberately vestigial: verification is not a
// property of an observation. What counts is the assessment folded with stored rehearsal evidence
// — the route completed, the digest still matches, both endpoints are still recognised — which is
// recomputed on every read. See [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].
//
// So a caller must hand over an assessment that has ALREADY been folded. One that has not simply
// reports `not_verified`, which is the honest answer to "has anybody tried this".
func JudgeLowering(c ProcedureCandidate, a CandidateAssessment, top Topology,
	application string) LoweringJudgement {

	out := LoweringJudgement{Relationship: c.Relationship, Application: application}
	seen := map[LoweringRefusal]bool{}
	add := func(r LoweringRefusal) {
		if !seen[r] {
			seen[r] = true
			out.Refusals = append(out.Refusals, r)
		}
	}

	if application == "" || c.Application == "" ||
		!strings.EqualFold(c.Application, application) {
		add(RefusalWrongApplication)
	}
	// THE gate. Everything else here is about whether the procedure can be SAID; this is
	// whether it is KNOWN.
	//
	// It used to ask `a.Verified`, which only a rehearsal can set — so a route Marco had
	// watched somebody walk cleanly was refused here however good the evidence was, and Fast
	// Learn produced every durable thing except the artifact. See
	// CandidateAssessment.CleanlyObserved for the measurement and for why the two kinds of
	// knowing are still told apart everywhere the difference matters.
	//
	// Narrowing this back to `a.Verified` must fail
	// TestARouteMarcoOnlyWatchedCanStillBeWrittenDown.
	if !a.Writable() {
		add(RefusalNotVerified)
	}
	if len(c.Steps) == 0 {
		add(RefusalNothingToDo)
	}
	// The endpoints, checked here as well as inside the evidence fold. A play that starts on
	// a screen nobody can find is a play about nothing.
	if c.Start.Subject == "" || !subjectKnown(top, c.Start.Subject) ||
		!subjectKnown(top, c.Relationship.To) {
		add(RefusalEndpointUnknown)
	}
	// AND somebody has to have said what that screen is called.
	//
	// Derived from durable memory for the ACTUAL source subject — never taken from a caller.
	// A play that could be handed any name would be a play whose first line means whatever
	// somebody passed in, and `Screen's Showing with "…"` is executable meaning rather than a
	// label: it has to resolve to something memory knows.
	// AND the same for where it is expected to ARRIVE.
	//
	// The destination comes from the verified relationship — the same edge the rehearsal
	// completed and the same one the evidence digest is about. It is not read back off the
	// screen after lowering, and it is not something a caller may supply.
	//
	// A screen may be the source of one play, the destination of another and an intermediate
	// of a third. It is one remembered subject with one name; the procedure merely refers to
	// it. Nothing role-specific is stored anywhere. See [[ADR-032-a-play-says-where-it-ends]].
	// THE NAME, whoever it came from — see RememberedSubject.Name. Requiring the Audience.s
	// own word here made Marco refuse to write down a play about a screen it had already
	// named correctly, and offered no way to decline the question.
	//
	// Deleting the Name() call must fail TestAPlayMaySayTheNameMarcoWorkedOut.
	if s, ok := top.Subjects[c.Start.Subject]; ok && s.Name() != "" {
		out.StartsOn = ScreenName(s.Name())
	} else {
		add(RefusalScreenUnnamed)
		if c.Start.Subject != "" {
			out.Unnamed = append(out.Unnamed, c.Start.Subject)
		}
	}
	if s, ok := top.Subjects[c.Relationship.To]; ok && s.Name() != "" {
		out.EndsOn = ScreenName(s.Name())
	} else {
		add(RefusalScreenUnnamed)
		if c.Relationship.To != "" && c.Relationship.To != c.Start.Subject {
			out.Unnamed = append(out.Unnamed, c.Relationship.To)
		}
	}

	for _, s := range c.Steps {
		if s.RequiresTextEntry {
			add(RefusalCannotSayText)
		}
		if len(s.Intents) == 0 {
			add(RefusalNothingToDo)
			continue
		}
		run := make([]LoweredAction, 0, len(s.Intents))
		for i, in := range s.Intents {
			switch {
			case in == NavPoint:
				// A CLICK, and whether it can be written down depends on whether Marco
				// knew what it was aimed at.
				//
				// It always used to be refused, on the grounds that a coordinate is not
				// something a play can say. True — and it was never the coordinate that
				// had to be said. Marco resolves the control at capture time under the
				// demonstration licence and keeps its label on the step; what was
				// missing was a way for a play to name a control durably, which
				// `Control.Called` now is.
				//
				// So the refusal narrows to what it was always about: a click nobody
				// could attribute to a control. See RefusalNoTargetToName.
				called := targetAt(s.Targets, i)
				if called == "" {
					add(RefusalNoTargetToName)
					continue
				}
				// The KIND travels with the name, narrowed to Marco's own
				// vocabulary rather than the provider's. It is what lets a
				// resolver tell a button from a caption sharing a word.
				var kind TargetKind
				if i < len(s.Targets) {
					kind = TargetKindOf(s.Targets[i].Role)
				}
				run = append(run, invoking(called, string(kind)))
			case !in.Known():
				add(RefusalInexpressible)
			default:
				run = append(run, navigation(in))
			}
		}
		if len(run) == len(s.Intents) {
			out.Steps = append(out.Steps, run)
		}
	}
	if len(out.Steps) != len(c.Steps) {
		// Some step could not be said in full. Refusals above say which; this stops a
		// partial play — half a procedure is a different procedure.
		//
		// Belt AND braces, honestly: every path that skips a step also adds a refusal, and
		// `Eligible` below clears Steps whenever there is one. Removing this line changes
		// no behaviour today. It stays because the two guards answer different questions —
		// "was anything refused" and "is the play complete" — and a future step kind that
		// could be dropped without a refusal would silently ship half a procedure.
		out.Steps = nil
	}

	sort.Slice(out.Refusals, func(i, j int) bool { return out.Refusals[i] < out.Refusals[j] })
	out.Eligible = len(out.Refusals) == 0 && len(out.Steps) > 0
	if !out.Eligible {
		out.Steps = nil
	}
	return out
}

// DescribeLowering renders a lowering judgement for a person.
func DescribeLowering(j LoweringJudgement) []string {
	if !j.Eligible {
		out := []string{"writing this down: not yet"}
		for _, r := range j.Refusals {
			out = append(out, "  reason: "+string(r))
		}
		return out
	}
	return []string{
		"writing this down: yes",
		"  " + itoa(len(j.Steps)) + " step(s), " + itoa(j.Presses()) + " press(es)",
	}
}

// itoa keeps DescribeLowering free of a fmt import for two numbers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ── the one string a user writes ──────────────────────────────────────────────

// ScreenName is what a person calls a screen.
//
// # Why this is a type and not a string
//
// Because this repository has spent a dozen milestones making arbitrary observed text hard to
// persist, and this is the one deliberate exception. A named type with a single constructor means
// an OCR line, an accessibility label or a window title cannot reach durable memory by being
// assigned to the right variable — somebody has to write `UserSuppliedScreenName(...)`, which is
// greppable, reviewable, and obviously a claim about provenance rather than about content.
//
// The distinction is not what the string says. `the pause menu` typed by a person is allowed; the
// same words read off a screen are not. See [[ADR-031-the-user-names-the-stage]].
type ScreenName string

// MaxScreenNameLength bounds what somebody may call a screen.
//
// Generous for a name and far too small for a paragraph of captured text, which is the point.
const MaxScreenNameLength = 60

// UserSuppliedScreenName is the ONLY way to make a ScreenName.
//
// It exists to be typed at a call site a reviewer can find. Passing observed text through it is
// possible in the sense that any lie is possible; it is not possible by accident.
func UserSuppliedScreenName(s string) (ScreenName, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", fmt.Errorf("a screen needs a name")
	case len([]rune(s)) > MaxScreenNameLength:
		return "", fmt.Errorf("that is longer than a name; %d characters is the most",
			MaxScreenNameLength)
	}
	for _, r := range s {
		// Control characters are not something a person types when naming a place. They are
		// what arrives when captured text is passed off as an answer.
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("a screen name is words, not control characters")
		}
	}
	return ScreenName(s), nil
}

// String renders the name for a play's source.
func (n ScreenName) String() string { return string(n) }

// targetAt is the admitted label of the control the i-th event was aimed at, empty when there
// was none.
//
// Aligned by POSITION with the step's intents — see DemonstrationStep.Targets — and tolerant of a
// short or absent list, because a leg in which nothing resolved carries no targets at all.
func targetAt(targets []SemanticTarget, i int) string {
	if i < 0 || i >= len(targets) {
		return ""
	}
	if !targets[i].Named() {
		return ""
	}
	return targets[i].Label
}
