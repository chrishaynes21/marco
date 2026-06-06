package osmod

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNoDrift guards against the embedded copy diverging from the canonical
// programs/os.marco. If this fails, sync the two files.
func TestNoDrift(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "..", "programs", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != Source {
		t.Fatalf("internal/osmod/os.marco is out of sync with programs/os.marco — copy programs/os.marco over it")
	}
}
