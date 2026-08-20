package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director show-me` — Marco points at what it is talking about.
//
// # What this command IS
//
// The product gesture, end to end, with nothing for a person to look up first: watch what I'm
// using, work out what you're referring to, and show me where that is. No window id, no handle, no
// subject reference, no accessibility tree.
//
// # What it is NOT
//
// It is not a drawing command. It cannot be given a rectangle, and it has no way to make Marco
// appear to mean something — every coordinate it prints came back from the Director, which
// computed it from geometry the accessibility provider measured. If Marco is not referring to
// anything, this command says so and draws nothing, which is the outcome that keeps a highlight
// worth trusting.

// runShowMe is `director show-me`.
func runShowMe(args []string) int {
	fs := flag.NewFlagSet("show-me", flag.ExitOnError)
	watch := fs.Duration("watch", 8*time.Second,
		"how long to watch what you're doing before pointing")
	interval := fs.Duration("interval", 400*time.Millisecond, "gap between looks")
	hold := fs.Duration("hold", 25*time.Second, "how long the highlight stays on screen")
	jsonOut := fs.Bool("json", false, "print as JSON")
	subject := fs.String("subject", "",
		"show what a remembered judgement refers to, by its subject id (developer override)")
	question := fs.String("question", "",
		"show what one question is about, by its question id (developer override)")
	quiet := fs.Bool("no-highlight", false,
		"work out where it is and report, without drawing anything")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	// WATCH. No target: the Director resolves what the person is working in and pins it.
	if *watch > 0 && *subject == "" && *question == "" {
		started, err := client.Observe(service.ObservePayload{
			Duration: *watch, Interval: *interval,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		if !*jsonOut {
			fmt.Printf("Watching %s…\n", started.Target)
		}
		if !waitForObservation(client, started.ID, *watch+15*time.Second) {
			fmt.Fprintln(os.Stderr, "director: the session did not finish in time")
			return 1
		}
	}

	raw, err := client.Observation(service.ObserveQuery{
		Point: &service.ObservePoint{Subject: *subject, Question: *question},
	})
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
		fmt.Print(renderPoint(view))
	}
	if !view.CanPoint() || *quiet {
		return 0
	}
	if err := showHighlight(view, *hold); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	return 0
}

// waitForObservation blocks until the session is no longer running.
func waitForObservation(c *service.Client, id string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		raw, err := c.Observation(service.ObserveQuery{ID: id})
		if err == nil {
			var v struct {
				Active bool `json:"active"`
			}
			if json.Unmarshal(raw, &v) == nil && !v.Active {
				return true
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// renderPoint is what a person reads.
//
// Marco's own sentence first, always. The measurements below it are for somebody diagnosing a
// highlight that looked wrong, and they are deliberately below the fold: a user who has to read a
// rectangle to understand what Marco meant has not been shown what Marco meant.
func renderPoint(v pointView) string {
	var b strings.Builder
	b.WriteString("\n" + v.Say + "\n")
	switch {
	case v.Question != "":
		b.WriteString("\n  " + v.Question + "\n")
	case v.Interpretation != "":
		b.WriteString("\n  " + v.Interpretation + "\n")
	}
	if v.About != "" {
		b.WriteString("\n  pointing at " + v.About + " in " + orUnknownApp(v.Application) + "\n")
	}
	if !v.CanPoint() {
		return b.String()
	}
	b.WriteString(fmt.Sprintf("  read from %s, sample %d\n", v.Source, v.Sequence))
	b.WriteString("\n  where\n")
	for i, box := range v.Boxes {
		b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, box))
	}
	return b.String()
}

func orUnknownApp(s string) string {
	if s == "" {
		return "the application in front"
	}
	return s
}

// showHighlight hands the rectangles to the presentation surface and waits for it to finish.
//
// The surface is a SEPARATE PROCESS, and that is architectural rather than incidental: it draws
// with a graphics library the engine may not depend on, it must be able to die without taking the
// Director with it, and it marks its own window as Marco-owned so perception excludes it. Nothing
// semantic crosses this boundary — a role, a sentence and some rectangles.
func showHighlight(v pointView, hold time.Duration) error {
	cmd, err := highlightCommand(boxesOf(v), v.Say, v.Role, hold)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// highlightCommand builds the surface invocation without running it.
//
// Separate from the two ways of running it because they are genuinely different: `show-me` waits,
// because the highlight IS the command's output; a learn session must not, because it is in the
// middle of a conversation and a blocked follower would stop reading what Marco says next.
func highlightCommand(boxes []box, say, role string, hold time.Duration) (*exec.Cmd, error) {
	bin := groundBinary()
	if bin == "" {
		return nil, fmt.Errorf("the highlight surface (marco-show) is not built or not on PATH")
	}
	payload, err := json.Marshal(struct {
		Boxes   []box  `json:"boxes"`
		Say     string `json:"say"`
		Role    string `json:"role"`
		Seconds int    `json:"seconds"`
	}{boxes, say, role, int(hold.Seconds())})
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd, nil
}

// box is the wire shape the surface reads. Deliberately the same field names pkg/referent uses:
// the rectangles cross this boundary unchanged, and a second spelling would be a second chance to
// transpose them.
type box struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func boxesOf(v pointView) []box {
	out := make([]box, 0, len(v.Boxes))
	for _, b := range v.Boxes {
		out = append(out, box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height})
	}
	return out
}

// groundBinary finds the highlight surface: beside this executable, or on PATH.
func groundBinary() string {
	name := "marco-show"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if self, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(self), name)
		if _, err := os.Stat(beside); err == nil {
			return beside
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return ""
}
