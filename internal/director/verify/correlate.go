package verify

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Correlating a result with the object that was bound.
//
//	Verification must establish that the observed result belongs to the same bound
//	object.
//
// The gap this closes is subtle and it is the one that makes a rename dangerous. Every
// check the Director had could be satisfied by the WRONG file: a label now reading
// "Budget.txt" is satisfied by any item that now reads "Budget.txt"; a destination file
// existing is satisfied by one that was already there; "the screen changed" is satisfied
// by a repaint. Each is consistent with success and equally consistent with having renamed
// the item next to the one the user meant.
//
// So correlation asks a different question — not "did something become the new name?" but
// "is the thing that became the new name the same object I was pointed at?" — and answers
// it from identity: the original resource is gone, the expected destination is there, the
// content is the content that was there before, and nothing else moved.
//
// Deliberately evidence-based rather than boolean. A correlation that could not be
// established is INCONCLUSIVE and says why, which is a different claim from "the wrong
// thing happened" and leads somewhere different.

// CorrelationMethod is how the observed result was tied to the bound object.
type CorrelationMethod string

const (
	// MethodResource: the backing paths were compared. The strongest available.
	MethodResource CorrelationMethod = "resource_identity"
	// MethodContent: the object's content was compared before and after, which is what
	// distinguishes a rename from a replacement.
	MethodContent CorrelationMethod = "content_identity"
	// MethodNativeID: the accessibility source's own identifier survived.
	MethodNativeID CorrelationMethod = "native_id"
	// MethodNone: nothing tied the result to the object. A LABEL match lands here, on
	// purpose: a caption is not identity.
	MethodNone CorrelationMethod = "none"
)

// Describe renders a method for a person.
func (m CorrelationMethod) Describe() string {
	switch m {
	case MethodResource:
		return "by comparing the file behind it"
	case MethodContent:
		return "by comparing what is inside it"
	case MethodNativeID:
		return "by the identifier the application gave it"
	case MethodNone:
		return "not established"
	}
	return string(m)
}

// CorrelationResult is the closed verdict vocabulary.
type CorrelationResult string

const (
	// Correlated: the observed result is the bound object, on identity evidence.
	Correlated CorrelationResult = "correlated"
	// Uncorrelated: the observed result is demonstrably NOT the bound object, or the
	// bound object was demonstrably not the thing that changed.
	Uncorrelated CorrelationResult = "uncorrelated"
	// CorrelationInconclusive: nothing available could settle it. Not a pass.
	CorrelationInconclusive CorrelationResult = "inconclusive"
)

// OK reports whether the result may be treated as verified.
func (r CorrelationResult) OK() bool { return r == Correlated }

// Origin locates a correlation in the request that produced it.
//
//	A verification mismatch must identify the originating goal, procedure, step, and
//	action node when available.
//
// Every field optional, because a correlation for a single direct request has no goal and
// no procedure and is no less valid for that.
type Origin struct {
	Goal       string `json:"goal,omitempty"`
	Procedure  string `json:"procedure,omitempty"`
	StepID     string `json:"step_id,omitempty"`
	StepIndex  int    `json:"step_index,omitempty"`
	ActionNode string `json:"action_node,omitempty"`
}

// Describe renders an origin in one line, empty when there is nothing to say.
func (o Origin) Describe() string {
	var parts []string
	if o.Goal != "" {
		parts = append(parts, "goal "+o.Goal)
	}
	if o.Procedure != "" {
		parts = append(parts, "procedure "+o.Procedure)
	}
	if o.StepIndex > 0 {
		parts = append(parts, fmt.Sprintf("step %d", o.StepIndex))
	}
	if o.ActionNode != "" {
		parts = append(parts, "node "+o.ActionNode)
	}
	return strings.Join(parts, ", ")
}

// Identity is one object, as identity rather than as appearance.
type Identity struct {
	// Resource is the backing path. The field that makes correlation possible.
	Resource string `json:"resource,omitempty"`
	// NativeID is the accessibility source's own identifier.
	NativeID string `json:"native_id,omitempty"`
	// Label is what it is called on screen. Recorded, never relied on — see MethodNone.
	Label string `json:"label,omitempty"`
	// ContentDigest summarises what is inside it, when that is knowable.
	ContentDigest string `json:"content_digest,omitempty"`
	// Exists reports whether the object was found at all.
	Exists bool `json:"exists"`
}

// Describe renders an identity in one line.
func (i Identity) Describe() string {
	switch {
	case i.Resource != "":
		return i.Resource
	case i.Label != "":
		return fmt.Sprintf("%q", i.Label)
	case i.NativeID != "":
		return "id " + i.NativeID
	}
	return "(nothing)"
}

// Correlation is the full account of tying one observed result to one bound object.
//
// Reusable across verbs: a rename fills Expected with the destination it asked for and a
// delete fills it with an absence, and both are answered by the same structure so a reader
// of a trace learns one shape rather than one per verb.
type Correlation struct {
	// Action is the verb this correlates ("rename", "delete").
	Action string `json:"action"`
	// Expected is the identity the action should have produced.
	Expected Identity `json:"expected"`
	// Observed is the identity that was actually found.
	Observed Identity `json:"observed"`
	// Method is how the two were tied together, MethodNone when they were not.
	Method CorrelationMethod `json:"method"`

	// Matching and Mismatching are the individual facts, each one a sentence. Kept
	// separately rather than as a score, because "the destination exists" and "the
	// original is gone" are different claims and only both together mean a rename.
	Matching    []string `json:"matching,omitempty"`
	Mismatching []string `json:"mismatching,omitempty"`

	// Confidence is 0..1, and is the fraction of the required evidence that was
	// positively established. It is reported, never thresholded here: the Result is the
	// decision.
	Confidence float64 `json:"confidence"`
	// Result is the verdict.
	Result CorrelationResult `json:"result"`
	// Reason is the verdict in one sentence.
	Reason string `json:"reason"`
	// Origin locates this in the request that produced it.
	Origin Origin `json:"origin,omitzero"`
}

// Describe renders the whole correlation for a trace.
func (c Correlation) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (%s)", c.Action, c.Result, c.Method.Describe())
	if c.Reason != "" {
		fmt.Fprintf(&b, " — %s", c.Reason)
	}
	if c.Result != Correlated {
		if where := c.Origin.Describe(); where != "" {
			fmt.Fprintf(&b, " [%s]", where)
		}
	}
	return b.String()
}

// Inspector reads the identity of a resource.
//
// Injected rather than implemented here for two reasons. The Director may be looking at a
// remote or virtual location where a stat is wrong; and a verification that reached the
// filesystem directly could not be tested without touching real files, which is precisely
// what this milestone must not do.
type Inspector interface {
	// Inspect returns what is at a resource. The second result is false when the
	// inspector cannot answer at all — which is INCONCLUSIVE, not absent.
	Inspect(resource string) (Identity, bool)
}

// RenameCorrelation is what a rename must be checked against.
type RenameCorrelation struct {
	// Bound is the object the user pointed at, as it was BEFORE the action.
	Bound Identity
	// RequestedName is the new name the user asked for.
	RequestedName string
	// Distractors are other objects that must not have moved. The half of the check
	// that catches renaming the item next to the intended one.
	Distractors []string
	// RequireContent demands that the content be shown to have survived. Set where a
	// rename must not be satisfied by a replacement that merely has the right name.
	RequireContent bool
	Origin         Origin
}

// CorrelateRename establishes whether the intended object became the requested name.
//
// The four facts, and why each alone is insufficient:
//
//   - The destination exists. Alone this is satisfied by a file that was already there.
//   - The original is gone. Alone this is satisfied by a deletion.
//   - The content is what it was. Alone this is satisfied by a copy.
//   - The distractors are untouched. Alone this says only that nothing else broke.
//
// Together they say: the object that was at A is now at B, unchanged, and nothing else
// moved. That is what a rename is, and no weaker combination distinguishes it from the
// mistakes it is confused with.
func CorrelateRename(c RenameCorrelation, insp Inspector) Correlation {
	out := Correlation{Action: "rename", Origin: c.Origin, Method: MethodNone}
	destinations := destinationsOf(c.Bound.Resource, c.RequestedName)
	if len(destinations) > 0 {
		out.Expected = Identity{Resource: destinations[0]}
	}

	if c.Bound.Resource == "" {
		// A label and nothing else. Refused as evidence rather than accepted as weak
		// evidence: a caption reading the new name is exactly what a rename of the WRONG
		// item also produces.
		out.Result = CorrelationInconclusive
		out.Reason = "the object had no file behind it, so there is no identity to check " +
			"the result against — a label now reading the new name would be equally true " +
			"if a different item had been renamed"
		out.Mismatching = append(out.Mismatching, "no backing resource to correlate")
		return out
	}
	if insp == nil {
		out.Result = CorrelationInconclusive
		out.Reason = "nothing could read the file behind the object, so the result could " +
			"not be tied to it"
		return out
	}

	required := 3
	if c.RequireContent {
		required = 4
	}
	established := 0

	// 1. The destination is there — under either legitimate name. See destinationsOf.
	var dest Identity
	var unreadable, missing []string
	for _, candidate := range destinations {
		found, known := insp.Inspect(candidate)
		if !known {
			unreadable = append(unreadable, candidate)
			continue
		}
		if !found.Exists {
			missing = append(missing, candidate)
			continue
		}
		dest = found
		out.Observed, out.Expected = found, Identity{Resource: candidate}
		out.Method = MethodResource
		break
	}
	switch {
	case dest.Exists:
		established++
		out.Matching = append(out.Matching, out.Expected.Resource+" is there")
	case len(unreadable) > 0 && len(missing) == 0:
		out.Mismatching = append(out.Mismatching, "could not read "+strings.Join(unreadable, " or "))
	default:
		out.Mismatching = append(out.Mismatching,
			strings.Join(append(missing, unreadable...), " or ")+" is not there")
	}

	// 2. The original is gone. Without this, "the destination exists" is satisfied by a
	// file that was already there and a rename that never happened.
	orig, origKnown := insp.Inspect(c.Bound.Resource)
	switch {
	case !origKnown:
		out.Mismatching = append(out.Mismatching, "could not read "+c.Bound.Resource)
	case orig.Exists:
		out.Mismatching = append(out.Mismatching,
			c.Bound.Resource+" is still there, so it was not what was renamed")
	default:
		established++
		out.Matching = append(out.Matching, c.Bound.Resource+" is gone")
	}

	// 3. Nothing else moved.
	moved := movedDistractors(c.Distractors, insp)
	if len(moved) == 0 {
		established++
		if len(c.Distractors) > 0 {
			out.Matching = append(out.Matching, fmt.Sprintf(
				"the %d other item(s) nearby are untouched", len(c.Distractors)))
		}
	} else {
		out.Mismatching = append(out.Mismatching, fmt.Sprintf(
			"%s moved, which the rename should not have touched", strings.Join(moved, ", ")))
	}

	// 4. It is the same object, not a replacement with the right name.
	if c.RequireContent {
		switch {
		case c.Bound.ContentDigest == "" || dest.ContentDigest == "":
			out.Mismatching = append(out.Mismatching,
				"the content could not be compared, so a replacement cannot be ruled out")
		case c.Bound.ContentDigest == dest.ContentDigest:
			established++
			out.Method = MethodContent
			out.Matching = append(out.Matching, "the content is unchanged")
		default:
			out.Mismatching = append(out.Mismatching,
				"the content differs, so this is a different object with the requested name")
		}
	}

	out.Confidence = float64(established) / float64(required)
	switch {
	case established == required:
		out.Result = Correlated
		out.Reason = fmt.Sprintf("%s became %s, unchanged, and nothing else moved",
			c.Bound.Resource, out.Expected.Resource)
	case len(out.Mismatching) > 0 && out.Method == MethodNone:
		out.Result = CorrelationInconclusive
		out.Reason = "nothing tied the result to the object that was pointed at: " +
			strings.Join(out.Mismatching, "; ")
	default:
		out.Result = Uncorrelated
		out.Reason = strings.Join(out.Mismatching, "; ")
	}
	return out
}

// movedDistractors reports which of the objects that should not have changed did.
func movedDistractors(paths []string, insp Inspector) []string {
	var moved []string
	for _, p := range paths {
		id, known := insp.Inspect(p)
		if !known {
			// Unknown is not evidence of absence. It is reported as moved so the
			// correlation errs toward refusing, which is the recoverable direction.
			moved = append(moved, p+" (could not be read)")
			continue
		}
		if !id.Exists {
			moved = append(moved, p)
		}
	}
	return moved
}

// destinationsOf is where a rename could legitimately have put an object.
//
// In the same folder, by definition: a rename that changed the folder would be a move, and
// treating the two as one is how "rename this to Budget" quietly relocates a file.
//
// TWO candidates, not one, and the reason is a setting the Director cannot see. A user who
// says "rename this to Budget" with file extensions hidden gets `Budget.txt`, and with
// them shown gets `Budget`. Both are correct renames of the bound object; which one happens
// is Explorer's business. Insisting on one would report a successful rename as a failure
// on half of all machines, and the check that matters — the original is gone, exactly one
// thing moved, the content survived — is unaffected either way.
func destinationsOf(resource, name string) []string {
	if resource == "" || name == "" {
		return nil
	}
	dir := filepath.Dir(resource)
	exact := filepath.Join(dir, name)
	out := []string{exact}
	if filepath.Ext(name) == "" {
		if ext := filepath.Ext(resource); ext != "" {
			out = append(out, exact+ext)
		}
	}
	return out
}

// ── one action, one bound object ──────────────────────────────────────────────

// CorrelateTarget establishes that what an action acted on is the object that was bound.
//
//	Verification must establish that the observed result belongs to the same bound
//	object.
//
// The cheap, always-available half of the claim, and it runs after every action that
// carried a binding. It compares IDENTITY — backing resource first, then the source's own
// identifier — and treats a matching label as no evidence at all, for the reason a label
// is never evidence here: two objects in different places share a caption.
//
// A missing observation is INCONCLUSIVE rather than a failure: plenty of actions leave
// nothing to compare, and reporting those as wrong-object would make the check noise that
// people learn to ignore.
func CorrelateTarget(action string, expected, observed Identity, origin Origin) Correlation {
	c := Correlation{
		Action: action, Expected: expected, Observed: observed,
		Method: MethodNone, Origin: origin,
	}
	switch {
	case expected.Resource != "" && observed.Resource != "":
		c.Method = MethodResource
		if strings.EqualFold(expected.Resource, observed.Resource) {
			c.Result, c.Confidence = Correlated, 1
			c.Matching = append(c.Matching, "acted on "+expected.Resource)
			c.Reason = "the action landed on the object that was bound"
			return c
		}
		c.Result, c.Confidence = Uncorrelated, 0
		c.Mismatching = append(c.Mismatching, fmt.Sprintf(
			"bound %s and acted on %s", expected.Resource, observed.Resource))
		c.Reason = c.Mismatching[0]
		return c

	case expected.NativeID != "" && observed.NativeID != "":
		c.Method = MethodNativeID
		if expected.NativeID == observed.NativeID {
			c.Result, c.Confidence = Correlated, 1
			c.Matching = append(c.Matching, "acted on the element that was bound")
			c.Reason = c.Matching[0]
			return c
		}
		c.Result, c.Confidence = Uncorrelated, 0
		c.Mismatching = append(c.Mismatching, fmt.Sprintf(
			"bound element %s and acted on %s", expected.NativeID, observed.NativeID))
		c.Reason = c.Mismatching[0]
		return c
	}

	c.Result = CorrelationInconclusive
	c.Reason = "there was no identity on both sides to compare, so which object was " +
		"acted on could not be established from the result alone"
	return c
}

// AsEvidence renders a correlation as a verification evidence line, for the record and
// the action-graph node.
//
// Weighted at 1 for a settled verdict and 0 for an inconclusive one, which is what the
// weight means everywhere else here: this is near-proof when identity could be compared
// and contributes nothing when it could not.
func (c Correlation) AsEvidence() directorapi.Evidence {
	weight := 1.0
	if c.Result == CorrelationInconclusive {
		weight = 0
	}
	return directorapi.Evidence{
		Kind:     "binding_correlation",
		Observed: c.Result == Correlated,
		Detail:   c.Describe(),
		Weight:   weight,
	}
}
