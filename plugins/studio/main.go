// Command studio is a bounded feasibility proof: can a Wails window host Marco's control surface
// and drive the PRODUCTION backend, with zero automated input?
//
// It is deliberately two programs in one binary, because the two gates it answers need different
// things:
//
//	studio -proof            headless. Starts a DRY Director of its own, issues one command
//	                         through the service client, and prints the four facts the gate asks
//	                         for: Start acknowledged, the canonical run reference, the
//	                         backend-derived phase, and an acknowledged Cancel. No window, so it
//	                         can run in a suite and cannot touch anybody's focus.
//	studio -seconds N        opens the window, renders the page, and quits by itself. The window
//	                         is what "page hosting" means and nothing else here needs one.
//
// Build:
//
//	go -C plugins/studio build -tags "desktop,production" -ldflags "-H windowsgui" -o studio.exe .
//
// Neither half of that is cosmetic.
//
//   - `-H windowsgui` sets the PE subsystem. Measured: 3 without it, 2 with it — and without it
//     a console window appears beside the WebView.
//   - `-tags "desktop,production"` is what compiles in the asset server that serves the embedded
//     page. WITHOUT THEM THE BUILD SUCCEEDS and the window opens onto nothing: the page is never
//     served, so it never runs, so it never calls back — which from outside is indistinguishable
//     from "WebView2 did not come up". Three attempts at gate 1 were spent on that.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	acts := flag.String("acts", "",
		"print what the Act surface would show for this store and exit; opens no "+
			"window, runs nothing, presses nothing")
	proof := flag.Bool("proof", false,
		"run the headless backend proof and print its evidence; opens no window")
	seconds := flag.Int("seconds", 0,
		"close the window by itself after this many seconds (0 = stay open)")
	phrase := flag.String("phrase", "click save", "the phrase the proof asks for")
	evidence := flag.String("evidence", "",
		"a file the PAGE appends to once it has rendered; the only way a windowsgui "+
			"binary can report that its webview came up")
	autostart := flag.Bool("autostart", false,
		"the PAGE asks for a dry run as soon as it has rendered, so one artifact carries "+
			"both the hosting gate and the backend gate")
	autocancel := flag.Bool("autocancel", false,
		"the PAGE cancels its own run at the first phase the backend reports. OFF by "+
			"default: a page that cancels by itself produces cancellation evidence no "+
			"person took part in, so a human trial leaves this off and presses the button")
	home := flag.String("home", "",
		"the Marco store to use. Empty means the one Marco keeps everything in, which is "+
			"what a person wants; pass a path to aim Studio at another")
	isolate := flag.Bool("isolate", false,
		"use a temporary store made fresh for this launch and removed when it closes. "+
			"For a proof, never for use: an isolated store has none of your "+
			"demonstrations in it")
	flag.Parse()

	if *acts != "" {
		os.Exit(runActsCheck(*acts))
	}
	if *proof {
		// PROOF MODE IS ALWAYS ISOLATED, whatever the flags say. It starts a Director
		// of its own and drives it, which is not a thing to do to somebody's store.
		os.Exit(runProof(*phrase, *home))
	}

	// ── THE STORE A PERSON'S DEMONSTRATIONS ARE ACTUALLY IN ──
	//
	// # The defect this closes
	//
	// Normal launch used `proofStore`, which with no `-home` makes a TEMPORARY directory
	// and points this process at it. So `studio.exe`, run the way it was documented, opened
	// a window onto an empty store: none of the owner's demonstrations, nothing to compose
	// from, nothing to select. Worse, a composed Act would have been written into the
	// PERSISTENT routes tree while naming a demonstration store that vanished on exit — a
	// play permanently pinned to a revision of something that no longer existed anywhere.
	//
	// Isolation was right for the feasibility proof and wrong for the product, and the two
	// now say so separately. `-isolate` is the proof's, and it says in its own help that an
	// isolated store has none of your demonstrations in it.
	//
	// Containment does not rest on this any more, which is why the product may use the real
	// store safely: `Backend.reach` REFUSES to attach to a Director it did not start, so a
	// window opened on the owner's home cannot cancel or shut down the service they are
	// running there. That guarantee is structural and independent of which store this is.
	store, release, err := chooseStore(*home, *isolate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "studio: %v\n", err)
		os.Exit(1)
	}
	defer release()

	b := &Backend{dry: true, home: store, evidence: *evidence,
		phrase: *phrase, autoStart: *autostart, autoCancel: *autocancel}
	// THE ACT SURFACE, which is what Studio is for. See acts.go.
	surface := newActs(store)
	defer b.Close()
	// AND THE ACT RUN, which outlived the window before this. See Acts.Close.
	defer surface.Close()
	if *seconds > 0 {
		// BEFORE the window, so it cannot depend on the window — and it takes the
		// backend with it. `os.Exit` runs no deferred function, so the first version
		// of this left a Director running on a temporary store after every windowed
		// proof. A shell that leaves a service behind when it closes is a shell that
		// leaks one per launch.
		go leaveBy(b, surface, release, *seconds)
	}
	err = wails.Run(&options.App{
		Title:  "Marco Studio",
		Width:  1100,
		Height: 760,
		// THE PAGE, SERVED FROM THE BINARY. No localhost, no browser, no npm build:
		// the asset server reads the embedded directory directly.
		AssetServer: &assetserver.Options{Assets: pageFS()},
		OnStartup: func(ctx context.Context) {
			b.ctx = ctx
			if *seconds > 0 {
				go quitAfter(ctx, *seconds)
			}
		},
		// AND THE WINDOW CLOSING ENDS THE RUN IT STARTED.
		//
		// Wired here as well as deferred above, because the two fire on different
		// ways out: a person closing the window comes through Wails, and whichever
		// happens first does the work. `Acts.Close` is safe to call twice — it
		// returns at once when there is no run.
		OnShutdown: func(context.Context) {
			surface.Close()
			b.Close()
		},
		// THE BOUND BACKEND. `window.go.main.Backend.Start(...)` in the page reaches
		// the method above; nothing in the page knows how a Director is spoken to.
		Bind: []any{b, surface},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "studio: %v\n", err)
		os.Exit(1)
	}
}

// proofStore is the store this shell's Director uses, and by default it is NOBODY'S.
//
// # The defect this closes
//
// The first version of this file read $MARCO_HOME and fell back to %AppData%\marco — the
// owner's real store. So the windowed shell, launched from any shell that exports MARCO_HOME or
// from none at all, aimed itself at the store the owner's own Director was already serving,
// connected to THAT Director, and then offered a Cancel button and a shutdown-on-close. The
// headless proof had a temporary store of its own and the window did not, which is the worst
// arrangement of the two: the evidence was produced in isolation and the thing a person
// actually launches was not.
//
// A feasibility shell must not be ABLE to cancel somebody's work. So the default is an isolated
// directory per launch, removed when the shell exits, and `-home` is the one way to aim it at a
// real store — named on the command line, never inherited.
//
// The environment is set as well as the field, because the two say different things: a spawned
// Director reads $MARCO_HOME to decide which store it serves, while `ConnectOptions.Dir` only
// says where the endpoint file is. Setting one and not the other is how the first proof started
// a child that went looking at the owner's store.
func proofStore(explicit string) (string, func(), error) {
	if explicit != "" {
		if err := os.Setenv("MARCO_HOME", explicit); err != nil {
			return "", nil, fmt.Errorf("the store named by -home could not be made this "+
				"process's home: %w", err)
		}
		return explicit, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "studio-shell-")
	if err != nil {
		return "", nil, fmt.Errorf("an isolated store could not be made: %w", err)
	}
	if err := os.Setenv("MARCO_HOME", dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("the isolated store could not be made this process's "+
			"home: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// pageFS is the embedded directory with index.html AT ITS ROOT.
//
// The asset server serves the FS root, and an `//go:embed all:frontend` FS has `frontend/` at
// the root rather than the page. The first attempt at this proof passed the FS whole: the window
// opened, nothing could be served, and the page's own callback never ran — which read, from
// outside, exactly like "the webview did not come up".
func pageFS() fs.FS {
	sub, err := fs.Sub(assets, "frontend")
	if err != nil {
		panic(err)
	}
	return sub
}

// leaveBy is the self-close a feasibility window must have, and it does NOT go through Wails.
//
// `quitAfter` asks the window to quit, which needs the app to have started — so if the window
// never initialises, the thing that cannot close itself is the thing most likely to be left on
// somebody's desktop. That happened. This exits the process regardless.
func leaveBy(b *Backend, surface *Acts, release func(), seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
	// THE RUN FIRST, AND THE STORE LAST. A temporary store removed before the run had
	// finished would take the run's own record with it.
	surface.Close()
	b.Close()
	release()
	os.Exit(0)
}

// chooseStore is the store Studio reads and writes, and which KIND of store it is.
//
// Three answers, and only one of them is a product launch:
//
//	-home <dir>   that one, named deliberately
//	-isolate      a temporary one, removed on exit — for a proof, never for use
//	neither       the store Marco keeps everything in, which is what a person wants
//
// The last is resolved exactly as `director` and `marco` resolve it: `$MARCO_HOME`, else the
// user's config directory. Resolved the same way ON PURPOSE — a second answer to "where is the
// store" is a window that shows a person demonstrations they do not have, or writes an Act
// somewhere they will never find it.
//
// It also sets `$MARCO_HOME` for this process, because the supervised runner Studio spawns reads
// that for itself: a child with a different answer would ground against one store while the
// window listed another.
func chooseStore(explicit string, isolate bool) (string, func(), error) {
	switch {
	case explicit != "" && isolate:
		return "", nil, fmt.Errorf("-home names a store and -isolate asks for a " +
			"temporary one; which was meant is not a guess this makes")
	case isolate:
		dir, err := os.MkdirTemp("", "studio-isolated-")
		if err != nil {
			return "", nil, fmt.Errorf("an isolated store could not be made: %w", err)
		}
		if err := os.Setenv("MARCO_HOME", dir); err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, err
		}
		return dir, func() { _ = os.RemoveAll(dir) }, nil
	case explicit != "":
		if err := os.Setenv("MARCO_HOME", explicit); err != nil {
			return "", nil, err
		}
		return explicit, func() {}, nil
	}
	dir := marcoHome()
	if err := os.Setenv("MARCO_HOME", dir); err != nil {
		return "", nil, err
	}
	return dir, func() {}, nil
}

// marcoHome is where Marco keeps everything, resolved as the other binaries resolve it.
func marcoHome() string {
	if d := os.Getenv("MARCO_HOME"); d != "" {
		return d
	}
	if c, err := os.UserConfigDir(); err == nil {
		return filepath.Join(c, "marco")
	}
	return "."
}
