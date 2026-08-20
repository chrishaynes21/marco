package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// A learned play, as a COMMAND.
//
// # What was measured
//
// PERFORM arrived on the observation door beside a dozen reads and was routed like one: no
// registry entry, no cancellable context, no lifetime the service knew about. So while a play was
// really typing and clicking:
//
//   - `director status` reported nothing running;
//   - `director stop` answered "nothing is running" — CANCEL_ACTIVE could not reach it at all;
//   - a second mutating request was accepted, and two things drove one desktop;
//   - every cancellation check inside rehearse.Live.Perform was dead, because the only context
//     ever handed in was context.Background().
//
// These enter through the real Server and a real Client, because the defect was entirely in the
// wiring: every piece worked, and nothing joined them.

// performingRuntime is a Director that can carry out a learned outcome.
//
// Through the PRODUCTION door: it implements Performer, which is what Server.performGoal asserts
// for. A fake that the server reached some other way would prove nothing about the routing.
type performingRuntime struct {
	*fakeRuntime
	started chan struct{}
	once    sync.Once
	perform func(ctx context.Context, q PerformQuery) (PerformView, error)
}

func (p *performingRuntime) PerformGoal(ctx context.Context, q PerformQuery) (PerformView, error) {
	p.once.Do(func() { close(p.started) })
	return p.perform(ctx, q)
}

func newPerformingRuntime(
	do func(ctx context.Context, q PerformQuery) (PerformView, error)) *performingRuntime {

	return &performingRuntime{
		fakeRuntime: newFakeRuntime(), started: make(chan struct{}), perform: do,
	}
}

// askToPerform sends one PERFORM and decodes the view, exactly as `director perform` and the
// `marco do` bridge both do.
func askToPerform(t *testing.T, c *Client, q PerformQuery) PerformView {
	t.Helper()
	raw, err := c.Observation(ObserveQuery{Perform: &q})
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	var v PerformView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decoding the view: %v", err)
	}
	return v
}

// STOPPING A PERFORMANCE REPORTS IT AS CANCELLED.
//
// # The mutations this kills
//
//   - hand PerformGoal a context.Background(): the runtime never sees Done, the cancel never
//     reaches the walk, and this times out.
//   - drop the registry Begin: `Cancel` finds no active command and answers "nothing is
//     running", which is what `director stop` said while a play was typing.
//   - render a cancelled performance as a failure: the command record and the view both stop
//     distinguishing "you stopped it" from "it broke".
func TestStoppingAPerformanceReportsItAsCancelled(t *testing.T) {
	rt := newPerformingRuntime(func(ctx context.Context, q PerformQuery) (PerformView, error) {
		select {
		case <-ctx.Done():
			// What Runtime.PerformGoal does at its next step boundary.
			return PerformView{
				Goal: q.Name, Refusal: "cancelled",
				Say: "You stopped it after 1 of 3 steps.",
			}, nil
		case <-time.After(5 * time.Second):
			return PerformView{Goal: q.Name, Arrived: true, Say: "Done."}, nil
		}
	})

	srv, dir := serve(t, rt)
	runner, stopper := dial(t, dir), dial(t, dir)

	// The request is answered on another goroutine, so nothing here reports through *testing.T
	// from it: a helper that called Fatalf off the test goroutine would race the cleanup that
	// closes these connections and hide whatever really happened.
	views := make(chan PerformView, 1)
	go func() {
		raw, err := runner.Observation(ObserveQuery{Perform: &PerformQuery{
			Name: "Open Dad's Settings", Subject: "subj_settings_mouse",
		}})
		var v PerformView
		if err == nil {
			_ = json.Unmarshal(raw, &v)
		}
		views <- v
	}()
	<-rt.started

	res, err := stopper.Cancel()
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Accepted {
		t.Fatalf("stopping a running play was refused: %q. A performance that never enters "+
			"the command registry is invisible to CANCEL_ACTIVE, so the Audience is told "+
			"nothing is running while their desktop is being typed into.", res.Message)
	}
	if res.Phrase != "Open Dad's Settings" {
		t.Errorf("the cancellation named %q, want the outcome being performed", res.Phrase)
	}

	select {
	case v := <-views:
		if v.Refusal != "cancelled" {
			t.Errorf("the view refused with %q, want cancelled", v.Refusal)
		}
		if v.Command == "" {
			t.Error("the view names no command, so nothing joins it to `director status`")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the performance did not stop. Its context is not the registry's — a play " +
			"handed context.Background() cannot be stopped at all.")
	}

	// AND THE HISTORY SAYS SO. "You stopped it" and "it failed" are different facts, and a
	// record that rendered them alike would tell somebody their play is broken.
	recent := srv.registry.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("%d command(s) recorded; a performance leaves no trace", len(recent))
	}
	if recent[0].State != CommandCancelled {
		t.Errorf("the record says %s, want cancelled", recent[0].State)
	}
	if !strings.Contains(recent[0].Reason, "stopped it") {
		t.Errorf("the record reads %q; it should say what the Audience did", recent[0].Reason)
	}
}

// A PERFORMANCE IS VISIBLE, AND IT REFUSES A CONCURRENT COMMAND.
//
// Two things driving one desktop is the state the mutating slot exists to prevent. A performance
// that stayed outside the registry was not merely unreportable — it was concurrent with anything
// else the Audience asked for.
//
// Deleting the registry Begin must fail this twice over: status goes blank and the second
// command runs.
func TestAPerformanceIsVisibleToStatusAndRefusesAConcurrentCommand(t *testing.T) {
	release := make(chan struct{})
	rt := newPerformingRuntime(func(ctx context.Context, q PerformQuery) (PerformView, error) {
		<-release
		return PerformView{Goal: q.Name, Arrived: true, Say: "Done."}, nil
	})

	_, dir := serve(t, rt)
	runner, other := dial(t, dir), dial(t, dir)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = runner.Observation(ObserveQuery{Perform: &PerformQuery{Name: "Mute Volume"}})
	}()
	<-rt.started
	// Released and JOINED before the test returns, so the in-flight request finishes before
	// the cleanup closes its connection.
	defer func() { close(release); <-done }()

	st, err := other.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Active == nil {
		t.Fatal("`director status` reports nothing running while a learned play is driving " +
			"real input. A performance outside the command registry is invisible.")
	}
	if st.Active.Phrase != "Mute Volume" {
		t.Errorf("status names %q, want the outcome being performed", st.Active.Phrase)
	}

	// A SECOND MUTATING REQUEST IS REFUSED, not queued: a play that waited its turn would
	// start against a screen the other command had already changed.
	id, _ := other.send(RequestExecutePhrase, ExecutePayload{Phrase: "click Edit"})
	resp, err := other.receive(id, 3*time.Second)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if resp.Type != ResponseBusy {
		t.Fatalf("a phrase submitted during a performance answered %s, want BUSY", resp.Type)
	}

	// And so is a second PERFORM, which reports it in the view's own vocabulary rather than
	// as a broken request.
	busy := askToPerform(t, other, PerformQuery{Name: "Open Mouse Settings"})
	if busy.Refusal != "busy" {
		t.Errorf("a concurrent performance refused with %q, want busy", busy.Refusal)
	}
}

// THE SUBJECT REACHES THE DIRECTOR.
//
// The identity is only useful if it survives the wire. A field that was added to the struct and
// dropped by the encoder would leave the join silently back on the words.
func TestTheSubjectTravelsOnThePerformRequest(t *testing.T) {
	asked := make(chan PerformQuery, 1)
	rt := newPerformingRuntime(func(ctx context.Context, q PerformQuery) (PerformView, error) {
		asked <- q
		return PerformView{Goal: q.Name, Arrived: true, Say: "Done."}, nil
	})
	_, dir := serve(t, rt)

	_ = askToPerform(t, dial(t, dir), PerformQuery{
		Name: "Open Dad S Settings", Subject: "subj_dads_settings", Application: "settings",
	})

	got := <-asked
	if got.Subject != "subj_dads_settings" {
		t.Errorf("the Director was asked with subject %q; the identity did not survive the "+
			"wire, so the join is back on the lossy name round trip", got.Subject)
	}
}

// A CANCELLED PERFORMANCE IS RECORDED AS CANCELLED.
//
// The mapping on its own, so the vocabulary can be checked without a running server. `cancelled`
// is the walker's word for an Audience-ended attempt — see cmd/director's
// TestTheCancelledWordIsTheWalkersWord — and it must not fall into the failure bucket.
func TestACancelledPerformanceIsRecordedAsCancelled(t *testing.T) {
	for _, c := range []struct {
		what string
		view PerformView
		want CommandState
	}{
		{"stopped", PerformView{Refusal: "cancelled"}, CommandCancelled},
		{"arrived", PerformView{Arrived: true}, CommandCompleted},
		{"refused", PerformView{Refusal: "not_learned"}, CommandFailed},
		{"ran but did not arrive", PerformView{}, CommandUnverified},
	} {
		if got := performState(c.view); got != c.want {
			t.Errorf("a %s performance is recorded as %s, want %s", c.what, got, c.want)
		}
	}
}

// A DIRECTOR THAT CANNOT PERFORM SAYS SO.
//
// Performer is asserted for rather than required of every Runtime: a Director that only observes
// is legitimate, and widening the interface would make every implementer claim it can drive input
// in order to keep compiling. The answer must still be an honest view rather than a fall-through
// to the observation branches, which would answer a performance with a list of sessions.
func TestADirectorThatCannotPerformRefusesInsteadOfObserving(t *testing.T) {
	_, dir := serve(t, newFakeRuntime())

	v := askToPerform(t, dial(t, dir), PerformQuery{Name: "Open Mouse Settings"})
	if v.Refusal != "no_performer" {
		t.Errorf("a Director with no performer answered %q, want no_performer", v.Refusal)
	}
}
