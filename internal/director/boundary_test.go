package director

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These two tests are the mechanical enforcement of the Director's architectural
// boundary. Both rules are one careless import away from being broken, and once
// broken they are expensive to restore — by the time anyone notices, a dozen
// packages depend on the shortcut. Catching it at `go test` costs nothing.
//
// They parse imports with go/ast rather than shelling out to `go list`, so they run
// anywhere, need no toolchain subprocess, and see files that don't compile yet.

const modulePath = "github.com/chaynes-simpleclouds/marco"

// forbidden are the package prefixes internal/director must never import. Each is
// either a platform implementation or the Marco engine itself; the Director reaches
// all of them through pkg/directorapi interfaces, wired in cmd/director.
var forbidden = []struct{ prefix, why string }{
	{modulePath + "/internal/oshost", "the executor is reached through directorapi.Actuator"},
	{modulePath + "/internal/winctx", "windows are reached through directorapi.WindowProvider"},
	{modulePath + "/internal/screen", "the screen is reached through directorapi.ScreenProvider"},
	{modulePath + "/internal/recorder", "input events are reached through directorapi.EventSource"},
	{modulePath + "/internal/secrets", "credentials are typed by name through directorapi.Actuator"},
	{modulePath + "/internal/runtime", "the Marco runtime is behind the Actuator and MarcoRunner adapters"},
	{modulePath + "/internal/bridgehost", "providers are constructed in cmd/director, not here"},
	{modulePath + "/internal/platform", "platform implementations are wired in, never imported"},
	{modulePath + "/internal/driver", "Marco programs are run through directorapi.MarcoRunner, wired in cmd/director"},
	{modulePath + "/internal/orchestrator", "teach/do workflow is a Marco concern"},
	{modulePath + "/internal/macroir", "the Director plans semantic actions, not recorded steps"},
	{modulePath + "/internal/codegen", "route codegen is a teach concern; the Director lowers its own typed steps in internal/director/marcostep"},
	{modulePath + "/internal/simplify", "macro simplification is a Marco concern"},
	{modulePath + "/internal/dispatch", "route dispatch is a separate system"},
	// A capability pack is contributed KNOWLEDGE about one application. The Director
	// defines what a pack IS (internal/director/game) and must never know what any
	// particular one contains — the moment it does, "the Director remains game-agnostic"
	// becomes a claim about discipline rather than a property of the build.
	{modulePath + "/internal/gamepacks", "capability packs are registered in cmd/director, never imported"},
}

// TestDirectorImportsNoPlatformCode walks every package under internal/director and
// fails on any import of a platform or engine implementation.
func TestDirectorImportsNoPlatformCode(t *testing.T) {
	checked := 0
	forEachImport(t, ".", func(file, imported string) {
		checked++
		for _, f := range forbidden {
			if imported == f.prefix || strings.HasPrefix(imported, f.prefix+"/") {
				t.Errorf("%s imports %s\n"+
					"  The Director must not depend on platform or engine code: %s.\n"+
					"  Add what you need to pkg/directorapi and wire the implementation in cmd/director.",
					file, imported, f.why)
			}
		}
	})
	t.Logf("checked %d imports under internal/director", checked)
}

// TestDirectorAPIIsStdlibOnly guards the other half of the contract. Marco's engine
// has zero external dependencies and that is load-bearing; a public contract that
// pulled in a dependency would leak it into every package that speaks the contract,
// including plugins and any future out-of-process Director.
func TestDirectorAPIIsStdlibOnly(t *testing.T) {
	apiDir := filepath.Join("..", "..", "pkg", "directorapi")
	found := false
	forEachImport(t, apiDir, func(file, imported string) {
		found = true
		// Standard library paths have no dot in their first segment.
		first, _, _ := strings.Cut(imported, "/")
		if strings.Contains(first, ".") {
			t.Errorf("%s imports %s — pkg/directorapi must import only the standard library",
				file, imported)
		}
	})
	if !found {
		t.Fatalf("no imports found under %s — the check is not actually looking at anything", apiDir)
	}
}

// forEachImport parses every .go file under dir (including tests) and calls fn for
// each import path.
func forEachImport(t *testing.T, dir string, fn func(file, imported string)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "fixtures" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			// A file that doesn't parse is someone else's test failure, not ours.
			t.Logf("skipping unparseable %s: %v", path, err)
			return nil
		}
		for _, spec := range f.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			fn(filepath.ToSlash(path), imported)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}

// platformInput are the things a duplicate keyboard, mouse, window or clipboard
// implementation inside the Director would have to reach for.
//
// Import guards catch a Director package that imports oshost. They do NOT catch one
// that reimplements it — a syscall to SendInput, a LazyDLL for user32, a direct
// OpenClipboard. That is the failure this test exists for, and it is not theoretical:
// the clipboard capability was first added to oshost and reached by a direct host
// call, which no import rule noticed because the import was legal.
var platformInput = []struct{ needle, why string }{
	{"runtime.HostCall", "a host call skips the parser and compiler; lower an Operation through marcoexec instead"},
	{".Invoke(", "hosts are reached by COMPILING Marco, never by invoking them"},
	{"MoveWindow(hwnd", "window movement is OS's MoveWindow, compiled"},
	{"syscall.", "platform calls belong in internal/oshost, reached as a Marco capability"},
	{"NewLazyDLL", "loading user32/kernel32 here would be a second input implementation"},
	{"SendInput", "keyboard and mouse input is OS's Key / OS's Type / OS's Click"},
	{"OpenClipboard", "the clipboard is OS's ClipboardGet / OS's ClipboardSet"},
	{"SetForegroundWindow", "windows are reached through directorapi.WindowProvider"},
	{"golang.org/x/sys", "no external dependency, and no second platform layer"},
}

// TestDirectorHasNoDuplicatePlatformImplementation greps every Director source file
// for the signatures of hand-rolled platform input.
//
// A text scan rather than an import scan, deliberately: the point is to catch code
// that does the work itself, which by definition is not importing the package that
// should have done it.
func TestDirectorHasNoDuplicatePlatformImplementation(t *testing.T) {
	scanned := 0
	forEachSourceFile(t, ".", func(path, src string) {
		scanned++
		for _, p := range platformInput {
			if !strings.Contains(src, p.needle) {
				continue
			}
			// A test may name these while asserting their absence, and a comment may
			// explain why they are absent. Only real code counts.
			if strings.HasSuffix(path, "_test.go") || allowedPlatformUse[path+" "+p.needle] {
				continue
			}
			for _, line := range strings.Split(src, "\n") {
				trimmed := strings.TrimSpace(line)
				if !strings.Contains(line, p.needle) || strings.HasPrefix(trimmed, "//") {
					continue
				}
				t.Errorf("%s implements platform input directly (%q): %s", path, p.needle, p.why)
			}
		}
	})
	if scanned == 0 {
		t.Fatal("scanned no Director source files — the walk is broken, not the boundary")
	}
}

// forEachSourceFile walks the Director tree and hands each .go file's source over.
func forEachSourceFile(t *testing.T, root string, fn func(path, src string)) {
	t.Helper()
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(path), string(b))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// allowedPlatformUse are the specific, argued exceptions to the scan above.
//
// A named list rather than a loosened pattern. Each entry is a place where the
// signature appears for a reason that has nothing to do with acting on the desktop,
// and spelling them out keeps the rule sharp: a new one has to be justified here
// rather than absorbed by a vaguer regex.
var allowedPlatformUse = map[string]bool{
	// Process creation, not desktop input. syscall.SysProcAttr is how the service is
	// detached from the console that launched it; there is no Marco capability for
	// "start yourself in a new process group", and there should not be — it is about
	// the Director's own lifecycle, not about the user's machine.
	"service/detach_windows.go syscall.": true,
}

// TestDirectorImportsNoWindowsShellCode is the boundary this milestone put under
// pressure.
//
//	Preserve platform boundaries: Windows-specific discovery belongs in the Windows
//	bridge or plugin implementation, not in generic Director procedures.
//
// Establishing a File Explorer item's path means asking the Windows shell which folder a
// view is showing and what it considers selected. That is COM, it is Windows-only, and it
// is Explorer-only — and a Director package that reached for it would be a Director that
// works for one file manager on one operating system.
//
// So the discovery lives in plugins/uia and internal/platform/uiaclient, and what crosses
// into the Director is a directorapi.ResourceIdentity on an observation: a path, a kind,
// and the evidence. The procedure still asks for "a file"; the platform supplies the
// answer.
//
// A separate test from the general one because the general one lists package PREFIXES,
// and this names the specific shortcut a future contributor would reach for.
func TestDirectorImportsNoWindowsShellCode(t *testing.T) {
	// Deliberately NOT "syscall": the service uses it to detach a process on Windows,
	// which is legitimate and already covered by the duplicate-implementation scan
	// below. What is forbidden here is reaching for the SHELL — COM automation, the
	// window backend, the accessibility client — to answer "what file is this?".
	shell := []string{
		"github.com/go-ole",
		"golang.org/x/sys/windows",
		modulePath + "/internal/winctx",
		modulePath + "/internal/platform/uiaclient",
		modulePath + "/internal/oshost",
	}
	forEachImport(t, ".", func(file, imported string) {
		for _, s := range shell {
			if imported == s || strings.HasPrefix(imported, s+"/") {
				t.Errorf("%s imports %s\n"+
					"  Windows shell discovery belongs in the bridge (plugins/uia) and the\n"+
					"  platform client, never in the Director. What crosses the boundary is a\n"+
					"  directorapi.ResourceIdentity on an observation — a path, a kind and the\n"+
					"  evidence — so a procedure asks for \"a file\" and the platform answers.",
					file, imported)
			}
		}
	})
}

// TestResourceIdentityCarriesNoCoordinates.
//
//	Do not add raw coordinates to semantic observations, bindings, procedures, or graph
//	metadata.
//
// Asserted over the SERIALISED form, because that is what reaches the action graph and a
// field that only looked coordinate-free in Go would still put a pixel in history.
func TestResourceIdentityCarriesNoCoordinates(t *testing.T) {
	raw, err := json.Marshal(directorapi.ResourceIdentity{
		Kind: directorapi.ResourceFile, Path: `C:\tmp\Alpha.txt`,
		ParsingName: `C:\tmp\Alpha.txt`, DisplayName: "Alpha.txt",
		Source: "shell_folder_view", Confidence: 1,
		Evidence: []string{"the shell reports this item as selected"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{`"x"`, `"y"`, "bounds", "point", "hwnd", "pidl", "handle"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Errorf("a serialised resource identity contains %s: %s", forbidden, raw)
		}
	}
}
