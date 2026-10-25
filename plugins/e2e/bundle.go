//go:build windows

package main

// THE BINARIES THIS RUN IS ABOUT, BUILT FRESH AND KEPT TOGETHER.
//
// # Why a bundle and not the tree
//
// Because Studio spawns the `marco.exe` and `director.exe` BESIDE ITSELF, not the ones beside
// each other. That is the trap CLAUDE.md spends a paragraph on: with a copy of Studio in
// `plugins\studio` and another at the repo root, a stale sibling is invisible — every binary
// looks freshly built, the window is current, and the recordings come from whenever that
// director was last made. It cost the owner a day of recordings.
//
// A harness that ran the tree's binaries would inherit that exactly, and worse: it would also be
// writing to the paths the owner's own Studio was built to, while their Studio may be open. So
// this run gets its own directory with its own four binaries in it, built from this tree in one
// breath, and nothing in the repository is written to.
//
// # And it says what they were
//
// Every binary's size and digest, the revision they were built from, and whether the tree was
// dirty. A green run against an unknown revision is not a result anybody can act on later, and a
// failure against a dirty tree is a different kind of news from a failure against a clean one.
//
// # The tags are not optional and not a detail
//
// A plain `go build` of Studio produces a binary that OPENS NO WINDOW: Wails guards on its own
// build tags and without them compiles a `CreateApp` that shows a message box and exits. The
// binary is otherwise perfectly ordinary, which is why one sat in the tree looking current until
// somebody double-clicked it. So the tags are stated here, a test holds them against
// `build.cmd`'s, and the built binary is checked for the guard's own sentence — the documented
// way to tell a windowless build from a real one without opening a window on somebody's desktop.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	fixtureTitle = "Marco Fixture"
	studioTitle  = "Marco Studio"

	// studioTags and studioLdflags are build.cmd's, and TestTheHarnessBuildsStudioLikeTheScript
	// holds the pair. Two lists of one fact is the shape this repository keeps finding; here
	// the cost of drift is a harness that opens no window and says the page never rendered.
	studioTags    = "desktop,production"
	studioLdflags = "-w -s -H windowsgui"

	// wailsGuard is the sentence a WINDOWLESS Studio contains and a real one does not.
	wailsGuard = "will not build without the correct build tags"
)

// identity is one binary that took part, and what it was.
type identity struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Modified string `json:"modified"`
	From     string `json:"from"`
}

type bundle struct {
	dir                                   string
	marco, director, studio, fixture, uia string
	identities                            []identity
}

// makeBundle builds this run's binaries and records what they are.
func makeBundle(m machine, dir string, art *artifacts) (*bundle, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("go"); err != nil {
		return nil, cannot("there is no Go toolchain on the path, so the binaries " +
			"under test cannot be built")
	}
	b := &bundle{dir: dir,
		marco:    filepath.Join(dir, "marco.exe"),
		director: filepath.Join(dir, "director.exe"),
		studio:   filepath.Join(dir, "studio.exe"),
		fixture:  filepath.Join(dir, "fixture.exe"),
		uia:      filepath.Join(dir, "uia.exe"),
	}
	// THE ENGINE AND THE DIRECTOR, FROM THE ROOT MODULE. Not from inside plugins: `go build`
	// stamps the VCS revision into a binary only when the package is in the main module, so a
	// director built from elsewhere cannot say what it is — and saying what it is, is the
	// whole point of internal/buildid.
	art.step("building marco.exe and director.exe")
	if err := b.goBuild(m.Root, "", b.marco, "./cmd/marco"); err != nil {
		return nil, err
	}
	if err := b.goBuild(m.Root, "", b.director, "./cmd/director"); err != nil {
		return nil, err
	}
	art.step("building studio.exe with -tags %s", studioTags)
	if err := b.goBuild(filepath.Join(m.Root, "plugins", "studio"), studioTags,
		b.studio, "."); err != nil {
		return nil, err
	}
	// AND THE WINDOWLESS BUILD IS CAUGHT HERE, not by a page that never renders.
	if windowless, err := contains(b.studio, wailsGuard); err != nil {
		return nil, err
	} else if windowless {
		return nil, fmt.Errorf("the Studio that was just built contains %q, so it "+
			"would open no window. The tags this harness used (%s) are not the ones "+
			"Wails needs", wailsGuard, studioTags)
	}
	// THE TWO THAT ARE NOT GO. Copied rather than built, and checked for staleness rather
	// than rebuilt: both are committed binaries, and a harness that rebuilt them would dirty
	// the tree it is reporting the revision of.
	for _, c := range []struct{ from, to, source string }{
		{filepath.Join(m.Root, "plugins", "fixture", "fixture.exe"), b.fixture,
			filepath.Join(m.Root, "plugins", "fixture", "Fixture.cs")},
		{filepath.Join(m.Root, "plugins", "uia", "uia.exe"), b.uia,
			filepath.Join(m.Root, "plugins", "uia", "Program.cs")},
	} {
		if err := freshEnough(c.from, c.source); err != nil {
			return nil, err
		}
		if err := copyFile(c.from, c.to); err != nil {
			return nil, err
		}
		b.identities = append(b.identities, identify(filepath.Base(c.to), c.to,
			"copied from "+rel(m.Root, c.from)))
	}
	for _, p := range []string{b.marco, b.director, b.studio} {
		how := "built from this tree"
		if p == b.studio {
			how = "built from this tree with -tags " + studioTags
		}
		b.identities = append(b.identities, identify(filepath.Base(p), p, how))
	}
	out, err := json.MarshalIndent(map[string]any{
		"head": m.Head, "dirty": m.Dirty, "root": m.Root,
		"screen":   fmt.Sprintf("%dx%d at %d dpi", m.ScreenW, m.ScreenH, m.DPI),
		"binaries": b.identities,
	}, "", "  ")
	if err == nil {
		art.keep("build.json", out)
	}
	return b, nil
}

func (b *bundle) goBuild(dir, tags, out, pkg string) error {
	args := []string{"build"}
	if tags != "" {
		args = append(args, "-tags", tags, "-ldflags", studioLdflags)
	}
	args = append(args, "-o", out, pkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("building %s in %s: %v\n%s", pkg, dir, err,
			strings.TrimSpace(string(combined)))
	}
	return nil
}

// freshEnough refuses a committed binary older than the source it was built from.
//
// NOT RUN rather than a rebuild. Rebuilding would change a tracked file and make the revision
// this run reports a lie; and the person who edited the source is the person who should decide
// when the binary is rebuilt. The message says the command.
func freshEnough(bin, source string) error {
	b, err := os.Stat(bin)
	if err != nil {
		return cannot("%s is not built, so this run has nothing to spawn. Build it: "+
			"powershell -File %s", filepath.Base(bin),
			filepath.Join(filepath.Dir(source), "build.ps1"))
	}
	s, err := os.Stat(source)
	if err != nil {
		return nil
	}
	if b.ModTime().Before(s.ModTime()) {
		return cannot("%s is older than %s, so this run would be testing a window "+
			"nobody built. Build it: powershell -File %s", filepath.Base(bin),
			filepath.Base(source), filepath.Join(filepath.Dir(source), "build.ps1"))
	}
	return nil
}

func identify(name, path, from string) identity {
	id := identity{Name: name, Path: path, From: from}
	if st, err := os.Stat(path); err == nil {
		id.Size = st.Size()
		id.Modified = st.ModTime().Format(time.RFC3339)
	}
	if data, err := os.ReadFile(path); err == nil {
		sum := sha256.Sum256(data)
		id.SHA256 = hex.EncodeToString(sum[:])[:16]
	}
	return id
}

func contains(path, phrase string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), phrase), nil
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return cannot("%s is missing, so this run has nothing to %s", from,
			filepath.Base(from))
	}
	return os.WriteFile(to, data, 0o755)
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}
