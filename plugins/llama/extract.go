package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Answering "which words in that sentence are this argument's value" with a local model.
//
// # What this question is
//
// A saved routine may declare that it takes text — a message to type, a name to search for — and
// somebody who says "write a note saying hello there" has already supplied it. This maps the
// sentence onto the arguments the routine declares, and that is the whole job. It does not decide
// WHICH routine (that is resolve), does not decide what the sentence MEANT (that is interpret),
// does not see the desktop and is never told anything about it.
//
// So the model's entire input is the person's own sentence and a list of argument names. Nothing
// about the screen, nothing about the routine beyond the names — which is why this is a good job
// for a small local model, and why nothing about anybody's work ever leaves the machine.
//
// Protocol (the fourth question in internal/resolver):
//
//	→ {"input":"write a note saying hello there","extract":{"names":["message"]}}
//	← {"values":[{"name":"message","start":20,"end":31}]}
//
// Every failure degrades to no `values` field at all, which Marco reads as "nothing extracted
// anything" and answers by asking the person for the value itself.
//
// # THE MODEL IS ASKED FOR WORDS AND THE PLUGIN TURNS THEM INTO OFFSETS
//
// The wire carries offsets only: there is no field on a span for text, so a value Marco did not
// send cannot be returned to it. That is the engine's boundary and this plugin does not get to
// soften it.
//
// But a 3B instruct model cannot count bytes, and one asked for `start` and `end` directly answers
// with numbers that are confidently wrong — which, validated, means nothing is ever extracted. So
// the model is asked for the thing it is good at: QUOTE THE WORDS. The plugin then LOCATES those
// words in the request and reports where it found them, and a value it cannot find is not reported
// at all.
//
// Which means invention dies twice, independently and for the same reason. A model that
// paraphrased, translated, corrected the spelling or made something up produces words that are not
// in the sentence, so there is nowhere to point and no span to send; and if a span were sent
// anyway, the engine copies the text out of its own copy of the request and never reads anything
// the plugin said about it. As `parseSemantic` puts it one question along: this is best-effort
// cleaning, never a boundary.

// extractRequest is the question, as it arrives.
type extractRequest struct {
	Names []string `json:"names"`
}

// span is where one argument's value is, in the request that was sent. NO TEXT FIELD, matching
// `resolver.Span` exactly — see above.
type span struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// extractResponse is what goes back when something was found.
type extractResponse struct {
	Values []span `json:"values,omitempty"`
}

func emitValues(v []span) {
	b, _ := json.Marshal(extractResponse{Values: v})
	os.Stdout.Write(append(b, '\n'))
}

const extractPrompt = "You pick out the parts of a computer user's request that are values for " +
	"named inputs. Reply with ONE JSON object mapping each input name to the user's own words " +
	"for it, and nothing else:\n" +
	"{\"<name>\":\"<the user's exact words>\"}\n" +
	"Rules:\n" +
	"- COPY THE WORDS EXACTLY as the user wrote them, character for character. Do not " +
	"translate, re-word, summarise, expand, correct the spelling or change the " +
	"capitalisation. The words you give back must appear in the request verbatim.\n" +
	"- Give the value only, not the words around it: \"write a note saying hello there\" with " +
	"an input named message -> {\"message\":\"hello there\"}.\n" +
	"- LEAVE OUT any name whose value the user did not say. Omitting a name is always better " +
	"than inventing one, and something else will ask them for it.\n" +
	"- Never invent a value and never use a default. If you cannot see the words in the " +
	"request, the user did not say them.\n" +
	"- If the request contains no values at all, reply exactly NONE.\n" +
	"Output JSON only."

func extractUser(input string, names []string) string {
	var b strings.Builder
	b.WriteString("Input names:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	fmt.Fprintf(&b, "\nUser asked: %q\n", input)
	b.WriteString("Reply with the JSON object, or NONE.")
	return b.String()
}

// extract asks the model which words are which value. nil means nothing was extracted.
func extract(ctx context.Context, cfg config, input string, names []string) []span {
	if strings.TrimSpace(input) == "" || len(names) == 0 {
		return nil
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: extractPrompt},
			{Role: "user", Content: extractUser(input, names)},
		},
		Temperature: 0, // copying words out of a sentence, not writing
		MaxTokens:   256,
		Stream:      false,
	})
	if err != nil {
		return nil
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var cr chatResp
	if json.NewDecoder(resp.Body).Decode(&cr) != nil || len(cr.Choices) == 0 {
		return nil
	}
	return parseValues(cr.Choices[0].Message.Content, input, names)
}

// parseValues digs the model's answer out of whatever prose surrounds it and turns each quoted
// value into a span of the request.
//
// A name the routine does not declare is dropped, and so is a value that is not in the sentence.
// Both are refusals rather than repairs, for the reason at the top of this file.
func parseValues(content, input string, names []string) []span {
	if strings.EqualFold(strings.Trim(strings.TrimSpace(content), "\"'`. \n\r"), "none") {
		return nil
	}
	raw := extractJSON(content)
	if raw == "" {
		return nil
	}
	var said map[string]string
	if json.Unmarshal([]byte(raw), &said) != nil {
		return nil
	}
	var out []span
	// IN THE ORDER THE ROUTINE DECLARES THEM, not the order a JSON object happened to be
	// ranged over in, so the same answer gives the same reply twice running.
	for _, n := range names {
		words, ok := said[n]
		if !ok {
			continue
		}
		start, end, found := locate(input, words)
		if !found {
			continue
		}
		out = append(out, span{Name: n, Start: start, End: end})
	}
	return out
}

// locate finds words in input and returns the half-open byte range holding them.
//
// # Why it tries more than once, and why that is not a repair
//
// Exact first. Then the same words with the quotes and spaces a model wraps them in stripped off;
// and either of those case-insensitively, because a small model will title-case a value it was
// told to copy. Each attempt is still a search for words the person actually wrote — what comes
// back is a RANGE, and the engine reads the text out of its own copy of the request, so a model
// that re-cased a value has the person's own capitalisation handed back to it instead of its own.
//
// A value that cannot be found by any of these is not reported. There is no further attempt that
// relaxes into a resemblance: "near enough to something they said" is how a value nobody said gets
// typed somewhere.
func locate(input, words string) (int, int, bool) {
	for _, candidate := range []string{words, strings.Trim(strings.TrimSpace(words), "\"'`")} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if i := strings.Index(input, candidate); i >= 0 {
			return i, i + len(candidate), true
		}
		if i := indexFold(input, candidate); i >= 0 {
			return i, i + len(candidate), true
		}
	}
	return 0, 0, false
}

// indexFold is strings.Index ignoring case, as a byte offset into s.
//
// An offset is only safe to report from a fold that preserves LENGTH, which folding ASCII does and
// folding in general does not. So a string that changes length when lowered is not matched this
// way at all, rather than being matched to the wrong bytes — and the engine would refuse the span
// that came of it anyway, having found it off a rune boundary.
func indexFold(s, needle string) int {
	ls, ln := strings.ToLower(s), strings.ToLower(needle)
	if len(ls) != len(s) || len(ln) != len(needle) {
		return -1
	}
	return strings.Index(ls, ln)
}
