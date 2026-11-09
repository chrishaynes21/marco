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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
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
	// Phase is where the recording is, in learnstate's words, and "" when nothing is
	// being recorded.
	//
	// The RECORDER'S answer and no longer this window's arithmetic over its own flags.
	// It used to be "preparing" or "recording", derived from a clock, which is all a
	// surface can say about a process that says nothing while it works — and it left the
	// one question that decides whether a demonstration is worth anything ("has the
	// evidence arrived?") answered by advice. See learnstate.
	Phase string `json:"phase,omitempty"`
	// Doing is the sentence at the top, and Left the seconds remaining in this phase.
	Doing string `json:"doing,omitempty"`
	Left  int    `json:"left"`
	// Presses is how many presses the recorder has seen, Watched how many of those it
	// could place on something, and Captured how many have a verified outcome.
	Presses  int `json:"presses,omitempty"`
	Watched  int `json:"watched,omitempty"`
	Captured int `json:"captured,omitempty"`
	// Unchecked is how many of those presses were MEASURED and produced nothing a run
	// could check — presses there is no longer anything to wait for.
	//
	// Shown as well as the phase, because a session can have one of each and the phase
	// is about the press that just happened. See learnstate.Unchecked.
	Unchecked int `json:"unchecked,omitempty"`
	// Missing is why the last press has no outcome yet, in the recorder's own words.
	Missing string `json:"missing,omitempty"`
	// StartKnown says Marco has read enough of the screen this recording starts from to
	// recognise it again, which is what a run needs before it presses anything.
	//
	// Shown for as long as it is false rather than only when somebody tries to finish:
	// it is fixable only while the recording is running — go back to that screen and
	// rest on it — and a captured outcome does not fix it. See learnstate.Event.
	StartKnown bool `json:"start_known"`
	// Build is this window's own revision, and Recorder the revision of the director
	// that is writing the stream — empty until one has said.
	//
	// Both shown, and never reduced to a yes/no: "they differ" is the answer somebody
	// has to act on, and the two values are what tells them WHICH to rebuild. See
	// internal/buildid, and `Mismatched`.
	Build    string `json:"build,omitempty"`
	Recorder string `json:"recorder,omitempty"`
	// Mismatched says the two are both known and not the same. False while a recorder
	// has not said which build it is, because unknown is not disagreement.
	Mismatched bool `json:"mismatched,omitempty"`
	// ScreenRead says Marco has read something on the screen in front right now, which a
	// press made there needs before it can be checked at all.
	//
	// Shown as a caution the moment it is false and a press has already been seen: it is
	// the one fault that is cheaper to prevent than to report, and the phases that ask it
	// of the starting screen stop asking after the first click.
	ScreenRead bool `json:"screen_read"`
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
	// progress is where the recorder writes what it knows, one JSON object per change.
	// A temporary file: a conversation between two processes, not something a person
	// keeps. Empty when it could not be made, which is not a reason to refuse a
	// recording — see Learn.
	progress string
	done     chan struct{}
	lines    []string
	over     bool
	said     string
	next     string
	failed   bool
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
	// AND NOT THROUGH WHATEVER IS ON PATH. The recorder a learn spawns is the director,
	// and a learn watched by a build nobody chose is the day of recordings this rule
	// exists because of. See theBundleIsIncomplete.
	if why := theBundleIsIncomplete("director.exe", directorBin()); why != "" {
		return why
	}
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

	// WHERE THIS RECORDING WILL SAY WHAT IT KNOWS. One file per recording in a
	// temporary directory, exactly as a run's events are: a failure to make it is not a
	// reason to refuse the recording, and the window then falls back to its own clock.
	progress := ""
	if f, ferr := os.CreateTemp("", "studio-learn-*.jsonl"); ferr == nil {
		progress = f.Name()
		_ = f.Close()
	}
	args := []string{"demonstrate-visual", name,
		"--application", application,
		"--seconds", fmt.Sprint(seconds),
		"--delay", learnPrepare.String(),
		// ── AND PICTURES, BECAUSE THIS IS A WINDOW ──
		//
		// The recorder keeps none by default and says so: "No frames and no
		// screenshots are kept." That is the right default for a command, and it
		// is the wrong one for a surface whose whole job is to show somebody what
		// they taught. Words are what RECOGNISE a screen at run time and they are
		// not what a person recognises — the owner read
		// `controls · display · profile · sound · video` off their own Act and
		// said they could tell which screen it meant "but not easily".
		//
		// So Studio asks, and says what is kept where it asks: one picture of each
		// screen and of each control pressed, cropped from the window being
		// watched, in this machine’s own Marco directory, read by nothing that
		// runs, never sent anywhere. See internal/director/preview, and the
		// recording card in the page.
		"--previews"}
	if progress != "" {
		args = append(args, "--progress", progress)
	}
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, directorBin(), args...)
	// AND NO CONSOLE WINDOW. The recorder is a console child and Studio has just
	// brought the person's game forward; a window appearing now puts itself in front
	// of it. See hideConsole.
	hideConsole(cmd)
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
	l.progress = progress
	l.done = make(chan struct{})
	a.mu.Unlock()
	// AND THE TRIAL HAS BEGUN. Noted once the child is running, so the journal records
	// a recording that started rather than one that was asked for.
	a.trial.recording(name, application, seconds)

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
		// AND WHAT CAME OF IT, WITH THE PICTURES THE STORE KEPT. Read after the
		// child exited and from the store rather than from what it said, for the
		// reason `reviewed` is: what gets recorded is what was actually saved.
		a.trial.recorded(l.name, said, next, failed, a.Pictures(l.name))
	}()
	return ""
}

// Done tells the recorder the person has finished showing it.
//
// `anyway` is the person saying it a second time, having been told what is still missing.
//
// # Why finishing is checked at all
//
// Because the recorder knows whether it obtained evidence and the person does not. A click
// whose outcome has not been read yet produces an interaction nothing can check: the Act built
// from it presses and then reports that it could not confirm anything, and nothing after it is
// performed. That is a wasted demonstration, and the only moment it can be prevented is the
// moment before the recording ends — a few hundred milliseconds of waiting, which the recorder
// can ask for and a person cannot estimate.
//
// So the FIRST press of "that's it" over a missing outcome refuses, and says what is missing in
// the recorder's own words. The second one finishes: it is their demonstration, the evidence may
// never arrive, and a window that would not stop recording is worse than one that recorded
// nothing useful.
func (a *Acts) Done(anyway bool) string {
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
	if !anyway {
		if why := a.notYet(); why != "" {
			return why
		}
	}
	// THE RECORDER'S OWN ENDING, not a kill: it then reports, learns a draft from the
	// walk, saves the artifact and writes the evidence beside it. A window that killed
	// the process here would throw all of that away and call it finishing.
	if _, err := in.Write([]byte("\n")); err != nil {
		return "the recorder could not be told to finish: " + err.Error()
	}
	return ""
}

// notYet is what is still missing from this recording, or "" when nothing is.
//
// Reads the recorder's own stream and nothing else. A window that decided this for itself would
// be estimating the thing the recorder measured. See learnstate and Done.
func (a *Acts) notYet() string {
	at, over, foreign := a.recordingNow()
	if foreign {
		return "the recorder running is a different build from this window, so " +
			"nothing it says can be read here. Rebuild with " +
			"plugins/studio/build.cmd and record again"
	}
	if over || at.Phase == "" {
		return ""
	}
	// ── WHAT IS MISSING ABOUT THE PRESS IN FLIGHT ──
	why := ""
	switch at.Phase {
	case learnstate.Reading:
		why = "not enough has been read on the screen you are starting from yet, " +
			"so an Act built from this could not tell it was there. Rest " +
			"where you are for a moment, then click"
	case learnstate.Ready:
		why = "no press has been seen yet, so there is nothing to save. Click the " +
			"control once with the mouse"
	case learnstate.Checking:
		said := at.Missing
		if said == "" {
			said = "the words that say it worked have not been read yet"
		}
		why = "the result of your click has not been captured yet: " + said +
			". Keep recording for a moment"
	case learnstate.Blind:
		why = "the press landed on nothing Marco could see, so nothing can be " +
			"replayed from it. Click the control itself, on its label"
	case learnstate.Unchecked:
		// AND THIS ONE DOES NOT SAY "KEEP RECORDING FOR A MOMENT".
		//
		// `Checking` says that, and it is right: the readings are still
		// arriving. Here they have arrived, so the same sentence would be the
		// window asking for a wait it has no use for — which is what it did for
		// twenty-nine seconds of the owner’s Rocket League recording. The
		// refusal stands, because finishing now saves a press with no check on
		// it, and what it asks for is a second attempt rather than patience.
		said := at.Missing
		if said == "" {
			said = "the words that say it worked were never read"
		}
		why = "the result of your click could not be captured: " + said +
			". Waiting will not change that now — try the control once " +
			"more from the screen before it"
	}
	// ── AND THE SCREEN AN ACT WOULD START ON, WHICH IS A SEPARATE FAULT ──
	//
	// Said as well as the above and not instead of it, because a captured outcome used
	// to end this question: the phase said `captured`, this said nothing, and the Act
	// that came out of it paused on a screen it could not name. The two have different
	// remedies and the person is owed both. `Reading` is already this sentence, so it is
	// not said twice.
	if !at.StartKnown && at.Phase != learnstate.Reading {
		start := "Marco has not read enough of the screen this starts from to " +
			"recognise it, so an Act built from this would wait for it " +
			"instead of pressing. Go back to that screen and rest there for " +
			"a moment"
		if why == "" {
			why = start
		} else {
			why += ". Also: " + start
		}
	}
	if why == "" {
		return ""
	}
	// THE WAY OUT, ONCE, wherever the sentence came from. It used to be written into
	// each of them, which is four places for one promise that the button has to keep.
	return why + " — or finish anyway."
}

// recordingNow is the recorder's latest account of itself, and whether it said it had ended.
//
// An unreadable or absent stream is an empty event and not an error: a recording whose progress
// file could not be made is a recording that still works, and every caller treats an empty
// phase as "the recorder has not said".
func (a *Acts) recordingNow() (at learnstate.Event, over, foreign bool) {
	a.mu.Lock()
	path := ""
	if a.learn != nil {
		path = a.learn.progress
	}
	a.mu.Unlock()
	if path == "" {
		return learnstate.Event{}, false, false
	}
	at, final, err := learnstate.Latest(path)
	if errors.Is(err, learnstate.ErrForeign) {
		// A RECORDER THIS BUILD CANNOT READ, which is not a recorder that has
		// said nothing. Reported as itself so the window can say which it is.
		return learnstate.Event{}, false, true
	}
	if err != nil {
		return learnstate.Event{}, false, false
	}
	return at, final, false
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
//
// WHAT IT HANDS BACK IS ALSO WHAT THE TRIAL RECORDS, which is why the journal is written from
// here rather than from a second reader of the recorder's stream. The claim the evidence has to
// support is about the states the OWNER WAS SHOWN while they were demonstrating — "settling is
// ass, it doesn't finish in time" is a sentence about this value — and a separate reader would
// be a record of what the window could have shown instead. Written only when it changed; see
// trials.saw.
func (a *Acts) Learning() Learning {
	out := a.learningNow()
	a.trial.saw(out)
	return out
}

// learningNow is Learning's answer.
func (a *Acts) learningNow() Learning {
	a.mu.Lock()
	l := a.learn
	if l == nil {
		a.mu.Unlock()
		return Learning{}
	}
	out := Learning{Name: l.name, Application: l.app, Active: !l.over,
		Over: l.over, Said: l.said, Next: l.next, Failed: l.failed,
		Transcript: append([]string(nil), l.lines...),
		// AND WHICH BUILD THIS WINDOW IS, so the one beside it can be
		// compared against something rather than against a timestamp.
		Build: a.myBuild()}
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
	// ── WHERE IT IS, BY OUR OWN CLOCK OVER OUR OWN NUMBERS ──
	//
	// `--delay` and `--seconds` were passed by this file; counting them is arithmetic
	// about what we asked for, not a claim about what the recorder is doing. The
	// recorder decides when it is over, and `Over` above is the only thing that says
	// it is.
	//
	// This is now the FALLBACK and the clock for the preparation delay, which is the one
	// stretch the recorder cannot report on: it has not started looking. Everything after
	// it comes off the stream below.
	since := time.Since(started)
	left := time.Duration(seconds)*time.Second - (since - learnPrepare)
	if left < 0 {
		left = 0
	}
	out.Left = int(left.Seconds())
	if since < learnPrepare {
		out.Phase = string(learnstate.Preparing)
		out.Left = int((learnPrepare - since).Seconds()) + 1
		out.Doing = fmt.Sprintf("Switch to %s. Recording starts in %ds.",
			l.app, out.Left)
		return out
	}

	// ── AND WHAT THE RECORDER SAYS, WHICH OUTRANKS IT ──
	//
	// The one question a person doing a demonstration needs answered is whether the
	// evidence has arrived, and this window cannot know it: the outcome is four terms
	// over readings on both sides of a press. The recorder measures them. So what it
	// says is what is shown, and the sentence per phase is stated once, in learnstate,
	// where both this and a test can reach it.
	at, _, foreign := a.recordingNow()
	if foreign {
		// SAID WHERE THE PHASE SENTENCE GOES, because this is what is happening:
		// the recording is running and this window cannot read a word of it.
		// Showing "Reading the starting screen…" here is what cost the owner a
		// day — a current window narrating a recorder it had never met.
		out.Phase = string(learnstate.Reading)
		out.Doing = "The recorder running is a different build from this window. " +
			"Nothing it reports can be read here — rebuild with " +
			"plugins/studio/build.cmd and record again."
		return out
	}
	if at.Phase == "" || at.Phase == learnstate.Ended {
		// NOTHING SAID YET. The child has started and its first look has not
		// landed, which is real and lasts a fraction of a second.
		out.Phase = string(learnstate.Reading)
		out.Doing = learnstate.Doing(learnstate.Reading, l.app)
		return out
	}
	out.Phase = string(at.Phase)
	out.Doing = learnstate.Doing(at.Phase, l.app)
	out.Presses, out.Watched, out.Captured = at.Presses, at.Watched, at.Captured
	out.Unchecked, out.Missing = at.Unchecked, at.Missing
	out.StartKnown, out.ScreenRead = at.StartKnown, at.ScreenRead
	// WHICH BUILD IS DOING THE RECORDING, beside which build is watching it.
	//
	// The protocol version refuses a stream this build cannot parse; this is the case
	// where the shape happens to match and the behaviour does not, and it is the thing
	// the owner had to work out from file timestamps. Nobody should have to.
	out.Recorder = at.Build
	// `Differ` and not `!Same`: an unstamped binary cannot vouch for a match AND
	// cannot be seen to disagree, and warning on it would make every pair that
	// carries no stamp an alarm about a difference nobody observed.
	out.Mismatched = buildid.Differ(out.Build, at.Build)
	return out
}

// myBuild is this window's revision, or whatever a test put in its place.
func (a *Acts) myBuild() string {
	if a.revision != nil {
		return a.revision()
	}
	return buildid.Revision()
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
// screen cannot be identified will be refused a run, for a reason that is about the screen and
// not the control. And one that saved nothing at all did not get that far.
//
// # And why the starting screen is asked about through the run's own term
//
// `Interaction.Recognisable`, which is `observe.ScreenText.Identifies` over the artifact's
// vocabulary for that screen — not `len(On) > 0`. `On` is a sample of six words at most for
// somebody to read, so its length cannot answer whether the screen has the three a run needs,
// and a two-word screen passed this and produced an Act that could only wait.
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
			//
			// `Recognisable` and not `len(s.On) > 0`: `On` is a sample of six
			// words at most for a person to read, so its length cannot answer
			// whether the screen has the three a run needs to tell it from
			// another. A two-word screen passed this and the Act built over it
			// waited for a screen the run would not identify.
			firstRead = s.Recognisable
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
				"did not read enough of the screen the first one starts from "+
				"to recognise it again.", quoted, checks),
			"Record it again, resting on that screen BEFORE you click so its " +
				"words are read. It takes three words to tell one screen " +
				"from another, and an Act that starts on a screen it cannot " +
				"name waits for it instead of pressing.",
			true
	}
	return fmt.Sprintf("Saved %s: %d press(es), %d of which Marco can perform and "+
			"check.", quoted, presses, checks),
		"Build an Act from it, and press Run on that.", false
}
