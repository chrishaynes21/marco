package observe

import (
	"sort"
	"strings"
)

// Recognising evidence Marco has seen in an earlier session, and refusing to when it cannot.
//
// # The problem this solves
//
// A validated hypothesis dies with its session. The user is asked "is this a settings screen?",
// says yes, and tomorrow the application restarts, every state and track is renumbered, and Marco
// asks again as though the answer never happened. That is the same nagging the proposal policy
// prevents within a session and cannot prevent across them.
//
// # Why Subject.Fingerprint is not simply declared durable
//
// It has the right ingredients and equality on it would be wrong four ways, each measured
// against the code that produces it:
//
//   - `Recurrence` GROWS. It is the episode count, so the same screen has a different
//     fingerprint every time it recurs — the one thing identity must not do.
//   - Role counts are EXACT. One missed detection turns `button:5` into `button:4`, and the
//     detector misses one regularly enough that state-local presence exists to measure it.
//   - Only `possible_choice_group` carries an `Envelope`. Every state subject has none, so an
//     identity keyed on geometry would simply be absent for most hypotheses.
//   - Role composition ALONE collides. "Five buttons" describes a settings screen, a level
//     select, a save-file list and a confirmation dialog, in every application ever written.
//
// So the fingerprint stays the source of evidence and equality is replaced by a comparison that
// tolerates the first two problems and refuses to guess about the last two.
//
// # The bar: false non-recognition is preferable to false memory
//
// Attaching a user's "yes" to the wrong screen is worse than asking them again. Asking twice is
// a small annoyance; inheriting a confirmation onto a different screen is a wrong belief with a
// human signature on it, and nothing downstream would ever question it. So the matcher returns
// `insufficient` wherever it cannot distinguish, and `same` only where a real discriminator
// agrees.
//
// # Identity is not the semantic label
//
// "This is the same subject" and "this subject is settings" are separate claims, and the code
// keeps them separate: matching is over STRUCTURE, and the semantic knowledge is what a matched
// subject then carries. Keying identity on the label would make every settings screen in every
// application the same object.

// MatchVerdict is the discrete outcome of comparing current evidence with a remembered subject.
//
// Four values and no similarity score. A float would invite picking the highest — and two
// remembered subjects at 0.71 and 0.69 are not "the first one", they are a case where Marco
// cannot tell, which is exactly what `insufficient` is for.
type MatchVerdict string

const (
	// MatchSame is established equivalence: the structure agrees AND a real discriminator
	// agrees. Only this may inherit semantic knowledge.
	MatchSame MatchVerdict = "same"
	// MatchCandidate is structural agreement with nothing distinctive to confirm it. Plenty
	// of screens have five buttons. It may be reported and must NOT inherit validation.
	MatchCandidate MatchVerdict = "candidate"
	// MatchDifferent is a positive disagreement on some signal.
	MatchDifferent MatchVerdict = "different"
	// MatchInsufficient is "cannot tell" — no evidence, or several remembered subjects fit
	// equally well. Deliberately distinct from `different`: one means Marco knows this is a
	// new thing, the other means Marco does not know.
	MatchInsufficient MatchVerdict = "insufficient"
)

// Established reports whether the verdict may carry semantic knowledge forward.
func (v MatchVerdict) Established() bool { return v == MatchSame }

// StructureSignature is the durable, identity-bearing description of a subject.
//
// Everything here is normalised, window-relative and derived from the closed vocabularies. There
// is deliberately no session reference, no window generation, no process id, no absolute
// coordinate and no raw text — see the privacy note on Recollection.
type StructureSignature struct {
	// Subject is what kind of thing this is: a screen, a transition, a group.
	Subject SubjectKind `json:"subject"`
	// Roles is the structural composition. Compared with tolerance, never for equality.
	Roles map[string]int `json:"roles,omitempty"`
	// Members is how many tracked structures make it up.
	Members int `json:"members,omitempty"`
	// Envelope is normalised window-relative geometry, present only where the subject has
	// one. A resolution change or a moved window does not alter it; that is the whole
	// reason geometry is stored normalised everywhere else in this system.
	Envelope *Region `json:"envelope,omitempty"`
	// Terms are the generic interface concepts, from the closed vocabulary in terms.go.
	// They are the strongest discriminator available and the only thing that separates a
	// video settings screen from an audio one.
	Terms []InterfaceTerm `json:"terms,omitempty"`
	// TermsKnown says perception had text to classify. False means UNKNOWN.
	//
	// Load-bearing. Without it an empty Terms slice means both "this screen carries no
	// interface concepts" and "OCR could not run", and the matcher below reads a
	// remembered subject's terms against the empty set as a positive DISAGREEMENT —
	// concluding that a screen differs because Marco could not look at it.
	TermsKnown bool `json:"terms_known,omitempty"`

	// ── a TARGET's identity, and nothing else's ──────────────────────────────
	//
	// Present only for SubjectTarget, the way Envelope is present only where a subject has
	// geometry. A target is not a composition: a screen is recognised by what it is MADE OF,
	// and a target by what it is CALLED, in a place. So none of the fields above apply to it
	// and none of these apply to anything else.

	// Label is the word the person saw on the control.
	//
	// Perception-derived and admitted under an explicit Learn licence — see
	// [[ADR-068-the-theater-is-the-durable-semantic-world]] for the privacy boundary. It is
	// evidence, never the Audience's own word; a name somebody CHOOSES lives in `Called`,
	// like a screen's.
	Label string `json:"label,omitempty"`
	// Kind is what sort of thing it is, from a small semantic vocabulary — button, item,
	// field, menu, link, tab, checkbox, window.
	//
	// Deliberately not a provider's control-type list. A durable target that said
	// "ListItem" would be remembering how Windows described it, which is exactly the
	// provider leak this subject kind exists to avoid.
	Kind string `json:"kind,omitempty"`
	// Place is the durable subject id of the screen this target was grounded in.
	//
	// Part of IDENTITY rather than a relationship, because "Mouse" means different things on
	// different screens and a target with no place is a name floating free of anywhere it
	// could be found.
	Place string `json:"place,omitempty"`
}

// SignatureOf extracts the durable signature from a hypothesis.
//
// Note what is dropped: `Recurrence`, because it grows; and the session-local `Ref`, because it
// is a counter. Both are present on the fingerprint and neither may reach durable identity.
func SignatureOf(h Hypothesis) StructureSignature {
	f := h.Subject.Fingerprint
	sig := StructureSignature{
		Subject: h.Subject.Kind, Members: f.Members, Envelope: f.Envelope,
		TermsKnown: f.TermsKnown,
	}
	if len(f.Roles) > 0 {
		sig.Roles = make(map[string]int, len(f.Roles))
		for k, v := range f.Roles {
			sig.Roles[k] = v
		}
	}
	for _, t := range f.Terms {
		// Admission, again. The terms reaching durable storage must be from the closed
		// vocabulary — a store is the last place arbitrary text should be able to settle.
		if t.Known() {
			sig.Terms = append(sig.Terms, t)
		}
	}
	sort.Slice(sig.Terms, func(i, j int) bool { return sig.Terms[i] < sig.Terms[j] })
	return sig
}

// Tolerances for structural comparison.
const (
	// RoleCountTolerance is how many detections a role's count may differ by and still
	// describe the same structure.
	//
	// One. The detector misses a control often enough that state-local presence exists to
	// measure it, so exact counts would fail to recognise a screen because one button was
	// not seen in one frame. Two would start merging a four-item menu with a six-item one.
	RoleCountTolerance = 1
	// MemberTolerance is the same allowance for the member count.
	//
	// It governs GROUP subjects only. A state's fingerprint carries no member count since
	// [[ADR-041-a-screen-is-not-its-dominant-group]] — it borrowed one from its dominant
	// structural group, and that group's size depended on how much of the session had
	// happened, which made a screen unrecognisable while it was the only place a session had
	// seen.
	//
	// LOW test debt, recorded rather than fixed: no test now crosses this boundary, because
	// group identity is not exercised at a tolerance edge anywhere. Widening it to 64 changes
	// no test. It is not dead — a group IS its members and the allowance is real for them —
	// but a future change to group identity has no guard here.
	MemberTolerance = 1
	// EnvelopeIoU is how much two normalised envelopes must overlap to be the same place.
	//
	// 0.90, deliberately stricter than the tracker's 0.30 match threshold. That threshold
	// answers "may this detection continue that track" between consecutive frames a second
	// apart; this answers "is this the same structure I saw on another day", and a bar
	// loose enough for frame-to-frame jitter would merge neighbouring panels.
	EnvelopeIoU = 0.90
)

// Discriminating reports whether this signature carries anything that could ever establish
// identity: interface concepts that were actually read, or a normalised envelope.
//
// A signature without one can only ever reach `candidate`, so a store that kept records for such
// subjects would accumulate entries nothing can ever match — unbounded growth in exchange for
// nothing. Refusing to remember them is the honest bookkeeping, and it means "Marco forgot"
// exactly where "Marco could never have recognised it" was already true.
func (s StructureSignature) Discriminating() bool {
	// A TARGET discriminates by its name, which is the only thing it has. One with no label
	// could never be resolved again, so storing it would add a record nothing can ever match
	// — the same unbounded-growth-for-nothing the rule below refuses for a screen.
	if s.Subject == SubjectTarget {
		return strings.TrimSpace(s.Label) != "" && s.Place != ""
	}
	return (s.TermsKnown && len(s.Terms) > 0) || s.Envelope != nil
}

// CompareStructure decides whether current evidence describes a remembered subject.
//
// The order is: disagreement first, then discrimination. A signal that positively disagrees
// settles it as `different`; only once nothing disagrees does the question become whether
// anything distinctive AGREES.
// chromeRoles are the roles whose presence is a fact about PRESENTATION rather than about
// what the screen is.
//
// # Why there is exactly one, and why it is this one
//
// The test a role has to fail to be listed here: does its arrival tell a person they are
// somewhere else? A progress bar arriving says the screen started loading; a text field
// arriving says it now offers somewhere to type. Both are real events, and the comment on
// the role-set check names them deliberately. A SCROLL BAR arriving says the content is a
// few pixels taller than the space for it — or, on Windows 11, that the pointer is over the
// region, because the platform auto-hides them.
//
// # Measured, not assumed
//
// Five live Learn attempts against Windows Settings on 2026-08-17 minted five durable
// subjects for the same pages. Two recordings of the Home page agreed on their terms and on
// every shared role count (button 15 vs 16, inside RoleCountTolerance), and differed by
// exactly one thing:
//
//	run 2:  … pane=3              text=32 …   terms=[back+settings]
//	run 5:  … pane=3 scroll_bar=1 text=32 …   terms=[back+settings]
//
// A whole role had arrived, so `sameRoleSet` said `different`, so no endpoint resolved, so
// no edge formed, and Learn could not complete on mainstream software however well every
// layer above it worked. Worse, since the scroll bar follows the pointer, the act of
// demonstrating is itself what changed the screen's identity.
//
// # Why this is narrow on purpose
//
// This design's worst failure is over-merging — two screens with identical structure that
// differ only in what their text says. Nothing here touches that: terms are compared exactly
// as before, the envelope rule is unchanged, counts are unchanged, and a screen that differs
// by any role a person could act on is still a different screen. The whole widening is that
// two screens which are otherwise identical are no longer told apart by a scroll bar.
//
// The roles are still RECORDED — a signature says what was seen. They are not COMPARED.
// See [[ADR-062-a-scroll-bar-is-not-a-screen]].
var chromeRoles = map[string]bool{"scroll_bar": true}

// identityRoles drops the chrome from a role composition, without touching the original.
func identityRoles(in map[string]int) map[string]int {
	if len(in) == 0 {
		return in
	}
	// Almost every comparison has no chrome at all; only pay for a copy when it does.
	chrome := false
	for role := range in {
		if chromeRoles[role] {
			chrome = true
			break
		}
	}
	if !chrome {
		return in
	}
	out := make(map[string]int, len(in))
	for role, n := range in {
		if !chromeRoles[role] {
			out[role] = n
		}
	}
	return out
}

// CompareStructure is ExplainStructure with the explanation discarded.
//
// One implementation of screen identity, deliberately. It used to be the whole comparison, and the
// diagnostic that explains a mismatch was going to be a second walk of the same rules — two
// answers to "are these the same screen", which would eventually disagree about the very thing
// they exist to decide.
//
// Live, that mattered immediately: Home and System were recognised on revisit and Bluetooth &
// devices and Mouse were not, and the only account available was a guess about button counts. See
// ExplainStructure for what is now reported.
func CompareStructure(current, remembered StructureSignature) MatchVerdict {
	return ExplainStructure(current, remembered).Verdict
}

// Recall finds the remembered subject that current evidence describes.
//
// # Ambiguity is not resolved by ranking
//
// If more than one remembered subject matches, the answer is `insufficient` — not the best of
// them. Two subjects that both look like this one are a case where Marco cannot tell them apart,
// and picking the one whose numbers happened to be marginally closer would attach a user's
// answer to a coin toss.
func Recall(current StructureSignature, remembered []RememberedSubject) (RememberedSubject, MatchVerdict) {
	var established []RememberedSubject
	var candidates []RememberedSubject
	for _, r := range remembered {
		switch CompareStructure(current, r.Structure) {
		case MatchSame:
			established = append(established, r)
		case MatchCandidate:
			candidates = append(candidates, r)
		}
	}
	switch {
	case len(established) == 1:
		return established[0], MatchSame
	case len(established) > 1:
		// Several remembered subjects are equally good matches. Marco does not know which,
		// and saying so is the only honest answer available.
		return RememberedSubject{}, MatchInsufficient
	case len(candidates) == 1:
		return candidates[0], MatchCandidate
	case len(candidates) > 1:
		return RememberedSubject{}, MatchInsufficient
	}
	return RememberedSubject{}, MatchDifferent
}

func sameRoleSet(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sameTerms(a, b []InterfaceTerm) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[InterfaceTerm]bool, len(a))
	for _, t := range a {
		seen[t] = true
	}
	for _, t := range b {
		if !seen[t] {
			return false
		}
	}
	return true
}

// iou is the intersection over union of two normalised regions.
func iou(a, b Region) float64 {
	x1, y1 := max64(a.X, b.X), max64(a.Y, b.Y)
	x2 := min64(a.X+a.Width, b.X+b.Width)
	y2 := min64(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	union := a.Width*a.Height + b.Width*b.Height - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// compareTargets decides whether two target signatures describe the same thing.
//
// # Exact, deliberately
//
// Every other subject in this system is matched with tolerance, because a screen legitimately
// gains a scroll bar or loses a row and is still the same screen. A target has no such slack: its
// identity is a NAME, and a name is either the same word or a different one. Two buttons on one
// screen differing only in their label are two targets, and any tolerance at all would merge
// them.
//
// The place is part of it. "Mouse" on the Bluetooth screen and "Mouse" somewhere else are
// different targets — a name is only unique somewhere.
//
// Case and surrounding space are ignored, because they are properties of how a label was rendered
// rather than of what it says, and a provider that trims differently must not mint a second
// target for the same control.
func compareTargets(current, remembered StructureSignature) MatchVerdict {
	if !sameLabel(current.Label, remembered.Label) {
		return MatchDifferent
	}
	if current.Place != remembered.Place {
		return MatchDifferent
	}
	// KIND disagreeing is a positive disagreement; kind UNKNOWN on either side is not. A
	// demonstration that could not tell what sort of thing it was has said nothing about it,
	// and unknown is not false.
	if current.Kind != "" && remembered.Kind != "" && current.Kind != remembered.Kind {
		return MatchDifferent
	}
	// A target with no label could never be found again, so it is not a subject at all — see
	// Discriminating, which refuses to store one.
	if strings.TrimSpace(current.Label) == "" {
		return MatchInsufficient
	}
	return MatchSame
}

// sameLabel compares two labels the way a person would read them.
func sameLabel(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// ── why two structures did or did not match ───────────────────────────────────

// Disagreement is one identity-bearing fact on which two structures differ, or agree.
//
// Closed vocabulary and numbers. No labels, no text, no coordinates — the same rule every other
// record in this package follows, and there is nothing here that could break it because a
// signature never held any of that.
type Disagreement struct {
	// Field is what was compared: "kind", "role_set", a role name, "members", "terms",
	// "envelope".
	Field string `json:"field"`
	// Current and Remembered are what each side had, as a person reads it.
	Current    string `json:"current,omitempty"`
	Remembered string `json:"remembered,omitempty"`
	// Decisive says this is why the verdict is what it is, rather than a detail that
	// happened to differ within tolerance.
	Decisive bool `json:"decisive,omitempty"`
}

// StructureComparison is the whole of why one structure matched another, or did not.
//
// # Why the matcher explains itself
//
// Because the alternative was a second similarity algorithm written for a diagnostic, and two
// implementations of "are these the same screen" would eventually disagree about the very thing
// under investigation. The verdict and the explanation come from ONE walk of the same checks.
//
// It exists because live recognition failed per-screen and nobody could say which field did it:
// Home and System recognised on revisit, Bluetooth & devices and Mouse did not, and the only
// theory available was "probably the button counts" — a guess, about the one mechanism the whole
// system rests on.
type StructureComparison struct {
	Verdict MatchVerdict `json:"verdict"`
	// Why is every identity-bearing field, agreeing or not, in the order compared.
	Why []Disagreement `json:"why,omitempty"`
	// Distinctive says something positively identified this as the same thing, rather than
	// merely nothing having contradicted it. See the candidate verdict.
	Distinctive string `json:"distinctive,omitempty"`
}

// Decisive is the fields that actually settled the verdict.
func (c StructureComparison) Decisive() []Disagreement {
	var out []Disagreement
	for _, d := range c.Why {
		if d.Decisive {
			out = append(out, d)
		}
	}
	return out
}

// ExplainStructure compares two structures and says why the answer is what it is.
//
// THE canonical comparison. CompareStructure is this function with the explanation discarded, so
// there is exactly one implementation of screen identity and a diagnostic cannot drift from the
// decision it is describing.
//
// Deleting the shared implementation — reimplementing either half — must fail
// TestTheExplanationAndTheVerdictAreOneImplementation.
func ExplainStructure(current, remembered StructureSignature) StructureComparison {
	out := StructureComparison{}
	note := func(field, cur, rem string, decisive bool) {
		out.Why = append(out.Why, Disagreement{
			Field: field, Current: cur, Remembered: rem, Decisive: decisive,
		})
	}

	if current.Subject != remembered.Subject {
		note("kind", string(current.Subject), string(remembered.Subject), true)
		out.Verdict = MatchDifferent
		return out
	}
	if current.Subject == SubjectTarget {
		out.Verdict = compareTargets(current, remembered)
		note("label", current.Label, remembered.Label, out.Verdict != MatchSame)
		return out
	}

	currentRoles, rememberedRoles := identityRoles(current.Roles), identityRoles(remembered.Roles)
	if !sameRoleSet(currentRoles, rememberedRoles) {
		note("role_set", roleSetOf(currentRoles), roleSetOf(rememberedRoles), true)
		out.Verdict = MatchDifferent
		return out
	}
	// EVERY shared role is reported, agreeing or not, because "which number moved and by
	// how much" is the question a person hardening identity is actually asking.
	over := false
	for _, role := range sortedRoles(currentRoles) {
		c, r := currentRoles[role], rememberedRoles[role]
		beyond := abs(c-r) > RoleCountTolerance
		if beyond {
			over = true
		}
		if c != r {
			note(role, itoa(c), itoa(r), beyond)
		}
	}
	if over {
		out.Verdict = MatchDifferent
		return out
	}
	if abs(current.Members-remembered.Members) > MemberTolerance {
		note("members", itoa(current.Members), itoa(remembered.Members), true)
		out.Verdict = MatchDifferent
		return out
	}

	termsComparable := current.TermsKnown && remembered.TermsKnown
	termsAgree := termsComparable && sameTerms(current.Terms, remembered.Terms)
	if termsComparable && !termsAgree {
		note("terms", termsOfSignature(current), termsOfSignature(remembered), true)
		out.Verdict = MatchDifferent
		return out
	}
	if !termsComparable {
		note("terms", "not read", "not read", false)
	}
	termsDiscriminate := termsAgree && len(current.Terms) > 0

	geometryAgrees := false
	if current.Envelope != nil && remembered.Envelope != nil {
		if iou(*current.Envelope, *remembered.Envelope) < EnvelopeIoU {
			note("envelope", "different", "different", true)
			out.Verdict = MatchDifferent
			return out
		}
		geometryAgrees = true
	}

	switch {
	case termsDiscriminate:
		out.Verdict, out.Distinctive = MatchSame, "the interface concepts agree and say something"
	case geometryAgrees:
		out.Verdict, out.Distinctive = MatchSame, "the envelope agrees"
	default:
		out.Verdict = MatchCandidate
		out.Distinctive = "nothing disagrees, and nothing is distinctive enough to say " +
			"this is the same place"
	}
	return out
}

// roleSetOf is the role names, sorted, as one readable string.
func roleSetOf(roles map[string]int) string {
	names := sortedRoles(roles)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// sortedRoles is the role names in a stable order.
func sortedRoles(roles map[string]int) []string {
	out := make([]string, 0, len(roles))
	for name := range roles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// termsOfSignature is the interface concepts as one readable string.
func termsOfSignature(sig StructureSignature) string {
	if len(sig.Terms) == 0 {
		return "none"
	}
	words := make([]string, 0, len(sig.Terms))
	for _, t := range sig.Terms {
		words = append(words, string(t))
	}
	sort.Strings(words)
	return strings.Join(words, ", ")
}
