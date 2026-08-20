package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// `director teach "open downloads"` — the human-facing way in.
//
// It starts a teaching session in the service and then follows it, printing each new thing Marco
// has to say. Following rather than driving: every decision is made in the service, and this
// process could be killed and restarted without changing what is being taught.
//
//	director teach "open downloads" --window-id window_37
//	director teach                                          # what is happening now
//	director teach --watch                                  # the evidence underneath
//	director teach --stop                                   # end it; nothing is kept
//
// The quoted phrase is what the USER wants the behaviour called. It is not a claim about what
// happened — Director still has to establish where you started, observe what you did, recognise
// where you ended, and compare examples before anything is written down.

// teachPollInterval is how often the follower asks what changed.
//
// Half a second. Fast enough that "go ahead" arrives while the user is still listening, slow
// enough that following a three-pass session costs nothing.
const teachPollInterval = 500 * time.Millisecond

func runTeach(args []string) int {
	// The name is the FIRST argument or there is none. Not "the first thing that is not a
	// flag": `--window-id window_37` would otherwise donate its value as the behaviour name,
	// and a teach session named after a window id is a confusing way to lose ten minutes.
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}

	fs := flag.NewFlagSet("teach", flag.ExitOnError)
	windowID := fs.String("window-id", "", "teach against the window with this ephemeral id")
	title := fs.String("window-title", "", "teach against the window whose title contains this")
	application := fs.String("application", "", "teach against this application's window")
	processID := fs.Int("process", 0, "teach against this process's window")
	actor := fs.String("actor", "", "the thing the play is about, e.g. Downloads")
	verb := fs.String("verb", "", "what it does, e.g. Open")
	dry := fs.Bool("dry", false, "rehearse against a recording host; nothing reaches the computer")
	watch := fs.Bool("watch", false, "show the evidence underneath, not only the plain reading")
	stop := fs.Bool("stop", false, "end the teaching session; nothing partial is kept")
	jsonOut := fs.Bool("json", false, "print as JSON")
	follow := fs.Bool("follow", true, "keep printing until the session ends")
	hold := fs.Duration("hold", 6*time.Second,
		"how long each START/DESTINATION highlight stays on screen")
	quiet := fs.Bool("no-highlight", false,
		"say what was established without pointing at it on screen")
	_ = fs.Parse(flagsFirst(rest))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	q := service.ObserveTeach{Watch: *watch, Cancel: *stop}
	if name != "" && !*stop {
		q.Name, q.Actor, q.Verb, q.Dry = name, *actor, *verb, *dry
		q.Target = windowref.Selector{
			EphemeralID: *windowID, Title: *title,
			Application: *application, ProcessID: uint32(*processID),
		}
		// A ZERO selector is allowed now and means "the one I'm working in". Only a
		// half-specified one is refused here — two selectors, or a flag with no value — which
		// is a mistake rather than an omission.
		if !q.Target.Zero() {
			if err := q.Target.Validate(); err != nil {
				fmt.Fprintf(os.Stderr,
					"director: %v\n\nName one window, or none at all to teach against "+
						"whatever you're working in:\n"+
						"  director teach %q\n"+
						"  director teach %q --window-id window_37   (director windows "+
						"lists them)\n", err, name, name)
				return 2
			}
		}
	}

	view, err := teachRequest(client, q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(view)
	}

	// What the play will be called, said BEFORE anybody demonstrates anything. The phrase
	// stays the outcome's name in the person's own words; this is the sentence a saved play
	// would become, derived from it — visible so it can be corrected rather than discovered
	// at the end.
	if name != "" && !*stop && view.WillBeCalled != "" {
		fmt.Printf("If this works out I'll write it down as `do %s`.\n"+
			"  (say it your way with --actor and --verb)\n\n", view.WillBeCalled)
	}
	printed := printTeach(view, *watch, "")
	shown := &groundingShown{hold: *hold, quiet: *quiet}
	defer shown.close()
	shown.print(view)
	asked := ""
	if !*follow || *stop || view.Settled {
		printTeachFooter(view, &asked)
		return 0
	}

	// Follow. A read-only poll: it starts nothing, samples nothing and answers nothing.
	//
	// It keeps following THROUGH a waiting phase rather than exiting at it. A question is not
	// the end of teaching — the user answers it in another terminal, the coordinator notices,
	// and the story continues in this one.
	for {
		time.Sleep(teachPollInterval)
		view, err = teachRequest(client, service.ObserveTeach{Watch: *watch})
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		printed = printTeach(view, *watch, printed)
		shown.print(view)
		printTeachFooter(view, &asked)
		if view.Settled {
			return 0
		}
	}
}

// teachRequest sends one teach request and decodes the view.
func teachRequest(client *service.Client, q service.ObserveTeach) (teachView, error) {
	raw, err := client.Observation(service.ObserveQuery{Teach: &q})
	if err != nil {
		return teachView{}, err
	}
	var v teachView
	if err := json.Unmarshal(raw, &v); err != nil {
		return teachView{}, err
	}
	return v, nil
}

// printTeach prints what changed since the last reading, and returns the new one.
//
// Only what CHANGED: a follower that reprinted the same line twice a second would bury the one
// moment the user is waiting for.
func printTeach(v teachView, watch bool, last string) string {
	if watch {
		for _, line := range v.Watching {
			fmt.Println(line)
		}
		fmt.Println()
		return v.Saying
	}
	if v.Saying == last {
		return last
	}
	fmt.Println(v.Saying)
	return v.Saying
}

// groundingShown prints each endpoint once, and points at it once.
//
// # Why once
//
// The follower polls twice a second. Printing "START — 8 controls I recognise this screen by" five
// hundred times, or relaunching the highlight surface five hundred times, would be worse than not
// showing it at all — and the second of those would put a new always-on-top window over the user's
// screen every half second while they were trying to demonstrate something.
//
// # Why it does not wait
//
// The surface exits on its own timer. Waiting for it would stop the follower reading what Marco
// says next, and the whole reason the start is shown at that moment is that Marco is about to say
// "go ahead" — a person staring at a highlight while the instruction sits unprinted has been given
// the picture and denied the sentence.
type groundingShown struct {
	hold  time.Duration
	quiet bool
	seen  map[string]bool
	// live holds the highlight surface each label currently owns, so the presentation can
	// be ENDED — not merely outlived — when its claim ends. The surface's own hold timer is
	// the backstop, never the lifecycle.
	live map[string]*exec.Cmd
	// launch is the test seam over "start a highlight surface"; nil means the real one.
	launch func(e groundedView, hold time.Duration) (*exec.Cmd, error)
}

// launchHighlight starts the real surface process.
func launchHighlight(e groundedView, hold time.Duration) (*exec.Cmd, error) {
	cmd, err := highlightCommand(boxesOfGrounded(e), e.Say, e.Role, hold)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Reaped in the background so a long teach session does not accumulate zombies.
	go func() { _ = cmd.Wait() }()
	return cmd, nil
}

func (g *groundingShown) print(v teachView) {
	if g.seen == nil {
		g.seen = map[string]bool{}
	}
	// Dismissals FIRST. A label whose claim is no longer current — the phase moved on, the
	// session settled, the learn failed — takes its highlight down before anything new goes
	// up. This is what "grounding is ephemeral presentation" means at the surface.
	current := map[string]bool{}
	for _, e := range v.Grounding {
		current[e.Label] = e.Current
	}
	for label, cmd := range g.live {
		if v.Settled || !current[label] {
			g.dismiss(label, cmd)
		}
	}
	if v.Settled {
		return
	}
	for _, e := range v.Grounding {
		if e.Say == "" || g.seen[e.Label] {
			continue
		}
		g.seen[e.Label] = true
		fmt.Println("\n  " + e.Say)
		// The sentence is always said; the PICTURE is drawn only while the claim it
		// presents is the session's current one. A status read arriving after the phase
		// moved on gets the words and no box.
		if g.quiet || !e.Current || !e.CanShow() {
			continue
		}
		launch := g.launch
		if launch == nil {
			launch = launchHighlight
		}
		cmd, err := launch(e, g.hold)
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			continue
		}
		if g.live == nil {
			g.live = map[string]*exec.Cmd{}
		}
		g.live[e.Label] = cmd
	}
}

// dismiss ends one owned highlight. Killing an already-exited surface is a no-op.
func (g *groundingShown) dismiss(label string, cmd *exec.Cmd) {
	delete(g.live, label)
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// close ends every highlight this follower still owns — called when the session settles and
// when the follower exits, so a refused or completed Learn leaves nothing on screen.
func (g *groundingShown) close() {
	for label, cmd := range g.live {
		g.dismiss(label, cmd)
	}
}

func boxesOfGrounded(v groundedView) []box {
	out := make([]box, 0, len(v.Boxes))
	for _, b := range v.Boxes {
		out = append(out, box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height})
	}
	return out
}

// printTeachFooter says what to do next, when there is something to do.
//
// Marco asks; the person answers through the commands that already accept answers. Teaching
// creates no shortcut around them, because two of the three questions below are about giving
// Marco permission or giving it a word, and neither is Teach's to supply.
// printTeachFooter says what to do next, once per question.
//
// `asked` remembers which prompt has already been printed, so following a session that waits four
// minutes for an answer does not print the same two lines five hundred times.
func printTeachFooter(v teachView, asked *string) {
	key := string(v.Phase) + "/" + string(v.QuestionID)
	if *asked == key {
		return
	}
	switch v.Phase {
	case teach.ReadyToRehearse:
		if v.QuestionID == "" {
			return
		}
		*asked = key
		fmt.Printf("\n  yes → director answer %s %s yes\n"+
			"  no  → director answer %s %s no\n"+
			"  (the question in full: director observation-session %s)\n\n",
			v.SessionID, v.QuestionID, v.SessionID, v.QuestionID, v.SessionID)
	case teach.Naming:
		if v.QuestionID == "" {
			return
		}
		*asked = key
		fmt.Printf("\n  director name-screen %s %s \"what you call it\"\n\n",
			v.SessionID, v.QuestionID)
	case teach.Refused:
		*asked = key
		// "NOTHING WAS SAVED" IS NOT ALWAYS TRUE. A teach that wrote the play and could not
		// register it refuses with `play_not_registered` over an artifact that is on disk;
		// telling that person to start again would throw their work away.
		if v.Play != "" {
			fmt.Printf("\nSaved as %s. It is a file you can read and edit.\n", v.Play)
			fmt.Println("Nothing can ask for it yet. `director teach --watch` says why; " +
				"once the way is clear:\n  director learned --register --name " + v.Play)
			return
		}
		// The retry does NOT suggest a window id. Naming one is a developer's override, and
		// offering it as the fix teaches people that the ordinary way is broken — when the
		// actual reason is almost always that Marco does not recognise the screen yet.
		fmt.Printf("\nNothing was saved. `director teach --watch` shows why, or try again:\n"+
			"  director teach %q\n", v.Name)
		if v.Refused == teach.DestinationNotRecognised {
			fmt.Println("\nMarco saw the change and can't yet recognise where it ended. " +
				"Letting it watch that screen for a little can help:\n" +
				"  director show-me --watch 30s --no-highlight")
		}
	case teach.Complete:
		*asked = key
		if v.Play != "" {
			fmt.Printf("\nSaved as %s. It is a file you can read and edit.\n", v.Play)
		}
		if !v.Registered {
			fmt.Println("Nothing can ask for it yet — register it when you want to be " +
				"able to:\n  director learned --register --name <name>")
		}
	}
}
