//go:build livevalidation

// Package live holds the OPT-IN validation harness: the scenarios that can only be
// settled by performing real input against a real application.
//
//	These must never run as part of the default unit-test suite.
//
// Two independent gates, deliberately. The build tag keeps this out of `go test ./...`
// entirely — it does not compile, let alone run — and the environment variable is a
// second, explicit acknowledgement that real UI input will occur. Either alone would be
// one accident away from a suite that drives Explorer on a developer's machine.
//
//	go test -tags livevalidation ./internal/director/live/ -run TestLiveExplorerRename -v
//	  with MARCO_LIVE_VALIDATION=i-understand-real-input-will-occur
//
// Every scenario works on artefacts it created in a temporary directory, verifies the
// FINAL EXTERNAL STATE from outside the Director — by reading the filesystem, not by
// asking the Director whether it thinks it succeeded — and cleans up after itself.
//
//	Fail closed when the expected application, window, document, or workspace cannot
//	be positively identified.
//
// So every guard below skips with a precise reason rather than proceeding hopefully, and
// every skip happens BEFORE any UI input. A harness that half-identified a window and
// acted anyway is exactly the thing that would rename the wrong file.
//
// There is exactly ONE scenario. Not because more would be hard, but because a second
// scenario written before the first has ever run is scaffolding that looks like coverage.
//
// # Why this is not under internal/director
//
// The Director may not import platform code — windows, input and the screen are reached
// through pkg/directorapi, and internal/director/boundary_test.go enforces it. This
// harness legitimately does import platform code: it ARRANGES THE SCENE, opening a folder
// window and bringing it to the front, which is setup rather than the thing under test.
// Putting it inside internal/director would have meant either breaking that rule or
// weakening the test that guards it, and neither is worth the tidier path.
//
// What the Director does here is the rename, and it does it through its own production
// path with nothing stubbed.
package live

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

const (
	liveEnv         = "MARCO_LIVE_VALIDATION"
	acknowledgement = "i-understand-real-input-will-occur"
	// binEnv points at a prebuilt director binary, so a run does not pay for a build.
	binEnv = "MARCO_DIRECTOR_BIN"
	// bridgeEnv points at the accessibility bridge. Without a working one the Director
	// sees nothing, and a scenario that cannot see cannot identify a window.
	bridgeEnv = "MARCO_UIA_BRIDGE"
)

// waitFor polls a condition until it holds or the deadline passes.
//
// A POLL, not a sleep: the thing being waited for is a separate process opening a window,
// which the Director cannot be notified about. The interval is short and the deadline is
// explicit, so a failure says "this did not happen within N seconds" rather than pausing
// for a guessed duration and hoping.
func waitFor(d time.Duration, check func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return check()
}

// ── gates ─────────────────────────────────────────────────────────────────────

// Require runs every prerequisite that can be checked before anything is launched.
//
//	The scenario must require: live build tag, environment variable, supported OS,
//	Explorer process, expected window, temporary directory identity.
//
// The first three are here; the rest are checked as the harness establishes them, each
// before the step that would need it. Every one of them SKIPS — never downgrades to
// simulation, and never reports a pass.
func Require(t *testing.T) {
	t.Helper()

	got := os.Getenv(liveEnv)
	if got == "" {
		t.Skipf("live validation is opt-in: set %s=%s to allow REAL UI input",
			liveEnv, acknowledgement)
	}
	if got != acknowledgement {
		t.Fatalf("%s is set to %q, which is not the acknowledgement. Set it to %q "+
			"exactly, or unset it.", liveEnv, got, acknowledgement)
	}
	if runtime.GOOS != "windows" {
		t.Skipf("this scenario drives Windows File Explorer; GOOS is %q", runtime.GOOS)
	}
	if _, err := exec.LookPath("explorer.exe"); err != nil {
		t.Skipf("explorer.exe was not found, so File Explorer cannot be driven: %v", err)
	}
}

// ── workspace ─────────────────────────────────────────────────────────────────

// Workspace is a temporary directory this test created, and what is in it.
//
// t.TempDir rather than a path under the user's Documents: nothing here may touch an
// existing user file, and a directory the framework owns cannot accidentally be one.
type Workspace struct {
	Dir string
	// Files maps a base name to its full path.
	Files map[string]string
	// Digests maps a full path to the content this test wrote, so "unchanged" is
	// checked against what was put there rather than against what is there now.
	Digests map[string]string
}

// NewWorkspace creates a scratch directory holding the named files.
func NewWorkspace(t *testing.T, contents map[string]string) *Workspace {
	t.Helper()
	// A DISTINCTIVE name inside the framework's directory, not the framework's directory
	// itself. Go names those `001`, `002`, … per test, so two runs of this scenario
	// produce two folders called `001` — and a File Explorer window left open by an
	// earlier run is then indistinguishable from this run's, which is precisely the
	// mis-identification the whole harness exists to avoid. This was found the first
	// time the scenario ran: it identified the PREVIOUS run's window and found no files
	// in it, because that folder had since been deleted.
	dir := filepath.Join(t.TempDir(), fmt.Sprintf("marco-live-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("creating the workspace: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("the scratch directory %q is not absolute; refusing to act relative to "+
			"an unknown working directory", dir)
	}
	// Windows hands back a short (8.3) path here often enough to matter: Explorer will
	// report the long one, and comparing the two as strings would look like a different
	// folder. Resolving it now makes the identity check meaningful.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	w := &Workspace{Dir: dir, Files: map[string]string{}, Digests: map[string]string{}}
	for name, body := range contents {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("preparing %s: %v", path, err)
		}
		w.Files[name], w.Digests[path] = path, body
	}
	return w
}

// Read returns a file's contents, and whether it is there.
func (w *Workspace) Read(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// Names lists what is in the directory now, sorted.
func (w *Workspace) Names(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(w.Dir)
	if err != nil {
		t.Fatalf("reading %s: %v", w.Dir, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// ── the Director under test ───────────────────────────────────────────────────

// Harness is a running Director and a connection to it.
//
// It launches its OWN service against its OWN temporary MARCO_HOME rather than attaching
// to whatever the developer happens to have running: a scenario that appended to the
// user's real action graph, or that inherited a half-configured daemon, would be testing
// something other than what it claims.
type Harness struct {
	t *testing.T
	// Dir is the service's home: endpoint file, token, action graph.
	Dir string
	// Client is the connection commands are submitted on.
	Client *service.Client
	// Confirmations is a SECOND connection, used to answer. The submitting connection
	// is blocked reading its command's events, and the command cannot finish until the
	// answer arrives.
	Confirmations *service.Client

	proc *exec.Cmd
	// answered records every confirmation this harness agreed to, so a scenario can
	// prove a confirmation actually happened rather than assuming it did.
	answered []service.ConfirmationPayload
	mu       sync.Mutex
	// asking tracks the answering goroutine, so teardown waits for it rather than
	// pulling the connection out from under it.
	asking  sync.WaitGroup
	stopAsk chan struct{}
	// explorerOpened records the folder a scenario opened, for the teardown message.
	explorerOpened string
	// dirTitle is the full path, which is the other title Explorer may use.
	dirTitle string
}

// StartDirector launches a Director service and connects to it.
//
// Skips — never fails — when the binary or the accessibility bridge cannot be found: a
// machine without them cannot run this scenario, and that is a missing prerequisite rather
// than a defect in the Director.
func StartDirector(t *testing.T) *Harness {
	t.Helper()

	bin := directorBinary(t)
	bridge := bridgePath(t)

	dir := t.TempDir()
	h := &Harness{t: t, Dir: dir, stopAsk: make(chan struct{})}

	cmd := exec.Command(bin, "serve", "--accessibility", bridge)
	cmd.Env = append(os.Environ(), "MARCO_HOME="+dir)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skipf("the Director could not be started from %s: %v", bin, err)
	}
	h.proc = cmd
	t.Cleanup(h.Close)

	// The service writes its endpoint file once it is listening. Polled rather than
	// slept on, with a deadline, so a service that failed to start says so.
	var client *service.Client
	ok := waitFor(30*time.Second, func() bool {
		ep, found := service.ReadEndpoint(dir)
		if !found {
			return false
		}
		c, err := service.Dial(ep, 10*time.Second)
		if err != nil {
			return false
		}
		client = c
		return true
	})
	if !ok {
		t.Skipf("the Director did not start listening in %s within 30s", dir)
	}
	h.Client = client

	ep, _ := service.ReadEndpoint(dir)
	answerer, err := service.Dial(ep, 10*time.Second)
	if err != nil {
		t.Skipf("a second connection could not be opened, so confirmations could not "+
			"be answered: %v", err)
	}
	h.Confirmations = answerer
	return h
}

// directorBinary finds or builds the Director.
func directorBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv(binEnv); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%s points at %s, which is not there: %v", binEnv, p, err)
		}
		return p
	}
	// Built rather than assumed, so a stale binary cannot be what is under test.
	out := filepath.Join(t.TempDir(), "director.exe")
	build := exec.Command("go", "build", "-o", out,
		"github.com/chaynes-simpleclouds/marco/cmd/director")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Skipf("the Director could not be built (set %s to skip building): %v", binEnv, err)
	}
	return out
}

// bridgePath finds the accessibility bridge.
//
// Without it the Director observes nothing, and a scenario that cannot see cannot identify
// a window — so this is a prerequisite, checked before anything is launched.
func bridgePath(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv(bridgeEnv), filepath.Join("..", "..", "..", "plugins", "uia", "uia.exe")}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	t.Skipf("the accessibility bridge was not found; set %s to its path. Without it the "+
		"Director cannot see into Explorer, and a window that cannot be identified must "+
		"not be acted on.", bridgeEnv)
	return ""
}

// Close shuts the service down and reports anything left behind.
func (h *Harness) Close() {
	// Close the window this scenario opened, through the Director, BEFORE the service
	// goes away. Deliberately not by terminating anything: Explorer folder windows are
	// hosted by the shell process, and killing it would take the user's taskbar with it
	// — a cleanup far worse than the mess it tidies.
	h.closeExplorer()

	if h.stopAsk != nil {
		close(h.stopAsk)
		h.stopAsk = nil
		// Wait for the answering goroutine to stop BEFORE its connection is closed.
		h.asking.Wait()
	}
	if h.Confirmations != nil {
		h.Confirmations.Close()
		h.Confirmations = nil
	}
	if h.Client != nil {
		_ = h.Client.Shutdown()
		h.Client.Close()
		h.Client = nil
	}
	if h.proc != nil {
		_ = h.proc.Wait()
		h.proc = nil
	}
	if h.explorerOpened != "" {
		h.t.Logf("LEFT OPEN: an Explorer window on %s. Close it when convenient; it is "+
			"a window on a temporary folder and nothing depends on it.", h.explorerOpened)
	}
}

// closeExplorer closes the one window this scenario opened.
//
//	Record the Explorer window created or adopted by the harness, close only that
//	positively identified window when safe, never terminate explorer.exe.
//
// Positively identified by TITLE, and the title is unique to this run — the workspace is
// named with the process id and a nanosecond stamp precisely so that no window belonging to
// the user, or to an earlier run, can match it. A close that guessed would shut a folder
// somebody was working in.
//
// Terminating the shell is never an option and is not a fallback: Explorer's folder windows
// are hosted by the process that draws the taskbar, so "cleaning up" that way would take the
// user's desktop with it. A window that will not close is REPORTED, which is the honest
// outcome and a far smaller cost.
func (h *Harness) closeExplorer() {
	if h.explorerOpened == "" {
		return
	}
	title := filepath.Base(h.explorerOpened)
	if err := winctx.CloseTitle(title); err != nil {
		// Reported SEPARATELY from the scenario's own result: a rename that worked and a
		// window that would not close are two different facts, and folding them together
		// would make a successful run look failed.
		h.t.Logf("CLEANUP: the Explorer window %q could not be closed: %v", title, err)
		return
	}
	h.t.Logf("closed the Explorer window on %s", h.explorerOpened)
	h.explorerOpened = ""
}

// ── confirmations ─────────────────────────────────────────────────────────────

// AutoConfirm answers every confirmation this scenario provokes.
//
// A REAL answer over the real protocol, on its own connection — not a stub confirmer and
// not a bypass. The Director asks exactly as it would ask a person, and the harness plays
// the person. That is what makes the scenario a test of the production path rather than of
// a test-only shortcut.
//
// It records what it agreed to, so a scenario can prove a confirmation actually happened.
func (h *Harness) AutoConfirm(approve bool) {
	// The client is captured, not read from the harness each pass: Close() releases it,
	// and a poll that dereferenced the field afterwards would dereference nil.
	client, stop := h.Confirmations, h.stopAsk
	if client == nil {
		return
	}
	h.asking.Add(1)
	go func() {
		defer h.asking.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(25 * time.Millisecond):
			}
			status, err := client.Status()
			if err != nil {
				// The connection is going away, which happens at teardown.
				return
			}
			if status.Confirmation == nil {
				continue
			}
			ask := *status.Confirmation
			if _, err := client.Confirm(ask.ID, approve); err == nil {
				h.mu.Lock()
				h.answered = append(h.answered, ask)
				h.mu.Unlock()
			}
		}
	}()
}

// Answered returns the confirmations this harness agreed to.
func (h *Harness) Answered() []service.ConfirmationPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]service.ConfirmationPayload{}, h.answered...)
}

// ── Explorer ──────────────────────────────────────────────────────────────────

// OpenExplorer opens a File Explorer window on a directory and waits for the Director to
// see it.
//
//	If any prerequisite fails: skip before UI input.
//
// The window is identified POSITIVELY — the application is Explorer and the title names
// this test's own directory — and the scenario skips if it cannot be. A harness that
// proceeded against "probably the right window" is the one that renames a real file.
func (h *Harness) OpenExplorer(w *Workspace) directorapi.Window {
	t := h.t
	t.Helper()

	// "/n," forces a NEW single-pane window and brings it to the front. Plain
	// `explorer.exe <dir>` is not enough on Windows 11: tabbed Explorer may open the
	// folder as a TAB in a window that already exists, leaving the new folder invisible
	// to a harness looking for a window — which is exactly what happened the first time
	// this scenario ran against a machine with Explorer already open.
	if err := exec.Command("explorer.exe", "/n,"+w.Dir).Start(); err != nil {
		t.Skipf("Explorer could not be opened on %s: %v", w.Dir, err)
	}
	h.explorerOpened, h.dirTitle = w.Dir, w.Dir
	want := filepath.Base(w.Dir)

	// Bring it to the FRONT, by title. The Director observes the foreground window, so a
	// window that opened behind is a window it cannot see — and a `go test` process does
	// not hold the foreground right, so Windows lets the new window open without giving
	// it focus. Setup, not the thing under test: the harness arranges the scene and the
	// Director performs the rename.
	//
	// By title rather than by application, because there may be several Explorer windows
	// and only one of them is this test's.
	waitFor(10*time.Second, func() bool { return winctx.ActivateTitle(want) == nil })

	var found directorapi.Window
	ok := waitFor(20*time.Second, func() bool {
		win, got := h.explorerWindow(want)
		if got {
			found = win
		}
		return got
	})
	if !ok {
		// What WAS visible, so a failure is diagnosable rather than merely disappointing.
		// The usual causes are an accessibility bridge that did not attach and an
		// Explorer that titles its windows differently than expected.
		var seen []string
		if status, err := h.Client.Status(); err == nil {
			for _, win := range status.Windows {
				seen = append(seen, fmt.Sprintf("%s %q", win.Application, win.Title))
			}
		}
		t.Skipf("no Explorer window naming %q appeared within 20s, so the workspace "+
			"could not be positively identified and NO INPUT WAS SENT.\n"+
			"  visible windows: %v\n"+
			"  If that list is empty the accessibility bridge did not attach; if it "+
			"holds an Explorer window under a different title, this scenario's "+
			"identification rule needs widening rather than relaxing.",
			want, seen)
	}
	t.Logf("identified Explorer window %s %q", found.ID, found.Title)
	return found
}

// explorerWindow finds a visible Explorer window whose title names a folder.
//
// Both conditions, and neither alone: an Explorer window with a different title is a
// different folder, and a window with the right title in another application is not
// Explorer. Ambiguity — two matches — is a refusal, not a choice.
func (h *Harness) explorerWindow(title string) (directorapi.Window, bool) {
	status, err := h.Client.Status()
	if err != nil {
		return directorapi.Window{}, false
	}
	accepted := acceptedTitles(title, h.dirTitle)
	var matches []directorapi.Window
	for _, win := range status.Windows {
		if !accepted[strings.ToLower(strings.TrimSpace(win.Title))] {
			continue
		}
		app := strings.ToLower(win.Application)
		if app != "explorer" && app != "explorer.exe" {
			continue
		}
		matches = append(matches, win)
	}
	// Exactly one. Two matching windows is an AMBIGUITY, and acting on either would be
	// choosing for the user — the same refusal the binding layer makes.
	if len(matches) != 1 {
		return directorapi.Window{}, false
	}
	return matches[0], true
}

// acceptedTitles is the closed set of titles a folder window may legitimately carry.
//
// A SET of exact forms rather than a substring rule. Explorer titles a folder window with
// the folder's name; Windows 11 appends " - File Explorer"; and with "show full path in
// title bar" on it uses the path instead. All four are the window this test opened.
//
// A substring match would also accept a window whose title merely mentions the folder — a
// text editor with the path in its caption, a second Explorer showing a parent — which is
// exactly the loose identification that would send input somewhere unintended.
func acceptedTitles(name, path string) map[string]bool {
	const suffix = " - File Explorer"
	out := map[string]bool{}
	for _, base := range []string{name, path} {
		if base == "" {
			continue
		}
		out[strings.ToLower(base)] = true
		out[strings.ToLower(base+suffix)] = true
	}
	return out
}

// ── requests ──────────────────────────────────────────────────────────────────

// Submit sends one request and waits for it to finish, logging every event.
func (h *Harness) Submit(phrase string) service.OutcomePayload {
	t := h.t
	t.Helper()
	t.Logf("→ %s", phrase)

	out, err := h.Client.Execute(phrase, false, func(ev service.ResponseEnvelope) {
		switch ev.Type {
		case service.ResponseConfirmationRequired:
			var ask service.ConfirmationPayload
			if ev.Decode(&ask) == nil {
				t.Logf("   ? %s", ask.Question())
			}
		case service.ResponseProgress:
			var pr service.ProgressPayload
			if ev.Decode(&pr) == nil && pr.Detail != "" {
				t.Logf("   · %s", pr.Detail)
			}
		}
	})
	if err != nil {
		t.Fatalf("submitting %q: %v", phrase, err)
	}
	for _, line := range out.Trace {
		mark := " "
		if !line.OK {
			mark = "!"
		}
		t.Logf("   %s %-10s %s", mark, line.Stage, line.Detail)
	}
	t.Logf("← %s: %s", out.State, out.Message)
	return out
}

// SelectedResource is the backing object the Director can see for the selected item.
//
//	Confirm the observed backing resource equals the expected full path.
//
// Asked BEFORE the request under test, which is the point: it establishes that the Director
// is about to act on the file this test created, rather than discovering afterwards that it
// acted on something else. A harness that only checked the result would have no way to tell
// "it renamed the right file" from "it renamed a file that happened to be there".
//
// Read from the Director's own perception diagnostics, so it is the SAME identity the
// binding layer will use — not a second opinion assembled by the harness.
func (h *Harness) SelectedResource() (*directorapi.ResourceIdentity, string) {
	p, err := h.Client.Explain()
	if err != nil {
		return nil, "the Director could not be asked what it sees: " + err.Error()
	}
	var found []*directorapi.ResourceIdentity
	for _, row := range p.Observations {
		if row.Resource.Known() {
			found = append(found, row.Resource)
		}
	}
	switch len(found) {
	case 0:
		return nil, "the Director sees no object with a file behind it in this window"
	case 1:
		return found[0], ""
	}
	// Several. The bridge attaches a resource only to the single selected item, so more
	// than one means the view has moved on since the selection — which is a reason to
	// stop rather than to pick.
	return nil, fmt.Sprintf("the Director sees %d objects with files behind them, so which "+
		"one is selected cannot be established", len(found))
}

// Graph returns the action graph the production runtime produced.
//
// Read from the FILE the service wrote rather than through the service's own history
// summary, for the same reason the filesystem is read rather than the Director asked:
// the claim under test is what the production runtime recorded, and a summary shaped for
// a listing cannot carry the binding snapshot or the verification evidence.
func (h *Harness) Graph() []actiongraph.ActionNode {
	t := h.t
	t.Helper()
	path := filepath.Join(h.Dir, "action-graph.json")
	nodes, err := actiongraph.Load(path)
	if err != nil {
		t.Fatalf("reading the action graph at %s: %v", path, err)
	}
	return nodes
}

// Cancel stops whatever is running, for a scenario that must abandon a request.
func (h *Harness) Cancel() {
	if h.Confirmations == nil {
		return
	}
	_, _ = h.Confirmations.Cancel()
}
