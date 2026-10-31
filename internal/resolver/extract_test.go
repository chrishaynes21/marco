package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// THE THIRD QUESTION ON THE ONE PLUGIN BOUNDARY.
//
// What this file is mostly about is the shape of an ANSWER, because the whole safety of the
// question is that a plugin cannot express an invented value — it names offsets and Marco copies
// the text out of the request it sent. So the cases below are the ways a plugin can be wrong about
// WHICH span, and each one is refused rather than repaired.
//
// See [[Extracting-argument-values-proposal-2026-10-30]].

// echoingPlugin answers with whatever `$FAKE_REPLY` holds, and writes the request it was given to
// `$FAKE_SAW` when that is set.
//
// ONE BINARY FOR EVERY CASE. Each shape below is a different reply rather than a different plugin,
// because `go build` per case is most of the runtime of this file and none of its meaning.
const echoingPlugin = `package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	b, _ := io.ReadAll(os.Stdin)
	if p := os.Getenv("FAKE_SAW"); p != "" {
		_ = os.WriteFile(p, b, 0o644)
	}
	fmt.Println(os.Getenv("FAKE_REPLY"))
}
`

// anExtractingPlugin builds the echoing plugin once and points the resolver at it.
func anExtractingPlugin(t *testing.T) {
	t.Helper()
	t.Setenv("MARCO_RESOLVER", buildPlugin(t, echoingPlugin))
}

// replying sets what the plugin will answer.
func replying(t *testing.T, json string) {
	t.Helper()
	t.Setenv("FAKE_REPLY", json)
}

// aRequest is 31 bytes: "hello there" is [20, 31) and the space before it is [19, 20).
//
// THE OFFSETS IN THIS FILE ARE WRITTEN OUT because that is what the test is about. A helper that
// computed them with `strings.Index` would be asserting that Marco can find a substring, and the
// property is that a span Marco did NOT choose indexes the right bytes.
const aRequest = `write a note saying hello there`

// A SPAN BECOMES A VALUE BY BEING COPIED OUT OF THE REQUEST.
//
// The plugin named offsets; the text came from the words Marco sent. That is the whole mechanism,
// and everything else in this file is a way of getting the offsets wrong.
func TestASpanIsCopiedOutOfTheRequest(t *testing.T) {
	anExtractingPlugin(t)
	replying(t, `{"values":[{"name":"message","start":20,"end":31}]}`)
	got, ok := Extract(context.Background(), aRequest, []string{"message"})
	if !ok || len(got) != 1 {
		t.Fatalf("nothing was extracted: %+v", got)
	}
	if got[0].Name != "message" {
		t.Errorf("the span names %q", got[0].Name)
	}
	if v := got[0].Value(aRequest); v != "hello there" {
		t.Errorf("the value came out as %q", v)
	}
}

// AND WHAT IS COPIED IS COPIED VERBATIM.
//
// Not trimmed, not collapsed, not normalised, because a value is used exactly as it was supplied
// and that is already the rule for one that was typed. A span that takes in the space before the
// words keeps it; the engine's only whitespace rule is that a span of NOTHING BUT whitespace is
// refused, which is a different claim and is checked below.
//
// A mutation that trimmed the value on its way out survived the rest of this file, which is why
// this is here: every other test names a span whose edges happen to be the words'.
func TestAValueIsCopiedVerbatimAndNeverTidied(t *testing.T) {
	anExtractingPlugin(t)
	// [19, 31) is the space before "hello there" and then the words.
	replying(t, `{"values":[{"name":"message","start":19,"end":31}]}`)
	got, ok := Extract(context.Background(), aRequest, []string{"message"})
	if !ok || len(got) != 1 {
		t.Fatalf("a span with a space in it was refused: %+v", got)
	}
	if v := got[0].Value(aRequest); v != " hello there" {
		t.Errorf("the value came out as %q, which has been tidied", v)
	}
	// AND THE PERSON'S OWN CAPITALISATION, for the same reason and because this is the
	// half that matters to a name: somebody who said Anne is not asking for anne.
	const named = "dm Anne saying Hello There"
	replying(t, `{"values":[{"name":"message","start":3,"end":7}]}`)
	got, ok = Extract(context.Background(), named, []string{"message"})
	if !ok || got[0].Value(named) != "Anne" {
		t.Errorf("a capitalised value came out as %+v", got)
	}
	replying(t, `{"values":[{"name":"message","start":15,"end":26}]}`)
	got, ok = Extract(context.Background(), named, []string{"message"})
	if !ok || got[0].Value(named) != "Hello There" {
		t.Errorf("capitalised words came out as %+v", got)
	}
}

// AND A PLUGIN CANNOT ANSWER WITH TEXT AT ALL.
//
// THE POINT OF THE WHOLE DESIGN, as a test: [Span] has no field for a value, so a plugin that
// returns one — invented, paraphrased, translated, re-cased, tidied — is not refused by a rule that
// could be forgotten. There is nowhere for the text to go.
func TestAPluginCannotAnswerWithTextOfItsOwn(t *testing.T) {
	anExtractingPlugin(t)
	for _, c := range []struct {
		why    string
		reply  string
		usable bool
	}{
		{"text instead of offsets",
			`{"values":[{"name":"message","value":"something I made up"}]}`, false},
		{"text under another field name",
			`{"values":[{"name":"message","text":"hello there"}]}`, false},
		// THIS ONE IS THE DISCRIMINATING CASE: valid offsets BESIDE invented text. The
		// offsets are what is used and the text it sent is never read, so what comes out
		// is the request's own words rather than the word the plugin asked for.
		{"text beside valid offsets",
			`{"values":[{"name":"message","start":20,"end":31,` +
				`"value":"GOODBYE"}]}`, true},
	} {
		replying(t, c.reply)
		got, ok := Extract(context.Background(), aRequest, []string{"message"})
		if ok != c.usable {
			t.Errorf("%s: extracted=%v, wanted %v (%+v)", c.why, ok, c.usable, got)
			continue
		}
		if c.usable && got[0].Value(aRequest) != "hello there" {
			t.Errorf("%s: the value came out as %q", c.why,
				got[0].Value(aRequest))
		}
	}
}

// EVERY WAY OF BEING WRONG ABOUT WHICH SPAN, REFUSED.
func TestABadSpanIsRefusedRatherThanRepaired(t *testing.T) {
	anExtractingPlugin(t)
	for _, c := range []struct{ why, reply string }{
		{"a name nobody asked about",
			`{"values":[{"name":"subject","start":20,"end":31}]}`},
		{"two spans for one argument",
			`{"values":[{"name":"message","start":20,"end":31},` +
				`{"name":"message","start":0,"end":5}]}`},
		{"an end past the request",
			`{"values":[{"name":"message","start":20,"end":999}]}`},
		{"a negative start",
			`{"values":[{"name":"message","start":-1,"end":31}]}`},
		{"an empty span",
			`{"values":[{"name":"message","start":20,"end":20}]}`},
		{"a backwards span",
			`{"values":[{"name":"message","start":31,"end":20}]}`},
		{"only whitespace",
			`{"values":[{"name":"message","start":19,"end":20}]}`},
		{"no values at all", `{"values":[]}`},
		{"a route-only plugin's answer", `{"route":"write-a-note"}`},
		{"an interpret-only plugin's answer", `{"semantic":{"kind":"dismiss"}}`},
		{"nothing that parses", `not json at all`},
		{"nothing at all", ``},
	} {
		replying(t, c.reply)
		if got, ok := Extract(context.Background(), aRequest,
			[]string{"message"}); ok {
			t.Errorf("%s was accepted: %+v", c.why, got)
		}
	}
}

// AND A SPAN THAT WOULD SLICE A CHARACTER IN HALF IS REFUSED.
//
// A model counting characters where Marco counts bytes is the likeliest way to arrive here, and a
// half character is not text anybody said. Asserted on a request whose bytes and runes differ: the
// é at [7, 9) is two bytes, so the whole thing is 17 bytes and "café au lait" is [4, 17).
func TestASpanOffARuneBoundaryIsRefused(t *testing.T) {
	anExtractingPlugin(t)
	const said = "say café au lait"
	replying(t, `{"values":[{"name":"message","start":8,"end":17}]}`)
	if got, ok := Extract(context.Background(), said, []string{"message"}); ok {
		t.Errorf("a span starting mid-character was accepted: %q", got[0].Value(said))
	}
	// AND THE SAME WORDS, ALIGNED, COME THROUGH WITH THE CHARACTER INTACT.
	replying(t, `{"values":[{"name":"message","start":4,"end":17}]}`)
	got, ok := Extract(context.Background(), said, []string{"message"})
	if !ok || got[0].Value(said) != "café au lait" {
		t.Errorf("an aligned span gave %+v", got)
	}
}

// ONE BAD SPAN DOES NOT COST THE PERSON THE GOOD ONES.
func TestAGoodSpanSurvivesABadOneBesideIt(t *testing.T) {
	anExtractingPlugin(t)
	const said = "dm sam saying hello there"
	replying(t, `{"values":[{"name":"person","start":3,"end":6},`+
		`{"name":"nobody-asked","start":0,"end":2}]}`)
	got, ok := Extract(context.Background(), said, []string{"person", "message"})
	if !ok || len(got) != 1 {
		t.Fatalf("the usable span was lost with the unusable one: %+v", got)
	}
	if got[0].Value(said) != "sam" {
		t.Errorf("the value came out as %q", got[0].Value(said))
	}
}

// WHAT THE PLUGIN IS TOLD IS THE WORDS AND THE NAMES, AND NOTHING ELSE.
//
// The boundary, asserted on the bytes that actually went to the subprocess. Nothing about the
// screen, and nothing about the routine beyond what it takes — because a value is a span of the
// sentence, so the evidence a screen question needs is deliberately absent and a plugin that
// wanted to send a desktop somewhere has nothing here to send.
func TestThePluginIsToldTheWordsAndTheNamesAndNothingElse(t *testing.T) {
	anExtractingPlugin(t)
	saw := filepath.Join(t.TempDir(), "asked.json")
	t.Setenv("FAKE_SAW", saw)
	replying(t, `{"values":[{"name":"message","start":20,"end":31}]}`)
	if _, ok := Extract(context.Background(), aRequest, []string{"message"}); !ok {
		t.Fatal("nothing was extracted, so there is nothing to check the question of")
	}
	b, err := os.ReadFile(saw)
	if err != nil {
		t.Fatal(err)
	}
	asked := string(b)
	for _, want := range []string{`"input"`, aRequest, `"extract"`, `"names"`, `"message"`} {
		if !strings.Contains(asked, want) {
			t.Errorf("the question does not carry %s: %s", want, asked)
		}
	}
	// AND IT IS NEITHER A ROUTE QUESTION NOR A VERB QUESTION. Those fields are absent
	// rather than empty-but-present, so a plugin cannot answer something nobody asked.
	for _, absent := range []string{`"routes"`, `"verbs"`, `"interpret"`, `"which"`,
		`"controls"`, `"application"`, `"window"`, `"mode"`} {
		if strings.Contains(asked, absent) {
			t.Errorf("the question carries %s: %s", absent, asked)
		}
	}
}

// A PLUGIN THAT HANGS BECOMES "NOTHING EXTRACTED".
//
// One of the properties that has to hold before any model is trusted: the honest failure is the
// one an unconfigured plugin already produces, so nothing downstream needs a second case for it.
func TestAPluginThatHangsExtractsNothing(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildPlugin(t, hanging))
	was := ExtractTimeout
	ExtractTimeout = 150 * time.Millisecond
	t.Cleanup(func() { ExtractTimeout = was })
	start := time.Now()
	if _, ok := Extract(context.Background(), aRequest, []string{"message"}); ok {
		t.Error("a plugin that never answered extracted something")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the question was not bounded: it took %v", took)
	}
}

// AND A CANCELLED CONTEXT IS THE SAME ANSWER, NOT A DIFFERENT ONE.
func TestACancelledQuestionExtractsNothing(t *testing.T) {
	anExtractingPlugin(t)
	replying(t, `{"values":[{"name":"message","start":20,"end":31}]}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := Extract(ctx, aRequest, []string{"message"}); ok {
		t.Error("a cancelled question extracted something")
	}
}

// AND NO PLUGIN AT ALL IS THE ORDINARY ANSWER.
func TestNoPluginExtractsNothing(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	if _, ok := Extract(context.Background(), aRequest, []string{"message"}); ok {
		t.Error("something was extracted with no plugin configured")
	}
	if ExtractorConfigured() {
		t.Error("an unset resolver reads as configured")
	}
}

// A QUESTION WITH NOTHING TO ASK ABOUT IS NOT ASKED.
//
// Checked on whether the plugin RAN, not only on the answer: a question with no names in it would
// get the same false back from a plugin that saw it and declined, and the thing worth holding is
// that nothing was spawned and nobody's sentence left the process.
func TestAQuestionWithNoNamesOrNoWordsIsNotAsked(t *testing.T) {
	anExtractingPlugin(t)
	saw := filepath.Join(t.TempDir(), "asked.json")
	t.Setenv("FAKE_SAW", saw)
	replying(t, `{"values":[{"name":"message","start":0,"end":1}]}`)
	tooMany := make([]string, MaxArgumentNames+1)
	for i := range tooMany {
		tooMany[i] = "message"
	}
	for _, c := range []struct {
		why   string
		input string
		names []string
	}{
		{"no names", aRequest, nil},
		{"no words", "", []string{"message"}},
		{"nothing but whitespace", "   \t ", []string{"message"}},
		{"an empty name", aRequest, []string{""}},
		{"too many names", aRequest, tooMany},
		{"a name that is prose", aRequest,
			[]string{strings.Repeat("x", MaxArgumentName+1)}},
	} {
		if _, ok := Extract(context.Background(), c.input, c.names); ok {
			t.Errorf("%s was asked and answered", c.why)
		}
		if _, err := os.Stat(saw); err == nil {
			t.Errorf("%s reached the plugin", c.why)
			_ = os.Remove(saw)
		}
	}
}

// AND THIS QUESTION NEVER READS THE SHADOW INTERPRETER'S VARIABLE.
//
// Two variables, separate on purpose: `$MARCO_RESOLVER` names the plugin that answers which
// routine a request meant and what a sentence meant, and `$MARCO_INTERPRETER` names the one that
// would answer which observed CONTROL was meant — the question `internal/director/meaning` holds
// in shadow mode and the only reader of that variable. Configuring extraction must not configure
// that, which is why this asks the first variable and `TestTheInterpreterIsNotWiredIntoAnything`
// stays green.
func TestExtractionDoesNotReadTheInterpretersVariable(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	t.Setenv("MARCO_INTERPRETER", buildPlugin(t, echoingPlugin))
	replying(t, `{"values":[{"name":"message","start":20,"end":31}]}`)
	if _, ok := Extract(context.Background(), aRequest, []string{"message"}); ok {
		t.Error("extraction answered out of the shadow interpreter's plugin")
	}
	if ExtractorConfigured() {
		t.Error("an interpreter alone reads as an extractor")
	}
}
