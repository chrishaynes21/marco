package theaterhost

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// The ACCESSIBILITY actor: one way to play the part, not the part itself.
//
// # What this is
//
// The Theater asks for a semantic target to be activated. This Actor answers "I can do that, by
// asking the application's own accessibility API", finds the control by the word on it, and
// invokes it.
//
// `Control.Called` lives HERE now, and only here. It was briefly what a saved play said, which
// welded a provider into a behaviour; it is a good execution strategy and a bad durable meaning.
// A play says what the person wanted; this says how one Actor does it tonight. See
// [[ADR-068-the-theater-is-the-durable-semantic-world]].
//
// # Why it goes through the ordinary act
//
// Because there is exactly one route from the Director to an effect, and adding a second — a
// direct call into the bridge from a host — would be a bypass of the whole marcoexec discipline.
// This Actor asks the Accessibility act for the same capability any Marco program would.

// AccessibilityActor performs a target activation through the accessibility bridge.
type AccessibilityActor struct {
	// host is the Accessibility act's implementation, or nil when this machine has none.
	host runtime.Host
}

// NewAccessibilityActor casts the accessibility bridge as an Actor, or returns one that cannot
// perform when there is no bridge.
//
// A nil host is a real answer, not an error: a machine without an accessibility bridge simply has
// one fewer actor available tonight, and the Theater says so rather than failing obscurely.
func NewAccessibilityActor(host runtime.Host) *AccessibilityActor {
	return &AccessibilityActor{host: host}
}

// The window an actor searches in TRAVELS with the request, never on the actor.
//
// It was briefly a builder — `InWindow` — set once at construction. One Theater serves a saved
// play and a live rehearsal in the same process, and a scope stored on a shared actor is one
// caller's window silently applied to another's production. It rides on Target and comes back on
// Candidate, so the program acts in the window the search found it in.

func (a *AccessibilityActor) Name() string { return "accessibility" }

// Available reports whether there is a bridge to ask at all.
func (a *AccessibilityActor) Available(context.Context) bool { return a.host != nil }

// Find asks the accessibility source what it has by this name.
//
// # Why this returns at most one, and why that is not the same as choosing
//
// Because the accessibility host resolves a name itself and REFUSES an ambiguous one — it answers
// `target_ambiguous` rather than picking. So the several-candidates case arrives here as a
// refusal, and this translates it back into the shape the Theater reasons about: two candidates,
// which the Theater then declines.
//
// The alternative — having this Actor return the first of several — would move a semantic
// decision inside a lookup where nothing could see it.
func (a *AccessibilityActor) Find(ctx context.Context, t Target) ([]Candidate, error) {
	if a.host == nil {
		return nil, fmt.Errorf("no accessibility bridge")
	}
	// A LOOK, not an action. This asks the host to resolve the name and tells it to do
	// nothing: the Snapshot capability reads, and reading is how an actor decides whether it
	// can play the part before it is cast.
	look := map[string]string{"Name": t.Name}
	if t.Window != "" {
		look["Window"] = t.Window
	}
	status, _, err := a.invoke(ctx, "Snapshot", look)
	if err != nil {
		return nil, err
	}
	switch {
	case status == "ok":
		return []Candidate{{Handle: t.Name, Describes: t.Name, Window: t.Window}}, nil
	case strings.Contains(status, string(TargetAmbiguous)):
		// TWO, deliberately, and the count is not the point — the Theater refuses on
		// "more than one", and this Actor is reporting that it could not tell rather
		// than how many it saw.
		return []Candidate{
			{Handle: t.Name, Describes: t.Name, Window: t.Window},
			{Handle: t.Name, Describes: t.Name + " (another)", Window: t.Window},
		}, nil
	default:
		return nil, nil
	}
}

// Perform invokes the control, resolved fresh at the moment of acting.
//
// Resolved AGAIN rather than carried from Find. The tree may have redrawn between deciding and
// doing, and a handle held across that gap is exactly the stale-identity failure durable targets
// exist to avoid — the host looks the name up against the tree as it stands now.
func (a *AccessibilityActor) Cast(c Candidate, w activate.Way) (string, bool) {
	if strings.TrimSpace(c.Handle) == "" {
		return "", false
	}
	called, err := quote(c.Handle)
	if err != nil {
		// A name that cannot be written down as a Marco literal is one this Actor cannot
		// express. Not a refusal by the control — there is nothing to send.
		return "", false
	}
	var b strings.Builder
	b.WriteString("// Cast by the accessibility actor.\n")
	b.WriteString("use accessibility.\n\n")
	b.WriteString("the Cast is an actor.\n\n")
	b.WriteString("this can Run.\n")
	b.WriteString("this's Run does...\n")
	fmt.Fprintf(&b, "    the ctl is a Control with Name %s", called)
	if c.Window != "" {
		win, werr := quote(c.Window)
		if werr != nil {
			return "", false
		}
		fmt.Fprintf(&b, ", Window %s", win)
	}
	b.WriteString(".\n")
	fmt.Fprintf(&b, "    do Accessibility's %s with ctl.\n", w)
	b.WriteString("    this is ok!\n\n")
	b.WriteString("the App is a script.\n\n")
	b.WriteString("do Cast's Run...\n")
	b.WriteString("    when ok?\n")
	b.WriteString("        log \"cast: done\".\n")
	b.WriteString("    or?\n")
	b.WriteString("        log that's error.\n")
	return b.String(), true
}

// quote renders a Go string as a Marco text literal.
//
// The one place user text becomes program text in this Actor. A name carrying a quote or a
// backslash must not be able to change what the program says, and a name that cannot be written
// at all is reported as inexpressible rather than escaped into something else.
func quote(s string) (string, error) {
	if strings.ContainsAny(s, "\"\\\n\r") {
		return "", fmt.Errorf("cannot be written as a Marco literal")
	}
	return "\"" + s + "\"", nil
}

// invoke calls one Accessibility capability with a Control set.
func (a *AccessibilityActor) invoke(ctx context.Context, action string,
	fields map[string]string) (string, runtime.Value, error) {

	set := runtime.NewSet()
	for k, v := range fields {
		set.Put(k, runtime.Text(v))
	}
	status, out, err := a.host.Invoke(runtime.HostCall{
		Act: "Accessibility", Action: action, Input: runtime.SetVal(set), Ctx: ctx,
	})
	if err != nil {
		return "", out, err
	}
	if status != "ok" {
		// The host's own sentence, carried through so a refusal keeps its reason.
		return errText(out), out, nil
	}
	return status, out, nil
}

// errText is the message inside a failed reply, empty when there is none.
func errText(v runtime.Value) string {
	if e := v.AsError(); e != nil {
		return e.Message
	}
	return "failed"
}
