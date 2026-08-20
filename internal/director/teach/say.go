package teach

import (
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What a person is told, and what a developer is told, from ONE value.
//
// Both readings are derived from the phase and the objects the phase points at — there is no
// second state to keep in step, which is the only reason a Watch panel and a Normal line can be
// trusted to describe the same session.
//
// The rule that separates them: Normal never names a subject id, a fingerprint, a verdict, a
// digest, a structural group or a recurrence count. Those are Director's backstage, and a play
// is something somebody reads.

// Say is the Normal-mode line: what Marco would tell the person in front of it.
func (s Session) Say() string {
	switch s.Phase {
	case WaitingForDemonstration:
		return "Go ahead — open what you want to show me, and do it normally."
	case EstablishingStart:
		return "Hold still a moment while I learn where we're starting."
	case ReadyForDemo:
		return "Okay — go ahead and show me."
	case Capturing:
		return "Watching."
	case EstablishingDestination:
		return "Got it. Let me make sure I recognise where that ended."
	case Evaluating:
		return "Thinking about what I saw."
	case NeedsAnotherExample:
		// TWO different situations reach this phase and they must not sound alike.
		//
		// With no example captured yet, the discovery pass has just worked out WHERE the
		// route goes and the capture is about to watch it properly. Nothing went wrong, and
		// saying "that wasn't clear enough" about a pass that succeeded is a lie that makes
		// a person doubt what they just did.
		if s.Examples == 0 {
			return "Good — I can see where that goes. Now let me watch you do it properly, " +
				"from where we started."
		}
		// With one captured, something in it was genuinely unreadable. Say WHAT: asking for
		// a repetition is asking somebody for their time, and "show me the whole thing
		// again" is the answer to every uncertainty at once, which makes it the answer to
		// none of them in particular.
		if why := unclear(s.Uncertain); why != "" {
			return why + " Could you show me that part again, from where we started?"
		}
		return "Something about that wasn't clear enough to act on. Could you show me " +
			"again, from where we started?"
	case ReadyToRehearse:
		return "I think I got it. Want me to try?"
	case Rehearsing:
		return "Trying it."
	case WaitingForStart:
		return "Okay — I'll try it when we're back there."
	case Naming:
		return "What do you call this screen?"
	case Lowering:
		if s.Attempt != nil && s.Attempt.Completed {
			return "That worked. Writing it down…"
		}
		return "Writing it down…"
	case Complete:
		return "I learned " + quote(s.Name) + ". You can ask me to do it later."
	case Cancelled:
		return "Stopped. I haven't kept anything from that."
	case Refused:
		// WHICH WINDOW Marco was watching, when the refusal is about not having seen
		// anything happen in it.
		//
		// # The live failure
		//
		// A person named a behaviour, pressed Start, walked to Settings and did the whole
		// thing. Marco said "I didn't see anything change, so there's nothing for me to
		// learn." It had fixed on File Explorer — the first window that was not Marco's own
		// surface — and watched that for the entire pass while the demonstration happened
		// somewhere else entirely.
		//
		// The sentence was true and unusable. Marco knew the answer the whole time and the
		// one sentence a person actually reads left it out, so the failure looked like "my
		// demonstration was not good enough" when it was "you were pointed at the wrong
		// window". Those need opposite responses.
		//
		// Only for this refusal: the others are about what was seen, and naming the window
		// there would be noise. See [[ADR-069-a-name-is-authored-and-can-be-taken-back]]
		// for the general rule this follows — Marco may not report about a thing it can
		// name without naming it.
		//
		// Deleting this must fail TestNothingChangedSaysWhichWindowItWatched.
		if s.Refusal == NothingChanged && s.Application != "" {
			return "I didn't see anything change in " + s.Application +
				" — that's the window I was watching. If you meant something else, " +
				"start again from there."
		}
		return s.Refusal.Say()
	}
	return ""
}

// Grounded is what Teach has decided about, and whether it can show you.
//
// # Why this is said at all
//
// Establishing the start is the most consequential thing a teach session does before anybody is
// asked to demonstrate anything, and until now it was invisible: Marco said "go ahead and show me"
// and the user had no way to find out that it had settled on the wrong screen. The same is true of
// the destination, and there it is worse — a wrong destination is discovered after two
// demonstrations and a rehearsal.
//
// Both readings are honest about their limits. A grounded start says what the highlight IS, and
// never that the start is those controls: the start is the screen, and what is drawn is the
// structure Marco recognises the screen by.
//
// Nothing here is a claim that grounding succeeded. A referent that cannot point still produces a
// line, because "I've decided where we're starting and I can't show you it" is something a person
// should be told rather than left to infer from an absent highlight.
func (s Session) Grounded() []GroundedEndpoint {
	var out []GroundedEndpoint
	if s.Start != "" {
		out = append(out, GroundedEndpoint{
			Label: "START", Role: observe.ReferentTeachStart,
			Say: groundingLine("START", s.StartReferent), Referent: s.StartReferent,
			Current: startLive(s.Phase),
		})
	}
	if s.Route.To != "" {
		out = append(out, GroundedEndpoint{
			Label: "DESTINATION", Role: observe.ReferentTeachDestination,
			Say:      groundingLine("DESTINATION", s.DestinationReferent),
			Referent: s.DestinationReferent,
			Current:  destinationLive(s.Phase),
		})
	}
	return out
}

// startLive and destinationLive are the phase windows during which each endpoint's
// presentation is a CURRENT claim rather than history.
//
// # Why a presentation has an owner at all
//
// A grounding highlight is ephemeral presentation of one decision, made at one moment — it
// is not durable semantic state, and a box drawn outside the moment it belongs to confirms
// whatever the person happens to be looking at now. Live, a bare `director teach` status
// read minutes later relaunched both highlights at wherever the window had moved to, and a
// SETTLED session went on offering its endpoints to every new reader forever.
//
// So each endpoint is live from the phase its decision lands in until the session's
// attention moves on, and NOTHING is live once the session settles — a failed, cancelled or
// completed Learn owns no presentation. The self-expiring surface is a backstop, never the
// lifecycle.
func startLive(p Phase) bool {
	return p == ReadyForDemo || p == Capturing
}

func destinationLive(p Phase) bool {
	switch p {
	case EstablishingDestination, NeedsAnotherExample, Evaluating, ReadyToRehearse:
		return true
	}
	return false
}

// GroundedEndpoint is one of the two decisions, with the sentence that goes with it.
//
// The label and the role travel WITH the referent so a surface never has to pair them up by
// position. Referent is nil when grounding was never attempted, and carries its own typed reason
// when it was attempted and could not point.
type GroundedEndpoint struct {
	Label    string
	Role     observe.ReferentRole
	Say      string
	Referent *observe.VisualReferent
	// Current says this endpoint's presentation belongs to the phase the session is IN.
	// A surface may draw only a current endpoint, and must dismiss what it drew when the
	// endpoint stops being one. The sentence is always available; the picture has an owner.
	Current bool
}

// groundingLine is one endpoint's sentence.
func groundingLine(label string, v *observe.VisualReferent) string {
	switch {
	case v == nil:
		return label + " — settled, but this Marco can't point at anything on your screen."
	case v.CanPoint():
		// The referent's own description, which says it is what the screen is RECOGNISED BY.
		// A wording that said "the start is these controls" would be a different and false
		// claim, and it is the one a surface drifts towards if this sentence lets it.
		return label + " — " + v.About + ". This is what I mean."
	}
	return label + " — settled, but " + cannotShow(v.Unavailable)
}

// cannotShow is why a settled endpoint cannot be pointed at, said as a teaching sentence.
//
// The REASONS are [observe.ReferentUnavailable]'s and there is no second vocabulary; only the
// wording is local. The shared sentences are question-shaped — "I know which structure I'm asking
// about" — and a teach session is not asking anything, so borrowing them verbatim would read as
// Marco being confused about what it is doing.
func cannotShow(u observe.ReferentUnavailable) string {
	switch u {
	case observe.ReferentNotAPart:
		return "there's nothing on that screen I can single out to show you."
	case observe.ReferentNotOnScreen:
		return "it isn't on screen at the moment, so I can't point to it."
	case observe.ReferentNothingWatched:
		return "I'm not watching anything right now, so I can't point to it."
	case observe.ReferentAnotherApplication:
		return "it belongs to a different application from the one on screen."
	case observe.ReferentCoordinatesUnreliable:
		return "I can't work out reliably where it is on this display, and I'd rather not " +
			"point at the wrong thing."
	}
	return "I can't point to it right now."
}

// Say is the Normal-mode explanation of one refusal.
//
// Plain sentences that say what Marco could and could not tell, in the terms a person can act on.
// The technical reason travels in Diagnostics and is never in here.
func (r Refusal) Say() string {
	switch r {
	case NoObservation:
		return "I couldn't watch — I lost sight of that window."
	case NothingChanged:
		return "I didn't see anything change, so there's nothing for me to learn."
	case DestinationNotRecognised:
		return "I saw the screen change, but I can't recognise where it ended well " +
			"enough to find it again later."
	case SeveralRoutes:
		return "Several things changed at once and I can't tell which one you meant. " +
			"Try showing me a single step."
	case GoalNotRemembered:
		return "I couldn't keep that name — it may already mean reaching somewhere else."
	case RouteNotRemembered:
		return "I can see the change, but I can't hold on to it well enough to be shown " +
			"again."
	case NotArmed:
		return "Something went wrong on my side — I wasn't watching for your example."
	case DemonstrationIncomplete:
		return "That example didn't finish where I expected, so I've set it aside."
	case RequiresTextEntry:
		return "I saw that this needed typing something. I don't keep what you type, so I " +
			"can't learn this one as a play you can re-run."
	case ActionNotAttributed:
		return "I saw where you ended up, but I couldn't tell what you did to get there."
	case NotAssessable:
		return "I couldn't make anything of that example."
	case DemonstrationsDisagree:
		return "Those two examples were different enough that I'm not ready to call them " +
			"the same thing."
	case EvidenceInsufficient:
		return "I watched, but what I saw doesn't add up to something I'd trust myself to " +
			"repeat."
	case ApplicationChanged:
		return "We left the application I was learning, so I stopped this example."
	case NameNotUsable:
		return "I can't use that as a name."
	case ExamplesExhausted:
		return "I've watched a couple of times and I'm still not confident enough to " +
			"call it learned."
	case RehearsalDeclined:
		return "Alright — I won't try it. I haven't written anything down."
	case RehearsalRefused:
		return "I decided not to try after all, so nothing happened."
	case RehearsalNotStarted:
		return "You said yes, and I couldn.t start the rehearsal. That is my end, not yours."
	case RehearsalFailed:
		return "I tried it and it didn't go the way I expected, so I haven't written it down."
	case NotLowerable:
		return "I can't write this down as something you'd be able to read and re-run."
	case NameRefused:
		return "I still don't have a name for that screen, so I can't write the play."
	case PlayNotRegistered:
		return "I wrote it down, and I couldn.t make it askable yet."
	case SaveFailed:
		return "I couldn't save it. Nothing was learned."
	case AnswerTimedOut:
		return "I waited a while and stopped. Nothing was written down."
	case NoTail:
		return "I can watch, but this Marco isn't set up to write anything down yet."
	}
	return "I've stopped."
}

// Watch is the Watch-mode panel: the same session, with the evidence underneath it.
//
// Every line here is read off an object that already owns the fact. Teach adds no number of its
// own, so a Watch panel cannot disagree with the session report.
func (s Session) Watch() []string {
	out := []string{"TEACHING  " + string(s.Phase), "  " + watchStage(s.Phase)}
	if s.Name != "" {
		out = append(out, "  asked for: "+quote(s.Name))
	}
	if s.Application != "" {
		out = append(out, "  application: "+s.Application)
	}
	if s.Start != "" {
		out = append(out, "  start: "+s.Start)
	}
	if s.Route.From != "" {
		out = append(out, "  route: "+s.Route.From+" → "+s.Route.To)
	}
	// The grounding, with its evidence rather than its sentence: which screen it was pinned to
	// and how many regions came back. A Watch panel that showed only the Normal line could not
	// distinguish "nothing was grounded" from "grounding returned nothing".
	for _, g := range []struct {
		label string
		state observe.ScreenStateID
		ref   *observe.VisualReferent
	}{
		{"start", s.StartState, s.StartReferent},
		{"destination", s.DestinationState, s.DestinationReferent},
	} {
		if g.state == "" && g.ref == nil {
			continue
		}
		line := "  " + g.label + " grounding: state=" + string(g.state)
		switch {
		case g.ref == nil:
			line += " (not grounded)"
		case g.ref.CanPoint():
			line += " regions=" + itoa(len(g.ref.Regions)) +
				" at_inference=" + itoa(g.ref.AtInference)
		default:
			line += " unavailable=" + string(g.ref.Unavailable)
		}
		out = append(out, line)
		// The resolver's own account, on its own line, whenever it was asked at all.
		//
		// The line above is the coarse reason and stays that way. This one is why
		// `coordinate_mapping_unreliable` was ever actionable: it has two causes that mean
		// opposite things — no trustworthy frame, or a group whose every member sits outside
		// the window — and a live Explorer refusal could not be attributed without it.
		if g.ref != nil {
			out = append(out, "    "+describeGrounding(g.ref.Diagnosis))
		}
	}
	if s.Examples > 0 {
		out = append(out, "  examples: "+itoa(s.Examples)+" of "+itoa(MaxExamples))
	}
	// What the navigation producer did. Printed whenever a pass has run, because a session
	// that attributed nothing and a session that was never watching produce the same silence
	// downstream and want opposite responses from a reader.
	if in := s.Input; in.Unavailable != "" || in.Received > 0 || s.Examples > 0 {
		out = append(out, "  input: "+describeInput(in))
	}
	if d := s.Demonstration; d != nil {
		out = append(out, "  demonstration: "+string(d.Reason)+
			" ("+itoa(d.Events)+" event(s), "+itoa(d.Checkpoints)+" checkpoint(s))")
		// A capture that never began has no steps to describe, and the three reasons it
		// might not have begun are the whole diagnosis. Printed only then, because once it
		// has begun the question no longer exists.
		if d.Checkpoints == 0 && !d.Waited.Empty() {
			out = append(out, "    waiting: "+describeWait(d.Waited))
		}
	}
	if a := s.Assessment; a != nil {
		out = append(out, "  assessment: "+string(a.Verdict)+" ["+reasonsOf(a)+"]")
	}
	if a := s.Attempt; a != nil {
		out = append(out, "  rehearsal: attempted="+boolWord(a.Attempted)+
			" completed="+boolWord(a.Completed)+" live="+boolWord(a.Live)+
			" ["+a.Terminal+a.Refusal+"]")
		// WHAT it did, step by step. `stopped_at_step` is true and useless on its own:
		// "it did not go as expected" with no way to see what it expected or what it got.
		for _, st := range a.Steps {
			line := "    step " + itoa(st.Step) + ": " + strings.Join(st.Intents, " ") +
				" → expected " + st.Expected
			if st.Observed != "" {
				line += ", observed " + st.Observed
			}
			out = append(out, line+" ["+st.Outcome+"]")
		}
	}
	if r := s.Readiness; r != nil {
		line := "  lowering: eligible=" + boolWord(r.Eligible)
		if r.Unnamed != "" {
			line += " unnamed=" + r.Unnamed
		}
		if len(r.Refusals) > 0 {
			line += " [" + strings.Join(r.Refusals, ", ") + "]"
		}
		out = append(out, line)
		// The generated Marco belongs in the developer reading and nowhere else. Normal mode
		// does not print a program at somebody.
		for _, l := range strings.Split(strings.TrimRight(r.Source, "\n"), "\n") {
			if l != "" {
				out = append(out, "    | "+l)
			}
		}
	}
	if sv := s.Saved; sv != nil {
		out = append(out, "  saved: "+sv.Name+" saved="+boolWord(sv.Saved)+
			" registered="+boolWord(sv.Registered))
	}
	if q := s.Question; q != nil {
		out = append(out, "  question: "+string(q.ID)+" in "+string(q.SessionID))
	}
	if s.Refusal != "" {
		out = append(out, "  refused: "+string(s.Refusal))
	}
	for _, d := range s.Diagnostics {
		out = append(out, "  · "+d)
	}
	return out
}

// watchStage is the one-line stage name, matching the phase rather than paraphrasing it.
func watchStage(p Phase) string {
	switch p {
	case WaitingForDemonstration:
		return "waiting for you to go to the application"
	case EstablishingStart:
		return "learning the starting place"
	case ReadyForDemo:
		return "ready — show me"
	case Capturing:
		return "watching your example"
	case EstablishingDestination:
		return "learning where that ended"
	case Evaluating:
		return "comparing what I saw with what I remember"
	case NeedsAnotherExample:
		return "waiting for another example"
	case ReadyToRehearse:
		return "waiting for permission to try once"
	case Rehearsing:
		return "rehearsing"
	case Naming:
		return "naming a screen"
	case Lowering:
		return "writing Marco"
	case Complete:
		return "learned"
	case Refused:
		return "refused"
	case Cancelled:
		return "cancelled"
	}
	return string(p)
}

// describeGrounding is the resolver's account of one grounding attempt, for the Watch panel.
//
// Developer-facing and deliberately mechanical: the funnel in the order the resolver applies it,
// so the step that lost the regions is the one where the numbers stop matching. It names no
// subject, reads no text and holds no coordinate — every value here is a count, a boolean, or a
// reason from the closed vocabulary.
//
// Deleting this must fail TestTheWatchPanelDistinguishesTheTwoUnreliableCauses.
// describeInput says what the navigation producer managed, in counts and closed reasons.
//
//	unavailable=…   nothing was watching at all; the refusal is ours, not the person's
//	received=0      it was watching and the keyboard never reached it
//	received>0 classified=0  events arrived and every one was discarded — `ignored` says why
//	dropped>0       the classifier fell behind; a diagnostic about Marco, never about the user
func describeInput(in observe.InputStats) string {
	if in.Unavailable != "" {
		return "unavailable=" + in.Unavailable
	}
	s := "received=" + itoa(in.Received) + " classified=" + itoa(in.Classified)
	if in.Conditional > 0 {
		s += " conditional=" + itoa(in.Conditional)
	}
	if in.Dropped > 0 {
		s += " dropped=" + itoa(in.Dropped)
	}
	// The pointer's own account. Present whenever a press was placed at all, because
	// "resolved to nothing" and "nothing was offered" are the two halves of the only
	// silence that decides whether a click can be learned.
	if in.PointerResolved+in.PointerUnnamed+in.PointerUnresolved > 0 {
		s += " pointer_resolved=" + itoa(in.PointerResolved) +
			" pointer_unnamed=" + itoa(in.PointerUnnamed) +
			" pointer_unresolved=" + itoa(in.PointerUnresolved) +
			" controls_offered=" + itoa(in.ControlsOffered)
	}
	for _, r := range sortedIgnores(in.Ignored) {
		s += " " + string(r) + "=" + itoa(in.Ignored[r])
	}
	return s
}

func sortedIgnores(m map[observe.IgnoreReason]int) []observe.IgnoreReason {
	out := make([]observe.IgnoreReason, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// unclear turns the blocking reasons into the sentence a person can act on.
//
// One sentence per reason, and the FIRST one only: a list of four things that were unclear is a
// wall, and the person can only fix one at a time anyway. Empty when nothing here has a plain
// wording, so the caller can fall back rather than print a vocabulary word at somebody.
func unclear(rs []observe.AssessmentReason) string {
	for _, r := range rs {
		switch r {
		case observe.ReasonIncompleteDemonstration:
			return "That didn't reach the place I was expecting."
		case observe.ReasonNoSteps:
			return "I saw where you ended up, but I couldn't tell what you did to get there."
		case observe.ReasonAmbiguousRun:
			return "There was a lot of moving around in there and I couldn't tell which " +
				"part was the actual step."
		case observe.ReasonBacktracking:
			return "It looked like you doubled back, so I'm not sure which way was the " +
				"real route."
		case observe.ReasonTransientCheckpoint:
			return "I know the route, but there's a screen along the way I can't reliably " +
				"recognise yet."
		case observe.ReasonNearCaptureBound:
			return "That ran long enough that I may not have seen the end of it."
		}
	}
	return ""
}

// describeWait says what a capture saw while it never started.
//
// The three failures it separates want three different responses from a person, which is why the
// panel prints them rather than the one sentence they all collapse into:
//
//	placed=0                     nothing was recognised at all — a recognition problem
//	elsewhere>0, on_start=0      the user was somewhere real that is not the start
//	unestablished>0              the start WAS seen, on a verdict too weak to begin on
func describeWait(w observe.ArmedWait) string {
	return "placed=" + itoa(w.Placed) +
		" unplaced=" + itoa(w.Unplaced) +
		" on_start=" + itoa(w.OnStart) +
		" elsewhere=" + itoa(w.Elsewhere) +
		" unestablished=" + itoa(w.Unestablished)
}

func describeGrounding(d observe.ReferentDiagnosis) string {
	return "watching=" + boolWord(d.Watching) +
		" frame=" + boolWord(d.FrameAvailable) + "/" + boolWord(d.FrameReliable) +
		" settled=" + boolWord(d.StateSettled) +
		" stands_for=" + boolWord(d.StandsForGroup) +
		" subject=" + boolWord(d.SubjectResolved) +
		" members=" + itoa(d.MembersTotal) +
		" present=" + itoa(d.MembersPresent) +
		" sized=" + itoa(d.MembersWithRegion) +
		" placeable=" + itoa(d.MembersPlaceable) +
		" whole_window=" + itoa(d.MembersWholeWindow) +
		" outside_window=" + itoa(d.MembersOutsideWindow) +
		" enclosing=" + itoa(d.MembersEnclosing) +
		" regions=" + itoa(d.Regions)
}

func quote(s string) string { return "\"" + s + "\"" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	b.Write(digits[i:])
	return b.String()
}
