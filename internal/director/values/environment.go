package values

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Environment is one running program's captured values.
//
//	Values belong to ONE executing program. When the program finishes, they disappear.
//
// There is no Save, no path and no persistence, and that absence is the design rather
// than an unfinished edge. A value is a fact about a screen at a moment; a persisted
// one would be reused later against a screen nobody was looking at, silently, in a
// context it was never captured for. Target variables persist because a QUERY stays
// meaningful; a value does not.
type Environment struct {
	mu     sync.RWMutex
	values map[string]Value
	// used records which steps consumed which values, so "who read this?" is
	// answerable while the program runs. Program-local like everything else here: it
	// is discarded by Clear along with the values themselves.
	used map[string][]Consumption
	// program names the run these values belong to, for diagnostics.
	program string
	// cleared records that the program this belonged to has finished.
	//
	// Distinct from "empty". A cleared environment must not be quietly reusable: a
	// later Bind into it would attach a value to a program that is over, and a later
	// Get would answer from one. Both are refused, so the only way to capture again is
	// a new program with a new environment.
	cleared bool
}

// MaxValues bounds one program's environment.
//
// A program is capped at ten steps, so an environment larger than this is a bug rather
// than an intention. Named so the limit is arguable instead of mysterious.
const MaxValues = 20

// MaxTextLength bounds a single non-secret value.
//
// 64 KiB: comfortably more than any field a person fills in, and small enough that a
// runaway capture cannot exhaust memory. Exceeding it REJECTS rather than truncating —
// a silently shortened value would be typed into a field as though it were complete.
const MaxTextLength = 64 * 1024

// NewEnvironment returns an empty environment.
func NewEnvironment() *Environment {
	return &Environment{values: map[string]Value{}, used: map[string][]Consumption{}}
}

// SetProgram names the run these values belong to, for diagnostics.
func (e *Environment) SetProgram(id string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.program = id
}

// Program is the run these values belong to.
func (e *Environment) Program() string {
	if e == nil {
		return ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.program
}

// RecordConsumption files a step's successful use of a value.
//
// Only a use that REACHED EXECUTION is recorded. A failed lookup is traced — it is a
// real event and worth seeing — but filing it here would make the history claim the
// value was used when it was not.
func (e *Environment) RecordConsumption(name string, c Consumption) {
	if e == nil {
		return
	}
	normalised, err := NormalizeName(name)
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleared {
		return
	}
	if _, bound := e.values[normalised]; !bound {
		return
	}
	e.used[normalised] = append(e.used[normalised], c)
}

// Bind stores a value under a name.
//
// Rebinding an existing name is REFUSED. Values are immutable, and a name that could be
// rebound would give a later step the power to change what an earlier step captured —
// the same hazard as a mutable value, reintroduced through the namespace.
func (e *Environment) Bind(name string, v Value) error {
	if e == nil {
		return fmt.Errorf("values: no environment is running")
	}
	normalised, err := NormalizeName(name)
	if err != nil {
		return err
	}
	if !v.verified {
		// Unreachable through New, which only produces verified values. Kept as a
		// belt-and-braces check because binding an unverified value is precisely the
		// "Unknown became empty" failure this package exists to prevent.
		return fmt.Errorf("values: %q was never verified, so it is not a value", normalised)
	}
	if v.Len() > MaxTextLength && v.visibility != VisibilitySecret {
		return fmt.Errorf("values: %q is %d characters and the limit is %d; it is rejected "+
			"rather than shortened, because a truncated value would be typed as though "+
			"it were complete", normalised, v.Len(), MaxTextLength)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleared {
		return fmt.Errorf("values: this program has finished, so %q cannot be captured "+
			"into it; program-local values do not outlive their program", normalised)
	}
	if len(e.values) >= MaxValues {
		return fmt.Errorf("values: a program may capture at most %d values", MaxValues)
	}
	if _, taken := e.values[normalised]; taken {
		return fmt.Errorf("values: %q is already captured in this program, and values "+
			"are immutable; use a different name", normalised)
	}
	e.values[normalised] = v
	return nil
}

// Get returns a value by name.
//
// Returns a COPY. Value's fields are unexported and it has no mutating methods, so the
// copy cannot be altered — but returning by value rather than by pointer makes that
// structural rather than a convention someone could break later.
func (e *Environment) Get(name string) (Value, bool) {
	if e == nil {
		return Value{}, false
	}
	normalised, err := NormalizeName(name)
	if err != nil {
		return Value{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	v, ok := e.values[normalised]
	return v, ok
}

// Clear discards every value and marks the environment finished.
//
// The EXPLICIT end of the lifetime, called from one place for every terminal outcome —
// completed, failed, unverified, unsafe, unobservable, cancelled, timed out, internal
// error. Relying on the environment simply becoming unreachable would work for the
// garbage collector and not for the guarantee: a secret would stay in memory for an
// unbounded time, and any component that had kept a pointer would still read from it.
//
// Returns how many values were discarded, which is what a trace event reports. The
// count is safe; the contents are not, and are never rendered here.
func (e *Environment) Clear() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	n := len(e.values)
	// Overwritten rather than dropped, so no other reference to the old map can still
	// read a value out of it.
	for name := range e.values {
		delete(e.values, name)
	}
	// The consumption history goes with them. It is program-local for the same reason
	// the values are: "step 3 typed the customer's email" is a fact about a run that is
	// over, and keeping it would be a durable record of a data flow the user was told
	// would not outlive its program.
	for name := range e.used {
		delete(e.used, name)
	}
	e.cleared = true
	return n
}

// Snapshot describes the whole environment safely, as copies.
//
// Taken under the lock and returned detached, which is the discipline the whole
// diagnostic layer depends on: a caller renders, formats and serialises this AFTER the
// lock is released, so a slow terminal or a large JSON payload can never delay the
// program that owns the values.
func (e *Environment) Snapshot() EnvironmentSnapshot {
	if e == nil {
		return EnvironmentSnapshot{TakenAt: time.Now(), Cleared: true, Values: []ValueSnapshot{}}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := EnvironmentSnapshot{
		ProgramID: e.program,
		TakenAt:   time.Now(),
		Cleared:   e.cleared,
		Values:    []ValueSnapshot{},
	}
	names := make([]string, 0, len(e.values))
	for n := range e.values {
		names = append(names, n)
	}
	// Stable ordering, so a reader comparing two snapshots sees what changed rather
	// than what moved.
	sort.Strings(names)

	for _, name := range names {
		v := e.values[name]
		// The consumption slice is COPIED. Returning the live one would let a caller
		// append to the environment's own history, or read it while a step appends.
		used := append([]Consumption(nil), e.used[name]...)
		out.Values = append(out.Values, ValueSnapshot{
			Name:       name,
			Kind:       v.kind,
			Visibility: v.visibility,
			ByteLength: v.Len(),
			CapturedAt: v.capturedAt,
			Verified:   v.verified,
			Provenance: v.prov,
			ConsumedBy: used,
			Preview:    v.preview(),
		})
	}
	return out
}

// Cleared reports whether the program that owned these values has finished.
func (e *Environment) Cleared() bool {
	if e == nil {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cleared
}

// Count is how many values are bound.
func (e *Environment) Count() int {
	if e == nil {
		return 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.values)
}

// Has reports whether a name is bound.
func (e *Environment) Has(name string) bool {
	_, ok := e.Get(name)
	return ok
}

// Names lists what has been captured, in a stable order.
func (e *Environment) Names() []string {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	out := make([]string, 0, len(e.values))
	for n := range e.values {
		out = append(out, n)
	}
	e.mu.RUnlock()
	sort.Strings(out)
	return out
}

// ErrUnknownValue is a reference to a value this program never captured.
//
// The message deliberately says "value", not "variable": a user who mistypes
// `${save}` for `$save` needs to be told which namespace they missed, and the two
// namespaces do not collide.
type ErrUnknownValue struct{ Name string }

func (e *ErrUnknownValue) Error() string {
	return fmt.Sprintf("Unknown value: %s", e.Name)
}

// Resolve looks a reference up, failing honestly when it is not there.
func (e *Environment) Resolve(name string) (Value, error) {
	v, ok := e.Get(name)
	if !ok {
		return Value{}, &ErrUnknownValue{Name: name}
	}
	return v, nil
}

// Snapshot describes the environment for diagnostics, without plaintext.
//
// Every field here is safe at any visibility: a name, a type, a source, a length. The
// content is never included, which is what lets this be printed, traced and serialised
// without a per-call-site decision about whether it is safe.
type Snapshot struct {
	Name       string     `json:"name"`
	Kind       Kind       `json:"kind"`
	Visibility Visibility `json:"visibility"`
	Source     string     `json:"source,omitempty"`
	Length     int        `json:"length"`
	// Preview is the content for NORMAL values only, and <secret> otherwise.
	Preview string `json:"preview"`
}

// Describe snapshots the environment safely.
func (e *Environment) Describe() []Snapshot {
	if e == nil {
		return nil
	}
	out := []Snapshot{}
	for _, name := range e.Names() {
		v, _ := e.Get(name)
		out = append(out, Snapshot{
			Name: name, Kind: v.kind, Visibility: v.visibility,
			Source: v.source, Length: v.Len(),
			// String(), not Plaintext(): the redaction is the type's, not this
			// function's, so it cannot be forgotten here.
			Preview: v.String(),
		})
	}
	return out
}
