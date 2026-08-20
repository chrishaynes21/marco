package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director sight` — show me what you're seeing.
//
// # What this is for
//
// Not an accessibility-tree viewer. A person asking "what does Marco see?" is not asking for a
// dump of every node an application exposes; they are asking whether Marco has understood where
// they are and what it is paying attention to. A tree renderer answers a question nobody has, and
// answering it convincingly is worse than not answering — it makes an implementation detail look
// like comprehension.
//
// So this shows four things, and only where each genuinely exists: which application is being
// watched, what Marco can perceive with, what it takes the current place to be, and what it is
// currently referring to. Anything Marco has not worked out is said plainly rather than left blank.
//
// # It changes nothing
//
// A read. No session is started, no proposal is answered, no memory is written, nothing is
// recognised as a result of looking. Inspecting what Marco believes must not alter what Marco
// believes, or the inspection is measuring itself.
//
// Deleting the read-only guarantee must fail TestLookingAtSightChangesNothing.

// runSight is `director sight`.
func runSight(args []string) int {
	fs := flag.NewFlagSet("sight", flag.ExitOnError)
	show := fs.Bool("show", false, "also point at it on screen")
	hold := fs.Duration("hold", 20*time.Second, "how long the highlight stays on screen")
	jsonOut := fs.Bool("json", false, "print as JSON")
	deep := fs.Bool("deep", false, "include provenance and the reasons behind each answer")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	raw, err := client.Observation(service.ObserveQuery{Point: &service.ObservePoint{}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	var view pointView
	if err := json.Unmarshal(raw, &view); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		fmt.Println(string(raw))
	} else {
		fmt.Print(renderSight(view, *deep))
	}
	if *show && view.CanPoint() {
		if err := showHighlight(view, *hold); err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
	}
	return 0
}

// renderSight is the calm reading. Deep adds the provenance behind each line.
//
// The ordinary view names no window handle, no ephemeral id, no subject reference and no
// fingerprint. If somebody needs one of those to understand what Marco is looking at, the surface
// has failed and the answer is to fix the surface rather than to print the identifier.
func renderSight(v pointView, deep bool) string {
	var b strings.Builder
	b.WriteString("\nWhat I'm seeing\n\n")

	b.WriteString("  watching      " + orUnknownApp(v.Application) + "\n")

	b.WriteString("  perceiving    " + sourceLine(v) + "\n")
	if deep {
		for _, s := range v.Sources {
			if !s.On && s.Reason != "" {
				b.WriteString("                  " + s.Name + " is off: " + s.Reason + "\n")
			}
		}
	}

	b.WriteString("  the place     " + orElse(v.Place, "I haven't settled on what this screen is") + "\n")

	// WHAT MARCO CAN ACT ON HERE. Only when it knows of something: "nothing" printed every
	// time would train a person to stop reading the line, and the empty case is the ordinary
	// one. See [[ADR-068-the-theater-is-the-durable-semantic-world]].
	if len(v.Targets) > 0 {
		b.WriteString("  I can act on  " + v.Targets[0] + "\n")
		for _, t := range v.Targets[1:] {
			b.WriteString("                " + t + "\n")
		}
	}

	// WHAT MARCO LAST DID. The other half of "what are you seeing" — a person almost always
	// asks just after watching it do something.
	if v.LastAction != "" {
		line := v.LastAction
		if v.LastActionWhen != "" {
			line += " — " + v.LastActionWhen
		}
		b.WriteString("  I last did    " + line + "\n")
	}

	if v.Source != "" && v.Source != "none" {
		// What is AVAILABLE and what this referent was actually read FROM are different
		// facts. A surface showing only the first lets "ocr on" be read as "Marco looked at
		// the pixels", which is the impression this must never leave.
		b.WriteString("  grounding on  " + v.Source + "\n")
	}

	b.WriteString("\n  referring to  ")
	switch {
	case v.CanPoint():
		b.WriteString(v.About + "\n")
		b.WriteString("                  " + v.Say + "\n")
	default:
		b.WriteString(v.Say + "\n")
	}
	if v.Question != "" {
		b.WriteString("\n  the question  " + v.Question + "\n")
	} else if v.Interpretation != "" {
		b.WriteString("\n  what I think  " + v.Interpretation + "\n")
	}

	if deep {
		b.WriteString("\n  provenance\n")
		b.WriteString("    geometry read from " + orElse(v.Source, "nothing") + "\n")
		if v.SourceWhy != "" {
			b.WriteString("    " + v.SourceWhy + "\n")
		}
		if v.Unavailable != "" {
			b.WriteString("    cannot point: " + v.Unavailable + "\n")
		}
		if v.Unmappable != "" {
			b.WriteString("    cannot place: " + v.Unmappable + "\n")
		}
		if v.Session != "" {
			b.WriteString(fmt.Sprintf("    from %s, sample %d\n", v.Session, v.Sequence))
		}
	}
	if v.CanPoint() {
		b.WriteString("\n  director sight --show   to point at it\n")
	}
	return b.String()
}

// sourceLine is the honest one-line answer to "what can you see with".
//
// Named sources with their real state, never a summary word. "Perceiving: yes" would be true of a
// Director reading an accessibility tree and of one reading pixels, and those are different claims
// about how much to trust a highlight.
func sourceLine(v pointView) string {
	if len(v.Sources) == 0 {
		return "unknown"
	}
	parts := make([]string, 0, len(v.Sources))
	for _, s := range v.Sources {
		state := "off"
		if s.On {
			state = "on"
		}
		parts = append(parts, s.Name+" "+state)
	}
	return strings.Join(parts, ", ")
}

func orElse(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
