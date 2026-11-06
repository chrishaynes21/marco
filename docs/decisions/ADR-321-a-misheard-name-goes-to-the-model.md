---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — A MISHEARD ROUTINE NAME IS RESOLVED BY THE MODEL, AND THE WIDGET ASKS FOR IT WHEN DIRECTOR FAILS. One live spoken attempt, \"cloud pull stone\" heard as \"cloud paul stone\", reached no model for two reasons: the widget asked Director which saved routine was meant only after `unavailable`, which a RUNNING Director never answers (it answers `failed`), and the Director started from Studio's Open widget had no $MARCO_RESOLVER, so its resolver had no model. The widget now asks after `failed` too, never offering to record a failure. The resolver plugin is found beside the executable (plugins\\llama\\llama.exe) when $MARCO_RESOLVER is unset; an explicit value, including empty, still wins. The value question is told the chosen routine's name and asked for the words naming it first, so a misheard name is not copied into the value; and the plugin's routine questions default to llama3.1:8b while the screen's keep llama3.2:3b (Experiment-072). A deterministic word-by-word matcher was tried for this and removed: recovering what somebody meant is the model's job."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/outcome.go
  - plugins/overlay/intake.go
  - internal/resolver/resolver.go
  - internal/resolver/extract.go
  - internal/dispatch/llm.go
  - internal/resolveroute/resolveroute.go
  - cmd/director/resolveroutewiring.go
  - plugins/llama/llm.go
  - plugins/llama/extract.go
  - plugins/llama/main.go
  - setup.ps1
---

# A misheard name goes to the model

Builds on [[ADR-267-director-owns-route-resolution]] and [[ADR-318-the-widget-starts-its-own-microphone]].

## What happened

Said live into the widget: "cloud pull stone". The recogniser heard **"cloud paul stone"**. The
latest-attempt record ([[ADR-320-why-didnt-that-work]]): spoken, 3 words, Director took it, ended
`failed`. Director's action history: nothing pressed. Nothing was offered.

- **The widget never asked.** Its resolve step was gated on `offersLearn()`: `unavailable` and no
  route. `unavailable` is what a Director that cannot be REACHED produces (`directorSubmit`); a
  running one finishes a phrase it cannot act on `failed`. The question was asked only while
  Director was down. The transcript test for a spoken paraphrase passed because its stand-in
  answered `unavailable`, which a running Director does not do.
- **There was no model to ask.** `$MARCO_RESOLVER` was set nowhere — not for the user, not on the
  machine, not by the launcher Studio uses. `setup.ps1 -Llama` pinned it into `overlay.cmd` only.
  So Director's resolver ran its deterministic arms alone, and those cannot recover a misheard
  name ("cloud paul stone" scores 0.56 against `cloud pull`, under the 0.6 at which it asks).
- A deterministic word-by-word matcher was written for this and REMOVED at the owner's direction:
  remapping words is not how Marco should understand somebody.

## Decision

- **The widget asks after a Director failure too** (`asksWhichRoutine`: no route, the child
  started, `unavailable` or `failed`). Asking is read-only and can only offer. The offer to RECORD
  the phrase stays `offersLearn`: a Director that tried and failed is never answered with "shall I
  learn it?".
- **The model is found where the other optional pieces are.** `resolver.Path()` is
  `$MARCO_RESOLVER` when it is set — empty meaning none — and otherwise
  `plugins\llama\llama.exe` beside this executable, as the widget finds `voice.exe`.
  `dispatch.PluginPath()` (the advisor) falls back to it too: they are one binary.
- **The value question names the routine.** `resolver.ExtractFor` carries the chosen routine's
  name (`extract.routine` on the wire; an older plugin ignores it), and the llama plugin asks for
  the words naming it under `routine_words` first, a key `parseValues` never reads. The value is
  still a span of Marco's own copy of the request.
- **Each kind of question has its default model**, and `$MARCO_LLM_MODEL` overrides both:
  llama3.2:3b for the screen's questions (interpret, which control) — unchanged, because
  Experiment-072 measured llama3.1:8b turning refusals into wrong actions there — and llama3.1:8b
  for the library's (which routine, which words are a value), where everything is reviewed before
  it runs. `setup.ps1 -Llama` pulls both and pins a model only when `-LlamaModel` is given.

## Verified (Windows only)

- **What the live attempt left** (read-only): the record and the action history above; the
  binaries were all `98b53a1`; `$MARCO_RESOLVER` set nowhere; Ollama running with llama3.2:3b,
  llama3.1:8b and qwen2.5:7b.
- **The real models, against the owner's real library, through Director's resolver code**, with
  the plugin built to a temporary path and nothing acted on:
  - before the change to the value question, both llama models picked
    `arkascended/focus/cloud-pull` for "cloud paul stone" and proposed the whole phrase as the
    value; qwen2.5:7b did not answer within 20s;
  - after it, two runs each: llama3.1:8b — "cloud paul stone" → `stone`, "cloud pole metal" →
    `metal`, "cloud pull stone" → `stone`, "pull some stone from the cloud" → `stone`, "cloud paul
    black pearl" → `paul black pearl` (wrong). llama3.2:3b recovered the routine for the misheard
    phrases and proposed no value, and proposed `some stone from the cloud` for the paraphrase.
- Tests: `plugins/overlay` `TestAMisheardSpokenNameIsOfferedWithItsValue`,
  `TestADirectorFailureIsNeverOfferedForRecording`; `internal/resolver`
  `TestTheResolverIsFoundBesideTheExecutable`; `internal/dispatch`
  `TestTheAdvisorIsThePluginBesideTheExecutable`; `internal/resolveroute`
  `TestTheValueQuestionNamesTheRoutine`; `plugins/llama` `TestTheDefaultModelDependsOnTheQuestion`,
  `TestEachQuestionIsAskedWithItsModel`, `TestTheValueQuestionNamesTheRoutine`.
- Mutants, each killed by an assertion: the widget's gate narrowed back to `unavailable`, the
  learn guard after a failure removed, the discovery dropped in `resolver` and in `dispatch`, the
  routine's name not passed, the prompt not naming it, the library's questions asked with the
  screen's model (extract and resolve).

## Not established

- **Not re-tried live.** It needs `plugins\llama\llama.exe` rebuilt (the plugin on disk predates
  this), the stack rebuilt (`plugins\studio\build.cmd`), and the running Director restarted so it
  finds the plugin.
- The installer ships no llama plugin, so an installed Marco still has no model.
- A model's answer is not deterministic. llama3.1:8b got a two-word value after a misheard name
  wrong; llama3.2:3b leaves misheard values empty for the review to fill.
- Every phrase nothing matches exactly now waits for the model (about 0.4s warm, up to ~9s cold).
- `gate.ps1` builds `director.exe` into the tree beside `plugins\llama\llama.exe`, so its desktop
  scenarios will ask the model when Ollama is running.

## Enforced by

- `plugins/overlay` `TestAMisheardSpokenNameIsOfferedWithItsValue`, `TestADirectorFailureIsNeverOfferedForRecording`.
- `internal/resolver` `TestTheResolverIsFoundBesideTheExecutable`.
- `internal/dispatch` `TestTheAdvisorIsThePluginBesideTheExecutable`.
- `internal/resolveroute` `TestTheValueQuestionNamesTheRoutine`.
- `plugins/llama` `TestTheDefaultModelDependsOnTheQuestion`, `TestEachQuestionIsAskedWithItsModel`, `TestTheValueQuestionNamesTheRoutine`.
