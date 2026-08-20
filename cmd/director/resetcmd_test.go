package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Emptying an acceptance sandbox, and never anything else.
//
// # What this is protecting
//
// Durable semantic memory is what Marco IS. Everything Marco has learned, every screen they
// have named, every route they corrected — one file. A reset that could reach it would be the
// single most destructive operation in the repository, and it would be one environment variable
// away from running by accident during a debugging session.
//
// So the guard is tested harder than the thing it guards.

// THE safety test. A reset never touches the real store.
//
// Two ways to arrive at it and both are refused: saying nothing, and naming it explicitly. The
// second matters more — "MARCO_HOME is set" is exactly the check somebody would write, and it
// passes for the real store.
func TestAResetRefusesTheRealStore(t *testing.T) {
	real := defaultHome()
	if strings.TrimSpace(real) == "" {
		t.Fatal("the default home resolves to nothing, so the guard has nothing to compare")
	}

	// Unset: the default location, which is the real store.
	if why := resetGuard("", real); why == "" {
		t.Error("a reset with MARCO_HOME unset was allowed. That is the real store, and " +
			"the operation would delete everything Marco has ever learned.")
	}
	// Whitespace is unset.
	if why := resetGuard("   ", real); why == "" {
		t.Error("a blank MARCO_HOME was treated as a sandbox")
	}
	// NAMED explicitly. Pointing MARCO_HOME at the default is not a declaration of
	// anything — it is the same directory by a longer route.
	if why := resetGuard(real, real); why == "" {
		t.Errorf("MARCO_HOME=%s was accepted as a sandbox; it is the real store", real)
	}
	// And spelled differently. A guard steppable by changing a slash or a letter case is
	// not a guard, and this runs on Windows.
	for _, spelling := range []string{
		filepath.ToSlash(real),
		strings.ToUpper(real),
		filepath.Join(real, "."),
		real + string(filepath.Separator),
	} {
		if why := resetGuard(spelling, real); why == "" {
			t.Errorf("MARCO_HOME=%q was accepted; it names the real store", spelling)
		}
	}
}

// A declared sandbox is allowed.
//
// The control. A guard that refused everything would be safe and useless, and the harness would
// go back to deleting files by hand — which is what it was doing.
func TestAResetAllowsADeclaredSandbox(t *testing.T) {
	if why := resetGuard(t.TempDir(), defaultHome()); why != "" {
		t.Errorf("a throwaway directory was refused: %s", why)
	}
}

// A refusal says what to do about it.
//
// This is read by somebody mid-acceptance-run who wants the sandbox emptied. "refused" alone
// sends them to delete the files themselves, next to the real store, by hand.
func TestAResetRefusalSaysHowToProceed(t *testing.T) {
	why := resetGuard("", defaultHome())
	if !strings.Contains(why, "MARCO_HOME") {
		t.Errorf("the refusal does not name what to set: %q", why)
	}
	if !strings.Contains(strings.ToLower(why), "sandbox") {
		t.Errorf("the refusal does not say a sandbox is what is wanted: %q", why)
	}
}

// ── what a reset actually clears ──────────────────────────────────────────────

// seedSandbox writes one of everything an acceptance run leaves behind.
func seedSandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{
		"semantic-memory.json", "semantic-memory.json.bak-r34",
		"semantic-memory.backup-120037.json",
		"action-graph.json", "director-history.json", "variables.json",
		"director-stop", "director-service.json",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	for _, sub := range []string{"demonstrations", "learned"} {
		p := filepath.Join(dir, sub)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatalf("seeding %s: %v", sub, err)
		}
		if err := os.WriteFile(filepath.Join(p, "one.json"), []byte("{}"), 0o600); err != nil {
			t.Fatalf("seeding %s content: %v", sub, err)
		}
	}
	return dir
}

// Everything one run can leave behind is cleared.
//
// Named individually because each is a way for the next run to be contaminated: subjects and
// candidates from semantic memory, actions from the graph, saved plays from `learned`, and — the
// one that bit hardest — hand-made backups that a later reopen could restore from.
func TestAResetClearsEverythingARunLeavesBehind(t *testing.T) {
	dir := seedSandbox(t)
	removed, kept := resetHome(dir, false)

	for _, want := range []string{
		"semantic-memory.json", "semantic-memory.json.bak-r34",
		"semantic-memory.backup-120037.json", "action-graph.json",
		"director-history.json", "variables.json", "director-stop",
		"director-service.json", "demonstrations", "learned",
	} {
		if !slices.Contains(removed, want) {
			t.Errorf("%s was not removed; the next run inherits it", want)
		}
		if _, err := os.Stat(filepath.Join(dir, want)); err == nil {
			t.Errorf("%s is still on disk", want)
		}
	}
	if len(kept) != 0 {
		t.Errorf("the sandbox still holds %v, so the next run is not clean", kept)
	}

	// AND THE DIRECTORY SURVIVES. This empties a sandbox; it does not remove one, because
	// the harness has already told the Director to use it.
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the sandbox itself was removed: %v", err)
	}
}

// Anything unrecognised is LEFT and SAID.
//
// A reset that deleted whatever it found would quietly become "remove the directory". A reset
// that silently left something would make "clean" a lie. Both matter, so it does neither.
func TestAnUnknownFileIsKeptAndReported(t *testing.T) {
	dir := seedSandbox(t)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	removed, kept := resetHome(dir, false)
	if slices.Contains(removed, "notes.txt") {
		t.Error("an unrecognised file was deleted. This empties a sandbox by a known list; " +
			"deleting whatever it finds is the operation it is careful not to be.")
	}
	if !slices.Contains(kept, "notes.txt") {
		t.Error("an unrecognised file was left WITHOUT saying so, which makes \"clean\" a lie")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("the file was reported as kept and is gone: %v", err)
	}
}

// A dry run removes nothing and reports the same list.
//
// So the harness — and a person about to empty something — can see what would go first.
func TestADryRunRemovesNothing(t *testing.T) {
	dir := seedSandbox(t)
	would, _ := resetHome(dir, true)
	if len(would) == 0 {
		t.Fatal("a dry run of a full sandbox reported nothing")
	}
	for _, name := range would {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was removed by a DRY run: %v", name, err)
		}
	}
	// And the real run agrees with what the dry run promised.
	did, _ := resetHome(dir, false)
	if !slices.Equal(would, did) {
		t.Errorf("the dry run said %v and the real run did %v", would, did)
	}
}

// An empty sandbox resets to nothing, without complaint.
func TestResettingAnEmptySandboxIsFine(t *testing.T) {
	removed, kept := resetHome(t.TempDir(), false)
	if len(removed) != 0 || len(kept) != 0 {
		t.Errorf("an empty sandbox reported removed=%v kept=%v", removed, kept)
	}
}

// ── a live Director is never reset out from under ─────────────────────────────

// A reset refuses while a Director is ANSWERING against the sandbox.
//
// It holds proposals, grants and session state in memory. Clearing the files under it would
// remove nothing it believes and would be written straight back — a reset that appeared to work
// and left the next run contaminated, which is the failure this whole command exists to end.
func TestAResetRefusesWhileADirectorIsAnswering(t *testing.T) {
	dir := t.TempDir()

	writeEndpointFile(t, dir, "127.0.0.1:1")
	answering := func(service.Endpoint) bool { return true }
	if why := resetBlockedBy(dir, answering); why == "" {
		t.Fatal("a reset was allowed while a Director was answering.\nIt holds proposals, " +
			"grants and session state in memory; clearing the files under it removes " +
			"nothing it believes and is written straight back.")
	}
}

// A STALE endpoint from a crashed Director does not block a reset.
//
// The other half, and it matters more than it looks: a crash is exactly when a sandbox most needs
// clearing, and a guard that trusted the file would make it unusable then. resetHome deletes that
// file too, so trusting it here would also make the two disagree.
//
// Deleting the reachability check must fail this.
func TestAStaleEndpointDoesNotBlockAReset(t *testing.T) {
	dir := t.TempDir()
	// The file says a Director is here; nothing answers.
	writeEndpointFile(t, dir, "127.0.0.1:1")
	silent := func(service.Endpoint) bool { return false }
	if why := resetBlockedBy(dir, silent); why != "" {
		t.Errorf("a stale endpoint blocked a reset: %s\nA crashed Director is exactly when "+
			"the sandbox most needs clearing.", why)
	}
}

// No endpoint at all is no obstacle.
func TestNoEndpointDoesNotBlockAReset(t *testing.T) {
	never := func(service.Endpoint) bool { return true }
	if why := resetBlockedBy(t.TempDir(), never); why != "" {
		t.Errorf("an empty sandbox was blocked: %s", why)
	}
}

// writeEndpointFile puts a plausible endpoint in a sandbox.
func writeEndpointFile(t *testing.T, dir, addr string) {
	t.Helper()
	body := `{"address":` + strconv.Quote(addr) + `,"token":"t","pid":1}`
	if err := os.WriteFile(service.EndpointPath(dir), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the endpoint: %v", err)
	}
	if _, ok := service.ReadEndpoint(dir); !ok {
		t.Fatal("the fixture endpoint is not readable, so this test proves nothing")
	}
}
