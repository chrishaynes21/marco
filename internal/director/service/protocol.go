// Package service turns the Director into a long-lived local process that client
// commands talk to, instead of something each command constructs from scratch.
//
// # Why
//
// Semantic replay made the old model untenable. "Repeat that ten times" takes real
// time, and a spoken "stop" arrives as a DIFFERENT PROCESS — which cannot cancel a
// context inside the one running the loop. Everything else followed from the same
// root: the accessibility bridge was rebuilt per command, so Chrome and VS Code lost
// their (slowly-acquired) hydration between every request; and conversation state
// could not outlive a process that exited after one phrase.
//
// So: client processes submit intent; the service owns state, observation,
// execution, verification and cancellation.
//
// # Transport: loopback TCP with a token
//
// Chosen from the options that support request/response, event streaming,
// cancellation, multiple clients, discovery and timeouts.
//
//   - A Windows NAMED PIPE would be the strongest choice on security grounds: it is
//     local by construction and can carry an ACL. Go's standard library cannot create
//     one, and the usual package (go-winio) is an external dependency — which Marco's
//     engine does not permit. Implementing overlapped pipe I/O by hand is real work
//     with real ways to get shutdown subtly wrong, and it would be Windows-only.
//   - gRPC is a large dependency for a JSON-shaped local protocol.
//   - LOOPBACK TCP is pure standard library, streams naturally over a connection,
//     handles many clients, and works unchanged on macOS and Linux later.
//
// The honest cost: a loopback port is reachable by any process running as this user,
// where a named pipe could be ACL'd. The mitigation is a 256-bit random token,
// regenerated per service start, stored in a 0600 file, and required on every
// connection before any request is read. That is protection against another local
// program stumbling onto the port — not against a process already running as you,
// which could read the token file anyway. A named pipe with a security descriptor is
// the upgrade path if that distinction ever matters.
//
// The listener binds 127.0.0.1 explicitly. It is never bound to a routable
// interface, and there is no configuration that would let it be.
package service

import "github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"

import (
	"encoding/json"

	"fmt"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/timeline"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
	"time"
)

// ProtocolVersion is the wire contract this build speaks.
//
// A mismatch fails explicitly rather than being negotiated away. A client and
// service that disagree about the shape of a command are more dangerous than a
// client that cannot connect: the failure would surface as the wrong thing
// happening on the desktop rather than as an error.
// Version 4 adds WORLD and EVENTS: the fused world state a front-end renders, and the
// cursor-based event log that lets it show change rather than only a snapshot.
//
// Version 5 adds GET_LIVE_ANALYSIS and GET_OBSERVATION_EVENTS: what a passive observation
// session has learned SO FAR, and its cursor-addressable findings — so a client can show
// evidence accumulating instead of waiting for the session to end.
//
// Version 6 adds PLAYBILL: the ONE read-only account a presentation renders. The
// diagnostic surfaces above each answer a specialist's question and a front-end wanting
// to say "I recognise this as the pause menu" had to poll four of them and join the
// results itself — which put the joining, and therefore the disagreement, in the client.
//
// Version 7 adds SUBJECT to PERFORM: the durable id of the outcome a learned play ends on,
// so a play and its goal are joined by identity instead of by the words of their names.
// Bumped even though the field is optional, because a client that omits it degrades to the
// lossy name join SILENTLY — a punctuated phrase answers not_learned rather than failing —
// and this check exists precisely so a build disagreement is loud instead of quiet.
//
// Version 8 merged the two acquisition requests. `ObserveTeach` and `ObserveLearn` described one
// event — the person demonstrates and Marco acquires — in two vocabularies, and they were not
// peers: the control surface's type was a facade translating its own verbs into the other's. There
// is one `ObserveLearn` now, carrying both the surface's verbs and the session's configuration,
// and `Surface` says which account of the session to answer with. `ObserveTeach.Watch` became
// `Evidence`, because `Watch` on the surviving type is Light Mode and merging them under one name
// would have made `director learn --watch` silently start it. See
// [[ADR-086-one-acquisition-one-word-one-request]].
//
// Version 9 adds SHOWING to the observation query: which remembered place is in front RIGHT NOW,
// in one named application. It exists because the process that runs a play is `marco`, and `marco`
// has no eyes — the Director owns the only fresh-look machinery in this tree. Until this verb, an
// EDITED learned play, which the authority seam has just told the person "runs like anything else
// you have written", refused at its own first line with "Marco could not check". The Plays view
// offers an edit button on every row, so that was an ordinary path: edit a play, lose the play.
//
// A bump for one optional field on a query that already has a dozen is arguable, so here is the
// argument, in the terms the paragraph above sets — does a build disagreement raise an error, or
// make the wrong thing happen on the desktop?
//
// TODAY it raises no error and does no harm either. A version-8 Director receiving a SHOWING query
// does not reject it: the field is one it has never heard of, so the query falls through to the
// session branches and answers with an observation snapshot, or with an error about there being no
// session at all. Decoded as a `ShowingView` that is an empty `Outcome`, which is not in
// `screenhost`'s vocabulary, which the client turns into a refusal. Safe — and safe only because
// two unrelated JSON shapes happen not to share a field name. Give an observation snapshot an
// `outcome` field one day, or rename `ShowingView`'s, and the same build mismatch becomes a play
// that believes it is standing on a screen nobody looked at: the wrong thing happening on the
// desktop, arrived at in silence, with the refusal that should have stopped it decoded away. That
// is precisely the hazard this check exists for, and it is the same reasoning Version 7 was bumped
// on for a field that was also optional.
// Version 10 adds COST to PERFORM: what carrying out a route spent finding out where it was —
// screen readings, Place resolutions, source establishments, and how many of those were answered
// by a proof the walk already held. Developer-facing, additive, and on a RESPONSE rather than a
// request.
//
// The same question as above, and this time the honest answer is that the harm is to a
// MEASUREMENT rather than to the desktop. A version-9 Director answering a version-10 client
// omits the object, which decodes to zeros — and zeros in these fields read as "this route never
// looked at the screen", which is the most flattering possible reading of the optimization they
// exist to test. Roadmap 35C is explicit that a live number must be measured or declared
// unmeasured, never guessed, and a silent zero is a guess wearing a measurement's clothes.
//
// So it is bumped for the same reason Version 7 was: not because the field is required, but
// because the degradation would be quiet and would be believed.
const ProtocolVersion = 10

// RequestType names what a client is asking for.
type RequestType string

const (
	// RequestExecutePhrase runs a natural-language request. Mutating.
	RequestExecutePhrase RequestType = "EXECUTE_PHRASE"
	// RequestCancelActive cancels whatever is running. Read-only with respect to the
	// desktop, so it is always accepted, including while a command is in flight —
	// which is the entire point of it.
	RequestCancelActive RequestType = "CANCEL_ACTIVE"
	// RequestStatus reports what the service is doing. Read-only.
	RequestStatus RequestType = "STATUS"
	// RequestExplainValue explains one program-local value. Read-only, and answerable
	// only while the owning program is running or paused.
	RequestExplainValue RequestType = "EXPLAIN_VALUE"
	// RequestCollections reports the running or paused program.s collections. Read-only.
	RequestCollections RequestType = "COLLECTIONS"
	// RequestHistory returns recent action nodes. Read-only.
	RequestHistory RequestType = "HISTORY"
	// RequestPerception asks what the perception pipeline did: which providers ran,
	// what the recent cycles produced, and what fusion made of the latest one.
	RequestPerception RequestType = "PERCEPTION"
	// RequestExplain asks for the same picture plus the account of every element:
	// what evidence produced it, what was refused, how its confidence was derived.
	// A separate request because reconstructing that is quadratic in the observation
	// count, and no ordinary command should pay for it.
	RequestExplain RequestType = "EXPLAIN"
	// RequestReadText asks for one OCR pass of the active window. Explicit, because
	// it captures the screen: no ordinary command triggers it.
	RequestReadText RequestType = "READ_TEXT"
	// RequestReadRegion asks for one visual pass over a region: what it looks like and
	// whether it changed. Explicit, like READ_TEXT, because it captures the screen.
	RequestReadRegion RequestType = "READ_REGION"
	// RequestWaitStatus asks what the Director is currently waiting FOR. Cheap and
	// read-only: it reports the wait in flight and starts nothing.
	RequestWaitStatus RequestType = "WAIT_STATUS"
	// RequestEditHistory returns the recent semantic edit outcomes: which strategy
	// each used and why it fell back. Read-only and cheap — it reads a record.
	RequestEditHistory RequestType = "EDIT_HISTORY"
	// RequestLowerings returns the recent lowered operations with their generated
	// Marco. Read-only, and already redacted before it is stored.
	RequestLowerings RequestType = "LOWERINGS"
	// RequestSemanticActions returns the recent semantic action outcomes: which verb
	// was asked for, which implementation the capability ladder chose, and which
	// stronger ones were unavailable and why. Read-only.
	RequestSemanticActions RequestType = "SEMANTIC_ACTIONS"
	// RequestRunOperation executes ONE lowered operation, for diagnostics and for
	// the operations that have no spoken phrase (launch, activate, window state).
	// Mutating: it goes through the same executor, guard and compiler a planned
	// action does, so it is a way to exercise the path rather than to bypass it.
	RequestRunOperation RequestType = "RUN_OPERATION"
	// RequestTrace returns one command.s phase trace. Read-only and cheap: it copies
	// timings, never payloads.
	RequestTrace RequestType = "TRACE"
	// RequestShowLast returns the most recent action node. Read-only.
	RequestShowLast RequestType = "SHOW_LAST"
	// RequestClarify answers a pending clarification question.
	RequestClarify RequestType = "CLARIFY"
	// RequestConfirm answers a pending confirmation.
	//
	// Read-only with respect to the desktop — it decides nothing and performs nothing;
	// it unblocks a command that is already waiting. Deliberately NOT mutating, so it
	// stays answerable while the command it answers is in flight, which is the entire
	// point of it.
	RequestConfirm RequestType = "CONFIRM"
	// RequestDemonstration controls and reads the demonstration recorder: start, stop,
	// abandon, list, show, extract and approve.
	//
	// ONE request type with an action field rather than seven, because they share a
	// subject and a payload shape and because the recorder is one thing — a client that
	// could start a session but not ask what it had recorded would be a strange client.
	//
	// NOT mutating with respect to the desktop, and that is exact rather than convenient:
	// starting a recording performs no input, extraction is pure computation over a stored
	// session, and approval writes a file and a registry entry. The one thing a
	// demonstration command never does is touch the screen — which is what makes it
	// answerable while a command it is recording is in flight.
	RequestDemonstration RequestType = "DEMONSTRATION"
	// RequestProcedures reports what the Director can do, built-in and learned.
	RequestProcedures RequestType = "PROCEDURES"
	// RequestVision runs one detection pass, or reads the frame log.
	//
	// A fresh pass CAPTURES THE SCREEN, which is why it is a request a caller makes
	// deliberately rather than something any command does on its behalf — the same rule
	// READ_TEXT follows. Reading the log captures nothing.
	RequestVision RequestType = "VISION"
	// RequestWindows lists the current live windows with ephemeral ids.
	RequestWindows RequestType = "WINDOWS"
	// RequestObserve starts a passive observation session.
	// RequestWorld returns the Director's CURRENT BELIEF — the fused world state, as
	// entities rather than as evidence.
	//
	// Read-only in the strongest sense available: it copies the world the Director
	// already holds. It performs no perception, starts no cycle, attaches no provider
	// and mutates no runtime state, exactly as STATUS's window list does. Asking what
	// the Director believes cannot change what it believes.
	RequestWorld RequestType = "WORLD"
	// RequestEvents returns the perception event log from a cursor.
	//
	// The protocol is otherwise request/response, so a front-end could only ever show a
	// snapshot. This is what lets it show CHANGE without polling twice and subtracting —
	// which would put the derivation in the client and make every client's version of
	// history slightly different.
	RequestEvents RequestType = "EVENTS"
	// RequestLiveAnalysis returns what a passive observation session has learned SO FAR.
	//
	// Read-only and cheap: it reads the analysis the session already accumulated. It
	// starts no session, takes no sample, and attaches no provider — a HUD polling it
	// cannot become a participant in the thing it is describing.
	RequestLiveAnalysis RequestType = "GET_LIVE_ANALYSIS"
	// RequestObservationEvents returns a session's live findings after a cursor.
	//
	// Separate from the perception EVENTS request: that one covers the Director's own
	// observation cycles, this one covers what a PASSIVE SESSION concluded. They have
	// different lifetimes and different sequence spaces, and merging them would make a
	// cursor ambiguous.
	RequestObservationEvents RequestType = "GET_OBSERVATION_EVENTS"
	// RequestObserve starts a passive observation session.
	RequestObserve RequestType = "OBSERVE"
	// RequestObservation reads, lists or cancels observation sessions.
	RequestObservation RequestType = "OBSERVATION"
	// RequestLearned writes down what Marco has learned, as ordinary Marco.
	//
	// A READ. It produces text and saves nothing — a separate request kind from execution
	// because the authority is different in kind: none at all.
	RequestLearned RequestType = "LEARNED"
	// RequestGame reports what capability pack serves the foreground application, what
	// each registered pack contributes, and what the Director can see of an inventory.
	//
	// Read-only with respect to the desktop: detection is recomputed from the world the
	// Director already observed, and nothing here observes, plans or acts.
	RequestGame RequestType = "GAME"

	// RequestPlaybill returns the ONE read-only account a presentation renders.
	//
	// Read-only in the strongest sense available: it copies state the Director already
	// holds — the same rule STATUS and WORLD follow. It starts no observation, takes no
	// sample, attaches no provider, runs no OCR pass and no vision inference, and forms
	// no hypothesis. A surface polling it cannot become a participant in the thing it
	// describes.
	//
	// It grants NOTHING. There is no field in the reply anything can act on: the pending
	// question carries the id its answer routes by and the route it travels, and both of
	// those are the ordinary paths a person already had. Rendering "ready to rehearse"
	// creates no rehearsal grant, and rendering "learned play available" authorises no
	// execution.
	RequestPlaybill RequestType = "PLAYBILL"

	// RequestShutdown stops the service.
	RequestShutdown RequestType = "SHUTDOWN"
	// RequestPing is the liveness check a client uses during discovery.
	RequestPing RequestType = "PING"
)

// Mutating reports whether this request may change the desktop.
//
// It is the gate behind "one mutating command at a time". Everything else stays
// answerable while a command runs, because a status query that blocked until a
// ten-iteration replay finished would be useless precisely when it is most wanted.
func (t RequestType) Mutating() bool { return t == RequestExecutePhrase }

// ResponseType names what the service is sending back.
//
// One request may produce SEVERAL responses: an acceptance, a start, any number of
// progress events, and exactly one terminal outcome. That is what lets a client show
// a ten-iteration replay as it happens rather than after it finishes.
type ResponseType string

const (
	// ResponseAcknowledged confirms a command was taken on. Named for what a person
	// sees rather than for the protocol's internals.
	ResponseAcknowledged ResponseType = "ACKNOWLEDGED"
	ResponseAccepted     ResponseType = "ACCEPTED"
	ResponseStarted      ResponseType = "STARTED"
	ResponseProgress     ResponseType = "PROGRESS"
	ResponseCompleted    ResponseType = "COMPLETED"
	ResponseUnverified   ResponseType = "UNVERIFIED"
	ResponseFailed       ResponseType = "FAILED"
	ResponseCancelled    ResponseType = "CANCELLED"
	// ResponseClarificationRequired asks the user which of several controls was meant.
	// Terminal for the request that produced it: the answer arrives as a new CLARIFY.
	ResponseClarificationRequired ResponseType = "CLARIFICATION_REQUIRED"
	// ResponseConfirmationRequired asks the user to agree to something before it
	// happens. NOT terminal, unlike a clarification: the command is still running and
	// still holding its bindings, and the answer arrives as a CONFIRM on another
	// connection while this one keeps streaming.
	ResponseConfirmationRequired ResponseType = "CONFIRMATION_REQUIRED"
	ResponseStatus               ResponseType = "STATUS"
	ResponsePerception           ResponseType = "PERCEPTION"
	ResponseBusy                 ResponseType = "BUSY"
	ResponseError                ResponseType = "ERROR"
	ResponsePong                 ResponseType = "PONG"
)

// Terminal reports whether this response ends the exchange for a request.
func (t ResponseType) Terminal() bool {
	switch t {
	case ResponseCompleted, ResponseUnverified, ResponseFailed,
		ResponseCancelled, ResponseStatus, ResponseBusy, ResponseError, ResponsePong,
		ResponseClarificationRequired:
		return true
	}
	return false
}

// RequestEnvelope wraps every client request.
type RequestEnvelope struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Type            RequestType     `json:"type"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	SentAt          time.Time       `json:"sent_at"`
}

// ResponseEnvelope wraps every service response.
type ResponseEnvelope struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Type            ResponseType    `json:"type"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	SentAt          time.Time       `json:"sent_at"`
}

// ── payloads ──────────────────────────────────────────────────────────────────
//
// Every payload is a concrete struct. Nothing on the wire is an interface value:
// an encoded interface cannot be decoded, because the decoder has no way to know
// which type to build. The same rule the action graph's storage follows, for the
// same reason.

// ExecutePayload is an EXECUTE_PHRASE request.
type ExecutePayload struct {
	Phrase string `json:"phrase"`
	// DryRun plans and verifies without performing real input.
	DryRun bool `json:"dry_run,omitempty"`
}

// HistoryPayload is a HISTORY request.
type HistoryPayload struct {
	Limit int `json:"limit,omitempty"`
}

// DemonstrationAction is what a DEMONSTRATION request asks for.
//
// A closed vocabulary. An unrecognised action is an error rather than a default, because
// the defaults available are "do nothing" — which would look like the recorder ignoring
// the user — and "start recording", which would be worse.
type DemonstrationAction string

const (
	DemoStart    DemonstrationAction = "start"
	DemoStop     DemonstrationAction = "stop"
	DemoAbandon  DemonstrationAction = "abandon"
	DemoList     DemonstrationAction = "list"
	DemoShow     DemonstrationAction = "show"
	DemoActive   DemonstrationAction = "active"
	DemoExtract  DemonstrationAction = "extract"
	DemoApprove  DemonstrationAction = "approve"
	DemoForget   DemonstrationAction = "forget"
	DemoExplain  DemonstrationAction = "explain"
	DemoLearned  DemonstrationAction = "learned"
	DemoProposal DemonstrationAction = "proposal"
)

// DemonstrationPayload is a DEMONSTRATION request.
type DemonstrationPayload struct {
	Action DemonstrationAction `json:"action"`
	// ID names a recorded demonstration, for show, extract, explain and approve.
	ID string `json:"id,omitempty"`
	// Name names a learned procedure, for explain and forget.
	Name string `json:"name,omitempty"`
	// Reason explains an abandonment.
	Reason string `json:"reason,omitempty"`
}

// DemonstrationResponse is what a DEMONSTRATION request returns.
//
// One shape for every action, with the parts that do not apply left out. A client renders
// whichever fields came back, so adding an action does not add a response type — and the
// fields are the domain's own types rather than restatements of them, because a service
// that reshaped a proposal would be a second opinion about what was extracted.
type DemonstrationResponse struct {
	// Recording is the open session, when one is.
	Recording *demo.Demonstration `json:"recording,omitempty"`
	// Demonstration is the one that was asked for.
	Demonstration *demo.Demonstration `json:"demonstration,omitempty"`
	// Demonstrations is the list, newest first.
	Demonstrations []*demo.Demonstration `json:"demonstrations,omitempty"`
	// Extraction is the proposal and the decisions behind it.
	Extraction *demo.Extraction `json:"extraction,omitempty"`
	// Explanation is those decisions grouped by the question they answer.
	Explanation *demo.Explanation `json:"explanation,omitempty"`
	// Learned is an approved procedure, and Procedures the list of them.
	Learned    *demo.Learned   `json:"learned,omitempty"`
	Procedures []*demo.Learned `json:"procedures,omitempty"`
	// Message is one sentence for a person.
	Message string `json:"message,omitempty"`
}

// AcceptedPayload confirms a command was taken on.
type AcceptedPayload struct {
	CommandID CommandID `json:"command_id"`
	Phrase    string    `json:"phrase"`
}

// ProgressPayload is one step of a long-running command.
type ProgressPayload struct {
	CommandID CommandID `json:"command_id"`
	// Stage names what is happening: "observe", "resolve", "execute", "verify",
	// "iteration".
	Stage string `json:"stage"`
	// Iteration and Total describe a replay's position, 0 when not applicable.
	Iteration int    `json:"iteration,omitempty"`
	Total     int    `json:"total,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Verified  bool   `json:"verified,omitempty"`
}

// OutcomePayload is a command's terminal result.
type OutcomePayload struct {
	CommandID CommandID    `json:"command_id"`
	Phrase    string       `json:"phrase"`
	State     CommandState `json:"state"`
	Message   string       `json:"message"`
	// CompletedActions is how many desktop actions actually happened. A cancelled
	// replay reports what it managed before stopping, because "cancelled" without a
	// count leaves the user unsure what state their machine is in.
	CompletedActions int `json:"completed_actions"`
	// LastActionNode is the last action graph node this command produced.
	LastActionNode string `json:"last_action_node,omitempty"`
	// Trace is the human-readable explanation, when the client asked for one.
	Trace []TraceLine `json:"trace,omitempty"`
	// Replay carries the per-iteration detail for a repeat.
	Replay *ReplaySummary `json:"replay,omitempty"`
}

// TraceLine is one line of a command's explanation.
type TraceLine struct {
	Stage  string `json:"stage"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

// ReplaySummary describes a repeat's iterations.
type ReplaySummary struct {
	SourceNode     string            `json:"source_node"`
	Requested      int               `json:"requested"`
	Completed      int               `json:"completed"`
	StoppedBecause string            `json:"stopped_because"`
	Confidence     ReplayConfidence  `json:"confidence"`
	Iterations     []ReplayIteration `json:"iterations,omitempty"`
}

// ReplayConfidence mirrors the executor's diagnostic, flattened for the wire.
type ReplayConfidence struct {
	Intent  float64  `json:"intent"`
	Target  float64  `json:"target"`
	Context float64  `json:"context"`
	Overall float64  `json:"overall"`
	Notes   []string `json:"notes,omitempty"`
}

// ReplayIteration is one pass of a repeat, flattened for the wire.
type ReplayIteration struct {
	Index    int    `json:"index"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
	Reason   string `json:"reason,omitempty"`
	NodeID   string `json:"node_id,omitempty"`
}

// BusyPayload is returned when a mutating command arrives while one is running.
type BusyPayload struct {
	ActiveCommandID CommandID `json:"active_command_id"`
	ActivePhrase    string    `json:"active_phrase"`
	// Total is the program.s step count, 0 for a single command.
	Total     int       `json:"total,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Iteration int       `json:"iteration,omitempty"`
	Message   string    `json:"message"`
}

// CancelPayload reports the outcome of a cancellation request.
type CancelPayload struct {
	Accepted  bool      `json:"accepted"`
	CommandID CommandID `json:"command_id,omitempty"`
	Phrase    string    `json:"phrase,omitempty"`
	Message   string    `json:"message"`
}

// StatusPayload is the service's self-report.
type StatusPayload struct {
	Running   bool          `json:"running"`
	PID       int           `json:"pid"`
	Uptime    time.Duration `json:"uptime"`
	UptimeStr string        `json:"uptime_human"`
	Version   int           `json:"protocol_version"`

	Active *ActiveSummary `json:"active,omitempty"`

	// Watching is whether ambient observation is on, and what it has noticed.
	//
	// A VALUE, not a pointer, unlike almost every optional thing around it. "Is Marco watching
	// me" is a question somebody is entitled to a straight answer to, and a field that is
	// absent when the answer is no cannot be told apart from a field a Director too old to
	// have it never sent. A pointer would make the common case — not watching — the absent
	// one. So it is always present and it always says. See ADR-093.
	//
	// (The `omitempty` next door on Active does the omitting; on a struct value the tag would
	// do nothing at all, which is why there isn't one here rather than a claim being made by
	// its absence.)
	Watching AmbientView `json:"watching"`

	// Providers reports accessibility lifecycle, per application.
	Providers []ProviderStatus `json:"providers,omitempty"`

	// AccessibilityUnavailable is why the Accessibility Actor cannot act, empty when it can.
	//
	// An EMPTY Providers list has two causes that look identical — nothing has been observed
	// yet, or there is no bridge to observe through — and only one of them is something the
	// person can fix. This is the one that says which. The Director boots without the bridge
	// now rather than refusing to start, so it has to be able to say what it lost.
	AccessibilityUnavailable string `json:"accessibility_unavailable,omitempty"`

	// Clarification is the question awaiting an answer, if any. A front-end reads
	// this to know that the next phrase is an ANSWER rather than a new request.
	Clarification *ClarificationPayload `json:"clarification,omitempty"`

	// Windows is what the Director can currently SEE, from its most recent observation.
	//
	// Added because nothing else exposed it and a client genuinely needs it: a
	// front-end that wants to say "acting in Explorer — tmp1234" had no way to ask, and
	// a harness that must positively identify the window it is about to drive had no
	// way to check before sending input. The perception diagnostics report which
	// providers ran and what fusion made of the evidence; neither answers "which
	// windows are there".
	//
	// Read-only and cheap: the last observed world's window list, copied. It starts no
	// observation, so asking cannot change what is in front of the user.
	Windows []directorapi.Window `json:"windows,omitempty"`

	// Confirmation is the agreement being waited on, if any. A front-end that was not
	// watching the command — or that reconnected — reads it here rather than losing
	// the question, which would leave the command blocked until it timed out.
	Confirmation *ConfirmationPayload `json:"confirmation,omitempty"`

	// Conversation is what "that" currently refers to.
	Conversation ConversationSummary `json:"conversation"`

	// Recent is the last few command sessions — distinct from action graph nodes.
	Recent []CommandResult `json:"recent,omitempty"`

	// Values are the running or paused program.s captured values, as safe metadata.
	// Empty when no program is active — which is the normal case, not a fault.
	//
	// Snapshots only: the payload has no field that can hold a value.s content, so
	// this cannot leak whatever a future field is added beside it.
	Values *values.EnvironmentSnapshot `json:"values,omitempty"`

	// Collections are the running or paused program.s bounded sets, as safe metadata.
	// Counts, states and digests — never member labels.
	Collections *collections.Snapshot `json:"collections,omitempty"`

	GraphNodes int `json:"graph_nodes"`
}

// ActiveSummary describes the command in flight.
type ActiveSummary struct {
	CommandID CommandID     `json:"command_id"`
	Phrase    string        `json:"phrase"`
	StartedAt time.Time     `json:"started_at"`
	Running   time.Duration `json:"running"`
	State     CommandState  `json:"state"`
	Iteration int           `json:"iteration,omitempty"`
	Total     int           `json:"total,omitempty"`
}

// ConversationSummary is what a follow-up phrase would resolve against.
type ConversationSummary struct {
	LastPhrase     string    `json:"last_phrase,omitempty"`
	LastActionNode string    `json:"last_action_node,omitempty"`
	LastCommandID  CommandID `json:"last_command_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ErrorPayload is a structured failure.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HistoryEntry is one action graph node, flattened for the wire.
//
// Flattened rather than sent as an ActionNode because a node holds an ActionSpec
// whose shape is a storage concern; a client showing a list needs a few fields, and
// coupling the wire to the storage schema would make either hard to change.
type HistoryEntry struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Phrase      string    `json:"phrase"`
	Goal        string    `json:"goal"`
	App         string    `json:"app,omitempty"`
	Role        string    `json:"role,omitempty"`
	Label       string    `json:"label,omitempty"`
	Success     bool      `json:"success"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	Parent      string    `json:"parent,omitempty"`
	SemanticKey string    `json:"semantic_key,omitempty"`
	ReplayOf    string    `json:"replay_of,omitempty"`
}

// HistoryPayloadResponse carries a list of nodes.
type HistoryPayloadResponse struct {
	Entries []HistoryEntry `json:"entries"`
}

// ── helpers ──

// NewRequest builds an envelope with the payload encoded.
func NewRequest(id string, t RequestType, payload any) (RequestEnvelope, error) {
	env := RequestEnvelope{
		ProtocolVersion: ProtocolVersion,
		RequestID:       id,
		Type:            t,
		SentAt:          time.Now(),
	}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return env, fmt.Errorf("service: encoding %s payload: %w", t, err)
		}
		env.Payload = raw
	}
	return env, nil
}

// NewResponse builds a response envelope with the payload encoded.
func NewResponse(id string, t ResponseType, payload any) ResponseEnvelope {
	env := ResponseEnvelope{
		ProtocolVersion: ProtocolVersion,
		RequestID:       id,
		Type:            t,
		SentAt:          time.Now(),
	}
	if payload != nil {
		if raw, err := json.Marshal(payload); err == nil {
			env.Payload = raw
		}
	}
	return env
}

// Decode reads a response payload into out.
func (r ResponseEnvelope) Decode(out any) error {
	if len(r.Payload) == 0 {
		return fmt.Errorf("service: %s response has no payload", r.Type)
	}
	return json.Unmarshal(r.Payload, out)
}

// Decode reads a request payload into out.
func (r RequestEnvelope) Decode(out any) error {
	if len(r.Payload) == 0 {
		return fmt.Errorf("service: %s request has no payload", r.Type)
	}
	return json.Unmarshal(r.Payload, out)
}

// CheckVersion reports whether an envelope's protocol version is usable.
//
// Explicit failure, never coercion. A client and service that disagree about the
// shape of a command would otherwise surface the disagreement as the wrong thing
// happening on the desktop.
func CheckVersion(got int) error {
	if got != ProtocolVersion {
		return fmt.Errorf(
			"protocol version mismatch: this build speaks version %d, the other side sent %d — "+
				"restart the Director service so both are the same build",
			ProtocolVersion, got)
	}
	return nil
}

// RunOperationPayload carries one operation to execute.
//
// The typed Operation, not a Marco string. A client that could submit source would be
// a way to run arbitrary Marco through the Director's privileges; a client that
// submits an Operation gets the same validation, the same foreground guard and the
// same compiler as everything else.
type RunOperationPayload struct {
	Operation marcoexec.Operation `json:"operation"`
}

// TracePayload asks for one command's phase trace.
type TracePayload struct {
	// CommandID names the command, or "last" for the most recent.
	CommandID string `json:"command_id"`
}

// ExplainValuePayload asks about one program-local value.
type ExplainValuePayload struct {
	Name string `json:"name"`
}

// ValueExplanationPayload is one value's safe account.
//
// Found distinguishes "the value is not there" from "there is no program", but the
// MESSAGE for both is the same: telling a caller that a value with that name once
// existed is a fact about a finished program this layer has no business remembering.
type ValueExplanationPayload struct {
	Found     bool                  `json:"found"`
	ProgramID string                `json:"program_id,omitempty"`
	Value     *values.ValueSnapshot `json:"value,omitempty"`
	Message   string                `json:"message,omitempty"`
}

// GameAction is what a GAME request asks for.
type GameAction string

const (
	// GameDetected reports what capability pack serves what is in front.
	GameDetected GameAction = "detected"
	// GameCapabilities reports what every registered pack contributes.
	GameCapabilities GameAction = "capabilities"
	// GameInventory reports what the Director can see of what the player holds.
	GameInventory GameAction = "inventory"
)

// GamePayload is a GAME request.
type GamePayload struct {
	Action GameAction `json:"action"`
	// Container narrows an inventory query to one container.
	Container string `json:"container,omitempty"`
}

// GameResponse is what a GAME request returns.
type GameResponse struct {
	// Active is what the Director believes it is looking at.
	Active game.Active `json:"active"`
	// Report is what every pack contributes, for the capabilities view.
	Report game.Report `json:"report,omitzero"`
	// Inventory is what the Director can see of the player's holdings.
	Inventory game.InventoryReport `json:"inventory,omitzero"`
	// Packs is how many are registered, so a client can tell "none detected" from
	// "none exist".
	Packs int `json:"packs"`
}

// VisionPayload is a VISION request.
type VisionPayload struct {
	// Region narrows the pass to a rectangle. Nil means the whole window.
	Region *directorapi.Rect `json:"region,omitempty"`
	// Target names the window to look at, instead of whatever is in front. Focus then
	// stops being an input — see windowref.Selector.
	Target windowref.Selector `json:"target,omitempty"`
	// Frames asks for the frame log instead of a fresh pass. A read, not a capture:
	// the two are different enough that conflating them would make a listing take a
	// screenshot.
	Frames bool `json:"frames,omitempty"`
	// Last asks for the most recent pass without performing one, for the same reason.
	Last bool `json:"last,omitempty"`
}

// VisionResponse is what a VISION request returns.
type VisionResponse struct {
	// Diagnostics is one pass, described.
	Diagnostics vision.Diagnostics `json:"diagnostics,omitzero"`
	// Frames is the recent frame log, newest first.
	Frames []vision.FrameRecord `json:"frames,omitempty"`
}

// WindowsPayload asks for the current live windows.
type WindowsPayload struct {
	// Application narrows the listing to one application's windows. Empty lists all.
	Application string `json:"application,omitempty"`
}

// WindowsResponse is the listing.
//
// Ephemeral ids only: no raw handle crosses this boundary, so a client cannot come to
// depend on one.
type WindowsResponse struct {
	Windows []windowref.Listing `json:"windows"`
}

// ── world state ───────────────────────────────────────────────────────────────

// WorldPayload asks for the current fused world.
type WorldPayload struct {
	// Limit bounds the entity list. Zero takes the service's own bound.
	Limit int `json:"limit,omitempty"`
	// Actionable narrows the listing to entities something could actually be done to.
	Actionable bool `json:"actionable,omitempty"`
}

// WorldEntity is one fused element, as a client may see it.
//
// # Identity is opaque, and that is deliberate
//
// Identity is a digest of the element's internal id, not the id itself. It is stable for
// as long as the Director considers the element the same thing, so a client can correlate
// across polls, select one, and ask for its explanation — and it carries nothing back. A
// raw ElementID may encode a platform RuntimeId or a window handle, and a client that came
// to depend on one would be depending on the Director's storage schema. Same rule as
// WindowsResponse's ephemeral ids.
type WorldEntity struct {
	Identity string `json:"identity"`
	Role     string `json:"role"`

	// Label is privacy-classified. It is produced by the SAME classifier a passive
	// observation session uses (observe.Classify), never by a second allowlist kept
	// here — two copies of that rule would eventually disagree, and the one that
	// disagreed quietly would be this one.
	Label observe.SafeLabel `json:"label"`

	// Confidence is belief that the element exists as described; LabelConfidence is
	// belief about what it is CALLED. Separate because a control can be certainly
	// present and uncertainly named.
	Confidence      float64 `json:"confidence"`
	LabelConfidence float64 `json:"label_confidence,omitempty"`

	// Sources are the providers that contributed, strongest first.
	Sources []string `json:"sources,omitempty"`

	// Actionable is the coarse question — can this be acted on right now. The finer
	// capabilities follow, because a HUD showing "why not" needs them.
	Actionable bool `json:"actionable"`
	Targetable bool `json:"targetable"`
	Focusable  bool `json:"focusable"`
	Invokable  bool `json:"invokable"`
	Enabled    bool `json:"enabled"`
	Visible    bool `json:"visible"`

	// StableCycles is the current UNBROKEN run of cycles this element has appeared in.
	// Stable says it has run long enough to be trusted as a thing rather than a flicker.
	StableCycles int  `json:"stable_cycles"`
	Stable       bool `json:"stable"`
	// AgeMS is how long the current run has lasted.
	AgeMS int64 `json:"age_ms"`
}

// WorldResponse is the believed world, bounded.
type WorldResponse struct {
	// Observed is when the snapshot's observation began; FreshnessMS is how old that
	// makes it now. A HUD needs the second to answer "is Marco still looking?".
	Observed    time.Time `json:"observed"`
	FreshnessMS int64     `json:"freshness_ms"`

	// App is the foreground application's stable key — the same normalised executable
	// name ProviderStatus already reports, not a window title.
	App string `json:"app,omitempty"`

	Entities []WorldEntity `json:"entities,omitempty"`
	// Total is how many the world held before the bound, so a truncated list is never
	// mistaken for the whole picture.
	Total     int  `json:"total"`
	Truncated bool `json:"truncated,omitempty"`
	// Believed is false when the Director holds no world yet — distinct from a world
	// with nothing in it, which is a real and different observation.
	Believed bool `json:"believed"`
}

// ── the playbill ──────────────────────────────────────────────────────────────

// PlaybillPayload asks for the current read-only account.
type PlaybillPayload struct {
	// Cursor is the highest observation sequence the caller already rendered, so the
	// reply carries only what is new. Zero asks for whatever is retained.
	Cursor uint64 `json:"cursor,omitempty"`
	// Diagnostics asks for the developer-level evidence UNDER what the account says.
	//
	// Opt-in, deliberately. Watch never needs it, and assembling it on every poll would
	// make the act of watching cost something — which is precisely what an observability
	// surface must not do to the system it observes.
	Diagnostics bool `json:"diagnostics,omitempty"`
}

// PlaybillResponse carries the account.
//
// The playbill's OWN type, forwarded whole. The service does not reshape it, summarise
// it or add to it: a reshaping here would be a second opinion about what the Director
// believes, sitting between the Director and every surface that renders it.
type PlaybillResponse struct {
	View playbill.View `json:"view"`
}

// ── perception events ─────────────────────────────────────────────────────────

// EventsPayload asks for events after a cursor.
type EventsPayload struct {
	Cursor uint64 `json:"cursor"`
	Limit  int    `json:"limit,omitempty"`
}

// EventsResponse carries the log slice and the numbers that make loss detectable.
//
// A client distinguishes three cases without guessing:
//
//   - NOTHING HAPPENED — Epoch unchanged, Newest == cursor, no events.
//   - EVENTS WERE MISSED — Oldest > cursor+1: the ring rolled past the cursor.
//   - THE SERVICE RESTARTED — Epoch differs from the one last seen. Sequence numbers
//     begin again at 1, and without an epoch that is indistinguishable from a rollover
//     to a client that had reached a high cursor.
type EventsResponse struct {
	// Epoch identifies this service instance. Opaque; only equality is meaningful.
	Epoch string `json:"epoch"`

	Events []timeline.Event `json:"events,omitempty"`

	// Newest is the highest sequence issued; Oldest is the lowest still retained.
	Newest uint64 `json:"newest"`
	Oldest uint64 `json:"oldest"`
}

// ── live observation findings ─────────────────────────────────────────────────

// LiveAnalysisPayload asks what a session has learned so far.
type LiveAnalysisPayload struct {
	// SessionID names a session. Empty means the active one.
	SessionID string `json:"session_id,omitempty"`
}

// LiveAnalysisResponse is the bounded current analysis.
//
// The summary is the observer's own `observe.LiveSummary`, forwarded unmodified. The
// service transports findings and has no opinion about them: it does not rank, threshold
// or re-classify, because a second opinion here would be a second analyser.
type LiveAnalysisResponse struct {
	// Available is false when no session has ever run. Distinct from a session with no
	// findings, which is a real and different observation — a client that showed empty
	// sections for the first would be inventing a result.
	Available bool `json:"available"`
	// Active says a session is running NOW. A completed session's summary is still
	// worth showing and must not be presented as live.
	Active bool `json:"active"`
	// ServiceGeneration identifies the service instance, so a client can tell a
	// restart from a cursor rollover.
	ServiceGeneration string `json:"service_generation,omitempty"`

	Summary observe.LiveSummary `json:"summary"`
}

// ObservationEventsPayload asks for a session's findings after a cursor.
type ObservationEventsPayload struct {
	SessionID string `json:"session_id,omitempty"`
	// After is the highest sequence the client already has.
	After uint64 `json:"after,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// ObservationEventsResponse carries the slice and the numbers that make loss detectable.
//
// A client tells four cases apart without guessing:
//
//   - NO NEW EVENTS — same generations, Newest == After, empty slice.
//   - EVENTS RETURNED — the ordinary case.
//   - EVENTS WERE MISSED — Gap is true: retention rolled past the cursor.
//   - THE SERVICE RESTARTED, or THE SESSION CHANGED — a generation differs.
//
// Gap is computed HERE rather than left to the client, because the server knows both the
// cursor it was given and what it still retains, and two clients doing that arithmetic
// differently is exactly the kind of divergence this protocol exists to prevent.
type ObservationEventsResponse struct {
	Available bool `json:"available"`
	Active    bool `json:"active"`

	// ServiceGeneration changes when the service restarts; SessionID changes when a
	// different session is being observed. Either invalidates a cursor.
	ServiceGeneration string            `json:"service_generation,omitempty"`
	SessionID         observe.SessionID `json:"session_id,omitempty"`

	Events []observe.LiveEvent `json:"events,omitempty"`
	Oldest uint64              `json:"oldest"`
	Newest uint64              `json:"newest"`
	// Gap says the requested cursor fell behind what is still retained, so events were
	// lost. The client's remedy is to refetch the analysis snapshot, never to infer.
	Gap bool `json:"gap,omitempty"`
}

// ── passive observation ───────────────────────────────────────────────────────

// ObservePayload starts a bounded passive observation session.
type ObservePayload struct {
	// Target names the window to watch. Exactly one primary selector.
	Target windowref.Selector `json:"target"`
	// Duration and Interval are validated against observe.Bounds before anything runs.
	Duration time.Duration `json:"duration,omitempty"`
	Interval time.Duration `json:"interval,omitempty"`
}

// ObserveQuery reads one session, or lists them.
type ObserveQuery struct {
	// ID names a session. Empty means the active one, or the most recent.
	ID string `json:"id,omitempty"`
	// List asks for every remembered session instead of one.
	List bool `json:"list,omitempty"`
	// Cancel stops the named session.
	Cancel bool `json:"cancel,omitempty"`
	// Insights asks for the full evidence and hypothesis report.
	Insights bool `json:"insights,omitempty"`
	// Answer records the user's response to a question the session asked.
	//
	// Carried on the existing observation query rather than on a new request kind: the
	// answer is about one session's hypothesis, it is routed by the same id, and a separate
	// protocol verb would be a second path to keep in agreement with this one forever.
	Answer *ObserveAnswer `json:"answer,omitempty"`
	// Rehearse asks Marco to attempt one authorized step. See ObserveRehearse.
	Rehearse *ObserveRehearse `json:"rehearse,omitempty"`
	// Ambient turns continuous watching on or off, or asks what it is doing.
	// See ObserveAmbient.
	Ambient *ObserveAmbient `json:"ambient,omitempty"`
	// Name answers a naming question with the word the user chose. See ObserveScreenName.
	//
	// A SEPARATE field rather than a text member on ObserveAnswer, deliberately. Answers to
	// every other question are three closed words; adding free text beside them would make
	// the proposal system a general place to persist arbitrary strings, and the point of
	// this one exception is that it is an exception.
	Name *ObserveScreenName `json:"name,omitempty"`
	// Learn starts, reads, finishes or cancels one acquisition session: the person
	// demonstrates and Marco acquires. See ObserveLearn.
	//
	// THE ONE ACQUISITION REQUEST. There used to be two — this one and `Teach` — and they
	// were not peers: the control surface's type was a facade that translated its own verbs
	// into the other's, so a single act was described twice, in two vocabularies, one of them
	// spending the word the product reserves for the opposite direction of travel (Marco
	// guiding a person). Several entrances send this one and differ only in how they
	// configure it.
	//
	// Carried on the observation query for the same reason Answer and Name are: acquisition
	// is bounded observation with a conversation around it, it runs through the one
	// observation registry, and a separate protocol verb would be a second path to keep in
	// agreement with this one forever. It adds no authority — see internal/director/learn.
	Learn *ObserveLearn `json:"learn,omitempty"`
	// Revise changes or withdraws an answer the user has already given.
	//
	// A SEPARATE field from Answer, deliberately, because they are separate operations:
	// answering stays one-shot so a double submit cannot overwrite what somebody said, and
	// changing your mind is something a caller has to mean. See ObserveRevise.
	Revise *ObserveRevise `json:"revise,omitempty"`
	// Knows reads, or corrects, what a person has explicitly told Marco.
	//
	// Carried on this query for the same reason every other verb here is: durable semantic
	// knowledge is what the observation path writes, and a second protocol verb would be a
	// second door onto the same records to keep in agreement forever. See ObserveKnows.
	Knows *ObserveKnows `json:"knows,omitempty"`
	// Point asks Marco where, on the screen right now, the thing it is referring to is.
	//
	// Carried here for the reason all of the above are: it reads one session's account, it is
	// routed by the same id, and the alternative is a second door onto the same records. It
	// WRITES nothing — pointing is a reading of what has already been decided.
	Point *ObservePoint `json:"point,omitempty"`
	// Reach asks how Marco would get from where the person is to a learned outcome.
	//
	// A READ over the durable topology and the goal records, and structurally incapable of
	// acting: the answer is a plan or an honest refusal, and execution still goes through a
	// saved play's own resolve → authorize → run. A known goal implies no action authority.
	Reach *ObserveReach `json:"reach,omitempty"`
	// Showing asks which remembered place is in front RIGHT NOW. See ObserveShowing.
	//
	// Carried on the observation query for the reason Point and Reach are: it is a read over
	// what the observation path already sees and already remembers, it travels through the one
	// observation registry, and a separate protocol verb would be a second door onto the same
	// records to keep in agreement with this one forever. It WRITES nothing and grants nothing
	// — recognising a screen is perception, and asking where you are must never be a way of
	// doing something.
	Showing *ObserveShowing `json:"showing,omitempty"`
	// Learning asks what durable knowledge has changed since a cursor. A read; see
	// ObserveLearning.
	Learning *ObserveLearning `json:"learning,omitempty"`
	// Perform CARRIES OUT a learned outcome, and is the only field here that can.
	//
	// Separated from Reach on purpose: planning and doing are different requests, and a
	// surface that could act by accident while asking a question is the failure ADR-029
	// exists to prevent. The Audience naming a behaviour is the authority event; the bounds
	// on real input are the same ones a rehearsal spends.
	Perform *PerformQuery `json:"perform,omitempty"`
	// Experiment asks what ONE thing Marco would most like to try, and why.
	//
	// A READ, and it chooses nothing durable. See ExperimentView for why exactly one.
	Experiment *ObserveExperiment `json:"experiment,omitempty"`
	// Test TRIES one connection Marco learned by watching, and is the second field here that
	// can drive real input.
	//
	// Separated from Perform because they are different acts with different meanings to a
	// person. Perform accomplishes what somebody ASKED FOR and leaves them where they asked to
	// be. Test proves a connection Marco believes in, needs a specific SOURCE it may have to
	// walk to first, and gives the desktop back afterwards — because nobody asked to be moved.
	//
	// A surface that labelled both "Try it" would be hiding the difference between "go there"
	// and "check what you learned", which is the difference between doing somebody a favour
	// and borrowing their computer.
	Test *TestQuery `json:"test,omitempty"`
	// Map asks what Marco knows about where somebody is — the graph, not the events.
	//
	// A READ. It establishes nothing, plans nothing new and cannot act; see MapView.
	Map *ObserveMap `json:"map,omitempty"`
}

// ObserveMap asks for Marco's map of the interface around where somebody is.
type ObserveMap struct {
	// Application scopes the map. Empty means whichever ambient watching last saw.
	Application string `json:"application,omitempty"`
}

// MapView is Marco's semantic map, as a person watches it being built.
//
// # The primary object of Observe
//
// Not an event feed. The dogfood finding this answers is that a person could not tell what Marco
// thought the screen was called, what it had just discovered, how Places related, or what any of
// it would let Marco do — because the surface showed a stream of process vocabulary and not the
// mental model underneath it.
//
// Four questions, in this order: where am I, what does Marco know around here, what did it just
// find, what can it reach.
//
// # Words, and never identifiers
//
// Every field a person reads is a word from `observe.PlaceWords` — the one naming function — so
// the map cannot call a screen something the rest of the product does not. Subjects travel because
// a surface has to address a place; nothing renders them.
type MapView struct {
	Application string `json:"application,omitempty"`
	// Here is where fresh perception says the person is, and HereWords what to call it.
	//
	// EMPTY IS A REAL ANSWER and means Marco does not recognise the screen in front. It is
	// never filled from memory: a remembered Place is not a visible Place, and a marker that
	// moved because Marco recalled something would say where somebody once was.
	Here      string `json:"here,omitempty"`
	HereWords string `json:"here_words,omitempty"`
	// HereKnown says perception resolved a durable Place. False with an application present
	// is the ordinary unknown-screen state.
	HereKnown bool `json:"here_known,omitempty"`
	// Places and Edges are the NEIGHBOURHOOD — what Marco knows within one step of here —
	// rather than the whole graph, because "what do you know around here" is a question
	// somebody can act on.
	Places []MapPlace `json:"places,omitempty"`
	Edges  []MapEdge  `json:"edges,omitempty"`
	// KnownPlaces and KnownEdges are the whole graph, as counts. So somebody watching the map
	// grow can see it grow without being shown all of it.
	KnownPlaces int `json:"known_places,omitempty"`
	KnownEdges  int `json:"known_edges,omitempty"`
	// Reachable is where the canonical planner says Marco could get to from here.
	Reachable []MapReach `json:"reachable,omitempty"`
}

// MapPlace is one screen on the map.
type MapPlace struct {
	// Subject addresses it and is never rendered.
	Subject string `json:"subject"`
	// Words is what to call it, from the one naming function.
	Words string `json:"words"`
	// Describes is what it is made of, which is how a person tells two unnamed screens
	// apart. Secondary to Words, never instead of it.
	Describes string `json:"describes,omitempty"`
	// Here marks the one the person is standing on.
	Here bool `json:"here,omitempty"`
}

// MapEdge is one connection Marco has learned.
//
// FROM, ACTION, TO — never "origin" and "arrival", which are the machinery's words and which a
// person reading the control centre could not make sense of.
type MapEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	FromWords string `json:"from_words"`
	ToWords   string `json:"to_words"`
	// Observations is how often this connection has been seen. A count rather than a
	// confidence: the map says what Marco has, and what that is worth is the planner's
	// business.
	Observations int `json:"observations,omitempty"`
}

// MapReach is somewhere Marco believes it could get to from here.
//
// Present because the CANONICAL planner produced a route, never because two places look connected.
// Verified says every step is one Marco has walked and checked; false is ordinary and means "I
// know a way and I have not earned every step of it yet".
type MapReach struct {
	Subject  string `json:"subject"`
	Words    string `json:"words"`
	Steps    int    `json:"steps"`
	Verified bool   `json:"verified,omitempty"`
}

// ObserveExperiment asks what Marco is focused on.
type ObserveExperiment struct {
	// Application scopes the question. Empty means whichever ambient watching last saw.
	Application string `json:"application,omitempty"`
}

// EdgeRef names one connection between two remembered screens.
//
// Ids, because it is an ADDRESS: the surface hands it straight back to ask for that exact
// connection to be tried. What a person reads is never this — see ExperimentView's words.
type EdgeRef struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ExperimentView is the ONE thing Marco would like to try, said in a sentence.
//
// # Why exactly one
//
// Because the dogfood failure it answers was not that Marco knew too little; it was that a person
// could not tell what it was focused on. A surface offering nine things Marco might try is the
// same undifferentiated stream in a different shape. One experiment, with its source, its action,
// its expected destination and its reason, is a thought somebody can follow and decide about.
//
// # Words AND ids, and they are not the same field
//
// `Edge` addresses the connection and is never rendered. `FromWords`, `ToWords` and `Action` are
// what a person reads, through the one place-naming function every other surface uses. A view
// carrying only ids would push naming into the client, which is how two surfaces come to call one
// screen two things.
type ExperimentView struct {
	Application string `json:"application,omitempty"`
	// Ready says there IS something worth trying. False with no refusal is the ordinary
	// early state: Marco has not watched anybody do anything twice yet.
	Ready bool    `json:"ready,omitempty"`
	Edge  EdgeRef `json:"edge,omitzero"`
	// FromWords, Action and ToWords are the hypothesis, in the order it reads: from HERE,
	// doing THIS, you arrive THERE.
	FromWords string `json:"from_words,omitempty"`
	Action    string `json:"action,omitempty"`
	ToWords   string `json:"to_words,omitempty"`
	// Why is Marco's one reason, from evidence and never from narrative.
	Why string `json:"why,omitempty"`
	// Seen and Sessions are the evidence behind Why, for a surface that wants to show it.
	Seen     int `json:"seen,omitempty"`
	Sessions int `json:"sessions,omitempty"`
}

// TestQuery asks Marco to try one connection it learned by watching.
type TestQuery struct {
	Application string `json:"application"`
	// From and To are the connection, by durable subject. The surface got them from
	// ExperimentView and hands them back unchanged — it does not choose an experiment by
	// describing one, because a description is not an identity.
	From string `json:"from"`
	To   string `json:"to"`
}

// RestoreView is what became of the desktop the person was using.
//
// # Why this is a value and not a silence
//
// Because "I put your computer back" and "I couldn't" are different facts about somebody's
// afternoon, and the second one is the one they need. A restoration that failed quietly leaves a
// person standing in Marco's experiment believing they were returned.
type RestoreView struct {
	// Attempted is false when there was nothing to restore — no window context, or nothing
	// in the foreground when the attempt began.
	Attempted bool `json:"attempted,omitempty"`
	// Application is the window Marco tried to bring back, for the sentence.
	Application string `json:"application,omitempty"`
	// Restored says a check confirmed it came forward. Never assumed from the call
	// succeeding.
	Restored bool `json:"restored,omitempty"`
	// Say is why not, when it did not.
	Say string `json:"say,omitempty"`
}

// ObserveShowing asks which remembered place is showing RIGHT NOW.
//
// # Why a client may ask this at all
//
// Because every learned play's generated Marco opens with `do Screen's Showing with "<place>"`,
// and the process that runs a play is `marco`, which cannot see. A learned play with intact
// provenance is delegated to the Director and never meets that problem; an EDITED one is not
// delegated — editing it makes it an ordinary play, which is the whole of the authority policy —
// so it takes the local runner and refused at its own first line.
//
// # Why it is a fresh LOOK and not a lookup
//
// The Director answers it out of `freshPlace`, the same body `PerformGoal` plans from, for the
// reason that body exists: answering from the newest FINISHED session tells somebody "you're
// already there" about a screen they have left. A guard answered from history is a guard that
// passes on the wrong screen, which is worse than no guard at all.
//
// # What it may never become
//
// A guess. The reply is a positive identification or a named refusal, and there is no field on it
// a caller can fall back to when it wanted a yes. See [[ADR-031-the-user-names-the-stage]],
// Decision 4: a Marco that cannot establish where it is "does not skip the guard, assume ok, fall
// back to OCR text matching, or degrade into blind replay."
type ObserveShowing struct {
	// Application scopes the look — the same normalised executable key everything else on this
	// query uses.
	//
	// Empty is a REFUSAL rather than "whatever happens to be in front". A play's entry
	// condition is about the application the play is in; answering it about a different one
	// would be answering a question nobody asked, and a screen guard that can be satisfied by
	// the wrong application is worse than no screen guard.
	Application string `json:"application,omitempty"`
}

// ShowingView is the answer: one durable subject, or one honest reason there is none.
type ShowingView struct {
	// Application is what was looked in, echoed back so a caller can tell an answer to its own
	// question from an answer to somebody else's.
	Application string `json:"application,omitempty"`
	// Outcome is `screenhost.Outcome`'s closed vocabulary — recognised, ambiguous,
	// unobservable, unavailable, unrecognised — as the bare string it already is.
	//
	// # Why the type is not imported here, and why that costs nothing
	//
	// The Director may not depend on platform or engine code: platform implementations are
	// wired in at a composition root, never imported by `internal/director`. That rule has a
	// test, and it is right — a wire protocol that pulled in a host package would make the
	// Director's own layering depend on what the client happens to be made of.
	//
	// So the string crosses, and BOTH ENDS are composition roots that legitimately hold the
	// type: `cmd/director` writes `string(screenhost.Unobservable)` and `cmd/marco` reads
	// `screenhost.Outcome(view.Outcome)`. Those are type conversions on a named string type,
	// not a translation table — no constant is declared twice, so there is still exactly one
	// vocabulary, and the way two vocabularies fail ("I could not look" arriving as "I looked
	// and it matched") has no place to occur.
	//
	// The residual risk is a word that is not in the vocabulary at all, from an older or newer
	// build. That is the client's to handle, and it handles it as a refusal — see
	// `liveScreens.CurrentSubject`.
	Outcome string `json:"outcome"`
	// Subject is the durable subject id, and is set ONLY when Outcome is `recognised`.
	//
	// A reader must still check the outcome rather than the emptiness of this field: an id
	// beside any other outcome is a bug, and the client treats it as one rather than as a
	// match.
	Subject string `json:"subject,omitempty"`
	// Why is the reason a look identified nothing, for diagnostics. It never changes what a
	// play is told — a play gets ok or failed, and nothing else.
	Why string `json:"why,omitempty"`
}

// ObserveReach asks for the plan toward one learned outcome, or the list of them.
type ObserveReach struct {
	// Name is the outcome, in the words it was learned under. Empty lists every learned
	// outcome for the application.
	Name string `json:"name,omitempty"`
	// Application scopes the answer. Empty means the most recently observed one.
	Application string `json:"application,omitempty"`
	// From is "if I were standing HERE, what would you do" — a subject id, empty for
	// wherever Marco last saw the person.
	//
	// # Why a diagnostic needs to be able to ask about somewhere else
	//
	// The route depends on where you are, so the interesting question is usually about a
	// place you are not: would it still take the long way from the Home page? Without this
	// the only answerable question is about the one screen a session happened to end on, and
	// on a fresh Director there is no such screen at all.
	//
	// It drives nothing and asserts nothing about where anybody actually is — the plan says
	// which source it was computed from.
	From string `json:"from,omitempty"`
}

// ReachView is what Marco knows about reaching learned outcomes.
type ReachView struct {
	Application string `json:"application,omitempty"`
	// Outcomes is the list, when no name was asked about.
	Outcomes []OutcomeView `json:"outcomes,omitempty"`
	// Name and Subject are the outcome asked about.
	Name    string `json:"name,omitempty"`
	Subject string `json:"subject,omitempty"`
	// Current is where Marco last saw the person standing, and AsOf which session that
	// reading came from. Honesty about staleness: the plan is FROM this place, and if the
	// person has moved since, the answer is about where they were.
	Current string `json:"current,omitempty"`
	AsOf    string `json:"as_of,omitempty"`
	// Satisfied says the person was already standing on the outcome.
	Satisfied bool `json:"satisfied,omitempty"`
	// Steps is the chain of VERIFIED edges, when one exists.
	Steps []ReachStepView `json:"steps,omitempty"`
	// Refusal is why there is no plan, in observe.PlanRefusal's closed vocabulary.
	Refusal string `json:"refusal,omitempty"`
	// KnownUnverified says a chain exists on observed evidence that has not all been
	// rehearsed — "I know a way, and I haven't earned every step of it yet."
	KnownUnverified bool `json:"known_unverified,omitempty"`
	// Candidates is what the words could equally have meant, when they meant more than one
	// thing. The diagnostic answer to the question `perform` refuses to guess at.
	Candidates []OutcomeView `json:"candidates,omitempty"`
	// Why is what made this route better than the others, in the evidence classes the
	// planner compared — never a score.
	//
	// A person about to watch Marco take the long way is owed the reason, and "cost 3" is not
	// one. These are the planner's own words for its own comparison; see observe.PathRank.
	Why []string `json:"why,omitempty"`
	// Actions, Contradicted and Verified are the same comparison in fields, for a client that
	// wants to render it rather than print it.
	Actions      int  `json:"actions,omitempty"`
	Contradicted int  `json:"contradicted,omitempty"`
	Verified     bool `json:"verified,omitempty"`
	// Say is the Normal-mode sentence.
	Say string `json:"say,omitempty"`
}

// OutcomeView is one learned outcome.
type OutcomeView struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	// Application is where this outcome lives. Empty in a single-application listing, where
	// the view already says which one, and always present in a candidate list — the whole
	// point of which is that the outcomes are in DIFFERENT applications.
	Application    string `json:"application,omitempty"`
	Demonstrations int    `json:"demonstrations,omitempty"`
}

// ReachStepView is one edge of a plan.
type ReachStepView struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ObservePoint asks for the desktop rectangles of what Marco currently means.
//
// No inputs beyond the role filter, deliberately. A caller does not get to say WHICH subject or
// WHERE it is — both of those are Director's, and a protocol that accepted a rectangle from
// outside would be a protocol through which a surface could make Marco appear to mean anything.
type ObservePoint struct {
	// Role narrows to one kind of reference, empty for whatever Marco is currently referring
	// to. A closed vocabulary — see observe.ReferentRole.
	Role string `json:"role,omitempty"`
	// Subject names a DURABLE remembered subject to point at — "show me what this judgement
	// refers to". Empty for whatever Marco is currently referring to.
	//
	// It is an identity to look up, never geometry. What comes back is resolved from what a
	// live session currently RECOGNISES as that subject; the stored envelope is not consulted,
	// because it can be years old and pointing with it would draw a box where something used
	// to be.
	Subject string `json:"subject,omitempty"`
	// Question names ONE proposal to point at the subject of — "show me what this question is
	// about". Empty for whatever Marco is currently referring to.
	//
	// Separate from Subject and not interchangeable with it. A question carries its own
	// subject, and the whole value of a button underneath a question is that it points at THAT
	// subject and refuses when it cannot — rather than falling through to whatever else Marco
	// happens to be referring to, which is a different claim wearing the same box.
	Question string `json:"question,omitempty"`
}

// ObserveKnows reads or corrects durable, user-supplied semantic judgements.
//
// # Why a judgement is reached by its SUBJECT here
//
// Every other correction path reaches an answer through the question that produced it, which works
// only while some session still holds that question. A subject Marco can no longer recognise has
// no live question and never will again — and its answer would then be uncorrectable forever. This
// verb reaches the judgement by the thing it is about, so a person can always withdraw something
// they said.
//
// It creates no authority. Reading what you told Marco, and changing it, are both claims about
// what something IS; neither is permission to do anything.
type ObserveKnows struct {
	// Subject and Kind name one judgement. Empty Subject means "list what is known".
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
	// Response is the new answer. Ignored when Withdraw is set.
	Response string `json:"response,omitempty"`
	// Withdraw leaves no active judgement, durably.
	Withdraw bool `json:"withdraw,omitempty"`
}

// ObserveRevise changes or withdraws a settled answer.
//
// It reaches the same durable path an answer takes and creates no authority of any kind: a
// semantic correction is a claim about what something IS, never permission to do anything.
type ObserveRevise struct {
	ProposalID string `json:"proposal_id"`
	// Response is the new answer. Ignored when Withdraw is set.
	Response string `json:"response,omitempty"`
	// Withdraw leaves no active judgement, durably — so a restart does not resurrect the
	// answer that was withdrawn.
	Withdraw bool `json:"withdraw,omitempty"`
}

// ObserveScreenName is what the user calls one screen, in reply to one naming question.
//
// The proposal id is REQUIRED and is the only thing that binds the name to a screen. The screen
// itself is on the proposal — this request never says which subject, because the user answering
// is looking at their own memory of playing rather than at Marco's record, and is quite likely
// somewhere else entirely by now.
type ObserveScreenName struct {
	ProposalID string `json:"proposal_id"`
	// Name is RAW user text and is validated at the boundary that decodes this request, by
	// observe.UserSuppliedScreenName. It is the only free-form string in this protocol that
	// is allowed to reach durable memory, and it gets there by being typed.
	Name string `json:"name"`
}

// ObserveAnswer is a reply to one proposal.
//
// The proposal id is REQUIRED and is the only thing that binds the answer to a question. It is
// never "the current hypothesis": by the time somebody answers, the screen has usually changed
// and the state that prompted the question may have been renumbered or may be gone.
type ObserveAnswer struct {
	ProposalID string `json:"proposal_id"`
	// Response is one of the closed observe.UserResponse values. Validated on arrival —
	// an unrecognised answer is refused rather than coerced into a default, because the
	// default would be either "yes" or "no" and both are wrong.
	Response string `json:"response"`
}

// ObserveStarted is the reply to a start request.
type ObserveStarted struct {
	ID       string        `json:"id"`
	Target   string        `json:"target"`
	Duration time.Duration `json:"duration"`
	Interval time.Duration `json:"interval"`
	State    string        `json:"state"`
}

// ObserveRehearse asks Marco to attempt ONE authorized step.
//
// A typed request rather than anything tunnelled through generic execution, because the authority
// is narrower than ordinary execution in kind: it exists only because a person answered one
// specific question with yes, it is single-use, and it is scoped to one route on one screen of one
// application. Reaching it through the same door as `director do` would make that invisible.
type ObserveRehearse struct {
	// ID names the session whose question was answered. Empty means the most recent.
	ID string `json:"id,omitempty"`
	// Step is which step of the authorized plan to attempt. 0 or 1 means the first.
	Step int `json:"step,omitempty"`
	// Live asks for REAL input. Absent, the rehearsal runs against a recording host and
	// nothing reaches the computer.
	//
	// Opt-in at the outermost layer on purpose: the decision to touch a desktop should be one
	// a person made in a sentence they can read back, not a default that arrived with a
	// release.
	Live bool `json:"live,omitempty"`
}

// RehearsalView is what one whole attempt produced, in the outward shape.
//
// No keys, no titles, no text, no screenshots — the same rule every other record follows, and
// nothing here could break it because a RehearsalResult never held any of that.
type RehearsalView struct {
	// Attempted says input was emitted. False means Marco refused BEFORE acting, and
	// Refusal says why — a distinction that matters more than any other field here.
	Attempted bool   `json:"attempted"`
	Refusal   string `json:"refusal,omitempty"`
	Detail    string `json:"detail,omitempty"`
	// Live says a real host was installed.
	Live bool `json:"live"`
	// Completed is the one thing that says the whole learned route survived.
	Completed bool `json:"completed"`

	Application string `json:"application,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Terminal    string `json:"terminal,omitempty"`

	Steps      []RehearsalStepView `json:"steps,omitempty"`
	StepsTaken int                 `json:"steps_taken,omitempty"`
	Planned    int                 `json:"planned,omitempty"`
	Inputs     int                 `json:"inputs,omitempty"`
	DurationMS int64               `json:"duration_ms,omitempty"`
	// Lines is the rendered form, so every surface says the same thing.
	Lines []string `json:"lines,omitempty"`
}

// RehearsalStepView is one attempted step.
type RehearsalStepView struct {
	Step         int      `json:"step"`
	Intents      []string `json:"intents,omitempty"`
	Expected     string   `json:"expected,omitempty"`
	Verification string   `json:"verification,omitempty"`
	Outcome      string   `json:"outcome,omitempty"`
	Observed     string   `json:"observed,omitempty"`
	Window       string   `json:"window,omitempty"`
	Settle       string   `json:"settle,omitempty"`
	Cancelled    bool     `json:"cancelled,omitempty"`
	Emitted      []string `json:"emitted,omitempty"`
	// Program is the legal Marco that was compiled and run.
	//
	// Safe to show and worth showing: it is the artefact ADR-005 makes the only route to an
	// effect, and it names navigation MEANINGS rather than keys.
	Program string `json:"program,omitempty"`
	// Detail is the host's own sentence when this step failed: which target, which
	// provider, what refused. Empty unless something went wrong. Diagnostic only.
	Detail string `json:"detail,omitempty"`
}

// LearnedQuery asks what Marco has learned well enough to write down.
//
// A read. It produces text and nothing else — no file, no registration, no resolution entry, and
// nothing that can run. See [[ADR-027-what-marco-learned-becomes-marco]].
// LearnedStep is one edge of an ordered walk, by durable subject.
type LearnedStep struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// LearnedQuery asks what Marco has learned, and optionally acts on it.
type LearnedQuery struct {
	// Application scopes the answer. Empty means the most recently observed one.
	Application string `json:"application,omitempty"`
	// Name and Verb are what the user chose to call the play and what it does.
	//
	// Two names because a play is a sentence: `do Volume's Mute...`. Both are required to
	// save, and neither is guessed — Director has no business naming something from a screen's
	// text or an application's.
	Name string `json:"name,omitempty"`
	Verb string `json:"verb,omitempty"`
	// Phrase is what the AUDIENCE called this behaviour, and what they will ask for.
	//
	// Name and Verb are the play.s Marco identity — `do MouseSettings.s Open` — and the slug
	// was taken from Name, so a person who asked for "Open Mouse Settings" got a route called
	// `mousesettings` and their own words resolved to nothing but a did-you-mean.
	//
	// Empty falls back to Name, which is every other caller.
	Phrase string `json:"phrase,omitempty"`
	// From and To name which route to act on, when more than one could be written down.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Walk is the ordered route this play should represent, when it is more than one edge.
	//
	// A demonstration of A to B to C is kept as two reusable edges, and each lowers to its
	// own play — which is right for reuse and wrong for the behaviour the Audience asked for.
	// Saving the terminal edge alone produced a play that began at B and refused its own
	// entry condition when asked from A.
	//
	// Empty means the single From/To above, which is every other caller.
	Walk []LearnedStep `json:"walk,omitempty"`
	// Save writes the named play where it can be read and edited, and NOT where the resolver
	// looks. Register moves it somewhere a later request can find it.
	//
	// Two flags rather than one because they are two permissions. Saving is "keep this";
	// registering is "and let me ask for it".
	Save     bool `json:"save,omitempty"`
	Register bool `json:"register,omitempty"`
	// Forget removes a registered play and its provenance. It forgets the PLAY, never what
	// Director observed.
	Forget bool `json:"forget,omitempty"`
}

// LearnedView is every demonstrated route, and the Marco for the ones that earned it.
type LearnedView struct {
	Application string            `json:"application,omitempty"`
	Plays       []LearnedPlayView `json:"plays,omitempty"`
	// Saved is what a save, register or forget did, when the request asked for one.
	Saved *LearnedSaved `json:"saved,omitempty"`
}

// LearnedPlayView is one route: whether it may be written down, and the play if it may.
type LearnedPlayView struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Eligible says a completed rehearsal still supports this, right now.
	Eligible bool `json:"eligible"`
	// Refusals is why not, in the closed vocabulary.
	Refusals []string `json:"refusals,omitempty"`
	Detail   string   `json:"detail,omitempty"`
	// Unnamed is the durable subjects this play cannot say the name of, in the order the
	// judgement wants them: source first, then destination.
	//
	// Carried so a caller following the lifecycle can tell WHICH screen is blocking without
	// re-deriving the judgement. Subject ids, and therefore developer-facing — a surface that
	// shows these to a person is showing them Director's backstage.
	Unnamed []string `json:"unnamed,omitempty"`
	// Source is ordinary Marco, compiled before it was shown to anybody.
	//
	// Inert. It is not saved, not registered, and not reachable by a later request — writing a
	// play down and being allowed to perform it are different things, and this milestone is
	// only the first.
	Source string `json:"source,omitempty"`
	// Lines is the rendered summary, so every surface says the same thing.
	Lines []string `json:"lines,omitempty"`
}

// LearnedSaved is what a save, register or forget did.
type LearnedSaved struct {
	// Name is the slug the play was stored under.
	Name string `json:"name,omitempty"`
	// Saved says the source and its provenance are on disk.
	Saved bool `json:"saved,omitempty"`
	// Registered says the resolver can now find it. Deliberately separate from Saved: a saved
	// play lives where discovery does not look, and moving it is a second decision.
	Registered bool `json:"registered,omitempty"`
	Forgotten  bool `json:"forgotten,omitempty"`
	// Source is the play as it was written.
	Source string `json:"source,omitempty"`
	// Lines is what a person is told.
	Lines []string `json:"lines,omitempty"`
}

// ObserveLearn is what a control surface can ask of the Learn lifecycle.
//
// # The whole vocabulary a person needs
//
// Name it, start, do the thing, stop, see what was understood, try it. Nothing on this type names
// a session, a proposal, a candidate, a grant, a window or a phase — those are implementation
// concepts, and a surface that had to know them would be a terminal with rounded corners.
//
// Exactly one verb per request. Two would need an order, and an order is a second lifecycle.
type ObserveLearn struct {
	// Start begins a session for Name. The person's own words, and the only string here.
	Start bool   `json:"start,omitempty"`
	Name  string `json:"name,omitempty"`
	// Recent asks Marco to learn what it has just WATCHED, rather than to watch something
	// new. The retrospective half of Learn.
	//
	// # Why it is a field here and not a request of its own
	//
	// Because it is the same act. Somebody names a behaviour and asks for it to be
	// remembered; the only difference is whether the demonstration is about to happen or has
	// already happened. Giving it its own request type would give it its own router, its own
	// name validation, its own view and eventually its own idea of what a Learn is — and the
	// whole point of retrospective Learn is that everything after the evidence is the path
	// that already exists.
	//
	// Name is required with it, and every other field means what it always did: Target
	// narrows which application's recent evidence to read, Actor and Verb still override the
	// derived play name.
	//
	// It never starts a session. A `--recent` with nothing usable behind it says so and
	// stops; quietly falling back to watching would answer a question about the past by
	// starting to record the future, which is not something to do to somebody by
	// implication. See ADR-094.
	Recent bool `json:"recent,omitempty"`
	// Stop is the person saying their demonstration is over.
	//
	// It KEEPS everything captured and lets the ordinary pipeline finish — see
	// learn.Coordinator.Finish. It is not Cancel and must never be routed to it.
	Stop bool `json:"stop,omitempty"`
	// Try answers the rehearsal question with a yes.
	//
	// It grants authority THROUGH the existing ledger, exactly as answering the question on
	// the command line does. There is no surface-only path to input, and a button is not a
	// reason to have one.
	Try bool `json:"try,omitempty"`
	// Called answers a "what do you call this screen?" question with the person's own word.
	//
	// The ONE question in this system answered with free text rather than one of three
	// closed words, and it stays an exception: it routes to the same ObserveScreenName
	// request the command line uses, where the raw string becomes a validated ScreenName at
	// the request boundary.
	Called string `json:"called,omitempty"`
	// Skip declines a question without answering it.
	//
	// NOT a no. "I would rather not say" is a different fact from "that is wrong", and
	// collapsing them would record a judgement the person never made.
	Skip bool `json:"skip,omitempty"`
	// Cancel abandons the attempt. Nothing partial survives it.
	Cancel bool `json:"cancel,omitempty"`
	// Places asks what Marco knows about where it has been, and what those places are
	// called. A read.
	Places bool `json:"places,omitempty"`
	// Rename gives a place the Audience's own word for it, or takes that word back when
	// Called is empty.
	//
	// AUDIENCE-INITIATED, with no question pending and none required. Answering a naming
	// question is a different path — this is somebody deciding to change their mind, which
	// is the thing that was impossible and forced a person to edit the store by hand.
	Rename bool `json:"rename,omitempty"`
	// Place is the durable place a rename applies to.
	//
	// An OPAQUE handle the surface got from Places and hands straight back. A person never
	// sees it: it is how the request says WHICH place, not something they choose.
	Place string `json:"place,omitempty"`
	// Watch and Unwatch turn LIGHT MODE on and off: an ordinary passive session, started
	// so place recognition can be watched without learning anything.
	//
	// Recognition only happens while something is observing, and until now the only way to
	// get something observing was to start a demonstration -- which made the instrument
	// require the experiment.
	// Remember makes the screen in front durable under the name given with it.
	//
	// The licence is the NAME, not the request: somebody looking at a screen and saying
	// what it is called is the human semantic event, the same one that lets a learn session
	// establish a place. A Remember with nothing typed establishes nothing.
	Remember bool `json:"remember,omitempty"`
	Watch    bool `json:"watch,omitempty"`
	Unwatch  bool `json:"unwatch,omitempty"`
	// Question, Session and Answer settle one of Marco's OWN open questions.
	//
	// Three fields rather than one because an answer is addressed: which question, in which
	// session, and what was said. A surface that could answer "the current one" would settle
	// whichever question happened to be first at the moment somebody clicked — the same
	// class of mistake as naming whichever screen Marco meant rather than the one they did.
	//
	// The panel used to report these questions, count them, and block the rehearsal behind
	// them while offering no way to answer any of them.
	Question string `json:"question,omitempty"`
	Session  string `json:"session,omitempty"`
	Answer   string `json:"answer,omitempty"`

	// ── The session's own configuration ────────────────────────────────────────────────
	//
	// These arrived here from `ObserveTeach`, which was a SECOND request type for the same
	// event: the person demonstrates and Marco acquires. It was not a peer — this type was a
	// facade over it, translating Start into one of its verbs and Stop into another — so the
	// two were one act described twice, in two vocabularies, one of them using the word the
	// product reserves for Marco guiding a PERSON. There is one acquisition request now.
	//
	// Several entrances send it and they differ only in how they configure it: the control
	// surface presses buttons, `director learn` names a window and a sentence.

	// Target names the window to learn against, for a start request.
	Target windowref.Selector `json:"target,omitzero"`
	// Actor and Verb are the two halves of the sentence a saved play becomes — `do
	// Downloads's Open …`. Both optional: Name is divided into them when it reads as a
	// two-word request, and these say so explicitly when it does not.
	//
	// They are the user's words in both cases. Director never invents either from a screen's
	// text or an application's.
	Actor string `json:"actor,omitempty"`
	Verb  string `json:"verb,omitempty"`
	// Finish is the person saying their demonstration is OVER.
	//
	// Not Cancel, and the difference is everything: Cancel throws the attempt away, and this
	// is the reason the attempt exists. It ends admission of new task input, keeps every
	// single thing already captured, and lets the ordinary pipeline finish — see
	// learn.Coordinator.Finish.
	//
	// `Stop` is the CONTROL SURFACE's word for the same intent, and it is not a synonym: a
	// Stop pressed before anything was demonstrated is abandonment, not completion, and the
	// surface decides which of the two it means from the phase it can see.
	Finish bool `json:"finish,omitempty"`
	// Surface says this request came from MARCO'S OWN control surface.
	//
	// Two jobs, and they are the same fact seen twice. It says the window this request
	// arrived from is Marco's, so the buttons the person presses there are not mistaken for
	// the task they are demonstrating (see surfaceowner.go) — and it says which account of
	// the session to answer with, because a control surface and a person at a terminal want
	// different readings of the same thing.
	//
	// A caller that is not a Marco surface must not set it, and nothing is inferred if it is
	// absent.
	Surface bool `json:"surface,omitempty"`
	// Evidence asks for what is underneath, rather than the plain reading.
	//
	// Named apart from `Watch` on purpose: Watch on this type is LIGHT MODE — watching where
	// you are without acquiring anything — and the two were separate fields on the two types
	// this one replaces. Merging them under one name would have made `director learn --watch`
	// silently start Light Mode.
	Evidence bool `json:"evidence,omitempty"`
	// Dry runs the authorised rehearsal against a recording host, so nothing reaches the
	// computer.
	//
	// A DEVELOPER switch. "Want me to try it once?" means trying it, and an attempt that
	// changes nothing on screen cannot verify a destination — so a dry run ends honestly at
	// the rehearsal rather than producing a play.
	Dry bool `json:"dry,omitempty"`
}

// PerformQuery asks Marco to carry out something it has learned.
//
// The Audience naming a behaviour they want. Distinct from a rehearsal, which is Marco asking
// whether it MAY try something once: here nobody is being asked, they have said to do it — see
// Runtime.PerformGoal for why that is a different authority event with the same bounds.
type PerformQuery struct {
	// Application scopes the outcome. Empty means the one most recently observed.
	Application string `json:"application,omitempty"`
	// Name is the learned outcome, in the Audience's own words.
	//
	// A LABEL, not an identity. It is what a log and a refusal say out loud, and it is the
	// join of last resort — for a goal remembered before subjects were written beside plays,
	// and for a client that has not been rebuilt.
	Name string `json:"name"`
	// Subject is the DURABLE remembered subject the outcome ends on. THE identity.
	//
	// A play's `routes.Origin.To` and its goal's `observe.Goal.Subject` are the same id,
	// written in the same breath by the same learn pass — cmd/director/learnedplay.go and
	// internal/director/learn. Joining on it is exact for every phrase, survives a rename of
	// either side, and matches no words at all.
	//
	// The join used to be Slug(phrase) -> prettyRoute -> EqualFold(Goal.Name), which holds
	// only for plain alphanumeric words: a play learned as "open dad's settings" registered as
	// open-dad-s-settings, was asked for as "open dad s settings", and answered not_learned.
	//
	// Empty is legitimate and means "no sidecar"; Runtime.PerformGoal then falls back to the
	// name join. See TestTheSubjectIdentifiesTheOutcomeAndTheNameIsOnlyALabel.
	Subject string `json:"subject,omitempty"`
}

// PerformStep is what one edge of a performed route did.
type PerformStep struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Verified says Theater produced the step AND the result was confirmed. Nothing else
	// counts: a step that ran and could not be checked is not a step that worked.
	Verified bool   `json:"verified"`
	Terminal string `json:"terminal,omitempty"`
	Refusal  string `json:"refusal,omitempty"`
	Detail   string `json:"detail,omitempty"`
	// Outcome is the walker's own classification of the last step, from rehearse.Outcome.
	//
	// The difference between "the control had moved" and "it went somewhere else entirely"
	// is the difference between trying again and looking for another way, and until 36F it
	// was computed by the walker and dropped here.
	Outcome string `json:"outcome,omitempty"`
	// Observed is the subject perception resolved AFTER the action, empty when nothing did.
	//
	// A failed step may still have moved the interface. Where it actually left Marco is the
	// only honest source to plan from next, and assuming it is still where the edge began is
	// how recovery walks off a screen it is not on.
	Observed string `json:"observed,omitempty"`
	// Cost is what this edge spent looking. Developer-facing; see PerformCost.
	Cost PerformCost `json:"cost,omitzero"`
}

// PerformView is what carrying out a learned outcome did.
type PerformView struct {
	Application string        `json:"application,omitempty"`
	Goal        string        `json:"goal,omitempty"`
	From        string        `json:"from,omitempty"`
	To          string        `json:"to,omitempty"`
	Steps       []PerformStep `json:"steps,omitempty"`
	// Arrived says a FRESH look confirms the Audience is where they asked to be. A plan that
	// ran to the end is not the same fact.
	Arrived bool `json:"arrived,omitempty"`
	// Refusal is why it did not happen, from a closed vocabulary.
	//
	// STOPPING IS ONE OF THE WORDS, not a field of its own. `cancelled` is already how the
	// walker names an attempt the Audience ended — rehearse.CancelledAttempt and
	// rehearse.RefusalCancelled both render as it — and this view could not tell "you stopped
	// it" from "it failed" only because nothing ever set it: the sole context reaching the
	// walker was context.Background(). A second boolean beside this would be a second answer
	// to one question. `busy` is the other word this layer adds, for a request that arrived
	// while something else was already being carried out.
	//
	// Deleting the cancelled wording must fail TestStoppingAPerformanceReportsItAsCancelled.
	Refusal string `json:"refusal,omitempty"`
	Say     string `json:"say,omitempty"`
	// Recovered is what happened when the first route did not work: what was tried, what went
	// wrong with it, and what Marco did instead.
	//
	// Present whether or not the goal was reached, because "it worked" and "it worked on the
	// second attempt" are different facts about somebody's afternoon — and a success reported
	// with no trace of the recovery would hide a broken control indefinitely.
	Recovered []string `json:"recovered,omitempty"`
	// Candidates is what the words could equally have meant, present only with the
	// `ambiguous_outcome` refusal.
	//
	// NAMED, because "that was ambiguous" is not something anybody can act on. A person who
	// taught one phrase in two applications needs to be told which two, and a client that
	// wants to put the choice in front of them needs the list rather than the sentence.
	Candidates []OutcomeView `json:"candidates,omitempty"`
	// Command is the registry id this performance ran under, so `director status`, a
	// CANCEL_ACTIVE and this view all name the same thing. Empty when nothing was begun.
	Command CommandID `json:"command,omitempty"`
	// Testing names the connection an EXPERIMENT was about, absent for an ordinary
	// performance. It is what makes the two distinguishable in one view: "go there" and
	// "check what you learned" produce the same kind of report and are not the same act.
	Testing *EdgeRef `json:"testing,omitempty"`
	// Positioned says Marco had to walk somewhere before it could try the thing being
	// tested. A fact about the attempt somebody should be able to see: an experiment that
	// moved them three screens to get to its starting point did more to their desktop than
	// one that did not.
	Positioned bool `json:"positioned,omitempty"`
	// Tried says the experimental action itself ran. FALSE with a refusal means Marco never
	// got as far as trying, which a person must not read as a result about the connection.
	Tried bool `json:"tried,omitempty"`
	// Restored is what became of the desktop the person was using. See RestoreView.
	Restored *RestoreView `json:"restored,omitempty"`
	// Cost totals what the whole performance spent looking, across every edge.
	// Developer-facing; see PerformCost.
	Cost PerformCost `json:"cost,omitzero"`
}

// PerformCost is what carrying out a route spent finding out where it was.
//
// # Developer-facing, and deliberately not a sentence
//
// Nobody asking Marco to open a settings screen wants to be told how many accessibility snapshots
// it took. This travels on the view so an Advanced surface and the 35C acceptance harness can
// report it, and nothing in this repository turns it into words for the Audience.
//
// # Why counts and durations both
//
// A duration is what a person feels, and it is the thing a test cannot assert: it depends on the
// machine, the provider, the size of the accessibility tree, and whatever else the desktop is
// doing. A COUNT is deterministic, and it is very nearly proportional to the duration, because
// reading the screen is what a walk actually spends its time on.
//
// So the suite gates the counts and a live run reports the durations. A number nobody measured is
// left out rather than guessed.
type PerformCost struct {
	// Samples is readings of the screen — the accessibility snapshots.
	Samples int `json:"samples,omitempty"`
	// Resolutions is how many of those readings were turned into a Place.
	Resolutions int `json:"resolutions,omitempty"`
	// Establishments is "where am I", asked from nothing.
	Establishments int `json:"establishments,omitempty"`
	// Confirmations is shortened checks of a proof already held, and Reused is how many
	// agreed. The gap between them is how often the shortcut was tried and thrown away.
	Confirmations int `json:"confirmations,omitempty"`
	Reused        int `json:"reused,omitempty"`
	// LookingMS and TotalMS are milliseconds: time inside those looks, and the whole walk.
	LookingMS int64 `json:"looking_ms,omitempty"`
	TotalMS   int64 `json:"total_ms,omitempty"`
}

// Add folds one edge's cost into a route's total.
func (c *PerformCost) Add(o PerformCost) {
	c.Samples += o.Samples
	c.Resolutions += o.Resolutions
	c.Establishments += o.Establishments
	c.Confirmations += o.Confirmations
	c.Reused += o.Reused
	c.LookingMS += o.LookingMS
	c.TotalMS += o.TotalMS
}

// AmbientView is what ambient watching is doing, and what it has noticed.
//
// # Visible on purpose
//
// Ambient observation is materially different product behaviour from an explicit Learn, and the
// difference a person cares about is that it is ON when they are not thinking about it. A
// background mode nobody can see the state of is the shape of surveillance whatever its
// intentions, so the state is a first-class answer rather than a diagnostic.
//
// Everything here is a count, a duration or a durable subject id. No labels, no titles, no screen
// text, no coordinates: the transient buffer behind it holds none of those, and this view could
// not report them if it wanted to.
type AmbientView struct {
	// Watching is the whole product question. Everything else is detail.
	Watching bool `json:"watching"`
	// WatchingForMS is how long this run of attention has lasted.
	WatchingForMS int64 `json:"watching_for_ms,omitempty"`
	// Application and Place are where the last reading put the Audience. Place is a durable
	// subject id, empty when the screen is not one Marco knows — which is ordinary, and not
	// a question anybody is asked.
	Application string `json:"application,omitempty"`
	Place       string `json:"place,omitempty"`
	// PerceptionDegraded says the most recent reading got no further than the window frame.
	// A different fact from an unrecognised screen; see ADR-090.
	PerceptionDegraded bool `json:"perception_degraded,omitempty"`
	// Places, Transitions and Recent are how much the transient buffer holds — DISTINCT
	// things, not events. These are the numbers that must not grow with how long somebody
	// watched, only with how much they did.
	Places      int `json:"places,omitempty"`
	Transitions int `json:"transitions,omitempty"`
	Recent      int `json:"recent,omitempty"`
	// Samples and Sessions are what it cost. AttentionMS is the current gap between
	// readings, which grows while nothing changes.
	Samples     int   `json:"samples,omitempty"`
	Sessions    int   `json:"sessions,omitempty"`
	AttentionMS int64 `json:"attention_ms,omitempty"`

	// Learning is whether Marco may turn what it watches into durable memory.
	//
	// A VALUE and always present, for exactly the reason `Watching` is: "is Marco building
	// permanent memory from what it sees" is a question somebody is entitled to a straight
	// answer to, and a field that is absent when the answer is no cannot be told apart from a
	// Director too old to have it.
	//
	// SEPARATE FROM WATCHING, and the separation is the product decision rather than a
	// implementation detail. Watching is attention; learning is memory. One switch that did
	// both would make `marco observe` mean something nobody agreed to. See ADR-095.
	Learning bool `json:"learning"`
	// Noticed and Learned are what the candidate ledger has done: relationships it has
	// evidence about, and how many of them have earned durable memory.
	//
	// Counts, never a subject id and never a control's name — the same boundary the rest of
	// this view holds. "Marco has learned three things by watching you" is a fact somebody
	// should be able to read; which three is a question for the store.
	Noticed int `json:"noticed,omitempty"`
	Learned int `json:"learned,omitempty"`
	// Candidates is how many relationships the durable ledger currently holds evidence about,
	// promoted or not. The number that must track how many DIFFERENT things somebody does,
	// never how long Marco watched.
	Candidates int `json:"candidates,omitempty"`
}

// ObserveAmbient turns ambient watching on or off, or asks what it is doing.
//
// One request with three shapes rather than three verbs: they are the same lifecycle asked about
// three ways, and a separate protocol entry for each would be three things to keep in agreement.
type ObserveAmbient struct {
	// Enable and Disable are the lifecycle. Neither set is a status read.
	Enable  bool `json:"enable,omitempty"`
	Disable bool `json:"disable,omitempty"`
	// Learn and Unlearn turn ambient LEARNING on and off — whether what Marco watches may
	// become durable memory.
	//
	// A separate pair from Enable and Disable, on the same request, because they are the same
	// lifecycle asked about a different thing. What they must never be is one pair: watching
	// and remembering are two things to agree to, and a switch that silently did both would
	// make `marco observe` a promise about permanence that nobody heard.
	//
	// `Learn` may turn watching on with it — asking Marco to learn from what it sees is
	// meaningless while it is not looking, and refusing would be pedantry about a state the
	// person plainly did not want. `Unlearn` never turns watching off: they asked for less
	// memory, not less attention.
	//
	// Named for what they do rather than for the field they set, so a reader of a request does
	// not have to know that "promotion" is what the policy calls it.
	Learn   bool `json:"learn,omitempty"`
	Unlearn bool `json:"unlearn,omitempty"`
	// Evidence asks what Marco has seen repeatedly and what it is waiting for.
	//
	// A READ, and it changes nothing. It is the answer to "why haven't you learned that
	// yet", which until it existed had no answer at all — the counts said how many
	// relationships had evidence and nothing said what any of them was short of, so the only
	// way to find out was to open the store and reason about the policy by hand.
	//
	// It deliberately names things: the control somebody pressed, and whether Marco
	// recognises the screens either side. That is a WIDENING of the diagnostic surface and it
	// is the right one — a person asking what Marco has recorded about them is entitled to
	// the answer, and a privacy boundary that made Marco's own memory unreadable to its owner
	// would be protecting the wrong party. See ADR-095.
	Evidence bool `json:"evidence,omitempty"`
}

// WatchedView is one thing Marco has seen repeatedly, and what it makes of it.
//
// The candidate ledger, rendered for a person. Counts, the semantic identity, and the POLICY's
// own verdict in its own words — so "seen twice and not learned" and "that control leads two
// different places" are told apart by Marco rather than inferred by whoever is reading.
type WatchedView struct {
	Application string `json:"application"`
	// Did and Control are what somebody did and what they did it to.
	Did     string `json:"did"`
	Control string `json:"control,omitempty"`
	// FromKnown and ToKnown say whether Marco recognises the screens either side. A screen it
	// does not recognise is ordinary and not a fault; a screen it cannot even describe is
	// what stops a relationship being learnable, and the verdict below says which.
	FromKnown bool `json:"from_known"`
	ToKnown   bool `json:"to_known"`
	// Seen is how many times this was traversed; Sessions across how many watching sessions.
	// Held apart because "twice in a minute" and "twice on different days" are different
	// strengths of the same fact.
	Seen         int `json:"seen"`
	Sessions     int `json:"sessions,omitempty"`
	Contradicted int `json:"contradicted,omitempty"`
	// Verdict and Why are the policy's own words, and Said is the sentence for a person.
	Verdict string `json:"verdict"`
	Why     string `json:"why"`
	Said    string `json:"said"`
	// Short is how many more traversals a policy that asked for corroboration is waiting for,
	// zero otherwise.
	Short int `json:"short,omitempty"`
	// Learned says this already became durable knowledge.
	Learned bool `json:"learned,omitempty"`
	// FirstSaw and LastSaw are when this way between two screens was first and last taken.
	//
	// BOTH, because one of them alone says the wrong thing. "Last seen an hour ago" describes
	// a thing somebody does and a thing they did once identically, and the span between the two
	// is what tells them apart — a relationship taken twelve times since Tuesday is a different
	// fact from twelve times in one afternoon, and the counts cannot say which.
	FirstSaw string `json:"first_saw,omitempty"`
	LastSaw  string `json:"last_saw,omitempty"`
}

// ObserveLearning asks what durable knowledge has changed since a cursor.
//
// A READ, and the narrowest one on this query: it starts nothing, samples nothing, and cannot
// create the knowledge it reports. `After` is a sequence number from a previous reply; zero means
// "everything the Director still holds".
type ObserveLearning struct {
	After uint64 `json:"after,omitempty"`
}

// LearningEvent is one committed change to durable semantic knowledge, in words.
//
// Rendered by the Director rather than by the client, because rendering means resolving a
// subject id to what the Place is called and only the Director has the store. `Description` is
// therefore already the sentence; a client that reassembled one from parts would be a second
// vocabulary for the same fact.
type LearningEvent struct {
	// Change is learned, strengthened, named or rebound.
	Change string `json:"change"`
	// Kind is place, edge or goal.
	Kind string `json:"kind"`
	// Application scopes it.
	Application string `json:"application,omitempty"`
	// Description is what to show: a Place's word, an edge as `from → to`, a goal as
	// `"name" → place`.
	Description string `json:"description"`
}

// LearningView is the reply: what changed, and where to resume.
type LearningView struct {
	Events []LearningEvent `json:"events,omitempty"`
	// Newest is the cursor to ask from next time.
	Newest uint64 `json:"newest"`
	// Missed is how many events fell out of the Director's ring before this read.
	//
	// Reported rather than swallowed. A feed that quietly started late would let somebody
	// conclude Marco had learned nothing during the gap, which is the one wrong answer this
	// surface must never give.
	Missed uint64 `json:"missed,omitempty"`
}
