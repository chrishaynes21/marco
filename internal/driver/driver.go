package driver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/graph"
	"github.com/chaynes-simpleclouds/marco/internal/lexer"
	"github.com/chaynes-simpleclouds/marco/internal/parser"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Diagnostic is the wire form of a compiler message. Suitable for editor /
// LSP integration via `marco check --json`.
type Diagnostic struct {
	Severity string `json:"severity"`        // "error" | "warning" | "info"
	Code     string `json:"code,omitempty"`  // short stable id (e.g. "dead-arm")
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"` // the offending source line
}

// Check parses, builds, and compiles path. On success returns nil. On failure
// returns a non-nil error and writes diagnostics to out.
//
// When jsonMode is true, diagnostics are emitted as a JSON array (one entry
// per error) regardless of success or failure — empty array on success. When
// false, the function emits the same human-readable text as RunFile would.
func Check(path string, out io.Writer, jsonMode bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	g, buildErr := buildGraph(string(src), dir, map[string]bool{})
	var compileErr error
	if buildErr == nil {
		compileErr = compile.Compile(g, nil)
	}
	combined := buildErr
	if combined == nil {
		combined = compileErr
	}
	if jsonMode {
		diags := []Diagnostic{}
		for _, d := range diagnosticsFromErr(combined, path) {
			if d.Line > 0 {
				if line := nthLine(string(src), d.Line); line != "" {
					d.Source = line
				}
			}
			diags = append(diags, d)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(diags); err != nil {
			return err
		}
		if combined != nil {
			return combined
		}
		return nil
	}
	if combined != nil {
		// In multi-error mode, render each diagnostic with its own source
		// preview block so editors / humans can scroll top-down.
		var multi compile.Errors
		if errors.As(combined, &multi) && len(multi) > 1 {
			for _, ce := range multi {
				header := ce.Error()
				if path != "" {
					header = path + ": " + header
				}
				fmt.Fprintln(out, header)
				if len(ce.Pos.Parts) > 0 {
					if pv := previewLine(string(src), ce.Pos.Pos.Line, ce.Pos.Pos.Col); pv != "" {
						fmt.Fprintln(out, pv)
					}
				}
			}
			return combined
		}
		decorated := decorateError(combined, path, string(src))
		fmt.Fprintln(out, decorated.Error())
		return decorated
	}
	return nil
}

// errToDiagnostic extracts a structured Diagnostic from any error returned by
// the build / compile pipeline. Falls back to a generic shape when the error
// isn't a *compile.Error.
func errToDiagnostic(err error, file string) Diagnostic {
	d := Diagnostic{File: file, Severity: "error", Message: err.Error()}
	var ce *compile.Error
	if errors.As(err, &ce) {
		d.Severity = ce.Severity.String()
		d.Code = ce.Code
		d.Message = ce.Msg
		if len(ce.Pos.Parts) > 0 {
			d.Line = ce.Pos.Pos.Line
			d.Column = ce.Pos.Pos.Col
		}
	}
	return d
}

// diagnosticsFromErr expands a compile.Errors aggregate into one Diagnostic per
// underlying *compile.Error. Non-aggregate errors yield a single Diagnostic;
// nil yields nothing.
func diagnosticsFromErr(err error, file string) []Diagnostic {
	if err == nil {
		return nil
	}
	var multi compile.Errors
	if errors.As(err, &multi) {
		out := make([]Diagnostic, 0, len(multi))
		for _, ce := range multi {
			out = append(out, errToDiagnostic(ce, file))
		}
		return out
	}
	return []Diagnostic{errToDiagnostic(err, file)}
}

func RunFile(path string, out io.Writer) error {
	return RunFileWithHosts(path, out, nil)
}

// RunFileWithHosts runs path with explicit foreign-act host providers (see
// spec/Hosts.md). A nil map uses the dryrun host for every foreign act. The
// special key "*" sets the default host for any act not named in the map.
func RunFileWithHosts(path string, out io.Writer, hosts map[string]runtime.Host) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	if err := runWith(string(src), dir, out, hosts); err != nil {
		return decorateError(err, path, string(src))
	}
	return nil
}

// RunSource runs a source string with no import support (single-file).
func RunSource(src string, out io.Writer) error {
	if err := runWith(src, "", out, nil); err != nil {
		return decorateError(err, "", src)
	}
	return nil
}

// PrintContracts builds the file and prints the inferred contract for every
// action and translator: the effective-status set computed by the analyzer,
// alongside any explicitly adopted contract for comparison.
func PrintContracts(path string, out io.Writer) error {
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
	for _, name := range g.Order {
		n := g.Nodes[name]
		if n.Kind != graph.KindAction && n.Kind != graph.KindTranslator {
			continue
		}
		if n.InferredStatuses == nil {
			continue
		}
		fmt.Fprintf(out, "%s\n", n.Name)
		fmt.Fprintf(out, "  inferred: { %s }\n", strings.Join(n.InferredStatuses, ", "))
		if n.AdoptedContractName != "" {
			if c, ok := g.Nodes[n.AdoptedContractName]; ok && c.Kind == graph.KindContract {
				allowed := make([]string, 0, len(c.AllowedStatuses))
				for _, as := range c.AllowedStatuses {
					allowed = append(allowed, as.Name)
				}
				fmt.Fprintf(out, "  contract %s allows: { %s }\n", c.Name, strings.Join(allowed, ", "))
			}
		}
	}
	return nil
}

// RunTests builds the file as usual but runs every `test ...` declaration in
// isolation instead of the script. Each test gets a fresh runtime; pass/fail
// is reported on out. Returns a non-nil error iff at least one test failed.
func RunTests(path string, out io.Writer) error {
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
	if len(g.Tests) == 0 {
		fmt.Fprintln(out, "no tests")
		return nil
	}
	fails := 0
	for _, t := range g.Tests {
		var buf testBuffer
		restore := applyScopedMocks(t)
		err := runtime.RunTest(g, t, &buf)
		restore()
		if err != nil {
			fmt.Fprintf(out, "FAIL %s: %s\n", t.Name, err.Error())
			if buf.Len() > 0 {
				fmt.Fprintln(out, indent(buf.String(), "    "))
			}
			fails++
			continue
		}
		fmt.Fprintf(out, "PASS %s\n", t.Name)
		if buf.Len() > 0 {
			fmt.Fprintln(out, indent(buf.String(), "    "))
		}
	}
	if fails > 0 {
		return fmt.Errorf("%d test(s) failed", fails)
	}
	return nil
}

// applyScopedMocks installs each per-test inline mock by swapping the action's
// BodyBlock. Returns a closure that restores the originals — callers must
// invoke it (typically deferred) before running the next test, since the
// graph is shared across runs.
func applyScopedMocks(t *graph.Node) func() {
	if len(t.ScopedInlineMocks) == 0 {
		return func() {}
	}
	type snap struct {
		action *graph.Node
		body   *graph.Block
	}
	snaps := make([]snap, len(t.ScopedInlineMocks))
	for i, m := range t.ScopedInlineMocks {
		snaps[i] = snap{action: m.Action, body: m.Action.BodyBlock}
		m.Action.BodyBlock = m.Body
	}
	return func() {
		for _, s := range snaps {
			s.action.BodyBlock = s.body
		}
	}
}

// testBuffer is an io.Writer that accumulates output for per-test reporting.
type testBuffer struct{ data []byte }

func (b *testBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}
func (b *testBuffer) Len() int     { return len(b.data) }
func (b *testBuffer) String() string {
	return string(b.data)
}

func indent(s, prefix string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func runWith(src, dir string, out io.Writer, hosts map[string]runtime.Host) error {
	g, err := buildGraph(src, dir, map[string]bool{})
	if err != nil {
		return err
	}
	if err := compile.Compile(g, nil); err != nil {
		return fmt.Errorf("compile: %w", err)
	}
	return runtime.RunWithHosts(g, out, hosts)
}

// posRE matches the `<line>:<col>:` prefix that lex / parse / graph / compile
// errors carry. The prefix is appended on its own line followed by a context
// preview of the offending source line and a caret at the column.
var posRE = regexp.MustCompile(`(\d+):(\d+):`)

// decorateError adds a source-line preview when the error message embeds a
// `line:col:` position. Returns the original error wrapped so callers can
// still match on its message via Error().
func decorateError(err error, path, src string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	loc := posRE.FindStringSubmatchIndex(msg)
	if loc == nil {
		return err
	}
	line, _ := strconv.Atoi(msg[loc[2]:loc[3]])
	col, _ := strconv.Atoi(msg[loc[4]:loc[5]])
	preview := previewLine(src, line, col)
	if preview == "" {
		return err
	}
	header := msg
	if path != "" {
		header = path + ": " + msg
	}
	return fmt.Errorf("%s\n%s", header, preview)
}

// nthLine returns the 1-indexed line of src, trimmed of trailing whitespace.
func nthLine(src string, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(src, "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[line-1], " \r\t")
}

// previewLine returns the indicated source line plus a caret pointing at the
// column. Returns empty string if the line can't be located.
func previewLine(src string, line, col int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(src, "\n")
	if line > len(lines) {
		return ""
	}
	srcLine := strings.TrimRight(lines[line-1], "\r")
	caret := ""
	if col > 0 {
		caret = strings.Repeat(" ", col-1) + "^"
	}
	if caret == "" {
		return "    " + srcLine
	}
	return "    " + srcLine + "\n    " + caret
}

// buildGraph parses and builds src, then recursively loads any `use <Module>.`
// declarations from the same directory and merges their nodes into the graph.
// loaded tracks already-loaded modules to break cycles.
func buildGraph(src, dir string, loaded map[string]bool) (*graph.Graph, error) {
	tokens, err := lexer.Lex(src)
	if err != nil {
		return nil, fmt.Errorf("lex: %w", err)
	}
	tree, err := parser.Parse(tokens)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	g, err := graph.Build(tree)
	if err != nil {
		return nil, fmt.Errorf("graph: %w", err)
	}

	for _, imp := range g.Imports {
		if loaded[imp.Module] && len(imp.Symbols) == 0 {
			continue
		}
		loaded[imp.Module] = true
		if dir == "" {
			return nil, fmt.Errorf("`use %s.` requires a file-based source", imp.Module)
		}
		modPath := filepath.Join(dir, imp.Module+".marco")
		modSrc, err := os.ReadFile(modPath)
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", imp.Module, err)
		}
		modG, err := buildGraph(string(modSrc), dir, map[string]bool{})
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", imp.Module, err)
		}
		if err := mergeGraphs(g, modG, imp.Symbols); err != nil {
			return nil, fmt.Errorf("merge %s: %w", imp.Module, err)
		}
	}

	return g, nil
}

// mergeGraphs copies nodes and listeners from src into dst. If symbols is
// non-empty, only the named symbols (and their owned children) are merged.
func mergeGraphs(dst, src *graph.Graph, symbols []string) error {
	allow := map[string]bool{}
	if len(symbols) > 0 {
		for _, s := range symbols {
			allow[s] = true
		}
	}
	include := func(name string, n *graph.Node) bool {
		if len(symbols) == 0 {
			return true
		}
		if allow[name] {
			return true
		}
		// Owned actions (e.g. `Game's Save`) inherit visibility from their owner.
		if n.Owner != nil && allow[n.Owner.Name] {
			return true
		}
		return false
	}
	for _, name := range src.Order {
		n := src.Nodes[name]
		if n.Kind == graph.KindPrimitive {
			continue
		}
		if n.IsScript {
			return fmt.Errorf("module declares a script (only entry file may)")
		}
		if !include(name, n) {
			continue
		}
		if _, dup := dst.Nodes[name]; dup {
			return fmt.Errorf("duplicate symbol %q", name)
		}
		if err := dst.AddNode(n); err != nil {
			return err
		}
	}
	dst.Listeners = append(dst.Listeners, src.Listeners...)
	return nil
}
