package execute

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verification that belongs to the bound object.
//
//	Verification runs automatically after real execution.
//	Verification failure must become an execution failure.
//	Verification success must attach evidence to the resulting graph node.
//
// Two checks at two scopes, both automatic, and neither requiring goals or replay to call
// anything by hand.
//
// The PER-ACTION one asks whether the thing that was acted on is the thing that was bound.
// It runs after every action carrying a binding, costs one comparison, and needs nothing
// outside the world already observed.
//
// The PER-GOAL one asks the question that only makes sense once the whole procedure has
// run: for a rename, did the bound file become the requested name, unchanged, with nothing
// else moved. It needs to read the filesystem, which is why the Inspector is injected and
// why its absence is reported as inconclusive rather than assumed to be fine.

// correlateTarget checks that the action landed on the bound object.
//
// Reports ok=false when there is no binding — most actions — so the caller adds nothing to
// the record rather than adding an empty verdict.
func (p *Pipeline) correlateTarget(ctx context.Context, in directorapi.Intent,
	resolved directorapi.ResolvedTarget, after directorapi.WorldState) (verify.Correlation, bool) {

	b := bindingFor(ctx, in)
	if !b.Bound() {
		return verify.Correlation{}, false
	}
	expected := verify.Identity{
		Resource: b.Resource, NativeID: b.NativeID, Label: b.Label, Exists: true,
	}
	observed := verify.Identity{
		NativeID: resolved.NativeID, Label: resolved.Label, Exists: resolved.ElementID != "",
	}
	// The acted-on element's own backing path, read from the world it was acted in.
	// Taken from the AFTER world rather than the binding, which is the point: it is the
	// observation that has to agree, not the record of what was intended.
	if el, ok := after.Element(resolved.ElementID); ok {
		observed.Resource = resourceOfElement(el)
	}
	return verify.CorrelateTarget(actionName(in), expected, observed, originOf(b, p)), true
}

// resourceOfElement reads a backing path off an element, empty when none is reported.
//
// The same attribute keys the binding layer reads, deliberately: a correlation that looked
// somewhere else could disagree with the binding about what an object's path is, and the
// disagreement would read as a wrong-object failure.
func resourceOfElement(el *directorapi.Element) string {
	for _, key := range []string{"path", "file_path", "resource", "uri", "full_path"} {
		if v, ok := el.Attributes[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// actionName is the verb a correlation reports against.
func actionName(in directorapi.Intent) string {
	if kind, ok := in.Parameters["semantic_kind"].(string); ok && kind != "" {
		return kind
	}
	return in.Verb
}

// originOf locates a correlation in the request that produced it.
func originOf(b *binding.Binding, p *Pipeline) verify.Origin {
	o := verify.Origin{
		Goal: b.Origin.Goal, Procedure: b.Origin.Procedure,
		StepID: b.Origin.StepID, StepIndex: b.Origin.StepIndex,
	}
	if prov := p.goalProvenance; prov != nil {
		if o.Goal == "" {
			o.Goal = prov.Goal
		}
		if o.Procedure == "" {
			o.Procedure = prov.Procedure
		}
	}
	if p.stepIndex > 0 {
		o.StepIndex, o.StepID = p.stepIndex, p.stepID
	}
	return o
}

// ── the whole-goal check ──────────────────────────────────────────────────────

// renameCheck is what a rename must be shown to have done, prepared BEFORE it runs.
//
// Prepared early because half of it is only knowable then: the content that must survive
// has to be read while the file still exists under its old name, and the files that must
// not move have to be listed while they are all still there.
type renameCheck struct {
	Bound       verify.Identity
	Requested   string
	Distractors []string
	Origin      verify.Origin
}

// prepareRenameCheck captures what a rename will have to be judged against.
//
// Returns nil for anything that is not a rename of a bound file, or when nothing can read
// the filesystem — in which case the ordinary per-step verification is all there is, and
// the diagnostics say so rather than implying a check that did not happen.
func (p *Pipeline) prepareRenameCheck(ex goal.Expansion, w *directorapi.WorldState) *renameCheck {
	if p.Resources == nil || ex.Goal.Kind != goal.Rename {
		return nil
	}
	b := firstBinding(ex)
	if !b.Bound() || b.Resource == "" {
		return nil
	}
	name := ex.Goal.Param(goal.ParamName)
	if name == "" {
		return nil
	}

	bound := verify.Identity{
		Resource: b.Resource, NativeID: b.NativeID, Label: b.Label, Exists: true,
	}
	// The content, read while the file still exists under its old name. Without this a
	// replacement carrying the requested name would be indistinguishable from a rename.
	if id, ok := p.Resources.Inspect(b.Resource); ok {
		bound.ContentDigest = id.ContentDigest
	}

	return &renameCheck{
		Bound: bound, Requested: name,
		Distractors: siblingResources(w, b.Resource),
		Origin: verify.Origin{
			Goal: string(ex.Goal.Kind), Procedure: ex.Procedure,
			StepID: b.Origin.StepID, StepIndex: b.Origin.StepIndex,
		},
	}
}

// run performs the correlation once the program has finished.
func (c *renameCheck) run(insp verify.Inspector, node string) verify.Correlation {
	c.Origin.ActionNode = node
	return verify.CorrelateRename(verify.RenameCorrelation{
		Bound: c.Bound, RequestedName: c.Requested, Distractors: c.Distractors,
		// Content correlation is REQUIRED for a rename: a file with the right name and
		// different contents is a replacement, and reporting it as a successful rename
		// would be the most expensive kind of wrong.
		RequireContent: c.Bound.ContentDigest != "",
		Origin:         c.Origin,
	}, insp)
}

// correlateGoal runs the whole-goal check and folds its verdict into the outcome.
//
// After execution, before the outcome is reported, and unconditional: neither goals nor
// replay has to remember to call it. A mismatch is an EXECUTION FAILURE — the per-step
// checks can only say that something changed in the expected shape, and that is satisfied
// by the expected change happening to the wrong object.
//
// An inconclusive correlation is recorded and does not fail the request: it means nothing
// could read the result back, which is a gap in evidence rather than a wrong outcome.
func (p *Pipeline) correlateGoal(out *ProgramOutcome, check *renameCheck) {
	if check == nil || p.Resources == nil || out.Status != directorapi.ResultDone {
		return
	}
	node := ""
	if n := len(out.Steps); n > 0 && out.Steps[n-1].Node != nil {
		node = string(out.Steps[n-1].Node.ID)
	}
	corr := check.run(p.Resources, node)
	out.Correlation = &corr

	switch corr.Result {
	case verify.Correlated:
		out.Message = corr.Reason
	case verify.Uncorrelated:
		out.Status = directorapi.ResultFailed
		out.Program.Status = program.StatusFailed
		out.Message = "the steps ran, and the result is not the object you pointed at: " +
			corr.Reason
	default:
		// Inconclusive. Reported as PARTIAL rather than done: the steps verified and the
		// outcome could not be confirmed, which is exactly what partial means everywhere
		// else here.
		out.Status = directorapi.ResultPartial
		out.Message = "the steps ran and the result could not be checked against the " +
			"object you pointed at: " + corr.Reason
	}
}

// siblingResources lists the other file-backed objects that must not have moved.
//
// From the world the binding was made against, which is the only place they are all
// visible: after the rename one of them has a different name, and a list built afterwards
// would be a list of what survived rather than of what was there.
func siblingResources(w *directorapi.WorldState, exclude string) []string {
	if w == nil {
		return nil
	}
	seen := map[string]bool{exclude: true}
	var out []string
	for _, el := range w.Elements {
		res := resourceOfElement(el)
		if res == "" || seen[res] {
			continue
		}
		seen[res] = true
		out = append(out, res)
	}
	sortStrings(out)
	return out
}

// sortStrings keeps the distractor list stable, so a failure names them the same way
// every run.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
