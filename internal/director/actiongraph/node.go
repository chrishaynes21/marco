// Package actiongraph is the Director's durable memory of what it has done and why.
//
// An ActionNode is not a log line. A log says "mouse clicked at (842,391)", which is
// true, useless, and false again the moment the window moves. A node says: the user
// asked to go back, that resolved to Chrome's Back button, it was clicked, and the
// page title changed afterwards. The first records an event; the second records
// KNOWLEDGE — and knowledge is what a later "do that again" can act on.
//
// The distinction is the whole package. Coordinates appear only inside execution
// metadata, as evidence of where an action landed. They are never the identity of
// what was done, because two clicks on the same button a day apart are the same
// action even though the pixels differ, and two clicks on the same pixel a day apart
// are usually not.
//
// # Append-only
//
// Nodes are immutable once added. Nothing is ever updated in place: if an action is
// later replayed and fails, that produces a NEW node, not an edit to the old one.
// History is a record of what happened, and rewriting it to match what happened next
// destroys exactly the information that makes a failure diagnosable.
//
// # This milestone builds the commits, not the checkout
//
// Replay, undo, batching and conversational reference ("that", "it", "again") will
// all be built on these nodes. None of them exists yet. What exists is the ability to
// ask, without executing anything: what happened, what was intended, what was acted
// on, and could it be done again right now.
package actiongraph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/inline"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// NodeID identifies one node in the graph.
type NodeID string

// ActionNode is one durable, immutable semantic action.
type ActionNode struct {
	ID        NodeID    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Intent is the parsed request that produced this action.
	Intent directorapi.Intent `json:"intent"`
	// Goal is what the action was for, in the user's terms ("click Save").
	Goal string `json:"goal"`
	// Reason is why it was done — the resolver's justification for this target over
	// the others. It answers "why that one?" long after the alternatives are gone.
	Reason string `json:"reason,omitempty"`

	// GoalProvenance is which goal and procedure produced this node, when the request
	// was a goal. DIAGNOSTIC ONLY: replay reads the semantic action below, never this,
	// and a node whose procedure has since been deleted replays unchanged.
	//
	// Omitted entirely for an ordinary request and for every node written before this
	// existed, so old graphs stay valid without a migration.
	GoalProvenance *GoalProvenance `json:"goal_provenance,omitempty"`

	// Binding is the deictic reference this action was aimed at, as it stood when the
	// action ran.
	//
	//	Binding metadata is audit and targeting information, not authorization.
	//
	// TARGETING, because a replay of this node re-establishes the same object from the
	// identity here rather than re-reading the word "this" against current focus. AUDIT,
	// because it says on what evidence the object was chosen. Never AUTHORIZATION: a
	// node whose action was confirmed carries that fact as history, and history is not
	// consent — see the replay policy.
	//
	// Omitted entirely for every non-deictic action and for every node written before
	// this existed, so old graphs decode unchanged and re-encoding one does not invent a
	// binding it never had.
	Binding *binding.Snapshot `json:"binding,omitempty"`

	// Editor is the inline editor this action was carried out in, when it was carried
	// out in one — Explorer's in-place rename box being the case that exists today.
	//
	//	Do not serialize ephemeral native handles as durable graph identity.
	//	Replay must not assume an old inline editor still exists.
	//
	// DIAGNOSTIC ONLY, and the snapshot is built so it cannot be anything else: it says
	// WHAT was edited, what the box contained before and after, and on what evidence it
	// was tied to the object — and deliberately not enough to find the control again. A
	// replay of this node re-enters edit mode from the binding above and derives a fresh
	// editor; there is no field here it could use to skip that.
	//
	// Omitted for every action that did not act on an editor, and for every node written
	// before this existed.
	Editor *inline.Snapshot `json:"editor,omitempty"`

	// Confirmation is what the confirmation gate concluded for this action, as a
	// STRING for the same reason the goal provenance is: it is a record, and a record
	// that decoded into a live decision type would invite something to act on it.
	Confirmation string `json:"confirmation,omitempty"`

	// Plan is the typed plan that was executed, in a form that survives storage.
	Plan PlanSnapshot `json:"plan"`

	// RequestedTarget is what the user SAID ("Save", "the other window"), kept
	// separate from what it resolved to. A future reference like "the same field as
	// before" needs the words, not only the element.
	RequestedTarget directorapi.ReferenceExpression `json:"requested_target"`

	// ResolvedTarget is what it actually acted on, as it was at the time. Historical
	// only: replay must re-resolve, never reuse this.
	ResolvedTarget TargetSnapshot `json:"resolved_target"`

	Preconditions     []directorapi.Condition `json:"preconditions,omitempty"`
	SuccessConditions []directorapi.Condition `json:"success_conditions,omitempty"`

	Verification directorapi.VerificationResult `json:"verification"`
	Outcome      OutcomeSummary                 `json:"outcome"`

	// Parent links this node to the action that led to it — "open the File menu"
	// then "click Save". Chains of these are what workflows will be made of.
	Parent *NodeID `json:"parent,omitempty"`

	// Metadata carries everything that is evidence rather than meaning: where the
	// click landed, how long it took, how many attempts. Deliberately segregated so
	// that nothing in the semantic model can come to depend on a coordinate.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PlanSnapshot is a plan in a form that can be stored and read back.
//
// It is not directorapi.Plan because that holds Action INTERFACES, and an interface
// cannot be decoded: the decoder has no way to know which concrete type to build.
// Storing a typed spec instead means a node written today is still readable by a
// build that has since added new action types.
type PlanSnapshot struct {
	Goal        string                `json:"goal"`
	Risk        directorapi.RiskLevel `json:"risk"`
	Assumptions []string              `json:"assumptions,omitempty"`
	Steps       []StepSnapshot        `json:"steps"`
	MaxAttempts int                   `json:"max_attempts,omitempty"`
}

// StepSnapshot is one plan step, stored.
type StepSnapshot struct {
	Index     int                     `json:"index"`
	Action    ActionSpec              `json:"action"`
	Expect    []directorapi.Condition `json:"expect,omitempty"`
	Rationale string                  `json:"rationale,omitempty"`
}

// ActionSpec is a serialisable description of an action.
//
// It is what replaces the Action interface in storage, and it is deliberately
// SEMANTIC: it carries the query that found the target, not the point the target
// happened to occupy. Rebuild turns it back into a live action, and the result
// re-resolves when executed — which is what makes an action from last week still
// mean something today.
type ActionSpec struct {
	Type directorapi.ActionType `json:"type"`

	// Query is how the target was described. The single most important field for
	// replay: it is the difference between "click Save" and "click (842,391)".
	Query *directorapi.ElementQuery `json:"query,omitempty"`
	// Description is what the user called it.
	Description string `json:"description,omitempty"`

	// SemanticKind is the VERB, for a semantic action — expand, select, submit.
	//
	// The whole of what a semantic action is stored as, and deliberately so: what the
	// user meant survives, and HOW it was carried out does not. The mechanism was
	// chosen from the control's capabilities at the moment it ran, so a replay must
	// choose again rather than repeat a decision made against a tree that has since
	// changed. A node that stored "clicked the disclosure arrow" would replay a
	// geometric guess into an application that may now expose ExpandCollapse.
	SemanticKind directorapi.SemanticActionKind `json:"semantic_kind,omitempty"`
	// Ordinal is "the second one", for a Choose.
	Ordinal int `json:"ordinal,omitempty"`

	Button directorapi.MouseButton `json:"button,omitempty"`
	Count  int                     `json:"count,omitempty"`

	Window    *WindowSpec    `json:"window,omitempty"`
	Placement *PlacementSpec `json:"placement,omitempty"`
}

// WindowSpec identifies a window semantically.
//
// The handle is recorded but must not be relied on: window handles are reissued
// every time an application restarts, so an old one either finds nothing or — worse
// — finds something else. Application and title are what survive.
type WindowSpec struct {
	Handle      string `json:"handle,omitempty"`
	Application string `json:"application,omitempty"`
	Title       string `json:"title,omitempty"`
}

// PlacementSpec is a window destination.
//
// Named is the semantic form ("left_half") and is what a replay should use; Bounds
// is the rectangle that was actually computed at the time, kept as evidence. On a
// different monitor layout the rectangle would be wrong and the name still right.
type PlacementSpec struct {
	Named     string            `json:"named,omitempty"`
	MonitorID string            `json:"monitor_id,omitempty"`
	Bounds    *directorapi.Rect `json:"bounds,omitempty"`
}

// TargetSnapshot is what the action acted on, as it was at the time.
//
// Every field here is HISTORICAL. Nothing in it may be used to locate the target
// again — that is what the query in ActionSpec is for. It exists to answer "what did
// you act on?" and to let replay analysis notice that the thing has moved, changed
// state, or gone.
type TargetSnapshot struct {
	ElementID string `json:"element_id,omitempty"`
	WindowID  string `json:"window_id,omitempty"`
	App       string `json:"app,omitempty"`

	Role  string `json:"role,omitempty"`
	Label string `json:"label,omitempty"`

	Bounds directorapi.Rect `json:"bounds"`

	Identity      IdentitySnapshot          `json:"identity"`
	Actionability directorapi.Actionability `json:"actionability"`

	// Confidence is how sure the resolver was at the time.
	Confidence float64 `json:"confidence"`
}

// IdentitySnapshot records what the target could be recognised by.
//
// This is the material a future re-resolution has to work with, and recording it now
// is what lets analysis say WHY a target could not be found again. An element with a
// unique application-authored id will be findable after the UI is rebuilt; one known
// only by a duplicated label will not.
type IdentitySnapshot struct {
	// NativeID is the source's own identifier (a UIA RuntimeId). Stable while the
	// element lives, reissued when it is recreated.
	NativeID string `json:"native_id,omitempty"`
	// AutomationID is application-authored and survives recreation.
	AutomationID string `json:"automation_id,omitempty"`
	// ParentLabel is the enclosing group's name — what distinguishes two identical
	// "Apply" buttons in different sections.
	ParentLabel string `json:"parent_label,omitempty"`
	// LabelUnique records whether the label identified exactly one element in its
	// window at the time. When false, re-resolution is expected to be ambiguous, and
	// analysis can say so before anything is attempted.
	LabelUnique bool `json:"label_unique"`
	// Durable is whether this element would have survived a rebuild of the UI.
	Durable bool `json:"durable"`
}

// OutcomeSummary is what came of the action.
type OutcomeSummary struct {
	Success bool                     `json:"success"`
	Status  directorapi.ActionStatus `json:"status"`
	// Reason is the verification verdict in words.
	Reason string `json:"reason,omitempty"`
	// FailureReason is set when the action did not succeed.
	FailureReason string `json:"failure_reason,omitempty"`

	Before directorapi.WorldStateSummary `json:"before"`
	After  directorapi.WorldStateSummary `json:"after"`

	Attempts   int   `json:"attempts,omitempty"`
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Reversible reports whether this action has a known inverse. Recorded now
	// because "undo the last thing you did" will need it, and it cannot be
	// reconstructed after the fact.
	Reversible bool `json:"reversible"`
}

// Changed reports whether the screen differed before and after.
func (o OutcomeSummary) Changed() bool {
	return o.Before.Fingerprint != "" && o.After.Fingerprint != "" &&
		o.Before.Fingerprint != o.After.Fingerprint
}

// SemanticKey is this action's identity as a MEANING rather than as an event.
//
// Two actions share a key when they express the same intent against the same class
// of target: "click Save in Notepad" today and "click Save in Notepad" tomorrow are
// the same action even if the button moved, the window was recreated, and every
// coordinate and handle differs.
//
// What is deliberately excluded is everything transient: element ids, window
// handles, bounds, timestamps. Including any of them would make every action unique,
// which would make the key useless for exactly the questions it exists to answer.
func (n ActionNode) SemanticKey() string {
	parts := []string{
		string(n.actionType()),
		strings.ToLower(strings.TrimSpace(n.ResolvedTarget.App)),
		strings.ToLower(strings.TrimSpace(n.ResolvedTarget.Role)),
		normaliseLabel(n.ResolvedTarget.Label),
	}
	// A window placement is part of what the action MEANS: moving a window left and
	// moving it right are not the same action.
	if len(n.Plan.Steps) > 0 && n.Plan.Steps[0].Action.Placement != nil {
		parts = append(parts, n.Plan.Steps[0].Action.Placement.Named)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:16]
}

// Equivalent reports whether two nodes describe the same semantic action.
func Equivalent(a, b ActionNode) bool { return a.SemanticKey() == b.SemanticKey() }

// actionType is the node's action type, from its first step.
func (n ActionNode) actionType() directorapi.ActionType {
	if len(n.Plan.Steps) == 0 {
		return ""
	}
	return n.Plan.Steps[0].Action.Type
}

// Action returns the node's action spec, and whether it has one.
func (n ActionNode) Action() (ActionSpec, bool) {
	if len(n.Plan.Steps) == 0 {
		return ActionSpec{}, false
	}
	return n.Plan.Steps[0].Action, true
}

// Describe renders the node in one line.
func (n ActionNode) Describe() string {
	if n.Goal != "" {
		return n.Goal
	}
	spec, ok := n.Action()
	if !ok {
		return "an unknown action"
	}
	if n.ResolvedTarget.Label != "" {
		return fmt.Sprintf("%s %q", spec.Type, n.ResolvedTarget.Label)
	}
	return string(spec.Type)
}

// Rebuild turns a stored spec back into a live action.
//
// The rebuilt action carries the QUERY, not the old coordinates, so executing it
// re-resolves against the current world. That is the point: a replayed action must
// find today's Save button, not click where yesterday's was.
func (s ActionSpec) Rebuild() (directorapi.Action, error) {
	ref := directorapi.ElementReference{
		Query:       s.Query,
		Description: s.Description,
	}

	switch s.Type {
	case directorapi.ActionClick:
		if s.Query == nil {
			return nil, fmt.Errorf("a stored click has no target query, so it cannot be re-resolved")
		}
		return directorapi.ClickAction{Target: ref, Button: s.Button, Count: s.Count}, nil

	case directorapi.ActionFocus:
		if s.Query == nil {
			return nil, fmt.Errorf("a stored focus has no target query, so it cannot be re-resolved")
		}
		return directorapi.FocusAction{Target: ref}, nil

	case directorapi.ActionSemantic:
		if !s.SemanticKind.Known() {
			return nil, fmt.Errorf("a stored action asks for %q, which is not an action "+
				"this Director knows how to perform", s.SemanticKind)
		}
		if s.SemanticKind.NeedsTarget() && s.Query == nil {
			return nil, fmt.Errorf("a stored %s has no target query, so it cannot be re-resolved",
				s.SemanticKind)
		}
		act := directorapi.SemanticAction{Kind: s.SemanticKind, Ordinal: s.Ordinal}
		if s.Query != nil {
			act.Target = ref
		}
		if s.Window != nil {
			// Rebuilt from application and title, never the stored handle — handles are
			// reissued on restart and would either miss or find a different window.
			act.Window = &directorapi.WindowReference{
				Application:   s.Window.Application,
				TitleContains: s.Window.Title,
				Description:   s.Window.Title,
			}
		}
		return act, nil

	case directorapi.ActionMoveWindow:
		if s.Window == nil {
			return nil, fmt.Errorf("a stored window move names no window")
		}
		if s.Placement == nil {
			return nil, fmt.Errorf("a stored window move has no destination")
		}
		// The window is rebuilt from its APPLICATION and TITLE, never its old
		// handle: handles are reissued on restart and would either miss or, worse,
		// find a different window.
		return directorapi.MoveWindowAction{
			Window: directorapi.WindowReference{
				Application:   s.Window.Application,
				TitleContains: s.Window.Title,
				Description:   s.Window.Title,
			},
			// Bounds are deliberately dropped. The named placement is what the
			// action meant; the rectangle was a consequence of the monitor layout at
			// the time, and replaying it on a different layout would be wrong.
			Placement: directorapi.WindowPlacement{
				Named:     s.Placement.Named,
				MonitorID: s.Placement.MonitorID,
			},
		}, nil
	}
	return nil, fmt.Errorf("no way to rebuild a %q action", s.Type)
}

// normaliseLabel matches the normalisation used by fusion, identity and target
// resolution, so a label decorated by one toolkit and not another still compares
// equal.
func normaliseLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", "")
	s = strings.TrimSuffix(s, "...")
	s = strings.TrimSuffix(s, "…")
	s = strings.TrimSuffix(s, ":")
	return strings.TrimSpace(s)
}
