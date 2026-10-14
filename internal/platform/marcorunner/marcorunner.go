// Package marcorunner compiles and executes Marco programs on the Director's behalf.
//
// It is the other half of the boundary. internal/director/marcostep produces a
// program; this runs it through Marco's real front end — lexer.Lex, parser.Parse,
// graph.Build, compile.Compile, runtime.RunWithHostsContext — by calling
// driver.RunSourceWithHostsCtx, which is the same entry point `marco run` uses.
//
// It lives in internal/platform for the reason every other adapter does: it knows
// both systems, so neither has to know the other. The Director sees only
// directorapi.MarcoRunner.
//
// The one thing this package adds beyond "call the driver" is a host TEE. Marco
// resolves a capability's return into a frame, and the driver hands back only an
// error — but ClipboardGet and GetValue exist to produce values the Director needs in
// Go. Rather than have the program log them and parse the text back, the runner wraps
// each host and records what it returned. Marco still executes the program; nothing
// is intercepted or substituted.
package marcorunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Runner executes Marco programs with a fixed set of hosts.
type Runner struct {
	hosts map[string]runtime.Host
	// dir is where a `use <module>.` import looks for a sibling file before falling
	// back to the built-in surface. Empty is correct for the Director: os and uia are
	// both embedded, and a stale sibling copy shadowing them is exactly the drift
	// osmod's test guards against.
	dir string
}

// New returns a runner over the given hosts, keyed by act name ("OS",
// "Accessibility"). A "*" entry serves any act with no specific host.
func New(hosts map[string]runtime.Host) *Runner {
	return &Runner{hosts: hosts}
}

var _ directorapi.MarcoRunner = (*Runner)(nil)

// Run compiles and executes one program.
//
// A compile failure returns an error and NOTHING is executed — the guarantee that
// makes an unsupported operation safe to attempt. That is not incidental: the
// compiler rejects `OS has no capability "X"` before the runtime is ever entered, so
// a step naming something Marco does not export cannot mutate the desktop.
// actionHost is the OS host with its cursor-restore withheld, and nothing else changed.
//
// # Why one of the driver's two courtesies does not belong here
//
// `driver.RunSourceWithHostsCtx` offers a real host two deferred guarantees. They look alike and
// they are not the same kind of thing.
//
// Releasing a held key is SAFETY. A KeyDown with no KeyUp leaves a key stuck down in somebody's
// application after Marco has gone, and nothing later can undo it. It fires here.
//
// Restoring the cursor is COURTESY, and it is courtesy to a person whose hand is on the mouse: a
// menu macro should put the pointer back where they left it rather than parking it on whatever
// it clicked last. That is right for a ROUTE, which is one complete errand.
//
// A Director action is not one complete errand. It is one operation in a deliberate sequence —
// move the pointer to a control, let the interface notice, then press it — and each operation is
// its own program. Restoring the cursor at the end of each one undoes the move before the press
// that depended on it, which makes the sequence impossible to express at all.
//
// Measured, not reasoned about. A live Rocket League run moved the pointer onto PLAY, waited
// 400ms and reported that the control's pixels had changed by exactly 0.0/255 — because the
// pointer was never there. The driver had put it back the instant the move's program ended, and
// the click that followed arrived on an item the cursor had never hovered.
//
// So: a Director program leaves the pointer where it put it. There is no person waiting to get
// their mouse back mid-sequence, and the next operation in the sequence is relying on where the
// last one left it.
type actionHost struct{ runtime.Host }

// ReleaseHeld keeps the safety half, delegating when the real host offers it.
func (a actionHost) ReleaseHeld() {
	if r, ok := a.Host.(interface{ ReleaseHeld() }); ok {
		r.ReleaseHeld()
	}
}

func (r *Runner) Run(ctx context.Context, name, program string) (directorapi.MarcoResult, error) {
	var out bytes.Buffer
	tees := make(map[string]*tee, len(r.hosts))
	hosts := make(map[string]runtime.Host, len(r.hosts)+1)
	for act, h := range r.hosts {
		t := &tee{inner: h}
		tees[act], hosts[act] = t, t
	}

	// The driver's SAFETY GUARANTEES are keyed off hosts["*"]: it releases any key a
	// KeyDown left held, and restores the cursor, both deferred so they fire on
	// success, error and cancellation alike.
	//
	// Passing only named acts meant neither ever ran for a Director program. Nothing
	// reaches it today — marcoexec emits no KeyDown — but "the guarantee exists
	// somewhere in the codebase" is not the same as "the guarantee covers this path",
	// and the difference is a key left held in the user's application.
	//
	// ONE GUARANTEE, NOT BOTH. See actionHost.
	if osHost, ok := r.hosts["OS"]; ok {
		hosts["*"] = actionHost{Host: osHost}
	}

	err := driver.RunSourceWithHostsCtx(ctx, program, r.dir, name, &out, hosts)

	res := directorapi.MarcoResult{
		Output:   out.String(),
		Returned: map[string]directorapi.MarcoValue{},
	}
	for _, t := range tees {
		t.mu.Lock()
		for k, v := range t.seen {
			res.Returned[k] = v
			if v.Status == "failed" {
				res.Failed = append(res.Failed, k)
			}
		}
		t.mu.Unlock()
	}
	if err != nil {
		return res, err
	}
	return res, nil
}

// tee wraps a Host and records what each capability returned.
//
// A pure observer: the call goes to the real host untouched and its results are
// returned untouched. If this ever needed to modify a call it would be a second
// execution path, which is the thing the boundary exists to prevent.
type tee struct {
	inner runtime.Host

	mu   sync.Mutex
	seen map[string]directorapi.MarcoValue
}

func (t *tee) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	status, data, err := t.inner.Invoke(c)

	v := directorapi.MarcoValue{Status: status}
	if v.Status == "" {
		v.Status = "ok"
	}
	if err != nil {
		v.Status, v.Error = "failed", err.Error()
	} else {
		decode(data, &v)
	}

	t.mu.Lock()
	if t.seen == nil {
		t.seen = map[string]directorapi.MarcoValue{}
	}
	// Keyed by act and capability, last write winning. A program that calls the same
	// capability twice keeps the LAST result, which is what a caller reading a value
	// back after a write wants.
	t.seen[fmt.Sprintf("%s's %s", c.Act, c.Action)] = v
	t.mu.Unlock()

	return status, data, err
}

// decode flattens a runtime.Value into the transport form.
//
// Through JSONFromValue rather than by walking the Value tree, for the same reason
// uiaclient does it: one marshal against a hundred lines of manual extraction that
// would need updating with every field.
func decode(val runtime.Value, out *directorapi.MarcoValue) {
	if val.IsAbsent() {
		return
	}
	if val.IsError() {
		if e := val.AsError(); e != nil {
			out.Error = e.Message
		}
		return
	}
	raw, err := json.Marshal(runtime.JSONFromValue(val))
	if err != nil {
		return
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return
	}
	switch v := decoded.(type) {
	case string:
		out.Text = v
	case map[string]any:
		out.Fields = make(map[string]string, len(v))
		for k, member := range v {
			out.Fields[k] = scalar(member)
		}
	default:
		out.Text = scalar(v)
	}
}

// scalar renders a JSON scalar as the text the Director will compare against.
//
// Booleans and numbers become their literal forms rather than being dropped: the
// Clipboard set's IsText and Empty are booleans, and losing them would collapse the
// three-state clipboard rule back to two.
func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case nil:
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
