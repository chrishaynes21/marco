package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// F7: the answer to "Which window" is read alone, and the narration typed after it is still there
// for the session. A buffered reader here would swallow the first phrase.
func TestTheWindowAnswerLeavesTheNarrationAlone(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev; r.Close() })
	if _, err := w.WriteString("2\nclick this\ndone\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	if got := strings.TrimSpace(readWindowAnswer()); got != "2" {
		t.Fatalf("the answer read was %q", got)
	}
	rest, _ := io.ReadAll(r)
	if string(rest) != "click this\ndone\n" {
		t.Errorf("the narration after the answer is %q", rest)
	}
}
