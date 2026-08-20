// Package compile turns action and script bodies into Block sequences of edges
// that the runtime can execute.
//
// Scope: the whole of Marco Core v1 (spec/Core.md) plus the supported extensions listed
// there. The "MVP / SaveApp target" this comment used to name has not described this file
// for a long time — 108 fixtures under testdata/ exercise contracts, translators, feeds,
// queues, channels, locking, concurrency and tests, and every one of them comes through here.
package compile

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Severity classifies a diagnostic. Error stops compilation; Warning and Info
// are advisory (currently unused at issue-sites — reserved for the warning
// pass once added). The model exists so machine-readable diagnostics can
// distinguish them.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	}
	return "error"
}

// Error is both a Go error and a structured diagnostic. The Code field is a
// short stable identifier ("dead-arm", "unreachable", …) for editor tooling.
// Severity defaults to SeverityError when zero-valued.
type Error struct {
	Msg      string
	Pos      ast.Sentence
	Code     string
	Severity Severity
}

func (e *Error) Error() string {
	if len(e.Pos.Parts) > 0 {
		return fmt.Sprintf("%s: %s", e.Pos.Pos, e.Msg)
	}
	return e.Msg
}

// Errors collects multiple compile diagnostics. It implements error so callers
// that only inspect the first message keep working; callers that want the full
// list can type-assert via errors.As(err, &compile.Errors{}). Sorted by source
// position when emitted by Compile.
type Errors []*Error

func (es Errors) Error() string {
	if len(es) == 0 {
		return ""
	}
	if len(es) == 1 {
		return es[0].Error()
	}
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}

// Unwrap exposes the individual errors for errors.Is / errors.As traversal.
func (es Errors) Unwrap() []error {
	out := make([]error, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

// asCompileError lifts any error to a *Error so it can be aggregated. If err
// is already a *Error or Errors, the existing structure is preserved.
func asCompileError(err error) []*Error {
	if single, ok := errors.AsType[*Error](err); ok {
		return []*Error{single}
	}
	if multi, ok := errors.AsType[Errors](err); ok {
		return multi
	}
	return []*Error{{Msg: err.Error()}}
}

// sortErrors orders diagnostics by file position so editor output is stable.
// Errors without a position sort last (in stable insertion order among
// themselves).
func sortErrors(es []*Error) {
	sort.SliceStable(es, func(i, j int) bool {
		ai, aj := len(es[i].Pos.Parts) > 0, len(es[j].Pos.Parts) > 0
		if ai != aj {
			return ai
		}
		if !ai {
			return false
		}
		pi, pj := es[i].Pos.Pos, es[j].Pos.Pos
		if pi.Line != pj.Line {
			return pi.Line < pj.Line
		}
		return pi.Col < pj.Col
	})
}

func Compile(g *graph.Graph, _ *ast.File) error {
	c := &compiler{g: g}

	for _, name := range g.Order {
		n := g.Nodes[name]
		if (n.Kind == graph.KindAction || n.Kind == graph.KindTranslator) && n.Body != nil {
			c.currentAction = n
			block, err := c.compileBlock(n.Body.Body)
			c.currentAction = nil
			if err != nil {
				return err
			}
			n.BodyBlock = block
		}
	}

	if script := g.Script(); script != nil && len(script.ScriptBody) > 0 {
		block, err := c.compileBlock(script.ScriptBody)
		if err != nil {
			return err
		}
		script.BodyBlock = block
	}

	for _, t := range g.Tests {
		if len(t.ScriptBody) == 0 {
			continue
		}
		c.currentTest = t
		block, err := c.compileBlock(t.ScriptBody)
		c.currentTest = nil
		if err != nil {
			return err
		}
		t.BodyBlock = block
	}

	for _, ld := range g.Listeners {
		block, err := c.compileBlock(ld.Source)
		if err != nil {
			return err
		}
		ld.Body = block
	}

	for _, dd := range g.DefaultDecls {
		expr, err := c.parseExpr(dd.Parts, &dd.Pos)
		if err != nil {
			return err
		}
		field := dd.TypeNode.FieldByName(dd.Field)
		if field == nil {
			return fmt.Errorf("%s: %s has no field %q for default", dd.Pos.Pos, dd.TypeNode.Name, dd.Field)
		}
		field.Default = expr
	}
	// applyInlineMocks rewrites graph nodes in place; downstream checks assume
	// it succeeded, so this stage still fails fast.
	if err := c.applyInlineMocks(); err != nil {
		return err
	}
	// markForeign flags exported, bodyless act capabilities as foreign before
	// any check pass reads the flag. Must run after body compilation (so a
	// `this's X does...` that follows the `exports` has been attached) and
	// before the checks slice.
	c.markForeign()
	// All check passes share c.errs. Each pass appends; we run them all so a
	// single editor invocation can surface multiple, independent diagnostics
	// (e.g. a dead arm AND a missing return). A pass may also return an error
	// directly for the legacy single-error path — those are merged in.
	checks := []func() error{
		c.checkExhaustiveness,
		c.checkDeadArms,
		c.checkUnreachable,
		c.checkInputs,
		c.checkCasts,
		c.checkThatRefs,
		c.checkRootClosure,
		c.checkAllowedShapes,
		c.checkNoReentrancy,
		c.checkPhrasesResolve,
		c.checkExportsBelongToActs,
	}
	for _, fn := range checks {
		before := len(c.errs)
		if err := fn(); err != nil {
			// Avoid double-reporting when the pass already used c.report.
			if len(c.errs) == before {
				c.errs = append(c.errs, asCompileError(err)...)
			}
		}
	}
	if len(c.errs) == 0 {
		return nil
	}
	sortErrors(c.errs)
	return Errors(c.errs)
}

// resolveSelfRel resolves `this's <Cap>` against the action currently being
// compiled. If the resolution succeeds, the returned action lets later passes
// (input checking, dead-arm, exhaustiveness) run as if the call were fully
// qualified. If we can't tell who `this` is (compiling a listener body, a
// test, or a script), return (nil, nil) and let the runtime resolve.
func (c *compiler) resolveSelfRel(capName string, s *ast.Sentence) (*graph.Node, error) {
	if c.currentAction == nil || c.currentAction.Owner == nil {
		return nil, nil
	}
	owner := c.currentAction.Owner
	cap := owner.CapByName(capName)
	if cap == nil {
		return nil, &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q%s",
			owner.Name, capName, suggestClosest(capName, capNames(owner)))}
	}
	return cap.Action, nil
}

// capNames lists the cap names declared on n. Used for did-you-mean.
func capNames(n *graph.Node) []string {
	names := make([]string, 0, len(n.Caps))
	for _, c := range n.Caps {
		names = append(names, c.Name)
	}
	return names
}

// applyInlineMocks replaces the body of every action targeted by an inline
// mock (`mock <Owner>'s <Cap> is <Status> [with <expr>].`) with a synthesized
// emission edge. Subsequent passes (exhaustiveness, allowed-shapes) validate
// the synthesized body against the action's contract just like a real return.
func (c *compiler) applyInlineMocks() error {
	for _, im := range c.g.InlineMocks {
		owner, ok := c.g.Nodes[im.Owner]
		if !ok {
			return &Error{Pos: im.Pos, Msg: fmt.Sprintf("mock target %q is not a known actor or contract", im.Owner)}
		}
		cap := owner.CapByName(im.Cap)
		if cap == nil || cap.Action == nil {
			return &Error{Pos: im.Pos, Msg: fmt.Sprintf("%s has no capability %q to mock%s",
				im.Owner, im.Cap, suggestClosest(im.Cap, capNames(owner)))}
		}
		parts := make([]ast.Part, 0, len(im.Body)+1)
		parts = append(parts, ast.Part{Kind: ast.PartWord, Value: "this", Pos: im.Pos.Pos})
		parts = append(parts, im.Body...)
		synth := &ast.Sentence{Parts: parts, Term: ast.TermBang, Pos: im.Pos.Pos}
		edges, err := c.compileReturn(synth)
		if err != nil {
			return err
		}
		cap.Action.BodyBlock = &graph.Block{Edges: edges}
		cap.Action.InferredStatuses = nil
	}
	return nil
}

// checkThatRefs validates that `that's <Field>` accesses inside an arm body
// (`when that is <Status>?`) target fields that exist on the callee's
// contract's allowed-status shape for that status. Skips when the callee
// has no adopted contract (only inferred), when the arm uses a non-status
// predicate, or when the field is `error` (always available on a failed
// frame).
func (c *compiler) checkThatRefs() error {
	type ctx struct {
		callee        *graph.Node
		matchedStatus string
	}
	var visitEdges func(host *graph.Node, edges []graph.Edge, parent ctx) error
	checkExpr := func(e graph.Expr, pos ast.Sentence, cur ctx) error {
		var walk func(e graph.Expr) error
		walk = func(e graph.Expr) error {
			switch x := e.(type) {
			case graph.RefExpr:
				if len(x.Path) < 2 || x.Path[0] != "that" {
					return nil
				}
				if cur.callee == nil || cur.matchedStatus == "" {
					return nil
				}
				field := x.Path[1]
				if field == "error" {
					return nil
				}
				contractName := cur.callee.AdoptedContractName
				if contractName == "" {
					return nil
				}
				contract, ok := c.g.Nodes[contractName]
				if !ok || contract.Kind != graph.KindContract {
					return nil
				}
				var shape *graph.Node
				var found bool
				for i := range contract.AllowedStatuses {
					as := &contract.AllowedStatuses[i]
					if as.Name == cur.matchedStatus {
						shape = as.Shape
						found = true
						break
					}
				}
				if !found {
					return nil
				}
				if shape == nil {
					return &Error{Code: "that-field", Pos: x.Pos, Msg: fmt.Sprintf(
						"`that's %s` is invalid: %s emits `%s` with no value",
						field, cur.callee.Name, cur.matchedStatus)}
				}
				if shape.Kind == graph.KindSet && shape.FieldByName(field) == nil {
					return &Error{Code: "that-field", Pos: x.Pos, Msg: fmt.Sprintf(
						"`that's %s` is invalid: %s.%s shape (%s) has no field %q",
						field, cur.callee.Name, cur.matchedStatus, shape.Name, field)}
				}
			case graph.BinaryExpr:
				if err := walk(x.L); err != nil {
					return err
				}
				return walk(x.R)
			case graph.ConstructExpr:
				for _, f := range x.Fields {
					if err := walk(f.Expr); err != nil {
						return err
					}
				}
			case graph.CastExpr:
				return walk(x.Source)
			case graph.ListExpr:
				for _, item := range x.Items {
					if err := walk(item); err != nil {
						return err
					}
				}
			case graph.ListAtExpr:
				if err := walk(x.List); err != nil {
					return err
				}
				return walk(x.Index)
			}
			return nil
		}
		_ = pos
		if e == nil {
			return nil
		}
		return walk(e)
	}
	checkPred := func(p graph.Pred, cur ctx) error {
		var walk func(p graph.Pred) error
		walk = func(p graph.Pred) error {
			switch x := p.(type) {
			case graph.AndPred:
				for _, sub := range x.Sub {
					if err := walk(sub); err != nil {
						return err
					}
				}
			case graph.OrPred:
				for _, sub := range x.Sub {
					if err := walk(sub); err != nil {
						return err
					}
				}
			case graph.NotPred:
				return walk(x.Sub)
			case graph.EqPred:
				return checkExpr(x.RHS, ast.Sentence{}, cur)
			case graph.NeqPred:
				return checkExpr(x.RHS, ast.Sentence{}, cur)
			}
			return nil
		}
		if p == nil {
			return nil
		}
		return walk(p)
	}
	visitEdges = func(host *graph.Node, edges []graph.Edge, parent ctx) error {
		cur := parent
		for i := range edges {
			e := &edges[i]
			switch e.Kind {
			case graph.EdgeBranchOpen:
				armCtx := cur
				if sp, ok := e.Pred.(graph.StatusPred); ok {
					if matchesFrameRef(sp.Ref, "that") {
						armCtx.matchedStatus = sp.Status
					}
				}
				if err := checkPred(e.Pred, cur); err != nil {
					return err
				}
				_ = armCtx
				// Inline body edges follow until the next group marker; they
				// inherit armCtx.
				j := i + 1
				for j < len(edges) {
					ej := &edges[j]
					if ej.Branch == e.Branch &&
						(ej.Kind == graph.EdgeBranchOpen ||
							ej.Kind == graph.EdgeBranchFallback ||
							ej.Kind == graph.EdgeBranchClose) {
						break
					}
					j++
				}
				if err := visitEdges(host, edges[i+1:j], armCtx); err != nil {
					return err
				}
				continue
			case graph.EdgeBranchFallback:
				armCtx := cur
				armCtx.matchedStatus = ""
				j := i + 1
				for j < len(edges) {
					ej := &edges[j]
					if ej.Branch == e.Branch &&
						(ej.Kind == graph.EdgeBranchOpen ||
							ej.Kind == graph.EdgeBranchFallback ||
							ej.Kind == graph.EdgeBranchClose) {
						break
					}
					j++
				}
				if err := visitEdges(host, edges[i+1:j], armCtx); err != nil {
					return err
				}
				continue
			case graph.EdgeBranchClose:
				continue
			case graph.EdgeInvokePhrase:
				if e.Action != nil {
					cur.callee = e.Action
					cur.matchedStatus = ""
				}
				if err := checkExpr(e.Expr, e.Pos, cur); err != nil {
					return err
				}
				continue
			case graph.EdgeStart, graph.EdgeExecute:
				if err := checkExpr(e.Expr, e.Pos, cur); err != nil {
					return err
				}
				if e.Body != nil {
					if err := visitEdges(host, e.Body.Edges, cur); err != nil {
						return err
					}
				}
				continue
			case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil,
				graph.EdgeLock, graph.EdgeFinally:
				if err := checkPred(e.Pred, cur); err != nil {
					return err
				}
				if e.Body != nil {
					if err := visitEdges(host, e.Body.Edges, cur); err != nil {
						return err
					}
				}
				continue
			}
			if err := checkExpr(e.Expr, e.Pos, cur); err != nil {
				return err
			}
			if err := checkPred(e.Pred, cur); err != nil {
				return err
			}
			if e.Ref != nil {
				if err := checkExpr(graph.RefExpr{Path: e.Ref, Pos: e.Pos}, e.Pos, cur); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.BodyBlock != nil {
			c.collect(visitEdges(n, n.BodyBlock.Edges, ctx{}))
		}
	}
	if script := c.g.Script(); script != nil && script.BodyBlock != nil {
		c.collect(visitEdges(nil, script.BodyBlock.Edges, ctx{}))
	}
	for _, t := range c.g.Tests {
		c.collect(visitEdges(nil, t.BodyBlock.Edges, ctx{}))
	}
	for _, ld := range c.g.Listeners {
		c.collect(visitEdges(nil, ld.Body.Edges, ctx{}))
	}
	return nil
}

// checkInputs verifies that every `do`/`start`/`execute` site supplies an
// input value compatible with the callee's declared input type. ConstructExpr
// values without an explicit Type adopt the declared shape (matching the
// behavior already used for return values in checkAllowedShapes).
func (c *compiler) checkInputs() error {
	var visit func(host *graph.Node, b *graph.Block)
	visit = func(host *graph.Node, b *graph.Block) {
		if b == nil {
			return
		}
		for i := range b.Edges {
			e := &b.Edges[i]
			switch e.Kind {
			case graph.EdgeInvokePhrase, graph.EdgeStart, graph.EdgeExecute:
				if e.Action == nil || e.Action.InputType == nil {
					if e.Body != nil {
						visit(host, e.Body)
					}
					continue
				}
				want := e.Action.InputType
				if e.Expr == nil {
					c.report(&Error{Code: "missing-input", Pos: e.Pos, Msg: fmt.Sprintf(
						"%s requires input of type %s but none was supplied",
						e.Action.Name, want.Name)})
				} else if ce, ok := e.Expr.(graph.ConstructExpr); ok && ce.Type == nil {
					ce.Type = want
					if want.Kind == graph.KindSet {
						for _, fld := range ce.Fields {
							if want.FieldByName(fld.Name) == nil {
								c.report(&Error{Code: "input-type", Pos: e.Pos, Msg: fmt.Sprintf(
									"%s expects %s but the value has no field %q",
									e.Action.Name, want.Name, fld.Name)})
							}
						}
					}
					e.Expr = ce
				} else {
					got := c.inferExprType(host, e.Expr)
					if got != nil && !shapeCompatible(got, want) {
						c.report(&Error{Code: "input-type", Pos: e.Pos, Msg: fmt.Sprintf(
							"%s expects input of type %s but got %s",
							e.Action.Name, want.Name, got.Name)})
					}
				}
				if e.Body != nil {
					visit(host, e.Body)
				}
			case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil,
				graph.EdgeLock, graph.EdgeFinally:
				visit(host, e.Body)
			}
		}
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.BodyBlock != nil {
			visit(n, n.BodyBlock)
		}
	}
	if script := c.g.Script(); script != nil && script.BodyBlock != nil {
		visit(nil, script.BodyBlock)
	}
	for _, t := range c.g.Tests {
		visit(nil, t.BodyBlock)
	}
	for _, ld := range c.g.Listeners {
		visit(nil, ld.Body)
	}
	return nil
}

// checkCasts validates `<expr> as a [partial] <Target>` at compile time. The
// runtime will accept any of: same-type identity, structural-bridge promotion
// (target's required fields all present on source), or a declared translator
// from source-type to target-type. If the source type is statically known and
// none of these apply, reject. `partial` casts always succeed structurally
// (they yield a bridge whose missing fields require `has` proof later) so the
// only check is that the target type exists, which the parser already enforces.
func (c *compiler) checkCasts() error {
	hasTranslator := func(src, dst *graph.Node) bool {
		for _, n := range c.g.Nodes {
			if n.Kind != graph.KindTranslator {
				continue
			}
			if n.OutputType != dst {
				continue
			}
			if src != nil && n.InputType != nil && n.InputType != src {
				continue
			}
			return true
		}
		return false
	}
	canCast := func(src, dst *graph.Node) bool {
		if src == nil || dst == nil {
			return true
		}
		if src == dst {
			return true
		}
		if dst.Kind == graph.KindSet && src.Kind == graph.KindSet {
			ok := true
			for _, fld := range dst.Fields {
				if src.FieldByName(fld.Name) == nil {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return hasTranslator(src, dst)
	}
	// Resolve the static type of an expression, augmented with per-block local
	// bindings (`the X is V.` creates a local with whatever V's type is). Falls
	// back to inferExprType for non-local refs.
	inferType := func(host *graph.Node, locals map[string]*graph.Node, e graph.Expr) *graph.Node {
		if x, ok := e.(graph.RefExpr); ok && len(x.Path) == 1 {
			if t, ok := locals[x.Path[0]]; ok {
				return t
			}
			if n, ok := c.g.Nodes[x.Path[0]]; ok && n.Kind == graph.KindSet {
				return n
			}
		}
		return c.inferExprType(host, e)
	}
	var checkExpr func(host *graph.Node, locals map[string]*graph.Node, e graph.Expr, pos ast.Sentence)
	checkExpr = func(host *graph.Node, locals map[string]*graph.Node, e graph.Expr, pos ast.Sentence) {
		switch x := e.(type) {
		case graph.CastExpr:
			checkExpr(host, locals, x.Source, pos)
			if x.Partial {
				return
			}
			src := inferType(host, locals, x.Source)
			if !canCast(src, x.Target) {
				srcName := "<unknown>"
				if src != nil {
					srcName = src.Name
				}
				c.report(&Error{Code: "cast-impossible", Pos: x.Pos, Msg: fmt.Sprintf(
					"no cast from %s to %s — declare a translator (e.g. `the %sTo%s is a translator.`) or use `as a partial %s` for a bridge",
					srcName, x.Target.Name, srcName, x.Target.Name, x.Target.Name)})
			}
		case graph.BinaryExpr:
			checkExpr(host, locals, x.L, pos)
			checkExpr(host, locals, x.R, pos)
		case graph.ConstructExpr:
			for _, f := range x.Fields {
				checkExpr(host, locals, f.Expr, pos)
			}
		case graph.ListExpr:
			for _, item := range x.Items {
				checkExpr(host, locals, item, pos)
			}
		case graph.ListAtExpr:
			checkExpr(host, locals, x.List, pos)
			checkExpr(host, locals, x.Index, pos)
		}
	}
	var visit func(host *graph.Node, b *graph.Block, locals map[string]*graph.Node)
	visit = func(host *graph.Node, b *graph.Block, parent map[string]*graph.Node) {
		if b == nil {
			return
		}
		// Each block snapshots its parent's locals; child bindings shadow but
		// don't leak back to siblings of the parent.
		locals := make(map[string]*graph.Node, len(parent))
		maps.Copy(locals, parent)
		for i := range b.Edges {
			e := &b.Edges[i]
			if e.Expr != nil {
				checkExpr(host, locals, e.Expr, e.Pos)
			}
			if e.Kind == graph.EdgeBindLocal && e.Status != "" {
				if t := inferType(host, locals, e.Expr); t != nil {
					locals[e.Status] = t
				}
			}
			if e.Body != nil {
				visit(host, e.Body, locals)
			}
		}
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.BodyBlock != nil {
			visit(n, n.BodyBlock, nil)
		}
	}
	if script := c.g.Script(); script != nil && script.BodyBlock != nil {
		visit(nil, script.BodyBlock, nil)
	}
	for _, t := range c.g.Tests {
		visit(nil, t.BodyBlock, nil)
	}
	for _, ld := range c.g.Listeners {
		visit(nil, ld.Body, nil)
	}
	return nil
}

// blockTerminates reports whether every path through b ends in a terminal
// return. Used by both unreachable-code analysis (where post-terminal code
// is rejected) and the phrase-resolution check (where missing terminals are
// rejected for action and translator bodies).
func (c *compiler) blockTerminates(b *graph.Block) bool {
	if b == nil {
		return false
	}
	return c.edgesTerminate(b.Edges)
}

func (c *compiler) edgesTerminate(edges []graph.Edge) bool {
	terminated := false
	i := 0
	for i < len(edges) {
		e := &edges[i]
		switch e.Kind {
		case graph.EdgeBranchOpen, graph.EdgeBranchFallback:
			groupTerm, hasFallback, closeIdx := c.armGroupTerminates(edges, i)
			if groupTerm && hasFallback {
				terminated = true
			}
			i = closeIdx + 1
			continue
		case graph.EdgeBranchClose, graph.EdgeFinally:
			i++
			continue
		case graph.EdgeReturnOK, graph.EdgeReturnOKWith,
			graph.EdgeReturnFailedWith, graph.EdgeReturnFailedWithError,
			graph.EdgeReturnPassthrough:
			terminated = true
		case graph.EdgeLock:
			// `lock X... <body>` always runs <body> after acquiring; if the
			// body always terminates, the lock-block always does too.
			if e.Body != nil && c.edgesTerminate(e.Body.Edges) {
				terminated = true
			}
		}
		i++
	}
	return terminated
}

func (c *compiler) armGroupTerminates(edges []graph.Edge, start int) (bool, bool, int) {
	groupID := edges[start].Branch
	allTerm := true
	anyArm := false
	hasFallback := false
	i := start
	closeIdx := start
	for i < len(edges) {
		e := &edges[i]
		if e.Branch == groupID && e.Kind == graph.EdgeBranchClose {
			closeIdx = i
			break
		}
		if e.Branch == groupID && (e.Kind == graph.EdgeBranchOpen || e.Kind == graph.EdgeBranchFallback) {
			if e.Kind == graph.EdgeBranchFallback {
				hasFallback = true
			}
			j := i + 1
			for j < len(edges) {
				ej := &edges[j]
				if ej.Branch == groupID &&
					(ej.Kind == graph.EdgeBranchOpen ||
						ej.Kind == graph.EdgeBranchFallback ||
						ej.Kind == graph.EdgeBranchClose) {
					break
				}
				j++
			}
			armTerm := c.edgesTerminate(edges[i+1 : j])
			anyArm = true
			if !armTerm {
				allTerm = false
			}
			i = j
			continue
		}
		i++
	}
	return anyArm && allTerm, hasFallback, closeIdx
}

// checkPhrasesResolve enforces the spec rule "Falling off the end of a phrase
// is a compile error." Every action and translator body must terminate on
// every path (explicit return or a fully-terminating branch group with `or?`).
func (c *compiler) checkPhrasesResolve() error {
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind != graph.KindAction && n.Kind != graph.KindTranslator {
			continue
		}
		if n.BodyBlock == nil {
			continue
		}
		if c.blockTerminates(n.BodyBlock) {
			continue
		}
		var pos ast.Sentence
		if n.Body != nil {
			pos = *n.Body
		}
		c.report(&Error{Code: "missing-return", Pos: pos, Msg: fmt.Sprintf(
			"%s falls off the end without an explicit return: every path must end in `this is <Status>!` (or `this is that!`)",
			n.Name)})
	}
	return nil
}

// checkUnreachable rejects edges that follow a terminal path. Two cases:
//   - a return immediately followed by more straight-line code in the same arm
//   - a branch group whose every arm terminates AND has a bare `or?` fallback
//     (meaning some arm always fires), followed by more code in the parent
//
// `finally...` blocks are deferred cleanup hooks and may follow a terminal
// return; their bodies are visited but the post-finally edges (if any) are
// still gated by the prior terminal.
func (c *compiler) checkUnreachable() error {
	var visitEdges func(edges []graph.Edge) (bool, ast.Sentence, error)
	var visitGroup func(edges []graph.Edge, start int) (bool, bool, int, error)
	visitEdges = func(edges []graph.Edge) (bool, ast.Sentence, error) {
		terminated := false
		var termPos ast.Sentence
		i := 0
		for i < len(edges) {
			e := &edges[i]
			if e.Kind == graph.EdgeBranchOpen || e.Kind == graph.EdgeBranchFallback {
				if terminated {
					return false, ast.Sentence{}, &Error{Code: "unreachable", Pos: e.Pos, Msg: fmt.Sprintf(
						"unreachable code after terminal return at %s", termPos.Pos)}
				}
				groupTerm, hasFallback, closeIdx, err := visitGroup(edges, i)
				if err != nil {
					return false, ast.Sentence{}, err
				}
				if groupTerm && hasFallback {
					terminated = true
					termPos = e.Pos
				}
				i = closeIdx + 1
				continue
			}
			if e.Kind == graph.EdgeBranchClose {
				i++
				continue
			}
			if e.Kind == graph.EdgeFinally {
				if _, _, err := visitEdges(e.Body.Edges); err != nil {
					return false, ast.Sentence{}, err
				}
				i++
				continue
			}
			if terminated {
				return false, ast.Sentence{}, &Error{Pos: e.Pos, Msg: fmt.Sprintf(
					"unreachable code after terminal return at %s", termPos.Pos)}
			}
			switch e.Kind {
			case graph.EdgeReturnOK, graph.EdgeReturnOKWith,
				graph.EdgeReturnFailedWith, graph.EdgeReturnFailedWithError,
				graph.EdgeReturnPassthrough:
				terminated = true
				termPos = e.Pos
			case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil,
				graph.EdgeLock, graph.EdgeStart:
				if e.Body != nil {
					if _, _, err := visitEdges(e.Body.Edges); err != nil {
						return false, ast.Sentence{}, err
					}
				}
			}
			i++
		}
		return terminated, termPos, nil
	}
	visitGroup = func(edges []graph.Edge, start int) (bool, bool, int, error) {
		groupID := edges[start].Branch
		allTerminate := true
		anyArm := false
		hasFallback := false
		closeIdx := start
		i := start
		for i < len(edges) {
			e := &edges[i]
			if e.Branch == groupID && e.Kind == graph.EdgeBranchClose {
				closeIdx = i
				break
			}
			if e.Branch == groupID && (e.Kind == graph.EdgeBranchOpen || e.Kind == graph.EdgeBranchFallback) {
				if e.Kind == graph.EdgeBranchFallback {
					hasFallback = true
				}
				j := i + 1
				for j < len(edges) {
					ej := &edges[j]
					if ej.Branch == groupID &&
						(ej.Kind == graph.EdgeBranchOpen ||
							ej.Kind == graph.EdgeBranchFallback ||
							ej.Kind == graph.EdgeBranchClose) {
						break
					}
					j++
				}
				armTerm, _, err := visitEdges(edges[i+1 : j])
				if err != nil {
					return false, false, 0, err
				}
				anyArm = true
				if !armTerm {
					allTerminate = false
				}
				i = j
				continue
			}
			i++
		}
		return anyArm && allTerminate, hasFallback, closeIdx, nil
	}
	visit := func(b *graph.Block) error {
		if b == nil {
			return nil
		}
		_, _, err := visitEdges(b.Edges)
		return err
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.BodyBlock != nil {
			c.collect(visit(n.BodyBlock))
		}
	}
	if script := c.g.Script(); script != nil && script.BodyBlock != nil {
		c.collect(visit(script.BodyBlock))
	}
	for _, t := range c.g.Tests {
		c.collect(visit(t.BodyBlock))
	}
	for _, ld := range c.g.Listeners {
		c.collect(visit(ld.Body))
	}
	return nil
}

// checkDeadArms rejects `when <ref> is <Status>?` arms attached to an invoke
// where the callee's effective emission set provably never produces <Status>.
// `exited` and `died` are runtime-panic canonicals that may arise from any
// callee, so they're never flagged. Predicate arms that aren't a bare status
// check are skipped — we can't reason about arbitrary expressions.
func (c *compiler) checkDeadArms() error {
	a := newAnalyzer(c.g)
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind == graph.KindAction || n.Kind == graph.KindTranslator {
			a.effectiveStatuses(n)
		}
	}
	var visit func(owner *graph.Node, b *graph.Block) error
	resolveCallee := func(owner *graph.Node, e *graph.Edge) *graph.Node {
		if e.Action != nil {
			return e.Action
		}
		if owner == nil || e.Status == "" {
			return nil
		}
		cap := owner.CapByName(e.Status)
		if cap == nil {
			return nil
		}
		return cap.Action
	}
	visit = func(owner *graph.Node, b *graph.Block) error {
		if b == nil {
			return nil
		}
		edges := b.Edges
		for i := range edges {
			e := &edges[i]
			switch e.Kind {
			case graph.EdgeInvokePhrase:
				callee := resolveCallee(owner, e)
				if callee == nil {
					continue
				}
				armStart := i + 1
				if armStart < len(edges) && (edges[armStart].Kind == graph.EdgeBranchOpen || edges[armStart].Kind == graph.EdgeBranchFallback) {
					if err := c.checkArmGroup(a, callee, "that", edges, armStart); err != nil {
						return err
					}
				}
			case graph.EdgeStart:
				callee := resolveCallee(owner, e)
				if callee != nil && e.Body != nil && len(e.Body.Edges) > 0 {
					if err := c.checkArmGroup(a, callee, e.Message, e.Body.Edges, 0); err != nil {
						return err
					}
				}
				if err := visit(owner, e.Body); err != nil {
					return err
				}
			case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil, graph.EdgeLock, graph.EdgeFinally:
				if err := visit(owner, e.Body); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.BodyBlock != nil {
			c.collect(visit(n.Owner, n.BodyBlock))
		}
	}
	if script := c.g.Script(); script != nil && script.BodyBlock != nil {
		c.collect(visit(nil, script.BodyBlock))
	}
	for _, t := range c.g.Tests {
		c.collect(visit(nil, t.BodyBlock))
	}
	for _, ld := range c.g.Listeners {
		c.collect(visit(ld.Target, ld.Body))
	}
	return nil
}

// checkArmGroup walks one branch group (starting at edges[start], which must
// be a BranchOpen or BranchFallback) and flags any status-arm that targets a
// status the callee provably never emits.
func (c *compiler) checkArmGroup(a *analyzer, callee *graph.Node, frameRef string, edges []graph.Edge, start int) error {
	group := edges[start].Branch
	effective := a.effectiveStatuses(callee)
	for j := start; j < len(edges); j++ {
		e := &edges[j]
		if e.Kind == graph.EdgeBranchClose && e.Branch == group {
			break
		}
		// Arm-body edges (Branch == nil) and edges from nested groups belong
		// to other contexts — skip but keep walking until our group closes.
		if e.Branch != group {
			continue
		}
		if e.Kind != graph.EdgeBranchOpen {
			continue
		}
		sp, ok := e.Pred.(graph.StatusPred)
		if !ok {
			continue
		}
		if !matchesFrameRef(sp.Ref, frameRef) {
			continue
		}
		switch sp.Status {
		case "exited", "died":
			continue
		}
		if _, hit := effective[sp.Status]; hit {
			continue
		}
		return &Error{Code: "dead-arm", Pos: e.Pos, Msg: fmt.Sprintf(
			"arm `when %s is %s?` is unreachable: %s never emits status %q\n  inferred contract: { %s }",
			frameRef, sp.Status, callee.Name, sp.Status,
			strings.Join(sortedKeys(effective), ", "))}
	}
	return nil
}

// checkNoReentrancy rejects synchronous `do` cycles between actions: an action
// that (transitively) invokes itself would deadlock or recurse without bound.
// `start` and `execute` spawn independent frames and are exempt.
func (c *compiler) checkNoReentrancy() error {
	a := newAnalyzer(c.g)
	visited := map[*graph.Node]bool{}
	onStack := map[*graph.Node]bool{}
	stackOrder := []*graph.Node{}
	stackPos := map[*graph.Node]ast.Sentence{}
	var visit func(n *graph.Node, fromPos ast.Sentence) error
	visit = func(n *graph.Node, fromPos ast.Sentence) error {
		if n == nil {
			return nil
		}
		if onStack[n] {
			// Cycle: render path from the first occurrence of n to fromPos.
			start := 0
			for i, h := range stackOrder {
				if h == n {
					start = i
					break
				}
			}
			cycle := append([]*graph.Node{}, stackOrder[start:]...)
			cycle = append(cycle, n)
			parts := make([]string, 0, len(cycle))
			parts = append(parts, cycle[0].Name)
			for i := 1; i < len(cycle); i++ {
				p := stackPos[cycle[i-1]]
				if i == len(cycle)-1 {
					p = fromPos
				}
				parts = append(parts, fmt.Sprintf("%s (at %s)", cycle[i].Name, p.Pos))
			}
			return &Error{Pos: fromPos, Msg: fmt.Sprintf(
				"reentrant `do` cycle: %s — synchronous self-call would deadlock; use `start` for an independent frame",
				strings.Join(parts, " → "))}
		}
		if visited[n] {
			return nil
		}
		onStack[n] = true
		stackOrder = append(stackOrder, n)
		for _, inv := range collectSyncInvokes(a, n, n.BodyBlock) {
			stackPos[n] = inv.pos
			if err := visit(inv.callee, inv.pos); err != nil {
				return err
			}
		}
		delete(stackPos, n)
		stackOrder = stackOrder[:len(stackOrder)-1]
		onStack[n] = false
		visited[n] = true
		return nil
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind != graph.KindAction && n.Kind != graph.KindTranslator {
			continue
		}
		if visited[n] {
			continue
		}
		// Bail on first reentrancy cycle: the walker's onStack/stackOrder
		// bookkeeping is not cleaned up on the error return path, so collecting
		// across siblings would leak state into subsequent visits. Cross-pass
		// collection still surfaces this alongside other diagnostics.
		if err := visit(n, ast.Sentence{}); err != nil {
			c.collect(err)
			return nil
		}
	}
	return nil
}

type syncInvoke struct {
	callee *graph.Node
	pos    ast.Sentence
}

// collectSyncInvokes gathers every synchronous `do` invocation reachable from
// the action's body, recursing into loops/locks/finally blocks. `start` and
// `execute` are excluded — they create independent frames.
func collectSyncInvokes(a *analyzer, host *graph.Node, b *graph.Block) []syncInvoke {
	var out []syncInvoke
	var walk func(*graph.Block)
	walk = func(b *graph.Block) {
		if b == nil {
			return
		}
		for i := range b.Edges {
			e := &b.Edges[i]
			switch e.Kind {
			case graph.EdgeInvokePhrase:
				if callee := a.resolveCallee(host, *e); callee != nil {
					out = append(out, syncInvoke{callee: callee, pos: e.Pos})
				}
			case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil, graph.EdgeLock, graph.EdgeFinally, graph.EdgeStart:
				// Recurse into nested bodies, but EdgeStart's *callee* itself
				// is exempt (spawned frame); we still need to walk its
				// observation body for nested `do` edges in arms.
				walk(e.Body)
			}
		}
	}
	walk(b)
	return out
}

// checkRootClosure runs the float analysis on every root body — scripts,
// tests, and listeners — and rejects any unhandled non-ok status. Roots have
// no caller, so this is the buck-stops-here boundary that makes the safety
// claim hold all the way up.
func (c *compiler) checkRootClosure() error {
	a := newAnalyzer(c.g)
	// Make sure every action's effective set is computed first so callees
	// referenced from roots resolve correctly.
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind == graph.KindAction || n.Kind == graph.KindTranslator {
			a.effectiveStatuses(n)
		}
	}
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		// Tests are intentionally exempt: they assert behavior, including
		// failure modes, so an unhandled `failed` IS the assertion.
		if n.Kind == graph.KindScript {
			c.collect(c.checkRootBody(a, n.Name, n, n.BodyBlock))
		}
	}
	for _, ld := range c.g.Listeners {
		if ld.Body == nil {
			continue
		}
		// Listeners run with `this` bound to their Target actor; fake a host
		// node so `do this's <Cap>` resolves against the target's caps.
		host := &graph.Node{Kind: graph.KindAction, Owner: ld.Target}
		label := "listener"
		if ld.Target != nil {
			label = fmt.Sprintf("listener on %s", ld.Target.Name)
		}
		c.collect(c.checkRootBody(a, label, host, ld.Body))
	}
	return nil
}

func (c *compiler) checkRootBody(a *analyzer, label string, host *graph.Node, b *graph.Block) error {
	if b == nil {
		return nil
	}
	emitted := a.collectFromBody(host, b)
	for _, s := range sortedKeys(emitted) {
		if s == "ok" || s == "exited" || s == "died" {
			continue
		}
		pos := emitted[s]
		return &Error{Pos: pos, Msg: fmt.Sprintf(
			"%s leaves status %q unhandled at root — handle with `when ... is %s?` or a bare `or?`",
			label, s, s)}
	}
	return nil
}

// checkExhaustiveness verifies that every action's full set of resolvable
// statuses (explicit returns ∪ floated from observation sites) is a subset
// of the contract it adopts via `that is <Contract>.`. Actions without an
// adopted contract get an inferred contract = their effective emissions.
//
// Floating: at every `do X...` / `start X as N...` site, the caller may
// handle some statuses with `when X is Y?` predicates and consume the rest
// via a bare `or?` fallback. Statuses left unhandled — neither matched nor
// caught by a bare `or?` — float up as the caller's own emissions.
func (c *compiler) checkExhaustiveness() error {
	a := newAnalyzer(c.g)
	// First, populate every action's effective set.
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind == graph.KindAction || n.Kind == graph.KindTranslator {
			a.effectiveStatuses(n)
			n.InferredStatuses = sortedKeys(a.emissions[n])
		}
	}
	// Then verify adopted contracts.
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind != graph.KindAction || n.AdoptedContractName == "" || n.BodyBlock == nil {
			continue
		}
		contract, ok := c.g.Nodes[n.AdoptedContractName]
		if !ok || contract.Kind != graph.KindContract {
			return fmt.Errorf("action %s adopts unknown contract %q", n.Name, n.AdoptedContractName)
		}
		// Runtime-panic canonicals are always allowed; `canceled` must be
		// declared explicitly via `this allows canceled.`
		allowed := map[string]bool{
			"ok":     true,
			"exited": true,
			"died":   true,
		}
		for _, as := range contract.AllowedStatuses {
			allowed[as.Name] = true
		}
		emitted := a.emissions[n]
		var violations []string
		var firstPos ast.Sentence
		var firstStatus string
		for _, status := range sortedKeys(emitted) {
			if allowed[status] {
				continue
			}
			if firstStatus == "" {
				firstStatus = status
				firstPos = emitted[status]
			}
			violations = append(violations, status)
		}
		if firstStatus != "" {
			declared := contractAllowedNames(contract)
			c.report(&Error{Code: "exhaustiveness", Pos: firstPos, Msg: fmt.Sprintf(
				"action %s emits status %q which is not allowed by contract %s\n  inferred contract: { %s }\n  contract %s allows: { %s }",
				n.Name, firstStatus, contract.Name,
				strings.Join(n.InferredStatuses, ", "),
				contract.Name,
				strings.Join(declared, ", "))})
		}
	}
	return nil
}

func contractAllowedNames(contract *graph.Node) []string {
	names := make([]string, 0, len(contract.AllowedStatuses))
	for _, as := range contract.AllowedStatuses {
		names = append(names, as.Name)
	}
	sort.Strings(names)
	return names
}

// checkAllowedShapes verifies that every `this is <Status> with <expr>!`
// produces a value compatible with the contract's `this allows <Status> with
// <Shape>` declaration. Only checks expressions whose type can be determined
// statically; conservatively skips refs and other dynamic forms (the runtime
// remains the source of truth there).
func (c *compiler) checkAllowedShapes() error {
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind != graph.KindAction || n.AdoptedContractName == "" || n.BodyBlock == nil {
			continue
		}
		contract, ok := c.g.Nodes[n.AdoptedContractName]
		if !ok || contract.Kind != graph.KindContract {
			continue
		}
		shapes := map[string]*graph.AllowedStatus{}
		for i := range contract.AllowedStatuses {
			as := &contract.AllowedStatuses[i]
			shapes[as.Name] = as
		}
		c.collect(c.checkBlockShapes(n, contract, shapes, n.BodyBlock))
	}
	return nil
}

func (c *compiler) checkBlockShapes(host, contract *graph.Node,
	shapes map[string]*graph.AllowedStatus, b *graph.Block) error {
	if b == nil {
		return nil
	}
	for i := range b.Edges {
		e := &b.Edges[i]
		switch e.Kind {
		case graph.EdgeReturnOKWith, graph.EdgeReturnFailedWith:
			as, ok := shapes[e.Status]
			if !ok || as == nil || as.Shape == nil {
				continue
			}
			if err := c.verifyReturnShape(host, contract, as, e); err != nil {
				return err
			}
		case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil, graph.EdgeLock, graph.EdgeFinally:
			if err := c.checkBlockShapes(host, contract, shapes, e.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyReturnShape checks (and if possible refines) the value expression of
// a return edge against the contract's declared shape. ConstructExpr without
// an explicit Type is back-filled from the shape so the runtime constructs a
// typed Set; mismatches emit a compile error.
func (c *compiler) verifyReturnShape(host, contract *graph.Node,
	as *graph.AllowedStatus, e *graph.Edge) error {
	// Multi-as ConstructExpr without an explicit type: adopt the shape and
	// validate the field names against it.
	if ce, ok := e.Expr.(graph.ConstructExpr); ok && ce.Type == nil {
		ce.Type = as.Shape
		// Field names must exist on the shape (only enforce for sets).
		if as.Shape.Kind == graph.KindSet {
			for _, fld := range ce.Fields {
				if as.Shape.FieldByName(fld.Name) == nil {
					return &Error{Pos: e.Pos, Msg: fmt.Sprintf(
						"action %s returns `%s with ... %s ...` but %s has no field %q",
						host.Name, as.Name, fld.Name, as.Shape.Name, fld.Name)}
				}
			}
		}
		e.Expr = ce
		return nil
	}
	got := c.inferExprType(host, e.Expr)
	if got == nil {
		// Unknown type — skip (over-permissive; runtime still validates).
		return nil
	}
	if shapeCompatible(got, as.Shape) {
		return nil
	}
	return &Error{Pos: e.Pos, Msg: fmt.Sprintf(
		"action %s returns `%s with <%s>` but contract %s allows %s with <%s>",
		host.Name, as.Name, got.Name, contract.Name, as.Name, as.Shape.Name)}
}

// typeCompatible reports whether values of `got` could ever satisfy a `want`
// type assertion. Used by type-predicate validation. Errors with non-error
// targets are incompatible; identity matches; otherwise we treat as
// incompatible (no subtyping in v1).
func typeCompatible(got, want *graph.Node) bool {
	if got == nil || want == nil {
		return true
	}
	if got == want {
		return true
	}
	if want.Name == "error" && got.Kind == graph.KindError {
		return true
	}
	if got.Kind == graph.KindError && want.Kind == graph.KindError {
		return true
	}
	return false
}

// staticInputRefType walks `input[/'s <Field>...]` against the host's declared
// InputType. Returns nil when the type can't be determined statically.
func (c *compiler) staticInputRefType(host *graph.Node, path []string) *graph.Node {
	if host == nil || host.InputType == nil || len(path) == 0 || path[0] != "input" {
		return nil
	}
	cur := host.InputType
	for _, seg := range path[1:] {
		if cur == nil || cur.Kind != graph.KindSet {
			return nil
		}
		f := cur.FieldByName(seg)
		if f == nil {
			return nil
		}
		cur = f.Type
	}
	return cur
}

// refString renders a possessive ref-path back to source form for diagnostics.
func refString(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(path[0])
	for _, seg := range path[1:] {
		out.WriteString("'s " + seg)
	}
	return out.String()
}

// shapeCompatible reports whether got fits the declared shape. Errors satisfy
// an `error` shape regardless of subtype; otherwise we require exact identity.
func shapeCompatible(got, shape *graph.Node) bool {
	if got == nil || shape == nil {
		return true
	}
	if got == shape {
		return true
	}
	if shape.Name == "error" && got.Kind == graph.KindError {
		return true
	}
	return false
}

// inferExprType returns a best-effort static type for e in the context of the
// host action. Returns nil when the type can't be determined statically.
func (c *compiler) inferExprType(host *graph.Node, e graph.Expr) *graph.Node {
	switch x := e.(type) {
	case graph.LiteralExpr:
		switch x.Kind {
		case ast.PartString:
			return c.g.Nodes["text"]
		case ast.PartNumber:
			return c.g.Nodes["number"]
		case ast.PartBoolean:
			return c.g.Nodes["boolean"]
		}
	case graph.ErrorLiteralExpr:
		return c.g.Nodes["error"]
	case graph.ConstructExpr:
		return x.Type
	case graph.CastExpr:
		return x.Target
	case graph.RefExpr:
		return c.refType(host, x.Path)
	case graph.BinaryExpr:
		l := c.inferExprType(host, x.L)
		r := c.inferExprType(host, x.R)
		if l != nil && l == r {
			return l
		}
	}
	return nil
}

// refType resolves `this's <Field>` chains against the host action's owner.
// Other roots (`that`, locals) return nil — those are tracked dynamically.
func (c *compiler) refType(host *graph.Node, path []string) *graph.Node {
	if host == nil || host.Owner == nil || len(path) < 2 || path[0] != "this" {
		return nil
	}
	cur := host.Owner
	for _, seg := range path[1:] {
		if cur == nil {
			return nil
		}
		f := cur.FieldByName(seg)
		if f == nil {
			return nil
		}
		cur = f.Type
	}
	return cur
}

func sortedKeys(m map[string]ast.Sentence) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// analyzer computes effective emission sets per action. Memoized; cycle-safe.
type analyzer struct {
	g                *graph.Graph
	emissions        map[*graph.Node]map[string]ast.Sentence
	inProgress       map[*graph.Node]bool
	cancellableCache map[*graph.Node]bool
	cancellableSeen  map[*graph.Node]bool
	cancelOrigin     map[*graph.Node]ast.Sentence
}

func newAnalyzer(g *graph.Graph) *analyzer {
	return &analyzer{
		g:                g,
		emissions:        map[*graph.Node]map[string]ast.Sentence{},
		inProgress:       map[*graph.Node]bool{},
		cancellableCache: map[*graph.Node]bool{},
		cancellableSeen:  map[*graph.Node]bool{},
		cancelOrigin:     map[*graph.Node]ast.Sentence{},
	}
}

// isCancellable reports whether n's body can produce a `canceled` status at
// runtime. True when the body contains `wait until` or `lock`, or invokes
// another cancellable action. Cycle-safe via a seen-set: in-progress nodes are
// treated as not-cancellable to avoid infinite recursion; the second pass
// will pick up real positives.
func (a *analyzer) isCancellable(n *graph.Node) bool {
	if n == nil {
		return false
	}
	if v, ok := a.cancellableCache[n]; ok {
		return v
	}
	if a.cancellableSeen[n] {
		return false
	}
	a.cancellableSeen[n] = true
	v := a.bodyCancellable(n, n.BodyBlock)
	a.cancellableSeen[n] = false
	a.cancellableCache[n] = v
	return v
}

func (a *analyzer) bodyCancellable(host *graph.Node, b *graph.Block) bool {
	if b == nil {
		return false
	}
	for i := range b.Edges {
		e := &b.Edges[i]
		switch e.Kind {
		case graph.EdgeWaitUntil, graph.EdgeLock:
			a.cancelOrigin[host] = e.Pos
			return true
		case graph.EdgeInvokePhrase, graph.EdgeStart, graph.EdgeExecute:
			callee := a.resolveCallee(host, *e)
			if callee != nil && a.isCancellable(callee) {
				a.cancelOrigin[host] = e.Pos
				return true
			}
		case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeFinally:
			if a.bodyCancellable(host, e.Body) {
				return true
			}
		}
	}
	return false
}

// markForeign flags every exported act capability that ended the build with no
// Marco body as a foreign action — its implementation lives in a host (see
// spec/Hosts.md). The flag distinguishes the intentional FFI seam (exported +
// bodyless) from a genuinely empty action, and lets later passes skip
// body-shape checks and infer the result contract.
func (c *compiler) markForeign() {
	for _, name := range c.g.Order {
		owner := c.g.Nodes[name]
		if owner.Kind != graph.KindActor {
			continue
		}
		for _, cap := range owner.Caps {
			act := cap.Action
			if cap.Exported && act != nil && act.Kind == graph.KindAction &&
				act.Body == nil && act.BodyBlock == nil {
				act.Foreign = true
			}
		}
	}
}

// foreignStatuses returns the effective status set for a foreign action: the
// statuses of its adopted contract if it declares one, else the default
// {ok, failed}. Foreign code can always fail, so `failed` is included unless a
// contract narrows it.
func foreignStatuses(g *graph.Graph, n *graph.Node) map[string]ast.Sentence {
	out := map[string]ast.Sentence{"ok": {}}
	if n.AdoptedContractName != "" {
		if ct, ok := g.Nodes[n.AdoptedContractName]; ok && ct.Kind == graph.KindContract {
			for _, as := range ct.AllowedStatuses {
				out[as.Name] = ast.Sentence{}
			}
			return out
		}
	}
	out["failed"] = ast.Sentence{}
	return out
}

// effectiveStatuses returns the action's full emission set (cached). For an
// action with an adopted contract we still compute from the body — the
// verifier later checks that this set ⊆ contract.allowed.
func (a *analyzer) effectiveStatuses(n *graph.Node) map[string]ast.Sentence {
	if cached, ok := a.emissions[n]; ok {
		return cached
	}
	if n.Foreign {
		out := foreignStatuses(a.g, n)
		a.emissions[n] = out
		return out
	}
	if a.inProgress[n] {
		// Cycle: return a placeholder; caller adds to its own set without
		// further propagation. Conservative — assumes ok only.
		return map[string]ast.Sentence{"ok": {}}
	}
	a.inProgress[n] = true
	out := a.collectFromBody(n, n.BodyBlock)
	a.inProgress[n] = false
	if len(out) == 0 {
		// An action whose body emits nothing explicitly resolves as ok.
		out["ok"] = ast.Sentence{}
	}
	if a.isCancellable(n) {
		// Cancellable bodies (lock/wait, or transitively a cancellable callee)
		// can resolve as `canceled` regardless of explicit returns. Surface
		// that in the effective set so callers must handle it.
		if _, already := out["canceled"]; !already {
			out["canceled"] = a.cancelOrigin[n]
		}
	}
	a.emissions[n] = out
	return out
}

// collectFromBody walks edges, accumulating both explicit emissions and
// floated emissions from unhandled call sites. The owner action `host` is
// used to resolve self-relative `do this's <Cap>` invokes.
func (a *analyzer) collectFromBody(host *graph.Node, b *graph.Block) map[string]ast.Sentence {
	out := map[string]ast.Sentence{}
	if b == nil {
		return out
	}
	edges := b.Edges
	// lastCallee is the most-recent invoke's resolved action, used by
	// `this is that!` passthrough to enumerate forwarded statuses.
	var lastCallee *graph.Node
	// armStack tracks the open branch arms encountered during the linear walk.
	// The top entry refines `this is that!` passthrough to forward only the
	// status the enclosing arm matched (e.g. `when that is Saved?` arms only
	// forward Saved). Bare `or?` arms keep nil match — fall back to forwarding
	// the full callee set.
	type armFrame struct {
		group  *graph.BranchGroup
		callee *graph.Node     // invoke this group's arms attach to (lastCallee at open time)
		match  map[string]bool // status names this arm forwards; nil = no filter
	}
	var armStack []armFrame
	currentArm := func() *armFrame {
		if len(armStack) == 0 {
			return nil
		}
		return &armStack[len(armStack)-1]
	}
	i := 0
	for i < len(edges) {
		e := edges[i]
		switch e.Kind {
		case graph.EdgeReturnOK, graph.EdgeReturnOKWith, graph.EdgeStateUpdate:
			name := e.Status
			if name == "" {
				name = "ok"
			}
			out[name] = e.Pos
		case graph.EdgeReturnFailedWith:
			name := e.Status
			if name == "" {
				name = "failed"
			}
			out[name] = e.Pos
		case graph.EdgeReturnFailedWithError:
			out["failed"] = e.Pos
		case graph.EdgeReturnPassthrough:
			if lastCallee != nil {
				effective := a.effectiveStatuses(lastCallee)
				arm := currentArm()
				if arm != nil && arm.callee == lastCallee && arm.match != nil {
					for s := range arm.match {
						if _, ok := effective[s]; ok {
							out[s] = e.Pos
						}
					}
				} else {
					for s := range effective {
						out[s] = e.Pos
					}
				}
			}
		case graph.EdgeBranchOpen:
			match := armMatchSet(e.Pred)
			if n := len(armStack); n > 0 && armStack[n-1].group == e.Branch {
				armStack[n-1].match = match
			} else {
				armStack = append(armStack, armFrame{
					group:  e.Branch,
					callee: lastCallee,
					match:  match,
				})
			}
		case graph.EdgeBranchFallback:
			if n := len(armStack); n > 0 && armStack[n-1].group == e.Branch {
				armStack[n-1].match = nil
			} else {
				armStack = append(armStack, armFrame{
					group:  e.Branch,
					callee: lastCallee,
					match:  nil,
				})
			}
		case graph.EdgeBranchClose:
			if n := len(armStack); n > 0 && armStack[n-1].group == e.Branch {
				armStack = armStack[:n-1]
			}
		case graph.EdgeForEach, graph.EdgeRepeat, graph.EdgeWhile, graph.EdgeWaitUntil, graph.EdgeLock, graph.EdgeFinally:
			maps.Copy(out, a.collectFromBody(host, e.Body))
		case graph.EdgeInvokePhrase:
			callee := a.resolveCallee(host, e)
			lastCallee = callee
			handled, hasBareOr, _ := a.analyzeTrailingBranches(edges, i+1, "that")
			a.addFloated(out, callee, handled, hasBareOr, e.Pos)
			// Don't skip past the branch group — let the loop pick up explicit
			// emissions from each arm body naturally.
		case graph.EdgeStart:
			callee := a.resolveCallee(host, e)
			lastCallee = callee
			// Observation arms for `start ... as N...` live in e.Body, not in
			// the parent's edge stream.
			var armEdges []graph.Edge
			if e.Body != nil {
				armEdges = e.Body.Edges
			}
			handled, hasBareOr, _ := a.analyzeTrailingBranches(armEdges, 0, e.Message)
			a.addFloated(out, callee, handled, hasBareOr, e.Pos)
			// Walk the observation body for explicit emissions inside arms.
			maps.Copy(out, a.collectFromBody(host, e.Body))
		}
		i++
	}
	return out
}

// armMatchSet extracts the single-status filter implied by a branch
// predicate. `when that is X?` and `when <ref> is X?` arms forward only X
// through `this is that!` passthrough. Compound predicates and non-status
// shapes return nil — the arm imposes no filter.
func armMatchSet(p graph.Pred) map[string]bool {
	sp, ok := p.(graph.StatusPred)
	if !ok {
		return nil
	}
	return map[string]bool{sp.Status: true}
}

// resolveCallee returns the action node a `do`/`start`/`execute` edge calls.
// Self-relative forms (`e.Action == nil`) resolve via the host action's
// owner, looking up the cap by the e.Status name.
func (a *analyzer) resolveCallee(host *graph.Node, e graph.Edge) *graph.Node {
	if e.Action != nil {
		return e.Action
	}
	if host == nil || host.Owner == nil || e.Status == "" {
		return nil
	}
	cap := host.Owner.CapByName(e.Status)
	if cap == nil {
		return nil
	}
	return cap.Action
}

// addFloated computes the unhandled set at an invoke site and adds its
// statuses (other than canonicals always allowed) to the caller's emissions.
func (a *analyzer) addFloated(out map[string]ast.Sentence, callee *graph.Node,
	handled map[string]bool, hasBareOr bool, pos ast.Sentence) {
	if hasBareOr || callee == nil {
		return
	}
	calleeStatuses := a.effectiveStatuses(callee)
	for s := range calleeStatuses {
		if handled[s] {
			continue
		}
		// Runtime-panic canonicals don't need explicit handling. `canceled`
		// is intentionally excluded — it must flow through exhaustiveness.
		switch s {
		case "exited", "died":
			continue
		}
		out[s] = pos
	}
}

// analyzeTrailingBranches inspects edges[start:] for a branch group attached
// to a preceding invoke. Returns the set of statuses handled by `if/when
// <frameRef> is X?` predicates, whether a bare `or?` fallback exists, and
// the index immediately after the branch group (or `start` if no branch
// group is present at this position).
func (a *analyzer) analyzeTrailingBranches(edges []graph.Edge, start int, frameRef string) (map[string]bool, bool, int) {
	handled := map[string]bool{}
	if start >= len(edges) {
		return handled, false, start
	}
	if edges[start].Kind != graph.EdgeBranchOpen && edges[start].Kind != graph.EdgeBranchFallback {
		return handled, false, start
	}
	group := edges[start].Branch
	hasBareOr := false
	closeIdx := start
	for j := start; j < len(edges); j++ {
		switch edges[j].Kind {
		case graph.EdgeBranchOpen:
			if edges[j].Branch == group {
				if sp, ok := edges[j].Pred.(graph.StatusPred); ok {
					if matchesFrameRef(sp.Ref, frameRef) {
						handled[sp.Status] = true
					}
				}
			}
		case graph.EdgeBranchFallback:
			if edges[j].Branch == group {
				hasBareOr = true
			}
		case graph.EdgeBranchClose:
			if edges[j].Branch == group {
				closeIdx = j + 1
			}
		}
		if closeIdx > start {
			break
		}
	}
	if closeIdx <= start {
		closeIdx = start
	}
	return handled, hasBareOr, closeIdx
}

func matchesFrameRef(ref []string, frameRef string) bool {
	if frameRef == "" {
		// Bare status shorthand on `that`.
		return len(ref) == 1 && ref[0] == "that"
	}
	if len(ref) == 1 && (ref[0] == frameRef || ref[0] == "that") {
		return true
	}
	return false
}

type compiler struct {
	g             *graph.Graph
	branchSeq     int
	currentAction *graph.Node // set during compileBlock for an action's body
	currentTest   *graph.Node // set during compileBlock for a test body — enables test-scoped mocks
	errs          []*Error    // collected diagnostics from check passes
}

// report appends a diagnostic for later aggregation. Returns the error so call
// sites can write `return c.report(&Error{...})` and still propagate when a
// pass needs to short-circuit a single walker arm.
func (c *compiler) report(e *Error) *Error {
	c.errs = append(c.errs, e)
	return e
}

// collect lifts any returned error into c.errs. Used by passes that retain
// fail-fast walkers but want to keep iterating over sibling top-level nodes
// (each action / script / test / listener gets its own error independently).
func (c *compiler) collect(err error) {
	if err == nil {
		return
	}
	if ce, ok := errors.AsType[*Error](err); ok {
		c.report(ce)
		return
	}
	c.report(&Error{Msg: err.Error()})
}

func (c *compiler) nextBranchID() int {
	c.branchSeq++
	return c.branchSeq
}

// compileBlock translates a body of sentences into a flat sequence of edges.
//
// Branch groups (a leading `when ...?` followed by zero or more `or when?` /
// `or?`) compile to:
//
//	BRANCH_OPEN(group, predicate kind) → predicate-body edges → BRANCH_CLOSE
//	BRANCH_FALLBACK(group)             → fallback-body edges → BRANCH_CLOSE
func (c *compiler) compileBlock(sents []ast.Sentence) (*graph.Block, error) {
	block := &graph.Block{}
	i := 0
	for i < len(sents) {
		s := &sents[i]
		// Branch group? Bare `or?` is also accepted as a single-arm catch-all
		// (handy for satisfying root-closure exhaustiveness with no behavior).
		if startsBranchGroup(s) || startsFallback(s) {
			groupEdges, consumed, err := c.compileBranchGroup(sents[i:])
			if err != nil {
				return nil, err
			}
			block.Edges = append(block.Edges, groupEdges...)
			i += consumed
			continue
		}
		// Test-scoped mock: `mock <Owner>'s <Cap> is <Status> [with <expr>].`
		// inside a test body. Synthesize the body and store on the test;
		// no edge is emitted for the call site.
		if c.currentTest != nil && isInlineMockSentence(s) {
			if err := c.addScopedInlineMock(s); err != nil {
				return nil, err
			}
			i++
			continue
		}
		// Otherwise: a single sentence.
		es, err := c.compileSentence(s)
		if err != nil {
			return nil, err
		}
		block.Edges = append(block.Edges, es...)
		i++
	}
	return block, nil
}

// isInlineMockSentence detects `mock <Owner>'s <Cap> is <Status> [with <expr>].`
// at sentence level — the same shape recognized by the graph builder for
// top-level mocks, lifted here so test bodies can carry per-test mocks.
func isInlineMockSentence(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod || len(s.Parts) < 6 {
		return false
	}
	return isWord(s.Parts[0], "mock") &&
		s.Parts[1].Kind == ast.PartWord &&
		s.Parts[2].Kind == ast.PartPossessive &&
		s.Parts[3].Kind == ast.PartWord &&
		isWord(s.Parts[4], "is") &&
		s.Parts[5].Kind == ast.PartWord
}

func (c *compiler) addScopedInlineMock(s *ast.Sentence) error {
	ownerName := s.Parts[1].Value
	capName := s.Parts[3].Value
	owner, ok := c.g.Nodes[ownerName]
	if !ok {
		return &Error{Pos: *s, Msg: fmt.Sprintf("mock target %q is not a known actor or contract", ownerName)}
	}
	cap := owner.CapByName(capName)
	if cap == nil || cap.Action == nil {
		return &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q to mock%s",
			ownerName, capName, suggestClosest(capName, capNames(owner)))}
	}
	parts := make([]ast.Part, 0, len(s.Parts)-3)
	parts = append(parts, ast.Part{Kind: ast.PartWord, Value: "this", Pos: s.Pos})
	parts = append(parts, s.Parts[4:]...)
	synth := &ast.Sentence{Parts: parts, Term: ast.TermBang, Pos: s.Pos}
	edges, err := c.compileReturn(synth)
	if err != nil {
		return err
	}
	c.currentTest.ScopedInlineMocks = append(c.currentTest.ScopedInlineMocks, graph.ScopedInlineMock{
		Action: cap.Action,
		Body:   &graph.Block{Edges: edges},
		Pos:    *s,
	})
	return nil
}

// startsBranchGroup matches the regular `?`-terminated branch header and the
// compact `!`-terminated `if X, Y!` / `or if X, Y!` form.
func startsBranchGroup(s *ast.Sentence) bool {
	switch s.Term {
	case ast.TermQuestion:
		if len(s.Parts) == 0 {
			return false
		}
		if isWord(s.Parts[0], "when") || isWord(s.Parts[0], "if") {
			return true
		}
		if isWord(s.Parts[0], "or") && len(s.Parts) > 1 &&
			(isWord(s.Parts[1], "when") || isWord(s.Parts[1], "if")) {
			return true
		}
	case ast.TermBang:
		if len(s.Parts) > 0 && isWord(s.Parts[0], "if") && hasTopLevelComma(s.Parts) {
			return true
		}
		if len(s.Parts) > 1 && isWord(s.Parts[0], "or") && isWord(s.Parts[1], "if") &&
			hasTopLevelComma(s.Parts) {
			return true
		}
	}
	return false
}

// startsFallback matches `or?` and the compact `or, Y!`.
func startsFallback(s *ast.Sentence) bool {
	switch s.Term {
	case ast.TermQuestion:
		return len(s.Parts) == 1 && isWord(s.Parts[0], "or")
	case ast.TermBang:
		return len(s.Parts) >= 3 &&
			isWord(s.Parts[0], "or") &&
			s.Parts[1].Kind == ast.PartComma
	}
	return false
}

func hasTopLevelComma(parts []ast.Part) bool {
	for _, p := range parts {
		if p.Kind == ast.PartComma {
			return true
		}
	}
	return false
}

func (c *compiler) compileBranchGroup(rest []ast.Sentence) ([]graph.Edge, int, error) {
	group := &graph.BranchGroup{ID: c.nextBranchID()}
	var out []graph.Edge
	i := 0
	for i < len(rest) {
		s := &rest[i]
		if !startsBranchGroup(s) && !startsFallback(s) {
			break
		}
		// header edge + body
		var headerEdge graph.Edge
		var bodyEdges []graph.Edge
		var err error
		if s.Term == ast.TermBang {
			// Compact form: header + body in the same sentence.
			headerEdge, bodyEdges, err = c.compileCompactBranch(s, group)
			if err != nil {
				return nil, 0, err
			}
		} else if startsFallback(s) {
			headerEdge = graph.Edge{Kind: graph.EdgeBranchFallback, Branch: group, Pos: *s}
			bodyEdges, err = c.compileBody(s.Body)
			if err != nil {
				return nil, 0, err
			}
		} else {
			headerEdge, err = c.compileBranchHeader(s, group)
			if err != nil {
				return nil, 0, err
			}
			bodyEdges, err = c.compileBody(s.Body)
			if err != nil {
				return nil, 0, err
			}
		}
		out = append(out, headerEdge)
		out = append(out, bodyEdges...)
		i++
		if startsFallback(s) {
			break
		}
	}
	out = append(out, graph.Edge{Kind: graph.EdgeBranchClose, Branch: group})
	return out, i, nil
}

// compileBranchHeader compiles a `when ...?` header into a single Edge whose
// Pred field holds the predicate tree.
//
// Supported leaf forms:
//
//	<status>                  // shorthand on `that`
//	<ref> is <status>         // explicit status check
//	<ref> is <Type>           // type check (Type is a graph node, not a status)
//	<ref> is <literal-or-ref> // equality
//	<ref> is not <rhs>        // inequality
//	<ref> has <Field>         // presence
//	<ref> exists              // existence
//
// Combinators: `and` / `or` between leaves (no precedence mixing in v1).
// compileCompactBranch handles the single-sentence compact branch forms:
//
//	if <pred>, <return-body>!
//	or if <pred>, <return-body>!
//	or, <return-body>!
//
// Returns the synthesized header edge and body edges (one synthesized return).
func (c *compiler) compileCompactBranch(s *ast.Sentence, group *graph.BranchGroup) (graph.Edge, []graph.Edge, error) {
	parts := s.Parts
	commaIdx := -1
	for i, p := range parts {
		if p.Kind == ast.PartComma {
			commaIdx = i
			break
		}
	}
	if commaIdx < 0 {
		return graph.Edge{}, nil, &Error{Pos: *s, Msg: "compact branch missing `,`"}
	}
	header := parts[:commaIdx]
	bodyParts := parts[commaIdx+1:]
	if len(bodyParts) == 0 {
		return graph.Edge{}, nil, &Error{Pos: *s, Msg: "compact branch missing return body"}
	}

	// Header: detect fallback (`or, ...`) vs predicate (`if X, ...` / `or if X, ...`).
	var headerEdge graph.Edge
	if len(header) == 1 && isWord(header[0], "or") {
		headerEdge = graph.Edge{Kind: graph.EdgeBranchFallback, Branch: group, Pos: *s}
	} else {
		predParts := header
		if isWord(predParts[0], "or") && len(predParts) > 1 && isWord(predParts[1], "if") {
			predParts = predParts[1:]
		}
		if !isWord(predParts[0], "if") {
			return graph.Edge{}, nil, &Error{Pos: *s, Msg: "expected `if` in compact branch"}
		}
		pred, err := c.parsePred(predParts[1:], s)
		if err != nil {
			return graph.Edge{}, nil, err
		}
		headerEdge = graph.Edge{Kind: graph.EdgeBranchOpen, Pred: pred, Branch: group, Pos: *s}
	}

	// Body: synthesize a `this is <bodyParts>!` sentence and compile it.
	syntheticParts := append([]ast.Part{
		{Kind: ast.PartWord, Value: "this", Pos: s.Pos},
		{Kind: ast.PartWord, Value: "is", Pos: s.Pos},
	}, bodyParts...)
	syntheticS := *s
	syntheticS.Parts = syntheticParts
	syntheticS.Body = nil
	bodyEdges, err := c.compileReturn(&syntheticS)
	if err != nil {
		return graph.Edge{}, nil, err
	}
	return headerEdge, bodyEdges, nil
}

func (c *compiler) compileBranchHeader(s *ast.Sentence, group *graph.BranchGroup) (graph.Edge, error) {
	parts := s.Parts
	if isWord(parts[0], "or") && len(parts) > 1 && (isWord(parts[1], "when") || isWord(parts[1], "if")) {
		parts = parts[1:]
	}
	if !isWord(parts[0], "when") && !isWord(parts[0], "if") {
		return graph.Edge{}, &Error{Pos: *s, Msg: "expected `when` or `if`"}
	}
	pred, err := c.parsePred(parts[1:], s)
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.Edge{
		Kind:   graph.EdgeBranchOpen,
		Pred:   pred,
		Branch: group,
		Pos:    *s,
	}, nil
}

// parsePred parses a predicate body, possibly combining leaves with `and`/`or`.
// Mixing `and` and `or` at the same level is not permitted.
func (c *compiler) parsePred(body []ast.Part, src *ast.Sentence) (graph.Pred, error) {
	// Split on top-level `and` / `or` words.
	clauses, op, err := splitTopLevel(body)
	if err != nil {
		return nil, &Error{Pos: *src, Msg: err.Error()}
	}
	if len(clauses) == 1 {
		return c.parseLeaf(clauses[0], src)
	}
	subs := make([]graph.Pred, 0, len(clauses))
	for _, cl := range clauses {
		p, err := c.parseLeaf(cl, src)
		if err != nil {
			return nil, err
		}
		subs = append(subs, p)
	}
	switch op {
	case "and":
		return graph.AndPred{Sub: subs}, nil
	case "or":
		return graph.OrPred{Sub: subs}, nil
	}
	return nil, &Error{Pos: *src, Msg: "internal: unknown predicate combinator"}
}

// splitTopLevel walks parts and splits at every top-level `and` or `or` word.
// Mixed combinators at the same level cause an error.
func splitTopLevel(parts []ast.Part) ([][]ast.Part, string, error) {
	var clauses [][]ast.Part
	var op string
	start := 0
	for i, p := range parts {
		if isWord(p, "and") || isWord(p, "or") {
			if op == "" {
				op = p.Value
			} else if op != p.Value {
				return nil, "", fmt.Errorf("cannot mix `and` and `or` in the same `when` (use parentheses or split into nested branches)")
			}
			clauses = append(clauses, parts[start:i])
			start = i + 1
		}
	}
	clauses = append(clauses, parts[start:])
	return clauses, op, nil
}

// parseLeaf parses a single (combinator-free) predicate clause.
func (c *compiler) parseLeaf(parts []ast.Part, src *ast.Sentence) (graph.Pred, error) {
	if len(parts) == 0 {
		return nil, &Error{Pos: *src, Msg: "empty predicate"}
	}
	// `not <pred>` — invert the sub-predicate. The `<ref> is not <rhs>?` form
	// is its own NeqPred handled below; this branch only fires when `not`
	// leads the clause.
	if isWord(parts[0], "not") {
		sub, err := c.parseLeaf(parts[1:], src)
		if err != nil {
			return nil, err
		}
		return graph.NotPred{Sub: sub}, nil
	}
	// `<status>` shorthand on `that` — accepts canonical or custom status names.
	if len(parts) == 1 && parts[0].Kind == ast.PartWord {
		return graph.StatusPred{Ref: []string{"that"}, Status: parts[0].Value}, nil
	}
	// `<ref> exists`
	if len(parts) >= 2 && isWord(parts[len(parts)-1], "exists") {
		ref, err := parseRef(parts[:len(parts)-1])
		if err != nil {
			return nil, &Error{Pos: *src, Msg: err.Error()}
		}
		return graph.ExistsPred{Ref: ref}, nil
	}
	// `it's been <N> seconds` — frame-relative time predicate.
	if len(parts) >= 5 &&
		isWord(parts[0], "it") &&
		parts[1].Kind == ast.PartPossessive &&
		isWord(parts[2], "been") &&
		parts[3].Kind == ast.PartNumber &&
		(isWord(parts[4], "seconds") || isWord(parts[4], "second")) {
		n, err := strconv.ParseFloat(parts[3].Value, 64)
		if err != nil {
			return nil, &Error{Pos: *src, Msg: fmt.Sprintf("invalid number %q", parts[3].Value)}
		}
		return graph.TimePred{Seconds: n}, nil
	}
	// `<ref> unlocked` — set advisory-lock state predicate.
	if len(parts) >= 2 && isWord(parts[len(parts)-1], "unlocked") {
		ref, err := parseRef(parts[:len(parts)-1])
		if err != nil {
			return nil, &Error{Pos: *src, Msg: err.Error()}
		}
		return graph.UnlockedPred{Ref: ref}, nil
	}
	// `<ref> has next` — queue dequeue (binds `item`).
	if idx := indexOfWord(parts, "has"); idx > 0 && idx == len(parts)-2 && isWord(parts[idx+1], "next") {
		ref, err := parseRef(parts[:idx])
		if err != nil {
			return nil, &Error{Pos: *src, Msg: err.Error()}
		}
		return graph.QueueHasNextPred{Ref: ref}, nil
	}
	// `<ref> has <Field>`
	if idx := indexOfWord(parts, "has"); idx > 0 && idx == len(parts)-2 && parts[idx+1].Kind == ast.PartWord {
		ref, err := parseRef(parts[:idx])
		if err != nil {
			return nil, &Error{Pos: *src, Msg: err.Error()}
		}
		return graph.HasPred{Ref: ref, Field: parts[idx+1].Value}, nil
	}
	// `<ref> is [not] <rhs>` — also accept `was` as a past-tense synonym for
	// inspecting a resolved frame inside `finally` bodies.
	idx := indexOfWord(parts, "is")
	if idx <= 0 {
		idx = indexOfWord(parts, "was")
	}
	if idx > 0 {
		ref, err := parseRef(parts[:idx])
		if err != nil {
			return nil, &Error{Pos: *src, Msg: err.Error()}
		}
		rhs := parts[idx+1:]
		negated := false
		if len(rhs) > 0 && isWord(rhs[0], "not") {
			negated = true
			rhs = rhs[1:]
		}
		if len(rhs) == 0 {
			return nil, &Error{Pos: *src, Msg: "expected value after `is`"}
		}
		// Single-word RHS: prefer canonical status, then graph type,
		// then fall back to a custom status name.
		if !negated && len(rhs) == 1 && rhs[0].Kind == ast.PartWord {
			name := rhs[0].Value
			if isStatusName(name) {
				return graph.StatusPred{Ref: ref, Status: name}, nil
			}
			if node, ok := c.g.Nodes[name]; ok {
				// State slots (`this's <Field>`) are intentionally polymorphic
				// — the script may assign values of any type. Only check refs
				// whose runtime type is truly fixed: `input` (immutable for
				// the frame).
				if c.currentAction != nil && len(ref) > 0 && ref[0] == "input" {
					if rt := c.staticInputRefType(c.currentAction, ref); rt != nil && !typeCompatible(rt, node) {
						return nil, &Error{Code: "impossible-type-pred", Pos: *src, Msg: fmt.Sprintf(
							"`%s is %s?` is impossible: %s has type %s, not %s",
							refString(ref), node.Name, refString(ref), rt.Name, node.Name)}
					}
				}
				return graph.TypePred{Ref: ref, Type: node}, nil
			}
			return graph.StatusPred{Ref: ref, Status: name}, nil
		}
		// Otherwise: equality / inequality with literal or ref.
		expr, err := c.parseExpr(rhs, src)
		if err != nil {
			return nil, err
		}
		if negated {
			return graph.NeqPred{Ref: ref, RHS: expr}, nil
		}
		return graph.EqPred{Ref: ref, RHS: expr}, nil
	}
	return nil, &Error{Pos: *src, Msg: "unsupported predicate clause"}
}

// indexOfWord returns the index of the first occurrence of word w as a Word
// part, or -1 if absent.
func indexOfWord(parts []ast.Part, w string) int {
	for i, p := range parts {
		if isWord(p, w) {
			return i
		}
	}
	return -1
}

// chainSegment is one clause of a run-on sentence. Gate is the predicate
// that must hold before this clause runs (nil means unconditional, used for
// the first clause and for `, then ...` continuations).
type chainSegment struct {
	parts []ast.Part
	gate  []ast.Part // raw predicate parts for `, when <pred>,`; nil for unconditional
}

// splitRunOnChain detects top-level `, then` and `, when <pred>,` connectors
// and splits parts into chain segments. Returns false (no segments) if no
// connector is present at the top level.
func splitRunOnChain(parts []ast.Part) ([]chainSegment, bool) {
	var segments []chainSegment
	cur := chainSegment{}
	consumed := false
	i := 0
	for i < len(parts) {
		if parts[i].Kind == ast.PartComma && i+1 < len(parts) {
			if isWord(parts[i+1], "then") {
				segments = append(segments, cur)
				cur = chainSegment{}
				i += 2
				consumed = true
				continue
			}
			if isWord(parts[i+1], "when") {
				// Find the next top-level comma to close the predicate.
				j := i + 2
				for j < len(parts) && parts[j].Kind != ast.PartComma {
					j++
				}
				if j >= len(parts) {
					// No closing comma — treat as not a chain.
					break
				}
				gateParts := parts[i+2 : j]
				segments = append(segments, cur)
				cur = chainSegment{gate: gateParts}
				i = j + 1
				consumed = true
				continue
			}
		}
		cur.parts = append(cur.parts, parts[i])
		i++
	}
	if !consumed {
		return nil, false
	}
	segments = append(segments, cur)
	return segments, true
}

func isBuiltinCap(name string) bool {
	switch name {
	case "retry", "cancel", "commit", "rollback":
		return true
	}
	return false
}

func lastIndexOfWord(parts []ast.Part, w string) int {
	for i, part := range slices.Backward(parts) {
		if isWord(part, w) {
			return i
		}
	}
	return -1
}

// compileMessageEmit produces an edge that publishes a message (says or write)
// to a target. The body slice is the portion between the verb and `to`; it is
// expected to be either `<MsgName>` or `<MsgName> with <expr>`.
func (c *compiler) compileMessageEmit(kind graph.EdgeKind, body []ast.Part, target string, s *ast.Sentence) ([]graph.Edge, error) {
	if len(body) == 0 || body[0].Kind != ast.PartWord {
		return nil, &Error{Pos: *s, Msg: "expected message name"}
	}
	msgName := body[0].Value
	rest := body[1:]
	var payload graph.Expr
	if len(rest) > 0 {
		if !isWord(rest[0], "with") {
			return nil, &Error{Pos: *s, Msg: "expected `with` before payload"}
		}
		expr, err := c.parseExpr(rest[1:], s)
		if err != nil {
			return nil, err
		}
		payload = expr
	}
	if target != "" {
		if n, ok := c.g.Nodes[target]; ok {
			switch kind {
			case graph.EdgeSays:
				if n.Kind != graph.KindActor && n.Kind != graph.KindChannel && n.Kind != graph.KindSet {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf(
						"`says ... to %s` is invalid: %s is a %s, not a channel, actor, or set",
						target, target, kindWord(n.Kind))}
				}
			case graph.EdgeWriteTo:
				if n.Kind != graph.KindFeed {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf(
						"`write ... to %s` is invalid: %s is a %s, not a feed",
						target, target, kindWord(n.Kind))}
				}
			}
		}
	}
	return []graph.Edge{{
		Kind:    kind,
		Status:  msgName,
		Message: target,
		Expr:    payload,
		Pos:     *s,
	}}, nil
}

// suggestClosest returns ", did you mean \"<name>\"?" when one of the
// candidates is within edit-distance threshold. Threshold scales with the
// shorter of (target, candidate) length: 1 for ≤4-char names, 2 otherwise.
// Returns "" when nothing close enough is found.
func suggestClosest(target string, candidates []string) string {
	if target == "" || len(candidates) == 0 {
		return ""
	}
	best := ""
	bestDist := -1
	for _, c := range candidates {
		if c == target {
			continue
		}
		d := editDistance(target, c)
		threshold := 2
		if min(len(target), len(c)) <= 4 {
			threshold = 1
		}
		if d > threshold {
			continue
		}
		if bestDist < 0 || d < bestDist {
			best = c
			bestDist = d
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf(", did you mean %q?", best)
}

// editDistance is the standard Levenshtein distance (insert/delete/substitute).
func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func kindWord(k graph.NodeKind) string {
	switch k {
	case graph.KindActor:
		return "actor"
	case graph.KindChannel:
		return "channel"
	case graph.KindList:
		return "list"
	case graph.KindMap:
		return "map"
	case graph.KindSet:
		return "set"
	case graph.KindFeed:
		return "feed"
	case graph.KindContract:
		return "contract"
	case graph.KindAction:
		return "action"
	case graph.KindError:
		return "error"
	case graph.KindQueue:
		return "queue"
	case graph.KindTranslator:
		return "translator"
	case graph.KindTest:
		return "test"
	}
	return "node"
}

// isStatusName reports whether w is one of the canonical status names. The
// runtime and compiler share this list with internal/runtime.StatusFromName.
func isStatusName(w string) bool {
	switch w {
	case "created", "running", "waiting", "ok", "failed", "canceled", "exited", "died":
		return true
	}
	return false
}

func (c *compiler) compileBody(sents []ast.Sentence) ([]graph.Edge, error) {
	block, err := c.compileBlock(sents)
	if err != nil {
		return nil, err
	}
	return block.Edges, nil
}

// compileSentence compiles a non-branch-header sentence. Supported forms:
//
//   - `this is ok!`                        → return ok
//   - `this is ok with this's <F>!`        → return ok with self.field
//   - `this is failed with error "<m>"!`   → return failed with literal error message
//   - `log <ref>.`                          → log built-in
//   - `do <Subject>'s <Action>...` (block) → invoke phrase, body is branch group on `that`
func (c *compiler) compileSentence(s *ast.Sentence) ([]graph.Edge, error) {
	parts := s.Parts
	switch s.Term {
	case ast.TermQuestion:
		// `what <ref>?` — debug introspection (no body).
		if len(parts) >= 2 && isWord(parts[0], "what") && len(s.Body) == 0 {
			// Special debug forms encoded via Status.
			if len(parts) == 2 && isWord(parts[1], "happened") {
				return []graph.Edge{{Kind: graph.EdgeWhat, Status: "happened", Pos: *s}}, nil
			}
			if len(parts) == 4 && isWord(parts[1], "was") && isWord(parts[2], "previous") && isWord(parts[3], "that") {
				return []graph.Edge{{Kind: graph.EdgeWhat, Status: "previous", Pos: *s}}, nil
			}
			if len(parts) == 4 && isWord(parts[1], "can") && isWord(parts[2], "that") && isWord(parts[3], "do") {
				return []graph.Edge{{Kind: graph.EdgeWhat, Status: "caps", Pos: *s}}, nil
			}
			expr, err := c.parseExpr(parts[1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeWhat, Expr: expr, Pos: *s}}, nil
		}
		return nil, &Error{Pos: *s, Msg: "unsupported `?` sentence outside a branch group"}
	case ast.TermBang:
		// `says <Msg> [with <expr>] to <Target>!` — emitting form may use `!`
		// per spec; route to message emit before falling through to returns.
		if len(parts) >= 4 && isWord(parts[0], "says") {
			toIdx := lastIndexOfWord(parts, "to")
			if toIdx > 1 && toIdx == len(parts)-2 && parts[toIdx+1].Kind == ast.PartWord {
				body := parts[1:toIdx]
				return c.compileMessageEmit(graph.EdgeSays, body, parts[toIdx+1].Value, s)
			}
		}
		return c.compileReturn(s)
	case ast.TermPeriod:
		// `stop.` — break out of the enclosing loop.
		if len(parts) == 1 && isWord(parts[0], "stop") {
			return []graph.Edge{{Kind: graph.EdgeStop, Pos: *s}}, nil
		}
		// `skip.` — continue to the next loop iteration.
		if len(parts) == 1 && isWord(parts[0], "skip") {
			return []graph.Edge{{Kind: graph.EdgeSkip, Pos: *s}}, nil
		}
		// `it is <Status>.` — non-returning state update on the current frame.
		if len(parts) == 3 && isWord(parts[0], "it") && isWord(parts[1], "is") && parts[2].Kind == ast.PartWord {
			return []graph.Edge{{
				Kind:   graph.EdgeStateUpdate,
				Status: parts[2].Value,
				Pos:    *s,
			}}, nil
		}
		// `it maps what it can.` — translator auto-mapping body.
		if len(parts) == 5 &&
			isWord(parts[0], "it") &&
			isWord(parts[1], "maps") &&
			isWord(parts[2], "what") &&
			isWord(parts[3], "it") &&
			isWord(parts[4], "can") {
			return []graph.Edge{{Kind: graph.EdgeAutoMap, Pos: *s}}, nil
		}
		// Run-on chain: `do X, then do Y[, when <pred>, do Z]!`.
		// Each segment runs in order; segments preceded by `, when <pred>,`
		// are wrapped in a one-arm branch group gated by the predicate.
		if segments, ok := splitRunOnChain(parts); ok {
			var out []graph.Edge
			for _, seg := range segments {
				if len(seg.parts) == 0 {
					continue
				}
				sub := &ast.Sentence{Parts: seg.parts, Term: s.Term, Pos: s.Pos}
				edges, err := c.compileSentence(sub)
				if err != nil {
					return nil, err
				}
				if seg.gate != nil {
					pred, err := c.parsePred(seg.gate, s)
					if err != nil {
						return nil, err
					}
					group := &graph.BranchGroup{ID: c.nextBranchID()}
					out = append(out,
						graph.Edge{Kind: graph.EdgeBranchOpen, Pred: pred, Branch: group, Pos: *s})
					out = append(out, edges...)
					out = append(out, graph.Edge{Kind: graph.EdgeBranchClose, Branch: group, Pos: *s})
				} else {
					out = append(out, edges...)
				}
			}
			return out, nil
		}
		// that is <Contract>.   — frame-level contract adoption inside an
		// action body. Records on the current action so the exhaustiveness
		// checker can verify emitted statuses match the contract.
		if len(parts) == 3 && isWord(parts[0], "that") && isWord(parts[1], "is") && parts[2].Kind == ast.PartWord {
			if c.currentAction != nil {
				c.currentAction.AdoptedContractName = parts[2].Value
			}
			return nil, nil
		}
		// it is a <Contract>.   (in-body adoption — accept as no-op)
		if len(parts) == 4 && isWord(parts[0], "it") && isWord(parts[1], "is") &&
			(isWord(parts[2], "a") || isWord(parts[2], "an")) && parts[3].Kind == ast.PartWord {
			return nil, nil
		}
		// expect <pred>.  — test assertion. Reuses the predicate parser, so all
		// `when ...?` predicate forms (status, exists, equality, has) work.
		if len(parts) >= 2 && isWord(parts[0], "expect") {
			pred, err := c.parsePred(parts[1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeExpect, Pred: pred, Pos: *s}}, nil
		}
		// log <expr>.
		if len(parts) >= 2 && isWord(parts[0], "log") {
			expr, err := c.parseExpr(parts[1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeLog, Expr: expr, Pos: *s}}, nil
		}
		// put <expr> into <Queue>.
		if len(parts) >= 4 && isWord(parts[0], "put") {
			intoIdx := indexOfWord(parts, "into")
			if intoIdx > 1 && intoIdx == len(parts)-2 && parts[intoIdx+1].Kind == ast.PartWord {
				expr, err := c.parseExpr(parts[1:intoIdx], s)
				if err != nil {
					return nil, err
				}
				targetName := parts[intoIdx+1].Value
				if n, ok := c.g.Nodes[targetName]; ok && n.Kind != graph.KindQueue && n.Kind != graph.KindList {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf(
						"`put ... into %s` is invalid: %s is a %s, not a queue or list",
						targetName, targetName, kindWord(n.Kind))}
				}
				return []graph.Edge{{
					Kind:    graph.EdgePutInto,
					Expr:    expr,
					Message: targetName,
					Pos:     *s,
				}}, nil
			}
		}
		// write <Msg> [with <expr>] to <Feed>.
		if len(parts) >= 4 && isWord(parts[0], "write") {
			toIdx := lastIndexOfWord(parts, "to")
			if toIdx > 1 && toIdx == len(parts)-2 && parts[toIdx+1].Kind == ast.PartWord {
				body := parts[1:toIdx]
				return c.compileMessageEmit(graph.EdgeWriteTo, body, parts[toIdx+1].Value, s)
			}
		}
		// says <Msg> [with <expr>] to <Target>.
		if len(parts) >= 4 && isWord(parts[0], "says") {
			toIdx := lastIndexOfWord(parts, "to")
			if toIdx > 1 && toIdx == len(parts)-2 && parts[toIdx+1].Kind == ast.PartWord {
				body := parts[1:toIdx]
				return c.compileMessageEmit(graph.EdgeSays, body, parts[toIdx+1].Value, s)
			}
		}
		// do <FrameRef> <Cap>.   — built-in capability invocation on a frame
		// e.g. `do that retry.`, `do that cancel.`
		if len(parts) == 3 &&
			isWord(parts[0], "do") &&
			parts[1].Kind == ast.PartWord &&
			parts[2].Kind == ast.PartWord &&
			isBuiltinCap(parts[2].Value) {
			return []graph.Edge{{
				Kind:   graph.EdgeFrameCap,
				Ref:    []string{parts[1].Value},
				Status: parts[2].Value,
				Pos:    *s,
			}}, nil
		}
		// cancel <Ref>.
		if len(parts) >= 2 && isWord(parts[0], "cancel") {
			ref, err := parseRef(parts[1:])
			if err != nil {
				return nil, &Error{Pos: *s, Msg: err.Error()}
			}
			return []graph.Edge{{
				Kind: graph.EdgeCancel,
				Ref:  ref,
				Pos:  *s,
			}}, nil
		}
		// do <Standalone> [with <expr>].   — translator or owner-less action
		if len(parts) >= 2 &&
			isWord(parts[0], "do") &&
			parts[1].Kind == ast.PartWord &&
			(len(parts) < 3 || parts[2].Kind != ast.PartPossessive) {
			name := parts[1].Value
			n, ok := c.g.Nodes[name]
			if ok && (n.Kind == graph.KindTranslator || n.Kind == graph.KindAction) {
				var input graph.Expr
				if len(parts) > 2 {
					if !isWord(parts[2], "with") {
						return nil, &Error{Pos: *s, Msg: "expected `with` after standalone action name"}
					}
					expr, err := c.parseExpr(parts[3:], s)
					if err != nil {
						return nil, err
					}
					input = expr
				}
				return []graph.Edge{{
					Kind:   graph.EdgeInvokePhrase,
					Action: n,
					Expr:   input,
					Pos:    *s,
				}}, nil
			}
		}
		// do <Subject>'s <Action> [with <expr>].
		// `this` is allowed as a subject and resolves to the runtime owner.
		if len(parts) >= 4 &&
			isWord(parts[0], "do") &&
			parts[1].Kind == ast.PartWord &&
			parts[2].Kind == ast.PartPossessive &&
			parts[3].Kind == ast.PartWord {
			subjName := parts[1].Value
			capName := parts[3].Value
			var input graph.Expr
			if len(parts) > 4 {
				if !isWord(parts[4], "with") {
					return nil, &Error{Pos: *s, Msg: "expected `with` after action name"}
				}
				expr, err := c.parseInputExpr(parts[5:], s)
				if err != nil {
					return nil, err
				}
				input = expr
			}
			if subjName == "this" {
				action, err := c.resolveSelfRel(capName, s)
				if err != nil {
					return nil, err
				}
				return []graph.Edge{{
					Kind:   graph.EdgeInvokePhrase,
					Action: action,
					Status: capName,
					Expr:   input,
					Pos:    *s,
				}}, nil
			}
			n, ok := c.g.Nodes[subjName]
			if !ok {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("unknown subject %q%s",
					subjName, suggestClosest(subjName, c.g.Order))}
			}
			cap := findCap(n, capName)
			if cap == nil {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q%s",
					subjName, capName, suggestClosest(capName, capNames(n)))}
			}
			return []graph.Edge{{Kind: graph.EdgeInvokePhrase, Action: cap.Action, Expr: input, Pos: *s}}, nil
		}
		// execute <Subject>'s <Action> [with <expr>].   — fire-and-forget spawn
		if len(parts) >= 4 &&
			isWord(parts[0], "execute") &&
			parts[1].Kind == ast.PartWord &&
			parts[2].Kind == ast.PartPossessive &&
			parts[3].Kind == ast.PartWord {
			subjName := parts[1].Value
			capName := parts[3].Value
			var input graph.Expr
			if len(parts) > 4 {
				if !isWord(parts[4], "with") {
					return nil, &Error{Pos: *s, Msg: "expected `with` after action name"}
				}
				expr, err := c.parseInputExpr(parts[5:], s)
				if err != nil {
					return nil, err
				}
				input = expr
			}
			if subjName == "this" {
				action, err := c.resolveSelfRel(capName, s)
				if err != nil {
					return nil, err
				}
				return []graph.Edge{{
					Kind:   graph.EdgeExecute,
					Action: action,
					Status: capName,
					Expr:   input,
					Pos:    *s,
				}}, nil
			}
			n, ok := c.g.Nodes[subjName]
			if !ok {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("unknown subject %q%s",
					subjName, suggestClosest(subjName, c.g.Order))}
			}
			cap := findCap(n, capName)
			if cap == nil {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q%s",
					subjName, capName, suggestClosest(capName, capNames(n)))}
			}
			return []graph.Edge{{Kind: graph.EdgeExecute, Action: cap.Action, Expr: input, Pos: *s}}, nil
		}
		// the <Name> is <expr>.  — local binding (action/script body)
		if len(parts) >= 4 &&
			isWord(parts[0], "the") &&
			parts[1].Kind == ast.PartWord &&
			isWord(parts[2], "is") {
			expr, err := c.parseExpr(parts[3:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind:   graph.EdgeBindLocal,
				Status: parts[1].Value,
				Expr:   expr,
				Pos:    *s,
			}}, nil
		}
		// inspect <expr>.
		if len(parts) >= 2 && isWord(parts[0], "inspect") {
			expr, err := c.parseExpr(parts[1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeInspect, Expr: expr, Pos: *s}}, nil
		}
		// show <expr>.   (UI render — in the CLI, prints with a "show" prefix)
		if len(parts) >= 2 && isWord(parts[0], "show") {
			expr, err := c.parseExpr(parts[1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeShow, Expr: expr, Pos: *s}}, nil
		}
		// <ref> is <expr>.   — assignment
		if idx := findIs(parts); idx > 0 {
			ref, err := parseRef(parts[:idx])
			if err != nil {
				return nil, &Error{Pos: *s, Msg: err.Error()}
			}
			expr, err := c.parseExpr(parts[idx+1:], s)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeAssign, Ref: ref, Expr: expr, Pos: *s}}, nil
		}
		return nil, &Error{Pos: *s, Msg: "unsupported `.` sentence — see spec/Core.md for the sentences Marco understands"}
	case ast.TermEllipsis:
		// for each [<Name>] in <ref>...
		if len(parts) >= 4 &&
			isWord(parts[0], "for") &&
			isWord(parts[1], "each") {
			rest := parts[2:]
			varName := "item"
			if isWord(rest[0], "in") {
				rest = rest[1:]
			} else if rest[0].Kind == ast.PartWord && len(rest) >= 2 && isWord(rest[1], "in") {
				varName = rest[0].Value
				rest = rest[2:]
			} else {
				return nil, &Error{Pos: *s, Msg: "expected `in` in `for each`"}
			}
			ref, err := parseRef(rest)
			if err != nil {
				return nil, &Error{Pos: *s, Msg: err.Error()}
			}
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind:   graph.EdgeForEach,
				Ref:    ref,
				Status: varName,
				Body:   body,
				Pos:    *s,
			}}, nil
		}
		// lock <ref>... — acquire the set's advisory lock for the body's duration.
		if len(parts) >= 2 && isWord(parts[0], "lock") {
			ref, err := parseRef(parts[1:])
			if err != nil {
				return nil, &Error{Pos: *s, Msg: "lock: " + err.Error()}
			}
			if len(ref) == 1 {
				if n, ok := c.g.Nodes[ref[0]]; ok && n.Kind != graph.KindSet {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf(
						"`lock %s` is invalid: %s is a %s, not a set",
						ref[0], ref[0], kindWord(n.Kind))}
				}
			}
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind: graph.EdgeLock,
				Ref:  ref,
				Body: body,
				Pos:  *s,
			}}, nil
		}
		// finally... — register cleanup body that runs on every terminal status.
		if len(parts) == 1 && isWord(parts[0], "finally") {
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{Kind: graph.EdgeFinally, Body: body, Pos: *s}}, nil
		}
		// while <pred>...
		if len(parts) >= 2 && isWord(parts[0], "while") {
			pred, err := c.parsePred(parts[1:], s)
			if err != nil {
				return nil, err
			}
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind: graph.EdgeWhile,
				Pred: pred,
				Body: body,
				Pos:  *s,
			}}, nil
		}
		// repeat <N> times... — run the body a fixed number of times.
		if len(parts) == 3 &&
			isWord(parts[0], "repeat") &&
			parts[1].Kind == ast.PartNumber &&
			isWord(parts[2], "times") {
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind:   graph.EdgeRepeat,
				Status: parts[1].Value, // literal count; parsed at runtime
				Body:   body,
				Pos:    *s,
			}}, nil
		}
		// start <Subject>'s <Action> [with <expr>] [as <Name>]...
		// Also supports `start this's <Cap>...` self-relative.
		if len(parts) >= 4 &&
			isWord(parts[0], "start") &&
			parts[1].Kind == ast.PartWord &&
			parts[2].Kind == ast.PartPossessive &&
			parts[3].Kind == ast.PartWord {
			subjName := parts[1].Value
			capName := parts[3].Value
			var action *graph.Node
			selfRel := subjName == "this"
			if selfRel {
				resolved, err := c.resolveSelfRel(capName, s)
				if err != nil {
					return nil, err
				}
				action = resolved
			} else {
				n, ok := c.g.Nodes[subjName]
				if !ok {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf("unknown subject %q%s",
						subjName, suggestClosest(subjName, c.g.Order))}
				}
				cap := findCap(n, capName)
				if cap == nil {
					return nil, &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q%s",
						subjName, capName, suggestClosest(capName, capNames(n)))}
				}
				action = cap.Action
			}
			rest := parts[4:]
			var input graph.Expr
			frameName := ""
			asIdx := indexOfWord(rest, "as")
			var middle []ast.Part
			if asIdx >= 0 {
				if asIdx == len(rest)-1 || rest[asIdx+1].Kind != ast.PartWord {
					return nil, &Error{Pos: *s, Msg: "expected name after `as` in start"}
				}
				frameName = rest[asIdx+1].Value
				middle = rest[:asIdx]
			} else {
				middle = rest
			}
			if len(middle) > 0 {
				if !isWord(middle[0], "with") {
					return nil, &Error{Pos: *s, Msg: "expected `with` before payload"}
				}
				expr, err := c.parseInputExpr(middle[1:], s)
				if err != nil {
					return nil, err
				}
				input = expr
			}
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			edge := graph.Edge{
				Kind:    graph.EdgeStart,
				Action:  action,
				Message: frameName,
				Expr:    input,
				Body:    body,
				Pos:     *s,
			}
			if selfRel {
				edge.Status = capName // resolved against owner at runtime
			}
			return []graph.Edge{edge}, nil
		}
		// wait until <pred>...
		if len(parts) >= 3 && isWord(parts[0], "wait") && isWord(parts[1], "until") {
			pred, err := c.parsePred(parts[2:], s)
			if err != nil {
				return nil, err
			}
			body, err := c.compileBlock(s.Body)
			if err != nil {
				return nil, err
			}
			return []graph.Edge{{
				Kind: graph.EdgeWaitUntil,
				Pred: pred,
				Body: body,
				Pos:  *s,
			}}, nil
		}
		// do <Subject>'s <Action> [with <expr>]... (with body) — script entry form.
		// Also supports `do this's <Cap>...` for self-relative invocation.
		if len(parts) >= 4 &&
			isWord(parts[0], "do") &&
			parts[1].Kind == ast.PartWord &&
			parts[2].Kind == ast.PartPossessive &&
			parts[3].Kind == ast.PartWord {
			subjName := parts[1].Value
			capName := parts[3].Value
			var input graph.Expr
			if len(parts) > 4 {
				if !isWord(parts[4], "with") {
					return nil, &Error{Pos: *s, Msg: "expected `with` after action name"}
				}
				expr, err := c.parseInputExpr(parts[5:], s)
				if err != nil {
					return nil, err
				}
				input = expr
			}
			body, err := c.compileBody(s.Body)
			if err != nil {
				return nil, err
			}
			if subjName == "this" {
				action, err := c.resolveSelfRel(capName, s)
				if err != nil {
					return nil, err
				}
				edges := []graph.Edge{{
					Kind:   graph.EdgeInvokePhrase,
					Action: action,
					Status: capName,
					Expr:   input,
					Pos:    *s,
				}}
				edges = append(edges, body...)
				return edges, nil
			}
			n, ok := c.g.Nodes[subjName]
			if !ok {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("unknown subject %q%s",
					subjName, suggestClosest(subjName, c.g.Order))}
			}
			cap := findCap(n, capName)
			if cap == nil {
				return nil, &Error{Pos: *s, Msg: fmt.Sprintf("%s has no capability %q%s",
					subjName, capName, suggestClosest(capName, capNames(n)))}
			}
			edges := []graph.Edge{{Kind: graph.EdgeInvokePhrase, Action: cap.Action, Expr: input, Pos: *s}}
			edges = append(edges, body...)
			return edges, nil
		}
		return nil, &Error{Pos: *s, Msg: "unsupported `...` sentence — see spec/Core.md for the sentences Marco understands"}
	default:
		return nil, &Error{Pos: *s, Msg: "unsupported sentence terminator — Marco sentences end in `.`, `!`, `?` or `...`"}
	}
}

func (c *compiler) compileReturn(s *ast.Sentence) ([]graph.Edge, error) {
	parts := s.Parts
	// this is that!  — adopt that's status/result/error verbatim.
	if len(parts) == 3 && isWord(parts[0], "this") && isWord(parts[1], "is") && isWord(parts[2], "that") {
		return []graph.Edge{{Kind: graph.EdgeReturnPassthrough, Pos: *s}}, nil
	}
	// this is <status>!
	if len(parts) == 3 && isWord(parts[0], "this") && isWord(parts[1], "is") && parts[2].Kind == ast.PartWord {
		return []graph.Edge{{Kind: graph.EdgeReturnOK, Status: parts[2].Value, Pos: *s}}, nil
	}
	// this is <status> with <expr>!
	if len(parts) >= 5 &&
		isWord(parts[0], "this") &&
		isWord(parts[1], "is") &&
		parts[2].Kind == ast.PartWord &&
		isWord(parts[3], "with") {
		// Special case: `this is failed with error "<msg>"!`
		if parts[2].Value == "failed" && len(parts) == 6 &&
			isWord(parts[4], "error") &&
			parts[5].Kind == ast.PartString {
			return []graph.Edge{{
				Kind:    graph.EdgeReturnFailedWithError,
				Status:  "failed",
				Message: parts[5].Value,
				Pos:     *s,
			}}, nil
		}
		// Multi-value return: `with X as A, Y as B` builds a set { A: X, B: Y }.
		if expr, ok, err := c.parseMultiAs(parts[4:], s); err != nil {
			return nil, err
		} else if ok {
			kind := graph.EdgeReturnOKWith
			if parts[2].Value != "ok" {
				kind = graph.EdgeReturnFailedWith
			}
			return []graph.Edge{{Kind: kind, Status: parts[2].Value, Expr: expr, Pos: *s}}, nil
		}
		expr, err := c.parseExpr(parts[4:], s)
		if err != nil {
			return nil, err
		}
		kind := graph.EdgeReturnOKWith
		if parts[2].Value != "ok" {
			kind = graph.EdgeReturnFailedWith
		}
		return []graph.Edge{{
			Kind:   kind,
			Status: parts[2].Value,
			Expr:   expr,
			Pos:    *s,
		}}, nil
	}
	return nil, &Error{Pos: *s, Msg: "unsupported return form"}
}

// parseMultiAs detects `<expr> as <Field> [, <expr> as <Field>]*` and returns
// a ConstructExpr with no Type. Returns ok=false if the shape doesn't match.
func (c *compiler) parseMultiAs(parts []ast.Part, src *ast.Sentence) (graph.Expr, bool, error) {
	// Need at least one top-level `as` for this form.
	if indexOfWord(parts, "as") < 0 {
		return nil, false, nil
	}
	// Split on top-level commas.
	var segments [][]ast.Part
	start := 0
	for i, p := range parts {
		if p.Kind == ast.PartComma {
			segments = append(segments, parts[start:i])
			start = i + 1
		}
	}
	segments = append(segments, parts[start:])

	ce := graph.ConstructExpr{Pos: *src}
	for _, seg := range segments {
		// Each segment must be `<expr> as <Field>`.
		asIdx := lastIndexOfWord(seg, "as")
		if asIdx <= 0 || asIdx != len(seg)-2 || seg[asIdx+1].Kind != ast.PartWord {
			return nil, false, nil
		}
		field := seg[asIdx+1].Value
		expr, err := c.parseExpr(seg[:asIdx], src)
		if err != nil {
			return nil, false, err
		}
		ce.Fields = append(ce.Fields, graph.ConstructField{Name: field, Expr: expr})
	}
	return ce, true, nil
}

// parseRef reads a reference chain like `this's State` or `that's error` or
// just `that` from a slice of sentence parts. After the first segment, ref
// elements may be strings or numbers (map keys), e.g. `Counts's "alice"` or
// `Map's 5`.
//
// Special case: a leading `its` greedily consumes the following Word as if it
// were a possessive chain, so `its error` parses as ["its", "error"]. This
// matches the spec where `its` is the possessive form of `it`.
func parseRef(parts []ast.Part) ([]string, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty reference")
	}
	if parts[0].Kind != ast.PartWord {
		return nil, fmt.Errorf("expected word at start of reference")
	}
	out := []string{parts[0].Value}
	i := 1
	if parts[0].Value == "its" && i < len(parts) && parts[i].Kind == ast.PartWord {
		out = append(out, parts[i].Value)
		i++
	}
	for i < len(parts) {
		if parts[i].Kind != ast.PartPossessive {
			break
		}
		i++
		if i >= len(parts) {
			return nil, fmt.Errorf("expected name after `'s`")
		}
		switch parts[i].Kind {
		case ast.PartWord, ast.PartString, ast.PartNumber:
			out = append(out, parts[i].Value)
		default:
			return nil, fmt.Errorf("expected name, string, or number after `'s`")
		}
		i++
	}
	return out, nil
}

func isWord(p ast.Part, w string) bool {
	return p.Kind == ast.PartWord && p.Value == w
}

// findIs returns the index of the first standalone `is` word in parts, or -1.
// `is` is standalone when it isn't part of a possessive chain (won't be
// preceded immediately by a Possessive marker; words separated only by
// whitespace are independent at the parts level).
func findIs(parts []ast.Part) int {
	for i, p := range parts {
		if isWord(p, "is") {
			return i
		}
	}
	return -1
}

// parseExpr converts the post-`is` portion of a sentence into an Expr.
// Supported forms:
//
//	"<text>"          — literal
//	42 / true / false — literal
//	error "<msg>"     — error literal
//	a <Type>          — empty construction
//	a <Type> with <Field> <Expr>, ...   — inline construction
//	an <Type> ...     — same with `an`
//	<ref chain>       — possessive reference
//	<expr> + <expr>   — concatenation / addition (left-to-right)
func (c *compiler) parseExpr(parts []ast.Part, src *ast.Sentence) (graph.Expr, error) {
	if len(parts) == 0 {
		return nil, &Error{Pos: *src, Msg: "expected expression"}
	}
	// `<expr> as a <Type>` / `<expr> as an <Type>` — cast / translator dispatch.
	// The `as` binds tighter than `+`, so split it last (rightmost wins) but
	// before the +-split so the source can itself be a `+` expression.
	if idx := lastTopLevelAsACast(parts); idx > 0 {
		// parts: [...source..., "as", "a"|"an", [optional "partial",] TypeWord]
		typeIdx := idx + 2
		partial := false
		if typeIdx < len(parts) && isWord(parts[typeIdx], "partial") {
			partial = true
			typeIdx++
		}
		if typeIdx >= len(parts) || parts[typeIdx].Kind != ast.PartWord {
			return nil, &Error{Pos: *src, Msg: "expected type name after `as a`"}
		}
		typeName := parts[typeIdx].Value
		typ, ok := c.g.Nodes[typeName]
		if !ok {
			return nil, &Error{Pos: *src, Msg: fmt.Sprintf("unknown type %q in cast", typeName)}
		}
		source, err := c.parseExpr(parts[:idx], src)
		if err != nil {
			return nil, err
		}
		return graph.CastExpr{Source: source, Target: typ, Partial: partial, Pos: *src}, nil
	}
	// Split on top-level `+` for binary operations (left-associative).
	if idx := lastTopLevelPlus(parts); idx > 0 {
		left, err := c.parseExpr(parts[:idx], src)
		if err != nil {
			return nil, err
		}
		right, err := c.parseAtom(parts[idx+1:], src)
		if err != nil {
			return nil, err
		}
		return graph.BinaryExpr{Op: "+", L: left, R: right}, nil
	}
	return c.parseAtom(parts, src)
}

// lastTopLevelAsACast finds the rightmost top-level `as a <Type>` (possibly
// `as a partial <Type>`) and returns the index of `as`. Returns -1 if none
// found.
func lastTopLevelAsACast(parts []ast.Part) int {
	for i := len(parts) - 3; i >= 0; i-- {
		if !isWord(parts[i], "as") {
			continue
		}
		if !(isWord(parts[i+1], "a") || isWord(parts[i+1], "an")) {
			continue
		}
		// Either `as a <Type>` or `as a partial <Type>`.
		if parts[i+2].Kind == ast.PartWord {
			if isWord(parts[i+2], "partial") {
				if i+3 < len(parts) && parts[i+3].Kind == ast.PartWord {
					return i
				}
				continue
			}
			return i
		}
	}
	return -1
}

// parseInputExpr parses the post-`with` portion of an invocation. It splits
// top-level `and` connectors into a list of positional arguments. With one
// element, behaves like parseExpr; with multiple, returns a ListExpr.
func (c *compiler) parseInputExpr(parts []ast.Part, src *ast.Sentence) (graph.Expr, error) {
	segments := splitTopLevelAnds(parts)
	if len(segments) == 1 {
		return c.parseExpr(parts, src)
	}
	items := make([]graph.Expr, 0, len(segments))
	for _, seg := range segments {
		expr, err := c.parseExpr(seg, src)
		if err != nil {
			return nil, err
		}
		items = append(items, expr)
	}
	return graph.ListExpr{Items: items}, nil
}

// splitTopLevelAnds splits on top-level `and` words. Returns the original
// parts as one segment if no `and` is found.
func splitTopLevelAnds(parts []ast.Part) [][]ast.Part {
	var segments [][]ast.Part
	start := 0
	for i, p := range parts {
		if isWord(p, "and") {
			segments = append(segments, parts[start:i])
			start = i + 1
		}
	}
	segments = append(segments, parts[start:])
	return segments
}

func (c *compiler) parseAtom(parts []ast.Part, src *ast.Sentence) (graph.Expr, error) {
	if len(parts) == 0 {
		return nil, &Error{Pos: *src, Msg: "expected expression"}
	}
	if len(parts) == 1 {
		switch parts[0].Kind {
		case ast.PartString, ast.PartNumber, ast.PartBoolean:
			return graph.LiteralExpr{Kind: parts[0].Kind, Text: parts[0].Value, Pos: *src}, nil
		}
	}
	// <ref> at <Index>   — list access
	if idx := lastIndexOfWord(parts, "at"); idx > 0 {
		left, err := c.parseAtom(parts[:idx], src)
		if err != nil {
			return nil, err
		}
		right, err := c.parseAtom(parts[idx+1:], src)
		if err != nil {
			return nil, err
		}
		return graph.ListAtExpr{List: left, Index: right}, nil
	}
	// error "<msg>"
	if len(parts) == 2 && isWord(parts[0], "error") && parts[1].Kind == ast.PartString {
		return graph.ErrorLiteralExpr{Message: parts[1].Value, Pos: *src}, nil
	}
	// a/an <Type> [with <Field> <Expr>, ...]
	if len(parts) >= 2 && (isWord(parts[0], "a") || isWord(parts[0], "an")) && parts[1].Kind == ast.PartWord {
		typeName := parts[1].Value
		typ, ok := c.g.Nodes[typeName]
		if !ok {
			return nil, &Error{Pos: *src, Msg: fmt.Sprintf("unknown type %q", typeName)}
		}
		ce := graph.ConstructExpr{Type: typ, Pos: *src}
		if len(parts) > 2 {
			if !isWord(parts[2], "with") {
				return nil, &Error{Pos: *src, Msg: "expected `with` after type name"}
			}
			fields, err := c.parseConstructFields(parts[3:], src)
			if err != nil {
				return nil, err
			}
			ce.Fields = fields
		}
		return ce, nil
	}
	ref, err := parseRef(parts)
	if err != nil {
		return nil, &Error{Pos: *src, Msg: err.Error()}
	}
	return graph.RefExpr{Path: ref, Pos: *src}, nil
}

func lastTopLevelPlus(parts []ast.Part) int {
	for i, part := range slices.Backward(parts) {
		if part.Kind == ast.PartPlus {
			return i
		}
	}
	return -1
}

// parseConstructFields parses `Field <Expr>, Field <Expr>, ...` —
// Each field's expression is a single literal or single-name reference.
func (c *compiler) parseConstructFields(parts []ast.Part, src *ast.Sentence) ([]graph.ConstructField, error) {
	var out []graph.ConstructField
	i := 0
	for i < len(parts) {
		if parts[i].Kind != ast.PartWord {
			return nil, &Error{Pos: *src, Msg: "expected field name in construction"}
		}
		name := parts[i].Value
		i++
		// Read value: literal or single-token ref.
		if i >= len(parts) {
			return nil, &Error{Pos: *src, Msg: fmt.Sprintf("expected value for field %s", name)}
		}
		// Find the end of this field's value (next comma or end of parts).
		valEnd := i
		for valEnd < len(parts) && parts[valEnd].Kind != ast.PartComma {
			valEnd++
		}
		expr, err := c.parseExpr(parts[i:valEnd], src)
		if err != nil {
			return nil, err
		}
		out = append(out, graph.ConstructField{Name: name, Expr: expr})
		i = valEnd
		if i < len(parts) && parts[i].Kind == ast.PartComma {
			i++
		}
	}
	return out, nil
}

func findCap(n *graph.Node, name string) *graph.Capability {
	for _, c := range n.Caps {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// checkExportsBelongToActs holds the one distinction between `act`, `scene` and `actor` that
// the compiler can genuinely check.
//
// # Why these three words share a Kind, and why that is fine
//
// An act, a scene and an actor all own state, declare verbs, and hear and say. At runtime they
// behave identically, so they share KindActor — and sharing machinery beneath distinct meanings
// is not a bug. What WOULD be a bug is the language losing the distinction the author wrote
// down, which is why Node.Declared keeps the authored word.
//
// # What the distinction actually is
//
//	act    organises what the play can do, and is the boundary that EXPOSES capabilities
//	       outwards. `this exports …` is an act's sentence, and an exported capability with no
//	       body is fulfilled by a host.
//	scene  describes where things happen. It may hold actors and declare verbs.
//	actor  is a thing in the play.
//
// Only the first of those is checkable from the graph alone, and it is the one that matters:
// `exports` on an actor used to compile, which meant "act" carried no obligation a reader could
// rely on. A scene or an actor that exports is claiming to be a module boundary, and the honest
// answer is to say so rather than to let the word drift.
func (c *compiler) checkExportsBelongToActs() error {
	for _, name := range c.g.Order {
		n := c.g.Nodes[name]
		if n.Kind != graph.KindActor || n.Declared == "act" || n.Declared == "" {
			continue
		}
		for _, cap := range n.Caps {
			if !cap.Exported {
				continue
			}
			pos := ast.Sentence{}
			if cap.Action != nil && cap.Action.Body != nil {
				pos = *cap.Action.Body
			}
			c.report(&Error{
				Pos: pos,
				Msg: fmt.Sprintf(
					"%s is %s %s, and only an act exports.\n"+
						"  An act is the boundary that offers capabilities outwards — it is "+
						"what `use` brings in and what a host fulfils.\n"+
						"  %s %s is part of the play, not a way in: give %s a verb with "+
						"`this can %s.` instead.",
					n.Name, article(n.Declared), n.Declared,
					capitalise(article(n.Declared)), n.Declared, n.Name, cap.Name),
			})
		}
	}
	return nil
}

func article(word string) string {
	if word == "" {
		return "a"
	}
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	}
	return "a"
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
