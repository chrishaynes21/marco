package runtime

import (
	"context"
	"fmt"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Host implements the foreign actions of an act (see spec/Hosts.md). A foreign
// action — an exported act capability with no Marco body — dispatches here
// instead of walking a BodyBlock. The returned (status, data) resolves the
// calling frame through the same path a Marco `this is <status> with <data>!`
// return takes; a non-nil err resolves the frame as `failed`.
type Host interface {
	Invoke(call HostCall) (status string, data Value, err error)
}

// HostCall is one foreign invocation handed to a Host.
type HostCall struct {
	Act    string          // owning act, e.g. "OS"
	Action string          // capability, e.g. "Key"
	Input  Value           // the `with ...` payload (may be Absent)
	Out    io.Writer       // serialized with `log`; hosts that print must use this
	Ctx    context.Context // canceled when the calling frame's tree is canceled
}

// DryRunHost is the default host: it logs the call and resolves ok. It performs
// no real OS effect, which keeps programs runnable — and golden tests
// deterministic and cross-platform — without a real provider registered.
type DryRunHost struct{}

func (DryRunHost) Invoke(c HostCall) (string, Value, error) {
	if c.Input.IsAbsent() {
		fmt.Fprintf(c.Out, "[dryrun] %s's %s\n", c.Act, c.Action)
	} else {
		fmt.Fprintf(c.Out, "[dryrun] %s's %s with %s\n", c.Act, c.Action, c.Input.String())
	}
	return "ok", Absent(), nil
}

// Run executes the script in the graph, writing `log` output to w. Foreign acts
// resolve through the dryrun host.
func Run(g *graph.Graph, w io.Writer) error {
	return RunWithHosts(g, w, nil)
}

// RunWithHosts is Run with explicit per-act host providers. An act named in
// hosts uses that Host for its foreign actions; any other foreign act falls
// back to the dryrun host.
func RunWithHosts(g *graph.Graph, w io.Writer, hosts map[string]Host) error {
	script := g.Script()
	if script == nil {
		return fmt.Errorf("no script declared")
	}
	if script.BodyBlock == nil {
		return fmt.Errorf("script %s has no body", script.Name)
	}
	return runFromEntry(g, script, w, hosts)
}

// hostFor returns the host registered for an act, or the default dryrun host.
func (r *runner) hostFor(act string) Host {
	if h, ok := r.hosts[act]; ok {
		return h
	}
	return r.defaultHost
}

// foreignNames recovers (act, action) names for a foreign action node from its
// owner's capability list.
func foreignNames(action *graph.Node) (act, name string) {
	if action.Owner != nil {
		act = action.Owner.Name
		for _, c := range action.Owner.Caps {
			if c.Action == action {
				name = c.Name
				break
			}
		}
	}
	return act, name
}

// invokeForeign dispatches a foreign action through its host and resolves child
// the same way a Marco return would, so call-site `when ok?`/`or?` arms see an
// identical frame. Runs on child's own goroutine (inside the spawn closure), so
// a blocking host call keeps `start`/`wait until` concurrent.
func (r *runner) invokeForeign(child *Frame, action *graph.Node) {
	act, name := foreignNames(action)
	call := HostCall{
		Act:    act,
		Action: name,
		Input:  child.Input,
		Out:    syncWriter{r},
		Ctx:    child.ctx(),
	}
	status, data, err := r.hostFor(act).Invoke(call)
	if err != nil {
		child.setStatus(StatusFailed, "")
		child.Error = &Err{Message: err.Error()}
		return
	}
	if status == "" {
		status = "ok"
	}
	setReturnStatus(child, status)
	if status == "failed" && data.IsError() {
		child.Error = data.AsError()
	} else if !data.IsAbsent() {
		child.Result = data
	}
}

// syncWriter funnels host output through the runner's outMu so it can't
// interleave with concurrent `log`/`inspect`/`show` writes.
type syncWriter struct{ r *runner }

func (w syncWriter) Write(p []byte) (int, error) {
	w.r.outMu.Lock()
	defer w.r.outMu.Unlock()
	return w.r.out.Write(p)
}
