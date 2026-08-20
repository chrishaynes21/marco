package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The visibility path, through the real transport.
//
// Everything here goes through a listening server and a connected client, because the
// thing being proved is that a presentation gets the Director's state — not that a struct
// can be copied. The parts a fake Director cannot prove (does the Director actually put
// its recognition verdict in there?) belong in cmd/director, and they are there.

// A. Director state reaches a presentation through the production request.
func TestDirectorStateReachesAPresentation(t *testing.T) {
	rt := newFakeRuntime()
	rt.playbill = playbill.View{
		Current: playbill.Current{
			Watching: true, Application: "testgame.exe",
			Recognition: playbill.Recognised, Screen: "the pause menu",
		},
		Learning: playbill.Learning{Stage: playbill.Observing},
		Recent: []playbill.Moment{
			{Seq: 7, At: time.Now(), Says: "the screen changed"},
		},
		Cursor: 7,
	}
	c := watchClient(t, rt)

	res, err := c.Playbill(PlaybillPayload{})
	if err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	v := res.View

	if !v.Reach.Live() {
		t.Fatalf("a running Director did not report itself present: %q", v.Reach)
	}
	if v.Current.Screen != "the pause menu" ||
		v.Current.Recognition != playbill.Recognised {
		t.Errorf("the Director's belief did not reach the presentation: %+v", v.Current)
	}
	if len(v.Recent) != 1 || v.Cursor != 7 {
		t.Errorf("the timeline did not survive the wire: %+v", v.Recent)
	}
	if v.Digest == "" {
		t.Error("no digest was computed, so a surface has nothing to coalesce on")
	}
	// It reads as a person would read it.
	if !strings.Contains(watchText(v), "I recognise this as “the pause menu”") {
		t.Errorf("the account did not render:\n%s", watchText(v))
	}
}

// B. THE MUTATION. Removing the runtime call must break this.
//
// Deleting `v := s.runtime.Playbill(p)` in playbillFor leaves a server that still answers,
// still says "present", and still renders — as an idle Director that is watching nothing
// and believes nothing, forever, with no error anywhere. That is exactly the failure this
// repository's wiring-test rule was written for.
func TestRemovingTheRuntimeCallEmptiesTheWatchSurface(t *testing.T) {
	rt := newFakeRuntime()
	rt.playbill = playbill.View{
		Current:  playbill.Current{Watching: true, Application: "testgame.exe"},
		Learning: playbill.Learning{Stage: playbill.Capturing, From: "the pause menu"},
	}
	c := watchClient(t, rt)

	res, err := c.Playbill(PlaybillPayload{})
	if err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	if rt.playbillCalls == 0 {
		t.Fatal("the server composed an account WITHOUT asking the Director. " +
			"Every surface would render a plausible, permanently idle Marco")
	}
	if res.View.Learning.Stage != playbill.Capturing {
		t.Fatalf("the Director's learning stage did not reach the surface: %q",
			res.View.Learning.Stage)
	}
	if !res.View.Current.Watching {
		t.Fatal("the surface reported not watching while the Director was")
	}
}

// The server contributes the COMMAND half, and only that half.
//
// The split exists so the layer that observes the desktop never gets a reference to the
// layer that drives it. If DOING started arriving from the runtime, that coupling has
// been reintroduced.
func TestTheServerContributesTheCommandHalf(t *testing.T) {
	rt := newFakeRuntime()
	release := make(chan struct{})
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		<-release
		return execute.Outcome{Status: directorapi.ResultDone}
	}
	c := watchClient(t, rt)

	go func() { _, _ = c.Execute("open settings", false, nil) }()
	defer close(release)

	// The account is answerable WHILE a command runs. A status surface that blocked
	// until a long command finished would be useless exactly when it is wanted.
	watcher := c.another()
	var v playbill.View
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := watcher.Playbill(PlaybillPayload{})
		if err != nil {
			t.Fatalf("Playbill during a command: %v", err)
		}
		v = res.View
		if v.Doing.Phase == playbill.Performing {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if v.Doing.Phase != playbill.Performing || v.Doing.What != "open settings" {
		t.Fatalf("a running command did not reach DOING: %+v", v.Doing)
	}
	if !strings.Contains(watchText(v), "I'm doing it.") {
		t.Errorf("the account did not say a command was running:\n%s", watchText(v))
	}
	if v.Normal().Word != "Running" {
		t.Errorf("the consumer reduction said %q while a command ran", v.Normal().Word)
	}
}

// A timeout becomes "couldn't verify", never "didn't work".
//
// The absence of confirmation is not evidence that nothing happened, and telling somebody
// their command failed when it may well have succeeded is the more dangerous mistake —
// because it invites them to run it again.
func TestATimeoutReadsAsUnverifiedRatherThanFailed(t *testing.T) {
	cases := map[CommandState]playbill.Phase{
		CommandCompleted:     playbill.Succeeded,
		CommandTimedOut:      playbill.Unverified,
		CommandUnverified:    playbill.Unverified,
		CommandBlocked:       playbill.Refused,
		CommandCancelled:     playbill.Cancelled,
		CommandFailed:        playbill.Failed,
		CommandInternalError: playbill.Failed,
	}
	for state, want := range cases {
		if got := phaseOf(state); got != want {
			t.Errorf("%s became %q, want %q", state, got, want)
		}
	}
}

// D. A pending question appears, and the answer travels the ORDINARY path.
func TestAPendingConfirmationAppearsAndIsAnsweredNormally(t *testing.T) {
	rt := newFakeRuntime()
	broker := NewConfirmationBroker()
	rt.confirmations = broker

	asked := make(chan struct{})
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		close(asked)
		ok, _ := broker.Confirm(ctx, execute.ConfirmationRequest{
			Scope: execute.ScopeAction, Action: "delete the selected files",
			Reason: "this cannot be undone",
		})
		if !ok {
			return execute.Outcome{Status: directorapi.ResultBlocked, Message: "you said no"}
		}
		return execute.Outcome{Status: directorapi.ResultDone}
	}
	c := watchClient(t, rt)
	go func() { _, _ = c.Execute("delete them", false, nil) }()
	<-asked

	watcher := c.another()
	var q *playbill.Question
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := watcher.Playbill(PlaybillPayload{})
		if err != nil {
			t.Fatalf("Playbill: %v", err)
		}
		if res.View.Question != nil {
			q = res.View.Question
			// The consumer reduction pulls forward for it, and for nothing else.
			if !res.View.Normal().Attention {
				t.Error("a pending question did not ask for the person's attention")
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if q == nil {
		t.Fatal("a pending confirmation never appeared on the visibility surface. " +
			"A person watching would see a command that had simply stopped")
	}
	if q.Via != playbill.ViaConfirm {
		t.Fatalf("the question named the response path %q; a surface would answer it "+
			"through the wrong door", q.Via)
	}
	if !strings.Contains(q.Asks, "delete the selected files") {
		t.Errorf("the question did not say what it was about: %q", q.Asks)
	}

	// THE ordinary path. Not a shortcut on the visibility surface — the same CONFIRM
	// request a terminal client makes, with the id the account carried.
	if _, err := watcher.Confirm(q.ID, true); err != nil {
		t.Fatalf("answering through the ordinary path failed: %v", err)
	}

	// And the question goes away, so nothing is left on screen waiting on nobody.
	for time.Now().Before(deadline) {
		res, _ := watcher.Playbill(PlaybillPayload{})
		if res.View.Question == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("the question stayed on the surface after it was answered")
}

// E. The surface cannot create authority.
//
// There is no request that turns a rendered "ready to rehearse" into a grant, and the
// account carries nothing that could be replayed into one.
func TestTheVisibilitySurfaceCannotAuthoriseAnything(t *testing.T) {
	rt := newFakeRuntime()
	rt.playbill = playbill.View{
		Learning: playbill.Learning{
			Stage: playbill.RehearsalOffered,
			From:  "the pause menu", To: "audio settings",
			Because: "I think I could do this — I'd need you to say yes first.",
		},
	}
	c := watchClient(t, rt)
	res, err := c.Playbill(PlaybillPayload{})
	if err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	// It SAYS so, which is the point of the surface...
	if !strings.Contains(watchText(res.View), "waiting for permission to try") {
		t.Errorf("the offer was not visible:\n%s", watchText(res.View))
	}
	// ...and there is no question attached, so there is nothing for a surface to answer
	// and nothing that could be mistaken for consent already given.
	if res.View.Question != nil {
		t.Error("an unasked rehearsal offer arrived carrying an answerable question")
	}
	// The PLAYBILL request is not mutating, so it can never take the execution slot.
	if RequestPlaybill.Mutating() {
		t.Fatal("PLAYBILL is a mutating request. Reading what Marco believes would " +
			"then queue behind — and block — the thing it is describing")
	}
}

// I. The Director does not care whether anybody is watching.
//
// A presentation disappearing mid-command must change nothing: a dropped connection is a
// client that stopped listening, not a user who changed their mind.
func TestAWatcherDisappearingDoesNotStopTheDirector(t *testing.T) {
	rt := newFakeRuntime()
	done := make(chan struct{})
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		select {
		case <-time.After(400 * time.Millisecond):
			close(done)
			return execute.Outcome{Status: directorapi.ResultDone}
		case <-ctx.Done():
			return execute.Outcome{Status: directorapi.ResultFailed, Message: "cancelled"}
		}
	}
	c := watchClient(t, rt)
	go func() { _, _ = c.Execute("do the thing", false, nil) }()

	// A watcher connects, reads, and hangs up mid-command.
	watcher := c.another()
	if _, err := watcher.Playbill(PlaybillPayload{}); err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	watcher.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the command did not finish after its watcher disconnected")
	}
}

// J. A Director that goes away is reported unavailable, not as stale belief.
func TestAnUnreachableDirectorIsReportedRatherThanRemembered(t *testing.T) {
	v := playbill.Unavailable(playbill.Absent, "the Director service is not running")
	if err := v.Admit(); err != nil {
		t.Fatalf("the unavailable account failed its own check: %v", err)
	}
	text := watchText(v)
	if !strings.Contains(text, "isn't watching anything") {
		t.Errorf("an absent Director did not read as absent:\n%s", text)
	}
	if strings.Contains(text, "I recognise") {
		t.Errorf("an absent Director claimed to recognise something:\n%s", text)
	}
}

// The account fails CLOSED. A Director that hands over something the guard refuses gets a
// playbill saying so, never the content.
func TestARefusedAccountIsReplacedRatherThanShipped(t *testing.T) {
	rt := newFakeRuntime()
	rt.playbill = playbill.View{
		// A window title where an application key belongs — the exact accident.
		Current: playbill.Current{Watching: true, Application: "Bank — Chrome"},
	}
	c := watchClient(t, rt)
	res, err := c.Playbill(PlaybillPayload{})
	if err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	if strings.Contains(res.View.Current.Application, "Bank") {
		t.Fatal("refused content reached the wire anyway")
	}
	if res.View.Why == "" {
		t.Error("the refusal was silent, so a leak would look like an empty panel")
	}
}

// Diagnostics is opt-in, so watching costs nothing extra.
func TestDiagnosticsAreOnlyAssembledWhenAskedFor(t *testing.T) {
	rt := newFakeRuntime()
	rt.playbill = playbill.View{
		Diagnostics: &playbill.Diagnostics{
			Providers: []playbill.Provider{{Name: "uia", Available: true, Observations: 4}},
		},
	}
	c := watchClient(t, rt)

	res, err := c.Playbill(PlaybillPayload{Diagnostics: true})
	if err != nil {
		t.Fatalf("Playbill: %v", err)
	}
	if res.View.Diagnostics == nil {
		t.Fatal("diagnostics were asked for and did not arrive")
	}
	if !strings.Contains(deepText(res.View), "uia") {
		t.Error("the diagnostics reading carried no provider")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// watchClient serves a fake Director and returns a client, plus the directory so a
// SECOND client can connect — which every question test needs, because the connection
// that submitted the command is blocked reading that command's events.
func watchClient(t *testing.T, rt Runtime) *watchConn {
	t.Helper()
	_, dir := serve(t, rt)
	return &watchConn{Client: dial(t, dir), dir: dir, t: t}
}

type watchConn struct {
	*Client
	dir string
	t   *testing.T
}

// another opens a second connection to the same service.
func (w *watchConn) another() *watchConn {
	w.t.Helper()
	return &watchConn{Client: dial(w.t, w.dir), dir: w.dir, t: w.t}
}

func watchText(v playbill.View) string { return linesText(v.Watch()) }
func deepText(v playbill.View) string  { return linesText(v.Deep()) }

func linesText(lines []playbill.Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(strings.Repeat("  ", l.Indent))
		b.WriteString(l.Text)
		b.WriteString("\n")
	}
	return b.String()
}
