package main

import "strings"

// THE HUD'S VOCABULARY, IN ONE PLACE.
//
// # Why this file exists
//
// The words the HUD accepts were written down three times, in three files, by three
// different hands, and they had already drifted:
//
//   - controller_windows.go's actSubmit switch listed the words that open a panel;
//   - acts.go's dispatch and intake.go's overlayVerb listed the words the overlay acts
//     on itself, or hands to a `marco` subcommand;
//   - view.go's marcoVerbs listed the words drawn in the accent colour, so a command
//     reads differently from a play name as you type it.
//
// The third had never heard of `ui`, `press`, `watch` or `voice`. Typing `ui` therefore
// looked like the name of a play right up until it opened the control centre — the surface
// whose whole job is to tell you what is about to happen was quietly telling you something
// else. Nothing failed to compile, because agreement between three switch statements is not
// a thing a compiler can check.
//
// It also made an obvious product question unanswerable. "What does this thing accept?" had
// no answer that could be computed; it could only be re-derived by reading three files, and
// the listing that tried (the old helpLines) had gone stale and lost its last caller.
//
// So: ONE table. Every site reads it, the listing is generated from it, and adding a word
// is one edit in one place.
//
// Two tests hold that, and they hold different halves of it. Adding a word here and not
// honouring it at a site must fail TestTheHudVocabularyHasOneDefinition, which drives all
// three consumers with every spelling. Giving a site a private list again — which is the
// direction the drift actually came from, and is invisible to a test that iterates this
// table — must fail TestNoConsumerDecidesForItselfWhatACommandWordIs.
//
// # What this table is NOT
//
// It is not a dispatcher. The three sites still do genuinely different things — one opens a
// panel on the window's own goroutine, one spawns a child process, one draws text — and
// pretending otherwise would produce a switch with three unrelated arms per word. What is
// single-sourced is the VOCABULARY: which words exist, which spellings are aliases of which,
// which site owns each, and what each one is for. That is the part that drifted.

// The canonical spelling of each word. Used as a constant everywhere a site tests for one,
// so an alias is resolved exactly once (canonicalWord) and never again.
const (
	cmdHelp        = "help"
	cmdConfig      = "config"
	cmdHere        = "here"
	cmdPlays       = "plays"
	cmdDiagnostics = "diagnostics"
	cmdPerception  = "perception"

	cmdLearn   = "learn"
	cmdNarrate = "narrate"
	cmdUI      = "ui"
	cmdEdit    = "edit"
	cmdVoice   = "voice"
	cmdExit    = "exit"

	cmdBind     = "bind"
	cmdUnbind   = "unbind"
	cmdPress    = "press"
	cmdForget   = "forget"
	cmdSimplify = "simplify"
	cmdRename   = "rename"

	cmdStop = "stop"
)

// cmdSite is which of the three consumers owns a word at run time.
type cmdSite int

const (
	// sitePanel: the overlay window itself. The word opens or changes a panel and never
	// leaves this process. Resolved by panelCommand.
	sitePanel cmdSite = iota
	// siteOverlay: acts.go's dispatch. The overlay acting on ITSELF (learn, voice, exit) or
	// spawning a `marco` subcommand about the catalogue (bind, forget, rename …). These are
	// instructions about Marco, not requests to do something on the desktop, which is why
	// they must never reach the intake.
	siteOverlay
	// siteStop: recognised by intent.IsControlPhrase, the ONE definition shared with the
	// engine's intake and the Director's phrase routing. Listed here so the HUD can SAY the
	// word exists — a person who cannot find out how to stop it has no way to stop it — and
	// deliberately never matched here. A second list of stop-words is the defect
	// TestControlWordsUseTheOneDefinition exists to prevent.
	siteStop
)

// panelKind is which panel a sitePanel word opens.
type panelKind int

const (
	panelNone panelKind = iota
	// panelHelpBrowser opens the control centre's Help screen. It is the FULL manual, and
	// deliberately not the same thing as panelPlays: one is a reference you read, the other
	// is a glance you take without leaving the game.
	panelHelpBrowser
	panelConfig
	// panelHere is what Marco sees, believes, is learning and needs, right now.
	panelHere
	// panelPlays is the in-HUD listing of what Marco can do here.
	panelPlays
	panelDiagnostics
	panelPerception
)

// hudCommand is one word the HUD accepts.
type hudCommand struct {
	// Word is the canonical spelling — the one the product advertises and the one every
	// site compares against.
	Word string
	// Aliases are spellings that still answer. They exist for muscle memory (`teach` for
	// `learn`, `watch` for `here`) and for the words somebody reaches for first
	// (`settings` for `config`). They are deliberately undocumented: the listing shows the
	// canonical word only, so there is one thing to learn and one thing to teach.
	Aliases []string
	// Arg is how the argument reads in the listing, or "" for a word that takes none.
	Arg string
	// What is the one plain sentence the listing shows. No backstage words: a person
	// reading this line should not have to know that Director, Theater or a route exist.
	What string
	// Site is which consumer owns it at run time.
	Site cmdSite
	// Panel is the panel it opens, for sitePanel words.
	Panel panelKind
	// Listed says the word appears in the in-HUD listing. An unlisted word still works;
	// it is an alias-grade spelling that is not worth a line of a person's attention.
	Listed bool
	// Hint says the word appears on the ALWAYS-VISIBLE line, the one drawn while nothing
	// is happening. That line fits four words and a person reads it while they are busy
	// doing something else, so it carries the four a new person needs first — how to give
	// Marco a play, how to see what it makes of the screen, how to find out what it can do,
	// and how to stop it. It is a SUBSET of Listed by construction and
	// TestTheIdleHintOnlyOffersWordsThatWork checks that nothing on it is a word that
	// would not answer.
	Hint bool
}

// hudCommands is the table. The order is the order the listing shows them, which is roughly
// the order a new person meets them: show it something, see what it sees, stop it.
var hudCommands = []hudCommand{
	{Word: cmdLearn, Aliases: []string{"teach"}, Arg: "<name>", Site: siteOverlay,
		Listed: true, Hint: true,
		What: "record a new play — demonstrate it, then the leader saves it"},
	{Word: cmdNarrate, Arg: "learn <name>", Site: siteOverlay, Listed: true,
		What: "learn a play by describing it step by step"},
	{Word: cmdHere, Aliases: []string{"watch", "director", "insight"}, Site: sitePanel,
		Panel: panelHere, Listed: true, Hint: true,
		What: "what Marco sees now, and whether it is waiting on you"},
	{Word: cmdPlays, Aliases: []string{"commands"}, Site: sitePanel, Panel: panelPlays,
		Listed: true, Hint: true,
		What: "this listing — everything you can ask for from here"},
	{Word: cmdStop, Site: siteStop, Listed: true, Hint: true,
		What: "stop whatever is running, wherever it is running"},
	{Word: cmdUI, Arg: "[view]", Site: siteOverlay, Listed: true,
		What: "open the control centre in your browser"},
	{Word: cmdEdit, Arg: "<play>", Site: siteOverlay, Listed: true,
		What: "open one play in the control centre"},
	{Word: cmdBind, Arg: "<key> <play>", Site: siteOverlay, Listed: true,
		What: "give a play a leader shortcut in this app"},
	{Word: cmdUnbind, Arg: "<key>", Site: siteOverlay, Listed: true,
		What: "take a leader shortcut back"},
	{Word: cmdRename, Arg: "<old> to <new>", Site: siteOverlay, Listed: true,
		What: "rename a play"},
	{Word: cmdForget, Arg: "<play>", Site: siteOverlay, Listed: true,
		What: "delete a play"},
	{Word: cmdSimplify, Arg: "<play>", Site: siteOverlay, Listed: true,
		What: "re-clean a play's steps"},
	{Word: cmdPress, Arg: "<key>", Site: siteOverlay, Listed: true,
		What: "press a key or a chord once"},
	{Word: cmdVoice, Aliases: []string{"mute", "unmute", "listen"}, Arg: "on|off",
		Site: siteOverlay, Listed: true,
		What: "listen for spoken commands, or stop listening"},
	{Word: cmdConfig, Aliases: []string{"cfg", "settings"}, Site: sitePanel, Panel: panelConfig,
		Listed: true,
		What:   "leader key, voice, size and appearance"},
	{Word: cmdHelp, Aliases: []string{"h", "?", "manual"}, Site: sitePanel, Panel: panelHelpBrowser,
		Listed: true,
		What:   "the full manual, in your browser"},
	{Word: cmdExit, Aliases: []string{"quit"}, Site: siteOverlay, Listed: true,
		What: "close the overlay"},

	// Unlisted. Both are real, both are for somebody who already knows what they are
	// asking for, and neither is worth a line in a listing a person reads while an
	// application they were using is still on screen behind it.
	{Word: cmdDiagnostics, Aliases: []string{"diagnose", "inspector", "inspect"},
		Site: sitePanel, Panel: panelDiagnostics,
		What: "the evidence underneath what Here said — this one captures the mouse"},
	{Word: cmdPerception, Aliases: []string{"explain"}, Site: sitePanel, Panel: panelPerception,
		What: "a frozen, element-by-element sample of what is on screen"},
}

// byWord indexes every spelling — canonical and alias — onto its entry.
//
// Built once, and it PANICS on a duplicate. A word that appeared twice would resolve to
// whichever entry the table happened to list first, which is the silent-drift failure this
// file exists to remove, reintroduced inside the fix.
var byWord = func() map[string]*hudCommand {
	m := map[string]*hudCommand{}
	for i := range hudCommands {
		c := &hudCommands[i]
		for _, w := range append([]string{c.Word}, c.Aliases...) {
			if _, dup := m[w]; dup {
				panic("overlay: the word " + w + " is defined twice in hudCommands")
			}
			m[w] = c
		}
	}
	return m
}()

// lookupCommand resolves one typed word — canonical or alias — to its entry.
//
// The input is lower-cased and trimmed because the HUD's command line only produces
// lowercase, but a phrase can also arrive spoken, and a spoken "Config." must be the same
// word as a typed one.
func lookupCommand(word string) (*hudCommand, bool) {
	c, ok := byWord[strings.ToLower(strings.TrimSpace(word))]
	return c, ok
}

// canonicalWord maps any accepted spelling onto the canonical one, so every site tests for
// exactly one string and an alias is resolved once rather than at every comparison.
func canonicalWord(word string) (string, bool) {
	if c, ok := lookupCommand(word); ok {
		return c.Word, true
	}
	return "", false
}

// isCommandWord reports whether a word is one of the HUD's own, for the accent colour in the
// command line. This is the reason a person can tell, while typing, that `ui` is about to do
// something to Marco rather than name a play.
func isCommandWord(word string) bool {
	_, ok := lookupCommand(word)
	return ok
}

// panelCommand resolves a whole submitted line to the panel it opens.
//
// It lives here rather than inside the controller's switch so that it can be tested at all:
// the controller is Windows-only and its switch runs on a goroutine fed by a low-level
// keyboard hook. The property worth testing — that the word a person types opens the panel
// they meant — has nothing to do with either.
//
// Only a bare word opens a panel. `config` opens the settings panel; `config the printer` is
// somebody asking for something to happen, and goes where every other phrase goes.
func panelCommand(line string) (panelKind, bool) {
	c, ok := lookupCommand(line)
	if !ok || c.Site != sitePanel {
		return panelNone, false
	}
	return c.Panel, true
}

// commandListing renders the in-HUD answer to "what can I do".
//
// # Why this is generated rather than written
//
// Because the hand-written version went stale and then went dead. It spelled the words out
// as prose, so nothing could check it against anything, and it lost its last caller without
// a single test noticing — three render branches in view.go were unreachable for as long as
// anybody can tell. Generating it from the table means the listing cannot describe a word
// that does not exist, and a word that exists cannot be missing from the listing.
//
// Deleting the generation must fail TestTheListingIsGeneratedFromTheTable.
func commandListing() []string {
	lines := []string{"`m then type:"}
	for _, c := range hudCommands {
		if !c.Listed {
			continue
		}
		lines = append(lines, "  "+padTo(c.Word+" "+c.Arg, 22)+c.What)
	}
	lines = append(lines, "`<key>  run the play bound to it   Esc  cancel", "")
	return append(lines, playGroups(activeApp(), listRoutesFull())...)
}

// playGroups is the scope-grouped listing of runnable plays, split out from commandListing
// so a test can drive it with a fixed set instead of whatever this machine happens to have
// learned.
//
// # Why the plays are grouped by where they apply
//
// The three groups mirror where a play APPLIES, which is the thing people like most about
// this product and the thing a flat list of names cannot convey:
//
//	CONTEXT  only in the app you are in, in place.
//	FOCUS    belongs to an app, and asking for it from anywhere BRINGS THAT APP FORWARD.
//	GLOBAL   anywhere, no app at all.
//
// The group headings say what each one MEANS rather than naming the scope, because the
// scope word is a thing to learn and "switches to the app" is not. Other apps' context
// plays are omitted: they cannot run from here, and a listing of things that will not work
// is worse than a shorter listing.
//
// Deleting the grouping must fail TestTheListingGroupsPlaysByWhereTheyApply.
func playGroups(app string, rows []routeInfo) []string {
	var context, focus, global []routeInfo
	for _, r := range rows {
		switch r.Scope {
		case "global":
			global = append(global, r)
		case "focus":
			focus = append(focus, r) // any app's focus play is reachable from here
		case "context":
			if strings.EqualFold(r.App, app) {
				context = append(context, r)
			}
		}
	}
	if len(context)+len(focus)+len(global) == 0 {
		return []string{"no plays yet — `m learn <name>"}
	}
	var lines []string
	add := func(title string, rs []routeInfo, tagApp bool) {
		lines = append(lines, title)
		if len(rs) == 0 {
			lines = append(lines, "  (none)")
			return
		}
		for i, r := range rs {
			if i >= 6 {
				lines = append(lines, "  …")
				break
			}
			label := r.Name
			if tagApp && !strings.EqualFold(r.App, app) {
				label += " (" + r.App + ")"
			}
			lines = append(lines, "  "+label)
		}
	}
	if app != "" {
		add("only in "+app+":", context, false)
	}
	add("switches to the app:", focus, true)
	add("anywhere:", global, false)
	return lines
}

// padTo right-pads s to at least n columns, so the listing's descriptions line up.
func padTo(s string, n int) string {
	s = strings.TrimRight(s, " ")
	if len(s) >= n {
		return s + "  "
	}
	return s + strings.Repeat(" ", n-len(s))
}

// overlayCommand resolves a submitted line's FIRST word to an entry the overlay performs
// itself — acts.go's dispatch and intake.go's overlayVerb, which are one site wearing two
// names: both are the overlay acting on Marco rather than on the desktop.
//
// It answers about the first word only, because these words take arguments: `learn my
// play`, `bind e open settings`, `rename a to b`. Whether the REST of the line is a
// well-formed use of the word is the caller's business and stays where it was — a play may
// legitimately be called "press the big red button", and overlayVerb's arity rules are what
// keep that phrase out of the local path. See TestOnlyTheOverlaysOwnVerbsStayLocal.
func overlayCommand(fields []string) (*hudCommand, bool) {
	if len(fields) == 0 {
		return nil, false
	}
	c, ok := lookupCommand(fields[0])
	if !ok || c.Site != siteOverlay {
		return nil, false
	}
	return c, true
}

// idleHint is the always-visible line's offer of what to type.
//
// # Why it is generated
//
// The hand-written version said `watch · ui · help · config` and had been saying it for
// long enough that two of those four were the wrong advice: it advertised the visual editor
// and the settings panel to somebody who had not yet taught Marco anything, and it never
// mentioned LEARN or STOP — the first thing a person does with this product and the thing
// they need most when it is doing something they did not expect. It also said `watch` for
// the panel the rest of the product calls HERE.
//
// Generating it from the same table the command line resolves against means the line
// cannot offer a word that does not answer. Deleting the generation must fail
// TestTheIdleHintOnlyOffersWordsThatWork.
func idleHint() string {
	var parts []string
	for _, c := range hudCommands {
		if c.Hint {
			// The word only, never its argument: the line is about 49 characters wide at
			// the default size and four words with their arguments do not fit on it. What
			// each one takes is one keystroke away, in the listing `plays` opens.
			parts = append(parts, "`m "+c.Word)
		}
	}
	return strings.Join(parts, "  ·  ")
}

// isWord reports whether a typed word is one of the given canonical words, resolving an
// alias first.
//
// It is the ONE comparison every site makes. `teach` is `learn` here, so it is `learn`
// everywhere, and a new alias is one line in the table rather than an edit at every site
// that ever compared against a spelling.
func isWord(typed string, want ...string) bool {
	c, ok := lookupCommand(typed)
	if !ok {
		return false
	}
	for _, w := range want {
		if c.Word == w {
			return true
		}
	}
	return false
}
