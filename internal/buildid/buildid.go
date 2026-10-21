// Package buildid says which build of Marco is running, from the stamp the toolchain leaves.
//
// # Why this exists
//
// Because "which binary is this?" was answered by comparing file timestamps, and that is not an
// answer anybody should have to work out. Studio spawns the `director.exe` beside its own
// executable; with two copies of Studio in a tree there are two directors, and a stale one is
// invisible — every file looks freshly built, the window is current, and the recordings come
// from whenever that director was last made. It cost the owner a day.
//
// Git identifies the SOURCE. It cannot say what is running. So each binary carries its own
// revision and says it: in the recorder's stream, in a run's record, and in Studio's technical
// details. Two builds that disagree are then a thing somebody can see rather than infer.
//
// # Where the stamp comes from
//
// `go build` records the VCS revision in the binary by default, and `debug.ReadBuildInfo` reads
// it back. Nothing has to be passed on the command line and nothing can be forgotten — except
// in one case, which is the case that bit: a package built from OUTSIDE its own module is not
// stamped at all, which is what `build.cmd` was doing to the director. `Revision` answers
// "unknown" there rather than inventing something, and the build script no longer does it.
package buildid

import (
	"runtime/debug"
	"strings"
)

// Revision is the short commit this binary was built from, with "+dirty" when the tree had
// uncommitted changes, or "unknown" when the toolchain left no stamp.
//
// Short, because this is read by a person comparing two of them in a window, and "+dirty" kept
// because a development build of one side and a committed build of the other is exactly the
// pair that goes wrong.
func Revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "unknown"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		return rev + "+dirty"
	}
	return rev
}

// Same reports whether two revisions describe the same build.
//
// An unknown on either side is NOT a match: a binary that cannot say what it is cannot be
// vouched for, and treating silence as agreement is how the thing this package exists for
// happened in the first place.
func Same(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" || a == "unknown" || b == "unknown" {
		return false
	}
	return a == b
}

// Differ reports whether two revisions are both known and not the same.
//
// NOT the negation of Same, and the difference is the whole point. `Same` answers "can I vouch
// that these match", which an unknown cannot. This answers "can I see that they do not", which
// an unknown also cannot — and conflating the two turns every pair of unstamped binaries into
// an alarm about a disagreement nobody has observed. A test binary is unstamped, which is how
// this was found.
//
// So: Same is for trusting, Differ is for warning, and between them sits "cannot tell", which
// is neither.
func Differ(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" || a == "unknown" || b == "unknown" {
		return false
	}
	return a != b
}
