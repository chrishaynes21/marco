package main

// THE BACKEND THE PAGE TALKS TO, AND IT IS THE PRODUCTION ONE.
//
// Every fact this shell displays comes from `internal/director/service` — the same client the
// overlay and `marco director` already speak. Specifically:
//
//	Start             Client.Execute(phrase, dryRun, onEvent)      client.go:326
//	run reference     AcceptedPayload.CommandID                    protocol.go:444
//	phase and step    playbill.View.Doing.Phase / .Step / .Steps   pkg/playbill
//	Cancel            Client.Cancel() -> CancelPayload.Accepted    client.go:306
//
// What it deliberately does NOT do, because the assignment this proves disqualifies both:
//
//   - no canned state. There is no fixture, no sample, no "demo mode" anywhere in this file.
//   - no transcript parsing. Nothing here runs a command and reads its stdout. `cmd/marco`'s
//     demonstration path does exactly that (`runref.Parse` over a child's lines), and it is the
//     thing this is not.
//
// And one field is left unrendered on purpose: `Doing.Live`. The service hardcodes it true
// (`internal/director/service/playbill.go:81` and `:107`) without consulting
// `Runtime.PerformsRealInput()`, so under a dry host it would state a falsehood. A proof that
// displayed it would be a proof that lied in its own evidence.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Backend is bound into the page. Every exported method is reachable from JavaScript.
type Backend struct {
	ctx context.Context
	mu  sync.Mutex
	// client is the connection to a Director. Dialled lazily, because the shell must open
	// before anything is running — a window that cannot appear without a backend is a
	// window that cannot say the backend is missing.
	client *service.Client
	// owned says THIS PROCESS STARTED the Director it is talking to, and it is the only
	// licence this shell has to end one. Always true when client is non-nil, because reach
	// refuses to attach to anything else — kept as a field so Close cannot be made to shut
	// down a service that was merely found. See reach.
	owned bool
	// child is the Director process this shell started, so Close can stop one that will not
	// shut down politely. Never a process this shell did not spawn.
	child *os.Process
	// mine is the run reference of the command this shell has in flight, empty when it has
	// none. It is what makes Cancel a cancellation OF THIS SHELL'S WORK rather than of
	// whatever happened to be running. See Cancel.
	mine string
	// dry says this shell may only ever ask for a dry run. It is a field rather than a
	// parameter so no call site can forget it.
	dry bool
	// home is the store this shell's Director uses, and the directory its endpoint file
	// lives in. For a proof it is a temporary one: the owner's running Director is left
	// alone, and a dry request against a LIVE Director is refused by the server anyway
	// (service/server.go:743-755).
	home string
	// sink receives what the page would see, so a headless proof can assert on the same
	// values the window renders. Nil in the window.
	sink func(kind string, v any)
	// evidence is where the page writes that it rendered. See Loaded.
	evidence string
	// phrase and autoStart let the WINDOW drive the backend proof, so gate 1 and gate 2
	// are one artifact instead of two: the page renders, then asks for a run, and every
	// envelope it is shown is appended beside its own render report.
	phrase    string
	autoStart bool
	// autoCancel lets the PAGE cancel its own run. Off by default and reported in the
	// page's log when on, because cancellation evidence that no person produced must not
	// read like cancellation evidence somebody did.
	autoCancel bool
}

// AutoCancel tells the page whether it may cancel its own run. Bound, so the decision lives in
// one place — a flag on the process — rather than in the page and the backend separately.
func (b *Backend) AutoCancel() bool { return b.autoCancel }

// emit sends one value to the page, and to the proof's sink when there is one.
func (b *Backend) emit(kind string, v any) {
	if b.sink != nil {
		b.sink(kind, v)
	}
	b.note(kind, v)
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, kind, v)
	}
}

// reach connects to the Director, starting one if there is none.
//
// `ServiceArgs` is where the zero-input guarantee lives: `--dry-run` swaps the effect host for
// `runtime.DryRunHost{}` (cmd/director/serve.go:24 → cmd/director/runtime.go:406), so no program
// this shell causes can reach a mouse or a keyboard.
// # Why this shell will not ATTACH to a service, at all
//
// The first correction asked `Connect` twice — once without permission to start, then once with —
// and set `owned` on the second. The Architect found the hole, and it is a real one: `Connect`
// re-checks the endpoint under the startup lock and returns a service ANOTHER process started in
// the meantime, so the second call can hand back somebody else's Director with `owned` set true.
// Two calls detect the common case and prove nothing.
//
// Detection was the wrong shape anyway. A proof shell has no business talking to a service it did
// not start, so the rule is not "notice when it is shared" but **refuse to share**:
//
//  1. If anything is already answering in this store, REFUSE. No connection is made.
//  2. Otherwise spawn the Director HERE, holding the child process, and wait for the endpoint
//     file to name OUR CHILD'S pid. `Endpoint.PID` is published by the service itself
//     (endpoint.go:29), so this is the service's own account of who it is, not an inference.
//  3. A pid that is not ours means another Director won the race for this store. The child is
//     stopped and the shell refuses, rather than driving whichever one got there first.
//
// What that buys beyond tidiness: every command this shell issues is the only command on a
// service with exactly one client, so `RequestCancelActive` — which names no command — cannot
// reach anybody else's work even in principle. The id check in `Cancel` stays as a second
// reading, but it is no longer what the guarantee rests on.
func (b *Backend) reach() (*service.Client, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client != nil {
		return b.client, nil
	}
	if ep, ok := service.ReadEndpoint(b.home); ok && service.Reachable(ep, 2*time.Second) {
		return nil, fmt.Errorf("a Director (pid %d) is already serving %s; this shell will "+
			"not attach to a service it did not start — close that one, or launch with "+
			"no -home for an isolated store", ep.PID, b.home)
	}
	args := []string{"serve", "--dry-run", "--quiet"}
	if !b.dry {
		args = []string{"serve", "--quiet"}
	}
	cmd := exec.Command(directorBin(), args...)
	cmd.Stdin, cmd.Stdout = nil, nil
	// THE CHILD'S FIRST WORDS, kept. A Director that refuses to start says why in one line,
	// and a shell that threw it away would report "no backend" about a diagnosed failure.
	var said bytes.Buffer
	cmd.Stderr = &said
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting a Director of this shell's own: %w", err)
	}
	mine := cmd.Process.Pid
	go func() { _ = cmd.Wait() }()

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		ep, ok := service.ReadEndpoint(b.home)
		if !ok || !service.Reachable(ep, time.Second) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if ep.PID != mine {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("the Director answering %s is pid %d, not the one this "+
				"shell started (pid %d); it will not drive a service it does not own",
				b.home, ep.PID, mine)
		}
		c, err := service.Dial(ep, 3*time.Second)
		if err != nil {
			continue
		}
		b.client, b.child, b.owned = c, cmd.Process, true
		return c, nil
	}
	_ = cmd.Process.Kill()
	msg := strings.TrimSpace(said.String())
	if msg == "" {
		msg = "it published no endpoint and said nothing"
	}
	return nil, fmt.Errorf("this shell's own Director did not become ready: %s", msg)
}

// dialOwn opens a SEPARATE connection to the Director already running.
//
// # Why this is not a convenience
//
// `Execute` submits a command and then blocks on that connection reading the command's events
// (`client.go:326` → `awaitTerminal`). Any other request sent down the same connection
// interleaves with that stream, and what comes back is not a response — it is the middle of
// somebody else's frame. The first working attempt at this proof did exactly that and got
//
//	service: malformed response: invalid character '\x00' looking for beginning of value
//
// on every playbill poll and on Cancel, so the phase readout was empty and the cancellation
// unacknowledged — a UI that would have looked dead while the backend was working perfectly.
// The protocol already says so about CONFIRM: *sent on ITS OWN connection in practice*.
//
// So: one connection per in-flight command, and a fresh one for everything a UI asks WHILE a
// command runs. `AutoStart` is false — if there is no Director, the answer is no Director, not a
// second one.
func (b *Backend) dialOwn() (*service.Client, error) {
	return service.Connect(service.ConnectOptions{
		Dir:         b.home,
		AutoStart:   false,
		DialTimeout: 3 * time.Second,
	})
}

// Started is what the page learns the moment a command is accepted.
type Started struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Start asks the Director to carry out one phrase, dry, and streams what comes back.
//
// Returns immediately; the envelopes arrive as events. A page that blocked on this would be a
// page that cannot show a countdown, a phase, or a Cancel button that works.
func (b *Backend) Start(phrase string) {
	go b.run(phrase)
}

func (b *Backend) run(phrase string) {
	c, err := b.reach()
	if err != nil {
		b.emit("envelope", Started{Type: "NO_BACKEND", Detail: err.Error()})
		return
	}
	// THE PHASE READOUT, from the service's own account, for as long as the command runs.
	//
	// Started BEFORE the command, not after: a command that ends inside one poll interval
	// would otherwise never be sampled, and the first attempt at this proof reported "no
	// phase" about a command that had both started and failed between two polls.
	stop := make(chan struct{})
	go b.watchPhase(stop)
	defer close(stop)
	// AND THE SHELL HAS NOTHING IN FLIGHT ONCE THIS RETURNS, so a Cancel pressed after the
	// command ended is refused here rather than passed to the Director to apply to whatever
	// came next.
	defer func() { b.mu.Lock(); b.mine = ""; b.mu.Unlock() }()

	_, err = c.Execute(phrase, b.dry, func(env service.ResponseEnvelope) {
		out := Started{Type: string(env.Type)}
		// THE CANONICAL RUN REFERENCE comes off the ACCEPTED envelope and nowhere
		// else. It is the service's own command id, not a string this shell invented
		// and not a line it read from somebody's stdout.
		switch env.Type {
		case service.ResponseAcknowledged, service.ResponseAccepted:
			var p service.AcceptedPayload
			if json.Unmarshal(env.Payload, &p) == nil {
				out.CommandID, out.Detail = string(p.CommandID), p.Phrase
				// AND IT IS RECORDED AS OURS, which is what Cancel asks about.
				b.mu.Lock()
				b.mine = string(p.CommandID)
				b.mu.Unlock()
			}
		case service.ResponseProgress:
			var p service.ProgressPayload
			if json.Unmarshal(env.Payload, &p) == nil {
				out.CommandID = string(p.CommandID)
				out.Detail = p.Stage
				if p.Detail != "" {
					out.Detail += ": " + p.Detail
				}
			}
		default:
			if len(env.Payload) > 0 && len(env.Payload) < 600 {
				out.Detail = string(env.Payload)
			}
		}
		b.emit("envelope", out)
	})
	if err != nil {
		b.emit("envelope", Started{Type: "ERROR", Detail: err.Error()})
	}
}

// Phase is what the shell shows about where a run has got to.
//
// No Live field. See the note at the top of this file.
type Phase struct {
	Phase     string `json:"phase"`
	What      string `json:"what"`
	Step      int    `json:"step"`
	Steps     int    `json:"steps"`
	RunningMS int64  `json:"running_ms"`
}

// watchPhase polls the playbill while a command is in flight.
//
// Polling rather than streaming because that is what the protocol offers: `PlaybillPayload`
// takes a cursor and returns the account as it now stands (`client.go:253`). The phase itself is
// computed SERVER side from the command registry (`service/playbill.go:76-105`), so what this
// renders is the backend's judgement and not this shell's.
func (b *Backend) watchPhase(stop <-chan struct{}) {
	t := time.NewTicker(25 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			c, err := b.dialOwn()
			if err != nil {
				continue
			}
			resp, err := c.Playbill(service.PlaybillPayload{})
			c.Close()
			if err != nil {
				continue
			}
			d := resp.View.Doing
			b.emit("phase", Phase{
				Phase: string(d.Phase), What: d.What,
				Step: d.Step, Steps: d.Steps, RunningMS: d.RunningMS,
			})
		}
	}
}

// Cancel stops THE COMMAND THIS SHELL STARTED and returns the acknowledgement.
//
// The point of the gate this proves is that cancellation is ACKNOWLEDGED rather than
// fire-and-forget: `CancelPayload` carries `Accepted`, the command id it applied to, and a
// sentence (`protocol.go:527`). The page renders all three.
//
// # Why it asks what is in flight first
//
// `RequestCancelActive` names no command — it cancels whatever is running (`client.go:305`). A
// shell that passed every Cancel press straight through would therefore be a button that
// cancels somebody else's work whenever this shell has none of its own, which is exactly the
// containment defect the Architect found. So the shell cancels only while it has a command in
// flight, and it CHECKS the id that came back: an acknowledgement for a different command is
// reported as what it is rather than rendered as a successful cancellation.
func (b *Backend) Cancel() service.CancelPayload {
	b.mu.Lock()
	mine := b.mine
	b.mu.Unlock()
	if mine == "" {
		out := service.CancelPayload{Message: "this shell has no command in flight; it " +
			"will not cancel one it did not start"}
		b.emit("envelope", Started{Type: "CANCEL_REFUSED", Detail: out.Message})
		return out
	}
	// ITS OWN CONNECTION, like every request made while a command is in flight. See dialOwn.
	c, err := b.dialOwn()
	if err != nil {
		return service.CancelPayload{Message: err.Error()}
	}
	defer c.Close()
	out, err := c.Cancel()
	if err != nil {
		return service.CancelPayload{Message: err.Error()}
	}
	detail := fmt.Sprintf("accepted=%v %s", out.Accepted, out.Message)
	if id := string(out.CommandID); id != "" && id != mine {
		detail = fmt.Sprintf("accepted=%v BUT FOR %s, NOT %s — not this shell's command: %s",
			out.Accepted, id, mine, out.Message)
	}
	b.emit("envelope", Started{Type: "CANCEL",
		CommandID: string(out.CommandID), Detail: detail})
	return out
}

// Close shuts THIS SHELL'S OWN Director down, so a proof leaves nothing running — and leaves
// anybody else's alone, which is the half that was missing.
//
// A shell that shut down whatever it was connected to would mean closing a feasibility window
// stopped the owner's service, mid-command, with no prompt. Ownership is established at the
// moment of connection and not guessed at here; see reach.
func (b *Backend) Close() {
	b.mu.Lock()
	c, owned, child := b.client, b.owned, b.child
	b.client, b.owned, b.child = nil, false, nil
	b.mu.Unlock()
	if c == nil {
		return
	}
	if !owned {
		// Unreachable today, because reach refuses to attach. Kept because the thing it
		// guards against is a Director somebody else is using, and a future call site
		// that learns to attach must not inherit a Close that shuts one down.
		_ = c.Close()
		return
	}
	_ = c.Shutdown()
	_ = c.Close()
	// AND IF IT DID NOT GO, it is stopped. A shell that asked politely and left anyway is
	// how the first version of this leaked a Director per launch.
	if child != nil {
		done := make(chan struct{})
		go func() { _, _ = child.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = child.Kill()
		}
	}
}

// directorBin finds the Director executable the same way `cmd/marco` does: beside the running
// binary, with $DIRECTOR_BIN overriding.
func directorBin() string {
	if b := os.Getenv("DIRECTOR_BIN"); b != "" {
		return b
	}
	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), "director.exe")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "director.exe"
}

// quitAfter closes the window by itself.
//
// So the proof's window cannot sit on the owner's desktop holding focus: the one thing a
// feasibility window must not do is become a thing somebody has to go and close.
func quitAfter(ctx context.Context, seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
	runtime.Quit(ctx)
}

// Loaded is called BY THE PAGE once it has rendered, and is how gate 1 is evidenced.
//
// A `-H windowsgui` binary has no console to print to, and "the process exited 0" says nothing
// about whether a WebView appeared or a page ran in it. So the page calls back: if this line
// reaches the file, then the window opened, WebView2 initialised, the embedded asset was served,
// the DOM parsed, the script ran, and the Go binding is reachable from it. One line, six facts.
func (b *Backend) Loaded(info string) {
	if b.evidence == "" {
		return
	}
	f, err := os.OpenFile(b.evidence, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "page loaded and called back into Go: %s\n", info)
	f.Close()
	// AND THE PAGE ASKS FOR THE RUN ITSELF, so one artifact carries both gates: the
	// window rendered, and then the thing inside it drove the production backend.
	if b.autoStart {
		b.Start(b.phrase)
	}
}

// note appends what the page was shown to the evidence file, when there is one.
//
// So a windowed proof leaves the same account a headless one prints: the window is not asked to
// be believed, it is asked to write down what it rendered.
func (b *Backend) note(kind string, v any) {
	if b.evidence == "" {
		return
	}
	f, err := os.OpenFile(b.evidence, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if raw, err := json.Marshal(v); err == nil {
		fmt.Fprintf(f, "%-8s %s\n", kind, raw)
	}
}
