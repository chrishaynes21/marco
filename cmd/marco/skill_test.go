package main

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/skills"
)

// skillDesk stands in for the desktop a skill acts on: these windows, front to back, and a record
// of what was raised and what address was opened. Nothing reaches the real one.
type skillDesk struct {
	raised []uintptr
	opened []string
}

func skillWorld(t *testing.T, provider string, ws ...skills.Window) (orchestrator.Deps, *skillDesk) {
	t.Helper()
	d := repeatWorld(t)
	desk := &skillDesk{}
	prevW, prevR, prevO, prevP := openWindows, raiseWindow, openAddress, searchProvider
	openWindows = func() []skills.Window { return ws }
	raiseWindow = func(h uintptr) error { desk.raised = append(desk.raised, h); return nil }
	openAddress = func(u string) error { desk.opened = append(desk.opened, u); return nil }
	searchProvider = func() string { return provider }
	// NO MODEL unless a test says otherwise: $MARCO_RESOLVER may be set on this machine.
	prevM, prevA := modelConfigured, askProgram
	modelConfigured = func() bool { return false }
	askProgram = func(context.Context, string, []resolver.ProgramChoice) (string, bool) { return "", false }
	t.Cleanup(func() { modelConfigured, askProgram = prevM, prevA })
	t.Cleanup(func() { openWindows, raiseWindow, openAddress, searchProvider = prevW, prevR, prevO, prevP })
	return d, desk
}

var discordDesk = []skills.Window{
	{Handle: 11, Image: "overlay", Title: "Marco"},
	{Handle: 12, Image: "discord", Title: "#raids - Discord"},
	{Handle: 13, Image: "discord", Title: "Discord"},
	{Handle: 14, Image: "code", Title: "x.go - Visual Studio Code"},
}

// THE EXPLICIT FORM IS READ BACK ON THE COMMAND LINE AS THE SKILL, and the bare name asks for its
// value, so a recalled "Focus app" asks for the app again.
func TestALineInASkillsFormReadsAsTheSkill(t *testing.T) {
	d, _ := skillWorld(t, "")
	r := readLine(d, "", "focus Discord", lineProposal{})
	if r.Kind != "skill" || r.Slug != skills.Focus || !r.Runnable || r.Values["app"] != "Discord" ||
		r.Readback != "Focus app · app: Discord" {
		t.Errorf("focus Discord read as %+v", r)
	}
	r = readLine(d, "", "Focus app", lineProposal{})
	if r.Kind != "skill" || r.Runnable || r.Pending != "‹app›" || !strings.Contains(r.Says, "App needed") {
		t.Errorf("Focus app read as %+v", r)
	}
	r = readLine(d, "", "search the web for Ark black pearl locations", lineProposal{})
	if r.Kind != "skill" || !r.Runnable || r.Values["query"] != "Ark black pearl locations" ||
		!strings.HasSuffix(r.Readback, "· provider: google") {
		t.Errorf("the search read as %+v", r)
	}
	if _, ok := r.Values["provider"]; ok {
		t.Error("the provider was sent as a value")
	}
	// A PROVIDER THAT IS NOT A WEB ADDRESS IS SAID BEFORE ENTER, not discovered after.
	d, _ = skillWorld(t, "calc.exe")
	if r := readLine(d, "", "search the web for x", lineProposal{}); r.Runnable || r.Says == "" {
		t.Errorf("a bad provider read as runnable: %+v", r)
	}
}

// A SAVED ROUTINE'S NAME WINS OVER A SKILL'S WORDS, on the command line and at the door.
func TestASavedRoutineWinsOverASkill(t *testing.T) {
	d, desk := skillWorld(t, "", discordDesk...)
	focus := routes.Route{Slug: "focus"}
	if err := d.Reg.Save(focus, runnableSrc+"// {{text}}\n"); err != nil {
		t.Fatal(err)
	}
	if r := readLine(d, "", "focus Discord", lineProposal{}); r.Kind != "routine" || r.Slug != "focus" {
		t.Errorf("the skill shadowed the routine: %+v", r)
	}
	_ = stdoutOf(t, func() {
		_, _ = runInvocation(d, invoke.Request{Text: "focus Discord", Source: invoke.SourceTyped})
	})
	if len(desk.raised) != 0 {
		t.Errorf("the skill ran instead of the routine: raised %v", desk.raised)
	}
}

// A SKILL CONFIRMED BY ITS IDENTITY RUNS: the program's most recently active window comes
// forward, and what ran is said. A program that is not open is said and nothing is raised.
func TestASkillRunsByItsIdentity(t *testing.T) {
	d, desk := skillWorld(t, "duckduckgo", discordDesk...)
	run := func(args ...string) (Outcome, string, error) {
		t.Helper()
		req, _, _, err := parseDoArgs(args)
		if err != nil {
			return "", "", err
		}
		var out Outcome
		said := stdoutOf(t, func() { out, err = runInvocation(d, req) })
		return out, said, err
	}
	out, said, err := run("--source=typed", "--skill=focus-app", "--arg=app=Discord")
	if out != OutcomePerformed || err != nil || len(desk.raised) != 1 || desk.raised[0] != 12 {
		t.Fatalf("focus Discord: %q %v raised %v\n%s", out, err, desk.raised, said)
	}
	if lastRan(said) != "Focus app · app: Discord" {
		t.Errorf("ran %q", lastRan(said))
	}
	out, _, err = run("--skill=focus-app", "--arg=app=Spotify")
	if out != OutcomeFailed || err == nil || !strings.Contains(err.Error(), "isn't open") ||
		len(desk.raised) != 1 {
		t.Errorf("Spotify: %q %v raised %v", out, err, desk.raised)
	}
	// MARCO'S OWN WINDOWS ARE NEVER BROUGHT FORWARD, even by name.
	if out, _, _ = run("--skill=focus-app", "--arg=app=overlay"); out == OutcomePerformed || len(desk.raised) != 1 {
		t.Errorf("focus overlay ended %q, raised %v", out, desk.raised)
	}
	out, said, err = run("--skill=search-the-web", "--arg=query=Ark black pearl locations")
	if out != OutcomePerformed || err != nil || len(desk.opened) != 1 ||
		desk.opened[0] != "https://duckduckgo.com/?q=Ark+black+pearl+locations" {
		t.Errorf("search: %q %v opened %v\n%s", out, err, desk.opened, said)
	}
	for _, bad := range [][]string{
		{"--skill=focus-app", "--arg=app=Discord", "--times=2"},
		{"--skill=focus-app", "--arg=window=Discord"},
		{"--skill=launch-rockets", "--arg=app=x"},
	} {
		if out, _, _ := run(bad...); out != OutcomeRefused {
			t.Errorf("%q ended %q", bad, out)
		}
	}
	if len(desk.raised) != 1 || len(desk.opened) != 1 {
		t.Errorf("a refused skill acted: raised %v opened %v", desk.raised, desk.opened)
	}
}

// TWO PROGRAMS ANSWERING TO THE NAME IS A QUESTION, AND NOTHING COMES FORWARD.
func TestAnAmbiguousAppIsAskedAbout(t *testing.T) {
	d, desk := skillWorld(t, "", skills.Window{Handle: 1, Image: "discordptb", Title: "Discord PTB"},
		skills.Window{Handle: 2, Image: "discordcanary", Title: "Discord Canary"})
	var out Outcome
	var err error
	_ = stdoutOf(t, func() {
		out, err = runInvocation(d, invoke.Request{Text: "focus discord", Source: invoke.SourceSpoken})
	})
	if out != OutcomeClarify || err == nil || !strings.Contains(err.Error(), "say which") ||
		len(desk.raised) != 0 {
		t.Errorf("%q %v raised %v", out, err, desk.raised)
	}
}

// WORDS IN A SKILL'S FORM AT THE DOOR ARE THE SKILL, once no routine took them.
func TestASkillInWordsIsPerformedAfterTheRoutines(t *testing.T) {
	d, desk := skillWorld(t, "", discordDesk...)
	asked := false
	prev := proposeRoutine
	proposeRoutine = func(string) (string, string, bool) { asked = true; return "", "", false }
	t.Cleanup(func() { proposeRoutine = prev })
	var out Outcome
	_ = stdoutOf(t, func() {
		out, _ = runInvocation(d, invoke.Request{Text: "focus discord", Source: invoke.SourceSpoken})
	})
	if out != OutcomePerformed || len(desk.raised) != 1 || asked {
		t.Errorf("ended %q, raised %v, resolver asked %v", out, desk.raised, asked)
	}
}

// A SKILL DIRECTOR'S RESOLVER NAMED IS PROPOSED, NEVER PERFORMED, and is not repeated.
func TestASkillDirectorNamedIsProposedNotRun(t *testing.T) {
	d, desk := skillWorld(t, "", discordDesk...)
	prev := proposeRoutine
	proposeRoutine = func(string) (string, string, bool) {
		return `{"answer":{"kind":"run","id":"builtin/focus-app","slug":"focus-app","name":"Focus app",` +
			`"builtin":true,"line":"focus Discord"}}`, "Focus app", true
	}
	t.Cleanup(func() { proposeRoutine = prev })
	var out Outcome
	said := stdoutOf(t, func() {
		out, _ = runInvocation(d, invoke.Request{Text: "switch me back to discord", Source: invoke.SourceSpoken})
	})
	if out != OutcomeClarify || len(desk.raised) != 0 || !strings.Contains(said, outcome.ProposedPrefix) ||
		!strings.Contains(said, "built-in skill") {
		t.Errorf("ended %q, raised %v:\n%s", out, desk.raised, said)
	}
	said = stdoutOf(t, func() {
		out, _ = runInvocation(d, invoke.Request{Text: "switch me back to discord five times"})
	})
	if out != OutcomeRefused || strings.Contains(said, outcome.ProposedPrefix) || len(desk.raised) != 0 {
		t.Errorf("a repeated skill ended %q:\n%s", out, said)
	}
}

// modelAnswers stands in for the model asked which open program was meant, and records what it was
// offered; configured false is a machine with no model set up.
func modelAnswers(t *testing.T, configured bool, label string) *[]string {
	t.Helper()
	var offered []string
	prevA, prevC := askProgram, modelConfigured
	askProgram = func(_ context.Context, _ string, ps []resolver.ProgramChoice) (string, bool) {
		offered = offered[:0]
		for _, p := range ps {
			offered = append(offered, p.Label)
		}
		return label, label != ""
	}
	modelConfigured = func() bool { return configured }
	t.Cleanup(func() { askProgram, modelConfigured = prevA, prevC })
	return &offered
}

// A NAME NO OPEN PROGRAM IS CALLED IS ASKED OF THE MODEL, which chooses among the open programs,
// and Marco raises the one it chose. A name that IS a program's is not asked about.
func TestAProgramNotNamedExactlyIsAskedOfTheModel(t *testing.T) {
	ws := []skills.Window{{Handle: 21, Image: "overlay", Title: "marco overlay"},
		{Handle: 22, Image: "steamwebhelper", Title: "Steam"},
		{Handle: 23, Image: "chatgpt", Title: "ChatGPT"}}
	d, desk := skillWorld(t, "", ws...)
	offered := modelAnswers(t, true, "chatgpt")
	focus := func(app string) (Outcome, string, error) {
		var out Outcome
		var err error
		said := stdoutOf(t, func() {
			out, err = runInvocation(d, invoke.Request{Skill: skills.Focus,
				Named: map[string]string{"app": app}, Source: invoke.SourceTyped})
		})
		return out, said, err
	}
	out, said, err := focus("chat cheap tea")
	if out != OutcomePerformed || err != nil || len(desk.raised) != 1 || desk.raised[0] != 23 {
		t.Fatalf("ended %q %v, raised %v\n%s", out, err, desk.raised, said)
	}
	if !strings.Contains(said, "brought ChatGPT forward") || !strings.Contains(said, "took \"chat cheap tea\"") {
		t.Errorf("did not say what the model took it for:\n%s", said)
	}
	if strings.Join(*offered, ",") != "steam,chatgpt" {
		t.Errorf("the model was offered %q", *offered)
	}

	// A NAME THAT IS A PROGRAM'S IS NOT ASKED ABOUT.
	*offered = []string{"not asked"}
	if out, _, _ := focus("ChatGPT"); out != OutcomePerformed || (*offered)[0] != "not asked" {
		t.Errorf("an exact name ended %q and the model was offered %q", out, *offered)
	}

	// THE MODEL NAMING NOTHING, OR NO MODEL, IS SAID, AND NOTHING COMES FORWARD.
	modelAnswers(t, true, "")
	if out, _, err := focus("spotify"); out != OutcomeFailed || err == nil ||
		!strings.Contains(err.Error(), "didn't recognise") {
		t.Errorf("no answer: %q %v", out, err)
	}
	asked := modelAnswers(t, false, "chatgpt")
	if out, _, err := focus("chat cheap tea"); out != OutcomeFailed || err == nil ||
		!strings.Contains(err.Error(), "no model is set up") || len(*asked) != 0 {
		t.Errorf("no model: %q %v, offered %q", out, err, *asked)
	}
	if len(desk.raised) != 2 {
		t.Errorf("raised %v", desk.raised)
	}
}

// WHAT BECAME OF A SKILL IS WRITTEN DOWN for "why didn't that work?", with what was said scrubbed.
func TestASkillsAttemptIsWrittenDown(t *testing.T) {
	d, _ := skillWorld(t, "")
	modelAnswers(t, true, "")
	_ = stdoutOf(t, func() {
		_, _ = runInvocation(d, invoke.Request{Skill: skills.Focus,
			Named: map[string]string{"app": "zorblax"}, Source: invoke.SourceTyped})
	})
	a, ok, err := lastattempt.Read(attemptAt())
	if !ok || err != nil || a.Routine != "Focus app" || a.Outcome != string(OutcomeFailed) || a.Because == "" {
		t.Fatalf("written down: %+v %v %v", a, ok, err)
	}
	if strings.Contains(a.Because, "zorblax") {
		t.Errorf("what was said was kept: %q", a.Because)
	}
}
