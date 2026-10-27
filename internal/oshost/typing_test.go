package oshost

// WHAT A TYPING SUBMITTED, AND WHAT IT LEFT BEHIND.
//
// The sequence is what had the defects in it, so the sequence is what these drive: a machine that
// writes down every code point it was asked for and takes as many of each character's events as
// the case says. Every one of these cases is a real machine's answer — `SendInput` returns a count
// and that count can be anything from zero to all of it — and not one of them could be posed while
// the loop sat beside the call.
//
// # The claims, and why each needs a control
//
// Every stopping case here has a successful one beside it, because the shape of this code is
// "stop and report": a gesture that reported a refusal for everything would satisfy each failure
// assertion on its own, and the control is what separates "it noticed" from "it always says so".
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/secrets"
)

// aTypist is the machine for these: it remembers what it was asked to submit, takes what the
// case tells it to, and can be told to refuse a release.
type aTypist struct {
	// takes is how many EVENTS to take for the nth code point; a negative number takes
	// all of them, and the last entry repeats for the rest of the text.
	takes []int
	// submitted is the code units of every code point it was asked for, in order,
	// whether or not it took them.
	submitted [][]uint16
	// released is every unit it was asked to release afterwards.
	released []uint16
	// refuseRelease makes the machine decline that attempt, which must not be reported
	// as a key left down.
	refuseRelease bool
	// stopAfter cancels the run once this many code points have been asked for, which is
	// how a stop arrives in the middle of a text.
	stopAfter int
	stop      func()
	n         int
	// paced counts the waits between characters, and stopInThePace makes the stop
	// arrive DURING one — which is the case the early return exists for and the one
	// the chord slice found by hand.
	paced         int
	stopInThePace bool
}

// pace is the wait between two characters. It cancels the run from inside the wait when the
// case asks for that, which is what a stop arriving mid-text looks like from in here.
func (t *aTypist) pace(context.Context) {
	t.paced++
	if t.stopInThePace && t.stop != nil {
		t.stop()
	}
}

func (t *aTypist) submit(units []uint16) int {
	t.submitted = append(t.submitted, units)
	t.n++
	if t.stop != nil && t.n == t.stopAfter {
		t.stop()
	}
	take := -1
	if len(t.takes) > 0 {
		i := t.n - 1
		if i >= len(t.takes) {
			i = len(t.takes) - 1
		}
		take = t.takes[i]
	}
	if take < 0 {
		return 2 * len(units)
	}
	return take
}

func (t *aTypist) release(unit uint16) bool {
	t.released = append(t.released, unit)
	return !t.refuseRelease
}

// asked is the code points this machine was handed, as text, so a failure message reads.
func (t *aTypist) asked() string {
	var b strings.Builder
	for _, units := range t.submitted {
		b.WriteString(string(utf16.Decode(units)))
	}
	return b.String()
}

// theEmoji is a code point above U+FFFF: two UTF-16 code units, four events, and the character
// the old code could not type at all.
const theEmoji = "\U0001F600"

// EVERY CHARACTER IS SUBMITTED AS ITS UTF-16 CODE UNITS, AND A NON-BMP ONE AS TWO.
//
// The defect this replaces: `sendUnicodeChar` cast the rune to `uint16`, so U+1F600 was sent as
// U+F600 — a private-use character — and `SendInput` took it, so the send succeeded and the
// routine typed something else. Silently, every time, with no account anywhere that could show it.
func TestACharacterIsSubmittedAsItsCodeUnits(t *testing.T) {
	m := &aTypist{}
	p, err := typeOut(context.Background(), m, "hé"+theEmoji, asLiteral)

	if err != nil {
		t.Fatalf("an ordinary text was refused: %v", err)
	}
	if p.submitted != 3 || p.total != 3 || p.partial {
		t.Fatalf("the progress is %+v, want 3 of 3 with nothing in doubt", p)
	}
	want := [][]uint16{{'h'}, {0xE9}, {0xD83D, 0xDE00}}
	if len(m.submitted) != len(want) {
		t.Fatalf("the machine was handed %d code point(s): %q", len(m.submitted),
			m.asked())
	}
	for i, units := range want {
		if len(m.submitted[i]) != len(units) {
			t.Errorf("code point %d went in as %d unit(s), want %d", i+1,
				len(m.submitted[i]), len(units))
			continue
		}
		for j, u := range units {
			if m.submitted[i][j] != u {
				t.Errorf("code point %d unit %d is %#04x, want %#04x",
					i+1, j+1, m.submitted[i][j], u)
			}
		}
	}
	// AND THE EMOJI IS NOT TRUNCATED. Named, because the assertion above would pass on
	// a gesture that sent the low surrogate alone — and 0xF600 is what the cast
	// produced, so a test that only counted units would have passed the old code too.
	if len(m.submitted) == 3 && len(m.submitted[2]) == 1 {
		t.Error("the non-BMP character went in as one code unit, which is the " +
			"truncation that typed a private-use character instead")
	}
}

// AND THE PROGRESS IS IN CODE POINTS, NEVER BYTES AND NEVER CODE UNITS.
//
// `"héllo"` is six bytes and five characters; two emoji are eight bytes, four code units and two
// characters. A count in either of the other units is a number that means nothing to the person
// reading it — "4 of 8 characters" about two emoji is not a smaller claim, it is a wrong one.
func TestTheProgressIsCountedInCodePoints(t *testing.T) {
	for _, c := range []struct {
		text string
		want int
	}{
		{"héllo", 5},
		{theEmoji + theEmoji, 2},
		{"", 0},
	} {
		p, err := typeOut(context.Background(), &aTypist{}, c.text, asLiteral)
		if err != nil {
			t.Fatalf("%q was refused: %v", c.text, err)
		}
		if p.total != c.want || p.submitted != c.want {
			t.Errorf("%q is counted as %d of %d characters, want %d", c.text,
				p.submitted, p.total, c.want)
		}
	}
}

// A MACHINE THAT TAKES NOTHING SUBMITS NOTHING, AND SAYS SO.
func TestATextRefusedBeforeItStartedSubmitsNothing(t *testing.T) {
	m := &aTypist{takes: []int{0}}
	p, err := typeOut(context.Background(), m, "hello", asLiteral)

	if err == nil {
		t.Fatal("a machine that took no event at all reported a typed text")
	}
	if !errors.Is(err, errPartlyTyped) {
		t.Errorf("the error is %v, which is not the one that carries the progress", err)
	}
	if p.submitted != 0 || p.total != 5 || p.partial {
		t.Fatalf("the progress is %+v, want 0 of 5 with nothing in doubt", p)
	}
	if n := len(m.submitted); n != 1 {
		t.Errorf("the machine was asked for %d characters after refusing the first", n)
	}
	if !strings.Contains(err.Error(), "0 of 5") {
		t.Errorf("the sentence is %q and does not say how much went in", err)
	}
}

// AND A REFUSAL MIDWAY KEEPS WHAT WENT IN.
//
// The case that matters most and the one nothing could say before: the field has `he` in it. A
// report of "the typing failed" describes this as though the application were untouched, and the
// step after it was written for a field that says `hello`.
func TestARefusalMidwayKeepsWhatWentIn(t *testing.T) {
	m := &aTypist{takes: []int{-1, -1, 0}}
	p, err := typeOut(context.Background(), m, "hello", asLiteral)

	if err == nil {
		t.Fatal("a refusal after two characters reported a typed text")
	}
	if p.submitted != 2 || p.total != 5 || p.partial {
		t.Fatalf("the progress is %+v, want 2 of 5 with nothing in doubt", p)
	}
	if !strings.Contains(err.Error(), "2 of 5") {
		t.Errorf("the sentence is %q and does not say how much went in", err)
	}
	// AND NOTHING AFTER IT WAS ASKED FOR. The stop is the point: a loop that carried on
	// would be submitting the rest of a value into a field that is already wrong.
	if n := len(m.submitted); n != 3 {
		t.Errorf("the machine was asked for %d characters, want the three it got "+
			"to: %q", n, m.asked())
	}
	// THE CONTROL: the same text, taken, is not reported as partial.
	if p, err := typeOut(context.Background(), &aTypist{}, "hello", asLiteral); err != nil ||
		!p.whole() {
		t.Errorf("a text the machine took is reported as %+v (%v)", p, err)
	}
}

// AND A CHARACTER TAKEN IN PART IS REPORTED AS UNCERTAIN, WITH ITS RELEASE ATTEMPTED.
//
// `SendInput` inserts events until it stops, so "half a character" is a real answer: the down of
// `l` went in and its up did not. What the application did with that is its own business — which
// is precisely why this is reported as uncertainty rather than counted either way.
func TestACharacterTakenInPartIsUncertainAndItsReleaseAttempted(t *testing.T) {
	m := &aTypist{takes: []int{-1, -1, 1}} // the third character: down taken, up not
	p, err := typeOut(context.Background(), m, "hello", asLiteral)

	if err == nil {
		t.Fatal("a character taken in part reported a typed text")
	}
	if p.submitted != 2 || !p.partial {
		t.Fatalf("the progress is %+v, want 2 submitted and one in doubt", p)
	}
	if !strings.Contains(err.Error(), "one more went in part way") {
		t.Errorf("the sentence is %q and does not say a character is in doubt", err)
	}
	// THE RELEASE IS ATTEMPTED FOR THE UNIT WHOSE DOWN WENT IN, which is the half of
	// this that is about the computer rather than about the report.
	if len(m.released) != 1 || m.released[0] != 'l' {
		t.Errorf("the machine was asked to release %v, want the unit whose down it "+
			"took", m.released)
	}
}

// AND A SURROGATE PAIR TAKEN IN PART RELEASES THE RIGHT HALF.
//
// Four events for one character, so there are three ways to take part of it. An ODD count ends on
// a down and owes an up; an EVEN one does not, and attempting a release there would be sending an
// event for a key that is not down — the mistake the chord slice wrote down for a refused press.
func TestAPartlyTakenSurrogatePairReleasesOnlyWhatIsOwed(t *testing.T) {
	for _, c := range []struct {
		what  string
		takes int
		want  []uint16
	}{
		{"the high surrogate's down alone", 1, []uint16{0xD83D}},
		{"both of the high surrogate's events", 2, nil},
		{"and the low surrogate's down too", 3, []uint16{0xDE00}},
	} {
		t.Run(c.what, func(t *testing.T) {
			m := &aTypist{takes: []int{c.takes}}
			p, err := typeOut(context.Background(), m, theEmoji, asLiteral)

			if err == nil {
				t.Fatal("part of a character reported a typed text")
			}
			if p.submitted != 0 || !p.partial {
				t.Fatalf("the progress is %+v, want nothing submitted and "+
					"one character in doubt", p)
			}
			if len(m.released) != len(c.want) {
				t.Fatalf("the machine was asked to release %v, want %v",
					m.released, c.want)
			}
			for i, u := range c.want {
				if m.released[i] != u {
					t.Errorf("it released %#04x, want %#04x",
						m.released[i], u)
				}
			}
		})
	}
}

// AND A RELEASE THE MACHINE REFUSES IS NOT A KEY LEFT DOWN.
//
// The account for a held key says *every keystroke anybody types now arrives modified*, and that
// is not true of this: a `KEYEVENTF_UNICODE` event is a character submission, nothing reads it as
// held, and borrowing that account would tell somebody their keyboard is in a state it is not.
// What they are owed is the uncertainty about one character, which is what this reports.
func TestARefusedReleaseIsNotReportedAsAKeyLeftDown(t *testing.T) {
	m := &aTypist{takes: []int{1}, refuseRelease: true}
	_, err := typeOut(context.Background(), m, "hello", asLiteral)

	if err == nil {
		t.Fatal("a character taken in part reported a typed text")
	}
	var stuck stillHolding
	if errors.As(err, &stuck) {
		t.Errorf("incomplete text is reported as something still held down: %v", err)
	}
	if len(m.released) != 1 {
		t.Error("the release was not even attempted")
	}
}

// A STOP BEFORE THE FIRST CHARACTER TYPES NOTHING AT ALL.
func TestAStopBeforeTypingSubmitsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &aTypist{}
	p, err := typeOut(ctx, m, "hello", asLiteral)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stopped run answered %v, and the cancellation has to keep its "+
			"identity or a person pressing Escape becomes a failed routine", err)
	}
	if len(m.submitted) != 0 {
		t.Errorf("a stopped run submitted %q", m.asked())
	}
	if p.submitted != 0 || p.total != 5 {
		t.Errorf("the progress is %+v, want 0 of 5", p)
	}
}

// AND A STOP DURING TYPING KEEPS WHAT WENT IN, AS A CANCELLATION.
//
// The ordinary way this happens: somebody presses Escape while a value is going into a field.
// Both halves are load-bearing. The progress has to survive, because the field is half full; and
// the error has to stay a cancellation, because the outcome word for a stop is `cancelled` and a
// stop that looked like a failure would turn pressing Escape into a defect report.
func TestAStopDuringTypingKeepsWhatWentIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &aTypist{stopAfter: 2, stop: cancel}
	p, err := typeOut(ctx, m, "hello", asLiteral)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stop mid-text answered %v rather than a cancellation", err)
	}
	if p.submitted != 2 || p.total != 5 || p.partial {
		t.Fatalf("the progress is %+v, want 2 of 5 with nothing in doubt", p)
	}
	if n := len(m.submitted); n != 2 {
		t.Errorf("the machine was asked for %d characters after the stop: %q", n,
			m.asked())
	}
	// AND THE SENTENCE FOR IT SAYS THE RUN WAS STOPPED, not that the machine refused.
	// Same field on the screen, two different things to tell somebody.
	said := notAllSubmitted(p, asLiteral, false).Error()
	if !strings.Contains(said, "stopped") || !strings.Contains(said, "2 of 5") {
		t.Errorf("the sentence for a stopped typing is %q", said)
	}
}

// A SECRET'S PROGRESS IS NEVER MEASURED, SO NO NUMBER ABOUT IT CAN BE REPORTED.
//
// Both facts are facts about the value: its LENGTH, and the POSITION it stopped at — a failure
// after seven characters is a prefix whose size somebody now knows. So the sentence for a secret
// carries no digits at all, and the one for a literal does, and the pair is the assertion:
// suppression that cannot be told from having nothing to say is not suppression.
func TestASecretSaysNoNumberAboutItself(t *testing.T) {
	m := &aTypist{takes: []int{-1, -1, 0}}
	_, err := typeOut(context.Background(), m, "s3cr3t-value", asSecret)

	if err == nil {
		t.Fatal("a refused secret reported a typed one")
	}
	said := err.Error()
	if strings.ContainsAny(said, "0123456789") {
		t.Errorf("the sentence for a refused secret is %q, and a number in it is a "+
			"fact about the value: its length, or how much of it somebody now "+
			"knows", said)
	}
	for _, leak := range []string{"s3cr3t", "value", "s3c"} {
		if strings.Contains(said, leak) {
			t.Errorf("the sentence contains %q of the secret: %q", leak, said)
		}
	}
	// AND THE CONTROL: the same refusal about a literal DOES say how far it got.
	_, err = typeOut(context.Background(), &aTypist{takes: []int{-1, -1, 0}},
		"s3cr3t-value", asLiteral)
	if err == nil || !strings.Contains(err.Error(), "2 of 12") {
		t.Errorf("a refused literal says %v, which does not say how much went in", err)
	}
}

// ── AND WHAT THE HOST DOES WITH IT ────────────────────────────────────────────

// aBackendThatTypes runs the PRODUCTION typing sequence against a machine a case arranges.
//
// The shape `aBackendWithARealChord` set: a host-level claim about an account needs the real
// progress in it, and the only way to get a partial one is a machine that takes part of a
// character. Everything else about the backend is the ordinary recording one.
type aBackendThatTypes struct {
	*recBackend
	typist *aTypist
}

func (b aBackendThatTypes) typeText(ctx context.Context, text string,
	kind textKind) (typingProgress, error) {

	return typeOut(ctx, b.typist, text, kind)
}

// aStore is a secrets store with whatever a case put in it.
type aStore map[string]string

func (s aStore) Set(name, value string) error { s[name] = value; return nil }
func (s aStore) Get(name string) (string, bool, error) {
	v, ok := s[name]
	return v, ok, nil
}
func (s aStore) Delete(name string) error { delete(s, name); return nil }
func (s aStore) List() ([]string, error)  { return nil, nil }

// typingHost is a host whose typing is real and whose machine is the one returned.
func typingHost(t *testing.T, m *aTypist) *Host {
	t.Helper()
	return &Host{b: aBackendThatTypes{recBackend: &recBackend{}, typist: m},
		sec: aStore{}}
}

// A REFUSED TYPING IS IN THE ACCOUNT THE COMMAND READS, AND IN THE ONE A STOP CANNOT DISCARD.
//
// It used to be in neither. `Invoke` dispatched typing through `ok(…)`, which turns an error into
// a failed step and records it nowhere — so `Unperformed` was empty and `marco do` reported
// success about a routine whose typing did not happen.
func TestARefusedTypingIsInTheAccountsAndCountsAsAnAttempt(t *testing.T) {
	h := typingHost(t, &aTypist{takes: []int{-1, -1, 0}})

	st, _, _ := call(h, "Type", runtime.Text("hello"))
	if st != "failed" {
		t.Errorf("a refused typing answered %q", st)
	}
	undone := h.Unperformed()
	if len(undone) != 1 || !strings.Contains(undone[0], "2 of 5") {
		t.Fatalf("the run's account of what it did not do is %v", undone)
	}
	if left := h.PartlyTyped(); !strings.Contains(left, "2 of 5") {
		t.Errorf("the account of what is left in the application says %q", left)
	}
	// AND IT COUNTS AS ONE ATTEMPTED INPUT ACTION. A rejected Type is an action this run
	// asked the machine for, so the count of what was ASKED is unchanged by whether it
	// was taken — the progress goes beside that number and never in place of it.
	if n := h.InputActions(); n != 1 {
		t.Errorf("the run says it asked for %d input action(s), want 1", n)
	}
}

// AND A STOP MID-TEXT IS IN THE SECOND ACCOUNT AND NOT IN THE FIRST.
//
// `Unperformed` is what did not happen and is what makes a command FAIL. A stop is not a failure
// — somebody pressed Escape — so a cancelled typing must not land there, or stopping a routine
// would report it as broken. What it must land in is the account a cancelled run cannot discard,
// because the field on the screen is half full either way.
func TestAStoppedTypingIsKeptWhereACancelledRunCannotDiscardIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &aTypist{stopAfter: 2, stop: cancel}
	h := typingHost(t, m)

	h.Invoke(runtime.HostCall{Action: "Type", Input: runtime.Text("hello"), Ctx: ctx})

	if undone := h.Unperformed(); len(undone) != 0 {
		t.Errorf("a stopped run reports %v as something that did not happen, which "+
			"would fail the command for a person pressing Escape", undone)
	}
	left := h.PartlyTyped()
	if !strings.Contains(left, "2 of 5") || !strings.Contains(left, "stopped") {
		t.Errorf("the account of what is left in the application says %q", left)
	}
}

// AND A TEXT THE MACHINE TOOK LEAVES BOTH ACCOUNTS EMPTY.
//
// The control for both of the above, and not a formality: an account that filled up on every
// typing would make every run report an incomplete field, and the warning would stop meaning
// anything long before anybody checked whether it was true.
func TestATypingThatWentInSaysNothing(t *testing.T) {
	h := typingHost(t, &aTypist{})

	st, _, _ := call(h, "Type", runtime.Text("hello"))
	if st != "ok" {
		t.Fatalf("an ordinary typing answered %q", st)
	}
	if undone, left := h.Unperformed(), h.PartlyTyped(); len(undone) != 0 || left != "" {
		t.Errorf("a typing that went in reports %v and %q", undone, left)
	}
	// AND AN EMPTY TEXT IS NOT A FAILURE EITHER. A route can carry an empty segment —
	// an argument filled with nothing — and submitting nothing is what it asked for.
	if st, _, _ := call(h, "Type", runtime.Text("")); st != "ok" {
		t.Errorf("typing an empty text answered %q", st)
	}
}

// A REFUSED SECRET NAMES THE SECRET AND NOTHING ABOUT THE VALUE.
//
// The name is already in the route — somebody can read `do OS's Secret "ark:password"` in the
// file — so naming it exposes nothing new, and it is the only thing that tells a person which
// one to look at. The value, its length and the position it stopped at are the things that must
// not travel, and they travel through four places: the error, the log, the durable record and
// the card. The account is where the last three get theirs.
func TestARefusedSecretNamesTheSecretAndNotTheValue(t *testing.T) {
	const value = "s3cr3t-value"
	h := typingHost(t, &aTypist{takes: []int{-1, -1, 0}})
	h.sec = aStore{"ark:password": value}

	st, _, _ := call(h, "Secret", runtime.Text("ark:password"))
	if st != "failed" {
		t.Errorf("a refused secret answered %q", st)
	}
	undone, left := h.Unperformed(), h.PartlyTyped()
	said := strings.Join(undone, " ") + " " + left
	if !strings.Contains(said, "ark:password") {
		t.Errorf("the accounts say %q and do not name which secret it was", said)
	}
	for _, leak := range []string{value, "s3cr3t", "value", "12", "2 of"} {
		if strings.Contains(said, leak) {
			t.Errorf("the accounts contain %q, which is a fact about the "+
				"value: %q", leak, said)
		}
	}
}

// ── AND THE THREE THINGS THE REVIEW OF THIS SLICE FOUND ───────────────────────

// A SECRET'S PROGRESS IS NOT MEASURED, WHICH IS A CLAIM ABOUT THE VALUE AND NOT ABOUT A SENTENCE.
//
// The sentence said no number from the start, and the numbers were being COMPUTED anyway and left
// out of it — redaction after measurement, which is the opposite of the design. A number that
// exists is one that gets logged by whatever logs values and printed by the next person to add a
// field to a report.
//
// So this reads the returned progress and the error's PAYLOAD rather than its words: the state
// that says "not all of it went in" survives, and nothing that counts anything does.
func TestASecretsProgressCarriesNoNumbers(t *testing.T) {
	m := &aTypist{takes: []int{-1, -1, 1}}
	p, err := typeOut(context.Background(), m, "s3cr3t-value", asSecret)

	if p.counted || p.submitted != 0 || p.total != 0 {
		t.Errorf("a secret's progress is %+v — a length and a position are both "+
			"facts about the value", p)
	}
	// AND THE STATE IS STILL THERE: part of it went in, which is what the field it was
	// going into now holds.
	if !p.partial || p.finished {
		t.Errorf("a secret refused part way reports %+v, which does not say it is "+
			"unfinished with one character in doubt", p)
	}
	var carried partlyTyped
	if !errors.As(err, &carried) {
		t.Fatalf("the error is %v, which carries no progress at all", err)
	}
	if carried.progress.counted || carried.progress.submitted != 0 ||
		carried.progress.total != 0 {
		t.Errorf("the error carries %+v, so the numbers are in the payload and "+
			"only the sentence leaves them out", carried.progress)
	}
	// AND THE LITERAL IS THE CONTROL: the same refusal about a literal DOES count,
	// because the text is in the route and on the screen already.
	p, _ = typeOut(context.Background(), &aTypist{takes: []int{-1, -1, 1}},
		"s3cr3t-value", asLiteral)
	if !p.counted || p.submitted != 2 || p.total != 12 {
		t.Errorf("a literal's progress is %+v, want 2 of 12 counted", p)
	}
}

// AND COMPLETION IS STATE AND NOT ARITHMETIC, so it answers the same way for both kinds.
//
// `whole()` compared `submitted` with `total`, which is why they were accumulated for a secret at
// all: without numbers the comparison would have said a refused secret was complete, and the
// account would have stayed silent about a field with part of a password in it.
func TestCompletionIsStateForBothKinds(t *testing.T) {
	for _, kind := range []textKind{asLiteral, asSecret} {
		p, err := typeOut(context.Background(), &aTypist{}, "hello", kind)
		if err != nil || !p.whole() {
			t.Errorf("a text the machine took reports %+v (%v)", p, err)
		}
		p, _ = typeOut(context.Background(), &aTypist{takes: []int{0}}, "hello", kind)
		if p.whole() {
			t.Errorf("a text refused at the first character reports itself whole: "+
				"%+v", p)
		}
		p, _ = typeOut(context.Background(), &aTypist{takes: []int{-1, -1, 0}},
			"hello", kind)
		if p.whole() {
			t.Errorf("a text refused midway reports itself whole: %+v", p)
		}
	}
}

// THE CADENCE IS BETWEEN THE CHARACTERS, AND A STOP DURING IT STOPS THE NEXT ONE.
//
// `$MARCO_TYPE_CADENCE_MS` paces the keystrokes so a fast application does not drop them, and the
// refactor that lifted this sequence out of the backend dropped the pacing: the knob and the wait
// stayed declared, nothing called them, and every text — including every legacy routine's — went
// out as fast as `SendInput` would take it.
//
// Both halves are asserted. The wait happens between characters and not after the last, so a
// four-character text waits three times; and a stop arriving DURING one of those waits stops the
// character that would have followed it, which is the chord slice's second defect in a different
// loop.
func TestTheCadenceIsBetweenCharactersAndAStopInItStopsTheNext(t *testing.T) {
	m := &aTypist{}
	if _, err := typeOut(context.Background(), m, "abcd", asLiteral); err != nil {
		t.Fatalf("an ordinary text was refused: %v", err)
	}
	if m.paced != 3 {
		t.Errorf("a four-character text waited %d time(s), want 3 — between them "+
			"and not after the last", m.paced)
	}
	if len(m.submitted) != 4 {
		t.Errorf("the machine was handed %d character(s)", len(m.submitted))
	}
	// AND A ONE-CHARACTER TEXT WAITS NOT AT ALL. The wait is between characters, and
	// there is nothing to be between.
	m = &aTypist{}
	if _, err := typeOut(context.Background(), m, "a", asLiteral); err != nil {
		t.Fatal(err)
	}
	if m.paced != 0 {
		t.Errorf("a one-character text waited %d time(s)", m.paced)
	}

	// THE STOP, FROM INSIDE THE WAIT.
	ctx, cancel := context.WithCancel(context.Background())
	m = &aTypist{stopInThePace: true, stop: cancel}
	p, err := typeOut(ctx, m, "abcd", asLiteral)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stop during the wait answered %v rather than a cancellation", err)
	}
	if len(m.submitted) != 1 {
		t.Errorf("the machine was handed %d character(s) after a stop during the "+
			"first wait, want the one that had already gone in", len(m.submitted))
	}
	if p.submitted != 1 || p.total != 4 || p.partial {
		t.Errorf("the progress is %+v, want 1 of 4 with nothing in doubt", p)
	}
}

// A SECRET THAT COULD NOT BE LOOKED UP IS IN THE ACCOUNT, AND CLAIMS NOTHING ABOUT TYPING.
//
// Two different outcomes used to go out through `fail` and reach no account at all: a secret the
// store has no value for, and a store that would not answer. The chain did its job — the arm
// ended the routine and nothing after it ran — and then the COMMAND reported `performed`, because
// `runRoute` turns a non-empty account into a failed run and the account was empty.
//
// AND IT IS THE OTHER ACCOUNT. Nothing was typed, so there is no half-filled field: a person
// whose secret was never found must not be told part of it may be sitting in a form.
func TestASecretThatCouldNotBeLookedUpIsInTheAccount(t *testing.T) {
	for _, c := range []struct {
		what  string
		store secrets.Store
		asked string
		says  string
	}{
		{"the store has no value for it", aStore{}, "ark:password", "is not set"},
		{"the store would not answer", aBrokenStore{}, "ark:password",
			"would not answer"},
		{"the routine does not say which secret", aStore{}, "", "does not say which"},
	} {
		t.Run(c.what, func(t *testing.T) {
			rec := &recBackend{}
			h := &Host{b: rec, sec: c.store}

			st, v, _ := call(h, "Secret", runtime.Text(c.asked))
			if st != "failed" {
				t.Fatalf("it answered %q", st)
			}
			said := ""
			if v.IsError() {
				said = v.AsError().Message
			}
			if !strings.Contains(said, c.says) {
				t.Errorf("the refusal is %q and does not say %q", said, c.says)
			}
			// THE ACCOUNT THE COMMAND READS. Without this the step fails, the
			// chain's arm ends the routine, and the command reports success.
			undone := h.Unperformed()
			if len(undone) != 1 || !strings.Contains(undone[0], c.says) {
				t.Fatalf("the run's account of what it did not do is %v", undone)
			}
			// AND NOT THE OTHER ONE, and nothing was typed.
			if left := h.PartlyTyped(); left != "" {
				t.Errorf("a secret that was never found is reported as text "+
					"left half submitted: %q", left)
			}
			if len(rec.typed) != 0 {
				t.Errorf("the machine was asked to type %v", rec.typed)
			}
		})
	}
}

// AND A SECRET THAT WAS FOUND IS IN NEITHER ACCOUNT.
//
// The control for all three above, and the one that keeps them from being satisfied by a host
// that complains about every secret.
func TestASecretThatWasFoundIsInNeitherAccount(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec, sec: aStore{"ark:password": "s3cr3t-value"}}

	st, _, _ := call(h, "Secret", runtime.Text("ark:password"))
	if st != "ok" {
		t.Fatalf("a secret that was found answered %q", st)
	}
	if undone, left := h.Unperformed(), h.PartlyTyped(); len(undone) != 0 || left != "" {
		t.Errorf("a secret that went in reports %v and %q", undone, left)
	}
	if len(rec.typed) != 1 || rec.typed[0] != "s3cr3t-value" {
		t.Errorf("the machine was asked to type %v", rec.typed)
	}
}

// aBrokenStore is a credential store that will not answer, which is a real state — a locked
// store, a service that is not running, a profile the process cannot read.
type aBrokenStore struct{ aStore }

func (aBrokenStore) Get(string) (string, bool, error) {
	return "", false, errors.New("the credential store is not available")
}
