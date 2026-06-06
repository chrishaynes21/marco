// Package orchestrator implements the named-route teach loop: run a route by
// name if it exists, otherwise ask the user to demonstrate it, record the
// demonstration, simplify it, generate a Marco route, save it, and run it — so
// next time the same name just runs. OS-agnostic; the OS-specific work is the
// injected recorder and host.
package orchestrator

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
)

// Deps are the orchestrator's collaborators (injectable for testing).
type Deps struct {
	Reg   routes.Registry
	Rec   recorder.Recorder
	Hosts map[string]runtime.Host // route execution hosts (e.g. {"*": oshost.New()})
	In    io.Reader               // prompts / confirmations
	Out   io.Writer               // user-facing output
}

// Do runs the named route if known; otherwise teaches it, then runs it.
func (d Deps) Do(name string) error {
	if d.Reg.Has(name) {
		return d.run(name)
	}
	fmt.Fprintf(d.Out, "I don't know %q yet.\n", name)
	if err := d.Teach(name); err != nil {
		return err
	}
	if d.Reg.Has(name) && d.confirm("Run it now? [Y/n] ") {
		return d.run(name)
	}
	return nil
}

// Teach records a demonstration of name, simplifies it into a route, and saves
// it. The user performs the actions and presses Esc to finish.
func (d Deps) Teach(name string) error {
	fmt.Fprintf(d.Out, "Show me how to %q. Do it now, then press Esc when finished.\n", name)
	if err := d.Rec.Start(); err != nil {
		return fmt.Errorf("can't record on this platform: %w", err)
	}
	waitForStop(d.Rec)
	events := stripTrailingStop(d.Rec.Stop())

	steps := simplify.Simplify(events, simplify.DefaultOptions())
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	src, err := codegen.Route(name, steps)
	if err != nil {
		return err
	}
	if err := d.Reg.Save(name, src); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Learned %q → %s (%d steps)\n", name, d.Reg.Path(name), len(steps))
	fmt.Fprintf(d.Out, "--- route ---\n%s\n", src)
	return nil
}

func (d Deps) run(name string) error {
	return driver.RunFileWithHosts(d.Reg.Path(name), d.Out, d.Hosts)
}

// waitForStop blocks until the stop key (Esc) is seen on the recorder's event
// stream, or the stream closes.
func waitForStop(rec recorder.Recorder) {
	ch := rec.Events()
	if ch == nil {
		return
	}
	for ev := range ch {
		if ev.Kind == recorder.EvKey && ev.Down && strings.EqualFold(ev.KeyName, "esc") {
			return
		}
	}
}

// stripTrailingStop removes the Esc keypress (and any trailing moves) that the
// stop gesture leaves at the end of the recording.
func stripTrailingStop(events []recorder.RecordedEvent) []recorder.RecordedEvent {
	end := len(events)
	for end > 0 {
		e := events[end-1]
		isStop := e.Kind == recorder.EvKey && strings.EqualFold(e.KeyName, "esc")
		isMove := e.Kind == recorder.EvMove
		if isStop || isMove {
			end--
			continue
		}
		break
	}
	return events[:end]
}

// confirm reads a line from In and reports whether it is an affirmative
// (empty/"y"/"yes"). Exposed for CLI flows that want an explicit prompt.
func (d Deps) confirm(prompt string) bool {
	fmt.Fprint(d.Out, prompt)
	if d.In == nil {
		return false
	}
	line, _ := bufio.NewReader(d.In).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}
