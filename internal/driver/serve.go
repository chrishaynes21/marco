package driver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// ServeFile runs path in persistent server mode (see spec/Hosts.md): the script
// body is startup, then the program stays alive delivering events read from `in`
// to its feeds until EOF. Each input line is a JSON object:
//
//	{"feed":"Hotkeys","event":"Leader"}
//	{"feed":"Hotkeys","event":"Stop"}
//	{"feed":"Chat","event":"Message","data":"hello"}
//
// This lets any external process — the AHK UI, a hotkey daemon, a test harness —
// drive a running Marco program by piping events to stdin.
func ServeFile(path string, in io.Reader, out io.Writer, hosts map[string]runtime.Host) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	g, err := buildGraph(string(src), dir, map[string]bool{})
	if err != nil {
		return decorateError(err, path, string(src))
	}
	if err := compile.Compile(g, nil); err != nil {
		return decorateError(fmt.Errorf("compile: %w", err), path, string(src))
	}
	es := mergeSources(in, hosts)
	err = runtime.Serve(g, out, hosts, es)
	closeBridges(hosts) // reap bridge subprocesses so none linger after the run
	if err != nil {
		return decorateError(err, path, string(src))
	}
	return nil
}

// mergeSources fans stdin and every bridge host's event stream into one source.
// A bridge host (an event-pushing subprocess, e.g. the overlay HUD) implements
// runtime.EventSource; merging lets it drive the served program just like stdin.
//
// Shutdown rule: when any BRIDGE source closes (e.g. the HUD window is closed,
// so the overlay process exits and its stdout hits EOF), the whole run ends —
// the session is built around those processes, so losing one tears it down.
// With no bridge sources present, behaviour is unchanged: the run ends when
// stdin closes (the classic `hotkeys | marco serve` pipe).
func mergeSources(in io.Reader, hosts map[string]runtime.Host) runtime.EventSource {
	var bridges []<-chan runtime.Event
	seen := map[runtime.EventSource]bool{}
	for _, h := range hosts {
		if es, ok := h.(runtime.EventSource); ok && !seen[es] {
			seen[es] = true
			bridges = append(bridges, es.Events())
		}
	}
	stdin := newStdinEventSource(in).Events()

	out := make(chan runtime.Event)
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	var wg sync.WaitGroup
	forward := func(in <-chan runtime.Event, endsRun bool) {
		defer wg.Done()
		for {
			select {
			case ev, ok := <-in:
				if !ok {
					if endsRun {
						closeDone()
					}
					return
				}
				select {
				case out <- ev:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}
	for _, b := range bridges {
		wg.Add(1)
		go forward(b, true)
	}
	// stdin ending only tears down the run when there are no bridge lifelines.
	wg.Add(1)
	go forward(stdin, len(bridges) == 0)

	go func() { wg.Wait(); close(out) }()
	return &mergedSource{ch: out}
}

type mergedSource struct{ ch chan runtime.Event }

func (m *mergedSource) Events() <-chan runtime.Event { return m.ch }

// closeBridges kills any bridge subprocesses registered as hosts.
func closeBridges(hosts map[string]runtime.Host) {
	closed := map[io.Closer]bool{}
	for _, h := range hosts {
		if c, ok := h.(io.Closer); ok && !closed[c] {
			closed[c] = true
			_ = c.Close()
		}
	}
}

// stdinEventSource decodes JSON event lines from a reader into runtime.Events.
type stdinEventSource struct{ ch chan runtime.Event }

type wireEvent struct {
	Feed  string `json:"feed"`
	Event string `json:"event"`
	Data  any    `json:"data"`
}

func newStdinEventSource(in io.Reader) *stdinEventSource {
	s := &stdinEventSource{ch: make(chan runtime.Event)}
	go func() {
		defer close(s.ch)
		sc := bufio.NewScanner(in)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var we wireEvent
			if err := json.Unmarshal(line, &we); err != nil || we.Feed == "" {
				continue
			}
			s.ch <- runtime.Event{
				Feed:    we.Feed,
				Message: we.Event,
				Payload: runtime.ValueFromJSON(we.Data),
			}
		}
	}()
	return s
}

func (s *stdinEventSource) Events() <-chan runtime.Event { return s.ch }
