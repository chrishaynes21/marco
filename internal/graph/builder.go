package graph

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
)

// Build walks the parsed file and produces the static program graph.
//
// MVP scope (per .claude/engine/09-mvp-slice.md): handle the declaration
// patterns used by the SaveApp target plus a single script body.
func Build(file *ast.File) (*Graph, error) {
	g := New()
	for name := range PrimitiveNames {
		_ = g.AddNode(&Node{Name: name, Kind: KindPrimitive})
	}
	// `any` is a special primitive that disables compile-time type guarantees.
	_ = g.AddNode(&Node{Name: "any", Kind: KindPrimitive})

	b := &builder{g: g}
	for i := range file.Body {
		s := &file.Body[i]
		if err := b.topLevel(s); err != nil {
			return nil, err
		}
	}
	if err := b.resolveTypes(); err != nil {
		return nil, err
	}
	return g, nil
}

type builder struct {
	g                    *Graph
	subject              *Node // current declared subject — what `this` / `it` refers to
	script               *Node
	pendingTypes         []pendingType
	pendingMembers       []pendingMember
	pendingCapTypes      []pendingCapType
	pendingAllowedShapes []pendingAllowedShape
}

type pendingType struct {
	owner    *Node // set/actor whose field needs typing
	fieldIdx int
	typeName string
	pos      ast.Sentence
}

type pendingMember struct {
	owner      *Node
	memberName string
	pos        ast.Sentence
}

func (b *builder) topLevel(s *ast.Sentence) error {
	if imp, ok := matchUseDecl(s); ok {
		b.g.Imports = append(b.g.Imports, imp)
		return nil
	}
	if isListDecl(s) {
		return b.listDecl(s)
	}
	if isMapDecl(s) {
		return b.mapDecl(s)
	}
	if isQueueDecl(s) {
		return b.queueDecl(s)
	}
	if isFeedDecl(s) {
		return b.feedDecl(s)
	}
	if isDeclareKind(s) {
		return b.declareKind(s)
	}
	if isFieldDecl(s) {
		return b.fieldDecl(s)
	}
	if isCanDecl(s) {
		return b.canDecl(s)
	}
	if isAllowsDecl(s) {
		return b.allowsDecl(s)
	}
	if isBuiltInContractDecl(s) {
		return b.builtInContractDecl(s)
	}
	if isCompoundCanDecl(s) {
		return b.compoundCanDecl(s)
	}
	if isHasMemberDecl(s) {
		return b.hasMemberDecl(s)
	}
	if isInlineMockDecl(s) {
		return b.inlineMockDecl(s)
	}
	if isMockDecl(s) {
		return b.mockDecl(s)
	}
	if isExportsDecl(s) {
		return b.exportsDecl(s)
	}
	if isTestDecl(s) {
		return b.testDecl(s)
	}
	if isTakesGivesDecl(s) {
		return b.takesGivesDecl(s)
	}
	if isAdoptionDecl(s) {
		return nil // accepted; contract / template adoption not yet enforced
	}
	if isDefaultDecl(s) {
		return b.defaultDecl(s)
	}
	if isStandaloneBody(s) {
		return b.standaloneBody(s)
	}
	if isActionBody(s) {
		return b.actionBody(s)
	}
	if listener, ok := b.matchListenerDecl(s); ok {
		return b.addListener(s, listener)
	}
	// Anything else at top level is part of the script body (deferred to compiler).
	if b.script == nil {
		return fmt.Errorf("%s: top-level sentence with no script declaration", s.Pos)
	}
	b.script.ScriptBody = append(b.script.ScriptBody, *s)
	return nil
}

// `the X is a/an Y.` — five parts (canonical), or
// `the <kind> X is a/an Y.` — six parts (kind-prefixed declaration).
//
// The trailing word must be a known kind keyword; otherwise this is an
// instance binding (handled later as a local-binding edge).
func isDeclareKind(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod {
		return false
	}
	parts := s.Parts
	if len(parts) == 5 &&
		isWord(parts[0], "the") &&
		parts[1].Kind == ast.PartWord &&
		isWord(parts[2], "is") &&
		(isWord(parts[3], "a") || isWord(parts[3], "an")) &&
		parts[4].Kind == ast.PartWord &&
		isKindKeyword(parts[4].Value) {
		return true
	}
	if len(parts) == 6 &&
		isWord(parts[0], "the") &&
		parts[1].Kind == ast.PartWord &&
		parts[2].Kind == ast.PartWord &&
		isWord(parts[3], "is") &&
		(isWord(parts[4], "a") || isWord(parts[4], "an")) &&
		parts[5].Kind == ast.PartWord &&
		isKindKeyword(parts[5].Value) {
		return true
	}
	return false
}

func isKindKeyword(w string) bool {
	switch w {
	case "set", "actor", "act", "scene", "script", "error", "contract", "channel", "translator", "template":
		return true
	}
	return false
}

func (b *builder) declareKind(s *ast.Sentence) error {
	parts := s.Parts
	var name, kindWord string
	if len(parts) == 5 {
		name = parts[1].Value
		kindWord = parts[4].Value
	} else {
		// Kind-prefixed: `the <kind-prefix> <Name> is a/an <Kind>.`
		// We use the trailing kind for classification.
		name = parts[2].Value
		kindWord = parts[5].Value
	}
	var kind NodeKind
	switch kindWord {
	case "set":
		kind = KindSet
	case "actor", "act", "scene":
		// Acts and scenes are structurally actors in MVP — they own state,
		// declare capabilities, and hear/say messages. The `exports`
		// declaration and scene-specific composition rules can layer on later.
		kind = KindActor
	case "script":
		kind = KindScript
	case "error":
		kind = KindError
	case "contract":
		kind = KindContract
	case "channel":
		kind = KindChannel
	case "translator":
		kind = KindTranslator
	case "template":
		kind = KindTemplate
	default:
		return fmt.Errorf("%s: unknown kind %q", s.Pos, kindWord)
	}
	n := &Node{Name: name, Kind: kind}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject = n
	if kind == KindScript {
		if b.script != nil {
			return fmt.Errorf("%s: multiple script declarations", s.Pos)
		}
		b.script = n
		n.IsScript = true
	}
	return nil
}

// `this's F is a/an T.`  or  `its F is a/an T.`
func isFieldDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod || len(s.Parts) < 5 {
		return false
	}
	// this's F is a T  -> this, 's, F, is, a/an, T
	if isWord(s.Parts[0], "this") && len(s.Parts) >= 6 &&
		s.Parts[1].Kind == ast.PartPossessive &&
		s.Parts[2].Kind == ast.PartWord &&
		isWord(s.Parts[3], "is") &&
		(isWord(s.Parts[4], "a") || isWord(s.Parts[4], "an")) &&
		s.Parts[5].Kind == ast.PartWord {
		return true
	}
	// its F is a T -> its, F, is, a/an, T
	if isWord(s.Parts[0], "its") &&
		s.Parts[1].Kind == ast.PartWord &&
		isWord(s.Parts[2], "is") &&
		(isWord(s.Parts[3], "a") || isWord(s.Parts[3], "an")) &&
		s.Parts[4].Kind == ast.PartWord {
		return true
	}
	return false
}

func (b *builder) fieldDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: field declaration with no current subject", s.Pos)
	}
	var fieldName, typeName string
	if isWord(s.Parts[0], "this") {
		fieldName = s.Parts[2].Value
		typeName = s.Parts[5].Value
	} else {
		fieldName = s.Parts[1].Value
		typeName = s.Parts[4].Value
	}
	b.subject.Fields = append(b.subject.Fields, Field{Name: fieldName, TypeName: typeName})
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    b.subject,
		fieldIdx: len(b.subject.Fields) - 1,
		typeName: typeName,
		pos:      *s,
	})
	return nil
}

// `this exports <Name>.` — marks a capability on the current subject as
// publicly callable. Used by acts to declare their public surface.
func isExportsDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 3 &&
		isWord(s.Parts[0], "this") &&
		isWord(s.Parts[1], "exports") &&
		s.Parts[2].Kind == ast.PartWord
}

func (b *builder) exportsDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `this exports` with no current subject", s.Pos)
	}
	name := s.Parts[2].Value
	for _, c := range b.subject.Caps {
		if c.Name == name {
			c.Exported = true
			return nil
		}
	}
	// Auto-create a placeholder capability so subsequent `this's <Name> does...`
	// finds something to attach to. Acts often `exports` before defining the
	// implementation block.
	action := &Node{
		Name:  b.subject.Name + "'s " + name,
		Kind:  KindAction,
		Owner: b.subject,
	}
	if err := b.g.AddNode(action); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject.Caps = append(b.subject.Caps, &Capability{Name: name, Action: action, Exported: true})
	return nil
}

// `test <Name>...` — declares a named test whose body is a mini-script.
// The test runs in isolation under `marco test`. Pass = body runs to
// completion without error or unhandled failure status.
func isTestDecl(s *ast.Sentence) bool {
	return len(s.Parts) == 2 &&
		isWord(s.Parts[0], "test") &&
		s.Parts[1].Kind == ast.PartWord &&
		s.Body != nil
}

func (b *builder) testDecl(s *ast.Sentence) error {
	name := s.Parts[1].Value
	n := &Node{
		Name:       name,
		Kind:       KindTest,
		ScriptBody: append([]ast.Sentence(nil), s.Body...),
	}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.g.Tests = append(b.g.Tests, n)
	return nil
}

// `mock <Original> with <Replacement>.` — graph-level swap.
func isMockDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 4 &&
		isWord(s.Parts[0], "mock") &&
		s.Parts[1].Kind == ast.PartWord &&
		isWord(s.Parts[2], "with") &&
		s.Parts[3].Kind == ast.PartWord
}

func (b *builder) mockDecl(s *ast.Sentence) error {
	b.g.Mocks = append(b.g.Mocks, MockDecl{
		Original:    s.Parts[1].Value,
		Replacement: s.Parts[3].Value,
		Pos:         *s,
	})
	return nil
}

// `mock <Owner>'s <Cap> is <Status> [with <expr>].` — the inline form. The
// compile step replaces the action's body with a synthetic emission edge.
func isInlineMockDecl(s *ast.Sentence) bool {
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

func (b *builder) inlineMockDecl(s *ast.Sentence) error {
	b.g.InlineMocks = append(b.g.InlineMocks, InlineMockDecl{
		Owner: s.Parts[1].Value,
		Cap:   s.Parts[3].Value,
		Body:  s.Parts[4:], // `is <Status> [with <expr>]`
		Pos:   *s,
	})
	return nil
}

// `it has <Member>.` — adds <Member> to the current subject's Members list.
// Used by group routing (`says X to <Set>`) where the Set is declared with
// member actors. The form is intentionally loose: `<Member>` is a single
// word that must resolve to a known node by the time member resolution runs.
func isHasMemberDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 3 &&
		isWord(s.Parts[0], "it") &&
		isWord(s.Parts[1], "has") &&
		s.Parts[2].Kind == ast.PartWord
}

func (b *builder) hasMemberDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `it has` with no current subject", s.Pos)
	}
	memberName := s.Parts[2].Value
	// Resolve eagerly if the node is already known; otherwise defer.
	// For MVP we do an eager-lookup pass after the file is built — store the
	// name in a field-style entry so resolveTypes can backfill the *Node.
	b.subject.Fields = append(b.subject.Fields, Field{
		Name:     memberName,
		TypeName: memberName, // member's type is itself
	})
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    b.subject,
		fieldIdx: len(b.subject.Fields) - 1,
		typeName: memberName,
		pos:      *s,
	})
	// Also remember this is a member entry so we can promote it once resolved.
	b.pendingMembers = append(b.pendingMembers, pendingMember{
		owner:      b.subject,
		memberName: memberName,
		pos:        *s,
	})
	return nil
}

// `this can N.`
func isCanDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 3 &&
		isWord(s.Parts[0], "this") &&
		isWord(s.Parts[1], "can") &&
		s.Parts[2].Kind == ast.PartWord
}

// `it can <Name> [with [a/an] <Type>] [, gives [a/an] <Type>] [, and is <Contract>].`
//
// All clauses except the action name are optional. The compound form is
// canonical inside contract declarations; in actors/scripts the bare
// `this can N.` form remains the standard.
func isCompoundCanDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) >= 3 &&
		isWord(s.Parts[0], "it") &&
		isWord(s.Parts[1], "can") &&
		s.Parts[2].Kind == ast.PartWord
}

func (b *builder) compoundCanDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `it can` with no current subject", s.Pos)
	}
	parts := s.Parts
	name := parts[2].Value
	for _, c := range b.subject.Caps {
		if c.Name == name {
			return fmt.Errorf("%s: capability %s already declared on %s", s.Pos, name, b.subject.Name)
		}
	}
	action := &Node{
		Name:  b.subject.Name + "'s " + name,
		Kind:  KindAction,
		Owner: b.subject,
	}
	if err := b.g.AddNode(action); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	cap := &Capability{Name: name, Action: action}

	// Walk the remaining parts, splitting on top-level `,` into clauses.
	clauses := splitTopLevelCommas(parts[3:])
	for i, cl := range clauses {
		// Strip leading `and` introducer on non-first clauses.
		if i > 0 && len(cl) > 0 && isWord(cl[0], "and") {
			cl = cl[1:]
		}
		if len(cl) == 0 {
			continue
		}
		if err := b.applyCanClause(s, cap, cl); err != nil {
			return err
		}
	}
	if cap.InputType != nil {
		action.InputType = cap.InputType
	}
	if cap.OutputType != nil {
		action.OutputType = cap.OutputType
	}
	if cap.AdoptedContractName != "" {
		action.AdoptedContractName = cap.AdoptedContractName
	}
	b.subject.Caps = append(b.subject.Caps, cap)
	return nil
}

// applyCanClause attaches one clause (with-input / gives / is-contract) to
// the capability under construction. Type words are queued for resolution.
func (b *builder) applyCanClause(s *ast.Sentence, cap *Capability, cl []ast.Part) error {
	if len(cl) == 0 {
		return nil
	}
	switch {
	case isWord(cl[0], "with"):
		// `with <Type>` or `with a/an [optional] <Type>` or `with a feed <Name>`.
		for _, p := range cl[1:] {
			if p.Kind == ast.PartWord && p.Value == "optional" {
				cap.InputOptional = true
				break
			}
		}
		typeName, err := extractTypeAfter(cl[1:])
		if err != nil {
			return fmt.Errorf("%s: %w", s.Pos, err)
		}
		if t, ok := b.g.Nodes[typeName]; ok {
			cap.InputType = t
		} else {
			b.pendingCapTypes = append(b.pendingCapTypes, pendingCapType{
				cap:      cap,
				typeName: typeName,
				slot:     "input",
				pos:      *s,
			})
		}
	case isWord(cl[0], "gives"):
		typeName, err := extractTypeAfter(cl[1:])
		if err != nil {
			return fmt.Errorf("%s: %w", s.Pos, err)
		}
		if t, ok := b.g.Nodes[typeName]; ok {
			cap.OutputType = t
		} else {
			b.pendingCapTypes = append(b.pendingCapTypes, pendingCapType{
				cap:      cap,
				typeName: typeName,
				slot:     "output",
				pos:      *s,
			})
		}
	case isWord(cl[0], "is"):
		// `is <Contract>` — store name; resolution can wait.
		if len(cl) < 2 || cl[1].Kind != ast.PartWord {
			return fmt.Errorf("%s: expected contract name after `is`", s.Pos)
		}
		cap.AdoptedContractName = cl[1].Value
	default:
		return fmt.Errorf("%s: unrecognized clause in `it can`: %v", s.Pos, partsString(cl))
	}
	return nil
}

// extractTypeAfter parses `[a|an [optional|feed]] <TypeName>` and returns the
// type name. Skips article and modifier words to keep the form forgiving.
func extractTypeAfter(parts []ast.Part) (string, error) {
	i := 0
	for i < len(parts) {
		if parts[i].Kind != ast.PartWord {
			return "", fmt.Errorf("expected word, got %v", parts[i].Kind)
		}
		v := parts[i].Value
		if v == "a" || v == "an" || v == "optional" || v == "feed" {
			i++
			continue
		}
		return v, nil
	}
	return "", fmt.Errorf("missing type name")
}

func splitTopLevelCommas(parts []ast.Part) [][]ast.Part {
	var out [][]ast.Part
	cur := []ast.Part{}
	for _, p := range parts {
		if p.Kind == ast.PartComma {
			out = append(out, cur)
			cur = []ast.Part{}
			continue
		}
		cur = append(cur, p)
	}
	out = append(out, cur)
	return out
}

func partsString(parts []ast.Part) string {
	var b []byte
	for i, p := range parts {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, p.Value...)
	}
	return string(b)
}

// pendingCapType defers cap input/output type resolution.
type pendingCapType struct {
	cap      *Capability
	typeName string
	slot     string // "input" or "output"
	pos      ast.Sentence
}

// `this allows <Status>.` or `this allows <Status> with <Shape>.`
// Recorded on the current subject contract / actor / template; runtime
// behavior is parse-only at this stage.
func isAllowsDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) >= 3 &&
		isWord(s.Parts[0], "this") &&
		isWord(s.Parts[1], "allows") &&
		s.Parts[2].Kind == ast.PartWord
}

func (b *builder) allowsDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `this allows` with no current subject", s.Pos)
	}
	// `this allows <Status>.` (3 parts) or
	// `this allows <Status> with [an optional] <Shape>.` (5+ parts).
	parts := s.Parts
	statusName := parts[2].Value
	allowed := AllowedStatus{Name: statusName}
	if len(parts) > 3 {
		// expect `with ...`
		if !isWord(parts[3], "with") {
			return fmt.Errorf("%s: expected `with` after status name", s.Pos)
		}
		shapeName, err := extractTypeAfter(parts[4:])
		if err != nil {
			return fmt.Errorf("%s: %w", s.Pos, err)
		}
		// Optional flag: present iff the words `an optional` (or `optional`)
		// precede the type name.
		for _, p := range parts[4:] {
			if p.Kind == ast.PartWord && p.Value == "optional" {
				allowed.Optional = true
				break
			}
		}
		// Resolve the shape type lazily — it might not be declared yet.
		if t, ok := b.g.Nodes[shapeName]; ok {
			allowed.Shape = t
		} else {
			b.pendingAllowedShapes = append(b.pendingAllowedShapes, pendingAllowedShape{
				owner:    b.subject,
				index:    len(b.subject.AllowedStatuses),
				typeName: shapeName,
				pos:      *s,
			})
		}
	}
	b.subject.AllowedStatuses = append(b.subject.AllowedStatuses, allowed)
	return nil
}

// isBuiltInContractDecl matches `this is <BuiltIn>.` and `this is <BuiltIn>
// with [an optional] <Shape>.` for the predefined contracts. The builder
// translates these into one or more `this allows ...` declarations on the
// current subject.
func isBuiltInContractDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod {
		return false
	}
	parts := s.Parts
	if len(parts) < 3 {
		return false
	}
	if !isWord(parts[0], "this") || !isWord(parts[1], "is") {
		return false
	}
	if parts[2].Kind != ast.PartWord {
		return false
	}
	return isBuiltInContractName(parts[2].Value)
}

func isBuiltInContractName(w string) bool {
	switch w {
	case "failable", "cancelable", "waitable", "lockable", "retryable", "iterable", "replayable":
		return true
	}
	return false
}

func (b *builder) builtInContractDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `this is %s` with no current subject", s.Pos, s.Parts[2].Value)
	}
	name := s.Parts[2].Value
	parts := s.Parts
	switch name {
	case "failable":
		// this is failable.             → this allows failed.
		// this is failable with error.  → this allows failed with error.
		// this is failable with <Type>. → this allows failed with <Type>.
		if len(parts) == 3 {
			b.subject.AllowedStatuses = append(b.subject.AllowedStatuses,
				AllowedStatus{Name: "failed"})
			return nil
		}
		synth := *s
		synth.Parts = append([]ast.Part{
			{Kind: ast.PartWord, Value: "this", Pos: parts[0].Pos},
			{Kind: ast.PartWord, Value: "allows", Pos: parts[1].Pos},
			{Kind: ast.PartWord, Value: "failed", Pos: parts[2].Pos},
		}, parts[3:]...)
		return b.allowsDecl(&synth)
	case "cancelable":
		if len(parts) != 3 {
			return fmt.Errorf("%s: `this is cancelable` takes no shape", s.Pos)
		}
		b.subject.AllowedStatuses = append(b.subject.AllowedStatuses,
			AllowedStatus{Name: "canceled"})
		return nil
	case "waitable", "lockable", "retryable", "iterable", "replayable":
		// v1: accept; runtime semantics are wired via concrete keywords
		// (`wait`, `lock`, `for each`, `retry`). No status declarations.
		return nil
	}
	return nil
}

type pendingAllowedShape struct {
	owner    *Node
	index    int
	typeName string
	pos      ast.Sentence
}

func (b *builder) canDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `this can` with no current subject", s.Pos)
	}
	name := s.Parts[2].Value
	for _, c := range b.subject.Caps {
		if c.Name == name {
			return fmt.Errorf("%s: capability %s already declared on %s", s.Pos, name, b.subject.Name)
		}
	}
	action := &Node{
		Name:  b.subject.Name + "'s " + name,
		Kind:  KindAction,
		Owner: b.subject,
	}
	if err := b.g.AddNode(action); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject.Caps = append(b.subject.Caps, &Capability{Name: name, Action: action})
	return nil
}

// matchUseDecl matches:
//
//	use <Module>.                      — full import
//	use <Module>'s <Sym>, <Sym>, ...   — selective import
func matchUseDecl(s *ast.Sentence) (ImportDecl, bool) {
	if s.Term != ast.TermPeriod || len(s.Parts) < 2 || !isWord(s.Parts[0], "use") {
		return ImportDecl{}, false
	}
	parts := s.Parts
	if parts[1].Kind != ast.PartWord {
		return ImportDecl{}, false
	}
	module := parts[1].Value
	if len(parts) == 2 {
		return ImportDecl{Module: module}, true
	}
	if parts[2].Kind != ast.PartPossessive {
		return ImportDecl{}, false
	}
	var syms []string
	for i := 3; i < len(parts); i++ {
		switch parts[i].Kind {
		case ast.PartWord:
			syms = append(syms, parts[i].Value)
		case ast.PartComma:
			// skip
		default:
			return ImportDecl{}, false
		}
	}
	return ImportDecl{Module: module, Symbols: syms}, true
}

// `the X is a list of T.`
func isListDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 7 &&
		isWord(s.Parts[0], "the") &&
		s.Parts[1].Kind == ast.PartWord &&
		isWord(s.Parts[2], "is") &&
		(isWord(s.Parts[3], "a") || isWord(s.Parts[3], "an")) &&
		isWord(s.Parts[4], "list") &&
		isWord(s.Parts[5], "of") &&
		s.Parts[6].Kind == ast.PartWord
}

func (b *builder) listDecl(s *ast.Sentence) error {
	name := s.Parts[1].Value
	itemTypeName := s.Parts[6].Value
	n := &Node{Name: name, Kind: KindList}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject = n
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    n,
		fieldIdx: -1,
		typeName: itemTypeName,
		pos:      *s,
	})
	return nil
}

// `the X is a queue of T.`
func isQueueDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 7 &&
		isWord(s.Parts[0], "the") &&
		s.Parts[1].Kind == ast.PartWord &&
		isWord(s.Parts[2], "is") &&
		(isWord(s.Parts[3], "a") || isWord(s.Parts[3], "an")) &&
		isWord(s.Parts[4], "queue") &&
		isWord(s.Parts[5], "of") &&
		s.Parts[6].Kind == ast.PartWord
}

func (b *builder) queueDecl(s *ast.Sentence) error {
	name := s.Parts[1].Value
	itemTypeName := s.Parts[6].Value
	n := &Node{Name: name, Kind: KindQueue}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject = n
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    n,
		fieldIdx: -1,
		typeName: itemTypeName,
		pos:      *s,
	})
	return nil
}

// `the X is a feed of T.`  or  `the X is a replayable feed of T.`
func isFeedDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod {
		return false
	}
	parts := s.Parts
	// the X is a feed of T → 7
	if len(parts) == 7 &&
		isWord(parts[0], "the") &&
		parts[1].Kind == ast.PartWord &&
		isWord(parts[2], "is") &&
		(isWord(parts[3], "a") || isWord(parts[3], "an")) &&
		isWord(parts[4], "feed") &&
		isWord(parts[5], "of") &&
		parts[6].Kind == ast.PartWord {
		return true
	}
	// the X is a replayable feed of T → 8
	if len(parts) == 8 &&
		isWord(parts[0], "the") &&
		parts[1].Kind == ast.PartWord &&
		isWord(parts[2], "is") &&
		(isWord(parts[3], "a") || isWord(parts[3], "an")) &&
		isWord(parts[4], "replayable") &&
		isWord(parts[5], "feed") &&
		isWord(parts[6], "of") &&
		parts[7].Kind == ast.PartWord {
		return true
	}
	return false
}

func (b *builder) feedDecl(s *ast.Sentence) error {
	parts := s.Parts
	name := parts[1].Value
	var itemTypeName string
	replayable := false
	if len(parts) == 7 {
		itemTypeName = parts[6].Value
	} else {
		itemTypeName = parts[7].Value
		replayable = true
	}
	n := &Node{Name: name, Kind: KindFeed, IsReplayable: replayable}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject = n
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    n,
		fieldIdx: -1,
		typeName: itemTypeName,
		pos:      *s,
	})
	return nil
}

// `the X is a map of K to V.`
func isMapDecl(s *ast.Sentence) bool {
	return s.Term == ast.TermPeriod &&
		len(s.Parts) == 9 &&
		isWord(s.Parts[0], "the") &&
		s.Parts[1].Kind == ast.PartWord &&
		isWord(s.Parts[2], "is") &&
		(isWord(s.Parts[3], "a") || isWord(s.Parts[3], "an")) &&
		isWord(s.Parts[4], "map") &&
		isWord(s.Parts[5], "of") &&
		s.Parts[6].Kind == ast.PartWord &&
		isWord(s.Parts[7], "to") &&
		s.Parts[8].Kind == ast.PartWord
}

func (b *builder) mapDecl(s *ast.Sentence) error {
	name := s.Parts[1].Value
	keyTypeName := s.Parts[6].Value
	valTypeName := s.Parts[8].Value
	n := &Node{Name: name, Kind: KindMap}
	if err := b.g.AddNode(n); err != nil {
		return fmt.Errorf("%s: %w", s.Pos, err)
	}
	b.subject = n
	// Map types resolve via a generic lookup — we record both as pending.
	b.pendingTypes = append(b.pendingTypes,
		pendingType{owner: n, fieldIdx: -2, typeName: keyTypeName, pos: *s},
		pendingType{owner: n, fieldIdx: -3, typeName: valTypeName, pos: *s},
	)
	return nil
}

// isDefaultDecl matches `<Type>'s <Field> is <expr> by default.`
func isDefaultDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod || len(s.Parts) < 7 {
		return false
	}
	parts := s.Parts
	if parts[0].Kind != ast.PartWord ||
		parts[1].Kind != ast.PartPossessive ||
		parts[2].Kind != ast.PartWord ||
		!isWord(parts[3], "is") {
		return false
	}
	last := len(parts) - 1
	return isWord(parts[last-1], "by") && isWord(parts[last], "default")
}

func (b *builder) defaultDecl(s *ast.Sentence) error {
	parts := s.Parts
	typeName := parts[0].Value
	fieldName := parts[2].Value
	typ, ok := b.g.Nodes[typeName]
	if !ok {
		return fmt.Errorf("%s: unknown type %q in default declaration", s.Pos, typeName)
	}
	exprParts := parts[4 : len(parts)-2]
	b.g.DefaultDecls = append(b.g.DefaultDecls, &DefaultDecl{
		TypeNode: typ,
		Field:    fieldName,
		Parts:    exprParts,
		Pos:      *s,
	})
	return nil
}

// `[this|it] takes [a/an] T.` / `[this|it] gives [a/an] T.` — translator
// type metadata. Both subjects (`this` / `it`) and optional `a/an` accepted.
func isTakesGivesDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod {
		return false
	}
	parts := s.Parts
	if len(parts) < 3 {
		return false
	}
	if !isWord(parts[0], "this") && !isWord(parts[0], "it") {
		return false
	}
	if !isWord(parts[1], "takes") && !isWord(parts[1], "gives") {
		return false
	}
	// Forms: `... takes T.` (3 parts), `... takes a T.` (4 parts).
	if len(parts) == 3 && parts[2].Kind == ast.PartWord {
		return true
	}
	if len(parts) == 4 &&
		(isWord(parts[2], "a") || isWord(parts[2], "an")) &&
		parts[3].Kind == ast.PartWord {
		return true
	}
	return false
}

func (b *builder) takesGivesDecl(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `takes`/`gives` with no current subject", s.Pos)
	}
	parts := s.Parts
	verb := parts[1].Value
	var typeName string
	if len(parts) == 3 {
		typeName = parts[2].Value
	} else {
		typeName = parts[3].Value
	}
	idx := -10 // sentinel for translator InputType
	if verb == "gives" {
		idx = -11 // sentinel for translator OutputType
	}
	b.pendingTypes = append(b.pendingTypes, pendingType{
		owner:    b.subject,
		fieldIdx: idx,
		typeName: typeName,
		pos:      *s,
	})
	return nil
}

// isAdoptionDecl matches contract / template adoption forms:
//
//	it is a <Contract>.       (4 parts: it, is, a/an, Word)
//	that is a <Contract>.     (4 parts)
//	that is <Contract>.       (3 parts: that, is, Word)
//	it uses <Template>.       (3 parts: it, uses, Word)
func isAdoptionDecl(s *ast.Sentence) bool {
	if s.Term != ast.TermPeriod {
		return false
	}
	parts := s.Parts
	if len(parts) == 4 &&
		(isWord(parts[0], "it") || isWord(parts[0], "that")) &&
		isWord(parts[1], "is") &&
		(isWord(parts[2], "a") || isWord(parts[2], "an")) &&
		parts[3].Kind == ast.PartWord {
		return true
	}
	if len(parts) == 3 &&
		isWord(parts[0], "that") &&
		isWord(parts[1], "is") &&
		parts[2].Kind == ast.PartWord {
		return true
	}
	if len(parts) == 3 &&
		isWord(parts[0], "it") &&
		isWord(parts[1], "uses") &&
		parts[2].Kind == ast.PartWord {
		return true
	}
	return false
}

// `this does...` — standalone body (no capability name). Used by translators.
func isStandaloneBody(s *ast.Sentence) bool {
	return s.Term == ast.TermEllipsis &&
		len(s.Parts) == 2 &&
		isWord(s.Parts[0], "this") &&
		isWord(s.Parts[1], "does")
}

func (b *builder) standaloneBody(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: `this does...` with no current subject", s.Pos)
	}
	if b.subject.Body != nil {
		return fmt.Errorf("%s: %s already has a body", s.Pos, b.subject.Name)
	}
	b.subject.Body = s
	return nil
}

// `this's N does...` (with body).
func isActionBody(s *ast.Sentence) bool {
	return s.Term == ast.TermEllipsis &&
		len(s.Parts) == 4 &&
		isWord(s.Parts[0], "this") &&
		s.Parts[1].Kind == ast.PartPossessive &&
		s.Parts[2].Kind == ast.PartWord &&
		isWord(s.Parts[3], "does")
}

func (b *builder) actionBody(s *ast.Sentence) error {
	if b.subject == nil {
		return fmt.Errorf("%s: action body with no current subject", s.Pos)
	}
	name := s.Parts[2].Value
	cap := findCap(b.subject, name)
	if cap == nil {
		return fmt.Errorf("%s: %s has no capability %q (declare with `this can %s.` first)", s.Pos, b.subject.Name, name, name)
	}
	if cap.Action.Body != nil {
		return fmt.Errorf("%s: capability %s on %s already has a body", s.Pos, name, b.subject.Name)
	}
	cap.Action.Body = s
	return nil
}

func findCap(n *Node, name string) *Capability {
	for _, c := range n.Caps {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// matchListenerDecl recognizes a top-level listener: either
// `when <Target> hears <Msg>?...` / `when <Target> reads <Msg>?...` or the
// `this`-relative form `when this hears <Msg>?...`.
// The optional `from <Source>` clause filters by source.
type listenerMatch struct {
	target  string // "this" or a node name
	verb    string // "hears" | "reads"
	message string
	from    string // empty if no `from` filter
}

func (b *builder) matchListenerDecl(s *ast.Sentence) (listenerMatch, bool) {
	if s.Term != ast.TermQuestion || len(s.Body) == 0 {
		return listenerMatch{}, false
	}
	parts := s.Parts
	if len(parts) < 4 || !isWord(parts[0], "when") || parts[1].Kind != ast.PartWord {
		return listenerMatch{}, false
	}
	verb := parts[2].Value
	if verb != "hears" && verb != "reads" {
		return listenerMatch{}, false
	}
	if parts[3].Kind != ast.PartWord {
		return listenerMatch{}, false
	}
	m := listenerMatch{
		target:  parts[1].Value,
		verb:    verb,
		message: parts[3].Value,
	}
	if len(parts) == 4 {
		return m, true
	}
	// Optional `from <Source>` tail.
	if len(parts) == 6 && isWord(parts[4], "from") && parts[5].Kind == ast.PartWord {
		m.from = parts[5].Value
		return m, true
	}
	return listenerMatch{}, false
}

func (b *builder) addListener(s *ast.Sentence, m listenerMatch) error {
	var target *Node
	if m.target == "this" {
		if b.subject == nil {
			return fmt.Errorf("%s: `when this hears` with no current subject", s.Pos)
		}
		target = b.subject
	} else {
		n, ok := b.g.Nodes[m.target]
		if !ok {
			return fmt.Errorf("%s: unknown listener target %q", s.Pos, m.target)
		}
		target = n
	}
	b.g.Listeners = append(b.g.Listeners, &ListenerDecl{
		Target:     target,
		Message:    m.message,
		Verb:       m.verb,
		FromSource: m.from,
		Source:     s.Body,
	})
	return nil
}

func (b *builder) resolveTypes() error {
	for _, pt := range b.pendingTypes {
		typ, ok := b.g.Nodes[pt.typeName]
		if !ok {
			ownerLabel := pt.owner.Name
			if pt.fieldIdx >= 0 {
				ownerLabel += "." + pt.owner.Fields[pt.fieldIdx].Name
			}
			return fmt.Errorf("%s: unknown type %q on %s", pt.pos.Pos, pt.typeName, ownerLabel)
		}
		switch pt.fieldIdx {
		case -1:
			pt.owner.ItemType = typ
		case -2:
			pt.owner.KeyType = typ
		case -3:
			pt.owner.ValueType = typ
		case -10:
			pt.owner.InputType = typ
		case -11:
			pt.owner.OutputType = typ
		default:
			pt.owner.Fields[pt.fieldIdx].Type = typ
		}
	}
	// Resolve `it has <Member>` entries to *Node and append to Members.
	for _, pm := range b.pendingMembers {
		mem, ok := b.g.Nodes[pm.memberName]
		if !ok {
			return fmt.Errorf("%s: unknown member %q on %s", pm.pos.Pos, pm.memberName, pm.owner.Name)
		}
		pm.owner.Members = append(pm.owner.Members, mem)
	}
	// Resolve deferred cap input/output types.
	for _, pc := range b.pendingCapTypes {
		t, ok := b.g.Nodes[pc.typeName]
		if !ok {
			return fmt.Errorf("%s: unknown type %q on capability %s", pc.pos.Pos, pc.typeName, pc.cap.Name)
		}
		switch pc.slot {
		case "input":
			pc.cap.InputType = t
		case "output":
			pc.cap.OutputType = t
		}
	}
	// Resolve deferred allowed-status shape types.
	for _, ps := range b.pendingAllowedShapes {
		t, ok := b.g.Nodes[ps.typeName]
		if !ok {
			return fmt.Errorf("%s: unknown shape type %q on `this allows`", ps.pos.Pos, ps.typeName)
		}
		ps.owner.AllowedStatuses[ps.index].Shape = t
	}
	return nil
}

func isWord(p ast.Part, w string) bool {
	return p.Kind == ast.PartWord && p.Value == w
}

// Script returns the (single) script node, or nil if none was declared.
func (g *Graph) Script() *Node {
	for _, name := range g.Order {
		n := g.Nodes[name]
		if n.IsScript {
			return n
		}
	}
	return nil
}

// FieldByName returns the field on a node, or nil.
func (n *Node) FieldByName(name string) *Field {
	for i := range n.Fields {
		if n.Fields[i].Name == name {
			return &n.Fields[i]
		}
	}
	return nil
}

// CapByName returns the capability on a node, or nil.
func (n *Node) CapByName(name string) *Capability {
	for _, c := range n.Caps {
		if c.Name == name {
			return c
		}
	}
	return nil
}
