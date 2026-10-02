package interpret_test

import (
	"os"
	"strings"
	"testing"
)

// read returns one of this package's own files, for the structural checks.
func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}

// importBlock returns just the import declaration, so a word appearing in a comment or a string
// cannot be mistaken for a dependency.
//
// The whole file was checked at first and it failed on its own documentation, which names the
// packages it deliberately does NOT reach — a guard that cannot tell a dependency from a sentence
// about a dependency is a guard that forbids explaining itself.
func importBlock(source string) string {
	i := strings.Index(source, "\nimport (")
	if i < 0 {
		return ""
	}
	rest := source[i:]
	j := strings.Index(rest, "\n)")
	if j < 0 {
		return rest
	}
	return rest[:j]
}
