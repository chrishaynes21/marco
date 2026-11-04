package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// waitInUse polls until InUse satisfies want, because a connection is counted and uncounted
// on the server's own goroutine.
func waitInUse(t *testing.T, s *Server, want func(string) bool, what string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := s.InUse()
		if want(got) {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: InUse = %q", what, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A Director started for somebody stops once nothing is using it, so everything that IS using
// it has to say so: a running command, a question still owed an answer, an open client. And
// once each of those has gone, nothing does.
func TestTheServiceSaysWhatIsUsingIt(t *testing.T) {
	rt := newFakeRuntime()
	release := make(chan struct{})
	started := make(chan struct{})
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		close(started)
		<-release
		return execute.Outcome{Status: directorapi.ResultDone, Message: "done"}
	}
	srv, dir := serve(t, rt)
	idle := func(s string) bool { return s == "" }

	waitInUse(t, srv, idle, "a fresh service")

	c := dial(t, dir)
	waitInUse(t, srv, func(s string) bool { return strings.Contains(s, "connection") },
		"an open client")

	go func() { _, _ = c.Execute("click File", false, nil) }()
	<-started
	if got := srv.InUse(); !strings.Contains(got, `running "click File"`) {
		t.Fatalf("a running command: InUse = %q", got)
	}
	close(release)
	_ = c.Close()
	waitInUse(t, srv, idle, "after the command finished and the client hung up")

	srv.pending.Set(ClarificationPayload{Question: "which one?"})
	if got := srv.InUse(); !strings.Contains(got, "question") {
		t.Fatalf("an unanswered question: InUse = %q", got)
	}
	srv.pending.Clear()
	waitInUse(t, srv, idle, "after the question was cleared")
}

// THE IDLE STOP AND A CONNECTION ARRIVING ARE ORDERED, both ways, and the order is forced rather
// than hoped for.
//
// A connection that reaches admission while the stop is being decided waits for the decision,
// and is then REFUSED with CodeStopping before any request of it is read, so no work is accepted
// that the stop cancels. (Reading InUse and then calling Shutdown would admit it between the two.)
func TestAConnectionArrivingAsTheIdleStopIsDecidedIsRefusedBeforeAnyWork(t *testing.T) {
	rt := newFakeRuntime()
	srv, dir := serve(t, rt)
	ep, ok := ReadEndpoint(dir)
	if !ok {
		t.Fatal("no endpoint was published")
	}

	arrived := make(chan struct{}, 1)
	srv.admitting = func() { arrived <- struct{}{} }
	type dialed struct {
		c   *Client
		err error
	}
	result := make(chan dialed, 1)
	srv.deciding = func() {
		// The decision has been made and admission is held shut. A client connects NOW, and
		// the stop does not go on until that connection is waiting to be admitted.
		go func() {
			c, err := Dial(ep, 2*time.Second)
			result <- dialed{c, err}
		}()
		<-arrived
	}

	if why, stopped := srv.ShutdownIfUnused(nil); !stopped {
		t.Fatalf("an unused service did not stop: %q", why)
	}
	r := <-result
	if r.err == nil {
		_ = r.c.Close()
		t.Fatal("a connection arriving after the decision to stop was admitted")
	}
	var re *RequestError
	if !errors.As(r.err, &re) || re.Code != CodeStopping {
		t.Fatalf("the refusal does not say the Director is stopping: %v", r.err)
	}
	if calls := rt.phrases(); len(calls) != 0 {
		t.Fatalf("work reached the runtime from a refused connection: %q", calls)
	}
}

// And one admitted BEFORE the decision is counted, so the service is in use and does not stop,
// and its work is done. Once it has gone, the service stops.
func TestAConnectionAdmittedBeforeTheIdleStopKeepsTheDirector(t *testing.T) {
	rt := newFakeRuntime()
	srv, dir := serve(t, rt)
	c := dial(t, dir)
	waitInUse(t, srv, func(s string) bool { return s != "" }, "an admitted client")

	if why, stopped := srv.ShutdownIfUnused(nil); stopped ||
		!strings.Contains(why, "connection") {
		t.Fatalf("stopped under an admitted client: stopped=%v why=%q", stopped, why)
	}
	if _, err := c.Execute("click File", false, nil); err != nil {
		t.Fatalf("the admitted client's work was not done: %v", err)
	}
	if calls := rt.phrases(); len(calls) != 1 || calls[0] != "click File" {
		t.Fatalf("runtime saw %q", calls)
	}
	_ = c.Close()
	waitInUse(t, srv, func(s string) bool { return s == "" }, "after the client hung up")
	if why, stopped := srv.ShutdownIfUnused(func() string { return "observing" }); stopped ||
		why != "observing" {
		t.Fatalf("the runtime's account was not asked: stopped=%v why=%q", stopped, why)
	}
	if why, stopped := srv.ShutdownIfUnused(func() string { return "" }); !stopped {
		t.Fatalf("did not stop once unused: %q", why)
	}
	select {
	case <-srv.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("decided to stop and did not")
	}
}
