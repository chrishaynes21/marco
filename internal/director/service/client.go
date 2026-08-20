package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The thin client.
//
// A client process does one thing: submit intent and render what comes back. It owns
// no Director state, constructs no providers, and cannot cancel anything by dying —
// which is the whole point of moving all of that into the service.

// Client is a connection to the Director service.
type Client struct {
	conn    net.Conn
	token   string
	reader  *bufio.Reader
	encoder *json.Encoder
	seq     int64

	// lastClarification holds the question from the most recent request, if it ended
	// in one. It is the client's only piece of Director state, and it is a question to
	// render — not a decision to make.
	lastClarification *ClarificationPayload
}

// Dial connects to a running service.
func Dial(ep Endpoint, timeout time.Duration) (*Client, error) {
	if err := CheckVersion(ep.ProtocolVersion); err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", ep.Address, timeout)
	if err != nil {
		return nil, fmt.Errorf("service: connecting to the Director at %s: %w", ep.Address, err)
	}
	c := &Client{conn: conn, token: ep.Token}
	if err := c.handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

// handshake presents the token before anything else is sent.
func (c *Client) handshake() error {
	if _, err := c.conn.Write([]byte(c.token + "\n")); err != nil {
		return fmt.Errorf("service: sending the token: %w", err)
	}
	c.reader = bufio.NewReader(c.conn)
	c.encoder = json.NewEncoder(c.conn)

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("service: the Director did not accept the connection: %w", err)
	}
	if strings.TrimSpace(line) != "ok" {
		return fmt.Errorf("service: the Director rejected this client (bad token?)")
	}
	return nil
}

// Close hangs up.
//
// Hanging up does NOT cancel anything. A command in flight keeps running and its
// outcome is retained for whoever asks next — because a dropped connection is a
// client that stopped listening, not a user who changed their mind.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// nextID mints a request id, unique within this client.
func (c *Client) nextID() string {
	n := atomic.AddInt64(&c.seq, 1)
	return fmt.Sprintf("req_%d_%d", os.Getpid(), n)
}

// send writes one request.
func (c *Client) send(t RequestType, payload any) (string, error) {
	id := c.nextID()
	env, err := NewRequest(id, t, payload)
	if err != nil {
		return "", err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := c.encoder.Encode(env); err != nil {
		return "", fmt.Errorf("service: sending %s: %w", t, err)
	}
	return id, nil
}

// receive reads one response, checking correlation and version.
//
// A response whose request id does not match what was asked is DROPPED rather than
// returned. Mismatched correlation is how a client ends up rendering one command's
// result as another's, and silently accepting it would make that invisible.
func (c *Client) receive(wantID string, timeout time.Duration) (ResponseEnvelope, error) {
	deadline := time.Now().Add(timeout)
	for {
		_ = c.conn.SetReadDeadline(deadline)
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return ResponseEnvelope{}, fmt.Errorf("service: reading a response: %w", err)
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		var env ResponseEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			return ResponseEnvelope{}, fmt.Errorf("service: malformed response: %w", err)
		}
		if err := CheckVersion(env.ProtocolVersion); err != nil {
			return ResponseEnvelope{}, err
		}
		if wantID != "" && env.RequestID != "" && env.RequestID != wantID {
			continue // someone else's; not ours to interpret
		}
		return env, nil
	}
}

// roundTrip sends a request and reads one response.
func (c *Client) roundTrip(t RequestType, payload any) (ResponseEnvelope, error) {
	id, err := c.send(t, payload)
	if err != nil {
		return ResponseEnvelope{}, err
	}
	return c.receive(id, 30*time.Second)
}

// Status asks what the service is doing.
func (c *Client) Status() (StatusPayload, error) {
	resp, err := c.roundTrip(RequestStatus, nil)
	if err != nil {
		return StatusPayload{}, err
	}
	var out StatusPayload
	return out, resp.Decode(&out)
}

// History returns recent action graph nodes.
func (c *Client) History(limit int) (HistoryPayloadResponse, error) {
	resp, err := c.roundTrip(RequestHistory, HistoryPayload{Limit: limit})
	if err != nil {
		return HistoryPayloadResponse{}, err
	}
	var out HistoryPayloadResponse
	return out, resp.Decode(&out)
}

// Perception asks what the observation providers and the fusion engine did.
//
// It reads the SERVICE's history rather than observing afresh, which is the whole
// value of it: a diagnostic that took its own snapshot would be reporting on a cycle
// nothing ever planned against, and would attach a second accessibility client to the
// desktop to do it.
func (c *Client) Perception() (diagnostics.Perception, error) {
	resp, err := c.roundTrip(RequestPerception, nil)
	if err != nil {
		return diagnostics.Perception{}, err
	}
	var out diagnostics.Perception
	return out, resp.Decode(&out)
}

// World asks what the Director currently BELIEVES, as entities.
//
// Cheap and read-only: the service copies the world it already holds. Polling this cannot
// start an observation, so a front-end may refresh at whatever rate it likes without
// steering the thing it is describing.
func (c *Client) World(p WorldPayload) (WorldResponse, error) {
	resp, err := c.roundTrip(RequestWorld, p)
	if err != nil {
		return WorldResponse{}, err
	}
	var out WorldResponse
	return out, resp.Decode(&out)
}

// Events returns the perception event log after a cursor.
//
// The caller compares the reply's Epoch and Oldest against what it last saw to tell
// "nothing happened" from "I missed some" from "the service restarted" — see
// EventsResponse. This call answers; it does not interpret.
func (c *Client) Events(p EventsPayload) (EventsResponse, error) {
	resp, err := c.roundTrip(RequestEvents, p)
	if err != nil {
		return EventsResponse{}, err
	}
	var out EventsResponse
	return out, resp.Decode(&out)
}

// LiveAnalysis asks what a passive observation session has learned so far.
//
// Cheap and read-only: the service reads accumulated analysis. Polling starts no session
// and takes no sample.
func (c *Client) LiveAnalysis(p LiveAnalysisPayload) (LiveAnalysisResponse, error) {
	resp, err := c.roundTrip(RequestLiveAnalysis, p)
	if err != nil {
		return LiveAnalysisResponse{}, err
	}
	var out LiveAnalysisResponse
	return out, resp.Decode(&out)
}

// ObservationEvents returns a session's live findings after a cursor.
//
// The reply carries Gap, the generations and the retained range; the caller renders them.
// It does not recompute whether something was missed — the service already decided.
func (c *Client) ObservationEvents(p ObservationEventsPayload) (ObservationEventsResponse, error) {
	resp, err := c.roundTrip(RequestObservationEvents, p)
	if err != nil {
		return ObservationEventsResponse{}, err
	}
	var out ObservationEventsResponse
	return out, resp.Decode(&out)
}

// Playbill asks for the one read-only account a presentation renders.
//
// Cheap and read-only: the service copies state it already holds. Polling it starts no
// observation, takes no sample and forms no interpretation, so a surface may refresh at
// whatever rate it likes without becoming part of what it is describing.
//
// It carries no authority. The reply's pending question, if any, names the id its answer
// routes by and the EXISTING request that carries it — Confirm, Clarified or Observation.
// There is nothing in the reply a caller can act on directly, and no method here that
// would let one.
func (c *Client) Playbill(p PlaybillPayload) (PlaybillResponse, error) {
	resp, err := c.roundTrip(RequestPlaybill, p)
	if err != nil {
		return PlaybillResponse{}, err
	}
	var out PlaybillResponse
	return out, resp.Decode(&out)
}

// Explain asks for the perception picture plus every element's account.
func (c *Client) Explain() (diagnostics.Perception, error) {
	resp, err := c.roundTrip(RequestExplain, nil)
	if err != nil {
		return diagnostics.Perception{}, err
	}
	var out diagnostics.Perception
	return out, resp.Decode(&out)
}

// ReadText asks the service for one OCR pass of the active window.
//
// Diagnostics only. It captures the screen, which is why it is a request a caller has
// to make deliberately rather than something any command does on its behalf.
func (c *Client) ReadText(region *directorapi.Rect) (ocr.Diagnostics, error) {
	resp, err := c.roundTrip(RequestReadText, ReadTextPayload{Region: region})
	if err != nil {
		return ocr.Diagnostics{}, err
	}
	var out ocr.Diagnostics
	return out, resp.Decode(&out)
}

// ReadRegion asks for one visual pass over a region.
func (c *Client) ReadRegion(region *directorapi.Rect) (visualstate.Diagnostics, error) {
	resp, err := c.roundTrip(RequestReadRegion, ReadTextPayload{Region: region})
	if err != nil {
		return visualstate.Diagnostics{}, err
	}
	var out visualstate.Diagnostics
	return out, resp.Decode(&out)
}

// ActiveWait asks what the Director is currently waiting for.
func (c *Client) ActiveWait() (waitengine.Snapshot, error) {
	resp, err := c.roundTrip(RequestWaitStatus, nil)
	if err != nil {
		return waitengine.Snapshot{}, err
	}
	var out waitengine.Snapshot
	return out, resp.Decode(&out)
}

// Cancel stops whatever is running.
func (c *Client) Cancel() (CancelPayload, error) {
	resp, err := c.roundTrip(RequestCancelActive, nil)
	if err != nil {
		return CancelPayload{}, err
	}
	var out CancelPayload
	return out, resp.Decode(&out)
}

// Shutdown stops the service.
func (c *Client) Shutdown() error {
	_, err := c.roundTrip(RequestShutdown, nil)
	return err
}

// Execute runs a phrase, calling onEvent for each response until a terminal one.
//
// The timeout is generous and per-response rather than overall: a ten-iteration
// replay legitimately takes a long time, and the thing worth timing out on is the
// service going silent, not the work taking a while.
func (c *Client) Execute(phrase string, dryRun bool, onEvent func(ResponseEnvelope)) (OutcomePayload, error) {
	id, err := c.send(RequestExecutePhrase, ExecutePayload{Phrase: phrase, DryRun: dryRun})
	if err != nil {
		return OutcomePayload{}, err
	}
	return c.awaitTerminal(id, onEvent)
}

// Confirm answers a pending confirmation.
//
// Sent on ITS OWN connection in practice: the connection that submitted the command is
// blocked reading that command's events, and the command cannot finish until the answer
// arrives. That is why CONFIRM is non-mutating — it stays answerable while a command is
// in flight, which is the entire point of it.
func (c *Client) Confirm(id string, approved bool) (ConfirmResultPayload, error) {
	resp, err := c.roundTrip(RequestConfirm, ConfirmPayload{ID: id, Approved: approved})
	if err != nil {
		return ConfirmResultPayload{}, err
	}
	var out ConfirmResultPayload
	return out, resp.Decode(&out)
}

// awaitTerminal reads events until a terminal one, handing each to onEvent.
//
// A CLARIFICATION_REQUIRED is terminal for the REQUEST but not for the exchange: the
// command stays open awaiting an answer. It is recorded on the client rather than
// returned as an error, because the caller's next move is to submit an answer, not to
// report a failure.
func (c *Client) awaitTerminal(id string, onEvent func(ResponseEnvelope)) (OutcomePayload, error) {
	c.lastClarification = nil
	for {
		resp, err := c.receive(id, 10*time.Minute)
		if err != nil {
			return OutcomePayload{}, err
		}
		if onEvent != nil {
			onEvent(resp)
		}
		if !resp.Type.Terminal() {
			continue
		}

		switch resp.Type {
		case ResponseCompleted, ResponseUnverified, ResponseFailed, ResponseCancelled:
			var out OutcomePayload
			return out, resp.Decode(&out)
		case ResponseClarificationRequired:
			var q ClarificationPayload
			if err := resp.Decode(&q); err != nil {
				return OutcomePayload{}, err
			}
			c.lastClarification = &q
			return OutcomePayload{}, nil
		case ResponseBusy:
			var busy BusyPayload
			_ = resp.Decode(&busy)
			return OutcomePayload{}, fmt.Errorf("%s", busy.Message)
		case ResponseError:
			var e ErrorPayload
			_ = resp.Decode(&e)
			return OutcomePayload{}, fmt.Errorf("%s: %s", e.Code, e.Message)
		default:
			return OutcomePayload{}, fmt.Errorf("service: unexpected response %s", resp.Type)
		}
	}
}

// ── discovery and auto-start ──────────────────────────────────────────────────

// ConnectOptions controls how a client finds or starts the service.
type ConnectOptions struct {
	// Dir is where the endpoint file lives.
	Dir string
	// ServiceBin is the executable to start, and ServiceArgs its arguments.
	ServiceBin  string
	ServiceArgs []string
	// AutoStart allows starting the service when none is running.
	AutoStart bool
	// StartTimeout bounds how long to wait for a newly started service.
	StartTimeout time.Duration
	// DialTimeout bounds each connection attempt.
	DialTimeout time.Duration
}

// Connect finds the running service, starting one if permitted.
//
// The sequence handles the case of two clients arriving at once, which is entirely
// ordinary — a user typing in one shell while the overlay dispatches a phrase in
// another:
//
//  1. Try the published endpoint. If something answers, use it.
//  2. Otherwise take a startup lock, so only one client starts a service.
//  3. Re-check the endpoint after taking the lock: another client may have started
//     one while this one waited, and starting a second would leave two services
//     fighting over the same desktop.
//  4. Start, wait for readiness, connect.
//
// A stale endpoint or lock file never blocks startup: both are validated by use, not
// by existence.
func Connect(opts ConnectOptions) (*Client, error) {
	dial := opts.DialTimeout
	if dial <= 0 {
		dial = 2 * time.Second
	}

	if ep, ok := ReadEndpoint(opts.Dir); ok {
		if c, err := Dial(ep, dial); err == nil {
			return c, nil
		}
		// Present but not answering: the service died without cleaning up. The file
		// is evidence, not authority.
	}
	if !opts.AutoStart {
		return nil, fmt.Errorf("the Director service is not running (start it with: director serve)")
	}

	lock, err := acquireStartupLock(opts.Dir, 30*time.Second)
	if err != nil {
		// Someone else is starting it. Wait for THEIR service rather than starting a
		// competing one. No startup log: this process did not spawn it, and inventing
		// one would attribute another client's child to this attempt.
		return waitForService(opts, dial, nil)
	}
	defer lock.release()

	// Re-check under the lock.
	if ep, ok := ReadEndpoint(opts.Dir); ok {
		if c, err := Dial(ep, dial); err == nil {
			return c, nil
		}
		RemoveEndpoint(opts.Dir)
	}

	startup, err := startService(opts)
	if err != nil {
		return nil, err
	}
	return waitForService(opts, dial, startup)
}

// startService spawns the service process, detached from this one.
//
// # The child's stderr is EVIDENCE, and it used to be thrown away
//
// `cmd.Stderr = nil` sent the Director's first words to the void. So a Director that refused to
// start — `director: accessibility bridge not found at ...`, one line, naming the exact missing
// file — was reported to the user as "the Director service did not become ready within 20s",
// followed by an instruction to run `director serve` in another terminal to find out why. The
// answer had already been written, on this machine, a second earlier.
//
// So it is captured instead, and travels to the error waitForService returns. Bounded, because a
// service that logs to stderr for an hour must not grow this client's memory, and DRAINED past
// the bound rather than left to fill, because a child that blocks writing to a full pipe would be
// a service the client hung by watching it.
func startService(opts ConnectOptions) (*startupLog, error) {
	if opts.ServiceBin == "" {
		return nil, fmt.Errorf("service: no Director executable configured to start")
	}
	cmd := exec.Command(opts.ServiceBin, opts.ServiceArgs...)
	// Detached: the service must outlive the client that happened to start it,
	// which is the entire reason it exists.
	cmd.Stdin = nil
	cmd.Stdout = nil

	// A pipe rather than cmd.StderrPipe(): that one is closed by cmd.Wait(), and the Wait
	// below runs immediately in its own goroutine so the child is not left a zombie. Owning
	// the ends here keeps the two independent.
	startup := &startupLog{}
	r, w, perr := os.Pipe()
	if perr == nil {
		cmd.Stderr = w
	}

	configureDetached(cmd)
	if err := cmd.Start(); err != nil {
		if r != nil {
			_, _ = r.Close(), w.Close()
		}
		return nil, fmt.Errorf("service: starting the Director (%s): %w", opts.ServiceBin, err)
	}
	if r != nil {
		// The parent's write end must go, or the read below never sees EOF when the
		// child exits and the drain goroutine outlives the process it was watching.
		_ = w.Close()
		go startup.drain(r)
	}
	// Release the child so it is not left as a zombie when this process exits.
	go func() { _ = cmd.Wait() }()
	return startup, nil
}

// startupLog is the beginning of a spawned Director's stderr, bounded.
//
// Only the beginning. A refusal to start is said in the first breath, and keeping the tail
// instead would mean holding a ring buffer of a healthy service's ordinary logging forever to
// preserve a line that was printed before any of it.
type startupLog struct {
	mu        sync.Mutex
	buf       []byte
	truncated bool
}

// startupLogLimit is how much of the child's first words are kept. Generous for a refusal,
// nowhere near enough to matter if a service simply logs.
const startupLogLimit = 8 << 10

// drain reads until the child closes its stderr, keeping the first startupLogLimit bytes.
//
// It keeps READING after the bound is reached and discards what it reads. Stopping would leave
// the pipe to fill, and a full pipe blocks the writer — turning a diagnostic into a service that
// wedges the moment it becomes chatty.
func (l *startupLog) drain(r *os.File) {
	defer r.Close()
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			l.add(chunk[:n])
		}
		if err != nil {
			return
		}
	}
}

func (l *startupLog) add(b []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	room := startupLogLimit - len(l.buf)
	if room <= 0 {
		l.truncated = true
		return
	}
	if len(b) > room {
		b, l.truncated = b[:room], true
	}
	l.buf = append(l.buf, b...)
}

// text is what the Director said, as one line, or empty when it said nothing.
func (l *startupLog) text() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var said []string
	for _, line := range strings.Split(string(l.buf), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			said = append(said, s)
		}
	}
	if len(said) == 0 {
		return ""
	}
	out := strings.Join(said, "; ")
	if l.truncated {
		out += " …"
	}
	return out
}

// waitForService polls for a reachable endpoint, bounded.
//
// startup may be nil — this client did not spawn the service — and is the child's own account of
// why it is not answering. Preferred over the generic sentence whenever there is one, because
// "the Director said: accessibility bridge not found at C:\...\uia.exe" is a thing a person can
// act on and "it did not become ready" is not.
func waitForService(opts ConnectOptions, dial time.Duration, startup *startupLog) (*Client, error) {
	timeout := opts.StartTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ep, ok := ReadEndpoint(opts.Dir); ok {
			if c, err := Dial(ep, dial); err == nil {
				return c, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if said := startup.text(); said != "" {
		return nil, fmt.Errorf(
			"the Director service did not start — it said: %s", said)
	}
	return nil, fmt.Errorf(
		"the Director service did not become ready within %s — try running `director serve` "+
			"in another terminal to see why", timeout)
}

// Clarify answers a pending question.
func (c *Client) Clarify(commandID CommandID, response string, onEvent func(ResponseEnvelope)) (OutcomePayload, error) {
	id, err := c.send(RequestClarify, ClarifyPayload{CommandID: commandID, Response: response})
	if err != nil {
		return OutcomePayload{}, err
	}
	return c.awaitTerminal(id, onEvent)
}

// Submit routes a recognised phrase and sends it.
//
// This is the whole of what a front-end does with speech: ask the service what it is
// currently doing, decide which REQUEST the phrase is, and send it. The decision is
// routing — control phrase, answer, or new request — never interpretation. Which
// candidate "the second one" refers to is settled by the Director.
func (c *Client) Submit(phrase string, onEvent func(ResponseEnvelope)) (Interaction, error) {
	status, err := c.Status()
	if err != nil {
		return Interaction{}, err
	}
	route := RoutePhrase(phrase, status)

	switch route.Kind {
	case RouteCancel:
		res, err := c.Cancel()
		if err != nil {
			return Interaction{}, err
		}
		return Interaction{Route: route, Cancel: &res}, nil

	case RouteClarify:
		out, err := c.Clarify(route.CommandID, phrase, onEvent)
		if err != nil {
			return Interaction{Route: route}, err
		}
		return c.interaction(route, out), nil

	default:
		out, err := c.Execute(phrase, false, onEvent)
		if err != nil {
			return Interaction{Route: route}, err
		}
		return c.interaction(route, out), nil
	}
}

// interaction assembles the result: a question if one was asked, an outcome otherwise.
func (c *Client) interaction(route PhraseRoute, out OutcomePayload) Interaction {
	if c.lastClarification != nil {
		return Interaction{Route: route, Clarification: c.lastClarification}
	}
	return Interaction{Route: route, Outcome: &out}
}

// Interaction is the result of submitting one phrase.
type Interaction struct {
	Route PhraseRoute
	// Outcome is set when the phrase ran a command.
	Outcome *OutcomePayload
	// Clarification is set when the Director asked a question instead.
	Clarification *ClarificationPayload
	// Cancel is set when the phrase was a control phrase.
	Cancel *CancelPayload
}

// Status renders the interaction as the one line a front-end should show.
func (i Interaction) Status() string {
	switch {
	case i.Cancel != nil:
		return i.Cancel.Message
	case i.Clarification != nil:
		return i.Clarification.Question
	case i.Outcome != nil:
		return string(i.Outcome.State) + ": " + i.Outcome.Message
	}
	return "no response"
}

// Edits asks for the recent editing outcomes.
func (c *Client) Edits() ([]edit.Outcome, error) {
	resp, err := c.roundTrip(RequestEditHistory, nil)
	if err != nil {
		return nil, err
	}
	var out []edit.Outcome
	return out, resp.Decode(&out)
}

// SemanticActions asks for the recent semantic action outcomes.
func (c *Client) SemanticActions() ([]uiact.Outcome, error) {
	resp, err := c.roundTrip(RequestSemanticActions, nil)
	if err != nil {
		return nil, err
	}
	var out []uiact.Outcome
	return out, resp.Decode(&out)
}

// LastSemanticAction is the most recent one, nil when none has run.
//
// A convenience over SemanticActions rather than a second request: "explain the last
// one" is the question people actually ask, and making the client pick the last entry
// would put the same off-by-one in every caller.
func (c *Client) LastSemanticAction() (*uiact.Outcome, error) {
	all, err := c.SemanticActions()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	last := all[len(all)-1]
	return &last, nil
}

// Lowerings asks for the recent lowered operations and their Marco.
func (c *Client) Lowerings() ([]marcoexec.Result, error) {
	resp, err := c.roundTrip(RequestLowerings, nil)
	if err != nil {
		return nil, err
	}
	var out []marcoexec.Result
	return out, resp.Decode(&out)
}

// RunOperation executes one lowered operation.
func (c *Client) RunOperation(op marcoexec.Operation) (marcoexec.Result, error) {
	resp, err := c.roundTrip(RequestRunOperation, RunOperationPayload{Operation: op})
	if err != nil {
		return marcoexec.Result{}, err
	}
	var out marcoexec.Result
	return out, resp.Decode(&out)
}

// Trace asks for one command's phase trace.
func (c *Client) Trace(id string) (*trace.Trace, error) {
	resp, err := c.roundTrip(RequestTrace, TracePayload{CommandID: id})
	if err != nil {
		return nil, err
	}
	var out *trace.Trace
	return out, resp.Decode(&out)
}

// ExplainValue asks about one program-local value.
//
// Answerable only while the owning program runs or is paused. There is no history
// fallback, deliberately: a finished value is gone, and a client that could ask
// "what was it?" afterwards would be asking the service to remember something it
// promised not to.
func (c *Client) ExplainValue(name string) (ValueExplanationPayload, error) {
	resp, err := c.roundTrip(RequestExplainValue, ExplainValuePayload{Name: name})
	if err != nil {
		return ValueExplanationPayload{}, err
	}
	var out ValueExplanationPayload
	return out, resp.Decode(&out)
}

// Collections returns the running or paused program's collections.
//
// Answerable only while the owning program lives. There is no history fallback: a
// completed collection is gone, and asking the service to reconstruct one would be
// asking it to remember something it promised not to.
func (c *Client) Collections() (collections.Snapshot, error) {
	resp, err := c.roundTrip(RequestCollections, nil)
	if err != nil {
		return collections.Snapshot{}, err
	}
	var out collections.Snapshot
	return out, resp.Decode(&out)
}

// Demonstration performs one demonstration action: start, stop, list, extract, approve.
//
// One method for every action, mirroring the one request type, so a client that gains an
// action gains a constant rather than a method — and so the error a service returns for an
// unknown action is the same error whichever client asked.
func (c *Client) Demonstration(p DemonstrationPayload) (DemonstrationResponse, error) {
	resp, err := c.roundTrip(RequestDemonstration, p)
	if err != nil {
		return DemonstrationResponse{}, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return DemonstrationResponse{}, fmt.Errorf("%s", e.Message)
	}
	var out DemonstrationResponse
	return out, resp.Decode(&out)
}

// Game asks what capability pack serves the foreground, what the packs contribute, or
// what the Director can see of an inventory.
func (c *Client) Game(p GamePayload) (GameResponse, error) {
	resp, err := c.roundTrip(RequestGame, p)
	if err != nil {
		return GameResponse{}, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return GameResponse{}, fmt.Errorf("%s", e.Message)
	}
	var out GameResponse
	return out, resp.Decode(&out)
}

// Vision runs one detection pass, or reads the frame log.
func (c *Client) Vision(p VisionPayload) (VisionResponse, error) {
	resp, err := c.roundTrip(RequestVision, p)
	if err != nil {
		return VisionResponse{}, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return VisionResponse{}, fmt.Errorf("%s", e.Message)
	}
	var out VisionResponse
	return out, resp.Decode(&out)
}

// LiveWindows lists the current live windows with ephemeral ids.
func (c *Client) LiveWindows(application string) ([]windowref.Listing, error) {
	resp, err := c.roundTrip(RequestWindows, WindowsPayload{Application: application})
	if err != nil {
		return nil, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return nil, fmt.Errorf("%s", e.Message)
	}
	var out WindowsResponse
	if err := resp.Decode(&out); err != nil {
		return nil, err
	}
	return out.Windows, nil
}

// Observe starts a passive observation session and returns immediately.
func (c *Client) Observe(p ObservePayload) (ObserveStarted, error) {
	resp, err := c.roundTrip(RequestObserve, p)
	if err != nil {
		return ObserveStarted{}, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return ObserveStarted{}, fmt.Errorf("%s", e.Message)
	}
	var out ObserveStarted
	return out, resp.Decode(&out)
}

// Observation reads, lists or cancels observation sessions.
//
// Returns raw JSON because the shape differs by query and the CLI renders it; the service
// owns the schema and the client does not need to duplicate it.
func (c *Client) Observation(p ObserveQuery) (json.RawMessage, error) {
	resp, err := c.roundTrip(RequestObservation, p)
	if err != nil {
		return nil, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return nil, fmt.Errorf("%s", e.Message)
	}
	return resp.Payload, nil
}

// Learned asks what Marco has learned well enough to write down.
//
// Returns raw JSON for the same reason Observation does: the service owns the schema and the CLI
// renders it. Nothing about this request can cause anything to happen.
func (c *Client) Learned(p LearnedQuery) (json.RawMessage, error) {
	resp, err := c.roundTrip(RequestLearned, p)
	if err != nil {
		return nil, err
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		return nil, fmt.Errorf("%s", e.Message)
	}
	return resp.Payload, nil
}
