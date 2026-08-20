package observe_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Recognising a subject across sessions, and refusing to when the evidence cannot carry it.
//
// The pair at the top of this file is the core proof of the milestone: one scenario that MUST be
// recognised and one that MUST NOT, differing only in the things that ought to matter.

// settingsScreen is a five-control screen whose text says settings.
func settingsScreen() observe.StructureSignature {
	return observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 5, "icon": 2},
		Members:    5,
		Terms:      []observe.InterfaceTerm{observe.TermSettings, observe.TermControls},
		TermsKnown: true,
	}
}

// ── THE PAIR ──────────────────────────────────────────────────────────────────

// The same subject, a new session: different ephemeral ids, a missed detection, jittered
// geometry. It must be recognised.
func TestTheSameSubjectIsRecognisedAcrossSessions(t *testing.T) {
	remembered := observe.RememberedSubject{
		ID: "subj_a", Application: "unknown-game", Structure: settingsScreen(),
	}

	// Session B: the detector missed one icon and one control this time, and nothing about
	// the ephemeral identifiers survives anyway — they were never in the signature.
	later := settingsScreen()
	later.Roles = map[string]int{"button": 4, "icon": 2}

	if got := observe.CompareStructure(later, remembered.Structure); got != observe.MatchSame {
		t.Fatalf("verdict %q for the same screen observed a second time with one control "+
			"missed. Cross-session memory that cannot survive a single missed detection "+
			"will never fire in practice — the detector misses one regularly enough that "+
			"state-local presence exists to measure it", got)
	}
}

// Two screens that look alike and are not the same thing. It must NOT merge them.
//
// This is the failure that would matter most: attaching a user's "yes, that's settings" to the
// audio page because both are five buttons in a column. Asking again is a small annoyance; a
// wrong belief with a human signature on it is not.
func TestTwoSimilarSubjectsAreNotMerged(t *testing.T) {
	video := settingsScreen()
	video.Terms = []observe.InterfaceTerm{observe.TermSettings, observe.TermDisplay}

	audio := settingsScreen()
	audio.Terms = []observe.InterfaceTerm{observe.TermSettings, observe.TermAudio}

	if got := observe.CompareStructure(video, audio); got == observe.MatchSame {
		t.Fatal("two settings sub-screens with identical structure and different text were " +
			"merged. Every settings page in an application has the same shape; the text is " +
			"the only thing that separates them, and collapsing them would attach one " +
			"screen's confirmation to another")
	}
}

// Structure alone is never enough to establish identity.
//
// "Five buttons" describes a settings screen, a level select, a save-file list and a
// confirmation dialog. Without a discriminator the honest verdict is `candidate`, and a
// candidate may not inherit a user's answer.
func TestStructureAloneIsOnlyACandidate(t *testing.T) {
	bare := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 5}, Members: 5,
	}
	got := observe.CompareStructure(bare, bare)
	if got == observe.MatchSame {
		t.Error("a screen with five buttons and nothing distinctive was treated as an " +
			"established match. Nearly every application has one")
	}
	if got != observe.MatchCandidate {
		t.Errorf("verdict %q, want candidate", got)
	}
	// And a candidate carries no validation.
	rec := observe.Recollection{Verdict: got, Subject: observe.RememberedSubject{
		Knowledge: []observe.SemanticKnowledge{{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}},
	}}
	if _, ok := observe.RecalledValidation(rec, observe.PossibleSettingsLikeState); ok {
		t.Error("a candidate match inherited a user's confirmation")
	}
}

// ── what must not affect identity ─────────────────────────────────────────────

// Geometry jitter within tolerance is the same place; a moved panel is not.
func TestGeometryToleranceRecognisesJitterAndRejectsAMove(t *testing.T) {
	at := func(x, y float64) observe.StructureSignature {
		r := observe.Region{X: x, Y: y, Width: 0.172, Height: 0.160}
		return observe.StructureSignature{
			Subject: observe.SubjectGroup, Roles: map[string]int{"button": 4},
			Members: 4, Envelope: &r,
		}
	}
	if got := observe.CompareStructure(at(0.4140, 0.4370), at(0.4143, 0.4372)); got != observe.MatchSame {
		t.Errorf("verdict %q for a panel that moved by three thousandths — that is detector "+
			"jitter, not a different screen", got)
	}
	if got := observe.CompareStructure(at(0.414, 0.437), at(0.700, 0.437)); got == observe.MatchSame {
		t.Error("a panel on the other side of the window was treated as the same place")
	}
}

// A whole new role means a different screen.
func TestAScreenThatGainsARoleIsDifferent(t *testing.T) {
	with := settingsScreen()
	with.Roles = map[string]int{"button": 5, "icon": 2, "progress_bar": 1}
	if got := observe.CompareStructure(with, settingsScreen()); got == observe.MatchSame {
		t.Error("a screen that gained a progress bar was treated as the same screen")
	}
}

// A different kind of subject is never the same subject.
func TestASubjectKindMismatchIsDifferent(t *testing.T) {
	group := settingsScreen()
	group.Subject = observe.SubjectGroup
	if got := observe.CompareStructure(group, settingsScreen()); got != observe.MatchDifferent {
		t.Errorf("verdict %q comparing a group with a screen", got)
	}
}

// Losing a term is a different subject, not a jittered one.
//
// Deliberately strict. A screen whose text no longer says `audio` may genuinely be a different
// page, and guessing wrong attaches the wrong answer.
func TestLosingATermIsNotTheSameSubject(t *testing.T) {
	fewer := settingsScreen()
	fewer.Terms = []observe.InterfaceTerm{observe.TermSettings}
	if got := observe.CompareStructure(fewer, settingsScreen()); got == observe.MatchSame {
		t.Error("a screen that stopped showing one of its two interface concepts was still " +
			"treated as established. That may be a different page")
	}
}

// ── ambiguity ─────────────────────────────────────────────────────────────────

// When several remembered subjects fit equally, the answer is "cannot tell".
//
// Not "the closest". Two subjects that both look like this one are a case where Marco does not
// know, and ranking them would attach a user's answer to a coin toss.
func TestAmbiguityReturnsInsufficientRatherThanTheClosest(t *testing.T) {
	one := observe.RememberedSubject{ID: "subj_1", Structure: settingsScreen()}
	two := observe.RememberedSubject{ID: "subj_2", Structure: settingsScreen()}

	_, verdict := observe.Recall(settingsScreen(), []observe.RememberedSubject{one, two})
	if verdict != observe.MatchInsufficient {
		t.Errorf("verdict %q with two equally good matches, want insufficient", verdict)
	}
}

// Nothing remembered is `different`, which is not the same as `insufficient`.
func TestNothingRememberedIsDifferentNotInsufficient(t *testing.T) {
	if _, v := observe.Recall(settingsScreen(), nil); v != observe.MatchDifferent {
		t.Errorf("verdict %q against an empty memory, want different — Marco knows this is "+
			"new, which is not the same as being unable to tell", v)
	}
}

// ── provenance ────────────────────────────────────────────────────────────────

// Observation-only knowledge never becomes a user's answer.
//
// A record existing is not a person agreeing. Persisting a hypothesis Marco formed and never
// asked about, then reading it back as validation, would manufacture a confirmation nobody gave.
func TestObservedOnlyKnowledgeIsNotValidation(t *testing.T) {
	rec := observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{Knowledge: []observe.SemanticKnowledge{{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeObserved,
		}}},
	}
	if _, ok := observe.RecalledValidation(rec, observe.PossibleSettingsLikeState); ok {
		t.Error("a hypothesis that was merely observed and stored came back as user " +
			"validation. Nobody was ever asked")
	}
}

// A remembered contradiction comes back as a contradiction.
func TestARememberedContradictionSurvivesAsOne(t *testing.T) {
	rec := observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{Knowledge: []observe.SemanticKnowledge{{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeContradicted,
		}}},
	}
	v, ok := observe.RecalledValidation(rec, observe.PossibleSettingsLikeState)
	if !ok {
		t.Fatal("a remembered contradiction produced no validation at all")
	}
	if v.Response != observe.ResponseContradicted {
		t.Errorf("response %q, want contradicted. A correction the system forgets overnight "+
			"is a correction the user has to give again every day", v.Response)
	}
}

// A remembered decline is not validation, and is not a denial.
func TestARememberedDeclineIsNotValidation(t *testing.T) {
	rec := observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{Knowledge: []observe.SemanticKnowledge{{
			Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeDeclined,
		}}},
	}
	if _, ok := observe.RecalledValidation(rec, observe.PossibleMenuLikeState); ok {
		t.Error("a decline came back as validation. It is a decision not to answer")
	}
}

// ── what may become durable ───────────────────────────────────────────────────

// The signature drops everything ephemeral, including the growing recurrence count.
func TestTheDurableSignatureDropsEphemeralEvidence(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	h.Subject.Ref = "state_7"
	h.Subject.Fingerprint.Recurrence = 3

	first := observe.SignatureOf(h)

	// The same screen, later in the session: renumbered and seen far more often.
	h.Subject.Ref = "state_19"
	h.Subject.Fingerprint.Recurrence = 400
	second := observe.SignatureOf(h)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("the durable signature changed when only the session-local id and the "+
			"recurrence count changed:\n  %+v\n  %+v", first, second)
	}
	if observe.CompareStructure(first, second) != observe.MatchSame {
		t.Error("the same screen stopped matching itself as its episode count grew")
	}
}

// THE privacy guard, applied recursively to everything that becomes durable.
//
// The same rule the input trace guards apply, on a store that outlives every session: if a field
// could contain arbitrary captured text, it must not exist.
func TestNothingCapturedCanReachDurableMemory(t *testing.T) {
	forbidden := []string{
		"keycode", "scancode", "rawkey", "rune", "character", "deviceid",
		"screenshot", "pixels", "image", "frame", "title", "label", "text",
		"username", "account", "path", "filename", "window", "process", "pid",
		"generation", "coordinate",
	}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
			rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			if rt.Kind() == reflect.Map {
				walk(rt.Key(), path+"[key]")
			}
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			here := path + "." + f.Name
			name := strings.ToLower(f.Name)
			for _, bad := range forbidden {
				if !strings.Contains(name, bad) {
					continue
				}
				if _, allowed := durableText[here]; allowed {
					continue
				}
				t.Errorf("%s (%s) could hold captured content, and this type is "+
					"written to a file that outlives every session. Durable memory "+
					"holds structure and closed vocabularies — nothing read off a "+
					"screen and nothing the user typed.\nIf this field is a "+
					"deliberate exception it belongs in durableText, with the "+
					"licence that admits it written down.", here, f.Type)
			}
			walk(f.Type, here)
		}
	}
	walk(reflect.TypeOf(observe.RememberedSubject{}), "RememberedSubject")
	// The durable TOPOLOGY takes the same sweep. It is written to the same file, outlives
	// the same sessions, and is the newest place a key identity or a screen coordinate could
	// have been smuggled in — a navigation run is exactly the shape that invites it.
	walk(reflect.TypeOf(observe.RememberedRelationship{}), "RememberedRelationship")
	// What a rehearsal PROVED is durable too — same file, same lifetime. It carries endpoints,
	// digests and a per-step outcome, and it is the newest durable evidence type, so it is the
	// most likely place a future field could smuggle a window title, a label or a process id in.
	walk(reflect.TypeOf(observe.RehearsalEvidence{}), "RehearsalEvidence")
}

// The durable-memory guard is load-bearing: a forbidden field added to rehearsal evidence is
// caught, it does not pass silently.
//
// Not an assertion that the current struct happens to be clean — that is what the test above does.
// This plants the careless future edit (a window-title string on a RehearsalEvidence-shaped
// record) and proves the same name-based scan the guard uses flags exactly it. If this ever stops
// failing for the planted type, the guard has gone hollow and the sweep above proves nothing.
func TestTheDurableEvidenceGuardIsLoadBearing(t *testing.T) {
	// The same forbidden-substring scan the guard applies, returning the offending field paths
	// instead of failing, so a test can assert it DOES fire.
	forbidden := []string{
		"keycode", "scancode", "rawkey", "rune", "character", "deviceid",
		"screenshot", "pixels", "image", "frame", "title", "label", "text",
		"username", "account", "path", "filename", "window", "process", "pid",
		"generation", "coordinate",
	}
	seen := map[reflect.Type]bool{}
	var scan func(rt reflect.Type, path string, hits *[]string)
	scan = func(rt reflect.Type, path string, hits *[]string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
			rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			if rt.Kind() == reflect.Map {
				scan(rt.Key(), path+"[key]", hits)
			}
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			name := strings.ToLower(f.Name)
			for _, bad := range forbidden {
				if strings.Contains(name, bad) {
					*hits = append(*hits, path+"."+f.Name)
				}
			}
			scan(f.Type, path+"."+f.Name, hits)
		}
	}

	// Control: the real record is clean under this scan (mirrors the guard above).
	var clean []string
	scan(reflect.TypeOf(observe.RehearsalEvidence{}), "RehearsalEvidence", &clean)
	if len(clean) != 0 {
		t.Fatalf("RehearsalEvidence already trips the scan at %v — fix the struct, not the test", clean)
	}

	// Planted violation: a careless future edit adds a passive-content field to the evidence.
	type rehearsalEvidencePlusLeak struct {
		observe.RehearsalEvidence
		WindowTitle string // the exact kind of field the durable contract forbids
	}
	seen = map[reflect.Type]bool{}
	var hits []string
	scan(reflect.TypeOf(rehearsalEvidencePlusLeak{}), "rehearsalEvidencePlusLeak", &hits)
	if len(hits) == 0 {
		t.Fatal("the durable-memory guard did NOT catch a WindowTitle field planted on rehearsal " +
			"evidence — the shape guard is hollow and a future engineer could add captured " +
			"content while every test stayed green")
	}
}

// A scroll bar appearing does not make it a different screen.
//
// THE live blocker of 2026-08-17: five Learn attempts against Windows Settings minted five
// durable subjects for the same pages, because the platform shows a scroll bar when content
// is a shade taller than its space — or when the pointer is over it, which means the act of
// demonstrating changed the screen's identity. Nothing else disagreed: the terms matched and
// every shared count was inside tolerance.
func TestAScrollBarDoesNotMakeItADifferentScreen(t *testing.T) {
	// The two Home-page recordings, reduced to the fields that decided it.
	without := observe.StructureSignature{
		Subject: observe.SubjectState,
		Roles: map[string]int{
			"button": 15, "group": 20, "list_item": 22, "text": 32, "pane": 3,
		},
		Terms:      []observe.InterfaceTerm{observe.TermBack, observe.TermSettings},
		TermsKnown: true,
	}
	with := observe.StructureSignature{
		Subject: observe.SubjectState,
		Roles: map[string]int{
			"button": 16, "group": 20, "list_item": 22, "text": 32, "pane": 3,
			"scroll_bar": 1,
		},
		Terms:      []observe.InterfaceTerm{observe.TermBack, observe.TermSettings},
		TermsKnown: true,
	}
	if got := observe.CompareStructure(with, without); got != observe.MatchSame {
		t.Fatalf("verdict = %q, want %q.\nThe same page is a new durable subject every time a "+
			"scroll bar happens to show, so no endpoint ever resolves and nothing can be "+
			"learned on real software.", got, observe.MatchSame)
	}
	// And symmetrically.
	if got := observe.CompareStructure(without, with); got != observe.MatchSame {
		t.Errorf("reversed verdict = %q, want %q", got, observe.MatchSame)
	}
}

// The widening is CHROME only. A role a person could act on still separates two screens,
// which is the over-merge this design fears most.
func TestARealRoleArrivingStillMakesItADifferentScreen(t *testing.T) {
	base := observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 4, "text": 6},
		Terms:      []observe.InterfaceTerm{observe.TermSettings},
		TermsKnown: true,
	}
	for _, role := range []string{"progress_bar", "text_field", "slider", "checkbox", "tab"} {
		gained := observe.StructureSignature{
			Subject:    observe.SubjectState,
			Roles:      map[string]int{"button": 4, "text": 6, role: 1},
			Terms:      []observe.InterfaceTerm{observe.TermSettings},
			TermsKnown: true,
		}
		if got := observe.CompareStructure(gained, base); got == observe.MatchSame {
			t.Errorf("a screen that gained a %s was called the same screen; only presentation "+
				"chrome may be ignored", role)
		}
	}
}

// Terms still separate two structurally identical screens. The widening did not reach them.
func TestChromeToleranceDoesNotWeakenTheTermDiscriminator(t *testing.T) {
	audio := observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 4, "scroll_bar": 1},
		Terms:      []observe.InterfaceTerm{observe.TermAudio},
		TermsKnown: true,
	}
	display := observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 4},
		Terms:      []observe.InterfaceTerm{observe.TermDisplay},
		TermsKnown: true,
	}
	if got := observe.CompareStructure(audio, display); got != observe.MatchDifferent {
		t.Fatalf("verdict = %q, want %q — two screens whose text disagrees are different "+
			"screens whatever their chrome does", got, observe.MatchDifferent)
	}
}

// durableText is the CLOSED list of durable fields allowed to hold text, and why each one is.
//
// # Why this is a list and not an absence
//
// Because until this existed, the exceptions passed by accident. `Called` survived the sweep
// above only because the word "called" happens not to appear in the forbidden list — nobody
// decided it was allowed, and nothing recorded the licence that admits it. A second exception
// added the same way would be indistinguishable from a leak.
//
// So every durable string is now either refused or written down here with the reason. Adding an
// entry is a privacy decision and looks like one in a diff.
var durableText = map[string]string{
	// AUDIENCE-AUTHORED. A person was asked what to call a screen and answered. The one
	// durable string that comes from a person rather than from perception, and the
	// distinction is the whole boundary: an OCR line, an accessibility label or a window
	// title may never land here, because none of them was offered.
	// [[ADR-031-the-user-names-the-stage]].
	"RememberedSubject.Called": "the Audience's own word for a screen",

	// PERCEPTION-DERIVED, and the narrowest widening this system has made. A durable target
	// is identified by the word on the control, so it cannot exist without one.
	//
	// Bounded four ways, and all four have to hold:
	//   - the Audience ACTIVATED this control themselves, during their own demonstration;
	//   - the session carried an explicit Learn licence;
	//   - the label already passed observe.AdmittedTargetLabel — the plaintext role
	//     allowlist, widened to activatable roles only under that licence, plus the shape
	//     filter either way;
	//   - it is stored as EVIDENCE about a target, never as the Audience's own word.
	//
	// What this does not permit: remembering visible text because it might be useful later.
	// Nothing else observed in a session becomes a target.
	// [[ADR-068-the-theater-is-the-durable-semantic-world]].
	"RememberedSubject.Structure.Label": "the word on a control the Audience activated",
}

// The exceptions are exactly the two that were decided, and no more.
//
// A companion to the sweep above, and the half that catches the opposite mistake: somebody
// widening the boundary by adding an entry rather than by adding a field. The count is asserted so
// a third exception cannot arrive quietly.
func TestTheDurableTextExceptionsAreTheOnesThatWereDecided(t *testing.T) {
	want := map[string]bool{
		"RememberedSubject.Called":          true,
		"RememberedSubject.Structure.Label": true,
	}
	for path := range durableText {
		if !want[path] {
			t.Errorf("%s is allowed to hold durable text and was not one of the decided "+
				"exceptions.\nWidening what Marco remembers about a person's screen is a "+
				"privacy decision: it needs an ADR, not a map entry.", path)
		}
	}
	for path := range want {
		if _, ok := durableText[path]; !ok {
			t.Errorf("%s was a decided exception and is no longer listed", path)
		}
	}
}
