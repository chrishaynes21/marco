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
`spec/` (language). **`docs/` is the Director's documentation** — one file per milestone,
each ending in a "Known gaps" section that states what is not proven (item 19).
Cross-session notes live in the author's memory; the durable project facts are also
captured here.

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

17. **Local-first NL resolver — `plugins/llama` (NEW).** A small language model that runs
    ON THE USER'S MACHINE, so `marco assistant`/`marco do` understands loose phrasing the
    deterministic `internal/nlu` matcher misses ("fire up the pirate game" →
    `start-sea-of-thieves`) — the step toward Marco feeling conversational. It's a drop-in
    for the existing **`$MARCO_RESOLVER`** seam (`internal/resolver` protocol,
    `{input,routes}→{route}`), so **ZERO engine changes** — the local model plugs into the
    exact spot `claude-resolver` did.
    - **One plugin, OpenAI-compatible.** It speaks `POST <base>/chat/completions`, so the
      SAME binary drives a **local llama via Ollama** (default `http://localhost:11434/v1`,
      no key), LM Studio / `llama.cpp` server, OR OpenAI's cloud (`MARCO_LLM_URL=
      https://api.openai.com/v1` + `MARCO_LLM_KEY`). **Local is the deliberate default** —
      no key, no cost, private, no public API to slam; cloud is opt-in only.
    - **Own module, `net/http` only** — no model SDK reaches the zero-dep engine, and unlike
      `claude-resolver` (Anthropic SDK) there's nothing to `go get`. `main.go` (stdin/stdout
      protocol) + `llm.go` (config from env, chat call, `pick` extracts a valid slug from a
      chatty small-model reply with token-boundary safety). **Graceful:** server down /
      missing / slow / garbled → empty route → the assistant falls back to its offline
      matcher/teach, never blocks (hard `MARCO_LLM_TIMEOUT_MS` cap, default 20s). The engine
      re-verifies the slug, so a hallucinated route can't run.
    - **Config (env):** `MARCO_LLM_URL` (default Ollama), `MARCO_LLM_MODEL` (default
      `llama3.2:3b`), `MARCO_LLM_KEY` (opt-in cloud), `MARCO_LLM_TIMEOUT_MS`.
    - **`setup.ps1 -Llama`** builds `llama.exe`, installs Ollama (winget `Ollama.Ollama`),
      `ollama pull`s `-LlamaModel`, and pins `MARCO_RESOLVER`+`MARCO_LLM_MODEL` into
      `overlay.cmd` (WINS over `-Resolver` when both built — both set `MARCO_RESOLVER`,
      local-first). Any missing piece (no winget/Ollama) → plugin still wired, degrades until
      Ollama runs. Tests: `plugins/llama` `llm_test.go` (exact/chatty/NONE/unknown-slug/
      server-down/empty-input via `httptest`, `pick` boundaries) — no live model needed.
18. **Route-level dispatch — `internal/dispatch`** (built as `internal/director`; **RENAMED**
    when the desktop Director landed — see item 19. `marco dispatch "<phrase>"`,
    `dispatch.Dispatcher`, `dispatch.PluginAdvisor`, `$MARCO_ASSISTANT`. The two systems are
    separate and neither imports the other: dispatch picks a saved MACRO, the Director decides
    what to do to the SCREEN.) The decision-maker that turns
    a plain line into ONE verified `Decision` — `run` (a checked slug) / `teach` (a new
    command name) / `chat` / `clarify` — so a front-end holds a natural back-and-forth.
    - **Independent of any LLM by design.** Its baseline is the deterministic offline matcher
      (`internal/nlu`): exact name → run, near match → clarify, else → teach. That works with
      NO model. Smarter understanding layers in via the **`Advisor`** interface — an optional
      intelligence source the director consults and always re-verifies (a proposed route must
      be one the user has, so no backend can make it run something that doesn't exist). A
      local LLM is one Advisor (`director.PluginAdvisor`, `llm.go`); a rules engine / remote /
      test double are equally valid. The director owns policy, the model is swappable.
    - **`Decide` vs `Propose`.** `Decide` = full policy, always returns a usable Decision (the
      overlay/`marco director` seam). `Propose` = ONLY the Advisor's validated suggestion, ok=
      false when unconfigured/unsure — so the interactive assistant layers the brain on top of
      its existing fuzzy-confirm flow WITHOUT regressing a legacy `$MARCO_RESOLVER`-only setup
      (a converse-illiterate resolver returns empty-intent → Propose false → classic path runs).
    - **Plugin protocol** (`plugins/llama` converse mode): `{mode:"converse",input,routes,app}
      → {intent,route,name,reply}`. The `mode` field lets ONE `llama.exe` serve both the legacy
      resolver and the director. Small-model-tolerant parse (`parseDecision`: extracts the JSON
      object from prose, snaps a `run` slug to a real route case-insensitively or downgrades to
      clarify, rejects an unknown intent). Graceful: model down/garbled → empty intent → director
      falls back.
    - **Wiring:** `internal/director.PluginPath` reads `$MARCO_ASSISTANT` then `$MARCO_RESOLVER`
      (same binary serves both, so `-Llama` enables conversation too — it now pins both).
      `cmd/marco/director.go`: `marco director "<phrase>" [--json]` (non-acting classifier seam
      for the overlay; `--json` emits the decision) + `converseTurn` woven into `runAssistant`
      (exact → run fast-path, else brain `Propose`, else classic flow). Tests: `internal/director`
      `director_test.go` (deterministic-only exact/clarify/teach, Advisor pass-through + hallucinated-
      route rejection + not-ok fallback + exact-skips-advisor, PluginAdvisor over a fake plugin),
      `plugins/llama` `llm_test.go` converse cases (teach/canonicalize/downgrade/wrapped-JSON/bad-
      intent/server-down).
    - **NOT yet:** overlay UI wiring (the ebiten front-end calling `marco dispatch --json` and
      rendering the reply / kicking off teach) — the engine seam is ready; that's a
      `plugins/overlay` MVC change. Voice stays "start demonstration" only (one teaching path).

19. **The Director — `internal/director`, `cmd/director`, `pkg/directorapi` (the large NEW
    subsystem).** Where dispatch runs a route you already taught, the Director builds a world
    model of the desktop from the accessibility tree and plans, executes and VERIFIES arbitrary
    semantic actions against it — "click Save", "focus the search box then type Director and
    press enter", "close every Notepad window". It is built ON Marco, not beside it.
    - **The Marco boundary is load-bearing.** The Director does not call `Host.Invoke`. It lowers
      each planned step to **legal Marco source** (`internal/director/marcoexec`), which then goes
      through lexer → parser → graph → compile → runtime like any route, so Marco's compiler
      validates every action before anything moves. There is **no second path** — the old
      `internal/platform/marcohost` adapter was deleted, and a test asserts the Director holds no
      duplicate platform implementation. This caught three capabilities (`ClipboardGet/Set`,
      `MoveWindow`, act `Accessibility`) that "worked" from `oshost` but no line of Marco could
      call; they're now declared (`internal/osmod/os.marco`, `internal/uiamod/uia.marco`).
    - **Shape:** `pkg/directorapi` is the dependency-free contract (world, observation, intent,
      plan, action, confidence, actionability); `internal/director/*` is the Director proper
      (perception, target, plan, program, execute, verify, wait, values, variables, collections,
      trace, actiongraph, memory, policy, edit, service); `internal/platform/*` are the adapters
      (`uiaclient`, `ocrclient`, `wincapture`, `winprovider`, `marcorunner`) — the Director sees
      only `directorapi` interfaces and imports no engine code. `plugins/uia` is the C#
      accessibility bridge (its own module, off the zero-dep engine).
    - **It's a SERVICE.** `director serve` holds the warm accessibility client (Chromium only
      exposes its tree to a sustained client, so per-command attach always got a cold shallow
      tree). `marco director "<phrase>"` is the thin client; the overlay/voice go through it.
      Plus a deep diagnostic surface: `status`, `explain`, `plan`, `trace`, `lower`, `wait`,
      `visual`, `ocr`, `observations`, `fusion`, `collections`, `graph`, `history`, `stop`.
    - **The docs are in `docs/`, one per milestone, and they carry the real detail** —
      `director-marco-boundary.md`, `-programs.md`, `-perception.md`, `-waits.md`, `-editing.md`,
      `-service.md`, `-cancellation.md`, `-collections.md`. Each ends with a **Known gaps**
      section stating what is NOT proven; read those before trusting a capability.
    - **Collections** (`internal/director/collections`, `execute/iterate.go`,
      service collection events, `director collections` / `explain collection`). A collection is
      a bounded semantic QUERY re-run every iteration — never a captured list of ids/handles —
      with a second bulk-policy gate (closed allowlist: `focus`/`activate`; a bulk click always
      asks), semantic-key member identity, verify-before-advance, and ordinal-drift protection
      across a clarification pause. See `docs/director-collections.md`.
    - **Newest milestone: SEMANTIC GOAL DECOMPOSITION** (`internal/director/goal`). The layer
      above Program: the user describes an OUTCOME and the Director produces the program.
      15 goals (`rename`, `create_folder`, `save`, `save_as`, `print`, `delete`,
      `close_without_saving`, `duplicate`, `download`, `move`, `copy`, `paste`, `open_file`,
      `open_settings`, `create_tab`) expand through 18 HAND-WRITTEN typed procedures — never
      LLM-generated — into ordinary `program.Program`s, validated by the ordinary validator,
      so **nothing downstream changed**: variables, collections, clarification, replay and
      verification all keep working untouched. Application overrides select most-specific-first
      (`explorer rename` reaches Rename from the context menu; `vscode rename symbol` is a
      DIFFERENT operation that shares the word and demands confirmation). Waits are step
      PRECONDITIONS (semantic, never sleeps); the "if a prompt appears" of close-without-saving
      is a BEST-EFFORT step, not a branch. Procedures declare their safety (mutations,
      destructive/external/irreversible, confirmation) before expansion, and missing
      requirements are refused as a typed question BEFORE anything runs. New diagnostics:
      `director goals`, `director procedures [name]`, `director explain goal "<request>"
      [--app X]` — which expands without running. See `docs/director-goals.md`, including its
      **HARDENED**: goal-level confirmation is now enforced between expansion and step 1
      (accepted/rejected/unavailable distinguished; unavailable never means yes); goal
      provenance is persisted on action-graph nodes as DIAGNOSTIC metadata that replay
      never reads; procedure labels are semantic ControlRoles with localized alias tables
      and an exact-match requirement for destructive choices; goal selection detects
      ambiguity deterministically instead of letting registration order decide;
      best-effort steps skip ONLY on a demonstrably absent target. New: `director goal
      --dry-run "<request>"`.
      **SAFE-BINDING milestone**: `internal/director/binding` adds deictic bindings with a
      CLOSED object-kind vocabulary (file/folder/document/editor_buffer/symbol/control/
      window/text_selection), decisive-evidence recording, ambiguity candidates, a
      stability token, and REVALIDATION before action (refresh on a harmless re-observe;
      REFUSE when focus moved — never silently re-bind). Implemented and unit-tested but
      **NOT yet wired into the execution path**. Registry validation is now enforced at
      service startup and in every CLI command (`goal.NewValidatedRegistry`). Live
      scenarios now SKIP with precise unmet prerequisites instead of failing by design —
      **no live scenario has ever run**. Still NOT implemented: action-level confirmation
      for non-goal actions, replay confirmation policy, and the live harness itself.
    - **SEMANTIC UI ACTIONS** (`internal/director/uiact`, `pkg/directorapi/
      semantic.go`). The action vocabulary grew from six verbs to **33** — expand, collapse,
      toggle, check, select, choose, invoke, open, close, dismiss, submit, confirm, cancel,
      refresh, back/forward, next/previous, undo/redo, copy/cut/paste, scroll_here,
      show_context_menu, maximize/minimize/restore, pin/unpin. The planner emits the VERB and
      stops; `uiact.Ladder` picks the implementation at execution time from the control's REAL
      capabilities (accessibility pattern → generic invoke → click → refuse), recording every
      stronger rung it rejected and why. Seven new Marco capabilities (`Accessibility's
      Invoke/Expand/Collapse/Toggle/Select/Deselect/ScrollIntoView`) in `uia.marco` + the C#
      bridge; the bridge now also reports each control's **patterns** and its expanded/checked
      state, which is what makes the ladder evidence-driven rather than a guess. Verification is
      per-verb (an expand is proved by the node reporting itself expanded or its children
      appearing — never by "the screen changed"). The action graph stores the verb and the
      query, never the mechanism, so a replay chooses the lowering again. New diagnostics:
      `director actions [name]`, `director explain action`. See
      `docs/director-semantic-actions.md`, including its Known gaps.

## How to build / test / run

```sh
go build ./... && go test ./...            # 73 packages, deterministic (stubs off-Windows)
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

## Director: Grounding DINO challenger, benchmarked (latest)

**Status: Grounding DINO runs end to end through the benchmark against real Rocket League
evidence. Verdict: CONTINUE TUNING, not promote.** Nothing is committed.

**The result, on a 6-frame privacy-reviewed Rocket League corpus:**

```
METRIC                    classical-cv   current  grounding-dino
detections                          92         8              13
accepted                            92         5               8
structural coverage               100%      100%             20%
temporal persistence               48%       22%             23%
false structure (lower)            67%      100%            100%
median latency                      0s     109ms          9.594s
acceptance floors            0.35/0.50 0.35/0.50   0.30 (calibrated)
SCORE                             61.9      53.4            20.2
```

Grounding DINO loses, and the reason is almost entirely **latency**: 9.6s per frame against
a 500ms sampling budget, scoring zero on that dimension. `torch` here is **CPU-only on a
machine with an RTX 5070**, so this is a measurement of an unaccelerated install rather than
of the model. That is the single most actionable follow-up.

**`internal/director/visionbench` additions:** closed versioned vocabulary (`game-ui-v1`,
digest in every report), `GroundingDINO` adapter over a Python subprocess, `Classical`
rectangle detector, `LoadFixture` with a required privacy-review field, threshold sweep,
per-backend acceptance calibration. `plugins/vision-groundingdino/detect.py` is detection
only — it is handed a prompt built by the Go side and never asked an open question.

**Two defects the first real run exposed, both mine:**
1. **`temporal persistence 102%`** — the same occurrences-vs-frames bug fixed in the observe
   metrics, reintroduced here. Several boxes of one class landing in one coarse ninth share
   an identity (intended) and each was counted. Now counts frames.
2. **The adapter and the acceptance filter spoke different vocabularies.** `NormaliseLabel`
   mapped onto Director roles (`pane`, `menu_item`); the filter speaks `vision.Class`
   (`panel`, `menu`). Twelve of thirteen detections were discarded as unknown and the report
   blamed the model. Retargeted onto the detector contract's own vocabulary.

**And a third, which was NOT a bug but read like one:** with production floors (0.35/0.50)
Grounding DINO lost 12 of 13 to confidence alone — it returned 0.32 for a correct text
region and 0.40 for a correct menu. Confidence scales are not comparable between models, so
a backend may now declare its own acceptance floors and the report states which were used.
Accepted rose from 1 to 8.

**Shadow-only is enforced by test**: `cmd/director/shadow_test.go` fails if any runtime
composition file mentions the challenger, if it is constructed anywhere but `benchcmd.go`,
or if the runtime imports the benchmark package.

**Not done:** the DNFC fixture corpus, region-classification mode (Part 9), scoped-OCR
impact measurement (Part 13), the statistical-variance report (Part 15), and memory
measurement (Part 16). Definition-of-Done items 1–4, 6–9, 11–14 are met; 5 is partial
(Rocket League only) and 10 is open.

## Director: vision benchmark harness

**Status: the harness is built, tested and mutation-verified. It has NOT yet been run
against real game fixtures, and no AI backend has been evaluated.** Nothing is committed.

**`internal/director/visionbench` (NEW)** — registry, frozen-fixture runner, metrics,
weighted scoring, comparison report. A backend is an adapter and one `Register` call; the
package owns no perception and never touches a desktop.

- **Frozen fixtures only.** Two backends benchmarked against two different moments are not
  comparable, and the difference would look exactly like a difference between models.
- **Acceptance mirrors production.** A detection is judged by what the Director *would*
  accept — the same confidence floors, the same structural bar, the same class vocabulary —
  so a backend whose boxes are all rejected downstream scores as having not helped.
- **Documented weights** (structural 30, temporal 25, naming 20, trust 15, latency 10) with
  the reasoning for each in the package doc. **Detection count carries weight zero** and
  appears only for context.
- **Every score shows its working**: each dimension reports its points and why, and the
  report names the dimension that separated the top two.
- **`Classical`** — a pure-Go rectangle detector as the second backend, proving pluggability
  with no model, weights or plugin. Not expected to win; expected to be measurable.

**The mutation tests found my own tests were too weak.** Of the three the spec asks for,
only one initially failed:
- *reward raw volume* — caught.
- *ignore anonymous detections* — NOT caught, because the noisy backend already lost on
  structure and stability, so the anonymity dimension was never load-bearing in any
  assertion. Added a test of two backends identical except that one names what it finds.
  (The first mutation attempt also didn't compile, which made the count meaningless until I
  wrote one that builds — worth remembering: a mutation that fails to compile proves
  nothing.)
- *randomise frame order* — NOT caught, because every fixture frame was identical, so
  reordering changed nothing, and the ordering test only asserted that the two backends
  agreed. Frames now differ in size and the test asserts the FIXTURE's order.

All three now fail the suite.

**Not done:** real DNFC and Rocket League fixture corpora (Parts 9–10), an actual AI backend
adapter (Part 11 — only the classical one exists), `director benchmark-vision --fixture`
wiring to the new harness, `director vision backends`, and shadow evaluation. Definition-of-
Done items 1–5 and 7 are met inside the package; 6 and 8 are open.

**Still outstanding from the previous milestone:** the accessibility provider is not pinned
to a session's window, so a live observation session is vision-only. That does not affect
this harness (a detector benchmark over frozen frames is vision-only by design), but it
still blocks any conclusion about the Director's combined perception.

## Director: detector benchmark + a defect that invalidates its premise

**Status: the benchmark harness is built and produces real numbers. The model comparison is
NOT valid yet, because a defect found while running it means the benchmark measured
vision alone rather than the Director's perception.** Nothing is committed.

**First, a crash the DNFC session caused** — `fatal error: too many callback functions`.
`syscall.NewCallback` allocates from a fixed process-wide table Go never frees. `Monitors()`
had leaked one per call for a long time, harmlessly, because it was rarely called; then
`onScreen()` began calling it PER WINDOW inside `LiveWindows()`, and a session sampling
every 2s exhausted the table and killed the service in under three minutes. Both callbacks
are now created once via `sync.Once`, and the monitor layout is read once per enumeration.
`internal/winctx/callback_windows_test.go` runs 4,500 enumerations — a regression crashes
the test binary, which is the right severity for "the process dies".

**`internal/director/observe/metrics.go` (NEW)** — semantic usefulness, not detection
volume: stable-entity yield, anonymous ratio, structural-role coverage, safe-label
opportunity, flicker rate, transition utility, unstable-structure ratio. Named thresholds,
every failure reported rather than the first, deterministic output. `director
benchmark-vision <session>` scores a stored session with no game running.

**Baseline, from the real 3-minute DNFC session (52 samples):**

```
stable entities            1        unstable  139
anonymous ratio           70%       (want under 60%)
structural roles           1%       (want 25%+)
safe-label opportunity     0%       (want 15%+)
flicker                  0.30       (want under 0.15)
transition utility        36%
vocabulary          icon=129 grid=10 pane=1
→ NOT suitable as a default: fails 5 of 7 thresholds
```

**Then the defect that invalidates the comparison.** Calibrating against VS Code — which has
a rich accessibility tree — produced 46 entities, **all of them `role=icon`**. Not one
accessibility element reached the sample.

Cause: the sampler pins the validated window for vision and OCR, which read
`Runtime.activeWindow`. The ACCESSIBILITY provider has its own notion of the active window
and is not pinned, so during a targeted session it observes whatever is actually in front —
the terminal — and `buildSample`'s window filter then correctly discards all of it. **A
targeted observation session is therefore vision-only by construction.**

So the baseline number above is honest about `icon_detect` ALONE, and says nothing about the
Director's combined perception. Every conclusion about model selection has to wait until
accessibility reaches a pinned session.

**Not done:** the second backend (only one learned detector exists on this machine; a
classical-CV candidate over `screen.DetectButtons` is writable but unwritten), Rocket League
benchmark evidence, role-mapping tables, open-vocabulary evaluation, hybrid routing, and the
fixture-based benchmark that needs no service. Definition-of-Done item 1 is met; 2–12 are
open.

**Next, in order:** pin the accessibility provider to the session's window; re-run the DNFC
baseline; only then compare backends.

## Director: observation privacy rewrite + mutation guard

**Status: the privacy defect is fixed and verified live. Durable storage, fixture export
and event streaming are NOT built** — deliberately, since nothing should become durable
while the classifier was wrong. Nothing is committed.

**The classifier is now two-stage, and the first stage is structural.**

The old rule asked what the TEXT looked like — word length, letter ratio, a marker
blacklist. That cannot work, and the live Chrome session proved it: `"Chris Haynes Plus"`
is three ordinary capitalised words, indistinguishable by shape from `"Exit To Menu"`.

What separates them is what they are ATTACHED to. A button's name is a fact about the
interface; text found inside an icon's box is a fact about the person using it. So:

1. **Structural eligibility.** Only `button`, `menu_item`, `menu`, `tab`, `checkbox`,
   `radio` may hold plaintext. `icon`, `text`, `image`, `unknown` and anything added later
   default to private — the allowlist is closed, so a new role is refused until somebody
   decides otherwise.
2. **Shape**, unchanged, as defence in depth: even a button's name is refused if it looks
   like a token.

Everything else keeps role + length + confidence + digest, which is all temporal analysis
ever needed — the digest is what makes "this changed" observable without "this said" being
retained.

**Verified live against the content that leaked.** Re-ran the same Chrome session and
scanned both the human report and `--json`: `Chris Haynes`, `Silly Fire`,
`Notes app assistant`, `Marco Language Design`, `blakeus` — all clean. Labels now render as
`(withheld, 6 chars, ff5d43925d81)`.

**What it costs, stated plainly.** The detector in use emits one class that maps to
`RoleIcon`, so a game's menu labels are withheld too — the same `RESUME GAME` that reads
perfectly is now a digest, because nothing structural vouches for it being a button. That is
the honest trade: a classifier loose enough to keep those would also keep a friends list.
The remedy is a detector with a real class vocabulary, which was already the top known gap
in `docs/director-vision.md`.

**Mutation guard (Part 5).** `Runtime.Handle` refuses while a session is active, naming the
session and how to stop it. Refusal rather than pause-and-resume: pausing leaves a gap that
looks like a quiet moment, and resuming stitches two halves together as though nothing
happened between them.

**Not done:** durable store (Part 2), fixture export (Part 3), event streaming to clients
(Part 4 — the runner publishes to `NopEvents` at the composition root), barrier tests for
blocked providers (Part 6), game-pack interpretation (Part 7), and the five-minute live
Rocket League run (Part 8). Definition-of-Done items 4 and 6 are met; 1–3, 5 and 7–10 are
open.

## Director: passive observation — runs live

**Status: the vertical slice works end to end against a real application.** Nothing is
committed. **One privacy defect is open and must be fixed before storage lands — see below.**

```
director observe-game --application chrome --duration 30s --interval 1s
  → returned in 32ms
director observation-session observe_1
  → State: observing   Target: application chrome → chrome   Samples: 3
director status            → answered in 30ms mid-sample
director cancel-observation observe_1 → returned in 30ms, evidence kept, INCOMPLETE
director observation-insights observe_1 → stable entities, changes, unreliable evidence,
                                          hypotheses with contradictions and validations
```

**What was wired** (`cmd/director/observe{wiring,snapshot,registry,cmd}.go`):
- `liveTarget` over the existing `windowref.Tracker` — adds no selector rules of its own, so
  a session cannot be more permissive than an ordinary command.
- `liveSampler` over the existing collector and fusion, pinning the validated window for the
  cycle. No session-specific perception, no second fusion.
- `buildSample` — the privacy and identity narrowing: no ElementID, no handle, no desktop
  coordinates, no raw OCR. Geometry is window-relative so it still compares after the game
  moves monitors.
- One-at-a-time registry, bounded to 10 finished sessions **in memory only** — a restart
  loses them and the listing says so.
- Protocol `OBSERVE` / `OBSERVATION`, five CLI commands, `--json` throughout.

**Two defects the live run found that synthetic tests could not:**
1. **Presence ratios above 100%** — "present in 925% of samples". Identity is
   role + label + coarse quadrant, so nine indistinguishable icons in one quadrant share one
   identity (intended), but `SamplesSeen` counted OCCURRENCES. Fixed to count samples, with
   `Occurrences` kept separately — the difference is what tells one control from nine.
   Regression test added.
2. **The label classifier kept a real person's name in the clear.** "Chris Haynes Plus"
   passed every rule: three ordinary words, all letters, no marker, high confidence. Shape
   cannot separate a person's name from a control's name, and the earlier "Qovisivre ys"
   case only failed because confidence was low. **NOT FIXED.** It reached terminal output
   only — nothing is persisted yet — but durable storage is the next milestone and this must
   be resolved first. The realistic options are an explicit opt-in for plaintext labels, or
   restricting the clear-text case to control-shaped vocabulary; both are judgement calls
   rather than bug fixes.

**Flag-ordering guard added** (`cmd/director/flagorder_test.go`): five argument orders of
`--application X --duration 3m --interval 500ms --json`, plus an assertion that every
value-taking flag is registered. That table has now failed silently twice.

**Not done:** durable storage, fixture export, game-pack interpretation, protocol event
emission (the runner publishes to `NopEvents` at the composition root), the barrier tests
for blocked-provider responsiveness, and mutation refusal against an observed window.

Gates: `go build ./...`, `go test ./...`, `gofmt` — all passing.

## Director: passive observation runner

**Status: the sampling loop is built and tested (16 tests, fake clock). The service
protocol, CLI, storage and fixture export are NOT.** There is still no
`director observe-game`. Nothing is committed.

**`internal/director/observesession` (NEW)** — the orchestration half. Owns scheduling,
bounds, the state machine, target loss and the OCR budget; owns no perception at all.
Everything arrives as a narrow interface it cannot construct: `Clock`, `Target`, `Sampler`,
`Events`.

- **The passive boundary now covers the runner too**, and both are mutation-verified.
  Adding `_ "internal/platform/wincapture"` to the runner surfaced `wincapture` and
  `internal/screen` and failed the test; adding `_ "internal/oshost"` to the core surfaced
  four packages. A package that receives a `Sampler` can only take samples; one that could
  BUILD one could eventually build something else.
- **Scheduled against intended times, not `sleep(interval)` after the work.** Sleeping after
  a 700ms capture on a 500ms interval drifts 700ms per cycle, so a three-minute session
  silently takes five. Missed slots are skipped and COUNTED rather than queued — a backlog
  would run captures back to back and describe a burst that never happened.
- **A sample exists only when ownership is proven.** The target is revalidated before every
  frame; a failure produces no sample rather than a sample with a caveat. Verified: when the
  target goes, the sampler is never called again.
- **Target loss is bounded and never substituted.** State becomes `target_unavailable`, a
  seam is marked in the timeline, and the session either reacquires (per selector
  semantics) or ends honestly. The reason literally contains "NOT complete", and
  `Result.Complete()` is false — only `Completed` licenses reading conclusions from the
  sample size.
- **Scoped OCR is the exception, not the rule.** First sample always reads; after that every
  Nth, capped per session. From the measured 9.0s for 39 regions, reading every frame would
  spend the whole budget re-reading unchanged text.
- **A relentlessly failing sampler ends the session** after 10 consecutive failures, rather
  than producing a full-length session of silence that looks like a game with nothing on
  screen. An occasional failure is survived and counted.
- **Status answers mid-sample.** The responsiveness test blocks inside the sampler and reads
  a snapshot; if the sampling path held the lock the control plane needs, it deadlocks
  rather than fails, so the timeout is the assertion.

**Two tests failed on first run because the bounds worked** — 3s and 1s sessions were
refused against the 5s minimum. Fixed the tests, not the bound.

**Not done:** the service protocol and its events (Part 10), the CLI commands (Part 11),
the active-session registry and busy policy (Part 8), sanitized storage (Part 12), fixture
export (Part 13), the privacy marker audit across every surface (Part 14), pack
interpretation (Part 16), and live validation (Part 18). The concrete `Target`/`Sampler`
implementations over the window tracker and perception pipeline (Parts 5-6) are also
unwritten — the interfaces exist and are tested against fakes, but nothing implements them
yet. Definition-of-Done items 3-8 are met inside the runner; **1, 2, 9-14 are open**.

Gates: `go build ./...`, `go test ./...`, `go test -race`, `go vet`, `gofmt`,
`GOOS=darwin` cross-build — all passing.

## Director: passive observation core

**Status: the analysis core is built and tested (24 tests). The sampling loop, service
protocol and CLI are NOT.** Nothing is committed.

**`internal/director/observe` (NEW)** — the session model, bounds, safe snapshots, temporal
analysis and hypothesis generation. Pure and deterministic: no platform, no capture, no
clock of its own, so all of it replays from a fixture with no game, detector or OCR.

- **The passive guarantee is a property of the build.** `boundary_test.go` walks the
  transitive import graph and fails if the package can reach an executor, actuator, OS host,
  recorder, driver, Marco runtime, planner, target resolver, `winctx` (activation/focus) or
  `os/exec`. **Mutation-verified:** adding one `_ "internal/oshost"` import surfaced four
  forbidden packages and failed the test. The session cannot click by accident because
  nothing it can reach knows how.
- **Bounds reject rather than truncate.** A three-hour request is refused with the limit
  quoted, not quietly shortened to fifteen minutes. The 500ms default interval comes from
  measured cost — a detection pass ran 170–730ms live, label reading ~230ms per control — so
  anything faster would queue rather than observe.
- **Only `completed` counts as success.** A session cut short by a lost target kept real
  evidence and an incomplete picture; calling it complete would let a reader draw
  conclusions from a sample size nobody told them about.
- **Privacy is opt-in plaintext.** A label is stored in the clear only if it is short,
  mostly letters, confidently read, free of person-shaped markers, and made of
  ordinary-length words. Everything else becomes role + length + digest — enough to observe
  that a label CHANGED without keeping what it said.
- **Jitter is not movement.** A box wandering inside 0.02 of relative width produces no
  transition; a genuine relocation produces exactly one.
- **Evidence and hypotheses are separate types.** Every `Insight` carries supporting
  entities, contradictions and a **required** recommended validation, and every concept in
  the closed vocabulary begins `possible_`. The package cannot emit "pause menu detected".
- Everything bounded and the drops counted: entities, transitions, hypotheses. A silent cap
  would read as "nothing more happened".

**One real gap found by the tests.** The privacy classifier kept
`TTVX-FINAL-SECRET-6b8d-XYZZYPLOV` in the clear — one token, 84% letters, under the length
cap, matching no marker. Whole-string character ratios cannot tell an identifier from a
label; word SHAPE can. Words are now capped at 15 characters and may not mix letters with
digits, which refuses tokens and session keys while keeping "EXIT TO MAIN MENU" and "50%".

**Not done:** the sampling loop itself (Part 5), target-loss and bounded reacquisition
during a session (Part 4), adaptive sampling and the OCR budget (Part 9), the session
analyzer's live wiring, pack interpretation (Part 12), storage and fixture export
(Parts 13–14), the `observe-game` / `observation-session` / `observation-insights` commands
and their protocol events (Parts 15–16), concurrency and responsiveness under load
(Part 17), and live validation (Part 20). Definition-of-Done items 1–3 and 9–14 are open;
items 4–8 are met within the core.

Gates: `go build ./...`, `go test ./...`, `gofmt`, `go vet`, `GOOS=darwin` cross-build — all
passing.

## Director: explicit window targeting

**Status: Part 1 of the passive-observation milestone is complete and proven live. The
observation session itself (Parts 2–9) is NOT built.** Nothing is committed.

**The gap this closes.** Every diagnostic looked at whatever was in front. Running one from
a terminal gives the terminal focus, which is why three separate live attempts at Rocket
League all described VS Code or the terminal instead. Focus is now an input only when
nobody says otherwise.

- **`windowref.Selector`** — one typed selector, exactly one primary of
  `--window-id` / `--window-title` / `--application` / `--process`. Two primaries is an
  error rather than a query needing a tie-break nobody asked for.
- **`Resolve`** returns `resolved` / `not_found` / `ambiguous` / `stale_id` /
  `unusable_selector`, never choosing by enumeration order. Ambiguity asks for a more
  specific selector and names which one.
- **`director windows`** lists current live windows with **ephemeral** ids and no raw
  handles. The header says the ids expire, because an id a person could write down and
  reuse tomorrow would be durable identity by another name.
- **Reacquisition rules differ by selector kind, deliberately:** `--application` MAY follow
  a restart (the executable name is exactly the identity that survives one); an ephemeral
  `--window-id` MAY NOT (it names one generation, and silently attaching to a replacement
  is the durable-identity mistake in a new costume); `--process` cannot outlive its process.
- **`director vision --application X`** captures X whatever has focus, on whatever monitor.
  Proven live: with `dnfc` in the foreground, `--application chrome` returned
  `application chrome, generation 2, offset -1920+0` (the left monitor) and
  `--application code` returned `code, generation 3, offset -967+0`.
- Every targeted capture still runs the full liveness/ownership validation from the
  previous milestone; the selector chooses WHICH window, not whether to check it.

**One fault found by the code's own documentation.** `director vision --application chrome
--json` silently printed non-JSON: `--application` was missing from `cmd/director`'s
`valued` flag table, so "chrome" was reordered behind `--json` and the flag package read the
application as `"--json"`. The table's existing comment warns about exactly this failure,
from a previous occurrence. All the new value-taking flags are now listed.

**Not done — the substance of the milestone:** the bounded passive `ObservationSession`
(Parts 2–3), temporal stability and transition analysis (Part 4), insight/hypothesis
generation (Part 5), pack interpretation of summaries (Part 7), the `observe-game`
diagnostics (Part 8), sanitized session storage and fixture export (Part 9), privacy
redaction (Part 10), adaptive sampling (Part 11), and their tests. The no-input structural
boundary test does not exist yet either. Part 1 was the stated prerequisite and is the only
part attempted.

Gates: `go build ./...`, `go test ./...`, `gofmt`, `GOOS=darwin` cross-build — all passing.

## Director: window liveness + stale-capture prevention

**Status: the reliability gate is closed.** The defect the live Rocket League run exposed —
a destroyed window handle kept, and its remembered bounds captured on another monitor — is
fixed at two independent layers, mutation-verified, and validated live against the real
Windows API. Nothing is committed. Read `docs/director-windows.md` first.

**What was actually wrong.** Two contributors, both needed:
1. `wincapture.CaptureWindow` FELL BACK to the caller's remembered bounds when the live
   lookup failed. A dead window's lookup always fails, so the camera was pointed at where
   the window used to be.
2. `activeWindow` returned the last observed world's window unvalidated — correct for
   choosing a candidate, but nothing asked the platform whether it still existed.

**`internal/director/perception/windowref` (NEW)** — the lifecycle, with no Windows code in
it. `Platform` is the seam, which is what makes destroyed windows, recycled handles and
ambiguous candidates testable at all.
- **Validation asks the platform every time**: window exists, expected process, process
  alive, same application, usable bounds, intersects a monitor. Cached bounds are never
  evidence of any of it.
- **Invalidation is atomic** — afterwards no caller can reach the old handle or bounds.
  There is nothing left to fall back TO, which makes the original failure structurally
  impossible rather than merely guarded.
- **Reacquisition searches by executable name**, never by old geometry, never assuming the
  handle is unchanged. Ranking: foreground → largest visible → **ambiguous**. Two equal
  candidates refuse rather than guess.
- **Epochs**: a generation per distinct window, incrementing on a different handle or
  process but NOT on a window that merely moved. Diagnostics show the generation and
  deliberately **not** the raw handle — a handle invites the across-time comparison that
  caused the incident.
- `valid` is an allow-list of one, so any state added later is refused by default.

**`internal/winctx/liveness.go` (NEW)** — `IsWindow`, `WindowProcessID`, `ProcessAlive`
(via exit code, not just an openable handle), `ProcessImage`, `LookUpWindow`, `LiveWindows`,
plus an `onScreen` test that asks about monitor intersection rather than the sign of X, so a
monitor left of the primary still works. Non-Windows stubs answer "no" — the safe direction.

**The capture-time guard.** `wincapture` no longer has a fallback: no lookup wired, or a
lookup that says the window is gone, is a refusal. Ownership is checked before AND after the
pixels are read, so a window that closes or changes hands mid-capture yields
`window_changed_during_capture` and no frame.

**Two faults found while building this**, both by the checks rather than by reasoning:
- **The first mutation attempts were not caught.** Removing the liveness check left the RL
  regression passing, because the process-exit check masked it; removing the process check
  left the recycled-handle test passing, because an application-name check masked it. Both
  tests were sharpened until each mutation fails on its own — a destroyed window whose
  process survives, and a handle recycled within the same application.
- **Every window reported `generation 0` live.** `Propose` overwrote the validated
  reference, so a proposal was indistinguishable from a fact and the confirm path skipped
  the one place an epoch is assigned. Proposals are now kept separate. Caught by running it,
  not by a test — and now pinned by three.

**Live validation.** `MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/
-run Live` launches a real program, validates its window, kills it and asserts the refusal.
On the machine where the incident happened: `validated: mspaint window, generation 1, at
1536x864+191+107` → `refused, correctly: unavailable — no window belonging to mspaint is
currently available to capture`.

**Not done:** the full Rocket League close/relaunch driven through `director` with a person
in Free Play. The mechanism is proven live and the exact scenario is a unit test, but that
end-to-end run has not happened. Region-scoped label reading is unregressed.

Gates: `go build ./...`, `go test ./...`, `go test -race`, `go vet`, `gofmt`, `GOOS=darwin`
cross-build, and both plugin modules — all passing.

## Director: live vision bring-up + region-scoped label reading

**Status: the first time any of this ran against a real game.** Rocket League, 2026-08-05.
Seven defects found and fixed, one new perception capability added. Nothing is committed.
`docs/director-vision.md` carries the detail; `fixtures/rocketleague/` carries the evidence.

**The model, audited rather than assumed.** `plugins/vision/models/icon_detect.onnx` is
Ultralytics YOLO11m (OmniParser), `names={0:'icon'}` — **one class** — trained on
`som_office`, i.e. office UI. Input `images[1,3,640,640]`, output `output0[1,5,8400]`.
ONNX Runtime 1.26.0 (the binding wants API 26; 1.23 fails loudly). Everything below follows
from that audit, which is why it came first.

**Defects found by running it, all fixed:**
1. **Every detection was labelled `button`.** The plugin's default label list starts with
   "button" and nothing asked the model its class name. `button` maps to a control role and
   `icon` to a weaker one, so a one-class icon detector announced 56 desktop icons as
   controls. Now reads the model's embedded `names`; explicit config still wins.
2. **Grid inference was unreachable.** Candidacy was gated on `ClassSlot` — an inventory
   word no general detector emits — so `gridsOver` returned on its first line on every
   screen. Invisible to synthetic tests, which say `slot` when they mean cell.
3. **`bySize` chained.** Single-linkage from an arbitrary seed merged distinct size
   families. Now sorted by area, so a group spans one bounded tolerance.
4. **No row/column spacing check.** One size family across a screen became one sprawling
   candidate that correctly failed the fill test. Spacing-run decomposition added.
5. **`membersOf` used nearest-line with no distance limit**, so boxes 500px away joined a
   two-row band.
6. **A 4MB bridge line cap killed plugins silently.** An RL frame encodes to ~4.6MB base64
   (game renders do not PNG-compress like desktops). The scanner hit the cap, the loop
   ended, the process exited, and the Director reported "the pipe has been ended" —
   indistinguishable from a crash. Now `bridgehost.MaxLine` (64MB) and overflow is reported
   on both stdout and stderr.
7. **The Director cached a dead window.** After Rocket League was closed and relaunched it
   kept reporting `hwnd:661516` — the destroyed handle — and captured at its stale bounds
   on another monitor. "169 detections on the rocketleague window" were VS Code icons. Not
   yet fixed; see Pending.

**Verified working rather than assumed:** capture of a fullscreen DX11 game; capture at
negative coordinates (a monitor left of primary); the stale-capture guard, which correctly
refused a frame taken while the window moved.

**New: region-scoped label reading.** A detected box is an anonymous rectangle; what makes
it addressable is what it says. `vision.LabelReader` crops each accepted structural
detection, enlarges it, reads it, and attaches the words as the observation's `Label`.
- **Scoped to the box because nothing else works.** On the live pause menu: whole-frame OCR
  read 1 string of 12, whole-frame at 2×/3× read 2, tiling read 151 and 127 mostly
  hallucinated, per-box read **4 of 4 exactly**. An engine pointed at arena texture invents
  glyphs — that negative result is what forced the design.
- **A label is a property OF a control**, attached to its detection, never a second
  observation at the same geometry. The grid-position lesson, applied again.
- **Two filters, not interchangeable:** shape rejects symbol soup; confidence rejects
  letter-shaped nonsense like `"Qovisivre ys"`, which no shape rule can catch.
- **Edge-trimming was written and reverted** — it rescues `"»)  (ee i"` as `"ee i"`, and was
  unnecessary because engines report words and border marks arrive as their own
  low-confidence spans. The note is kept in the code so the next person does not redo it.
- **Bounded:** 39 boxes cost 9.0s unbounded; `MaxLabels` (24, largest first) plus a minimum
  box size brings it to 3.7s, and `director vision` reports what it skipped.
- `capture.Scale` is a hand-written bilinear upscaler (the engine takes no deps), verified
  on real Rocket League pixels: tesseract reads all four button labels from its output.
- The providers never learn about each other; the `ocr.Engine` adapter lives at the
  composition root (`cmd/director/labelreader.go`).

**What this model can and cannot do**, which is the milestone's actual answer:
- **In play: it cannot help.** Free Play yields *zero* detections at the 0.25 floor; at 0.05
  exactly one box — the boost gauge, correctly located — at **0.13**. OCR reads nothing:
  Rocket League draws "33" as disconnected horizontal bars, unreadable at three PSMs even
  cropped and 3× upscaled. No threshold fixes this.
- **In menus: it works.** The pause menu's four buttons at 0.59–0.64 on defaults, all four
  labels read exactly.

**Not done:** the Rocket League capability pack (game state, HUD entities). Stopped
deliberately before it, on the user's call — region-scoped OCR was the prerequisite and is
now in.

Gates: `go build ./...`, `go test ./...`, `go test -race`, `go vet`, `gofmt`, and both
plugin modules — all passing.

## Director: vision perception provider

**Status: complete off the desktop, never run against a live detector.** The Director can
now accept observations produced from IMAGES instead of accessibility, under the same rules
every other provider obeys. **No model has been loaded and no screen has been detected in.**
Nothing is committed.

Read `docs/director-vision.md` first — it carries the design, the thresholds and the Known
gaps.

- **Vision produces observations; fusion produces belief.** Vision may never execute,
  resolve a target, choose an action, verify an outcome or bypass fusion. Those are not
  conventions: the provider lives in `perception/providers` beside accessibility and OCR,
  and `perception_boundary_test.go` already forbids anything outside
  `internal/director/perception`, `internal/recorded` and `cmd/director` from seeing an
  observation at all.
- **`internal/director/perception/capture` (NEW)** — the frame and its transform back to
  desktop coordinates, hoisted out of OCR so both providers place boxes the same way. OCR
  now aliases it.
- **`.../providers/vision` (NEW)** — `Detector` is three lines (`Detect`, `Model`), so the
  backend is replaceable: OpenCV today, YOLO tomorrow. `provider.go` does capture → detect →
  filter → observations; `grid.go` reads repeated shapes as grids; `frame.go` keeps a
  20-frame log **and never keeps a picture**.
- **Opt-in, always.** `Observe` returns nothing unless the request carries `SourceVision`
  (`observation.WithVision(region)`). A Director with the provider wired but nobody asking
  behaves exactly as it did before it existed.
- **The image goes DOWN; coordinates do not come back UP.** The Director captures and sends
  the frame; the plugin answers image-local; the provider applies its own transform. A
  plugin returning desktop coordinates would be placing observations, which needs the window
  bounds, DPI scale and monitor origin it has no way to get right. `plugins/vision`'s
  `Detect` gained that image mode alongside its original capture-it-yourself mode.
- **Vision never fabricates actionability.** Seeing *Delete* does not produce a Delete
  button. `element()` leaves `Enabled`/`Visible`/`Focused` **nil** — vision cannot know any
  of the three. Structural classes carry a HIGHER confidence bar (0.50) than text (0.35),
  because "there is a button here" is a stronger claim. `TestTextDoesNotBecomeAButton` was
  verified to catch a real regression (weakening the check to `if false && !class.Structural()`
  fails it).
- **The safety actually rests on fusion rank + policy, and the doc says so.** Fusion ranks
  vision at 2 (OCR 3, accessibility 6) and defaults an *unreported* `Enabled` to true — a
  deliberate pre-existing decision, so the carefully-preserved nil does not survive into
  belief. The doubt is carried by `Confidence` and `Sources`, which is what policy reads.
  `TestAVisionOnlyWorldIsNotSafeToActIn` proves the gate, against a control world of
  identical role composition seen structurally.
- **Grids are geometric and unnamed.** ≥4 cells, ≥2 rows and columns, sizes within 25%,
  alignment within 40%, 0.6 regularity → `grid`/`grid_row`/`grid_column`/`grid_index`
  attributes. Palworld's interpreter reads them without knowing vision produced them.
- **Diagnostics lead with what was REFUSED.** `director vision [--region] [--last]`,
  `director explain vision`, `director frames`; unknown classes are marked `?` because a
  model this build cannot speak to looks exactly like a model that found nothing. Protocol:
  `RequestVision`. All control plane. With no detector installed — the ordinary case — it
  names why and points at `$DIRECTOR_VISION`.
- **The bridge seam is tested, and testing it found a silent killer.** `decode` and
  `errText` marshalled a `runtime.Value` DIRECTLY; its fields are unexported, so the
  payload came out `{}`, unmarshalled into the detection struct without complaint, and
  yielded **zero detections and no error**. Against a live plugin the provider would have
  reported an empty screen forever and every diagnostic would have agreed. Fixed to go
  through `runtime.JSONFromValue`, as `ocrclient` already did.
  `internal/platform/visionclient/visionclient_test.go` (10 tests) now covers encode →
  call → decode, and holds the three distinguishable failures apart: no plugin, a plugin
  that failed, a plugin that answered with nothing.
- **Five faults the checks caught while building**, all fixed: the marshal bug above; grid
  position emitted as a
  second same-source observation produced two elements per cell (fusion deliberately never
  merges same-source observations — `TestSameSourceNeverMerges`), fixed by running the grid
  pass BEFORE building observations; two fusion tests asserted the wrong thing (OCR outranks
  vision, so OCR wins disputed bounds; incompatible roles do not merge at all); and the
  policy test passed for the wrong reason twice (a hand-built world had zero stored
  confidence; a buttons-only fixture tripped the coverage gate instead of the source gate).

Gates: `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...` (modulo the
four known Windows LL-hook `unsafe.Pointer` notes) and `gofmt` — all passing.

**Next:** the live validation, which is now the same run for all four outstanding
milestones. Build `plugins/vision` with a model, set `$DIRECTOR_VISION`, put Palworld in
front, and check in order: `director game` detects it, `director vision` sees boxes at all,
`director vision` finds the inventory as a GRID, `director explain inventory` turns cells
into entities, and `director observations` shows vision contributing beneath whatever
structure exists. The honest expectation is that step 3 is where it breaks.

## Director: game capability framework

**Status: complete off the desktop, never run against a game.** Games are capability packs
rather than Director special cases; the first pack (Palworld) registers, detects,
contributes and expands through the ordinary pipeline. **Nothing has been run against a
running game.** Nothing is committed.

Read `docs/director-games.md` first — it carries the design, the arithmetic and the Known
gaps.

- **The claim is checkable.** Adding a game is one line in `registeredPacks()`
  (`cmd/director/gamewiring.go`) and nothing else; `internal/director/boundary_test.go`
  forbids `internal/director` importing `internal/gamepacks`, so it is a property of the
  build rather than a discipline. Same shape as the platform adapters.
- **`internal/director/game` (NEW)** — the `Capability` interface, registry, detection,
  inventory semantics, conditions, safety policy and the diagnostics surface. There is no
  execution in it: a pack contributes six kinds of thing and every one plugs into an
  extension point that already existed (interpreters → enrichment, procedures →
  `goal.Registry`, roles → the alias table, conditions → the wait engine, verifiers →
  `verify`, policies → `policy`).
- **Packs reason over BELIEF, not evidence.** The first design had packs contributing
  `observation.Provider`s; `perception_boundary_test.go` correctly refused it — only
  `internal/director/perception` may see observations. A pack now contributes an
  `Interpreter` over fused ELEMENTS, applied by `Registry.Enrich` after fusion. A pack
  therefore cannot capture a screen or contribute evidence nobody weighed. A pack needing a
  real new source contributes an ordinary perception provider at the composition root.
- **Core hooks added (the Director gains extension points, not game knowledge):**
  `directorapi.EntityIdentity` carried observation → fusion → element (mirrors
  `ResourceIdentity`); `verify.EvidenceSource` (additive, attributed, capped at 0.7, and an
  inconclusive verdict it does not rescue STAYS inconclusive); `policy.Rule` (a `Verdict`
  with `Refuse`/`Confirm` and **no field meaning allow**); `goal.RegisterControlRole` so a
  pack names controls by meaning and gets the ordinary alias matching.
- **Quantity is a pointer, deliberately.** An empty slot holds zero; an unreadable slot
  holds an unknown number. `Inventory.Full()` returns *(full, known)* and every selection
  reports what it skipped — "deposit everything" that silently left four things behind is
  the failure this prevents.
- **Safety is structural, not a filter.** The `Automation` vocabulary has no value for
  combat, aiming, movement or player interaction, so a pack cannot permit one. `Protected`
  refuses absolutely; nothing-permitted refuses; `Competitive` confirms every action.
  `game.Procedure` pairs a procedure with the automation it declares, and registration
  REFUSES a pack shipping automation it did not permit — the Director does not start.
- **Detection combines four signals** (process 0.40, title 0.35, interface 0.60, the pack's
  own entities 0.70), combining probabilistically past a 0.60 threshold: no single weak
  signal reaches it, any two do. A tie between packs detects NOTHING.
- **Two general outcomes arrived with the first pack:** `goal.Sort` and `goal.Craft`, with
  generic procedures and built-in `sort_command`/`craft_command` roles. Neither is a game
  concept — that is the test for vocabulary-versus-pack, and the pack is what surfaced it.
- **Diagnostics:** `director game`, `director capabilities`, `director explain game`,
  `director explain inventory`, `marco games`; `director procedures` lists the pack's.
  Protocol: `RequestGame`. All control plane — none takes the command lock.
- **Three faults the checks caught while building**, all fixed: two Palworld procedures
  collided on `open_settings` (the registry validator — which is what led to `Sort`/`Craft`);
  the pack-as-perception-provider design (the perception boundary test); and detection
  weights that did not deliver what their own comment claimed (0.35+0.30 = 0.545, below the
  threshold the comment promised two signals would clear).

Gates: `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...` (modulo the
four known Windows LL-hook `unsafe.Pointer` notes) and `gofmt` — all passing.

**Next:** the live validation — run `director serve` with Palworld in front and check
`director game` detects it, `director explain inventory` sees anything at all, and a
procedure resolves. The honest expectation is that Palworld exposes no accessibility tree,
detection falls back to process+title, and the interpreter sees nothing — which is the
result that tells you the next piece of work is a vision provider.

## Director: demonstration recording + semantic procedure extraction

**Status: complete off the desktop, unproven on it.** The user can demonstrate a task once
and the Director extracts a reusable procedure from the SEMANTICS of what it verified —
recorded, extracted, explained, approved and registered alongside the built-ins. Every part
is regression-tested against fixtures and against the real pipeline. **No demonstration has
been performed on a live desktop.** Nothing is committed.

Read `docs/director-demonstrations.md` first — it carries the design and the Known gaps.

- **`internal/director/demo` (NEW).** The whole layer: `Demonstration`/`Step` (semantic
  only — verbs, semantic targets, waits, evidence KINDS, and action-graph node
  REFERENCES), `Recorder`, `Extract`, `Validate`, `Unsafe`, `Explain`, `Learned`, `Store`.
- **Recording adds no observation path.** The recorder's one subscription is
  `Runtime.Handle`'s own outcome — a request already observed, executed, verified and
  recorded. It observes nothing, so "recording never bypasses verification" is structural:
  there is nothing to record until an action has been verified.
- **Nothing mechanical survives**, asserted over the serialised form rather than by
  inspection: no coordinates, handles, native/element/runtime ids, screenshots or OCR, and
  no value the value layer called sensitive. A CLICK is recorded as an INVOKE — the same
  act at the level a procedure is made of.
- **The goal comes from the ACTIONS, never the phrase** (`signatures`): 13 of the 15
  outcomes have recovery signatures; `delete` and `close_without_saving` are deliberately
  absent because safety refuses every demonstration of them first. `copy → paste` is a
  DUPLICATE. Two signatures that fit equally well REFUSE.
- **One parameter rule: a value becomes a parameter when the user TYPED it.** Four
  exceptions keep typed text constant (application name, control name, procedural verb,
  empty). The SUBJECT is generalised too — the first step aimed at a content element
  becomes "the object the user points at", which is what makes the learned procedure answer
  "rename this file to Q4".
- **Two refusal layers, at two moments.** Safety at session CLOSE (may this EVER be
  learned: credentials, auth, payment, destructive controls, bulk, confirmation, unverified,
  cancelled, clarified) — a durable fact, not a verdict to be re-reached later. Validation at
  extraction (does this DESCRIBE a procedure). Both refuse the whole demonstration.
- **Approval is a step nothing skips.** `Extract` returns an `Extraction`; the registry
  takes a `goal.Procedure`; only `Approve` bridges them, and the type is the gate. The
  service re-runs the extraction rather than accepting a candidate from a client, so
  approval cannot become a way to author procedures.
- **A learned procedure is an ordinary procedure.** `AsProcedure` is an adapter over stored
  DATA — it evaluates nothing. It expands, validates, binds, confirms, lowers to Marco and
  verifies by the built-in path because it IS the built-in path. The demonstrated value is
  never typed again: the procedure declares the requirement its parameters imply, so a
  rename with no new name is refused BEFORE expansion as a typed question.
- **Precedence is PROVENANCE, not specificity** (`goal.Procedure.Learned`,
  `onlyLearned`): between two procedures constraining the same amount, the one demonstrated
  here wins over the shipped one. Two LEARNED procedures stay ambiguous. Registry
  validation exempts the learned/built-in pair so teaching the Director something it
  already knew cannot stop the service from starting.
- **Every decision is recorded where it fires** and stored WITH the approved procedure, so
  `explain procedure` answers months later with the reasoning the user approved.
- **Wiring:** `demo.Recorder`+`demo.Store` on the daemon (control plane — no `r.mu`, so
  they answer while the command they are recording runs); `RequestDemonstration` protocol
  action; `director demonstrate start|stop|abandon|status`, `demonstrations`,
  `demonstration <id>`, `extract <id> [--why|--approve]`, `explain procedure <name>`;
  `director procedures` marks learned ones `*`.
- **The round trip runs in CI** (`roundtrip_test.go`): a demonstration driven through the
  REAL pipeline — observe, resolve, plan, policy, execute, re-observe, verify, record —
  recorded, extracted, approved, registered, and expanded again for a DIFFERENT file with a
  DIFFERENT name.
- **Two extraction rules the round trip forced**, both now stated in the docs: rename is
  recovered from ordering (rename command invoked, then text typed) rather than from "the
  text went into the inline editor", because a verified action record carries no control
  CLASS; and destructiveness is read from the control ROLE, not from verb reversibility —
  the latter calls invoke/paste/submit irreversible and refused every demonstration.

Gates: `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...` (modulo the
four known Windows LL-hook `unsafe.Pointer` notes) and `gofmt` — all passing.

**Next:** the live validation in the milestone — demonstrate a rename in real Explorer,
extract, approve, and run the learned procedure on another file; then create-folder and
duplicate. That needs a desktop and has not been attempted.

## Director: Explorer rename-editor modelling

**Status: complete off the desktop, unproven on it.** The inline-editor model, its pipeline
wiring, the graph evidence, the diagnostics and the docs are all done and regression-tested
against fixtures built from the real captured Explorer tree. **The live run has still never
executed since the wiring landed** — that is the one thing left, and it is the only way the
remaining claims get proven. Nothing is committed.

Gates as of now: `go build ./...`, `go test ./...`, **`go test -race ./...`**, `go vet
./...` (modulo the four known Windows LL-hook `unsafe.Pointer` notes) and `gofmt` all pass.

### What this session finished

- **Part 14 — pipeline regressions** (`internal/director/execute/editor_test.go`, 11 tests
  over an Explorer fixture that CONTAINS the decoy): rename mode acts on the bound file ·
  the text goes to the verified editor · text that did not land fails the step · a commit is
  aimed at the open editor and proved by its closure · a commit leaving it open fails · an
  absent editor stops without reaching the host, the resolver or the graph · two editors ask
  · the derived pin does not escape the request · the node records the edit and no way to
  re-find the control · replay re-derives · the diagnostics account for it.
- **Two real defects the regressions found**, both in code written and never run end to end:
  1. **`directorapi.ElementQuery.Constrained()` did not count `NativeID`**, so the one way a
     derived target can be expressed was refused as "not specific enough to look for" — the
     editor step could never have resolved.
  2. **A commit re-applied the discovery-time value check** and refused the very editor it
     was about to commit (the box no longer holds the original name by then). New
     `inline.FindOpen` drops clause 5 for a commit only; `deriveEditor` picks it via
     `isCommit`.
- **Part 11 — graph evidence.** `actiongraph.ActionNode.Editor *inline.Snapshot`, stamped by
  `noteEditor`. Carries property, resource, class, initial/final value and evidence, and
  **no element id or native id** (asserted over the serialised form). `unpinEditor` also
  takes the pinned native id back out of the node's stored plan, and
  `actiongraph.analyzeElement` now reports a derived-editor node as deriving its target at
  run time instead of `TARGET_MISSING` — before that, such a node was unreplayable on the
  strength of a handle nobody should have been keeping.
- **Part 15 — diagnostics.** `BindingDiagnostics` gained `EditorOutcome` (the post-action
  verdict, kept apart from the derivation), `Describe()` renders the editor with its
  evidence/missing/candidates, `Empty()` accounts for it, and `director graph <node>` grew
  an **Edited** section plus a "target: derived at run time" line. `attempt` now takes the
  diagnostics (nil from replay, which reports its own way).
- **Part 16 — docs.** `docs/director-goals.md` has a full **The inline editor** section (the
  decoy, the captured tree, the five correlation clauses and where clause 5 is dropped,
  derived-target vs binding, verification, what history keeps, the two defects, coverage),
  new status-table rows, and rewritten Known gaps.

### The finding this milestone rests on

Captured from a real Windows 11 Explorer window, before and after invoking the command-bar
Rename button (element count identical, 166 → 166 — one control replaces another):

```
ClassName    UIRenameTextElement     ← exists ONLY in rename mode; the marker
ControlType  ControlType.Edit
AutomationId ""                      ← none, so the class is the only handle
Value        "Alpha.txt"             ← the item's current display name
Focused      true
parent       UIItemsView ("Items View")   ← a SIBLING of the row, not a descendant
```

At the same moment the selected row's Name cell (`AutomationId System.ItemNameDisplay`,
`ClassName UIProperty`) goes **empty** — the editor replaces its presentation.

**The decoy that broke the previous run:** a details view has an Edit control per column
per row. The selected row's Name cell is an Edit with value `Alpha.txt` and a ValuePattern.
The old step targeted "whatever holds focus", wrote `Budget` into that cell, verified
successfully, and renamed nothing. Any rule matching on *contents* picks the decoy.

Other observed facts worth keeping:
- Rename mode **exits when the window loses foreground**. Observing does not disturb it
  (checked); anything that steals focus does.
- The editor's live `ValuePattern` reported *Unsupported* from PowerShell even though the
  cached walk reports `value`. The edit ladder's fallbacks (select-all + type) are the
  expected path; this has not been exercised live.
- Extensions were SHOWN on this machine — the editor opened containing `Alpha.txt`, so
  replacing with `Budget` yields `Budget` (no extension). The live test accepts either
  `Budget` or `Budget.txt`; a deterministic expectation should be derived from the
  editor's observed initial value.

### What is implemented and unit-tested

- **`internal/director/inline`** (new): `Editor`, `Property`, `Result`
  (verified/mismatched/ambiguous/absent/unverified), `Verification`, `Snapshot`.
  `Find` correlates on: known editor CLASS (`editorClasses` table) · same window · exactly
  one · the bound item still selected · initial value matches the item's name (extension
  optional). `VerifyValue` re-finds by native id and compares. `VerifyClosed` is explicitly
  only half the commit check and says so.
  Tests cover: real editor found · Name cell refused · search box refused · address bar
  refused · distractor editor refused · two editors ambiguous · other window ignored ·
  selection moved refused · container-focus handled · value verified/wrong/closed/replaced ·
  hidden extension · snapshot keeps no way to re-find the control.

### The pipeline wiring (now regression-tested — see "What this session finished")

- `directorapi.ElementQuery.NativeID` — exact match, set only by a step that derived the
  control in the same request. **`Constrained()` now counts it** (it did not, which is the
  first of the two defects above).
- `directorapi.ReferenceExpression.RequiresEditor` — "the editor for the thing I bound".
- `goal.Directive.TargetEditor`; `explorer rename` uses it for **set text** and **confirm**.
- `execute/editor.go` — `deriveEditor` (request-local, pins the reference by native id;
  `inline.FindOpen` for a commit), `verifyEditorValue`, `verifyEditorClosed`, `isCommit`,
  `statusForEditor`, `unpinEditor`.
- `prepare()` derives the editor before planning and STOPS if it cannot.
- `attempt()` adds `inline_editor_value` and `inline_editor_closed` evidence, fails the step
  on either, and records the verdict on the diagnostics.

### Not done

- Part 8 per-step verification classification beyond the above.
- **The live run.** Three attempts before this session all skipped **before any input**
  because an app titled `DNFC` held the foreground and the harness would not steal it. It
  has not been attempted since; **nothing in this milestone is live-tested.**

### Gates

`go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...` (modulo the four
known Windows LL-hook `unsafe.Pointer` notes) and `gofmt` — all passing, full sweep re-run.

**C# bridge: rebuilt successfully, no warnings** (`built D:\Macros\marco\plugins\uia\uia.exe`).
`Shell.cs` was added to `build.ps1`'s source list in the previous milestone; nothing in the
bridge changed this milestone.

### Housekeeping

One Explorer window is **left open** on a temp folder
(`...\TestLiveExplorerRename3141286802\001\marco-live-26180-...`). The harness's
`winctx.CloseTitle` posted WM_CLOSE and the window did not close within its 1s window,
probably because it was behind the foreground app. `explorer.exe` was never touched.

### Next step

Run the live scenario on a desktop with no fullscreen app holding the foreground:

```sh
go build -o "$TEMP/director.exe" ./cmd/director
MARCO_LIVE_VALIDATION=i-understand-real-input-will-occur \
MARCO_DIRECTOR_BIN="$TEMP/director.exe" \
MARCO_UIA_BRIDGE="$PWD/plugins/uia/uia.exe" \
  go test -tags livevalidation -count=1 -run TestLiveExplorerRename -v ./internal/live/
```

Expect the interesting failure to be step 3's *mechanism*: the editor may not accept
`SetValue`, in which case the edit ladder must fall through to select-all-and-type. That
path has never run against this control. Everything ahead of the mechanism is now
regression-tested, so a live failure should be readable: the trace carries an **editor**
stage saying which control was accepted and why, and the step fails rather than reporting
success if the text lands somewhere else.

## Director: Explorer shell-item identity

The blocker from the previous milestone is closed: the bridge now surfaces a **canonical
path** for the selected File Explorer item, and it flows all the way to the binding. The
live rename is **still not passed** — read `docs/director-goals.md` ("Status of each
claim") before claiming otherwise.

- **`plugins/uia/Shell.cs`** (new, and `build.ps1` now compiles it). Uses
  `IShellFolderViewDual` via the shell's window list, matched by HWND: it gives the view's
  current folder and its selected items, each with `Path`, `IsFolder`, `IsLink`,
  `IsFileSystem`. Late-bound through reflection, so the build is still one `csc.exe` call.
  **Fails closed** on: not-Explorer, no view, virtual location, no/multi selection,
  non-filesystem item, uncanonicalisable path, path not directly inside the view's folder,
  vanished item, shell/filesystem kind disagreement, tree/shell selection-count
  disagreement, caption mismatch. A path is **never** assembled from folder + caption.
- **`directorapi.ResourceIdentity`** (new): kind, path, parsing name, display name,
  source, confidence, link, evidence. No handles, no PIDLs, no coordinates — a test
  asserts that over the serialised form. Carried on `Observation` and `Element`, both as
  omitted-when-absent pointers so an older bridge decodes unchanged.
- **`binding.classify`** consults it first, so kind comes from the shell: a folder called
  `Reports.txt` is a folder, and a file called `Reports` is a file. Re-identification uses
  the same path (`resourcePath`), and a native-id match whose resource DIFFERS is now
  refused — that is a view that navigated, reusing element ids.
- **`winctx.ActivateTitle` / `CloseTitle`**: bring forward and politely close ONE window by
  title, refusing an ambiguous match. `CloseTitle` posts WM_CLOSE. Neither ever terminates
  a process; `explorer.exe` is never killed.
- **Diagnostics**: `director observations` prints the resource, its source, confidence and
  evidence under the observation it belongs to. The live harness reads the same
  diagnostics to confirm the observed path equals the expected one **before** submitting.

- **Two procedure defects, found live and fixed.** `explorer rename` opened the context
  menu (a Windows 10 assumption — Win11 has a command-bar Rename button, and a context menu
  is its own top-level window the Director never walks). And a step naming no control
  reached the resolver with an empty query: an edit with no target now means "the field the
  previous step opened", a targetless verb means "the focused control", both ANAPHORIC and
  needing no binding.

Live status: **not passed.** Binding resolves live with the real shell path. Of the four
rename steps, 1 (select) and 3 (set text to "Budget") verify against real Explorer, and 2
(invoke Rename) verified once and was unverified on a later run — the Invoke lands, but the
focus change is not always observed inside the settle window. Step 4 has not been reached.
No rename has completed; no filesystem verification has passed. **Next: step 2's
verification timing**, not its execution.

## Director: runtime activation + first live run

The Director now confirms, verifies and runs end to end from the daemon — and the scenario
has been run against a real desktop. **It is not live-tested**: real input occurred and the
rename did not happen. Read `docs/director-goals.md` ("Status of each claim") before making
any claim about what is validated.

- **The daemon installs a `Confirmer`.** `service.ConfirmationBroker` implements
  `execute.Confirmer`, is built in `NewRuntime` and wired into the pipeline. It publishes
  `CONFIRMATION_REQUIRED` to the client watching the command and blocks for a `CONFIRM` on
  another connection. `director confirm` shows the question with no argument and answers it
  with `yes`/`no`. Nobody listening → `unavailable` → blocked. 90s timeout → the same.
  Abandoned → `cancelled`. **No configuration returns yes without a person.**
- **Verification runs automatically.** Per action: the object acted on must be the object
  bound (`verify.CorrelateTarget`), attached to the graph node as `binding_correlation`
  evidence, a demonstrated mismatch failing the record. Per goal: `verify.CorrelateRename`
  against a filesystem `Inspector` (`osResources` in the daemon), prepared *before* the
  program runs because the content and the sibling list are only knowable then.
- **`Runtime.Windows()`** — new, because nothing let a client ask which windows exist.
  Observes on demand under a `TryLock` when its last look is stale.
- **Live harness moved to `internal/live`** (build tag `livevalidation`). It may import
  platform code — it arranges the scene — and `internal/director/boundary_test.go`
  correctly forbids that inside the Director.
- **Two production defects found by running it**, both fixed with regressions:
  `HandleRequest` never reached the goal layer for a single-clause request (so "rename this
  file to Budget" was parsed as a *variable* rename); and a selected file was unreachable
  when the container held keyboard focus.
- **Remaining blocker:** `plugins/uia` does not surface a shell item's parsing path, so an
  Explorer list item has no backing resource and the binding layer refuses — correctly.
  That is the next piece of work, and it is bridge work.

To run it:

```sh
go build -o "$TEMP/director.exe" ./cmd/director
MARCO_LIVE_VALIDATION=i-understand-real-input-will-occur \
MARCO_DIRECTOR_BIN="$TEMP/director.exe" \
MARCO_UIA_BRIDGE="$PWD/plugins/uia/uia.exe" \
  go test -tags livevalidation -run TestLiveExplorerRename -v ./internal/live/
```

## Director: runtime binding enforcement + universal confirmation

The binding layer is now **in force at run time**, and action-level confirmation exists.
Precise status lives in `docs/director-goals.md` ("Status of each claim") — read that
before making any claim about what is validated. **No live scenario has ever run.**

- **`internal/director/binding`** grew a request-scoped `Store` (one mutable binding per
  request, on the `context`), a durable `Snapshot` for the action graph, and `Origin`
  provenance. `ReferenceExpression` gained `RequiresBinding` / `BindingID` /
  `ExpectedKind` — the handle, not the binding, because a reference is copied into a
  plan, a record and a node while revalidation *refreshes* the binding.
- **`goal.Expand(r, g, binder)`** resolves deictic directives through the binder and
  attaches the binding to the step's reference. `goal.Plan(r, g)` is the display-only
  form used by `explain goal` and `--dry-run`: it binds nothing, so the program it makes
  is refused by `program.ValidateBound` and cannot run by accident. Procedures declare
  `Expect binding.ObjectKind`; one that points at something without declaring a kind is
  refused at expansion.
- **Three guards**: expansion, `ValidateBound` before step 1, and `requireBinding` per
  step. `TestTheUntypedFocusFallbackIsUnreachable` sweeps every procedure.
- **Revalidation** happens in `prepare`, after the plan is built and *before* the policy
  and confirmation gates, against the world that step already observed. Refused → nothing
  reaches the host, and the binding is never re-pointed at what is focused now. Both the
  guard and the refusal were verified to FAIL against unfixed code.
- **Confirmation**: one `Confirmer`, scopes `goal`/`action`/`replay`, closed outcomes
  `not_required`/`accepted`/`rejected`/`unavailable`/`cancelled`. Coverage rules live in
  `coveredByGoalConfirmation` (risk, destructiveness, concrete resource, material binding
  change) and are stored on the request `context`, never on `Pipeline`.
- **Replay**: stored confirmation is disclosed and never reused; an irreversible repeat is
  confirmed afresh; a deictic node re-establishes its object from the stored snapshot by
  resource then native id, and is refused if it cannot.
- **`verify.CorrelateRename`** ties a result to the bound object (destination exists AND
  original gone AND content preserved AND distractors untouched). *Structurally tested*
  against fixtures; not yet called from the rename path.
- **The shipped service wires no `Confirmer`** — `Runtime.SetConfirmer` is the point, and
  the daemon leaves it nil, so anything needing confirmation is BLOCKED rather than
  silently allowed.

## Pending / known issues / open decisions

- ~~**The Director caches a dead window handle.**~~ FIXED — see the window-liveness
  section above and `docs/director-windows.md`. Validation, reacquisition, epochs and a
  capture-time guard; mutation-verified and confirmed live against the real Windows API.
- **Perception cannot be pointed at a non-active window.** `--region` narrows what is
  LOOKED AT within the captured active window, not what is captured (the OCR diagnostic
  prints "desktop coordinates", which reads as though it does). Live game work needs a
  terminal, taking focus means the game is not active, so a `--window <id|title>` target
  would remove a whole class of confusion. Deliberate design today; a gap for game work.
- **`visualstate` and `vision` share `SourceVision`.** Distinct provider names, one source
  identity. `rt.visual` is not in the collector today so nothing collides, but any request
  including `SourceVision` would activate both, and fusion never merges same-source
  observations — duplicate elements for one thing. Latent.
- **Rocket League needs a different detector.** `icon_detect` covers menus and overlays and
  cannot see the in-play HUD at all. A pack built on it can answer "is the pause menu
  open" and must not claim to know the boost value.

- **Director control plane blocked behind desktop work:** ~~`ActiveValues`,
  `ActiveCollections` and `AbandonProgram` read the paused program under `Runtime.mu` —
  the lock `Handle` holds for the ENTIRE duration of a desktop command — so
  `director status`, `director collections` and `explain value` all hung until the
  running command finished. Worst in the common case: a program WITH collections set
  `activeCollections` and returned early, so it was the ordinary command with no
  collections that hung status.~~ **Fixed** — `paused` now has its own `pausedMu`
  (command path takes `mu` then `pausedMu`; the control plane takes `pausedMu` alone).
  `cmd/director/lockrule_test.go` grew a second, behavioural guard and its textual scan
  now FOLLOWS same-receiver helper calls — the bug hid behind a delegation
  (`ActiveCollections` mentions no lock; the `mu` was one call down in
  `liveCollections`). Both guards were verified to fail against the unfixed code.
- **`director status` never rendered its collections section:** ~~`renderActiveCollections`
  was written and the server already put the snapshot in the status payload, but
  `printStatus` never called it.~~ **Fixed** — wired, and silent when nothing holds a
  collection.
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
OCR/Vision resolvers can still label it); `off` = plain coordinates, no anchors/resolvers.
**CV IS NOW OFF BY DEFAULT** — a feature flag, because the anchor stack isn't reliable on real
game UIs yet; **unset = off** (was: anchors on). Turn CV on with `MARCO_CV=on`/`max` or
`MARCO_ANCHORS=1/on` (`setup.ps1 -CV`). Run-time scoring stays conservative either way (kitchen
sink captures MORE signals, never clicks less safely). The one flag to A/B the whole CV
stack; re-run `setup.ps1` + relaunch `overlay.cmd` to flip. Tests: `recorder`
`cv_windows_test.go`.

**`MARCO_CV=off` is a first-class ALPHA mode: pure coordinates + recorded timings, no CV.** On
real game UIs the anchor wait-gate was the failure mode — it hovers, polls for a match that
never confidently resolves, and on timeout clicks the recorded coordinate anyway, by which time
the menu has moved → "random" clicks. So `off` now fully scraps CV at BOTH ends: teach captures
plain coordinates (recorder), AND (1) `orchestrator.teachOptions` drops the 50ms wait cap
(`MaxWaitMs`→~∞) so the ROUTE carries the REAL recorded pauses as `Sleep` steps (the pacing the
CV poll used to provide), and (2) `oshost.doFind` short-circuits to the recorded coordinate
immediately (no hover, no poll) — so existing anchored routes stop waiting too, no re-teach
needed to un-hang them (though re-teaching gives the proper timings vs the old capped 50ms).
Gate: `cvOff()` (`$MARCO_CV=off` or `$MARCO_ANCHORS=0/off`), defined in both packages. Teach at
the game's real pace — the pauses you make become the replay waits. Tests: `oshost`
`TestFindCVOffClicksRecorded`.

**`marco edit "<route>"` — a web editor for a route's TIMINGS + CLICK COORDINATES**
(`cmd/marco/edit.go`), the companion to CV-off mode (where waits + coordinates ARE the whole
route). Self-contained: `net/http` is stdlib, so it holds the zero-dep rule (no plugin needed).
It resolves the route (`Reg.Resolve`, falling back to `findRouteByName` across scopes), reads the
`.marco`, serves a local editor page (auto-opens the browser), and writes edits straight back via
`Reg.Save`. The page shows the top-level action/wait sequence (`parseSteps`: `do …` body lines;
nested Find arms + decls hidden): each `Sleep` is an editable ms input, and each click/move at a
named Point gets editable x,y inputs (its coords, looked up from the Point decls via
`parsePoints`), plus the full source in a `<details>`. Save posts `{waits[], points{name:[x,y]}}`;
`applyWaits` rewrites the Nth `Sleep with N.` (`sleepRE`) and `applyPoints` rewrites each edited
Point's X,Y (`pointDeclRE`), shifting its window-relative `RelX,RelY` by the SAME delta so both
stay consistent. Tests: `cmd/marco` `edit_test.go`
(`TestEditParseSteps`/`TestEditApplyPoints`/`TestEditRebuild`); verified end-to-end via the local
server. NOTE: coord editing attaches to top-level `Click/Move with pN` steps (CV-off routes); an
anchored route's top-level `Find` steps carry their coords on the anchor/fallback — editing those
(anchor `Timeout`/`X,Y`) is the obvious next extension.

**Editor now also DELETES steps and converts a click → DRAG.** The save engine is line-keyed
(`rebuild` + `saveReq{Waits{line:ms}, Points{name:xy}, Deletes[line], Drags{line:[fromX,fromY,
toX,toY]}}`): a `✕` marks a step's source line for removal (an unused Point decl left behind
compiles fine — verified), and a `drag` toggle on a click/move reveals a "to" point and rewrites
that line into a real `Drag` (a `the dragN is a Drag with FromX…ToY…` decl + `do OS's Drag with
dragN`, numbered after any existing drags via `dragNumRE`). Line-keying means a delete doesn't
disturb the other edits. Verified end-to-end: click→drag + delete + wait edit → the route still
compiles.

**Real drag primitive — `OS's Drag` (NEW).** Codegen used to fake a StepDrag as "move-to-start +
click-end" (the old `// TODO drag (no hold primitive)`); now there's a genuine one. `os.marco`
exports `Drag` + a `Drag` set (`FromX,FromY,ToX,ToY,Button`); `oshost.doDrag` reads it and calls
`backend.drag`, which (Windows) presses the button at the start, GLIDES to the end as a trail of
injected MOVE events (so a game registers the drag motion, not a teleport), then releases;
`codegen` `StepDrag` emits `the dragN is a Drag …` + `do OS's Drag with dragN`. Backend interface
gained `drag(ctx, button, x1,y1,x2,y2)` (+ stub + `recBackend`). Absolute coords (window-relative
is a later add). Tests: `oshost` `TestDrag`.

**`MARCO_CV_SENSITIVITY`** (0 = strict … 1 = loose, default 0.5) — the **CV find dial**,
the fine control beside the coarse `MARCO_CV` switch. One knob that scales how eagerly an
anchor is found, trusted, and followed. **0.5 reproduces every legacy value exactly** (each
knob is a symmetric `cvLerp(strict, loose)` centred on its old value; each also keeps its own
env override), so the dial is a no-op until dragged; the LOOSE end is deliberately aggressive
(these are game UIs). It reaches three groups:
  - `oshost` — ACT-ON-evidence gates (`oshost.cvLerp`): `findConfidence` (0.95→0.25, legacy 0.6;
    `$MARCO_FIND_CONFIDENCE`), `locateFloor` (1.0→0.80, 0.90) and `moveMargin` (0.22→0.02, 0.12)
    — how sure it must be to click a found box and to FOLLOW a moved one. PLUS `findTimeoutScale`:
    the short click-gate timeout (≤5s, the 1500ms `DefaultFindTimeoutMs`) stretches 1×→4× above
    neutral so a slower-appearing button gets more time before the route clicks anyway ("it went
    too fast"); long wait-for-screen barriers untouched. Applied at run time in `doFind` → helps
    EXISTING routes.
  - `screen` — FOUND gates + edge detection (`screen.cvLerp`), the fix for "the button is on
    screen but it has no idea": `MatchThreshold` (1.00→0.50, legacy 0.75; `$MARCO_FIND_THRESHOLD`)
    and `edgeMatchThreshold` (0.80→0.30, 0.55; `$MARCO_EDGE_MATCH`) are the pixel/edge locators'
    `Found` gates — below them a present-but-imperfect button contributes ZERO evidence
    (`scoreAnchor` only credits a locator when `l.found`), so no confidence gate could rescue it;
    `EdgeTolerance` (150→50, legacy 100; `$MARCO_EDGE_TOLERANCE`) drives edge matching AND
    AutoCrop/DetectButtons. Run-time gates help existing routes; crop/edge-detect changes need a
    **re-teach**.
  - `screen` — CROP-SIZE gates (`screen.cropScale`, 1× at/below neutral → **4×** at full-loose,
    floored at legacy so a stricter dial never shrinks a crop): the capture-time gates that were
    rejecting HUGE game buttons into the small fallback patch — `maxButtonDim` (600→2400),
    `maxRecenterPx` (140→560), `buttonSearchWin` (480→1920) and the `AutoCropAt` fallback radius
    (×cropScale). Loosen the dial and re-teach → a big button is captured whole ("let the finder
    work its magic"). Capture-time, so **re-teach** to take effect.
Exposed as the overlay config editor's **`cv find`** slider (`config.go` `Sensitivity`, step
0.05); the overlay passes `MARCO_CV_SENSITIVITY` to each spawned `marco` when off-neutral
(`acts.go streamChild`) — run AND teach — so a drag takes effect on the **next command with no
restart** (`marco do` runs the OS host in-process; `screen`/`recorder` vars read the env at
package init of the fresh process). Tests: `oshost` `sensitivity_test.go`
(`TestCVSensitivityMapping`/`FindTimeoutScale`), `screen` `sensitivity_test.go`
(`TestCVSensitivityGates`/`TestCropScaleGates`/`TestGateEnvOverride`).

**Cursor glide (hover-settle).** The find hover no longer TELEPORTS the cursor to the
recorded point (one absolute `MOUSEEVENTF_MOVE`) — a game UI that lights a control only when
the pointer physically travels onto it (RL's menu tiles) never saw the entry. `oshost.glide`
now PULLS the cursor there as a trail of injected MOVE events (`winctx.CursorPos` start →
interpolated steps → land on target), used at the hover-settle in `doFind`. `$MARCO_FIND_GLIDE=0`
reverts to a single jump; the off-Windows stub (`CursorPos`→(0,0)) falls back to one move, so
tests are unchanged. The OS `Move` act stays a single atomic move. Tests: `oshost`
`TestFindHoversBeforeMatching` (asserts the glide LANDS on the recorded point).

**Local LLM resolver + director** (`plugins/llama`): drop-in for `MARCO_RESOLVER` (route
matching) AND `MARCO_ASSISTANT` (the conversational director's Advisor). `MARCO_LLM_URL`
(default `http://localhost:11434/v1`, i.e. local Ollama), `MARCO_LLM_MODEL` (default
`llama3.2:3b`), `MARCO_LLM_KEY` (opt-in cloud, e.g. OpenAI), `MARCO_LLM_TIMEOUT_MS` (20000).
`MARCO_ASSISTANT` (director brain plugin; falls back to `MARCO_RESOLVER`).

`MARCO_ROUTES`, `MARCO_STOP_KEY` (f12), `MARCO_ARG_KEY` (f9; `off`), `MARCO_ANCHORS`,
`MARCO_FIND_HOVER`/`MARCO_FIND_WIGGLE`/`MARCO_FIND_GLIDE` (0 disables), `MARCO_FIND_SETTLE_MS` (150),
`MARCO_RESOLVER`, `MARCO_BIN`, `MARCO_OVERLAY_IDLE`, `MARCO_VOICE_WAKE`,
`MARCO_NARRATE_LOCK` (lock file path; default `$TEMP/marco-narrate.lock`),
`MARCO_NO_PANIC_STOP`/`MARCO_NO_TEACH`/`MARCO_SIMPLIFY_SAVES` (set by the overlay).

## Not done / explicitly out of scope this session

No commit/push was made (commit only when the user asks). No merge of
`feat/host-ffi`. No new dependencies added to the engine.

---

# Session: navigation producer → hypotheses → first unknown-game live run (2026-08-09)

Three milestones landed and a fourth is queued with its design already settled. The tree is green
(`go build ./...`, `go test ./...`, `go test -race ./...` across 68 packages, `gofmt`, `go vet`,
`go run ./cmd/docscheck`), on `feat/host-ffi`, **nothing committed**.

Read the canonical notes rather than this section for architecture: [[Navigation]],
[[Hypotheses]], [[ADR-013-navigation-is-meaning-not-keys]],
[[ADR-014-hypotheses-are-evidence-not-identity]], and the milestone records
`docs/director-navigation.md`, `docs/director-hypotheses.md`,
`docs/experiments/Experiment-008-unknown-game-discovery.md`.

## 1. The live navigation producer

`internal/platform/navsource` — a `WH_KEYBOARD_LL` hook that never suppresses, ignores injected
events, and classifies raw key codes into a **closed intent vocabulary**
(`up/down/left/right/confirm/back/pause/point`) on a worker goroutine. The hook callback does one
thing: a non-blocking offer into a bounded 256-slot queue. Raw key identity dies inside the
package — `rawEvent` is unexported, has no `String` method, and appears in no escaping type.

**Two defects found, both the same kind — a complete mechanism nothing invoked:**

- The composition root never opened a subscription. Deleting `navSource.Open(...)` from
  `Runtime.newObservationSampler` failed **no test in the repository**, because the existing
  wiring test built its own subscription. A shipped build would have installed the hook,
  classified every intent, and discarded all of them.
- The hook's drop counter was wired to nothing — `noteDropped` was dead code and the Windows
  backend counted into a package-level atomic nobody read — making backpressure unobservable in a
  design whose entire justification for dropping is that it beats blocking.

Also landed: **edge-local order** (`ScreenTransition.Sequences`, so `down, down, confirm` is no
longer flattened into a set), traces that carry navigation on **every** slot including skipped
ones so production and replay agree on attribution, and a producer diagnostic block that
separates "nobody pressed anything" from "nothing was listening".

## 2. Semantic hypothesis generation

`internal/director/observe/hypothesis.go` + `terms.go`.

The audit found `observe.Insight` already existed and was production-wired — but it reads
`Findings`, the authoritative entity timeline. Everything the discovery stack had built lives in
`ShadowTotals`, and **no generator read any of it**. `ShadowRegion` carries no text; OCR labels
live on the authoritative side. `observe.Sample` carries both, which is where the join went.

**The design decision this turns on:** OCR text is classified to meaning **at the boundary and
then discarded**, exactly as key codes are. `SemanticEvidenceFrom` matches whole words against a
closed vocabulary of *generic interface concepts* and returns terms; the text does not travel
with the result. A typed username matches nothing, so it cannot become evidence — not by rule,
but because there is nowhere to put it. `backpack` is not `back`; a redacted label is never
consulted.

Seven interpretations, all `possible_*`. Ceilings enforced by test: **geometry alone never names
a screen**, text alone never names a screen, navigation alone never names a screen, and
text-entry requires an accessibility role rather than a shape. `classify` checks contradictions
**before** support and they cannot be outvoted; `contested` is terminal; there is deliberately
**no confidence float**, because 0.62 cannot distinguish thin evidence from evidence half
pointing the other way.

Production call site: `observe.Hypotheses(stats.Shadow, ...)` on the terminal `Result` in
`runner.go`, then `observationView` → protocol → `renderHypotheses` in the CLI.

## 3. First live run on a game Marco had never been taught

Schedule I, three minutes, target discovered at run time, no game-specific configuration. OCR was
unavailable (no tesseract) and accessibility reported the application `unobservable (1 elements,
nothing operable)`, so this tested **structure + navigation** with the text leg absent.

52 valid inferences, 215 detections, **10 screen states and 48 tracks**. State-relative presence
reproduced on a second game with no retuning (`shadow_1`: 43/52 `bursty` globally, **18/18
`persistent` within its own state**). **Replay reproduced production exactly**, including the
ordered `pause → pause` run — parity confirmed on live data rather than on a fixture.

It formed **one `supported`** hypothesis (a recurring menu-like screen of 6 grouped controls
across 3 visits) and four `contested` ones, and it never named a screen — there was no text, and
the ceiling held.

**Two limits it measured, which are the value of the run:**

- **Navigation admission is the blocker.** 1086 physical events produced **7 intents**: 765
  repeats suppressed, **117 refused as `ambiguous_gameplay_key`** (WASD/Space), 0 dropped.
  Exactly one of 21 observed edges carried any attribution. The conservative policy is behaving
  exactly as specified and, on a WASD-driven game, refuses nearly everything the player did.
- **The group uniformity measure does not generalise.** Rocket League's evenly stacked menu column
  scored 0.97; every recurring group here scores 0.00–0.01, so `possible_choice_group` is
  contested on spacing for essentially every real group. Recorded rather than retuned — one game
  is not a basis for replacing a rule fitted to another.

Also observed, neither diagnosed: cadence is tighter than Experiment-007 (1218ms median
inference, 3807ms median recorded gap against a 2s interval, 13 of 65 slots skipped, 21 samples
late), and segmentation looks loose (10 states in three minutes, six of them with a single
inference, `state_unknown` involved in seven of 21 edges).

## 4. NEXT TASK — state-conditional navigation admission

The measured blocker on the product goal. [[ADR-013-navigation-is-meaning-not-keys]] already
names this as the correct fix and cited the missing precondition — screen state on the producer's
side of the boundary — which now exists and is proven live.

**No code was written. Three tasks are queued. The decisions below were settled during design and
should not be re-derived:**

- **The predicate must be non-circular.** Compute "this screen looks like a set of choices" from
  ONE inference's **raw regions only**, in `observe` — the same discipline that keeps state
  identity independent of tracking (`TestStateIdentityDoesNotDependOnTracking`). It must not read
  tracks, groups, states or hypotheses.
- **The loop does not close, and that must stay true.** Screen state is decided from `regions`
  only; inputs are used only in `note()` for correlation. So admission depending on screen
  appearance does not make screen appearance depend on input. Re-verify after the change.
- **Context is pushed from the composition root** (`liveSampler.Sample`), stored on the `Source`,
  and read on the **worker** thread — never inside the hook callback.
- **Freshness comes from flip-off, not from the TTL.** The sampler sets `menuLike=false` on the
  next non-menu-like inference, so the error window is one inference gap (~3.8s measured). The
  TTL exists only to guard against inferences stopping altogether. A stale "menu-like" flag would
  manufacture navigation evidence during gameplay, which is the exact failure being avoided.
- **Mapping:** W/A/S/D → up/left/down/right and Space → confirm, admitted **only** under a fresh
  menu-like context. With no context the behaviour is unchanged, so
  `TestGameplayAmbiguousKeysAreRefusedAndCounted` must still pass untouched.
- **Conditionally admitted intents are weaker evidence and must say so.** An intent admitted
  because Marco *believed* the screen was menu-like is a weaker claim than one from an
  unambiguous key. Carry a flag on `InputEvent`, count it in `InputStats`, surface it
  per-transition, and let a hypothesis qualify support resting entirely on it. This
  **deliberately requires extending the type allowlist in
  `TestRawKeyIdentityCannotCrossTheBoundary`** — that test exists to make such an addition a
  visible act, and a `bool` is not key identity. Extend it consciously; do not loosen the rule.
- **Mandatory:** a composition-root wiring test, plus the mutation — delete the context push and a
  production-path test must fail. This subsystem has now produced four mechanisms that were
  complete, unit-tested and unreachable.

## Housekeeping

- `director.exe` at the repo root is a build artifact and is gitignored. The service it ran was
  **stopped**, which removed the keyboard hook.
- The live trace (65 slots) is in the session scratchpad and is **not** in the repo. It contains
  no key identity and no text — only closed-vocabulary intents and normalised geometry — but it
  was not promoted to a fixture, because committing a capture of somebody's screen session is the
  user's decision and not a tool's. See [[Vision-Corpus-Workflow]].
- Shadow vision needs `MARCO_SHADOW_VISION=screenparser`,
  `MARCO_SCREENPARSER_MODEL=tools/vision-export/weights/screenparser-1280.onnx` and
  `MARCO_ONNXRUNTIME=tools/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll`.
  `MARCO_SHADOW_TRACE=<file>` captures a replayable trace.
- **OCR is unavailable on this machine** (no tesseract on PATH), so every text-supported
  hypothesis is untested live. The deterministic tests cover it.

## Not done / explicitly out of scope this session

No commit or push (commit only when the user asks). No merge of `feat/host-ffi`. No new engine
dependencies. No capability, no execution authority, no generated Marco and no proposal loop —
the discovery stack still ends at hypotheses, by design.

---

# Session: state-conditional navigation admission (2026-08-09)

Executes the next task recorded in the previous handoff. Tree is green — `go build ./...`,
`go test ./...`, `go test -race ./...` (68 packages), `go build -tags onnxvision ./...`,
`gofmt`, `go vet`, `docscheck`, linux/darwin cross-builds — on `feat/host-ffi`, **nothing
committed**.

Canonical: [[Navigation]], [[ADR-013-navigation-is-meaning-not-keys]] (amended), Roadmap item 8.

## A stale roadmap entry, found and corrected

The audit hit two canonical documents disagreeing. `Roadmap.md` item 8 said **"The proposal loop
— NEXT"**, written before the first live run. [[Experiment-008-unknown-game-discovery]], later
and also canonical, concluded the proposal loop was *unreachable* — *"Marco can see the screen
and cannot see how the player reached it"* — and ADR-013 already named the fix.

The roadmap simply had not been updated. It now records state-conditional admission as item 8
(done) and the proposal loop as item 9 (next), with a note explaining the supersession, because
the next resumed session would otherwise trip over the same thing.

## What was built

W/A/S/D and Space are read as `up/down/left/right/confirm` **while, and only while, the last
observation showed a set of choices on screen**. With no context, behaviour is unchanged.

- `observe.MenuLike(regions)` — the predicate, over ONE inference's raw detections. Three or more
  controls carrying a nameable role. It reads no tracks, states, groups or hypotheses.
- `Source.SetScreenContext(menuLike, at)` — worker-side store, never touched by the hook.
- `InputEvent.Conditional`, `InputStats.Conditional`, `ScreenTransition.ConditionalOnly` — the
  weaker class of evidence, carried and counted rather than resolved silently.

## The decision that matters most: no spacing rule

The obvious predicate is "an evenly spaced column", and `StructuralGroup.Uniformity` already
measures it. It was deliberately NOT used. Rocket League's menu column scores 0.97; every
recurring group in Schedule I scores 0.00–0.01. Both are real interfaces full of real choices, so
a spacing rule would admit navigation in one game and refuse it in the other — a Rocket League
rule wearing the costume of a screen-shape rule, invisible until a third game.

The signal used instead is how many controls carry a role whose name may be said, which is
layout-independent and separates choices from play in both recorded games with one threshold.

## Freshness has two bounds, and the second is easy to miss

- The assessment must not be **newer** than the press. A key pressed during play, immediately
  before a menu appeared for another reason, must not become navigation retroactively.
- It must not be older than `ScreenContextTTL` (10s). That is a **backstop against inferences
  stopping**, not the freshness mechanism. The real bound is **flip-off**: the composition root
  sets the context on every valid inference, including the not-menu-like ones.
- A **skipped** inference leaves the assessment standing — unknown is not false, and flipping off
  there would lose admission every time the cadence gate declined (13 of 65 slots live).

## Privacy

The type allowlist in `TestRawKeyIdentityCannotCrossTheBoundary` was widened by exactly one
entry, `bool`, with the reasoning recorded **in the test**. Both privacy guards fired on the
first build and had to be extended deliberately — which is what they exist to force.

The flag says nothing about which key was pressed: `up` from an arrow and `up` from W are
indistinguishable downstream. No character key can reach the classifier under any context
(`TestContextDoesNotAdmitCharacterKeys`).

## Measured result that changed our understanding

**The gain cannot be measured from any existing trace, and this is structural.** Admission is
decided inside the producer at the moment of the press; a trace records the *outcome*, not the
keystroke, because raw key identity dies in the adapter. Replaying the Schedule I trace under the
new code therefore still shows 7 intents and 117 refusals — correctly.

What the trace CAN bound, and now reports: **17 of its 52 valid inferences (33%) showed a set of
choices.** A third of that session was eligible for admission. `director shadow-trace` prints
this under `choice screens`, with an explicit note that it bounds the opportunity and does not
replay it.

## Mutations — eight, each killed its test

| mutation | test |
|---|---|
| delete the production `SetScreenContext` call | `TestTheProductionSamplerPushesTheAdmissionContext` +2 |
| predicate always says menu-like | `TestASparseScreenDoesNotLicenseAmbiguousKeys` +1 |
| drop the "assessment must predate the press" guard | `TestAnAssessmentMadeAfterThePressCannotJustifyIt` +1 |
| drop the TTL backstop | `TestAStaleScreenContextStopsAdmitting` +1 |
| stop marking context-admitted intents | `TestAmbiguousKeysBecomeNavigation...` +1 |
| count an observation conditional despite a real key | `TestOneUnambiguousKeyMakes...` +1 |
| drop the all-conditional contradiction | `TestAnActionRestingOnlyOnContextAdmittedKeysIsContested` |
| push context on every sample, not just valid ones | `TestASkippedInferenceLeavesTheAssessmentStanding` |

All six mutated files verified byte-identical to their pre-mutation state afterwards.

## NEXT TASK — the proposal loop (Roadmap item 9)

Surface a `supported` hypothesis as a question and record the answer as evidence in its own
right. A declined proposal must be kept: it stops re-proposal every session and is the only
ground truth this system will ever get. Still no capability, no execution authority, no generated
Marco.

## One open question a live run would answer

**Do the newly admitted intents actually correlate with screen changes, or only with walking
around inside a menu screen?** 33% of the session was eligible; whether that converts into
attributed edges cannot be determined offline. The shortest run that answers it is 60–90s of any
application with a keyboard-driven menu — open a menu, move the selection with WASD, back out,
twice. Not Rocket League specifically; any unknown application is better evidence.

## Housekeeping

- `director.exe` at the repo root was **rebuilt** this session (gitignored). An earlier stale
  build briefly produced misleading replay output; rebuild before trusting any `shadow-trace`
  reading.
- No service is running. No keyboard hook is installed.
- The Schedule I trace remains in the session scratchpad, uncommitted.

---

# Session: the proposal / confirmation loop (2026-08-09)

Executes Roadmap item 9. Tree is green — `go build ./...`, `go test ./...`, `go test -race ./...`
(68 packages), `go build -tags onnxvision ./...`, `gofmt`, `go vet`, `docscheck`, linux/darwin
cross-builds — on `feat/host-ffi`, **nothing committed**.

Canonical: [[Hypotheses]], [[ADR-015-a-question-is-evidence-not-settlement]],
[[director-proposals]], Roadmap item 9.

## The audit result that shaped it

Three existing question-shaped mechanisms, none a fit: `ConfirmationBroker` blocks and is
command-scoped; `Clarification` is execution-time ambiguity resolution; `Insight.Validation` is a
string describing *how a person could settle it*, not user validation. Nothing uninvoked.

`LiveRecorder` had already solved **materiality** — keyed on the evidence-set digest, with the
failure recorded in-file (keying on a growing count republished an unchanged claim forever). That
principle was reused rather than re-derived.

## What it does

`evidence → hypothesis → eligibility → question → answer → validation evidence → Result → CLI`

- Ledger updated at the runner's per-sample choke point, beside `foldShadow`.
- Answers enter via `ObserveQuery.Answer` and route to the active runner **or a finished
  session's stored Result** — the latter is the ordinary case.
- `director answer <session> <question-id> yes|no|not-now`.

Three answers kept distinct all the way to the CLI. Confirmation adds `FromUser` support and
promotes to `validated` **only with no contradictions**; contradiction adds a `FromUser`
contradiction and leaves the observations listed; decline touches no semantic evidence.

## The defect a failing test found

Question identity originally included the interface terms, so adding a term created a NEW
question — a declined "is this a settings screen?" returned the moment OCR read one more label,
bypassing suppression entirely. Identity is now **structural only** (kind, subject kind, roles,
members); terms moved to the evidence digest, where they correctly make a declined question
materially new and re-ask it as a re-ask (`Asked: 2`).

## Mutations — seven; two survived at first and both were real test gaps

Mandatory four all bite now. Two initially survived:

- **Binding the answer to the earliest open question** survived because the test answered the
  *first* of two — which that implementation also gets right by coincidence. The test now answers
  the **second**.
- **Allowing a contested hypothesis to be asked about** survived because `contested` implies
  contradictions, so the no-contradictions gate caught it first. A sharper mutation removing the
  status gate outright proved it *is* load-bearing for `tentative`. The overlap is recorded in
  ADR-015 rather than tidied away.

## Incidental fix

`Runner.Run` computed default thresholds into locals but the per-sample path read the unresolved
`cfg`, so a session's live feed and its final report could be computed at different settings.
Already true of `Insights`; fixed rather than reproduced, since a second consumer reading
unresolved thresholds is how that becomes load-bearing.

## NEXT TASK — cross-session hypothesis identity (Roadmap item 10)

A validated hypothesis dies with its session. `Subject.Fingerprint` exists and nothing consumes
it, so a screen rediscovered next run is a new subject, gets a new question, and the user is
asked something they already answered — the nagging the proposal policy prevents *within* a
session and cannot prevent *across* them. Needs a fingerprint matcher, a store for validated
hypotheses, and an honest answer when two sessions' evidence is merely similar rather than same.

## No live run is needed

The loop is proven by scripted sessions, production-path tests at runner and protocol level,
replay equivalence and seven mutations. Nothing about it depends on environmental evidence. The
one open environmental question from the previous milestone — whether context-admitted navigation
correlates with screen changes — is unrelated and was deliberately not bundled in.

---

# Session: cross-session semantic memory (2026-08-09)

Executes Roadmap item 10. Tree is green — `go build ./...`, `go test ./...`, `go test -race ./...`,
`go build -tags onnxvision ./...`, `gofmt`, `go vet`, `docscheck`, linux/darwin cross-builds — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[Semantic-Memory]], [[ADR-016-cross-session-identity-is-structural-and-conservative]],
[[director-semantic-memory]], Roadmap item 10.

## Subject.Fingerprint was rejected as durable identity

It stays the evidence source; equality was replaced by a tolerant comparison. Four measured
reasons: `Recurrence` grows every episode; role counts break on one missed detection; only
`possible_choice_group` carries an envelope so most subjects have no geometry; and role
composition alone collides across unrelated screens.

## The rule that matters

**`same` requires a discriminator** — matching interface terms, or a matching envelope at
IoU ≥ 0.90. Structure alone is `candidate` and inherits nothing. Several matches is
`insufficient`, never the closest. Four verdicts, no similarity score.

The honest consequence: **an application with no readable text and no envelope is never
recognised.** OCR is unavailable on this machine, so this feature would not currently fire live —
which is why Roadmap item 11 is "make OCR available and measure whether recognition fires".

## A settled assumption this milestone disproved

ADR-015 defined the material-change digest as "the kinds of support and contradiction present,
and the interface terms". The support-source set **grows within a session** (`[structure]` →
`[recurrence structure]` once a second episode lands), so memory recorded the digest at ANSWER
time and a later session compared it at first-RECALL time. They never agreed and **every declined
question returned on every restart**. Corrected to kind + structural identity + terms; recorded in
ADR-016.

## Mutations — seven mandatory, all bite; two initially spared the headline test

Both survivors were test weaknesses, not false alarms:

- The ephemeral-id mutation passed because the answered question was about a `choice_group`,
  whose `group_1` ref is stable across fixtures. The test now answers the **settings** question,
  whose subject genuinely renumbers (`state_2` → `state_1`), and session B uses a deliberately
  reordered sampler so the tracker mints different identities as a restart does.
- (The same class of weakness as the previous milestone's "answer the first open question" gap.)

## NEXT TASK — Roadmap item 11: make OCR available and measure

Interface terms are the only discriminator most screens have, and `tesseract` is not installed,
so this machine has never produced one. Every text-supported hypothesis and every cross-session
recognition is untested against real perception. Make the OCR path run, then re-measure on
captured evidence where possible: how many terms a real session yields, how many hypotheses
become text-supported, whether two sessions of one application recognise each other.

Either answer is informative. If real OCR yields too few terms to discriminate, the honest
conclusion is that recognition needs the envelope path extended to state subjects — **not** a
looser bar.

## No live run was needed or performed

Cross-session identity was proven with two runners over one store file, a deliberately renumbered
second session, the adversarial similar-but-different pair, a corruption fixture and seven
mutations.

---

# Session: OCR as a semantic discriminator — measured, and it isn't one (2026-08-09)

Roadmap item 11. Tree is green — build, tests, `-race`, `-tags onnxvision`, `gofmt`, `vet`,
`docscheck`, linux/darwin cross-builds — on `feat/host-ffi`, **nothing committed**.

Canonical: [[Experiment-009-ocr-as-a-semantic-discriminator]], [[Semantic-Memory]], Roadmap 11.

## Decision: `OCR_DISCRIMINATOR_INSUFFICIENT`

Not because OCR fails. It runs through production, its output is correctly classified, its raw
text correctly dies at the boundary. It simply **supplied no interface term that accessibility had
not already supplied, and none at all where accessibility supplied nothing.**

The A/B that settles it — same application, 45s each:

| | terms | term observations | provenance |
|---|---|---|---|
| VS Code, OCR **on** | `back, notifications, search` | 13/19 | 4 refused, 1691 quarantined (ocr) |
| VS Code, OCR **off** | **identical** | 16/24 | clean |
| Steam (roles, no accessible names) | **none**, `terms_known:false` | 0/9 | 2 refused, 784 quarantined |

No installation was needed: tesseract v5.4.0 was already present at
`C:\Program Files\Tesseract-OCR\tesseract.exe`, merely not on `PATH` and `$MARCO_TESSERACT` unset.

## Two defects found by AUDITING the path, not by any failing test

Both would have made the measurement meaningless, and both were fixed before measuring.

- **Terms could never qualify in a live session.** Scoped OCR runs on ~1 inference in 6
  (`sequence == 1 || sequence%6 == 0`), and the term ratio divided by EVERY inference against a
  0.50 threshold — capping a perfectly stable term at ≈0.17. The discriminator was structurally
  unreachable in production. Every test that seemed to exercise it set the evidence on every
  sample directly.
- **Unavailable was indistinguishable from empty.** `CompareStructure` read a remembered subject's
  terms against a session that could not read any and returned `MatchDifferent` — turning "Marco
  could not look" into "this is a different screen".

Fixes: `SemanticEvidence.Observed`, `ScreenState.TermObservations` (the correct ratio
denominator), `Fingerprint.TermsKnown` / `StructureSignature.TermsKnown`, and unknown terms are no
longer evidence in either direction. Also `Store.Remember` now refuses subjects with no
discriminator — they could never be matched, so records for them grow the file forever and can
never be read.

**The matcher was NOT loosened.** Conservative rules stand.

## Why OCR cannot currently help, architecturally

OCR text becomes a label only via fusion's `TextFilledMissingLabel` — the element must be
structurally real AND its role must be in the plaintext allowlist (`button`, `menu_item`, `menu`,
`tab`, `checkbox`, `radio`). So a term needs **accessibility for the structure** and text landing
on it. Where accessibility names things, OCR is redundant; where it exposes roles without names —
the case OCR exists for — the text did not attach, and much of it was quarantined by the
target-provenance guard first.

## Cost measured

OCR 760ms/pass (capture 28ms, **recognise 731ms**) against ScreenParser 625ms median / 667ms p95.
Both at a 2s cadence: **11 late samples of 9 taken** with OCR, **0 of 24** without.

## NEXT TASK — Roadmap item 12: give the vision detector's roles a nameable path

The missing discriminator is not text-reading, it is structure Marco is allowed to name.
ScreenParser supplies `button`/`menu` regions on exactly the surfaces accessibility cannot see,
and they carry no label and therefore no term. The question is whether a nameable role from the
shadow detector plus text inside its box can produce a term where accessibility exposes nothing.
If it cannot, the honest conclusion is that cross-session identity on those surfaces must rest on
the envelope rather than on semantics.

## Not measured

Cross-session recognition on live data. No real session produced a `supported` hypothesis, because
an idle window never RECURS — recurrence needs a screen to appear, go, and come back, which needs
interaction. Not requested of the user.

---

# Session: giving vision-derived structure somewhere safe for a word to belong (2026-08-09)

Roadmap item 12. Tree is green — build, tests, `-race` (including `cmd/director` at 169s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds, both plugins — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[ADR-017-structure-earns-a-name-text-never-earns-structure]],
[[Experiment-010-vision-structure-as-a-semantic-path]], Roadmap 12.

## Decision: `VISION_SEMANTIC_PATH_PROVEN`

The previous milestone ended `OCR_DISCRIMINATOR_INSUFFICIENT` and pointed upstream: the missing
evidence was never text-reading, it was **structure Marco is allowed to name**. That turned out
to be exactly right, and connecting the two halves was mostly wiring.

## The seam, found by tracing before writing anything

Three links missing, one drifted:

- **The experiment had no reader.** `newShadowVision` built a `vision.Provider` and never set
  `Reader`. Scoped OCR had never run on a ScreenParser detection — the mechanism existed,
  fully tested, wired only into the authoritative provider.
- **`ShadowRegion` has nowhere to put a word** — role, geometry, confidence, nameable.
- **`SemanticEvidenceFrom(sample.Entities)` reads the FUSED side only**, and shadow evidence
  never reaches fusion by construction. Nothing the detector found could ever be classified.
- **Nameability was four allowlists** — privacy classifier, shadow diagnostic, vision benchmark,
  and (newly) the reader. Two carried a written rationale for being copies.

## What changed

- `directorapi.ElementRole.NameablePlaintext` is now THE allowlist. The other three delegate.
  It gates what is **READ**, not only what is kept — an unsayable region costs no OCR round trip
  and its text never enters the process.
- `Class.Nameable()`; scoped reading restricted to nameable structural classes; `text` regions
  read for SCREEN-level meaning only, producing terms and never a control.
- `contested()` — overlapping nameable regions above `AmbiguousOverlap` (0.25 of the smaller)
  are refused, not resolved. Nesting is not a contest: stacked menu buttons overlap ~6% through
  detector jitter and must both be named.
- Scoped reading is now opt-in **per cycle** on `SourceOCR` in the request, which is what
  `WithPixels` already documented. It used to run on every vision pass.
- `observe.ScreenTextEvidence` and `SemanticEvidence.Merge`; `shadowEntities` /
  `shadowScreenText` in `shadowSampleFor`, gated on `TargetProven`.
- `observe.ShadowLabels` on the sample and the totals: unsayable / ambiguous / skipped / read /
  unreadable / screen texts. Every way of producing no terms used to look identical.

## Measured

Same fixture, same production path, reader absent then present:

| | structure | nameable | terms | observed |
|---|---|---|---|---|
| before | 6 | 4 | none | **false** (unknown, not empty) |
| after | 6 | 4 | `audio, back, controls, settings` | true |

Structure **unchanged** — text enriches structure and never creates it, and the test asserts the
counts are equal. Budget on that pass: unsayable 2, attempted 5, read 4, screen texts 1,
ambiguous 0, skipped 0. The icon and the panel cost nothing.

**The discriminator question.** Two settings screens with identical composition:
structure-only `candidate` → structure+terms **`different`**. That is the false-merge case
ADR-016 has never had a way to resolve.

| case | verdict |
|---|---|
| A same subject twice | `same` |
| B similar structure, different words | `different` |
| C one control permanently unreadable | `different` |
| C' one control unreadable 1 pass in 3 | `same` |

C/C' answer Part 24: the exact-set rule is **not** the bottleneck. The per-state term ratio
absorbs intermittent evidence, and a term lost on every reading is a real difference. **The
matcher was not changed.**

**Cost, on a real captured panel** (`fixtures/vision/rocketleague/05-pause-panel.png`, tesseract
5.4.0, 3×): whole panel 521ms → **0 spans**; scoped region median **129ms**, p95 136ms. The
whole-panel failure reproduced two milestones later, on a different fixture. Arbitrary 47px bands
read `EXIT TO MAIN MENU` exactly and `US Gem EVE CETTIN(SS` where a band straddled two buttons —
more evidence that the structure is what makes the reading trustworthy.

Budget arithmetic: 6 labels + 2 texts ≈ 1.0s on top of a ~0.9s inference against a 2s cadence, so
a label pass will occasionally cost one skipped shadow slot. Counted, not silent.

## Eight mutations, each fails a production-path test

reader deleted · nameability gate removed · text allowed to become an element · every role made
nameable · unknown collapsed into empty · provenance bypassed · terms dropped before
`SignatureOf` · association call disconnected with every helper left intact. The last is the
instructive one: readings still happened (read 4, screen texts 1) and no term appeared.

## Deliberate deviation from a recorded decision

`visionbench.nameableRole` and `cmd/director.nameableRole` carried written rationales for being
*copies* of the privacy allowlist ("a benchmark measures what a backend could offer, a policy
decides what may be stored"). Converged anyway: now that nameability gates what is read, a role
counted nameable in one place and withheld in another promises evidence the system refuses to
collect. Recorded in ADR-017.

## NEXT — Roadmap 13: what CONNECTS two remembered semantic states

Marco can now recognise a screen across sessions without accessibility and say what it is about.
It can say nothing about the relationship *between* two such screens. The evidence exists and is
already correlated — transitions carry closed-vocabulary navigation intents per state — but no
durable claim about a PAIR of subjects has ever been formed. Still not procedure learning: the
claim is "these two remembered states are related, and this kind of navigation was observed at
the boundary", not "here is how to get there".

## Not measured

No live run. The accessibility-poor fixture models such a screen; it is not one. And the shipping
detector (`icon_detect`, one class, `icon`) still cannot benefit — correctly, and now *measured*
by `LabelsUnsayable` rather than inferred.

---

# Session: remembering that two screens are connected (2026-08-09)

Roadmap item 13. Tree is green — build, tests, `-race` (including `cmd/director` at 186s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds — on `feat/host-ffi`,
**nothing committed**.

Canonical: [[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]], [[Semantic-Memory]],
Roadmap 13.

## What Marco can now remember

> I recognise this subject. I recognise that one. I have seen the first become the second before —
> this navigation was around it, this often, across this many sessions.

It still cannot say how to perform the transition, and nothing here can be executed. That
distinction is the milestone.

## The audit found the machinery mostly already existed

`ScreenTransition` already carried every category the durable record needed — `Preceded`,
`Unattributed`, `ConditionalOnly`, ordered `Sequences`. What did not exist was any way to key an
edge on something that survives a restart: session-local `state_3 → state_8` is renumbered by
the next run.

So the durable object is a projection of `ScreenTransition` whose endpoints are
`RememberedSubject` ids, resolved through `Memory.Recall` — the identity layer that already
exists. **The relationship layer does no matching of its own.** The Action Graph was considered
and rejected: its edges serve executable semantics, and putting an observation there is the
shortest path to executing it.

## Shape

- `observe.RememberedRelationship` — from/to subject ids, application, observations, sessions,
  per-intent support, unattributed, conditional-only, bounded ordered runs, dropped-run count.
- `observe.RelationshipsFrom(totals, hypotheses, application, memory)` — current evidence first,
  memory second. Both endpoints must reach `same`; anything less stays session-local **and is
  reported**, because "nothing transitioned" and "nothing was recognised" are different sessions.
- Store: `relationships` beside `subjects` in the same file, same lifecycle. Referential
  integrity at the write (unknown or cross-application endpoint refused and counted) and at the
  load (orphan dropped and counted).
- **Choke point: `Runner.Run`, at session end, once.** The transition tally GROWS while a session
  runs, `Sessions` counts independent corroborations and only a finished session is one, and the
  store writes its whole file atomically — so a batch is one write where n edges would be n.

## Two defects found by tracing, not by a failing test

- **One screen had two durable identities.** `possible_menu_like_state` set `Members` on the
  fingerprint; `possible_reversible_place` and `possible_text_entry_state` did not. `Members` is
  identity-bearing with a ±1 tolerance, so 4 vs 0 compared as DIFFERENT and `Remember` stored one
  screen as TWO subjects. Consequences ran downstream quietly: a question answered about one
  record did not suppress the other, and an endpoint resolved or not depending on which
  interpretation named it. Fixed — `stateFingerprint`, one function, four call sites. Every
  existing test compared a signature with itself, which is why nothing caught it.
- **A privacy assertion was really a capacity assertion.** The boundedness test folded fifty
  observations first, so `Preceded` was already at its cap when it offered `VK_RETURN` — refused
  by the BOUND, not by admission. Mutation 6 survived it. Split into
  `TestARawKeyIdentityIsRefusedByAdmissionNotByCapacity`, which starts empty; the on-disk store
  test now offers a raw key too, since a file that never saw one cannot show it refuses one.

## Eight mutations, each fails a production-path test

delete the write · identity from ephemeral state ids · flatten direction · drop unattributed ·
context-admitted treated as strong · persist raw key identity · unresolved endpoint admitted ·
duplicate edge per session. The last two are the interesting ones: the duplicate mutation grew
the topology 2→4 across sessions and made 2000 observations into 2000 records.

## Verified deterministically

Cross-session corroboration through two runners over one store file, with session-local ids
renumbered; a third session that corroborates rather than duplicates; direction preserved with
different navigation each way; branching (A→B, A→C, B→A) kept apart; ordered runs kept as
plural counted observations; unattributed and context-admitted evidence surviving intact;
2000 observations plateauing to one record with dropped variants counted; replay reproducing the
same edges from the trace's safe representation.

**No live run, and none needed** — Part 31's own reasoning. The model is deterministic and the
product-visible sentence ("I've seen you move between these two screens before") belongs to the
next layer.

## NEXT — Roadmap 14: turn a corroborated relationship into a QUESTION

Marco holds a map and cannot say anything about it to the person who walked it. The next layer
decides when an edge has earned a sentence — enough observations, enough independent sessions,
navigation that is not entirely unattributed and not entirely context-admitted — and puts it
through the proposal loop that already exists. Still not "shall I learn how to do that".
Proposal before capability, confirmation before execution.

## One thing to know before touching this

A durable edge needs BOTH endpoints to be subjects a person has settled, so early sessions
produce session-local evidence and no topology at all. That is the design and it is reported,
not silent. Tests that need topology seed both screens through `Store.Remember` — the production
write — rather than depending on which questions the proposal policy happened to ask.

---

# Session: asking whether to learn a habit (2026-08-09)

Roadmap item 14. Tree is green — build, tests, `-race` (including `cmd/director` at 167s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds, both plugins — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[ADR-019-an-invitation-to-learn-is-not-a-correction]], [[Semantic-Memory]],
Roadmap 14.

## What Marco can now ask

> "I've seen you go from the settings screen to another screen several times.
>  Do you want me to learn how you do that?"

A yes buys a pending `LearningRequest` on the relationship record. **No procedure, no capability,
no action, no route, no recorder.** Obtaining a demonstration is the next milestone precisely
because it may need to watch more than passive discovery does — a yes that silently widened
observation would be the worst reading of an invitation.

## The one thing that had to be got right

The existing proposal loop was reused, not duplicated. But its `no` means *your interpretation is
wrong* and becomes a durable contradiction; the new `no` means *I don't want that learned* and
must leave every observation intact. Sharing the machinery without separating the meanings would
let "don't bother" delete evidence.

`Proposal.Ask` is the whole distinction — `confirm_semantic_interpretation` vs
`learn_observed_relationship`, empty reading as semantic so older records still parse. Nothing
anywhere infers meaning from the wording. `Runner.Respond` branches on it.

| | semantic | learning |
|---|---|---|
| yes | `KnowledgeConfirmed` | `LearningPending` |
| no | `KnowledgeContradicted` | `LearningRefused` — preference, evidence untouched |
| not now | suppressed till shape changes | same rule, over the edge's digest |

## Eligibility — discrete, no float, every refusal named

≥2 independent sessions · ≥3 observations · dominant intent over ≥half · unattributed ≤half ·
not only context-admitted · not ≥3 one-off run variants · endpoints present and not
wholly rejected. Volume never rescues weakness: 20 observations with `confirm` before 3 is
refused where 6 with `confirm` before 6 is offered.

Refusals are closed (`insufficient_sessions`, `navigation_too_weak`, `too_much_unattributed`,
`conditional_only`, `runs_inconsistent`, `endpoint_unresolved`, `already_declined`,
`already_refused`, `learning_pending`, `another_question_open`, `already_asked`) and **every**
remembered edge is judged and reported. "Marco did not ask" is otherwise undebuggable.

## The design decision worth knowing

**The review runs once, at session end**, after this session's transitions are folded into the
store. Not per sample. Two reasons: the evidence is only complete there, and it makes the
semantics-before-behaviour priority STRUCTURAL rather than a race — a per-sample invitation won
the single interruption slot on sample 1, before any hypothesis had accumulated enough recurrence
to be worth asking about. That was a real failure, caught by
`TestASemanticQuestionTakesPriorityOverAnInvitation`, and moving the call fixed it properly
instead of adding a priority rule.

Materiality uses a **banded** session count (few/several/many) plus the intent set, the
unattributed category, the conditional-only flag and the run shapes. Raw counts are excluded —
a digest that moved with them brings a declined question back within minutes.

## Nine mutations, each fails a production-path test

delete the review call · Sessions replaced by raw observations · drop Unattributed · treat
ConditionalOnly as strong · yes marks the edge learned · no wipes the evidence · decline treated
as no · answer binds to the best current edge · digest tracks raw counts.

## Found on the way

Two screens sharing a confirmed interpretation read as *"from the settings screen to the settings
screen"* — a transition nobody made, inviting the user to correct a claim Marco never intended.
The second name is now dropped. Surfaced by mutation 2's failure output, not by a test.

## A test-only trap worth remembering

Two `semanticmemory.Store` handles over one file is not a configuration production ever has (a
Director owns exactly one), and the last whole-file write clobbers the other. A test that adds
evidence while a runner holds the store must share the handle — `seedRelationshipIn` exists for
that — or it measures the clobber.

## NEXT — Roadmap 15: bounded demonstration capture

A pending request is a person saying "yes, watch me do it". Nothing yet watches. The next layer
is the deliberate, consented, bounded capture of ONE clean example — start at the named subject,
observe the navigation, arrive at the other — kept as a procedure CANDIDATE. Still proposal
before capability: turning a candidate into something executable is a further decision with its
own confirmation.

---

# Session: watching one demonstration (2026-08-09)

Roadmap item 15. Tree is green — build, tests, `-race` (including `cmd/director` at 169s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds, both plugins — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[ADR-020-watch-me-is-permission-to-observe-not-to-act]], [[Semantic-Memory]],
Roadmap 15.

## What Marco can now say

> "I watched one example of how you moved from this screen to that one. I observed these
>  navigation events and these intermediate screens. I have not verified that repeating them
>  would work."

`ProcedureCandidate` has no `Execute`, no registration anywhere, and a `Verified` field that is
always false and exists to be read. It lives in `internal/director/observe`, whose boundary test
already proves nothing reachable from there can act.

## The recorder was deliberately NOT reused

`internal/recorder` captures raw keys and feeds `demo.Procedure`, a type the executor runs. Both
disqualifying: a yes to one question about one transition is not permission to record keystrokes,
and the output here must be evidence rather than something anything can run.

What IS reused: the closed `NavIntent` vocabulary the navigation producer already emits, the
per-sample observation path, `Memory.Recall` for resolving where the user is, and
`EditableFields` as the text-entry boundary. **A yes widens the privacy boundary by nothing.**

## Shape

- `observe.Capture` — armed / capturing / complete / incomplete / cancelled. `NewCapture` is the
  only constructor and needs a `RelationshipRef`; `armCapture` is the only caller and reads
  `PendingLearning` from durable memory. Authorisation is a shape, not a check.
- Start and end from **current** evidence. The request says A→B, which is a claim about history;
  where the user is standing now is a question only this cycle can answer.
- Bounds: 60 events, 8 checkpoints, 8 intents per run, 90 observations, 2 restarts. Exceeding one
  **stops** with its own reason rather than truncating.
- Steps, not a flat list: `A —run→ X —run→ B`. An unrecognised X is a TRANSIENT checkpoint and is
  never promoted into memory.
- Persistence: completed candidates survive, one per relationship, beside the topology. An active
  capture is never persisted, so it can never be resumed.

## Two subtleties the implementation had to get right

Both the same class of mistake — a quantity that MATURES within a visit being treated as identity:

- **`Members` excluded from checkpoint comparison.** It is the dominant structural group's size,
  which is 0 for a screen's first few observations and 4 later. Comparing on it split one screen
  into a new checkpoint every time the group came into focus. Same reason `Recurrence` is
  excluded from durable identity.
- **A transient checkpoint that is later recognised is upgraded in place**, not appended. It is
  the same screen; Marco just learned what it was. Appending inserted an empty navigation run and
  made the destination look like it was reached without the user doing anything. The terminal
  rules (`settleAt`) are shared by both paths, because a mismatch reached by the second route is
  exactly as much a mismatch.

## Eight mutations, each fails a production-path test

delete the arm call · capture without approval · drop ordered input · complete on timeout ·
accept the wrong destination · store a raw key · candidate becomes executable · unfinished capture
not ended with the session.

## Not done, and honestly

**Replay parity (Part 26) is not implemented.** A candidate is derived from `Memory.Recall`
against the durable store, and the shadow trace carries no store. Reproducing a candidate from a
trace needs the memory state of the moment as well, which is a second durable artefact this
milestone did not create. The inputs the capture consumes are all safe and replayable —
`CaptureInput` is intents, a signature and a verdict — so the gap is the memory snapshot, not the
representation. Recorded rather than papered over.

## NEXT — Roadmap 16: is a candidate a reproducible procedure?

One watched example and no idea whether it generalises. The question is whether that can be
settled WITHOUT executing uncontrolled input: a second demonstration to compare, step-by-step
consistency, checkpoints that could be verified during a future attempt. The honest first
deliverable is a judgement with named reasons — `single_example`, `steps_disagree`,
`requires_text_entry`, `transient_checkpoint_unverifiable` — in the same shape as the
learning-eligibility policy, rather than a promotion.

---

# Session: what Marco knows from one watched example (2026-08-09)

Roadmap item 16. Tree is green — build, tests, `-race` (including `cmd/director` at 168s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds, both plugins — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[ADR-021-a-judgement-is-recomputed-not-recorded]], [[Semantic-Memory]], Roadmap 16.

## The decision that shapes everything else

**A verdict is never written onto the candidate.**

```
ProcedureCandidate  (durable, fixed, what happened)
      + Topology    (durable, IMPROVING, what Marco can recognise)
      ↓ CandidateAssessment (derived, never stored)
```

What Marco can conclude depends on what Marco remembers, and memory improves. A transient
checkpoint nobody could recognise today becomes a remembered subject the moment the user names
that screen — and the same demonstration becomes more verifiable with no new observation. A
stored verdict would freeze a judgement made when Marco knew less.

Enforced by a test that fails if `ProcedureCandidate` ever grows `Verdict`, `Assessment` or
`Reasons`. It also makes replay parity statable: same candidate + same topology → same
assessment, and there is no third input.

## Verdicts and reasons

`candidate_consistent` (the CEILING — the observation hangs together, never "it works") ·
`insufficient_evidence` · `ambiguous` · `invalid`. **No confidence float**: the useful output is
the list of checkpoints Marco could not check, because that list is actionable and a number is
not.

Eleven closed reasons, each with `ResolvableByDemonstration()`. That split is the whole
preparation for the next milestone without asking the user for anything: an ambiguous run is
something a cleaner example would settle; a transient checkpoint and a text-entry boundary are
not — one needs the user to name a screen, the other needs consent and a representation.

`single_demonstration_only` is always present and deliberately not a downgrade. If it blocked the
best verdict, the best verdict would be unreachable and meaningless.

## Central question: verifiability, not performability

Per step: *could Marco later tell whether it succeeded?* Coverage is a LIST with named gaps, not
a percentage — "70% verifiable" tells a reader nothing; "the second screen cannot be recognised"
tells them exactly what to fix.

Volume is not strength: `near_capture_bound` is a reason for LESS confidence, because a
demonstration near a bound may be missing the end of itself.

## Comparison, built and proven without asking for a second demonstration

Endpoints, checkpoint sequence (durable id where there is one, safe structure where there is not),
text-entry markers, and the **decisive** navigation of each run.

Directional intents move a selection and commit to nothing, so `down, down, confirm` vs
`down, down, down, confirm` → `compatible` (the same move one row further away). Everything else
is decisive, so `left, back, down, confirm` does not reduce to `confirm` → `different`.
Deliberately not an edit distance over raw input, which would call every honest repeat different.

## Eight mutations, each fails a production-path test

verified-by-default · transient checkpoint treated as verifiable · drop RequiresTextEntry ·
flatten ordered runs · ignore checkpoint identity in comparison · delete the production assessment
call · store a verdict in the candidate · raw keys in the assessment.

**One was masked and re-run.** "Ignore transient unverifiability" (removing the first branch)
survived, because a second branch — `!subjectKnown` on an empty subject id — catches the same
case. That overlap is real defence in depth and a poor mutation target; the faithful form (mark
the transient checkpoint verifiable) fails three tests. Worth remembering: overlapping guards make
mutations look survivable when the behaviour is fine.

## NEXT — Roadmap 17: ask for the second demonstration, and only where it would help

The assessment already says which gaps another example could close. What is missing is the loop
that acts on it: propose a second demonstration ONLY when `ResolvableByDemonstration` is true,
capture it through the machinery that exists, compare with `CompareCandidates`.

Two agreeing demonstrations are the first evidence in this system that a procedure is
*reproducible* rather than merely observed — the point at which "verified" stops being
structurally unreachable. Still not permission to perform it.

The failure to avoid: asking for a demonstration Marco already knows would not help. The user has
done enough live runs.

---

# Session: asking for a second example, and mostly deciding not to (2026-08-10)

Roadmap item 17. Tree is green — build, tests, `-race` (including `cmd/director` at 181s),
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin cross-builds, both plugins — on
`feat/host-ffi`, **nothing committed**.

Canonical: [[ADR-022-ask-only-when-you-can-say-what-it-would-resolve]], [[Semantic-Memory]],
Roadmap 17.

## The governing rule, implemented

> Do not ask the user for more evidence merely because Marco is uncertain. Ask only when the
> evidence model can explain what another example would resolve.

`FollowUpFrom` reads the assessment's verdict and partitions its reasons by
`ResolvableByDemonstration`. It inspects the candidate for **nothing** — duplicating the
assessment's rules in proposal code would be a second answer to "is this evidence any good".

**One non-resolvable blocker is enough to stop.** `single_demonstration_only` +
`requires_text_entry` has a gap another example would close and a gap it would not; the second
means the exercise stays unusable however well the first goes. The report still separates them,
so the refusal is legible rather than flat.

## A correction to ADR-021

`transient_checkpoint_unverifiable` was on the non-resolvable side, arguing that watching an
unrecognised screen again does not make it recognisable. True, and it answers the wrong question.
A second example cannot give the screen an identity but it CAN corroborate that the same
unrecognised screen appears at the same point of the same route. *"Would another example reduce
my uncertainty"* and *"would it fix this gap"* are different questions. Moved, with the reasoning
kept beside the method.

## A real defect found on the way

**A fulfilled learning request stayed `pending`.** `armCapture` reads `PendingLearning`, so every
later session would have re-armed and watched the same route again — forever, without asking. Fixed
with `LearningFulfilled`, written when a completed candidate is stored.

## Shape

- `AskSecondDemonstration` — a third `no` with a third meaning. Refusing here withdraws nothing:
  candidate 1, the original request, the relationship's evidence and every endpoint's meaning are
  untouched.
- Lineage `relationship → candidate 1 → assessment → follow-up → candidate 2`, with
  `ProcedureCandidate.Sequence` and candidates keyed on `(relationship, sequence)`. Candidate 1 is
  never mutated — a comparison is worthless if one side has been edited.
- Bounded at two: `second_demo_already_captured`.
- `FollowUpDigest` over verdict + reason set + coverage pattern. Never counts, never timestamps.
- Two agreeing → drop `single_demonstration_only`, reach `candidate_consistent`. Two differing →
  `demonstrations_disagree`, `ambiguous`, both kept, never averaged. **Nothing promoted.**

## Transient checkpoints now resolve at assessment time

A screen unrecognisable when it was SEEN may be recognisable now. `resolveTransient` matches the
checkpoint's structure against current memory (Members excluded — it matures within a visit, same
reason as everywhere else). That is what makes "the judgement changes shape, so the declined
question comes back" work through real memory rather than by editing a candidate.

## Ten mutations, each fails a production-path test

ask regardless of resolvability · ignore blockers · bind the answer elsewhere · two demos means
verified · flatten the comparison · NO withdraws the original request · NOT NOW becomes permanent ·
unlimited demos · delete follow-up generation · delete the comparison.

Two needed a second attempt: binding "to the newest route" happened to pick the right one, and
`Remember` with an empty signature is refused by the store's discriminator guard so the
contradiction mutation did nothing. Both re-run in faithful form. Worth remembering — a mutation
that is silently a no-op looks exactly like a passing test.

## NEXT — Roadmap 18: decide what a REHEARSAL is, before anything is performed

The strongest state reachable is one consistent candidate corroborated by a second agreeing one,
with no authority at all. The next question is the first that touches acting: what evidence would
justify Marco attempting a transition ITSELF, and under what containment?

The deliverable is a DESIGN — promotion criteria, what a rehearsal may touch, how each step is
verified before the next, what aborts it, what the user approves first — not an executor. Every
milestone so far has been arranged so this decision could be made deliberately rather than
arrived at by accident.

## Live runs

Still none needed, and none requested. The loop is now complete enough that ONE real
demonstration would test the product interaction rather than plumbing — that is the first live
run worth asking for, and it belongs after the rehearsal boundary is designed.

---

# Session: language/spec reconciliation — what IS Marco? (2026-08-10)

A deliberate direction correction, not a Director milestone. Tree is green — build, tests,
`-race`, `-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin — on `feat/host-ffi`,
**nothing committed**.

Canonical: `spec/Core.md` (new, the only `status: normative` page), `CLAUDE.md` governance rule.

## What was actually wrong

Not the implementation. **The compiler implements essentially everything the spec describes** —
108 fixtures under `testdata/` exercise contracts, translators, feeds, queues, channels, locking,
concurrency and tests. The drift was that nothing said which parts were a *promise*: 26 spec
pages, no status on any of them, so a future session reading a normative-looking page reasonably
concludes Marco is obliged to implement it. That incentive is backwards.

## What changed

- **`spec/Core.md`** — four nouns (`actor`, `act`, `script`, `set`), one way to do things
  (`do X's Y with …`), four endings, branch/loop/wait/cleanup, the pronouns. Everything else is
  listed as a supported extension with a pointer. Every example compiles.
- **Every spec page declares `status:`** — normative (1), reference (22), historical (1: Audit),
  experimental (2: Inference, Reference Modes).
- **`internal/spectest`** — normative examples are compiled through the real
  lex→parse→build→compile pipeline; pages must be classified; generated routes are held to
  Core's vocabulary and off the backstage one; the generator's own source is scanned for what it
  is CAPABLE of emitting.

## Things the drift protection found on its first run

- **A corrupted route.** `routes/notepad/say-hello.marco` ended with a stray backtick.
- **A backstage leak in the generator.** Every anchored route carried a comment explaining that
  "the engine resolves by SCORING signals into a confidence… how near a candidate is…". That is
  the resolver's algorithm printed on stage. Rewritten to say what an Anchor *is*.
- **Me, writing spec drift while writing the spec.** My first draft of Core.md documented
  `this can Nudge with a Point.` and `this can Measure, gives a number.` — those are *contract*
  forms (`it can …`), not actor forms. An actor capability is just `this can <Cap>.`; shapes live
  on a contract. The compile gate caught it within a minute of existing.

## The finding to act on eventually

**`actor`, `act` and `scene` are three words for one implementation kind** (`KindActor`). `act`
earns its keep — it is the host surface and the only place `exports` appears. `scene` appears
**once** in the entire corpus and carries no distinction the compiler can see. Core v1 promises
`actor`, `act`, `script`; `scene` was left working and flagged in `builder.go`. Deleting a
keyword is a decision, not a cleanup.

## Deliberately NOT done

No compiler rewrite. The sentence-oriented lexer/parser is Marco's identity and showed no
correctness blocker. A typed semantic-statement layer between the sentence AST and the graph is
*arguable* — `compile.go` does match on word positions — but there is no drift or duplication it
would currently remove, and architecture for its own sake is how a readable language acquires a
framework. Recorded, not built.

`this's` / `that's` audited and unchanged.

## Six mutations, each fails

a normative example stops compiling · a page loses its status · a route uses an out-of-Core
construct · a route names a backstage concept · the normative page is gutted · the generator
re-emits the resolver explanation.

Two needed strengthening after surviving: the vacuity guard counted "more than zero" examples
(now a floor of 12), and the generator gate ran one fixture that never reached the anchored path
(now scans the emitter's source).

---

# Session: act, scene, actor, verb — the distinction made durable (2026-08-10)

Roadmap item 18, and the last language milestone. Tree is green — build, tests, `-race`,
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin — on `feat/host-ffi`,
**nothing committed**.

## The audit

`verb` is **not a keyword and never was** — it is the reader-facing name for `this can <Name>.`
plus `this's <Name> does…`. Nothing to reconcile; the word belongs in prose, not in the lexer.

`act`, `scene` and `actor` all lower to `KindActor`. That sharing is correct — at run time each
one holds things, knows verbs, and hears and says — but the authored word was **thrown away**,
and with it the distinction. Verified: `this exports Click.` on an `actor` compiled. So `act`
carried no obligation a reader could rely on. **Finding B: an implementation gap erasing a
language distinction**, not harmless sharing.

The single corpus `scene` (`testdata/act_scene`) encoded the collapse rather than the
distinction: an "act" with a body and no `exports`, and a "scene" indistinguishable from an
actor. It was testing that the three words are interchangeable.

## The change, and how small it is

- `Node.Declared` — the authored word survives into the graph. No IR redesign, no new kind.
- `checkExportsBelongToActs` — only an act offers capabilities outwards. The error says what to
  write instead.
- `testdata/act_scene` rewritten to demonstrate the distinction.

**No new syntax.** A scene holding an actor needed none: `this's Hero is a Knight.` is the
sentence Marco already uses to say a thing has a thing, and a scene saying it about an actor is
a scene holding an actor. Inventing a keyword for that would have been inventing a keyword for
a sentence the language could already write.

## Mutations: five killed, one not

parse `scene` as `act` · erase the authored word · delete the distinction check · a scene verb
need not finish — all killed by `internal/spectest`.

**Unkilled, reported rather than papered over:** dropping a scene's field declarations in the
builder did not fail anything. My first containment test was proving the wrong thing (it removed
the actor entirely, so it failed on the `do Knight's Charge` call rather than on the holding); I
isolated it, and the isolated case still could not be made to distinguish. Either the field path
is not where I looked or the rejection comes from elsewhere. **A scene holding something that is
not in the play may not be checked at all.** Worth an hour when somebody next touches the
builder; not worth manufacturing a test for now.

The generator mutation could not be applied — the emit line did not match my pattern. Also
unresolved.

## Decisions

| Word | Meaning | Implementation matched? | Change |
|---|---|---|---|
| act | the way in; offers capabilities outwards | **no** | `exports` now belongs to an act |
| scene | where things happen; holds actors, knows verbs | partly — expressible, not distinguished | authored word kept; fixture rewritten |
| actor | a thing in the play | yes | none |
| verb | what one of them does | yes (not a keyword) | documented as prose, not syntax |

**Core v1 did not change** — no construct added, none renamed. **Director lowering unchanged**;
the previous mechanical proof stands, and nothing Director emits uses `exports`.

## NEXT — back to Director

Language work is CLOSED. Reopen only when a concrete Director requirement proves Core v1
insufficient — not to make the implementation more elegant.

The queued Director work: **decide what a rehearsal is, before anything is ever performed.** The
strongest state the learning loop can reach is one consistent candidate corroborated by a second
agreeing one, with no authority at all. The next question is the first that touches acting — what
evidence would justify Marco attempting a transition itself, and under what containment. The
deliverable is a design with promotion criteria and a safety boundary, not an executor.

---

# Session: designing the rehearsal boundary (2026-08-10)

Roadmap item 19. **Design only — no executor was built.** Tree is green — build, tests, `-race`,
`-tags onnxvision`, `gofmt`, `vet`, `docscheck`, linux/darwin — on `feat/host-ffi`,
**nothing committed**.

Canonical: [[ADR-023-rehearsal-is-attempt-scoped-authority]], Roadmap 19.

## The audit's best finding

**The seven "cannot execute" invariants were already true structurally.** Every learned type —
`RememberedSubject`, `RememberedRelationship`, `LearningRequest`, `ProcedureCandidate`,
`CandidateAssessment`, `Capture`, `Proposal` — is defined in `internal/director/observe`, whose
transitive imports reach nothing that can touch a desktop. `TestTheObservationPackageCannotAct`
already guaranteed the package; what it did not guarantee is that the learned types still LIVE
there. The risk was never "somebody adds an import" — it is "somebody moves `ProcedureCandidate`
somewhere more convenient", where it keeps every field, passes every one of its own tests, and
quietly acquires a neighbourhood that can act.

So the types are now NAMED in `authority_test.go`. Moving one out fails loudly.

## The design, in five lines

`candidate_consistent` (observation) → `rehearsal_eligible` (judgement) → `rehearsal_authorized`
(permission, one attempt) → `verified_procedure` (an experiment succeeded once) →
`generated_marco` (the durable thing).

Eligibility is derived from `CandidateAssessment`, in the same shape `FollowUpFrom` uses. **No
second scoring system, no confidence float.**

The rule: *every action Marco proposes to take has a corresponding observable expectation it can
check afterwards.* Default refusal.

## The counterexample, stated rather than weakened

That rule excludes `down` in a menu — it moves a selection Marco's segmentation cannot see. Three
options considered; the accepted one is `progress_unobservable`: take the step, re-verify the
screen is still the same screen, and record the intermediate as unobservable rather than as a
success. `down, down, confirm` rehearses as four verified moments, two of which carry weaker
evidence and say so.

## Decisions worth knowing

- **A yes to "watch me" is not a yes to rehearse.** `AskRehearse` is a new typed proposal kind;
  overloading the existing yes would broaden authority silently.
- **The grant has no durable representation at all** — the simplest way to guarantee it cannot
  survive a restart.
- **Text entry is rehearsal-ineligible and stays that way.** Nothing was retained, so there is
  nothing to replay, and the fix is not to start retaining. The path is a parameter supplied at
  invocation — the shape Marco already uses for secrets.
- **A naked coordinate is not a target.** Rehearsable only when the click resolves to a semantic
  control; coordinates travel as corroboration, never identity.
- **Marco never navigates to the start.** That would need another procedure it has not verified.
- **One successful rehearsal is enough** for the first verified state. The corroboration bar was
  already paid at `candidate_consistent`; requiring N would be inventing a threshold.
- **A `RehearsalResult` is a separate record.** The observation is not mutated into "verified",
  exactly as an assessment lives beside a candidate rather than inside it.

## Core v1 lowering: checked, not assumed

Core v1 can express everything a verified procedure does — ordered intents, an act to perform
them through, both endings, the entry point. **The two gaps are NAMES**: what to call the actor
and the verb. That is a question for the person, not a language change. The one thing Core v1
cannot express is a text-entry parameter, which is why text entry is rehearsal-ineligible rather
than a language request.

**No language change proposed. Language work stays frozen.**

## NEXT — Roadmap 20: eligibility and the authorization request

The first implementation step, and deliberately the one that generates no input: turn
`CandidateAssessment` into a `RehearsalJudgement`, add `AskRehearse` with the readable question,
and record an attempt-scoped grant that expires. All testable without a desktop, and it ends one
step short of acting — which is where a milestone about authority should end.

---

# Roadmap 20 — rehearsal eligibility and attempt-scoped authorization (2026-08-10)

The first implementation step of rehearsal, and deliberately the one that **performs no desktop
input at all**. The strongest state the system can now reach is an inert grant.

`internal/director/observe/rehearsal.go` — judgement, question, grant.
`internal/director/observesession/runner.go` — `reviewRehearsal` at session end,
`authorizeRehearsal` on the yes, `Grant`, `RevokeRehearsal`.
`cmd/director/observeregistry.go`, `observecmd.go` — the explainability surface.

## Evidence is not authority

Four things a reader will want kept apart, and the milestone is mostly the keeping-apart:

- an **assessment** says what the demonstration evidence supports;
- a **judgement** says whether that is enough to *ask* for one controlled experiment;
- a **question** is a question;
- a **grant** is one user's yes to one attempt.

A previous yes to *shall I learn this?* authorised **watching**. `AskRehearse` is a separate
`AskKind` so that permission cannot be reached from this one — and that is checked, not asserted:
`TestOnlyARehearsalYesCreatesAuthority` says yes to the learning question and to the
second-demonstration question and proves neither produces authority.

## The rule that decides eligibility

Every action Marco proposes must have an observable expectation it can check afterwards. Three
per-step verdicts, because two would force a lie:

| | |
|---|---|
| `directly_verifiable` | lands on a *different* remembered screen |
| `progress_unobservable` | lands back on the screen it started on — `down` in a menu |
| `unverifiable` | lands somewhere Marco cannot resolve |

`Progressed()` — deliberately not `Succeeded()` — is true for the middle one. It permits
*continuing*, nothing more, and only while the screen it must remain on is still there. Bounded at
`MaxUnobservableRun = 2`, and the last step must always be directly verifiable, because a
rehearsal that ends on a step Marco can only contain is an attempt with no answer either way.

**The design's discriminator was wrong and the code found it.** ADR-023 described the middle case
as "a transient checkpoint that stayed inside a screen" — but an unresolved transient checkpoint
is exactly what `CandidateAssessment` already refuses, so on that reading `progress_unobservable`
was structurally unreachable. Same failure mode as the term-ratio denominator. Corrected to a
comparison of where the step landed against where it started.

Fifteen refusals in a closed vocabulary. No confidence float anywhere.

## The grant

Scoped to one application, one starting screen, one destination; bounded by `MaxInputs`,
`MaxUnobservable` and a 30-second `MaxDuration`; **consumed when the attempt begins**, not when it
succeeds, so a crash halfway through leaves nothing reusable. Scope is re-checked at claim time
rather than trusted from issue time. Cancellation revokes it.

It cannot act: no executor-shaped method, no field that could hold a key, a label, a title or a
pixel, and it lives in the analysis core whose transitive imports reach no keyboard, mouse or
runtime. It is never written to disk — `TestNoAuthoritySurvivesARestart` reads the store back and
proves there is no section a grant could be restored from.

The judgement is **recomputed when the yes arrives**, and fails closed two ways: if Marco no
longer believes the evidence, and if the evidence merely *changed* while staying good enough. The
second half is what a verdict comparison cannot see — the user agreed to one attempt at what they
were shown, and a revised demonstration is no longer that.

## Two things found

- **Two empty application names compare equal.** A candidate belonging to no application passed
  the scope check whose only job was confining it to one.
- **Authority has to outlive the session that asked.** Marco cannot rehearse while passively
  watching somebody else, so the registry keeps the last runner past `finish()` purely to hold the
  grant. Nothing durable; the next session replaces it.

## Mutation gate

18 mutations, 17 killed. The two survivors are reported rather than papered over, both
defence-in-depth rather than sole gates: the plan's `unverifiable` branch (the assessment already
refuses every unresolvable checkpoint) and the runner's one-active-grant guard (`MaxOpen` is 1 and
`ReviewRehearsal` short-circuits on `granted`, so production cannot reach it — the reachable half
of that invariant *is* killed). Detail in ADR-023.

## NEXT — Roadmap 21: one rehearsal step, dry

Lower a single directly-verifiable step to the point immediately before input and prove, against a
host that records instead of acting, that what *would* have been sent is exactly the step's
intents, that the grant was claimed first, and that a scope or bound violation aborts before
anything reaches the boundary. Still no real input.

---

# Roadmap 21 — one rehearsal step, dry (2026-08-10)

One authorized step lowered to the last thing before a computer, which is a notebook. **No
keyboard, mouse, controller, accessibility or OS actuation of any kind.**

`internal/director/rehearse` — the attempt.
`internal/platform/recordhost` — the notebook.
`cmd/director/rehearsedry.go` — the composition root, and the only place that chooses a host.

## The audit found the boundary already built

[[ADR-005-legal-marco-only]] settled where the last line is, years of this codebase ago:

```
marcoexec.Operation → legal Marco → lexer → parser → graph → compile → runtime → runtime.Host
```

`oshost.Host` presses keys. `recordhost.Host` appends a line. **Nothing else differs.** So there
is no dry stack: the dry path IS the real path with its host swapped, using the same lowering,
the same encoder, the same compiler, the same runtime and the same frame scheduler. A dry recorder
called directly from rehearsal code would have proved that the shortcut works, which is the
failure mode this repository has now recorded four times.

`internal/director/rehearse` therefore imports no host and cannot build one. It takes a
`directorapi.MarcoRunner`, and an import test holds that.

## A meaning, never a key

The Director learned that somebody **confirmed**. It did not learn Enter.

Lowering a `NavIntent` to a key chord would have put a device binding — a property of a keyboard
and an application — inside the Director, throwing away on the acting side exactly what
[[ADR-013-navigation-is-meaning-not-keys]] protects on the watching side. So:

- `os.marco` gained `this exports Navigate.`
- `marcoexec` gained `KindNavigate`, lowering to `do OS's Navigate with "confirm".`
- the intent→key table lives backstage in `internal/oshost/navigate.go`

**Not navsource's table reversed.** Watching admits W/A/S/D and Space as *conditional* evidence —
they mean navigation only on a screen that looked like a set of choices. Acting cannot be
conditional: there is one key to press, and pressing W when the arrow key was meant would drive
the car. `point` is deliberately absent, because a pointer press needs a position and a position
is not a meaning.

One new act capability, no new syntax. **Language work stays frozen.**

## The claim comes first, and a step is atomic

The grant is spent BEFORE anything can be produced, and it stays spent if setup then fails —
otherwise "one attempt" becomes "as many attempts as it takes to get past the setup". A step whose
whole ordered run does not fit its budget is refused before its first input rather than truncated:
half of `down, down, confirm` is not a smaller version of the procedure, it is a different one
ending somewhere no demonstration ever went. Twelve prechecks, all of them before the first
effect.

Fourteen refusals in a closed vocabulary, every one of them meaning the same thing about the
world: zero calls reached the boundary.

## And it claims nothing

Two words: `would_emit` and `refused`. A `DryStep` is an engineering artefact — it verifies
nothing, counts towards nothing, alters no candidate, no relationship and no memory, and no part
of the learning loop reads it. See [[ADR-024-a-dry-step-is-not-evidence]].

`progress_unobservable` **does** lower — it is a step Marco may take — and arrives carrying its
weaker marker, which is what stops it being read as arrival. A grant's `MaxUnobservable` is now the
authorized plan's own longest unobservable run rather than the constant, so a plan whose every step
lands on a remembered screen carries a budget of zero.

## Found while building it

**Answering a question on a FINISHED session never reached memory.** The registry recorded the
answer in the durable ledger and stopped there. Every question this system asks is asked at
session *end*, so `g.active` is nil by the time anybody can answer one — meaning the finished
branch is the only branch, and a user could say "yes, learn that" every session forever while
Marco never once armed a capture. Fixed in `observationRegistry.Answer`; the wiring test found it
on its first run.

## And a second defect, caught by reading the output

A dry run CLASSIFIED the post-input screen. With a recording host the application does not move, so
it reported `wrong_state` — "the screen became A, which is not what that step was for" — which
reads as a failed step when nothing had been sent. `Live` is now told whether its actuator reaches
a computer; when it does not, the emission is the whole of what happened and the record says Marco
did not try it.

## Mutation gate

18 mutations, 17 killed. The survivor is equivalent and reported rather than papered over: the
early `GrantConsumed` check in `BeginDryAttempt` duplicates the grant's own claim-time refusal, so
removing it produces the same closed reason by the other route. It stays for the message.

## NEXT — Roadmap 22: one rehearsal step, live

The first real learned input. Claim the grant, establish the current source subject through
perception rather than as an argument, emit exactly one semantic input, settle, observe, and
classify: `directly_verified`, `progress_unobservable`, `wrong_state`, `target_moved`,
`target_unavailable`, `ambiguous`, `unobservable`. Then stop — no second step, and a
`RehearsalResult` becomes the first record in this system entitled to say Marco tried something.

---

# Roadmap 22 — one rehearsal step, live (2026-08-10)

The first milestone in which something Marco learned by watching may move a real computer.

`internal/director/rehearse/live.go` — the attempt.
`cmd/director/rehearserun.go` — the composition root, and the only thing that chooses a real host.
`director rehearse [--live]` — the trigger, and the only thing that spends a grant.

See [[ADR-025-one-move-then-look]].

## Nothing above the host changed

Same `Attempt`, same claim, same `LowerStep`, same `KindNavigate`, same legal Marco, same compiler
and runtime. [[ADR-005-legal-marco-only]] holds unchanged — the live input is the same program the
dry milestone compiled, with `oshost.Host` beneath instead of the notebook.

What this milestone adds is everything that must be true *around* one real input:

```
look → compare → CLAIM → re-check the window → emit → settle → look again → classify → stop
```

## The starting screen comes from perception

Roadmap 21 accepted the source subject as scope input, which was right for a milestone that could
not act and is not right now. `Live.establish` watches the window for a bounded run of
observations and resolves the screen through `SignatureOfState` → `Memory.Recall` — the same path a
demonstration capture uses. Wrong screen, unrecognised screen, ambiguous screen: nothing is sent.

The grant says the user demonstrated A → B, which is a claim about history. Whether the interface
is showing A *now* is a question only looking can answer.

## The claim, and the guard after it

The permission is spent **after** the comparison and **before** the emission. Before it Marco has
not yet been able to act and a mismatch costs nothing; after it an input is possible, so the grant
is gone whatever happens — otherwise "one attempt" becomes "as many as it takes to get past setup".

Then the final guard, as late as it can possibly be: re-acquire the window and compare identity,
process and generation. This closes the race that matters most — **verify the screen, the user
alt-tabs, the keystroke lands in their email.** Anything moved, zero input.

## A refusal is not a result

`RehearsalResult` exists only when input was emitted. The line is `StepEmission.Reached`, the
moment the program is handed to the runner: before it nothing was sent; after it part of the run
may have landed, and claiming otherwise would be a claim that is not available. Host failure is
therefore an OUTCOME (`input_failed`), not a refusal.

Seven outcomes, and two words that must not become one:

| | |
|---|---|
| `progress_unobservable` | a property of the STEP, known before Marco tried. Containment held. It does not mean the selection moved correctly. |
| `unobservable` | a RUNTIME failure to look. Input went out; Marco cannot say what came of it. |

`directly_verified` requires the SPECIFIC expected subject. A screen that became a different
remembered screen is `wrong_state` — treating change as success is how a procedure gets promoted
for going somewhere nobody asked for.

## Settling is watching

The post-input wait watches the screen state, stops the moment it has held for three consecutive
observations, and is bounded at eight. Not `execute`'s region waiter — that lives behind the
execution pipeline the rehearsal path must not reach, and it answers a question about pixels where
this layer's evidence is screen states. Not a sleep either.

## Acting is an explicit decision, twice

`rehearse.Live` is built with perception and memory — neither can affect anything — and is
INCAPABLE of emitting until the composition root calls `WithActuator`. `--live` then chooses the
real host. No session spends a grant, no session-end review spends one, nothing spends one on a
timer, and a later session **withdraws** an unspent authorization rather than leaving it lying
around. `Live` also takes a `Recogniser` — one method, and it is a read — so a rehearsal has no
write to memory to reach.

## Found while building it

**The live path compared the grant's digest against itself.** The scope was built with
`Evidence: g.Evidence`, so the "has the evidence moved since the yes" check could not fail. It now
comes from the recomputed judgement. The wiring test caught it.

## Mutation gate

18 mutations, 17 killed. The survivor is equivalent and reported rather than papered over: the
early source-subject check duplicates the grant's own claim-time comparison, which returns the same
refusal and leaves the grant unspent either way. It stays because it fails before the claim.

## No live run was performed

The capability is implemented and proved deterministically against a scripted world: a window that
can move or vanish, a screen that changes when the input lands, and a runner that records instead
of acting. **No application was launched and no physical input was emitted.** The first real
actuation is the user's to choose, with `director rehearse --live`.

## NEXT — Roadmap 23: the multi-step rehearsal state machine

Under one attempt grant: step → verify → step → verify through the whole candidate, stopping at the
first outcome that is not `directly_verified` or a contained `progress_unobservable`. Then — and
only then — a route that completes may produce a whole-procedure `RehearsalResult` eligible for
verification, which is the first thing that could ever make `ProcedureCandidate.Verified` true.

---

# Roadmap 23 — the multi-step rehearsal state machine (2026-08-10)

Marco proposes one step. Reality answers. Only then may Marco propose the next — and a rehearsal
succeeds only when the WHOLE learned route survives that conversation.

`internal/director/rehearse` — the state machine and the whole-attempt result.
`internal/director/observe/rehearsal.go` — `RehearsalEvidence`, and what it does and does not prove.
`internal/director/semanticmemory` — where a completed route is kept.

See [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].

## The seam, generalised rather than replaced

The audit found the terminal point exactly where Roadmap 22 left it: `Attempt.LowerStep` set
`AttemptLowered`, and nothing could follow. That is now:

```
open --lower--> acted --observe--> open        the only loop
  |               |                  |
  +---------------+------------------+--> finished | cancelled
```

`LowerStep` may run only from `open`. **ACT → ACT is impossible at the type**, so an orchestrator
with a bug is REFUSED (`awaiting_observation`) rather than typing twice — the invariant is a
property of the transition graph, not of somebody remembering to look.

No second lowering path. No second actuator. No sequence executor beside the proven primitive: the
machine calls `LowerStep` repeatedly and everything below it is untouched.

## The authorization needed no widening

`AskRehearse` already asks *"I'd like to try it once myself, one step at a time, and stop the moment
the screen isn't what I expected"* — a bounded multi-step attempt described in the user's own words.
Nothing about what they approved changed.

The ATTEMPT owns authorization; a step owns none. Bounds come from the plan: inputs from the
judgement, steps from its length, unobservable run from its longest contained stretch, duration from
the grant. The grant is claimed once, before the first step, spent for the whole attempt however it
ends, and cannot be resumed, reused or restored.

## Containment comes from the candidate, never from the result

The subtle one. A step the candidate said was **directly verifiable**, whose screen then did not
change, is `wrong_state`. Inferring `progress_unobservable` afterwards would let every step that did
nothing report itself as safely contained — and would eventually promote a procedure that presses
keys into a void.

And **a route cannot succeed on containment**: the final step must be directly verified, because
containment says the screen did not change and a destination nobody arrived at is not reached.

## Verification became derived

`ProcedureCandidate.Verified` stays false and is now deliberately vestigial. Verification is not a
property of an observation.

A completed attempt stores `observe.RehearsalEvidence`: which candidate, which digest, which
endpoints, how many steps and inputs, the per-step outcome vocabulary. **Nothing executable** —
reproducing any of it means lowering the candidate again under a fresh authorization.

`CandidateAssessment.WithRehearsal` then recomputes verification every time it is asked, and three
things must hold: the route completed, the digest still matches, both endpoints are still
recognised. A stored boolean would go on saying yes after the demonstration was revised or a screen
was contradicted — the discipline [[ADR-021-a-judgement-is-recomputed-not-recorded]] established,
applied to the strongest claim in the system.

Persistence is required because the NEXT milestone cannot ask an exited process whether the route
once worked.

## Failure stays evidence

A host that could not send is never read as the procedure being wrong; a wrong destination is never
read as the host failing. Nothing invalidates a candidate, nothing but a completed route is stored,
and every failure needs a fresh authorization to try again.

## Mutation gate

**18 mutations, 18 killed.** Four needed re-running in faithful form: three of my first attempts
were masked by an invariant that independently prevented the bad behaviour (the settle-time window
check already catches a window that moved before the guard did; a prefix cannot reach
`terminalAfter` because a verified non-last step always permits continuing), and one only blanked a
field rather than taking the shortcut it was meant to model. The corrected versions target the guard
at the exact acquisition before step 2, `Completed()` itself, and a genuine bypass of `marcoexec`
for step 2+.

## READY for the first meaningful live test

The smallest safe one: an application with a **reversible** two-step menu transition — open a
settings screen, then open one of its sub-screens — with `director rehearse --live`. Two steps is
enough to exercise act → observe → verify → act, and reversible means a wrong outcome costs a press
of Escape. It does not need to be any particular game. **I have not run it.**

## NEXT — Roadmap 24: lower a verified procedure into readable Marco

The first milestone that turns learned behaviour into something a person can read, edit and delete.
Core v1 already expresses everything a verified route does; the two gaps are NAMES — what to call
the actor and what to call the verb — and both are questions for the user.

---

# Roadmap 24 — what Marco learned becomes Marco (2026-08-10)

*Director watched and verified the behaviour. What it learned is now ordinary readable Marco.*

`internal/director/observe/lowering.go` — may it be written down?
`internal/director/marcoexec/play.go` — writing it.
`cmd/director/learnedplay.go`, `director learned` — the surface.

See [[ADR-027-what-marco-learned-becomes-marco]].

## The audit, and why nothing new was invented

`internal/codegen` is the TEACH-side generator and is explicitly forbidden to the Director. There is
no `internal/director/marcostep` — the comment naming it is stale; that package is `marcoexec`, which
already owns "a typed intention in, legal Marco out, one encoder, one set of escaping rules". A
learned play is that same responsibility, so it lives there rather than in a third generator with a
third set of escaping bugs.

The compile gate lives in `internal/spectest`, which already holds generated Marco to Core's
vocabulary and to the backstage-word rule with the REAL lexer, parser, graph builder and compiler.

## The four things that make it Marco rather than IR

**Meanings only.** The lowering input is `[][]NavIntent` — nothing else is handed to the generator.
Not filtered at it: a field that exists eventually gets printed, so a `Digest` or `Checkpoint` field
on the judgement type fails a test whether or not anything prints it.

**The compiler decides.** Against the canonical `os.marco`, not the spec harness's stub. A meaning
Core has no sentence for is a language-expression gap — reported, lowering stopped, language
unchanged.

**One actor, one verb, no manufactured scene.** A learned route has no place in the play, and
inventing one to look like an AST would be the exact shape this milestone exists to avoid.

**Provisional names that say so.** `UnnamedShortcut` and `Run`. Guessing a meaningful name from a
screen's text would leak OCR into the language; guessing from the application would name the play
after the wrong thing. `naming_required` was not needed — an inert artifact nothing can ask for by
name is safe to give a placeholder.

## What it produces

```marco
// Marco learned this by watching. Rename it and change it however you like.
use os.

the UnnamedShortcut is an actor.

this can Run.
this's Run does...
    do OS's Navigate with "confirm".
    do OS's Navigate with "down".
    do OS's Navigate with "down".
    do OS's Navigate with "confirm".
    this is ok!

the App is a script.

do UnnamedShortcut's Run...
    when ok?
        log "done".
    or?
        log that's error.
```

## Inert means inert

No file, no registry, no resolution path, nothing that can run. `marcoexec` cannot reach a runtime,
host, platform adapter, rehearsal grant or execution path. The test that matters snapshots the
working tree before and after and requires it byte-unchanged — not "contains no suspicious name",
because a play written anywhere a registry might later scan becomes resolvable by a request nobody
made.

## Mutations

12 run, 11 killed. The survivor is equivalent — every path that skips a step also adds a refusal,
and `Eligible` already clears the steps — and is now documented at the guard itself rather than
counted as coverage.

**Two of my own mutations exposed weak tests before the code:** one test built its expectation AFTER
lowering, so a generator that reordered its input in place would have agreed with the bug; another
watched only `routes/` for a write that could land anywhere.

## What this milestone does NOT mean

| | |
|---|---|
| observe a procedure | **yes**, since Roadmap 15 |
| verify a procedure | **yes**, derived from a completed rehearsal (Roadmap 23) |
| express it as Marco | **yes** — this milestone |
| persist the source | **no** |
| register it as a learned skill | **no** |
| resolve it from a later request | **no** |
| execute it | **no** — and generating Marco is not authorization to run Marco |

## A gap found and left visible

The repository has **no route-metadata sidecar mechanism**. Provenance is carried in the response
beside the source rather than inside it, because the artifact is not persisted and there is nothing
yet to attach metadata to. It becomes load-bearing the moment a play IS saved: a learned route that
cannot say which demonstration and which rehearsal it came from is a file nobody can audit.

## NEXT — Roadmap 25: name it, save it, and only then let it be asked for

Four separate permissions in the order a person meets them — name, save, register, invoke. The
prerequisite Roadmap 24 exposed is the second one's: route metadata.

---

# Roadmap 25 — a learned play is a file with a past (2026-08-10)

*Director can turn verified learning into a named, auditable, durable Marco play that a fresh Marco
process can find again — without that lifecycle granting itself permission to perform it.*

`internal/routes/origin.go`, `internal/director/marcoexec/play.go`, `cmd/director/learnedplay.go`,
`director learned --name --verb --save --register --forget`. See
[[ADR-028-a-learned-play-is-a-file-with-a-past]].

## The prerequisite, re-verified

Roadmap 24 said there is no route-metadata mechanism. Checked against the tree: half true. There is
no metadata sidecar, but there IS a **companion-file convention** — a taught route keeps its
recording at `<slug>.rec.json` beside the source, and `Delete` and `Rename` already carry it along.
The shape existed; the record did not. Provenance is now `<slug>.origin.json` in the same shape.

## Saved and registered are different PLACES

Route discovery is a directory scan: `global/`, an app's loose files, `context/`, `focus/`. Nothing
else is read. So a saved play lives in `<app>/learned/` — on disk, readable, editable, auditable,
**structurally invisible to the resolver**. Registering is moving it somewhere the resolver looks.

`saved == registered` is not a mistake code can make: there is no boolean to get wrong. And no
registry was invented, because discovery already is one.

## The two-file problem

Source first, provenance second, both atomic through temp+rename. A crash between them leaves a
`.marco` with no sidecar — an ordinary authored play, claiming nothing. The reverse would leave
provenance describing a file that does not exist, and a later unrelated file under that slug would
inherit a past it never had; `Origin` refuses that separately, so it is unreachable twice.

## The user owns the file

Editing a learned play is allowed and changes nothing about permission — it still resolves, still
compiles, still runs under whatever authority ordinary routes run under. What changes is the CLAIM:
the digest stops matching and the provenance reads `edited`. Registering an edited *staged* play is
refused, because registering on Director's authority a file Director did not write would be
vouching for somebody else's edit.

## Naming

Two names, because a play is a sentence: `do Volume's Mute...`. Validated against Marco's own rule
with a readable refusal before the compiler's — not a second grammar. Naming REGENERATES from the
same ordered meanings; a substitution is a rename that can change a procedure sharing a word, and
the file would still compile.

## Mutations — and a harness bug worth recording

12 run, 12 killed. But the first "all killed" run was **wrong**: an earlier invocation had aborted
on a shell syntax error mid-script, leaving a mutation applied and never restored, so three of the
"kills" were build failures against a damaged file. The clean re-run found three genuine survivors,
two of which were my patterns failing to match after I repaired the file. Re-run individually by
hand, all three die.

The lesson is the one this repository keeps relearning: **a mutation that "dies" for a reason you
have not looked at has not been checked.**

## What is true now, and what is not

| | |
|---|---|
| observe behaviour | **yes** |
| learn relationships | **yes** |
| verify a procedure | **yes** (derived from a completed rehearsal) |
| write readable Marco | **yes** |
| name it | **yes** — the user does |
| persist it | **yes** |
| audit where it came from | **yes** |
| survive restart | **yes** |
| discover it later | **yes**, by the ordinary resolver |
| resolve a later request to it | **yes** |
| execute it automatically | **no** |

## NEXT — Roadmap 26: the last boundary

Everything up to *"I know which play you mean"* works and survives a restart. What remains is the
sentence after it. The question is not how to run a `.marco` — Marco has always done that. It is
whether a learned play resolved from a natural request reaches the ordinary invocation boundary on
the ordinary authority, or needs one of its own. **Audit `marco do` first:** if resolution already
runs immediately with no seam between, that seam is the milestone.

---

# Roadmap 26 — resolution is not permission (2026-08-10)

*A learned play reaches the same execution door as every other Marco play, but knowing which play
to use is never confused with permission to open that door.*

`internal/orchestrator/authority.go`, `Deps.Resolve`, `Deps.Do`, `AskFirst` at the composition
root. See [[ADR-029-resolution-is-not-permission]].

## The audit found no door

```
Do(name) → Reg.Resolve(app, name) → d.Run(rt) → driver.RunFileWithHosts(...)
```

One function. No seam. `confirm` existed but only for teaching. So the milestone was not "add
learned-play permission" — it was **build the door**, for every route.

## What it is

`Deps.Resolve` returns a `Resolved`: which play, what kind, what state its provenance is in — a
value that can be inspected, logged and refused, with nothing on it that runs anything. `Authorize`
is the door. `Run` is behind it.

| play | verdict |
|---|---|
| authored, taught | allowed — somebody wrote it |
| learned, provenance intact | ask, once per invocation |
| learned, edited | allowed as ordinary — it is the user's writing now |
| learned, no way to ask | refused — fail closed |

`Declined` (the user said no), `Refused` (Marco declined) and resolution failing are three
different sentences. Authority is per invocation and is never written down: no `Trusted: true`, and
asking again next time is correct rather than missing.

After the decision it is `.marco` → lexer → parser → graph → compile → runtime → Host. No Director
executor, no candidate replay, no learned host, no fast path.

8 mutations, 8 killed.

## THE GAP — and it is the next milestone

**A learned play does not encode its own starting state, and Core v1 cannot express one.**

Application context IS covered: a learned play registers as a `context/` route, which ordinary
Marco already refuses to run unless that application is in front. SCREEN-level start state is not.
Director verified *"from subject A"*; the play says only `do OS's Navigate with "down".`

So a learned play invoked from the wrong screen inside the right application presses its keys
anyway. **A confirmation does not fix that** — a user saying yes does not make an invalid starting
state correct — and it was deliberately not hidden behind a Director graph consulted at execution
time. The promise is that what Director learned becomes Marco; a play whose safety depends on
something the play cannot say has not kept it.

## Where the system stands

| | |
|---|---|
| observe, recognise, learn, remember, ask | **yes** |
| watch a demonstration, compare two | **yes** |
| request and perform bounded rehearsal | **yes** |
| verify a whole route | **yes** — derived from a completed rehearsal |
| generate readable Marco, name, save, register, resolve | **yes** |
| authorize an invocation | **yes** — this milestone |
| execute through ordinary Marco | **yes**, proved into a recording host |
| verify the final real-world result | **no** — nothing observes after an ordinary run |
| start-state safety | **partial** — application yes, screen no |

## The product question, answered

*If I teach Marco something today, close it, reopen tomorrow and ask for it — what happens?*

**In plain terms:** Marco finds the play you named, tells you it learned that one by watching you,
and asks whether to run it. If you say yes it runs it like any other play you have. If you edited
the file, it stops mentioning that it learned it and just runs it, like anything else you wrote.

**The chain:** phrase → `nlu`/resolver → `Registry.Resolve` → `Classify` (reads `.origin.json`,
digest checked) → `Authorize` → `AskFirst` → `driver.RunFileWithHosts` → lexer → parser → graph →
compile → runtime → Host.

**The caveat:** it does not yet check you are on the right SCREEN. That is Roadmap 27.

## NEXT — Roadmap 27: let a play say where it starts

A LANGUAGE question before a Director one. Director knows the answer; Marco has no way to hear it.
Investigate in order: whether an existing Core construct already carries it; if not, whether this
is the concrete Director requirement that justifies reopening Core v1 — which the governance rule
reserves for exactly this case; and only then what the sentence should be.

Until then a live Marco Moment would be a keypress into an unverified screen.

---

# Roadmap 27 — a play says where it begins (2026-08-10) — PARTIAL

**Core v1 did not change. No syntax was added.** See
[[ADR-030-a-play-says-where-it-begins]].

## The investigation stopped at step one

`when?` / `or?` over a capability that answers ok-or-failed already expresses an entry condition —
the same shape `OS's Focus` has always used to mean *"ok if the active window matches"*. It
compiles today. Contracts and anchors were never reached for: a familiar word is not automatically
the right abstraction.

**What was missing was a CAPABILITY, not a construct.** Nothing could answer *"is the screen the
user named the one in front?"*. A capability is declared in an act — the route
[[Marco-Boundary]] already prescribes, and the one `Navigate` took in Roadmap 21.

`internal/screenmod/screen.marco` declares one read-only export. Every capability in it READS;
deciding must not be doing. An act rather than an undeclared actor on purpose: the compiler checks
capabilities of declared acts, so a typo fails at compile time.

## The compiler found the shape

The first attempt wrapped the steps inside the `when ok?` arm and ended the `or?` arm with a
period. Refused: *"falls off the end without an explicit return"* — the four endings are
exclamations, and a body whose only returns are inside a two-armed block is not recognised as
returning.

The shape that compiles is the early return — *ask, and stop if the answer is no* — which is also
the one a person would say out loud. The readable shape and the legal shape turned out to be the
same shape.

## Before and after

```marco
// BEFORE
this's Mute does...
    do OS's Navigate with "down".
    do OS's Navigate with "confirm".
    this is ok!

// AFTER
this's Mute does...
    do Screen's Showing with "the pause menu"...
        when ok?
            log "starting".
        or?
            this is failed with error "this play starts on the pause menu"!
    do OS's Navigate with "down".
    do OS's Navigate with "confirm".
    this is ok!
```

A novice can read: what it is called, what it expects first, what it does, and what happens if it
cannot start.

## Fail closed by construction

Mismatch, ambiguity, unobservable, unavailable target, or a host that cannot answer at all are all
*not ok*, and the `or?` arm returns before any effect. Under `cmd/marco` the fallback host is
`oshost`, whose unknown-action branch returns **failed** — so a Marco with no recogniser wired
refuses a guarded play rather than running it. Silence is never yes.

## Why this is PARTIAL — read before Roadmap 28

**The capability is declared but not fulfilled.** No host answers `Screen's Showing`, so today
every guarded play refuses. Safe, deliberate, unfinished: a learned play is currently unrunnable
until Director exposes a read-only recogniser.

Two things remain, and they are one milestone: the read-only Director act host, and asking the user
what the screen is called. `"the pause menu"` is a USER's word; Director must not guess it from
OCR, by the same rule that governs naming the play.

**And it settles the availability question plainly:** a learned play cannot validate its own
starting screen without Director. Application context is ordinary Marco; screen recognition is
Director's. The separation is "Director figures out the play, Marco performs it" — not "Marco
performs it alone".

## Postconditions

The same mechanism would express one: `do Screen's Showing with "…"... when ok? … or? this is
failed …!` reads identically after the steps as before them. Nothing was implemented for it, but
start conditions and final verification are NOT fundamentally different in the language — which is
worth knowing when the verification milestone comes.

## NEXT — Roadmap 28

Fulfil the act, and let the user name the screen. Then prove the wrong-screen Marco Moment end to
end. Only then is a live Marco Moment defensible.

---

# Roadmap 28 — the user names the stage (2026-08-10) — PARTIAL

**CORE_V1_CHANGED: NO.** No syntax was added.

`RememberedSubject.Called` + `NameSubject`/`SubjectNamed`, `internal/platform/screenhost`, wired in
`cmd/marco`. See [[ADR-031-the-user-names-the-stage]].

## Read this first: Roadmap 27's guard did not guard

The shape recorded in ADR-030 **compiled and did nothing**. A return inside an `or?` arm ends the
arm, not the capability, so the steps after the block ran anyway and the keys went out on the wrong
screen.

A test that read the source and asked the compiler shipped it. What caught it was running the play
against a Screen host that said no and watching `OS's Navigate` come out regardless. The shape that
works nests the steps INSIDE the `when ok?` arm — structural, because there is no line after the
block for control to fall through to.

**Compiling is not behaving.** ADR-030 has been corrected rather than quietly rewritten; the
original reasoning is kept beside the correction.

## What was built

**One durable user-supplied string.** The privacy boundary is provenance, not content: `the pause
menu` typed by a person is allowed; the same words read off a screen are not.

**Exact and application-scoped.** No substring, no fuzzy match, no cross-application fallback. Two
subjects in one application may not share a name; a duplicate arriving some other way is ambiguous,
and ambiguous refuses.

**A host that looks and compares and can do nothing else** — three read methods, reaching no OS
host, driver, window activation, grant, execution pipeline or orchestrator. Five internal outcomes
collapse to `failed` at the language boundary and stay apart in diagnostics, because *"I could not
look"* and *"I looked and it was different"* send you to fix different things.

**Standalone Marco fails closed.** Memory but no recogniser, so guarded plays refuse. Director
figures out the play; Marco performs it; Director still provides the eyes while it does. Stated
plainly rather than contorted around.

**The sidecar enforces nothing.** Delete the guard and the play runs anywhere — proved
adversarially, and correct.

## Results

| | |
|---|---|
| wrong screen | zero effects |
| ambiguous | zero |
| unobservable | zero |
| no recogniser | zero |
| unknown name | zero |
| another application in front | zero |
| correct screen | the two calls the `.marco` names, in order |
| name survives restart | yes, tied to the durable subject |
| guard removed from source | play runs; Marco stops claiming it verified it |

## Why PARTIAL

**The naming question is not wired.** `NameSubject` exists and is tested; nothing yet asks *"What do
you call this screen?"* through the proposal ledger, and the save flow does not require a screen
name before lowering. A guarded play can currently only be produced by a caller that already knows
the name.

## Postconditions

`POSTCONDITION_REUSABLE`. The same sentence reads identically after the steps — `do Screen's Showing
with "controller settings"... when ok? this is ok! or? this is failed …!`. Nothing was implemented
for it; the smallest next step after Roadmap 29 would be emitting it for the verified destination.

## LIVE_MARCO_MOMENT: NOT_READY

Blocker: the naming question. Every safety property holds, but no user has been asked what a screen
is called, so no guarded play can be produced by the ordinary flow.

## NEXT — Roadmap 29

Ask the user what the screen is called, through the existing proposal ledger, bound to the durable
subject rather than to whatever is current when the answer arrives.

---

# Roadmap 29 — the name is required, not requested (2026-08-10) — PARTIAL

**CORE_V1_CHANGED: NO.**

## What landed

**`observe.ScreenName`, with one constructor.** `UserSuppliedScreenName` is the only way to make
one, and `Store.NameSubject` now takes that type rather than a `string`. Observed text cannot
become a screen name by being assigned to the right variable — somebody has to write a conversion
a reviewer can grep for. The privacy exception is now a property of the type rather than a rule.

**`AskNameScreen`** added to the closed ask vocabulary.

**Lowering REQUIRES the name, resolved from memory.** `JudgeLowering` reads `Called` for the actual
source subject and refuses `screen_unnamed` when it is absent. The Roadmap 28 loophole — a caller
passing any name it liked — is closed: the caller cannot supply it at all.

**`RefusalScreenUnnamed` is distinct from a placeholder case, deliberately.** A play's actor and
verb may be provisional because naming a behaviour can wait. A screen name cannot: `Screen's
Showing with "…"` is executable meaning, and `"UnnamedScreen"` resolves to nothing.

**The production path now emits the guard**, in the corrected nested shape:

```marco
this's Run does...
    do Screen's Showing with "the pause menu"...
        when ok?
            do OS's Navigate with "confirm".
            this is ok!
        or?
            this is failed with error "this play starts on the pause menu"!
```

The screen name in that line came from durable memory for the real source subject, and the whole
file compiles against both canonical act surfaces.

## What did NOT land — read before Roadmap 30

**The naming QUESTION is still not surfaced or answered through the ProposalLedger.** The ask kind
exists; nothing produces the proposal, and no response path routes a user's answer into
`NameSubject`. A screen is named today only by a caller that already has the name.

**The mutation gate was not run.** Roadmap 28 also skipped it and Roadmap 29 was supposed to close
both. It did not. Two milestones of architectural claims now rest on tests that have never been
adversarially checked, and that debt should be paid before anything else here is called finished.

I am recording both plainly rather than reporting a milestone as complete on the strength of the
parts that did land.

## LIVE_MARCO_MOMENT: NOT_READY

Blockers, in order: the naming question is not production-wired; the mutation gate for Roadmaps
28–29 has not been run.

## Roadmap 31 — Marco actually asks what the screen is called

Roadmaps 28 and 29 built a question and a durable write and never connected them. This closes it.

**The demand.** `cmd/director/learnedplay.go:92` — when `JudgeLowering` refuses a
rehearsal-verified candidate with `screen_unnamed`, the lowering loop raises the question for
`c.Start.Subject`. That refusal is the *only* trigger. Marco does not name screens for tidiness,
and it does not scan memory for unnamed subjects.

**The identity.** `observe.SubjectRef{Application, ID}` on `Proposal.Screen`, alongside the
`RelationshipRef` learning questions already carry. Durable, so a delayed answer still knows what
it is about; application-scoped, so an answer typed while another program is in front does not
land in the wrong namespace.

**The answer.** A typed request of its own — `service.ObserveScreenName` → `director name-screen
<session> <question-id> "the pause menu"` — converted to `observe.UserSuppliedScreenName` at
`cmd/director/observeregistry.go:611`, the one production conversion point. The generic
`ProposalLedger.Respond` refuses `AskNameScreen` outright: yes/no/not-now stays three words.

**The write.** `cmd/director/observeregistry.go:450`. It happens *before* the question is settled,
so a refused name leaves the question open — an invalid answer must not consume the only prompt
the user will ever get.

**The recompute.** No cache is patched. The next `LearnedPlay` re-derives the judgement from
memory (`learnedplay.go:81`), the name is read back from the durable subject, and the Roadmap 24
generator emits the guarded play with the user's exact words.

### Two things the mutation gate found that the tests did not

**A vacuous cross-application test.** The first version switched the *selector* and not the target,
so every session still reported `testgame` — the mutation that binds an answer to the application
in front survived, because nothing had switched. `otherApp` fixes it: the `Ref` decides which
application a session is about, not the request. *A test that cannot fail is worse than no test,
and only the mutation showed it.*

**Two survivors that are honest.** The `s.Called != ""` guard in `ReviewScreenName` and the
cross-record dedup in `ProposeScreenName` are both defence-in-depth: the trigger already prevents
the first, and the one-open-question budget already prevents the second. They are reported as
survivors rather than dressed up.

## Roadmap 32 — a play says where it ends

A learned play now tells the whole story itself: **where it belongs, what happens, and how Marco
knows the scene ended where it was supposed to.**

**Success moved.** `this is ok!` is now inside the arrival check. There is exactly one success
ending in a generated play and the only path to it runs through a positive identification of the
screen the play said it would finish on. Emitting every key is a claim about the *host*; arriving
is a claim about the *application*, and only the second is what anybody wanted.

**The destination is verified semantics.** `LoweringJudgement.EndsOn` comes from durable memory
for `c.Relationship.To` — the same edge the rehearsal completed. `LowerPlayBetween` takes both
names or neither; no caller may supply either.

**One naming need, not two.** No `AskNameDestinationScreen`. `LoweringJudgement.Unnamed` lists
the subjects still needing a name, source first, and `learnedplay.go` asks about `Unnamed[0]`.
Naming it and recomputing surfaces the second. A subject that is the destination of one play may
be the source of the next; `Called` lives on the subject and nothing role-specific is stored.

**The two failures stay different in kind.** Wrong start: zero effects, the arrival is never even
asked about. Wrong destination: the effects already happened and the play still fails — with no
undo, no retry, no recovery input, no memory mutation.

### What the mutation gate found

**Two hardcoded-name weaknesses, same class as Roadmap 30's.** Both fixtures used the same
destination string the mutation would inject, so "the caller supplies the destination name"
survived. Fixed by giving the `cmd/director` fixtures a distinctive name (`the audio page`); the
mutation now kills.

**Four kills came from the Marco compiler, not from a test.** Returning `ok!` early, moving the
arrival check before the effects, and de-nesting the steps all produce source the compiler
rejects — *unreachable code after terminal return*, *falls off the end without an explicit
return*. That is a real structural guarantee and it is reported as what it is: the language
refusing the weakening, not a behavioural test catching it.

**The ordering claim rests on construction, not on a killed mutation.** `travellingStage` only
moves when the recording host sees the final navigation, so a check before the effects would see
the source and fail; `TestAPlayThatArrivesWhereItSaidSucceeds` additionally asserts the world was
looked at twice. Two attempts at a faithful "check before effects" mutation were both rejected by
the compiler, so that specific mutation remains unproven.

## NEXT — the audit

Director's chain is complete end to end: learn → verify → generate Marco → persist → resolve →
authorize → execute → verify. Before building anything else on it, run the saved independent
adversarial audit against the whole chain.

---

# Visibility — one account, three readings (2026-08-10)

Director passed its adversarial audit and was frozen. The first blocker to real-world testing
was not capability; it was that a person watching Marco could not see what it believed or why.

Marco had ten diagnostic surfaces and no way to say what it was doing. `status`, `world`,
`events`, `perception`, `explain`, `observation`, `live-analysis`, `observation-events`,
`learned`, `game` — each answers a specialist's question precisely, and a front-end that wanted
to say *"I recognise this as the pause menu"* had to poll four of them and join the results
itself. The overlay's Director panel did exactly that, which is why it showed provider counts
and fusion totals: those were the questions it could answer without joining anything.

## What was built

**`pkg/playbill`** — the canonical read-only account. CURRENT, SEEING, THINKING, LEARNING,
DOING, the one pending question, WHY, and a bounded semantic timeline; plus an opt-in
DIAGNOSTICS section. Stdlib only, and it imports no `internal/` package, so a presentation has
no path to the analysis it describes.

**Three readings, one value.** `View.Normal()` is one word and a sentence, `View.Watch()` is the
panel, `View.Deep()` is `Watch()` plus evidence — all in the shared package, so the overlay and
`marco director watch` cannot disagree about what a sentence says.

**`PLAYBILL`, protocol v6.** Composed in two halves that stay apart on purpose:
`Runtime.Playbill` (cmd/director) owns perception and the learning lifecycle, `Server.playbillFor`
owns the command half. Merging them would hand the layer that observes the desktop a reference to
the layer that drives it.

**The overlay** now decodes into `playbill.View` itself (a `replace` on the main module, as
plugins/ocr and plugins/vision already do) rather than into a mirrored struct. `watch` opens
Watch, `diagnostics` opens Diagnostics, `perception` keeps the old frozen per-element snapshot.
The always-visible hint line is the NORMAL reading of the same value.

## What live testing found in the first ten minutes

**The timeline flooded.** A first look at Chrome produced thirty near-identical moments in one
instant — a hypothesis per candidate region, all created together. Two fixes, both presentational:
runs of the same sentence collapse to `×N`, and `ValidationRecommended` stopped being a moment at
all (it is advice, already carried on the reading in THINKING; a timeline is what *happened*).

**"I can't see" was the wrong sentence.** Watch said Marco could not make out the screen while
accessibility was feeding 687 observations a cycle. Both were true — screen recognition runs on
the structural detector, and accessibility is not it. Now: *"I can't tell one screen from another
here"*, with `Diagnostics.Structure` naming why.

**A count without its denominator.** An early SEEING mixed a per-screen number with a
whole-session one and rendered *"204 of 5 things have a name"*. Every field in `Seeing` now maps
one-to-one onto something the session already counts.

## What it cost

~11 µs per read, ~16 µs with diagnostics, ~2 µs to render, ~2.6 µs to digest — against a
perception loop that samples every 500 ms and spends 170–730 ms of it on detection.
`cmd/director/playbillcost_test.go`.

## Constraints

[[ADR-033-one-account-many-presentations]], [[ADR-034-visibility-grants-no-authority]],
[[ADR-035-uncertainty-survives-the-screen]]. Subsystem note: [[Visibility]].

## NEXT

Unfamiliar-software testing. Watch is sufficient to see what Marco is doing; what it reveals
first is that screen recognition depends entirely on the structural detector, and that detector
did not run against Chrome or VS Code in any of the live checks above. That is the next thing to
look at, and it is a perception question rather than a visibility one.

---

# Making real applications become screens (2026-08-10)

Watch found the first unfamiliar-software blocker within minutes of shipping, and the
diagnosis it produced was accurate about the symptom and wrong about the cause:

```
accessibility  687 obs
fusion         687 obs -> 685 elements
observation    0 screens   0 transitions
    recognition: no structural detector ran
```

Two independent defects, and the second is why the first took so long to see.

## Defect 1 — screen segmentation had exactly one evidence source, and it was opt-in

`ScreenSegmenter.Observe` was called from exactly one place, fed `Sample.Shadow.Regions` —
the EXPERIMENTAL detector's output. The detector is opt-in behind `$MARCO_SHADOW_VISION`
because it costs ~1.25 GB.

So the default Director had **no screen model for any application**, and the whole learning
stack above it — recognition, relationships, demonstrations, rehearsal, plays — was reachable
only through an experiment. `Sample.Entities` carried the same window-relative geometry and
the same role vocabulary the segmenter wants, and there was no path from it.

Fixed at the earliest wrong layer: the sampler→session evidence boundary. `Sample.Structure`
is now a provenance-carrying composition; `StructureOf` prefers the fused authoritative world
and falls back to the detector where fusion saw no structure. See
[[ADR-036-a-screen-is-a-composition-not-a-provider]].

## Defect 2 — vision's opt-in was enforced on one door of two

`Observe` checked `req.Includes(SourceVision)` and said so in a comment. `ObserveTargeted`
did not, and the collector prefers `ObserveTargeted`. Every ordinary cycle therefore ran a
vision pass — a screen capture — that nobody asked for, and reported the resulting window
failure as a degradation in every live diagnosis this project has ever printed.
[[ADR-037-opt-in-is-enforced-on-every-door]].

## What the fixtures had that reality did not

Every screen-segmentation fixture hand-built `ShadowSample{Ran: true, TargetProven: true,
Regions: …}`. All of them encoded "a proven structural detector exists". None exercised the
default deployment, and none supplied accessibility structure — because there was no path
for it. The regression fixture was written first, watched fail, and then fixed.

## What the mutation gate found

Eight mutations, all killed — **after** two survivors exposed real coverage gaps:

- **the screen model published only when the experiment ran.** Every test had either a
  detector on every slot or no detector at all. Production has neither: the cadence gate
  declines most slots, so a Director with both sources held a screen model and reported none.
- **the unobserved gate deleted.** Nothing drove a session whose composition was unobserved,
  so "unknown must not become an empty screen" was unenforced.

## Live result

Chrome, VS Code, File Explorer and Task Manager all now form one stable screen from
accessibility alone — 52 to 128 stable structures, real role vocabularies, classified
interface terms, and hedged hypotheses. `vision: no window to look at` is gone from every
diagnosis.

Discrimination at accessibility scale was **measured** rather than assumed: two 128-element
screens separate below ~63% shared composition and merge above ~75%. So the live "1 screen"
means nothing changed, not that everything matches.

## NEXT

Transitions were never exercised live — that needs one human UI interaction, and passive
observation cannot manufacture one. Everything else in the perception path is now validated
against real unfamiliar software.

---

# The transition substrate, without a human (2026-08-11)

Screens now form on real software. The remaining question was whether the machinery immediately
downstream of them is ready for the first human demonstration — asked and answered entirely
through deterministic analysis, because the user was unavailable.

## The realistic fixtures

`internal/director/observe/screenfixture` holds compositions shaped from what the live sessions
measured: 128 structures over twelve roles (VS Code), 52 over eight (Chrome), 12 over two (Task
Manager). Role and window-relative geometry only — there was never any content to redact.

Building them found the first thing: a fixture that scattered its list items over a lattice
produced screens Marco could observe and never remember. Real trees have vertical runs, and a
structural group is what earns a screen a hypothesis.

## Defect — a session bound sized for the wrong evidence

`MaxActiveTracks = 128` was exactly one realistic accessibility screen. A two-screen session
evicted **1,460 tracks** and produced structure for one screen out of two — so on real software
every session could see a transition and none could ever remember one.

Raised to 1024/2048 with the arithmetic written down, and measured: 328 µs per inference at 716
tracks against a 176–298 ms perception cycle. [[ADR-038-session-bounds-are-sized-for-the-evidence]].

## Defect — an arbitrary string supplied as a ROLE reached the durable store

A role is a kind, and nothing checked that. A label arriving in that field entered the screen
signature, the track table and from there `StructureSignature.Roles`, which is stored forever.
Now refused on shape at the single point every structural region passes — shape rather than an
allowlist, because real trees report `link`, `combo_box`, `progress_bar` and `scroll_bar` and an
allowlist would have discarded most of an accessibility tree while looking careful.

## Defect — Watch leaked a durable subject id

`Learning.Silence` forwarded a developer-facing rendered line that names subjects, putting
`subj_ad7cea89aecd` on a Watch panel. The closed `LearningRefusal` vocabulary is now carried
beside the prose and translated; the prose no longer crosses.

## Two scale limits recorded and NOT changed

Both are the same shape as the screen-model defect — a detector-scale constant meeting
accessibility-scale evidence — and both need a measurement of real software:

- **`StateMatchSimilarity = 0.55` cannot serve both sources.** At accessibility scale a real
  interface change scores 0.693 while harmless churn scores 0.882; the detector's own frames of
  the *same* screen score 0.71–0.79. Measured: only a full content-region replacement is
  separable today. A command palette (0.883) is indistinguishable from churn.
- **A screen can only be remembered if text was read.** `Discriminating()` counts terms or an
  envelope; a whole screen has no envelope. So whenever a scoped reading does not happen — about
  five samples in six, by design — the screens seen in that window can never become durable.

## What the substrate does

Everything below the two limits works, through the production path, at realistic scale:
stability under jitter/churn/reorder, transitions, unknown→unknown, attribution separated from
existence, staleness refused, context-admitted evidence kept weaker, recurrence counted across
real session boundaries, and the adversarial four-key variant correctly refused with
`navigation_too_weak`.

Twelve mutations, all killed — three only after survivors exposed real coverage gaps (a held key
inflating a correlation, an off-by-one in Watch's own count, and an over-sensitive identity).

## NEXT

One thirty-second human interaction. See the milestone report; the interaction is a VS Code
**Settings tab**, not the command palette, because the palette is the case the identity model
currently cannot see.

---

# Roadmap 34 correction — Learn becomes goal-centric (2026-08-17)

The live-training sessions had turned into test-rig choreography: exact starting screens,
returns to START, waits for grounding boxes. The correction is to the learning model itself,
not to its staging. `Learn(A → B)` used to memorise a route and weld the capability to A; now
the DESTINATION is the capability, the demonstrated route is evidence for one known way in,
and planning composes verified edges from wherever the person currently is.

## The model

```
Learn "open mouse settings"
    demonstration:  A --x--> B --y--> C
    Marco learns:   goal = C                       (Goal record; structurally no start)
                    edges A→B, B→C                 (one candidate each; durable topology)
    later, from B:  plan = B→C                     (never a walk back to A)
    later, from X:  plan = X→B→C if X→B is known
    no known chain: "I know what you want to reach, but I don't yet know
                     how to get there from here."
```

`observe.Goal` + `GoalStore` (semanticmemory), `observe.PlanToGoal` (BFS over the topology,
usable-edge predicate), `director reach` (read surface over rehearsal-verified edges, honest
`known_unverified` middle case). [[ADR-056-a-goal-is-a-destination-not-a-route]].

## Capture first, interpret second

The live run's worst failure: the person clicked and Marco threw the click away, because a
later layer could not interpret the surrounding state. `ShadowTotals.InputLog` now banks every
admitted input event BEFORE any gate — structural return, quiet expiry, buffer consumption —
with the context known at the time. Interpretation failure is no longer capture failure.
[[ADR-057-attributed-input-survives-interpretation]].

## Clicks resolve to controls, at event time

Each valid inference pushes the window's actionable controls (bounds, role, admitted label,
focused) to the navigation producer beside the frame it already pushes; the worker resolves a
placed press to the smallest containing control and a confirm to the focused one. The label
gate is ONE function beside the classifier (`observe.AdmittedTargetLabel`): the canonical
plaintext role allowlist, or — during an explicit Learn pass — the activatable control the
person's own event landed on, shape-filtered either way. The one deliberate privacy widening
of the milestone, argued in [[ADR-058-a-demonstrated-target-may-keep-its-name]] and named in
both type-allowlist tests and the candidate sweep.

Downstream: session transitions carry `TargetedSequence` (durable topology strips to plain
`NavSequence` structurally); candidates keep their steps' targets; a resolved click assesses
`candidate_consistent`; the rehearsal lowers it to `Accessibility's Invoke` on the control
resolved live at emission time (`rehearse.ControlResolver`, uniqueness demanded). A saved play
still refuses `cannot_say_pointer` — run-time name-resolving activation is follow-on.

## Why live rehearsals "never fired"

Three defects, all fixed and tested:

1. **They fired and always failed.** `StepRecord.Application`/`Source` were never populated,
   so every step's outcome recalled against the EMPTY application and classified
   `unrecognised` → `stopped_at_step`, unconditionally, on every application. Invisible to
   tests because the memory fake ignored the application; the fakes are now scoped and the fix
   is mutation-gated (`TestALiveStepIsClassifiedAgainstItsOwnApplication`).
2. **Input has no address.** No foreground gate existed, so the keys landed in the terminal
   the person typed `director answer ... yes` into. A real attempt now refuses
   `window_not_in_front` BEFORE the grant is claimed (patient case upstream), re-checks before
   every step, and `WaitingForStart` is paced (2s) instead of a flat-out perception loop.
   [[ADR-060-input-has-no-address]].
3. **A yes could create nothing, silently.** Every authorization dead-end was a silent return;
   ten minutes later teaching blamed the person (`rehearsal_declined`). The runner now records
   a closed `AuthorizationRefusal`, the tail carries it up (`teach.GrantDiagnoser`), and the
   timeout names the real cause.

## Grounding got a lifecycle

Highlights were fire-and-forget processes with no dismissal path anywhere; every bare
`director teach` status read re-launched stale boxes, and concurrent surfaces adopted each
other's window (shared title). Now: each grounded endpoint carries `Current` (phase-windowed;
nothing is current once the session settles), the follower draws only current claims and
kills what it drew when the claim ends, and `marco-show` titles its window per-pid.
[[ADR-059-a-presentation-belongs-to-its-claim]].

## Also written down

ADR-053/054/055 (the 2026-08-13 one-shot corrections) existed only as code comments; they are
real notes now, marked as recorded retrospectively.

## Mutation gates run

input-log bank deletion; goal-write no-op; per-edge candidate storage skip; grounding
`Current` copy drop; rehearsal step-scope assignment deletion. All killed by named tests.

## Known gaps

- Click routes are learnable, rehearsable, plannable — not yet saveable as plays.
- Multi-edge plans are knowledge; execution is still per-edge through saved plays.
- The armed-capture fallback still requires the person to be recognised at the terminal
  edge's start; the one-shot watched pass is the primary path and does not.
- The live acceptance run (real Settings, real click) needs human hands — see E2E.md.

## Closeout

```
LEARNING_MODEL:                      GOAL_CENTRIC
DEMONSTRATION_IS_CAPABILITY:         NO          DEMONSTRATION_IS_EVIDENCE:  YES
GOAL_REQUIRES_ORIGINAL_START:        NO (structurally; test-enforced)
RAW_ATTRIBUTED_INPUT_CAN_BE_DROPPED: NO (InputLog banks before every gate)
CLICK_CAPTURE:                       YES         CLICK_TARGET_RESOLUTION:    YES (event time)
KEYBOARD_TARGET_RESOLUTION:          YES (confirm → focused control)
MULTI_EDGE_DEMONSTRATION_DECOMPOSED: YES         REQUIRED_WAYPOINT_INFERRED: NO
CURRENT_STATE_TO_GOAL_PLANNING:      YES (PlanToGoal + `director reach`)
ALREADY_AT_GOAL:                     satisfied   UNKNOWN_ROUTE:              honest refusal
REHEARSAL_AUTHORITY_CHANGED:         NO          PATIENT_REHEARSAL:          preserved + extended
STALE_GROUNDING_CAN_LINGER:          NO          USER_CHOREOGRAPHY_REQUIRED: NO
VISION_CHANGED: NO   OCR_CHANGED: NO   CORE_V1_CHANGED: NO   AUTHORITY_CHANGED: NO
PRIVACY_CHANGED:  YES — explicitly (ADR-058, the admitted target label)
LEARNING_SEMANTICS_CHANGED: YES — explicitly, route-centric → goal-centric (ADR-056)
```

Everything above is proven deterministically: 76 packages green, docscheck clean, five
mutation runs killed by named tests.

## NEXT

The live acceptance run — the one gate that needs a person, because foreground activation
and injected-input attribution are refused by design. E2E.md § "Realistic Learn acceptance"
(~5 min): Learn "open mouse settings" with one ordinary click; answer yes from the terminal
and leave the terminal in front (verifies the foreground gate); `director reach` from
somewhere that is not the demonstrated start. Roadmap 34 closes on that run reading as
normal computer use.

---

# Live E2E round 1 — what the cold service found (2026-08-17)

Everything below ran against a real `director serve` on an isolated cold `$MARCO_HOME`, with
live Windows Settings as the target. No injected input: the demonstration half still needs
human hands, so this round proves the parts around it.

## The blocker the deterministic suite could not see

`director teach "open mouse settings"` — **the phrase in the roadmap, in E2E.md and in the
milestone's own statement of what a normal person should be able to say** — was refused
before anything was observed:

```
director: "open mouse settings" is 3 word(s), and a play is a sentence of two …
```

`playNameFor` demanded the phrase be exactly two words. The unit tests passed because they
asserted the PARSER'S RULE (`"open the downloads folder"` → refused) rather than the
acceptance criterion. A test can encode a policy perfectly and still be testing the wrong
thing; only running the sentence a person would actually type showed it.

It was also the milestone's own failure mode: telling somebody their outcome must be phrased
in exactly two words, or that they must learn `--actor`/`--verb`, is Marco's protocol leaking
into the user's sentence.

**Fixed**: first word is the verb, the REST is the actor (`open mouse settings` →
`do MouseSettings's Open`; `MuteEveryone` is `CheckPlayName`'s own example of a legal actor).
One word is still refused — it cannot be a sentence of two — and still at the request
boundary, before anybody demonstrates.

The original objection to welding is answered rather than overruled. It was written when the
phrase WAS the play's name; under the goal-centric model the phrase is the OUTCOME's name,
kept verbatim on the durable goal, and the play name is a separate artifact. So the
derivation is **said out loud** — `teachView.WillBeCalled`, printed before the demonstration:

```
If this works out I'll write it down as `do MouseSettings's Open`.
  (say it your way with --actor and --verb)
```

## What the live round confirmed

- **Cold service, honest refusals.** `reach` on a Director that has observed nothing:
  *"nothing has been observed yet"*. After observing: *"I haven't learned any outcomes here
  yet."*, and an unknown outcome names itself in the refusal. Exit codes 1/0/1.
- **Perception against real Settings.** A 20s targeted session took 35 samples, accessibility
  proved its target on every one, one screen formed from 24 grouped controls, four hypotheses
  with their contradictions. Live role vocabulary: 12 `text`, 4 `list_item`, 4 `button`,
  3 `group`, 1 `list`.
- **`pushActionables` has real evidence to push.** Eight of those persistent controls are
  `Clickable()` roles, and the nav items are `list_item` — exactly the case the demonstration
  licence in [[ADR-058-a-demonstrated-target-may-keep-its-name]] was written for (nameable
  only under Learn). A click on a Settings nav item has something to resolve to.
- **The choreography is gone, and this is the headline.** On a COLD store — Settings never
  established, no question ever answered about it, no `show-me` warm-up — `teach` reached
  *"Okay — go ahead and show me"* and grounded the start (16 controls). Under the old model
  this refused at its first step with `start_not_recognised`.

## Still needs human hands

The demonstration itself, the rehearsal firing into the right window, and reuse from a
different start. E2E.md § "Realistic Learn acceptance" is the script.

## Round 2 — the highlight surface, and one thing left unexplained

The first teach run (`--no-highlight`) saw nothing change on an idle Settings. The second,
with highlights on, refused `destination_not_recognised`:

```
2 transition(s) were seen and none had two recognisable endpoints:
  both_unresolved=1 source_unresolved=1 bridge:endpoint_unresolved=1
```

The obvious suspect was Marco's own presentation perturbing Marco's own observation: UIA
reports occlusion through `IsOffscreen`, so a topmost box over the watched window could
change what accessibility says about it. Owned-surface exclusion covers the VISION path and
would not cover this.

**Tested, and refuted.** A controlled 25s observation of Settings with a `marco-show` box
held over it for 12s produced one screen (`state_1`), no new states and no transitions —
identical to the box-free baseline. Accessibility-derived composition is not perturbed by
an overlay. The surface also behaved: `FOREGROUND_HWND_UNCHANGED: YES`,
`COMPANION_RECEIVED_POINTER: NO (0)`, click-through styles all set, and its window found by
the new per-pid title.

**So the two transitions are unexplained.** Not reproduced, cause not established, and
recorded here as an observation rather than a conclusion — the same discipline the
desktop-hitch note in CLAUDE.md follows. Candidates not yet separated: genuine live content
in Settings home over a 45s pass, or focus-related churn. Worth watching for during the
human run; if `destination_not_recognised` appears on a real demonstration, `--watch` names
the cause and this is the first place to look.

**What this did prove about ADR-059**: the dismissal path was NOT exercised live. The
surface exited ~3s in for its own safety reason (`p.blocking` — it found itself in front of
a window that was not the foreground app, which is correct behaviour when teaching a
background window). A live dismissal check needs Settings actually in front, so it belongs
to the human run. The lifecycle itself stays covered by the mutation-gated unit tests.

**Consequence for the human run**: have Settings genuinely in the foreground. Teaching a
background window works for perception but makes the highlights bail out immediately.

---

# Live E2E round 2 — three defects fixed, and the wall behind them (2026-08-17)

Five live Learn attempts against real Windows Settings, with a person clicking. Each one
found something the deterministic suite could not, and the last one found the wall.

## Fixed: every mouse click on a default Director was discarded

`pushNavContext` was gated on the SHADOW sample having run. Vision is off by default, so
`shadowSampleFor` returns nil, `ensureShadow` supplies `{Ran: false}`, and the gate returned
on EVERY cycle — the navigation producer never received a window frame, so
`windowBounds.fresh` was false forever and every pointer press became `unplaceable_pointer`.
Live: `received=2 classified=0 unplaceable_pointer=2` for the two clicks the person made.

The same shape as the 2026-08-10 defect where screen segmentation had one evidence source
and it was opt-in. A window's rectangle is a fact about the WINDOW, from the reference the
runner validated — not from an experiment. Ungated; `pushActionables` had inherited the same
gate, so click-target resolution had never once run live either.
`TestADefaultDirectorGivesThePointerAFrame`, mutation-gated.

(Also confirmed by the same run: the desktop spans X −1920→+1920, and presses on the OTHER
monitor were correctly refused. Not every `unplaceable_pointer` is a defect.)

## Fixed: the cause and its effect arrived in different samples

With clicks placed, both transitions still came out `unattributed 2/2`. Attribution required
the input and the screen change to land in the SAME inference — true of a keyboard menu that
flips on key-down, false of a Settings page that takes a beat to render. So the click was
drained on one sample and the change was visible on the next, and the route was discovered
and unlearnable (`no_attributed_navigation`).

A run now survives ONE quiet inference and is still offered to a change on it, bounded by
`TrackAbsenceTolerance` — which already means "one dip is not a disappearance", not a number
invented for the occasion. `TestAnOldKeypressIsNotForcedOntoALaterEdge` still passes: its gap
is deliberately three. `TestAChangeOneInferenceAfterTheClickIsAttributedToIt`, mutation-gated.

## Fixed: a failed Learn made its own name unusable

`RememberGoal` refused a name already bound to another subject. A goal written by an earlier
FAILED attempt therefore blocked re-teaching under the same words — the person punished for
Marco's failure. Now it REBINDS, which keeps the one-name-one-outcome invariant `reach`
depends on while letting a person say what they mean now. The same rule `NameSubject` already
follows for screens: renaming is ordinary and drops the old name.
`TestTeachingTheSameNameAgainRebindsIt`.

## THE WALL: screen identity does not survive a scroll bar

Five attempts minted **five durable subjects for the same Settings pages**. Two recordings of
the Home page, field by field:

```
run 2:  button=15 combo_box=1 group=20 image=14 link=1 list=2 list_item=22
        menu=1 menu_item=1 pane=3          text=32 text_field=1 unknown=2 window=4
        terms=[back+settings]  envelope=none
run 5:  button=16 combo_box=1 group=20 image=14 link=1 list=2 list_item=22
        menu=1 menu_item=1 pane=3 scroll_bar=1 text=32 text_field=1 unknown=2 window=4
        terms=[back+settings]  envelope=none
```

Terms agree. Every shared role is within `RoleCountTolerance` (button 15 vs 16). The single
decisive difference is that **a scroll bar appeared**, and `CompareStructure` opens with:

```go
// The same roles must be present. A screen that gained a whole role — a progress bar, a
// text field — is a different screen, not a jittered one.
if !sameRoleSet(current.Roles, remembered.Roles) { return MatchDifferent }
```

That rule is right for the detector's small vocabulary, where a progress bar appearing is a
real event. At accessibility scale — fourteen roles, hundreds of elements — a scroll bar is
chrome that appears whenever content is a pixel taller. So the same page is a different
screen, no endpoint resolves, no edge forms, and Learn cannot complete on mainstream
software however well everything above it works.

It is the same FAMILY as the documented open debt in [[Semantic-Memory]] ("a durable Envelope
can strand a subject"): identity criteria chosen for one evidence scale, meeting another.

**Deliberately NOT fixed here.** That note says fixing the matcher "must not be done
opportunistically — it is screen identity, and every remembered subject depends on it", and
loosening `sameRoleSet` mid-E2E to make a test pass is the definition of opportunistic. The
risk runs the other way too: this design's stated worst failure is over-merging two screens
that differ only in what their text says. It needs its own task and its own measurement
across several real applications, the way [[Experiment-011-two-level-identity-against-real-software]]
was done.

**Second-order consequence worth stating**: because identity fails, every attempt appends a
new subject. Five runs left seven subjects for what are three real screens. A store that
accumulates a subject per attempt is a store that gets slower and less discriminating the
more it is used.

## What IS proven live, above identity

Name derivation and its announcement; cold start with no prior establishment; both endpoints
grounded with correct on-screen rectangles; the click placed, resolved and admitted; the
route discovered; the goal remembered and rebound; every refusal honest, named, and pointing
at its real cause through the closed vocabularies. The goal-centric machinery works as far as
the identity layer lets it reach.

## Round 3 — the click vertical works live, and what still does not

Ten live Learn attempts against Windows Settings. The headline, from the tenth:

```
input: received=2 classified=1 pointer_resolved=1 pointer_unnamed=0
       pointer_unresolved=0 controls_offered=41
```

A person clicked a Settings nav item. The press was placed inside the watched window,
matched against 41 offered controls, resolved to the smallest one containing it, and its
name was admitted under the Learn licence. **The raw-click → placed → semantic-target
vertical of [[ADR-058-a-demonstrated-target-may-keep-its-name]] works against real software.**
Nothing above the identity layer is now blocking a click-driven demonstration.

Getting there took two more fixes and two diagnostics, all found live:

- **Every click was discarded on a default Director** (frame gated on the vision experiment)
  and **cause and effect landed in different samples** (attribution required them to share
  one). Both recorded above.
- **`unplaceable_pointer` was reported for presses made while nothing was watching.**
  Placement was checked before session membership, so a click made before a session started
  read as "Marco could not place your click" — a perception fault — instead of "you clicked
  before I was watching". Six such presses sent one diagnosis in entirely the wrong
  direction. `TestAPressWithNoSessionSaysSoRatherThanBlamingPlacement`.
- **A resolved-but-unnamed press read as unresolved.** A list item's text is not on the
  plaintext allowlist, so passive observation withholds it — correctly. Reporting that as
  "landed on nothing" reads as a coordinate-space fault. The outcome is now three-way:
  `pointer_resolved` · `pointer_unnamed` · `pointer_unresolved`, beside `controls_offered`.
  Those two numbers are what turned a week of guessing into one measurement.

### The remaining wall is still identity, and it has a second face

The chrome fix ([[ADR-062-a-scroll-bar-is-not-a-screen]]) removed one cause. Another remains,
and it is the one `PlaceToEstablish`'s settled check was already written against: **Windows
Settings renders in stages**. Across ten runs the same Home page was fingerprinted with 24
members and with 10, and `MemberTolerance` is 1. Runs alternate between resolving both
endpoints and resolving neither — `both_unresolved=2 destination_unresolved=1
source_unresolved=1` on the tenth.

So a demonstration is captured, attributed and resolved, and then has nowhere durable to
attach about half the time. Deliberately not fixed here for the same reason the scroll bar
was fixed narrowly: this is the identity matcher, every remembered subject depends on it, and
the honest next step is the measurement [[Experiment-011-two-level-identity-against-real-software]]
did — several applications, repeated visits, quantify what actually fluctuates — rather than
another tolerance chosen mid-E2E.

### Process note, recorded because it cost the most

Ten live attempts were used as a DEBUG loop. The memory note *live play confirms, it does not
debug* says exactly why not to, and it was right: each attempt cost a person several minutes,
produced one bit of information, and half the failures were mine (a background terminal the
user could not see, a store I corrupted with a BOM-writing PowerShell cleanup, a trigger that
started the clock before they had read the instructions). The two counters added at the end
answered in one run what nine had not. **Instrument first, then look once.**

---

# Light Mode: a Sight surface, and what identity measurement actually found (2026-08-17)

Two things, in the order the campaign required them: measure before touching identity, and
build the surface that makes the next round of measurement cheap.

## `director light` — what Marco's Accessibility brain currently understands

A live reading of the canonical account. It exists because the alternative was archaeology:
several live Learn rounds chased the wrong layer because the number that mattered was on no
surface at all.

```
Watching — I'm watching applicationframehost.

  I CAN ACT ON
    40 controls I could aim at
      System · Back · Home · Bluetooth · Add device · View all devices · …
      26 more I'm not allowed to name here
    focused: Bluetooth

  SEEING
    119 things are holding still here
      button, combo_box, group, image, link, list, list_item, menu, …
      words I know here: back, settings

  THINKING
    This might be a settings screen. …the evidence points both ways.
```

- **One account, not a second one.** It renders `playbill.View` through the SAME
  `Normal`/`Watch`/`Deep` readings the overlay uses, so a terminal and a surface cannot
  disagree about what Marco is doing. `--debug` is the existing Deep reading.
- **A read, structurally.** The whole command is `Client.Playbill`: no session, no sample, no
  answer, no memory, no authority. Refreshing faster changes nothing about what Marco
  believes.
- **New section: `Offers`.** The one part of the playbill that carries observed interface
  text, and it introduces NO new permission — every name passes
  `observe.AdmittedTargetLabel`, the single policy [[ADR-058-a-demonstrated-target-may-keep-its-name]]
  already defined: plaintext role allowlist, widened to activatable controls only under a
  Learn licence, shape filter either way. A withheld name is listed by role and COUNTED, so
  the gate shows its work rather than looking like perception failing.
- Guarded like everything else that carries text: `admitOffers` bounds the list and holds
  names to the same shape rule a screen name gets.

**It found a defect in its first live reading.** It reported *"1 control I could aim at"*
while watching a Settings window offering forty: a session fuses its own world for its pinned
window and never touches the foreground pipeline's, so the surface was confidently describing
a different window. `Runtime.lastWatched` now carries the session's world and the surface
reads whichever is fresher. `TestLightModeDescribesTheWatchedWindow`, mutation-gated — as is
the label gate, which was bypassed on purpose and killed by five tests.

**Privacy, verified live rather than assumed.** Windows Settings exposes the signed-in account
as a `button` whose label is a name and an email address. The role allowlist passes it; the
shape filter refuses it. Zero occurrences of `@`, the address or the name in the surface
output. `TestSightWithholdsPrivateTextEvenFromAnAllowlistedRole`.

## Identity: measured, and the leading hypothesis retired

[[Experiment-014-identity-variance-across-real-applications]] has the full record. The two
results that matter:

**Same-place revisits are STABLE where it counts.** Six independent visits each:

| Settings | 15/15 same | VS Code | 15/15 same |
|---|---|---|---|
| Chrome | 4/15 | Discord | 10/15 |

Settings — the application that failed live — produced byte-identical signatures every time.
Chrome and Discord vary in content-bearing roles because they were displaying live content.

**The "24 vs 10 members" premise was wrong, and reading the producer retired it.** A screen
state's signature is `Roles + Terms` only: `stateFingerprint` never sets `Members` or
`Envelope`. Those figures came from the GROUNDING line, which reports the dominant structural
group's size — a quantity durable identity never reads.

**What remains, and it is an evidence-completeness question, not a tolerance one.**
`settledComposition()` requires two observations and every role's modal count seen twice.
That is *"the composition has a stable mode"*, not *"this is the complete composition"*. Over
a 13-sample dwell the mode is the finished page, which is why idle visits are stable. Over the
few samples a place gets while somebody navigates through it — which is every live failure so
far — the mode is whatever was on screen for those frames.

`RoleCountTolerance` unchanged at 1, and nothing app-specific added.

## Method defect, recorded rather than buried

The short-visit batch is void: the driver read the session list before a new session
registered and captured the previous session's id (`inf=41` for a 2s visit is impossible).
Duration independence is therefore NOT yet measured, and it is precisely the measurement the
settle hypothesis needs. Batch 1 is sound — ids increment monotonically and every file's
application matches its target.

## 2026-08-17 — the two-step Learn: two stacked causes, both found by replaying the failure

**Where this started.** The cold-store live Learn (`Settings Home → Bluetooth & devices →
Mouse`) refused with `destination_not_recognised`, having captured, attributed and semantically
resolved the whole demonstration. Everything worked and nothing was learned.

### Cause 1 — a pass remembered only the place it ended on

All three screens settled and carried terms; all three passed every quality gate. Only Mouse
became durable, because establishment considered `ShadowTotals.CurrentState` alone. Both edges
therefore had an endpoint nothing could resolve:

```
Home      → Bluetooth    destination_unresolved
Bluetooth → Mouse        source_unresolved
```

`observe.PlacesToEstablish` now returns every settled place, each clearing the SAME four gates
independently, in first-sighting order, bounded by `MaxCheckpoints`.
[[ADR-063-a-pass-remembers-every-place-it-settled-on]] — which amends exactly one bound of
ADR-047 and reopens nothing else about it.

### Cause 2 — and the first one was hiding it

Fixing establishment moved the report from `both_unresolved=2` to `destination_unresolved=2
source_unresolved=2` and still produced **zero edges**. The walk was:

```
state_1 → ? → state_2 → ? → state_3
```

Every real navigation crosses a frame Marco cannot place. `Transitions` is keyed by
`(From, To)`, so two excursions aggregate to two entries into `state_unknown` and two exits out
of it, and `A→C, B→B` fits the same counts as `A→B, B→C`. Bridging refused
`ambiguous_interval` and lost **both** adjacencies. At ordinary human speed that is the common
case, not an edge case: the one-shot multi-step Learn was impossible except at one step per
session.

`ScreenSegmenter` now keeps the session's **walk** — `ShadowTotals.Crossings`, every change in
order, written at the one call site every change already passes through, thin on purpose (no
navigation, no counts; those stay on the aggregate). `unsettledIntervals` reads it, so each
excursion has one entry and one exit by construction. `A → ? → C → ? → B` yields `A→C` and
`C→B`: C stays in the middle rather than being refused OR skipped.
[[ADR-064-the-order-of-a-walk-is-evidence]].

### How both were found without a person at the keyboard

`identityprobe -replay` runs a recorded session through the PRODUCTION establishment and
relationship path against a cold store in a temp directory — `PlacesToEstablish`,
`EstablishPlace`, `RelationshipsFrom`, no reimplementation of any gate. It printed the three
places establishing and zero edges in one pass, which is what separated cause 2 from cause 1.
Recordings that predate the walk say so rather than reading as a finding.

### Gates

`TestEveryPlaceOnTheRouteBecomesDurable` and `TestAnUnlicensedPassEstablishesNoPlaceHowManyItSees`
(the licence did not widen, only the count). `TestATwoStepWalkLeavesTwoDurableEdges` and
`TestAWalkThroughAMiddleScreenIsNotShortened` run the whole thing through the real session.
Mutations run and restored byte-identically: narrowing `PlacesToEstablish` to the current state;
making the runner's loop skip non-current candidates; disabling the crossing log; refusing more
than one interval (which reproduces the live failure exactly). The two real captured Settings
sessions still replay — their walk is reconstructed only where the aggregate leaves one possible
ordering, and the fixture fails rather than guesses otherwise.

**Not yet confirmed live.** The recorded session cannot answer this one: it predates the walk,
and with two entries and two exits its order is genuinely unrecoverable. It needs one
demonstration on a cold store.

## 2026-08-17 (later) — the Learn control surface, and the CLI defect that forced it

**The redirect.** Live Learn acceptance had failed too many times for reasons that were not about
Marco: wrong foreground because the person returned to the terminal, session ids required for
product actions, CLI flag ordering, PowerShell harnesses, needing to know when Marco was armed.
The last one was decisive — `director rehearse --live` could not be invoked at all — and the
conclusion was that Learn was being exercised through the wrong interface.

### The CLI defect, fixed and audited

`runRehearse` declared `-step`, `-live` and `-json`, parsed them, and then handed the SAME
unconsumed slice to `observationQuery`, whose flag set knows only `-json`. Every invocation
carrying a flag died with "flag provided but not defined: -live", so the only path to learned
input was unreachable — while `director rehearse` with no flags worked and made the command look
healthy. Found while holding a rehearsal grant that could not be spent.

Fixed by separating the parse from the request (`observationRequest`), which makes the shape
unavailable rather than merely corrected. Two tests: `TestRehearseArgumentsReachTheLivePath` over
every shape a person types, and `TestNoCommandParsesItsArgumentsTwice`, which walks the package's
AST — including one level of delegation, because the first version was evaded by splitting the bug
across two functions.

The audit found a second, larger instance of the same silence: `flagsFirst` reorders arguments
before the flag package sees them, so a value-taking flag missing from the `valued` table has its
value read as the wrong token. It had bitten four times. **Twenty-two more were latent.**
`TestEveryValueTakingFlagIsInTheReorderingTable` now holds the whole package to it.

### Operating Marco is not demonstrating to it — ADR-065

Two halves of one cause. Pressing Start brings Marco to the front, so a session that resolved its
window then pinned the control panel; and the button presses themselves are real clicks inside the
watched rectangle, because the overlay lies *over* it.

`observe.IgnoreMarcoOwned` joins the closed vocabulary; `SetSurfaceOwner` is pushed by the
composition root beside the window frame; `teach.WaitingForDemonstration` holds a surface-started
session until there is a subject that is not Marco. Doubt falls toward KEEPING the person's
evidence: with no ownership answer, or a stale one, input is admitted.

### Stop is a product event — ADR-066

A demonstration used to end on a forty-five-second clock or on the person happening to hold still.
Both are guesses about something the person knows. `Coordinator.Finish` ends admission, keeps
everything captured, and lets the ordinary pipeline run on a shorter pass — the observation layer
already defines cancellation as "stop early and keep the evidence".

### The panel

A **Learn** tab in the control centre (`marco ui`): name, Start Learning, live status, Stop
Learning, Try It, cancel, and what Marco currently understands — application, place, actions
captured, how many named a control, controls on offer, the transition, what it would be called.
`learnview.go` projects it from the coordinator's own session; the surface never parses prose and
holds no state. Every verb turns into a request the Director already served: Start is a teach,
Stop is a finish, **Try It is an ANSWER to the rehearsal question** through the same ledger, and
Cancel is a cancel.

Verified live over HTTP: Start → `waiting_for_demonstration`, goal derived, correct controls
offered, honest degradation when no Director is running.

### Known gaps, recorded rather than papered over

- The teaching owner publishes a session only when a step RETURNS, so during a long pass the panel
  shows the phase from before it. Adequate today (the sentence reads "go ahead") and worth fixing.
- Tab-out to an unrelated application (Part 6) is not yet distinguished from the target: ownership
  separates Marco from everything else, not the target from a third application.
- The panel does not yet show last-action / target-resolving / effect, and no live UI-driven Learn
  acceptance has been run.

## 2026-08-17 (34B) — the Theater: what a learned play refers to

A learned play was about to say `do Accessibility's Invoke with a Control called "Mouse"`. It
worked and it welded a **provider** into a **behaviour**. This is what it says instead:

```marco
use theater.
    the target1 is a Target with Name "Mouse", Kind "button".
    do Theater's Activate with target1.
```

**Theater is the production layer, not a renamed store.** Director decides what should be put on;
Theater knows the Repertoire, looks at the Stage, casts an Actor that can play the part tonight,
performs, and verifies. `semanticmemory` is the storage engine underneath the Repertoire — one
part of Theater, not all of it. No `theater/` package was created for the metaphor.

**Marco's `scene` is untouched.** It is a first-class language word guarded by spectest, and
Director's `screen_state` is a different thing that rhymes. No mapping, no homonym.

### Durable Target

`SubjectTarget`, through the canonical subject machinery — same match-or-append, same id
derivation, same bound. Identity is **label + kind + place**, matched EXACTLY: a name is not a
composition, and the tolerance that makes screens robust would merge two differently-named buttons.
A kind that disagrees is a disagreement; a kind that is *missing* says nothing, which is what keeps
an accessibility-trained target reachable by a resolver that has no opinion about control types.

Nothing provider-shaped can be in it. `TargetSignature` has no parameter for a runtime id, handle,
rectangle or coordinate, and a reflective test walks the type so a field added tomorrow is caught.

### Actors, not Players

An Actor is a class of capability that can find and perform. Marco's `actor` is a thing in the
play; a Theater Actor is a thing that can perform a part — same metaphor, opposite sides of the
curtain. Finding and performing are one interface because they are one competence: an Actor that
finds a control through the tree presses it there; one that finds it by sight points at it.

`Control.Called` is now an **Accessibility executor detail** and nothing else. It kept every test.

### What the mutations found

Twelve run, twelve killed — but three survived first and each exposed a real hole:

- **the play naming its provider** appeared to survive because the test result was CACHED. Every
  mutation run now uses `-count=1`. Worth remembering.
- **"unknown kind disagrees"** survived because nothing tested it. That rule is the whole of
  portability and it was documented in a comment and asserted nowhere.
- **the host bypassing the Theater** survived because every test drove `Theater.Activate`
  directly. A host answering `ok` without casting anybody would have passed the entire suite and
  activated nothing.

### The restart proof, in its strongest form

Not "it worked after one restart". The accessibility actor sends a **label** over the bridge on
every call and never an element id, asserted on the wire — so no runtime id is ever held across a
redraw, and every future restart is covered rather than the one a test happened to perform.

### Privacy

`RememberedSubject.Structure.Label` is perception-derived and crosses the line `Called` drew. The
exceptions are now a closed list with the licence written against each and their count asserted —
which also fixed something older: `Called` had been passing only because "called" happens not to
appear in the forbidden word list. Nobody had decided it was allowed.

## Roadmap 34C — grounded, reversible naming

The 34B live run ended on a question Marco could not ground: *"what do you call this screen?"*, with
two candidate screens and byte-identical wording for either. The person answered about the one they
meant; the word landed on the other. Then it could not be repaired — withdrawing the answer removed
the judgement and left the name, and uniqueness reserved the word against every other place
permanently. The screen they had actually meant could never receive it.

The repair, at the time, was: stop the Director, back up `semantic-memory.json`, edit it by hand.
That is the whole justification for this milestone.

### The two rules

> **Marco may not ask the Audience to name something it cannot ground for them.**
> **An audience-supplied name must be reversible.**

[[ADR-069-a-name-is-authored-and-can-be-taken-back]] has the full derivation. The short form:

- **Grounding is a description, not an identifier and not a highlight.** `KnownPlace.Describes`
  says what a place is made of ("about audio, 14 things on it"); `Handle` is opaque and never
  shown. Two places may not read alike — that is asserted, because identical descriptions
  reproduce the failure exactly. Boxing the whole accessibility tree would say nothing about which
  thing is meant.
- **The identity is bound before the question.** `AnswerName` writes to the subject the QUESTION
  named. A stale proposal names the screen it was raised about or names nothing.
- **Three operations, one subject id.** Rename does not mint a subject (every route pointing at
  the old id would be orphaned); unname does not delete one (removing what somebody CALLS a place
  says nothing about the place). An empty name is a **retraction**, not a validation error.
- **Uniqueness is over live names only.** A word nothing is currently called is free.
- **The namespace comes off the subject.** Deriving it from session context made the first
  implementation fail with `no remembered screen … in ""` — the same bug class, smaller.
- **No focus choreography.** The person is typing into Marco's own field, so Marco is necessarily
  in front. A foreground gate would make naming impossible.
- **Perception's word is not the Audience's word.** An observed label stays `Structure.Label` with
  `Learned` provenance and never becomes `Called`.

### Sight grew the two lines it was missing

`director sight` now says what Marco can **act on** here — the Theater's durable targets grounded
in the settled place, as label and kind — and what it **last did**, from the action graph. Both
lines are omitted when there is nothing true to put in them; a surface that prints "nothing" every
time trains a person to stop reading it.

### What the mutations found

Fourteen non-equivalent mutations, fourteen killed. Two were found to be **equivalent** and are
recorded as such rather than faked:

- **a rename with no place** is refused twice — once up front, once because an empty handle belongs
  to no subject. Neither guard is individually killable. The test now asserts the OUTCOME (no
  subject changed) instead of only that an error came back, and the redundancy is documented as
  deliberate. It was replaced in the count by the real live defect: deriving the application from
  session context instead of from the place, which kills four tests.
- **`p.Verdict != MatchSame` in `targetsHere`** restated `p.Subject == ""`: `Subject` is filled
  only for `MatchSame`, and every target is grounded in a real place id. The clause was removed
  rather than defended, and the test now asserts the structural property both halves rest on.

An honest equivalent mutation is worth more than a kill bought by weakening the code.

## Roadmap 34E — one production body

**Director is the brain; Theater is the body.** The audit that established which side of that line
each responsibility falls on is [[34E-director-theater-audit]]; this is what came of it. The
decision is [[ADR-070-one-production-body-and-the-caller-brings-the-verification]].

### The cause, not the symptom

The first live rehearsal after the Theater landed ended at
`cannot_express: the control does not implement InvokePattern` on every Windows Settings
navigation step. Settings is built almost entirely from *selection* items, so this was the most
obvious application anybody would teach Marco with. Phase 1 gave both paths the same activation
ladder (`internal/activate`, Invoke → Select → Expand → Toggle) and that fixed the symptom. It left
the cause: two bodies that have to be kept in step by hand, only one of which runs during teaching.

### What changed

- **`internal/production`** is the contract — `Request`, `Authority`, `Verifier`, `Report`, one
  closed `Refusal` vocabulary. Types and interfaces, no machinery, so the Director can name a
  production without importing the Theater. `internal/director/rehearse` still imports nothing
  that can act, which the boundary test continues to assert.
- **An Actor writes legal Marco and performs nothing.** `Actor.Cast` returns a program; the
  Production boundary runs it through an injected `MarcoRunner`. Every real input therefore passes
  the compile gate, and a dry run has something to record. `AccessibilityActor.Perform` — a direct
  host call — is deleted.
- **The live rehearsal's own path is gone.** `liveControls`, the element-id resolution inside
  `LowerStep`, and `pressThrough` are deleted rather than kept as a fallback. A press crosses as a
  name, a kind and a window; no runtime id survives the gap between deciding and doing.
- **Verification travels with the request.** A standalone runtime brings none and gets
  `not_verified`; a rehearsal lends `observeOutcome` itself. `theaterhost.Verifier`
  (`Changed(ctx) bool`) is deleted — it was a second idea of verification in the same file, and
  with a nil verifier it made every standalone saved play report `failed`.
- **`cmd/director` wires the Theater** with the same runner the rehearsal already chose, so a dry
  production lands in the notebook exactly as its navigation does.

### Things worth knowing next time

- **The window travels on the request, never on the Actor.** It was briefly an `InWindow` builder.
  One Theater serves a saved play and a live rehearsal in the same process, and a scope stored on
  a shared actor is one caller's window silently applied to another's production. Same reasoning
  retired the construction-time verifier.
- **The loop asks its own verifier whether it ran**, not the report. A fake producer that claimed
  to have verified without asking left the step with no outcome at all — which is how that was
  found. `StepEmission` carries nothing about the result at all now.
- **`Reached` is `Performed || perform_failed`.** Nothing was sent for `target_not_found` or an
  ambiguous name, so those stay refusals; a cast program that ran and had its capability declined
  did reach the machine, and the record of that must not be thrown away.
- **A cast program cannot re-enter the Theater**, and the guarantee has two halves. The textual
  half is the Actor (a cast program names only the concrete act); the structural half is
  `cmd/marco/theaterwiring.go`, which builds the runner's act map without the Theater in it. No
  depth counter.
- **`internal/director/uiact` still lowers its own control activations** for the UI action path.
  Separate caller, separate migration — named here so it is not mistaken for covered.

### What the mutations found

Twelve non-equivalent mutations, twelve killed, across four packages. One was found **equivalent**
and is recorded rather than faked: mapping `"Accessibility"` to the recorder in the dry rehearsal's
act map changes nothing observable, because `marcorunner` installs the OS host as the default for
any act with no entry and here that is the same recorder. It is written out anyway — an
Accessibility call served by the OS host is an accident of the fallback, and the live map names
both explicitly.

Two tests that were gating the deleted verification (`TestAnActivationThatChangedNothingIsNotSuccess`,
`TestWithNoVerifierTheResultIsUnverified`) were removed rather than adapted; the invariant they
held moved to `perform_test.go` with the verification itself, and ADR-068's *Enforced by* was
corrected to say so.
