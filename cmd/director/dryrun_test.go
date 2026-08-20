package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// CLI argument handling.
//
//	Fix and regression-test argument parsing so option values such as `--app` can
//	never be reordered into positional request text.
//
// The bug this guards against is silent rather than loud: flagsFirst moved "explorer"
// into the positional list, the request became "explorer", and the command reported that
// "explorer" was not a goal. Nothing errored — the output simply answered a question
// nobody asked.

func TestAnOptionValueIsNeverReorderedIntoTheRequest(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string // the request that should survive reordering
	}{
		{"flag after request", []string{"rename this file", "--app", "explorer"}, "rename this file"},
		{"flag before request", []string{"--app", "explorer", "rename this file"}, "rename this file"},
		{"equals form", []string{"rename this file", "--app=explorer"}, "rename this file"},
		{"single dash", []string{"rename this file", "-app", "explorer"}, "rename this file"},
		{"window flag", []string{"e246", "--window", "hwnd:12"}, "e246"},
		{"n flag", []string{"last", "-n", "5"}, "last"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := flagsFirst(c.args)

			// The positional tail is everything after the last flag-or-value. The
			// property that matters: the option's VALUE must not be in it.
			var positionals []string
			for i := 0; i < len(got); i++ {
				a := got[i]
				if strings.HasPrefix(a, "-") {
					if !strings.Contains(a, "=") && valued[a] && i+1 < len(got) {
						i++ // skip its value
					}
					continue
				}
				positionals = append(positionals, a)
			}
			if len(positionals) != 1 || positionals[0] != c.want {
				t.Errorf("positionals = %v, want exactly [%q].\n"+
					"An option's value reordered into the request makes the command "+
					"answer a question nobody asked.", positionals, c.want)
			}
		})
	}
}

// TestEveryValuedFlagTheCommandsDefineIsDeclared is the guard against the NEXT
// occurrence of this bug.
//
// The valued map has to be updated whenever a non-boolean flag is added, and forgetting
// is silent. This checks the flags the goal commands actually define.
func TestEveryValuedFlagTheCommandsDefineIsDeclared(t *testing.T) {
	// Flags that take a value in the commands added by the goal milestones.
	for _, flag := range []string{"--app", "-app"} {
		if !valued[flag] {
			t.Errorf("%s takes a value and is not in the valued map, so its argument "+
				"will be silently treated as positional text", flag)
		}
	}
	// And the booleans must NOT be there, or they would swallow the request.
	for _, flag := range []string{"--json", "--dry-run", "--chain", "--last"} {
		if valued[flag] {
			t.Errorf("%s is a boolean and is in the valued map, so it would swallow the "+
				"request that follows it", flag)
		}
	}
}

// TestABooleanFlagDoesNotSwallowTheRequest is the complementary failure.
func TestABooleanFlagDoesNotSwallowTheRequest(t *testing.T) {
	got := flagsFirst([]string{"--dry-run", "close without saving"})
	if !slices.Contains(got, "close without saving") {
		t.Fatalf("the request disappeared: %v", got)
	}
	// It must still be a positional, not the boolean's value.
	if got[len(got)-1] != "close without saving" {
		t.Errorf("args = %v, want the request last as a positional", got)
	}
}

// ── dry run ───────────────────────────────────────────────────────────────────

// TestDryRunReachesNoProviderAtAll is the safety property, checked by reading the
// source rather than by running it.
//
// A behavioural test cannot prove a NEGATIVE about UI input without a desktop to not
// touch. What it can prove is that the code path constructs nothing that could: the
// dry-run command builds a registry, parses, expands and validates, and names no
// bridge, no provider and no client.
func TestDryRunReachesNoProviderAtAll(t *testing.T) {
	src := readSource(t, "dryrun.go")
	for _, forbidden := range []string{
		"bridgehost.", "uiaclient.", "winprovider.", "connect(",
		"Observe", "Snapshot", "Focus(", "Click(",
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("the dry-run command references %q; a dry run must not focus "+
				"windows, observe, or perform input", forbidden)
		}
	}
}

func TestDryRunSaysPlainlyThatNothingRan(t *testing.T) {
	src := readSource(t, "dryrun.go")
	if !strings.Contains(src, "Nothing was run, focused, or observed") {
		t.Error("the dry-run output does not state that nothing happened; a reader " +
			"cannot tell it apart from a real run's output")
	}
}

// readSource reads one file of this package, for the source-scanning guards above.
func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
