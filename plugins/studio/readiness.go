package main

// WHETHER THIS WINDOW CAN DO THE TWO THINGS IT IS FOR, ASKED BEFORE SOMEBODY TRIES.
//
// # What was wrong
//
// Every way of failing here was discovered by failing. The recorder refusing a platform arrived
// at Stop, after a whole demonstration. A missing `marco.exe` arrived at Play, as a spawn error
// over a game somebody had just arranged. A library that could not be written arrived at Save,
// which is the one moment a recording exists nowhere else. Each of those is knowable in advance
// and none of them was said in advance.
//
// Worse than late: a missing sibling `marco.exe` does not fail at all. `marcoBin` falls back to
// the bare name, Windows resolves it on PATH, and playback runs whichever `marco.exe` that finds —
// which may be a different build of a different age. That is the trap this project has paid for
// twice (see internal/buildid), and it is silent.
//
// # What this is NOT, and the line is the point
//
// It is not a promise that anything will work. It says what has been ESTABLISHED, by doing the
// check, and `Limits` says out loud what it did not establish: that the hooks will install, that
// the application is running, that a press will land, that the screen will change. A readiness
// panel that implied any of those would be the same false claim as a green suite over an untested
// moment, moved to the front of the window where somebody trusts it before they start.
//
// So every problem below comes from an action taken now:
//
//	the recorder     a platform fact — a backend exists, or none does
//	the executables  os.Stat beside this binary, then exec.LookPath, which is what the spawn does
//	the library      a file CREATED and removed in it, which is what Save will do
//	the build        this binary's own stamp, which is either there or is honestly "unknown"
//
// See [[ADR-254-what-a-recording-captures-is-stated]] for the slice this is part of.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// Readiness is what this window can establish about itself.
type Readiness struct {
	// Build is this Studio's revision, or "unknown" when the toolchain left no stamp.
	//
	// First, because a diagnostic that does not say what it is reporting ON is one
	// somebody reads about the wrong binary. That is not a hypothetical here: a window
	// and a recorder built a day apart cost the owner a day of recordings, and the only
	// way to tell was comparing file timestamps. See internal/buildid.
	Build string `json:"build"`
	// Installation is what this installed Marco is made of, piece by piece — each Go
	// binary's revision and every file's hash, from the manifest the bundle carries, with
	// the hashes verified against what is on disk.
	//
	// Separate from `Build` because `Build` is THIS binary and a bundle is five of them,
	// and because the one piece that is not Go has no revision to report: the
	// accessibility provider is built by the .NET Framework compiler that ships with
	// Windows and is identified by its bytes alone. A line that showed only this window's
	// revision would be a label standing in for an identity. See
	// whatThisInstallationIsMadeOf.
	Installation string `json:"installation,omitempty"`
	// Marco and Director are the executables this window would spawn, and Library the
	// directory it reads and writes routines in. Shown whether or not anything is wrong
	// with them: this is the line somebody checks when the window and the command
	// disagree about what exists.
	Marco    string `json:"marco"`
	Director string `json:"director"`
	Library  string `json:"library"`
	// Problems is everything established to be wrong, each with what it stops and what
	// to do about it. Empty is the ordinary case and the page says so.
	Problems []Problem `json:"problems"`
	// Limits is what this check did not establish. Always present, never conditional: a
	// panel that dropped it when everything passed would read as a promise exactly when
	// somebody was most likely to believe one.
	Limits string `json:"limits"`
}

// Problem is one thing that is wrong, in the three parts a person needs.
type Problem struct {
	// Says is what is wrong, as a statement about this machine.
	Says string `json:"says"`
	// Blocks is what cannot be done until it is fixed, in the window's own words.
	// Empty means it blocks nothing and is worth knowing anyway.
	Blocks string `json:"blocks,omitempty"`
	// Do is the action that would fix it, or "" when there is nothing to suggest.
	//
	// Named rather than left to a person to work out, because every one of these has
	// exactly one answer and withholding it makes a diagnostic into a riddle.
	Do string `json:"do,omitempty"`
}

// readinessLimits is what a readiness check cannot answer, and it is not negotiable.
//
// Spelled out because the honest half of this panel is the half that says what it does not know.
// Installing the hooks takes the keyboard and the mouse of the whole desktop, so nothing here can
// have tried it; and nothing here has looked at a screen, started an application or sent a press.
const readinessLimits = "This is what Marco has checked on this machine just now. It does " +
	"not say the input hooks will install, that the application you are recording is " +
	"running, that a press will land where you pressed it, or that what you are " +
	"automating will do what you want. Nothing here looks at a screen."

// Ready is what this window can establish about its ability to record and to play.
//
// # Asked again rather than once at start-up
//
// Because every one of these changes while the window is open: a build script replaces the
// sibling executables, a drive fills up, somebody moves the library. A panel answered once would
// describe the machine as it was when Studio opened — which is the shape of defect that cost the
// owner a day of recordings, arriving in the place somebody goes to rule it out.
//
// # And the cost of asking, which is real and is accepted
//
// The library check WRITES, because nothing else is an answer, so this is a file created and
// removed in somebody's routine library every time it is asked. The page therefore polls it on
// its own slow cadence rather than with the rest, and never during a recording or a run: the
// alternative to asking is learning the answer at Save, which is the one moment a recording
// exists nowhere else. `Registry.Save` does the same write, and `Routines` skips the file either
// way — it is not a `.marco` and has no entrance.
func (a *Acts) Ready() Readiness {
	made, madeProblems := whatThisInstallationIsMadeOf()
	out := Readiness{
		Build:        a.myBuild(),
		Installation: made,
		Marco:        marcoBin(),
		Director:     directorBin(),
		Library:      a.routesDir,
		Problems:     []Problem{},
		Limits:       readinessLimits,
	}
	out.Problems = append(out.Problems, madeProblems...)
	// ── WHICH BINARY IS THIS ──
	//
	// "unknown" is what `buildid` answers when the toolchain left no VCS stamp, which is
	// what happens to a package built from outside its own module — the exact mistake
	// `build.cmd` used to make with the director. Reported rather than shown as a word,
	// because a window that cannot name itself cannot be compared with anything.
	if out.Build == "" || out.Build == "unknown" {
		out.Problems = append(out.Problems, Problem{
			Says: "this Studio cannot say which build it is, so it cannot be " +
				"compared with the marco and director beside it",
			Do: "build it with plugins\\studio\\build.cmd, which builds each " +
				"binary from inside its own module so the revision is stamped",
		})
	}
	// ── CAN ANYTHING BE RECORDED AT ALL ──
	if !recorder.Available() {
		out.Problems = append(out.Problems, Problem{
			Says:   "this platform has no input recorder, so nothing can be watched",
			Blocks: "recording",
			Do: "Marco records on Windows. macOS and Linux have no input " +
				"backend yet, and nothing here can work around that",
		})
	}
	// ── AND THE TWO CHILDREN, WHERE THE SPAWN WILL LOOK FOR THEM ──
	out.Problems = append(out.Problems,
		whereverTheSpawnWouldFind("marco.exe", out.Marco, "playing a routine",
			"Play would run whichever marco.exe is on PATH, which may be a "+
				"different build of a different age")...)
	out.Problems = append(out.Problems,
		whereverTheSpawnWouldFind("director.exe", out.Director,
			"recording a demonstration Marco watches, and composing an Act",
			"it would run whichever director.exe is on PATH, which may be a "+
				"different build of a different age")...)
	// ── AND CAN A ROUTINE BE KEPT ──
	if p, bad := aLibraryThatCanBeWritten(out.Library); bad {
		out.Problems = append(out.Problems, p)
	}
	return out
}

// whereverTheSpawnWouldFind asks about one child executable the way the spawn will.
//
// # Three answers, not two
//
// Beside this binary is right. Nowhere is a failure. The third is the one that matters: ON PATH
// BUT NOT BESIDE. `marcoBin` and `directorBin` fall back to the bare name when the sibling is
// missing, so the spawn succeeds and runs a binary from somewhere else — a different build, of a
// different age, performing somebody's routine or recording their demonstration. It has cost this
// project a day of recordings once already, and it never looks like a fault.
//
// `exec.LookPath` is used because it is what `exec.Command` itself uses for a bare name. A check
// that searched PATH its own way would be answering about a resolution the spawn does not perform.
func whereverTheSpawnWouldFind(name, resolved, blocks, elsewhere string) []Problem {
	// AN ABSOLUTE PATH MEANS THE SIBLING WAS FOUND, or that somebody named one. Either
	// way the spawn will use exactly that file, so the only question is whether it is
	// still there — asked again, because a build script can have replaced it since.
	if filepath.IsAbs(resolved) {
		if _, err := os.Stat(resolved); err == nil {
			return nil
		}
		return []Problem{{
			Says:   fmt.Sprintf("%s is named but is not there: %s", name, resolved),
			Blocks: blocks,
			Do: "build it and put it beside this window, or point " +
				strings.ToUpper(strings.TrimSuffix(name, ".exe")) +
				"_BIN at one that exists",
		}}
	}
	// A BARE NAME MEANS THE SIBLING IS MISSING and the fallback is in play.
	found, err := exec.LookPath(resolved)
	if err != nil {
		return []Problem{{
			Says: fmt.Sprintf("there is no %s beside this window and none on "+
				"PATH either", name),
			Blocks: blocks,
			Do: "run plugins\\studio\\build.cmd, which makes every binary " +
				"and puts them beside each other",
		}}
	}
	return []Problem{{
		Says: fmt.Sprintf("there is no %s beside this window. %s is on PATH, so "+
			"%s", name, found, elsewhere),
		Blocks: "",
		Do: "run plugins\\studio\\build.cmd with this window CLOSED, which " +
			"puts a freshly built one beside it",
	}}
}

// aLibraryThatCanBeWritten establishes whether a routine could be kept in this directory.
//
// # By writing, because nothing else is an answer
//
// A directory that exists is not a directory that can be written to, and the difference arrives
// at Save — the one moment a recording exists nowhere else. Permissions, a full disk, a path
// that has become a file, a drive that has gone away: each of them reads as "the directory is
// there" to anything short of a write.
//
// So this creates a file and removes it, which is what `Registry.Save` will do, and it does it in
// the library rather than in a temporary directory, because the question is about THIS place. The
// file is named so that one left behind by a crash is recognisable as this and not as somebody's
// routine, and `Routines` skips it anyway: it is not a `.marco` file and has no entrance.
func aLibraryThatCanBeWritten(dir string) (Problem, bool) {
	if strings.TrimSpace(dir) == "" {
		return Problem{
			Says:   "this window has no library to read or write routines in",
			Blocks: "recording and playing",
			Do:     "set MARCO_ROUTES to a directory and start Marco again",
		}, true
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Problem{
			Says: fmt.Sprintf("the library %s cannot be made: %v", dir, err),
			// SAVE AND PLAY BOTH, because a library that cannot be made is
			// one nothing can be read out of either.
			Blocks: "keeping a recording, and finding one afterwards",
			Do:     "set MARCO_ROUTES to a directory you can write to",
		}, true
	}
	f, err := os.CreateTemp(dir, ".studio-can-write-*")
	if err != nil {
		return Problem{
			Says: fmt.Sprintf("nothing can be written into the library %s: %v",
				dir, err),
			Blocks: "keeping a recording — Save would fail with the recording " +
				"held nowhere else",
			Do: "set MARCO_ROUTES to a directory you can write to, or free " +
				"space on this drive",
		}, true
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return Problem{}, false
}

// theBundleIsIncomplete names a child this window cannot spawn safely, or "" when it can.
//
// # Why a bare name is a refusal and not a fallback
//
// `marcoBin` and `directorBin` answer an absolute path when the sibling is there or when
// somebody named one, and a BARE NAME when neither is true. A bare name handed to
// `exec.Command` is resolved against PATH — so on a machine with a development tree on PATH,
// an installed Studio with a missing sibling performs routines through a binary nobody chose,
// of whatever age, and every window in sight looks current. That is the defect that cost the
// owner a day of recordings, arriving from the one direction a packaged install can take it.
//
// `Ready` has reported this for a while and did not stop it: the problem it raised for the
// on-PATH case blocked nothing, so the window warned and then spawned it anyway. For a shipped
// alpha that has to be a refusal, because an incomplete bundle is a thing to be told about
// rather than a thing to work around — see
// [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
//
// The explicit overrides still win. `$MARCO_BIN` and `$DIRECTOR_BIN` are statements by whoever
// started this window, and an absolute path that does not exist is a different complaint, made
// by `whereverTheSpawnWouldFind` and reported in the panel.
func theBundleIsIncomplete(what, resolved string) string {
	if filepath.IsAbs(resolved) {
		return ""
	}
	return fmt.Sprintf("this Marco is incomplete: there is no %s beside this window, and "+
		"running whichever one is on PATH would be running a build nobody here "+
		"chose. Reinstall Marco, or point %s_BIN at the one you mean",
		what, strings.ToUpper(strings.TrimSuffix(what, ".exe")))
}

// ── WHAT THIS INSTALLATION IS MADE OF ─────────────────────────────────────────

// bundled is one piece of an installed Marco, as the bundle's manifest records it.
type bundled struct {
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
	Revision string `json:"revision"`
}

// bundleManifest is what `pack.ps1` wrote beside these binaries.
type bundleManifest struct {
	Version string    `json:"version"`
	Packed  string    `json:"packed"`
	Files   []bundled `json:"files"`
}

// whatThisInstallationIsMadeOf reports each piece of a packaged Marco, or nothing in a tree.
//
// # Why the manifest and not each binary's own answer
//
// Because only some of them can answer. `internal/buildid` reads the VCS stamp `go build` leaves,
// which works for every Go binary here and does not exist for the accessibility provider — that
// one is built by the .NET Framework `csc.exe` that ships with Windows and carries no such stamp.
// A report that asked each binary what it was would therefore be silent about the one component
// whose identity nobody can otherwise establish.
//
// So `pack.ps1` asks each Go binary at pack time, through `go version -m`, and hashes everything.
// A revision says which SOURCE a binary came from; a hash says whether THIS is that binary. The
// manifest carries both, and this verifies the hashes against what is actually on disk — because
// a manifest is a claim, and a claim about files anybody can replace is worth exactly as much as
// the check behind it.
//
// # And a tree is not a broken installation
//
// A development build has no manifest and never will: `build.cmd` makes the binaries and writes
// nothing beside them. That is reported as what it is rather than as a problem, because the
// window's own `Build` and the two sibling checks already cover a tree — and a diagnostic that
// cried about a development build would be one nobody reads on an installation.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func whatThisInstallationIsMadeOf() (string, []Problem) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil
	}
	at := filepath.Join(filepath.Dir(exe), "manifest.json")
	raw, err := os.ReadFile(at)
	if err != nil {
		return "not an installed Marco: these binaries were built in a tree, so there " +
			"is no manifest of what they are", nil
	}
	// AND A BYTE-ORDER MARK IS NOT A BROKEN MANIFEST.
	//
	// `json.Unmarshal` refuses one outright, and every Windows tool that writes text is
	// one keystroke away from adding it — `Set-Content -Encoding utf8` in Windows
	// PowerShell does, which is how the first packed bundle came to report "reinstall
	// Marco" about a manifest that was correct. The producer writes none now; this
	// survives one anyway, because a reader that fails on it tells somebody to reinstall
	// over an encoding.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var m bundleManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", []Problem{{
			Says: "this installation's manifest cannot be read, so nothing here can " +
				"say what it is made of: " + err.Error(),
			Do: "reinstall Marco",
		}}
	}
	var said []string
	var problems []Problem
	for _, f := range m.Files {
		where := filepath.Join(filepath.Dir(exe), filepath.FromSlash(f.File))
		// THE PROVIDER IS NOT WHERE THE MANIFEST NAMES IT. `uia.exe` is laid out
		// under `plugins\uia\` because that is where `marco` looks; the manifest
		// names the asset. Checked where it LANDS, through the one table that knows.
		if f.File == "uia.exe" {
			where = filepath.Join(filepath.Dir(exe), "plugins", "uia", "uia.exe")
		}
		id := f.Revision
		if id == "" {
			id = "no revision (not a Go binary)"
		}
		got, err := hashOf(where)
		switch {
		case err != nil:
			problems = append(problems, Problem{
				Says: fmt.Sprintf("%s is in this installation's manifest and is "+
					"not on disk: %s", f.File, where),
				Do: "reinstall Marco",
			})
			continue
		case got != f.SHA256:
			problems = append(problems, Problem{
				Says: fmt.Sprintf("%s is not the file this installation was "+
					"packaged with — its contents have changed since", f.File),
				Do: "reinstall Marco, or if you replaced it on purpose, know that " +
					"nothing here can say what it is",
			})
			continue
		}
		said = append(said, fmt.Sprintf("%s %s", f.File, id))
	}
	return fmt.Sprintf("Marco %s, packaged %s: %s", m.Version, m.Packed,
		strings.Join(said, " · ")), problems
}

// hashOf is the SHA-256 of a file, lowercase hex, as the manifest records it.
func hashOf(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
