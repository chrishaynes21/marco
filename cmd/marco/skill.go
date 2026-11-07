package main

// BUILT-IN DESKTOP SKILLS ON THE COMMAND LINE AND AT THE DOOR (internal/skills).
//
// `marco line` reads "focus Discord" as the skill Focus app with app: Discord, but only when no
// saved routine took the line — a routine's name always wins — and reads it back as
// "Focus app · app: Discord". Enter on that read-back runs `marco do --skill=focus-app
// --arg=app=Discord`, which performs the skill by its identity and never reads words again.
//
// Words reaching the intake in a skill's explicit form (a terminal, a reading that timed out) are
// performed as the skill after every routine has had its turn, as words naming a routine with its
// value are. A paraphrase is Director's to read: its resolver is offered the skills beside the
// routines, and an answer naming one is PROPOSED with the skill's explicit line for review,
// never performed. See [[ADR-327-built-in-desktop-skills]].

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
	"github.com/chaynes-simpleclouds/marco/internal/skills"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// readSkill is the reading of a line in a skill's explicit form, and false when it is not one.
//
// FOCUS APP IS READ BACK WITH THE PROGRAM IT WILL BRING FORWARD, not only the words: "Focus app ·
// app: chat gp jpg · program: ChatGPT", and the window's identity travels as the value `window`,
// so Enter raises the window that was read back and nothing is worked out again. When no open
// program is called the words, the model is asked — only with resolve (`--resolve`), because the
// reading is asked as somebody types; without it the reading says it is finding the program, is
// not runnable, and asks to be read again (Resolving).
//
// Reading back the words without the program must fail TestFocusIsReadBackWithTheProgramItBringsForward.
func readSkill(line string, resolve bool) (lineReading, bool) {
	s, value, ok := skillOf(line)
	if !ok {
		return lineReading{}, false
	}
	out := lineReading{Line: line, Kind: "skill", ID: s.ID(), Slug: s.Slug, Name: s.Name,
		Form: "skill", Needs: true}
	if value == "" {
		out.Missing = []string{s.Arg}
		out.Pending = pendingSlots(out.Missing, true)
		out.Says = neededWord(out.Missing) + " · " + s.Name + " " + out.Pending
		return out, true
	}
	out.Values = map[string]string{s.Arg: value}
	out.From = map[string]string{s.Arg: "typed"}
	out.Parts = []linePart{{Label: s.Arg, Value: value}}
	if s.Slug == skills.Search {
		// WHERE IT WILL BE SEARCHED, before it is: the words leave the machine. Shown as a part
		// of the read-back and never sent as a value: the provider is the setting's.
		provider := searchProvider()
		if _, err := skills.SearchURL(provider, value); err != nil {
			out.Says = err.Error()
			return out, true
		}
		out.Parts = append(out.Parts, linePart{Label: "provider", Value: skills.ProviderName(provider)})
	}
	if s.Slug == skills.Focus {
		ws := openWindows()
		w, err := skills.ChooseWindow(value, ws, isMarcoProgram)
		var none skills.NotOpen
		switch {
		case errors.As(err, &none) && !resolve:
			out.Resolving = true
			out.Says = "finding the program…"
			return out, true
		case errors.As(err, &none):
			p, ok, why := programByModel(value, ws, nil)
			if !ok {
				out.Says = err.Error() + why
				return out, true
			}
			w = p.Window
		case err != nil:
			out.Says = err.Error()
			return out, true
		}
		out.Values["window"] = windowTerm(w)
		// THE PROGRAM, NOT WHAT WAS HEARD: once a program is chosen, the words that asked for it
		// ("church tpg") say nothing anybody needs. They still travel as the value.
		out.Parts = []linePart{{Label: "program", Value: skills.AppName(w.Title)}}
	}
	out.Readback = readbackOf(s.Name, out.Parts)
	out.Runnable = true
	return out, true
}

// searchProvider is the configured search provider (overlay.json "search"), "" for the default.
var searchProvider = func() string { return widgetconfig.Load().Search }

// performSkillRequest performs a skill confirmed by its identity, `--skill=… --arg=<arg>=…`.
func performSkillRequest(req invoke.Request) (Outcome, error) {
	s, ok := skills.Of(req.Skill)
	if !ok {
		return OutcomeRefused, fmt.Errorf("%q is not one of Marco's built-in skills", req.Skill)
	}
	if req.Times > 0 {
		return OutcomeRefused, fmt.Errorf("a built-in skill is not repeated; nothing was run")
	}
	for k := range req.Named {
		if k != s.Arg && !(s.Slug == skills.Focus && k == "window") {
			return OutcomeRefused, fmt.Errorf("%s takes %s, not %s; nothing was run", s.Name, s.Arg, k)
		}
	}
	return performSkill(s, req.Named[s.Arg], req.Named["window"])
}

// performSkill does one skill with its value, and says what ran on `[ran] `. window is the window
// a Focus app read-back named (windowTerm), or "" to work it out here (a terminal, the intake).
//
// Working the window out again when one was read back must fail TestEnterRaisesTheWindowThatWasReadBack.
func performSkill(s skills.Skill, value, window string) (Outcome, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return OutcomeClarify, fmt.Errorf("%s · %s", neededWord([]string{s.Arg}), s.Name+" "+
			pendingSlots([]string{s.Arg}, true))
	}
	// THE SKILL, NEVER THE VALUE: a query is somebody's words.
	mlog.Info("intake: a built-in skill", "skill", s.Slug)
	switch s.Slug {
	case skills.Focus:
		ws := openWindows()
		var w skills.Window
		var err error
		if window != "" {
			// THE WINDOW THAT WAS READ BACK, still that window, or nothing.
			var ok bool
			if w, ok = windowOf(window); !ok {
				return OutcomeFailed, fmt.Errorf("the window that was read back has closed since, " +
					"so nothing was brought forward")
			}
		} else {
			w, err = skills.ChooseWindow(value, ws, isMarcoProgram)
		}
		var amb skills.Ambiguous
		var none skills.NotOpen
		switch {
		case errors.As(err, &amb):
			return OutcomeClarify, err
		case errors.As(err, &none):
			// NO PROGRAM IS CALLED THAT, so the model is asked which open one was meant: nobody
			// should have to know that Steam's window is steamwebhelper.exe, and a recogniser
			// hears "chat cheap tea". It chooses among the programs that are open and nothing
			// else; Marco raises the window.
			got, ok, why := programByModel(value, ws, nil)
			if !ok {
				return OutcomeFailed, fmt.Errorf("%v%s", err, why)
			}
			w = got.Window
		case err != nil:
			return OutcomeFailed, err
		}
		if err := raiseWindow(w.Handle); err != nil {
			return OutcomeFailed, fmt.Errorf("%s is open, and %v", skills.AppName(w.Title), err)
		}
		// WHAT RAN, WITH THE PROGRAM IT BROUGHT FORWARD, not only the words that asked for it.
		fmt.Printf("%s%s\n", outcome.RanPrefix, readbackOf(s.Name, []linePart{{Label: "program",
			Value: skills.AppName(w.Title)}}))
		// THE APPLICATION'S NAME, NOT ITS WINDOW'S TITLE: a title can carry a document's name.
		fmt.Printf("brought %s forward\n", skills.AppName(w.Title))
		return OutcomePerformed, nil
	case skills.Search:
		u, err := skills.SearchURL(searchProvider(), value)
		if err != nil {
			return OutcomeRefused, err
		}
		if err := openAddress(u); err != nil {
			return OutcomeFailed, fmt.Errorf("the browser could not be asked to open the results: %v", err)
		}
		fmt.Printf("%s%s\n", outcome.RanPrefix, readbackOf(s.Name, []linePart{{Label: s.Arg, Value: value}}))
		return OutcomePerformed, nil
	}
	return OutcomeRefused, fmt.Errorf("%s has nothing to perform it", s.Name)
}

// skillInWords performs words in a skill's explicit form that reached the intake, when no routine
// took them. ok is false when they are not that form.
//
// A ROUTINE THE COMMAND LINE WOULD READ IN THEM WINS, in any form: "focus Discord" with a routine
// called "focus" that takes a value is that routine's, not the skill's, even here where the
// intake does not read a bare value itself — the words go on to Director, which names the
// routine for review, as they did before there were skills.
//
// Skipping the check must fail TestASavedRoutineWinsOverASkill.
func skillInWords(d orchestrator.Deps, phrase, app string) (Outcome, bool, error) {
	s, value, ok := skillOf(phrase)
	if !ok {
		return "", false, nil
	}
	if _, _, _, _, routine := lineRoutine(d, app, phrase); routine {
		return "", false, nil
	}
	out, err := performSkill(s, value, "")
	return out, true, err
}

// openWindows, raiseWindow and openAddress are the desktop a skill acts on. Package variables so
// a test can stand in for the desktop; production never reassigns them.
var (
	openWindows = func() []skills.Window {
		var out []skills.Window
		for _, w := range winctx.LiveWindows() {
			// A WINDOW WITH NO TITLE is a program's helper, not a window anybody switches to.
			if strings.TrimSpace(w.Title) == "" {
				continue
			}
			out = append(out, skills.Window{Handle: w.Handle, PID: w.ProcessID, Image: w.Image,
				Title: w.Title, Minimized: w.Minimized})
		}
		return out
	}
	raiseWindow = winctx.Raise
	openAddress = winctx.Launch
)

// marcoPrograms are Marco's own programs, which "focus" never brings forward.
var marcoPrograms = map[string]bool{"marco": true, "director": true, "overlay": true,
	"marco-app": true, "marco-macros": true, "studio": true, "web-ui": true, "voice": true,
	"llama": true}

func isMarcoProgram(image string) bool { return marcoPrograms[strings.ToLower(image)] }

// skillProposal is whether a proposal line names a built-in skill.
func skillProposal(line string) bool {
	var out struct {
		Answer struct {
			Builtin bool `json:"builtin"`
		} `json:"answer"`
	}
	return json.Unmarshal([]byte(line), &out) == nil && out.Answer.Builtin
}

// programByModel asks the configured model which open program somebody meant, and answers that
// program, or why there is none to add to "isn't open".
//
// Asking nothing when no program matched exactly must fail TestAProgramNotNamedExactlyIsAskedOfTheModel.
func programByModel(said string, ws []skills.Window, routines []string) (skills.Program, bool, string) {
	ps := skills.Programs(ws, isMarcoProgram)
	if len(ps) == 0 {
		return skills.Program{}, false, ""
	}
	if !modelConfigured() {
		return skills.Program{}, false, "; no model is set up to work out which open program you meant"
	}
	offer := make([]resolver.ProgramChoice, 0, len(ps))
	byLabel := map[string]skills.Program{}
	for _, p := range ps {
		offer = append(offer, resolver.ProgramChoice{Label: p.Label, Name: p.Name, Exe: p.Window.Image})
		byLabel[p.Label] = p
	}
	label, ok := askProgram(context.Background(), said, offer, routines)
	if !ok {
		mlog.Info("intake: the model named no open program", "offered", len(offer))
		return skills.Program{}, false, ", and Marco's model didn't recognise it as any program that is open"
	}
	mlog.Info("intake: the model named an open program", "program", byLabel[label].Window.Image)
	return byLabel[label], true, ""
}

// askProgram is the model's answer. A package variable so a test answers for it; production never
// reassigns it.
var askProgram = resolver.WhichProgram

// modelConfigured is whether a model can be asked at all, a variable for the same reason.
var modelConfigured = resolver.Configured

// keepSkillAttempt writes down what became of a skill run by its identity, for "why didn't that
// work?" (lastattempt.go): the skill, the outcome and why, with what was said scrubbed out of it
// as every attempt's is.
func keepSkillAttempt(req invoke.Request, out Outcome, err error) {
	a := lastattempt.Attempt{At: time.Now().Format(time.RFC3339), Source: string(req.Source),
		Outcome: string(out), Stage: "running"}
	if s, ok := skills.Of(req.Skill); ok {
		a.Routine = s.Name
	}
	if err != nil {
		var said []string
		for _, v := range req.Named {
			said = append(said, v)
		}
		a.Because = lastattempt.Scrub(err.Error(), said)
	}
	if werr := lastattempt.Write(attemptAt(), a); werr != nil {
		mlog.Info("intake: the latest attempt could not be written down", "err", werr)
	}
}

// skillOf is the skill a line asks for: an explicit form ("focus Discord", "search the web for
// …"), or everyday words ("open chat gp t", "switch me back to Discord") when what follows them
// is the name of a program open right now (skills.Opens). The window list is read only for a line
// that begins with those words.
//
// Claiming them without the program being open must fail TestEverydayWordsAreFocusOnlyForAnOpenProgram.
func skillOf(line string) (skills.Skill, string, bool) {
	if s, v, ok := skills.Read(line); ok {
		return s, v, true
	}
	if rest, ok := skills.ReadOpen(line); ok && skills.NamesOpenProgram(rest, openWindows(), isMarcoProgram) {
		s, _ := skills.Of(skills.Focus)
		return s, rest, true
	}
	return skills.Skill{}, "", false
}

// proposeOpenProgram answers everyday words naming an open program the recogniser misheard
// ("open chad gpt" with ChatGPT open): the model is asked which open program was meant, and the
// one it names is PROPOSED as "focus <its name>" for review on the widget's command line, never
// raised from here — the words were two guesses, that an application was meant and which. ok is
// false when they are not that form, a routine reads them, or the model names nothing; they then
// go on to Director as before.
//
// Not asking must fail TestAMisheardOpenIsProposedAsFocus.
func proposeOpenProgram(d orchestrator.Deps, phrase, app string) (Outcome, bool) {
	rest, ok := skills.ReadOpen(phrase)
	if !ok {
		return "", false
	}
	if _, _, _, _, routine := lineRoutine(d, app, phrase); routine {
		return "", false
	}
	// YOUR ROUTINES' NAMES GO WITH IT, as not applications: "open sosh al" is "open social"
	// misheard, and is left for Director. Measured live, ADR-327.
	var names []string
	for _, rt := range d.Reg.List() {
		names = append(names, plays.Pretty(rt.Slug))
	}
	p, ok, _ := programByModel(rest, openWindows(), names)
	if !ok {
		return "", false
	}
	s, _ := skills.Of(skills.Focus)
	b, err := json.Marshal(service.ResolveRouteResponse{Answer: resolveroute.Answer{
		Kind: dispatch.IntentRun, ID: s.ID(), Slug: s.Slug, Name: s.Name, Scope: "built in",
		Source: "advisor", Resolver: "Marco's model named an open program",
		Builtin: true, Line: skills.LineFor(s.Slug, p.Name)}})
	if err != nil {
		return "", false
	}
	fmt.Printf("%s%s\n", outcome.ProposedPrefix, b)
	fmt.Printf("Marco's model took \"%s\" to mean %s, which is open. Nothing has run; review it first.\n",
		rest, p.Name)
	return OutcomeClarify, true
}

// windowTerm is a window's identity on the wire, `<handle>:<owner>`: the window AND the process
// that owned it when it was read back, so a handle Windows has given to another window since is
// not raised.
func windowTerm(w skills.Window) string { return fmt.Sprintf("%d:%d", w.Handle, w.PID) }

// windowOf is the window a term names, if it is still that window.
func windowOf(term string) (skills.Window, bool) {
	h, p, ok := strings.Cut(strings.TrimSpace(term), ":")
	if !ok {
		return skills.Window{}, false
	}
	hwnd, err1 := strconv.ParseUint(h, 10, 64)
	pid, err2 := strconv.ParseUint(p, 10, 32)
	if err1 != nil || err2 != nil || hwnd == 0 {
		return skills.Window{}, false
	}
	return lookUpWindow(uintptr(hwnd), uint32(pid))
}

// lookUpWindow is the window with a handle, if that process still owns it. A package variable so
// a test can stand in for the desktop; production never reassigns it.
var lookUpWindow = func(hwnd uintptr, pid uint32) (skills.Window, bool) {
	lw, ok := winctx.LookUpWindow(hwnd)
	if !ok || lw.ProcessID != pid {
		return skills.Window{}, false
	}
	return skills.Window{Handle: lw.Handle, PID: lw.ProcessID, Image: lw.Image, Title: lw.Title,
		Minimized: lw.Minimized}, true
}
