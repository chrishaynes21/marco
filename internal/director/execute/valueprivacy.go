package execute

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"strings"
)

// What a consumed value leaves behind.
//
//	Sensitive data may exist in memory long enough to execute the requested
//	effect, but it may not become history.
//
// Those are two different lifetimes and the whole of this file is about keeping them
// apart. A customer's email HAS to appear in the generated Marco program — there is no
// other way to put it into a field — and it has to be in the record long enough for the
// verifier to compare what the control now says against what was intended. Neither of
// those requires it to be in a JSON file tomorrow morning.
//
// So the plaintext is stripped at exactly one point: after the action has run and been
// verified, before the node is built. Earlier would break verification; later is too
// late, because the node is durable the moment it is added.

// valueUse is what a step read from the program environment.
//
// Held on the outcome rather than passed around, because the two places that need it —
// the redaction below and the node's metadata — are far apart in handleParsed and
// threading a parameter between them would mean every intermediate signature grew one.
type valueUse struct {
	Ref values.Reference
	Val values.Value
}

// redactHistory removes a sensitive consumed value from everything durable.
//
// PUBLIC values are left alone. A window title or a literal the user typed is exactly
// the sort of thing history is for, and redacting it would make the record useless
// while protecting nothing. The rule is about content that was private when it was
// read, not about where it came from.
// plans is VARIADIC because more than one plan survives a retry: the node is built
// from the plan that was executed first, and the outcome keeps the one that replaced
// it. Both are retained, so cleaning either alone leaves the value in the other — which
// is precisely what the audit test caught, twice, once for each of them.
func redactHistory(use *valueUse, in *directorapi.Intent, rec *directorapi.ActionRecord,
	plans ...*directorapi.Plan) {

	if use == nil || use.Val.Visibility() == values.VisibilityNormal {
		return
	}
	secret := use.Val.Plaintext()
	if secret == "" {
		return
	}

	// The intent's own text, which is what the node's Intent field carries.
	if in != nil && in.Text == secret {
		in.Text = values.Redacted
	}
	// Every step of the plan, and every EXPECTATION on it.
	//
	// The expectations matter as much as the actions, and are easier to overlook: a
	// verification condition for "set the text" necessarily contains the text the
	// control should end up holding, plus a human description quoting it. The audit
	// found both still carrying the value after the actions had been cleaned.
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		for i, s := range plan.Steps {
			plan.Steps[i].Action = redactAction(s.Action, secret)
			for j := range plan.Steps[i].Expect {
				redactCondition(&plan.Steps[i].Expect[j], secret)
			}
		}
		plan.Goal = strings.ReplaceAll(plan.Goal, secret, values.Redacted)
	}
	// The record's action, which is what the node's Plan snapshot is built from and
	// what a replay would otherwise read.
	if rec != nil {
		rec.Action = redactAction(rec.Action, secret)
		// The verifier's own account of what it compared. Its Reason and Evidence quote
		// the expected and observed text verbatim — which is the most useful diagnostic
		// there is and, for a sensitive value, the last place it may be kept.
		rec.Verification.Reason = strings.ReplaceAll(rec.Verification.Reason, secret, values.Redacted)
		for i, e := range rec.Verification.Evidence {
			rec.Verification.Evidence[i].Detail =
				strings.ReplaceAll(e.Detail, secret, values.Redacted)
		}
		rec.FailureReason = strings.ReplaceAll(rec.FailureReason, secret, values.Redacted)
		rec.Execution.Detail = strings.ReplaceAll(rec.Execution.Detail, secret, values.Redacted)
	}
}

// redactStages cleans the human-readable trace lines.
//
// Easy to forget and among the most exposed things here: stages are what `director
// execute` prints, what the service sends to every connected client, and what a bug
// report gets pasted into. The executor's own "verified: the control now contains …"
// line quotes the value in full, which is exactly the diagnostic you want for a public
// value and exactly the leak you cannot have for a private one.
func redactStages(use *valueUse, stages []Stage) {
	if use == nil || use.Val.Visibility() == values.VisibilityNormal {
		return
	}
	secret := use.Val.Plaintext()
	if secret == "" {
		return
	}
	for i := range stages {
		stages[i].Detail = strings.ReplaceAll(stages[i].Detail, secret, values.Redacted)
	}
}

// redactCondition strips the value out of one expectation, recursively.
func redactCondition(c *directorapi.Condition, secret string) {
	if c.Value == secret {
		c.Value = values.Redacted
	}
	c.Description = strings.ReplaceAll(c.Description, secret, values.Redacted)
	for i := range c.Sub {
		redactCondition(&c.Sub[i], secret)
	}
}

// redactAction replaces sensitive text inside one action.
//
// Type-switched rather than reflective: only the actions that CARRY text can leak it,
// and an action type added later should have to be considered here rather than silently
// handled by a generic walk that might miss a new field.
func redactAction(a directorapi.Action, secret string) directorapi.Action {
	switch act := a.(type) {
	case directorapi.EditAction:
		if act.Text == secret {
			act.Text = values.Redacted
		}
		return act
	case directorapi.TypeAction:
		if act.Text == secret {
			act.Text = values.Redacted
		}
		return act
	}
	return a
}

// Metadata keys for a mutation that consumed a program-local value.
//
// Enough to explain the action completely — what it depended on, what sort of thing
// that was, how freely it could be shown — and never the value itself. That is what
// makes the node both honest and safe to keep: a reader a month later can see that this
// edit typed a captured email without learning the email.
const (
	MetaInputKind       = "input_kind"
	MetaValueName       = "value_name"
	MetaValueKind       = "value_kind"
	MetaValueVisibility = "value_visibility"
	// MetaInputProgramValue is MetaInputKind's value for a consumed capture.
	MetaInputProgramValue = "program_value"
)

// noteValueUse records the value-reference metadata on a node.
//
// Recorded for EVERY consumed value, public or not. Replay has to refuse all of them —
// a public value is no more resurrectable than a private one, because the program that
// captured it is over either way — and the refusal reads from this metadata.
func noteValueUse(node *actiongraph.ActionNode, use *valueUse) {
	if node == nil || use == nil {
		return
	}
	if node.Metadata == nil {
		node.Metadata = map[string]any{}
	}
	node.Metadata[MetaInputKind] = MetaInputProgramValue
	node.Metadata[MetaValueName] = use.Ref.Name
	node.Metadata[MetaValueKind] = string(use.Val.Kind())
	node.Metadata[MetaValueVisibility] = string(use.Val.Visibility())
}

// DependedOnProgramValue reports whether a node consumed a program-local value, and
// which one.
//
// The name is safe to report — it is what the user called it — and is the whole content
// of the replay refusal. The value it stood for is not stored anywhere, which is what
// makes the refusal honest rather than a policy that could be overridden.
func DependedOnProgramValue(node actiongraph.ActionNode) (string, bool) {
	if node.Metadata == nil {
		return "", false
	}
	if kind, _ := node.Metadata[MetaInputKind].(string); kind != MetaInputProgramValue {
		return "", false
	}
	name, _ := node.Metadata[MetaValueName].(string)
	return name, true
}

// applicationOf names the application a window belongs to, for a safe event field.
//
// The application is public context — which program received the value, not what the
// value was — and is the single most useful thing in a consumption event: it is how a
// reader checks that a captured email went into the CRM rather than into a chat window.
func applicationOf(w directorapi.WorldState, window directorapi.WindowID) string {
	if window == "" {
		return ""
	}
	if win, ok := w.Window(window); ok {
		return win.Application
	}
	return ""
}

// Collection provenance on an iteration's node.
//
// Metadata keys for a mutation performed as one member of a bounded set. Enough to
// explain the iteration's lineage completely — which collection, which position, which
// member — and never enough to reconstruct the membership and replay it.
const (
	MetaCollectionName     = "collection_name"
	MetaCollectionKind     = "collection_kind"
	MetaCollectionQuery    = "collection_query_summary"
	MetaCollectionOrdering = "collection_ordering"
	MetaIterationIndex     = "iteration_index"
	MetaIterationLimit     = "iteration_limit"
	MetaMemberDigest       = "member_semantic_key_digest"
	MetaProgramID          = "program_id"
	MetaStepID             = "step_id"
)

// iterationProvenance is what one member's node records about its collection.
//
// Held on the pipeline for the duration of a single member, in the same way the step
// position is: every node-building path needs it and nothing else does. Cleared after
// the member, so an ordinary action that follows an iteration cannot inherit it.
type iterationProvenance struct {
	Name      string
	Kind      string
	Query     string
	Ordering  string
	Index     int
	Limit     int
	Digest    string
	ProgramID string
	StepID    string
}

// noteIteration records collection lineage on a member's node.
//
// The member's semantic key is stored as a DIGEST rather than in full. The key is built
// from the application, role, normalised label and native id — private text and provider
// identifiers — and the Action Graph is durable. The digest keeps what the metadata is
// for (detecting duplicate processing, explaining lineage) and drops what it is not for
// (reconstructing the member and acting on it again).
func noteIteration(node *actiongraph.ActionNode, prov *iterationProvenance) {
	if node == nil || prov == nil {
		return
	}
	if node.Metadata == nil {
		node.Metadata = map[string]any{}
	}
	if prov.Name != "" {
		node.Metadata[MetaCollectionName] = prov.Name
	}
	node.Metadata[MetaCollectionKind] = prov.Kind
	node.Metadata[MetaCollectionQuery] = prov.Query
	node.Metadata[MetaCollectionOrdering] = prov.Ordering
	node.Metadata[MetaIterationIndex] = prov.Index
	node.Metadata[MetaIterationLimit] = prov.Limit
	node.Metadata[MetaMemberDigest] = prov.Digest
	if prov.ProgramID != "" {
		node.Metadata[MetaProgramID] = prov.ProgramID
	}
	if prov.StepID != "" {
		node.Metadata[MetaStepID] = prov.StepID
	}
}

// PartOfCollection reports whether a node was one member of a bounded iteration.
//
// Used by replay: an action that only makes sense as part of a set cannot be replayed
// from the set's history, because the set no longer exists.
func PartOfCollection(node actiongraph.ActionNode) (string, bool) {
	if node.Metadata == nil {
		return "", false
	}
	name, ok := node.Metadata[MetaCollectionName].(string)
	if ok && name != "" {
		return name, true
	}
	// An inline collection has no name; the query summary is what identifies it.
	if q, has := node.Metadata[MetaCollectionQuery].(string); has && q != "" {
		return q, true
	}
	return "", false
}
