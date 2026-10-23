package main

// THE PAGE IS A DOCUMENT, AND IT HAS TO START LIKE ONE.
//
// # Why this exists
//
// Because it did not. A sed edit meant to replace a line PREPENDED to the file instead, and the
// page shipped with a stray fragment of its own JavaScript in front of the doctype:
//
//	$("outcome").textContent = w.outcome || (w.active ? "running" : "")<!doctype html>
//
// Two consequences, neither of which a Go test was looking for. A text node before the doctype
// puts the document in QUIRKS MODE, where the box model and the stylesheet mean something else
// than what they were written against. And the fragment itself renders — a line of code across
// the top of the window, in the one surface whose whole job is to be trustworthy about what a run
// is doing.
//
// It survived a source review and a green suite because nothing read the first line of the file.
// This reads it.

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// THE FIRST LINE IS THE DOCTYPE, and nothing precedes it.
func TestThePageBeginsAsADocument(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	if !strings.HasPrefix(page, "<!doctype html>\n") {
		first := page
		if i := strings.IndexByte(page, '\n'); i >= 0 {
			first = page[:i]
		}
		t.Fatalf("the page does not begin with its doctype, so it renders in "+
			"quirks mode and whatever is in front of it renders too.\n"+
			"  first line: %q", first)
	}
	// AND THE DOCUMENT'S OWN FURNITURE IS THERE, each exactly once: a second charset or
	// a second title is the other half of the same kind of damage.
	for _, once := range []string{"<!doctype html>", "<meta charset=", "<title>"} {
		if n := strings.Count(page, once); n != 1 {
			t.Errorf("%q appears %d time(s) in the page", once, n)
		}
	}
	// AND NO SCRIPT ESCAPED ITS BLOCK. Every line of the page's JavaScript lives inside
	// the one script element; a statement outside it is either dead or visible, and a
	// reader cannot tell which from looking at the window.
	body := page[strings.Index(page, "<script>"):]
	if strings.Contains(page[:strings.Index(page, "<script>")], "textContent") {
		t.Error("there is JavaScript above the script element")
	}
	if !strings.Contains(body, "</script>") {
		t.Error("the script element is never closed")
	}
	// AND NOTHING THAT IS NOT TEXT IS IN IT.
	//
	// A NUL byte renders as nothing and makes every tool treat the page as
	// binary: grep stops quoting its matches, a diff stops showing them, and a
	// reviewer reads a file their tools have given up on. It arrives the same way
	// every time — a separator meant as the four characters \x00 written by a
	// patch that interpreted the escape, so the page got the character instead.
	// It has happened three times in one sitting, which is what this is for.
	if n := strings.Count(page, "\x00"); n != 0 {
		t.Errorf("the page holds %d NUL byte(s), so every tool that reads it treats it as binary", n)
	}
}

// AND EVERY METHOD THE PAGE CALLS EXISTS ON THE BACKEND.
//
// # Why this is a test and not a compile error
//
// Because there is no compiler between these two files. The page reaches the backend through
// `window.go.main.Acts`, which Wails fills in at run time from whatever `*Acts` exports, so a
// name that does not match is not a build failure — it is `undefined is not a function`, thrown
// inside a `.then` chain nobody is watching, in a window that otherwise looks finished. The
// button does nothing and says nothing about why.
//
// That is the shape of defect this repository has recorded eight times: complete code on both
// sides and no call between them. A rename on the Go side, or one character out in the page, is
// invisible to `go build ./...`, to `go test ./...`, and to a source review — the two spellings
// are never on the same screen.
func TestEveryMethodThePageCallsIsOnTheBackend(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	surface := reflect.TypeOf(&Acts{})
	for i := 0; i < surface.NumMethod(); i++ {
		have[surface.Method(i).Name] = true
	}
	// `acts.Name(` and `acts.Name)` both count: the page passes some of these as
	// values (see askThenDelete), which is a call site like any other.
	calls := regexp.MustCompile(`\bacts\.([A-Z]\w*)`).FindAllStringSubmatch(string(raw), -1)
	if len(calls) == 0 {
		t.Fatal("no backend calls were found in the page, so this proves nothing " +
			"— the pattern that finds them has stopped matching")
	}
	seen := map[string]bool{}
	for _, c := range calls {
		name := c[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		if !have[name] {
			t.Errorf("the page calls acts.%s, and *Acts has no such method. "+
				"Wails binds whatever *Acts exports, so this is not a "+
				"build failure: the button throws and says nothing", name)
		}
	}
	// AND THE FOUR THE RECORDING FLOW NEEDS ARE AMONG THEM, by name.
	//
	// Stated separately because the loop above only checks the calls that ARE there. A
	// page that never reached one of these would pass it, and a Stop button that is not
	// wired to anything is the whole of what could go wrong here.
	for _, want := range []string{"StartRecording", "StopRecording",
		"CancelRecording", "SaveDraft", "Recording"} {
		if !seen[want] {
			t.Errorf("nothing in the page calls acts.%s, so that part of "+
				"record → Stop → review → Save has no button", want)
		}
	}
}
