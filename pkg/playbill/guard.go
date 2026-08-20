package playbill

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The admission guard.
//
// # Why a playbill checks itself
//
// Because it crosses a protocol boundary and is then rendered, logged, screenshotted
// and left on a monitor for hours. Everything else in the Director that leaves the
// process has an admission rule; a visibility surface is the one most likely to be
// asked to relax one, because the person asking is trying to debug something and the
// content would help.
//
// So the rule is enforced by the type rather than by discipline: Admit runs at the
// publication site, and a playbill that fails it is REPLACED by one that says so. Fail
// closed. A surface showing "the visibility record failed its own check" is a bug
// report; a surface showing a window title because the guard was skipped is a leak.
//
// # The three classes of content
//
// USER-SUPPLIED — a screen name somebody typed. Allowed, because they chose it, and
// it reaches memory only through a typed answer to a naming question.
//
// CLOSED VOCABULARY — recognition verdicts, stages, phases, interface terms, provider
// names, the Director's own generated sentences. Allowed, because the set of things
// they can say is fixed in the source and none of it came off a screen.
//
// PASSIVELY OBSERVED ARBITRARY CONTENT — window titles, raw OCR, accessibility labels,
// typed text, keys, screenshots, model prose. Refused, and there is no field on the
// View type that could hold one. That is checked structurally by the test beside this
// file, which walks the type graph rather than trusting any one value.
//
// # This guard does not widen the boundary and cannot
//
// Nothing here decides that something MAY be shown. It only refuses shapes that should
// never have been built. Whether a label may be read in the clear was decided by the
// Director's classifier long before this package existed, and there is deliberately no
// second copy of that rule here — two copies eventually disagree, and the one that
// disagreed quietly would be this one.

const (
	// MaxName bounds a user-supplied screen name. Generous for a name and far too
	// short for a sentence somebody pasted, a path, or a document title.
	MaxName = 64
	// MaxSentence bounds a Director-authored sentence.
	MaxSentence = 240
	// MaxTerms, MaxSources, MaxReadings and MaxLinks bound the lists.
	MaxTerms    = 24
	MaxSources  = 12
	MaxReadings = 12
	MaxLinks    = 12
	// MaxSilence bounds the closed reasons for not asking.
	MaxSilence = 8
)

// Admit reports whether this playbill may cross the boundary.
//
// Every failure names the field, because a guard that says "refused" and not "where"
// turns a one-line fix into an afternoon.
func (v View) Admit() error {
	if err := reach(v.Reach); err != nil {
		return err
	}
	if err := sentence("why", v.Why); err != nil {
		return err
	}
	if err := opaque("epoch", v.Epoch); err != nil {
		return err
	}
	if err := v.admitCurrent(); err != nil {
		return err
	}
	if err := v.admitSeeing(); err != nil {
		return err
	}
	if err := v.admitOffers(); err != nil {
		return err
	}
	if err := v.admitThinking(); err != nil {
		return err
	}
	if err := v.admitLearning(); err != nil {
		return err
	}
	if err := v.admitTeaching(); err != nil {
		return err
	}
	if err := v.admitDoing(); err != nil {
		return err
	}
	if err := v.admitQuestion(); err != nil {
		return err
	}
	if len(v.Recent) > MaxMoments {
		return fmt.Errorf("playbill: %d moments exceeds the bound of %d",
			len(v.Recent), MaxMoments)
	}
	for i, m := range v.Recent {
		if err := sentence(fmt.Sprintf("recent[%d].says", i), m.Says); err != nil {
			return err
		}
		if err := tone(m.Tone); err != nil {
			return err
		}
	}
	return v.admitDiagnostics()
}

func (v View) admitCurrent() error {
	c := v.Current
	switch c.Recognition {
	case Unobservable, Unknown, Candidate, Ambiguous, Contested, Recognised:
	default:
		return fmt.Errorf("playbill: %q is not a recognition verdict", c.Recognition)
	}
	// The application KEY, not a window title. Checked as a key shape: a normalised
	// executable name has no spaces and no punctuation beyond a dot or a dash, where
	// a title is a sentence with a document name in it.
	if err := key("current.application", c.Application); err != nil {
		return err
	}
	return userName("current.screen", c.Screen)
}

func (v View) admitSeeing() error {
	s := v.Seeing
	if len(s.Terms) > MaxTerms {
		return fmt.Errorf("playbill: %d interface terms exceeds the bound of %d",
			len(s.Terms), MaxTerms)
	}
	for _, t := range s.Terms {
		// A closed-vocabulary word. Anything with a space, a digit run or punctuation
		// in it is text that was read rather than a term that was classified.
		if err := vocab("seeing.terms", t); err != nil {
			return err
		}
	}
	if len(s.Sources) > MaxSources {
		return fmt.Errorf("playbill: %d sources exceeds the bound of %d",
			len(s.Sources), MaxSources)
	}
	for _, src := range s.Sources {
		if err := key("seeing.sources", src); err != nil {
			return err
		}
	}
	return nil
}

// admitOffers bounds the one section that carries observed interface text.
//
// # Why a name is allowed here at all
//
// Because it arrived through `observe.AdmittedTargetLabel` — the single policy the semantic
// target path uses ([[ADR-058-a-demonstrated-target-may-keep-its-name]]) — before it was ever
// put in this struct. This guard does not re-derive that decision; it enforces the SHAPE a
// name may have, so a caller that skipped the policy cannot smuggle a sentence, a document
// title or a paragraph of content through a field labelled "name".
//
// The role is held to the strict identifier rule for the same reason `seeing.sources` is: a
// window title arriving in a role field fails on its spaces before anybody reads it.
func (v View) admitOffers() error {
	o := v.Offers
	if len(o.Named) > MaxOffers {
		return fmt.Errorf("playbill: %d named offers exceeds the bound of %d",
			len(o.Named), MaxOffers)
	}
	check := func(field string, off Offer) error {
		if err := key(field+".role", off.Role); err != nil {
			return err
		}
		// A control's name is the same shape as a screen's: a word or a few, typed or
		// admitted, never a sentence.
		return userName(field+".name", off.Name)
	}
	for i, off := range o.Named {
		if err := check(fmt.Sprintf("offers.named[%d]", i), off); err != nil {
			return err
		}
	}
	return check("offers.focused", o.Focused)
}

func (v View) admitThinking() error {
	t := v.Thinking
	if len(t.Readings) > MaxReadings {
		return fmt.Errorf("playbill: %d readings exceeds the bound of %d",
			len(t.Readings), MaxReadings)
	}
	for i, r := range t.Readings {
		switch r.Standing {
		case Tentative, Supported, Disputed, Confirmed, Recalled, Withdrawn:
		default:
			return fmt.Errorf("playbill: %q is not a standing", r.Standing)
		}
		at := fmt.Sprintf("thinking.readings[%d]", i)
		if err := sentence(at+".says", r.Says); err != nil {
			return err
		}
		if err := sentence(at+".because", r.Because); err != nil {
			return err
		}
		if err := sentence(at+".settles", r.Settles); err != nil {
			return err
		}
		for j, b := range r.But {
			if err := sentence(fmt.Sprintf("%s.but[%d]", at, j), b); err != nil {
				return err
			}
		}
	}
	if len(t.Links) > MaxLinks {
		return fmt.Errorf("playbill: %d links exceeds the bound of %d",
			len(t.Links), MaxLinks)
	}
	for i, l := range t.Links {
		at := fmt.Sprintf("thinking.links[%d]", i)
		if err := screenRef(at+".from", l.From); err != nil {
			return err
		}
		if err := screenRef(at+".to", l.To); err != nil {
			return err
		}
	}
	return nil
}

func (v View) admitLearning() error {
	l := v.Learning
	switch l.Stage {
	case NotLearning, Observing, AwaitingEvidence, Asking, Capturing, Comparing,
		RehearsalOffered, Rehearsing, Rehearsed, PlayAvailable,
		Establishing, ShowMe, Naming, Saved:
	default:
		return fmt.Errorf("playbill: %q is not a learning stage", l.Stage)
	}
	if err := screenRef("learning.from", l.From); err != nil {
		return err
	}
	if err := screenRef("learning.to", l.To); err != nil {
		return err
	}
	if err := sentence("learning.because", l.Because); err != nil {
		return err
	}
	if len(l.Silence) > MaxSilence {
		return fmt.Errorf("playbill: %d silence reasons exceeds the bound of %d",
			len(l.Silence), MaxSilence)
	}
	for i, s := range l.Silence {
		if err := sentence(fmt.Sprintf("learning.silence[%d]", i), s); err != nil {
			return err
		}
	}
	return nil
}

func (v View) admitDoing() error {
	d := v.Doing
	switch d.Phase {
	case NotDoing, AwaitingPermission, CheckingStart, Performing, CheckingResult,
		Succeeded, Unverified, Failed, Refused, Cancelled:
	default:
		return fmt.Errorf("playbill: %q is not an execution phase", d.Phase)
	}
	// What is the person's OWN request, echoed back. Allowed for the same reason a
	// screen name is: they typed or said it. Bounded like a sentence.
	if err := sentence("doing.what", d.What); err != nil {
		return err
	}
	if err := sentence("doing.because", d.Because); err != nil {
		return err
	}
	if err := screenRef("doing.expected", d.Expected); err != nil {
		return err
	}
	return screenRef("doing.reached", d.Reached)
}

func (v View) admitQuestion() error {
	q := v.Question
	if q == nil {
		return nil
	}
	if strings.TrimSpace(q.ID) == "" {
		// A question with no id is a question nobody can answer, and a surface would
		// render it forever.
		return fmt.Errorf("playbill: a question must carry the id its answer routes by")
	}
	if err := opaque("question.id", q.ID); err != nil {
		return err
	}
	switch q.Wants {
	case WantsChoice, WantsName:
	default:
		return fmt.Errorf("playbill: %q is not an answer kind", q.Wants)
	}
	switch q.Via {
	case ViaConfirm, ViaClarify, ViaProposal:
	default:
		// A question with no route is a question a surface would have to invent a way
		// to answer, which is the one thing this representation must never invite.
		return fmt.Errorf("playbill: %q is not an existing response path", q.Via)
	}
	if err := sentence("question.asks", q.Asks); err != nil {
		return err
	}
	if err := screenRef("question.about", q.About); err != nil {
		return err
	}
	for _, a := range q.Answers {
		if err := vocab("question.answers", a); err != nil {
			return err
		}
	}
	return nil
}

func (v View) admitDiagnostics() error {
	d := v.Diagnostics
	if d == nil {
		return nil
	}
	for i, p := range d.Providers {
		at := fmt.Sprintf("diagnostics.providers[%d]", i)
		if err := key(at+".name", p.Name); err != nil {
			return err
		}
		if err := sentence(at+".why", p.Why); err != nil {
			return err
		}
		if p.Score != 0 && p.Metric == "" {
			// A bare number with no name is exactly how a provider's own score turns
			// into "confidence" in a reader's head.
			return fmt.Errorf("playbill: %s carries a score with no metric name", at)
		}
		if err := vocab(at+".metric", p.Metric); err != nil {
			return err
		}
	}
	for _, g := range d.Fusion.Degraded {
		if err := sentence("diagnostics.fusion.degraded", g); err != nil {
			return err
		}
	}
	if err := opaque("diagnostics.proposal", d.Proposal); err != nil {
		return err
	}
	if err := vocab("diagnostics.proposal_status", d.ProposalStatus); err != nil {
		return err
	}
	if err := vocab("diagnostics.verdict", d.Verdict); err != nil {
		return err
	}
	if err := vocab("diagnostics.structure_source", d.StructureSource); err != nil {
		return err
	}
	if err := vocab("diagnostics.authority", d.Authority); err != nil {
		return err
	}
	if err := sentence("diagnostics.rehearsal_outcome", d.RehearsalOutcome); err != nil {
		return err
	}
	return sentence("diagnostics.memory", d.Memory)
}

// Admitted returns this playbill if it passes, and a refusal playbill if it does not.
//
// The form a publication site uses. Returning the failure AS a playbill rather than as
// an error is deliberate: the caller is about to serialise something either way, and a
// path that could forget to check would be a path that shipped the unchecked one.
func (v View) Admitted() View {
	if err := v.Admit(); err != nil {
		out := Unavailable(v.Reach, "the visibility record failed its own check: "+err.Error())
		out.Epoch, out.UptimeMS = v.Epoch, v.UptimeMS
		return out.WithDigest()
	}
	return v
}

// ── shapes ────────────────────────────────────────────────────────────────────

func reach(r Reach) error {
	switch r {
	case Unreachable, Absent, Present:
		return nil
	}
	return fmt.Errorf("playbill: %q is not a reach", r)
}

func tone(t Tone) error {
	switch t {
	case "", Plain, Good, Doubt, Alarm, Muted, Accent:
		return nil
	}
	return fmt.Errorf("playbill: %q is not a tone", t)
}

// sentence accepts one line of Director-authored prose, bounded.
//
// One LINE, specifically. A multi-line string in a field like this is how a stack
// trace, a provider payload or a block of read text arrives somewhere it was never
// meant to be, and the length bound alone would not catch it.
func sentence(field, s string) error {
	if s == "" {
		return nil
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("playbill: %s is not valid text", field)
	}
	if utf8.RuneCountInString(s) > MaxSentence {
		return fmt.Errorf("playbill: %s is %d characters, over the %d bound",
			field, utf8.RuneCountInString(s), MaxSentence)
	}
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			return fmt.Errorf("playbill: %s spans more than one line", field)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("playbill: %s contains a control character", field)
		}
	}
	return nil
}

// userName accepts a name a PERSON chose. Short, one line, printable.
func userName(field, s string) error {
	if s == "" {
		return nil
	}
	if utf8.RuneCountInString(s) > MaxName {
		return fmt.Errorf("playbill: %s is %d characters, over the %d bound for a name "+
			"— a name that long is a sentence, and this field only ever holds a word "+
			"somebody typed", field, utf8.RuneCountInString(s), MaxName)
	}
	return sentence(field, s)
}

// screenRef accepts either a user-given screen name or one of the ordinary phrases the
// narrator substitutes when there is no name.
func screenRef(field, s string) error { return userName(field, s) }

// key accepts a normalised identifier: a provider name, an application key.
//
// Deliberately strict. This is the field a window TITLE would arrive in if somebody
// wired the wrong thing, and a title fails on its spaces before anyone reads it.
func key(field, s string) error {
	if s == "" {
		return nil
	}
	if len(s) > MaxName {
		return fmt.Errorf("playbill: %s is too long for an identifier", field)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return fmt.Errorf("playbill: %s = %q is not an identifier — "+
				"this field takes a normalised key, never a window title", field, s)
		}
	}
	return nil
}

// vocab accepts a word from a closed vocabulary.
func vocab(field, s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 48 {
		return fmt.Errorf("playbill: %s = %q is too long for a vocabulary word", field, s)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("playbill: %s = %q is not a closed-vocabulary word", field, s)
		}
	}
	return nil
}

// opaque accepts an identity whose only meaningful operation is equality.
func opaque(field, s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 96 {
		return fmt.Errorf("playbill: %s is too long for an opaque identity", field)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == ':' || r == '.':
		default:
			return fmt.Errorf("playbill: %s = %q is not an opaque identity", field, s)
		}
	}
	return nil
}

// admitTeaching holds the boundary on the explicit-teaching section.
//
// Three things are checked, and each of them is a way this section could become the leak
// every other section was built to avoid:
//
//   - `Asked` is the user's own words and travels as a sentence, bounded like every other
//     free string here. It is the only free text in this section.
//   - `Did` is the CLOSED navigation vocabulary. Not "text that looks like an intent" —
//     membership, checked here, because the underlying type is a string and a raw key
//     identity assigned to the right field would otherwise reach a screen.
//   - `Learned` is a play NAME, not a path. A file path on a status panel is somebody's
//     home directory on a status panel.
func (v View) admitTeaching() error {
	t := v.Teaching
	if err := sentence("teaching.asked", t.Asked); err != nil {
		return err
	}
	if err := sentence("teaching.because", t.Because); err != nil {
		return err
	}
	if err := screenRef("teaching.learned", t.Learned); err != nil {
		return err
	}
	if strings.ContainsAny(t.Learned, `/\`) {
		return fmt.Errorf("playbill: teaching.learned looks like a path, not a play name")
	}
	if len(t.Did) > MaxDidIntents {
		return fmt.Errorf("playbill: %d attributed actions exceeds the bound of %d",
			len(t.Did), MaxDidIntents)
	}
	for i, d := range t.Did {
		if !KnownIntent(d) {
			return fmt.Errorf(
				"playbill: teaching.did[%d] is %q, which is not a navigation meaning", i, d)
		}
	}
	if len(t.Progress) > MaxTeachSteps {
		return fmt.Errorf("playbill: %d teaching steps exceeds the bound of %d",
			len(t.Progress), MaxTeachSteps)
	}
	for i, s := range t.Progress {
		if err := sentence(fmt.Sprintf("teaching.progress[%d].name", i), s.Name); err != nil {
			return err
		}
		switch s.State {
		case StepPending, StepCurrent, StepDone, StepSkipped:
		default:
			return fmt.Errorf("playbill: %q is not a step state", s.State)
		}
	}
	// Completion is an ARTIFACT, and this is the one place a presentation could be handed
	// a lie about it. A section that says a play was learned and cannot name it is a
	// section that got its answer from a phase.
	if t.Stage() == Saved && t.Learned == "" {
		return fmt.Errorf("playbill: teaching says a play was saved and does not name one")
	}
	return nil
}

// Stage is the teaching section's own reading of where it has got to.
//
// Derived from the facts rather than carried beside them, so there is no second field to
// fall out of step with `Learned`, `Armed` and `Stopped`.
func (t Teaching) Stage() Stage {
	switch {
	case t.Learned != "":
		return Saved
	case t.Stopped:
		return NotLearning
	case t.Armed:
		return ShowMe
	case t.Active:
		return Establishing
	}
	return NotLearning
}

// NavigationMeanings is the closed vocabulary a presentation may be shown.
//
// The Director's own navigation intents. Held here as a set rather than imported because
// this package imports nothing — see the boundary note at the top of the file — and
// duplicating a CLOSED list of eight words is a smaller risk than opening the door.
// A meaning that is not on this list never reaches a screen, whatever produced it.
var NavigationMeanings = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"confirm": true, "back": true, "pause": true, "point": true,
}

// KnownIntent reports whether a string is a navigation meaning.
func KnownIntent(s string) bool { return NavigationMeanings[s] }
