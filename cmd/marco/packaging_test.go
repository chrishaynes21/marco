package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// THE PACKAGED LAYOUT IS WHAT MARCO LOOKS FOR.
//
// # The live failure
//
// `marco` finds two things by LOOKING rather than by being told: the Director service
// (directorBin, in director.go) and the accessibility provider (accessibilityBridge, in
// assistant.go). Both look beside the executable first, so that a packaged install works with
// no environment variable.
//
// Nothing checked that anything ever PUT them there. The single-file installer
// (plugins/marco-app) staged marco.exe, marco-macros.exe and the overlay and nothing else, so an
// installed Marco had no Director to talk to and no actor for the Theater to cast — and a
// learned play was recognised, resolved, delivered, and did nothing visible at all.
//
// # Why the test reads source text
//
// Because plugins/marco-app is a separate module with its own go.mod and there is no go.work, so
// `go test ./...` here never reaches it. A guard living there would be a guard nobody executes,
// which is the same defect one level up. It lives HERE, beside the two lookups it protects.
//
// Mutations this kills:
//   - dropping director.exe or uia.exe from the `staged` table (shipped nowhere, or not at all)
//   - extracting uia.exe flat into bin\ instead of bin\plugins\uia\ (tier 2 then misses it)
//   - adding an entry to `staged` that pack.ps1 never produces (installer ships it empty)
//   - narrowing //go:embed back to named files, which is a COMPILE error on any tree where
//     those files have not been built — i.e. every fresh clone and every CI run
func TestThePackagedLayoutIsWhatMarcoLooksFor(t *testing.T) {
	src := readRepoFile(t, "plugins/marco-app/main.go")
	pack := readRepoFile(t, "pack.ps1")

	// 1. The embed must not name individual build outputs.
	//
	// `//go:embed` is a compile-time pattern and a named file that has not been built is a
	// build FAILURE, not a missing asset. None of the staged files exist in a fresh checkout,
	// so naming them meant the installer module only compiled on a machine that had already
	// packed it — and adding a name broke every tree that had packed the previous set.
	for _, line := range strings.Split(src, "\n") {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, "//go:embed ") {
			continue
		}
		if strings.Contains(l, "assets/") {
			t.Errorf("the installer names individual staged files in its embed: %q.\n"+
				"Those files do not exist until pack.ps1 has run, so this does not "+
				"compile on a clean clone or in CI. Embed the tracked DIRECTORY "+
				"(all:assets) and let extractBins report a missing file by name.", l)
		}
	}

	// 2. The destinations are exactly the paths this package's lookups search.
	//
	// Not "director.exe is in there somewhere" — WHERE. directorBin() joins
	// filepath.Dir(os.Executable()) with "director.exe"; accessibilityBridge() joins it with
	// "plugins/uia/uia.exe". marco.exe is extracted into bin\, so these are relative to bin\.
	table := stagedTable(t, src)
	for asset, wantDest := range map[string]string{
		"director.exe": "director.exe",
		"uia.exe":      "plugins/uia/uia.exe",
	} {
		got, ok := table[asset]
		if !ok {
			t.Errorf("the installer does not extract %s.\n"+
				"Without director.exe an installed Marco has no Director to "+
				"auto-start; without uia.exe the Theater has no actor and every "+
				"learned play silently does nothing.", asset)
			continue
		}
		if got != wantDest {
			t.Errorf("the installer extracts %s to %q, but marco looks for it at %q "+
				"beside its own executable — so it is shipped and never found.",
				asset, got, wantDest)
		}
	}

	// 3. pack.ps1 actually produces every file the table extracts. An entry it never stages
	//    ships as a missing asset, and the person who could have fixed it has gone home.
	for asset := range table {
		if !strings.Contains(pack, asset) {
			t.Errorf("plugins/marco-app extracts assets/%s but pack.ps1 never stages "+
				"it; the installer would ship without it.", asset)
		}
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// stagedEntry matches one {"asset", "dest"} pair of the installer's extraction table.
var stagedEntry = regexp.MustCompile(`\{"([^"]+)",\s*"([^"]+)"\}`)

func stagedTable(t *testing.T, src string) map[string]string {
	t.Helper()
	i := strings.Index(src, "var staged = []struct")
	if i < 0 {
		t.Fatal("plugins/marco-app/main.go has no `staged` table; nothing says WHERE each " +
			"embedded binary is written, and the two lookups in this package depend on it")
	}
	j := strings.Index(src[i:], "\n}")
	if j < 0 {
		t.Fatal("the `staged` table in plugins/marco-app/main.go is unterminated")
	}
	out := map[string]string{}
	for _, m := range stagedEntry.FindAllStringSubmatch(src[i:i+j], -1) {
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		t.Fatal("the `staged` table is empty; the installer extracts nothing")
	}
	return out
}
