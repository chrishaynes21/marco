package navsource

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The HOOK PUMP invariant, as an executable rule.
//
// # Two invariants, not one
//
// This package has always been careful about the CALLBACK invariant: hook callbacks must be
// bounded and fast, because Windows silently unhooks one that overruns LowLevelHooksTimeout, and a
// dropped keyboard hook takes the stop key down with it. That rule was honoured throughout and is
// not the rule this file is about.
//
// The PUMP invariant is separate and was not written down anywhere: the thread that installs a
// low-level hook must BLOCK waiting for messages, because Windows can only deliver the callback
// while that thread is in a message wait. A PeekMessage loop with a Sleep between polls satisfies
// every callback rule and still leaves the thread asleep instead of waiting — which charges every
// keystroke and every mouse move on the entire desktop up to a scheduler quantum of latency.
//
// It never looks like a bug. The hooks keep working; Windows only unhooks past 300ms and adds the
// latency silently below that. It was found by a person saying their cursor felt heavy.
//
// # Why this reads the source
//
// Because the failure is the ABSENCE of a blocking call, and no runtime assertion can see that. A
// timing test would have to distinguish 15ms of added input latency from a busy machine, which is
// exactly the ambiguity that let this survive. The rule is structural, so the test is structural.
//
// Comments cannot satisfy it: this walks the AST, where comments do not exist.

// TestTheHookPumpBlocksRatherThanPolls holds the rule for EVERY hook site in the repository.
//
// Discovered rather than listed. The defect existed in two places at once — this package and
// internal/recorder — while a third, plugins/overlay, had always been correct, and a test naming
// the files it knew about would have been written against exactly the two that were wrong. Anyone
// who installs a low-level hook tomorrow is covered by this the moment they do it.
func TestTheHookPumpBlocksRatherThanPolls(t *testing.T) {
	sites := hookSites(t)
	if len(sites) < 2 {
		t.Fatalf("found %d hook site(s); the walk is not finding them and this test is "+
			"proving nothing", len(sites))
	}
	for _, name := range sites {
		calls := callsIn(t, name)
		if !calls["procGetMessageW"] {
			t.Errorf("%s installs a low-level hook and never calls GetMessage. Windows "+
				"delivers a hook callback only while the installing thread waits on "+
				"messages; a thread that is not waiting adds latency to every input event "+
				"on the whole desktop", name)
		}
		for _, banned := range []string{"procPeekMessageW", "procSleep"} {
			if calls[banned] {
				t.Errorf("%s calls %s. A polled pump with a sleep between polls looks "+
					"equivalent to a blocking one and is not: the thread spends its time "+
					"asleep rather than in a message wait, and Sleep(1) is up to a full "+
					"~15.6ms quantum at the default timer resolution", name, banned)
			}
		}
	}
}

// The shutdown half of the same invariant.
//
// A blocking pump cannot notice a closed channel, so it must be woken deliberately — and the hook
// must be removed by the thread that installed it. Without the wake, stopping a source falls back
// to its timeout and the hook outlives the Source that owns it.
func TestTheHookPumpIsWokenForShutdown(t *testing.T) {
	calls := callsIn(t, "navsource_windows.go")
	if !calls["procPostThreadMessageW"] {
		t.Error("nothing posts a message to the pump thread. A blocking GetMessage returns " +
			"when a message arrives and at no other time, so a source stopped without one " +
			"leaves its hooks installed until the process dies")
	}
	if !calls["procGetCurrentThreadId"] {
		t.Error("the pump does not publish its thread id, so there is nowhere to post the " +
			"wakeup to")
	}
	if !calls["procUnhookWindowsHookEx"] {
		t.Error("nothing unhooks")
	}
}

// hookSites is every file in the repository that installs a low-level Windows hook.
//
// Found by looking for the call, not by a list somebody maintains. plugins/overlay is a separate
// module and is walked anyway: the invariant is about what the OS is asked to do, and the module
// boundary has no opinion about that.
func hookSites(t *testing.T) []string {
	t.Helper()
	const repoRoot = "../../.."
	var out []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil // an unreadable corner of the tree is not this test's business
		case d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules"):
			return fs.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go"):
			return nil
		}
		if callsIn(t, path)["procSetWindowsHookExW"] {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	return out
}

// callsIn is every function called in one file, by the name at the call site.
//
// The AST rather than the text, so a comment describing the old implementation — and this package
// has a long one, deliberately — cannot fail the test, and a call cannot pass it by being
// mentioned in prose.
func callsIn(t *testing.T, file string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		// A file this test cannot parse is not a hook site it can judge. Generated and
		// vendored corners of a tree are not worth failing an invariant over.
		return map[string]bool{}
	}
	out := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// The Win32 shape is proc.Call(...), so the receiver of .Call is the name that
		// matters. A bare identifier call is recorded too, for anything else.
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if id, ok := fn.X.(*ast.Ident); ok && strings.HasPrefix(id.Name, "proc") {
				out[id.Name] = true
			}
		case *ast.Ident:
			out[fn.Name] = true
		}
		return true
	})
	return out
}
