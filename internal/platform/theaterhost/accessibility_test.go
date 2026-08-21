package theaterhost_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// The accessibility actor names controls by their LABEL, and never carries a handle.
//
// # Why this is the restart proof
//
// A saved play runs tomorrow, against a tree that has been rebuilt. Every runtime id in it is
// different. The play survives that only if nothing between the durable target and the invocation
// ever holds one — so this asserts the shape of what actually goes over the bridge: a `Called`,
// every time, and no `Element`.
//
// It is a stronger claim than "it worked after a restart", because it holds for every restart
// rather than for the one a test happened to perform.

// recordingHost is an Accessibility host that answers as told and remembers what it was asked.
type recordingHost struct {
	calls  []runtime.HostCall
	fields []map[string]string
	reply  string
}

func (h *recordingHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.calls = append(h.calls, c)
	// The provider's own availability question, answered the way plugins/uia answers it.
	//
	// A fake standing in for a provider has to answer what the provider answers, or every test
	// using it measures a Marco whose one Actor reports itself unable to act — which is a real
	// state, and not the one any of these tests is about.
	if c.Action == "Available" {
		s := runtime.NewSet()
		ok := h.reply == "" || h.reply == "ok"
		s.Fields["Available"] = runtime.Bool(ok)
		if !ok {
			s.Fields["Reason"] = runtime.Text(h.reply)
		}
		return "ok", runtime.SetVal(s), nil
	}
	got := map[string]string{}
	if set := c.Input.AsSet(); set != nil {
		for _, k := range []string{"Name", "Element", "Window"} {
			if v, ok := set.Get(k); ok {
				if s := v.AsText(); s != "" {
					got[k] = s
				}
			}
		}
	}
	h.fields = append(h.fields, got)
	if h.reply == "" || h.reply == "ok" {
		return "ok", runtime.Absent(), nil
	}
	return "failed", runtime.ErrVal(&runtime.Err{Message: h.reply}), nil
}

// Nothing but a name crosses the bridge.
//
// Mutation: carry the element id from Find into Perform. It works in the session that learned it
// and fails at the first redraw — the exact failure a durable target exists to prevent, and the
// one that would look like flakiness rather than like a bug.
func TestTheAccessibilityActorSendsANameAndNeverAHandle(t *testing.T) {
	host := &recordingHost{}
	actor := theaterhost.NewAccessibilityActor(host, "uia.exe")

	found, err := actor.Find(context.Background(), theaterhost.Target{Name: "Mouse"})
	if err != nil || len(found) != 1 {
		t.Fatalf("Find returned %d candidate(s), err=%v", len(found), err)
	}
	// LOOKING goes over the bridge, and carries only a name.
	if len(host.fields) != 1 {
		t.Fatalf("%d call(s) over the bridge for a look", len(host.fields))
	}
	if host.fields[0]["Name"] != "Mouse" {
		t.Errorf("the look asked for Name=%q", host.fields[0]["Name"])
	}
	if host.calls[0].Action != "Snapshot" {
		t.Errorf("the look called %q, want a read", host.calls[0].Action)
	}

	// ACTING is a program, not a bridge call — and the name is what it carries.
	program, ok := actor.Cast(found[0], activate.Invoke)
	if !ok {
		t.Fatal("the actor could not express an activation")
	}
	if !strings.Contains(program, `Name "Mouse"`) {
		t.Errorf("the cast program does not name the control:\n%s", program)
	}
	if strings.Contains(program, "Element") {
		t.Errorf("the cast program carries an element id.\nA runtime id identifies a "+
			"control in the tree as it stands now; a play holding one works until the "+
			"first repaint and then fails obscurely.\n%s", program)
	}
	// And casting sends nothing: the Production boundary runs the program.
	if len(host.fields) != 1 {
		t.Errorf("casting reached the bridge %d extra time(s); it must perform nothing",
			len(host.fields)-1)
	}
}

// An ambiguous name reaches the Theater as several candidates, so the Theater refuses.
//
// The host resolves names itself and declines an ambiguous one. This actor must translate that
// back into "more than one" rather than swallowing it as "not found" — a swallowed ambiguity
// becomes silence, and the person is told their screen is wrong when it is Marco that cannot tell.
func TestAnAmbiguousNameBecomesSeveralCandidates(t *testing.T) {
	host := &recordingHost{reply: string(theaterhost.TargetAmbiguous) + ": two of them"}
	actor := theaterhost.NewAccessibilityActor(host, "uia.exe")

	found, err := actor.Find(context.Background(), theaterhost.Target{Name: "Mouse"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) < 2 {
		t.Fatalf("an ambiguous name produced %d candidate(s); the Theater refuses on more "+
			"than one, so fewer means the ambiguity was swallowed", len(found))
	}
}

// A name nothing matches is no candidates, not an error.
func TestANameNothingMatchesIsNoCandidates(t *testing.T) {
	host := &recordingHost{reply: string(theaterhost.TargetNotFound) + ": nothing here"}
	found, err := theaterhost.NewAccessibilityActor(host, "uia.exe").
		Find(context.Background(), theaterhost.Target{Name: "Mouse"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("%d candidate(s) for a name nothing matched", len(found))
	}
}

// With no bridge, the actor is simply not available tonight.
func TestWithNoBridgeTheActorIsUnavailable(t *testing.T) {
	actor := theaterhost.NewAccessibilityActor(nil, "uia.exe")
	av := actor.Availability(context.Background())
	if av.Ready {
		t.Error("an actor with no bridge reported itself available")
	}
	// And it must SAY WHY. An unavailable Actor with no reason is the case this whole change
	// exists to remove: a person told their play is broken when what is true is that their
	// machine cannot act, and nothing on any surface can tell them which.
	if av.Reason == "" {
		t.Error("an unavailable actor gave no reason, so no surface above it can say one")
	}
	if !strings.Contains(av.Reason, "uia.exe") {
		t.Errorf("the reason does not name the provider it could not reach: %q", av.Reason)
	}
	if _, err := actor.Find(context.Background(), theaterhost.Target{Name: "Mouse"}); err == nil {
		t.Error("an actor with no bridge searched anyway")
	}
	if !strings.EqualFold(actor.Name(), "accessibility") {
		t.Errorf("the actor is called %q", actor.Name())
	}
}

// A scoped search looks in that window, and the program acts in it.
//
// The scope rides on the Target into Find and comes back on the Candidate, so the program acts
// where the search found it. It was briefly stored on the Actor instead — one Theater serves a
// saved play and a live rehearsal in the same process, and a scope on a shared actor is one
// caller's window silently applied to another's production.
//
// Deleting either half — the Window in the look, or the Window in the cast — must fail this.
func TestAScopedProductionLooksAndActsInThatWindow(t *testing.T) {
	host := &recordingHost{}
	actor := theaterhost.NewAccessibilityActor(host, "uia.exe")

	found, err := actor.Find(context.Background(),
		theaterhost.Target{Name: "Mouse", Window: "hwnd:100"})
	if err != nil || len(found) != 1 {
		t.Fatalf("Find returned %d candidate(s), err=%v", len(found), err)
	}
	if host.fields[0]["Window"] != "hwnd:100" {
		t.Errorf("the look was scoped to %q; an unscoped search finds a control of the "+
			"right name in whatever else is open", host.fields[0]["Window"])
	}
	program, ok := actor.Cast(found[0], activate.Invoke)
	if !ok {
		t.Fatal("the actor could not express an activation")
	}
	if !strings.Contains(program, `Window "hwnd:100"`) {
		t.Errorf("the cast program is not scoped to the window it searched:\n%s", program)
	}
}

// An unscoped production says nothing about a window rather than saying "".
//
// Empty means "whatever is in front", and writing `Window ""` would say something different and
// wrong — the same rule the Control set follows for an omitted Kind.
func TestAnUnscopedProductionOmitsTheWindow(t *testing.T) {
	actor := theaterhost.NewAccessibilityActor(&recordingHost{}, "uia.exe")
	program, ok := actor.Cast(theaterhost.Candidate{Handle: "Mouse"}, activate.Invoke)
	if !ok {
		t.Fatal("the actor could not express an activation")
	}
	if strings.Contains(program, "Window") {
		t.Errorf("an unscoped production wrote a window into the program:\n%s", program)
	}
}

// A bridge that is THERE and cannot work is unavailable, and says the provider's own reason.
//
// # Why `host != nil` was never the question
//
// On the Director's path a host is constructed unconditionally — `bridgehost.New(path)` launches
// lazily and never returns nil — so a zero-byte, unbuilt or unrunnable `uia.exe` produced a
// non-nil host and an Actor that reported itself READY. The roster then said "accessibility:
// ready", the Theater cast it, and the failure arrived several steps later inside a play as
// `perform_failed`: a person told their play is broken when what is true is that their machine
// cannot act at all.
//
// So the mutation this exists to catch is not a deleted nil check. It is dropping the round trip
// — `return a.host != nil` — which leaves TestWithNoBridgeTheActorIsUnavailable green, because
// that test only ever supplies a nil host. This one supplies a host that ANSWERS, and answers no.
//
// Both halves are load-bearing. Ready must be false, or the Theater casts an actor that cannot
// act; and the provider's own sentence must survive, or every surface above can say nothing more
// useful than "unavailable" about a machine somebody has to fix.
func TestAProviderThatSaysItCannotActIsUnavailableWithItsReason(t *testing.T) {
	const said = "uia.exe could not be started"
	host := &recordingHost{reply: said}
	actor := theaterhost.NewAccessibilityActor(host, "uia.exe")

	av := actor.Availability(context.Background())
	if av.Ready {
		t.Fatal("an actor whose provider says it cannot act reported itself ready.\n" +
			"Availability that only asks whether a host object exists cannot fail: the " +
			"Director builds one for a path that holds nothing, the roster says " +
			"\"accessibility: ready\", and the refusal arrives mid-play as perform_failed.")
	}
	if !strings.Contains(av.Reason, said) {
		t.Errorf("the provider's own reason did not survive: %q, want it to carry %q.\n"+
			"A reason invented here would send somebody looking for the wrong fault.",
			av.Reason, said)
	}
	// And it was ASKED — the answer came over the bridge rather than from a path check.
	var asked bool
	for _, c := range host.calls {
		if c.Action == "Available" {
			asked = true
		}
	}
	if !asked {
		t.Error("nothing asked the provider whether it could act")
	}
}
