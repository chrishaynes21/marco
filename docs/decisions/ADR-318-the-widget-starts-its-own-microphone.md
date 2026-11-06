---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — THE WIDGET STARTS ITS OWN MICROPHONE, AND A SPOKEN LINE GETS THE TYPED LINE'S REVIEW. Voice reached the widget only through overlay.cmd, which piped voice.exe into marco serve; Studio's Open widget and the installed Marco.exe --overlay start the same serve with nothing in front, so their widget had a voice toggle and a wake phrase and heard nothing, and said nothing about it. The overlay now starts plugins\\voice\\voice.exe beside marco.exe itself, with the wake phrase from overlay.json, and forwards that process's own Voice lines unchanged onto the bridge's stdout, so they reach overlay.marco's `when Voice reads Final` and come back as RunVoice through the one intake. The pipe is gone from overlay.cmd and from setup.ps1, which writes it. The widget says whether voice is listening, muted, starting or unavailable and why. And a spoken line naming a routine that takes values opens the command line on its read-back, as a typed one does, instead of going to the intake as words."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/voice.go
  - plugins/overlay/main.go
  - plugins/overlay/acts.go
  - plugins/overlay/config.go
  - overlay.cmd
  - setup.ps1
---

# The widget starts its own microphone

## What was wrong

- **Only one launcher had voice.**
  - `overlay.cmd` ran `voice.exe --model … | marco serve …`, and read the wake phrase with
    `marco wake`.
  - Studio's **Open widget** (`internal/widgetstack` `Args`) and the installer's `runOverlay`
    run `marco serve` with nothing on its input.
  - Those widgets had the voice toggle, the wake setting and the `Voice` feed, and nothing ever
    spoke into them. Silence looked the same as a quiet room.
- **A spoken line skipped the review.** `acts.go` sent only a TYPED line to `typedLine`, so
  "cloud pull stone" said aloud went to the intake as words, while typed it opened the read-back
  first.

## Decision

- **The overlay starts the recogniser.**
  - It finds `plugins\voice\voice.exe` beside `marco.exe`, with its model in
    `plugins\voice\model` (where `setup.ps1 -Voice` builds it).
  - `$MARCO_VOICE_BIN` and `$MARCO_VOSK_MODEL` override. `$MARCO_NO_VOICE=1` is
    `setup.ps1 -NoVoice`.
  - The wake phrase is `$MARCO_VOICE_WAKE`, else overlay.json's `wake`, else `marco`.
  - Its Partial and Final lines are forwarded unchanged; heartbeats are dropped. No phrase is read
    in the overlay, and there is no voice parser.
- **One recogniser per microphone.** The pipe is gone from `overlay.cmd` and from the launcher
  `setup.ps1` writes. Two would run every spoken command twice.
- **It says its state.** `voiceSays` is one sentence: listening for "‹wake›", muted (and how to
  unmute), starting, or unavailable with the reason. The reasons: left out by `-NoVoice`, not
  installed, no model, built without the recogniser, no microphone, `libvosk.dll` missing. It is
  logged when it changes, it answers the `voice on|off` toggle, and the settings panel's voice
  line reads `on · listening`, `on · unavailable` or `off`.
  - "Listening" means the recogniser stayed up past its load (4s). Every failure it has ends the
    process during load.
  - voice.exe's stderr is kept in memory only for its last `voice: <error>` line. Its log lines
    quote what was said, so they are never kept.
- **A spoken line gets the typed line's review.** A spoken phrase goes through `typedLine` too.
  It never has a read-back on screen yet, so a line naming a routine that takes values always
  opens the command line on what was heard, with the read-back under it, and Enter runs it. A new
  spoken request drops a pending Director proposal rather than being refused as an edit of it.
  Control phrases still never wait.

## Verified (Windows only)

- **With a transcript fixture** (`plugins/overlay/testdata/voice-transcript.jsonl`, voice.exe's
  own output after wake gating, replayed by a stand-in process through the real start, pipe and
  forwarder):
  - the partials and finals are forwarded in order, heartbeats and other lines are not;
  - the child is given the model and the wake phrase from overlay.json;
  - it is said to be listening while it runs, and why it stopped afterwards, with no speech in
    the sentence.
- **The three spoken requests, from the fixture's finals, through `dispatch` RunVoice:**
  - an exact name (`open social`) goes to the intake as `do --source=spoken open social`;
  - a paraphrase (`bring up the social menu`) goes to the intake, and when nothing takes it, the
    routine Director names is offered;
  - a value said (`cloud pull stone`) opens the review and runs nothing; Enter runs
    `--play=cloud-pull --expect=v1 --arg=resource=stone`;
  - a paraphrase Director proposes a value for (`pull some stone from the cloud`) opens the
    review on `cloud pull resource:stone`.
- **The engine leg:** the real `overlay.marco` under the real driver turns Voice events pushed BY
  THE OVERLAY BRIDGE, with nothing on stdin, into `Heard` and `RunVoice`.
- **Mutants**, 11, all killed by an assertion: never started from main, heartbeats forwarded, wake
  not passed, a log line kept as the reason, never listening, the reason dropped, `-NoVoice`
  ignored, muted not said, the spoken review skipped, overlay.marco's Final unhandled, and the
  old piping overlay.cmd.

## Not established

- **No real microphone has been used.** Nothing here was launched on the desktop: not
  overlay.cmd, not Open widget, not a real voice.exe. E2E 34 and 34a are the live checks.
- **The installed bundle ships no recogniser.** `Marco.exe --overlay` now looks for one and says
  it is not installed. Shipping voice.exe, libvosk and a model is a packaging decision of its own.
  → Shipped by [[ADR-319-the-installer-ships-voice]].
- A reviewed spoken line is confirmed by Enter, and runs as `--source=typed`. Confirming the
  read-back by voice ("yes") is not built.
- An overlay.cmd generated by an older setup.ps1, and not regenerated, still pipes voice.exe, so
  with this overlay it would run two recognisers. The one in this tree is updated.
  → The widget now starts none when one is already running: [[ADR-319-the-installer-ships-voice]].
- `setup.ps1` holds non-ASCII characters without a BOM, against the CLAUDE.md rule. That is not
  this change's doing, and the edit kept the file's encoding as it was.

## Enforced by

- `plugins/overlay` `TestTheWidgetStartsItsOwnMicrophone`, `TestVoiceSaysWhyItCannotListen`,
  `TestVoiceSaysWhenItIsMuted`, `TestASpokenLineWithAValueIsReviewedLikeATypedOne`,
  `TestEveryLauncherGetsVoiceFromTheWidget`.
- `internal/driver` `TestTheWidgetsOwnVoiceEventsReachRunVoice`.
