# Rocket League live-capture fixtures

Derived evidence from a real Rocket League Free Play session, captured 2026-08-05 on a
1920×1080 borderless window.

**No screenshots are stored here, deliberately.** The frames these came from contain the
player's account name, party state and Spotify activity. What is kept is what perception
actually produced — frame metadata, detector output, OCR output — which carries none of
that and is the only part any test needs. `freeplay.json` and `pause.json` run with no
game, no model, no plugin and no desktop.

## How they were captured

```
model        models/icon_detect.onnx   Ultralytics YOLO11m (OmniParser), 1 class {0:'icon'}
input        images[1,3,640,640] → output0[1,5,8400]
trained on   som_office — office UI, not games
runtime      ONNX Runtime 1.26.0
ocr          tesseract, --psm 3 whole-frame / --psm 7 per-box
capture      GDI BitBlt of the window rect; transform identity, scale 1.0
```

## What they show

`freeplay.json` — car still, ball at the far goal, HUD is a single boost gauge reading 33.

- The detector finds **nothing** at its 0.25 floor.
- Dropping to 0.05 surfaces exactly one box, `(1643,831) 195×202` at **0.13** — which is
  the boost gauge, correctly localised and far below any usable threshold.
- Whole-frame OCR returns **zero** results. The boost digits are drawn as disconnected
  horizontal bars in a thin translucent font over a moving 3D scene; three PSMs, cropped
  and 3× upscaled, all read nothing.

`pause.json` — the pause menu over the same arena.

- The detector finds the four menu buttons at **0.59–0.64**, above the provider's 0.50
  structural bar, at the default threshold.
- Whole-frame OCR returns **one** line ("HOLD FOR PLAYLISTS"), because Tesseract's global
  binarisation is dominated by the bright arena behind the panel.
- OCR scoped to each detected box returns **all four labels exactly**.

That contrast is the finding these fixtures exist to hold: the same frame, the same OCR
engine, 1 line whole-frame versus 4/4 per-box.

## What they do NOT license

Neither fixture supports reading the in-play HUD. A pack that claimed to know the boost
value from this evidence would be inventing it.
