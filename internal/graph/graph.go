// Package graph holds the static program graph that the parser/builder produce
// before execution. The runtime walks this graph; the compiler validates it.
package graph

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
)

// NodeKind names every kind of node Marco can place in the graph.
type NodeKind int

const (
	KindUnknown NodeKind = iota
	KindActor
	KindAction
	KindScript
	KindSet
	KindList
	KindMap
	KindQueue
	KindFeed
	KindChannel
	KindTranslator
	KindTest
	KindTemplate
	KindPrimitive // text, number, boolean, time, id
	KindError
	KindContract
	KindStatus
)

func (k NodeKind) String() string {
	switch k {
	case KindActor:
		return "actor"
	case KindAction:
		return "action"
	case KindScript:
		return "script"
	case KindSet:
		return "set"
	case KindList:
		return "list"
	case KindMap:
		return "map"
	case KindQueue:
		return "queue"
	case KindFeed:
		return "feed"
	case KindChannel:
		return "channel"
	case KindTranslator:
		return "translator"
	case KindTest:
		return "test"
	case KindTemplate:
		return "template"
	case KindPrimitive:
		return "primitive"
	case KindError:
		return "error"
	case KindContract:
		return "contract"
	case KindStatus:
		return "status"
	default:
		return "unknown"
	}
}

// Node represents one named entity in the program graph.
type Node struct {
	Name       string
	Kind       NodeKind
	Fields     []Field
	Caps       []*Capability
	Owner      *Node          // for `this can X` declared inside an actor
	Body       *ast.Sentence  // action: the `does...` sentence; nil otherwise
	ScriptBody []ast.Sentence // script: collected top-level non-declaration sentences
	BodyBlock  *Block         // populated by phrase compiler
	IsScript     bool
	IsReplayable bool // for feeds: buffer history & replay to late listeners

	// Container element types
	ItemType  *Node // list element type
	KeyType   *Node // map key type
	ValueType *Node // map value type

	// Translator type metadata
	InputType  *Node
	OutputType *Node

	// AdoptedContractName names the contract this action adopts via
	// `that is <Contract>.` inside the action body. Resolved by Compile;
	// used by the exhaustiveness checker.
	AdoptedContractName string

	// Foreign marks an action whose implementation is provided by a host in
	// another language. Set by the compiler on exported act capabilities that
	// end the build with no Marco `does...` body. The runtime dispatches these
	// through a Host instead of walking a BodyBlock; the compiler skips
	// body-shape checks and infers the result contract (declared, or the
	// default {ok, failed}). See spec/Hosts.md.
	Foreign bool

	// Members are nodes added via `it has <Member>.` on a set declaration.
	// Used for group routing: `says X to <Set>` fans out to each member.
	Members []*Node

	// AllowedStatuses are status declarations on contracts:
	// `this allows <Status> [with [an optional] <Shape>].`
	AllowedStatuses []AllowedStatus

	// InferredStatuses is the effective-status set computed by the
	// exhaustiveness analyzer for actions and translators. Populated during
	// Compile from explicit returns and floated child statuses. For
	// actions/translators only; nil otherwise.
	InferredStatuses []string

	// ScopedInlineMocks lists inline mocks declared inside a test body. The
	// test runner snapshots each Action's BodyBlock, installs the override,
	// runs the test, then restores. Empty for non-test nodes.
	ScopedInlineMocks []ScopedInlineMock
}

// ScopedInlineMock is a per-test override of an action's body. The compiler
// pre-synthesizes Body so the test runner only swaps pointers.
type ScopedInlineMock struct {
	Action *Node
	Body   *Block
	Pos    ast.Sentence
}

// AllowedStatus is one `this allows <Status> with <Shape>` clause.
type AllowedStatus struct {
	Name     string
	Shape    *Node // shape type, nil if absent
	Optional bool  // `with an optional <Shape>` flag
}

// Field is a typed slot on a set or actor.
type Field struct {
	Name     string
	Type     *Node // resolved later; may be nil during build
	TypeName string
	Default  Expr // optional default value, set by `<X>'s <F> is <V> by default.`
}

// Capability is an action declared as `this can X` plus its action node.
//
// On contracts, the compound form `it can X with Y, gives Z, and is W.` also
// fills InputType, OutputType, and AdoptedContractName (resolved later) for
// member specification.
type Capability struct {
	Name                string
	Action              *Node
	InputType           *Node
	InputOptional       bool // `with an optional <Type>` flag
	OutputType          *Node
	AdoptedContractName string // unresolved; runtime checks if needed
	// Exported is set by `this exports <Name>.` declarations on acts.
	// Tracked but not yet enforced as a public/private boundary.
	Exported bool
}

// Block is the compiled body of an action: a flat sequence of edges that
// the runtime executes in order. Filled by the phrase compiler.
type Block struct {
	Edges []Edge
}

// EdgeKind enumerates the runtime edge types implied by Marco sentence forms.
// MVP supports the subset listed in 09-mvp-slice.md.
type EdgeKind int

const (
	EdgeUnknown EdgeKind = iota
	EdgeReturnOK              // this is ok!
	EdgeReturnOKWith          // this is ok with <expr>!
	EdgeReturnFailedWithError // this is failed with error "<msg>"!
	EdgeReturnFailedWith      // this is failed with <expr>!
	EdgeBranchOpen            // when <pred>?
	EdgeBranchFallback        // or?
	EdgeBranchClose           // end of branch group
	EdgeLog                   // log <expr>.
	EdgeInvokePhrase          // do <action>.
	EdgeFieldExists           // when <ref> exists?
	EdgeAssign                // <ref> is <expr>.
	EdgeForEach               // for each item in <ref>...
	EdgeWhile                 // while <pred>...
	EdgeStart                 // start <action> as <Name>...
	EdgeWaitUntil             // wait until <pred>...
	EdgePutInto               // put <expr> into <Queue>.
	EdgeWriteTo               // write <Msg> [with <Data>] to <Feed>.
	EdgeSays                  // says <Msg> [with <Data>] to <Target>.
	EdgeFinally               // finally... — runs on every terminal status.
	EdgeBindLocal             // the <Name> is <expr>.  — local binding
	EdgeInspect               // inspect <expr>.
	EdgeFrameCap              // do <ref> <cap>.  — invoke built-in cap on a frame
	EdgeCancel                // cancel <ref>.    — cancel a frame
	EdgeShow                  // show <expr> [with <data>].  — UI render
	EdgeWhat                  // what <ref>?      — debug introspection
	EdgeAutoMap               // it maps what it can.  — translator auto-map
	EdgeStop                  // stop.   — break out of enclosing loop
	EdgeSkip                  // skip.   — continue to next iteration
	EdgeStateUpdate           // it is <Status>.  — non-returning status update
	EdgeLock                  // lock <ref>...  — acquire per-set lock for body
	EdgeExecute               // execute <action>.  — fire-and-forget spawn
	EdgeCast                  // <expr> as a <Type>  — translator dispatch / coercion
	EdgeReturnPassthrough     // this is that!  — adopt that's status/result/error verbatim
	EdgeExpect                // expect <pred>.  — test assertion; fails the test if pred is false
)

// Edge is one runtime instruction in a Block.
type Edge struct {
	Kind EdgeKind
	// Generic operands; meaning depends on Kind. Document each use site.
	Ref     []string // a chain of names — e.g. ["this", "State"], ["that", "error"]
	Action  *Node    // for EdgeInvokePhrase: action node to invoke
	Status  string   // for return edges (e.g. "ok", "failed")
	Message string   // for EdgeReturnFailedWithError: literal error message
	Expr    Expr     // for EdgeAssign: right-hand side
	Pred    Pred     // for EdgeBranchOpen / EdgeWhile: composite predicate
	Body    *Block   // for EdgeForEach / EdgeWhile: the inner body
	Branch  *BranchGroup
	Pos     ast.Sentence // origin sentence (kept for diagnostics)
}

// BranchGroup is the metadata for a `when ... ? / or? ` group. One BranchGroup
// is shared by all the edges that belong to the same group; the open edges
// reference it so the runtime can route correctly.
type BranchGroup struct {
	ID int
}

// Graph is the program-level container of nodes.
type Graph struct {
	Nodes        map[string]*Node
	Order        []string // declaration order, for stable iteration
	Listeners    []*ListenerDecl
	Imports      []ImportDecl
	DefaultDecls []*DefaultDecl
	Mocks        []MockDecl
	InlineMocks  []InlineMockDecl
	Tests        []*Node // KindTest nodes, in declaration order
}

// MockDecl is `mock <Original> with <Replacement>.` — the runtime resolves
// references to Original via Replacement when looking up actions/nodes.
// Scoping (per-test vs global) is an open spec question; for now mocks are
// global and applied at graph-build time.
type MockDecl struct {
	Original    string
	Replacement string
	Pos         ast.Sentence
}

// InlineMockDecl is `mock <Owner>'s <Cap> is <Status> [with <expr>].` — the
// compile step replaces the target action's body with a single emission edge
// constructed from these parts. The compiler validates the status is allowed
// by the action's contract and that the (optional) value matches the declared
// shape. Scoping is global, matching MockDecl.
type InlineMockDecl struct {
	Owner string
	Cap   string
	// Body is the parts after `mock <Owner>'s <Cap>` — i.e. `is <Status> [with
	// <expr>]`. Compiled by re-using the action-return parser.
	Body []ast.Part
	Pos  ast.Sentence
}

// DefaultDecl is a `<Type>'s <Field> is <expr> by default.` declaration. The
// expression is parsed by the compile step and assigned to Field.Default.
type DefaultDecl struct {
	TypeNode *Node
	Field    string
	Parts    []ast.Part
	Pos      ast.Sentence
}

// ImportDecl is one parsed `use <M>.` or `use <M>'s <Sym>, <Sym>.` line.
type ImportDecl struct {
	Module  string
	Symbols []string // empty means "import everything"
}

// ListenerDecl is a `when X hears Y [from Z]?` (or `... reads Y?`)
// registration parsed at the declaration level. The compiler turns its body
// into a Block; the runtime attaches it to the target on startup.
type ListenerDecl struct {
	Target     *Node    // actor / channel / feed it listens on
	Message    string   // message name; "" matches any (future)
	Verb       string   // "hears" | "reads"
	FromSource string   // optional: only fire when message came from this name
	Body       *Block
	Source     []ast.Sentence // raw body sentences (compiler input)
}

func New() *Graph {
	return &Graph{Nodes: map[string]*Node{}}
}

func (g *Graph) AddNode(n *Node) error {
	if _, dup := g.Nodes[n.Name]; dup {
		return fmt.Errorf("duplicate declaration: %s", n.Name)
	}
	g.Nodes[n.Name] = n
	g.Order = append(g.Order, n.Name)
	return nil
}

// PrimitiveNames are reserved type names recognized by the builder.
var PrimitiveNames = map[string]bool{
	"text":    true,
	"number":  true,
	"boolean": true,
	"time":    true,
	"id":      true,
	// `error` is the built-in placeholder for "any error type" in shape
	// positions like `this allows failed with an optional error.` It does
	// not gate construction (custom error types still declared via
	// `the X is an error.` keep their own nodes).
	"error": true,
}

// Build is implemented in builder.go.
