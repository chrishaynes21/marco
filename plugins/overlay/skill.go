package main

// A BUILT-IN DESKTOP SKILL DIRECTOR NAMED, REVIEWED ON THE COMMAND LINE.
//
// "switch me back to Discord" names no routine; Director's resolver may answer that it is the
// built-in skill Focus app, with app: Discord. The answer carries the skill's explicit line
// ("focus Discord"), and the command line is opened on it: the engine reads it back as
// "Focus app · app: Discord", the value can be changed in place, and Enter runs it like any
// read-back (`marco do --skill=focus-app --arg=app=…`). Nothing runs from the answer itself.
//
// The line is read AS A SKILL OR NOT AT ALL: if a saved routine has taken those words (a routine
// called "focus" that takes a value), the line is not put on screen, because the read-back would
// be of a routine Director never named. See [[ADR-327-built-in-desktop-skills]] in the engine.

import "strings"

// reviewSkill opens the command line on a skill Director named, or says why not.
//
// Running the skill from the answer without this review must fail
// TestASkillDirectorNamedIsReviewedBeforeItRuns.
func reviewSkill(h *model, name, slug, line string, times int) disposition {
	if times > 0 {
		h.log(name + " is one of Marco's built-in skills, and those are not repeated, so nothing was run")
		return saidWhyNot
	}
	line = strings.TrimSpace(line)
	if line == "" {
		h.log("Director named " + name + " and said nothing to put on the command line, so nothing was run")
		return saidWhyNot
	}
	r, ok := askLine(line, nil)
	switch {
	case !ok:
		h.log("Marco could not read " + name + " back, so nothing was run")
		return saidWhyNot
	case r.Kind != "skill" || r.Slug != slug:
		h.log("Director named the built-in " + name + ", and \"" + line + "\" here is " +
			saidAs(r) + ", so nothing was put on the command line and nothing was run")
		return saidWhyNot
	}
	h.clearProposal()
	h.setReading(line, r)
	reopenLine(h, line)
	if r.Runnable {
		h.setStatus("Enter runs it · edit to change")
	} else {
		h.setStatus(r.Says)
	}
	return offeredRun
}

// saidAs names what a line read as instead.
func saidAs(r lineReading) string {
	if r.Kind == "routine" {
		return "the play " + routeWords(r.App, r.Slug, r.Focus)
	}
	return "something else"
}

// focusSkill is Focus app's slug, the engine's (internal/skills).
const focusSkill = "focus-app"

// focusRan is the program a Focus app run brought forward, from its `[ran] ` line ("Focus app ·
// program: ChatGPT"), when it performed.
func focusRan(r childRun) (string, bool) {
	if r.result != "performed" {
		return "", false
	}
	prog, ok := strings.CutPrefix(r.ran, "Focus app · program: ")
	return strings.TrimSpace(prog), ok && strings.TrimSpace(prog) != ""
}

// searchSkill is Search the web's slug, the engine's (internal/skills).
const searchSkill = "search-the-web"

// firstEnterRuns is whether a skill runs on the first Enter rather than after its read-back:
// Focus app always, and Search the web unless the person asked for searches to be confirmed
// (overlay.json "confirmSearch", off by default — the owner's call: "I don't mind that").
//
// Ignoring the setting must fail TestASearchRunsOnTheFirstEnterUnlessConfirmed.
func firstEnterRuns(r lineReading) bool {
	return r.Slug == focusSkill || (r.Slug == searchSkill && !confirmSearch())
}

// confirmSearch is the setting, read where it is needed. A package variable so a test can set it;
// production never reassigns it.
var confirmSearch = func() bool { return cfgSnapshot().ConfirmSearch }
