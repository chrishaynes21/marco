package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What must stay true of the HUD's vocabulary.
//
// The defect these hold is not a crash and never was. The words the HUD accepts were
// written down three times — the controller's submit switch, acts.go/intake.go's dispatch,
// and view.go's accent map — and three switch statements agreeing is not something a
// compiler can check. They had already drifted: `ui`, `press`, `watch` and `voice` worked
// and were not highlighted, so the surface whose entire job is to tell you what is about to
// happen was quietly telling you they were the names of plays.
//
// So these tests do not check that the table is well formed. They drive the THREE CONSUMERS
// with every spelling in it, which is the only version of the claim worth making.

// noMarco points MARCO_BIN at a path that cannot exist.
//
// NOTHING IN THIS FILE MAY SPAWN marco.exe: on this surface it performs real input. Several
// of the things under test shell out to ask what plays exist or to open a browser, and with
// no binary to start they fail immediately and harmlessly — which is also what a fresh
// machine looks like, so the listing's empty state gets exercised for free.
func noMarco(t *testing.T) {
	t.Helper()
	t.Setenv("MARCO_BIN", filepath.Join(t.TempDir(), "no-such-marco.exe"))
}

// everySpelling is every word the HUD accepts, canonical and alias alike.
func everySpelling() []string {
	var out []string
	for _, c := range hudCommands {
		out = append(out, c.Word)
		out = append(out, c.Aliases...)
	}
	return out
}

// TestTheHudVocabularyHasOneDefinition is the headline: all three consumers answer from the
// table, for every spelling in it.
//
// It fails if a word is added to the table and not honoured by a consumer, and — because
// each consumer is exercised through the function production calls — it fails if a consumer
// is rewired to answer from a list of its own instead.
func TestTheHudVocabularyHasOneDefinition(t *testing.T) {
	for _, w := range everySpelling() {
		c, ok := lookupCommand(w)
		if !ok {
			t.Fatalf("%q is in the table and does not resolve", w)
		}

		// CONSUMER 3 — view.go. Every accepted word reads as a command while it is being
		// typed. This is the drift that shipped: four words worked and none of them were
		// accented, so they looked like play names until they did something else.
		runs := editRuns(w, nil, th.name)
		if len(runs) == 0 || runs[0].col != th.name {
			t.Errorf("%q is accepted but is not drawn as a command word", w)
		}

		switch c.Site {
		case sitePanel:
			// CONSUMER 1 — controller_windows.go, through the resolver it now calls.
			got, isPanel := panelCommand(w)
			if !isPanel || got != c.Panel {
				t.Errorf("%q is a panel word and the controller would not open its panel "+
					"(got %v, want %v)", w, got, c.Panel)
			}
		case siteOverlay:
			// CONSUMER 2 — acts.go's dispatch and intake.go's overlayVerb, which are one
			// site under two names: the overlay acting on Marco rather than the desktop.
			if _, ok := overlayCommand([]string{w, "x", "y"}); !ok {
				t.Errorf("%q is one of the overlay's own words and dispatch would send it "+
					"to the desktop intake", w)
			}
		case siteStop:
			// Deliberately owned by NOBODY here. `stop` is recognised by
			// intent.IsControlPhrase, the one definition shared with the engine's intake
			// and the Director's phrase routing; a second answer in this package is the
			// defect TestControlWordsUseTheOneDefinition exists to prevent. It is in the
			// table so the HUD can SAY the word exists — a person who cannot find out how
			// to stop it has no way to stop it.
			if _, isPanel := panelCommand(w); isPanel {
				t.Errorf("%q opened a panel; the stop word must not be answered here", w)
			}
			if _, ok := overlayCommand([]string{w}); ok {
				t.Errorf("%q was claimed as an overlay verb; stop belongs to the shared "+
					"definition", w)
			}
		}
	}

	// And the other direction: a word that is NOT in the table is a play name to all three.
	// Without this the whole file passes trivially on a table that claimed everything.
	for _, phrase := range []string{"open the settings", "zzz-not-a-command"} {
		if runs := editRuns(phrase, nil, th.name); len(runs) > 0 && runs[0].col == th.name {
			t.Errorf("%q was drawn as a command word", phrase)
		}
		if _, isPanel := panelCommand(phrase); isPanel {
			t.Errorf("%q opened a panel", phrase)
		}
		if _, ok := overlayCommand(strings.Fields(phrase)); ok {
			t.Errorf("%q was claimed by the overlay instead of reaching the intake", phrase)
		}
	}
}

// TestNoConsumerDecidesForItselfWhatACommandWordIs is the half a behavioural test cannot
// reach.
//
// Driving the consumers proves they honour the table. It cannot prove they honour ONLY the
// table: a private list holding a word the table has never heard of is invisible to a test
// that iterates the table, and that is precisely the drift that shipped.
//
// So this reads the consumers' source and looks for the two SHAPES a second vocabulary
// takes, both of which this repository has actually grown:
//
//	switch cmd { case "help", "?", "h":        the controller's submit switch
//	var marcoVerbs = map[string]bool{…}        view.go's accent list
//	fields[0] == "ui"                          acts.go's dispatch chain
//
// It deliberately does NOT flag every quoted word. `marco director stop` is an argv, "listen"
// is also the name of a HUD state, and a test that cannot tell those from a comparison
// against something a person typed would be turned off within a month. What is forbidden is
// deciding, from a literal, whether a TYPED WORD is a command — which is the one question
// commands.go exists to answer.
func TestNoConsumerDecidesForItselfWhatACommandWordIs(t *testing.T) {
	shapes := []*regexp.Regexp{
		regexp.MustCompile(`switch +(cmd|word|typed|fields\[0\]|first) *\{`),
		regexp.MustCompile(`(cmd|word|typed|fields\[0\]|first) *(==|!=) *"[^"]+"`),
		regexp.MustCompile(`map\[string\]bool\{`),
	}
	for _, f := range []string{"controller_windows.go", "view.go", "acts.go", "intake.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			for _, re := range shapes {
				if re.MatchString(line) {
					t.Errorf("%s:%d decides for itself what a command word is:\n\t%s\n"+
						"the spellings live in commands.go; ask isWord, panelCommand or "+
						"overlayCommand", f, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestEveryPanelWordOpensItsPanel drives openPanel itself, because a word can resolve
// perfectly to a panel that nothing opens.
//
// The class of defect this repository keeps rediscovering is complete code that nothing
// invokes: the listing this replaces was correct, was generated from nothing, and had no
// caller at all.
func TestEveryPanelWordOpensItsPanel(t *testing.T) {
	noMarco(t)
	for _, c := range hudCommands {
		if c.Site != sitePanel {
			continue
		}
		h := newModel()
		openPanel(h, c.Panel)
		s := h.snapshot()
		switch c.Panel {
		case panelHelpBrowser:
			// The manual is a BROWSER view, so the evidence is that it tried to open one
			// rather than putting a panel on screen.
			if s.helpOn {
				t.Errorf("%q opened an in-HUD panel; the manual belongs in the browser", c.Word)
			}
			if !strings.Contains(strings.Join(s.logs, "\n"), "control centre") {
				t.Errorf("%q did not reach the control centre: %q", c.Word, s.logs)
			}
		case panelConfig:
			if !s.configOn || !inConfig.Load() {
				t.Errorf("%q did not open the settings editor", c.Word)
			}
			inConfig.Store(false)
		case panelHere:
			if s.wmode != watchOn {
				t.Errorf("%q did not show what Marco sees", c.Word)
			}
		case panelPlays:
			if !s.helpOn || len(s.help) == 0 {
				t.Errorf("%q did not open the listing", c.Word)
			}
		case panelDiagnostics:
			if s.wmode != watchDeep || !s.inspectorOn {
				t.Errorf("%q did not open diagnostics with the mouse captured", c.Word)
			}
		case panelPerception:
			if !s.insightOn {
				t.Errorf("%q did not freeze a perception snapshot", c.Word)
			}
		default:
			t.Errorf("%q names a panel nothing opens", c.Word)
		}
	}
}

// TestTheListingIsGeneratedFromTheTable holds the claim commands.go makes.
//
// The hand-written listing it replaces went stale — it never mentioned `ui`, `press` or
// `voice` — and then went dead. Generating it means the listing cannot describe a word that
// does not exist, and a word that exists cannot be missing from it.
func TestTheListingIsGeneratedFromTheTable(t *testing.T) {
	noMarco(t)
	got := strings.Join(commandListing(), "\n")
	for _, c := range hudCommands {
		listed := strings.Contains(got, c.Word) && strings.Contains(got, c.What)
		switch {
		case c.Listed && !listed:
			t.Errorf("%q is offered by the HUD and is missing from the listing", c.Word)
		case !c.Listed && strings.Contains(got, c.What):
			t.Errorf("%q is unlisted on purpose and its line is in the listing", c.Word)
		}
	}
	// STOP is in the listing even though nothing in this package answers it. A person who
	// cannot find out how to stop it has no way to stop it.
	if !strings.Contains(got, cmdStop) {
		t.Error("the listing does not say how to stop what is running")
	}
	// With no engine to ask, the listing still has to say something useful rather than
	// showing a person an empty space where their plays are not.
	if !strings.Contains(got, "no plays yet") {
		t.Errorf("the empty state does not tell a new person what to do:\n%s", got)
	}
}

// TestTheListingGroupsPlaysByWhereTheyApply holds the second claim commands.go makes.
//
// The three groups are the thing people like most about this product, and a flat list of
// names conveys none of it: whether a play runs here, brings an application forward, or
// works anywhere is the whole question somebody has while looking at the list.
func TestTheListingGroupsPlaysByWhereTheyApply(t *testing.T) {
	rows := []routeInfo{
		{Name: "heal", App: "game", Scope: "context"},
		{Name: "buy potions", App: "shop", Scope: "context"},
		{Name: "open inbox", App: "mail", Scope: "focus"},
		{Name: "take a note", Scope: "global"},
	}
	got := strings.Join(playGroups("game", rows), "\n")

	if !strings.Contains(got, "only in game:") || !strings.Contains(got, "heal") {
		t.Errorf("this app's own play is not shown as such:\n%s", got)
	}
	// Another app's CONTEXT play cannot run from here, and a listing of things that will
	// not work is worse than a shorter listing.
	if strings.Contains(got, "buy potions") {
		t.Errorf("a play that cannot run from here was offered:\n%s", got)
	}
	// A FOCUS play is reachable from anywhere and says which application it will bring
	// forward — that is the behaviour, and the label is the only place it is stated.
	if !strings.Contains(got, "switches to the app:") || !strings.Contains(got, "open inbox (mail)") {
		t.Errorf("a focus play is not shown as one that switches apps:\n%s", got)
	}
	if !strings.Contains(got, "anywhere:") || !strings.Contains(got, "take a note") {
		t.Errorf("a global play is not shown as working anywhere:\n%s", got)
	}
	// The headings say what each group MEANS. The scope word is a thing to learn;
	// "switches to the app" is not.
	for _, backstage := range []string{"context —", "focus —", "global —", "route"} {
		if strings.Contains(got, backstage) {
			t.Errorf("the listing made a person learn the word %q:\n%s", backstage, got)
		}
	}
}

// TestTheIdleHintOnlyOffersWordsThatWork holds the always-visible line.
//
// It offered `watch · ui · help · config` — two of which are the wrong first advice for
// somebody who has taught Marco nothing yet — and mentioned neither LEARN, the first thing a
// person does with this product, nor STOP, the thing they need most when it is doing
// something they did not expect. Generating it from the table is what keeps it from
// offering a word that does not answer.
func TestTheIdleHintOnlyOffersWordsThatWork(t *testing.T) {
	hint := idleHint()
	for _, want := range []string{cmdLearn, cmdStop} {
		if !strings.Contains(hint, want) {
			t.Errorf("the always-visible line does not mention %q: %q", want, hint)
		}
	}
	// Every word on it must be one the HUD answers, and one the listing also shows — the
	// hint is a subset of the listing, never a fourth vocabulary.
	for _, part := range strings.Split(hint, "·") {
		w := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "`m "))
		c, ok := lookupCommand(w)
		if !ok {
			t.Errorf("the hint offers %q, which the HUD does not accept", w)
			continue
		}
		if !c.Hint || !c.Listed {
			t.Errorf("the hint offers %q, which the table does not mark for it", w)
		}
	}
	// The panel is about 49 characters wide at the default size. A hint that wraps to two
	// rows on a resting HUD is the reason a person stops reading the line.
	if len([]rune(hint)) > 49 {
		t.Errorf("the hint is %d characters and will wrap: %q", len([]rune(hint)), hint)
	}
}

// TestTheWordsAgreeWithTheProductsOwnVocabulary holds the rename.
//
// The HUD called the "what Marco sees" panel `watch` while the control centre called the
// same belief HERE. One belief with two names is a thing a person has to learn twice, so it
// is HERE in both — and `watch` still answers, undocumented, exactly as `teach` still
// answers for `learn`. An alias nothing exercises is an alias that disappears in the next
// tidy-up with nothing to say so.
func TestTheWordsAgreeWithTheProductsOwnVocabulary(t *testing.T) {
	noMarco(t)
	c, ok := lookupCommand(cmdHere)
	if !ok || c.Panel != panelHere {
		t.Fatal("`here` does not open what Marco sees")
	}
	for _, alias := range []string{"watch", "director", "insight"} {
		if got, ok := lookupCommand(alias); !ok || got.Word != cmdHere {
			t.Errorf("%q no longer reaches what Marco sees; muscle memory broke", alias)
		}
	}
	// The listing shows the canonical word only: one thing to learn, one thing to teach.
	if strings.Contains(strings.Join(commandListing(), "\n"), "watch") {
		t.Error("the listing advertises the old word as well as the new one")
	}
}
