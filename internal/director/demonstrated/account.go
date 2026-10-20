package demonstrated

// THE ACCOUNT OF ONE RUN, AND WHY IT IS NOT THE RUNTIME'S ERROR.
//
// A saved play HANDLES ITS OWN FAILURE. Its script ends
//
//	do ArkGraphicsLive's Run...
//	    when ok?
//	        log "…: every action verified".
//	    or?
//	        log that's error.
//
// which is a program that completes successfully whether or not the routine worked — the `or?`
// arm is the play dealing with the failure, exactly as it should. So the runtime returns nil for
// a run whose first interaction was contradicted, and a caller reading the error would report a
// refused press as a successful routine. The first draft of this bridge's acceptance asserted on
// the error and passed four tests it should have failed.
//
// So the account is kept here, one record per sentence, and the outcome is read from it.

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// ErrCancelled is a run that was stopped rather than one that failed.
//
// A sentinel, so the distinction survives being wrapped: "somebody stopped this" and "this did
// not work" are different facts about a half-finished run, and a report that confused them would
// tell a person their routine is broken when what happened is that they pressed stop.
var ErrCancelled = errors.New("the run was cancelled")

// Ask is one sentence of the play, and what the Director made of it.
type Ask struct {
	// Run is the canonical run reference every record of this run shares.
	Run string
	// Name is the interaction as the play named it, and Step which interaction that is.
	Name string
	Step int
	// Verdict is what the post-action evidence said, and Decided whether a verdict was
	// REACHED at all.
	//
	// Both, because `Verdict`'s zero value is `VerdictUnasked` — itself a real answer, for
	// an interaction whose demonstration recorded no outcome — and that is not the same as
	// no verdict having been reached. A control that was never even found returns no
	// verdict, and a reader that could not tell those apart would report "the deadline
	// passed with nothing to go on" about a press that never happened.
	Verdict workflowrun.Verdict
	Decided bool
	// Because is the sentence behind the verdict.
	Because string
	// Refused is set when the interaction did not complete, with the reason.
	Refused string
	// Cancelled says this sentence ended because the run was stopped.
	Cancelled bool
}

// Verified reports an interaction that was performed and shown to have worked.
func (a Ask) Verified() bool {
	return a.Decided && a.Verdict == workflowrun.VerdictVerified
}

// Account is what one run of one play asked for, in order, and how it ended.
type Account struct {
	mu        sync.Mutex
	reference string
	// want is how many interactions the play has. Known because the play is GENERATED from
	// the bindings, so its sentences and the plan's interactions are the same set.
	want  int
	asked []Ask
	// cut is why the run ended without reaching every sentence, when something other than
	// a sentence ended it, and cancelled says that ending was a cancellation. See Cut.
	cut       string
	cancelled bool
}

// NewAccount starts the record for one run of a play with this many interactions.
func NewAccount(reference string, want int) *Account {
	return &Account{reference: reference, want: want}
}

// Note records one sentence's outcome.
func (a *Account) Note(k Ask) {
	a.mu.Lock()
	defer a.mu.Unlock()
	k.Run = a.reference
	a.asked = append(a.asked, k)
}

// Cut records that the RUN ended from outside, which no sentence can say.
//
// # The defect this closes
//
// Cancelling the context aborts the whole frame tree — `runtime.RunWithHostsContext` says so:
// in-flight host calls stop and `finally` blocks run. So a run cancelled just as its first
// interaction finished does not reach the second sentence AT ALL: the host is never called, there
// is no second Ask, and every Ask there is says verified.
//
// `Outcome` then reported `verified` about a two-interaction routine that performed one. It was
// found by a test that failed one run in six, and the intermittence was the two orderings rather
// than noise: sometimes the second sentence ran and was refused, sometimes the runtime got there
// first. Half a routine reported as a finished one is the worst sentence this product could show
// somebody, so the shortfall is part of the account.
func (a *Account) Cut(why string, cancelled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cut == "" {
		a.cut, a.cancelled = why, cancelled
	}
}

// Asked is what the play asked for, in order.
func (a *Account) Asked() []Ask {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Ask(nil), a.asked...)
}

// Reference is the run's canonical identity.
func (a *Account) Reference() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reference
}

// Outcome is how the whole run ended, in the vocabulary both binaries already share.
func (a *Account) Outcome() runref.Outcome {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.asked) == 0 {
		if a.cancelled {
			return runref.Cancelled
		}
		return runref.Refused
	}
	for _, k := range a.asked {
		// THE ORDER OF THESE IS THE WHOLE FUNCTION.
		//
		// A stop outranks everything: somebody chose it. A VERDICT outranks a refusal,
		// because the coordinator refuses to carry on BECAUSE of the verdict — reading
		// the refusal first reported every contradicted press as "refused", which says
		// Marco pressed nothing about a run that pressed. A refusal with no verdict is
		// the honest last case: nothing was attempted.
		if k.Cancelled {
			return runref.Cancelled
		}
		if k.Decided {
			switch k.Verdict {
			case workflowrun.VerdictVerified:
				continue
			case workflowrun.VerdictContradicted:
				return runref.Contradicted
			default:
				return runref.Unconfirmed
			}
		}
		if k.Refused != "" {
			return runref.Refused
		}
		// Asked for, nothing decided, nothing refused: the looks ran out before
		// anybody got to say. An earlier draft fell through to `verified` here, which
		// reported a run that pressed nothing as a run that worked.
		return runref.Unconfirmed
	}
	// EVERY SENTENCE IT REACHED WAS VERIFIED — but did it reach them all? See Cut.
	if a.want > 0 && len(a.asked) < a.want {
		if a.cancelled {
			return runref.Cancelled
		}
		return runref.Unconfirmed
	}
	return runref.Verified
}

// Short is one line about how the run ended, for a report or a status.
func (a *Account) Short() string {
	out := a.Outcome()
	done := 0
	for _, k := range a.Asked() {
		if k.Verified() {
			done++
		}
	}
	a.mu.Lock()
	want, cut := a.want, a.cut
	a.mu.Unlock()
	s := fmt.Sprintf("%s — %d of %d interaction(s) verified", out, done, want)
	if cut != "" {
		s += " (" + cut + ")"
	}
	return s
}

// Sentence is a verdict as something a person could act on.
func Sentence(v workflowrun.Verdict, because string) string {
	lead := string(v)
	switch v {
	case workflowrun.VerdictContradicted:
		lead = "the screen said something else after the press"
	case workflowrun.VerdictUnconfirmed:
		lead = "there was not enough evidence to say whether it worked"
	case workflowrun.VerdictUnasked:
		lead = "it could not be checked"
	}
	if because == "" {
		return lead
	}
	return lead + " — " + because
}

// VerdictOf is a verification's verdict, or the unasked zero when none was reached.
//
// Read beside `Ask.Decided`, never alone: the two of them say "no verdict" and "a verdict that
// happens to be unasked" apart, and they are different runs.
func VerdictOf(v *workflowrun.Verification) workflowrun.Verdict {
	if v == nil {
		return workflowrun.VerdictUnasked
	}
	return v.Verdict
}

// ErrExpired is a run that ran out of the time it was given, which is not a cancellation.
//
// Both end every wait — the deadline lives in the context, so a prompt already open closes with
// it — and they mean opposite things afterwards: somebody chose to stop this, against Marco was
// given N seconds and used them. A report that called expiry a cancellation would say the first
// about the second.
var ErrExpired = errors.New("the run ran out of the time it was given")

// WhyItEnded is which of the two a context error is, as an error a caller can carry.
//
// Nil for a context that has not ended.
func WhyItEnded(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %v", ErrExpired, err)
	default:
		return fmt.Errorf("%w: %v", ErrCancelled, err)
	}
}
