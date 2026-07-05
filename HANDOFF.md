# Handoff — Marco (feat/host-ffi)

State as of this handoff: **all green** (Go packages pass), gofmt + vet clean (modulo
the standard Windows LL-hook `unsafe.Pointer` notes), branch `feat/host-ffi` (unmerged).
Binaries built and current: `marco.exe`, `marco-macros.exe` (repo root),
`plugins/overlay/overlay.exe`, `plugins/ocr/ocr.exe`. The engine has **zero external
deps**; overlay (ebiten), voice (vosk), ocr (tesseract CLI) are separate modules.

**Logging is at production defaults:** `$MARCO_LOG` unset → **info** (notable events +
warnings + errors only; a normal click is silent, a low-confidence anchor is a Warn, a
followed move is an Info). `MARCO_LOG=debug` turns on the full per-click scoring /
coordinate / origin trace. The forced `MARCO_LOG=debug` is no longer written into
`overlay.cmd`. Bridge children inherit serve's stderr (bridgehost), so all logs reach
the console.

Read this, then `README.md` (user-facing), `E2E.md` (manual test walkthrough), and
`spec/` (language). Cross-session notes live in the author's memory; the durable
project facts are also captured here.

## What this session built (on top of the host-FFI port)

1. **Named command arguments.** Declare at teach time with `with`: `teach "say hello
   with name"` → route `say hello`, arg `name`. Place the value during a demo by
   tapping the **arg key F9** (no typing `{{…}}` into the app); run as `say hello
   name:chris`. Multi-word values run to the next `key:`. Positional `with a, b`
   (→ `{{1}}`,`{{2}}`) still works as a fallback.
   - `internal/routes/args.go`: `SplitArgs` (teach decl / positional), `ParseInvocation`
     (run → route + named map + positional), `ApplyArgs` (fill, escaped), `ArgNames`.
   - `internal/simplify`: `Options.ArgKey` (`MARCO_ARG_KEY`, default `f9`) + `ArgNames`;
     the Nth F9 tap → `{{ArgNames[N-1]}}` (named) or `{{N}}`.
   - `internal/codegen`: `Route(name, app, steps, dir, declaredArgs...)`; `splitSecrets`
     keeps numeric + declared-plain `{{name}}`, everything else → `Secret`.
   - `cmd/marco`: `dispatchDo(d, name, named, positional)` + `runRoute`;
     `runAssistantDo`/`runDo` call `ParseInvocation` BEFORE nlu resolution.

2. **Secret named args (passwords, remembered).** A declared arg named
   password/pass/pwd/pin/secret/token/otp/apikey/key (`codegen.isSecretArg`) is NOT
   text-substituted — codegen emits `do OS's Secret with "<routebase>:<argname>"`
   (route-qualified, never in the route source). `oshost.Host.SetArgs(named)` (set in
   `cmd/marco runRoute`) lets `doSecret` use a value passed inline AND remember it
   (`sec.Set`), falling back to the credential store next run. So `login to facebook
   with username, password` → enter once, reuse after.

3. **Voice / narrate teach.** `internal/voiceteach` (pure `Parse` + `Session` over an
   injectable `Env`; `OSEnv` reads cursor/window/screen). `marco teach --narrate
   <name>` reads one phrase per stdin line (typed OR piped from the voice plugin).
   Overlay: say/type **"narrate teach <name>"** → `acts.go` spawns the child and
   forwards every `Run` phrase to it. Vocabulary: click/anchor this/wait for this
   screen/wait N/type/press/activate/undo/done/cancel. Narration also makes **named**
   args (`New(env, argNames...)` + `Session.resolveArg`: "type person" / "type the
   first argument" → `{{person}}`).

4. **Overlay teach prompts, redone.** Prompts render BELOW the command line
   (terminal-style transcript), answered **type-then-Enter** (press y/n/s, see `› y`,
   Enter to send; Esc = no), with a live `● rec M:SS` timer during recording. Scope
   prompt **inverted** → app-only is the happy path (yes/Enter). See
   `overlay-teach-prompt-handshake` memory + `plugins/overlay/{model,view,controller_windows,acts}.go`.

5. **Overlay auto-pop args.** `marco args "<phrase>"` prints a route's arg labels;
   `pollArgHints` (debounced 150ms) → `drawArgHints` shows `name: password: (tab)`
   after the command; **Tab** (`actAcceptHint`) appends the next `name:`.

6. **`forget all` / `delete all`** — confirm, then wipe every route (`cmd/marco
   runForget`). Fixes the old "no route named delete all".

7. **Overlay opacity** — idle floor up (`textIdle` 0.85, config `Idle` default 0.72),
   **focus → fully solid**, config opacity slider **live-previews** while the editor
   is open (`view.go` opacity switch; `config.go`).

8. **README synced**, **`E2E.md`** added (manual test guide, verified scriptable parts).

9. **First-class anchors with a resolver chain.** An anchor is now resolved **a couple
   of ways** as a wait gate, tried each poll round until one succeeds (`oshost.doFind`):
   (1) **Image** template match (primary), then (2) **Colour** — the pixel under the
   recorded click point matches the colour captured at teach time. Colour only ever
   *upgrades* a would-be timeout into a confident resolve; it never overrides a positive
   image match, so it's strictly safe in the wait-gate model (the route clicks the
   recorded coordinate either way). The click pixel is read for free from the same
   armed full-frame capture (`recorder_windows.go` worker → `screen.ColorAt`), carried
   on `RecordedEvent.Color` → `macroir.Step.Color` (`simplify`) → the `Anchor` set's
   `Color`/`X`/`Y` fields (`codegen.emitImageClick`). Backward compatible: anchors
   taught before this carry no colour and run image-only — re-teach to gain the
   fallback. Resolver chain is the extension point for future plugin resolvers
   (OCR text, vision) per the dep boundary. Tests: `oshost` `TestFind{ImageResolves,
   ColorFallback,ColorMismatchFails}`, `codegen` `TestImageClick{ColorResolver,NoColor}`.

10. **Third anchor resolver: text (OCR), as a plugin.** Image and colour are GATES
    (confirm a static screen → click the recorded coordinate); text is a LOCATOR for a
    target that MOVED (reflowing UI) → click where the word is now. OCR needs a dep, so
    it's a **bridge host plugin**, not engine code:
    - `plugins/ocr` (own module): a `Text` act with a `Find` action. Captures via the
      engine's `internal/screen` (local `replace`, so no new engine dep), OCRs with the
      **tesseract CLI** (cross-platform, behind an `ocrEngine` interface for future
      native backends), parses TSV, matches the word/phrase → centre Point. Pure
      parse/match unit-tested (`ocr_test.go`); tesseract not needed to test.
    - Engine wiring: `macroir.Step.AnchorText`; `internal/textmod` embeds `text.marco`
      (the `Text` surface; `use text.` resolves via `driver.builtinModule` like `os`);
      `codegen.emitAnchoredClick` emits a `Text's Find` fallback that clicks `that` (the
      located point), reusing one `Anchor` set for both hosts; `voiceteach` `ClickText`
      ("click the text X"). `text.marco` does NOT redeclare `Point` (uses os.marco's —
      avoids a duplicate-symbol merge error when a route `use os.`+`use text.`).
    - Run wiring: `cmd/marco` `newDeps` adds `Hosts["Text"] = bridgehost.New($MARCO_OCR)`
      (lazy) when set; `setup.ps1 -OCR` builds `ocr.exe`, **installs tesseract** (winget,
      or `-TesseractUrl` silent install; `Install-Tesseract`/`Find-Tesseract`), and writes
      `MARCO_OCR` + a pinned `MARCO_TESSERACT` into `overlay.cmd`. **Graceful absence:**
      with no Text host, `Text's Find` falls through
      to the OS host (the `*` default), which now declines a text-only anchor quietly
      (`oshost.doFind`) → route clicks its recorded coordinate. Needs `tesseract` (or
      `$MARCO_TESSERACT`). Tests: `codegen` `TestTextOnlyClickRoute`/`TestGateAndTextClickRoute`,
      `voiceteach` ClickText cases, `plugins/ocr` parse/match.

11. **Route scopes are now folders: context / focus / global.** Each app owns two
    subfolders beside the top-level `global/`:
    - `routes/global/<slug>.marco` — **global**: app-less, runs anywhere, no switch.
    - `routes/<app>/context/<slug>.marco` — **context**: only while `<app>` is in front,
      no switch (the in-app command).
    - `routes/<app>/focus/<slug>.marco` — **focus**: from anywhere; brings `<app>` to the
      front first (codegen prepends `Activate`).
    `internal/routes`: `Route` gains `Focus bool` (App""=global; App+!Focus=context;
    App+Focus=focus). `Resolve` order: foreground app's context → its focus → any other
    app's focus (activates it) → global. `Save`/`SaveRecording`/`ScopeDir` now take a
    `Route`. **Legacy:** a loose `routes/<app>/<slug>.marco` (pre-split) reads + stays as
    context (`locDir` fallback) — existing routes work untouched; new ones use subfolders.
    Teach: `saveTaught` maps scopeContext/Focus/Global → the folders (only focus passes
    an app to codegen, so only focus activates); `TeachAuto` defaults to context.
    Display: `marco routes [--json]` carries `scope` ("context"|"focus"|"global");
    overlay help groups into **context / focus / global** (acts.go `helpLines`). Tests:
    `routes` `TestFocusResolvesAnywhere`/`TestContextBeatsFocusInApp`/`TestFolderLayout`/
    `TestLegacyLooseContext`. NOTE: old "focus" routes saved before this live in
    `routes/global/` with an `Activate` in the body (e.g. mute-discord) — they still work
    (display as global, behave like focus); re-teach to reclassify them as focus.

12. **Anchors are a SCORED bundle (Phase 1).** `oshost.doFind` no longer chains image→
    colour; it SCORES candidate locations and fuses signals into a CONFIDENCE
    (`scoreAnchor`). Candidates: the recorded point P (resolved window-relative via
    `anchorPoint` — fixes the old stale-absolute colour/position bug) and, when the image
    matches elsewhere, its best-match centre Lb (so a MOVED target is followed). Confidence
    = evidence from **image** (`screen.Match.Score`, now exposed) + **colour** (tolerance-
    aware `colorScore`), normalised over available signals; **position is a selection
    prior, NOT evidence** (P is always dist 0 from itself, so it can't certify
    correctness — it only favours the nearer candidate). Weights `wImage 0.8 / wColor 0.2`,
    `positionFloor 0.7`; threshold `findConfidence()` = `$MARCO_FIND_CONFIDENCE` (default
    0.6). Above threshold → ok at the located point (codegen's confident arm clicks
    `that`); below → failed + an INFO "low confidence … re-teach to repair this anchor"
    line, and the route's `or?` clicks the recorded coordinate (or tries Text/OCR). codegen
    emits the anchor with X,Y (+ RelX,RelY) always; confident arm clicks `that`. The gate
    timeout dropped 20000→1500ms (fail fast; the diff-mask experiment was removed). Tests:
    `oshost` `TestFind{ImageConfident,MovedTarget,ColorCarries,LowConfidence}`. NOT yet:
    edge signal (Phase 2), last-known location (Phase 3), Text fused into the score (still a
    separate `or?` locator — cross-host), and the inline overlay "repair this anchor?"
    prompt (today it's a log hint + re-teach).

13. **Button recognition (all pure Go, `internal/screen`).** Built on one edge primitive
    (`edgeStrength`, `EdgeTolerance`):
    - **Auto-crop to the button** (`AutoCrop`, used by the recorder capture worker): at
      capture it snaps the saved template to the DETECTED button containing the click —
      which also drops adjacent disconnected things (a tooltip is a separate component).
      Falls back to the edge bounding box, then unchanged (dense/flat patch). Capture radius
      bumped 40→64.
    - **Edge-match signal** (`Screen.FindEdge` → `findEdgeDT`): production CV — matches the
      button's OUTLINE via a **distance transform** (`distanceTransform`, 2-pass chamfer), so
      a template edge only has to be NEAR a screen edge, tolerant to anti-aliasing and a small
      shift, and indifferent to fill colour (survives recolour/theme). Scored 0..1 by average
      edge closeness over subsampled template edge points (`edgePointCap` 160) with a cheap
      sample prefilter; `Ambiguous` set when a distinct second outline scores as well. Tunable:
      `EdgeTolerance` via `$MARCO_EDGE_TOLERANCE`; `edgeSlack`/`edgeMatchThreshold` consts.
      Wired into `oshost.scoreAnchor` as a second LOCATOR beside image — `wImage 0.5 / wEdge
      0.3 / wColor 0.2`; the scorer is generalised over a `[]locator`, a locator only creates a
      move candidate when STRONG and colour-corroborated, and is gated on a region-restricted
      search (`!region.Empty()`, so a full-screen wait-image stays image-only). Existing
      anchors get edge for free (same Image path). Tests: `TestFindEdgeRecoloured`.

    - **Multi-scale** (`matchMultiScale` / `findEdgeMultiScale`): both the pixel and edge
      matchers try the template at several scales (default `1.0, 0.8, 1.25`, tunable via
      `$MARCO_FIND_SCALES`) via a bilinear `resize`, keeping the best — so a DPI/resolution
      change that renders the control a different size than the recorded template still
      resolves. The edge path computes the screen distance transform ONCE (`edgeDT`) and
      reuses it across scales (`matchOutline`), so it stays cheap. Tests: `TestFindEdgeMultiScale`,
      `TestResizeDims`.

    - **Colour-histogram (palette) confirm** (`colorHistogram`/`histIntersection`, `screen`):
      `match` computes the 64-bucket RGB histogram intersection between the template and the
      matched window and returns it as `Match.ColorScore`; the scorer folds it in (`wColorHist`)
      as a palette confirm on an image match — order-invariant, so a small warp/AA that hurts
      the spatial score doesn't hurt this. Tests: `TestColorHistogramSamePalette`.
    - **Window-name context** (`Anchor.Window`): a click records the foreground window TITLE
      (`winctx.ForegroundTitle`, captured in `recorder.newClick`, threaded recorder→simplify→
      `macroir.Step.Window`→codegen→Anchor). At run time `oshost.windowFactor` multiplies the
      anchor's confidence by how well the live foreground title matches — a clear wrong-window
      mismatch (Steam's library vs friends vs store) is knocked below the threshold (fail-safe),
      a shared significant word keeps most of it (drifting titles). Only applied once a window
      was Activated this run (else the overlay's focus would false-mismatch). Tests:
      `TestWindowMatchFactor`, codegen `TestImageClickWindowContext`.
    - **Last-known-location** (`internal/oshost/anchorcache.go`): each confident resolve is
      cached (keyed by template path, JSON under the user cache dir, atomic write); the next
      run widens the search box to cover BOTH the recorded point and last-known and boosts the
      position prior there — so the search FOLLOWS a UI that drifts across runs, with the
      recorded point still a safety net. `$MARCO_ANCHOR_CACHE=0` disables;
      `$MARCO_ANCHOR_CACHE_FILE` overrides the path. Tests: `TestAnchorCacheRoundTrip`.

    - **Move-following is hardened** (the demonstrated point is sacred). To click a LOCATED
      point other than the recorded one, the scorer now requires: a locator ≥ `locateFloor`
      (raised **0.85→0.90**), colour corroboration (`colorCorroborates` returns false with no
      recorded colour, so an image-only anchor never moves), AND the moved candidate must beat
      the recorded point's score by `moveMargin` (**0.12**) — otherwise it stays put. A
      look-alike button in a busy menu / a lenient brightness match can no longer drag the
      click off where you clicked (the freeplay regression). `scoreAnchor` also recovers from
      any matcher panic → falls back to the recorded point (a bad template can't crash a run),
      and a last-known cache entry more than `maxDriftPx` (**600px**) from the recorded point
      is ignored as stale/poisoned. Tests: `TestFindStaysWhenRecordedStillValid` (+ the
      existing move/stay tests). Race-clean (`go test -race`).

    Logging is at production tiers: per-click `find:`/`click:` trace is **Debug**; a
    low-confidence anchor is a **Warn** (`re-teach to repair`); `MARCO_LOG=debug` for the
    full trace.
    - **Segmentation** (`DetectButtons`): connected-component (dilated edges) button-rectangle
      detection over any image — drives AutoCrop, and **`SnapToButton`** (windowed, fast on a
      full screenshot) is wired into the **OCR resolver** (`plugins/ocr`): "click the text X"
      now snaps to the centre of the button CONTAINING the word, not the word itself. Tests:
      `screen` `TestAutoCrop*`/`TestDetectButtons`/`TestSnapToButton`, `oshost` `TestFindEdgeConfirms`.
    - **Edges are Sobel** (`edgeStrength`): a symmetric 3×3 Sobel gradient magnitude on
      luminance (replacing the old single-neighbour diff), so faint and anti-aliased borders
      are caught. `EdgeTolerance` default rescaled to 100 (Sobel magnitude is ~4× a raw step);
      still `$MARCO_EDGE_TOLERANCE`.
    - **Lighting/contrast-invariant pixel match — normalised cross-correlation** (`match`):
      when a template's compared pixels have real internal contrast (`opaqueVaried`), the
      matcher models the screen window as a linear transform of the template — predicted =
      `screenMean + gain·(template − templateMean)`, gain = std ratio (`meanStdOpaque`,
      clamped `gainLo..gainHi`). The MEAN term cancels a brightness/white-balance/theme shift
      (night mode, dimming, f.lux); the GAIN term cancels a contrast/gamma change. Pre-filter
      is offset-invariant (sample DELTAS) so a shifted match isn't pre-rejected. Gated on
      opaque-pixel contrast so a flat/uniform-foreground template stays exact-tolerance —
      backward-compatible. Tests: `TestMatchBrightnessInvariant`, `TestMatchContrastInvariant`.
    - **Oriented-edge matching** (`orientedEdgeDT`/`matchOutline`, `orientationBin`): edge
      matching now keys on edge ORIENTATION, not just presence — gradient direction is bucketed
      into `orientBins`=4 over [0°,180°) (folded so contrast inversion shares a bin), and a
      template edge only scores against a screen edge running the SAME way (its own bin's
      distance transform). Rejects the accidental edge alignments a busy/game UI throws up.
      `sobelGrad` exposes the gradient components; the per-orientation DTs are still computed
      once and shared across scales.

14. **Key holds.** A held key (e.g. hold Q while clicking) records as explicit `KeyDown`/
    `KeyUp` so the hold PERSISTS across the steps between. New OS acts `KeyDown`/`KeyUp`
    (`osmod/os.marco`, `oshost` actions, backend `keyDown`/`keyUp`), new `macroir.StepKeyDown`/
    `StepKeyUp`, codegen renders `do OS's KeyDown/KeyUp with "q"`.
    - **Detection** (`simplify.markHolds`, lookahead): a non-modifier key press is a hold when
      it lasts ≥ `holdThreshold` (**500ms**) OR another action (a click / another key) happens
      before its release; otherwise it stays a tap/`Type`. The 500ms is DETECTION-only — the
      real held duration is preserved for free by the existing timing→waits pass (KeyDown at
      press time, KeyUp at release time, the gap becomes a wait).
    - **Safety:** `oshost.Host` tracks held keys; `ReleaseHeld()` (deferred by the driver via
      the `heldKeyReleaser` interface, alongside the cursor restore) releases any still down at
      route end — success, error, or Esc — so a per-command process can never exit with a key
      STUCK down. Tests: `simplify` `TestHoldKey*`/`TestQuickTapIsNotHold`, `oshost`
      `TestKeyDownReleasedAtRouteEnd`/`TestKeyUpClearsHold`.
    - **Tap linger:** plain key taps now hold ~25ms (`tapHold`, `$MARCO_KEY_HOLD_MS`) between
      down and up so a fast game/app registers them instead of dropping an instant tap.

15. **Teach-time text anchors (demonstrated, not just narrated).** A DEMONSTRATED anchor
    (tap the anchor key, then click a moving target) now gains a text locator for free:
    after `simplify`, the orchestrator OCRs each anchored click's captured button template
    and writes the recognised label to `macroir.Step.AnchorText`, so codegen emits the same
    `Text's Find` move-following fallback a narrated "click the text X" produces. Previously
    text anchors were narration-only.
    - **New OCR action `Read`** (`plugins/ocr/read.go`): image→text (the reverse of `Find`'s
      word→point). Input `{ Image }` (base64 PNG of the button crop) → `{ Text }`; declines
      (`failed`, no error) when nothing readable, so the anchor stays gate-only. `joinLabel`
      orders words into reading order (lines top-to-bottom, words left-to-right) so a two-word
      label ("Start Game") still matches at run time; `hasLetter` rejects OCR punctuation noise.
      Wired into `main.go handle()` as `case "read"`. Pure parse/order unit-tested
      (`read_test.go`) — tesseract not needed.
    - **Engine wiring** (`internal/orchestrator`): `enrichTextAnchors`/`labelAnchors`/
      `readAnchorText` call `Hosts["Text"].Invoke(Text's Read, {Image, ClickX, ClickY})` per
      anchored click (recursing loop bodies), after `Simplify` in `Teach` (both passes),
      `TeachAuto`, and `SimplifyRoute` (re-simplify rebuilds steps from raw events, so anchors
      are re-labelled each time — text isn't stored in the recording). **Graceful + additive:**
      no Text host wired (`$MARCO_OCR` unset) → skipped entirely; OCR engine absent / button
      unreadable → anchor stays image/colour-only. The Text host was already in `newDeps()`
      (shared by teach), so no new run wiring. `text.marco` documents `Read` (not a route
      capability — teach calls it directly over the bridge). Tests: `orchestrator`
      `TestTeachTextAnchorFromOCR`/`TestTeachTextAnchorNoHost`, `plugins/ocr` `read_test.go`.
    - **Read the button UNDER the click, not the whole crop.** Whole-crop OCR mislabels: a
      captured menu yields a jumble of every button's text, and the one you clicked (often
      highlighted → inverted contrast) is the one OCR misses. So the click's position inside
      the template is threaded through: `screen.AutoCropAt` now also returns the ORIGINAL
      click-local point (even when the anchor re-centres on a wider button); recorder stores
      it on `RecordedEvent.ClickX/Y` (both armed full-frame and auto-crop paths); `simplify`
      carries it to `macroir.Step.AnchorClickX/Y`; the orchestrator passes it to `Read`.
      `plugins/ocr` `labelAt` picks the contiguous same-line run around the word nearest the
      click — a small inter-word gap keeps a multi-word label ("EXIT TO MAIN MENU") whole, a
      wide gap splits side-by-side buttons (YES | NO), and a click farther than
      `clickReachFactor` (×word height, default 2; `$MARCO_OCR_CLICK_REACH`) from any readable
      word DECLINES (an unreadable highlighted button stays gate-only instead of grabbing a
      neighbour). Verified on the real RL pause menu: RESUME GAME / CHANGE MODE/MATCH /
      SETTINGS each read correctly, the highlighted EXIT TO MAIN MENU declines.
    - **Preprocessing for OCR** (`plugins/ocr/preprocess.go`): grayscale → bilinear upscale
      (`$MARCO_OCR_SCALE`, default 3, **scale-adaptive**: capped so the long side stays under
      `maxUpscaledLong`=2600, so a full-screen run-time capture isn't tripled into a slow OCR)
      → global Otsu to clean FILLED dark-on-white, background-oriented. `$MARCO_OCR_RAW=1`
      bypasses. **The key recall fix was PSM, not thresholding:** tesseract now runs **PSM 3**
      (full page segmentation, `$MARCO_OCR_PSM`) instead of sparse PSM 11 — PSM 11 SKIPS a
      solid-filled button (a highlighted/selected row), PSM 3 reads it, and tesseract 4+
      auto-handles inverted text so one pass covers both polarities. (A local adaptive
      threshold was tried and rejected — it left thick game-font strokes hollow/unreadable;
      Otsu's filled strokes + PSM 3 is what works.) Plus precision gates in `Read`: tesseract
      confidence floor (`Word.Conf`, `$MARCO_OCR_MIN_CONF` default 65) and ≥2 letters/token, so
      an icon's stray glyph never becomes a label.
    - **Validated on the real RL crops** (`Read` with the click point): RESUME GAME, CHANGE
      MODE/MATCH, SETTINGS, NO, YES (highlighted), ACCEPT (stylized) all read correctly; EXIT
      reads "EXIT TQ MAIN MENU" (an O→Q misread — harmless because run-time `Find` OCRs the same
      way, below); the mute icon declines. Stylized/highlighted buttons that PSM 11 + old
      preprocessing couldn't read now resolve.
    - **Run-time `Find` uses the SAME pipeline** (`plugins/ocr/find.go` `locate`): it now
      preprocesses the capture and OCRs with PSM 3 identically to teach-time `Read`, mapping the
      matched word's centre back from upscaled space (÷scale) before `SnapToButton` on the
      original capture. So a label captured at teach time — including any OCR quirk like the
      O→Q — is read back identically at run time and therefore MATCHES (self-consistent).
      `$MARCO_OCR_DUMP=<path>` writes the binarized image for inspection.
    - **Demonstrated text anchors search a REGION, not the whole screen** (`codegen`
      `TextSearchMargin`=400): a demonstrated anchor (it carries the recorded click) emits
      `X1/Y1/X2/Y2` of ±400px around that point on the `Text` anchor, so run-time `Find`
      OCRs just that box — faster and no stray match elsewhere on a busy desktop. A narrated
      text-only anchor (no coordinate) keeps the whole-screen search. Tests: `codegen`
      `TestGateAndTextClickRoute` (region present), `TestTextOnlyClickRoute` (absent).
    - **Recall limit:** icon-only buttons (grad-cap, shovel) have no text — they stay
      image/edge/colour-anchored (correct), and are the motivation for the Vision resolver
      (item 16). Tests: `plugins/ocr` `preprocess_test.go` (scale cap, Otsu separation),
      `read_test.go` (click-point selection, precision gates).

16. **Semantic vision resolver — `plugins/vision` (NEW, scaffolded + validated).** A learned
    UI-element detector as a fourth resolver, for the controls OCR and geometry can't anchor —
    an icon on a gradient with no text and no clean edge. It's a bridge host plugin (sibling of
    `plugins/ocr`), so its heavy ONNX dependency never reaches the zero-dep engine.
    - **Pipeline** (mirrors the user's sketch): capture (`internal/screen`) → letterbox+normalise
      (`yolo.go`, pure Go) → ONNX YOLOv8-style detector (`backend_onnx.go`) → decode+NMS
      (`yolo.go`) → labelled boxes. The `Vision` act exports **Detect** (region → `{Elements:[{Label,
      X,Y,W,H,Score}]}`) and **Locate** (`{Label?, X?,Y? hint, region?}` → a `{X,Y}` Point, same
      shape as `Text's Find` so it drops into the same `or?` slot). `Locate` picks the highest-
      scoring element of the requested label (case-insensitive substring; omit = any), nearest an
      optional hint. OCR-of-crops is NOT done here — compose with the `Text` host so each resolver
      stays single-purpose.
    - **Dependency-free by default, real backend opt-in.** `backend_null.go` (default) has no
      external dep, declines gracefully (`Ready()=false`) — the plugin builds, runs, and speaks the
      bridge protocol everywhere. The real detector is `backend_onnx.go` behind `//go:build
      onnxvision`, using **`github.com/yalue/onnxruntime_go`** (cgo, like the voice plugin; loads
      `onnxruntime.dll` dynamically, works on Windows — the cgo-free `onnxruntime-purego` does NOT
      build on Windows, that was tried). **Validated:** default build+tests green; the tagged build
      COMPILES against the real API (cgo+gcc, 5.4MB) — only actual inference is unvalidated (needs
      the runtime lib + a model). `go.mod` carries NO external require (the dep is added only when
      building with the tag).
    - **Engine wiring:** `internal/visionmod` embeds `vision.marco` (`use vision.` resolves via
      `driver.builtinModule` like `os`/`text`); `cmd/marco newDeps` adds `Hosts["Vision"]=
      bridgehost.New($MARCO_VISION)` (lazy). **`setup.ps1 -Vision` AUTO-PROVISIONS the whole real
      detector** (kitchen sink): `Install-OnnxRuntime` downloads the ONNX Runtime Windows lib
      (`onnxruntime-win-x64-$ver.zip` → `onnxruntime.dll`); `Get-VisionModel` downloads the
      OmniParser `icon_detect` weights and EXPORTS them to ONNX via ultralytics (Python) — or takes
      `-VisionModel <onnx>` (BYO); then `go get`s the binding and builds `-tags onnxvision` (needs
      gcc). Any missing piece (no gcc/Python/download) → null detector + a note on what's missing,
      never hard-fails. Pins `MARCO_VISION`/`MARCO_ONNXRUNTIME`/`MARCO_VISION_MODEL`/`_LABELS`
      (OmniParser is single-class → `icon`) + a `--host Vision=bridge:` line into `overlay.cmd`.
      Downloads land in `_dl/` + `plugins/vision/models/` (gitignored). Overrides:
      `-OnnxRuntimeVersion`, `-VisionModelUrl`, `-VisionLabels`. Env: `MARCO_VISION_MODEL`, `MARCO_ONNXRUNTIME`,
      `MARCO_VISION_LABELS` (or `labels.txt` beside the model), `MARCO_VISION_SIZE`/`CONF`/`IOU`,
      `MARCO_VISION_INPUT`/`OUTPUT`. Tests: `plugins/vision` `yolo_test.go` (letterbox, decode, NMS,
      un-letterbox), `detect_test.go` (Locate selection, null detector declines).
    - **Teach integration — DONE (the single demonstration path).** A demonstrated anchor is
      now labelled by BOTH wired hosts in one teach-time walk (`orchestrator.enrichAnchors` →
      `readAnchorText` + `readAnchorVision`): the Text host's `Read` (image→text) sets
      `AnchorText`, the Vision host's new **`Identify`** action (image+click→class label, the
      teach-time analog of Locate; `plugins/vision/identify.go`, `elementAt` picks the element
      whose box contains the click) sets `macroir.Step.AnchorVision`. Codegen's
      `emitAnchoredClick` is now a general ORDERED locator chain — gate → Text's Find → Vision's
      Locate → recorded coordinate (text before vision: exact beats class) — emitting `use
      vision.`, a `Label`+region+hint vision anchor, and the nested `or?` arms. (Narration is NOT
      wired and won't be — per the plan, voice will just START demonstration teaching, so there's
      one teaching path.) Tests: `orchestrator` `TestTeachVisionAnchorFromDetector`, `codegen`
      `TestGateTextVisionClickRoute` (chain + ordering), `plugins/vision` `identify_test.go`.
    - **Debug spike: `marco vision detect <png> [out]`** (`plugins/vision/cmd_detect.go`, engine
      passthrough in `cmd/marco/vision.go`): runs the detector over a SCREENSHOT FILE (no live
      capture), prints a `{label, score, box}` table, and writes an annotated copy (one colour per
      class + legend). The way to test a candidate model on *your* UIs before trusting it in teach.
      Null build → "no model"; `-tags onnxvision` + model → real output.
    - **The real gating risk is the MODEL, not the wiring.** A YOLOv8 UI detector (e.g. Microsoft
      OmniParser v2) must be sourced/licensed AND verified to detect *these* game UIs — likely a
      domain gap (trained on web/desktop UIs), the same problem OCR hit; and `Identify` runs the
      detector on a small button CROP, which a full-screen-trained model may handle poorly (a
      teach-time spike would tell). The pure-Go pre/post is model-agnostic, so swapping models is
      config, not code. Graceful throughout: no model → no vision label → anchor unaffected.

## How to build / test / run

```sh
go build ./... && go test ./...            # 19 packages, deterministic (stubs off-Windows)
go build -o marco.exe ./cmd/marco
go build -o marco-macros.exe ./cmd/marco-macros
go -C plugins/overlay build -o overlay.exe .
.\overlay.cmd                              # Windows: launches voice|serve + overlay stack
```
- The overlay is long-running — **restart `overlay.cmd`** to pick up a new
  `overlay.exe`. Engine binaries (`marco.exe`) are spawned fresh per command, so they
  take effect immediately. `$MARCO_BIN` overrides which engine the overlay shells to.
- CLI tests that mutate routes: set `$MARCO_ROUTES` to a temp dir. `marco do` uses the
  REAL OS host (types for real) — never run it blind in a test; use `marco run --host
  dryrun` or the Go tests for behavior checks.

## Architecture orientation

- **Engine** (`cmd/marco`, `internal/*`): lexer→parser→graph→compile→runtime;
  `driver` is the run/serve/check entry; `routes` is the registry + arg parsing;
  `orchestrator` is the teach/run loop (`Teach`, `TeachAuto`, `TeachVoice`,
  `SimplifyRoute`, `dispatchDo` via cmd). `oshost` fulfils the `OS` act; `secrets`
  is the credential store; `winctx`/`screen`/`recorder` are the OS surfaces (Windows
  + cross-platform stubs).
- **Overlay** (`plugins/overlay`): MVC — `model.go` (state), `view.go` (ebiten draw),
  `controller_windows.go` (global LL keyboard hook — callbacks MUST return fast, see
  `ll-hook-callbacks-must-return-fast` memory), `acts.go` (the `Overlay` act + child
  spawning). Behavior lives in `plugins/overlay/overlay.marco`.
- **Run stack:** `marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay plugins/overlay/overlay.marco`,
  with `voice.exe | …` piping Voice events into serve's stdin.

## Pending / known issues / open decisions

- **URL/colon edge:** ~~`routes.ParseInvocation` reads a leading `word:` as a named
  arg, so `go to http://…` mis-splits.~~ **Fixed** — `ParseInvocation` skips any
  token whose value starts with `//` (URL schemes), so URLs in route phrases work.
- **Auto-pop needs a saved route:** `marco args` resolves against saved routes, so
  labels don't appear until a route exists. Fine for normal use.
- **Voice continuous-listen:** ~~requires the wake word per phrase during
  narrate-teach.~~ **Fixed** — while a narrate-teach session is active the overlay
  writes `$TEMP/marco-narrate.lock`; the voice plugin re-arms after each command
  phrase (instead of disarming), so only ONE wake-word utterance is needed to start
  the session. Override the lock path with `$MARCO_NARRATE_LOCK`.
- **macOS/Linux:** `winctx`/`screen`/`recorder`/`secrets` have stubs only; backends
  are additive work.
- **Anchor resolvers:** image + colour in the engine (item 9), text/OCR as a plugin
  (item 10) — all three reuse one `Anchor` set. ~~teach-time text capture for
  *demonstrated* (not just narrated) text anchors~~ **Done** (item 15 below). Next OCR
  candidate: a native no-install backend behind `ocrEngine` (Windows.Media.Ocr, macOS
  Vision). **Claude-vision auto-crop** stays a deferred opt-in plugin (armed anchors are
  whole-frame capture; user crops manually for now).
- **Arg key / secret-name list** are conventions (F9; password/pin/token/…) —
  overridable later; `MARCO_ARG_KEY` exists, the secret-name list is hardcoded in
  `codegen.isSecretArg`.

## Env vars

**`MARCO_CV`** (master CV switch — `setup.ps1 -CV` / `-NoCV`): `max` = KITCHEN SINK — every
demonstrated click becomes a multi-signal anchor (image+edge+colour+histogram+window, OCR
text locator, Vision class locator, last-known cache), EVERY button, even a non-distinctive
patch (`recorder.cvKitchenSink`: anchors all buttons + skips the `Distinctive` gate so the
OCR/Vision resolvers can still label it); `off` = plain coordinates, no anchors/resolvers;
unset = today's per-knob defaults. Run-time scoring stays conservative either way (kitchen
sink captures MORE signals, never clicks less safely). The one flag to A/B the whole CV
stack; re-run `setup.ps1` + relaunch `overlay.cmd` to flip. Tests: `recorder`
`cv_windows_test.go`.

**`MARCO_CV_SENSITIVITY`** (0 = strict … 1 = loose, default 0.5) — the **CV find dial**,
the fine control beside the coarse `MARCO_CV` switch. One knob that scales how eagerly an
anchor is found, trusted, and followed. It reaches SIX derived knobs across two packages
(each keeps its own env override, and **0.5 reproduces every legacy value exactly**, so the
dial is a no-op until dragged):
  - `oshost` (via `oshost.cvLerp`) — the ACT-ON-evidence gates: `findConfidence` (0.9→0.3,
    legacy 0.6; `$MARCO_FIND_CONFIDENCE`), `locateFloor` (0.99→0.81, 0.90) and `moveMargin`
    (0.20→0.04, 0.12), i.e. how sure it must be to click a found box and to FOLLOW a moved one.
  - `screen` (via `screen.cvLerp`) — the FOUND gates + edge detection, the fix for "the button
    is on screen but it has no idea": `MatchThreshold` (0.90→0.60, legacy 0.75;
    `$MARCO_FIND_THRESHOLD`) and `edgeMatchThreshold` (0.70→0.40, 0.55; `$MARCO_EDGE_MATCH`) are
    the pixel/edge locators' `Found` gates — below them a present-but-imperfect button
    contributes ZERO evidence (`scoreAnchor` only credits a locator when `l.found`), so no
    confidence gate could rescue it; and `EdgeTolerance` (130→70, legacy 100;
    `$MARCO_EDGE_TOLERANCE`) which drives BOTH edge matching and **AutoCrop/DetectButtons** — a
    looser dial detects fainter button borders so the capture-time crop snaps to the control
    (the "cropping doesn't work great" lever; crop changes need a **re-teach** to take effect,
    the found/confidence gates apply to existing routes immediately).
Exposed as the overlay config editor's **`cv find`** slider (`config.go` `Sensitivity`, step
0.05); the overlay passes `MARCO_CV_SENSITIVITY` to each spawned `marco` when off-neutral
(`acts.go streamChild`) — run AND teach — so a drag takes effect on the **next command with no
restart** (`marco do` runs the OS host in-process; `screen`'s vars read the env at package
init of the fresh process). Tests: `oshost` `sensitivity_test.go`, `screen`
`sensitivity_test.go` (`TestCVSensitivityGates`/`TestGateEnvOverride`).

`MARCO_ROUTES`, `MARCO_STOP_KEY` (f12), `MARCO_ARG_KEY` (f9; `off`), `MARCO_ANCHORS`,
`MARCO_RESOLVER`, `MARCO_BIN`, `MARCO_OVERLAY_IDLE`, `MARCO_VOICE_WAKE`,
`MARCO_NARRATE_LOCK` (lock file path; default `$TEMP/marco-narrate.lock`),
`MARCO_NO_PANIC_STOP`/`MARCO_NO_TEACH`/`MARCO_SIMPLIFY_SAVES` (set by the overlay).

## Not done / explicitly out of scope this session

No commit/push was made (commit only when the user asks). No merge of
`feat/host-ffi`. No new dependencies added to the engine.
