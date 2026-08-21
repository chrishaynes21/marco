// Package homelock says which live process owns one Marco home, and which may drive the desktop.
//
// # Two different ownerships, and collapsing them would be wrong
//
// `$MARCO_HOME` is not merely a directory. It names one semantic Marco world: one semantic
// memory, one command registry, one account of where the Audience is standing. That world should
// have at most one live Director owning it — otherwise two processes independently believe they
// are the Director for it, two observation loops persist conflicting evidence, and one can cancel
// while the other keeps acting.
//
// The physical desktop is a SECOND thing, and it is not scoped by home at all. Two homes may
// legitimately coexist — an acceptance sandbox beside the real store is exactly the arrangement
// this repository's harnesses use — and both of them look at, and could drive, the same screen.
// Home ownership cannot answer that question because it is deliberately per-home.
//
// So there are two claims here:
//
//	Home      "I am the Director for this Marco world."      one per canonical home
//	Desktop   "I am the only runtime driving this screen."   one per machine, per production
//
// Neither is authority. The Audience saying "do this" is a third thing, minted per invocation at
// the ordinary door, and a lease that granted it would be exactly the shortcut
// [[ADR-029-resolution-is-not-permission]] refuses.
//
// # Why a file is not the primitive
//
// Measured, live: three `director.exe` processes were running, two of them serving the same
// sandbox home, because nothing at startup asked whether the home already had an owner. The
// client had a file lock; the Director itself had none.
//
// A file cannot answer this. It survives the process that wrote it, so a crash leaves a claim
// nobody holds; a PID inside it can be reused by an unrelated process; and a staleness timeout
// is a guess about how long a start takes. What is needed is a claim whose lifetime is the
// PROCESS's, released by the operating system when the process ends however it ends.
//
// On Windows that is a named kernel object. Elsewhere there is a stub, and it says plainly that
// it enforces nothing — see the platform files.
package homelock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Claim is one held ownership. Releasing it is idempotent, and the operating system releases it
// anyway when the process ends.
type Claim interface {
	// Release gives the claim up. Safe to call more than once, and safe on a nil-ish claim,
	// so a caller can `defer` it beside the error check rather than around it.
	Release()
	// Name is the kernel object this claim is held on, for diagnostics. Never a token, never
	// a path: see Identity.
	Name() string
}

// Identity is the canonical, comparable name of one Marco home.
//
// # Why a hash and not the path
//
// Two reasons, and the second is the one that matters. A kernel object name has a length limit
// and may not contain a separator, so a path cannot be one; and this string appears in
// diagnostics, where a filesystem path is more than a reader needs and more than Marco should
// volunteer about somebody's machine.
//
// # What canonical means here
//
// The same home spelled differently must be the same home, or two Directors both start and both
// believe they own it. Handled: relative against absolute, forward against back slashes, trailing
// separators, `.` and `..` components, and — on Windows only — letter case. Symbolic links are
// resolved when the path exists, because a junction to a home is that home.
//
// Case is folded ONLY where the platform's own filesystem is case-insensitive. Folding everywhere
// would merge two genuinely different homes on a case-sensitive filesystem, which is the opposite
// mistake and a worse one.
//
// Deleting any normalisation must fail a case of TestOneHomeSpelledSeveralWaysIsOneHome.
func Identity(home string) string {
	clean := strings.TrimSpace(home)
	if clean == "" {
		clean = "."
	}
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	// A junction or symbolic link to a home IS that home. Only meaningful when the path
	// exists; a home that has not been created yet normalises on its spelling alone, which
	// is correct — there is nothing to resolve.
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	// There is no `filepath.Clean` here and there was one. `Abs` cleans on the way out and
	// `EvalSymlinks` returns a clean path, so it changed nothing measurably — deleting it left
	// every case green, which makes it a claim nothing can test rather than a safeguard. Dot
	// components, doubled separators and trailing separators are handled above; the cases hold
	// them against a home that does NOT exist, which is the only shape where EvalSymlinks
	// cannot quietly do the work for us.
	if caseInsensitiveFilesystem {
		clean = strings.ToLower(clean)
	}
	// Separator direction last, so `Clean` has already done its work in the platform's own
	// terms. The hash must not depend on which slash somebody typed.
	clean = filepath.ToSlash(clean)
	sum := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(sum[:8])
}

// ClaimHome takes ownership of one Marco home for as long as this process lives.
//
// Returns `ErrHeld` when another live process already owns it. That is not a failure of this
// function: it is the answer, and the caller's job is to say so honestly rather than to start
// anyway or to take the claim away from whoever has it.
//
// # It must be called BEFORE anything else
//
// A process that has not claimed the home must not open the semantic store, start perception,
// publish an endpoint, register a command or emit input. Every one of those is an act of the
// runtime owner, and doing them first is how two Directors end up half-owning one world before
// either of them notices. See the ordering in `cmd/director`.
func ClaimHome(home string) (Claim, error) {
	return claim(homeObject(Identity(home)))
}

// ClaimDesktop takes the right to drive the physical desktop.
//
// # Machine-wide on purpose, and narrower than the Director
//
// The desktop is one screen, one keyboard, one pointer, and it does not know about homes. A
// sandbox Director and the real one are different worlds that share it, so a claim scoped by
// home would let them interleave real input — which is the failure this exists to prevent.
//
// It is held around a PRODUCTION rather than for the life of the Director, deliberately. A
// Director that held it from startup would stop every other Director on the machine from ever
// acting, including the sandbox ones this repository's own acceptance harnesses run; held around
// the act of emitting, it stops only what actually conflicts. Observation needs no claim at all,
// so two homes may watch the same screen at once — which is what makes a debug Director useful
// while the real one works.
func ClaimDesktop() (Claim, error) {
	return claim(desktopObject())
}

// ErrHeld is another live process holding the claim.
var ErrHeld = fmt.Errorf("homelock: already held by another process")

// Held reports whether an error is the ordinary "somebody else has it" answer.
func Held(err error) bool { return err == ErrHeld }

// homeObject and desktopObject are the kernel object names.
//
// `Local\` scopes them to the login session rather than the machine, which is the right boundary:
// two people signed in at once have two desktops and two sets of processes, and one must not
// refuse the other. The prefix is inert on platforms without the concept.
func homeObject(identity string) string { return `Local\marco-home-` + identity }

// desktopObject TAKES NO HOME, and that is the whole design rather than an omission.
//
// There is one screen. A name that carried a home identity would give every home its own
// desktop, which is precisely the failure this lease exists to prevent: a sandbox Director and
// the real one typing at the same time, each holding a lease it genuinely owned.
//
// The signature is the guarantee — this function has nothing to scope itself by — and no test
// asserts it, because there is no way to write one from inside a single process: a mutation that
// appended some identity would append the SAME identity in every caller here. It is stated
// instead, and the property it protects is held one layer up by
// TestTwoRuntimesDoNotInterleaveRealInput.
func desktopObject() string { return `Local\marco-desktop` }

// Owner is what a diagnostic surface may say about a claim it could not take.
//
// Everything here is metadata. None of it proves anything and none of it may be used to decide
// ownership: `PID` in particular is reported because a person reading `director status` wants it,
// and is never trusted, because process ids are reused.
type Owner struct {
	// Home is the canonical identity, not the path.
	Home string
	// PID and StartedAt come from discovery metadata written by the owner, when there is
	// any. Empty is ordinary: a claim can be held by a process that has not published
	// anything yet.
	PID       int
	Address   string
	StartedAt string
}

// EnvHome is the environment variable that names a home, so callers agree on the spelling.
const EnvHome = "MARCO_HOME"

// FromEnv is the home this process was pointed at, or the given default.
func FromEnv(fallback string) string {
	if dir := strings.TrimSpace(os.Getenv(EnvHome)); dir != "" {
		return dir
	}
	return fallback
}
