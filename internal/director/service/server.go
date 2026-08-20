package service

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Runtime is everything the service needs from the Director.
//
// An interface, so the server can be tested with a fake and so the platform wiring
// stays where it belongs. The service knows about commands, cancellation and
// connections; it knows nothing about accessibility bridges or input hosts.
type Runtime interface {
	// Handle runs one phrase to completion, reporting progress as it goes.
	Handle(ctx context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome
	// HandleClarified re-runs a phrase with a clarification answer applied. The
	// answer narrows the query; the request is resolved again from scratch.
	HandleClarified(ctx context.Context, phrase string, refinement intent.Refinement,
		progress func(ProgressPayload)) execute.Outcome
	// ActiveValues is a SAFE snapshot of the running or paused program.s captured
	// values. Metadata only — the snapshot type has no field that can hold content.
	//
	// On the Runtime rather than reachable through the graph or the trace, because it
	// describes something ALIVE: when the program ends the answer becomes empty, and
	// no amount of history can or should reconstruct it.
	ActiveValues() values.EnvironmentSnapshot
	// ActiveCollections is a SAFE snapshot of the running or paused program.s
	// collections. Queries, counts and digests — never a member list.
	ActiveCollections() collections.Snapshot
	// AbandonProgram discards a paused program and its values. A cancelled program is
	// terminal, and its data flow must end with it.
	AbandonProgram(reason string)
	// Graph is the durable action history.
	Graph() actiongraph.Graph
	// Providers reports accessibility lifecycle.
	Providers() []ProviderStatus
	// Perception reports what the observation providers and the fusion engine did.
	// Diagnostics only: nothing plans from it, and the service does not interpret it.
	Perception() diagnostics.Perception
	// World is the current BELIEF, as entities. It copies the world the Director
	// already holds: it observes nothing, starts no cycle, and changes no state.
	World(p WorldPayload) WorldResponse
	// LiveAnalysis is what a passive observation session has learned so far. Reads
	// accumulated analysis; starts nothing and samples nothing.
	LiveAnalysis(p LiveAnalysisPayload) LiveAnalysisResponse
	// Playbill is the perception-and-learning half of the ONE account a presentation
	// renders: what the Director is looking at, what evidence is reaching it, what it
	// makes of that, and where the Learn lifecycle has got to.
	//
	// The command half is the SERVER's, and is folded in by playbillFor — see
	// playbill.go for why the two halves are not merged here.
	//
	// Read-only in the same sense as World and LiveAnalysis: it copies state that
	// already exists. It must not observe, sample, capture, infer or authorise, and a
	// front-end polling it must not change what it is describing.
	Playbill(p PlaybillPayload) playbill.View
	// ObservationEvents is a session's findings after a cursor.
	ObservationEvents(p ObservationEventsPayload) ObservationEventsResponse
	// Events is the perception event log from a cursor.
	Events(p EventsPayload) EventsResponse
	// Explanation is Perception plus the account of every element. Separate because it
	// is expensive and rarely wanted.
	Explanation() diagnostics.Perception
	// ReadText performs one OCR pass for diagnostics. It observes and reads; it never
	// executes, and cannot — it returns evidence, and evidence has no path to an action.
	ReadText(ctx context.Context, region *directorapi.Rect) ocr.Diagnostics
	// OCRUnavailable is why OCR cannot run, empty when it can.
	OCRUnavailable() string
	// ActiveWait reports the wait currently running, if any. A wait can run for tens of
	// seconds during which the Director looks busy and says nothing; this is what makes
	// a considered pause distinguishable from a hang.
	ActiveWait() waitengine.Snapshot
	// ReadRegion performs one visual pass for diagnostics. Like ReadText it observes
	// and looks; it never executes.
	ReadRegion(ctx context.Context, region *directorapi.Rect) visualstate.Diagnostics
	// RunOperation executes one operation through the ordinary path: validate,
	// confirm the context, lower, compile, run. Not a bypass — the same executor.
	RunOperation(ctx context.Context, op marcoexec.Operation) marcoexec.Result
	// TraceFor returns one command.s phase trace, nil when there is none.
	TraceFor(id string) *trace.Trace
	// Lowerings reports the recent lowered operations and the Marco they became.
	// Diagnostics and export material; secret operations are stored redacted.
	Lowerings() []marcoexec.Result
	// Edits reports the recent editing outcomes. Diagnostics only: nothing plans from
	// it, and it exists so "why did it type instead of setting the value?" is answered
	// from a record rather than a guess.
	Edits() []edit.Outcome
	// SemanticActions returns the recent semantic action outcomes — the verb, the
	// chosen implementation, and the stronger ones that were unavailable. Diagnostic
	// only, and safe to serve while a command runs: see the control-plane lock rule.
	SemanticActions() []uiact.Outcome
	// AttachedAt is when the accessibility client came up.
	AttachedAt() time.Time
	// Windows is what the Director could see at its last observation. Diagnostics and
	// identification only: it starts no observation and nothing plans from it.
	Windows() []directorapi.Window
	// Confirmations is the broker the runtime installed as its execute.Confirmer, so
	// the server can publish a pending question to the client watching the command and
	// route the answer back.
	//
	// Nil is legal and means this runtime cannot ask: every action needing agreement is
	// then BLOCKED, which is the safe failure and the one the daemon had before this
	// was wired. It is never a default yes.
	Confirmations() *ConfirmationBroker

	// ── demonstrations ────────────────────────────────────────────────────────
	//
	// Recording what a user DEMONSTRATES, and extracting a reusable procedure from it.
	// Every method here is control plane: none touches the desktop, so all of them stay
	// answerable while the command they are recording is in flight — which is the case
	// they exist for.

	// StartDemonstration opens a recording session; StopDemonstration closes and stores
	// it; AbandonDemonstration discards it.
	StartDemonstration() (*demo.Demonstration, error)
	StopDemonstration() (*demo.Demonstration, error)
	AbandonDemonstration(reason string) (*demo.Demonstration, error)
	// ActiveDemonstration is the open session, nil when none is.
	ActiveDemonstration() *demo.Demonstration
	// Demonstrations lists what has been recorded; Demonstration reads one back.
	Demonstrations() ([]*demo.Demonstration, error)
	Demonstration(id demo.ID) (*demo.Demonstration, error)
	// ExtractProcedure proposes a procedure. It installs nothing.
	ExtractProcedure(id demo.ID) (demo.Extraction, error)
	// ApproveProcedure installs an extracted procedure into the live registry.
	ApproveProcedure(id demo.ID, by string) (*demo.Learned, error)
	// ForgetProcedure removes a learned procedure from the store.
	ForgetProcedure(name string) error
	// LearnedProcedures is what has been learned.
	LearnedProcedures() []*demo.Learned

	// ── capability packs ──────────────────────────────────────────────────────
	//
	// What application a pack serves, what the packs contribute, and what the Director
	// can see of an inventory. Control plane, like everything above: detection is
	// recomputed from the world the Director already observed, and nothing here
	// observes, plans or acts.

	// DetectedGame is what the Director believes it is looking at.
	DetectedGame() game.Active
	// GameCapabilities is what every registered pack contributes.
	GameCapabilities() game.Report
	// GameInventory is what the Director can see of what the player holds.
	GameInventory(container string) game.InventoryReport

	// ── vision ────────────────────────────────────────────────────────────────

	// ReadVision performs one detection pass for diagnostics. It observes and looks;
	// it never executes, and cannot — it returns evidence, and evidence has no path to
	// an action.
	ReadVision(ctx context.Context, region *directorapi.Rect, target windowref.Selector) vision.Diagnostics
	// LastVision is the most recent pass, without performing one.
	LastVision() vision.Diagnostics
	// Frames is the recent frame log, newest first. It holds no pictures.
	Frames() []vision.FrameRecord
	// LiveWindows lists the current live windows with ephemeral ids.
	LiveWindows(ctx context.Context, application string) []windowref.Listing
	// StartObservation begins a bounded passive session and returns at once.
	StartObservation(ObservePayload) (ObserveStarted, error)
	// Observation reads, lists or cancels sessions. The result is a safe view.
	Observation(ObserveQuery) (any, error)
	// LearnedPlay writes down what Marco has learned, as ordinary Marco. A read: it saves
	// nothing, registers nothing and runs nothing.
	LearnedPlay(LearnedQuery) (LearnedView, error)
	// VisionUnavailable is why vision cannot run, empty when it can.
	VisionUnavailable() string
}

// Server is the long-lived Director.
type Server struct {
	runtime  Runtime
	registry *Registry
	convo    *Conversation
	pending  pendingClarification
	dir      string

	listener  net.Listener
	token     string
	startedAt time.Time

	ctx      context.Context
	cancel   context.CancelFunc
	shutdown sync.Once
	wg       sync.WaitGroup
}

// Config configures a server.
type Config struct {
	// Dir is where the endpoint file and action graph live.
	Dir string
	// Runtime is the Director this service exposes.
	Runtime Runtime
}

// NewServer builds a server. It does not listen yet.
func NewServer(cfg Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		runtime:  cfg.Runtime,
		registry: NewRegistry(),
		convo:    NewConversation(),
		dir:      cfg.Dir,
		ctx:      ctx,
		cancel:   cancel,
	}
	// A Director with work of its own to publish gets the registry, once, here.
	//
	// Nearly everything that drives the desktop arrives as a request this server routes, and
	// those enter the registry where they are routed. One thing does not: a live rehearsal
	// reached from inside a Learn episode, several layers below any handler, in answer to
	// "want me to try it once?". It types for real, and it was invisible to `director status`,
	// unrefusable by a concurrent request and unreachable by CANCEL_ACTIVE.
	//
	// Deleting this must fail TestTheServerHandsItsRuntimeTheCommandRegistry.
	if reg, ok := cfg.Runtime.(CommandRegistrar); ok {
		reg.UseCommands(ctx, s.registry)
	}
	return s
}

// Listen binds the service and publishes its endpoint.
//
// 127.0.0.1 explicitly, and port 0 so the operating system picks a free one — there
// is no configuration that could bind this to a routable interface, which is the
// point. The port and token are then published in the endpoint file, which is the
// only way a client learns either.
func (s *Server) Listen() (Endpoint, error) {
	token, err := NewToken()
	if err != nil {
		return Endpoint{}, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Endpoint{}, fmt.Errorf("service: listening on loopback: %w", err)
	}

	s.listener = ln
	s.token = token
	s.startedAt = time.Now()

	ep := Endpoint{
		ProtocolVersion: ProtocolVersion,
		Address:         ln.Addr().String(),
		Token:           token,
		PID:             os.Getpid(),
		StartedAt:       s.startedAt,
	}
	if err := WriteEndpoint(s.dir, ep); err != nil {
		_ = ln.Close()
		return Endpoint{}, err
	}
	return ep, nil
}

// Serve accepts connections until the server is shut down.
func (s *Server) Serve() error {
	defer RemoveEndpoint(s.dir)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				s.wg.Wait()
				return nil
			default:
			}
			// A single bad accept is not a reason to take the service down.
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			s.wg.Wait()
			return err
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

// Shutdown stops the service.
func (s *Server) Shutdown() {
	s.shutdown.Do(func() {
		s.cancel()
		if s.listener != nil {
			_ = s.listener.Close()
		}
		RemoveEndpoint(s.dir)
	})
}

// Done is closed when the service is shutting down.
func (s *Server) Done() <-chan struct{} { return s.ctx.Done() }

// handleConn serves one client connection.
//
// The connection is authenticated BEFORE any request is read. A request that arrives
// before the token has been checked is never parsed, let alone executed.
func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := json.NewEncoder(conn)
	var writeMu sync.Mutex

	send := func(env ResponseEnvelope) {
		writeMu.Lock()
		defer writeMu.Unlock()
		// A write failure means the client has gone. That is not an error and not a
		// cancellation — the work continues, and the outcome is retained for
		// whoever asks next.
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_ = writer.Encode(env)
	}

	// Handshake: one line, the token, before anything else. A short deadline so a
	// connection that never authenticates cannot hold a slot open.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	presented := strings.TrimSpace(line)
	// Constant-time, so the comparison cannot be used to learn the token a byte at
	// a time. Cheap, and the alternative is a subtle mistake nobody would notice.
	if subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
		send(NewResponse("", ResponseError, ErrorPayload{
			Code: "unauthorized", Message: "invalid or missing token",
		}))
		return
	}
	if _, err := conn.Write([]byte("ok\n")); err != nil {
		return
	}

	for {
		// No read deadline while idle: a client may hold a connection open across a
		// long command, and timing it out would look like the service dying.
		_ = conn.SetReadDeadline(time.Time{})
		line, err := reader.ReadString('\n')
		if err != nil {
			return // client hung up
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req RequestEnvelope
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			send(NewResponse("", ResponseError, ErrorPayload{
				Code: "malformed", Message: err.Error(),
			}))
			continue
		}
		if err := CheckVersion(req.ProtocolVersion); err != nil {
			send(NewResponse(req.RequestID, ResponseError, ErrorPayload{
				Code: "protocol_version", Message: err.Error(),
			}))
			continue
		}
		if s.dispatch(req, send) {
			return
		}
	}
}

// dispatch handles one request. It returns true when the connection should close.
func (s *Server) dispatch(req RequestEnvelope, send func(ResponseEnvelope)) bool {
	switch req.Type {
	case RequestPing:
		send(NewResponse(req.RequestID, ResponsePong, nil))

	case RequestCollections:
		send(NewResponse(req.RequestID, ResponseStatus, s.runtime.ActiveCollections()))

	case RequestExplainValue:
		var p ExplainValuePayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponseStatus, s.explainValue(p.Name)))

	case RequestStatus:
		send(NewResponse(req.RequestID, ResponseStatus, s.status()))

	case RequestHistory:
		var p HistoryPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponseStatus, s.history(p.Limit)))

	case RequestPerception:
		// Passed straight through. The service does not summarise, threshold or
		// interpret perception diagnostics — doing so would put a second opinion about
		// what fusion did between the caller and the fusion engine.
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.Perception()))

	case RequestWorld:
		// Passed straight through, like PERCEPTION. The service transports belief and
		// has no opinion about it — it does not filter, rank or re-classify, because a
		// second opinion here would be a second world model.
		var p WorldPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.World(p)))

	case RequestEvents:
		var p EventsPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.Events(p)))

	case RequestExplain:
		// Passed straight through, like PERCEPTION. The service transports explanations
		// and has no opinion about them: it does not summarise, threshold or act on one,
		// and nothing it does changes because an explanation exists.
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.Explanation()))

	case RequestReadText:
		var p ReadTextPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.ReadText(s.ctx, p.Region)))

	case RequestReadRegion:
		var p ReadTextPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.ReadRegion(s.ctx, p.Region)))

	case RequestWaitStatus:
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.ActiveWait()))

	case RequestRunOperation:
		var p RunOperationPayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError, ErrorPayload{Code: "malformed", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.RunOperation(s.ctx, p.Operation)))

	case RequestTrace:
		var p TracePayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.TraceFor(p.CommandID)))

	case RequestLowerings:
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.Lowerings()))

	case RequestObserve:
		var p ObservePayload
		_ = req.Decode(&p)
		started, err := s.runtime.StartObservation(p)
		if err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "observe", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponsePerception, started))

	case RequestObservation:
		var p ObserveQuery
		_ = req.Decode(&p)
		// PERFORM IS THE ONE FIELD HERE THAT ACTS, so it is the one that gets a command.
		//
		// Everything else on ObserveQuery is a read and rightly goes straight through. A
		// performance drives real input, and until it entered the registry it was invisible
		// to `director status`, unrefusable by a concurrent request and unreachable by
		// CANCEL_ACTIVE — `director stop` answered "nothing is running" while a play was
		// typing. See perform.go.
		//
		// Deleting this branch must fail TestStoppingAPerformanceReportsItAsCancelled.
		if p.Perform != nil {
			s.performGoal(req.RequestID, *p.Perform, send)
			break
		}
		out, err := s.runtime.Observation(p)
		if err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "observation", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponsePerception, out))

	case RequestLearned:
		// THE lowering call site. A read that produces text: nothing is saved, nothing is
		// registered, and nothing on this path can reach a computer.
		var p LearnedQuery
		_ = req.Decode(&p)
		out, err := s.runtime.LearnedPlay(p)
		if err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "learned", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponsePerception, out))

	case RequestLiveAnalysis:
		// Forwarded unmodified, like PERCEPTION. The service does not summarise or
		// interpret findings — the observer already did, and a second opinion between
		// them would be a second analyser.
		var p LiveAnalysisPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.LiveAnalysis(p)))

	case RequestObservationEvents:
		var p ObservationEventsPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.ObservationEvents(p)))

	case RequestPlaybill:
		// THE visibility path. Every presentation — the overlay's Watch and Diagnostics
		// modes, the consumer headline, `marco director watch` — reads this and nothing
		// else, which is what stops two surfaces from disagreeing about what Marco is
		// doing. Read-only, and answerable while a command is in flight: a person
		// watching a long command is exactly who this is for.
		var p PlaybillPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception, s.playbillFor(p)))

	case RequestWindows:
		var p WindowsPayload
		_ = req.Decode(&p)
		send(NewResponse(req.RequestID, ResponsePerception,
			WindowsResponse{Windows: s.runtime.LiveWindows(s.ctx, p.Application)}))

	case RequestVision:
		var p VisionPayload
		_ = req.Decode(&p)
		out := VisionResponse{}
		switch {
		case p.Frames:
			out.Frames = s.runtime.Frames()
		case p.Last:
			out.Diagnostics = s.runtime.LastVision()
		default:
			out.Diagnostics = s.runtime.ReadVision(s.ctx, p.Region, p.Target)
		}
		send(NewResponse(req.RequestID, ResponsePerception, out))

	case RequestGame:
		var p GamePayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "malformed", Message: err.Error()}))
			break
		}
		out, err := s.gameDiagnostics(p)
		if err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "game", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponseStatus, out))

	case RequestDemonstration:
		var p DemonstrationPayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "malformed", Message: err.Error()}))
			break
		}
		out, err := s.demonstration(p)
		if err != nil {
			send(NewResponse(req.RequestID, ResponseError,
				ErrorPayload{Code: "demonstration", Message: err.Error()}))
			break
		}
		send(NewResponse(req.RequestID, ResponseStatus, out))

	case RequestEditHistory:
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.Edits()))

	case RequestSemanticActions:
		send(NewResponse(req.RequestID, ResponsePerception, s.runtime.SemanticActions()))

	case RequestShowLast:
		send(NewResponse(req.RequestID, ResponseStatus, s.history(1)))

	case RequestCancelActive:
		send(NewResponse(req.RequestID, ResponseStatus, s.cancelActive()))

	case RequestExecutePhrase:
		var p ExecutePayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError, ErrorPayload{
				Code: "malformed", Message: err.Error(),
			}))
			return false
		}
		s.execute(req.RequestID, p, send)

	case RequestClarify:
		var p ClarifyPayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError, ErrorPayload{
				Code: "malformed", Message: err.Error(),
			}))
			return false
		}
		s.clarify(req.RequestID, p, send)

	case RequestConfirm:
		var p ConfirmPayload
		if err := req.Decode(&p); err != nil {
			send(NewResponse(req.RequestID, ResponseError, ErrorPayload{
				Code: "malformed", Message: err.Error(),
			}))
			break
		}
		send(NewResponse(req.RequestID, ResponseStatus, s.confirm(p)))

	case RequestShutdown:
		send(NewResponse(req.RequestID, ResponseStatus, StatusPayload{
			Running: false, PID: os.Getpid(),
		}))
		s.Shutdown()
		return true

	default:
		send(NewResponse(req.RequestID, ResponseError, ErrorPayload{
			Code: "unknown_request", Message: fmt.Sprintf("unknown request type %q", req.Type),
		}))
	}
	return false
}

// confirm routes a person's answer back to the command waiting on it.
//
// It performs nothing and decides nothing: it unblocks a command that has already been
// told what it is asking about. A malformed or stale answer is REFUSED rather than
// applied to whatever is open — an answer written for one question must not agree to a
// different one.
func (s *Server) confirm(p ConfirmPayload) ConfirmResultPayload {
	broker := s.runtime.Confirmations()
	if broker == nil {
		return ConfirmResultPayload{
			Message: "this Director has no way to ask for confirmations, so there is " +
				"nothing to answer",
		}
	}
	if err := broker.Answer(p.ID, p.Approved); err != nil {
		return ConfirmResultPayload{Message: err.Error()}
	}
	word := "declined"
	if p.Approved {
		word = "agreed"
	}
	return ConfirmResultPayload{Accepted: true, Message: word}
}

// execute runs one phrase, streaming progress.
func (s *Server) execute(requestID string, p ExecutePayload, send func(ResponseEnvelope)) {
	phrase := strings.TrimSpace(p.Phrase)
	if phrase == "" {
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "empty_phrase", Message: "nothing was asked",
		}))
		return
	}

	cmd, ctx, err := s.registry.Begin(s.ctx, requestID, phrase)
	if err != nil {
		var busy *ErrBusy
		if asBusy(err, &busy) {
			send(NewResponse(requestID, ResponseBusy, BusyPayload{
				ActiveCommandID: busy.Active.ID,
				ActivePhrase:    busy.Active.Phrase,
				StartedAt:       busy.Active.StartedAt,
				Iteration:       busy.Active.Iteration,
				Total:           busy.Active.Total,
				Message:         busyMessage(busy.Active),
			}))
			return
		}
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "begin_failed", Message: err.Error(),
		}))
		return
	}

	send(NewResponse(requestID, ResponseAcknowledged, AcceptedPayload{
		CommandID: cmd.ID, Phrase: phrase,
	}))

	progress := func(ev ProgressPayload) {
		ev.CommandID = cmd.ID
		s.registry.Progress(cmd.ID, ev.Iteration, ev.Total, "")
		send(NewResponse(requestID, ResponseProgress, ev))
	}

	// A new request supersedes any unanswered question: the user moved on.
	s.pending.Clear()

	// Watch for confirmations for as long as this command runs, and stop watching when
	// it ends. Scoped to the command because a question belongs to the request that
	// asked it: publishing to a client that has moved on would leave the question open
	// until it timed out, and the command blocked behind it.
	if broker := s.runtime.Confirmations(); broker != nil {
		stop := broker.Watch(cmd.ID, func(ask ConfirmationPayload) {
			send(NewResponse(requestID, ResponseConfirmationRequired, ask))
		})
		defer stop()
	}

	outcome := s.runtime.Handle(ctx, phrase, progress)
	s.complete(requestID, cmd, phrase, outcome, send)
}

// complete turns an outcome into a terminal response, asking for clarification when
// the Director could not tell which control was meant.
func (s *Server) complete(requestID string, cmd *ActiveCommand, phrase string,
	outcome execute.Outcome, send func(ResponseEnvelope)) {

	// Ambiguity is not a failure — it is a question. Reporting it as a failure would
	// throw away the candidates the user is about to choose between.
	if outcome.Status == directorapi.ResultNeedsClarification && outcome.Resolution != nil {
		if ask, ok := clarificationFor(cmd.ID, phrase, outcome); ok {
			s.pending.Set(ask)
			s.registry.Finish(cmd.ID, CommandBlocked, 0, ask.Question)
			s.convo.Record(cmd.ID, phrase, "")
			send(NewResponse(requestID, ResponseClarificationRequired, ask))
			return
		}
	}

	state, respType := classify(outcome)
	// A terminal response must never be empty. An outcome that reached here with no
	// message says nothing to the user — it renders as a bare ":" — and hides whatever
	// actually happened. This turned a diagnosable ambiguity into a silent dead end.
	// One function owns terminal wording, and it can never return empty. See
	// terminal.go for the bare ":" this replaced.
	if to, isTimeout := trace.Timeout(errOf(outcome)); isTimeout {
		state, respType = CommandTimedOut, ResponseFailed
		outcome.Message = to.Error()
	}
	outcome.Message = TerminalReason(state, outcome)
	completed := completedActions(outcome)
	lastNode := ""
	if outcome.Node != nil {
		lastNode = string(outcome.Node.ID)
	}
	s.registry.Progress(cmd.ID, 0, 0, lastNode)
	result := s.registry.Finish(cmd.ID, state, completed, outcome.Message)

	// Conversation state is recorded whatever the outcome, because "do that again"
	// resolves against the action GRAPH — which only holds what actually happened —
	// while this records what was last ASKED. Conflating them would make a failed
	// phrase the thing a follow-up refers to.
	s.convo.Record(cmd.ID, phrase, lastNode)

	send(NewResponse(requestID, respType, OutcomePayload{
		CommandID:        cmd.ID,
		Phrase:           phrase,
		State:            result.State,
		Message:          outcome.Message,
		CompletedActions: completed,
		LastActionNode:   lastNode,
		Trace:            traceOf(outcome),
		Replay:           replaySummary(outcome),
	}))
}

// cancelActive stops whatever is running.
//
// THREE ARMS, in the order a person would expect them. Whatever is DRIVING THE DESKTOP wins the
// first stop, because that is the thing they are looking at when they say it: a command, or a
// performance, or a live rehearsal — all of which are one registry entry. Then a question they are
// being held at. Then, only when nothing is being done to their computer, the Learn episode they
// are in the middle of.
//
// The ordering is a decision and not an accident. A "stop" during a live rehearsal inside a Learn
// episode ends the TYPING and leaves the episode standing, so the person can say what they want
// to happen next; saying stop again abandons the episode. Collapsing the two would throw away a
// demonstration somebody had just given because they wanted the keyboard to stop moving.
//
// Deleting the learning arm must fail TestStoppingDuringADemonstrationCancelsTheEpisode.
func (s *Server) cancelActive() CancelPayload {
	active, ok := s.registry.Cancel()
	if !ok {
		// A PAUSED command is not a running one — the registry finished it as blocked
		// while the Director waits on the user — but "stop" must still abandon it.
		// Without this, saying "stop that" at a clarification reported "nothing is
		// running" and left the question pending, so the next unrelated phrase would
		// have been read as an answer to it.
		if ask, pending := s.pending.Get(); pending {
			s.pending.Clear()
			msg := fmt.Sprintf("cancelled %q — nothing further was done", ask.Phrase)
			if ask.Program() {
				msg = fmt.Sprintf("cancelled %q at step %d of %d — %d step(s) had completed",
					ask.Phrase, ask.StepIndex, ask.StepCount, ask.CompletedSteps)
			}
			return CancelPayload{Accepted: true, CommandID: ask.CommandID, Phrase: ask.Phrase, Message: msg}
		}
		// A LEARN EPISODE IS NOT A COMMAND, and this is the arm that used to be missing.
		//
		// Nothing is being done to the desktop during a demonstration — the person is
		// driving and Marco is watching — so no registry slot is claimed and neither of the
		// two arms above can see it. Saying "stop" answered "nothing is running" while the
		// demonstration went on capturing. See stop.go for why an episode is cancelled
		// through the ordinary acquisition request rather than by a second implementation
		// of ADR-066's Cancel.
		if out, learning := s.cancelLearning(); learning {
			return out
		}
		return CancelPayload{Accepted: false, Message: "nothing is running"}
	}
	return CancelPayload{
		Accepted: true, CommandID: active.ID, Phrase: active.Phrase,
		Message: fmt.Sprintf("cancelling %q — it will stop at the next safe point", active.Phrase),
	}
}

// status assembles the service's self-report.
func (s *Server) status() StatusPayload {
	out := StatusPayload{
		Running:      true,
		PID:          os.Getpid(),
		Uptime:       time.Since(s.startedAt).Round(time.Second),
		Version:      ProtocolVersion,
		Providers:    s.runtime.Providers(),
		Conversation: s.convo.Summary(),
		Values:       activeValuesOf(s.runtime),
		Collections:  activeCollectionsOf(s.runtime),
		Recent:       s.registry.Recent(5),
		Windows:      s.runtime.Windows(),
		// Said even when nothing else is wrong, because the alternative is a status
		// report that looks healthy on a Director that cannot see a single control.
		AccessibilityUnavailable: accessibilityReason(s.runtime),
	}
	if ask, ok := s.pending.Get(); ok {
		out.Clarification = &ask
	}
	if broker := s.runtime.Confirmations(); broker != nil {
		if ask, ok := broker.Pending(); ok {
			out.Confirmation = &ask
		}
	}
	out.UptimeStr = humanDuration(out.Uptime)

	if active, ok := s.registry.Active(); ok {
		out.Active = &ActiveSummary{
			CommandID: active.ID, Phrase: active.Phrase,
			StartedAt: active.StartedAt, Running: time.Since(active.StartedAt).Round(time.Second),
			State: active.State, Iteration: active.Iteration, Total: active.Total,
		}
	}
	if g := s.runtime.Graph(); g != nil {
		if nodes, err := g.Recent(0); err == nil {
			out.GraphNodes = len(nodes)
		}
	}
	return out
}

// history returns recent action graph nodes.
func (s *Server) history(limit int) HistoryPayloadResponse {
	if limit <= 0 {
		limit = 20
	}
	g := s.runtime.Graph()
	if g == nil {
		return HistoryPayloadResponse{}
	}
	nodes, err := g.Recent(limit)
	if err != nil {
		return HistoryPayloadResponse{}
	}

	out := HistoryPayloadResponse{Entries: make([]HistoryEntry, 0, len(nodes))}
	for _, n := range nodes {
		e := HistoryEntry{
			ID: string(n.ID), Timestamp: n.Timestamp, Phrase: n.Intent.Raw,
			Goal: n.Goal, App: n.ResolvedTarget.App,
			Role: n.ResolvedTarget.Role, Label: n.ResolvedTarget.Label,
			Success: n.Outcome.Success, Status: string(n.Outcome.Status),
			Reason:      firstNonEmpty(n.Verification.Reason, n.Outcome.FailureReason),
			SemanticKey: n.SemanticKey(),
		}
		if n.Parent != nil {
			e.Parent = string(*n.Parent)
		}
		if v, ok := n.Metadata["replay_of"].(string); ok {
			e.ReplayOf = v
		}
		out.Entries = append(out.Entries, e)
	}
	return out
}

// ── outcome mapping ───────────────────────────────────────────────────────────

// classify maps a Director outcome onto a command state and a response type.
//
// UNVERIFIED stays its own outcome rather than collapsing into failure. "I did it
// but could not confirm it" is different information from "it did not work", and the
// difference decides whether a user should try again — which is exactly the mistake
// that sent a browser two pages back in an earlier milestone.
func classify(out execute.Outcome) (CommandState, ResponseType) {
	switch out.Status {
	case directorapi.ResultDone:
		return CommandCompleted, ResponseCompleted
	case directorapi.ResultCancelled:
		return CommandCancelled, ResponseCancelled
	case directorapi.ResultPartial:
		return CommandUnverified, ResponseUnverified
	case directorapi.ResultBlocked, directorapi.ResultNeedsConfirmation:
		return CommandBlocked, ResponseFailed
	case directorapi.ResultNeedsClarification:
		return CommandFailed, ResponseFailed
	}
	return CommandFailed, ResponseFailed
}

// completedActions is how many desktop actions a command actually performed.
func completedActions(out execute.Outcome) int {
	if out.Replay != nil {
		return out.Replay.Completed
	}
	if out.Record != nil && out.Record.Execution.Performed {
		return 1
	}
	return 0
}

func traceOf(out execute.Outcome) []TraceLine {
	lines := make([]TraceLine, 0, len(out.Stages))
	for _, s := range out.Stages {
		lines = append(lines, TraceLine{Stage: s.Name, Detail: s.Detail, OK: s.OK})
	}
	return lines
}

func replaySummary(out execute.Outcome) *ReplaySummary {
	if out.Replay == nil {
		return nil
	}
	r := out.Replay
	sum := &ReplaySummary{
		SourceNode: string(r.Source.ID), Requested: r.Requested,
		Completed: r.Completed, StoppedBecause: r.StoppedBecause,
		Confidence: ReplayConfidence{
			Intent: r.Confidence.Intent, Target: r.Confidence.Target,
			Context: r.Confidence.Context, Overall: r.Confidence.Overall,
			Notes: r.Confidence.Notes,
		},
	}
	for _, it := range r.Iterations {
		entry := ReplayIteration{
			Index: it.Index + 1, Status: string(it.Analysis.Status),
			Verified: it.Verified, Reason: it.Reason,
		}
		if it.Node != nil {
			entry.NodeID = string(it.Node.ID)
		}
		sum.Iterations = append(sum.Iterations, entry)
	}
	return sum
}

func asBusy(err error, out **ErrBusy) bool {
	if b, ok := err.(*ErrBusy); ok {
		*out = b
		return true
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

var _ = io.EOF

// clarificationFor builds the question to ask, from the candidates that were actually
// in contention.
//
// Two filters, and both matter for different reasons:
//
//   - Only VIABLE candidates. The rejected ones are part of the explanation, not part
//     of the choice: offering a disabled control or a piece of inert text invites the
//     user to pick something that cannot be acted on.
//   - Only CONTENDERS. Live against VS Code, an unbounded list asked "3 controls match
//     'new' about equally well" and then offered four — the fourth being an unrelated
//     update notification that merely contained the word. A user picking it would have
//     picked something the Director never considered a match, and the count they were
//     told would not have matched the list they were shown.
//
// The offered list is a PREFIX of the usable candidates, never a re-ordering or a
// de-duplication. That is what keeps "the second one" meaning the same thing to the
// user and to the resolver, which indexes the same prefix when the answer comes back.
func clarificationFor(id CommandID, phrase string, out execute.Outcome) (ClarificationPayload, bool) {
	ask := ClarificationPayload{
		CommandID: id, Phrase: phrase,
		Question: out.Message, AskedAt: time.Now(),
	}
	if ask.Question == "" {
		ask.Question = "which one did you mean?"
	}

	// A question that interrupted a SEQUENCE carries where it interrupted it, so a
	// client can say "step 2 of 4" and a reader can see that the completed steps are
	// not about to be re-run. Empty for a single request, which is how a client tells
	// the two apart.
	if po := out.Program; po != nil && po.StoppedAt > 0 {
		ask.ProgramID = string(po.Program.ID)
		ask.StepIndex, ask.StepCount = po.StoppedAt, len(po.Program.Steps)
		ask.CompletedSteps = po.Completed
		if po.Pending != nil {
			ask.StepID = string(*po.Pending)
		}
	}
	// A question raised INSIDE a bounded iteration carries the offer.s identity, so an
	// answer can be checked against the list the user actually saw.
	if c := out.Collection; c != nil && c.EventID != "" {
		ask.EventID = c.EventID
		ask.CollectionName = c.CollectionName
		ask.Iteration = c.PausedAt
		ask.CompletedItems = c.Completed
	}

	// The contenders are consumed AS GIVEN. No re-filtering by Rejected, by score, or
	// by any other eligibility rule — the resolver already decided which candidates are
	// safe to offer, and a second opinion here is how the two layers came to disagree.
	//
	// The only thing applied is a length cap, and only because a spoken choice between
	// more than a handful is not a choice a person can make.
	for i, c := range out.Resolution.Contenders {
		if i >= maxClarificationCandidates {
			break
		}
		ask.Candidates = append(ask.Candidates, ClarificationCandidate{
			// The index is the position in the CONTENDER list, which is exactly what an
			// ordinal answer will index back into.
			Index: i + 1,
			ID:    string(c.ElementID),
			Label: c.Label,
			Role:  string(c.Role),
		})
	}

	// The question was written by the resolver against the full contest. If the offer
	// had to be trimmed to fit a spoken choice, the wording is replaced rather than
	// left to contradict the list underneath it.
	if n := out.Resolution.ContenderCount(); n > len(ask.Candidates) {
		ask.Question = fmt.Sprintf("%s — the closest %d are:",
			strings.TrimSuffix(ask.Question, "."), len(ask.Candidates))
	}

	// An AMBIGUOUS resolution that cannot produce a question is an internal
	// inconsistency, not a normal outcome — Resolution.Consistent() is the promise it
	// broke. Falling through silently is what produced an empty ":" for a user; saying
	// so makes the regression visible the moment it happens.
	return ask, len(ask.Candidates) >= directorapi.MinContenders
}

// maxClarificationCandidates bounds what is offered. A spoken choice between more
// than a handful of options is not a choice a person can make.
const maxClarificationCandidates = 5

// clarify answers a pending question.
func (s *Server) clarify(requestID string, p ClarifyPayload, send func(ResponseEnvelope)) {
	ask, pending := s.pending.Get()
	if !pending {
		// An answer with no question is NOT a new desktop command. "The first one"
		// searched for as a control label would either find nothing or, far worse, find
		// something — so a phrase that clearly answers a question is refused honestly
		// rather than reinterpreted.
		msg := "there is no question waiting for an answer"
		if p.EventID != "" {
			msg = "That clarification is no longer active."
		}
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "no_question", Message: msg,
		}))
		return
	}
	// An answer to a question that is no longer the pending one is refused rather
	// than applied to whatever is pending now — the user answered something else.
	if p.CommandID != "" && p.CommandID != ask.CommandID {
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "stale_question",
			Message: fmt.Sprintf("that answers %s, but the question waiting is %s",
				p.CommandID, ask.CommandID),
		}))
		return
	}
	// The OFFER's identity, checked explicitly.
	//
	//	A clarification belongs to one event in one observed collection state.
	//
	// Membership drift can replace a pending question with a fresh one at the same
	// iteration of the same command. Without this check an answer written for the old
	// contender list would be applied to the new one — the same failure a stale ordinal
	// causes, reached through the transport rather than through the world.
	//
	// A MISMATCH is rejected; an ABSENT id is not, so an older client that sends only a
	// phrase keeps working.
	if p.EventID != "" && ask.EventID != "" && p.EventID != ask.EventID {
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code:    "stale_clarification",
			Message: "That clarification is no longer active.",
		}))
		return
	}

	refinement, understood := intent.ParseClarification(p.Response)
	if !understood {
		// Not an answer. The phrase is a new request, and treating it as a choice
		// would act on a control the user never picked.
		s.pending.Clear()
		s.execute(requestID, ExecutePayload{Phrase: p.Response}, send)
		return
	}
	if refinement.Cancel {
		s.pending.Clear()
		// The paused PROGRAM is abandoned too, not just the question.
		//
		// Clearing only the pending question left the program alive with its captured
		// values still bound and still visible to `director status` — a cancelled
		// program whose data flow outlived it. Found live: `explain value title` kept
		// answering after the user said "stop".
		s.runtime.AbandonProgram("the clarification was cancelled")
		send(NewResponse(requestID, ResponseCancelled, OutcomePayload{
			CommandID: ask.CommandID, Phrase: ask.Phrase,
			State: CommandCancelled, Message: "cancelled — nothing was done",
		}))
		return
	}

	cmd, ctx, err := s.registry.Begin(s.ctx, requestID, ask.Phrase)
	if err != nil {
		var busy *ErrBusy
		if asBusy(err, &busy) {
			send(NewResponse(requestID, ResponseBusy, BusyPayload{
				ActiveCommandID: busy.Active.ID, ActivePhrase: busy.Active.Phrase,
				StartedAt: busy.Active.StartedAt,
				Message:   "something else is already running",
			}))
			return
		}
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "begin_failed", Message: err.Error(),
		}))
		return
	}
	s.pending.Clear()

	send(NewResponse(requestID, ResponseAcknowledged, AcceptedPayload{
		CommandID: cmd.ID, Phrase: ask.Phrase,
	}))
	progress := func(ev ProgressPayload) {
		ev.CommandID = cmd.ID
		s.registry.Progress(cmd.ID, ev.Iteration, ev.Total, "")
		send(NewResponse(requestID, ResponseProgress, ev))
	}

	outcome := s.runtime.HandleClarified(ctx, ask.Phrase, refinement, progress)
	s.complete(requestID, cmd, ask.Phrase, outcome, send)
}

// busyMessage says what the Director is doing, and how far through it is.
//
// The step position is the part that matters. "Already running X" leaves the user
// wondering whether it has hung; "step 3 of 5" tells them it is working, and tells
// them how much is left to decide whether to wait or to stop it.
func busyMessage(a ActiveCommand) string {
	if a.Total > 1 {
		return fmt.Sprintf(
			"BUSY\ncurrent program: %q\nstep %d of %d\n"+
				"say \"stop\" to cancel it, or wait for it to finish",
			a.Phrase, a.Iteration, a.Total)
	}
	return fmt.Sprintf(
		"BUSY\ncurrent command: %q\nsay \"stop\" to cancel it, or wait for it to finish",
		a.Phrase)
}

// terminalReason supplies a message for an outcome that arrived without one.
//
// Never a generic "failed". The interesting case is an ambiguity that produced no
// offerable question: the user asked for something that matched several things, none
// of which could be offered, and saying so is what lets them rephrase.
func terminalReason(out execute.Outcome) string {
	if out.Status == directorapi.ResultNeedsClarification {
		if out.Resolution != nil && len(out.Resolution.Candidates) > 0 {
			return fmt.Sprintf(
				"that matched %d things but none could be offered as a choice — "+
					"name the control more precisely",
				len(out.Resolution.Candidates))
		}
		return "that was ambiguous and no candidates could be offered — " +
			"name the control more precisely"
	}
	if out.Error != "" {
		return out.Error
	}
	return fmt.Sprintf("the request ended as %s with no explanation recorded", out.Status)
}

// explainValue answers a question about one live program-local value.
//
// Read-only and answerable while a command runs: it never takes the command lock for
// anything but a pointer read, and does all its work on a detached snapshot. That is
// what keeps `director explain value` responsive while the Director is blocked on a
// desktop call.
func (s *Server) explainValue(name string) ValueExplanationPayload {
	snap := s.runtime.ActiveValues()
	v, found := snap.Find(name)
	if !found {
		// The same message whether the program ended, the name was mistyped, or
		// nothing is running. See ValueExplanationPayload.
		return ValueExplanationPayload{
			Message: fmt.Sprintf("Unknown program-local value: %s", name),
		}
	}
	return ValueExplanationPayload{Found: true, ProgramID: snap.ProgramID, Value: &v}
}

// activeValuesOf returns the live values for the status payload, nil when there are
// none.
//
// Nil rather than an empty snapshot so an idle status carries no values section at all
// — the payload stays the shape it has always been when nothing is running, and a
// client that has never heard of values is unaffected.
func activeValuesOf(rt Runtime) *values.EnvironmentSnapshot {
	snap := rt.ActiveValues()
	if len(snap.Values) == 0 {
		return nil
	}
	return &snap
}

// activeCollectionsOf returns the live collections for the status payload, nil when
// there are none.
//
// Nil rather than an empty snapshot, so an idle status carries no collection section at
// all and a client that has never heard of collections is unaffected.
func activeCollectionsOf(rt Runtime) *collections.Snapshot {
	snap := rt.ActiveCollections()
	if len(snap.Collections) == 0 {
		return nil
	}
	return &snap
}
