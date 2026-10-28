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
// # Nothing is told where anything is
//
// The Director, the accessibility provider and Studio's own two children are laid out where each
// LOOKS for them: director.exe beside marco.exe, plugins\uia\uia.exe under it, and marco.exe and
// director.exe beside studio.exe — which is the same directory, so one layout satisfies all of
// it. The store and the library are NOT stated: `internal/marcopaths` answers where they are, for
// every binary at once, and an installer that answered too would be a second resolver. See run,
// and `keepRelativeOverrides` for the one thing this launcher does to a path somebody else set.
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
	"strings"
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

	ver, _ := assets.ReadFile("assets/version.txt")
	if err := install(binDir, string(ver)); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// ── WHERE THE BINARIES ARE IS THIS LAUNCHER'S BUSINESS. WHERE THE DATA IS, IS NOT ──
	//
	// `MARCO_BIN` names an executable, which is a fact about this installation and the one
	// thing here worth stating: it is the marco this bundle unpacked, and a child that
	// looked elsewhere would be a different build.
	//
	// `MARCO_HOME` and `MARCO_ROUTES` are NOT set, and setting them was a defect of this
	// slice's own making. `internal/marcopaths` is the one place that answers where the
	// store and the library are — `$MARCO_HOME`, else `%APPDATA%\marco` — for `marco`,
	// `director` and Studio alike, and an installer that overrode it took two things away
	// at once: an explicit `$MARCO_HOME` somebody had set, and the shared default every
	// other way into Marco uses. An existing Studio user would have installed the alpha
	// and been shown an EMPTY LIBRARY, with their routines exactly where they left them
	// and nothing saying so.
	//
	// So installation and data resolution are separate. Nothing here reads the library,
	// writes to it, or names it — and nothing here seeds a starter routine into it either:
	// see the note on seedHello for why that went, and for the one thing it cost.
	//
	// An older BUNDLED installation does have data under this same directory, because the
	// launcher used to put it there. That is a conflict rather than a thing to migrate, and
	// Studio names it: see `whatOtherLibraryIsHere` in plugins/studio, which is where the
	// one resolver lives and where a person can act on the answer. Nothing is moved or
	// merged, by anybody.
	//
	// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
	os.Setenv("MARCO_BIN", filepath.Join(binDir, "marco.exe"))
	if err := keepRelativeOverrides(); err != nil {
		return err
	}
	fmt.Printf("Marco %s\n", string(ver))

	rest := os.Args[1:]
	if legacy, args := wantsTheOverlay(rest); legacy {
		return runOverlay(binDir, args)
	}
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

// dataOverrides are the environment variables that say where somebody's data is.
//
// The two `internal/marcopaths` reads, named here as strings because this is its own module and
// cannot import it — which is also why nothing here derives what they MEAN. A list rather than
// every variable in the environment because most of them are not paths: a rule that rewrote
// `MARCO_VISION_SIZE` or `MARCO_STOP_KEY` would be a rule about words that happen to look like
// files.
var dataOverrides = []string{"MARCO_HOME", "MARCO_ROUTES"}

// keepRelativeOverrides makes a relative override mean the directory it meant where it was typed.
//
// # The defect
//
// The children are started from `bin\` — see runStudio — so a sibling they spawn by a bare name
// cannot be resolved against whatever directory somebody double-clicked this launcher in. That
// moves the meaning of a RELATIVE `$MARCO_HOME` or `$MARCO_ROUTES` with it: `MARCO_ROUTES=routes`
// in a terminal means `.\routes` THERE, and the same words through this launcher meant
// `bin\routes` — a different library from the same invocation, which is the defect the one
// resolver exists to prevent, arriving by the back door.
//
// # Why this is not a second resolver
//
// Because it answers nothing. An override that is absent stays absent, an absolute one is
// untouched, and `marcopaths` decides what any of it means as it always did. The only thing done
// here is to spell a path somebody else wrote against the directory they wrote it in, which is
// the one fact this process has and the child does not.
//
// Done before the child's directory changes, and for the overlay as well as for Studio: both
// start from `bin\`.
func keepRelativeOverrides() error {
	for _, name := range dataOverrides {
		was := strings.TrimSpace(os.Getenv(name))
		if was == "" || filepath.IsAbs(was) {
			continue
		}
		// AGAINST THIS PROCESS'S DIRECTORY, which is the invocation's: this runs
		// before any child is started from anywhere else.
		abs, err := filepath.Abs(was)
		if err != nil {
			return fmt.Errorf("%s is %s, and this directory cannot be resolved to "+
				"say which one that is: %w", name, was, err)
		}
		if err := os.Setenv(name, abs); err != nil {
			return fmt.Errorf("%s could not be set to %s: %w", name, abs, err)
		}
	}
	return nil
}

func localAppData() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d
	}
	return os.TempDir()
}

// install puts this bundle's binaries where Studio and marco look for them, COMPLETE OR NOT AT ALL.
//
// # What the previous version did, and the state it could leave
//
// It wrote each file into `bin\` in turn. Windows refuses to overwrite an executing file, so an
// upgrade while anything from the last one is running fails PART WAY — and the files before the
// one that failed had already been replaced. The error said "nothing has been changed", which was
// the one thing that was not true: a new Studio beside an old marco and an old director is
// exactly the mixed installation the whole sibling discipline exists to prevent, and it looks
// current from every direction.
//
// # So a version is staged whole and then becomes the one in use
//
// Each bundle unpacks into its OWN directory — `bin-<version>` — and `bin\` is replaced by it
// only once every file is written. Staging into a fresh directory can fail halfway and cost
// nothing: nothing is using it, and the installation in `bin\` has not been touched.
//
// The activation is a directory swap, which is the one step that can still fail against running
// binaries — so it is tried, and on failure the OLD `bin\` is put back and the message says to
// close Marco. Either the whole new version is in use or the whole old one is, and never a
// mixture of the two.
//
// Previous versions are left on disk rather than deleted. A directory that was in use a moment
// ago is not something to remove while its processes may still be exiting, and it is the thing
// that makes the fallback above possible.
func install(binDir, ver string) error {
	// ALREADY THIS VERSION: nothing to do, and nothing touched. The stamp lives inside the
	// active directory, so a half-activated install cannot claim to be complete.
	if ver != "" {
		if b, err := os.ReadFile(filepath.Join(binDir, stampName)); err == nil &&
			string(b) == ver {
			return nil
		}
	}
	staged := binDir + "-" + safeName(ver)
	if err := os.RemoveAll(staged); err != nil {
		return fmt.Errorf("clearing %s: %w", staged, err)
	}
	if err := stage(staged, ver); err != nil {
		// NOTHING IS IN USE HERE, so there is nothing to put back. The half-written
		// staging directory is left for the next attempt to clear.
		return err
	}
	return activate(binDir, staged)
}

// stage writes the whole bundle into a directory nothing is using.
func stage(dir, ver string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, s := range staged {
		data, err := assets.ReadFile("assets/" + s.asset)
		if err != nil {
			// The embed is a directory, so a missing asset arrives here rather than
			// as a build error. Say which one: an installer that shipped without the
			// Director or without the accessibility provider looks, to its user,
			// exactly like a Marco that ignores them.
			return fmt.Errorf("this build was packaged without %s — re-run pack.ps1",
				s.asset)
		}
		dst := filepath.Join(dir, filepath.FromSlash(s.dest))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			return fmt.Errorf("writing %s: %w", s.dest, err)
		}
	}
	// THE STAMP LAST, INSIDE THE VERSION IT DESCRIBES. A stamp written first would
	// describe a directory that is not finished yet, and a staging directory that is
	// never activated would then be indistinguishable from an installation.
	return os.WriteFile(filepath.Join(dir, stampName), []byte(ver), 0o644)
}

// activate makes a staged directory the one in use, or leaves the old one exactly as it was.
//
// # The three ways out, and the third one used to claim the second one's guarantee
//
// It activates; or it fails and the version that was there is back; or it fails and the recovery
// cannot be completed either — which is the one case where somebody is left with no Marco at all.
// The previous version of this ignored both the clearing of the recovery directory and the
// rollback's own error, so the third case reported itself as the second: "the version you had is
// still the one in use", about an installation that was not there. That is the one sentence a
// person in that position must not be told, because it sends them to look at a running Marco
// instead of at their own disk.
//
// So each of those errors is handled, and the message for the third names the directory their
// previous version is in and what to do with it.
//
// # What is left on disk, and why
//
// `bin-previous` is the version that was in use, moved aside for the length of one rename. It is
// cleared only when there IS an installation to move into it — so a rollback that failed leaves
// it alone: `bin\` does not exist in that case, there is nothing to move aside on the next
// attempt, and the one copy of their previous version survives until something is activated over
// it.
func activate(binDir, staged string) error {
	recovery := binDir + "-previous"
	movedAside := false
	if _, err := os.Stat(binDir); err == nil {
		// THE RECOVERY DIRECTORY HAS TO BE EMPTY FIRST, and this is where that is
		// decided rather than hoped for. An earlier attempt can have left one, and a
		// rename onto it fails — which was reported as the installation being in
		// use, which is a different thing to go and do something about.
		if err := os.RemoveAll(recovery); err != nil {
			return fmt.Errorf("the previous version at %s could not be cleared, so "+
				"there is nowhere to keep the version you have while this one "+
				"is installed. Nothing has been changed: %w", recovery, err)
		}
		if err := rename(binDir, recovery); err != nil {
			return inUse(err, "Marco")
		}
		movedAside = true
	}
	if err := rename(staged, binDir); err != nil {
		if !movedAside {
			// NOTHING WAS MOVED ASIDE, so there was no version here to lose: this
			// is a first install that did not finish.
			return inUse(err, "Marco")
		}
		// PUT THE OLD ONE BACK. The window between these two renames is the only
		// moment this installation is not whole, and leaving it there would be worse
		// than the mixed binaries this function exists to avoid: there would be no
		// Marco at all.
		if rerr := rename(recovery, binDir); rerr != nil {
			return fmt.Errorf("this version could not be installed (%v), and the "+
				"version you had could not be put back (%v). It is complete "+
				"and it is at %s — close Marco and anything it started, then "+
				"run this again, or rename that directory to %s: %w",
				err, rerr, recovery, binDir, err)
		}
		return fmt.Errorf("this version was not installed, and the version you had is "+
			"still the one in use. Close Marco — the Studio window and anything "+
			"it started — and run this again: %w", err)
	}
	return nil
}

// rename is os.Rename, replaced by a test to arrange a failure that is a MOMENT rather than a
// state.
//
// A rollback that cannot complete needs the SECOND rename above to fail after the first has
// already happened, and no arrangement of files on disk does that: whatever makes a directory
// unmovable makes it unmovable in both directions, and a test cannot step between two adjacent
// lines. The alternative to a seam is an untested recovery path, and this is the path that
// decides whether somebody still has a Marco.
//
// The first rename is deliberately NOT through it. That one is reachable from a test by ordinary
// means — a directory in the way is enough — and a seam in front of it would be a second way to
// fake what a real upgrade already does.
var rename = os.Rename

// inUse says what to do about a file something is holding, or passes the error on.
//
// Windows answers ERROR_SHARING_VIOLATION (32) or ERROR_LOCK_VIOLATION (33) for that, and
// neither maps onto `fs.ErrPermission` — so a check on permission alone reads "Marco is open" as
// "your account cannot write here", which is the wrong thing to go and look at. By NUMBER rather
// than by message text, because the message is localised.
//
// Nothing is killed. A window that is recording, or holding a draft that exists nowhere else, is
// not something an installer may close on somebody's behalf.
func inUse(err error, what string) error {
	if errors.Is(err, fs.ErrPermission) || isInUse(err) {
		return fmt.Errorf("%s is still running, so this version was not installed. "+
			"Close it — the Studio window and anything it started — and run this "+
			"again. The version you had is still the one in use", what)
	}
	return err
}

// stampName is the file inside an active bundle that says which version it is.
const stampName = ".version"

// safeName is a version turned into something that can be a directory name.
func safeName(ver string) string {
	if ver == "" {
		return "staged"
	}
	out := make([]rune, 0, len(ver))
	for _, r := range ver {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r == '-', r == '.', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
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
