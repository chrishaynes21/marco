// Command marco-app is the bundled, self-extracting Marco for Windows. pack.ps1 embeds the
// engine (marco.exe, marco-macros.exe), the Director service (director.exe), the accessibility
// provider (uia.exe), the overlay (overlay.exe + overlay.marco), and the starter routes into
// this single binary. On launch it unpacks them under %LOCALAPPDATA%\Marco and runs the overlay
// serve stack — so a user double-clicks one Marco.exe with no toolchain.
//
// The Director and the accessibility provider are not passed to anything: they are laid out
// where marco.exe already LOOKS for them (director.exe beside it, plugins\uia\uia.exe under it),
// so a packaged install has a Director to talk to and an actor to cast without configuration.
// Ship them and a learned play performs; omit them and it is recognised, delivered, and does
// nothing visible.
//
// It is its own module (stdlib only) so a plain `go build ./...` in the engine never needs the
// staged binaries; build the bundle with pack.ps1.
package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	{"marco.exe", "marco.exe"},
	{"marco-macros.exe", "marco-macros.exe"},
	{"director.exe", "director.exe"},
	{"uia.exe", "plugins/uia/uia.exe"},
	{"overlay.exe", "overlay.exe"},
	{"overlay.marco", "overlay.marco"},
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
	// routes they teach) are never overwritten.
	if _, err := os.Stat(routesDir); os.IsNotExist(err) {
		if err := seedHello(routesDir); err != nil {
			return fmt.Errorf("seed hello route: %w", err)
		}
	}

	marco := filepath.Join(binDir, "marco.exe")
	os.Setenv("MARCO_BIN", marco)
	os.Setenv("MARCO_ROUTES", routesDir)
	fmt.Printf("Marco %s — routes at %s\n", string(ver), routesDir)

	// Mirror overlay.cmd: marco serve wires the OS + Overlay bridge hosts and runs overlay.marco.
	args := []string{"serve",
		"--host", "OS=bridge:" + filepath.Join(binDir, "marco-macros.exe"),
		"--host", "Overlay=bridge:" + filepath.Join(binDir, "overlay.exe"),
		filepath.Join(binDir, "overlay.marco")}
	args = append(args, os.Args[1:]...) // pass through any extra flags
	cmd := exec.Command(marco, args...)
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
