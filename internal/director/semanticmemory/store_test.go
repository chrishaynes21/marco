package semanticmemory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The durable half: what survives a restart, what happens when the file is broken, and what
// stops the file growing forever.

func settingsSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 5},
		Members:    5,
		Terms:      []observe.InterfaceTerm{observe.TermSettings, observe.TermControls},
		TermsKnown: true,
	}
}

func confirmed(kind observe.HypothesisKind) observe.SemanticKnowledge {
	return observe.SemanticKnowledge{
		Kind: kind, Status: observe.KnowledgeConfirmed, Evidence: "ev1",
		Support:  []observe.EvidenceSource{observe.FromStructure, observe.FromText},
		Answered: 1,
	}
}

func open(t *testing.T, path string) *semanticmemory.Store {
	t.Helper()
	s, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("Open: %s", why)
	}
	return s
}

// ── survival ──────────────────────────────────────────────────────────────────

// An answer written by one store is readable by the next.
func TestKnowledgeSurvivesAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")

	first := open(t, path)
	if err := first.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	// A different process, as far as this store is concerned.
	second := open(t, path)
	rec := second.Recall("unknown-game", settingsSignature())
	if rec.Verdict != observe.MatchSame {
		t.Fatalf("verdict %q after reopening; the answer did not survive", rec.Verdict)
	}
	k, ok := rec.Subject.Find(observe.PossibleSettingsLikeState)
	if !ok || k.Status != observe.KnowledgeConfirmed {
		t.Fatalf("knowledge %+v (found=%v), want confirmed", k, ok)
	}
}

// A first run with no file is not an error.
func TestAMissingFileIsAFirstRunNotAFailure(t *testing.T) {
	s, why := semanticmemory.Open(filepath.Join(t.TempDir(), "nothing-here.json"))
	if why != "" {
		t.Fatalf("a missing memory file reported %q; the first run has no file", why)
	}
	if got := s.Recall("app", settingsSignature()).Verdict; got != observe.MatchDifferent {
		t.Errorf("verdict %q against empty memory, want different", got)
	}
}

// Memory is namespaced by application.
//
// Conservative and deliberate: two applications that happen to present structurally identical
// screens are not assumed to mean the same thing by them.
func TestMemoryIsNamespacedByApplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	if err := s.Remember("game-one", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if got := s.Recall("game-two", settingsSignature()).Verdict; got == observe.MatchSame {
		t.Error("a subject learned in one application was recognised in another. That may " +
			"eventually be desirable and it is a much stronger claim than this milestone " +
			"can support")
	}
}

// ── corruption ────────────────────────────────────────────────────────────────

// A corrupt file degrades visibly, and is neither discarded nor overwritten.
//
// Silently returning "empty" would tell the user their answers were forgotten by pretending they
// were never given — and the next write would destroy the evidence.
func TestCorruptMemoryDegradesVisiblyAndIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	original := []byte("{this is not json")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("writing the corrupt fixture: %v", err)
	}

	s, why := semanticmemory.Open(path)
	if why == "" {
		t.Fatal("a corrupt memory file opened cleanly. An unreadable store that reports " +
			"itself empty is indistinguishable from one the user never wrote to")
	}
	if !strings.Contains(why, path) {
		t.Errorf("the reason does not say which file: %s", why)
	}
	if s == nil {
		t.Fatal("Open returned no store; a Director whose memory is corrupt must still run")
	}

	// It is inert rather than dangerous: recall cannot claim anything.
	if got := s.Recall("app", settingsSignature()).Verdict; got == observe.MatchSame {
		t.Error("a corrupt store claimed to recognise something")
	}
	// And it refuses to write, so the broken file survives for recovery.
	if err := s.Remember("app", settingsSignature(),
		confirmed(observe.PossibleMenuLikeState)); err == nil {
		t.Error("a corrupt store accepted a write; the unreadable file would have been " +
			"replaced and whatever it contained lost")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Error("the corrupt file was modified; a person may want to recover it")
	}
}

// An unknown version is refused rather than reinterpreted.
func TestAnUnknownVersionIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	raw, _ := json.Marshal(map[string]any{"version": 99, "subjects": []any{}})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	_, why := semanticmemory.Open(path)
	if why == "" {
		t.Fatal("a future format version was loaded as though it were understood")
	}
	if !strings.Contains(why, "version") {
		t.Errorf("the reason does not mention the version: %s", why)
	}
}

// ── boundedness ───────────────────────────────────────────────────────────────

// Observing the same subject over and over does not grow the store.
//
// The property that makes this affordable: growth is per SUBJECT, not per observation. A screen
// seen ten thousand times is one record.
func TestRepeatedObservationDoesNotGrowTheStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)

	for i := 0; i < 200; i++ {
		if err := s.Remember("unknown-game", settingsSignature(),
			confirmed(observe.PossibleSettingsLikeState)); err != nil {
			t.Fatalf("Remember %d: %v", i, err)
		}
		s.NoteSession("unknown-game", settingsSignature())
	}
	if got := s.Count(); got != 1 {
		t.Fatalf("%d subjects after 200 observations of one screen, want 1", got)
	}

	// And the file itself has not grown without bound.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() > 8<<10 {
		t.Errorf("the memory file is %d bytes for a single remembered subject", info.Size())
	}

	// Answers accumulate as a count rather than as records.
	rec := s.Recall("unknown-game", settingsSignature())
	k, _ := rec.Subject.Find(observe.PossibleSettingsLikeState)
	if k.Answered < 2 {
		t.Errorf("answered count %d after repeated confirmation; the count is where "+
			"repetition shows", k.Answered)
	}
}

// Two genuinely different subjects are two records.
func TestDifferentSubjectsAreStoredSeparately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)

	audio := settingsSignature()
	audio.Terms = []observe.InterfaceTerm{observe.TermSettings, observe.TermAudio}
	video := settingsSignature()
	video.Terms = []observe.InterfaceTerm{observe.TermSettings, observe.TermDisplay}

	if err := s.Remember("g", audio, confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := s.Remember("g", video, confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if got := s.Count(); got != 2 {
		t.Errorf("%d subjects for two structurally identical screens with different text, "+
			"want 2", got)
	}
}

// ── the file itself ───────────────────────────────────────────────────────────

// Nothing captured reaches the file.
//
// Asserted against the bytes on disk rather than the type, because that is what actually
// outlives the session and what a person would find if they opened it.
func TestTheStoredFileContainsNothingCaptured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	if err := s.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body := strings.ToLower(string(raw))
	for _, leak := range []string{
		"screenshot", "pixel", "keycode", "scancode", "password", "hwnd",
		"generation", "process", "state_", "shadow_", "group_",
	} {
		if strings.Contains(body, leak) {
			t.Errorf("the memory file contains %q", leak)
		}
	}
	// What it SHOULD contain: structure and closed vocabulary.
	if !strings.Contains(body, "settings") || !strings.Contains(body, "button") {
		t.Error("the memory file does not contain the structural evidence it needs to match on")
	}
}

// The durable TOPOLOGY carries nothing captured either.
//
// A navigation run is exactly the shape that invites a leak: it is a sequence of things the
// player did, and the honest-looking version of it holds keys. What is on disk must be closed
// vocabulary — `down`, `confirm` — and never `S`, `Enter` or a scan code, and the check is on
// the BYTES rather than on the types, because a type can be correct and a marshaller creative.
func TestTheStoredTopologyContainsNothingCaptured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	if err := s.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := s.Remember("unknown-game", otherSignature(),
		confirmed(observe.PossibleMenuLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	subjects := s.Subjects()
	if len(subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(subjects))
	}
	if _, err := s.RememberRelationships("unknown-game",
		[]observe.RelationshipObservation{{
			From: subjects[0].ID, To: subjects[1].ID,
			Evidence: observe.RelationshipEvidence{
				Observations: 3,
				// A raw key identity is OFFERED here, deliberately. A file that never
				// saw one cannot show that it refuses one.
				Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2, "VK_RETURN": 9},
				Unattributed: 1,
				Sequences: []observe.NavSequence{
					{Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 2},
					{Intents: []observe.NavIntent{"S", "Enter"}, Count: 4},
				},
			},
		}}); err != nil {
		t.Fatalf("RememberRelationships: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body := string(raw)
	for _, leak := range []string{
		"keycode", "scancode", "vk_", "\"key\"", "hwnd", "generation", "process",
		"state_", "shadow_", "group_", "\"S\"", "Enter", "screenshot", "pixel",
	} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
			t.Errorf("the memory file contains %q", leak)
		}
	}
	// And what it SHOULD contain: a directed edge in closed vocabulary.
	for _, want := range []string{"relationships", "\"confirm\"", "\"down\"", "observations"} {
		if !strings.Contains(body, want) {
			t.Errorf("the memory file does not contain %q, so the topology it claims to "+
				"hold is not readable", want)
		}
	}
}

// A relationship refuses an endpoint the store does not hold.
//
// Referential integrity at the WRITE, not only at the load. An edge to a subject that is not in
// the store is unreadable the moment it is written: nothing can resolve the other end, and
// nothing can explain it to a person.
func TestARelationshipWithAnUnknownEndpointIsRefused(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "memory.json"))
	if err := s.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	real := s.Subjects()[0].ID

	update, err := s.RememberRelationships("unknown-game", []observe.RelationshipObservation{
		{From: real, To: "subj_doesnotexist",
			Evidence: observe.RelationshipEvidence{Observations: 4}},
		{From: "subj_alsonot", To: real,
			Evidence: observe.RelationshipEvidence{Observations: 4}},
		// A self-loop is not an edge between two subjects.
		{From: real, To: real, Evidence: observe.RelationshipEvidence{Observations: 4}},
	})
	if err != nil {
		t.Fatalf("RememberRelationships: %v", err)
	}
	if update.Created != 0 || update.Corroborated != 0 {
		t.Errorf("an edge with an endpoint the store does not hold was written: %+v", update)
	}
	if update.Rejected != 3 {
		t.Errorf("rejected %d of 3; a refusal that is not counted is a refusal nobody can see",
			update.Rejected)
	}
	if got := len(s.Relationships()); got != 0 {
		t.Errorf("%d relationship(s) exist", got)
	}
}

// An application boundary is not crossed by an edge.
//
// Subjects are namespaced by application, conservatively and deliberately. A relationship
// inherits that rather than introducing new cross-application semantics of its own.
func TestARelationshipDoesNotCrossTheApplicationBoundary(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "memory.json"))
	if err := s.Remember("game-one", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := s.Remember("game-two", otherSignature(),
		confirmed(observe.PossibleMenuLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var one, two string
	for _, r := range s.Subjects() {
		if r.Application == "game-one" {
			one = r.ID
		} else {
			two = r.ID
		}
	}
	update, err := s.RememberRelationships("game-one", []observe.RelationshipObservation{{
		From: one, To: two, Evidence: observe.RelationshipEvidence{Observations: 5},
	}})
	if err != nil {
		t.Fatalf("RememberRelationships: %v", err)
	}
	if update.Created != 0 || update.Rejected != 1 {
		t.Errorf("an edge between two applications was accepted: %+v", update)
	}
}

// otherSignature is a second, clearly distinct subject.
func otherSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 5},
		Members:    5,
		Terms:      []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay},
		TermsKnown: true,
	}
}

// ── what the user calls a screen ──────────────────────────────────────────────

// A name belongs to one application, and never leaks into another.
//
// Two programs may both have a `settings`. A play that asked for the wrong one would press keys in
// the wrong place, so the scope is exact and there is no fallback that would let it be otherwise.
func TestAScreenNameIsScopedToItsApplication(t *testing.T) {
	dir := t.TempDir()
	s, _ := semanticmemory.Open(filepath.Join(dir, "memory.json"))

	a := rememberOne(t, s, "gameone", observe.TermSettings, observe.TermControls)
	b := rememberOne(t, s, "gametwo", observe.TermAudio, observe.TermDisplay)
	if err := s.NameSubject("gameone", a, "settings"); err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := s.NameSubject("gametwo", b, "settings"); err != nil {
		t.Fatalf("the same name in another application was refused: %v", err)
	}

	got, ok := s.SubjectNamed("gameone", "settings")
	if !ok || got.ID != a {
		t.Fatalf("gameone's settings resolved to %+v", got)
	}
	if got, ok := s.SubjectNamed("gametwo", "settings"); !ok || got.ID != b {
		t.Fatalf("gametwo's settings resolved to %+v", got)
	}
	// And nothing resolves in an application that has no such name.
	if _, ok := s.SubjectNamed("gamethree", "settings"); ok {
		t.Fatal("a name resolved in an application that never used it")
	}
}

// Two screens in one application may not share a name.
func TestASecondScreenCannotTakeATakenName(t *testing.T) {
	dir := t.TempDir()
	s, _ := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	a := rememberOne(t, s, "gameone", observe.TermSettings, observe.TermControls)
	b := rememberOne(t, s, "gameone", observe.TermAudio, observe.TermDisplay)

	if err := s.NameSubject("gameone", a, "settings"); err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := s.NameSubject("gameone", b, "settings"); err == nil {
		t.Fatal("two screens took the same name; a play's first line would be ambiguous")
	}
	// Renaming the SAME screen is the ordinary case and is allowed.
	if err := s.NameSubject("gameone", a, "the settings screen"); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	if _, ok := s.SubjectNamed("gameone", "settings"); ok {
		t.Error("the old name still resolves")
	}
	if got, ok := s.SubjectNamed("gameone", "the settings screen"); !ok || got.ID != a {
		t.Errorf("the new name resolved to %+v", got)
	}
}

// An ambiguous name resolves to nothing at all.
//
// `NameSubject` refuses a duplicate, so this state cannot arise through the API — which is exactly
// why the resolver must not trust that. A memory file is a plain JSON file a person can edit, an
// older version may have written one, and a merge can produce one. The resolver is the last thing
// standing between a duplicated name and a play that presses keys on whichever screen happened to
// be stored second.
//
// Nearest-match is not an option here. There is no safe way to guess which of two screens the
// person meant, so Marco says it does not know and the guarded play refuses.
func TestAnAmbiguousScreenNameResolvesToNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	s, _ := semanticmemory.Open(path)
	a := rememberOne(t, s, "gameone", observe.TermSettings, observe.TermControls)
	b := rememberOne(t, s, "gametwo", observe.TermAudio, observe.TermDisplay)
	if err := s.NameSubject("gameone", a, "settings"); err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := s.NameSubject("gametwo", b, "settings"); err != nil {
		t.Fatalf("naming: %v", err)
	}

	// The file is edited so both screens sit in one application under one name — the state the
	// API refuses to create. Rewriting the application name keeps this independent of the
	// on-disk field layout.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	edited := strings.ReplaceAll(string(raw), "gametwo", "gameone")
	if edited == string(raw) {
		t.Fatal("the fixture did not change; the stored file names no application")
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	got, ok := reopened.SubjectNamed("gameone", "settings")
	if ok {
		t.Fatalf("an ambiguous name resolved to %q; a play would begin on whichever screen "+
			"was stored last", got.ID)
	}
	// An unambiguous name in the same file still resolves, so this is ambiguity being
	// refused rather than the store having given up.
	if err := reopened.NameSubject("gameone", a, "the settings screen"); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	if got, ok := reopened.SubjectNamed("gameone", "the settings screen"); !ok || got.ID != a {
		t.Errorf("an unambiguous name resolved to %+v", got)
	}
}

// A name survives a restart, tied to the durable subject.
func TestAScreenNameSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	s, _ := semanticmemory.Open(path)
	a := rememberOne(t, s, "gameone", observe.TermSettings, observe.TermControls)
	if err := s.NameSubject("gameone", a, "the pause menu"); err != nil {
		t.Fatalf("naming: %v", err)
	}

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	got, ok := reopened.SubjectNamed("gameone", "the pause menu")
	if !ok {
		t.Fatal("the name did not survive a restart")
	}
	if got.ID != a {
		t.Errorf("it resolved to %q, not the screen it was given to", got.ID)
	}
}

// rememberOne stores one subject and returns its durable id.
func rememberOne(t *testing.T, s *semanticmemory.Store, app string,
	terms ...observe.InterfaceTerm) string {

	t.Helper()
	sig := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4},
		Terms: terms, TermsKnown: true,
	}
	if err := s.Remember(app, sig, observe.SemanticKnowledge{
		Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	for _, r := range s.Subjects() {
		if r.Application == app && len(r.Structure.Terms) > 0 && r.Structure.Terms[0] == terms[0] {
			return r.ID
		}
	}
	t.Fatal("the subject was not stored")
	return ""
}

// A learn EPISODE folds its evidence and claims one independent sighting, not three.
//
// `Sessions` is what the invitation policy reads as "this keeps happening" — separate sittings,
// separate window generations, separate days. An explicit learn runs several bounded passes back
// to back at one keyboard, and counting each of them would let Marco manufacture the corroboration
// that the threshold exists to require.
//
// The other half is here too, and matters as much: an ordinary session that says nothing still
// corroborates. The zero value counts, so a caller that forgets the field cannot silently stop
// passive observation from accumulating evidence.
func TestALearningEpisodeClaimsOneSessionAndAnOrdinaryOneStillClaimsItsOwn(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "memory.json"))
	if err := s.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := s.Remember("unknown-game", otherSignature(),
		confirmed(observe.PossibleMenuLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	subjects := s.Subjects()
	from, to := subjects[0].ID, subjects[1].ID

	fold := func(sameEpisode bool) {
		t.Helper()
		if _, err := s.RememberRelationships("unknown-game",
			[]observe.RelationshipObservation{{
				From: from, To: to, SameEpisode: sameEpisode,
				Evidence: observe.RelationshipEvidence{Observations: 2},
			}}); err != nil {
			t.Fatalf("RememberRelationships: %v", err)
		}
	}

	// One learn attempt: the pass that created the edge, then two more of the same episode.
	fold(false)
	fold(true)
	fold(true)

	edge := s.Relationships()[0]
	if edge.Sessions != 1 {
		t.Fatalf("one learn episode claimed %d independent sessions, want 1", edge.Sessions)
	}
	if edge.Observations != 6 {
		t.Errorf("the episode folded %d observations, want 6 — the evidence is real and must "+
			"still be counted", edge.Observations)
	}

	// And two ordinary sittings on later days each claim their own.
	fold(false)
	fold(false)
	if edge := s.Relationships()[0]; edge.Sessions != 3 {
		t.Fatalf("after two ordinary sessions the count is %d, want 3; the episode rule must "+
			"not weaken passive observation", edge.Sessions)
	}
}

// ── revision ──────────────────────────────────────────────────────────────────

// Correcting one interpretation leaves the subject's other interpretations alone.
//
// One screen carries several readings at once — it can look settings-like AND menu-like — and each
// is answered separately. A correction to one of them says nothing about the others, so the write
// has to replace the matching entry rather than the list.
//
// Mutation M14: `upsert` returning just the new entry. It survived until this test existed, and
// the product-level revision tests could not see it: their two questions were about two different
// subjects, so each lived in its own record.
func TestRevisingOneInterpretationLeavesTheOthersAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	first := open(t, path)
	for _, kind := range []observe.HypothesisKind{
		observe.PossibleSettingsLikeState, observe.PossibleMenuLikeState} {
		if err := first.Remember("unknown-game", settingsSignature(), confirmed(kind)); err != nil {
			t.Fatalf("Remember %s: %v", kind, err)
		}
	}

	// The user withdraws ONE of the two answers.
	withdrawn := confirmed(observe.PossibleSettingsLikeState)
	withdrawn.Status = observe.KnowledgeRetracted
	if err := first.Remember("unknown-game", settingsSignature(), withdrawn); err != nil {
		t.Fatalf("withdrawing: %v", err)
	}

	rec := open(t, path).Recall("unknown-game", settingsSignature())
	if n := len(rec.Subject.Knowledge); n != 2 {
		t.Fatalf("the subject holds %d interpretations after one was corrected, want 2. "+
			"Correcting an answer deleted knowledge the user never mentioned", n)
	}
	menu, ok := rec.Subject.Find(observe.PossibleMenuLikeState)
	if !ok {
		t.Fatal("the untouched interpretation is gone")
	}
	if menu.Effective() != observe.JudgementConfirmed {
		t.Errorf("the untouched interpretation now reads %q", menu.Effective())
	}
	settings, ok := rec.Subject.Find(observe.PossibleSettingsLikeState)
	if !ok || settings.Effective() != observe.JudgementNone {
		t.Errorf("the withdrawn interpretation reads %q, want no active judgement",
			settings.Effective())
	}
}
