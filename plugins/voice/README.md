# voice — the offline voice layer (Vosk)

Captures the microphone, runs **offline** speech recognition with
[Vosk](https://alphacephei.com/vosk/) (cross-platform, no cloud), and emits the
transcript as Marco feed events on stdout — the same event seam `globals` /
`ahk-hotkeys` use:

```
{"feed":"Voice","event":"Partial","data":"open ch"}    # live, as you speak
{"feed":"Voice","event":"Final","data":"open chrome"}  # a finished phrase
```

**The widget starts it.** `overlay.exe` looks for `plugins\voice\voice.exe` beside
`marco.exe` (model in `plugins\voice\model`; `$MARCO_VOICE_BIN` / `$MARCO_VOSK_MODEL`
override, `$MARCO_NO_VOICE=1` starts none), gives it the wake phrase from overlay.json, and
forwards these lines onto its own bridge output. So every way of starting the widget has voice
— `overlay.cmd`, Studio's Open widget, `Marco.exe --overlay` — and the HUD says whether it is
listening or why not. Don't also pipe it into `marco serve`: if one is already running from the
same path the widget starts none and says so, but two would run every command twice. Its log
carries `words=N`, never the phrase. See ADR-318 and ADR-319.

`overlay.marco` turns **Partial** into a live preview (`Overlay`'s `Heard`) and
**Final** into a command (`Overlay`'s `RunVoice`, the same intake a typed line uses).

For a pipe test with no widget, it can still feed a served program's stdin:

```
voice --demo | marco serve --host Overlay=bridge:overlay plugins/overlay/overlay.marco
```

## Wake word (two-phase activation)

It waits for the assistant word **marco** before doing anything:

1. Say **"marco"** → the HUD shows **listening...** (armed for ~8 s).
2. Say your command → it runs (e.g. *"login to facebook"*).

Saying it in one breath — *"marco, login to facebook"* — also works. A phrase
heard without the wake word shows in the preview but isn't run. Change the word
with `--wake <word>` / `$MARCO_VOICE_WAKE` (installer: `setup.cmd -Voice -Wake
"computer"`); `--wake ""` (or `off`) listens to every phrase, no activation.

## It is told your routine names

The widget writes a phrase list — your routine names, its own words, the wake word, stop and the
"why didn't that work" questions — and passes it as `$MARCO_VOICE_PHRASES`. A second recogniser
listens restricted to that list, and wherever it heard one of those phrases whole, its spelling
replaces the open recogniser's ("cloud poll stone" becomes "cloud pull stone"); values and anything
else keep what the open recogniser heard. The list is re-read between phrases when it changes. Its
log carries how many entries, never the names. See ADR-323.

It is told the programs that are open too, so *"focus discord"* or *"switch to steam"* is heard as
said: the words of a focus (focus, focus on, switch to, go back to, ...) and each open program's
name, as its executable and as its window calls itself, kept current as programs open and close.
Only an application's own name is taken from a window title, never a page, a document or a person.
A name Vosk has no word for (`destiny2`) is left out, as a play name is. See ADR-344.

Build it with `plugins\studio\build.cmd` (cgo, against `libvosk.dll` here). The real-model test:

```
set CGO_ENABLED=1& set CGO_CFLAGS=-I.& set CGO_LDFLAGS=-L. -lvosk& go test -tags vosk .
```

## How it knows you're done

No key/word to end — it's silence-based. Speak your phrase and **pause**; Vosk
detects end-of-speech and finalizes. If Vosk doesn't endpoint, a fallback
finalizes a phrase that's stayed unchanged for ~1.2 s, so a pause always
completes it. (A phrase heard without the wake word shows in the preview but
isn't run.)

## Optional — needs cgo + a native lib + a model

Unlike the other layers, the real recognizer is **not** pure Go:

- **cgo + a C toolchain** (Windows: mingw-w64 `gcc`; Linux: `gcc`; macOS: clang).
- **libvosk** on the link/run path — download the prebuilt library for your OS
  from the [Vosk releases](https://github.com/alphacep/vosk-api/releases) and put
  `libvosk.dll` (Windows) / `.so` / `.dylib` next to `voice.exe` (or on PATH /
  `LD_LIBRARY_PATH`), with its header for the build.
- **a model** — e.g. `vosk-model-small-en-us-0.15` (~50 MB); unzip it and point
  `--model` (or `$MARCO_VOSK_MODEL`) at the folder.

Build it with cgo:

```
go -C plugins/voice build -tags '' -o voice.exe .   # CGO_ENABLED=1 (the default with a C compiler)
```

The alpha **installer sets all of this up for you** when you opt into voice.

## Demo mode (no cgo, no model)

So the pipe is testable without any of the above:

```
go -C plugins/voice build -o voice.exe .   # builds even with CGO_ENABLED=0
voice --demo | marco serve --host Overlay=bridge:overlay plugins/overlay/overlay.marco
```

`--demo` emits a canned `Partial…Final "say hello"`, which flows through exactly
like real speech. Without cgo, the real recognizer returns a clear "rebuild with
cgo + libvosk" error.
