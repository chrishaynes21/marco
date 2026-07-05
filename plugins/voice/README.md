# voice — the offline voice layer (Vosk)

Captures the microphone, runs **offline** speech recognition with
[Vosk](https://alphacephei.com/vosk/) (cross-platform, no cloud), and emits the
transcript as Marco feed events on stdout — the same event seam `globals` /
`ahk-hotkeys` use:

```
{"feed":"Voice","event":"Partial","data":"open ch"}    # live, as you speak
{"feed":"Voice","event":"Final","data":"open chrome"}  # a finished phrase
```

Pipe it into a served program:

```
voice --model vosk-model-small-en-us-0.15 | marco serve --host Overlay=bridge:overlay plugins/overlay/overlay.marco
```

`overlay.marco` turns **Partial** into a live preview (`Overlay`'s `Heard`) and
**Final** into a command (`Overlay`'s `Run`). So speaking a route name runs it.

## Wake word (two-phase activation)

It waits for the assistant word **marco** before doing anything:

1. Say **"marco"** → the HUD shows **listening...** (armed for ~8 s).
2. Say your command → it runs (e.g. *"login to facebook"*).

Saying it in one breath — *"marco, login to facebook"* — also works. A phrase
heard without the wake word shows in the preview but isn't run. Change the word
with `--wake <word>` / `$MARCO_VOICE_WAKE` (installer: `setup.cmd -Voice -Wake
"computer"`); `--wake ""` (or `off`) listens to every phrase, no activation.

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
