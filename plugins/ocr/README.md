# ocr — the Marco text resolver (a `Text` host)

The third anchor resolver. Marco's engine resolves an anchor two ways on its own —
**image** template match and **colour** at the click point — both *gates* that
confirm a static screen, then click the **recorded coordinate**. This plugin adds a
third, a **locator**: it OCRs the screen, finds the requested word, and reports
**where it is now** — for a target that *moved* (a reflowing UI: Discord's mute
button, a web button). The route then clicks there.

It's a **bridge host** (see `spec/Hosts.md`): a subprocess speaking newline-delimited
JSON on stdio, exactly like `marco-macros` (the `OS` host).

```
→ {"act":"Text","action":"Find","input":{"Text":"Mute","Timeout":3000}}
← {"status":"ok","data":{"X":640,"Y":480}}      # centre of the matched word
← {"status":"failed"}                            # not on screen → route falls back
← {"status":"failed","error":"tesseract: ..."}  # OCR engine unavailable
```

It lives in its **own Go module** so its OCR dependency never reaches the zero-dep
engine. It reuses the engine's cross-platform screen capture (`internal/screen`) via a
local `replace`, so it gets Windows capture today and macOS/Linux as those backends
land — the engine stays dependency-free.

## OCR backend

Cross-platform by one code path: it shells out to the **`tesseract` CLI**, which runs
on Windows, macOS and Linux. The backend sits behind an `ocrEngine` interface, so a
native no-install backend (Windows.Media.Ocr, macOS Vision) can drop in per-OS later.

- **Windows:** `setup.ps1 -OCR` installs tesseract for you (via winget, or a
  `-TesseractUrl <installer.exe>` silent install) and pins `MARCO_TESSERACT`. To do it
  by hand: `winget install UB-Mannheim.TesseractOCR`.
- **macOS / Linux:** `brew install tesseract` / `apt install tesseract-ocr`.
- If `tesseract` isn't on `PATH`, point `$MARCO_TESSERACT` at the binary.

## Build & wire

```sh
go -C plugins/ocr build -o ocr.exe .
```

Routes execute via `marco do`, which launches the resolver **lazily** when
`$MARCO_OCR` names the binary — nothing spawns until a text anchor actually runs:

```sh
export MARCO_OCR=$PWD/plugins/ocr/ocr.exe      # Windows: set "MARCO_OCR=...\ocr.exe"
```

`setup.ps1 -OCR` builds it and adds that line to `overlay.cmd` for you. With no
`MARCO_OCR` wired, a route's text anchor degrades gracefully: `do Text's Find` falls
through to the OS host, which declines, and the click uses its recorded coordinate.

## Teaching a text anchor

Narrate it — position the cursor over the target (the recorded-coordinate fallback)
and say **"click the text Mute"** (or "click the word …"). codegen emits a `Text's
Find` that clicks where the word is, falling back to the cursor point.
