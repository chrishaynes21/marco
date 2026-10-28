// Command marco-app is the bundled, self-extracting Marco for Windows. pack.ps1 embeds Studio
// (studio.exe), the engine (marco.exe, marco-macros.exe), the Director service (director.exe),
// the accessibility provider (uia.exe), the legacy overlay (overlay.exe + overlay.marco) and a
// manifest of what each piece is, into this single binary. On launch it unpacks them under
// %LOCALAPPDATA%\Marco and opens STUDIO — so a user double-clicks one Marco.exe with no
// toolchain.
//
// # Studio is the front door, and the overlay is asked for
//
// The product is record → Stop → review → Save → Play, and the review is the half that cannot
// be had from a command line. This launcher ran the overlay serve stack, which meant an
// installed Marco could record and could not show anybody what it was about to write — so every
// guard that depends on a review was unreachable to whoever installed it. The overlay is still
// here, still works, and starts only when asked with --overlay.
//
// # Nothing is told where anything is, except the store
//
// The Director, the accessibility provider and Studio's own two children are laid out where each
// LOOKS for them: director.exe beside marco.exe, plugins\uia\uia.exe under it, and marco.exe and
// director.exe beside studio.exe — which is the same directory, so one layout satisfies all of
// it. The store and the library ARE stated, because this launcher is what decides which
// installation this is; see run.
//
// It is its own module (stdlib only) so a plain `go build ./...` in the engine never needs the
// staged binaries; build the bundle with pack.ps1.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// The staged stack, embedded at build time by pack.ps1.
//
// # Why the DIRECTORY and not a list of files
//
// Because `//go:embed` is a COMPILE-time pattern: a named file that does not exist is a build
// error, not a missing asset. None of these files exist in a fresh checkout — pack.ps1 builds
// and stages them first — so naming them individually meant this module did not compile until
// someone had packed it, and adding a name broke every tree that had packed the old set.
//
// `all:assets` matches the directory, which is tracked (assets/.gitkeep) and therefore always
// present. The module compiles on a clean clone, and what it CARRIES is decided by what
// pack.ps1 staged. The check that moved from compile time to run time is extractBins below,
// which names the missing file; pack.ps1 verifies the same list before it builds, so a
// forgotten asset still fails on whoever ships rather than on whoever installs.
//
//go:embed all:assets
var assets embed.FS

// staged is what gets unpacked, and WHERE — the embed name on the left, the path under bin\
// on the right.
//
// # Why the destination is not always the file name
//
// Because two of these are found by LOOKING rather than by being told. cmd/marco's
// directorBin() looks for director.exe beside marco.exe; accessibilityBridge() looks for
// plugins/uia/uia.exe beside marco.exe. Laying them out that way is the whole reason an
// installed Marco has a Director and an actor without a single environment variable — and
// putting uia.exe flat in bin\ would silently give the Theater nobody to cast again.
//
// This table and the //go:embed line above must name the same files. Changing one and not
// the other must fail TestThePackagedLayoutIsWhatMarcoLooksFor in cmd/marco.
var staged = []struct{ asset, dest string }{
	{"studio.exe", "studio.exe"},
	{"marco.exe", "marco.exe"},
	{"marco-macros.exe", "marco-macros.exe"},
	{"director.exe", "director.exe"},
	{"uia.exe", "plugins/uia/uia.exe"},
	{"overlay.exe", "overlay.exe"},
	{"overlay.marco", "overlay.marco"},
	{"manifest.json", "manifest.json"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Marco:", err)
		os.Exit(1)
	}
}

func run() error {
	base := filepath.Join(localAppData(), "Marco")
	binDir := filepath.Join(base, "bin")
	routesDir := filepath.Join(base, "routes")

	ver, _ := assets.ReadFile("assets/version.txt")
	if err := extractBins(binDir, string(ver)); err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	// Seed a single "hello" starter route on first run only; a user's later edits (and any
	// routes they learn) are never overwritten.
	if _, err := os.Stat(routesDir); os.IsNotExist(err) {
		if err := seedHello(routesDir); err != nil {
			return fmt.Errorf("seed hello route: %w", err)
		}
	}

	// THE SAME STORE AND LIBRARY EVERY BINARY RESOLVES, SAID OUT LOUD.
	//
	// `internal/marcopaths` is the one place that answers where the routine library and the
	// Marco home are, for `marco`, `director` and Studio alike — and its default is `routes`
	// INSIDE the store, resolved absolutely. Naming both here is not a second answer: it is
	// this launcher stating which store this installation uses, so nothing downstream
	// depends on a working directory. See the library note in CLAUDE.md.
	os.Setenv("MARCO_HOME", base)
	os.Setenv("MARCO_BIN", filepath.Join(binDir, "marco.exe"))
	os.Setenv("MARCO_ROUTES", routesDir)

	rest := os.Args[1:]
	if legacy, args := wantsTheOverlay(rest); legacy {
		return runOverlay(binDir, args)
	}
	fmt.Printf("Marco %s — routes at %s\n", string(ver), routesDir)
	return runStudio(binDir, rest)
}

// overlayFlag is how somebody asks for the old front door.
//
// # Why the overlay is still here and is not started
//
// Because it is a different product surface, not an older version of this one: a command line
// over a hotkey, with no way to show anybody what a recording became before it is saved. That
// made it the right thing to ship when it was the only thing there was, and it makes it the
// wrong default now — a person who installs Marco and gets the overlay cannot see a review, so
// every guard that depends on one is unreachable to them.
//
// It is not removed, because people use it and because `marco` itself is unchanged underneath
// both. It is asked for.
const overlayFlag = "--overlay"

// wantsTheOverlay reports whether the legacy surface was asked for, and the rest of the arguments.
func wantsTheOverlay(args []string) (bool, []string) {
	for i, a := range args {
		if a == overlayFlag {
			return true, append(append([]string{}, args[:i]...), args[i+1:]...)
		}
	}
	return false, args
}

// runStudio opens the window, which is this alpha's front door.
//
// # Why the window and not the serve stack
//
// Because the product is record → Stop → review → Save → Play, and the review is the half that
// cannot be had from a command line. Studio spawns the `marco.exe` and `director.exe` beside
// ITSELF, which is why they are laid out in the same directory it is — see `staged`.
func runStudio(binDir string, args []string) error {
	studio := filepath.Join(binDir, "studio.exe")
	cmd := exec.Command(studio, args...)
	// FROM ITS OWN DIRECTORY, so a child it spawns by a relative name cannot be resolved
	// against whatever directory this launcher happened to be started from.
	cmd.Dir = binDir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// runOverlay is the legacy surface, which mirrors overlay.cmd.
func runOverlay(binDir string, args []string) error {
	marco := filepath.Join(binDir, "marco.exe")
	full := []string{"serve",
		"--host", "OS=bridge:" + filepath.Join(binDir, "marco-macros.exe"),
		"--host", "Overlay=bridge:" + filepath.Join(binDir, "overlay.exe"),
		filepath.Join(binDir, "overlay.marco")}
	full = append(full, args...)
	cmd := exec.Command(marco, full...)
	cmd.Dir = binDir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func localAppData() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d
	}
	return os.TempDir()
}

// extractBins unpacks the embedded binaries into binDir, skipping the copy when the stamped
// version already matches — so relaunching doesn't relock a running instance's exes.
func extractBins(binDir, ver string) error {
	stamp := filepath.Join(binDir, ".version")
	if ver != "" {
		if b, err := os.ReadFile(stamp); err == nil && string(b) == ver {
			return nil
		}
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, s := range staged {
		data, err := assets.ReadFile("assets/" + s.asset)
		if err != nil {
			// The embed is a directory now, so a missing asset arrives here rather than
			// as a build error. Say which one: an installer that shipped without the
			// Director or without the accessibility provider looks, to its user, exactly
			// like a Marco that ignores them.
			return fmt.Errorf("this build was packaged without %s — re-run pack.ps1", s.asset)
		}
		dst := filepath.Join(binDir, filepath.FromSlash(s.dest))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			// IS IT RUNNING, OR IS IT BROKEN? Two different things to be told.
			//
			// Windows refuses to overwrite a file that is executing, so an upgrade
			// while Studio is open fails exactly here — and "unpack: Access is
			// denied" sends somebody to look at permissions for a thing that is
			// simply open. Nothing is killed: a window that is recording, or has a
			// draft in it that exists nowhere else, is not something an installer
			// may close on somebody's behalf. So it says what to do and stops.
			if errors.Is(err, fs.ErrPermission) || isInUse(err) {
				return fmt.Errorf("%s is in use, so this version could not be "+
					"installed. Close Marco — including the Studio window "+
					"and anything it started — and run this again. Nothing "+
					"has been changed", s.dest)
			}
			return err
		}
	}
	return os.WriteFile(stamp, []byte(ver), 0o644)
}

// helloRoute is the one starter route the bundle drops in — a friendly global "hello" so a
// first-run user has something to run (and to see what a route looks like in `marco edit`).
const helloRoute = `// Marco starter route — a friendly hello. Edit or delete it freely.
use os.

the Hello is an actor.
this can Run.
this's Run does...
    do OS's Type with "hello".
    do OS's Sleep with 500.
    this is ok!

the App is a script.
do Hello's Run...
    when ok?
        log "hello: done".
    or?
        log that's error.
`

// seedHello writes the starter hello route into the global scope.
func seedHello(routesDir string) error {
	dst := filepath.Join(routesDir, "global", "hello.marco")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(helloRoute), 0o644)
}

// isInUse reports whether a write failed because something is executing the file.
//
// Windows answers ERROR_SHARING_VIOLATION (32) or ERROR_LOCK_VIOLATION (33) for that, and
// neither maps onto `fs.ErrPermission` — so a check on permission alone reads "Marco is open" as
// "your account cannot write here", which is the wrong thing to go and look at.
//
// By NUMBER rather than by message text, because the message is localised and a check on English
// words is a check that works on the machine it was written on.
func isInUse(err error) bool {
	var e syscall.Errno
	if !errors.As(err, &e) {
		return false
	}
	return e == 32 || e == 33
}
