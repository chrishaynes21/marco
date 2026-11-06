---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — THE INSTALLER SHIPS VOICE, ONE RECOGNISER LISTENS PER MICROPHONE, AND NOTHING HEARD REACHES A LOG. pack.ps1 builds voice.exe with cgo from the commit it packs and stages it with libvosk, its three MinGW runtimes and the speech model, under plugins\\voice beside marco.exe, which is where the widget looks; packing without them, or with a voice.exe built without cgo, is refused. The launcher unpacks them (the model as a tree) and Studio verifies each file against the manifest. The widget does not start a recogniser when a voice.exe from the same path is already running, which is what an overlay.cmd written by an older setup.ps1 leaves, and says so. And the widget's log, the child output it echoes, and voice.exe's own log carry the length of what was said and never the words, nor any value supplied with them."
supersedes: []
affects:
  - interface
  - packaging
source_paths:
  - pack.ps1
  - plugins/marco-app/main.go
  - plugins/studio/readiness.go
  - plugins/overlay/voice.go
  - plugins/overlay/quiet.go
  - plugins/overlay/acts.go
  - plugins/overlay/intake.go
  - plugins/overlay/resolve.go
  - plugins/voice/main.go
  - cmd/marco/intake.go
---

# The installer ships voice

Follows [[ADR-318-the-widget-starts-its-own-microphone]], which left three things open: the
installed bundle had no recogniser, an old launcher would start a second one, and the widget's
logs still carried what was said.

## Decision

- **The bundle carries the recogniser, built from the commit it packs.**
  - `pack.ps1` builds `plugins/voice` with `CGO_ENABLED=1` against `plugins\voice\libvosk.dll`
    and refuses a `voice.exe` whose `go version -m` does not say `CGO_ENABLED=1`. A cgo-free
    build compiles and answers every start with "built without cgo".
  - It stages `libvosk.dll` and `libgcc_s_seh-1.dll`, `libstdc++-6.dll`, `libwinpthread-1.dll`
    (libvosk's own imports) beside it, and the model directory as `plugins\voice\model`.
  - Those native pieces are not in the repository. `setup.ps1 -Voice` downloads them; packing
    without them, or without gcc, throws and says how to get them. A smaller bundle without voice
    is not offered.
  - The bundle grows by about 127 MB: libvosk and libstdc++ are 26 MB each, the small English
    model 68 MB.
- **They land where the widget looks.** The launcher's `staged` table extracts them under
  `bin\plugins\voice\`; `stagedTrees` unpacks the model as a directory, and an empty or missing
  model is reported by name like a missing file. `voice.exe` is a Go binary with a revision in
  the manifest, and every model file is hashed. Studio verifies each and names the model once
  ("the speech model (14 files)").
- **One recogniser per microphone.** Before starting one, the widget asks the process table for a
  `voice.exe` whose image is the path it would start. An old `overlay.cmd` pipes that same
  `voice.exe` into `marco serve`, and starts it before serve has compiled the program that starts
  the widget, so it is already running when the widget asks. The widget then starts none, listens
  through that one (its lines reach overlay.marco through serve's stdin, as they always did), and
  says: *voice is listening through a recogniser that was already running (voice.exe, process N),
  so the widget did not start a second on the same microphone — if an older overlay.cmd started
  it, run setup.ps1 again*. The settings line reads `on · another recogniser`. A `voice.exe` at
  another path is not taken for this one.
- **Nothing heard reaches a log** (`plugins/overlay/quiet.go`).
  - The widget's own log lines carry `heard(phrase)`, the word count, for every phrase, name and
    reply they used to quote; a child's arguments are shown by `argsShown`, which keeps the
    subcommand and the flags that say what runs and replaces `--arg` values and bare words.
  - **The error path:** a child's output is echoed to the console through `redactionFor(args)`,
    which takes out the request and each value given with it, as whole words. That is where
    `marco do` and Director quote the request back ("heard: …", a refusal naming the value). A
    narrate-learn session redacts each phrase as it is forwarded. The announced `[route]` is a
    library name and is echoed as it is.
  - A run that was named by identity (`--play=`) is logged under the routine's name; one that
    announced no route is logged by its length.
  - `voice.exe` logs `words=N` where it logged the phrase. The widget already kept only its last
    error line; the console of anything that pipes it no longer gets the transcript either.
  - The engine's `intake: finished` line logs the word count, not the words.
  - The HUD still SHOWS what was heard and what ran. It is the person's own screen, not a log.
- **A model libvosk cannot read is named.** libvosk prints `ERROR (VoskAPI:…) … does not contain
  model files` and then crashes rather than returning the error, so the widget keeps that line as
  well as `voice:` lines, and says the speech model could not be loaded.

## Verified (Windows only)

- **The voice block of `pack.ps1`, run alone** into a scratch assets directory (the full pack would
  overwrite the owner's `dist\Marco.exe`): it built `voice.exe` with `CGO_ENABLED=1` and staged the
  four libraries and the 14 model files, and it restored `CGO_ENABLED` afterwards. The PowerShell
  parser reports no errors in `pack.ps1`.
- **The staged `voice.exe` loads libvosk from beside itself:** started with a model path that does
  not exist, libvosk's own error came back. No microphone was opened.
- **The layout:** `cmd/marco` `TestThePackagedLayoutIsWhatMarcoLooksFor` checks the voice files'
  destinations, that the widget's lookup is the one the layout serves, the model tree, and that
  `pack.ps1` stages the model and refuses a cgo-free build.
- **The guard, against the real process table:** a copy of the test binary named `voice.exe` is
  started and held, and the widget starts no second one and names that process.
- **The logs:** a spoken line with a unique value goes through the intake, a child's echo quoting
  it, the run's ending, a resolve and a learn that cannot start, and neither the phrase nor the
  value is in the log. `voice.exe --demo` built without cgo at `MARCO_LOG=debug` logs `words=2` and
  not the phrase.

## Not established

- **A packed bundle has not been built or installed.** `pack.ps1` was not run end to end, and the
  `-tags packaging` installation tests have not run against a bundle with voice in it; `gate.ps1`
  runs both.
- **No real microphone has been used**, from any launcher.
- The guard sees a recogniser started BEFORE the widget. One started after the widget is running
  (somebody runs an old overlay.cmd beside an open widget) is a second widget as well, and is not
  detected.
- The dev tree's own `plugins\voice\voice.exe` is from an older commit and still logs phrases on
  its stderr; the widget discards that stream except for its error line. `plugins/studio/build.cmd`
  does not build voice.
- `pack.ps1`, like `setup.ps1`, holds non-ASCII characters without a BOM, against the CLAUDE.md
  rule. That predates this change; the lines added are ASCII.

## Enforced by

- `cmd/marco` `TestThePackagedLayoutIsWhatMarcoLooksFor`.
- `plugins/studio` `TestAnInstalledMarcoSaysWhatItIsMadeOf`.
- `plugins/overlay` `TestTheWidgetDoesNotStartASecondRecogniser`, `TestNothingHeardReachesTheLog`,
  `TestTheRecogniserLogsNoTranscript`, `TestTheLogKeepsWhatRanAndLosesWhatWasSaid`,
  `TestTheLogSaysHowACommandEnded`, `TestVoiceSaysWhyItCannotListen`.
