package demo

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What may never be learned.
//
//	Never learn from failed executions, cancelled programs, unsafe actions,
//	destructive bulk operations, sensitive value demonstrations, authentication flows,
//	credential entry or payment flows.
//
// The refusal is checked when the session CLOSES rather than when a procedure is asked
// for, and the reason is stored on the demonstration. Two consequences, both wanted: a
// refused demonstration cannot be talked into becoming a procedure by asking again with
// different rules, and the user is told at the moment they finish rather than later when
// they have forgotten what was in it.
//
// Every clause below refuses a demonstration OUTRIGHT rather than dropping the offending
// step. A procedure learned from a demonstration with the credential entry removed is a
// procedure that logs in without logging in — it would run, do half a task, and leave the
// user looking at an authentication prompt it has no account of.

// RefusalMessage is what the user is told, verbatim, when a demonstration cannot become a
// procedure.
//
// One sentence, fixed, and the same every time. A refusal that varied its wording by
// cause would invite a reader to work out which cause they had hit and try to avoid it,
// and none of these are things to route around.
const RefusalMessage = "This demonstration cannot safely become a reusable procedure."

// Unsafe reports whether a demonstration may never become a procedure, and why.
//
// The reason is for the user and is deliberately specific — the fixed RefusalMessage plus
// the clause that fired — because "this cannot be learned" with no account of why is
// indistinguishable from a bug.
func Unsafe(d *Demonstration) (string, bool) {
	if d == nil {
		return RefusalMessage + " There is no demonstration.", true
	}
	for _, check := range safetyChecks {
		if why, hit := check(d); hit {
			return RefusalMessage + " " + why, true
		}
	}
	return "", false
}

// safetyCheck is one refusal clause.
type safetyCheck func(*Demonstration) (string, bool)

// safetyChecks are the clauses, in the order they are reported.
//
// Ordered from the most specific and most serious to the most general, so a demonstration
// that trips several is refused for the one the user most needs to hear.
var safetyChecks = []safetyCheck{
	sensitiveValues,
	authenticationFlow,
	paymentFlow,
	destructiveBulk,
	unsafeAction,
	failedExecution,
	cancelledProgram,
}

// sensitiveValues refuses a demonstration that handled a credential or private value.
func sensitiveValues(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if s.Sensitive {
			return fmt.Sprintf(
				"Step %d entered a value into %s, which the Director treats as private. "+
					"Its content was never recorded, and a procedure that replayed it would "+
					"have to be.", s.Index, s.Target.Describe()), true
		}
	}
	return "", false
}

// authWords are the labels and phrases that mean an authentication flow.
//
// A closed list, matched against control labels and step phrases. Deliberately broad: a
// demonstration wrongly refused as a login costs one re-run, and a login wrongly learned
// becomes a procedure that types a password into whatever holds focus.
var authWords = []string{
	"sign in", "signin", "sign-in", "log in", "login", "log-in", "logon", "log on",
	"authenticate", "authentication", "two-factor", "two factor", "multi-factor",
	"verify your identity", "unlock", "credentials",
}

func authenticationFlow(d *Demonstration) (string, bool) {
	if word, where, hit := anyWord(d, authWords); hit {
		return fmt.Sprintf(
			"It looks like an authentication flow (%s mentions %q). Signing in is not a "+
				"procedure to replay: it involves credentials the Director must never hold.",
			where, word), true
	}
	return "", false
}

// paymentWords are the labels and phrases that mean money is moving.
var paymentWords = []string{
	"payment", "pay now", "checkout", "check out", "billing", "credit card",
	"debit card", "card details", "purchase", "place order", "buy now", "subscribe",
	"transfer funds", "send money", "confirm payment",
}

func paymentFlow(d *Demonstration) (string, bool) {
	if word, where, hit := anyWord(d, paymentWords); hit {
		return fmt.Sprintf(
			"It looks like a payment flow (%s mentions %q). A procedure that spends money "+
				"is not something to install from one demonstration.", where, word), true
	}
	return "", false
}

// destructiveBulk refuses a demonstration that deleted, discarded or overwrote — and
// refuses ANY bulk operation, destructive or not.
//
// Two clauses in one because they share a cause. A destructive action learned from one
// demonstration is a procedure that deletes on request with no further thought; a bulk
// operation learned from one demonstration is that, multiplied by however many members the
// set happens to have next time.
//
// Destructiveness is read from the CONTROL ROLE rather than from the verb's reversibility.
// Those are different questions, and using the wrong one refuses everything: the semantic
// vocabulary calls invoke, paste and submit irreversible because an ordinary undo cannot be
// relied on to reverse them, which is a correct thing to say about a click on an unknown
// button and a useless test for "did this destroy something". ControlRole.Destructive is
// the vocabulary's own declaration of the controls that lose work when chosen wrongly, and
// that is the question being asked here.
func destructiveBulk(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if s.Target.Role.Destructive() {
			return fmt.Sprintf(
				"Step %d used %s. A procedure that removes or discards is not learned from "+
					"a demonstration.", s.Index, s.Target.Role.Describe()), true
		}
	}
	for _, n := range d.Notes {
		if strings.Contains(n, "collection of") {
			return "It iterated a bounded set. What that set contains next time is not what " +
				"was demonstrated, so a procedure learned from it would act on things " +
				"nobody has seen.", true
		}
	}
	return "", false
}

// unsafeAction refuses a demonstration containing an action that needed confirmation.
//
// Not because confirmation went wrong — it may well have been granted — but because a
// procedure carrying that step would put the same question to the user every time it ran,
// from a procedure they approved once. Consent to an action is not consent to a machine
// that performs it on demand.
//
// Read from what the confirmation gate RECORDED, not from the words in a note. The gate
// publishes its outcome on every request that passed one, and searching prose for the word
// "confirm" would refuse a rename whose last step is called "confirm the new name".
func unsafeAction(d *Demonstration) (string, bool) {
	if len(d.Confirmed) == 0 {
		return "", false
	}
	return fmt.Sprintf(
		"It contains an action the Director had to ask about before performing (%s). "+
			"Agreeing to it once is not agreeing to a procedure that does it whenever it "+
			"is called.", strings.Join(d.Confirmed, "; ")), true
}

// failedExecution refuses a demonstration in which anything did not verify.
//
//	Recording never bypasses verification.
//
// A step that could not be confirmed is a step nobody knows the effect of, and a procedure
// is the claim that these steps produce this outcome.
func failedExecution(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if !s.Verified {
			return fmt.Sprintf(
				"Step %d (%s) did not verify — it ended %s. A procedure is a claim that "+
					"these steps produce this outcome, and an unverified step is not part "+
					"of one.", s.Index, s.Describe(), s.Status), true
		}
	}
	for _, n := range d.Notes {
		if strings.Contains(n, "did not produce a verified action") {
			return "Something in it did not produce a verified action: " + n, true
		}
	}
	return "", false
}

// cancelledProgram refuses a demonstration whose program stopped part-way.
func cancelledProgram(d *Demonstration) (string, bool) {
	for _, n := range d.Notes {
		if strings.Contains(n, "ended "+string(directorapi.ResultCancelled)) ||
			strings.Contains(n, "ended "+string(directorapi.ResultFailed)) ||
			strings.Contains(n, "ended "+string(directorapi.ResultBlocked)) {
			return "A program in it did not finish: " + n, true
		}
		if strings.Contains(n, "needed the user to say which control was meant") {
			return "A step needed the user to say which control was meant. What a learned " +
				"procedure would aim at next time is exactly the question that had to be " +
				"asked, so there is no answer to record.", true
		}
	}
	return "", false
}

// anyWord searches the demonstration's own text for one of a list of words.
//
// Over the CONTROL LABELS and the step phrases — the parts that describe the interface the
// user was looking at. It deliberately does not search values, because values are not
// stored.
func anyWord(d *Demonstration, words []string) (word, where string, hit bool) {
	fields := []struct{ where, text string }{}
	for _, s := range d.Steps {
		fields = append(fields,
			struct{ where, text string }{fmt.Sprintf("step %d", s.Index), s.Target.Label},
			struct{ where, text string }{fmt.Sprintf("step %d", s.Index), s.Target.Phrase},
			struct{ where, text string }{fmt.Sprintf("step %d", s.Index), s.Phrase})
	}
	for _, r := range d.Requests {
		fields = append(fields, struct{ where, text string }{"the request", r})
	}
	for _, f := range fields {
		lower := strings.ToLower(f.text)
		if lower == "" {
			continue
		}
		for _, w := range words {
			if strings.Contains(lower, w) {
				return w, f.where, true
			}
		}
	}
	return "", "", false
}
