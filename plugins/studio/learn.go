package main

// LEARNING A SCENE: the recorder Marco already had, reachable from the window.
//
// # The gap this closes
//
// Studio could list what had been demonstrated and could not record one. So the one thing Marco
// is for — you show it something once — was the one thing the window could not do, and the
// owner's way in was a terminal command with six flags whose session ends when somebody presses
// ENTER at a prompt they cannot see. Their question, which is the whole of this file's reason:
// "how do i do a demonstration? And if not, isn't that something marco can do the UI can't?"
//
// # What this is NOT
//
// It is not a second recorder. `director demonstrate-visual` is the recorder, with its own
// settle rule, its own correlation of presses to controls, its own refusal to write a coordinate
// as a control's identity, and its own evidence file beside the artifact. This starts it, says
// what it is doing while it runs, ends it when the person says they are done, and then reads the
// store to say what came of it. Four things, none of which is a judgement about pixels.
//
// # The lifecycle, and whose claim each part of it is
//
//	prepare    Studio brings the application forward     ours, and refused here if it cannot
//	wait       the recorder's own --delay                ours: we chose it, so we may count it
//	record     the recorder watches                      the recorder's, until somebody is done
//	end        a newline on its stdin                    the recorder's own "press ENTER"
//	review     what the STORE has afterwards             the artifact's, never a parsed sentence
//
// The countdown is deliberately Studio's own arithmetic over a number Studio passed, and not a
// claim about the recorder's state. The review is deliberately the saved artifact and not the
// child's output: a surface that read its verdict out of printed sentences would change what it
// told somebody the moment somebody improved a sentence.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// learnPrepare is how long somebody gets to switch to the game before recording starts.
//
// The recorder's own flag, passed explicitly rather than left to its default, because this
// window shows a countdown to it and a countdown to a number nobody passed is a window counting
// down to a guess.
const learnPrepare = 5 * time.Second

// learnLongest is the recorder's own ceiling, so a forgotten window cannot ask for more.
const learnLongest = 300

// Learning is what the page shows while a demonstration is being recorded, and after it.
type Learning struct {
	Name        string `json:"name,omitempty"`
	Application string `json:"application,omitempty"`
	Active      bool   `json:"active"`
	// Phase is "preparing" or "recording" while this is in flight, and "" otherwise.
	Phase string `json:"phase,omitempty"`
	// Doing is the sentence at the top, and Left the seconds remaining in this phase.
	Doing string `json:"doing,omitempty"`
	Left  int    `json:"left"`
	// Over says a recording has finished and this is what came of it.
	Over       bool     `json:"over,omitempty"`
	Said       string   `json:"said,omitempty"`
	Next       string   `json:"next,omitempty"`
	Failed     bool     `json:"failed,omitempty"`
	Transcript []string `json:"transcript,omitempty"`
	// Learned is the demonstration as the store has it, once there is one. The review
	// reads the artifact rather than the recorder's words: it is the thing that will be
	// performed, and it is the only account that cannot drift from one.
	Learned *Demonstration `json:"learned,omitempty"`
}

// learning is one recording in flight, or the last one.
type learning struct {
	name, app string
	seconds   int
	started   time.Time
	cmd       *exec.Cmd
	in        *os.File
	stop      func()
	done      chan struct{}
	lines     []string
	over      bool
	said      string
	next      string
	failed    bool
}

// Applications is the running applications with a window, by the name they can be asked for.
//
// For choosing the game by something recognisable instead of typing `arkascended`: the
// executable name is what every selector in Marco matches on, so it is the name a person has to
// end up with, and a list they can point at is the difference between that and a guess.
func (a *Acts) Applications() []string {
	seen := map[string]bool{}
	for _, c := range a.windowsNow() {
		if c.Application == "" || c.Minimized || !c.OnScreen {
			continue
		}
		// A WINDOW WITH NO SIZE IS NOT SOMETHING SOMEBODY IS LOOKING AT. Every
		// process on a desktop owns message-only and zero-area windows, and a list
		// of forty of those is a list nobody can choose from.
		if onScreenArea(c) < 10000 {
			continue
		}
		seen[c.Application] = true
	}
	out := make([]string, 0, len(seen))
	for app := range seen {
		out = append(out, app)
	}
	sort.Strings(out)
	return out
}

// Learn records a demonstration, through `director demonstrate-visual`.
//
// Returns "" when the recorder has been started, or the sentence to show the person. Everything
// refusable is refused before the application is brought forward.
func (a *Acts) Learn(name, application string, seconds int) string {
	name = strings.TrimSpace(name)
	application = strings.TrimSpace(application)
	switch {
	case name == "":
		return "say what to call it first — the name is what you will look for later"
	case application == "":
		return "choose the application to watch"
	case seconds < 5 || seconds > learnLongest:
		return fmt.Sprintf("a recording lasts between 5 and %d seconds",
			learnLongest)
	}
	// AN ACT MUST NOT BE NAMED AFTER ONE OF ITS OWN INTERACTIONS, and a demonstration
	// must not be named after an Act for the same reason: both become declarations in
	// the play that composes them. Said here rather than discovered from a compile error
	// two steps later.
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, name) {
			return fmt.Sprintf("there is already an Act called %q, and a "+
				"demonstration cannot share its name — call this "+
				"something else", c.Name)
		}
	}

	a.mu.Lock()
	switch {
	case a.closing:
		a.mu.Unlock()
		return "Studio is closing, so nothing new is started"
	case a.busy:
		a.mu.Unlock()
		return "a run is in flight; one thing at a time on one screen"
	case a.learn != nil && !a.learn.over:
		a.mu.Unlock()
		return "a recording is already in flight"
	}
	l := &learning{name: name, app: application, seconds: seconds}
	a.learn = l
	a.mu.Unlock()
	give := func(why string) string {
		a.mu.Lock()
		a.learn = nil
		a.mu.Unlock()
		return why
	}

	// AND THE APPLICATION IN FRONT, because a recording of the wrong window is a
	// demonstration of nothing and takes a person's whole session to find out.
	if why := a.raise(application); why != "" {
		return give(why)
	}

	args := []string{"demonstrate-visual", name,
		"--application", application,
		"--seconds", fmt.Sprint(seconds),
		"--delay", learnPrepare.String()}
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, directorBin(), args...)
	cmd.Env = append(os.Environ(), "MARCO_HOME="+a.home)
	// ITS STDIN IS HOW A PERSON SAYS THEY ARE DONE. The recorder ends on a newline —
	// "Press ENTER when you are done" — and a child with no terminal has nobody to press
	// it, so the window's own button writes it. See Done.
	inR, inW, perr := os.Pipe()
	if perr != nil {
		stop()
		return give("the recorder could not be started: " + perr.Error())
	}
	cmd.Stdin = inR
	pr, pw, oerr := os.Pipe()
	if oerr != nil {
		stop()
		inR.Close()
		inW.Close()
		return give("the recorder could not be started: " + oerr.Error())
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		stop()
		inR.Close()
		inW.Close()
		pr.Close()
		pw.Close()
		return give("the recorder could not be started: " + err.Error())
	}
	pw.Close()
	inR.Close()

	a.mu.Lock()
	l.cmd, l.in, l.stop, l.started = cmd, inW, stop, time.Now()
	l.done = make(chan struct{})
	a.mu.Unlock()

	go a.readLearning(pr, l)
	go func() {
		err := cmd.Wait()
		stop()
		// AND WHAT CAME OF IT, FROM THE STORE. Read after the child has exited, so
		// what is reported is what was actually saved rather than what was said.
		said, next, failed, _ := a.reviewed(l.name, err)
		a.mu.Lock()
		l.over, l.said, l.next, l.failed = true, said, next, failed
		if l.in != nil {
			l.in.Close()
			l.in = nil
		}
		close(l.done)
		a.mu.Unlock()
	}()
	return ""
}

// Done tells the recorder the person has finished showing it.
func (a *Acts) Done() string {
	a.mu.Lock()
	l := a.learn
	var in *os.File
	if l != nil && !l.over {
		in = l.in
	}
	a.mu.Unlock()
	if in == nil {
		return "nothing is being recorded"
	}
	// THE RECORDER'S OWN ENDING, not a kill: it then reports, learns a draft from the
	// walk, saves the artifact and writes the evidence beside it. A window that killed
	// the process here would throw all of that away and call it finishing.
	if _, err := in.Write([]byte("\n")); err != nil {
		return "the recorder could not be told to finish: " + err.Error()
	}
	return ""
}

// StopLearning abandons a recording.
//
// Deliberately different from Done: nothing is saved. A person who pressed this did not mean
// "keep what you have", and a half-recording saved under the name they chose is worse than none.
func (a *Acts) StopLearning() string {
	a.mu.Lock()
	l := a.learn
	a.mu.Unlock()
	if l == nil || l.over {
		return "nothing is being recorded"
	}
	if l.stop != nil {
		l.stop()
	}
	return ""
}

// Learning is the recording in flight, or the last one and what came of it.
func (a *Acts) Learning() Learning {
	a.mu.Lock()
	l := a.learn
	if l == nil {
		a.mu.Unlock()
		return Learning{}
	}
	out := Learning{Name: l.name, Application: l.app, Active: !l.over,
		Over: l.over, Said: l.said, Next: l.next, Failed: l.failed,
		Transcript: append([]string(nil), l.lines...)}
	started, seconds := l.started, l.seconds
	a.mu.Unlock()

	if out.Over {
		// THE DEMONSTRATION ITSELF, as the surface lists every other one: the same
		// `Demonstrations` a person chooses from, so what they are shown in review
		// is what they will be shown tomorrow.
		for _, d := range a.Demonstrations() {
			if strings.EqualFold(d.Name, l.name) {
				kept := d
				out.Learned = &kept
				break
			}
		}
		return out
	}
	// WHERE IT IS, BY OUR OWN CLOCK OVER OUR OWN NUMBERS. `--delay` and `--seconds`
	// were passed by this file; counting them is arithmetic about what we asked for,
	// not a claim about what the recorder is doing. The recorder decides when it is
	// over, and `Over` above is the only thing that says it is.
	since := time.Since(started)
	if since < learnPrepare {
		out.Phase = "preparing"
		out.Left = int((learnPrepare - since).Seconds()) + 1
		out.Doing = fmt.Sprintf("Switch to %s. Recording starts in %ds.",
			l.app, out.Left)
		return out
	}
	out.Phase = "recording"
	left := time.Duration(seconds)*time.Second - (since - learnPrepare)
	if left < 0 {
		left = 0
	}
	out.Left = int(left.Seconds())
	out.Doing = "Recording. Click the control once with the mouse, then wait for the " +
		"result to settle."
	return out
}

// readLearning keeps what the recorder said, in the order a person at a terminal would see it.
func (a *Acts) readLearning(pr *os.File, l *learning) {
	defer pr.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		a.mu.Lock()
		l.lines = append(l.lines, line)
		a.mu.Unlock()
	}
}

// reviewed is what came of a recording, read from the store.
func (a *Acts) reviewed(name string, waited error) (said, next string, failed bool,
	found bool) {
	var d *Demonstration
	for _, c := range a.Demonstrations() {
		if strings.EqualFold(c.Name, name) {
			kept := c
			d = &kept
			break
		}
	}
	said, next, failed = learnedOf(name, d, waited)
	return said, next, failed, d != nil
}

// learnedOf is what came of a recording, in words, and what to do about it.
//
// # Why the artifact and not the recorder's output
//
// Because the artifact is what will be performed. The recorder prints a good report and a
// surface that read its verdict out of those sentences would be a surface whose verdict changed
// the next time somebody improved one. Everything below is a question about the saved
// demonstration, asked the same way the Act surface asks it.
//
// # Why there are four failures and not one
//
// Because they have four different remedies, and "try again" over all of them is the window
// guessing. A demonstration with no control at all means the clicks were not clicks Marco could
// correlate — usually a menu driven by the keyboard. One with a control and no outcome means the
// click landed and nothing was read afterwards, which is a pause too short. One whose starting
// screen has no words will be refused a run, for a reason that is about the screen and not the
// control. And one that saved nothing at all did not get that far.
func learnedOf(name string, d *Demonstration, waited error) (said, next string, failed bool) {
	quoted := fmt.Sprintf("%q", name)
	if d == nil {
		why := "nothing was saved"
		if waited != nil {
			why = "the recorder stopped: " + waited.Error()
		}
		return "Nothing was recorded as " + quoted + " — " + why + ".",
			"Marco could not make a control out of what it saw. Click the " +
				"control with the MOUSE: keys are watched and cannot name " +
				"a control. Technical details has what the recorder said.",
			true
	}
	presses, checks, firstRead := 0, 0, false
	for _, s := range d.Steps {
		if !s.Pressable {
			continue
		}
		if presses == 0 {
			// THE FIRST PRESS IS WHERE AN ACT OVER THIS WOULD START, and the
			// words on that screen are what would establish it.
			firstRead = len(s.On) > 0
		}
		presses++
		if s.Verifiable {
			checks++
		}
	}
	switch {
	case presses == 0:
		return "Marco recorded " + quoted + ", and nothing in it presses anything.",
			"Click the control with the MOUSE: a control is identified from a " +
				"pointer press landing inside something Marco found, and a " +
				"highlight moved by the arrow keys names nothing.",
			true
	case checks == 0:
		return "Marco saw your click and could not confirm what changed.",
			"Record it again, and after the click WAIT. Two readings are " +
				"dropped as mid-response and five have to agree before " +
				"Marco will call something a result, so a second is not " +
				"enough.",
			true
	case !firstRead:
		return fmt.Sprintf("Marco recorded %s with %d press(es) it can check, and "+
				"read no words on the screen the first one starts from.", quoted,
				checks),
			"Record it again, resting on that screen BEFORE you click so its " +
				"words are read. An Act that starts on a screen with no " +
				"words waits for it instead of pressing.",
			true
	}
	return fmt.Sprintf("Saved %s: %d press(es), %d of which Marco can perform and "+
			"check.", quoted, presses, checks),
		"Build an Act from it, and press Play on that.", false
}
