//go:build windows

package main

// IS THERE A DESKTOP TO DO THIS ON, AND IS IT SAFE TO DO IT HERE.
//
// # Why every one of these is a refusal and not a failure
//
// This harness performs real input into a real window on a real screen — Marco's input, through
// Marco's own entrance, but real. The conditions it needs are not conditions a change to Marco
// can satisfy: a session with a visible desktop, nobody else's fixture in the way, and binaries
// that exist. A harness that reported "FAILED" when it could not find a screen would be blaming
// the code for the machine, and the first person to see that would reasonably conclude the change
// they just made had broken something.
//
// So an unmet condition is NOT RUN, with the reason and what would satisfy it, and NOT RUN is
// never green. The distinction has to survive into the exit code, because the thing that reads
// this most often is a script.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// notRun is a reason this harness cannot run here.
//
// Carried as an error so it travels back through the ordinary paths, and recognised by type at
// the top so nothing has to parse a sentence to tell "cannot" from "wrong".
type notRun struct{ why string }

func (n notRun) Error() string { return n.why }

func cannot(format string, args ...any) error {
	return notRun{why: fmt.Sprintf(format, args...)}
}

// machine is what this desktop is, as the harness found it.
//
// Reported whether or not anything fails, because half the questions a failure raises are about
// the machine: how big the screen was, what it was scaled to, whether somebody was using it.
type machine struct {
	ScreenW, ScreenH int
	DPI              int
	Remote           bool
	OtherStudio      string
	Root             string
	Head             string
	Dirty            bool
}

// checkDesktop establishes that there is somewhere to run, and says what it found.
func checkDesktop() (machine, error) {
	m := machine{
		ScreenW: systemMetric(smCXScreen),
		ScreenH: systemMetric(smCYScreen),
		DPI:     screenDPI(),
		Remote:  systemMetric(smRemoteSession) != 0,
	}
	if m.Remote {
		return m, cannot("this is a remote session, and injected input is refused on a " +
			"session that is not the one receiving input. Run it on the machine's own " +
			"desktop")
	}
	if m.ScreenW <= 0 || m.ScreenH <= 0 {
		return m, cannot("there is no screen on this session (%dx%d), so there is no "+
			"window for anything to be pressed in", m.ScreenW, m.ScreenH)
	}
	// NOBODY ELSE'S FIXTURE. Refused rather than killed: this harness does not know whose
	// it is. Two windows called "Marco Fixture" make the application name ambiguous — which
	// is a thing that has already cost a false failure once, in the acceptance suite.
	if found := strays(fixtureTitle); len(found) > 0 {
		return m, cannot("something is already running the fixture: %s. Close it — two "+
			"windows of that name make the application ambiguous, and the harness will "+
			"not end a process it did not start", strings.Join(found, ", "))
	}
	// ANOTHER STUDIO IS FINE, AND IS WRITTEN DOWN.
	//
	// Not a refusal: the owner has every right to have Studio open, and every window this
	// harness reads or presses is addressed by the process id of the child IT started, so a
	// second one cannot be mistaken for ours. It is recorded because it competes for one
	// thing that cannot be scoped — the foreground — and the Play scenarios need ours to
	// hold it.
	if found := strays(studioTitle); len(found) > 0 {
		m.OtherStudio = strings.Join(found, ", ")
	}
	root, err := repoRoot()
	if err != nil {
		return m, err
	}
	m.Root = root
	m.Head, m.Dirty = gitState(root)
	return m, nil
}

// repoRoot is the tree this harness builds Marco from.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", cannot("this harness cannot tell where it is: %v", err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", cannot("this has to run inside the Marco tree, and %q is not in one "+
		"(no CLAUDE.md above it)", dir)
}

// gitState is what the tree was when this ran, which is half of what a result means.
//
// A revision and whether anything was uncommitted. Not a failure when git says nothing — a tree
// exported without its history is still a tree somebody can test — but the report says so,
// because "it passed" against an unknown revision is not a thing anybody can act on later.
func gitState(root string) (string, bool) {
	head, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", false
	}
	status, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	return strings.TrimSpace(string(head)), strings.TrimSpace(string(status)) != ""
}
