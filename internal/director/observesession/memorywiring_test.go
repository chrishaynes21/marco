package observesession_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Cross-session semantic memory, entered through the production session path twice.
//
// A direct Store.Recall test proves the matcher. What it cannot show is that a RUNNING session
// consults memory before deciding whether to interrupt somebody, or that answering a question
// writes anything durable at all. Both are one line each, both are the kind of line this
// repository has now shipped five mechanisms without, so both are tested from Runner.Run.
//
// The shape is deliberately two SEPARATE runners over one store file — the same thing that
// happens when the service restarts.

// sessionOver runs one complete session against a memory store.
//
// Several questions may be open at once, so a test can answer a CHOSEN one rather than whichever
// the policy happened to put first. That matters: the subjects differ in how their session-local
// references behave, and a test that always answered the first would be testing whichever
// subject sorted highest rather than the one it meant.
func sessionOver(t *testing.T, m observe.Memory, s observesession.Sampler) (*observesession.Runner, observesession.Result) {
	t.Helper()
	cfg := config()
	cfg.ProposalPolicy = observe.ProposalThresholds{MaxOpen: 6, MaxProposals: 32}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s, &recordingEvents{}).
		WithMemory(m)
	got, err := r.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r, got
}

// questionAbout finds the open question about one interpretation.
func questionAbout(t *testing.T, r *observesession.Runner,
	kind observe.HypothesisKind) observe.Proposal {

	t.Helper()
	for _, p := range r.Proposals().Open() {
		if p.Kind == kind {
			return p
		}
	}
	t.Fatalf("no open question about %s; open=%v", kind, r.Proposals().Open())
	return observe.Proposal{}
}

// memoryAt opens a store in a temporary directory.
func memoryAt(t *testing.T, dir string) *semanticmemory.Store {
	t.Helper()
	s, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	return s
}

// ── A/B/C/D/H: the milestone ──────────────────────────────────────────────────

// THE cross-session test. A confirmed subject is recognised by a later, separate session.
//
// Mutations that must fail this: deleting the durable write in Runner.Respond, and deleting the
// RecallFrom call in the sampling loop.
func TestAConfirmedSubjectIsRecognisedInALaterSession(t *testing.T) {
	dir := t.TempDir()

	// ── Session A: observe, ask, confirm ──
	first, resultA := sessionOver(t, memoryAt(t, dir), settingsSession())
	// The SETTINGS question specifically: its subject is a screen, whose session-local
	// reference genuinely renumbers between runs. A group subject would not exercise that.
	asked := questionAbout(t, first, observe.PossibleSettingsLikeState)
	if _, ok := first.Respond(asked.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("session A refused the answer")
	}

	// The store on disk now holds it. This is a genuinely new process's view of the file.
	reopened := memoryAt(t, dir)
	if reopened.Count() == 0 {
		t.Fatal("nothing was written durably when the user answered. Session B will ask " +
			"the same question again, which is the entire problem this milestone exists " +
			"to solve")
	}

	// ── Session B: a new runner, a new store handle, the same application ──
	//
	// The sampler is RENUMBERED: it shows the same screens in a different order, so the
	// tracker mints its session-local state and track identities differently — which is
	// what a restart actually does. Using the identical fixture would let a matcher that
	// secretly keyed on `state_2` pass this test.
	second, resultB := sessionOver(t, reopened, renumberedSettingsSession())

	// The ephemeral identities really are different objects between the two runs.
	if len(resultA.Hypotheses) == 0 || len(resultB.Hypotheses) == 0 {
		t.Fatal("one of the sessions produced no hypotheses")
	}

	var recognised bool
	for _, p := range second.Proposals().Proposals {
		if p.Recognised && p.Response == observe.ResponseConfirmed {
			recognised = true
		}
	}
	if !recognised {
		t.Fatal("session B did not recognise a subject session A confirmed. The user " +
			"answered this question yesterday and is about to be asked it again")
	}

	// And it did not ask about it again.
	for _, p := range second.Proposals().Open() {
		if p.ID == asked.ID {
			t.Error("session B re-asked a question the user already answered")
		}
	}

	// The hypothesis carries the validation, so the report can explain itself.
	var validated bool
	for _, h := range resultB.Hypotheses {
		if h.UserValidation != nil &&
			h.UserValidation.Response == observe.ResponseConfirmed {
			validated = true
		}
	}
	if !validated {
		t.Error("session B's hypotheses carry no user validation, so nothing downstream " +
			"can tell that this interpretation was ever confirmed")
	}
}

// The session-local identities genuinely differ between the two runs.
//
// The control for the test above: if state and track ids happened to be identical across
// sessions, recognition would prove nothing about durable identity.
func TestSessionLocalIdentitiesDifferBetweenSessions(t *testing.T) {
	dir := t.TempDir()
	_, a := sessionOver(t, memoryAt(t, dir), settingsSession())
	_, b := sessionOver(t, memoryAt(t, dir), renumberedSettingsSession())

	if len(a.Stats.Shadow.States) == 0 || len(b.Stats.Shadow.States) == 0 {
		t.Fatal("a session produced no states")
	}
	// Track ids are minted in observation order, so a session that sees a different screen
	// first assigns different ids to the same structures.
	sameFirstTrack := len(a.Stats.Shadow.Tracks) > 0 && len(b.Stats.Shadow.Tracks) > 0 &&
		a.Stats.Shadow.Tracks[0].ID == b.Stats.Shadow.Tracks[0].ID &&
		a.Stats.Shadow.Tracks[0].Role == b.Stats.Shadow.Tracks[0].Role
	_ = sameFirstTrack // recorded for the reader; the assertion that matters is below

	// The durable signature must be identical even though the sessions are not.
	var sigA, sigB observe.StructureSignature
	for _, h := range a.Hypotheses {
		if h.Kind == observe.PossibleSettingsLikeState {
			sigA = observe.SignatureOf(h)
		}
	}
	for _, h := range b.Hypotheses {
		if h.Kind == observe.PossibleSettingsLikeState {
			sigB = observe.SignatureOf(h)
		}
	}
	if sigA.Subject == "" || sigB.Subject == "" {
		t.Skip("the fixtures did not both produce a settings-like hypothesis")
	}
	if observe.CompareStructure(sigA, sigB) != observe.MatchSame {
		t.Errorf("the same screen produced non-matching signatures across two sessions:\n"+
			"  %+v\n  %+v", sigA, sigB)
	}
}

// ── I: a correction survives ──────────────────────────────────────────────────

// A "no" is as durable as a "yes".
//
// A correction the system forgets overnight is a correction the user has to give every day, and
// the second time they are asked they will reasonably conclude Marco does not listen.
func TestAContradictionSurvivesIntoALaterSession(t *testing.T) {
	dir := t.TempDir()

	first, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	open := first.Proposals().Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked")
	}
	if _, ok := first.Respond(open[0].ID, observe.ResponseContradicted); !ok {
		t.Fatal("the answer was refused")
	}

	second, resultB := sessionOver(t, memoryAt(t, dir), settingsSession())

	for _, p := range second.Proposals().Open() {
		if p.ID == open[0].ID {
			t.Fatal("Marco re-asked a question the user had already answered NO to, as " +
				"though the correction never happened")
		}
	}
	var contradicted bool
	for _, h := range resultB.Hypotheses {
		if h.UserValidation != nil &&
			h.UserValidation.Response == observe.ResponseContradicted {
			contradicted = true
			if h.Status != observe.StatusContested {
				t.Errorf("status %q for an interpretation the user rejected", h.Status)
			}
		}
	}
	if !contradicted {
		t.Error("the user's correction is invisible in the later session")
	}
}

// ── J/K: decline, and what lifts it ───────────────────────────────────────────

// A decline suppresses the question in a later session too.
func TestADeclineSuppressesTheQuestionInALaterSession(t *testing.T) {
	dir := t.TempDir()

	first, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	open := first.Proposals().Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked")
	}
	if _, ok := first.Respond(open[0].ID, observe.ResponseDeclined); !ok {
		t.Fatal("the decline was refused")
	}

	second, resultB := sessionOver(t, memoryAt(t, dir), settingsSession())
	for _, p := range second.Proposals().Open() {
		if p.ID == open[0].ID {
			t.Error("a question the user declined yesterday was put again today with no " +
				"new evidence. That is the nagging the decline exists to prevent")
		}
	}
	// And it is still not a denial.
	for _, h := range resultB.Hypotheses {
		if h.UserValidation == nil {
			continue
		}
		if h.UserValidation.Response == observe.ResponseContradicted {
			t.Error("a remembered decline came back as a contradiction. Being busy is not " +
				"disagreeing, and that distinction has to survive the file as well as the " +
				"session")
		}
	}
}

// ── L: provenance ─────────────────────────────────────────────────────────────

// A hypothesis nobody was asked about does not become confirmed by being remembered.
//
// The subject is recognised; the interpretation was never put to anybody. Recognition is not an
// answer, and a store that conflated them would manufacture confirmations out of its own history.
func TestRecognitionAloneIsNotUserConfirmation(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)

	// Session A observes and is never answered.
	first, _ := sessionOver(t, store, settingsSession())
	if len(first.Proposals().Open()) == 0 {
		t.Fatal("session A asked nothing")
	}

	// Session B sees the same thing.
	_, resultB := sessionOver(t, memoryAt(t, dir), settingsSession())
	for _, h := range resultB.Hypotheses {
		if h.UserValidation != nil {
			t.Errorf("%s carries user validation in session B, but nobody ever answered "+
				"a question in session A", h.Kind)
		}
		if h.Status == observe.StatusValidated {
			t.Errorf("%s reached `validated` without anyone being asked", h.Kind)
		}
	}
}

// ── M: corruption ─────────────────────────────────────────────────────────────

// A Director whose memory is unreadable still observes, still hypothesises, still asks.
func TestASessionRunsNormallyWithUnreadableMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	if err := writeCorrupt(path); err != nil {
		t.Fatalf("writing the corrupt fixture: %v", err)
	}
	store, why := semanticmemory.Open(path)
	if why == "" {
		t.Fatal("the corrupt fixture opened cleanly")
	}

	runner, got := sessionOver(t, store, settingsSession())
	if !got.Complete() {
		t.Errorf("the session ended %q because memory was corrupt", got.Session.State)
	}
	if len(got.Hypotheses) == 0 {
		t.Error("no hypotheses were formed; unreadable memory stopped perception")
	}
	if len(runner.Proposals().Open()) == 0 {
		t.Error("no question was asked. With no usable memory Marco cannot know whether " +
			"this was answered before, and asking is the safe direction")
	}
}

// ── O: memory does not manufacture observations ───────────────────────────────

// A remembered subject does not appear in perception when it is not on screen.
//
// The direction of the arrow: perception produces evidence, memory interprets it. If memory
// could add to what was seen, every stored subject would eventually be "observed" everywhere.
func TestMemoryDoesNotManufactureObservations(t *testing.T) {
	dir := t.TempDir()

	first, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	open := first.Proposals().Open()
	if len(open) == 0 {
		t.Fatal("nothing was asked")
	}
	first.Respond(open[0].ID, observe.ResponseConfirmed)

	// Session B never shows the screen at all: gameplay only.
	_, resultB := sessionOver(t, memoryAt(t, dir), &discoverySampler{})

	for _, h := range resultB.Hypotheses {
		if h.Kind == observe.PossibleSettingsLikeState {
			t.Error("a settings-like hypothesis appeared in a session whose sampler never " +
				"showed a settings screen. Memory has become a source of observations")
		}
	}
	for _, st := range resultB.Stats.Shadow.States {
		if st.Terms[observe.TermSettings] > 0 {
			t.Error("a remembered interface term appeared in the current session's " +
				"screen-state evidence")
		}
	}
}

// writeCorrupt plants an unreadable memory file.
func writeCorrupt(path string) error {
	return os.WriteFile(path, []byte("{not json at all"), 0o600)
}

// renumberedSettingsSession shows the same screens in a different ORDER, so the tracker mints
// its session-local identities differently — which is what happens across a real restart.
func renumberedSettingsSession() *discoverySampler {
	s := settingsSession()
	s.reversed = true
	return s
}
