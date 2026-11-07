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
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/skills"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// readSkill is the reading of a line in a skill's explicit form, and false when it is not one.
func readSkill(line string) (lineReading, bool) {
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
		if k != s.Arg {
			return OutcomeRefused, fmt.Errorf("%s takes %s, not %s; nothing was run", s.Name, s.Arg, k)
		}
	}
	return performSkill(s, req.Named[s.Arg])
}

// performSkill does one skill with its value, and says what ran on `[ran] `.
func performSkill(s skills.Skill, value string) (Outcome, error) {
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
		w, err := skills.ChooseWindow(value, ws, isMarcoProgram)
		var amb skills.Ambiguous
		var none skills.NotOpen
		how := ""
		switch {
		case errors.As(err, &amb):
			return OutcomeClarify, err
		case errors.As(err, &none):
			// NO PROGRAM IS CALLED THAT, so the model is asked which open one was meant: nobody
			// should have to know that Steam's window is steamwebhelper.exe, and a recogniser
			// hears "chat cheap tea". It chooses among the programs that are open and nothing
			// else; Marco raises the window.
			got, ok, why := programByModel(value, ws)
			if !ok {
				return OutcomeFailed, fmt.Errorf("%v%s", err, why)
			}
			w, how = got.Window, " (Marco's model took \""+value+"\" to mean "+got.Name+")"
		case err != nil:
			return OutcomeFailed, err
		}
		if err := raiseWindow(w.Handle); err != nil {
			return OutcomeFailed, fmt.Errorf("%s is open, and %v", skills.AppName(w.Title), err)
		}
		fmt.Printf("%s%s\n", outcome.RanPrefix, readbackOf(s.Name, []linePart{{Label: s.Arg, Value: value}}))
		// THE APPLICATION'S NAME, NOT ITS WINDOW'S TITLE: a title can carry a document's name.
		fmt.Printf("brought %s forward%s\n", skills.AppName(w.Title), how)
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
	out, err := performSkill(s, value)
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
func programByModel(said string, ws []skills.Window) (skills.Program, bool, string) {
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
	label, ok := askProgram(context.Background(), said, offer)
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
