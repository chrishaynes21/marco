// Package widgetstack is what the widget (the overlay stack) is made of, where those pieces are
// laid out, and the command that starts it, for a surface that starts it on somebody's behalf
// (Studio's Open widget, S7 of docs/Surfaces-alignment-proposal-2026-11-02.md).
//
// # It is the installer's stack, not a new one
//
// The installed Marco starts the widget with `Marco.exe --overlay` (plugins/marco-app
// `runOverlay`): `marco serve` with the macros bridge as the OS host, the overlay as the Overlay
// host, and overlay.marco as the program, all from the installation's bin\ directory. Args is that
// command. The installer is its own module, named `marco-app`, which Go's internal rule keeps from
// importing this package, so its copy stays where it is. TestTheStackIsTheOneTheInstallerStarts
// reads the installer's source and fails if the two drift.
//
// # Two layouts, both found by looking
//
//	installed   every piece flat beside studio.exe and marco.exe, as the bundle unpacks them
//	tree        a development tree: marco.exe, director.exe and marco-macros.exe at the root,
//	            the overlay and its program under plugins\overlay, as overlay.cmd uses them
//
// Nothing here starts anything, and nothing here decides whether the pieces belong together: that
// is the caller's question, asked of every piece's build revision (internal/buildid).
package widgetstack

import (
	"os"
	"path/filepath"
)

// Layout is where each piece of the widget stack is, beside one directory.
type Layout struct {
	// Kind is "installed" or "tree".
	Kind string
	// Dir is the directory the stack runs from.
	Dir                                      string
	Marco, Director, Macros, Overlay, Script string
}

// Component is one piece of the stack, for a report.
type Component struct {
	// Name is how a person knows it: marco.exe, overlay.marco, …
	Name string
	Path string
	// Program says it is a Go executable with a build revision to compare. The overlay's
	// program (overlay.marco) is source and has none.
	Program bool
}

// Beside finds the stack laid out beside dir: the installed layout when the overlay is there,
// else the tree layout when it is under plugins\overlay, else the installed layout's paths
// (whose pieces are then reported as missing).
func Beside(dir string) Layout {
	installed := Layout{Kind: "installed", Dir: dir,
		Marco:    filepath.Join(dir, "marco.exe"),
		Director: filepath.Join(dir, "director.exe"),
		Macros:   filepath.Join(dir, "marco-macros.exe"),
		Overlay:  filepath.Join(dir, "overlay.exe"),
		Script:   filepath.Join(dir, "overlay.marco"),
	}
	if exists(installed.Overlay) {
		return installed
	}
	tree := installed
	tree.Kind = "tree"
	tree.Overlay = filepath.Join(dir, "plugins", "overlay", "overlay.exe")
	tree.Script = filepath.Join(dir, "plugins", "overlay", "overlay.marco")
	if exists(tree.Overlay) {
		return tree
	}
	return installed
}

// Components is every piece the stack needs, in the order a report lists them. director.exe is
// one of them although the command does not name it: marco finds it beside itself, and a widget
// without it cannot perform a routine somebody demonstrated (CLAUDE.md).
func (l Layout) Components() []Component {
	return []Component{
		{Name: "marco.exe", Path: l.Marco, Program: true},
		{Name: "director.exe", Path: l.Director, Program: true},
		{Name: "marco-macros.exe", Path: l.Macros, Program: true},
		{Name: "overlay.exe", Path: l.Overlay, Program: true},
		{Name: "overlay.marco", Path: l.Script},
	}
}

// Args is the command after marco.exe: the installer's `runOverlay`, spelled for this layout.
func (l Layout) Args() []string {
	return []string{"serve",
		"--host", "OS=bridge:" + l.Macros,
		"--host", "Overlay=bridge:" + l.Overlay,
		l.Script}
}

// Launcher is what a person would run themselves to start this stack and see what it says.
func (l Layout) Launcher() string {
	if l.Kind == "tree" {
		return filepath.Join(l.Dir, "overlay.cmd")
	}
	return "Marco.exe --overlay"
}

// IsOverlay is whether an executable at path is this stack's overlay or any other Marco widget:
// an overlay.exe with an overlay.marco beside it. A widget from another installation is still a
// widget holding the keyboard hook, and a second one would fight it for the leader.
func IsOverlay(path string) bool {
	if !equalFold(filepath.Base(path), "overlay.exe") {
		return false
	}
	return exists(filepath.Join(filepath.Dir(path), "overlay.marco"))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
