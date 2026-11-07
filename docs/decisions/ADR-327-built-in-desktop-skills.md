---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — MARCO HAS BUILT-IN DESKTOP SKILLS, AND THE FIRST TWO ARE FOCUS APP AND SEARCH THE WEB. \"focus Discord\" brings the most recently active window of a program that is already open to the front, says so when none is open, asks which when the name matches more than one program, and never launches anything. \"search the web for …\" opens the configured provider's results (overlay.json `search`: a named provider or an https address with {query}; google by default) for the words as entered, in Windows' default browser, and opens nothing that is not a web address. A saved routine's name always wins over a skill's words. The explicit forms are read locally (`marco line` reads them back as \"Focus app · app: Discord\", Enter runs `marco do --skill=focus-app --arg=app=…`); a paraphrase is Director's resolver's, which is offered the skills beside the routines only when the caller asks (`skills`), and an answer naming one carries `builtin` and the skill's explicit line, which the widget puts on its command line for review. Which window, and which address, are decided by Marco and never by a model. Skills are not repeated."
supersedes: []
affects:
  - interface
source_paths:
  - internal/skills
  - cmd/marco/skill.go
  - cmd/marco/line.go
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - cmd/marco/propose.go
  - cmd/marco/resolve.go
  - cmd/director/resolveroutewiring.go
  - internal/resolveroute/resolveroute.go
  - internal/director/service/protocol.go
  - internal/widgetconfig/settings.go
  - plugins/overlay/skill.go
  - plugins/overlay/line.go
  - plugins/overlay/resolve.go
  - plugins/overlay/recall.go
---

# Built-in desktop skills

Builds on [[ADR-301-the-command-line-reads-back-what-it-will-run]], [[ADR-322-resolve-before-reading-the-screen]], [[ADR-325-repeat-a-routine-a-fixed-number-of-times]] and [[ADR-326-the-command-line-recalls-commands]].

## Why

The owner asked for two things that should work from the widget by voice or by typing, with
nothing demonstrated first: "Focus Discord" (bring an open window forward, the most recently
active of several; say so when it isn't open; launching is a separate command) and "Search the
web for Ark black pearl locations" (the default browser, a configured provider, the query as
entered; browser and provider are separate settings). Saved routine names keep priority, the
explicit forms work locally, and the LLM may map a paraphrase ("switch me back to Discord") to the
same operation, while Marco resolves the actual window or builds the URL. An ambiguous app name is
asked about.

## Decision

- **One package, no desktop** (`internal/skills`, stdlib only): the skills, their explicit forms
  (`focus <app>`, `focus app <app>`, `switch to <app>`, `search the web for <query>`,
  `web search <query>`, and each name alone, which asks for the value), the window choice over a
  list the caller supplies, and the search address.
- **Which window.** Front to back, the first window of the program the name matches: the
  executable is the name, else it begins with it, else the program's titles end with it (as
  applications write their own, "… - Visual Studio Code"). A title merely mentioning the name is
  not the program. Two programs at one step are a question. Marco's own windows are never chosen.
  Raised through `winctx.Raise`, which reports whether it really came forward. Not open is said
  and nothing is launched.
- **Which address.** A named provider or an https/http template with `{query}`; the words are
  query-encoded and otherwise untouched. Anything else, including a template whose host would be
  the words, is refused before the shell sees it. The read-back names the provider, because the
  words leave the machine.
- **Where they are read.** On the command line only when no routine took the line (`readLine`).
  At the intake, after the grammar and only when no routine reads the words in any form
  (`skillInWords`); performed, as words naming a routine with its value are. In the widget a skill
  always takes a value, so it runs only from its read-back on screen, typed or spoken.
- **A paraphrase.** `marco resolve` and the intake's proposal ask Director with `skills: true`, and
  Director appends the skills as candidates (`builtin/<slug>`, everywhere, one argument). An
  answer naming one is marked `builtin` with the skill's explicit line and the value proposed;
  the intake proposes it and runs nothing, and the widget opens the command line on that line, or
  says why not when the line reads as anything but that skill. Studio does not ask, because its
  Play button runs routines.
- **Recall.** A skill is kept by its name ("Focus app"), like a routine.
- **Logs.** The skill's name only, never the app or the query; `[ran] ` carries what ran, for the
  history.

## Not established

- Not tried live: no window has been raised and no browser opened by this code.
- A program minimised to the tray has no window to raise and is reported as not open.
- The control centre's Config view has the `search` field; the widget's own settings panel does
  not.

## Enforced by

- `internal/skills` `TestTheExplicitFormsAreRead`, `TestALineWrittenForASkillReadsAsIt`, `TestFocusTakesTheMostRecentlyActiveWindow`, `TestFocusAsksWhenTheNameIsAmbiguousAndSaysWhenNothingIsOpen`, `TestASearchOpensOnlyAWebAddress`.
- `cmd/marco` `TestALineInASkillsFormReadsAsTheSkill`, `TestASavedRoutineWinsOverASkill`, `TestASkillRunsByItsIdentity`, `TestAnAmbiguousAppIsAskedAbout`, `TestASkillInWordsIsPerformedAfterTheRoutines`, `TestASkillDirectorNamedIsProposedNotRun`.
- `internal/resolveroute` `TestASkillTheResolverNamesIsAnsweredAsTheSkillsLine`; `cmd/director` `TestASkillIsOfferedOnlyToACallerThatAsked`.
- `plugins/overlay` `TestASkillRunsOnlyFromItsReadBack`, `TestASkillDirectorNamedIsReviewedBeforeItRuns`.
