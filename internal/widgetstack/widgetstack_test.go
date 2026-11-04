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

// The installed layout is flat, the tree layout keeps the overlay under plugins\overlay, and a
// directory with neither is described as the installed layout so its pieces read as missing.
func TestTheStackIsFoundWhereEachLayoutPutsIt(t *testing.T) {
	inst := t.TempDir()
	touch(t, filepath.Join(inst, "overlay.exe"), filepath.Join(inst, "overlay.marco"))
	if l := Beside(inst); l.Kind != "installed" || l.Overlay != filepath.Join(inst, "overlay.exe") ||
		l.Script != filepath.Join(inst, "overlay.marco") || l.Marco != filepath.Join(inst, "marco.exe") {
		t.Errorf("installed: %+v", l)
	}
	tree := t.TempDir()
	touch(t, filepath.Join(tree, "plugins", "overlay", "overlay.exe"))
	if l := Beside(tree); l.Kind != "tree" || l.Overlay != filepath.Join(tree, "plugins", "overlay", "overlay.exe") ||
		l.Macros != filepath.Join(tree, "marco-macros.exe") || !strings.HasSuffix(l.Launcher(), "overlay.cmd") {
		t.Errorf("tree: %+v", l)
	}
	if l := Beside(t.TempDir()); l.Kind != "installed" {
		t.Errorf("nothing there: %+v", l)
	}
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
