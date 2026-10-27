package oshost

// TYPED TEXT IS A SEQUENCE OF SUBMISSIONS AND AN ACCOUNT OF HOW FAR IT GOT.
//
// `do OS's Type with "hello"` reads like one thing and is five, and the ones after the first can
// be refused on their own. What that leaves behind is not "the typing failed": it is a text box
// with `he` in it, on somebody's screen, and a routine whose next step was written for a field
// that says `hello`.
//
// Three things were wrong with that, and each is a different kind of wrong:
//
//   - NOTHING WAS ACCOUNTED. `Invoke` dispatched typing through `ok(…)` rather than
//     `delivered(…)`, so a rejected send became a failed step recorded in no account at all.
//     `Unperformed` was empty, and `marco do` reported success about typing that did not happen.
//   - NOTHING WAS PROPAGATED. `StepType` was in `emit` and in no arm of `propagating`, so typed
//     text was never a link: whatever followed it ran whether the text arrived or not. The
//     keystroke's defect, one step along, and worse — a half-filled field changes what the press
//     after it means, and the window guard cannot see the difference because the window is the
//     same window.
//   - PARTIAL DELIVERY WAS UNREPRESENTABLE. The loop stopped on the first refusal or on the
//     stop, and returned an error that said neither how much had gone in nor that the question
//     had an answer. A stop pressed mid-word is the ordinary case of this, not the exotic one.
//
// # What is counted, and what counting it does NOT claim
//
// SUBMISSIONS, and the word is chosen. `SendInput` returning a count says the system TOOK that
// many events into the input stream; it does not say an application received them, that the
// application with the keyboard is the one the routine meant, or that what arrived was rendered.
// So the account says `submitted`, and the only account that can say `arrived` is the
// application's own — which is what the `text/` desktop scenarios read, and why they exist.
//
// THE UNIT IS A UNICODE CODE POINT, never a Go byte offset and never a UTF-16 code unit. `"é"` is
// two bytes and one character; an emoji is four bytes, one character and TWO UTF-16 units. A
// count in bytes would be a number that means nothing to the person reading it, and a count in
// code units would say `2 of 1` for a single emoji.
//
// A code point goes in as a down and an up for each of its code units, ALL IN ONE CALL — so
// nothing interleaves between the halves of one character, which is the same reason `clickAt`
// positions and presses in one batch. What the machine takes is counted per event, so a code
// point is in one of three states and all three are different: every event taken (SUBMITTED),
// no event taken (REFUSED, and nothing of that character went in), or some of them (PARTIAL —
// what the application does with half a surrogate pair is its business, and this cannot say).
//
// # And a secret is accounted in a register of its own
//
// Not by redacting afterwards. A secret's progress is never MEASURED: the gesture is told which
// kind of text it is carrying, so no number about it is ever computed, let alone written into an
// error, a log line, a durable record or a card. Length and failure position are both facts about
// a password — a 7-character failure position is a 7-character prefix somebody knows — so the
// sentence for a secret says that it did not all go in and says nothing else.
//
// # What this deliberately does not do
//
// It does not undo what went in. There is no key for "un-type" and the text is in an application
// that has already acted on it; a routine that tried to clear the field would be a routine typing
// more input into a situation it cannot read. The incompleteness is REPORTED and not recovered,
// which is the chord's rule for a key it could not release.
//
// And it does not report a stuck key. A release the machine refuses after an accepted down is an
// attempt that failed, recorded as uncertainty about that one character — not as something still
// held down on the computer. A `KEYEVENTF_UNICODE` event is a character submission and not a
// modifier: nothing reads it as held, nothing arrives modified because of it, and borrowing the
// account that exists for a held Ctrl would tell somebody their keyboard is in a state it is not.
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// textMachine is the machine, one code point at a time.
//
// Two methods and no state, the shape `keyMachine` has and for the same reason: the sequence is
// what had the defects in it, and it could not be reached while it sat inside the function that
// calls `SendInput`. Everything a test needs to arrange — a refusal before the first character, a
// refusal in the middle of one, a release the machine will not take — is a return value here.
type textMachine interface {
	// submit sends every event of one code point in ONE call — a down and an up for each
	// of its UTF-16 code units — and says how many of those events the machine TOOK.
	//
	// The count rather than a bool, because the difference between 0 and "some" is the
	// difference between a character that did not go in and a character that went in
	// part way, and those are two different things to tell somebody.
	submit(units []uint16) int
	// release attempts the up for a unit whose down the machine took and whose up it
	// did not, and says whether it took that.
	release(unit uint16) bool
	// pace waits between two characters, and returns early when the run is stopped.
	//
	// ON THE MACHINE AND NOT IN THE SEQUENCE, which is `keyMachine.settle`'s shape and
	// its reason: the cadence is a property of the real keyboard — `$MARCO_TYPE_CADENCE_MS`,
	// there so a fast application does not drop characters — and the EARLY RETURN is
	// what a stop looks like from in here. A sequence that slept would be a sequence a
	// test has to wait for, and the one case that matters is a stop arriving during the
	// wait.
	pace(ctx context.Context)
}

// textKind is whether what is being typed may be described in a diagnostic.
//
// A PARAMETER AND NOT A LATER REDACTION. A number computed about a secret is a number that can be
// logged by mistake, so none is ever computed: the kind travels with the text, and the sentence
// for a secret is built without ever counting anything.
type textKind int

const (
	// asLiteral is text that is in the route and on the screen already, so a count of how
	// much of it went in tells nobody anything they could not read.
	asLiteral textKind = iota
	// asSecret is a value from the credential store, which is in no route and on no
	// screen. Its length and the position it stopped at are both facts about it.
	asSecret
)

// typingProgress is how far a typing got.
//
// # Two registers, and a secret is never in the one with numbers in it
//
// `finished` and `partial` are STATE: every character went in, and one character is in doubt.
// Both are answerable about any text without measuring it. `submitted` and `total` are code
// points, and they are accumulated ONLY for a literal — `counted` says whether they mean
// anything.
//
// That is the difference between suppression and redaction, and it is the whole point: a number
// that was computed and then left out of a sentence is a number that exists, in a value that gets
// passed around, logged by whatever logs values, and printed by the next person to add a field to
// a report. For a secret, the length and the position it stopped at are facts about the value —
// a failure position is a prefix whose size somebody now knows — so they are never derived.
type typingProgress struct {
	// counted says `submitted` and `total` mean anything. False for a secret.
	counted   bool
	submitted int
	total     int
	// partial is one code point whose events the machine took some of — at most one,
	// because the sequence stops there.
	partial bool
	// finished says the sequence reached the end of the text without a refusal or a stop.
	finished bool
}

// whole reports whether every character was submitted and none was left in doubt.
//
// ASKED OF THE STATE AND NOT OF THE NUMBERS, so it answers the same way for a secret, where
// there are none. It used to compare `submitted` with `total`, which is why they were being
// accumulated for a secret at all.
func (p typingProgress) whole() bool {
	return p.finished && !p.partial
}

// typeOut submits a text one code point at a time, and says how far it got.
//
// The error is the machine's refusal or the run's cancellation; the progress is returned
// ALONGSIDE it rather than inside it, because the caller needs both for every outcome. A refused
// typing and a stopped typing leave the same thing behind — a partly filled field — and differ
// only in what the run is called afterwards.
func typeOut(ctx context.Context, m textMachine, text string, kind textKind) (typingProgress,
	error) {

	p := typingProgress{counted: kind == asLiteral}
	if p.counted {
		p.total = utf8.RuneCountInString(text)
	}
	first := true
	for _, r := range text {
		// THE CADENCE, BETWEEN CHARACTERS AND NOT AFTER THE LAST ONE.
		//
		// `$MARCO_TYPE_CADENCE_MS` paces the keystrokes so a fast application or a
		// game does not drop them, and it was in the loop this replaced. The pace
		// goes BEFORE each character except the first — rather than after each
		// except the last — because then the condition is about this character and
		// not about how many are left, which is what the old byte-indexed version
		// got subtly wrong for multi-byte text.
		if !first {
			m.pace(ctx)
		}
		first = false
		// NOTHING MORE IS SENT TO A RUN THAT HAS STOPPED, asked before each character
		// rather than once at the top — and AFTER the wait above rather than before
		// it, which is what makes one check answer two questions. A stop is somebody
		// pressing Escape while the text is going in, and it can arrive while the
		// previous character was being submitted or while this one was being waited
		// for; `pace` returns early when the run is cancelled, so both arrive here.
		// The point of checking is that the next character does not go, and the point
		// of returning the progress is that what already went is not forgotten.
		//
		// A SECOND CHECK INSIDE THE BRANCH ABOVE WAS WRITTEN FIRST AND DELETED: the
		// mutation gate removed it and nothing failed, because this one had already
		// answered. Two checks where one is load-bearing is a line nobody can test.
		if err := ctx.Err(); err != nil {
			return p, err
		}
		units := utf16.AppendRune(nil, r)
		events := 2 * len(units)
		took := m.submit(units)
		switch {
		case took >= events:
			if p.counted {
				p.submitted++
			}
		case took <= 0:
			// NOTHING OF THIS CHARACTER WENT IN, so there is nothing in doubt
			// about it and nothing to release.
			return p, notAllSubmitted(p, kind, true)
		default:
			// PART OF ONE CHARACTER WENT IN. An odd count means the last event
			// the machine took was a DOWN, so that unit is owed an up — and the
			// attempt is made whether or not it succeeds, for the chord's
			// reason: the alternative is leaving it in order to report the
			// first thing that went wrong.
			p.partial = true
			if took%2 == 1 {
				_ = m.release(units[(took-1)/2])
			}
			return p, notAllSubmitted(p, kind, true)
		}
	}
	p.finished = true
	return p, ctx.Err()
}

// notAllSubmitted is the one place the sentence for incomplete typing is built, in the register
// the text allows.
//
// TWO CALLERS AND ONE SENTENCE. The gesture builds it for a refusal, which it returns; the host
// builds it for a STOP, which it does not — a cancellation keeps its own identity on the way out,
// because a run that stopped looking like a run that failed would turn somebody pressing Escape
// into a defect. Both leave the same thing on the screen and both have to say so.
func notAllSubmitted(p typingProgress, kind textKind, refused bool) error {
	return partlyTyped{progress: p, kind: kind, refused: refused}
}

// errPartlyTyped is the identity, for recognising one of these rather than reading it.
//
// Unexported for `errKeyNotReleased`'s reason: nothing outside needs to ask. The host answers the
// question for its callers by keeping the sentence in an account a cancelled run cannot discard.
var errPartlyTyped = errors.New("text that was not all submitted")

// partlyTyped is text the machine did not take all of, as both a sentence and an identity.
//
// It is deliberately NOT a `stillHolding`. That interface is what the host's held-key account
// asks for, and text that is merely incomplete is not something the computer has not finished
// doing: nothing is down, nothing arrives modified, and a person told their keyboard may be stuck
// would be told something untrue. See the note at the top of this file.
type partlyTyped struct {
	progress typingProgress
	kind     textKind
	// refused separates the machine saying no from the run being stopped. Both leave a
	// partly filled field; only one of them is something going wrong.
	refused bool
}

// Error says what is on the screen, in the unit a person reads.
//
// A SECRET SAYS NO NUMBER. Not the length, not how far it got — both are facts about the value,
// and a failure position is a prefix somebody now knows the size of.
func (t partlyTyped) Error() string {
	what := "the machine would not take"
	if !t.refused {
		what = "the run was stopped before"
	}
	if !t.progress.counted {
		return fmt.Sprintf("%s all of this secret, so part of it may have been "+
			"submitted and the field it was going into is incomplete", what)
	}
	return fmt.Sprintf("%s all of this text: %s, so the field it was going into is "+
		"incomplete", what, t.progress.said())
}

// said is the progress in words, for a literal.
//
// SUBMITTED AND NOT ARRIVED, which is the whole of what this boundary can honestly claim: the
// system took the events. Whether an application received them is a question only the
// application can answer.
func (p typingProgress) said() string {
	out := fmt.Sprintf("%d of %d characters were submitted", p.submitted, p.total)
	if p.partial {
		out += ", and one more went in part way"
	}
	return out
}

func (partlyTyped) Is(target error) bool { return target == errPartlyTyped }

// PartlyTypedText and PartlyTypedSecret are the sentences for text a run did not finish
// submitting, and the one place they are written.
//
// Exported for `KeyNotReleased`'s reason: the words travel. The host keeps them in an account of
// its own (`Host.PartlyTyped`), `marco` puts them in the durable record of a run it is otherwise
// discarding the error from, and Studio shows them to somebody. A test at any of those layers
// asks for the sentence from here rather than retyping it — two copies of a sentence agree until
// one of them is improved.
//
// `refused` separates the machine saying no from the run being stopped, because they are two
// different things to tell somebody about the same half-filled field.
func PartlyTypedText(submitted, total int, refused bool) error {
	return notAllSubmitted(typingProgress{counted: true, submitted: submitted,
		total: total}, asLiteral, refused)
}

// PartlyTypedSecret says a secret did not all go in, and says nothing else: no length, and no
// position, because a failure position is a prefix whose size somebody now knows.
func PartlyTypedSecret(refused bool) error {
	return notAllSubmitted(typingProgress{}, asSecret, refused)
}
