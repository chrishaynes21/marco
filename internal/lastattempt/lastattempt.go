// Package lastattempt is what became of the latest command somebody asked Marco for, kept so that
// "why didn't that work?" has an answer.
//
// # What was missing
//
// A routine that RAN leaves a run record (cmd/marco routinerecord.go), and Studio's History reads
// it. A command that was refused BEFORE it ran — a value missing, the routine saved again since it
// was read back, another application in front, nothing answering to the words — printed its
// sentence once, into a console or a status line that had gone by the time anybody asked, and
// left nothing. Those are the attempts a person most often wants explained, because nothing
// visible happened. The run records do not take them on purpose: a refusal pressed nothing, so
// there is no run to cite (see performOnePlay). This is a different record for a different
// question — not "what did this run do" but "what happened to what I just asked for".
//
// # One record, the latest, and nothing anybody said
//
// It is written by the intake (cmd/marco runInvocation), which every entrance reaches, and it is
// REPLACED each time: the question is about the last thing, and a history of requests is exactly
// the store of what people say that Marco does not keep. So it carries the shape of the request
// (how many words, from which entrance), the routine it became when it became one, the outcome,
// the stage it ended at, the reason in Marco's words and the next step when one is known — and
// every one of those sentences is passed through Scrub with what was supplied, so a value that a
// refusal quoted does not reach the disk.
//
// # The answer is a lookup
//
// Asks is a fixed set of phrasings, so the common question asks no model, and Says composes the
// answer from the record alone. See [[ADR-320-why-didnt-that-work]].
package lastattempt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// FileName is the record's name in the Marco home.
const FileName = "last-attempt.json"

// Stages: where an attempt ended. A stage is not an outcome — `refused` happens at several — and
// the next step depends on which.
const (
	StageMatching  = "matching"  // nothing answered to the words
	StageScope     = "scope"     // the routine may not run with what is in front
	StageWindow    = "window"    // the window in front could not be bound
	StageAuthority = "authority" // the authority declined it
	StageVersion   = "version"   // the routine changed since it was confirmed or pressed
	StageValues    = "values"    // a value it needs was missing, or offered where it may not be
	StageReading   = "reading"   // the routine could not be read
	StageRunning   = "running"   // it ran, and this is how it ended
	StageDirector  = "director"  // Director took the request, and this is how it ended
)

// Attempt is the latest command, as it may be written down.
type Attempt struct {
	Version int    `json:"version"`
	At      string `json:"at"`
	// Source is the entrance — typed, spoken, hotkey, cli — and Words how long the request
	// was. Never the words themselves.
	Source string `json:"source,omitempty"`
	Words  int    `json:"words"`
	// Routine and Application are the routine it became, when it became one: an identity
	// out of the library, not something said.
	Routine     string `json:"routine,omitempty"`
	Application string `json:"application,omitempty"`
	Outcome     string `json:"outcome"`
	Stage       string `json:"stage"`
	Because     string `json:"because,omitempty"`
	Next        string `json:"next,omitempty"`
	// Run is the run record's reference, when the attempt got far enough to claim one.
	Run string `json:"run,omitempty"`
}

// Path is where the record is, in a Marco home.
func Path(home string) string { return filepath.Join(home, FileName) }

// Write replaces the record, whole: written beside it and renamed over it, so a reader never
// sees half of one.
func Write(home string, a Attempt) error {
	if a.Version == 0 {
		a.Version = 1
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(home, FileName+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), Path(home)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Read is the record, or ok=false when there has not been an attempt yet.
func Read(home string) (Attempt, bool, error) {
	raw, err := os.ReadFile(Path(home))
	if os.IsNotExist(err) {
		return Attempt{}, false, nil
	}
	if err != nil {
		return Attempt{}, false, err
	}
	var a Attempt
	if err := json.Unmarshal(raw, &a); err != nil {
		return Attempt{}, false, fmt.Errorf("%s cannot be read: %w", Path(home), err)
	}
	return a, true, nil
}

// Withheld is what a supplied value reads as in the record.
const Withheld = "‹what you said›"

// Scrub takes every whole-word occurrence of anything supplied out of a sentence, ignoring case,
// longest first so a phrase goes before a value inside it.
func Scrub(s string, said []string) string {
	var keep []string
	for _, w := range said {
		if w = strings.TrimSpace(w); w != "" {
			keep = append(keep, w)
		}
	}
	sort.Slice(keep, func(i, j int) bool { return len(keep[i]) > len(keep[j]) })
	for _, w := range keep {
		s = replaceWord(s, w, Withheld)
	}
	return s
}

func replaceWord(s, word, with string) string {
	low, lw := strings.ToLower(s), strings.ToLower(word)
	if len(low) != len(s) || len(lw) != len(word) {
		low, lw = s, word // lowering changed a length: match exactly instead
	}
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(low[i:], lw)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		j += i
		end := j + len(lw)
		if edge(s, j, true) && edge(s, end, false) {
			b.WriteString(s[i:j])
			b.WriteString(with)
		} else {
			b.WriteString(s[i:end])
		}
		i = end
	}
}

func edge(s string, at int, before bool) bool {
	var r rune
	switch {
	case before && at == 0, !before && at >= len(s):
		return true
	case before:
		r, _ = utf8.DecodeLastRuneInString(s[:at])
	default:
		r, _ = utf8.DecodeRuneInString(s[at:])
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// questions are the phrasings of "why didn't that work?", normalised by normal. A fixed set, so
// the common case asks no model; anything else is an ordinary request.
var questions = map[string]bool{
	"why":                       true,
	"why not":                   true,
	"what happened":             true,
	"what went wrong":           true,
	"why didnt that work":       true,
	"why didnt it work":         true,
	"why did that not work":     true,
	"why did it not work":       true,
	"why did that fail":         true,
	"why did it fail":           true,
	"why didnt that run":        true,
	"why didnt it run":          true,
	"why didnt anything happen": true,
	"why did nothing happen":    true,
	"what was wrong":            true,
	"what was the problem":      true,
}

// Asks reports whether a phrase is somebody asking why the last command did not work. With or
// without the wake word's comma, a question mark, an apostrophe, or "marco" in front.
func Asks(phrase string) bool {
	return questions[normal(phrase)]
}

func normal(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("’", "", "'", "", "?", " ", ",", " ", ".", " ", "!", " ").Replace(s)
	f := strings.Fields(s)
	if len(f) > 1 && f[0] == "marco" {
		f = f[1:]
	}
	return strings.Join(f, " ")
}

// Says is the answer, in a person's words: what it was, what became of it and when, why, and what
// to do next when that is known. Composed from the record alone.
func (a Attempt) Says(now time.Time) []string {
	what := "your last command"
	if a.Routine != "" {
		what = "“" + a.Routine + "”"
	}
	when := ""
	if at, err := time.Parse(time.RFC3339, a.At); err == nil {
		when = " (" + ago(now.Sub(at)) + ")"
	}
	var head string
	switch a.Outcome {
	case "performed":
		head = capital(what) + " ran" + when + "."
	case "refused":
		head = capital(what) + " was refused before anything ran" + when + "."
		if a.Stage == StageDirector {
			head = capital(what) + " was refused" + when + "."
		}
	case "unavailable":
		head = "Nothing took " + what + when + "."
	case "clarify":
		head = "Marco asked a question about " + what + " and it was not answered" + when + "."
	case "cancelled":
		head = capital(what) + " was stopped" + when + "."
	case "unverified":
		head = capital(what) + " ran, and Marco could not confirm it worked" + when + "."
	case "failed":
		head = capital(what) + " started and failed" + when + "."
	default:
		head = capital(what) + " ended as " + a.Outcome + when + "."
	}
	out := []string{head}
	if a.Because != "" {
		out = append(out, "Why: "+a.Because)
	} else if a.Outcome == "performed" {
		out = append(out, "It did what it was saved to do; if that was not what you wanted, "+
			"the routine itself needs changing.")
	}
	if a.Next != "" {
		out = append(out, "Next: "+a.Next)
	}
	if a.Run != "" {
		out = append(out, "Its run record: "+a.Run)
	}
	return out
}

func capital(s string) string {
	if s == "" || strings.HasPrefix(s, "“") {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

func ago(d time.Duration) string {
	switch {
	case d < 0:
		return "just now"
	case d < 10*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	case d < 2*time.Minute:
		return "a minute ago"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "an hour ago"
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
