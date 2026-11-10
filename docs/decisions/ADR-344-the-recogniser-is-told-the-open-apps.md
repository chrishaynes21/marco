---
type: adr
status: accepted
date: 2026-11-09
decision: "ACCEPTED — THE RECOGNISER IS TOLD THE OPEN APPS. The phrase list the widget writes for voice.exe (ADR-323) now also holds the words that begin or name a built-in skill and the everyday asks for an application (focus, focus on, switch to, search the web, go back to, ...), and each open program's name: its executable, and the name its window gives it where the title marks that name as the application's own (after \" - \", \" — \", \" – \" or \" | \"), or where the whole title IS the program (\"Destiny 2\" for destiny2). The rest of a title — a page, a document, a person — is never listed. Marco's own programs are left out, from one list (skills.Ours) that focus also uses. The names are re-read with the rest of the list every 15s, so a program opened while voice listens reaches it. The point is to get the name right at the recogniser, so the model is asked which program was meant less often."
supersedes: []
affects:
  - interface
source_paths:
  - internal/skills/skills.go
  - plugins/overlay/voicephrases.go
---

# The recogniser is told the open apps

Follows [[ADR-323-the-recogniser-is-told-the-routine-names]] and [[ADR-327-built-in-desktop-skills]].
The owner asked for it to reduce how often the model is asked: before this, a misheard program name
in "focus …" reached the model's question about which open program was meant (`programByModel`).

## Why only an application's own name

A title with no separator is usually the program ("Steam") but can be a person in a chat window or
a document. Such a title is listed only when it reduces to the executable's name, so "Alex Rivera"
in `signal.exe` is not listed and `signal` is.

## Limits

- Vosk accepts a phrase only when it knows every word, so an executable like `destiny2` or
  `msedgewebview2` is left out by voice.exe; "Destiny 2" is listed and is probably left out too,
  since the small model is unlikely to have a word "2" (not checked). Those still rely on the open
  recogniser and the model.
- More listed phrases is more room for the list to snap an open word onto one of them; only a whole
  listed phrase corrects (ADR-323), and a word an open program is named after was already likely.
- Up to 15s after a program opens before it is listed.

## Not established

- Not tried with a microphone. Held at the list the widget writes, with a stated desktop.

## Mutation run

Eight mutants (the title guard, the executable name, the whole-title case, Marco left out, the
everyday asks, both widget call sites, cmd/marco's use of the shared list); all killed.

## Enforced by

- `internal/skills`: `TestOnlyAnApplicationsOwnNameIsSpoken`, `TestTheWordsOfASkillAreSpoken`.
- `plugins/overlay`: `TestTheRecogniserIsToldTheRoutineNames`.
