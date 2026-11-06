package main

// WHAT SOMEBODY SAID OR TYPED IS NEVER WRITTEN TO THE WIDGET'S LOG.
//
// # Why
//
// A command can carry a value somebody supplied — "cloud pull stone", a search term, a name — and
// once voice is on, everything the microphone hears on the way to a command is a candidate for one.
// The widget's log is its stderr: the overlay.cmd console, or whatever a launcher points it at,
// and either outlives the moment. The HUD shows what was heard because the person is looking at
// it; the log is for finding out later what HAPPENED, and that needs the shape of the request,
// the identity of what ran and how it ended — never the words.
//
// So the log carries `heard(phrase)` (how many words), `argsShown(args)` (the flags that say WHAT
// runs, kept; every bare word of the request and every value replaced), and a child's own output
// is echoed through `redactionFor(args)`, because `marco do` and the Director quote the request
// back in exactly the lines a person reads when something went wrong — the error path.
//
// Held by TestNothingHeardReachesTheLog (voice_test.go), which drives a spoken line with a unique
// value through the intake, the read-back, an overlay verb, a learn and the child's error echo.

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// heard is a phrase as a log line may carry it: how long it was, never what it said.
func heard(s string) string {
	return fmt.Sprintf("‹%d words›", len(strings.Fields(s)))
}

// argsShown is a child's argument list as a log line may carry it. The subcommand and every flag
// that names what runs (--play=, --app=, --focus, --source=, --expect=) are kept; `--arg=k=v`
// keeps its name and loses its value; every bare word — the request itself — is replaced.
func argsShown(args []string) string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		switch {
		case i == 0:
			out = append(out, a)
		case strings.HasPrefix(a, "--arg="):
			name, _, _ := strings.Cut(strings.TrimPrefix(a, "--arg="), "=")
			out = append(out, "--arg="+name+"=…")
		case strings.HasPrefix(a, "-"):
			out = append(out, a)
		default:
			out = append(out, heard(a))
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}

// redaction removes a request's own words, and the values supplied with it, from a line that a
// child printed about it.
type redaction struct{ said []string }

// redactionFor is the redaction for one child: its bare words (the phrase, a name) and its
// `--arg` values, longest first so a phrase goes before the value inside it.
func redactionFor(args []string) redaction {
	var said []string
	for i, a := range args {
		switch {
		case i == 0:
		case strings.HasPrefix(a, "--arg="):
			if _, v, ok := strings.Cut(strings.TrimPrefix(a, "--arg="), "="); ok {
				said = append(said, v)
			}
		case strings.HasPrefix(a, "-"):
		default:
			said = append(said, a)
		}
	}
	var keep []string
	for _, s := range said {
		if s = strings.TrimSpace(s); s != "" {
			keep = append(keep, s)
		}
	}
	sort.Slice(keep, func(i, j int) bool { return len(keep[i]) > len(keep[j]) })
	return redaction{said: keep}
}

// apply replaces every whole-word occurrence of what was said, ignoring case.
func (r redaction) apply(line string) string {
	for _, s := range r.said {
		line = replaceWord(line, s, heard(s))
	}
	return line
}

// replaceWord replaces each occurrence of word in s that is not part of a longer word.
func replaceWord(s, word, with string) string {
	low, lw := strings.ToLower(s), strings.ToLower(word)
	if lw == "" || len(low) != len(s) {
		// Lowering changed the length (a rare non-ASCII case): match exactly instead.
		low, lw = s, word
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
		if wordEdge(s, j, true) && wordEdge(s, end, false) {
			b.WriteString(s[i:j])
			b.WriteString(with)
		} else {
			b.WriteString(s[i:end])
		}
		i = end
	}
}

// wordEdge is true when position at in s is not inside a word: the rune before it (when
// before) or at it is not a letter or a digit.
func wordEdge(s string, at int, before bool) bool {
	var r rune
	if before {
		if at == 0 {
			return true
		}
		r, _ = utf8.DecodeLastRuneInString(s[:at])
	} else {
		if at >= len(s) {
			return true
		}
		r, _ = utf8.DecodeRuneInString(s[at:])
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// narrationQuiet is the redaction for a narrate-learn session, which learns what was said one
// phrase at a time: each phrase, and what follows its first word ("type <text>" echoes as the
// text), is added as it is forwarded to the child.
var narrationQuiet growingRedaction

type growingRedaction struct {
	mu   sync.Mutex
	said []string
}

func (g *growingRedaction) reset(first ...string) {
	g.mu.Lock()
	g.said = nil
	g.mu.Unlock()
	for _, s := range first {
		g.add(s)
	}
}

func (g *growingRedaction) add(phrase string) {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.said = append(g.said, phrase)
	if f := strings.Fields(phrase); len(f) > 1 {
		g.said = append(g.said, strings.Join(f[1:], " "))
	}
}

func (g *growingRedaction) apply(line string) string {
	g.mu.Lock()
	said := append([]string(nil), g.said...)
	g.mu.Unlock()
	sort.Slice(said, func(i, j int) bool { return len(said[i]) > len(said[j]) })
	return redaction{said: said}.apply(line)
}

// namesAPlay is true when a child was told which routine to run by identity, so the name it was
// tracked under is the library's, not something said.
func namesAPlay(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--play=") {
			return true
		}
	}
	return false
}
