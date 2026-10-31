package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const said = "write a note saying hello there"

// spansFor runs one model reply through the whole question and reports what came back, keyed by
// argument name, with each span resolved to the words it points at.
func spansFor(t *testing.T, reply string, names ...string) map[string]string {
	t.Helper()
	srv := serverReplying(t, reply, nil)
	defer srv.Close()
	out := map[string]string{}
	for _, s := range extract(context.Background(), cfgFor(srv.URL), said, names) {
		if s.Start < 0 || s.End > len(said) || s.Start >= s.End {
			t.Errorf("%s: the span %d..%d does not index the request",
				s.Name, s.Start, s.End)
			continue
		}
		out[s.Name] = said[s.Start:s.End]
	}
	return out
}

// THE ORDINARY CASE: THE MODEL QUOTES THE WORDS AND THE PLUGIN SAYS WHERE THEY ARE.
func TestExtractQuotedWordsBecomeASpan(t *testing.T) {
	var body string
	srv := serverReplying(t, `{"message":"hello there"}`, &body)
	defer srv.Close()
	got := extract(context.Background(), cfgFor(srv.URL), said, []string{"message"})
	if len(got) != 1 {
		t.Fatalf("got %+v, want one span", got)
	}
	if got[0].Name != "message" || said[got[0].Start:got[0].End] != "hello there" {
		t.Errorf("got %+v, pointing at %q", got[0], said[got[0].Start:got[0].End])
	}
	// AND THE PROMPT CARRIES THE SENTENCE AND THE NAMES, which is all it is allowed to
	// carry. The engine asserts the same thing about the wire; this is the other end of it.
	if !strings.Contains(body, "hello there") || !strings.Contains(body, "message") {
		t.Errorf("the prompt is missing the request or the names: %s", body)
	}
}

// A VALUE THAT IS NOT IN THE SENTENCE IS NOT REPORTED.
//
// The plugin's half of the rule the engine holds structurally. A model that paraphrased,
// translated, corrected the spelling, substituted a default or simply made something up has
// produced words there is nowhere to point at, so no span is sent at all.
func TestExtractInventedWordsAreNotReported(t *testing.T) {
	for _, c := range []struct{ why, reply string }{
		{"a value nobody said", `{"message":"something I made up"}`},
		{"a paraphrase", `{"message":"greetings to you"}`},
		{"a correction", `{"message":"Hello, there!"}`},
		{"a default", `{"message":"N/A"}`},
		{"a name the routine does not declare", `{"subject":"hello there"}`},
		{"nothing at all", `NONE`},
		{"an empty value", `{"message":""}`},
		{"whitespace", `{"message":"   "}`},
		{"prose with no JSON in it", `I could not find a value for that.`},
		{"the answer to a different question", `{"route":"write-a-note"}`},
	} {
		if got := spansFor(t, c.reply, "message"); len(got) != 0 {
			t.Errorf("%s was reported: %v", c.why, got)
		}
	}
}

// AND A VALUE THE MODEL RE-CASED COMES BACK IN THE PERSON'S OWN CASING.
//
// Not a repair: what is reported is a RANGE, so the words come from the request either way. A
// model that title-cased what it was told to copy gets the person's own capitalisation handed back
// to it — which is the property that makes locating case-insensitively safe at all.
func TestExtractRecasedWordsComeBackAsWritten(t *testing.T) {
	if got := spansFor(t, `{"message":"Hello There"}`, "message")["message"]; got != "hello there" {
		t.Errorf("got %q, want the request's own casing", got)
	}
}

// AND THE QUOTES AND SPACES A MODEL WRAPS A VALUE IN ARE NOT PART OF IT.
func TestExtractStripsTheModelsOwnPunctuation(t *testing.T) {
	for _, reply := range []string{
		`{"message":"\"hello there\""}`,
		`{"message":"  hello there  "}`,
		`{"message":"'hello there'"}`,
	} {
		if got := spansFor(t, reply, "message")["message"]; got != "hello there" {
			t.Errorf("%s gave %q", reply, got)
		}
	}
}

// A CHATTY MODEL IS STILL UNDERSTOOD, and one that answered in prose alone is not guessed at.
func TestExtractChattyReply(t *testing.T) {
	got := spansFor(t, "Sure! Here you go:\n```json\n"+
		`{"message":"hello there"}`+"\n```\nHope that helps.", "message")
	if got["message"] != "hello there" {
		t.Errorf("got %v", got)
	}
}

// SEVERAL ARGUMENTS, AND THE ONES IT COULD NOT FIND SIMPLY GO UNANSWERED.
func TestExtractAnswersWhatItCanAndLeavesTheRest(t *testing.T) {
	got := spansFor(t, `{"message":"hello there","recipient":"Anne"}`, "message", "recipient")
	if len(got) != 1 || got["message"] != "hello there" {
		t.Errorf("got %v, want only the message", got)
	}
}

// AND THE ANSWER IS IN THE ORDER THE ROUTINE DECLARES, so the same reply reads the same twice.
func TestExtractReportsSpansInTheDeclaredOrder(t *testing.T) {
	srv := serverReplying(t, `{"note":"hello there","kind":"note"}`, nil)
	defer srv.Close()
	for i := range 5 {
		got := extract(context.Background(), cfgFor(srv.URL), said,
			[]string{"kind", "note"})
		if len(got) != 2 || got[0].Name != "kind" || got[1].Name != "note" {
			t.Fatalf("run %d gave %+v", i, got)
		}
	}
}

// A QUESTION WITH NOTHING TO ASK ABOUT IS NOT ASKED.
//
// Checked on whether the model was REACHED, not only on the answer: a question with no names in it
// would get the same nothing back from a model that saw it and declined, and the thing worth
// holding is that nobody's sentence left the machine for a question that had none.
func TestExtractWithNoNamesOrNoWordsAsksNothing(t *testing.T) {
	var body string
	srv := serverReplying(t, `{"message":"hello there"}`, &body)
	defer srv.Close()
	for _, c := range []struct {
		why   string
		input string
		names []string
	}{
		{"no names", said, nil},
		{"no words", "", []string{"message"}},
		{"nothing but whitespace", "   	 ", []string{"message"}},
	} {
		if got := extract(context.Background(), cfgFor(srv.URL), c.input,
			c.names); got != nil {
			t.Errorf("%s was answered: %+v", c.why, got)
		}
		if body != "" {
			t.Errorf("%s reached the model: %s", c.why, body)
			body = ""
		}
	}
}

// AND A MODEL THAT ANSWERS WITH A STATUS EXTRACTS NOTHING.
//
// A model server that is up but refusing — out of memory, model not pulled, rate limited — sends a
// status and often a body beside it, and a body read without looking at the status is a reply from
// something that said it was not replying.
func TestExtractIgnoresABodyThatCameWithAStatus(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable,
		http.StatusInternalServerError, http.StatusTooManyRequests,
		http.StatusNotFound, http.StatusUnauthorized} {
		srv := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(chatResp{Choices: []struct {
					Message message `json:"message"`
				}{{Message: message{Role: "assistant",
					Content: `{"message":"hello there"}`}}}})
			}))
		got := extract(context.Background(), cfgFor(srv.URL), said,
			[]string{"message"})
		srv.Close()
		if got != nil {
			t.Errorf("a %d answered with %+v", status, got)
		}
	}
}

// AND A MODEL THAT IS NOT THERE EXTRACTS NOTHING.
func TestExtractWithNoModelExtractsNothing(t *testing.T) {
	if got := extract(context.Background(), cfgFor("http://127.0.0.1:1"), said,
		[]string{"message"}); got != nil {
		t.Errorf("an unreachable model extracted %+v", got)
	}
}

// THE ANSWER ON THE WIRE HAS NOWHERE TO PUT TEXT.
//
// `resolver.Span` has no field for a value, so this must not grow one either: a plugin that could
// name its own text would be asking the engine to trust it, and the engine's refusal of that is
// the one thing a plugin cannot be relied on to keep. Asserted on the bytes that go out.
func TestTheAnswerCarriesOffsetsAndNeverText(t *testing.T) {
	b, err := json.Marshal(extractResponse{Values: []span{{Name: "message",
		Start: 20, End: 31}}})
	if err != nil {
		t.Fatal(err)
	}
	if wire := string(b); wire != `{"values":[{"name":"message","start":20,"end":31}]}` {
		t.Errorf("the answer goes out as %s", wire)
	}
	// AND THE CLAIM IS ABOUT THE SPAN, not the envelope around it: `values` is the list and
	// a `value` would be the text, which is the whole difference. Marshalled alone so that
	// the one is not read for the other.
	one, err := json.Marshal(span{Name: "message", Start: 20, End: 31})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(one); got != `{"name":"message","start":20,"end":31}` {
		t.Errorf("a span goes out as %s", got)
	}
}

// AND A LENGTH-CHANGING FOLD IS NOT MATCHED, rather than matched to the wrong bytes.
//
// İ is two bytes and lowers to a one-byte i, so every offset after it moves when the sentence
// is lowered. A case-insensitive match found in the lowered string therefore names bytes that are
// somebody else's in this one, and here it would be a space and seven letters of the word. Not
// finding it costs the person a value they then type themselves; finding it there types the wrong
// thing.
func TestLocateDoesNotFoldWhenFoldingMovesTheBytes(t *testing.T) {
	const shifted = "İ say istanbul now"
	if start, end, ok := locate(shifted, "ISTANBUL"); ok {
		t.Errorf("a fold that moved the bytes was located anyway, at %q",
			shifted[start:end])
	}
	// AND THE WORD AS WRITTEN IS STILL FOUND, so the guard refuses a fold rather than a
	// sentence with an awkward character in it.
	start, end, ok := locate(shifted, "istanbul")
	if !ok || shifted[start:end] != "istanbul" {
		t.Errorf("the word as written was not located: %d..%d %v", start, end, ok)
	}
}
