package homelock_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/platform/homelock"
)

// One Marco home, however it is spelled.
//
// # Why this is the first thing
//
// Ownership is keyed on the home's identity. If two spellings of one directory produce two
// identities, two Directors both claim, both succeed, and every guarantee above this line is
// gone — with no error anywhere, because each of them genuinely did acquire the thing it asked
// for.
//
// The spellings below are not hypothetical. `configDir()` builds a path from an environment
// variable and `filepath.Join`; a harness writes one with forward slashes; a person types one
// with a trailing separator; PowerShell hands over a different case on Windows. All of them are
// the same directory and must be the same home.
// # Why the home does not exist yet
//
// `filepath.EvalSymlinks` resolves a path that IS there, and on Windows it hands back the real
// spelling — right case, right separators, no dot components. So a test against an existing
// directory is testing EvalSymlinks, and every normalisation below it can be deleted without the
// test noticing. Measured: three of them survived exactly that way.
//
// A home that has not been created is the ordinary case anyway. The first `director serve` for a
// sandbox claims it before anything makes the directory, and that is precisely when two spellings
// must still be one home.
func TestOneHomeSpelledSeveralWaysIsOneHome(t *testing.T) {
	base := filepath.Join(t.TempDir(), "NotCreatedYet")
	want := homelock.Identity(base)

	for _, spelling := range []struct{ name, path string }{
		{"a trailing separator", base + string(filepath.Separator)},
		{"forward slashes", filepath.ToSlash(base)},
		{"a dot component", filepath.Join(base, ".")},
		{"up and back down", filepath.Join(base, "sub", "..")},
		{"doubled separators", base + string(filepath.Separator) + string(filepath.Separator)},
	} {
		t.Run(spelling.name, func(t *testing.T) {
			if got := homelock.Identity(spelling.path); got != want {
				t.Fatalf("%q is a different home from %q.\nTwo spellings of one "+
					"directory means two ownership claims, both granted, and "+
					"two Directors that each believe they own the world.",
					spelling.path, base)
			}
		})
	}

	// CASE, and only where the platform's own filesystem folds it. Folding everywhere would
	// merge two genuinely different homes on a case-sensitive filesystem, which is the
	// opposite mistake and the worse one.
	t.Run("case, on a filesystem that ignores it", func(t *testing.T) {
		upper := strings.ToUpper(base)
		if upper == base {
			t.Skip("this path has no case to change")
		}
		got := homelock.Identity(upper)
		if runtime.GOOS == "windows" && got != want {
			t.Errorf("%q is a different home from %q on Windows, where they are the "+
				"same directory", upper, base)
		}
		if runtime.GOOS != "windows" && got == want {
			t.Errorf("%q was folded into %q on a case-sensitive filesystem, where "+
				"they are two different directories", upper, base)
		}
	})
}

// AND TWO DIFFERENT HOMES ARE TWO DIFFERENT HOMES.
//
// The negative control, and it is not a formality: an identity function that returned a constant
// would pass every case above. It would also let one Director anywhere on the machine stop every
// other, which breaks the acceptance sandboxes this repository runs beside a real store.
func TestTwoHomesAreNotOneHome(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if homelock.Identity(a) == homelock.Identity(b) {
		t.Fatal("two different directories share one ownership identity. A sandbox " +
			"Director would then be refused because the real one is running, and one " +
			"Marco per machine is not the rule — one per home is.")
	}
}

// A HOME HAS ONE OWNER, AND THE SECOND CLAIM IS REFUSED RATHER THAN GRANTED.
//
// Skipped where there is no backend. The stub enforces nothing and says so; asserting here would
// be asserting against a file that documents its own gap.
func TestAHomeHasOneOwner(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("no ownership backend on this platform; see homelock_stub.go")
	}
	home := t.TempDir()

	first, err := homelock.ClaimHome(home)
	if err != nil {
		t.Fatalf("the first claim on an unowned home failed: %v", err)
	}

	if _, err := homelock.ClaimHome(home); !homelock.Held(err) {
		first.Release()
		t.Fatalf("a second claim on an owned home returned %v, want the held answer.\n"+
			"Two Directors would both start and both believe they own this world.", err)
	}

	// AND RELEASING GIVES IT BACK. Without this the assertion above passes for a claim that
	// can never be taken at all, and the first Director to run would own the home forever.
	first.Release()
	second, err := homelock.ClaimHome(home)
	if err != nil {
		t.Fatalf("the home stayed owned after its owner released it: %v.\nA Director that "+
			"exits must give the home back, or a restart is impossible.", err)
	}
	second.Release()
}

// AND A DIFFERENT HOME IS NOT AFFECTED BY AN OWNED ONE.
//
// The property that keeps the acceptance sandboxes working: a real Director running must not stop
// a sandbox one from starting. One Director per home, never one per machine.
func TestOwningOneHomeLeavesAnotherFree(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("no ownership backend on this platform; see homelock_stub.go")
	}
	real, sandbox := t.TempDir(), t.TempDir()

	held, err := homelock.ClaimHome(real)
	if err != nil {
		t.Fatalf("claiming the first home: %v", err)
	}
	defer held.Release()

	other, err := homelock.ClaimHome(sandbox)
	if err != nil {
		t.Fatalf("a sandbox home could not be claimed while another home was owned: %v.\n"+
			"This is the arrangement every acceptance harness in this repository "+
			"uses, and it must keep working.", err)
	}
	other.Release()
}

// THE DESKTOP IS ONE, WHATEVER THE HOME.
//
// The second lease, and the reason it exists: two Directors serving two different homes are two
// separate worlds sharing one keyboard. Home ownership cannot arbitrate that — it is per-home by
// design — so a claim scoped by home would let them interleave real input.
func TestTheDesktopIsOneWhateverTheHome(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("no ownership backend on this platform; see homelock_stub.go")
	}
	first, err := homelock.ClaimDesktop()
	if err != nil {
		t.Fatalf("claiming an unheld desktop: %v", err)
	}

	if _, err := homelock.ClaimDesktop(); !homelock.Held(err) {
		first.Release()
		t.Fatalf("a second desktop claim returned %v, want the held answer.\nTwo runtimes "+
			"would then type on one keyboard at once, and neither would know.", err)
	}

	first.Release()
	again, err := homelock.ClaimDesktop()
	if err != nil {
		t.Fatalf("the desktop stayed held after release: %v. A finished production must "+
			"give the keyboard back.", err)
	}
	again.Release()
}

// RELEASING TWICE IS NOT A CRASH.
//
// A claim is released by a `defer` on a clean exit and by the operating system otherwise, and
// the two can both happen. A caller must be able to release without checking first.
func TestReleasingAClaimTwiceIsHarmless(t *testing.T) {
	held, err := homelock.ClaimHome(t.TempDir())
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	held.Release()
	held.Release()
}
