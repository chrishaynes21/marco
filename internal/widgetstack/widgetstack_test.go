package widgetstack

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func touch(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The installed layout is flat beside Studio. A development tree is recognised by its source
// (go.mod and plugins\studio\build.cmd), from its root AND from its plugins\studio, because
// build.cmd puts a studio.exe in both; either way the stack is the root's. A directory that is
// neither is described as the installed layout so its pieces read as missing.
func TestTheStackIsFoundWhereEachLayoutPutsIt(t *testing.T) {
	inst := t.TempDir()
	touch(t, filepath.Join(inst, "overlay.exe"), filepath.Join(inst, "overlay.marco"))
	if l := Beside(inst); l.Kind != "installed" || l.Dir != inst || l.Overlay != filepath.Join(inst, "overlay.exe") ||
		l.Script != filepath.Join(inst, "overlay.marco") || l.Marco != filepath.Join(inst, "marco.exe") ||
		l.Launcher() != "Marco.exe --overlay" || l.Rebuild() != "reinstall Marco" {
		t.Errorf("installed: %+v", l)
	}

	tree := aTree(t)
	for _, from := range []string{tree, filepath.Join(tree, "plugins", "studio"), filepath.Join(tree, "PLUGINS", "Studio")} {
		l := Beside(from)
		if l.Kind != "tree" || l.Dir != tree ||
			l.Marco != filepath.Join(tree, "marco.exe") || l.Director != filepath.Join(tree, "director.exe") ||
			l.Macros != filepath.Join(tree, "marco-macros.exe") ||
			l.Overlay != filepath.Join(tree, "plugins", "overlay", "overlay.exe") ||
			l.Script != filepath.Join(tree, "plugins", "overlay", "overlay.marco") ||
			l.Launcher() != filepath.Join(tree, "overlay.cmd") {
			t.Errorf("tree, from %s: %+v", from, l)
		}
		if want := filepath.Join(tree, "plugins", "studio", "build.cmd"); !strings.Contains(l.Rebuild(), want) ||
			strings.Contains(l.Rebuild(), "reinstall") {
			t.Errorf("tree, from %s: the fix is %q, not the build script %s", from, l.Rebuild(), want)
		}
	}

	// NOT YET BUILT IS STILL A TREE: the layout is decided by the source, so an overlay that has not
	// been built reads as missing at its tree path, never as a missing installation.
	bare := t.TempDir()
	touch(t, filepath.Join(bare, "go.mod"), filepath.Join(bare, "plugins", "studio", "build.cmd"))
	if l := Beside(filepath.Join(bare, "plugins", "studio")); l.Kind != "tree" || l.Dir != bare ||
		l.Overlay != filepath.Join(bare, "plugins", "overlay", "overlay.exe") {
		t.Errorf("an unbuilt tree: %+v", l)
	}

	// A STRAY OVERLAY AT A TREE'S ROOT DOES NOT MAKE IT AN INSTALLATION.
	touch(t, filepath.Join(tree, "overlay.exe"), filepath.Join(tree, "overlay.marco"))
	if l := Beside(tree); l.Kind != "tree" {
		t.Errorf("a tree with a stray flat overlay: %+v", l)
	}

	// Neither half of the marker alone is a tree, and a plugins\studio outside a tree is not one.
	half := t.TempDir()
	touch(t, filepath.Join(half, "go.mod"), filepath.Join(half, "plugins", "overlay", "overlay.exe"))
	if l := Beside(half); l.Kind != "installed" {
		t.Errorf("go.mod without the build script: %+v", l)
	}
	loose := filepath.Join(t.TempDir(), "plugins", "studio")
	touch(t, filepath.Join(loose, "overlay.exe"), filepath.Join(loose, "overlay.marco"))
	if l := Beside(loose); l.Kind != "installed" || l.Dir != loose {
		t.Errorf("an installation that happens to be in a plugins/studio: %+v", l)
	}
	if l := Beside(t.TempDir()); l.Kind != "installed" {
		t.Errorf("nothing there: %+v", l)
	}
}

// aTree is a development tree's markers and its built stack, as plugins\studio\build.cmd leaves
// it.
func aTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	touch(t, filepath.Join(root, "go.mod"), filepath.Join(root, "plugins", "studio", "build.cmd"),
		filepath.Join(root, "marco.exe"), filepath.Join(root, "director.exe"),
		filepath.Join(root, "marco-macros.exe"), filepath.Join(root, "studio.exe"),
		filepath.Join(root, "plugins", "studio", "studio.exe"),
		filepath.Join(root, "plugins", "overlay", "overlay.exe"),
		filepath.Join(root, "plugins", "overlay", "overlay.marco"))
	return root
}

// Args is the installer's runOverlay command, spelled for a layout. The installer cannot import
// this package (its module is `marco-app`), so its source is read: the hosts and the program must
// be the same, in the same order. Changing either without the other must fail this.
func TestTheStackIsTheOneTheInstallerStarts(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "plugins", "marco-app", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)func runOverlay\(binDir string, args \[\]string\) error \{.*?full := \[\]string\{(.*?)\}`).FindSubmatch(src)
	if m == nil {
		t.Fatal("the installer's runOverlay command could not be found")
	}
	got := regexp.MustCompile(`\s+`).ReplaceAllString(string(m[1]), "")
	want := `"serve","--host","OS=bridge:"+filepath.Join(binDir,"marco-macros.exe"),"--host","Overlay=bridge:"+filepath.Join(binDir,"overlay.exe"),filepath.Join(binDir,"overlay.marco")`
	if got != want {
		t.Errorf("the installer starts\n%s\nand this package was written against\n%s", got, want)
	}
	l := Beside(t.TempDir())
	if a := l.Args(); strings.Join(a, " ") != "serve --host OS=bridge:"+l.Macros+" --host Overlay=bridge:"+l.Overlay+" "+l.Script {
		t.Errorf("Args is %q", a)
	}
}

// A widget is an overlay.exe with its program beside it, wherever it was installed.
func TestAnOverlayIsOneWithItsProgramBesideIt(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "OVERLAY.EXE"), filepath.Join(dir, "overlay.marco"))
	if !IsOverlay(filepath.Join(dir, "OVERLAY.EXE")) {
		t.Error("a widget was not recognised")
	}
	other := t.TempDir()
	touch(t, filepath.Join(other, "overlay.exe"))
	if IsOverlay(filepath.Join(other, "overlay.exe")) || IsOverlay(filepath.Join(dir, "marco.exe")) {
		t.Error("something that is not a widget was taken for one")
	}
}
