package driver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	es := newStdinEventSource(in)
	if err := runtime.Serve(g, out, hosts, es); err != nil {
		return decorateError(err, path, string(src))
	}
	return nil
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
