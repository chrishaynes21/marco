package directorapi

import "time"

// IntentKind names what the user wants at the top level.
type IntentKind string

const (
	// IntentAct is a direct request to do something ("click Save").
	IntentAct IntentKind = "act"
	// IntentRepeat re-runs a previous action ("do that again", "five more times").
	IntentRepeat IntentKind = "repeat"
	// IntentUndo reverses a previous action ("undo that", "put it back").
	IntentUndo IntentKind = "undo"
	// IntentStop cancels whatever is running. Always honoured immediately, never
	// queued behind anything, and never subject to confirmation.
	IntentStop IntentKind = "stop"
	// IntentQuery asks about the world without changing it ("what's on screen?").
	IntentQuery IntentKind = "query"
	// IntentClarify is the user answering a question the Director asked.
	IntentClarify IntentKind = "clarify"
	// IntentUnknown means the parser could not classify the input. It is a real
	// outcome, not an error — the Director asks rather than guessing.
	IntentUnknown IntentKind = "unknown"
)

// Intent is the parsed, typed form of one user request.
//
// Parsing is deliberately separate from planning: the parser says WHAT was asked,
// the resolver says WHICH objects it refers to, and the planner says HOW to do it.
// Keeping them apart is what lets a deterministic parser handle "click Save" with
// no model at all, while the same downstream machinery serves a model-parsed
// request like "tidy up these windows".
type Intent struct {
	Kind IntentKind `json:"kind"`
	// Raw is the user's original input, verbatim.
	Raw string `json:"raw"`
	// Verb is the normalised action word ("click", "type", "move", "scroll").
	Verb string `json:"verb,omitempty"`
	// Targets are the referring expressions found in the request, in order.
	Targets []ReferenceExpression `json:"targets,omitempty"`
	// Text is literal content to enter, for a type-style request.
	Text string `json:"text,omitempty"`
	// Count is an explicit repetition count ("five times"). 0 means unspecified.
	Count int `json:"count,omitempty"`
	// Until is a termination condition expressed in the request ("until you see my
	// name"), left as text for the resolver to turn into a Condition.
	Until string `json:"until,omitempty"`
	// Parameters carries verb-specific extras a skill or planner understands.
	Parameters map[string]any `json:"parameters,omitempty"`
	// Confidence is how sure the parser is of this reading, 0..1.
	Confidence float64 `json:"confidence"`
	// Ambiguity, when non-empty, is what the Director should ask about before acting.
	Ambiguity string `json:"ambiguity,omitempty"`
}

// ReferenceExpression is one referring phrase from the request, before resolution.
// It keeps the user's words alongside a structural reading, because the words are
// what a clarification question has to quote back.
type ReferenceExpression struct {
	// Phrase is the user's words ("the second tab", "that", "the other window").
	Phrase string `json:"phrase"`
	// Kind classifies the reference.
	Kind ReferenceKind `json:"kind"`
	// Query is the structural reading where the parser could produce one.
	Query *ElementQuery `json:"query,omitempty"`
	// Plural marks a reference to a set ("these", "all the rows", "every selected
	// file") — the difference between one action and a bounded iteration.
	Plural bool `json:"plural,omitempty"`

	// RequiresBinding marks a reference that may not be executed until it has been
	// resolved to a concrete, typed, evidenced object.
	//
	// Set on a deictic reference — "this file" — where the words alone name nothing
	// checkable. A reference carrying this and no BindingID is REFUSED at execution
	// rather than aimed at whatever holds focus, which is the silent fallback this
	// field exists to make impossible.
	RequiresBinding bool `json:"requires_binding,omitempty"`
	// BindingID names the request-scoped binding that proves what this reference
	// points at. It means nothing outside the request that minted it, which is exactly
	// how long a binding may be trusted for.
	//
	// A handle rather than the binding itself, because a reference is copied into a
	// plan, a record and a graph node, and revalidation REFRESHES a binding — five
	// copies of a mutable fact is five chances to describe one object and act on
	// another. The single live copy lives in the request's binding store.
	BindingID string `json:"binding_id,omitempty"`
	// ExpectedKind is the sort of object this reference must resolve to ("file",
	// "folder", "symbol"), as declared by whatever produced it.
	//
	// A string rather than a typed kind because the vocabulary is owned by the binding
	// package, which is built on this one. Unknown values are refused at resolution.
	ExpectedKind string `json:"expected_kind,omitempty"`

	// RequiresEditor marks a reference to the INLINE EDITOR a previous step opened on
	// the bound object — a file manager's in-place rename box.
	//
	// A derived target: it did not exist when the user spoke, it is created by acting on
	// something that did, and it lives only until the edit ends. Resolved immediately
	// before the step that uses it, from the observation that step made, and never
	// stored — a remembered editor is a control that has since closed.
	//
	// Declared here rather than expressed as a query because a procedure must not name a
	// platform's control classes. It says "the editor for the thing I bound"; which
	// control that is on this machine is the platform's answer.
	RequiresEditor bool `json:"requires_editor,omitempty"`
}

// ReferenceKind classifies a referring expression by how it must be resolved.
type ReferenceKind string

const (
	// ReferenceLiteral names something directly ("the Save button").
	ReferenceLiteral ReferenceKind = "literal"
	// ReferenceDeictic points at the current context ("this", "here", "it") and
	// resolves against the cursor, the focus, or the selection.
	ReferenceDeictic ReferenceKind = "deictic"
	// ReferenceAnaphoric points backwards in the conversation ("that", "the same
	// one", "it" after a prior mention) and resolves against history.
	ReferenceAnaphoric ReferenceKind = "anaphoric"
	// ReferenceOrdinal picks by position ("the second tab", "the last one").
	ReferenceOrdinal ReferenceKind = "ordinal"
	// ReferenceSpatial picks by relative position ("the one on the left", "the
	// button above it").
	ReferenceSpatial ReferenceKind = "spatial"
	// ReferenceContrastive picks the other member of a pair ("the other monitor",
	// "the other window").
	ReferenceContrastive ReferenceKind = "contrastive"
	// ReferenceSelection refers to whatever is selected.
	ReferenceSelection ReferenceKind = "selection"
)

// EntityType names what kind of thing a reference resolved to.
type EntityType string

const (
	EntityElement     EntityType = "element"
	EntityWindow      EntityType = "window"
	EntityApplication EntityType = "application"
	EntityMonitor     EntityType = "monitor"
	EntityAction      EntityType = "action"
	EntityText        EntityType = "text"
)

// ResolvedReference is one resolution of one referring expression.
//
// It always carries Alternatives and an Explanation, even when confident. That is
// the whole contract of the resolver: a target the Director cannot justify is a
// target it should not act on, and low confidence on a destructive action is what
// triggers a clarifying question instead of a click.
type ResolvedReference struct {
	EntityID   string     `json:"entity_id"`
	EntityType EntityType `json:"entity_type"`
	Confidence float64    `json:"confidence"`
	// Explanation is why this was chosen, in one sentence.
	Explanation string `json:"explanation,omitempty"`
	// Alternatives are the other candidates considered.
	Alternatives []ReferenceCandidate `json:"alternatives,omitempty"`
	// Expression is the phrase this resolves.
	Expression ReferenceExpression `json:"expression"`
}

// ReferenceCandidate is one considered candidate for a reference.
type ReferenceCandidate struct {
	EntityID   string     `json:"entity_id"`
	EntityType EntityType `json:"entity_type"`
	Label      string     `json:"label,omitempty"`
	Score      float64    `json:"score"`
	Reason     string     `json:"reason,omitempty"`
}

// ConversationState is the Director's EXPLICIT memory of the exchange.
//
// It is structured on purpose. Relying on a language model's conversation memory to
// remember what "it" meant is unreliable and unauditable; storing the resolved
// entity IDs means "make it smaller" targets the same Spotify window as "move it to
// the other monitor" did, provably, and works with no model at all.
type ConversationState struct {
	// CurrentGoal is what the user is trying to achieve across turns.
	CurrentGoal string `json:"current_goal,omitempty"`
	// MentionedEntities are things referred to recently, most recent first —
	// the candidate pool for anaphoric references.
	MentionedEntities []EntityReference `json:"mentioned_entities,omitempty"`
	// LastTarget is what the previous action acted on, the default for "it".
	LastTarget *ResolvedReference `json:"last_target,omitempty"`
	// LastAction is the previous action, the referent of "that" and the thing "do
	// that again" repeats.
	LastAction *ActionRecord `json:"last_action,omitempty"`
	// ActivePlan is a plan currently running or paused for confirmation.
	ActivePlan *Plan `json:"active_plan,omitempty"`
	// PendingQuestion is a clarification awaiting an answer. While set, the next
	// user input is interpreted as a reply to it.
	PendingQuestion *Clarification `json:"pending_question,omitempty"`
}

// EntityReference is one remembered mention.
type EntityReference struct {
	EntityID    string     `json:"entity_id"`
	EntityType  EntityType `json:"entity_type"`
	Label       string     `json:"label,omitempty"`
	MentionedAt time.Time  `json:"mentioned_at"`
	// Phrase is how the user referred to it.
	Phrase string `json:"phrase,omitempty"`
}

// Clarification is a question the Director needs answered before it can proceed.
type Clarification struct {
	Question string `json:"question"`
	// Options are the choices, when the ambiguity is a choice between candidates.
	Options []ReferenceCandidate `json:"options,omitempty"`
	// BlockedPlan is the plan waiting on the answer.
	BlockedPlan *Plan `json:"blocked_plan,omitempty"`
	// Reason distinguishes a genuine ambiguity from a policy confirmation.
	Reason  string    `json:"reason,omitempty"`
	AskedAt time.Time `json:"asked_at"`
}
