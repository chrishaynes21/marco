# plugins/vision — semantic UI-element resolver

A Marco **bridge host** (see `spec/Hosts.md`) that finds UI controls with a learned
detector instead of matching recorded pixels. Where the engine's image/colour/edge
anchors are *geometric* and `Text` is *OCR*, `Vision` is *semantic*: it answers "where is
the **button** / **icon** / **menu item** / **prompt**", including controls with no clean
edges (an icon on a gradient) that the pure-Go `DetectButtons` can't segment.

It's a separate Go module so its heavy dependency (an ONNX Runtime detector) never reaches
the zero-dep engine. It reuses the engine's cross-platform screen capture via a local
`replace`.

## Pipeline

```
screenshot (internal/screen)
  └─ letterbox + normalise        (yolo.go, pure Go)
      └─ ONNX detector            (backend_onnx.go, -tags onnxvision, cgo)
          └─ decode + NMS         (yolo.go, pure Go)
              └─ labelled boxes → Detect / Locate (detect.go)
```

OCR of detected crops is **not** done here — compose with the `Text` host (`Vision's
Detect` → for each box, `Text's Read`) so each resolver stays single-purpose.

## Protocol

```
→ {"act":"Vision","action":"Detect","input":{"X1":..,"Y1":..,"X2":..,"Y2":..}}
← {"status":"ok","data":{"Elements":[{"Label":"button","X":..,"Y":..,"W":..,"H":..,"Score":..}]}}

→ {"act":"Vision","action":"Locate","input":{"Label":"button","X":..,"Y":..}}
← {"status":"ok","data":{"X":640,"Y":480}}     // centre of the best match (a Point)
← {"status":"failed"}                          // nothing matched (route falls back)
← {"status":"failed","error":"...no model..."} // detector unavailable
```

`Locate` picks the highest-scoring element of the requested `Label` (case-insensitive
substring; omit to match any), preferring the one nearest an optional `X,Y` hint.

## Build

```sh
# Default: dependency-free NULL detector — builds & runs everywhere, declines gracefully.
go -C plugins/vision build -o vision.exe .

# Real detector: ONNX Runtime backend (cgo, like the voice plugin — needs gcc/mingw). The
# binding loads onnxruntime.dll dynamically, so no onnxruntime headers/source are needed.
go -C plugins/vision get github.com/yalue/onnxruntime_go
CGO_ENABLED=1 go -C plugins/vision build -tags onnxvision -o vision.exe .
```

`setup.ps1 -Vision` builds the null detector; add `-OnnxRuntime <onnxruntime.dll>
-VisionModel <model.onnx>` for the real one (it pins the env below into `overlay.cmd`).

## Run / env

Wire it with `--host Vision=bridge:vision`, or set `MARCO_VISION=<path to vision.exe>` (the
spawned `marco do` launches it lazily, like `MARCO_OCR`).

| env | meaning | default |
|---|---|---|
| `MARCO_VISION_MODEL` | path to the `.onnx` detector | — (required for real detector) |
| `MARCO_ONNXRUNTIME` | path to `onnxruntime` shared lib | binding default (`onnxruntime.dll`) |
| `MARCO_VISION_INPUT` / `MARCO_VISION_OUTPUT` | model tensor names | `images` / `output0` |
| `MARCO_VISION_LABELS` | comma-separated class names, in model order | `labels.txt` beside the model, else `button,icon,menu item,prompt` |
| `MARCO_VISION_SIZE` | model input side (px) | `640` |
| `MARCO_VISION_CONF` | confidence threshold | `0.25` |
| `MARCO_VISION_IOU` | NMS IoU threshold | `0.45` |

## Debug / model spike

Before wiring a model into teaching, point the spike tool at real screenshots to see whether
it detects *your* UIs:

```sh
marco vision detect screenshot.png            # or: vision.exe detect screenshot.png
# → a table of {label, score, box} per element + screenshot-detected.png with coloured boxes
```

It runs the detector over a FILE (no live capture), prints what it found, and writes an
annotated copy (one colour per class, with a legend). The default build reports "no model";
build with `-tags onnxvision` and set `$MARCO_VISION_MODEL`/`$MARCO_ONNXRUNTIME` for real output.

## The model

A YOLOv8-style detection model exported to ONNX (output `[1, 4+nc, n]`). Microsoft's
[OmniParser](https://github.com/microsoft/OmniParser) v2 is the reference UI detector;
verify its license and that it detects *your* UIs before relying on it — a model trained on
web/desktop UIs may need fine-tuning for stylized game menus (the same domain gap the OCR
resolver hits). The pure-Go pre/post-processing here is model-agnostic and unit-tested
(`yolo_test.go`), so swapping models is a config change, not code.
