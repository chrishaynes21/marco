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

// AND THE BUNDLE SHIPS STUDIO, BUILT SO THAT IT OPENS A WINDOW.
//
// # What was shipped before, and what it cost
//
// The installer staged `marco`, `marco-macros`, `director`, `overlay` and `uia`, and the
// launcher ran the overlay serve stack. No root script mentioned Studio and the README never
// did — so the record → Stop → review → Save → Play flow, and every guard in it, reached nobody
// who installed Marco. It existed only as a development build.
//
// And the consequence is sharper than a missing feature: the shipped overlay can record and can
// simplify, and when it spawns the simplify child it says so has no preview. So an installed
// Marco could WRITE a routine and could not show anybody what it was about to write, which makes
// every guard that depends on a review unreachable.
//
// # Why this reads source text, and what it cannot establish
//
// Same reason as the test above: `plugins/marco-app` and `plugins/studio` are separate modules
// with no go.work, so `go test ./...` here never reaches either. A guard living there is a guard
// nobody executes.
//
// It cannot establish that the PACKED artifact is correct — only that the script asks for the
// right thing. `pack.ps1` checks the built bytes for the Wails guard's own message box, which is
// the only check that can tell the two builds apart, and the packaged bundle is exercised from an
// isolated location separately.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestTheBundleShipsStudioAsItsFrontDoor(t *testing.T) {
	src := readRepoFile(t, "plugins/marco-app/main.go")
	pack := readRepoFile(t, "pack.ps1")
	table := stagedTable(t, src)

	// 1. STUDIO IS EXTRACTED, AND BESIDE THE TWO CHILDREN IT SPAWNS.
	//
	// Studio spawns the `marco.exe` and `director.exe` beside ITSELF, not the ones beside
	// each other — so the three have to land in one directory. They all land in bin\, which
	// satisfies that and `marco`'s own two lookups at once.
	for _, asset := range []string{"studio.exe", "marco.exe", "director.exe"} {
		dest, ok := table[asset]
		if !ok {
			t.Errorf("the installer does not extract %s. Studio spawns the marco "+
				"and director beside ITSELF, so all three have to be in one "+
				"directory or a current window runs whatever is on PATH.", asset)
			continue
		}
		if strings.Contains(dest, "/") {
			t.Errorf("the installer extracts %s to %q, which is not beside the "+
				"others — Studio looks in its own directory and would find "+
				"nothing there", asset, dest)
		}
	}

	// 2. IT IS BUILT WITH THE WAILS TAGS, AND THE BUILD IS CHECKED.
	//
	// A plain `go build` of Studio produces a binary that opens NO WINDOW: without
	// `production`, Wails compiles a CreateApp that shows a message box and exits. The
	// binary is otherwise ordinary — flags, usage, `-acts` and `-proof` all work — so one
	// sat in the tree looking current until somebody double-clicked it.
	for _, want := range []string{"desktop,production", "-H windowsgui"} {
		if !strings.Contains(pack, want) {
			t.Errorf("pack.ps1 does not build Studio with %q, so the bundle would "+
				"ship a studio.exe that opens no window and says so only in a "+
				"message box", want)
		}
	}
	if !strings.Contains(pack, "will not build without the correct build tags") {
		t.Error("pack.ps1 does not check the built studio.exe for the Wails guard's " +
			"own message box, which is the only way to tell the two builds apart " +
			"without opening a window on somebody's desktop")
	}

	// 3. EVERY GO BINARY'S REVISION IS CHECKED AT PACK TIME.
	//
	// `go build` stamps the VCS revision only for a package in the main module of the
	// build, so one built from outside its module answers "unknown" — the mistake
	// `build.cmd` made with the director once. A bundle of those makes every run record
	// unciteable, and `build` is the field that would have named the stale recorder.
	if !strings.Contains(pack, "vcs.revision") {
		t.Error("pack.ps1 never asks a staged binary what revision it was built from, " +
			"so a bundle of unstamped binaries would ship and every run record " +
			"would say `unknown`")
	}
	if !strings.Contains(pack, "manifest.json") {
		t.Error("pack.ps1 writes no manifest, so nothing in the bundle says what each " +
			"piece is — and the one component that is not Go has no VCS stamp to " +
			"ask for, only the hash of its bytes")
	}
	if _, ok := table["manifest.json"]; !ok {
		t.Error("the manifest is written at pack time and not extracted, so an " +
			"installed Marco cannot say what it is made of")
	}

	// 4. AND THE OVERLAY IS NOT STARTED BY DEFAULT, WHICH IS THE WHOLE POINT.
	//
	// It is still shipped and still works. A launcher that opened both would put two front
	// doors on one desktop, and a launcher that opened only the overlay is what this fixes.
	if !strings.Contains(src, "--overlay") {
		t.Error("the launcher has no explicit way to ask for the legacy overlay, so " +
			"either it is gone or it is still the default")
	}
	studio := strings.Index(src, "func runStudio")
	overlay := strings.Index(src, "func runOverlay")
	if studio < 0 || overlay < 0 {
		t.Fatal("the launcher does not have one entrance per surface, so which one " +
			"starts is decided somewhere other than in one place")
	}
	if _, ok := table["overlay.exe"]; !ok {
		t.Error("the overlay was removed from the bundle; it is meant to stay " +
			"available, just not started")
	}
}
