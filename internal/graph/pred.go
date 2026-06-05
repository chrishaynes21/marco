package graph

// Pred is the predicate AST used by `when` headers and `while` loops.
type Pred interface{ isPred() }

// StatusPred is `<ref> is <Status>?` (or shorthand `<Status>?`).
type StatusPred struct {
	Ref    []string
	Status string
}

func (StatusPred) isPred() {}

// ExistsPred is `<ref> exists?`.
type ExistsPred struct {
	Ref []string
}

func (ExistsPred) isPred() {}

// HasPred is `<ref> has <Field>?`.
type HasPred struct {
	Ref   []string
	Field string
}

func (HasPred) isPred() {}

// EqPred is `<ref> is <expr>?`.
type EqPred struct {
	Ref []string
	RHS Expr
}

func (EqPred) isPred() {}

// NeqPred is `<ref> is not <expr>?`.
type NeqPred struct {
	Ref []string
	RHS Expr
}

func (NeqPred) isPred() {}

// AndPred / OrPred combine sub-predicates.
type AndPred struct{ Sub []Pred }

func (AndPred) isPred() {}

type OrPred struct{ Sub []Pred }

func (OrPred) isPred() {}

// TypePred is `<ref> is <Type>?` (runtime type assertion).
type TypePred struct {
	Ref  []string
	Type *Node
}

func (TypePred) isPred() {}

// QueueHasNextPred is `<Queue> has next?` — pops the head into `item` if
// present and returns true, returns false if empty.
type QueueHasNextPred struct {
	Ref []string
}

func (QueueHasNextPred) isPred() {}

// UnlockedPred is `<Set> unlocked?` — true when the set's advisory lock is
// not held. In `wait until <Set> unlocked?` it blocks until the lock is
// observed unheld.
type UnlockedPred struct {
	Ref []string
}

func (UnlockedPred) isPred() {}

// TimePred is `it's been <N> seconds?` — true when the current frame has
// existed for at least N seconds. Used by `when` headers and `wait until`.
type TimePred struct {
	Seconds float64
}

func (TimePred) isPred() {}

// NotPred negates its sub-predicate. `not <pred>`.
type NotPred struct {
	Sub Pred
}

func (NotPred) isPred() {}
