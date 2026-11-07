---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — MARCO HAS BUILT-IN DESKTOP SKILLS, AND THE FIRST TWO ARE FOCUS APP AND SEARCH THE WEB. \"focus Discord\" brings the most recently active window of a program that is already open to the front, says so when none is open, asks which when the name matches more than one program, and never launches anything. A name no open program is called is asked of the configured model, which chooses among the open programs only (amended the same day). \"search the web for …\" opens the configured provider's results (overlay.json `search`: a named provider or an https address with {query}; google by default) for the words as entered, in Windows' default browser, and opens nothing that is not a web address. A saved routine's name always wins over a skill's words. The explicit forms are read locally (`marco line` reads them back as \"Focus app · app: Discord\", Enter runs `marco do --skill=focus-app --arg=app=…`); a paraphrase is Director's resolver's, which is offered the skills beside the routines only when the caller asks (`skills`), and an answer naming one carries `builtin` and the skill's explicit line, which the widget puts on its command line for review. Which address is decided by Marco and never by a model; a model may choose which open program, among the ones Marco offers. Skills are not repeated."
supersedes: []
affects:
  - interface
source_paths:
  - internal/skills
  - internal/resolver/program.go
  - plugins/llama/program.go
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
  Raised through `winctx.Raise`, which reports whether it really came forward. Nothing is
  launched.
- **A name no open program is called** ("chat cheap tea", "my browser", "vs code") is asked of
  the configured model (amended 2026-11-07, after "switch to chatgpt" did not work for the owner
  and a person cannot be expected to know an executable's name). It is offered the open programs,
  each once, labelled from the application's own name in its window title and never the rest of
  a title (`skills.Programs`), asked which one was meant (the plugin's `program` question,
  `resolver.WhichProgram`), and its answer is accepted only as one of the labels it was sent:
  it chooses among windows that exist and Marco raises the one chosen, saying what it took the
  words to mean. An exact name is not asked about, because a model cannot improve on the program
  being called that and it would cost a second and a running model server. No model, or no
  answer, is said beside "isn't open". Tuned on a real desktop's list with llama3.1:8b: the
  first wording refused "vs code" and took the nonsense "zorblax" for Trackmania; the shipped one
  took "chat cheap tea", "the chat bot", "my browser", "music", "visual studio" and "dis cord" to
  the right program and refused "zorblax", "photoshop" and "word", and still refuses "vs code".
- **Everyday words** ("open X", "go to X", "go back to X", "switch me back to X", "switch me to X",
  "bring up X", "show me X") are Focus app ONLY when X, reduced the same way, is an open
  program's executable or the application's name in its window title (`skills.Opens`,
  `NamesOpenProgram`); otherwise the line goes on as before. Added the same day, after the owner
  said "open chat gp t" and Director's resolver chose the routine "enter freeplay": with only
  labels to go on, llama3.1:8b sent "switch me back to discord" to `discord-mute` and "open
  chatgpt" to `search-the-web`, and descriptive extra labels fixed one case and broke another. These
  words also name routines ("open social", "go to free play"), so they are claimed only for a
  program that is open, which leaves a misheard routine name to Director as it was.
  And when no open program is called X exactly (the recogniser heard "open chad gpt"), the
  intake asks the model which open program was meant, telling it the person's routine names as
  NOT applications, and PROPOSES "focus <that program>" for review rather than raising it; the
  model naming nothing leaves the words for Director (`proposeOpenProgram`). Measured with
  llama3.1:8b on the owner's desktop: "chad gpt", "chat cheap", "this cord" and "my browser" to
  the right program, and "sosh al", "so shell", "free plea", "cloud pole" and "zorblax" to none;
  without the routine names "sosh al" went to Discord. "spot if i" is not found.
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
  history. A skill run by its identity writes the last attempt, scrubbed, for "why didn't that
  work?".

## Not established

- Not tried live: no window has been raised and no browser opened by this code.
- A program minimised to the tray has no window to raise and is reported as not open.
- The control centre's Config view has the `search` field; the widget's own settings panel does
  not.

## Enforced by

- `internal/skills` `TestTheExplicitFormsAreRead`, `TestALineWrittenForASkillReadsAsIt`, `TestFocusTakesTheMostRecentlyActiveWindow`, `TestFocusAsksWhenTheNameIsAmbiguousAndSaysWhenNothingIsOpen`, `TestASearchOpensOnlyAWebAddress`, `TestTheOpenProgramsAreNamedAsTheirWindowsNameThem`, `TestAnOpenIsFocusOnlyForAProgramThatIsOpen`.
- `cmd/marco` `TestALineInASkillsFormReadsAsTheSkill`, `TestASavedRoutineWinsOverASkill`, `TestASkillRunsByItsIdentity`, `TestAnAmbiguousAppIsAskedAbout`, `TestASkillInWordsIsPerformedAfterTheRoutines`, `TestASkillDirectorNamedIsProposedNotRun`, `TestAProgramNotNamedExactlyIsAskedOfTheModel`, `TestASkillsAttemptIsWrittenDown`, `TestEverydayWordsAreFocusOnlyForAnOpenProgram`, `TestAMisheardOpenIsProposedAsFocus`, `TestAMisheardOpenLeavesARoutinesWordsAlone`.
- `internal/resolveroute` `TestASkillTheResolverNamesIsAnsweredAsTheSkillsLine`; `cmd/director` `TestASkillIsOfferedOnlyToACallerThatAsked`.
- `plugins/overlay` `TestASkillRunsOnlyFromItsReadBack`, `TestASkillDirectorNamedIsReviewedBeforeItRuns`.
- `internal/resolver` `TestOnlyAnOfferedProgramIsAnswered`; `plugins/llama` `TestTheProgramAnswerIsOneThatIsOpen`, `TestTheProgramQuestionIsAnswered`.
