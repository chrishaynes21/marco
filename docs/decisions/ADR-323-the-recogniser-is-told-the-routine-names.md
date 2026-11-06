---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — THE RECOGNISER IS TOLD THE ROUTINE NAMES. voice.exe runs a second Vosk recogniser on the same audio, restricted to a phrase list the widget writes (routine names, the widget's own words, the wake phrase, the stop phrases, the \"why\" questions) plus [unk]. A Final is the two merged by timing: wherever the list recognised one of its phrases WHOLE, its words replace the open recogniser's; everywhere else (values, free requests) the open transcript stands. A lone listed word is not a correction, and a listed word with no open word under it is not added. The list is kept current while voice listens, and plugins\studio\build.cmd now builds voice.exe with cgo."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/voice/phrases.go
  - plugins/voice/voice_cgo.go
  - plugins/overlay/voicephrases.go
  - plugins/overlay/voice.go
  - internal/director/intent/clarify.go
  - internal/lastattempt/lastattempt.go
  - plugins/studio/build.cmd
---

# The recogniser is told the routine names

Builds on [[ADR-318-the-widget-starts-its-own-microphone]] and [[ADR-322-resolve-before-reading-the-screen]].

## Why

The microphone is Vosk with its small 2020 US-English model, open vocabulary: it knows nothing about
the routines somebody has. Live, "cloud pull stone" was heard "cloud paul stone". ADR-322 recovers
that through the model; this stops most of it happening, before anything is resolved.

## Decision

- **Two recognisers, one stream** (`voice_cgo.go`, `decoder`). The open one drives the live partials
  and decides where a phrase ends; the second is `NewRecognizerGrm` over the list plus `[unk]`. Both
  are created together and fed every buffer, so their word times share one clock (measured: times
  run on across `SetGrm` and `Reset`), and the list changes with `SetGrm` between phrases.
- **The merge** (`phrases.go`): `whole` keeps a listed word only inside one of the list's phrases,
  consecutively, longest first; `merge` replaces an open word with the listed words covering it (each
  overlapping by more than half of the shorter). Measured on the real model: without `whole`,
  "black pearl" came back "black pull".
- **The grammar holds only phrases whose every word the model knows** (`FindWord`).
- **The widget writes the list** (`voicephrases.go`) before starting voice.exe, passes it as
  `$MARCO_VOICE_PHRASES`, and rewrites it every 15s when it differs (one `marco routes`). Names only;
  nothing anybody said. The stop phrases and "why" phrasings come from the one place each is defined
  (`intent.ControlPhrases`, `lastattempt.Phrasings`).
- **`build.cmd` builds voice.exe** with cgo against `plugins\voice\libvosk.dll`, refuses without gcc
  or the library, and refuses a stamp without `CGO_ENABLED=1`, as `pack.ps1` does.

## Verified (Windows only)

Real model, speech synthesised by Windows (System.Speech), no microphone (`go test -tags vosk`):

| said | open recogniser alone | with the list |
|---|---|---|
| cloud pull stone | cloud poll stone | cloud pull stone |
| cloud pull metal | cloud pull medal | cloud pull medal |
| open social | opensocial | open social |
| open the settings menu please | (same) | (same) |
| computer cloud pull black pearl | (same) | (same) |

Mutants killed: the list not written before the start, its path not passed, it not kept current;
`whole` keeping every word; `merge` never correcting; the decoder ignoring the list; skipping `whole`
in the decoder; `build.cmd` without the voice build or without its cgo check.

## Not established

- **Tried live once, not measured.** After a rebuild with `build.cmd` (2026-11-06), the owner, with a
  real microphone and voice: "much better at hearing cloud pull now". That is a report, not a rate;
  how often a name is heard right first time has not been counted.
- **Values are not corrected**: routines declare no fixed values, so "metal" heard as "medal" stays
  (ADR-322's review is where it is fixed). A routine name with a word the model does not know is not
  in the list.
- A listed phrase can still be forced onto speech that only resembles it; the review shows it and
  nothing runs without Enter.
- The installer's voice.exe comes from `pack.ps1`, which builds the same source; not re-packed.

## Enforced by

- `plugins/voice` `TestThePhraseListCorrectsOnlyWhatItRecognised`, `TestOnlyAWholePhraseCorrectsWhatWasHeard`, `TestTheGrammarHoldsOnlyWordsTheModelKnows`, `TestThePhraseListIsReReadWhenItChanges`; `-tags vosk` `TestTheRealModelCorrectsARoutineNameAndKeepsTheRest`.
- `plugins/overlay` `TestTheRecogniserIsToldTheRoutineNames`.
- `internal/director/intent` `TestEveryOfferedControlPhraseStops`; `internal/lastattempt` `TestEveryOfferedPhrasingIsAnswered`.
- `plugins/studio` `TestTheBuildScriptMakesTheWholeStackFromOneCommit`, `TestTheBuildScriptRefusesAMicrophoneWithoutCgo`.
