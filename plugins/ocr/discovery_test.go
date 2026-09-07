package main

import (
	"os"
	"path/filepath"
	"testing"
)

// WHAT SOMEBODY SAID EXPLICITLY WINS.
//
// The existing contract, and it stays first: a person who points $MARCO_TESSERACT at a binary
// has answered the question, and a search that overrode them would be Marco deciding it knew
// better about their own machine.
func TestAnExplicitTesseractWins(t *testing.T) {
	t.Setenv("MARCO_TESSERACT", `C:\somewhere\else\tesseract.exe`)
	if got := findTesseract(); got != `C:\somewhere\else\tesseract.exe` {
		t.Errorf("findTesseract() = %q; an explicit setting must win over any search", got)
	}
}

// AND TESSERACT IS FOUND WHERE WINDOWS INSTALLS IT.
//
// # The defect this closes
//
// Tesseract was installed at the standard location and Marco reported OCR unavailable, because
// the lookup was $MARCO_TESSERACT or the bare name on PATH — and the usual Windows installer does
// not put itself on PATH. The capability was present and unreachable: nothing missing, nothing
// broken, and the product behaving as though the feature did not exist.
//
// Deleting the standard-location search must fail this.
func TestTesseractIsFoundWhereWindowsInstallsIt(t *testing.T) {
	dir := t.TempDir()
	// A program directory with an ordinary install inside it.
	installed := filepath.Join(dir, "Tesseract-OCR", "tesseract.exe")
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_TESSERACT", "")
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("LOCALAPPDATA", "")
	// PATH emptied, so this cannot pass by finding a real tesseract on the machine that
	// happens to be running the suite.
	t.Setenv("PATH", dir)

	if got := findTesseract(); got != installed {
		t.Errorf("findTesseract() = %q, want the standard install at %q", got, installed)
	}
}

// AND A MACHINE WITHOUT IT STILL SAYS SO USEFULLY.
//
// The fallback is the bare name, so the failure a person sees is still the one they can act on:
// `exec: "tesseract": executable file not found` names the thing to install. A search that
// returned an empty string would produce an error about nothing.
func TestWithoutTesseractTheNameSurvivesForTheError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_TESSERACT", "")
	t.Setenv("ProgramFiles", dir)
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("PATH", dir)

	if got := findTesseract(); got != "tesseract" {
		t.Errorf("findTesseract() = %q; the bare name is what makes the error nameable", got)
	}
}
