package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// `director light` — watch what Marco's Accessibility perception currently understands.
//
// # Why this exists, next to `director sight`
//
// `sight` answers one question once: what is Marco referring to right now. This answers a
// different one, continuously: what does Marco's LIGHT MODE brain think is happening, while
// you use the computer.
//
// It was built because the alternative was worse. Hardening Learn against real software
// meant a long sequence of runs where a person performed an action, it failed, and the only
// way to find out why was a CLI archaeology round afterwards — several of which chased the
// wrong layer because the number that mattered was not on any surface. A live reading of the
// same canonical account costs nothing and answers most of those questions before they are
// asked.
//
// # It is a READ, and structurally cannot be anything else
//
// The whole of it is `Client.Playbill`, which starts no session, takes no sample, answers no
// question, writes no memory and carries no authority. Refreshing it faster changes nothing
// about what Marco believes — which is the property that lets a person leave it running
// beside the application they are teaching.
//
// # One account, three readings
//
// Nothing here composes its own picture of the Director. It renders `playbill.View` through
// the SAME `Normal`, `Watch` and `Deep` readings the overlay uses, so a surface and a
// terminal cannot come to different conclusions about what Marco is doing. The only thing
// this file decides is how often to ask.
func runLight(args []string) int {
	fs := flag.NewFlagSet("light", flag.ExitOnError)
	every := fs.Duration("every", time.Second, "how often to refresh")
	once := fs.Bool("once", false, "print one reading and stop")
	debug := fs.Bool("debug", false, "add the evidence behind each answer")
	jsonOut := fs.Bool("json", false, "print the raw account as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	if *every < 250*time.Millisecond {
		// A floor rather than an error. The reading is cheap, but a surface spinning
		// faster than the Director samples is showing the same answer repeatedly and
		// charging the machine for it — and this milestone is about Light Mode being
		// something a person can leave running.
		*every = 250 * time.Millisecond
	}

	last := ""
	for {
		resp, err := client.Playbill(service.PlaybillPayload{Diagnostics: *debug})
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		if *jsonOut {
			return printJSON(resp.View)
		}
		out := renderLight(resp.View, *debug)
		// Only when it CHANGED. A terminal that reprints an identical screen every
		// second is one nobody can read a change out of — which is the entire job.
		if out != last {
			fmt.Print(out)
			last = out
		}
		if *once {
			return 0
		}
		time.Sleep(*every)
	}
}

// renderLight is the whole reading: the headline, then the panel.
//
// It renders the shared `playbill` readings and adds nothing of its own. A line that appears
// here appears identically in the overlay, because both ask the same value the same question.
func renderLight(v playbill.View, debug bool) string {
	var b strings.Builder
	b.WriteString("\n")
	h := v.Normal()
	b.WriteString(h.Word)
	if h.Detail != "" {
		b.WriteString(" — " + h.Detail)
	}
	b.WriteString("\n\n")

	lines := v.Watch()
	if debug {
		lines = v.Deep()
	}
	for _, l := range lines {
		b.WriteString(renderLine(l))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// renderLine turns one playbill line into terminal text.
//
// Indentation carries the structure the Kind already decided; this adds no hierarchy of its
// own, so a reordering upstream cannot leave the indentation describing the old shape.
func renderLine(l playbill.Line) string {
	switch {
	case l.Text == "":
		return ""
	case l.Head:
		return "  " + l.Text
	}
	return strings.Repeat("  ", l.Indent+2) + l.Text
}
