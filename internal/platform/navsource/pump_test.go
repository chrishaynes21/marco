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

// AND THE WAKE IS RETRIED, AT EVERY SITE THAT WAKES ONE.
//
// # The bug this is the test for
//
// A Windows thread has no message queue until it first calls a message function, and
// `PostThreadMessage` to a thread that has not got one FAILS. Every pump here publishes its thread
// id before installing its hooks, deliberately, so that a stop racing a start still has somewhere
// to aim — and that leaves a window in which the wakeup lands nowhere. A single unchecked post
// into that window is lost; `GetMessage` then waits for a message that will never arrive, and the
// caller waiting for the pump to finish waits with it. Forever.
//
// It happened twice, in two packages, and cost about eighteen minutes of test time before either
// was understood: this package's suite ran 495s before being killed, and `cmd/marco` hit its
// 600-second limit twice — recorded both times as an unexplained timeout with contention as the
// hypothesis, which it was not.
//
// # Why it is tree-wide and why THIS is the shape of it
//
// The blocking half above is tree-wide and was written that way on purpose: the defect existed in
// two places at once, and a test naming files would have been written against the two that were
// wrong. The shutdown half was NOT, and that is exactly how the second site kept the bug after the
// first was fixed — the fix, the diagnosis and the comment explaining it all existed, in a file
// this test did not look at.
//
// So the rule is structural and applies to whoever wakes a pump: the post must sit inside a loop.
// A retry is the only thing that closes the window, because the window is a race and the caller
// cannot tell from the outside whether the post it just made was delivered. Sites that never stop
// their pump — plugins/overlay installs one hook for the life of the process — post nothing and
// are not asked to.
func TestAPumpWakeupIsRetriedRatherThanPostedOnce(t *testing.T) {
	var waking []string
	for _, name := range hookSites(t) {
		if !callsIn(t, name)["procPostThreadMessageW"] {
			continue // nothing here stops a pump
		}
		waking = append(waking, name)
		if !postIsRetried(t, name) {
			t.Errorf("%s posts its wakeup once and then waits for the pump to "+
				"finish. A thread with no message queue yet refuses the post, and "+
				"the wait that follows never ends: the pump is asleep in "+
				"GetMessage and the message that would wake it was never queued. "+
				"Post it in a loop until the pump answers, with a bound so a pump "+
				"that has already died cannot wedge the caller either", name)
		}
	}
	if len(waking) < 2 {
		t.Errorf("only %d hook site(s) wake a pump (%v); two are known to, so this test "+
			"is checking less than it reads as", len(waking), waking)
	}
}

// postIsRetried reports whether every PostThreadMessage in a file is inside a loop.
//
// Every one, not any one: a site that retries in one place and posts once in another has the bug
// in the second place, and the second place is the one that will be waited on.
func postIsRetried(t *testing.T, name string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	// The loops, by the span of source they cover, so a call can be placed inside one.
	type span struct{ from, to token.Pos }
	var loops []span
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ForStmt:
			loops = append(loops, span{v.Pos(), v.End()})
		case *ast.RangeStmt:
			loops = append(loops, span{v.Pos(), v.End()})
		}
		return true
	})
	retried := true
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Name != "procPostThreadMessageW" {
			return true
		}
		inside := false
		for _, l := range loops {
			if sel.Pos() > l.from && sel.End() < l.to {
				inside = true
			}
		}
		if !inside {
			retried = false
		}
		return true
	})
	return retried
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
