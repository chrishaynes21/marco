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

# Roadmap 34F Phase 0 — one runnable product, proven live (2026-08-20)

The 34F audit found a product that could learn and could not run what it learned. Phase 0 closed
that, and the closing was done by the live acceptance rather than by the deterministic work: four
of the five defects below were invisible to a green suite and were found by asking for a learned
play on a real desktop.

## What is proven, end to end

A **cold** process, an unrelated application in front (VS Code), a Play taught once and never
taught again:

```
marco do "Open Mouse Settings"        app=code
  route found            open-mouse-settings   scope=applicationframehost
  authority              allowed (learned_play) — asked, and answered
  delegating to the Director
  step 1 of 2: verified
  step 2 of 2: verified
  Done.                  performed 2 of 2 steps        exit 0
```

Twice, from two independent cold starts, with the Director killed and its endpoint file gone
between them. Learn established three durable Places (`Home`, `Bluetooth & devices`, `Mouse`), two
edges, both `directly_verified`, and the Play was saved AND registered with no developer command.

The chain, each arrow a real implementation: overlay/`marco do` → `dispatchDo` → `Registry.Resolve`
→ `orchestrator.Classify`/`Authorize` → `Resolved.Learned()` fork → `service.PerformQuery` →
`Runtime.PerformGoal` → durable goal and application → `bringForward` → live desktop selector →
`freshPlace` → `observe.PlaceNow` → `PlanToGoal` over verified edges → execution grant (epoch
`asked`) → `rehearse.Live.Perform` → Theater → `AccessibilityActor.Cast` → legal Marco → compile
gate → `bridgehost`/UIA → real input → per-edge verification → a second fresh look →
`confirmArrival`.

**No second performer was created.** `cmd/marco` gained a socket client, not eyes.

## The five defects, and which the suite could see

| # | defect | found by |
|---|---|---|
| 1 | a learned Play registered as a **context** route, so it resolved only while its own application was already in front — asking for it from anywhere else offered to TEACH a play learned four minutes earlier | live |
| 2 | making it a focus route broke collision refusal: `Has` is scope-exact, so a learned play in `focus/` no longer collided with an authored play of the same name in `context/` | the suite, immediately |
| 3 | `applicationframehost` hosts Settings, XBOX and Realtek Audio Console; activating by application name raised the wrong window and the look honestly said `place_unknown` about a window nobody asked about | live |
| 4 | the accessibility bridge is a console executable, so starting it created a console window, and **a new console window takes the foreground** — stealing it from the application the Director had just brought forward. Cold path only | live |
| 5 | `Arrived = final == subject` made `"" == ""` true, so an unseeable screen reported "Done." | mutation, not the suite |

Defect 2 is the one worth remembering: a fix that widens a scope silently narrows a guard, and the
guard was written against a *file* when the invariant is about a *name*. `Registry.nameTaken` now
asks every scope that could answer.

## What the acceptance measured that no test could

- **A responsive application's structure signature is size-sensitive.** Resizing Settings from
  1936x1048 to 1500x950 collapsed its navigation pane, changed the control counts, and the taught
  Place stopped matching. [[ADR-072-a-place-is-not-its-viewport]] says a place is not its viewport,
  and that still holds — but an application that REFLOWS is genuinely showing a different
  composition, and Marco is right not to recognise it. Recorded, not fixed.
- **`director sight` answers from the newest finished session.** Three consecutive looks returned
  identical numbers including the visit count, which read as a stuck live view and is really the
  stale-evidence path doing exactly what it says. It is a poor probe for "what is in front now";
  `freshPlace` is the one with the freshness rule.
- **A fullscreen game holds the Windows foreground lock**, and nothing — Marco included — can
  activate over it. Three independent mechanisms failed before this was attributed.

## Follow-ons, recorded and NOT fixed

1. Redundant Stage settling: the planning look and the first establish settle the same screen twice.
2. Window disambiguation is by title, verified by looking. It is correct but costs one look per
   candidate, and titles are an address, never identity.
3. A locally-run generated play still swallows its own failure in its `or?` arm and exits 0. The
   bridge path reports honestly; the local path does not.
4. Learned-play names with punctuation or apostrophes do not round-trip slug→goal and refuse with
   `not_learned`. The durable `Origin.To → Goal.Subject` join fixes it and needs no protocol change.
5. `plugins/uia/uia.exe` is locked while the stack runs, so `setup.ps1` fails if run live.

## Decisions

[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]] — the save/register wall
is about the mechanism, not about who crosses it.
[[ADR-080-a-learned-play-is-asked-for-from-anywhere]] — focus, not context, and why a collision is
about a name rather than a file.
[[ADR-078-a-learned-play-is-performed-by-the-director]] — corrected: `bringForward` preceding the
Stage read IS now gated, contrary to what it previously claimed.

# Roadmap 34F Phase 1 — the product noun is a Play (2026-08-20)

Phase 0 made one runnable product. Phase 1 gave it a vocabulary, and a surface that shows the
whole of what a person has — including the plays that are saved and cannot yet answer.

**No file moved and no directory was renamed.** `internal/routes` keeps its name, `routes/` keeps
its name, `$MARCO_ROUTES` keeps its name, and `marco routes` keeps its name, its result set and
its published JSON keys. Product vocabulary and implementation vocabulary are allowed to differ —
the same divergence this repository already runs for Learn/`teach`.

## What landed

- **`internal/plays`** — one package holding the product projection AND the product wording:
  `List` / `Registered` / `Staged` / `Find`, `ScopeOf`, `LifeOf`, `AfterLearn`, `KindWord`. Kinds
  present as **Authored / Recorded / Learned**; standing is one of `ready | edited | unverified |
  saved | stuck`, each with its badge and its sentence.
- **An additive registry API** — `ListStaged`, `KindOf`, `KindOfStaged`, `HasRecording`,
  `MoveOrigin`, `LearnedFocus`. `Registry.List` and `Registry.Resolve` were not touched: the
  Plays surface JOINS two enumerations rather than widening one, because `Resolve` walks `List`.
- **One decider for provenance** — `orchestrator.Classify` now calls `reg.KindOf`, the same call
  the listing makes, so the authority door and the listing cannot disagree about a file.
- **The control centre's Routes tab is the Plays surface** — registered and staged together,
  `GET /api/plays`, `POST /api/register`, delete via `Unregister`, and `/api/scope` carrying the
  `.origin.json` verbatim through `MoveOrigin`. The landing tab with no play named is Plays. The
  view identifiers (`routes`, `nav('routes')`, `marco ui routes`) deliberately did not move.
- **`marco plays` and `marco register`** — the listing that shows a saved play, and the command
  that listing names. `marco routes` stayed narrow on purpose: it is what may be OFFERED to a
  front end, and a staged name would advertise something `marco do` cannot find.
- **One unknown-command prefix** — `"no play matches "`, with `plugins/overlay`'s suppression
  constant moved to match. The wire prefix `"[route] "` is protocol and did not change.

## The two defects this made visible

1. **A scope change stripped a learned play's past.** `/api/scope` carried the `.marco` and the
   `.rec.json` and left the `.origin.json` behind — so the moved play re-listed as *Authored*, and
   an orphaned sidecar sat under the old scope waiting for the next play saved there. Invisible
   while nothing displayed provenance.
2. **Writing the sidecar at the destination would have been worse.** `SaveWithOrigin` recomputes
   the digest, so a play the person had edited would come out of a move reading `ready` — Marco
   silently re-vouching for an artifact it never verified. `MoveOrigin` copies the bytes instead.

## Decisions

[[ADR-081-a-durable-behaviour-is-a-play]] — the product noun, the four alternatives that were
rejected (including renaming the package), and the costs of keeping two vocabularies.
[[ADR-082-a-plays-past-travels-with-the-file]] — Rename and the scope change carry provenance
verbatim, so a move can neither lose a past nor re-verify an edited play.

## Where to read it

[[Plays]] is the new subsystem note: the Play and the Binding, the three kinds, the three scopes,
the five standings, and a table of which call decides each fact. [[Learned-Plays]] was corrected
where it still said a learned play lands in `context/` — it lands in `focus/`, and has since
[[ADR-080-a-learned-play-is-asked-for-from-anywhere]]. README documents `marco plays` and
`marco register`; E2E section **G** walks a staged play across the line and back.

# Roadmap 34F Phase 2 — one invocation intake (2026-08-20)

Phase 1 gave the product a noun. Phase 2 gave it one door. **Typing, speaking, a hotkey, a
clicked Run and the CLI are entrances, not different Marcos** — and which one you used no longer
decides what your words mean.

The rule, in one sentence: *if Marco already knows exactly what the Audience means, it uses the
durable thing it knows; otherwise Director works out what they mean from the live world.*

## What landed

- **`internal/invoke`** — the one semantic decision, PURE. `Decide(plays, Request) Decision`,
  five arms in a fixed order: control phrase → pending question → explicit identity → exact
  durable match → Director. `Request.Source` is recorded and **never consulted**; lookup is exact
  and never fuzzy.
- **`cmd/marco/intake.go`** — `runInvocation`, the one process-side entry, and six outcomes
  (`performed | clarify | refused | unavailable | cancelled | failed`) with distinct exit codes,
  announced on `[result] ` beside the unchanged `[route] `. `MARCO_TRACE_INTAKE=1` adds
  `[intake] source=… decision=… play=… why=…`, which is how the acceptance is *seen*.
- **Deleted: `dispatchDo` and `resolveTarget`.** The 0.75-score `internal/nlu` match and the
  `$MARCO_RESOLVER` external model no longer sit in front of the door on the product path. Both
  packages survive as a developer surface (`marco assistant`'s confirm loop, `args`, `simplify`,
  `bind`, `dispatch`).
- **`plugins/overlay`** — the `RunVoice` → `marco director` branch is GONE. Typed and spoken build
  the same argv and differ in `--source` alone; both are asynchronous; control words are
  recognised locally for immediacy only, and `cancelRun` also spawns `marco director stop`.
  Six outcomes are read off the wire instead of guessed from an exit code.
- **Explicit identity end to end** — a clicked Run posts `{slug, app, scope}` and spawns
  `--play=`; `marco hotkey` resolves its binding once. A Play a surface already holds is never
  turned back into words and guessed at again.
- **Director: PERFORM is a registry command** — visible to `director status`, refusing a
  concurrent mutating request, reachable by `CANCEL_ACTIVE`, and holding a real cancellable
  context. `cmd/director/perform.go` has zero `context.Background()` left.
- **The subject is the identity** — `Origin.To → Goal.Subject`, matched before any name join.
  `ProtocolVersion` 6 → 7; rebuild everything together or the version check refuses.

## The four defects this made visible

1. **A learned play could not be stopped.** `rehearse.Live.Perform` checks `ctx.Err()` before
   every step and has a cancelled terminal and a cancelled refusal ready — all dead, because the
   only context ever handed in was `context.Background()`.
2. **PERFORM acted outside the command registry.** `director status` said nothing was running
   while a play typed and clicked, `director stop` answered "nothing is running", and a second
   mutating request was accepted concurrently.
3. **A typed "stop" offered to record a demonstration called "stop".** It missed Play lookup,
   fell through the unknown-command path, and became the teach offer.
4. **`marco do` was never the exact-match intake the audit assumed.** Two semantic layers ran in
   front of the registry, so with both "open settings" and "open the settings" registered, asking
   for the second ran the first — silently, in front of the authority door.

## Decisions

[[ADR-083-one-invocation-intake]] — one intake, transport does not decide meaning, and the three
alternatives rejected (teach Director about Plays; make Play lookup fuzzy; keep two intakes and
sync them).
[[ADR-084-a-plays-identity-is-its-subject]] — the `Origin.To → Goal.Subject` join, and why the
name is consulted for exactly one legacy case and never first.
[[ADR-085-a-performance-is-a-registry-command]] — visible, refusable, stoppable; this is what
makes `stop` work on a running Play.

## Where to read it

[[Invocation]] is the new subsystem note: the canonical rule, source invariance, explicit
identity, all five arms with the reason each sits where it does, the cancellation exception, the
six outcomes and the three wire lines, and a table of every entrance. [[34F-legacy-marco-product-audit]]
§22 records Phase 2 as landed and what it found that the audit had not. README documents the one
intake in user terms; E2E section **H** proves cross-surface sameness with `MARCO_TRACE_INTAKE=1`,
and needs a person at a microphone for the spoken half — a Final transcript cannot be scripted.

**Still owed:** Phase 3, one stop. A locally-run play still stops by the panic-stop hook and the
overlay's child kill rather than through the same authority.

# Roadmap 34F Phase 3 — one vocabulary: Learn, Teach, Do (2026-08-20)

Phase 2 gave the product one door. Phase 3 gave it one **vocabulary**, and the vocabulary is
vertical — it means the same thing in a package name, a CLI verb, a wire field and a sentence a
person reads:

> **LEARN** — the person acts, Marco watches and acquires. *(built)*
> **TEACH** — the person acts, Marco guides them through it. *(reserved, not built)*
> **DO** — Marco acts, the person delegates. *(built)*

The old spelling had all three collapsed onto `teach`, so the one direction the product does not
yet do was occupying the name of the one it does.

## What landed

- **Packages** — `internal/director/teach` → **`internal/director/learn`**; `internal/voiceteach`
  → **`internal/voicelearn`**. Files moved with them: `teachcmd.go` → `learncmd.go`,
  `teachtail.go` → `learntail.go`, `teachwiring.go` → `learnsessionwiring.go`,
  `teachgrounding.go` → `learngrounding.go`, and their tests.
- **The two acquisition request types were MERGED.** `service.ObserveTeach` is gone. There is one
  `service.ObserveLearn` carrying both the control surface's verbs and the session's own
  configuration (`Target`, `Actor`, `Verb`, `Surface`, `Dry`, `Finish`, `Evidence`).
  `ObserveQuery.Teach` is gone; everything goes through `ObserveQuery.Learn`, and `Surface` says
  which surface is asking and therefore which account of the session comes back. Two shapes of one
  act at two altitudes had been two types, and the translation between them was a facade.
- **`ProtocolVersion` 7 → 8.** Rebuild everything together and restart `overlay.cmd`, or the
  version check refuses — which is the intended failure, not a bug to work around.
- **Identifiers** — `Runtime.Teaching` → `Runtime.LearnSession`, `teachView` → `learnSessionView`,
  `teaching` → `learnSession`, `teachTail` → `learnTail`, `IntentTeach` → `IntentLearn`,
  `ReferentTeachStart/Destination` → `ReferentLearnStart/Destination`.
- **CLI and command words** — `marco learn` and `director learn` are canonical; the overlay says
  `learn <name>` and `narrate learn <name>`. `teach` still answers everywhere it ever did
  (`case "learn", "teach":`) as an **undocumented compatibility alias**, because it is the muscle
  memory the product shipped with. It is documented exactly once, in README's command reference,
  and it is retiring.
- **Not renamed, because it was never acquisition** — the overlay's interactive-prompt plumbing
  (any subcommand's prompts, not just a demonstration) became `promptPipe` / `promptAsk` /
  `setPrompt`. The `Learned*` family (`routes.KindLearned`, `director learned`, `LearnedPlay`)
  is a read over what Director wrote down, and `playbill.Learning` is derived from the passive
  observation session — both were already the right word.
- **A governance test** — `cmd/marco` `TestNoLiveAcquisitionCodeIsNamedTeach` refuses any
  Teach-spelled identifier in live acquisition code, and names the alias explicitly so it cannot
  grow silently.

## What was deliberately left alone

A mechanical replace would have destroyed the record, so three classes were kept by hand:

1. **Dated ADR bodies.** ADR-048's *What was wrong* and *Considered and rejected* sections quote
   the word in its wrong sense **as evidence**; rewriting them makes the ADR incoherent.
2. **Quoted transcripts** of live runs (ADR-079, ADR-080). Editing quoted evidence falsifies it.
3. **The reservation itself.** Every sentence saying Teach means Marco guiding a person got
   *more* correct and was left as written — including `plays.KindWord` returning **Recorded**,
   which is the ADR-048 decision already in force.

Ordinary English also stayed ordinary: a comment that says a thing "teaches the reader" is about
a human being instructed, not about Marco acquiring.

## Decisions

[[ADR-086-one-acquisition-one-word-one-request]] — one acquisition, one word, one request: why
Learn wins the name outright, why the alias is undocumented rather than blessed, and why the two
request types were merged instead of kept in sync. It supersedes nothing in
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] — it finishes it, because ADR-048
named the three sentences and the code kept only one of them.

## Where to read it

CLAUDE.md now carries the three-word rule as a load-bearing invariant, with the governance test
named. README documents the alias once. E2E section **A** gained step **6a**, which exercises the
`teach` alias in the HUD — the only place the overlay's command-word alias can be proven, because
the Go suite cannot drive the command line.

# Roadmap 34F Phases 3–5 — one stop, one account, and the record (2026-08-20)

Phases 0–2 gave the product one runnable path, one noun and one door. This entry covers the rest of
the 34F campaign in one go, because it landed in one commit (`ac8da6c`) plus the documentation pass
that closed it.

**A numbering note, so nobody loses an hour to it.** The HANDOFF entry above is headed *"Phase 3 —
one vocabulary"*; the campaign plan calls that the second half of Phase 2, and calls **this**
Phase 3. The commits are the truth: `fca45e0` is one intake + the acquisition rename, `ac8da6c` is
one stop + one account.

## What was built

**One stop, and it crosses a process boundary** ([[ADR-087-one-stop-and-it-crosses-a-process-boundary]]).
`stop` was already a control phrase from every entrance, and the Director's own performances were
already registry commands — and neither was enough, because the thing a person is most often trying
to stop is a Play running in a **different process** from the one they said it to. `marco do`
performs an authored or recorded Play inside a short-lived child; the overlay starts one, so does
the control centre, so does a hotkey, so does a terminal. `internal/stopsignal` is the one stop that
reaches them.

**`finally` had never once run on a cancellation** ([[ADR-088-cleanup-runs-when-the-audience-stops]]).
`spec/Core.md` is normative and says `finally` runs *including cancellation*, with a worked example
of releasing a held key — which is precisely the work that must still happen when somebody presses
stop, and the one case in which it never happened. The mechanism was a single `if`: `runBlock`
bailed out on a cancelled frame and called `runFinallies`, which re-entered `runBlock` **on the same
still-cancelled frame**, so the cleanup body's first ordinary edge hit the same guard and returned.
The whole suite was green while a normative spec sentence was false.

**The second invocation spine is gone.** `orchestrator.Deps.Do` / `.Resolve` / `.Run` was a
complete resolve → authorize → run → learn-on-unknown path with **no production caller** — and
worse than the live one: no context (so no panic-stop, no cancellation) and no `routes.ApplyArgs`.
The authority test suite still entered through it, which is exactly how the last authority bypass
hid. Deleted, with the tests moved onto the production entry.

**A live rehearsal can now be stopped.** `learnTail.Rehearse` took a `context.Context` with no
name — Go's way of saying *I am obliged to accept this and I intend to ignore it* — and
`rehearserun.go` then called the walker with `context.Background()`. Pressing **Try** in the Learn
panel started real input that nothing in the product could stop: not the panel's Cancel (it
cancelled a context that call had discarded), not `director stop` (no registry command existed for
it), not the overlay's Esc, not a spoken "stop". Fixed at both ends, and `cancelActive` gained a
third arm so a Learn episode is something the stop can find.

**One live-walker composition root.** `perform.go` built the same object `rehearserun.go` built and
differed from it in one line: it never called `WithForeground`. `Live.behind` returns false when
`inFront` is nil, so the per-step "input would land somewhere else" refusal was **live for the
rehearsal Marco asks permission for and dead for the play the Audience asks for** — exactly
backwards. There is one root now, and `TestEveryLiveWalkerChecksTheForeground` enumerates every
function in `cmd/director` that builds a `rehearse.Live` rather than naming files.

**Marco's Screen host asks the Director which place is showing** (protocol v9). It used to return
`unavailable` unconditionally — a true statement about that process and a false one about the
system, since the eyes exist in a sibling process one dial away. This also closed a hole nobody had
listed: an **edited** learned play loses `Resolved.Learned()`, so it takes the local runner, so it
used to refuse at its own first `do Screen's Showing` line. Editing is invited, the authority door
deliberately blesses it, and the result was a Play that refused at its own first line.

**One outcome vocabulary.** `internal/outcome` owns the six words
(`performed`/`clarify`/`refused`/`unavailable`/`cancelled`/`failed`), the `"[result] "` and
`"[route] "` wire literals and the exit codes; engine, HUD and control centre all import it. The
control centre now reports **real** outcomes — it used to fire-and-forget a child and render
`running:` forever — through a run id the page polls (`cmd/marco/runaccount.go`). Actor
availability asks its provider and carries a reason, empty always when it can act.

## What was found

Nine read-only audits were written mid-campaign and are the source material for the three new
notes. Their findings, re-verified against the closing tree, and several of the headlines had
already been fixed by the time the record was written — which is itself the finding worth keeping.

- **The dangerous defects were all invisible to a green suite.** A dead foreground guard, a
  `finally` that never ran, an uncancellable rehearsal, a second spine only tests could enter. Four
  for four, and each one had complete, well-commented, correct-looking code.
- **The "one intake" guard is weak.** `TestEveryEntranceRoutesThroughTheOneIntake` is a string grep
  over two named files. It would pass unchanged if a tenth entrance appeared tomorrow. The
  precedent to copy is `internal/platform/navsource/pump_test.go`, which walks the whole tree for
  hook sites rather than naming them.
- **There is no second Marco left, but there are five narrow primitives** that reach real input
  without passing `Decide`: `marco press`, `marco run`, `marco serve`, `marco director "<phrase>"`
  and `director op`. Four are defensible; the work is to **name them in the model** rather than to
  close them.
- **`/api/bind` still writes a binding it never validates**, reintroducing the exact defect
  `bindKey` was written to prevent — *a binding that reports success and can never fire is worse
  than one that refuses*.
- **The Observe stack is far less Learn-welded than anybody expected.** Light Mode already runs a
  passive session with a **zero Episode**, and place recognition works inside it. Exactly one
  production site sets `EstablishPlaces` true — and that one boolean grants at least four separate
  licences.
- **The genuine 35A blocker is cold start, not Learn.** A durable relationship needs both endpoints
  already Established; on a fresh install nothing is, and places become durable only through Learn
  or through the Audience typing a name. **The ambient loop cannot start itself.** That is a policy
  decision for 35A with an ADR, not a 34F fix.

## What is owed

- **`/api/bind` → `bindKey`.** The most concrete unclosed defect in the campaign. Small.
- **One `Stage.Now()`** owning evidence selection in front of `PlaceNow`. Eight callers choose their
  own evidence today, so there is no single answer to *what is Marco looking at* — the precondition
  for Observe.
- **An enumerating one-intake test**, in the shape of the pump test.
- **One `internal/marcopaths`** — `$MARCO_HOME`, `$MARCO_ROUTES` and the uia bridge are each derived
  independently in two binaries. Break #3 is in remission, not cured.
- **A shared overlay config struct.** `cmd/marco/oconfig.go` hand-mirrors the overlay's `Config` and
  does not preserve unknown keys, so the control centre can **delete a setting it does not know
  about**.
- **One page naming the three authority regimes** (`orchestrator.Authorize`, the rehearsal grant,
  and `internal/director/policy`), which today a reader has to discover.
- **The CV/anchor default**, still undecided since the audit; three hand-maintained copies of the
  flag.
- **One Activity.** At `ac8da6c` the overlay still keeps a private command history beside the
  playbill. Work on a shared Activity account was in flight in the same tree while this entry was
  written (`cmd/marco/activity.go`), so check before starting it.
- **An Advanced surface.** The control centre has six unmarked tabs and no advanced section at all,
  which is why several developer-grade strings have nowhere to go.

## Where to read it

- [[34F-duplication-matrix]] — canonical owner, remaining duplicates, why each remains, and the
  second-Marco question re-verified path by path.
- [[34F-legacy-matrix]] — every major system with exactly one status, the compatibility table, and
  the exhaustive CLI verb surface of `cmd/marco` and `cmd/director`.
- [[34F-observe-readiness]] — the perception dependency graph, the three gating questions answered
  with proof, the cold-start bootstrap problem, and the four-tier privacy seam.
- [[34F-legacy-marco-product-audit]] now carries a dated banner recording what landed and which of
  its own findings are stale. **Its body was deliberately not rewritten** — it is the record of
  what was true then.
- [[Roadmap]] gained **35A Observe**, **35B Learn as explicit attention and admission over
  Observe**, and **35C Teach**. Observe is not implemented; nothing in the tree is called Observe.

---

## Roadmap 35A + 35B — observation stops being a privilege of Learn, and Learn stops replaying you

Three commits on `feat/host-ffi`: `90a0bce`, `f1fe7d3`, `a0d1ae5`.

**35A: the inversion, measured first.** `Episode.EstablishPlaces` was set true at ONE site and read
at FOUR, permitting four unrelated things. Three are real licences and stayed. The fourth was not:
`AdmittedPlaceName` opened with `if !demonstration { return "" }`, so outside an explicit Learn
Marco could recognise the Place it was standing on and could not say what it appeared to be called.
"Where am I?" had no answer unless you were teaching it something.

The argument beside that gate was *"a Place's name is read off somebody's screen, and passive
observation has no business writing that down."* The second half is still policy; the first is a
different question answered with the same word. Traced: the durable write was **already** licensed
one level up, inside `establishPlace`. The gate on the inference was a second gate that bought no
privacy. It is gone; the shape filter that actually protects a person was always unconditional and
now carries the whole weight.

The remaining three are named permissions — `EstablishPlaces`, `AcquireRouteEvidence`,
`NameActivatedTargets` — grantable one at a time, with `LearnLicence()` as the one place Learn's
grant is written down. **Four mutations survived the first version of that split**, including
hard-wiring the privacy-carrying permission to `true`; every test granted all three, so a swapped
guard was invisible to all of them. Closed by tests that drive the production path with permissions
that DIFFER. See [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]'s amendment.

**35B: Fast Learn.** Every required edge used to be REHEARSED before a route could be written down
— "want me to try?", a yes, and Marco driving the real desktop, twice for a two-hop route, to learn
something the person had just finished showing it.

The measurement that settles it: the rehearsal question was raised under **exactly** the conditions
that already made the evidence sufficient (`CandidateConsistent`, nothing `Blocking()`), and
anything less was refused upstream and never reached a rehearsal at all. There was no middle state.
The question obtained no information — it obtained a permission, for an action nobody had asked
Marco to take.

So a clean demonstration is admitted on the strength of what the person showed: `EdgeObserved`
beside `EdgeVerified`, deliberately not folded into it, because they are claims about different
actors. Rehearsal survives as a tool via `WithRehearsal(true)`. `verifiedEdges` became
`plannableEdges` and accepts either kind of knowing — without which a Fast-Learned Play would have
saved and then refused to run. Authority, foreground and per-edge verification are untouched and
are the reason widening planning weakens nothing. See
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]].

**Two governance findings, both about gates I had built.** The wiring-claim checker scans line by
line, so a claim whose test name WRAPPED onto the next line was invisible — **119 claims in this
tree end a line with the phrase**, and it had been run, passed, and reported as closing the problem
while never checking any of them. And it read `.go` only: the docs vault held 25 more phantom
citations in ADR `Enforced by` sections, which CLAUDE.md names as a hard rule, while `docscheck`
said "no problems" throughout. Both holes are closed and the scan is now testable directly.

**Not done, and owed.** Live acceptance — `ACCEPTANCE.md` section E is the twenty-minute
walkthrough, and steps 16–20 need a person demonstrating a real route in Windows Settings. Nothing
yet upgrades an observed edge to execution-proven when a later run succeeds; the seam is
`RehearsalEvidence` and the write belongs with 35C. Automatic Place naming was verified against its
existing regressions (nav-rail, loading/reflow) rather than re-derived from fresh accessibility
dumps.

**35C: fast verified execution — carry proof forward.** Marco proved the same unchanged fact four
times in a two-edge play: once to plan, once before edge one, once before edge two, and once to
confirm arrival. Three of those four asked a question that had been positively answered moments
earlier, twice by the very action that would have changed the answer. Not wrong — redundant, and
"nothing had changed" is a claim that has to be justified rather than assumed, which is why it had
never been removed.

The rule is two lines and only the second is interesting: **Marco may avoid proving the same
unchanged fact twice; Marco may not act on a fact it can no longer justify.** `StageEvidence`
carries a Place, the window reference it was established on, when, and how it came to be known;
`Justifies` refuses on six independent arms. A verified outcome becomes the next edge's source, the
planning look becomes edge one's, and the last edge's proof answers the arrival question. Measured
deterministically: 10 screen readings where it used to be 15.

**The first version of this was less safe than the code it replaced, and the measurement is the
argument.** `Justifies` checks everything a stored fact can be checked against, and cannot see the
one thing that actually happens — somebody clicked. Moving Settings from one screen to another
changes no window, no process, no generation and no foreground. Establishing from scratch would
have caught it. So a proof now buys a SHORTER question rather than no question: one reading, which
must resolve to the same Place, and any disagreement — including an unreadable frame — falls
through to the full establish. It must never refuse on one frame, because a single reading can
catch a transition six readings and a settle would look past.

Two things came out of tracing the path rather than optimising it. `freshPlace` left its
observation session running for the remaining six of its eight seconds, **sampling the screen
alongside the walk it existed to start** — bounded, so never a leak, and contending for the one
accessibility provider with every reading the route took. And 35B's open seam is closed: a
successful performance now ADDS execution evidence beside what was demonstrated, so the history can
say both that the human showed it and that Marco later performed and verified it. A failed run adds
nothing and erases nothing.

**Three mutations survived and each one changed the code rather than the test.** Sourcing the proof from
the plan's expectation instead of perception is unreachable through any walk — a route completes
only on a directly verified last step, which is DEFINED as those two agreeing — so the rule was
extracted to `provedBy` and is held directly with a record no walker can produce. A guard screening
the handoff on `step.Verified` sat above the check that already ends the walk, and was deleted
rather than left as a claim nothing can test. And `StageEvidence.SameWindow` was written, used, and
removed: no caller acts on the proof's window, so every use of it was measurably equivalent to
nothing.

**And it was run live, on Windows Settings, and the fixture predicted it exactly.** Home ->
Bluetooth & devices -> Mouse: 10 screen readings, 0 full establishments, 2 confirmations both
accepted, 4 Place resolutions -- every count identical to the deterministic three-screen fixture,
which is the strongest evidence available that the fixture describes the thing. 4533 ms wall,
3018 ms inside the walk, 399 ms inside the two confirmations. From that last figure a reading cost
about 200 ms on that machine, so the two reused proofs avoided roughly 3.2 s: a ~6.2 s walk
reduced to ~3.0 s. DERIVED and labelled derived wherever it is printed -- the same route was not
run against the old code, because a switch to turn the handoff off would be a second path through
the one part of this system that must have only one.

**Not done, and owed.** One run on one machine for one route is not a distribution.
`acceptance-35c.ps1` reproduces it in two commands against a COPY of the operator's store. See
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]].

**35D: a Place is not its presentation.** A real store had accumulated TWELVE durable Places for
THREE Settings pages — Home, Bluetooth & devices and Mouse, each recorded three times at a
different window size — plus two loading frames and a search state. Twelve relationships for what
is two transitions. The consequence a person sees: a Play learned on a wide Settings Home stops
recognising Settings Home when the window is narrow.

Identity diverged in `ExplainStructure`, which compares a screen's role histogram count by count
with an absolute tolerance of one. Measured across the recordings, the counts that moved were
always the same ones — `group`, `pane`, `text`, `link` — and on Mouse the `link` role ARRIVED, so
the role-set check called the third recording a different screen before a single count was
compared. What never moved: `list_item`, `combo_box`, `slider`, `image`, `text_field`.

So [[ADR-062-a-scroll-bar-is-not-a-screen]]'s own test — does this role's arrival tell a person
they are somewhere else? — was applied to four more roles, and the tolerance above the floor
became a SHARE of the count rather than a number. Below seven that share rounds to one, so every
small composition is compared exactly as before, which is where the four-versus-six-item worry
lives. Nine recordings of three pages now become three Places.

**`button` was in the layout set for a day and the suite caught it.** Its count does move with
reflow, but two existing fixtures told screens apart by button count alone and began merging.
Dropping a role a person can PRESS is a real loss of discrimination, and this file's own bar is
that a false merge is worse than a false miss. The tolerance moved instead. The margin between
"moves with reflow" (three) and "is a different page" (five) is one detection — thin, and the
honest width of the gap the measurement found.

**The fix would have made an existing store worse without one more change.** Under the new rule a
fresh reading of Home matches all three Home records, and `Recall` answered `insufficient` for
more than one match — refusing to recognise a page it had just correctly worked out it knows three
times over. Records that are the same Place as EACH OTHER are now one Place written down twice,
answered with the lowest id; records that match the reading and not each other are still
ambiguous. That distinction needed the non-transitivity of tolerant matching to be taken
seriously.

**Stated normatively, because it is the easiest wrong inference available:** same Place after a
resize does NOT mean the same live Stage evidence. A resize moves every control in the window,
and a carried proof exists precisely to act without looking again. Both answers are true at once,
and 35C is untouched.

**Not done, and owed.** The existing duplicates are ALIASED, not merged — the store keeps its
twelve subjects and twelve relationships, and migration needs its own design because it has to
carry edges, names, targets, provenance and Plays. And no semantic name was ever captured for any
of the twenty-one subjects in that store: automatic naming exists and produced nothing, so
identity still rests on composition and terms alone. See
[[ADR-091-a-place-is-not-its-presentation]].

**35E: one Director per home, one hand on the keyboard.** Found by listing what was actually
running: three `director.exe` processes, TWO of them serving the same `$MARCO_HOME`. Two
observation loops on one desktop, two writers to one semantic store, either able to cancel while
the other kept acting, and each answering "where is the Audience standing" from its own private
world. Nothing prevented it — the CLIENT had a startup lock so two clients would not spawn two
services, and `director serve` had none, so running it twice always made two Directors.

Three ownerships, and collapsing any two would be wrong: home ownership says "I am the Director
for this Marco world"; the desktop lease says "I am the only runtime driving this screen"; Audience
authority says "Marco may perform this action". This roadmap adds the first two and leaves the
third exactly as it was.

**Home ownership cannot cover the desktop**, which is the part worth remembering. Two homes are two
legitimate worlds — a sandbox beside the real store is how every harness here runs — and they
share one keyboard. A lease scoped by home would let them type at once, each holding something it
genuinely owned. So the desktop lease is machine-wide and held around a PRODUCTION rather than for
the life of a Director: held from startup it would stop every sandbox from ever acting; held
around the walk it stops only what conflicts, and observation needs no lease at all.

**A file cannot be the primitive.** It survives the process that wrote it, its PID can be reused,
and a staleness timeout is a guess about how long a start takes. The claim is a named kernel
object whose lifetime is the process's — released by Windows however the process ends, so a crash
cannot brick a home. The endpoint file stays as DISCOVERY, validated by connecting rather than
believed.

**Three normalisations survived deletion because the fixture used an existing directory.**
`filepath.EvalSymlinks` resolves a real path and hands back the real spelling, so the test was
testing EvalSymlinks. Against a home that does not exist — which is the ordinary case, since the
first `director serve` for a sandbox claims before anything creates the directory — the case
folding is load-bearing and the explicit `Clean` is genuinely redundant, so it was deleted rather
than left as a claim nothing can test.

**Not done, and owed.** No live acceptance: whether two `director serve` invocations on one home
refuse correctly on a real desktop is UNMEASURED. And `Connect`'s file-based startup lock is now a
second, redundant mechanism — a losing child refuses at startup instead of racing — worth
simplifying and not worth changing blind. See
[[ADR-092-one-director-per-home-one-hand-on-the-keyboard]].

**36A: `marco observe` — Marco pays attention.** The first ambient product lifecycle on the
observation substrate 35A built. Somebody types it and then uses their computer normally; Marco
keeps up with which application and which screen they are on, and what just changed.

Four things it deliberately is not, and each one is gated. Not a recording: nothing reaches disk,
and what is held in memory carries no labels, titles, text or coordinates. Not a Learn: no
questions, no naming. Not authority: watching grants no permission and takes no desktop lease —
observation is the one thing 35E deliberately left ungated. Not a second observer: the registry
allows one session, and ambient watching is its LOWEST-priority consumer.

**Storage growth tracks novelty, never time.** Ten thousand sightings of one screen are one entry
with a count of ten thousand. Bounds are on distinct things — 256 places, 512 edges, a 64-move
recent walk — and past them the least recently seen is forgotten, counted rather than silently.
That property is the whole reason an always-on observer is acceptable at all.

**The default licence is none.** Sessions go through `Start`, which hands the runner
`Episode{}` — so however long ambient watching runs it cannot make anything durable. Not a policy
check that could be forgotten: there is no licence in the object. A screen Marco does not know
stays unknown and nobody is asked to name it.

**Six mutations "survived" and all six were my verification lying.** `git diff` reports nothing for
UNTRACKED files, and every file in this roadmap was new — so the check that confirmed the edit had
landed passed vacuously for all of them. Re-run with grep, five were real survivors and every one
found a genuine gap: a second supervisor loop against the same object, backoff with no test at all,
provenance unasserted at the supervisor level, an autostart flag the fixture could not see, and —
the one that mattered — **an unknown screen breaking the walk through it**. Home → unknown frame →
Bluetooth must still be one transition, and one line does that.

**Yielding, not layering, and the cost is written down.** The prettier design is one continuous
session whose licences change as consumers come and go; that is a rewrite of the session runner.
Ambient waits instead, so a Learn interrupts it rather than borrowing it, and the ambient tally has
a gap for its duration. The registry is what enforces one observer; the yield only buys quiet, and
deleting it breaks nothing — measured.

**Not done, and owed.** No ambient promotion: repeated evidence does not become durable knowledge,
and that seam is left clean rather than taken. "Learn what I just did" would know where somebody
went and not what they clicked. And every live number — CPU, memory, sample rate, Learn-over-Observe
— is UNMEASURED. See [[ADR-093-observe-is-attention-not-recording]].

## 36B: `marco learn "..." --recent` — the thing Marco just watched becomes useful

36A ended with an honest gap written into its own ADR: the trail knew where somebody went and not
what they pressed, so "learn what I just did" would have produced a route with no way to walk it.
36B closes that, and the sentence it makes true is the whole roadmap:

```
marco observe
   ... use your computer ...
marco learn "open mouse settings" --recent
done
```

No repeat demonstration. No naming a screen. No naming a button. No rehearsal. No input from Marco
at any point.

**A demonstration is `Place → activated control → Place`, and none of it is a coordinate.** The
press was already resolved against the accessibility tree at the moment it happened — that
machinery predates this roadmap — so what enters the trail is a role and an admitted name. There
is no position field for `button = (742, 318)` to live in.

**The privacy boundary moved by exactly one thing, deliberately.** The name of the ONE control a
person's own action landed on, plus what a screen appears to be called. Neither widens what Marco
PERCEIVES: both already cross under the zero licence, through the canonical plaintext allowlist and
the unconditional place-name rule. What ambient watching still cannot have is the WIDER label
stage — the one that admits list items, links and icons, where somebody's documents and contacts
live — because that is `NameActivatedTargets` and it holds no licences at all.

**The one correlation that had to be right.** An event is drained about a second after it happens,
by which time the click has usually already changed the screen — so whatever is in front when the
event is read is very often the DESTINATION rather than the source. Attributing to it produces "on
the Bluetooth page, press Bluetooth", which is wrong in a way that reads as entirely reasonable.
Every admitted event carries the session-local screen state it was banked against; the action is
filed against that.

**The defect this landed on, and it was a real one.** ADR-089 says in as many words that "a Play
can now be saved that Marco has never executed". It changed the Learn coordinator and it changed
planning, and it did NOT change the lowering gate, which went on asking a field only a completed
live rehearsal can set. So Fast Learn produced every durable thing except the artifact — places,
edges, candidates, the goal, and then `no route is ready to be written down`. Nothing caught it:
every save test in `cmd/director` writes a rehearsal record into its fixture first, and the test
that named the gate asserted the pre-089 rule and passed. Found here because the retrospective path
makes it unmissable. ADR-027 is amended.

**An unreachable discriminator, written and then removed.** Widening the gate raised the question
of what a FAILED attempt should do, and the honest answer turned out to be that the store cannot
represent one: `rememberRehearsal` is called only for a completed live route. An `Attempted` field
was written, measured to be unsettable, and deleted. What stands is `Rehearsed` — a rehearsal of
this ROUTE is on record and has not verified — which is reachable and stops a second demonstration
of a route quietly lowering instead of the one that failed.

**Nothing on this path can act.** No input, no desktop lease, no authority, no second performer.
Asserted through `beginPerformance`'s counter rather than through the absence of a symptom, because
that slot is the one every actuating entrance funnels through — which is also how the observer now
tells Marco's own work from the person's.

**Not done, and owed.** Ambient promotion is still not built: repeated evidence does not become
knowledge on its own, and the seam is now much better fed than it was. Text entry stays structural.
A drag is refused rather than flattened into an activation. A multi-leg walk saves its terminal leg
as the play and lets planning walk the rest. And the live acceptance is UNMEASURED —
`acceptance-36b.ps1` is the harness, and the one step that would drive the desktop is deliberately
handed to the person. See [[ADR-094-observe-gathers-evidence-learn-promotes-it]].

**Fifty mutations, six survivors, and every one was about what the tests entered rather than what
the code did.** The drain in `sample` was production code nothing invoked — the third uncalled
mechanism this repository has found. The licence test passed with the licence check deleted,
because it asked only whether *an* error came back. `ambientLook` had no test at all. And the
sentence 36A and 36B both rest on — *however long watching runs it cannot make anything durable* —
was enforced by nothing at all; the first fix for that drove the session runner instead of the
passive door and the mutation survived a second time. Two pieces of code came out: a write-only
state map that looked load-bearing, and a guard that restated a rule enforced elsewhere.

## 36C: `marco observe learn` — repeated observation may become knowledge

36B ended with the gap written into its own ADR: repeated evidence did not become knowledge on its
own. 36C closes it, and the sentence is:

```
marco observe
marco observe learn
   [do a thing]
   [later, do the same thing again]
Marco knows it
```

Nobody typed Learn. Nobody was asked anything. Nothing was invented that they did not do.

**Watching and remembering are two things to agree to, so they are two switches.** Learning is off
by default and does not survive a restart — the same two rules watching itself follows, for a
sharper version of the same reason. `marco observe` and `director status` both report which,
either way, because a status that folded them into one would let somebody read "watching" and not
know whether their desktop was becoming permanent memory.

**A candidate is not knowledge.** `observe.WatchedEdge` says *I have seen this*; a
`RememberedRelationship` says *I know this*, and a planner handed one will walk it. Collapsing them
would make the first sighting of anything immediately plannable. Candidate evidence lives in the
same file (one atomic rename, one home, no second durability implementation) behind an interface
that cannot write a subject, a relationship, a goal or a judgement.

**The policy is pure and dull.** Two independent occasions — a minute apart, or in different
watching sessions. No contradiction, and a contradiction is `Never` rather than `Wait`, because
more of the same evidence deepens it. Both endpoints describable, the control nameable, human work
only. No score anywhere: a number between nought and one is a way of avoiding a decision, and the
failure mode here is permanent.

**Identity is structure, and two bugs proved it.** Keying candidates on a content hash split the
evidence for one screen read at two widths across two records that would each have waited forever;
recomputing the handle on every fold did the same thing a second way. The handle is assigned once
and kept, exactly as `RememberedSubject.ID` is, and matching goes through `CompareStructure` — the
canonical test, with the tolerances that exist because two readings of one screen differ. A third
bug gave both ends of a relationship the same transient name, so the admission boundary established
one place and produced an edge from it to itself.

**One admission path.** `promotion` is 36B's object; ambient promotion builds one and hands it a
one-step demonstration. No goal, no play, no name invented from anonymous behaviour, no rehearsal,
no grant, no lease, no input. And `plannableEdges` now asks `CleanlyObserved` rather than restating
it — it was a second copy of the rule the lowering gate grew in 36B, which is one rule with two
futures.

**Not in Activity, and that is a decision.** Every node in the action graph is a replayable desktop
action with an intent and a binding; a memory operation is neither. Visibility is the two status
surfaces and the ledger's own record of when each thing became knowledge.

**Not done, and owed.** Ambient learning stays off by default — the architecture makes the two
lifecycles separately controllable, which is what 36C was for, and whether the shipping product
couples them is a consent decision rather than a default to drift into. `ByMarco` evidence is
discarded rather than kept for execution confidence. There is no unlearning: contradictory evidence
after promotion is recorded and does nothing. And the live acceptance is UNMEASURED —
`acceptance-36c.ps1` is the harness, it drives nothing, and it insists you do the route twice. See
[[ADR-095-repeated-observation-may-become-knowledge]].

### 36C, live: the first run of ambient watching found a twenty-second blind spot

The Go suite was green for three roadmaps and the first live run said: one screen recognised, no
transitions, nothing learned. Every number was honest and every one of them was about the wrong
window.

**An ambient session pins the foreground at START and reads it for its whole twenty seconds.**
Watching was switched on from a terminal; the person opened Settings a second later and walked the
route; Marco read the terminal throughout. A mode whose promise is "use your computer normally"
cannot be blind for a third of a minute after every window switch — and it had been, in all three
roadmaps, because nothing had ever run it against a desktop that moved.

The session now ends AT the switch and the supervisor opens one on the new window immediately, with
attention back at busy. One `winctx.Active()` per reading, which is far cheaper than the reading it
gates.

**And the harness could not say which of five things had happened.** `0 moves` covers: no session
started, sessions took no readings, perception degraded, screens read but not recognised, and
screens recognised that never changed. `acceptance-36c.ps1 -Round` printed five counts and none of
the reasons — the same one-sentence-for-four-problems failure the Director itself was fixed for
twice, reproduced in the harness meant to catch it. It now explains, and `-Why` dumps everything
the Director already knows.

**And the fix was not enough, twice over.** Ending the session at a switch stopped Marco reading
the wrong window; it did not make it start reading the right one promptly, because the supervisor's
next move was the between-sessions ATTENTION wait — a backoff about how long to leave a desktop
that has been sitting still, applied to somebody who had just switched program. The advice that
came out of the first fix was "give it a couple of seconds", which is ceremony, and ambient
watching exists to remove ceremony. It now continues immediately, and the wait between readings is
broken into tenth-of-a-second glances at the foreground: a cheap Win32 call against a
two-hundred-millisecond screen reading, three orders of magnitude apart, so asking often costs
nothing. Measured at ~120ms from switch to watching.

**A third bug came out of looking.** A screen Marco cannot name is keyed on the session-local state
it was read from, and those counters restart every twenty seconds — so two different unrecognised
screens either side of a boundary compared EQUAL, no transition was recorded, and the evidence
vanished silently. Not a rare edge: one boundary every twenty seconds, all day. The name now
carries the session.

**There was no way to see any of this happening, and that was the real complaint.** Every mode in
the harness was a snapshot taken afterwards; a number at the end cannot say when it went wrong or
what Marco was looking at while it did. `acceptance-36c.ps1 -Tail` is a live view — which window is
being read, whether the page is readable, whether the screen is known, and the counts — printing
only on change. One line of it would have made the original diagnosis immediate: the application
column would have read `powershell` the whole time.

**And the harness silently changed what was being tested.** `Use-Sandbox` sets `$env:MARCO_HOME`
at process scope, which PowerShell keeps for the whole terminal — so the second `-Setup` in one
window looked for the person's real memory at the sandbox path it had just deleted, said "starting
cold", and ran the entire acceptance against an empty store. `-Report` then told them their real
store was under TEMP, which is not true and would be alarming if believed. The real store is
resolved once now, before anything is sandboxed, and pinned so later invocations agree.

**Marco can now say what it is waiting for.** "Noticed four relationships, learned none" is true
and useless: one occasion short, an unnameable control, a button that leads two ways and a screen
nothing can establish are four situations with four different remedies, and the counts cannot tell
them apart. `marco observe status --evidence` reports every relationship the ledger holds, across
every application, with the policy's own verdict and its own sentence. `ambient.Describe` had those
sentences since the day it was written and NOTHING CALLED IT — the third unreachable thing this
session, after an unreachable discriminator and an uncalled mechanism.

It names the control and says whether the screens are known. That is a deliberate widening of the
diagnostic surface: a person asking what Marco has recorded about them is entitled to the answer,
and a privacy boundary that made Marco's own memory unreadable to its owner would be protecting the
wrong party. It is a read — it judges, which is pure, and reports.

## 36C.1: the graph is the knowledge — one clean traversal, and the 60-second ceremony removed

36C shipped with a threshold: two independent occasions, at least a minute apart or in different
watching sessions, before ambient evidence became durable knowledge. The threshold was wrong, and
the reason it was wrong is worth keeping rather than quietly editing out of the ADR.

It came from thinking of the unit as a **demonstration** — something a person performs, which one
performance is too thin a sample of. The unit is not a demonstration. It is **one edge of a
semantic graph of the computer.** A person pressing "Bluetooth & devices" on Settings Home and
arriving at the Bluetooth page has not given Marco a sample of a habit; they have told it **what
that control does**, which is a fact about the program and not about them. Waiting for a second
crossing was asking somebody to prove that a door they had just walked through was still a door.

**The cost was not only latency.** Under the old rule, two crossings watched a week apart were two
pieces of *pending* evidence rather than two known edges, so the route composed from them did not
exist. Marco could give back only what it had been shown as a whole. That is the behaviour of a
workflow recorder, and the whole point of ambient observation is that Marco is not one.

**What changed.** `DefaultOccasions = 2` became `DefaultTraversals = 1`. `IndependentGap` and
`ambient.Independent` are gone — the sixty-second window existed only to stop somebody flicking
between two pages from counting as several demonstrations, and with no threshold to protect there
is nothing for it to do. `WatchedEdge.Occasions` became `WatchedEdge.Sessions` and is now
**provenance**: it says how widely an edge has been evidenced, it is read by eviction and by the
report, and it gates nothing. `Policy.Traversals` remains, so a deployment that wants corroboration
before believing anything can still ask for it.

**Repetition did not stop mattering; it moved.** It used to be a gate in front of the edge and it
is now strength on the edge. A crossing of something Marco already knows folds into the existing
relationship through the same admission path the first one took — `Runtime.strengthen` — so the
count rises and nothing new is created. Without that, a way somebody takes every day and one they
took once would look identical to everything downstream, forever.

**Two defects fell out of making the change, both of which only exist because promotion is now
immediate.**

The first: a promoted edge grew a pending twin. The first traversal establishes both screens, so
the *next* traversal reads them as durable subjects — and the candidate matcher compared ends
either as two ids or as two structures, with no arm for one of each. So it matched nothing, minted
a second candidate, and the ledger would have carried an edge and a shadow of it growing in
parallel forever. `sameEnd` now takes a `Recogniser` and resolves the mixed case by asking the
store whether the described end IS the recognised one. Held by
`TestATraversedEdgeIsNotRelearnedOnceItsScreensAreKnown`.

The second: contradiction was one-sided. A control that leads two different places marks the
candidate it disagrees with — but the observation that CAUSED the disagreement went on to be judged
on its own and, with a threshold of one, promoted immediately. The graph would then have recorded
that pressing X on A leads to C on the strength of an observation whose entire significance was
that it disagreed with something. Both records are marked now.

**And an honest consequence, recorded in the ADR rather than left for somebody to find.** Learning
on the first clean traversal means the first edge is often already knowledge by the time a second
crossing contradicts it, and there is no unlearning. The faster rule is more willing to be wrong
about a genuinely ambiguous control, in exchange for being useful about the overwhelming majority
that are not. Unlearning is the roadmap that fixes that; a higher threshold is not, because it buys
the ambiguous case by taxing every other one.

**The headline gate is a route nobody demonstrated.** `A --X--> B` watched on one occasion,
`B --Y--> C` watched in a different session with nothing connecting them, and the planner asked for
A → C returns a two-step plan. That is the test that would fail if any of this were quietly
rebuilt as workflow capture, and it is where the claim now lives:
`TestEdgesWatchedApartComposeIntoARouteNobodyDemonstrated`.

**The tests that fell were the interesting part.** Five failed on the policy change and every one
of them was encoding the workflow model: *seeing something once is not knowing it*, *flicking back
and forth is one occasion*, *further sightings do not learn the same thing twice*. They were not
wrong about the code — they were right about the code and wrong about the product, which is the
only kind of test failure worth reading carefully. Each was rewritten to the corrected model or
removed; `TestAskingWhatMarcoIsWaitingForSaysWhy` now gets its "waiting" row from a policy that was
explicitly asked for corroboration, because the default no longer produces one.

**Live acceptance remains UNMEASURED.** `acceptance-36c.ps1` no longer asks anybody to do the route
twice with something else in between: round 1 is expected to produce knowledge on its own, and
round 2 is for a DIFFERENT way out of a screen already seen, which is what shows edges composing.
It also records a baseline of the store at `-Setup`, because that step copies a real semantic memory
in — without one the harness would have credited ambient watching with everything the person had
ever taught Marco.

**The mutation gate found four things, and the pattern is the same one this repository keeps
paying for.** Sixty-three attacks over three rounds. The first round killed 29 of 38 that applied.

*A field written and never read.* `WatchedEdge.First` — when a relationship was first taken — has
existed since 36C and nothing anywhere read it. Deleting the guard that keeps it stable survived,
because there was nothing downstream to notice. It is now on the evidence read as a span: "traversed
twelve times, over six days", silent when the span is under a day. That is the honest fix rather
than deleting the field, because the count alone genuinely cannot tell a route somebody has used all
week from one they clicked through a dozen times in a single confused afternoon, and neither can the
last-seen. Fourth unreachable thing in two roadmaps, after a dead discriminator, an uncalled drain
and unreachable sentences.

*A branch every existing test walked past.* `endKey` returning a recognised end's subject id — the
thing that stops the admission boundary establishing a SECOND copy of a screen it already has.
Every test in the file handed the ledger transient keys at both ends, the way a cold Marco does, so
none of them could reach it; and in life it is reached on the second edge out of every screen. The
graph would have forked, with the edges divided between two copies of one place and neither able to
reach the other's destinations. `TestAnEdgeOutOfAKnownScreenReusesIt` enters the way life does.

*Two fixtures that agreed on two criteria at once.* The eviction order test paired candidates that
differed on the rule under test AND on a later tiebreak, so deleting the rule left the comparison
passing on the tiebreak. Both `promoted is strongest` and `more sessions breaks a tie` did it. Every
fixture now loses on the criterion being tested and wins on all the others, which is the only shape
that can attribute the answer.

*And a guard whose reachability I nearly talked myself out of.* `stamp`'s zero-time check looked
defensive: every edge goes through `Fold`, and `Fold` always sets `First`. But the ledger is a FILE
— an older Marco, a hand edit, a future rename — and the loader reads what is on disk rather than
what this version would have written. `TestASummaryWithNoTimesReportsNone` writes such a row and
asks for the report, because a date in the year one is exactly the sort of value somebody quotes
back as a fact about their own afternoon.

Two survivors in round one were the harness lying rather than the code being untested: the attack
list named the wrong packages for `WeakerThan`, whose tests live in `ambient` and not beside it.
Three more were reported BROKEN by the verified-apply check — the attack file used `|` as its field
separator and the perl alternation in three expressions contained one. A gate that had silently
called those three "survived" would have sent me looking for coverage that already existed; the
verified apply is why it did not.

## 36C.2: Learn teaches the graph — the boundary that was not there

36C.1 established the model on the ambient side: what watching learns is a semantic graph edge, and
edges watched apart compose into routes nobody demonstrated. This roadmap went looking for the same
correction on the explicit Learn side, expecting to find a private route store and remove it.

**It was not there.** The audit traced every Learn entrance to the same store, through the same
identity test, into the same planner:

- A live `marco learn "…"` is an ordinary observation session carrying `LearnLicence()`. The runner
  writes places through `PlaceStore.EstablishPlace` and edges through
  `Memory.RememberRelationships` — **one observation per transition**, endpoints resolved by
  `Recall`, which is `CompareStructure`.
- `marco learn "…" --recent` builds a `promotion` — 36B's licence-gated admission object — and
  admits **one candidate per leg**.
- Ambient promotion builds the identical object and admits one candidate per promoted edge.
- An edge's identity is `(application, from-subject, to-subject)`. No name, no episode, no session,
  no timestamp, no coordinate.
- A goal has no start. It is a name bound to a destination subject, decided by ADR-056 in the
  goal-centric roadmap and already correct.

So the deliverable is the proof, not the correction — and the proof is what was missing.

**The seam had never been crossed by a test.** Every Learn test drove Learn; every Observe test
drove Observe. Nothing in seventy-odd packages had ever taught one edge through one door, the next
edge through the other, and asked the planner to walk both. That is the case a person reaches on
their second afternoon, and it is exactly the case a second graph would break. The mutation that
namespaces explicit Learn into `settings_learned` — the most literal possible "second graph" —
survived the whole suite before this roadmap, and dies now.

`cmd/director/onegraph_test.go` is eighteen gates through the two production doors against one
store: both compose orders, both duplicate orders, a route entered halfway, another way in found
later by watching alone, a shorter way found later that the demonstration does not veto, two names
over one route duplicating nothing, and the edges surviving both the episode and the process.

**Two things were genuinely at risk and are now held.** Prospective Learn establishes places at a
different call site from the promotion boundary — two call sites, one store — so a drift between
them would have produced a second copy of every screen somebody had both walked past and been
shown, with every existing test still green. And the mutation that re-mints a place the other mode
established now dies against a real licensed pass driven through the production registry.

**The Play was audited rather than assumed, because it was the likeliest place to find private
route knowledge.** A learned Play is a registered phrase plus a readable generated `.marco` file
with an entry guard on one screen and a fixed sequence of presses — genuinely route-shaped. It is
not the execution path. Asking for one by name goes: resolver → `performLearned`, which refuses to
run the file locally → `PerformGoal`, which resolves the name to a destination, brings the
application forward, takes a **fresh look at where the person actually is**, and asks
`observe.PlanToGoal` over the canonical topology. Measured: the same goal asked from Home and from
Printers & scanners produces two different first steps.

**The compatibility debt, recorded rather than hidden.** The saved `.marco` still describes one way
in from one starting screen and is not regenerated as the graph improves, so somebody reading it
reads the demonstration rather than what Marco would now do. That is a readability gap and not an
ownership gap — deleting the entire routes tree leaves the graph, the goal and the plan intact, and
there is a gate that does exactly that.

**A structural finding worth naming:** the semantic store has no delete path at all. Forgetting a
play unregisters a phrase and *cannot* reach the topology, because there is nothing to reach it
with. The gate that holds "forgetting a play is not forgetting what Marco saw" is therefore holding
a property that is currently structural. It is worth keeping precisely because the first delete
anybody adds is the moment it stops being.

**And a false survivor the verified-apply check could not catch.** The first attempt at that gate's
mutation established a place from a signature carrying only a member count. The edit applied — so
the harness called it a survivor — but the store refused the under-described signature, so the
mutation did nothing at all. A no-op that changes bytes is not a no-op the byte comparison can see;
only reading the survivor and asking "would this actually have done anything" catches it. Rerun
with a signature the store accepts, it dies.

**And one gate that could not see the arm it was aimed at.** Every fixture in the new file walks
COLD ground — screens the observer can describe and not name — because that is what a first
afternoon looks like. The admission boundary has a second arm for the other case: a key the
observer already recognises IS the subject, and there is nothing to establish. Deleting that arm
survived all seventeen one-graph gates, because none of them ever handed the trail a subject id.
The store is idempotent by signature, so nothing would break today — which is precisely why no
test could see it. `TestAnExplicitLearnOverFamiliarScreensEstablishesNothing` walks the second
afternoon instead, and the mutation dies.

## 36D: language names the outcome — three defects in the thinnest layer

36C.2 proved the two layers below this one: one canonical graph, taught through either door, with
the route chosen at invocation from wherever somebody is standing. What sits above is the step
where words become a destination, and the audit went looking for the first boundary that stops
natural goal association from working.

It found three, all in the same place, and all of them silent.

**One phrase meaning two things resolved by sort order.** `PerformGoal` searched every application
holding goals, sorted, and took the first match. Measured: two outcomes named "open settings", one
in Windows Settings and one in a mail client, resolved to `mail` — because `m` sorts before `s`.
The person who taught it in Settings would have had their mail client brought forward and a route
walked in it, and nothing anywhere would have said so. Deterministic is not the same as right; a
sort order is not evidence about which afternoon somebody is having. It is now
`ambiguous_outcome`, refused before anything is brought forward, naming both applications and the
flag that answers the question.

**A name that stopped meaning what it meant said nothing.** The store REBINDS a reused name rather
than refusing — deliberately, for a reason measured live on 2026-08-17: a goal left behind by a
*failed* learn held its name hostage, so refusing punished the person for Marco's own earlier
failure. That rule is right and none of this touched it. What was missing was the saying. Measured:
teach "mouse settings" for one screen, teach it again for another, and Marco replies

> "I saw what you did. Learned it as mouse-settings. 2 screen(s) I hadn't seen before are now ones
> I know. Still watching."

Not one word about the name having meant somewhere else. They would believe they had two commands.
`observe.ReboundFrom` is now read before the write on both Learn paths and travels to the person.
Rebinding is still what happens; silence is not.

**The diagnostic and the performer were two loops.** `director reach` exists so somebody can ask
what Marco would do without Marco doing it, which is only worth having if it is the *same* answer.
It was a second similar-looking loop: name-only, first-match, unable to honour a supplied identity
and unable to report ambiguity. Somebody debugging "why did it go to the wrong place" would have
been told the right place by a tool that could not see the defect. There is one `resolveGoal` now,
and `PerformGoal` and `Reach` both call it.

**The mutation gate then found a fourth, in my own fix.** The shared-resolver claim in `reach`
survived — because `reach` searched one application, and within one application a name means one
outcome, so the copy and the shared rule behaved identically. The claim was decorative. Fixing it
properly also fixed a real limitation: `reach` derived its application from the last finished
session and refused with "nothing has been observed yet" on a fresh Director — which is exactly
the state somebody asks in. It now searches every application holding outcomes when nobody names
one, the same way `PerformGoal` always has, and the ambiguity refusal became reachable there.

**And one guard nearly went unheld for the same reason as 36C.1's timestamp.** `resolveGoal`
refuses an empty phrase. `RememberGoal` will not write a nameless goal, so nothing this Marco
produces can match one — but the store is a file, and the loader drops only goals whose SUBJECT is
gone. A nameless goal survives a load, and then asking Marco for nothing would fold-compare equal
to it and resolve to a real destination it would walk to. The test writes that file.

**Aliases needed nothing built.** Two names for one destination are two goal records pointing at
one subject: two user-facing names, one outcome, no duplicate topology, no duplicate destination.
The representation was already there; what was missing was a gate saying both work, neither is
weakened by the other, and forgetting one leaves the other and everything they both point at.

Twenty mutations, twenty killed. No new store, no new planner, no NLP, no LLM, and no
foreground-preference disambiguation — reading which application is in front and quietly preferring
its goal is tempting and is resolution context being smuggled into identity. Refusing with the
choice named is the honest first behaviour, and the other is recorded as a follow-on.

## 36E: the planner prefers better evidence, and says why

The planner was breadth-first, shortest chain, ties on subject id. Every edge cost one, and the
only question asked of an edge was a boolean.

**Two things it already knew and threw away.** `plannableEdges` computed which of two kinds of
knowledge an edge had — a completed rehearsal still vouching for it, or a clean human
demonstration — and collapsed both to `true`. So Marco knew which edges it had actually performed
and planned as though it did not. And `WatchedEdge.Contradicted`, the record that one control had
been seen leading two different places, was written by the observer, kept durably, and **read by
nothing** once the edge was promoted: an edge Marco was demonstrably confused about planned exactly
like one it was sure of.

**The model is two small ordinals and a flag.** No score, no weights, nothing to tune. Classes:
verified, watched-more-than-once, watched-once, and a zero value that ranks worst so an ungraded
edge cannot win by default. Routes compare lexicographically on contradictions, effort, weakest
class, actions, and finally the step ids.

**Contradiction is first and is never traded away.** A route Marco does not understand is not
something to weigh against a saved keystroke. It does not make the edge *ineligible* — the
disagreement is about which of two destinations a control reaches and either might be right, so
when it is the only way it is still a way, and the plan says it goes through one. Turning a
preference into a safety boundary would let a single confused reading take a destination away.

**Verification is worth exactly one action.** Both extremes are wrong: ignoring it throws away the
only evidence Marco has about its own ability, and letting it win outright makes Marco open four
windows rather than one to save a hypothetical. One action is the smallest bounded answer that says
*I would rather use the way I have actually done, if it is not much further*. It is a policy, it is
written down, and it is the thing to change if it is wrong — not the shape of the comparison.

**Repetition saturates at two, immediately.** The second sighting is the whole of what repetition
tells the planner: more evidence that the fact is real. It is not evidence that the person prefers
this way, and a class that kept counting would let a habit outvote a contradiction by volume —
Marco would be modelling somebody's routine rather than their computer. Fourteen mornings on the
long way still lose to one clean traversal of the short one.

**The weakest edge, never an average.** Two verified edges and one contradicted one is still a
route with a bad edge in it, and the bad edge is the one that will fail. Likewise two-of-two
verified beats three-of-five: a raw count of verified edges would prefer the longer route because
it has more of them.

**The search is Dijkstra over `(subject, weakest class so far)`.** The class is in the state and
not just the cost because the not-fully-verified penalty is a *path* property, so the cheapest way
to a subject is not necessarily the cheapest way that is still fully verified. Four states per
subject over a bounded topology. Cycles cannot improve a route structurally — actions only increase
and the weakest class only falls — which is also why it terminates.

**Freshness was left out, deliberately.** A durable relationship carries no timestamp; ranking on
one would mean inventing it, and decay that made known graph facts evaporate is worse than none.

**The mutation gate found the same class of defect twice, both mine.** A claim is decorative until
something produces the thing it describes: dropping the traversal counts from the production grade
changed nothing any test could see, so the whole saturating-strength dimension was live in the
policy and dead in production. The pure tests proved what `ClassObservedOften` MEANS; nothing
proved the real Director ever produced one.
`TestTheProductionGradeReportsHowOftenAnEdgeWasWatched` now does, and the same shape of gate holds
the verified class — a `plannableEdges` returning a flat "everything is watched once" would have
satisfied every pure test and ranked nothing at all.

**And `reach` gained `--from`.** The question somebody debugging a route actually has is usually
about a place they are *not*: would it still take the long way from the Home page? Until now the
only answerable question was about the one screen a session happened to end on, and on a fresh
Director there is no such screen — so the explanation surface could not be reached in the state it
is most wanted. It drives nothing and the answer says which source it used.

`director reach` now prints the planner's own reasons rather than a number:

    step 1: Home → Bluetooth & devices
    step 2: Bluetooth & devices → Mouse

    chosen because: 2 actions, every step is one Marco has done and checked

**The largest remaining gap is failure.** `rememberRehearsal` records only completed routes, so a
failed attempt leaves the graph untouched — the right default, since a target that moved is not a
semantic contradiction, but it means a repeatedly-failing edge currently ranks like any other
observed one. That is the next roadmap and it pairs with replanning, which 36E deliberately did not
touch: the route is chosen before execution, and if edge two fails the existing behaviour is
unchanged.

## 36F: a failed attempt is not a false edge

`performPlan` stopped at the first edge that failed and `PerformGoal` returned. That was right
while Marco knew one way to anywhere. 36E made it wrong: Marco usually knows several now, and a
control that has moved since yesterday is a reason to take the other one rather than to give up.

**Two things the walker computed and dropped.** Every step is classified — `target_moved`,
`target_unavailable`, `wrong_state`, `unobservable` — and whatever perception resolved *after* the
action is recorded as `StepRecord.Observed`. Neither reached the view, so recovery would have had a
word like `ended_unverified` and nothing else: unable to tell a stale handle from a screen that led
somewhere else, and unable to say where Marco was standing.

**Nothing new was invented.** `rehearse` already names every way a step can end; what was missing
was the READING — which of those words describe a world that moved, and which describe a boundary
Marco must not work around. Cancellation, a revoked grant, a spent bound and an unreadable screen
stop. `target_moved` earns the same edge again. Everything else about the interface may replan.
**And everything unrecognised stops**, because a word nobody has decided about is not a word to
guess is safe.

**Where Marco actually is** is the half that only shows up in life. A failed step may still have
moved the interface: the action ran, the destination did not appear, and something else did.
Planning from where the edge *began* would be planning from a screen Marco is not on — and the next
edge's source guard would refuse it, which looks like a second failure and is really the first one
repeated.

**Attempt-scoped, layered over durable rank.** The attempt's failures refuse ELIGIBILITY rather
than lowering a rank, so a failed edge cannot creep back in by being the best of a bad set — and
everything else about the alternate route is ranked by exactly the rules the first one was. There
is no weaker fallback mode. When `PerformGoal` returns, the memory is gone: a verified edge that
broke today ranks unchanged tomorrow.

Bounded at three replans, twelve total actions and two attempts per edge, plus loop detection that
reads a revisited Place alongside the replan count — walking back to try another way out of a
screen is exactly what recovery is *for*, so a repeated Place is not wrong on its own.

**The mutation gate was the worst result of the session, and every survivor was mine.** Seven of
twenty-one survived, all because the fixtures were too clean:

*Three boundary arms were invisible.* My `classify` fixtures set only a `Refusal`, so every unset
field fell to the stop-by-default arm and deleting the cancellation, authority and bounds cases
entirely left the test green. In production a step carries a refusal AND the walker's last
classification — a cancelled attempt really does arrive with `target_unavailable` on it — and then
the ORDER is the whole policy. The fixtures now carry both.

*Two guards said the same thing to the test.* The unreadable-source and unknown-place branches have
different sentences and my test checked only that *something* was said, so deleting either survived.

*And two claims were never exercised at all.* Every `carryOn` gate handed it a recoverable failure,
so a `carryOn` that ignored the classification entirely would have satisfied all of them — the
measurement that catches it is a boundary with a perfectly good alternate route sitting unused. And
every bound was checked by pre-loading the attempt and asking once, which proves the bounds are
READ and not that they are FED: a recovery that never incremented anything would recover forever.
`TestRecoveryCountsWhatItSpendsAcrossReplans` drives the loop instead.

That is the third session in a row where the gate found that a claim was decorative rather than
that the code was wrong. The shape is always the same: a fixture that cannot distinguish the thing
being tested from its default.

**`retryMechanics` is classified and not yet acted on.** `target_moved` is recognised as the one
failure that deserves the same edge again after a fresh target resolution; the loop currently
replans for it like any other recoverable failure. The classification was the part that had to be
right first, and the bound it needs already exists.

## 37A: Marco sees through evidence — the audit, and the one seam that was a coin toss

The roadmap's first deliverable was to establish the perception pipeline rather than infer it from
package names. It is:

    window (validated, pinned) → providers.Collector.Collect → observation.Cycle
      → fusion.Engine.Fuse → directorapi.WorldState → buildSample → observe.Sample
      → session → PlaceNow / target resolution / ReachOfState

**There is one door.** `Engine.Fuse` is called from exactly four places — the session sampler, the
foreground pipeline, the one-shot inspect reading, and the text path — and each serves a different
surface rather than a different interpretation. **Every semantic consumer is a session**: ambient
Observe runs a long unlicensed one, Learn a licensed one, execution takes a short look, and 36F's
recovery takes another after a failure. All four go through `liveSampler.Sample`, so all four
consume the same fused reading. They have four sensor budgets, not four perceptions.

**And the asymmetry holds from above too.** `observe`, `semanticmemory` and `ambient` cannot import
a provider, a capture surface or the fusion engine. That is structural: no amount of editing inside
them can grow a second reading of the screen.

**What is not admissible, stated plainly.** The ScreenParser ONNX detector is `ShadowOnly` — the
collector routes its evidence to `Cycle.Shadow` and fusion never sees it. So the honest answer to
"can visual parsing repair a degraded accessibility reading today" is: the *authoritative* visual
provider can, and the ScreenParser experiment cannot, because its evidence has no authority.
Promoting it is a decision with its own evidence to gather, not a wiring change.

**The one thing that was wrong was a coin toss.** `pushActionables` iterated `world.Elements` — a
map — and truncated at `MaxActionables`. On a screen offering more clickable controls than the
bound, *which* of them reached the navigation producer depended on Go's map iteration, which Go
randomises per range. That set is what a human click is attributed against, so the same press could
resolve to a Target on one reading and to nothing on the next — and from outside, "Marco didn't see
what you clicked" is indistinguishable from a perception failure. A bound is fine; which half of a
screen it keeps must not be arbitrary. The offering is now sorted into reading order before the
bound.

The neighbouring paths were checked and left alone: `buildSample` already sorted its ids, and
`AdmittedPlaceName` is order-independent by construction — it returns nothing when two entries
disagree, so there is no first to pick.

**The mutation gate found four more, two of them mine.** Reading the actionables back handed the
caller the live slice the classifier resolves presses against, and my own test could not see it
because a later push had already replaced the list before the check. The `buildSample` id sort had
a comment explaining that it exists so a recorded fixture replays byte-for-byte, and nothing held
it. The window-coherence guard — which stops a neighbour's controls being folded into this Place's
structure — had no test at all.

**And one equivalent mutation, recorded rather than papered over.** `earlierOnScreen`'s final
tie-break on element id makes the comparator total, which the sort contract requires. It cannot
change the offering: two elements that tie on geometry, role and label produce byte-identical
`navsource.Actionable` values, so which one survives truncation is unobservable at this seam. The
line stays because a non-total comparator is undefined behaviour, and the mutation is equivalent
because it is.

## 37B: visual presence is not legal actionability — and ScreenParser stays in the shadows

The roadmap asked whether ScreenParser's evidence has earned a seat at fusion. The audit found
two things, and the second changed what the roadmap was for.

**The measurement cannot be taken here.** ScreenParser is opt-in behind three environment
variables and is nil on an ordinary Director. The model is now on disk —
`tools/vision-export/weights/screenparser-1280.onnx`, 97.4 MB, which was missing when
Experiment-010 last looked — but **no ONNX Runtime shared library is installed**, so the detector
refuses before it starts. Every number the roadmap asks for — match rate, conflict rate,
stability, per-class calibration, cost — has no way to be produced. Fabricating them was the one
thing forbidden above all, so they are reported UNMEASURED and the decision follows from that.

**The firewall admission would have needed did not exist.** `Element.Actions()` derives capability
from ROLE — a thing whose role is `button` is `Invokable` — and `Targetable()` asked only
`Enabled && ClickableByBounds && Any()`. So an element whose entire evidence was a detector
classifying a rectangle as `button` read as **legally targetable**, with nothing anywhere having
claimed a mechanism to press it. Unreported state defaults to usable and any visible box is
clickable-by-bounds, so a detection needed nothing else.

It was safe only because ScreenParser is shadow-only. **A safety property that depends on an
experiment's configuration is not a safety property** — it would have evaporated silently the
moment anyone did what this roadmap was asking about, and no existing test could have seen it.

Affordance and capability are now separate questions. `Actuable` is decided from provenance:
accessibility, native, DOM and plugin sources can say how to operate a control; a vision detector
and an OCR reader describe pixels. The list is explicit and short rather than a rank comparison,
because a threshold would silently admit whatever source is added next.

**And it denies on evidence, never on absence — which was measured rather than reasoned.** The
first attempt required a positive actuating source and broke five tests across three packages:
hand-built queries, capability-pack enrichment, fixtures. "Nobody recorded where this came from"
and "only a camera saw it" are different claims. So the rule denies only when every source
positively is a pixel source.

**The refusal also learned to say which.** Two very different windows answered `Blind()`
identically — one with nothing in it, one full of controls only a camera saw. Told "nothing in
this window can be operated", a person looking at a screen full of buttons would reasonably
conclude their application was broken. It now says the window was described by the screen alone,
with no accessibility information — the diagnostic that would matter most on the day a detector
IS admitted.

**What it would take to reconsider** is written down in the ADR rather than left as a mood: a
runtime, a bounded corpus of coherent readings, match and conflict rates against healthy
accessibility, stability across repeated inference, evidence from a naturally degraded reading,
per-class calibration, and cost against the ambient cadence. The provider's own documentation
records ~0.9s per frame and 1.25 GB resident, which is why it already runs on a separate cadence
with skip-never-queue — against Observe's 1s active cadence that is the whole budget, and it is
the single most likely reason the answer stays no.

Even with all seven, admission should be constrained: geometry and presence before role, role
before structure, and never actionability. The firewall makes that last one structural rather
than a matter of care.

### 37B, corrected: the measurement WAS possible, and it was taken

The paragraph above says the measurement could not be taken because no ONNX Runtime was
installed. **That was wrong, and it was wrong in the way worth recording.** The runtime is in the
repository — `tools/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll`, with an older
1.26 copy beside the plugin. My search looked in `$LOCALAPPDATA`, `$ProgramFiles`, `C:\onnxruntime`
and three levels of `D:\`, and never looked in the repository I was standing in. A background
search I had started and forgotten finished afterwards and contradicted the conclusion I had
already committed.

Three things were needed and all three were present: the model, the runtime, and
`go build -tags onnxvision`. The 1.26 copy fails honestly — *"The requested API version [28] is
not available"* — so even the wrong runtime names the mismatch.

**Measured over the frozen v2 corpus**, 39 frames and 120 annotated regions:

    structural   P 90%  R 63%
    nameable     P 81%  R 88%
    latency      median 895–932ms   p95 ~1.0s
    ScoreV2      66.8   (classical CV 5.0, Grounding DINO P 77% / R 17%)

By sequence it reads menus well and gameplay HUD barely at all — 100%/83% on a static pause menu,
0% recall on freeplay. That is what a UI-trained detector should do, and nameable precision and
recall had been **0% for four milestones**.

**And the benchmark was comparing one model against itself.** `current` and `screenparser` came
back byte-identical: the challenger's backend configured the plugin with process-wide
`os.Setenv`, and a bridge host launches its child on first USE — during the run, after every
backend is constructed. The baseline's plugin spawned inheriting ScreenParser. This is the same
defect `newShadowVision` records having been caught in the first live start, fixed there and not
here: the shape a fix takes when it is applied to where the symptom appeared rather than to every
caller of the pattern. With per-child environment, `current` correctly reports UNAVAILABLE.

**The decision does not change, but its reason does.** REMAIN_SHADOW_ONLY on measured cost and an
unmeasured workload — 895ms against Observe's 1-second cadence is the whole budget on one sensor,
and the corpus is a game while the loop that matters runs against desktop applications. Settings
is menu-shaped, so the result is encouraging; encouraging is not measured.

The firewall half of 37B stands unchanged and was always the more important half.


## 37C — the desktop perception corpus, and what a detector is actually for

37B left one sentence hanging: *"the corpus is a game while the loop that matters runs against
desktop applications."* 37C built that corpus and asked the question admission turns on — not
"is ScreenParser good?" but **what does it add to perception that already works?**

Six coherent desktop moments, each a screenshot plus the production fused world for that same
moment, pinned to one window and bracketed by a re-acquisition either side so incoherence is
recorded rather than assumed away. Three families, two reflow pairs. Plus a synthetic
`browser-fixture.html` carrying two items planted to be got wrong: a real button that is
`disabled`, and a `<span>` styled exactly like the buttons beside it and wired to nothing.

**The answer is nothing measurable.** 537 production elements against 473 detections: 64% of
detections have no production element under them at IoU, which looks generous until it is read.
**All 302 of those have their centre inside an element production already perceived.** Narrowing
to the fairest possible reading — a detection whose only container is an unlabelled,
non-actionable region — leaves 35 (12%), topping out at 0.49 confidence, and they are boxes
without meanings: the detector says `text`, not what the text says.

**Its most confident unique finding in the whole corpus is the trap.** `#looks-like-a-button`,
at 0.63. ScreenParser calls it a `button`; production calls it `text, actionable=false`.
Production also reads the disabled button as disabled, which no detector class encodes. On the
one item where visual and semantic truth disagree, the detector is confidently wrong. That is
ADR-101's firewall measured rather than reasoned about.

**The performance gate, skipped in 37A and 37B, was finally taken.** Fusion costs **0–1ms**. The
entire cost of production perception is the accessibility walk — 104–120ms on Settings, and
**~1.5s on an Explorer tree**. A ScreenParser pass is 645–1379ms. So the real performance problem
in the stack is the accessibility walk, it is not fusion, and it is not the detector. Both layers
show zero jitter across three unchanged readings.

`37C_DECISION = SCREENPARSER_STRONG_DEGRADED_REPAIR_CANDIDATE`. Both halves are needed:
NOT_USEFUL_ENOUGH where accessibility is healthy, and Experiment 015's result stands where it is
absent. Those are one finding seen from two sides — ADR-102: **a detector is admitted by the
absence of better evidence, never by its own quality.** The next question is degradation
detection, not admission.

**Two things were caught on the way, both of the same shape as the session's running theme.**
The `valued` reordering table swallowed `--dir`, which is the sixth time a value-taking flag has
been silently reordered; the gate that exists to prevent it was blind to `fs.Var`, whose name is
its *second* argument. Closed, and the closure is mutation-proven — `--redact` is the one Var
flag no String flag shares a name with, so deleting the fix now fails a test. And the corpus was
captured from a real desktop, so it carried the person's name, email and home-folder tree.
Redaction is a command with a recorded result rather than a manual pass, it is **geometric**
rather than name-based (source that carries the name to scrub it is the leak it was written to
prevent), and three tests hold the committed artifacts to it.

Also corrected: `docs/subsystems/Vision.md` still said no ONNX Runtime was installed. That claim
was retracted in 37B's ADR and HANDOFF and not in the subsystem note.


## 37D — knowing when the primary sensor has not actually shown you the interface

37C ended pointing here: a detector is admitted by the ABSENCE of better evidence, so the next
question is detecting that absence. 37D's audit found the answer already largely built, which
changed the work from "write a classifier" to "name what one already decides, and prove it is
right about real applications rather than about its own fixtures".

`observe.ReachOfState` has judged this since 35C, and it was built carefully: it decides on
ARRANGEMENT rather than richness — find the largest space any structure claims, and if it
covers a serious share of the window with almost nothing inside it and nowhere else is
populated, the page is present and unread. It names no application, consults no clock, and
depends on no optional sensor. `PlaceNow` asks it BEFORE consulting memory, so a shell-only
reading never becomes an unrecognised page. The audit found no count heuristic, no latency
input, no app-specific branch and no sensor dependency anywhere in the quality path.

**What was missing was proof and a name.** Every fixture in the package was written by the same
hand as the rule, so a rule subtly wrong about real applications would pass all of them. 37C's
committed corpus fixed that: the seven real desktop readings — Settings at two widths, a second
Settings Place, Explorer, a browser at two widths, and Paint — all classify as content reached,
which is what they were.

Paint is the interesting one. Its canvas is 68% of the window and empty, and it is sufficient
because its ribbon sits INSIDE the same top-level pane, so somewhere in the window has things in
it. Not a special case for drawing applications: the populated-panel rule, doing exactly what it
was written for. Captured live at 106 elements and committed.

`observe.Sufficiency{State, Reason, Vacancy}` is the new name and the seam 37E consumes — three
states that must not collapse, five bounded reasons, and NO sensor named. It says "the primary
reading did not represent the interface", never "run ScreenParser", because that decision has to
weigh cost against what is missing and belongs to whatever schedules perception.

**The custom-surface policy is stated rather than left to a threshold.** A window whose
accessibility reports only frame furniture around an empty client area is `incomplete`, and a
game viewport is the clearest case. That is the intended answer: the verdict is about the
READING, the description shown to an owner contains no word like "broken" or "failed", and a
game surface is precisely where extra perception is worth spending. A classifier calling it
sufficient would close the door on the case that motivated the whole line of work.

**History is deliberately not consulted.** It could corroborate a collapse, and the structural
evidence already detects the original live failure without it — so adding it would be
unfalsifiable complexity carrying a real hazard, an expectation that fabricates current
elements. The seam is named if a future case ever earns it.

**The mutation gate found five survivors and none was equivalent.** All five were the same
defect as 36F and 37A: fixtures too clean to see what they test. Every degraded fixture was
small, so "more than twenty structures is healthy" passed; nothing made ratio and count
disagree; nothing had two vacancies to choose between; nothing sat between 0.40 and 0.20; and
the sufficiency tests only asserted the two reasons the headline cases produce. Four new
fixtures, each sitting on the far side of exactly one boundary, and 14 of 14 now die.

Cost, measured: judging a reading is p95 514µs against 104ms–1.6s to acquire it. **The
performance problem in perception is the accessibility walk and it is not this.**

Live: two healthy Settings Places sufficient; the same Place at 111 elements wide and 57 narrow
BOTH sufficient; Explorer at 294 elements and 1625ms sufficient. No naturally incomplete reading
occurred, so no live degraded case is claimed.

**35D's resize acceptance is still unmeasured, and the reason is worth recording.** Sufficiency
at both widths is half the claim; the other half is that both are the same Place, and the
capture-to-totals adapter cannot produce a StructureSignature — every sample signs as empty and
`CompareStructure` then answers "candidate" for any pair, including two obviously different
Settings pages. Checked rather than assumed, and the limitation is written into
`assessdesktop.go` so the next person does not claim it either.


## 37E — perception is a budget, not a habit

Two questions, both answered by counting rather than reasoning. `director walk-audit` runs a
window's reading repeatedly through the production collector and fusion engine and reports what
each walk cost.

**The bridge is not the problem.** One Snapshot is a single bulk CacheRequest over the whole
subtree — every property of every node in one cross-process call, which is the right shape.
It keeps no state between calls, subscribes to NO UI Automation events, and has no cheap
targeted read: `Locate` exists for actuation and searches the same tree. Exactly one walk per
Collect, so there was no duplicate to eliminate — Level 0 and Level 1 on the roadmap's ladder
had nothing to take. Polling costs nothing either; `placeHereIn` reads a session's accumulated
evidence and starts no walk. The sampler already refuses to queue a backlog.

	File Explorer   298 elements   1674ms mean   identical every reading
	Settings        155 elements     70ms mean   identical every reading

**What was wrong was how often, and the answer was already in the tree.** Ambient watching's
`attention` grows from one second to eight while nothing changes and snaps back the moment
something does — governing the gap BETWEEN twenty-second sessions, while each session it opened
sampled at a flat second regardless. About seven full walks of an unchanged Explorer tree per
session, roughly 42% of wall-clock rebuilding it. Passing the same attention through as the
session's interval is the whole change: 7 walks a session to 2, ~42% to ~12%. Settings, 18 to 2.

It retains NO state. Every sample is still a complete fresh walk taken at the moment it is
reported; there are fewer of them when nothing is happening. Nothing is cached, so nothing can
go stale — it is not on the caching ladder at all. And it reaches ambient only: `freshLookInterval`
is 400ms and is not the supervisor's, so execution does not slow because the desktop is quiet.

**And nothing consulted sufficiency to decide sensors.** ScreenParser is opt-in at process
level and already cadence-gated, so it was never unbounded — but when enabled it inferred
against healthy readings 37C had already proved it adds nothing to. `observe.EscalationOf` is
now the one place that decides whether more perception is worth paying for, and the shadow
provider is its first consumer. It names no sensor: `SpendMore` says more evidence is worth
buying, and which evidence belongs to whoever holds one and knows its cost.

Three rules carry it. A sufficient reading buys nothing — 37C measured that directly. A fresh
incomplete reading settles first, because a page mid-navigation is briefly indistinguishable
from one that failed to arrive and waiting is free. And nobody waiting is not worth spending
on: a game viewport is incomplete for as long as it is in front, which is a standing condition
rather than an event.

The gate declines only on a positive answer. No session, no memory, nothing settled yet are all
Marco not knowing — and a gate reading those as "no need" would silently end the experiment it
gates while looking like an optimisation. Deny on evidence, never on absence, the same rule as
`Provenance.OnlyDescribesPixels`.

**The mutation gate found four survivors and every one was the same shape.** The policy and the
cadence both had thorough unit tests, and reverting the production cadence, making the gate
decline on ignorance, and having the gate reach its own verdict instead of asking the policy all
left them green. The rules were tested; nobody had tested that production asks them. 14 of 14
die now.

Nothing about the firewall moved. No cache, no event subsystem, no targeted refresh, no
persistent scene graph — the last is documented as a follow-on rather than quietly begun.


## 37F — the loop was not open, it was stuck open

37E left one thing outstanding: EscalationOf could decline an expensive sensor and nothing ever
acquired one because it said yes. The expectation was a missing acquisition path.

**There was no missing path.** `observation.Request.Include` already opts extra sensors into an
ordinary cycle. `WithVision` and `WithPixels` already exist, and WithPixels is documented as
"what an application exposing no accessibility tree needs". `rt.vision` and `rt.ocr` are
AUTHORITATIVE providers in the production collector — no ShadowOnly marker, so their evidence
reaches fusion and therefore reaches SufficiencyOf. And the session sampler already asked for
vision.

It asked on EVERY sample. 37E's gate reached shadow.Provider, which by construction can never
reach belief; the authoritative detector beside it was ungated. Measured, detector configured:

	Settings    66ms / 155 elements  →   940ms / 176 elements   sufficient either way
	Explorer  1567ms / 298 elements  →  2460ms / 307 elements   sufficient either way

Fourteen times the cost on Settings for the same answer — 37C's finding arriving from the other
direction. So the change is one condition: the sampler asks the same wiring that already gated
the shadow provider, and declines only on a positive statement that the reading suffices.
~875ms saved per sufficient sample on Settings, ~890ms on Explorer, verdict unchanged on both.

**And the firewall became a standing gate rather than an argument.** The browser fixture carries
two controls planted in 37C for exactly this. `#looks-like-a-button` — a span styled like the
buttons beside it, wired to nothing, which ScreenParser called a `button` at 0.63, its highest
confidence unique detection in the whole desktop corpus — stays `text` and stays untargetable
after visual evidence is admitted, with the visual account recorded rather than discarded so the
disagreement remains reviewable. `#disabled-action` stays disabled. And a detection with nothing
beside it is readable and not operable, which is the case repair exists FOR and where the
firewall matters most.

"I now know where I am" while still saying "I cannot safely act on that control" is a successful
repair, not a partial failure.

11 mutations, 11 killed — including the four that would quietly restore the old behaviour and
the five that would let pixels become permission.

**One honest cost.** On a machine with $MARCO_VISION_MODEL configured, sessions previously fused
visual evidence into every reading, so a Place learned there has a signature built from richer
evidence — 176 elements against 155. Recognising it now compares a smaller signature against a
larger remembered one. Stated rather than measured: the capture-to-totals adapter still cannot
build a StructureSignature (37D), so it cannot be checked offline, and it needs the same
live-store session 35D is waiting on. It is confined to configured machines; with no model the
detector reported unavailable and contributed nothing.

## 37G — ask the actual graph what place Marco thinks it is in

Three phases had left the same measurement outstanding and none of them could take it offline:
35D's resize acceptance, 37D's finding that the capture-to-totals adapter cannot build a
`StructureSignature`, and 37F's stated risk that a Place learned with visual evidence has a
richer signature than the same page read by accessibility alone.

So this one asked the running system: an isolated `$MARCO_HOME`, a real `semanticmemory.Store`,
`director learn` to establish and passive sessions to resolve, with `ObserveShowing` — the one
"where am I standing" door — printing the durable subject. `acceptance-37g.ps1` runs it. Marco
emitted no desktop input: navigation is `ms-settings:` shell activation, resizing is
`SetWindowPos`.

**Two defects had to be fixed before the question could be asked at all, and both were invisible
to a green 85-package suite.**

*Every observation session stopped after one sample.* `liveSampler.Sample` holds `Runtime.mu` for
the whole collect-and-fuse, 37F put the escalation gate inside that section, and the gate's
`incompleteFor` took `Runtime.mu` to guard a timestamp. Not reentrant.

	pre-37E build            9 samples / 12s
	HEAD, gate bypassed     14 samples / 12s
	HEAD                     1 sample, then silence

The FIRST sample survived — nothing had settled, so the gate returned before reaching the lock —
which is why a hang looked like a slow start. Learn, Light Mode, ambient watching and the fresh
look a performance takes were all dead on the branch. Every existing test of that gate calls it
directly, holding nothing.

*A window emptied because it had been watched for longer.* `ReachOfState` divides structures
inside the largest space by the structures the observation found; the second half returned every
structure ever seen in the state, which grows with the session.

	 14 samples    466 ever-seen    142 present    recognised
	 27 samples    817 ever-seen    142 present    recognised
	 40 samples   1024 ever-seen     88 present    UNREADABLE
	183 samples   1024 ever-seen     88 present    UNREADABLE

Eighty-seven structures sat inside the content region throughout. Recognition stopped working
part way through every long look and reported it as a fact about the page — the one diagnosis
that classifier exists to route correctly.

**Then the measurement.** One Settings page is one Place from 1500px down to 850px, a 43% width
reduction, with a signature that is not merely inside tolerance but IDENTICAL, while the fused
element count moves 115 → 84. Both directions. Two clean restarts change nothing. Mouse and
Bluetooth stay distinct at every width pair including narrow against narrow, and the store held
exactly two subjects for two pages.

Below about 850px Windows Settings removes its navigation pane — thirteen images and twelve of
fifteen list items — and adds a search term. The matcher says `different` and Marco mints
nothing. Recorded, not fixed: making it match means dropping `image`, `list_item` and
`text_field` from identity, which is what tells Settings pages apart, so the reflow case would be
bought with the false-merge case. A false miss is preferred to a false merge.

**Sensor richness: recognition survives it, establishment does not.** A Place remembered from
accessibility alone resolves to the same subject with the detector contributing, and the
signature is byte-identical even though the fused world grows 115 → 135 elements. But one cold
`director learn` with the detector configured left TWO Mouse Places: the gate is ignorant early
in the pass so it buys the visual pass, a Place is established carrying nine `icon` structures,
the reading is then placed and sufficient so the gate declines, the composition changes, and the
open licence makes that durable too. Every later reading resolves to the second one; the first is
an orphan. Bounded to machines with `$MARCO_VISION_MODEL` set, and recorded rather than fixed —
holding the sensor set for a session would make a game viewport buy an inference every sample,
which is the expense ADR-104 exists to refuse.

12 mutations, 12 killed. Two survived first: one because the drift fixture put its churn inside
the region it was meant to dilute, and one because a second filter written for the principle
turned out unreachable against the live population — deleted rather than kept as a guard nobody
can hold.

## 37H — a sensor appearing is not the screen changing

37G ended with one defect diagnosed and left alone: with `$MARCO_VISION_MODEL` configured, one
cold `director learn` on one Windows Settings page nobody touched left TWO durable Places. 37H
reproduced it deterministically, found the layer, and prevented the birth rather than cleaning up
the orphan.

**The chain, and every link of it is correct on its own.** The pass begins with nothing settled,
so `EscalationOf` does not know whether the reading suffices and keeps the visual pass (ADR-104:
ignorance is not a decline). Twenty-one detections nothing structural reported join the
composition as `icon`, and one element accessibility DID report — as `unknown`, which fusion
rightly treats as no claim — is named `icon` by the detector. That settles. The reading is now
placed and sufficient, so the gate stops buying (ADR-105). The composition changes back. The
segmenter sees a coherent part of the surface replaced and calls it a different state of the same
surface, which is what that branch is for. And the licence, still open, makes both durable —
because `PlacesToEstablish` walks every settled state, since a demonstration walks THROUGH places.

	state_1   inferences 2    settled   … icon 22 …
	state_2   inferences 9    settled   … unknown 1 …   local_from=state_1  surface=state_1
	transitions   state_1 -> state_2   count 1   unattributed 1

Marco concluded that the world had changed when only its evidence had.

**The suspected heart was not the heart.** The roadmap's hypothesis was `unmatched + licence =
establish`. Measured, no: the licence acts on settled screen STATES, and establishing every state
a pass settled on is right — an intermediate place that never becomes durable leaves the edges
either side of it unresolvable. The establishment layer asked the segmenter which screens it had
seen and was told two. It was told correctly, from evidence that was wrong. Licence semantics are
unchanged.

**What the composition is made of, measured on the same window seconds apart:**

	vision=false — 115 elements       vision=true — 136 elements
	  button   accessibility x17        button  accessibility x14 + accessibility+vision x3
	  unknown  accessibility x1         icon    accessibility+vision x1
	                                    icon    vision x21

Two different things, and the difference decided the fix. Both single-bit rules were tried and
both were wrong: keeping everything a structural source reported leaves the twenty-one boxes in
the identity, and dropping anything a structural source did not NAME removed `text x29` and
`unknown x1` from a real page. Accessibility described those text nodes and said they were text; a
poorer claim than `button` is not the same as no account at all.

So `directorapi.KindEvidence` says who accounted for what a thing IS — described / pixel-named /
pixel-only — which is a different question from `OnlyDescribesPixels`, which asks whether anything
but a camera reported that the thing is THERE. Pixel-named counts as `unknown`: the object is
real, its kind is a detector's word. Pixel-only is not counted, unless pixels are the whole
reading, which is the case the detector exists for. Classified in `buildSample` beside the chrome
classifier — the last point where evidence and provenance are both in scope — and read at
`NewScreenSignature`, the one choke point the segmenter and the durable fingerprint share. A
label, not a removal.

	                       before    after
	screen states              2         1
	durable Places             2         1
	samples / elapsed    12/6.3s   12/6.1s
	providers proven      a11y+vision  a11y+vision

The surviving subject is `subj_71727a02470f` — byte-identical to the Place a Director with no
detector at all establishes for the same page. A configured machine and an unconfigured one now
learn the same Place, which also closes the compatibility cost ADR-105 recorded. The whole 37G
matrix re-run with the detector on is unchanged: same Mouse, Bluetooth distinct, two restarts
stable, two subjects in the store.

Selective perception is untouched. Nothing was pinned, delayed, frozen or reconciled; no orphan
cleaner, no migration, no matcher loosening, and the rule names no sensor.

10 mutations, 10 killed.

## 37J — what a reflow removes cannot always be told from where you are

37G left one false miss on purpose: below about 800px Windows Settings removes its navigation
pane and the same destination stops being recognised. 37H then supplied a vocabulary that looked
like it might generalise — evidence ABOUT a screen is not evidence a screen is identified BY —
so 37J asked the same question of responsive layout.

**Measured first.** Three destinations (Mouse, Bluetooth & devices, Printers & scanners), six
widths each, through the production path into `SignatureOfState`. The breakpoint is between 850
and 800px and is identical on all three, verified SEMANTICALLY rather than by width: a
`button "Open Navigation"` and a `button "Expand search box"` appear and the navigation list goes.

Lost on every page: 13 `image`, 1 `text_field`, the navigation's `list_item`s, and the selected
navigation entry — which is where `AdmittedPlaceName` reads the page's name, so a collapsed
reading does not know what it is called. Gained: the `search` term. The decisive field is the
role set, on all three.

**Nothing shipped, and two independent reasons why.**

The same interface event is a CONTRADICTION on one page and a SILENCE on another. Mouse has
three list items of its own, so the navigation's twelve leaving takes `list_item` 15 → 3 — a
positive count disagreement on a shared role. Bluetooth and Printers have no content list items,
so the identical event removes the role entirely. The durable signature is a flat role histogram;
`ScreenSignature` has cells and `StructureSignature` deliberately does not, so nothing can tell
those apart. "Absence is not contradiction" is a real seam in `sameRoleSet` — and it does not
reach the page this phase was written for.

And the rule that would work is a measured false merge. Over every pair:

	current matcher                            same 18/45   false merges  0
	shared roles only, terms exact             same 18/45   false merges  0
	shared roles only, terms nested            same 36/45   false merges 15
	shared roles + tolerant counts + nesting   same 36/45   false merges 15

All fifteen are Mouse against Printers: `back settings` is a subset of `back controls settings`.
Requiring a shared distinctive term removes them, but `settings` is generic in an operating
system and destination-bearing in a game's settings menu — which is what the term vocabulary was
built for — so distinctiveness cannot be declared statically.

Printers collapsed settles it: its whole account is `back search settings` plus button, list,
menu, menu_item, window — the furniture of every page of that application. UNKNOWN is the only
honest answer there, and any rule lenient enough to change it would recognise anything.

**What survives, and is not being read.** The collapsed reading contains `button "Mouse"` and
`text "Mouse"` in a breadcrumb — the page names itself at BOTH presentations, and
`AdmittedPlaceName` reads the selected navigation item, which is exactly what disappears. That is
the route to closing this, and it is a perception change rather than a matcher change. It is not
free: the same run caught Printers naming itself "Printers & scanners" at 1500 and 1000 and
"Bluetooth & devices" — its section — at 850. A name that moves between presentations would move
identity with it.

So `responsive_test.go` is a firewall rather than a fix: the measured signatures as fixtures, the
current answer stated with the field that decides it, and a mutation gate that kills the three
loosenings this experiment rejected. `identityprobe` now asks `ExplainStructure` instead of
re-deriving a comparison — it had been reporting layout roles as decisive that the matcher had
already dropped — and reports terms as lost and gained rather than as two lists.

3 mutations, 3 killed. 35D responsive acceptance: PASSED to 850px, CONSERVATIVE_UNKNOWN below.

## 37K — a screen carries several true names at once

37J ended by pointing at naming: the collapsed page still contains its own name in a breadcrumb,
and `AdmittedPlaceName` reads the selected navigation item, which is what the collapse removes. It
also caught Printers naming itself after its SECTION at one width and not the others. 37K went to
find out why, and the answer is a measured negative with two wrong names recorded rather than
fixed.

**The audit.** `placeNameEvidence` (producer, cmd/director) walks the fused world and keeps only
elements the provider reports as SELECTED; `observe.AdmittedPlaceName` is the rule; the result
reaches `ScreenState.PlaceNames`, `settledPlaceName`, `PlaceNamesToRecord` and
`Store.ObserveSemanticName`, with `promotion.establish` covering the retrospective path. Everything
is gated on selection, which is why a collapsed layout produces no name at all.

`AdmittedPlaceName` became `ExplainPlaceName` with the verdict discarded — the
`ExplainStructure`/`CompareStructure` shape — so the reasoning could be read without a second
implementation existing to disagree with it. Suite green before any behaviour question was asked.
Then `director name-probe`, which calls the production producer and the production rule and prints
every claim, its semantic level, the trail evidence it came from, and why. It reports the
presentation as whether anything claims to be the selected destination — not from a window width,
which describes one machine's DPI, and not from a label like `Open Navigation`, which describes one
operating system in one language.

**A screen carries several true names at once.** On Printers at 1500px: `Settings` is the shell,
`Bluetooth & devices` is the section AND the selected rail item, `Printers & scanners` is where you
are. The existing rule separates them with the TRAIL — the group of sibling buttons containing the
selected word is the path, and the entry that is not the selected word is the leaf.

**The Printers trap, in the rule's own account:**

	1500  siblings [Bluetooth & devices, Printers & scanners]  -> "Printers & scanners"
	 850  siblings [More, Printers & scanners]                 -> "Bluetooth & devices"
	 750  no element reports itself as selected                -> no name

At 850 the breadcrumb's ancestor has collapsed into an overflow control, so no group holds the
selected word, the trail lookup finds nothing, and the section is admitted by default. At 750 the
breadcrumb is back and complete — and nothing is selected, so the rule never sees it.

**The obvious fix was implemented, measured, and rejected.** Requiring a trail to corroborate a
selected item fixes Printers at 850 and fixes Visual Studio Code, which today names itself
`Terminal (Ctrl+`)` — a keyboard hint, the exact failure ADR-076's own comment warned about. And it
breaks Settings HOME, which has no trail either: measured, the sibling groups are `[Add device, View
all devices]`, `[Close Settings, …]`, `[Ethernet 2 Connected, …]`, `[Get more storage, …]` and none
holds `Home`. ADR-076 recorded a one-entry trail there; this Windows build no longer produces one,
and Home is named correctly today by the same path that gets Printers wrong.

So `Home` and `Printers at 850` arrive as the identical shape — one selected navigable item, no
trail — with opposite correct answers. Nothing in the evidence the rule receives separates them.

Stage A is PARTIAL: ownership unified, the rule explains itself, destination-versus-section
explicit and mutation-proven where a trail exists, ambiguity honest — and naming does not survive
the breakpoint. Stage B was NOT ATTEMPTED on the roadmap's own condition. Confirmed live, two cold Learns on the same page:

	at 1500px   one Place, subj_71727a02470f, semantic=[Mouse]
	at  750px   one Place, subj_421b923d9093, semantic=[]

so the unnamed-Place debt is unchanged and now has a measured cause — and the two subjects differ,
which is 37J-s false miss showing up in the same run.

Place identity is byte-identical — `recall.go` and `screenstate.go` untouched — and the 37J
firewall passes unchanged.

8 mutations, 8 killed.

## 37L — a navigation rail is a list of places you could go

37K reduced destination naming to one ambiguity: a selected navigable item with no trail means
opposite things on two real screens. Settings Home — selected `Home`, and Home IS the destination.
Printers at 850px — selected `Bluetooth & devices`, and the destination is its child, whose
breadcrumb ancestor has collapsed into an overflow. 37L asked whether anything else Marco has
could separate them. Two routes, both measured, both closed.

**The hierarchy above the selection is byte-identical.** `director name-probe --deep` now reports
what contains every claim:

	Home @1500     selected list_item "Home"                in [group pane window unknown window window]
	Printers @850  selected list_item "Bluetooth & devices" in [group pane window unknown window window]

There IS a structural difference elsewhere — the breadcrumb's buttons sit outside the content pane
at depth 4, every content button passes through a `pane`, and Home has no such group at all, which
is coherent for a root. It was not used: identifying a group by the roles above it is the shape
37D measured into the ground, it rests on one application, and it does not finish the job. At 850
the group is `[More, Printers & scanners]` — two unordered entries, the selected word in neither,
and `PlaceNameEvidence` carries no rectangle on purpose, so "the last one" is not a question Marco
may ask.

**The graph cannot referee it, and the reason is the rail.** The hypothesis was: if the selected
Place has a remembered edge to a currently visible label, that label is the destination and the
selection is only the section. Settings Home kills it — measured, the rail publishes every one of
its children:

	Home, System, Bluetooth & devices, Network & internet, Personalization, Apps, Accounts,
	Time & language, Gaming, Accessibility, Privacy & security, Windows Update, Sound, Display…

A rail is a list of places you COULD go, and it is on every wide page. So "an edge to a visible
label" is satisfied everywhere and the rule would demote every parent, starting with the one
screen whose name is currently right. Constraining it to the breadcrumb group needs the Stage A
signal that is not available. A second objection never had to be reached: the graph's
`Printers & scanners` was named by this same rule at the widths where the trail exists, so
refereeing the rule with it is circular unless name provenance is persisted — and building that to
rescue an already-unsafe idea would be the wrong order.

So: `CONSERVATIVE_UNKNOWN`, nothing changed, and one boundary now enforced rather than intended —
`placeNameEvidence` reads one fused world and nothing else. Memory may help interpret current
evidence; memory may never become current evidence.

Known wrong and staying wrong: Printers at its overflow width takes its section's name; VS Code
names itself `Terminal (Ctrl+`)` (its ancestry does differ — a `tab_list` — so a local rule is
imaginable there, on one data point, not built); and a Learn while collapsed produces an unnamed
Place.

Place identity untouched, 37J firewall unchanged. 4 mutations, 4 killed.

**This closes the naming and identity investigations.** The two screens are indistinguishable to
everything Marco can currently read, and the fact that separates them — a breadcrumb is a path, a
rail is a menu — is not exposed by UI Automation on this application. The next question is not
architectural. It is how much any of this costs in use.

## 38A — dogfood 1: watch me, tell me what you learned

The 37-series closed as READY_FOR_DOGFOOD. This is the first phase whose question is not "does
this subsystem hold" but "what happens when somebody uses it". Two things came out of turning
Marco on.

**A surface that did not exist.** Nothing told a person when durable knowledge changed, so
`marco observe --follow` now prints a line per COMMITTED change — a Place established, a way
remembered, a name settled, a destination bound to a word:

	+ learned place       Mouse
	+ named               Mouse

The events come from `semanticmemory.Store` after its own write succeeded. Nothing upstream
announces, nothing predicts: a Place refused at a bound, a signature matching a record already
held, a file that could not be written all arrive as silence. Four words rather than one —
learned, strengthened, named, rebound — because a feed that said "learned" every time somebody
walked a familiar route would train them to stop reading it. Names resolve when somebody looks,
not when the write happened, since a Place is established on one pass and named on a later one.
Across the whole session it produced ZERO false events.

**A defect that stopped the headline loop dead.** From a cold home, doing exactly what the product
invites:

	marco observe learn   → Marco is watching, learning from what it sees
	marco learn "…"       → phase: refused
	                        refused: no_observation
	                        "I couldn't watch — I lost sight of that window."

Every time. With watching OFF the identical command reached `ready_for_demo` and established a
Place — so it is a slot conflict, and the message points at the window, which sends somebody to
look for a fault that is not there. One observation runs at a time by design; ambient watching held
it and Learn was refused. Watch me, then teach me could not be walked from a command line at all.

Fixed as a blocker: background attention yields to a demonstration, which is the rule Light Mode
already followed, and only ever the session ambient itself started. **The first attempt at the fix
did not work** — it went into `Runtime.Learn`, which only the control surface reaches, so
`marco learn` went on being refused exactly as before. The rule was right and nothing on the path a
person types called it. Fourth time in this series. After the fix: `ready_for_demo`, one Place
committed, the feed said so, and watching came back.

**And the observation half needs a hand on the mouse.** Seven Settings pages, 180 samples, 10
transitions recorded — and zero candidate edges. `noticed` requires exactly one attributed human
ACTION per transition, and actions come from real input. Navigation driven by `ms-settings:` URIs
presses nothing, so from Marco's point of view the screen changed on its own and it correctly
learned nothing. That is the mechanism working; this method cannot exercise it. Edges, goals,
execution, composition, different starting places, midway entry and recovery all sit behind it and
were not reached.

Also found: `marco observe status` says "0 screens and 8 moves" while `--evidence` says "hasn't
watched you go anywhere yet". Both true under their own meanings — the transient buffer versus the
candidate ledger — and together they read as a contradiction that names neither cause.

Verified: feed reports committed knowledge, 0 false events, Place established from cold, survives
restart, recognised after restart, watching resumes after a demonstration. Not reached: everything
that needs a click.

DEMO_READINESS: ENGINEERING_DEMO. The next session needs a human hand: `marco observe learn
--follow`, then navigate Settings by CLICKING, and watch for `+ learned way`.

## 38B — a window that records has to leave itself out of the recording

Studio can now record the ordinary way: press **Record**, go and do the thing, come back and press
**Stop recording**, look at what it got, then **Save** it where you say or **Cancel** it. Four
buttons, four backend calls, and nothing on disk until Save.

This is the recorder-first path — `internal/recorder` → `internal/simplify` → `internal/codegen`,
the widget recorder Marco shipped with, reached from a window for the first time. It is not the
visual recorder in `learn.go`, which is unchanged and still answers the other question (*can Marco
recognise this control again*). The owner asked for it in one sentence: "We need a cleaner way to
end basic recordings like this than f7, I prefer a button to end recording."

**The button is the whole difficulty.** A stop KEY is pressed wherever the person already is. A
stop BUTTON means bringing a window forward and clicking in it, and the hooks record that like any
other click — so every replay would raise Studio and press Stop, and the switch that brought
Studio forward would decide which application the routine was filed under. It took five
corrections to get the exclusion right, each one the same shape: *the fixture was easier than
production.*

1. The "too fast for the poller" fallback consulted **that same poller's last sample**. Event-time
   process id instead, read in the hook from two window-data reads.
2. The builder stamped the process on button-down only — and a button is activated on RELEASE, so
   a strip walking back from the end met an unattributed release and stopped there.
3. The name rule ran after the process rule and could still delete a click the process rule had
   just spared. Known ownership outranks a sampled name in **both** directions.
4. The tail was not enough: a glance at Studio mid-demonstration leaves a switch in the middle, and
   `simplify.foldActivates` reads the step before a switch as the gesture that performed it. So the
   owner's window comes out wherever it appears.
5. And removing that switch leaves its sampled **return**, naming the window that never left the
   front — which `foldActivates` collapses one step too late to save the click beside it. The
   draft path collapses it first.

Number 5 is the ordinary case, not the exotic one, and it hid because the fixture omitted the
return and emitted the owner's app twice with nothing between — a stream the poller cannot
produce, since it emits only on change. Mutation testing found four of the five; reading found
none.

**And the recorder runs in Studio's own process**, which every other child does not.
`orchestrator.Record` writes nothing, and a draft across a process boundary would have to be
written somewhere for the parent to review it — the thing it exists not to do. So the hooks are in
the window: shutdown must end a live capture and wait for the pump, and the one predicate that
answers "is something being recorded" now answers for both recorders, because a run started under
these hooks would press its own clicks into somebody's draft with no process boundary to notice.

Also: `Begin`/`Finish` split out of `Record`, so a recorder that will not start is refused at the
button rather than discovered at Stop. And `TestEveryMethodThePageCallsIsOnTheBackend` — there is
no compiler between the page and `*Acts`, so a misspelled binding was
`undefined is not a function` inside a `.then` chain nobody watches.

See [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] and
[[ADR-250-the-recorder-runs-in-the-window]].

**Not done, and next:** the owner has not trialled this. Line A and B (outer window size, window
DPI as its own term, process image, carried through `macroir.Step` → `codegen`) and line C (the
press-moment guard on the guarded path) are the planned work after it. The trial journal does not
yet say which recorder a note came from.

## 38C — a coordinate is a claim about a place, and the place can change

Lines A, B and C. A press now records what was true at the instant somebody made it, carries it
into saved legal Marco, and a run checks it again immediately before pressing.

```
the p1 is a Point with X 842, Y 274, RelX 722, RelY 210,
  RelTo "window", WinW 1920, WinH 1080, Dpi 144, Process "arkascended.exe".
```

**What each term is for.** `RelTo` says which rectangle the offset is measured from — one value
today, recorded anyway, because without it the day a recorder measures from the client area is
the day every saved route is silently reinterpreted. `WinW`/`WinH` are the OUTER dimensions,
matching the origin they are measured from. `Dpi` is its own term because an app that handles
`WM_DPICHANGED` relayouts inside a rectangle that never moved — same size, everything somewhere
else. `Process` is the file name, not the path: the path answers "same installation", which is a
different question and makes a routine useless on another machine.

**Where each is read.** The basis, size, DPI and origin in the hook at the press, from the window
the attribution ALREADY identified — one `GetForegroundWindow` per click, because two reads are
two instants and the second could describe a window the first did not attribute. The process
image cannot be read there (it opens a handle, which is what makes Windows drop the hooks), so it
is resolved once the hooks are down, from the process id read at the press.

**Absent is not a default.** A DPI nobody could read is absent, not 96: a guard comparing against
96 would pass on one machine and fail on every other, having checked an invented number. And
nothing is written without the basis.

**Carrying the terms is the opt-in.** No flag. A point that carries them asks to be checked; a
point without them is pressed by exactly the rule it was saved with, so every route already on
the owner's machine runs as it did.

**The guard.** `Activate` hands back the window it settled on and the host keeps the handle —
"activate, then press" is two instants and the desktop belongs to somebody else in between.
Before every press that handle is re-read: still a window, still in front, origin, outer size,
DPI, and who owns it NOW, because handles are reused. Every comparison is exact; any disagreement
refuses and names the term. It does NOT fall back to the recorded coordinate — that is pressing a
remembered pixel on a desktop just shown not to match.

**The cache question, answered.** The origin snapshot stays for the legacy path and is unused by
the guarded one. It exists to survive a focus flicker the legacy path cannot detect; the guarded
path detects one directly through `Front`, and the cache is actively wrong there because it
answers about the FIRST press of a route and is kept for the rest of it.

**One resolver, three ways in.** A click, a `Move` and the recorded point a vision-free `Find`
hands back. `Move` because codegen emits `Move` then `Click "right"` — guarding only the click
would leave every press that is not left-handed unchecked. `Find` because with vision off it
short-circuits to the demonstrated point and the route then clicks what it got back; left alone,
the early return handed back a bare X,Y and that was the one bypass.

**Found while doing it:** the click point was the RELEASE coordinate while `RelX,RelY` came from
the press, so a hand that drifted a pixel left a step whose absolute fallback and
window-relative offset named different pixels — the fallback being the less accurate one.
Nothing tested it either way.

Seventeen mutations killed. Two suite guards caught misses no test of mine would have:
`routes/os.marco` had drifted from the canonical act surface it shadows, and `spectest`'s
backstage-concept gate rejected the word "evidence" in an act surface comment.

See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

**Outstanding, and separate:** the owner's Stop-button trial (recording and the window), and a
live acceptance run of guarded coordinate replay. Everything above is automated evidence only.

## 38D — a check that is written and does not run

Source review of 38C found three gaps, all of them the same shape. Not a missing comparison: a
comparison whose condition was the thing it was meant to be comparing.

**Every term was checked "if both sides know it".** Which reads as careful and means the guard's
strength depends on how much the recording happened to capture — with nothing in the route, the
log or the review saying which checks did not happen. A press with no recorded DPI passed on a
desktop redrawn at another scale. A press with no recorded program passed in a window belonging
to something else. The log line said `click: guarded` for both. The entry condition had the same
flaw at a larger size: it required a stated basis AND a usable offset, so a point claiming a
window with no offset inside it missed the guard entirely and was pressed at its recorded
coordinate.

So the terms go all of them or none, and the two sides carry it differently. The recording side
DEGRADES — a window that will not report its size, a Windows with no `GetDpiForWindow`, a press
whose offset falls outside the window it was attributed to, a program that had exited by the time
the hooks came down: each records an ordinary point and is pressed by the rule every route before
this was saved with. The replay side REFUSES — missing, malformed, out of range, an unsupported
basis, and anything the desktop will not report, each named in the sentence. Studio's review
writes `(window not measured)` beside such a press, so the difference in what the routine will do
is visible before the routine is kept.

**A handle is not an identity.** Preparation kept the handle and the app string, and two copies
of one program share an executable name — so a handle inherited by a second instance of the game
answered the image-name check correctly, reported a window of the recorded size and scale, and
was a window the route never brought to the front. The process id is now read at `Activate` and
compared before the press. It has to be read there: asked at the press it would be the identity
of whatever holds the handle by then, compared against itself.

**And the press has to land in the window.** Origin plus an offset is a point on the desktop. Same
program, same process, same size, same scale, in front, alive — and the arithmetic could still
send the cursor out of the window into whatever sits beside it, under a guard that had just
confirmed the target was exactly as demonstrated. The offset is now required to lie inside the
recorded rectangle, far edges exclusive, so a press on a maximised window's own edge is still
honoured.

Fifteen mutations killed. Seven of the fifteen are a written check that does not run, which is
the shape to look for next.

See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

**Still outstanding, and still separate:** the owner's Stop-button trial, and a live acceptance
run of guarded coordinate replay.

## 38E — a failed measurement is not permission to skip the check

The recording-side half of 38D was rejected, and rightly. Making the RECORDER give up its basis
when a measurement failed is the same hole one layer earlier: a press with no basis is an ordinary
point, and an ordinary point is pressed at its recorded coordinate having checked nothing. So a
window that would not report its size, a Windows without `GetDpiForWindow`, or a program that had
exited before the hooks came down each turned the safeguards off for a recording somebody was
about to be offered. Preserving old routines means preserving the behaviour of routes already on
disk; it never meant letting a NEW recording run unchecked because a measurement failed.

**The basis is the contract and everything else is a measurement.** `RelTo` is written for every
press a measuring recorder makes, whether or not the measurements arrive, because it says which
rectangle the press is ABOUT and that is true either way. The saved route therefore says three
things apart: no basis is LEGACY and runs as it always did; a basis with every term is GUARDED; a
basis with a term missing is INCOMPLETE and refused, naming what it lacks. Which state a press is
in does not depend on a measurement having succeeded.

`codegen` had the same branch: a point was written window-relative when it had an OFFSET, so a
press that declared the window and could not be measured in it came out byte-identical to a legacy
point. It branches on the basis now.

**Review says it before the routine is kept** — beside the step, once for the draft, and with the
one thing somebody actually needs: whether recording it again would help. A window that would not
describe itself this time is a moment that passed; a Windows that cannot report a scale is not, so
that recording cannot be made on that machine at all and saying "try again" would be the worse
mistake. Saving stays allowed — the draft is a true record of what somebody did — and is not
quiet: the Save message and the trial journal both say it will refuse.

Fourteen mutations killed. Twelve of the thirty-three mutations across 38C–E are a check that was
WRITTEN and does not run, at three different layers — the guard asking "if both sides know it",
the recorder asking "if every term arrived", the generator asking "if there is an offset".

**And a gap found while wiring the chain, not caused by it:** a context-scoped recording — Studio's
default — cannot run a guarded press at all. That scope strips the Activate and passes no
application to `codegen`, because switching nothing is its whole promise, so nothing binds a
window and every press refuses. `everywhere` is the same; only `app-anywhere` works. Pinned by a
test and left for a decision, because what PREPARATION means for a scope with no preparation step
is a change to the guarded contract.

See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

**Still outstanding:** the owner's Stop-button trial, and a live acceptance run of guarded replay —
which the context-scope gap blocks until it is decided.

## 38F — a refusal stops the routine, and a guarded recording is saved to bring its window forward

Two settled decisions, and one of them found that the obvious way to write failure propagation in
Marco silently does not work.

**A refusal stops the routine.** The next press of a sequence means nothing once the one before it
did not happen, so a guarded route is now generated as a CHAIN: the preparation and every guarded
press each in their own capability, calling the next only when its own action went through, with
the single `this is ok!` inside the success of the last one. `this is that!` adopts the outcome
verbatim, so a refusal three presses along comes back out of `Run` as the refusal it was.

The reason it is a chain rather than a flat body with early returns is worth keeping: **a terminal
return inside a non-exhaustive `when` arm does not return.** The arm fires, the ending is ignored,
execution falls through, and the caller is told ok. Measured on a three-line program. It is why
every fixture in the repository pairs its arms, and it is not changed here — Marco's own semantics
are nothing to do with this contract — but it is the reason the shape is the shape. Written flat,
the working shape is one indent level per press, which at thirty presses is unreadable; hence a
capability each.

A guarded press is also never folded into a `repeat` now, because a repeat has no way to say "stop
if that one did not happen".

**And the run's answer is data.** A refused routine still ends tidily: a root handler is
compulsory in Marco, so the failure is caught, logged, and the script finishes — and `marco do`
would have exited successfully about a routine that pressed nothing. The host keeps its own
refusals and the command fails on them. Which contract a route carries is read from the route, so
a legacy route reports exactly what it always did. On the Act path the answer already existed:
`marcoexec` maps a capability that returned failed to a runtime failure.

**A guarded recording is saved to bring its application forward.** The gap found last increment is
closed by withdrawing the choice rather than defaulting it: Studio says "Runs in Ark. Marco brings
it forward before playback." and hides the three-way select, an impossible scope is refused at Save
with the reason, and a routine already saved in that state is listed and saved again from its own
stored demonstration — re-simplified, not rewritten, with the old file removed only once the new
one is written.

Sixteen mutations killed. Two of them survived first and the TESTS were wrong, not the code: "every
route is a chain" went unnoticed because a legacy route's chain carries no `Step2` to notice, and
"the preparation does not propagate" went unnoticed because the test read only the first refusal
when the difference is one against three.

Also fixed on the way: a recorded right-click generated no point at all, so it pressed wherever the
cursor happened to be.

See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

**Still outstanding:** the owner's Stop-button trial, and a live acceptance run of guarded replay.

## 38G — a press that did not happen, by either route

Two result gaps, and both were the same mistake in different places: a list named for one kind of
failure, used as though it held every kind.

**A press the machine will not deliver.** The host's account held refusals — a press it could not
check, an application that never came forward — and not a backend that rejected the input after
the guard passed. From where a person is standing those are the same outcome: the press did not
happen. The chain stopped correctly, the root handler caught the failure correctly, the program
ended tidily, and the command reported success. `SendInput` really does get blocked: an elevated
window owning the foreground, UIPI, a game with raw input and a lower-integrity sender. The
account is now of everything the run did not do, the delivery error is kept verbatim, and
`runRoute` fails on any of the three.

The command-level case needed a seam: `runRoute` asserted the concrete `*oshost.Host`, which only
the real host satisfies, and the real host sends real input — so the only command test that could
be written was an absent application, the cheapest failure and the furthest from the press. It
now asserts an interface, and a host that takes everything but the input poses the case.

**A right-click guarded the aim and not the press.** `do OS's Move with p1` wrapped, then
`do OS's Click with "right".` with its result ignored — so a press the machine refused was
followed by the next step anyway, and the press itself carried none of the point's checks.
Rejected for this milestone rather than supported: doing it properly means one guarded operation
with a button beside the point, which is a permanent word in the language for a case no recording
has needed yet. Drags, anchored presses and repeated blocks are refused with it, for the same
reason — the chain wraps one `do` sentence and none of those is one.

**And my own gate had the same shape of hole.** "Does a press ask to be checked" and "can this
version wrap it" came out as one question, so a recording whose only press was a right-click had
no step to wrap, was therefore not a guarded route, and was generated as an ordinary block: a
press that declared the window it was made in, pressed without any of it being looked at. Found by
a test I had written for the refusal, failing for the wrong reason.

Ten mutations killed. Routes where nothing asked to be checked are untouched throughout, and each
of the three changes has the legacy half tested beside it.

See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

**Still outstanding:** the owner's Stop-button trial, and a live acceptance run of guarded replay.

## 38H — a library is a place, and saving was not the end of the flow

Two gaps between a working mechanism and something a person can use, both found by reading the
handover rather than the code: §62 of `E2E.md` called itself "Record → Stop → Review → Save →
reload → Play" and then sent the owner to a terminal. That proves a Studio-recording-to-CLI path.
It is not Studio acceptance.

**The library was wherever you were standing.** `marco`, `director` and Studio each resolved the
route store in their own words, and the words agreed — `$MARCO_ROUTES, else "routes"`. The defect
is not the duplication. It is that the sentence is RELATIVE: the answer was whichever directory
each process happened to be started from, so the product had as many libraries as it had launch
locations. A Studio double-clicked in `plugins\studio` kept its routines in
`plugins\studio\routes`; `marco do "ark graphics"` run from the repo root looked in `marco\routes`
and said there was no such routine. Both behaved exactly as written, and the walkthrough carried
the remedy as an instruction — *launch both from the repo root* — which is a developer working
around their own tooling.

So there is one resolver, `internal/marcopaths`, the answer is absolute including a relative
`$MARCO_ROUTES`, and the default is `routes` inside the store every part of Marco already shares.
The store rule went the same way for a sharper reason than tidiness: it was written out four times
as well, once with a comment saying it *"has to stay the same rule"*, and the library is now
INSIDE the store — so two binaries disagreeing about the store is two binaries with two libraries.

**Moving a default makes the old one go quiet rather than disappear**, which is the worse of the
two: the routines are still there, nothing says so, and a person reads an empty list and concludes
their work is gone. So the window reports which library it reads, and names any library-shaped
directory beside its launch location that holds routines, with the count and what to do about it.
Nothing is adopted and nothing is moved — deciding which of two libraries is somebody's is not a
decision code gets to make for them.

**And the library is now a list with a Play button.** A saved routine is selected and performed
without leaving the window: the window finds it, refuses NOW if it would not run, brings its
application forward itself — only the foreground process may, which is the same split `Play`
already draws for an Act — and spawns `marco do --play` with the route's whole identity and the
library named explicitly. Studio acquires no execution loop; the answer is the published outcome
word read through `internal/outcome`, which both ends import, so it is protocol rather than
transcript-parsing. One run slot is shared with Acts and with recording, because there is one
screen. And a routine's run is rendered and journalled as what it is — no canonical reference, no
phase, no question — because a view expecting a supervised performance reports a perfect routine
as a run that cited nothing.

Each routine is listed with the contract it was saved under, read off the saved Marco by one
reader tied by test to the six terms the press guard requires: the one that checks every press
says so, the one that cannot press anything is not offered, and the one missing a measurement
names what it lacks before anybody presses Play and watches it refuse.

**Six of forty mutations survived, and five were a weak test of mine rather than weak
code.** The child's environment was asserted to CONTAIN the library, which `os.Environ()` already
satisfied whenever the two agreed — and in a test they agree by construction, so deleting the
explicit setting passed. The module filter was tested with `os.marco`, which `routes.routeSlug`
already excludes by name everywhere, so the fixture asked nothing. Nothing tested that the window
brings the application forward, which is the behaviour the foreground lock makes load-bearing.
Nothing read what `-trial` prints, so the branch that prints a routine's run could be deleted
whole. And the refusal sentence was asserted from a two-line fixture that could not tell the first
sentence from the last.

The sixth was a real defect in the page: **two places decided whether Play was live**, and the
poll's copy overwrote the click's four hundred milliseconds after every click. So the click's half
could be made always-enabled and nothing changed — a duplicate whose later writer silently wins,
which is the arrangement that makes the other five possible. One function decides now, asked from
both. One deliberate no-op control survived.

§62 is relabelled as the engineering diagnostic it is, and §63 is the product trial: Record → Stop
→ Review → Save → select → Play, all in Studio, opened by double-clicking the binary on purpose.

See [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]].

**Still outstanding:** the owner's trial of §63. None of this has been done on a real desktop.

## 38I — a name is not an identity

Review found the selection half of 38H wrong, and it is the shape this project keeps finding: a
check that is correct about the wrong subject.

A route IS its scope and its slug. `settings` saved for two applications is two routines, and so
is `settings` saved twice for one application under two scopes — which is why `doCommand` hands
the child all three. But everything ABOVE that travelled by NAME: the page kept `r.name`, looking
a routine up returned the first row that matched, and `PlayRoutine` matched a name or a slug and
took the first of those. So clicking the second `settings` card displayed the first one's saved
Marco, the first one's sentence about its presses and the first one's Play button — and pressing
it performed the first one's route. **Handing the child the full identity was already right, and
it was happening after the wrong routine had been chosen.**

The listing had it a second time with a worse result. "Cannot press anything" was keyed by SLUG,
and a slug is shared by every scope it was ever saved under — so `<app>/context/settings` being
unable to press anything marked `<app>/focus/settings` unable to press anything too: a valid
routine disabled by an invalid namesake, with the namesake's sentence beside it explaining a scope
it is not in. And the repair card had it a third time, in the same file: `Resave` matched a name
and took the first that answered, so the button beside one row saved the other row's routine and
reported the first one's name while doing it.

So every one of them carries `<app>/<scope>/<slug>`, which is the route's place in the library and
therefore what its identity is — not the file path, which `Registry.Path` resolves to wherever a
legacy route actually sits, making it an answer about the filesystem rather than about the route.
The page keeps it through selection, preview and Play; the backend requires an exact match and
refuses a bare name rather than resolving it, because a term that identifies two routines must not
pick one of them; and both listings are keyed by it.

Eight mutations, one survivor: `Resave` was made to accept a name as well as an identity and
nothing failed, because the test handed it an identity and never asked what a name would do. One
is recorded as unfalsifiable rather than killed — the listing's sort has a tiebreak on identity
and `Registry.List` already returns a total order, so the sort never sees equal keys and removing
the tiebreak changes nothing observable. It stays because the order must not depend on an upstream
sort being total, and the comment on it says that instead of claiming a defect somebody saw.

The page's own test asked the question in the only way that can: two rows with the SAME name, one
runnable and one not, clicked in turn. The version of that harness with two different names could
not have failed.

See [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]].

**Still outstanding:** the owner's trial of E2E.md §63. None of this has been done on a real
desktop.

## 38J — a click's two edges can belong to two processes

The first mouse-driven trial of the Studio flow. The guarded half passed on a real desktop: a
routine recorded in the window, selected in the window, played from the window — Escape then a
guarded click landing on the right control; the window **moved** and it still pressed the right
places; the window **resized** and it refused, pressed nothing, and said why in the banner; Stop
mid-run ended as cancelled.

Then the owner recorded in a second application and could not save it at all:

```
drag from -740, 348 to 50, 374
```

They had performed no drag.

**The press that raises a window arrives while the previous window is still in front.** Clicking a
window is what causes the activation, so at the instant of the press the foreground is the old
one. `internal/recorder` already knows this and says so where it computes a window offset, and
refuses to compute one in that case. The EXCLUSION did not know it. Working edge by edge, it kept
half of every click that crossed the boundary — the press of the click onto Studio, whose release
was Studio's, and the release of the click back, whose press was Studio's. The one rule that
existed for this took the release of an owner's press only when that release was unattributed and
at the identical point, so neither real direction matched.

What was left was a press on Studio's window and a release in the application with no press
between them. `internal/simplify` read that as a drag, correctly by every rule it has — and a drag
cannot be saved in a routine whose presses are checked, so the whole recording was refused. The
phantom's two coordinates are the two clicks' surviving halves.

**So clicks are paired before anything is excluded, and the pair decides together.** Per button,
because buttons interleave; in order and never across a second press of the same button, because a
button cannot be released twice without being pressed again — so the release after a press is that
press's own, and reaching past a later press would invent a gesture out of two different clicks. A
click either of whose edges is the owner's is the owner's, which is right about intent too: a press
in the application whose release is in Studio IS the click that raised Studio, and a press in
Studio whose release is in the application IS the click that went back. Deliberately not "the
edges disagree, so exclude it" — a click in the game whose release lands in a window the game
itself raised has disagreeing edges and neither is Studio's, and it is a demonstrated press.

One existing case changed and it was the old rule's own encoding: a release of the owner's press
that had MOVED used to be kept. It is that press's release, because nothing pressed the button in
between, and keeping it is leaving half a click in the stream.

**Why every test passed through all of this.** Every click fixture in the tree gives both edges of
a click one process id, and a real desktop does not. The acceptance case this ADR is proudest of —
a demonstration surviving a mid-recording glance — was passing for that reason. ADR-249's own
mutation note had said the release-match clause "could have been `true` and the acceptance tests
would still have passed"; it was not `true`, it was too narrow, and the gap was the same gap.

The Studio-level regression test was run with the pairing removed before it was trusted, and it
reproduced the owner's review character for character — the same phantom, the same step list, the
same refusal sentence. Four mutations on the pairing, all killed.

See [[ADR-249-ending-a-recording-is-not-part-of-the-recording]].

**Trial status: the whole of §63 is now shown live.** Guarded replay, the library, selection by
identity, the run's own answer — and, on retest, mid-recording preservation: the owner clicked onto
Studio in the middle of a recording, clicked back, pressed another control, and the recording kept
both presses, invented no gesture, and saved.

**The retest failed once more before it passed, and that failure was a stale binary.** The first
attempt after the fix still reported the drag — from a Studio window that had been open since
before the build. Studio installs the hooks and makes the draft IN ITS OWN PROCESS: `recorder.New`
and `withoutOwner` both run there, not in the director it spawns. So a window left open across a
build runs the old recorder however fresh the binaries beside it are. It is the stale-sibling trap
one step further in — there the wrong copy was on disk, here the right copy was on disk and the
wrong one was already running, which no check of the files can see. §63 now says to close Studio
before building and to open the one that was just built.

## 38K — a key beside a press that is checked

A test-only increment, and a finding that came out of it.

The owner's first live routine was a keystroke and then a press: Escape to close a menu, then a
click on what was behind it. Nothing in the tree combined the two. Every test of the chain used
presses alone, and every test of a key used a route with nothing guarded in it, so the one thing
the chain does with an ordinary step — put it in the capability of the press that FOLLOWS it, so
it runs inside the previous press's success — was carried by a comment and by one live run.

`internal/codegen/keyandpress_test.go` holds it as behaviour rather than as text: the real
generator, the real runtime, and an OS host that writes down what it was asked to do in order and
refuses one nominated thing. A key in order between two presses; nothing after a press that
refused; a key before the first press waiting for the application to come forward, so Escape
cannot be typed into whatever was in front; and the tail, which has no further press to carry it.

**The mutation gate found a gap in the product, not in the tests.** A key promoted to a link of
its own passed every test in the file, so the question was what production does when the KEY is
the thing that cannot be performed. It carries on: the press after it goes ahead and the routine
reports that it performed. Nothing notices, because the guard reads the window and the window is
the same window — while the menu the Escape was meant to close is still open. That is the shape
the owner's own routine has. It is recorded as a known gap in
[[ADR-251-a-press-checks-the-window-it-was-recorded-in]] and pinned by
a test which recorded what it did then and was meant to be the one that failed when somebody
decided it. Making a key propagate is a change to that contract — keys would go in
`Unpropagatable` and every key would become a link — so it was scoped separately and not taken
here.

**It was decided immediately, and §38L below is the correction.** That test is gone, replaced by
the cases that assert a keystroke stops everything after it; this section is kept as the record of
how the gap was found, which was a mutation and not a live run.

One mutation SURVIVED and is recorded as unfalsifiable rather than closed: every key flushed into
the previous link's success instead of the next link's preamble. Those two places are the same
instant, so a test telling them apart would hold a shape with no consequence. The first version of
this file had exactly that test, with a comment claiming the refusing path would distinguish them.
It does not, and the test was removed rather than kept as a passing assertion about nothing.

## 38L — a keystroke that did not happen

The gap §38K found, closed as a bounded correction: the accepted Escape → guarded-click sequence
now stops if Escape cannot be delivered. Two lines of production code and a fixture's worth of
tests at three levels.

**A keystroke is a link.** Not because anything checks it — nothing measures a window for a key —
but because it can fail to be DELIVERED, and what follows it then acts on a screen the key never
changed. `propagating` now returns a guard for a key as well as for a press and the preparation,
so each one gets its own capability and calls the next only inside its own success.

**A repeated key is each of its presses.** `simplify` folds a run of the same key into one step
with a count, before anything knows what the recording is for, and the generator wrote that as a
`repeat` block — which cannot stop a routine, since its body runs again after the thing that did
not happen. `keyByKey` expands the count back into one link per keystroke, for a chain only; a
legacy route keeps its `repeat`. Refusing it at generation instead would have stopped a recording
with a double-tap in it from saving at all, which nobody asked for.

**And the failure is kept in the account.** `doKey` returned `ok(err)`: the status said failed and
nothing was recorded, so the chain had no reason to stop and `Unperformed` — the list `marco do`
turns into a failed command and Studio reads back as the reason — was empty. It is now wrapped in
`delivered`, like every other input send, and says *the escape key did not happen: <what the
machine said>*.

**Three levels, because no one of them establishes it.** `internal/codegen` holds what the
generator writes and what the runtime does with it, through a host that writes down what it was
asked to do and refuses one nominated thing: a key that did not happen stops everything after it,
whether it sits before the first press, between two, or after the last — with controls around it
the machine takes, so what stopped the routine is the keystroke and not the arrangement.
`internal/oshost` takes the owner's own shape through the production recorder, simplifier,
generator, registry, compiler and the real `Host` with the backend rejecting the keystroke, and
asserts the account keeps the machine's words. `cmd/marco` holds the command's answer, which is
the wire Studio reads, and its legacy counterpart asserts the press after the key was still
attempted — preserved behaviour, not merely the absence of an error.

The gate is what separates those levels: not keeping the failure in the account is invisible to the
generator's tests and fatal to the host's; the command no longer reading the account is invisible
to both and fatal only at the command. Seven mutations, all killed, plus a no-op control.

**Still not covered, and said so in the ADR**: typed text and held keys. `Type` is one step that
can emit several sentences, so it is not one `do` to wrap, and what a partly typed line means is
its own question; a hold is a pair, and making it propagate means deciding what the release owes a
press that did not happen. Their failures stop nothing and are not in the account either, so a
guarded routine whose typing was dropped carries on and reports that it performed — which is
exactly what a keystroke did until this increment.

The live trial stands for what it showed. It does not establish this: staging it means making the
machine refuse one keystroke and take everything else.

**And it reaches a routine when that routine is written.** The rule is the generator's, so a
routine already on disk keeps the shape it was generated with — the owner's own first routine among
them, which will go on pressing past a keystroke that did not arrive until it is recorded again or
saved again. Nothing rewrites saved Marco, and nothing should. The way out exists already (the
demonstration is stored beside the routine and Studio saves it again from that), but Studio offers
it only for a routine that cannot press anything at all, so this case is neither offered nor
announced. Recorded as a gap in the ADR.

The follow-up has a decided shape and is separately scoped: a routine should carry a
GENERATION-CONTRACT version rather than the build that made it, because an older build alone does
not establish that a routine needs updating. Studio would identify an affected contract, explain
the difference, and offer an explicit re-save from the stored demonstration, with the review shown
first. Nothing rewritten automatically.

## 38M — what is left of the coordinate MVP (assessment, not work)

A design-and-estimation pass, asked for because the brief that prompted it commissioned a bridge
that already exists: Studio reuses the widget recorder through the orchestrator draft path, saves
legal Marco through the production generator, and plays saved coordinate routines through
`marco do`. [[Coordinate-MVP-assessment-2026-10-24]] is the whole of it. Nothing was implemented.

The three findings worth carrying in the narrative:

**The missing bridge is an account, not a recorder.** `Inspect` and `Report` already exist in
Studio and work; they are keyed on a run reference that only the supervised performer produces, so
they are unreachable for exactly the routines this MVP is about. `plugins/studio/history.go`'s own
reader already carries a `routine` field that nothing writes.

And less of the machinery is missing than the first pass said. `internal/director/runref` is
already importable and already holds the citation scheme and the word `started`; `reserveRun`
creates the record with `O_CREATE|O_EXCL` and writes a started record at once, and `saveRun`
finalises with no `O_CREATE`, so it cannot touch another attempt. Exclusive reservation, an
interruption leaving an unfinished attempt, and a safe finalisation are already solved there. The
work is to make `marco do` — the EXECUTOR — reserve, cite and finalise its own record, with only
the shared machinery extracted; Director's record implementation stays where it is. That is the
spine at 9 to 15 hours, with everything else 8 to 12 more.

**And the two paths do not share a vocabulary.** `runref` says `verified`, `unconfirmed`,
`contradicted`; `internal/outcome` says `performed`, `unverified`, `unavailable`, `clarify`. Three
words overlap. Nothing may convert `performed` into `verified` — a routine presses and looks at
nothing afterwards — so the record carries an explicit KIND, each kind keeps its own words, and a
routine's record OMITS the verification counts rather than writing three zeroes, because three
zeroes are a claim and omission is the truth.

**`MARCO_CV`/`MARCO_ANCHORS` are read into package variables at process start**, so Studio
inherits whatever launched it for the life of the window — and with anchors on, a press whose
cropped region is distinctive carries a template, which makes the whole guarded recording
unsaveable at Save. The same demonstration therefore saves or is refused depending on an inherited
setting and the pixels under the pointer. That is the gap between an engineering switch and an
explicit saved capability.

**A chord needs no new representation and already propagates**, since the keystroke rule is on
`StepKey` and does not read the string. What it has is two defects. The modifier loop returns on
the first rejected `sendKeyDown` without releasing the modifiers already pressed, and nothing
tracks them, so a partly delivered `ctrl+shift+esc` can leave Ctrl held with the end-of-route
unwind unable to help. And cancellation does not stop the tap: the settling `sleepCtx` returns the
instant the context is cancelled and execution falls straight into the final key-down, so input
goes out after the person asked for it to stop. The first version of the assessment called that
path correct, which was wrong — cleanup is attempted there, which is not the same claim. Forced
termination cannot be cleaned up at all and is not pretended otherwise.

Also recorded there, because it is where existing data can be lost: `Registry.Save` is a plain
`os.WriteFile`, so re-recording a routine under a name silently replaces the one saved under it —
and re-recording is the action a repeatable MVP encourages.

## 38N — saving does not replace a routine unless somebody says so

The first piece of the coordinate MVP to be authorized, taken before the run record because it is
the only remaining item that protects something you cannot get back. Capped at the safeguard: no
version history, no automatic renaming, nothing of generation-contract versioning.

`Registry.Save` is a plain file write, so recording under a name already in the library replaced
what was there — the source, and a moment later the `.rec.json` beside it, which is the copy a
routine is ever saved again from. The person most exposed is the one re-recording all afternoon to
compare attempts, which makes the likeliest casualty the routine they were comparing against.

**The policy is at the Studio boundary and nowhere lower.** The orchestrator, the overlay and
`marco learn` overwrite on purpose; teaching a name again is how you correct one there. The
mutation that pushes a refusal down into `Deps.Save` kills the test holding their behaviour AND
the one holding Studio's own repair path, which is a deliberate replacement too.

**Three ways in, one place that writes** (`plugins/studio/replace.go`). Ordinary Save refuses and
names what is there and when it was kept. `SaveDraftAs` keeps both and refuses a collision of its
own. `ReplaceDraft` carries the identity somebody agreed to replace. `SaveDraft` in recording.go is
now a delegation to the one writer.

**The identity is the generator's own.** `orchestrator.Deps.Target` is the route a draft would be
saved as; `Save` calls it and so does the collision check. The mutation that drops the scope from
that derivation kills five tests — a collision looked for in the wrong place is no protection.

**And the save asks again.** `Collision` is for the page, so it can offer the choice; it is not
what protects the library. Between the question and the answer somebody can save that name from a
terminal. Replace also checks that the identity it would write is the one it was told to replace.

**The ordinary button cannot reach a replacement**, and that is held in a browser
(`TestOrdinarySaveNeverReplacesAndTheChoiceIsPressed`): a page that looked first and replaced for
somebody would pass every backend test while doing the one thing they exist to prevent.

Seven mutations, all killed, plus a no-op control. R1–R3 remain paused.

## 38O — one claim per attempt, and two vocabularies in one directory

R1 of the first milestone, and an extraction with no behaviour change: the discipline of recording
an attempt is now `internal/runs`, and the record in it is still each caller's own.

**What is shared is narrow and all of it is about a namespace.** The place
(`marcopaths.Runs`, `marcopaths.RunsDir` — one spelling for a directory two binaries write and a
third reads); the claim on a reference, made exclusively before anything is sent; the provisional
record left in that claim at once, so an attempt that dies retrieves as an attempt that did not
finish rather than retrieving nothing; and the finalisation with no `O_CREATE`, so a record can
only be written where a claim already is. `runs.Reserve` is handed the record it leaves behind as
a function of the id it takes, and never reads a field of one.

**What is NOT shared is the record, because the vocabularies do not convert.** Evidence
(`verified`, `contradicted`, `unconfirmed`) against delivery (`performed`, `unverified`,
`unavailable`, `clarify`). Nothing turns `performed` into `verified`; `unverified` is not
`unconfirmed`. Absent verification evidence stays absent rather than becoming three zeroes.
[[ADR-253-the-claim-is-shared-and-the-record-is-not]]

**`cmd/director`'s `RunRecord` did not move.** `reserveRun`, `runPath` and `saveRun` keep their
names and became callers; the supervised record keeps its own fields, its own seven words and its
own `buildid` stamp. The whole 150-second director suite passing unchanged is the evidence that
the extraction changed nothing about that command.

**The gate found a real gap rather than confirming the work.** A mutation deriving the place from
`$MARCO_HOME` alone passed every test, because every test sets it — and Studio sets it, while
`marco do` typed into a terminal does not. `filepath.Join("", "runs")` is a RELATIVE directory,
which is the defect `marcopaths` exists because of, about to repeat itself one layer up with a
bug report's reference pointing at nothing. `TestTheStoresPlaceIsAnswerableWithNothingInTheEnvironment`
exists because of that measurement. A second mutation, deriving the place without `marcopaths` at
all, did not compile — recorded as redone, not as a kill.

Seven mutations killed, one no-op control surviving. R2 and R3 next.

## 38P — a routine's run is something you can cite

R2 of the first milestone. `marco do` claims its own reference before the first input, prints the
citation, and finalises a record of its own kind.

**The executor owns the reference.** Not the surface that pressed Play: Studio spawns `marco do`
and so does the overlay, so a reference minted by whichever one asked would differ by who asked,
and a run started from a terminal would have none. Claimed after the authority door — a run the
door declined pressed nothing — and before the first input, which is held by MEASUREMENT: a host
counts the records in the store on the first action it is asked for and finds its own already
there. `TestTheReferenceIsClaimedBeforeTheFirstInput`, and the mutation that moves the claim to
the end is killed by that test and by nothing else.

**The outcome stays off the citation line.** `runref.Parse` strips a parenthesised word only when
it is one of the supervised seven, so `(performed)` would be left inside the record's path.
Teaching that parser a routine's words would be the conversion ADR-253 forbids wearing a parser's
clothes. `[result] <word>` already carries it.

**The digest is of the saved file, not of the substituted text.** A digest of the substituted text
cites source that is nowhere on disk, and the substituted text holds whatever the person passed on
the command line — which would make a durable file out of their arguments.
`TestTheDigestIsOfTheFileAndNotOfWhatWasTypedIntoIt` checks both halves.

**The contract is read once, by the reader that already existed.** `orchestrator.ContractOf` knows
all three of ADR-251's contracts and has its required terms held against the guard by test. The
first version of this read the source itself with `strings.Contains(src, "RelTo \"")` — a second,
weaker answer that cannot see `incomplete` at all. And the rule is `legacy` LEAVES, not `anything
but guarded` leaves: an incomplete route states a window and refuses, so the second form would let
that refusal pass as success. Nothing covered an incomplete route at the command, so that mutation
survived the first gate — `TestAnIncompleteRoutinesRefusalStillFailsTheCommand` closes it.

**A correction to the assessment.** It says `oshost` "already wraps every send in `delivered`". It
does not — four call sites against eleven sending actions — so a count taken there read zero for a
routine that typed its password or dragged something. The count is now taken in `Invoke` against a
closed classification, with a test that reads the switch and fails for an action nobody
classified. ACTIONS asked for, not primitives delivered: one smoothed Move is one action, a press
refused by its guard still asked, and the count is ABSENT rather than zero when no host can say.

Nine record mutations plus two on the count and two on Studio's reader, all killed; the record
version bumped as a surviving no-op control; and one structural choice recorded as unfalsifiable
rather than claimed as a kill. ADR-253 §R2, Demonstrations, HANDOFF 38P.

**And writing records from a second command leaked into the owner's store.** `cmd/director` has
had a `TestMain` isolating `$MARCO_HOME` since its own records leaked out of a suite, and its
comment ends *the next thing to leak that way might not be a record.* It was a record, from
`cmd/marco`: several tests there drive `runInvocation` with a registry in a `t.TempDir()` and no
`MARCO_HOME`, so `go test ./cmd/marco/` wrote three records into the directory the owner keeps
their work in, every run. Six were sitting in it by the time it was looked at, and were removed.
Found by looking rather than by a failing test — nothing in the suite was red.

`cmd/marco` now has the same isolation, asserted the same way, and the comparison both guards use
is one function (`marcopaths.SameStore`) because three things ask "is this the real store" and one
of them is destructive. The weakest implementation was already in `marcopaths`: `Clean` and a
case-fold with no `Abs`.

**One consequence stated and parked.** Every `marco do` of a non-learned route writes a record now,
including an overlay hotkey — a key held down in a game is one record per press. Nothing is at
risk and `Forget` removes them, but the directory is bounded only by use and `History` reads all of
it to show twenty-five. Retention is a product decision and is not this increment's to invent.

## 38Q — a routine you can report on

R3, and the milestone's last increment. Writing the record was half of it; the other half is what
somebody is SHOWN, and that half was wrong in a way no test noticed because the reader predates
there being two kinds.

**Studio's history read every record in the supervised vocabulary.** `saidOf` and `nextOf` are the
seven words of EVIDENCE, so a routine's `performed` fell to their default and a run was described
— and offered a remedy — in the language of something it never did: *demonstrate it again, resting
after the click so the result is read.* Not a conversion into `verified`, the same mistake one step
quieter. The record's `kind` is now asked and nowhere else is; guessing from which fields are
filled in would be this window deciding what kind of run somebody had.

**`nextOfRoutine`** offers the remedies a routine has — put the window back, record it again, read
what it said — and gives `performed` NOTHING, because there is nothing to do about a run where
every press happened.

**The kind word is one constant.** `runs.KindRoutine`, imported by `cmd/marco` and by Studio. A
misspelling on either side is not a compile error, it is a record silently read in the wrong
vocabulary.

**And a row no longer says `0 checked` about a routine.** Not a count — the claim that it checked
and found nothing, sitting beside an Act that really did check two things. Held in a BROWSER, both
rows, both directions, because it is the one place this rule could break on screen while every
backend test passed.

**Report works from a routine's reference** and carries its SOURCE as it stands, found by the
identity in the record rather than by what the window has selected. And it says when that source
is **no longer the source that ran**, by comparing the record's digest to the file's — the one
thing a routine's problem report can say that nothing else can, and the likely case rather than
the unlikely one, because re-recording is what somebody does between a failure and reporting it.

Nine mutations killed and a no-op control surviving; one of the kills is a test that exists only
because the gate found its absence — an unfinished ROUTINE record, which is acceptance check 9.
One rewording recorded as a kill rather than claimed as a control.

## 38R — changes requested on R1–R3, and what they had in common

R1–R3 were reviewed and three gaps came back. One mistake in three places: a value that was
ABSENT was read as a value that was zero, or false, or somebody else's — and in every one of them
the result on screen or in a file was not a blank but a confident false claim, offered to the
person at the moment they had least ability to check it.

**A report attached whichever transcript the window was holding.** `Report` copied `a.transcript`
without asking which run it belonged to, and the transcript is this window's memory of ONE child,
cleared and refilled every time anything runs. Play B, report A, and the file carried A's
reference and A's record with B's output underneath, headed *WHAT IT SAID AS IT RAN*. A wrong
transcript is worse than none — it is evidence about the wrong run, in the one file somebody sends
precisely because they cannot explain what happened. The lines have an owner now, read under the
same lock as the lines themselves, and a run that does not own them is told the section is
unavailable rather than being handed somebody else's. The ACT section had the identical shape,
found by whichever Act was open, and is found by the record now too: the routine half had been
corrected one increment earlier and the fix did not take its own sibling with it, which is a shape
this repository keeps catching.

**A routine's row said `input disabled`.** `live` belongs to the supervised record; a routine's
states no liveness, because `marco do` has no *write it down instead* mode to distinguish. Read
into a plain `bool` it was `false`, and the page printed those words beside a run that had just
typed somebody's password. The count had the mirror defect: `input_actions` is absent in the
provisional record a killed `marco.exe` leaves, and **that attempt may already have sent input**,
so an `int` rendered `0 input actions asked for` — the three-zeroes claim, on screen, about the run
a person is most likely to be asking somebody else about. Both are pointers through the record,
`PastRun`, the JSON and the page; a routine says which KIND of run it was where the badge would be;
and an Act that genuinely had input disabled still says so, which is the direction that proves the
claim was made absent rather than false.

**And an interrupted record identified the hour and not the program.** `build`, `source_digest` and
`contract` were derived from what the performance returned and written at the end — the half of a
run that may never happen. The saved source is read ONCE now, into a `snapshot` taken before the
reference is claimed, and the provisional record carries all three. The same snapshot executes, so
no second read can make the record cite source the run never performed. Held by saving the routine
OVER ITSELF from inside the first host action and requiring the two writes to agree.

**And the director suite's intermittent failure has a cause now.** `cmd/director`'s
`TestAnUnsettledNameIsNotWrittenDownByWatching` fails under full-suite load and passes in
isolation, and this corrects what §38Q said about it. It is **not** production deciding a tie by map order: `observe.settledPlaceName`
refuses a tie and returns the same answer for every map iteration order, which was checked by hand
over the cases. It is the TEST. Its wait loop breaks the instant both words have been SEEN, then
asserts about what the ambient sampler decides from a session that is still running and still
counting — and the precondition is measured on a shadow snapshot that is not the tally the decision
reads. Inserting a 600ms sleep in that gap fails it **6 of 6 times**, which is the cause
established rather than inferred: under load the scheduler supplies the same gap. The fix is the
test's — stop the session before sampling, or wait on the tally the rule actually reads — and it is
not applied here, because R1–R3 is what was under review. **See §38V: fixing it found that the gap
was the smaller half of the story.**

Eight mutations killed and a no-op control surviving. Two are worth the record: one was
**unfalsifiable** on its first attempt (it guessed a route path by hand, the read failed and
nothing changed) and was redone through the registry with a panic on a failed read; and the page's
absent count renders as the literal word `undefined` rather than as `0`, so the negative assertion
alone does not catch it — the test asserts what the row MUST say, not only what it must not.

## 38S — what a recording captures is stated, and not inherited from a shell

The first piece of the next authorized slice: **explicit coordinate recording**. Readiness and
Record again are the other two and are separate commits.

Nothing here was coded wrong. `captureAnchors` and `cvKitchenSink` are package variables read ONCE,
from `$MARCO_CV` / `$MARCO_ANCHORS`, when a process starts — the right shape for a command you set
a switch and run, and the wrong one for a window that stays open all afternoon holding the
environment of whichever shell launched it. And the setting is not cosmetic. With anchors inherited
on, a click whose cropped region happened to be *distinctive* carried a template; a press found by
matching a picture cannot be stopped at, so `codegen.Unpropagatable` refuses the WHOLE guarded
recording at Save. **Record the same menu twice and you can get two different answers, with nothing
on screen to explain either** — a variable nobody in the window can see, and the pixels that
happened to be under the pointer.

The quieter half was worse. `learnOptions` caps every recorded gap to 50ms when CV is on, because
the anchor poll was supposed to do the real waiting. So a Studio launched with `$MARCO_CV=on`
against a flat menu captured **no anchor to wait on and took the pauses out as well** — a routine
with its timings removed and nothing put in their place, which presses a menu before it has drawn
and looks perfectly correct in review.

So `recorder.Mode` is a statement about ONE recording, handed to the recorder BEFORE `Start`
because the hooks read it and the first press can already have been seen. `Inherited` is the zero
value and is every caller there was. `Coordinates` is the coordinate and the clock and nothing
else, and it beats the environment at every point that would capture, crop, colour-sample or listen
for the anchor key — all of which ask one function, `anchoring()`. `Start` also clears any
unconsumed arm, because the anchor key's arm is a one-shot that outlives the recording it was
tapped in. Studio states it once, to the thing that acts on it, and the word travels on the draft,
into the saved source as `// Recorded as: coordinates.`, and from there into the run record through
`orchestrator.ModeOf` off the same single read as the digest and the contract.

**Absence survives all of it**, which is §38R's lesson one layer up: no line when nothing was
stated, `unstated` read back from a file with no line, `unstated` for a word this build does not
recognise rather than echoing it into a record as something verified, and a blank rendered as a
blank rather than as `undefined` or as the likeliest answer. Every routine already in a library
says nothing, and what those captured depended on the environment of a process that has long since
exited.

Nine mutations: eight killed, one recorded as **unfalsifiable** (the anchor-key gate in
`keyboardProc` is redundant with the one in `recordClickDown`, so removing it changes no
behaviour), and a no-op control surviving. **Two of them changed the code rather than the tests**,
which is the whole point of running them:

- The arm gate had been written as `anchoring() && anchorArmed.Load()`. `anchoring()` is the
  conjunction, so that made an armed press do nothing whenever the global switch was off — taking
  the tap-then-click anchor away from EVERY caller instead of from this recording. It is
  `!plainCoordinates.Load()` now, and the test was rebuilt around a **right-click**, which is the
  one press the arm alone decides. The first version of that test evaluated the conjunction in its
  own body and was therefore unfalsifiable; it drives `recordClickDown` now.
- Studio stated the mode at two sites — once to the backend and once to the capture the page reads.
  Deleting the backend's left the window still promising coordinates over a recording that had gone
  back to inheriting them. The page's word is a COPY of what the recorder was told.

See [[ADR-254-what-a-recording-captures-is-stated]]. Readiness and Record again are not started.

## 38T — readiness is what has been checked, and it says what it has not

The second of the three authorized Studio pieces. Record again is not started.

Five ways of being unable to record or play, and every one of them used to be found by hitting it:
the recorder refusing a platform at **Stop**, after a whole demonstration; a missing `marco.exe` at
**Play**, over a game somebody had just arranged; a library that cannot be written at **Save**,
which is the one instant a recording exists nowhere else; a window that cannot name its own build,
found by comparing file timestamps.

The fifth was never found at all. `marcoBin` and `directorBin` fall back to the bare name when the
sibling is missing, `exec.Command` resolves it on PATH, and playback runs whichever binary that
finds — a different build of a different age, performing somebody's routine, successfully. It is
the trap `internal/buildid` exists because of, and it cost a day of recordings.

So `Acts.Ready` answers each by DOING something: the recorder is a platform fact
(`recorder.Available`, which explicitly does not claim the hooks will install — that can only be
known by installing them and taking the whole desktop's keyboard); each executable is an `os.Stat`
beside this binary and then the `exec.LookPath` the spawn itself performs, giving THREE answers
rather than two, of which "on PATH but not beside" is the silent one; the library is a file created
and removed in it, because a directory that exists is not a directory that can be written to; and
the build is this binary's stamp or the honest word `unknown`.

Each problem carries what is wrong, what it **stops**, and what to **do**, because every one of
them has exactly one answer. And `Limits` is UNCONDITIONAL — the hooks installing, the application
running, a press landing, a screen being looked at are all named as things NOT established, on a
clean machine as much as a broken one, because a clean machine is when a readiness panel is
believed.

Asked again rather than once, because the binaries and the disk change while the window is open.
On its own slow cadence and never during a recording or a run, because the library check writes and
a recording is watching the keyboard in this same process.

Seven mutations, all killed, plus a surviving control. **One changed the tests rather than the
code**: replacing the library write-probe with `os.Stat` SURVIVED, because the only library test
then in place was caught by `MkdirAll` failing and never exercised the write. A second test was
written — a real directory with an `icacls` deny, so `MkdirAll` succeeds and only putting a file in
it finds out — and the mutation is killed by that.

**And the browser test found a defect no mutation would have.** Starting the readiness poll inside
the bindings callback threw a `ReferenceError`: that callback runs SYNCHRONOUSLY part-way down a
script whose later `let` declarations are still in their temporal dead zone, and the poll reads
`runActive` first thing. It took the WHOLE bootstrap with it — every other poll included — and
silently, because a `-H windowsgui` binary has no console to print it to. The other five polls
survive that ordering only because everything they touch sits inside a `.then`. The readiness poll
starts on the next tick now, with the reason written beside it.

No button was disabled: the ask was to show actionable problems, and a control greyed out with its
explanation somewhere else is worse than one that refuses and says why. Nothing runs a child to ask
its build either — `marco.exe` and `director.exe` have no version flag, and a panel polled all
session must not start a process that performs real input, so the panel names their LOCATIONS and a
run record names the `marco` that performed it.

See [[ADR-255-readiness-is-what-has-been-checked]].

## 38U — recording a routine again, and the offer that replaces nothing

The third and last of the authorized Studio pieces.

Re-recording is the ordinary activity of getting a routine right, and until now it meant reading
the name off the card, typing it back into the box at the top of the page exactly, remembering
which application the first one was saved for, and picking the same scope. Get the name wrong by a
space and you have two routines instead of a new version of one.

So the routine offers it. `RecordAgain` takes the route's whole IDENTITY and not its name —
`settings` can be saved for two applications under two scopes, and a window holding the word would
start a recording of whichever it found first and then offer to replace THAT — and it looks the
identity up again at the moment the button is pressed, because a card can be on screen for a
routine somebody has just deleted from a terminal. Both entrances go through one
`beginRecording`, so the five refusals live in one place: a second door that skipped them would let
a recording start while a run was in flight, and a runner pressing things while the hooks watch
records Marco's own presses as the person's.

**`again` is a label and not a licence, and that is the whole design.** There is no new save path.
Save goes through the same `keep` every recording goes through: it asks what is at the target,
refuses a collision, and offers *Save as* or *Replace that routine by name*. §38N exists because
`Registry.Save` is a plain file write, so keeping a recording under a name already in the library
replaced the source and the `.rec.json` beside it with nothing asked and nothing said — and the
likeliest casualty was the routine somebody was comparing against. A Record-again button that then
replaced on an ordinary Save would have reintroduced exactly that, through a door marked
convenience, and the person pressing it would have had more reason than anybody to expect it.

What the label buys is three notices: which routine this is a second recording of, said during the
recording as well as at review because the expectation forms before Save is in front of anybody;
the routine's own scope preselected, so a replacement is not silently rescoped; and a warning when
the demonstration was made in a DIFFERENT application. That last is the one case where Save
succeeds and does the wrong thing — the identity a save writes is decided by the application that
was OBSERVED, so a re-recording made elsewhere is a different route, with no collision to refuse
and a second routine quietly kept beside the one somebody set out to replace.

Eight mutations, all killed, plus a surviving control. **One changed the tests** — the third time
in these three commits. Showing the *recorded elsewhere* warning unconditionally SURVIVED, because
a claim-level assertion cannot tell an empty shown box from an absent one; an empty box is
invisible today only because `.no` and `.muted` are colour-only classes, which is a property of
this page's CSS and not of its logic. The rendered tests assert the box's presence as well as its
text now, with that reason written beside the helper.

**And review found a ninth, in the lookup.** The loop matched the identity against one
`a.Routines()` and then took `a.Routines()[i]` — a SECOND reading of the library, indexed by a
position established in the first. They agree only while nothing changes, and a library changes: a
routine that goes from the front of the listing moves everything behind it, so the index that had
identified the right route identifies a namesake one position along, or runs off the end of a
shorter slice and panics. The fix keeps the match rather than its index, which is what
`PlayRoutine` already does for the same reason. None of the eight mutations could have found it:
every one of them changed what the code says, and this was a disagreement between two readings of
something outside the process. `TestRecordAgainKeepsTheRoutineItMatchedWhileTheLibraryChanges`
removes and restores the first routine in the listing while asking for the ones behind it, and it
fails on the old form within two passes.

See [[ADR-256-recording-a-routine-again]]. All three authorized pieces are in. R4's explicit
coordinate mode is the mode STATEMENT only: there is no second mode to choose between and no
control that offers one, because the other mode is the anchor stack and that is the open question
`cvOff()` names, which needs an experiment rather than a dropdown.

## 38V — the director flake, and a fixture that named one branch and exercised another

`cmd/director`'s `TestAnUnsettledNameIsNotWrittenDownByWatching` has failed intermittently under
full-suite load for three sessions. §38R established the gap it fails in: the precondition loop
broke the instant both words had been SEEN, and the assertion was then made about what the ambient
sampler decides from a session still running and still counting. That was right as far as it went.

Fixing it found the rest. **The fixture could not produce the tie it was named for.** It said
`Mouse` and `Trackpad` on alternating frames and called that two words said equally often; the
tally never tied. Printing the shadow shows why: one screen's sightings split across two screen
states — `state_1` holding `Mouse:4, Trackpad:1` and `state_2` holding `Trackpad:2`, with the
eighth sighting dropped as a transition frame — and `settledPlaceName` settles a lead of ONE as
soon as the leader reaches `StatePromotionCount`, which is two. So the test was green only while
every count was still below the threshold: **it was passing on the too-thin branch while claiming
to hold the tie**, and it failed whenever the session had got far enough for a lead to clear it.
Waiting for a tie instead of for both words — the obvious fix — hangs for the full sixty-second
deadline and then reports `map[Mouse:4 Trackpad:3]`, which is the finding stated as a failure.

So the fixture says what it means. Each word is said ONCE, with unnamed frames between them and
either side, and then the names stop: `creditTerms` credits only a name that is there and
`sameSampler` repeats its last frame for as long as the session runs, so **no name can ever reach
the promotion threshold in any state, whenever the sample is taken and however the sightings
land** — losing one leaves the other, still unsettled. There is no gap left to race. The
precondition QUOTES the rule through `observe.SettledPlaceNameFor` rather than counting sightings
itself, because a precondition that re-derived the rule would be the test agreeing with itself, and
it now also requires that a word was read at all — a name not written down is not evidence of
anything if perception read nothing. The assertion asks the whole store rather than the one subject
the session happened to establish first, since an unsettled word landing on a second state would be
written against a second subject the old line could not see.

The tie is held where it can be held exactly: `observe.TestATiedNameIsLeftUnresolved`, over a
state built by hand, asked forty times so no map order can answer it.

Mutated to confirm it still holds what it is for: `PlaceNamesToRecord` reading the tally instead of
what settled out of it is **killed** 2 of 2, naming the word and the Place it was written for.
Four consecutive runs green, about 1.8s each. See [[ADR-113-learning-is-inside-watching]].

**The two screen states are PARKED by the Architect.** One scripted screen produced `state_1` and
`state_2` with a sighting dropped between them, which may be ordinary flapping before a state
settles or may not; it is written down here because it is how the fixture was understood, and it is
explicitly not a reason to reopen perception work or to hold up the coordinate MVP. And this closes
the one identified test defect — it is not a claim that the suite cannot flake.

## 38W — a desktop harness, and the one thing it prints on every green run

Studio's flow was testable in two places and provable in neither. The browser tests render the
real page with the Go bindings STUBBED; the backend tests drive the real backend with the recorder
and the desktop REPLACED. Between them sat the whole product path — page, binding, backend,
library, generated Marco, child executor, native input, window guard, run record — and the only
thing that had ever exercised it was the owner working through E2E.md by hand, eighteen bullets at
a time, after every change.

`plugins/e2e` is that path, run by one command. Its own module, so `go test ./...` at the root
never sees it: a harness that performs real input into a window on the real screen should not be
something anybody can start by accident, and an environment gate is a thing to remember where a
separate module is a fact about the tree.

**The investigation came first, and two of its three findings changed the brief.** The controlled
application already existed — `plugins/fixture`, fixed title, fixed layout, fixed control names,
an oracle Marco may never read — so it was extended rather than replaced (§38V's commit before
this one: an input witness, keyboard observation, and a geometry/DPI channel). Studio's controls
turned out to be pressable through the OS: `go-webview2` deliberately clears
`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` so no debug port can be opened, but Chromium publishes the
page to UI Automation, `plugins/uia` already reads that, and **AutomationId is the DOM id** — so
`rail-scenes` and `routinePlayBtn` are addressable, `Invoke` runs the page's own handler, and a
snapshot is the operating system's account of the window rather than the backend being asked what
it would have rendered.

**And the third finding is a wall.** Both recorder hooks test the injected flag and discard what
they find, by design, because attributing Marco's own presses to a person would let a replay be
learned as a demonstration. There is no switch and there must not be one, so no unattended run can
demonstrate anything. Three options went to the Architect — operator-assisted recording, a USB HID
device, or a gated door in the recorder — and the answer was none of them: build the unattended
harness, put native recording outside its first coverage boundary, and SAY SO. A VM with
host-controlled virtual input is parked as a later investigation, to be measured before it is
claimed.

So routines are SEEDED through the production generator and labelled seeded. `Deps.DraftOf` is new
and shared with `Resave`: events, `simplify`, `codegen`, `routes.Registry` — the same four steps
through the same functions, so a seeded routine is the routine the product would have written from
the same demonstration. The events are measured rather than invented: the press point comes from
the fixture's own account of where its control is, and the window on the press from
`winctx.WindowShape`, the identical call the recorder makes at the identical moment.

**The spine runs in 3.6 seconds, unattended.** A bundle built fresh into its own directory, an
isolated store, the fixture up, a guarded routine seeded at a measured coordinate, the real Studio
launched and its page reporting that it rendered, `rail-scenes` pressed, the card chosen by
identity, Play pressed — and then the two accounts: the fixture received `left_down on "Save"` at
45,14 into the control, and Marco wrote `outcome="performed" mode=coordinates contract=guarded
input_actions=1 build=<rev>`. Both agree, which is the only ending this calls passed.

Every report carries three lines, and the third is a limit rather than a score: passed desktop
scenarios, passed recording-integration scenarios, and `physical recording capture: NOT
EXERCISED`. Printed on a green run in the same words, because a limit mentioned only beside a
failure is a limit nobody reads — and a single total over the two layers would read as "the
recording path works on Windows", which is the one thing nothing here establishes.

Two smaller things came with it. The routine cards were bare `div`s with an `onclick` — no name,
no role, no keyboard path, nothing for assistive technology to announce or activate, so a person
who cannot use a mouse could record a routine and then be unable to select it; they are controls
now, with ids built from the route's whole identity. And `internal/acceptance`'s oracle boundary
test, which already refused any production file that named the state file, now covers the event
log as well and allows the two harnesses as directories rather than as a list of files somebody
extends without thinking about it.

See [[ADR-257-a-desktop-harness-that-says-what-it-did-not-prove]] and [[Desktop-Harness]].

**And then eight desktop scenarios and one recording layer, 9 of 9 in about 36 seconds.** The
moved window keeps the press (the guarded offset re-based onto the window where it is now); the
resized window is refused with **nothing at all sent**, which only the new event log can
establish; two runs produce two distinct references; **Stop during a run ends it as cancelled and
the press after the stop never arrives**; the `marco.exe` whose parent is this run's own Studio,
killed mid-run, leaves `started` with no `ended`; readiness notices its own sibling moved aside;
and a routine saved while the window is open appears in it beside the one that was already there.
The recording path is the second layer: 24 named scripted-recorder tests, run explicitly, because
a `-run` pattern that matches nothing is the quietest possible green and a skip is not a pass.

Two observations came out of it, neither changed. **A guarded press refused because the window
changed size is recorded as `failed`, not `refused`** — and `activityOutcome` writes the
distinction down itself: *a door saying no is not a thing going wrong, and the whole reason the
vocabulary separates refused from failed is so a person can tell them apart.* A window of another
size is a door saying no, and what the person is told is that their routine stopped at something
that did not happen. The scenario accepts either word for "nothing was pressed" and reports which
it got; which word is right is a product decision. **And the stale-binary trap is live on this
machine**: with the sibling moved aside the panel answered with its third case, not its second —
`D:\Macros\marco\marco.exe is on PATH, so Play would run whichever marco.exe is on PATH, which may
be a different build of a different age`. The repo root is on PATH here, so the readiness case that
never fails loudly is not hypothetical.

One defect in the harness, found by the harness. Four scenarios refused in a row with *the fixture
says its window is 716x529 and Marco reads it as 626x469* — because a `world` built fresh per
scenario reset the counter that keeps their stores apart, so every scenario used store 1 AND the
same artifact paths. The second scenario's wait for *the fixture wrote its oracle* was satisfied
instantly by the first scenario's file, and everything it then read described a window that had
already closed. One world, a number per scenario, and a scope per scenario's artifacts.

## §38X — two false-green paths in the harness, closed

The Architect reviewed the harness and approved its shape, with two corrections. Both were ways
the harness could report success without having established anything, which is the one defect
class a harness must not have.

**A partially skipped run exited 0.** The verdict was *something passed and nothing failed*, so a
run in which a scenario declined — the window would not resize, the fixture would not start, the
foreground was not ours — reported success with that scenario's question never asked. The one most
able to decline is the resized-window refusal, which is also the one that establishes the guard
refuses at all. A skip is now an ending of its own: `verdictOf` names every case, including a
selection that matched nothing (which establishes nothing while having failed nothing) and a
failure beside a skip (where the failure wins, because a finding outranks a gap). A partial run
says `INCOMPLETE` in words above the counts, and `TestASkippedScenarioIsNotASuccess` asserts four
shapes of partial run separately not to exit 0. `run.cmd` now builds and runs the binary instead of
`go run`, which reports a child's code as a sentence on stderr and exits 1 itself — flattening 2
and 3 into "a scenario failed" for whatever reads it.

**A run finishing did not establish that the fixture had seen what it was already sent.** The
refusal scenarios took the run record's `ended` as the moment that made "the log is empty" a fact.
It is half of it: `ended` says nothing more will be SENT, and the log is written when the
fixture's message loop DISPATCHES a message. A fixture that is behind can hold a press in its
queue while the harness reads the log and reports that the guard refused — a press that happened,
reported as a press that did not. A longer wait is the same sentence with a bigger number in it.

So the fixture answers for itself: the harness writes a token and the fixture writes it back once
its pump has gone **idle after reading the request**, an idle meaning the queue held nothing at
all. An idle rather than an acknowledged message, because a posted message is retrieved *ahead* of
queued input — a sentinel would come back with the presses still behind it, which is worse than no
barrier because it would look like proof. Five scenarios cross it, and
`TestEveryCountOfWhatArrivedIsTakenAfterTheBarrier` reads the rule off the syntax, because the
mistake is an omission and nobody catches those by reading.

It is proved before the first scenario rather than assumed. `--selftest` makes the fixture queue a
press in its own message queue and stop its loop until released; the harness reads the log there
and must find nothing — **that read is the false green, reproduced on purpose** — asks for the
barrier while the loop is still stopped, and reads again afterwards, where the press must be. The
sentence it produces is printed in every report. The mode is confined to the self-check by
`TestOnlyTheBarrierSelfCheckStagesAPress`, because it is the one thing here that produces a press
nobody sent. What the barrier does NOT establish is written where it is defined: input the system
has accepted but not yet handed to that queue, which a system thread does and nothing in user mode
orders against.

**And the transcript is retained, by the window that gathers it.** `studio-screen.txt` is every
word UI Automation can read off the page, which is not the child's output: that lives in a `<pre>`
behind a collapsed `<details>`, correctly absent from the accessibility tree. The product already
gathers it — **Report a problem** writes the record, what the run said as it ran, and the routine
as it stands, into one file and sends it nowhere — so the spine presses that button and keeps what
the window wrote. A product path with no automated coverage at all is now exercised, the window
says where the file went and the harness reads it from there, and an unavailable transcript is
labelled by the product rather than silently absent.

Verified: 9 of 9 again, the barrier's self-check held (the staged press invisible before it and
present after), `-only` with a name that matches nothing exits 2 through `run.cmd`, and the
retained report holds eight lines of the child's own output. The outcome-word observation stays
parked, as asked.

## §38Y — the barrier's self-check was checking the wrong instant

The Architect closed the exit-code and transcript corrections and found one more, in the thing I
had claimed was the proof: **the self-check could not have caught a barrier that acknowledged on
receipt**, which is the faulty alternative it exists to reject.

The fixture reached `Drain` only from its 50ms timer, and that timer cannot fire while `Stage` is
holding the loop. So the sequence was: press queued, loop held, harness asks, loop released, **pump
dispatches the press and writes it down**, and only then does a tick reach `Drain`. An
implementation answering the moment it read the request would have answered after the press was
already in the log, and the check would have passed. My report to the owner asserted the opposite.

Measured rather than argued. With `drainAt = idles + 1` mutated to `drainAt = idles` — the idle
requirement removed — the previous self-check printed `drain barrier: checked against a fixture
holding a queued press` in its usual words and the run exited 0.

The fix is that the holding loop calls `Drain` itself, so the request is read with the press still
pending and nothing being dispatched. No idle can occur there, so a correct barrier is SILENT, and
the silence is what is asserted — at one instant, on three things somebody else wrote: the request
file is gone (the question was taken), the fixture's blocked marker is still there (it is still
behind), and the log still holds no press. The same mutation now ends the run at exit 3 with *the
fixture acknowledged the barrier while a press was still pending in its own queue*. A fixture that
caught up before it could be checked is NOT RUN, because silence that was not under pressure
establishes nothing. No interval is waited anywhere in it.

Removing the `Drain` call from the holding loop does not quietly weaken the check either: the
harness waits for the request to be taken while the fixture is behind, so a fixture that cannot
take it there reports NOT RUN and the whole run exits 2. That is enforced by the behaviour of every
single invocation rather than by a source scan.

## §38Z — and the silence was read one pass before the answer was decided

The Architect accepted the fixture's new state and found the race still open: **`Drain` evaluates
whether to acknowledge on a LATER call than the one that reads the request, and the harness waited
only for the deletion.** With the idle requirement mutated out, this schedule still passed — the
fixture consumes the request and returns without answering; the harness sees the deletion, finds no
answer, releases the loop; the press is dispatched; a later `Drain` answers; the final log check
passes. The exit 3 in §38Y demonstrated detection on those runs without eliminating the schedule.

They were right, and the §38Y report was one pass too optimistic in the same way the §38X report
had been.

**Why it passed anyway, and what that was worth.** Two arbitrary constants: the holding loop called
`Drain` every 25ms and `until` polls files every 100ms, so the harness usually noticed the deletion
only after the reply had been written. Caught 3 runs out of 3 — on the strength of a poll interval.
Slow the fixture's reply evaluation to 400ms, which is exactly the deliberately delayed processing
this kind of check is meant to survive, and the previous self-check printed `drain barrier: checked
against a fixture holding a queued press` in its usual words and exited 0, with an
acknowledge-on-receipt barrier standing in front of it. 2 runs out of 2.

The fix is a fixture-owned checkpoint for the DECISION rather than the question. `Drain` now reports
what it did — nothing, took the question, declined for want of an idle, or answered — and the
holding loop publishes that to `<state>.considered` with the token in it. The harness waits for a
decision about ITS token, then asserts silence and releases. What it is looking at has already had
its chance to answer, and a checkpoint from a previous barrier cannot stand in because the token is
per-question.

Measured both ways, under the 400ms delay that defeated the old check: the new one ends the run at
exit 3 (*what it decided was to acknowledge it*, and the window's own word for it quoted back), 2 of
2. And removing the checkpoint from the fixture, with the barrier itself correct, makes the run NOT
RUN at exit 2 — *the request has been read and not yet decided* — rather than passing quietly.

`TestTheSelfCheckLooksAfterTheFixtureHasDecided` is the ninth boundary test: nothing in
`checkTheBarrier` may read whether the fixture acknowledged the barrier, and above all nothing may
release its held loop, before the fixture has said which way it decided. Held off the syntax
because this instant has now been missed twice in twenty lines and both times the code read as
correct.

The lesson is the one worth keeping: **a self-check has to be aimed at the instant the wrong
implementation does the wrong thing, and that instant is easy to miss by one pass of a loop.** Three
versions of this check; the first two passed for reasons unrelated to what they claimed.

The Architect approved `5eccbcb` and **closed the harness review for this scope**: `plugins/e2e` is
now the automated regression gate for the scenarios it covers, and no further owner walkthrough is
needed for it. Both limits stay explicit by instruction, which is also how they are already held —
`TestTheReportAlwaysSaysPhysicalCaptureWasNotExercised` prints physical capture as NOT EXERCISED on
a green run in the same words as on a red one, and the barrier's own definition says in the code
that a drained fixture queue is not proof that Windows had finished handing input to it. What the
harness still cannot do is the FULL PHYSICAL-RECORDING PATH: a gesture Windows captured from a
person's own hand, through the hooks, into a draft. That is what remains unexercised, and it is
unexercised by construction rather than by omission — both hooks discard injected input.

Narrowed deliberately, because the first version of this sentence said more than it should: review,
Save, Record again and the replacement refusals are NOT inherently manual, and parts of each are
already covered by the 24 named scripted-recorder tests the harness runs as its second layer. What
no automated run can supply is the physical gesture at the start of them. `E2E.md` §63 is the
walkthrough for the whole path end to end with a person's hands on it, which is a different claim
from "none of this is tested".

## §39 — a repair edited a claim out of somebody's file, and took the pauses with it

`orchestrator.Resave` saves a guarded routine again under the scope its presses need, rebuilt from
the demonstration stored beside it. It rebuilt the draft with `Unstated`, and that was two defects
in one argument.

The visible one: a routine that stated `// Recorded as: coordinates.` came out of a repair stating
nothing. Absence is the honest answer for a file that never said — writing it over a file that did
say is a claim about somebody's afternoon edited out of their routine. Their card loses the
sentence and every run record of it afterwards reads `unstated`.

**The one that changes what the routine does:** the stated mode is also what lifts the 50ms wait
cap, because the cap only ever made sense when an anchor poll did the real waiting. The repair runs
under a `Deps` built for the repair, which states nothing, so a Studio launched with `$MARCO_CV=on`
rebuilt the routine with every recorded pause capped and no anchor to wait on in their place — a
menu pressed before it has drawn, out of a repair. Studio is the window that inherits those flags
all afternoon and the repair button is in it.

The fix makes the mode travel with the recording instead of with whichever `Deps` is holding it.
`Resave` reads it from the SOURCE of the file it is about to replace, through `ModeOf`, so an
unknown word still reads as unstated and a repair never starts carrying an unverified claim.
`learnOptionsFor(mode)` is the distinction that was missing: `DraftOf` simplifies under the mode it
is GIVEN and `Capture.Finish` under the mode its capture ran under, rather than both asking the
`Deps` they happen to hold. Nothing is versioned and nothing is migrated.

Six tests, all through the production repair path (`Unrunnable` then `Resave`, which is what the
button calls), and the first two have to be a pair: with the simplification mutated back to the
`Deps`' mode, the word stays in place and the routine's longest pause goes from 800ms to 250ms, so
only the pauses test fails. Dropping the stated mode again fails the word test, the pauses test,
Studio's card test and the run-record test, each in its own sentence. The listing is held in
`plugins/studio` and the run record in `cmd/marco`, because the record is read off the source: a
repair that edits the source changes every record after it without anything in `cmd/marco`
changing.

**Named and not fixed, deliberately:** `SimplifyRoute` — the "simplify further" path — has the same
shape. It rebuilds a saved route from the same stored recording, simplifies under the environment,
and writes through `codegen.Route` rather than `RouteAs`, so it drops the mode line and caps the
gaps exactly as the repair did. It was out of the scope the Architect bounded this to, and it is a
second behaviour change in a second user-facing path (it would start writing a line where it never
wrote one), so it is recorded here rather than folded in quietly.

## §40 — chords: three defects in six events, and the promise nobody was keeping

`do OS's Key with "ctrl+shift+a"` is one line of Marco and six events at the boundary. The last
two are a promise: a modifier this process pressed stays down on somebody's computer until this
process lets go of it. Windows does not clean that up, the application cannot see that it is
happening, and the person's next keystroke — and every one after it — arrives as something else.

Three defects lived in the middle of that sequence, inside the function that calls `SendInput`,
which is why none of them was reachable by a test:

1. **A rejected modifier left the earlier ones down.** The loop returned its error from the middle
   of itself. `ctrl+shift+esc` with the Shift rejected returned *SendInput rejected key "shift"*
   with Ctrl still held on the desktop, and the reported half was the smaller half.
2. **A stop during the modifier settle stopped nothing.** `sleepCtx` returns early when the run is
   cancelled — that early return IS the signal — and the next line pressed the key regardless. So
   Escape or the panic key, pressed in the one window where a chord was in flight, sent the chord.
   It had never fired, which is its own argument for a test.
3. **A release the machine refused was discarded.** The reverse loop ignored every return value, so
   the worst of the three outcomes reported success: nothing in the run record, nothing in the exit
   status, and a modifier held down.

The sequencing moved to `sendChord`, written against a `keyMachine` that takes one key event at a
time — `down`, `up`, `settle`, three methods and no state — so a refused modifier, a stop
mid-flight and a release the machine will not take are all arrangeable without a desktop. Every
key pressed is released before the function returns, in reverse, whatever happened; a key the
machine would not press is not released, because there is nothing down to undo; a stuck key
outranks a refused keystroke in what the caller is told, EXCEPT that a cancellation keeps its
identity, since a stop that stopped reading as a stop would turn a person pressing Escape into a
routine that failed. And the failure names the part and the chord — *the machine refused the "c"
in "ctrl+c"* — rather than the whole spec.

Ten tests, four mutations, each killed by the test named for it and by no other: the bare return
from the modifier loop, the missing context check after the settle, the missing one at the
entrance, and the discarded release failures (that one fails three tests, which is the single
place the tests overlap on purpose).

**And the desktop half, which those cannot reach.** `plugins/e2e` `keys/a chord arrives as a
chord` plays `ctrl+shift+a` into the field the routine clicked into first, on three accounts: the
order of the key messages the application received; the modifier state the application read from
the SYSTEM at the instant the key arrived, which is new in the fixture and is the fact rather than
the evidence — a sender whose modifier events the system never applied produces the same four
messages in the same order and delivers a plain A; and what the desktop is still holding
afterwards, read through `GetAsyncKeyState`, because no message tells an application about a key
nobody released. The last is read before the run too: a modifier already held is a fact about the
desktop, so the scenario declines rather than fails. The demonstration is RECORDED as a chord, not
written as one — `simplify` folds the held modifiers, and a harness that wrote `ctrl+shift+a`
itself would be asserting against its own opinion of what a demonstration produces.

It passed first time, which is worth saying because the three unit-level defects were real: the
chord path has always worked when the machine took every event. What it could not do was fail
safely.

### Three flakes the new scenario shook out, two of them fixed

Three consecutive full runs each failed a DIFFERENT scenario, and chasing it mattered more than
re-running until green — a gate that fails for its own reasons is a gate people learn to ignore.
(I briefly concluded the chord change had caused the first one, on a two-fail/two-pass bisect;
three more runs showed it passing 3 of 3 with the change in. The bisect was too small.)

- **Done was pressed before the window offered it.** `waitForRecord` returns when the child has
  written its account; Done becomes pressable when the window has noticed the child exit. Two
  instants, a real gap. It waits for the button now, in the same shape `play` already used.
- **The fixture's snapshot could not be read while it was being replaced.** The fixture wrote a
  temp file, deleted the real one, and moved the temp over it — so a reader arriving in the gap
  was told *Access is denied*, which is Windows's answer for a deletion-pending file and not a
  sentence anybody would connect to a file being rewritten. `File.Replace` now, which is atomic
  and keeps the file always present, with the other exception it can throw caught beside the one
  that was; and the harness retries a transient read anyway, because a reader of another process's
  file does not get to assume how that process writes it.
- **Not fixed:** once in about ten runs, the first scenario failed with Marco's own refusal — *the
  fixture window is no longer in front, so a press would land in whatever is*. Marco was right to
  refuse and the harness was right to report it; what took the foreground between the activation
  and the press is not established. It fails SAFE, so it is recorded rather than worked around.

### §40a — and the stop was still taking the stuck key with it

Found in review of §40. `sendChord` reports a cancellation and a refused release together, with
both identities intact — and then `performOnePlay` sets the outcome to `cancelled`, clears the
error, and leaves `because` empty, because a stop is not a failure and never carried an error out.
So the durable record said `cancelled` with nothing beside it while Ctrl was down, and
`saidOfRoutine` had the same gap one layer on: *"X" was stopped part of the way through.* and no
more.

**The error could not be the channel.** Two things happen to it on the way up: `runRoute` flattens
the host's account into the sentence *"X" stopped: …*, so nothing above it can recognise anything
inside it, and `performOnePlay` then throws it away. So the host keeps this one the way it already
keeps what it did not do — `Host.NotReleased` beside `Host.Unperformed`, and two lists rather than
one on purpose: everything in `Unperformed` is something that did not happen, and this is the
opposite shape, something that DID happen and has not finished. `cmd/marco` asks the host after
the run, through a `stillHolder` interface for the same reason `unperformer` is one.

From there: into `because`, onto the run's own output as the LAST thing it says (which is how the
window sees it at all — Studio keeps the final sentence before the outcome word and does not parse
it), and through `alsoSaid` into the card's sentence. The outcome stays `cancelled` throughout.

Six tests across the three layers, each mutation-checked: the production-host cancellation path
with the machine refusing to release Ctrl (and its control, which is what stops the
implementation from warning after every stop), the record and the said-out-loud half and the
ordering in `cmd/marco`, and the card both ways in `plugins/studio`.

**One wording correction carried through the record:** "everything pressed is released" is
narrowed everywhere to "every pressed key gets a release ATTEMPT". The machine can refuse a
release — that refusal is the entire subject here — so the stronger sentence was a promise the
code cannot make.

**A debugging note worth keeping.** The first version of the production-host test looked like the
sequence was ignoring stops: the stand-in machine cancelled the context and returned at once, and
`sendChord` then read a context that had not been cancelled yet. A stop reaches a host call by
TRAVELLING — the runtime watches the entry context from a goroutine of its own and cancels the
frame tree when it fires — and the real `settle` waits, so it sees it. The stand-in now waits for
the cancellation to arrive, bounded so a wrong arrangement fails rather than hangs. It cost an
afternoon of suspecting working code, which is the second time this week a stand-in that did not
wait made a correct implementation look broken.

### §40b — the gate for chords, and two questions that are not one question

Closed at `2aa5d21`. Three notes from the review worth keeping:

**The gate.** `plugins\e2e\run.cmd -only keys` is the regression check for an executable change to
the chord path, and it needs no owner walkthrough. A documentation change needs no run.

**A foreground interruption stays a failure or a NOT RUN.** It fails safe, and a harness that
re-ran until green would convert the one honest answer into noise. What is genuinely about this
desktop rather than about Marco is declined BY NAME — a modifier already held, an unmeasurable
fixture, a remote session — which is a different thing from retrying.

**And a correction to my own summary:** I listed the refused-release question and
`internal/stopsignal.Raise`'s unretried *Access is denied* side by side as "the same question of
whether a refused Windows call deserves one retry". They are not the same question. A refused
release leaves a key down on somebody's desktop and the consequence is continuous — every
keystroke after it arrives modified — so the answer has to weigh a retry against making an input
path do more work while something is already wrong. A refused stop signal fails to deliver one
message, and the consequence is that one thing did not happen, which is an ordinary delivery
failure with ordinary answers. Grouping them by the shape of the call rather than by what happens
next is how two unrelated decisions get made together and neither gets thought about.

## §41 — held keys: the same promise, kept by the end of a run

The next slice after chords, and the same decision (ADR-258) rather than a new one. A hold —
`hold Q, click, release Q`, which `simplify` produces whenever a key stays down across something
else — is a key held down on somebody's computer deliberately. The difference from a chord is who
cleans up: the END OF A RUN does, so a hold whose release goes wrong goes wrong at the moment
nobody is watching any more.

Four of the chord defects were there one layer up, and a fifth of its own:

1. **`doKeyUp` forgot the key before finding out whether the release worked.** `delete` came
   before the call. The route-end sweep is the one thing that could try again, and it was being
   told the key had already come up.
2. **The sweep discarded every answer** — `_ = h.b.keyUp(...)` — so the one path whose entire
   purpose is "nothing is left down" was the one path that could not say otherwise.
3. **The keys were held in a map**, released in whatever order Go produced. A pile of holds coming
   apart in no order is a different keyboard state every run, and nothing could pin it. A list
   now, released in the reverse of the order they went down.
4. **`keyDown` returned `ctx.Err()` after a successful send.** The worst of the five: a run
   cancelled in that instant pressed the key, reported the hold as FAILED, and was therefore never
   written down as holding anything. A key down for ever, invisible to the sweep, the record and
   the person. A send the machine took is now reported as having happened, because it did.
5. **And nothing stopped a stopped run from starting a hold** — a key pressed down by a routine
   that is already over, whose release has already happened.

A refused release stays on the list so the sweep tries once more; one refusal then success is the
ordinary transient and that retry is the only recovery here. Whatever the sweep still cannot
release goes into `Host.NotReleased`, so it reaches the record, the run's own output and the card
by the path the chord slice already built — and `Host.held` is a list now, because a chord's stuck
modifier and a sweep's stuck hold are two different sentences and a single field let the second
replace the first.

Ten tests. Five mutations over the five fixes, each killed by the case named for it and no other
except where they overlap on purpose. One of the five is held by reading the SOURCE —
`TestTheHoldPrimitivesDoNotReportAStop` — because from above the backend "the machine took it and
the run was cancelled" and "the machine refused it" are the same error, and the only way to
exercise the real primitive is to send real input into somebody's desktop. And one is an ORDERING
claim at the command layer: the sweep is a deferred call inside the run, so a key only the sweep
discovered has to reach the record, which it does because the account is read afterwards.

**Not done, and it is the remaining piece of this slice:** a desktop scenario for a hold. The
harness can seed one — `simplify` already produces the steps — and the fixture's key log, with
the modifier state it now records, is the right oracle. It is left unwritten rather than written
and unrun: adding an unexercised scenario to a gate makes that gate's first run a debugging
session instead of a regression check, and the owner's screen was not available.

### §41a — a hold's failure went nowhere, twice

Two P1s in review of §41, both about PROPAGATION rather than cleanup.

**The generator propagated `Key` and not `KeyDown`/`KeyUp`.** A guarded routine's hold was written
as an ordinary step, so `hold Q → click → release Q` could click after Q failed to go down. The
click is a different gesture without the Q, and the window guard cannot tell — the window is the
same window. The release is the half that looks harmless: everything after a release the machine
refused runs with the key still down, so the presses that follow are modified presses, and the
sweep recovering the key later is not the same as the routine having been right to carry on. Both
are links now, in newly generated guarded routines; nothing on disk is rewritten.

**And the host recorded neither failure.** `doKeyDown` returned a failed step, `doKeyUp` returned
the error, and neither reached `Unperformed` — so the command read an empty account and reported
success about a routine that ran with a key in the wrong state. Both go through `delivered` now.

Neither half is sufficient, which is why the tests go through generation → runtime → production
host: a propagated step whose failure is not in the account lets the command report success, and
an account without propagation lets the routine go on pressing. Mutating the generator kills the
stopping tests and the control; mutating the host kills the account tests and the legacy case.

**And a run that finished holding a key was reported as performed.** `performOnePlay` read the
cleanup account in the cancelled and failed branches and not in the one every successful run
reaches. A routine that finished normally while holding a key, whose sweep then could not release
it, had no cancellation and no execution error — so it fell through to `performed` with nothing
said. The earlier sweep-to-record test forced a cancellation and could not see this path, which is
exactly what the review said.

It is `failed` now — *it having been tried, and gone wrong* — with the key in the record, on the
run's output and on the card. Its control is the one that matters most: that branch answers for
every successful run of every routine, so a check that was wrong about the empty case would turn
the product's ordinary answer into a failure.

**One imprecision left on purpose.** Studio's sentence for a failed routine is *"X" stopped, and
nothing after the press that did not happen was done* — right when a hold or release failed
mid-routine, slightly wrong when only the route-end sweep failed, because nothing stopped and
nothing was skipped. The specific sentence arrives beside it through `alsoSaid`. Rewording a
shipped sentence for every failure to suit this one case costs more than it buys, so it is
recorded here rather than quietly changed.

### §41b — closing §41a, and an observed timeout with a hypothesis about its cause

> **Since explained, and the hypothesis was wrong.** The timeout recurred on 2026-10-26, the
> panic dump was kept, and it named a deadlock in `recorder.Stop` — not contention. §41j has it.
> This section is left as written, because how the guess was recorded is the part worth keeping.

Approved through `a637996`. Two limits came with the approval and belong next to the work rather
than in a message: **existing saved routines are not automatically regenerated** (recorded in
[[ADR-258-a-chord-is-a-promise-to-let-go]] under what did not move), and **the desktop hold
scenario remains unverified** — it can be completed through the automated harness when desktop
execution is available, with no owner walkthrough. It is unwritten on purpose: adding an
unexercised scenario to a gate makes that gate's first run a debugging session instead of a
regression check, and the owner's screen was not available.

**And a correction to my own report.** One combined background run had `cmd/marco` hit its 600s
package limit while `plugins/studio` finished in 63s. Run alone with nothing competing the same
package gave `ok … 12.117s`, exit 0, with no failing test in it. I reported that as contention,
because I had been compiling and running other Go programs at the time and this package is already
documented as having been starved past that same limit once before (CLAUDE.md, the corpus test).

That is a HYPOTHESIS, and the isolated pass is not evidence for it. An isolated pass establishes
one thing: the suite passes in isolation. It does not establish what the slow run was waiting on,
and it does not establish that the slow run will not happen again — a single non-reproduction over
one attempt is weak evidence either way, and the two facts I reasoned from (my own concurrent load,
and a previously recorded starvation of the same package) are circumstantial. Nothing was measured
while it was slow: no goroutine dump, no per-test timing, no `-cpu` or `GOMAXPROCS` comparison. So
the honest record separates the two: *an OBSERVED 600s timeout in `cmd/marco`, once, not
reproduced* — that part is a fact — and *contention as a HYPOTHESIS about its cause, with no
measurement supporting it*. Writing it as "a timeout that is a hypothesis" blurs them, which is
the slip worth not making twice: the event happened, and only the explanation is a guess.

If it recurs, the thing to capture is the panic dump the timeout already prints — it names the
goroutines that were alive — rather than re-running the package alone, which is the one
experiment that cannot answer the question.

Filed beside the harness's unexplained foreground refusal (§40b) as a second open anomaly that
fails safe.

### §41c — the hold scenario, written while the screen was unavailable

The desktop hold scenario was outstanding because the owner's screen was not free. That is a
reason to defer EXECUTION, not writing, so it is written: two scenarios, `hold/a hold is held
across the press` and `hold/a stopped run lets go of the key`, both **implemented with desktop
execution pending**.

**What needed building.** The fixture could not see a held non-modifier at all.
`Control.ModifierKeys` answers for Ctrl, Shift and Alt — the whole of a chord's claim and
half of a hold's — so a window asked whether Q was down could only ever have said "no
modifiers". It now reports the state of keys the harness NAMES (`--watch q`), read with
`GetKeyState` at the instant the message is handled, and on PRESSES as well as on keys, because
the claim is about what was held when the click arrived. Named rather than enumerated: a fixture
reporting every key's state is a keylogger.

**Q and not a modifier.** `markHolds` skips modifiers by name, so Ctrl held across a click
produces no key step at all — a modifier hold would not have worked. And the cost of the scenario
failing is a key left down on a real desktop: a letter repeats into whatever has the keyboard, a
modifier changes every keystroke anybody types until somebody works out why.

**The gate.** Both carry `onlyWhenAsked`, so a default run skips them and every report names them
as *implemented and NOT EXERCISED* beside the physical-capture limit. Filed under `hold/` and not
`keys/` so `-only keys` — the chord's gate — does not sweep them in. One green run takes the flag
off, and half of `TestTheHoldScenariosAreAskedForByName` goes with it.

**And the oracle is tested without a desktop.** A scenario is an arrangement plus a set of
questions; the arrangement needs a screen and the questions are functions over a log. So each
question is fed a log the fixture could really have written, in both directions, including the one
case no other account can see: the same messages in the same order with the system never having
applied the key, which delivers a plain press. Eight mutations, eight killed — and one of them
survived at first, because the tests beside it exercised the assertion functions and not the chain
that calls them.

**One of those mutations found something real.** The barrier-ordering boundary test guarded
`presses` reads and not `keys` reads — so every absence the CHORD scenario asserts, and every one
the hold scenarios assert, was crossing no barrier by that test's reckoning. Extending it
immediately flagged two reads in failure paths that cannot cross the barrier, because they happen
when no run record appeared at all. Rather than exempt them, those reads now return PROSE
(`arrivedSoFar`): a reading taken without the barrier is a lower bound on what arrived, which is
evidence for a reader and an unsound basis for a verdict, and making it structurally unusable as
data beats remembering not to use it as data.

**Still outstanding, and now only this:** one green desktop run. No owner walkthrough — the
scenarios are automated end to end.

### §41d — and a Ctrl-held click is a separate defect, now measured

Closed the hold scenario's source review; the Architect drew one line worth keeping. The hold work
covers a NON-MODIFIER hold such as Q. It establishes nothing about a Ctrl- or Shift-held click,
and that limitation is separate.

Measured rather than assumed, because the shape of the gap decides whether it is a small fix or a
language question. A demonstrated Ctrl, click, release Ctrl comes out of `simplify` as
`activate`, `wait`, `click` — **a plain click**, with the modifier nowhere in the steps.
`markHolds` skips modifiers by name, which is right when the thing between the edges is another
key (that is how `ctrl+shift+a` becomes one chord step) and wrong when it is a click, because
there is no chord for it to fold into.

And the other half, also checked: `hold ctrl → click → release ctrl` is already fully
expressible. Built as IR by hand it generates `do OS's KeyDown with "ctrl"`, the guarded click and
`do OS's KeyUp with "ctrl"`; `driver.CheckSource` accepts the result and `Unpropagatable` reports
nothing, so every step is a link in the chain and a failed modifier press stops the click. So the
whole of the defect is in `markHolds`, needing no new word and no new field on a click step.

Not fixed, and deliberately: it changes what `simplify` makes of recordings that already exist,
which is a decision rather than a correction. Recorded in [[Demonstrations]] and in
[[ADR-258-a-chord-is-a-promise-to-let-go]] under what did not move. It is also why the desktop
hold scenario holds Q — a modifier would have been dropped before it ever reached a routine, so
the scenario would have been asserting about a plain click.

Worth naming the category: this is a loss of recording fidelity that refuses nothing. A Ctrl+click
and a click are different acts in most applications — one adds to a selection, one replaces it —
so what comes out is confidently wrong rather than visibly incomplete, which is the kind of defect
every account in this system is weakest against.

### §41e — the Ctrl-held click, fixed

§41d recorded a dropped Ctrl+click as a separate open defect. The Architect's call was that it is
incorrect playback rather than missing coverage, so it was next, as a bounded correction with no
language change. [[Demonstrations]] holds the rule and the refusals; this is what it cost and what
it taught.

**Two changes, and either alone does nothing.** `markHolds` decides which presses are holds and
`lower` decides what is written down. The modifier branch in `lower` ran BEFORE the hold markers
were read and `continue`d past them — so teaching the classifier about modifiers on its own would
have produced correct marks that nothing consumed. The mutation run shows it from both sides:
disabling the classifier and making the lowering skip modifier marks each kill the same tests.

**The rule.** A modifier is a hold when it spans a pointer action, and only then. Modifiers around
a KEY still fold into a chord — `ctrl+shift+a` is one step and must stay one, because the chord
presses and releases them itself. And the duration rule that makes an ordinary key a hold after
half a second does not apply to a modifier: held over nothing it modified nothing.

**A key under a hold is just that key.** `ctrl` down, `a`, click, `ctrl` up comes out as
`KeyDown ctrl`, `Key "a"`, the click, `KeyUp ctrl`. Not `ctrl+a`, which would press the modifier
again and release it before the click — the Architect named that one in advance. And no `Text` on
the key, which is the half I would have missed: `Text` is what lets keys coalesce into a `Type`
step, and `Type` sends characters rather than keystrokes, so `a` under a held Ctrl would arrive as
the letter after a fold two passes later that nothing at that point can see.

**The refusal needed a channel, so `Simplify` returns an error.** Every caller then has to answer
for it — a compiler question rather than something to remember, and the one mutation that tried to
drop the reason from a draft would not compile. Four answers, one enforcement: `Learn` and
`LearnAuto` print and save nothing, `SimplifyRoute` prints and leaves the routine already on disk
alone, a draft carries the sentence so review shows it beside the steps, and `Save` refuses —
which is the only path from draft to routine.

**A surviving mutation that is a result rather than a gap.** Which edge of a click decides "spans
a pointer action" can be changed with no test failing. That is because every recording where the
two edges disagree is refused first: a modifier pressed or released in the middle of a press was
held for PART of a click, and part is not a thing a routine can say. Recorded as an equivalence
rather than chased, and the test that depends on it says so.

**And a third gated desktop scenario**, `hold/a ctrl-held click arrives with ctrl held`. The
fixture now reports the modifier state on PRESSES as well as on keys, which is what settles the
claim nothing else can: the four messages of a chord and the two of a modified press look the same
from outside, and only the application's own reading of the keyboard says which arrived. Ctrl
rather than Shift or Alt — Alt over a press opens menus, Shift extends selections, and Ctrl+click
on a text field or a button does nothing anywhere.

Nineteen mutations across the two rounds, eighteen killed, one equivalent.

### §41f — and the extra modifier the fix dropped

A defect in §41e, found in review. `Ctrl↓ Shift↓ A↓ A↑ Shift↑ click Ctrl↑`: Ctrl spans the click
so it is a hold, Shift spans only the A so it is still chord state — and the held-key branch
emitted a bare `Key "a"`, dropping the Shift. Playback performed Ctrl+A and then Ctrl+click where
Ctrl+Shift+A and then Ctrl+click were demonstrated. Reproduced before fixing: the step list came
out `keydown(ctrl) key(a) click keyup(ctrl)`.

It is `Key "shift+a"` now — the extra modifier folded into a chord, the held one left out of it,
because `sendChord` presses and releases exactly what it is given and naming Ctrl there would let
go of it inside a hold that is still open.

**And the obvious guard turned out to be dead code.** I wrote the fold as "chord state MINUS the
held classes", with a `heldClasses` helper to compare them. Mutating the exclusion away failed
nothing — because a modifier that becomes a hold never reaches the branch that sets the flags, so
the flags are already false. The exclusion could not have mattered; two mechanisms guaranteed the
same thing and only one was doing it.

So it is removed. Dead code that looks load-bearing is worse than none: the next reader trusts the
filter instead of the ordering, and the ordering is what was wrong in the first place.

**Which left a load-bearing claim no behavioural test can reach.** No input distinguishes "the
held modifier was filtered out of the chord" from "it never became chord state" — getting it right
one way and the other way produce identical output. So
`TestTheHoldMarkersAreReadBeforeTheModifierState` reads it off the source: the hold markers must
be consumed before the branch that records modifier state. The `:=` that declares the four flags
at the top of `lower` is an assignment to the same names and means the opposite thing, so the test
discriminates on the token — which is why it is an AST walk rather than a grep. Reordering the two
branches now fails six tests including that one.

Same technique and same reason as `internal/oshost`'s `TestTheHoldPrimitivesDoNotReportAStop`:
some rules are about the shape of the code, because the behaviour they produce cannot be told from
the behaviour of getting them wrong a different way.

**The ordering claim end to end** needed one log rather than four. `recBackend` keeps its calls per
kind, so nothing in it said whether the click happened inside the hold or after its release; it
now also keeps `calls`, one ordered sequence, and the mixed case runs the production `sendChord`
into that same log. The result is readable as the gesture: `hold ctrl`, Shift down, A down, A up,
Shift up, `click left`, `release ctrl` — with Ctrl appearing exactly twice, which is the assertion
a chord naming it would break while leaving every ordering claim still satisfiable.

Twenty-three mutations across the three rounds, twenty-two killed, one equivalent.

### §41g — drags, scoped and paused

Not started. The gate in front of them is the three hold scenarios running green —
`plugins\e2e\run.cmd -only hold`, an automated acceptance and not an owner walkthrough. Written
down because a scope agreed in review and not recorded is a scope the next session re-derives.

Guarded drags, when they are unpaused:

| | |
|---|---|
| a fresh window check before mouse-down | not a cached one. The same rule a guarded press follows, and the reason it exists: a check that read a remembered rectangle would be asking about the window as it was |
| cancellation DURING movement | a drag is the one gesture with a long middle, so a stop can arrive between the press and the release |
| a release attempt on every exit | success, refusal, stop. The same undertaking as a hold, and the same limit: attempted by code that gets to run, reported rather than recovered |
| a delivery failure stopping what follows | a drag that did not happen leaves the screen in the state the rest of the routine was not demonstrated against |

And the oracle is the fixture's received-input log, for the whole gesture rather than its ends: a
press, the movement, the release, and where each arrived. `NoteClick` already records three
coordinate spaces per button edge; what a drag needs beyond that is whether the window saw the
movement BETWEEN them, which is the part a replay can skip while both ends still look right.

A stuck mouse button is worse than a stuck modifier, which is worth saying before the work rather
than after: a held Ctrl changes what keystrokes mean, and a held left button drags whatever the
pointer passes over. That is the first thing to make impossible to leave behind.

### §41h — the repair tool broke the file it was repairing, and the counters said it worked

Found while going to apply a one-line correction to §41b's heading: the heading's bytes were
mojibake, and the damage was mine. A `perl -0777 -pi` substitution had used a `\x{2014}` escape
for an em-dash. That escape makes a wide CHARACTER, which upgrades the whole file to character
semantics for that run — so printing it re-encoded every multi-byte sequence that was already
there. **1924 characters double-encoded**, 1291 em-dashes, 369 arrows, 38 section marks,
introduced in `32819a2` and surviving three commits, because that kind of damage is only visible
where somebody happens to be looking at the line.

The fix collapses one layer: a run of `C3 xx (C2 yy)+` decoded as UTF-8 gives codepoints all below
0x100, and re-encoding those as Latin-1 gives the original bytes back. Anything that does not
round-trip is left exactly as it was.

**The evidence that supports it is the byte comparison, not the counts.** The repaired file is
byte-identical to `c07af66` — the last clean version — for every line that existed then, with a
single pure-addition hunk for what was written since. The character counts also reconcile
(20+1291 em-dashes, 5+38 section marks, 2+369 arrows), and that is the weaker half: a repair that
DELETED the damaged characters satisfies the counts just as well as one that restored them. Which
is not hypothetical —

**the first three attempts deleted them, and reported success.** `Encode::decode` with a true
CHECK **consumes its argument in place**: the part it processed is removed from the scalar it was
handed. Validating `$bytes` emptied it before the return, so the substitution replaced each
damaged run with nothing while `$fixed` still counted up to 1924. A counter maintained by the code
under suspicion is not a check on it. The tool now validates copies, and the thing that caught the
bug was instrumenting the output rather than reading the summary line.

So: a byte-level rewrite of somebody's file is checked by comparing bytes against the last
known-good version. Not by asking the rewriter how it went.

**Parked, by review decision, and separate from the input work:** `E2E.md` (265 runs) and
[[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]] (50) carry the same
double-encoding and PREDATE this session — different cause, same shape. They are deliberately left
alone so what I broke stays distinguishable from what I found, and they are repairable on request
with the method above.

### §41i — the hold gate, run

The screen became available, so the three scenarios written in §41c and §41e ran:
`plugins\e2e\run.cmd -only hold`, **3 of 3, exit 0**, on a 1920x1080 96-dpi desktop. Then the
whole suite with them in it: **12 of 12 desktop and 1 of 1 recording, exit 0**, with no
NOT SELECTED line anywhere in it, which is the part that says the gate is empty rather than
merely quiet.

The claim worth naming is the Ctrl one, because it is the one nothing headless can make. A
demonstrated Ctrl+click arrived at the application **with Ctrl held by the application's own
reading of the keyboard at the instant the press was processed**. What was in doubt was never the
step list — the unit tests settle that — but whether the real scan code, sent by the real
`SendInput`, is applied by the system in time for the press that follows it. It is.

`onlyWhenAsked` came off all three. They are ordinary members of a default run now, still filed
under `hold/` and not `keys/` so the chord's selector does not sweep them in — a separate decision
that outlives the gate. The machinery stays for drags, which is the next thing that will be
written before it can be run; `TestAGatedScenarioIsNamedInTheSummary` tests it against scenarios
of its own, so it does not depend on anything actually being gated. And
`TestTheHoldScenariosAreAskedForByName` is now `TestTheHoldScenariosRunLikeAnyOther`, asserting
the inverse of what it used to: re-gating one of the three would quietly remove the regression
check it was written to be, and the only trace would be a line in a report nobody reads twice.

**And the run found a defect — in the harness.** The Ctrl scenario passed, and its own artifact
trail said the press inside the hold arrived holding NOTHING:

```
· the fixture received #4 key down ControlKey on "Name" with ctrl held
· the fixture received #5 mouse left_down on "Save" at screen 258,372 (45,14 into it), window …
```

The verdict read `presses[1].Mods` and was right. The LINE printed `Watched` and left `Mods` out,
so the one account the verdict rests on was missing from the evidence beside it — and for a press
there is no other reading, since `Watched` is only populated when a scenario names a key to watch
and this one names none.

In the passing direction that is cosmetic. The failing direction is why it is a defect: had the
press really arrived unmodified, the finding would have named the right thing while every step
above it showed a log in which nothing was held either way, and the next person would have had no
way to tell a product defect from a harness that forgot to ask. **A reader could not tell "nothing
was held" from "that was not printed",** which is the same shape as §38X and §38Z — a harness
whose output is ambiguous about its own silence.

Both kinds of line go through one `heldAt` now, both accounts in every line, and an empty reading
says `nothing held` out loud. `pressesAre`, which is what a FAILURE carries, had the same bug and
reads the same way. Two tests hold it, and the second is the one that matters:
`TestAPressTheVerdictAcceptsIsNotLoggedAsHoldingNothing` reaches the verdict through the
production assertion, so moving the claim to a different account fails it unless the line moves
with it.

Which is the small lesson from a green run: **the first thing to read on a passing scenario is
whether its own trail says what its verdict says.** This one did not, and nothing was failing.

### §41j — the timeout was a deadlock, and the fix for it was already in the tree

§41b recorded a 600s `cmd/marco` timeout as an observation with contention as the hypothesis, and
said what to do if it recurred: **keep the panic dump, do not re-run the package alone.** It
recurred on the root suite after the hold gate passed. The dump was kept. It took about four
minutes to read and the hypothesis was wrong.

```
panic: test timed out after 10m0s
  running tests:
    TestAHotkeyResolvesInTheScopeItWasBoundIn/the_binding_lands_in_the_application_it_was_made_in (9m53s)

goroutine 305 [chan receive, 9 minutes]:
  recorder.(*winRecorder).Stop(…)        recorder_windows.go:371
  cmd/marco.withPanicStop(…)             panicstop.go:78
  cmd/marco.performOnePlay(…)            intake.go:499
  …
goroutine 288 [syscall, 3 minutes, locked to thread]:
  recorder.(*winRecorder).Start.func3()  recorder_windows.go:345   ← GetMessage
```

Not contention: a deadlock, in the shutdown of the recorder, on the path **every real-input run
takes**. `withPanicStop` starts a recording to watch for the stop key and stops it when the play
ends, so this is `marco do`, and every Play in Studio, and not only a test.

**The mechanism.** `Stop` wakes the hook pump by posting WM_QUIT to the thread that installed the
hooks, because only that thread may remove them and it is asleep in `GetMessage` until a message
arrives. A Windows thread has **no message queue until it first calls a message function**, and a
post to a thread without one fails. The pump publishes its thread id *before* installing its
hooks — deliberately, so a Stop racing a Start still has somewhere to aim — and that is the
window. A single unchecked post into it is lost; `GetMessage` waits for a message that will never
come, and `Stop` waits with it, unbounded. A SHORT play is the shape that posts into the window,
which is why a test suite found it and a person demonstrating for thirty seconds did not.

**And the fix was already in the tree.** `internal/platform/navsource` had the identical bug, found
earlier, fixed with a retried post and a bounded wait, and commented with the mechanism spelled out
— including that its own suite "ran 495s before being killed". It was never carried to
`internal/recorder`. Two packages, one defect, eighteen minutes of wall clock between them, and the
second one was diagnosable by reading a comment in the first.

The reason it was not caught is worth more than the fix. `TestTheHookPumpBlocksRatherThanPolls`
walks every hook site in the repository, on explicit reasoning written into it: *the defect existed
in two places at once, and a test naming the files it knew about would have been written against
exactly the two that were wrong.* The shutdown half, `TestTheHookPumpIsWokenForShutdown`, named ONE
FILE. So the half that was generalised held, and the half that was not is where a fixed bug
survived next door. **A test that names a file is a test about that file, however general the rule
it is checking.**

So the correction is three things:

| | |
|---|---|
| the retry | the post goes in a loop until the pump answers, bounded at 5s so a pump that already died cannot wedge a caller either |
| the report | an expired bound is a different outcome, not a success: it stops recording and says in plain words that the hooks are installed until the process exits, because the hooks can only be removed by a thread that is not answering, and closing the capture queue while a callback could still send into it would panic inside a hook. Reported, not recovered, and not forced — the same rule the holds follow |
| the rule, generalised | `TestAPumpWakeupIsRetriedRatherThanPostedOnce` walks the tree like its sibling, requires every wake to sit inside a loop, and skips sites that never stop a pump (the overlay installs one hook for the life of the process). It fails if fewer than two sites wake one, so it cannot quietly end up checking nothing |

Both directions verified rather than asserted: moving the post out of its loop fails the structural
test and names the file; posting the wake to thread 0 makes the behavioural test fail in 5.01s with
the report printed, which is what proves the bound is real and that the duration assertion is not
decoration. `cmd/marco` then ran `ok … 12.237s`.

**What this does not establish.** The race is microseconds wide and scheduling-dependent, so
neither green run proves it is gone — the structural test is the one that does not depend on
catching it. And the harness's unexplained foreground refusal (§40b) is still open and is a
separate anomaly; one answered question is not two.

### §41k — guarded drags, traced and not built

Asked for by the Architect as an investigation: trace capture → simplification → generation →
execution, say what is reusable and what is missing, and propose the smallest contract. **Nothing
in this section is implemented, and the contract at the end is PROPOSED, NOT AGREED.**

**The first finding reframes the rest: drags already work end to end.** They are not missing. What
is missing is the GUARD. A demonstrated drag is captured, simplified into `StepDrag`, generated as
`do OS's Drag with drag1`, and performed by `oshost.doDrag` → `winBackend.drag`. It has been there
since before the guarded-press work, and it is unguarded.

And the tree already says so, in one line of `codegen.cannotPropagate`: **"a DRAG resolves no
offset, so it carries no window and is checked against nothing."** `Unpropagatable` therefore
refuses to SAVE a guarded recording that contains a drag, by name, before anybody presses Save. So
the present state is honest — a drag is either in an unguarded routine or it is refused — and the
work is to make the guarded form representable, not to invent a gesture.

#### What each layer has, and what it has not

| layer | reusable as it stands | missing |
|---|---|---|
| capture (`internal/recorder`) | the press edge already carries the whole window contract: `RelX`, `RelY`, `RelTo`, `WinW`, `WinH`, `Dpi`, `Process`, read at the press because a press is the only moment it can be read | the RELEASE edge carries no window at all, by design — `newClick` stamps geometry only when `down`. So the drop point exists only as an absolute coordinate, or as an offset computed from the PRESS's measurement |
| simplification (`internal/simplify`) | `StepDrag` is formed from the down/up pair when a move was seen and the distance passes `DragThresholdPx`; every field above is in hand at that moment, in `downRelX`, `downIn`, `downWindow` | it DISCARDS all of it. The step gets `X, Y, X2, Y2, Button` and nothing else — the comment beside it says so plainly: "Drags carry neither" |
| generation (`internal/codegen`) | a drag is already ONE `do` sentence, which is the property the propagation chain needs, and the exact reason non-left clicks and anchored presses cannot join it | `cannotPropagate` names "a drag"; `guardedStep` admits only `StepClick` and `StepMove`; the `Drag` set in `os.marco` has no window fields to carry a contract in |
| execution (`internal/oshost`) | `clickTarget` already resolves a Point against a freshly measured window and REFUSES with the term that disagreed; `delivered` already keeps a refused send as something that did not happen; `holding` / `released` / `ReleaseHeld` already sweep at route end and report what would not come up | `doDrag` uses none of them. It reads four ints, logs, and calls `ok(h.b.drag(…))` |
| the backend | `keyDown` / `keyUp` exist as separate primitives, which is what lets the ledger bracket a held key | there is **no `mouseDown` / `mouseUp`**. `drag` is one monolithic call, so no code outside it can release the button it pressed |
| fixture evidence | both edges are logged — `what` is the button plus `_down` or `_up` — with three coordinate spaces and the window rect each time | `WmMouseMove: break; // Deliberately not logged`. The movement BETWEEN the edges, which is the whole of what makes a drag a drag, is invisible to the only account outside both programs |

#### Three defects found by reading, not by running

1. **A cancelled drag still completes the drop.** `winBackend.drag` checks the context at the top
   of its interpolation loop and breaks — and then sends the landing move to the destination and
   the release anyway. So a stop during a drag releases the button (good) at the DESTINATION
   (wrong): the object is dropped where the routine was going rather than where the pointer was
   when somebody said stop.
2. **No entrance check.** Nothing tests the context before the press, so a drag dispatched after a
   stop has already been raised presses and drags regardless. `sendChord` needed exactly this check
   and has it ([[ADR-258-a-chord-is-a-promise-to-let-go]]); the drag path never got one.
3. **Every delivery failure is discarded.** The `send` closure inside `drag` ignores the result of
   `sendInputs` entirely and the function returns only the context's error, so a refused press, a
   refused move and a refused release are indistinguishable from a performed gesture. `doDrag`
   compounds it by using `ok(...)` rather than `delivered(...)`: a drag the machine refused is not
   in `Unperformed`, is not counted in `InputActions`, and reports `performed`.
   **One clause of that is wrong and is corrected in §41l**: a drag IS counted, because the count
   is taken in `Invoke` against `sendsInput` and not inside `delivered`, which is the whole reason
   it is taken there. `TestTypingAndDraggingAreAskedFor` is the test that says so, and it was
   written against exactly this mistake.

Also worth stating because it is the dangerous one: the button is in no ledger. `ReleaseHeld` knows
keys. An exit between the press and the release — a panic, a kill, a stop that unwinds past the
call — leaves the left button DOWN with nothing in the process that knows to lift it, and a held
left button drags whatever the pointer passes over.

#### The smallest contract that covers the five requirements

| requirement | the smallest mechanism | what it costs |
|---|---|---|
| fresh window validation before mouse-down | `simplify` stops discarding the press's contract; the `Drag` set carries it in the same field names `Point` already uses; `doDrag` resolves the From point through `clickTarget` and refuses with the term that disagreed. ONE measurement, taken before the press | nine fields on an existing set in `os.marco`, every name already in the language on `Point`. No Core change, no new act, no new word |
| the destination | resolved as an offset in the SAME measurement, not re-measured mid-gesture: a window that moved during the drag would otherwise move the destination under the pointer | the end offset is computed from the press's rect, since the release edge records none. A demonstration whose window moved mid-drag is ambiguous and is refused rather than guessed |
| cancellation during movement | release WHERE IT IS and stop: no landing move, no drop at the destination. The inter-step sleep becomes a select on the context so a stop lands within a step rather than at the next boundary, and an entrance check matches `sendChord`'s | one behaviour change to an existing primitive, and it changes what a stopped drag does today |
| release attempts on every exit | split the backend's `drag` into `mouseDown` → movement → `mouseUp`, and register the button in the existing ledger so the route-end sweep attempts it. Attempted, reported, not recovered, not forced — the holds' rule, unchanged | two new backend primitives and their stubs. This is the only genuinely new machinery in the proposal |
| delivery-failure propagation | `send` keeps what `sendInputs` returned; `doDrag` reports through `delivered`; the chain gains a `StepDrag` case in `propagating`, which needs nothing new because a drag is already one sentence; `cannotPropagate` loses its drag case last of all | the refusal channel already exists for recordings that cannot be represented — a mid-drag window move joins it |
| independent fixture evidence | log mouse movement **only while a button is down**, which is exactly drag motion and is bounded: a drag is about twenty-five moves. Keep the no-input rule — the fixture observes | a counted path with its ends, and whether each point was inside the window. Enough to tell a drag from two clicks, which is the part a replay can skip while both ends still look right |

**Preserve existing saved Marco.** The same decision as the holds: `the Drag is a set` keeps every
field it has, `do OS's Drag with …` keeps its meaning, and a routine already on disk keeps running
exactly as it does — unguarded, absolute, no window check. New recordings get the guarded form.
Explicit regeneration can pick it up; nothing rewrites anything.

**What no amount of this establishes** is unchanged: the hooks discard injected input, so no
automated run can show that Windows captured a physical drag. The fixture can show that a drag
ARRIVED as one.

### §41l — the three drag defects, fixed

§41k found them by reading and did not touch them. These three were the part of that trace that
stands alone: none of them needs the guarded-drag contract, none of them changes what a drag
means, and all three are about what happens on the way OUT of a gesture. The contract is still
proposed and still not agreed.

**They were the chord's three defects.** Not analogous ones — the same three, one device over, in
the same hiding place: the whole gesture sat inside the function that calls `SendInput`, where
nothing could arrange a refusal or a stop.

| the defect | what it did | what it does now |
|---|---|---|
| a stop during the travel | broke out of the interpolation loop and then sent the landing move and the release anyway, so the button was let go of AT THE DESTINATION: the thing being dragged was dropped where the routine was going, not where the pointer had got to | releases where the pointer IS. The gesture tracks the last point the machine accepted, and every exit releases there |
| no entrance check | a drag reached with the run already stopped pressed the button and dragged, discovering the cancellation at the top of the loop — after the press, which is the event that cannot be taken back | checked twice: in `dragGesture` before the aim, the way `sendChord` does, and in `doDrag`, where it holds for every backend and can be driven by a test |
| every delivery failure discarded | the send closure ignored what `sendInputs` returned and the function returned only the context's error; `doDrag` then reported through `ok` rather than `delivered`, so a refused drag was absent from `Unperformed` and a command reading that account read an empty one | each event's answer is kept, the gesture says WHICH event the machine would not take, and `doDrag` reports through `delivered` |

**And one clause of §41k's third defect was wrong.** It said a refused drag "is not counted in
`InputActions`". It is: the count is taken in `Invoke` against `sendsInput`, which is exactly where
that function's own comment says it is taken and why — an earlier plan to count inside `delivered`
would have read zero for a routine that typed or dragged, and
`TestTypingAndDraggingAreAskedFor` exists because of it. The test was already in the tree and the
trace contradicted it. Reading the function and not its test is how.

#### The shape of the fix is the chord's, deliberately

`internal/oshost/drag.go` now holds the order and the undertaking, against a `pointerMachine` that
takes one pointer event at a time; `winBackend.drag` is three lines and supplies the real one
(`thePointer`). Every event carries its own absolute point — which is how `clickAt` already sends a
click, and the property that makes "release where the pointer is" expressible at all. `withRelease`
serves both gestures now; the one word that differed, "keystroke", is a parameter.

A refused move ENDS the travel rather than carrying on. A gesture that did not go where it said it
went must not finish by dropping something somewhere nobody chose, so the button comes up at the
last point the pointer reached.

#### What is deliberately NOT fixed

**The button is in no ledger.** A key that goes down is written into an account the route-end sweep
reads; a button is not. So an exit between the press and the release — a panic, a kill, a stop that
unwinds past the call — leaves the button down with nothing in the process that knows to lift it,
and a held left button drags whatever the pointer passes over. Closing that needs the backend's
`drag` split into `mouseDown` → movement → `mouseUp` so something outside it can release, and the
ledger taught about a button. Both are in §41k's proposed contract, which is the Architect's to
agree. What is fixed here is every exit this function reaches.

The two limits are unchanged and are the chord's words: a failed release is **reported, not
recovered**, and **forced termination is outside all of it**.

#### Seven mutations, each killed

The project's standard, and two of them earned their keep:

- restoring the old break-and-fall-through → `TestAStoppedDragLetsGoWhereThePointerIs` and
  `TestAStoppedDragSaysBothWhatStoppedAndWhatIsStuck` fail.
- **the first mutation of that defect SURVIVED**, and it was the fix that was over-complete rather
  than the test that was weak: putting the `break` back in the loop alone changes nothing, because
  the check I had added after the loop catches it too. The faithful mutation removes both. Worth
  writing down — a surviving mutation is usually the test's fault and this one was not.
- aiming the cancelled release at the destination while leaving the stop's identity intact → the
  narrower mutation, and the one that proves the assertion is about WHERE rather than about
  whether. Fails `TestAStoppedDragLetsGoWhereThePointerIs` on the place.
- removing the gesture's entrance check → `TestADragDispatchedAfterAStopPressesNothing` fails,
  naming the three events a stopped run sent.
- removing `doDrag`'s → `TestAStoppedRunDragsNothing` fails on both halves: the status and the
  machine having been reached.
- discarding the release's answer → `TestADragTheMachineRefusesIsNotReportedAsDone` and the
  stop-and-stuck pair fail.
- shrugging off a refused move → **this one survived too, and that WAS the test's fault.** The fake
  machine refused everything from the nth event onwards, which refuses the release as well, and a
  failed release produces an error of its own — so the case about a refused move saw a failure and
  a released button whether or not the refused move was noticed. It names one event at a time now,
  and asserts the exact list of moves the travel asked for. The lesson is the oracle's:
  **a fake that fails broadly makes a narrow claim unfalsifiable.**
- putting `ok` back in `doDrag` → `TestADragTheMachineRefusedIsInTheAccount` fails with an empty
  account.

Restored from a scratchpad copy and verified byte-identical, not with `git checkout`.

#### Verification

Root suite clean. `gofmt` clean, `docscheck` no problems, `internal/wiringcheck` green (it is what
checks the test names this added to ADR-258 actually exist). `go vet ./internal/oshost` reports the
two pre-existing `unsafe.Pointer` lines in `clipboard_windows.go` and nothing new.

**No desktop run.** Nothing here needs one and nothing here would be established by one: the
provenance wall is unchanged, and all three fixes are about paths a desktop run does not take —
a stop mid-gesture, a machine that refuses input, a run that has already ended. `plugins/e2e` has
no drag scenario yet; that is part of the contract's fifth requirement (independent fixture
evidence of the full gesture), and the fixture still does not log movement.

### §41m — three findings against §41l, and the one that is really about seams

The Architect reviewed §41l by reading the source, having run nothing. All three findings are
real, and two of them are in the half that lifting the gesture out of `SendInput` does not reach.

#### [P1] The warning channel ended at the keyboard

`Host.NotReleased` is the account a cancelled run cannot discard: `performOnePlay` throws a stopped
run's error away on purpose, because a stop is not a failure, so the half that is STILL HAPPENING
is kept where the discarding cannot reach it. That is the channel the chord slice built, and
`noteHeld` looked for `notReleased` specifically — a stuck KEY.

So the drag's stuck button went out through `delivered`, the list of things that did NOT happen,
and nowhere else. A stop mid-drag recorded `cancelled` with nothing beside it while the button was
down. It is the chord's own defect reproduced for the other device, and the worse one: a held left
button drags whatever the pointer passes over, and there is no window in front of it saying so.

`noteHeld` asks for `stillHolding` now — an interface both `notReleased` and `buttonStuck`
implement. **The question is the shape of the thing and not the device**, and a third device cannot
be added without passing through it. `doDrag` feeds it, the same two calls a keystroke gets.
`ButtonNotReleased` is exported for the reason `KeyNotReleased` is: the words travel, and a test
at any layer asks for them from there rather than retyping them.

The record, the output and the Studio card needed **no change at all** — they take whatever
sentence the host gives and do not read it. That is worth noticing: the channel was built
generically and the only thing device-specific in it was the one `errors.As` target.

#### [P1] A cancellation during the aim still permitted the mouse-down

The entrance check sat at the top of the gesture, and the press came after a send. The aim IS a
send; a send takes time; a stop arriving while the machine accepted the aim found nothing between
it and the button going down. §41l's own words were "a stopped run presses nothing", and that was
true of the function and not of the moment.

The check is now immediately before the press as well. This is the same class as the chord's second
defect (the stop during the settle) and the same class as
[[tested-mechanism-untested-moment]] — **a tested mechanism is not a tested moment.** I wrote the
entrance check by copying `sendChord`'s and did not ask what happens between it and the thing it is
protecting.

#### [P2] The native adapter did not implement the contract the gesture was written to

The one that matters most, because it is about what the extraction proved.

`dragGesture` is written against a machine whose press and release each carry their own point —
that is the property the whole first fix rests on, since it is what lets a release be aimed at
where the pointer IS rather than at the destination. The Windows adapter supplied that point as
normalised absolute coordinates **on the button event, with no `MOUSEEVENTF_MOVE`**. Windows moves
the cursor for an event that carries that flag; coordinates on a `LEFTDOWN` without it are
coordinates on an event that is not a movement. So the press happened wherever the pointer already
was, and the gesture's contract read as satisfied by an adapter that was not repositioning
anything. My comment asserted the property in prose and the code did not have it.

Press and release are each the movement and then the button **in one `SendInput` call** now, which
is `clickAt`'s shape — that function had this right all along, and I read it while writing this and
still got it wrong. One call because separate calls let other input interleave between the
reposition and the button: a person's own mouse, or anything else injecting. Two events rather than
one event carrying both flags, because the order of the movement and the press WITHIN a single
`MOUSEINPUT` is not something the documentation gives, while the order of two events in a batch is
what a batch means.

The aim's own move is now partly redundant, since the press repeats it. Kept: an aim is a real
event an application may act on, a refused aim is a distinct outcome from a refused press, and a
second absolute move to the point the pointer is already at is not a gesture anybody can see.

#### The lesson, and it generalises past this slice

**Lifting a sequence out of the syscall makes the SEQUENCE reviewable and says nothing whatever
about the adapter underneath it.** §41l's verification section said "no desktop run — nothing here
needs one", and that was right about the three defects it fixed and wrong as a statement about the
slice: the headless tests could not have seen a missing flag, and I had not given them anything to
look at. `sendInputs` is a var now so what is actually handed to Windows can be read, and the flags
are asserted — including that a release after an intervening movement still carries the point it
was asked for, which is the claim the gesture actually depends on.

This belongs beside [[two-lists-one-fact]] and [[fix-did-the-visible-half]]: a seam that makes the
interesting half testable will happily leave the other half untested and looking fine.

#### Six more mutations, each killed

- `noteHeld` narrowed back to `notReleased` → the stuck-button account test fails.
- the `noteHeld` call removed from `doDrag` → the same test fails.
- the pre-press check removed → `TestADragStoppedWhileItAimsDoesNotPress` fails, naming the three
  events a stopped run sent.
- the `MOUSEEVENTF_MOVE` event dropped from press and release → the flags test fails on both.
- the batch split into two `SendInput` calls → fails on the call count.
- the release given a later point → fails on the coordinates, for both of its events.

Restored byte-identical from a scratchpad copy.

#### Verification

Root suite clean, `plugins/studio` ok (70s), `plugins/e2e` vet and tests ok, `gofmt` clean,
`docscheck` no problems, `wiringcheck` green. The flag tests are Windows-only by build tag and ran
on this machine. **Still no desktop acceptance run**, and the drag's own harness scenario does not
exist — that is the contract's fifth requirement, and the fixture still does not log movement.

**Closed at `9290a93`.** The Architect reviewed the source and accepted all three: stuck-button
warnings survive a cancellation, a cancellation after the aim prevents the mouse-down, and the
native button batches carry the positioning. Reviewed by reading, not by running the gates — which
is how all six findings in §41l and §41m were found, and worth keeping in the record beside them.

**The backend correction is closed. Three things are explicitly NOT**, and none of them is a defect
in what is now there:

- **guarded drags** — §41k's contract, PROPOSED and NOT AGREED. Nothing implemented.
- **the button ledger** — a button that goes down is in no list the route-end sweep reads, so a
  return that leaves the gesture by a path it does not own — a panic unwinding past it — leaves it
  down. The contract PROPOSES a ledger, and proposes splitting the backend's `drag` into
  `mouseDown` → movement → `mouseUp` so something outside it can release. **Both are proposed
  designs, not agreed requirements**, and the paragraph above said otherwise.
  **The warning channel is NOT this**: a release the machine refuses is reported now, everywhere a
  stuck key is. What is missing is an attempt by code that runs after an exit the gesture does not
  reach.

  **And a ledger would not recover from the runner being killed**, which this had widened past what
  the chord's own limit says. A ledger lives inside the runner: it reaches ordinary returns, and
  the unwinding where deferred code still runs, and nothing in a terminated process runs at all.
  That needs an owner which OUTLIVES the runner, and it is outside this scope — not a thing a
  ledger is being deferred on. The sentence that has been right since the chord is the one to keep:
  a release attempt **by code that gets to run**. §41l's "a panic, a kill, a stop that unwinds past
  the call" is left as written; this is the correction to it.
- **desktop acceptance** — there is no drag scenario in `plugins/e2e`, and the fixture still does
  not log mouse movement (`WmMouseMove: break`). That is the contract's fifth requirement, and
  until it exists no run outside this process has seen a drag arrive as one.

### §41n — the guarded-drag contract, proposed against the source

**Nothing in this section is implemented.** It supersedes §41k's contract, which was written before
the backend correction and is wrong in three places; those are named below.

**The hold gate is not pending.** `-only hold` ran 3 of 3 exit 0 on 2026-10-26, the
`onlyWhenAsked` lines are gone from `scenario_play.go`, and `plugins/e2e/hold_test.go:238` now
FAILS if one reappears — the gate test was inverted when they were ungated, so the recorded status
is enforced rather than remembered. First run's artifacts are at
`C:\Users\chris\AppData\Local\marco-e2e\20261026-155330\artifacts`. No rerun.

#### 1. Window-relative start and destination

**The release edge records no geometry at all.** `newClick` stamps `RelTo`, the shape and the
offset only `if down` (`recorder_windows.go:614`). So today the destination exists only as the
release's absolute coordinate, and §41k proposed deriving its offset from the PRESS's rectangle.

**That is an unchecked fallback and is rejected.** An offset is a measurement of a rectangle; the
press's rectangle was measured at the press. Subtracting it from a point that arrived later
produces a number that is an offset into the window *as it was*, attributed to a point captured
*after*, with nothing able to tell whether the window moved in between. It would pass every check
at replay and localise the drop against a rectangle that had gone.

**So both edges are measured, and a gesture that spans two rectangles is refused.**

- `newClick` stamps the release edge the same way it stamps the press: `RelTo`, `WinW/WinH/Dpi`,
  the containment test, `WinRel`. The attribution (`clickAttribution`) already runs on both edges,
  so the delta is `clickShape(hwnd)` — `GetWindowRect` + `GetDpiForWindow`, already judged
  hook-safe for every press. **This roughly doubles the per-mouse-event hook cost and has to be
  measured, not asserted** (the pump invariant's own note: a correctness rule is not a measured
  win, and the reverse holds too).
- `RecordedEvent` gains `WinX, WinY` — the origin `clickShape` already returns and currently
  discards. Both edges carry it. **It is not emitted into the route**: `simplify` uses it to decide
  and throws it away, so the durable artefact keeps offsets only. That is the
  no-absolute-identity rule, and it means the origin is demonstration evidence rather than a
  replay target.
- `simplify` then requires, for a `StepDrag` to carry the contract: both edges stamped, the same
  `RelTo`, the same `WinX/WinY/WinW/WinH/Dpi/Process`, both `WinRel`, and both offsets inside the
  rectangle. **Identical rectangles, not merely identical sizes** — a window that moved mid-drag
  gives two consistent offsets that describe a gesture no static window can reproduce, and
  "the window moved while this was being demonstrated" is a sentence a person can act on.
- Anything short of that: the step keeps `RelTo` and not the measurements, which is the third saved
  contract `measured()` already documents (`codegen.go:321`) — asked to be checked, cannot be, and
  refused rather than run.

**The `Drag` set gains nine fields** (`internal/osmod/os.marco:57`), every one a scalar:
`FromRelX, FromRelY, ToRelX, ToRelY, RelTo, WinW, WinH, Dpi, Process`. Five names already exist on
`Point`; **four are new** — §41k said "every name already on `Point`", which was wrong. The
asymmetry is deliberate: reusing `Point`'s bare `RelX/RelY` for the start beside `FromX/FromY`
would leave a reader asking which end it belongs to.

No Core change and no new act. Nesting a `Point` inside `Drag` is a language change (set fields are
scalars — [[inline-set-to-foreign-act]]) and a second argument does not exist (`do X's Act with Y`
takes one). `os.marco` is the host module rather than Core, and `Point`'s own window fields were
added there for [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] without reopening the
language — same precedent, and worth the Architect confirming rather than my assuming.

#### 2. Fresh validation immediately before mouse-down

**`checkGuarded` already is this check**, in full: something bound, the handle's owner known, the
window still in front, it reports its size, it reports its DPI, the handle's owner unchanged since
preparation, the program's image name matches, the size exact, the DPI exact, and the recorded
offset inside the recorded rectangle (`oshost.go` ~2060–2145, `measuredFrom` ~1800–1865). Every
disagreement names the term. No tolerance anywhere, by decision.

What is missing is that it resolves ONE offset. The smallest change:

- split it into the validation (returns the live rectangle or the refusal) and the arithmetic;
- a click resolves one offset from that rectangle, a drag resolves two;
- **one measurement, not two.** Re-measuring for the destination would let a window that moved
  during the travel relocate the drop under the pointer, which is the hazard the guard exists for
  wearing the guard's own clothes.
- `doDrag` resolves both points BEFORE entering the gesture, and refuses with the disagreeing
  term having sent nothing. That ordering is what makes "resize refusal before mouse-down" a claim
  the harness can assert on an empty event log.

#### 3. Cancellation, release point, delivery failures

**Done, in `9290a93`.** A stop during the travel releases at the last point the machine accepted; a
refused move ends the travel and releases there; the entrance check is read before the aim and
again immediately before the press; every refusal `SendInput` reports is kept; `doDrag` reports
through `delivered` and feeds `noteHeld`. The native adapter sends the movement and the button in
one `SendInput` call.

What remains for "delivery failures stopping subsequent actions" is the chain, and it needs no new
machinery: a drag is already ONE `do` sentence, which is the property `cannotPropagate` requires
and the reason non-left clicks and anchored presses cannot join. So `guardedStep` admits
`StepDrag`, `propagating` gains a case returning `OS's Drag with dragN`, and `cannotPropagate`
drops its drag case — **in that order and last of all** (see increments).

Free, and worth naming so it is not re-derived: `anyGuarded` (`simplify.go:840`) keys on
`RelTo != ""`, so the moment a drag carries the basis it is already kept out of loop folding, and
`guardsAnything` already turns the route into a chain on the same test.

#### 4. Cleanup on returns and panic unwinding — **neither a ledger nor a split is necessary**

The question the Architect left open, answered from the code:

- **Ordinary returns** are covered now. `dragGesture` attempts a release on every exit it reaches.
- **Panic unwinding** is covered by a `defer` **inside `dragGesture`**, guarded by a local flag so
  it cannot double-release. Nothing else is required: the defer runs during the unwind whoever
  recovers above it, and the gesture is synchronous, so there is no abandoned goroutine to chase.
- **A ledger plus a split backend buys exactly one thing**: the route-end sweep's SECOND attempt
  after a first refusal, which is what `doKeyUp` keeps a refused key on the list for. That is worth
  having and it is not this contract's minimum.
- **But there is one place a slot of host state is genuinely needed**, and it is about reporting
  rather than releasing. A panic skips `doDrag`'s return, so `h.noteHeld(what, err)` never runs —
  the deferred release ATTEMPT happens and its failure has nowhere to go. So: the host writes the
  sentence it would say *before* entering the gesture and clears it on a clean release. One slot,
  no list, no sweep, no new backend primitive. That is the smallest thing that makes "a button may
  still be down" survive a panic.

**Forced termination stays excluded, and the boundary is the one corrected in `99d8bb3`**: a ledger
lives inside the runner, so it reaches ordinary returns and the unwinding where deferred code runs,
and nothing in a terminated process runs at all. Recovering from a kill needs an owner that
outlives the runner. The guarantee is a release attempt by code that gets to run.

#### 5. Compatibility

- **Existing saved Marco is unchanged.** A saved `the drag1 is a Drag with FromX … Button "left"`
  has no `RelTo`, so `doDrag` takes the legacy path exactly as it does today: four ints, absolute,
  no window check. Same three-contract structure as `Point`, and the same decision as the holds.
  Nothing rewrites anything; explicit regeneration can pick the guarded form up.
- **The review/Save refusal** is `codegen.Unpropagatable`, called by `plugins/studio/recording.go:519`
  before the button and again by the generator at `codegen.go:106`. `cannotPropagate` is a pure
  function of the step, so after the change it refuses by the step's own contents: a drag that
  carries the full contract is allowed, a drag that asked and could not be measured is refused
  naming what is missing, and a drag whose window changed mid-gesture is refused naming that. Three
  sentences where there is now one ("a drag").
- **A pre-existing gap worth checking rather than fixing here**: `Unpropagatable` iterates the top
  level only, while `guardsAnything` recurses. A drag inside a loop body would slip past it today —
  unreachable, because `anyGuarded` turns folding off for a guarded recording, and that is the only
  thing standing between them.

#### 6. Automated acceptance — four scenarios, all SEEDED

The fixture needs one change: **log mouse movement while a button is down**
(`Fixture.cs:637`, `case WmMouseMove: break;`). Only while a button is down, which is exactly drag
motion and is bounded at about twenty-five points; the no-input rule is untouched, the fixture
observes. `Geometry(b)` already stamps every record with the window as it was at that instant, so
each scenario reads the window from the fixture's own account rather than asking afterwards.

| scenario | the claim | asserted on |
|---|---|---|
| `drag/a drag arrives as a drag` | down at the start, movement between, up at the destination, all inside the window, the ends matching the demonstrated offsets re-based onto the live window | the received log: one `*_down`, N>1 moves, one `*_up`, their three coordinate spaces, and `mods`/`watched` showing nothing held |
| `drag/moved window still gets the drag` | the window moves and BOTH ends re-base | the same log, against the moved rectangle — `theWindowMoved` is the template, `place` + `until` already exist |
| `drag/resized window is refused with nothing sent` | the window resizes and the routine refuses **before mouse-down** | the received log is EMPTY — no down, no move, no up. The existing resize scenario asserts emptiness the same way and for the same reason: a state file cannot tell a refusal from a press four pixels wide |
| `drag/a stopped drag lets go where it got to` | stopped once the fixture has LOGGED the down, the up arrives at the last point the pointer reached and not at the destination | the log's own ordering, and the run record reading `cancelled` with the stuck-button sentence beside it when the release is refused |

The fourth is the desktop half of the first defect and is the one that could not exist before the
fixture logged movement: without it there is no "last point the pointer reached" outside Marco's
own account, and a verdict resting on Marco's account would be Marco marking its own work.

**Physical recording capture stays separate and is NOT exercised by any of these.** All four seed a
demonstration as recorder events and drive the production simplifier, generator, registry and host;
the hooks discard injected input (`llkhfInjected`/`llmhfInjected`), so no automated run can show
Windows captured a physical drag. Every report ends with `physical recording capture: NOT
EXERCISED` and these add nothing to that line. What they establish is that a drag ARRIVED as one.

#### Reusable, and missing

**Reusable without change:** `dragGesture` and `thePointer`; `delivered`, `Unperformed`,
`noteHeld`, `NotReleased`, `stillHolding`; `wholeTerm`, `farthestPixel`, `supportedBasis`;
`measuredFrom`'s stated/malformed/usable reading and its sentences; `clickShape`; the chain —
`chainOf`, `link`, `guardsAnything`, `anyGuarded`, `keyByKey`; `Unpropagatable` as the review
surface; e2e `setUpSeeded`, `place`, `until`, `waitForPress`, `waitForRecord`, `countRecords`, and
the moved/resized pair as templates; the fixture's `Geometry`, `ModifiersNow`, `WatchedNow` and its
message filter.

**Reusable after a split:** `checkGuarded` — the validation is the whole requirement and only the
arithmetic is single-offset.

**Missing:** release-edge geometry and `WinX/WinY` in the recorder; the both-edges rule in
`simplify`; nine fields in `os.marco`; emission, `guardedStep`, `propagating` and a refined
`cannotPropagate` in `codegen`; the two-offset resolve and the pre-gesture validation in `oshost`;
the deferred release and the one host slot; movement-while-down in the fixture; four scenarios.

#### Bounded increments

Ordered so that **nothing can save or run a partly-guarded drag at any point**, which is the
property that makes them safe to land one at a time. Increment 6 is the only one that changes what
a person can save, and everything it depends on is tested before it.

| # | increment | gate |
|---|---|---|
| 0 | split `checkGuarded` into validation and arithmetic, no behaviour change | the existing `guardedpress_test.go` mutations must still all die |
| 1 | recorder: release-edge geometry, `WinX/WinY` | recorder tests; **and a hook-cost measurement**, stated as a number |
| 2 | `simplify`: carry the contract on `StepDrag`, require both edges, refuse a mid-gesture change | simplify tests. Still unsavable — `cannotPropagate` refuses "a drag", so nothing runs |
| 3 | `os.marco` fields + codegen emission, `cannotPropagate` untouched | `spectest`, codegen tests; the generator testable while nothing is runnable |
| 4 | `oshost`: two-offset guarded resolve, `doDrag` validates before mouse-down | oshost tests, each refusal term mutated |
| 5 | the deferred release and the host slot that survives a panic | a panic injected mid-travel; the sentence must arrive |
| 6 | **the switch**: `guardedStep` admits `StepDrag`, `propagating` gains its case, `cannotPropagate` drops the drag and gains two sentences | end-to-end through the production save path; `Unpropagatable`'s three outcomes |
| 7 | fixture movement-while-down + `build.ps1` | the fixture's own tests; the log bounded on an ordinary click |
| 8 | the four scenarios | a desktop gate, when the screen is available. First result preserved; no rerun to obtain green |

#### What §41k got wrong

1. "every name already on `Point`" — four of the nine field names are new.
2. "is not counted in `InputActions`" — already corrected in §41l; a drag is counted in `Invoke`.
3. The destination resolved from the press's measurement. That is the unchecked fallback this
   contract refuses; both edges are measured instead, and a gesture spanning two rectangles is
   refused at Save.

And it proposed the split backend and the ledger as what the work "needs". Neither is necessary for
this scope — a `defer` inside the gesture plus one slot of host state covers returns and panic
unwinding. The ledger remains available as later work for the sweep's retry.

### §41o — guarded drags, built in eight increments

The Architect approved §41n's approach with four corrections, and all four landed. The contract is
in [[ADR-259-a-guarded-drag-is-two-points-in-one-window]]; this is the trace.

**The hold gate was not pending** and was not re-run: it is recorded green, and
`plugins/e2e/hold_test.go` now FAILS if one of those three is re-gated, so the status is enforced
rather than remembered.

#### The four corrections, and what each changed

1. **Validate at mouse-down, after aiming.** §41n put the validation in `doDrag`, before the
   gesture. The gesture then AIMS — an injected pointer move, which changes applications: a hover,
   a highlight, a panel that lays itself out differently, a window that resizes to fit what is now
   under the cursor. A check before the gesture started is a check before its own first event. So
   `dragGesture` takes a `stillThere` and asks it after the aim and immediately before the press,
   with **no input sent in between**, and the answer is compared against the SNAPSHOT the two
   points were computed for — not merely against the recorded terms, because `confirmWindow`
   compares the live window with what was DEMONSTRATED and nothing was demonstrated about where
   the window would be. A change refuses; it does not re-base.
2. **The same recording window, not matching rectangles and names.** Two windows of one executable
   at one size on one display satisfy every comparison of geometry, DPI and program name and are
   two different windows. `RecordedEvent` carries `HWnd` and `WinX/WinY` now, both recording
   evidence that never reaches a route, and `simplify` asks identity BEFORE the describing terms.
3. **Track accepted button-down, not entry.** §41n proposed one slot written before the gesture and
   cleared on a clean release. That reports a stuck button after a panic during the aim, when
   nothing was pressed — and a warning that fires when nothing happened teaches people to ignore
   the warning. Four states now (`noDown`, `buttonDown`, `letGo`, `buttonStuckDown`), armed on an
   ACCEPTED press, with one release path every exit goes through including the deferred one.
4. **Stay inside the fixture's evidence.** No count of movement, because Windows coalesces moves
   and a number is an assertion about the scheduler. A stop after the release is NOT RUN. No stuck
   button manufactured on a desktop.

#### Eight increments, in an order with a property

Nothing could save or run a partly-guarded drag at any point. Increment 6 is the only one that
changed what a person can save, and everything it depends on was tested before it landed.

| # | commit | what |
|---|---|---|
| 0 | `0e7c68c` | split `checkGuarded` into `confirmWindow` + the arithmetic, no behaviour change |
| 1 | `e1e2436` | both edges of a click measured; `HWnd`, `WinX/WinY`; hook cost measured |
| 2 | `ab5fc80` | `simplify`'s six outcomes, and `ToRelX/ToRelY` on the step |
| 3 | `7a74ba8` | nine fields on the `Drag` set; emission; Save still refused, and that was asserted |
| 4 | `2da5cc8` | two ends from one measurement, and the revalidation at mouse-down |
| 5 | `7321d45` | the deferred release, armed on an accepted press |
| 6 | `900424b` | **the switch**: `guardedStep`, `propagating`, `cannotPropagate`, and Studio's review |
| 7 | `71a3621` | the fixture logs movement while a button is down |
| 8 | `91b0643` | four scenarios, gated, NEVER RUN |

#### What mutation testing found that reading did not

Thirty-odd mutations across the eight. Six survived first, and only two of those were because the
fix was over-complete; the other four were real gaps:

- **The identity comparison had no case at all** (increment 2). The Architect's own finding, and my
  first test did not cover it: every case I had written changed the geometry, so dropping the
  handle comparison changed nothing. A second window of the same program at the same place is the
  case, and it is now in the table.
- **The unmeasurable branch was unreachable** (increment 2). Every fixture that got near it tripped
  an earlier case. The case that reaches it is a drag ending OUTSIDE the window — which turned out
  to deserve a sentence of its own, because it is something the person did.
- **The shape comparison had no case** (increment 2): a window resized between the edges, with the
  origin unchanged. That is the whole reason size and scale are separate terms.
- **The gesture's own revalidation call was uncovered** (increment 4), because the recording
  backend answers the callback itself. The tests that drive the PRODUCTION gesture were missing.
- **"The drag is in the route" cannot tell a LINK from an ordinary step inside a chain** (increment
  6). Same words, same gesture, same reading to a person — and everything after it runs whether the
  gesture happened or not. A link's guard is `do … ...` followed by `when ok?`; an ordinary step
  ends in a full stop. That is the only thing worth asserting there.
- **And `go test` CACHES past a change to a file outside the module being tested** (increment 7).
  Three mutations of the fixture's C# survived because of it: the gate was deleted, the suite ran
  again, and it said `ok (cached)`. A test that goes green against a broken fixture on every run
  but the first is worse than no test, because the one run that would catch it is the one nobody
  repeats. `plugins/fixture` is a Go package now whose only job is to `//go:embed` the C# as a
  BUILD INPUT, and the rule's test lives beside it.

One mutation was **equivalent** and is recorded in the code rather than fixed: replacing `up.rx`
with `s.X2 - down.x` — deriving the destination from the press's rectangle — passes everything,
because once the two edges must report the same origin the expressions are identical. So the second
measurement earns its place as **the check and not the arithmetic**, and the mutation that proves
it is the origin comparison.

#### And one test of my own failed on something real

`TestTheDragScenariosAreGatedUntilTheyHaveRun` asserts the scenarios are not under another
family's `-only` selector. My first names were `drag/moved window still gets the drag` and
`drag/resized window is refused with nothing sent` — both contain "window", which is the guarded
press's regression gate, so a flag typed for a press change would have swept them in. Renamed, and
the test asks about **every registered family** now, read off the scenarios rather than listed: a
list would have been written against the collisions that existed the day it was written. That is
the pump invariant's lesson in a third place.

#### Verification

Root suite clean (`go test ./...`), `plugins/studio` ok, `plugins/e2e` ok, `plugins/fixture` ok,
`gofmt` clean, `docscheck` no problems, `wiringcheck` green — which is what checks that every test
ADR-259 cites exists.

**No desktop run, and the screen was not touched.** The four scenarios are written, gated, and
have established nothing. The standing position is that the screen is the owner's unless they say
otherwise; `-only drag` is the gate when it is available, the first result and its artifacts are to
be preserved, and failures are not to be re-run for green.

`physical recording capture: NOT EXERCISED` is unchanged and cannot change here — the hooks discard
injected input, so every scenario seeds a demonstration and plays it. What they will establish is
that a drag ARRIVED as one.

#### Still open

- **The button ledger**, which would buy the route-end sweep's SECOND attempt after a refusal.
  Not needed by this contract and not a defect in the gesture.
- **Non-left clicks and anchored presses** still cannot propagate, for their own reasons.
- The parked items from before: the `E2E.md` and ADR-252 double-encoding, the harness's
  unexplained foreground refusal (§40b), `stopsignal.Raise`'s unretried *Access is denied*.

### §41p — three findings against §41o: one instant, one order, two claims

Review of the slice above found three, and the shape of all three is the same: a check or a claim
that reads as stronger than what it establishes.

#### A stop arriving while the window was being read still pressed

`dragGesture` read the context, aimed, read it again, asked the window, and pressed. The second
read was the §41m fix for the gap the aim opens — and it closed that gap and opened the next one.
The confirmation sends nothing, which is why it was allowed to sit between a stop check and the
press at all; but it reads a rectangle, a scale and a process name off a live desktop, and a stop
arriving while it did that found nothing between its own answer and the button going down. The
window was still there, the callback said so, and the press went out on a run that had already
been stopped.

So there are three reads now — the stop, the window, the stop again — and the rule they prove is
the general one this keeps teaching in a new place: **a check belongs against the press, so
whatever is last before the press is what has to be read there**, including a check that was
itself the fix for the instant before. The one before the window stays, and has its own test:
somebody who presses Escape is owed "it was stopped" and not "that window changed size".

Both are pinned, and each by one mutation: deleting the last read presses after a confirmation
that cancelled and agreed, and deleting the middle one asks a desktop about a run that is over.

#### An outcome word is not a timestamp

`theDragHadAlreadyFinished` decided whether the stopped-drag scenario had exercised anything from
the record's outcome and the existence of movement. Neither orders the stop against the release,
and read as a timestamp the outcome fails in BOTH directions:

- a stop Marco **ignored** finishes as `performed`, which the check excused as "too late" and
  reported NOT RUN — filing the most serious thing the scenario can find as nothing happened;
- a stop that landed **after** a completed gesture still records `cancelled`, because the run was
  stopped between steps — which convicted a drag nothing interrupted of dropping at its
  destination.

The ordering evidence is the fixture's own counter, read on both sides of the Stop request. An
event at or below the first reading was logged before the request; one above the second was logged
after it; what falls between arrived DURING the request and nothing orders it, so that is reported
as ambiguous — not a pass, not a failure, and the sentence says which two things it cannot put in
order. An ignored stop is now a finding with its own sentence, and a log with no release at all
returns to `theButtonWasNotReleased` rather than becoming NOT RUN: that one survived the first
mutation run, and it is the same mistake one claim over.

Whether the SCENARIO takes that evidence, and in that order, is a property of a function body only
a desktop runs — so it is read off the source the way the hold scenario's absence claim is. A
bracket taken at the top of the scenario bounds nothing, and an upper bound that is never read is
not a bound: `TestTheStoppedDragScenarioBracketsItsStop` kills both, and the three verdicts have to
come after the ordering.

#### "A move arrived" and "not the destination" are both weaker than they read

The travel claim counted movement. A replay that jumped straight to the far end and let go there
sends a move of its own, at the destination, and satisfied it — so the check that existed to tell a
drag from a teleport passed the teleport. What is required is a received point that is **neither
end**, which no count can stand in for.

And that log has a second reading the oracle cannot separate from a teleport: an application that
never got the processor during the travel and was told the pointer's position once, at the end. The
failing sentence names both, because one of them sends somebody to the machine and not to Marco.

The release claim asserted the button did not come up at the destination, which passes a release at
a point the pointer never visited — a corner of the box the ends make, the origin, whatever a
mistaken implementation aimed at. "It let go somewhere else" is not "it let go where it got to", so
the release also has to be ON the travel: inside the demonstrated ends and on the line between
them, to within the band the gesture's own integer stepping produces and no wider. What the oracle
CANNOT say is which point of the travel the pointer had reached when the stop landed — coalescing
means the log does not name the pointer's last position, and the instant a stop arrives is not
observable from outside the run — so the passing sentence claims an order and a place, and not an
instant.

#### Verification

Fifteen mutations of the claim code, one survivor, fixed and re-killed; six of the scenario's
wiring, all killed; two of the gesture's new reads, both killed. Root suite clean, `plugins/e2e`
ok, `gofmt` clean, `docscheck` no problems, `wiringcheck` green.

**Still no desktop run and the screen was not touched.** The four scenarios remain gated and have
established nothing; `physical recording capture: NOT EXERCISED` is unchanged.

### §41q — the order of the claims was itself a claim

One finding against §41p, and it is the third of this shape in the slice: a defect sitting behind
its own excuse.

The stopped-drag scenario asked the two NOT RUN exemptions first — the stop's place in the log, and
whether the pointer got anywhere — and then the verdicts, the missing release among them. Each
claim had its own test. The composition had none, and the composition is where the defect was: a
log with a press and nothing else has no movement in it, so the scope exemption answered first,
filed the run as NOT RUN, and nothing ever asked whether the button came up. **A left button held
down on somebody's desktop is the worst thing this scenario can find, and it was the one thing it
could not report.**

So the held button is asked FIRST, above both exemptions and not merely above the travel one: an
ordering the bracket cannot settle would have swallowed it exactly the same way, and that case is
one line over from the one review named. For it to be asked there, `theButtonWasNotReleased` had to
become silent about a log with no press in it — nothing went down, so nothing is down, and the
sentence it carries is about a button that is held. The log with no press is already NOT RUN, from
the ordering claim, which is where it belongs.

The whole decision now lives in `theStoppedDragsVerdict`, which returns the sentence and whether it
is NOT RUN, in four steps: the held button, the ordering, the scope, then the verdicts. It is one
function so that the ORDER can be tested — which is the real repair, since the pieces were never
what was wrong. The scenario keeps only the mapping from that answer to a disposition, and the
source test now reads both halves of it: a scenario that called only `fail` would turn every NOT
RUN into a failure, and one that called only `skip` would turn a held button into nothing happened.

#### Verification

Ten mutations, all killed. Six of the composed decision — the held button asked after the
exemptions (which reproduces exactly the reported defect: "a press and nothing else" comes back as
NOT RUN for the travel claim), not asked at all, its precondition removed, the two exemptions
swapped, and each disposition flipped. Four of the scenario's wiring, read off the source: each
disposition replaced by the other, the decision not asked, and the bracket's upper bound taken
before Stop.

Two of those mutations had to be rewritten to compile — deleting a guard left a variable unused —
and one first run was a **false survivor**: the perl pattern matched the same five lines in an
earlier scenario in the same file, so the file changed, the target did not, and the test was
right to pass. The trap is the one this project keeps hitting: confirming a mutation applied is
not confirming it applied *where it was aimed*.

`plugins/e2e` ok, `gofmt` clean, `docscheck` no problems, `wiringcheck` green.

**Still no desktop run and the screen was not touched.** The four scenarios remain gated and have
established nothing; `physical recording capture: NOT EXERCISED` is unchanged.

### §41r — the drag gate ran: one product defect, two harness defects, and a green fourth run

The screen became available and `plugins\e2e\run.cmd -only drag` ran for the first time. **It took
four runs to get a result, and not one of the three findings was in the drag path.** What that
supports is that these runs found no defect in the drag path: every claim and every mutation test
came through them unchanged. It is not the broader statement that the drag path is correct, which
four runs of four scenarios do not establish. What stopped them was everything around the claims.

#### Run one: 0 of 4, and the defect was in a reader

All four failed on the seeded routine's own precondition: *the seeded routine came out legacy
rather than guarded*. The routine was guarded — the artifact holds the file, with `RelTo "window"`,
both ends' offsets, the size, the scale and the program. What was legacy was the READING of it.

`orchestrator.ContractOf` recognised one shape, `is a Point with`, because when ADR-251 wrote it a
guarded action was a press. A guarded drag matched nothing, so the route read back as **legacy** —
*its presses are not checked; each one happens at the point it was recorded at, whatever is there
now.* Studio prints that on the card somebody reads before pressing Play, and `cmd/marco` records
it with a stopped run. **The one routine that does check its window was described to a person as
the kind that cannot.**

The fix is two lists and a boundary. A drag's terms are not a press's — `internal/oshost` asks for
`FromRelX`/`FromRelY` and `ToRelX`/`ToRelY`, each pair required — so the SHAPE chooses the list,
because "is this complete" has no answer until something says what complete would be. And a term
is matched as the term after a `with ` or a `, `, not as a name followed by a space anywhere in the
line: **`FromRelX 193` ends in `RelX 193`**, so the old check would have read a drag measured at
one end only as carrying both of a press's offsets — ready for the guard to refuse, which is the
dangerous direction. The same oversight had Studio's `saidTerms` showing a person `FromRelX` off
their own card; its list is read back from `ContractOf` now rather than written beside it.

Five mutations, all killed. The first reproduces the desktop failure exactly.

#### Runs two and three: 3 of 3 and a NOT RUN, twice, for two different reasons

The three ordinary scenarios passed on run two and have passed on every run since. The stopped
drag came out **NOT RUN** — the ambiguity reported by §41p's bracket, on its first desktop run,
and right both times:

> the release is #27 and the log stood at #13 when Stop was requested and #27 once it had been, so
> it arrived while the stop was being requested and nothing here orders the two

**A drag's travel is 24 steps of 8ms: about 190ms, fixed however far the drag goes.** Everything
the scenario does while the gesture is in flight has to fit inside that, and twice it did not.

- Noticing the press cost a **100ms poll tick** — half the gesture spent before the stop was asked
  for. `waitForMouse` asks every 5ms now, which is affordable because of what the condition reads:
  the fixture's log is a file it appends to and this reads with one `os.ReadFile`, so asking more
  often costs the application nothing and cannot perturb the gesture. Waits on windows and
  processes keep the 100ms cadence. Run three: the lower bound moved from #13 to #4.
- Pressing a button costs **longer than the gesture**. `uia.press` takes a fresh
  four-thousand-node snapshot of Studio's window before invoking anything — deliberately fresh,
  because a UIA runtime id goes stale on a repaint and that page repaints on a poll. Run three
  measured the request at **277ms against a 190ms travel**, so the drag was over before the button
  was invoked and nothing could order the two.

So the stopped drag **raises** the stop instead of pressing it, and that is recorded where it
happens. It is the same stop — `Acts.Cancel` is `stopsignal.Raise(stopsignal.Home())` and nothing
else, F9 and `director stop` raise the same generation file, `marco do` watches it. What it stops
exercising is Studio's Stop BUTTON, which is the whole reason this harness presses buttons rather
than calling backends — so it is not lost quietly: `hold/a stopped run lets go of the key` presses
the button and a hold can afford the snapshot. The raise also checks the generation advanced,
because a raise into the wrong home would leave the run finishing normally and be reported as
Marco ignoring a stop — a finding about the product manufactured by the arrangement.

#### Run four: 4 of 4, exit 0

| | |
|---|---|
| `drag/a drag arrives as a drag` | 26 events: a press, **24 movements with the button held**, and a release |
| `drag/it follows an app that moved` | the window moved +240,+120 and the drag went 553,292 → 498,492 inside it |
| `drag/a resized app is refused with nothing sent` | 626x469 → 716x529, stopped as `failed`, **nothing sent at all** |
| `drag/a stopped drag lets go where it got to` | the button came up at **295,238** — on the travel toward 258,372 and not at it |

The bracket on that last one was #5 before and #6 after, and the request took **10ms**. The stop
is ordered before the release by the fixture's own counter, which is what §41p built.

**What was NOT changed to get them green:** the bracket, the ordering rule, and every claim.
Making the harness quick enough to land a stop inside a gesture is not the same act as widening
what counts as having landed one — and an unorderable result is still NOT RUN.

The gate is off: `onlyWhenAsked` is gone from all four, and
`TestTheDragScenariosAreGatedUntilTheyHaveRun` became `TestTheDragScenariosRunLikeAnyOther`,
asserting the inverse, in the commit where they passed.

#### Verification

Root suite green (111 packages); `plugins/e2e`, `plugins/studio`, `plugins/fixture` ok; `gofmt`
clean; `docscheck` no problems. Mutations: five on the contract reader, one on the poll cadence
(*it took 102ms to notice an event written 10ms in*), all killed.

**Physical recording capture is still NOT EXERCISED** and still cannot be. Every one of the four
green scenarios played a SEEDED demonstration; what they establish is that a drag ARRIVED as one.
A person demonstrating in Studio remains the only evidence that Windows captured a physical
gesture.

Preserved artifacts: `20261027-050109` (0 of 4), `20261027-051215` and `20261027-051618` (3 of 3,
1 not run), `20261027-052006` (4 of 4) under `%LOCALAPPDATA%\marco-e2e`.

#### What the review ruled, and two things this section had to correct about itself

Approved at `38d90f1`: the contract reader recognises guarded drags and checks both endpoints'
terms, and the direct stop-signal exception is accepted for what this scenario tests —
cancellation DURING drag playback — on the condition that the distinction stays explicit, which
is where it lives: `studioRun.stopNow`, [[Desktop-Harness]], and ADR-259's decision line all say
that Studio's Stop button is exercised by the hold scenario and not by this one. Source reviewed,
harness not run, physical recording capture still NOT EXERCISED.

Two corrections to the reporting, both in this section's own first draft, and both the same kind
of mistake — a number and a claim that read better than the evidence:

- **It was FOUR runs, not three.** The 0-of-4 that found the contract reader is a run. A count of
  the runs that found something is not a count of the runs it took.
- **"The drag path was correct" is broader than four runs establish.** What they support is that
  these runs found no defect in the drag path: every claim and every mutation test came through
  them unchanged. A gate passing is evidence about the gate's questions, and the smallest version
  of a finding is the one worth writing down.

### §41s — a press is a button and a place

The relay asked for guarded right- and middle-click support through Studio's existing recording
path, with one instruction that decided the shape of it: **inspect the smallest representation
that carries the button and target together in one guarded operation, and do not generate a
guarded aim followed by an unchecked click at the cursor.**

#### The refusal described the emission, not the press

A right-click could not be saved in a routine whose presses are checked, and the entry in
`cannotPropagate` said why: *an aim and then a press. Wrapping the aim leaves the press unwrapped,
and the press is the thing that acts.* True of two generated sentences — `do OS's Move with p1`
then `do OS's Click with "right"`, which presses wherever the cursor has got to — and never true
of a right-click. **Everything else was already there.** `newClick` takes the button as a
parameter and measures the window the same way for all three; `simplify` has carried `Button` on a
click step since before any of this; `clickAt(ctx, button, x, y)` has always taken a button; and
the guard is about a Point and knows nothing about buttons. The missing piece was somewhere in the
saved file to put it.

So: `Point` gains `Button`, beside the coordinates, written only when it is not left.

- **Beside the coordinates and not among the window terms**, because nothing about it is compared
  with the desktop. That is what keeps `ContractOf`'s term list unchanged — a right press reads
  back as guarded with no new shape for any reader of the contract to learn, which is the lesson
  §41r paid for.
- **Absent means left**, so not one saved routine changes text. Writing `Button "left"` onto every
  guarded press in existence would be a diff in every file a person keeps, for no change in
  meaning.
- **A host-module change and not a Core one.** One field on a set `os.marco` already declares,
  where `Drag` has carried its own button since it existed. Reported before implementing, per the
  relay's instruction; no Core change was needed.

#### Two defects found while building it, neither about right or middle

- **An unknown button name was pressed as LEFT.** `mouseButtonFlags` returns the left flags for
  anything it does not recognise, so a route naming a button that does not exist — a hand-edited
  file, a typo, a term from a later recorder — pressed the left button at a point the guard had
  just resolved and checked. Every safeguard doing its whole job while the act was the wrong act:
  in most applications one of those opens a menu and the other chooses something in it. It is a
  refusal now, naming the value, kept in the account `marco do` reads, and asked FIRST — before
  the window, because a button nothing can press is a fact about the route, and a refusal naming
  the window would send somebody to look at their application for a defect that is in a file.
- **The press read the stop nowhere.** The hold reads it, the keystroke reads it, the drag reads
  it twice — and a click dispatched into a cancelled run went to the machine, with `clickAt`
  returning the cancellation AFTER the send, so it was reported as a press that failed having
  happened. It is read twice now: on entry, so a stopped run neither presses nor interrogates a
  desktop it was never going to press in; and again immediately before the button goes down,
  because confirming the window reads a rectangle, a scale and a program name off a live desktop
  and takes time of its own. ADR-259's ordering, applied one instant earlier.

#### What was deliberately left alone

A press that measured nothing still emits `do OS's Click with "right"` and still presses where the
cursor is: that is the contract every legacy route was saved under. Director's own lowering
(`internal/marcoexec`) still aims and then clicks — those Points carry no window, so nothing there
is guarded and there is no check to leave on the wrong side of the aim — and its two comments
claiming the language could not do better were corrected rather than its behaviour. No routine is
regenerated.

#### Four desktop scenarios, run green the day they were written

`button/` — two buttons arriving as themselves, a moved app, a resized app refusing with nothing
sent, and a Ctrl-held right press. **4 of 4, exit 0, on the first run**
(`plugins\e2e\run.cmd -only button`, artifacts at `%LOCALAPPDATA%\marco-e2e\20261027-114450`):

| | |
|---|---|
| two buttons | `right_down` on Save at 258,372, then `middle_down` on Cancel at 358,372 |
| moved app | the window moved +220,+130 and the right press still landed 45,14 into Save |
| resized app | 626x469 → 706x519, stopped as `failed`, **nothing sent at all** |
| ctrl-held | `right_down` on Save **with ctrl held**, by the application's own reading of the keyboard, and nothing held afterwards |

They carried `onlyWhenAsked` until that run and the gate came off in the same commit, with
`TestTheButtonScenariosRunLikeAnyOther` asserting the inverse by name.

**Passing first time is worth noting because the two families before them did not** — the holds
took two runs and the drags four, and §41r is the story of the drags'. That is a fact about this
slice rather than about the harness: the press, the guard, the window check, the stop, the drain
barrier and the fixture's log were all exercised here already, and what this contract added was a
button on a Point. The arrangement was not new; only the fact being asserted was.

**The fixture's log is the whole evidence, and that is not a preference.** Nothing inside Marco
knows which button went down — `SendInput` with the wrong flag is a successful send — so a left
press delivered where a right one was demonstrated is a run that worked by every account Marco
has. And `save_presses` cannot corroborate them: WinForms raises Click for a left press and not
for the other two, so the oracle that confirms every left-handed scenario in that harness is
silent for these by design. Said out loud in the scenarios, in the subsystem note and in the ADR,
because a reader who expects the counter to move would read its stillness as a finding.

**Physical recording capture remains NOT EXERCISED.**

#### Verification

Root `go build ./...` and `go test ./...` exit 0 (111 packages, 0 FAIL); `plugins/e2e`,
`plugins/studio` and `plugins/fixture` ok; `gofmt -l internal cmd plugins` clean; `docscheck` no
problems; `wiringcheck` green — it caught ADR-251 citing the Studio test this slice renamed, and
then caught the replacement prose for naming the dead test in backticks, which is the same lesson
§41q recorded one slice earlier.

**Mutations: 17 applied, 16 killed on the first pass and one survivor fixed.** The survivor is
worth the paragraph. M3 widened `emit`'s cursor-click case back to every non-left press and the
suite stayed green — because a MEASURED press makes the route a chain, so that case is never
reached for one. The condition I had written there was a hand-copied negation of the question
`clickPoint` already asks, and the shape it protected turned out to be real and untested: a press
carrying an offset and no basis, which the left button has always emitted as a Point and the
non-left branch beside it sent to the cursor. Two answers to one question, with nothing in either
file saying why they differed. `aboutAPlaceInAWindow` is that question now, asked once by both
callers, and `TestAPressThatKnowsAnOffsetIsAPointWhicheverButtonItUses` holds the second one. Both
mutations of it are killed.

The other sixteen: the button never reaching the file; a left press acquiring one; the non-left
refusal coming back; an unknown button pressed as left; silence refused rather than read as left;
the press going down with the left button whatever was asked; the refusal not kept in the account
`marco do` reads; each of the two stop readings removed separately; the seeding precondition never
refusing; the button claim never firing; the count claim never firing; one scenario dropping the
claim from its `firstOf`; the gate re-applied after the green run; and Studio's review line
dropping the button. Each killed exactly the tests that are about it.

### §41t — typed text is a sequence of submissions

The relay's next slice, bounded, with seven corrections to the contract I had proposed. Three
commits: the host, the generator, the desktop scenario and the docs.

#### What was wrong, and the three different kinds of wrong

`do OS's Type with "hello"` reads like one thing and is five events, any of which the machine can
refuse on its own.

- **Nothing was accounted.** `Invoke` dispatched typing through `ok(…)` rather than `delivered(…)`,
  so a rejected send became a failed step recorded in no account at all: `Unperformed` empty,
  `marco do` reporting success about typing that did not happen.
- **Nothing was propagated.** `StepType` was in `emit` and in no arm of `propagating`. Typed text
  was never a link, so whatever followed it ran whether it arrived or not — the keystroke's defect
  one input along, and worse: a half-filled field changes what the press after it MEANS, and the
  window guard cannot see the difference.
- **Partial delivery was unrepresentable.** The loop stopped at the first refusal and said neither
  how far it got nor that the question had an answer. A stop pressed mid-word is the ordinary way
  this happens.

#### The corrections that changed the design

The relay's, and each one made the contract smaller or more exact:

- **Count accepted input, not "characters arrived."** `SendInput` acceptance does not prove an
  application received anything. The account says SUBMITTED, with uncertainty for a partially
  accepted character, and the unit is a Unicode CODE POINT — never a Go byte offset, never a
  UTF-16 unit. `4 of 8 characters` about two emoji is not a smaller claim; it is a wrong one.
- **`input_actions` stays attempts.** A rejected Type counting as one attempted action is correct;
  progress goes beside that number rather than in place of it.
- **Partial progress survives cancellation**, outcome still `cancelled`, warning in the record,
  the output and the card — and **not through the stuck-key account**. A held key means every
  keystroke anybody types next arrives modified; a half-filled field means nothing of the sort.
- **Never expose content in diagnostics.** The backend QUOTED the rejected character into its
  error, which for a secret is one character of a password in a log line. For a secret, content,
  length and failure position are all suppressed — by never computing them, so there is no number
  to leak rather than a redaction somebody has to remember.
- **Expand mixed steps before chaining.** One `StepType` emits several sentences, and wrapping
  only the first or the last leaves the original defect where it matters most.
- **Preserve saved-source compatibility**, with legacy continuation asserted behaviourally.
- **`sendUnicodeChar` truncated non-BMP characters**, which I had not found: the rune was cast to
  `uint16`, so U+1F600 went in as U+F600, a private-use character, and `SendInput` took it. A
  successful send of the wrong character, silently, every time.

#### What the mutation gate found this time

**17 applied across the three commits, 15 killed first pass, two survivors, and both were about
something not being exercised rather than something being wrong.**

The first: the **adapter**. The gesture is driven by a fake machine — which is what makes a
refusal mid-character arrangeable at all — and that establishes nothing about what the real one
hands Windows. Collapsing the counting send back into a bool changed no test. It is the drag's
lesson one device over (`MOUSEEVENTF_MOVE` missing from a button event documented as repositioning
the pointer), and `TestACharactersEventsCarryItsCodeUnitsInOneCall` holds it now: the units, one
call per character, the count coming back from the machine.

The second: a **wiring test that passed by reading the wrong thing**. It searched the scenario file
from the scenario's own `func` line to the end, so every claim's own DEFINITION further down
satisfied it — deleting a call from the scenario changed nothing. It is an AST walk of that one
function now. A text search for a function name finds the function.

#### The desktop half

`text/typed text arrives and the next step follows it` — **1 of 1, exit 0, on the first run**
(`plugins\e2e\run.cmd -only text`, artifacts at `%LOCALAPPDATA%\marco-e2e\20261027-124349`). The
fixture's own `Name` field held `marco typed this 42` character for character, `Notes` beside it
was untouched, and Save arrived as record **#42** — after the last character, by the application's
own counter. `input_actions=3`: the press, the typing, the press.

This is the first family whose oracle is the application's own CONTENT rather than the messages it
received, and it is the one place that can say ARRIVED where every account inside Marco can only
say submitted.

**Physical recording capture remains NOT EXERCISED.**

#### §41u — three findings on §41t, and what the gate found behind one of them

The review of the typing slice returned three. Each was real, and the first was a P1.

- **[P1] A secret that could not be LOOKED UP reached no account.** `doSecret` returned `fail`
  for a missing credential and for a store that would not answer, and wrote nothing down. The
  chain did its job — the arm ended the routine and nothing after it ran — and then the COMMAND
  reported `performed`, because the rule that fails a run reads `Unperformed` and the account was
  empty. A person was told their routine had run while the field the password was going into was
  empty. The fix is one function, `couldNotLookUp`, and the account it writes to is the one for
  things that DID NOT HAPPEN: nothing was typed, so a person whose secret was never found must
  not be told that part of it may be sitting in a form.
- **[P2] The refactor had silently removed the typing cadence.** `typeCadence` and
  `$MARCO_TYPE_CADENCE_MS` stayed declared and nothing called them. Lifting the sequence out of
  the backend left the pacing behind, so every text — including every legacy routine's — went out
  as fast as `SendInput` would take it, which is the thing the knob exists to prevent. The wait is
  on the MACHINE now, the chord's `settle` shape and its reason: the cadence belongs to the real
  keyboard, and the early return is what a stop looks like from inside a wait.
- **[P2] A secret's progress was still being measured.** The sentence left the numbers out and
  the gesture computed them anyway — redaction after measurement. `typingProgress` has two
  registers now: `finished` and `partial` are state, answerable about any text, and `submitted`
  and `total` are accumulated only for a literal. What had forced the numbers into existence was
  `whole()`, which compared them — so without them a refused secret would have reported itself
  complete, and the account would have stayed silent about a field with part of a password in it.

**The command-level test needed a seam, and the seam needed a reason.** The rule that fails a run
lives in `cmd/marco`, and a test with a host of its own would be a test of the rule against a
stand-in written to satisfy it. So `oshost.NewWithSecrets` is exported, with the reason in its
doc: a test may not put a value in somebody's credential manager, nor read whatever is in there.
The control stores an EMPTY value, which is a real state and submits no characters, so the step
after the secret is reached without typing into whatever has focus on the machine running the
tests. That step is a guarded press with no `Activate` in front of it, so it refuses for want of
a bound window — which is what makes "the step after the secret was reached" observable as
`input_actions` of 2 rather than 1, with no input sent at all.

**And the gate found a fourth thing behind the second.** My first fix for the cadence put a stop
check inside the pacing branch; removing it changed nothing, because the check at the top of each
character had already answered. One check sits after the wait and answers both questions — a stop
during the previous submission, and a stop during the wait. Two checks where one is load-bearing
is a line nobody can test, and it reads as a guard somebody added for a reason.

**Mutations: 8 applied, 7 killed, one survivor equivalent.** The survivor was `typeCadence <= 0`
weakened to `< 0`: `time.After(0)` fires immediately, so a zero cadence behaves the same either
way and no assertion can separate them.

The text gate was **re-run** after the fixes rather than taken on trust, since the typing path
itself changed: 1 of 1, exit 0, the field holding `marco typed this 42` character for character
and Save at #42 again (artifacts `20261027-132002`). **Physical recording capture remains NOT
EXERCISED**, and these three failure paths are headless by nature — a desktop cannot be made to
refuse a send.

### §41v — a routine states which generation wrote it

`fa57c6a` → `df3bc32` → `71ec609` → `c0165f5` → `044060b`, on `feat/demonstrated-acts`. The
Architect's work order after the Type/Secret slice closed: generation-contract versioning,
starting now, with no historical generations reconstructed from unstamped files.

**The gap, and why it was invisible.** A routine keeps the shape it was generated with, for ever,
because nothing rewrites saved Marco — ADR-251's rule, and the right one. So every rule about what
a generated routine DOES reaches a routine when that routine is written: presses checked, chords
and holds and keystrokes propagating, a drag's two ends, a press's button, typed text stopping what
follows it. A routine saved before the last of those goes on acting past text that never arrived
and reports a run that performed. ADR-251 said so at the time and scoped the answer out.

Nothing on disk said which rules wrote a file, and **nothing can work it out**. The shapes in a
routine are a function of the demonstration as much as of the generator — and that inference has
already failed here in exactly this shape: before `draggedTerms` existed, `ContractOf` knew only
about a `Point`, so a guarded drag read as **legacy**, Studio told somebody a routine that checks
its window checks nothing, and four desktop scenarios failed on the precondition before anything
about a drag was exercised.

**So the generator stamps it, and the stamp is three things it is not.** `// Generation: 1.`,
written **unconditionally** — whatever called it, that generator wrote the file, and a condition
there would be a file written by it that does not say so, which is where this differs from every
other line in that header. Not the BUILD: ADR-251 settled that an older build does not establish
that a routine needs anything. Not a CORE change: a comment is not a word of the language, the
same ground `// Recorded as:` stands on, reported as such before implementing. And not a VERDICT —
a current-generation routine can press without checking anything, because that is what its
demonstration asked for, so what the presses promise stays `ContractOf`'s separate question and
both facts are shown with neither derived from the other.

**Four readings, and the damage from merging any two.** Current, older-and-recognised, unstated,
unknown. *Unstated means exactly that* — not "written before versioning", which is a claim about
when a file was made that a handwritten routine, or one somebody deleted the line from, does not
support. *Unknown is neither of its neighbours*: `current` would be this build writing down that it
wrote a file it cannot read, into the one artifact a person cites when a routine behaved
differently somewhere else, and `unstated` would lose the only fact the file does state. So the
reading and the declared value travel separately everywhere — in the reading, on the card, in the
record. Malformed, conflicting and **duplicate** all read as unknown, duplicates included when the
two agree: a file answering one question twice has been edited by something, and which answer it
means is not in the file. Nothing is offered for an unknown generation, and what is withheld is
*only* the offer — the routine stays listed, stays runnable, and runs under exactly the rules it
always had, held against a control run of the same routine unstamped that has to reach the same
outcome for the same reason.

**Generation 1 has nothing behind it, on purpose.** Numbering the five rule changes that landed
before today 1..5 would place routines in a history they never stated, and the only way to decide
which one a file belonged to would be to infer it from its shapes. The history is **one list**, so
the current generation is its last entry rather than a constant beside it — `everyMeasuredTerm`'s
lesson one layer up, where a generation added to a counter and not to the words would be read,
found older, and never explainable. And the difference walk is exercised against **synthetic**
future generations, with the one authority swapped under `t.Cleanup`, because with one real
generation three exported readers would have no live input at all and this repository's record is
eight cases of complete code never invoked.

**Older is not affected.** A difference is a fact about two GENERATORS; whether it changes anything
for one routine is a fact about that ROUTINE. So each rule carries its own question about a saved
source, and a routine is claimed to need replacing only where its contents answer it. A card that
announced every difference as a defect would be telling somebody to replace a file over a rule it
never exercises — and the next time it said so about a real one they would have learnt to ignore
it.

**The flow is four steps and three of them write nothing.** Select the exact routine, prepare a
version from its own stored demonstration, review both sources in full plus any refusal, then
explicitly replace or cancel. `Resave` is **not** this flow and stays what it is: it repairs a
routine that cannot press anything at all, so it is immediate on purpose, and it RESCOPES through
the save path that chooses where a routine lives. Neither is allowed here — the scope is read off
where the routine already lives, so a replacement cannot quietly move somebody's routine.

Preparation writes nothing, which is what makes cancelling the absence of an operation rather than
a tidy-up a crash can interrupt. A replacement re-reads both files it was built from and refuses if
either moved, naming which: somebody can edit the routine or record the same name again while the
review is open, and writing then would replace a source nobody looked at with a rendering of a
recording that is no longer there — silently, because the result would look like an ordinary
success. **That is also the whole of the protection for a hand-edited routine**: the edit is the
left-hand pane, so replacing it is a decision, and what is not allowed is for that to happen
without the review being current.

`Save`'s own rendering became the one function both callers use. A review built from a copy of
those four lines would be a second answer to what a draft becomes, and the disagreement would be
between the source somebody approved and the source that replaced theirs.

**A rebuild today's rules would refuse is a reviewable answer, not an error.** A recording with a
press found by matching a picture was saved happily by an older build and produces no routine now,
so "what would I get if I rebuilt this" has three answers rather than two, and the third is shown
in the generator's own words beside the source it would have replaced.

#### What the mutation gate found, and the two it found twice

**31 mutations, 28 killed, one equivalent, and two real holes that were the same hole.** A routine
this build just wrote rebuilds **byte for byte** from its own recording — so in the two tests that
used one, the saved and rebuilt sources were identical, and two different wrongs were invisible: a
`Prepare` that saved its own result changed nothing a digest could see, and a `Replace` that wrote
the OLD source instead of the reviewed one wrote the same bytes either way. *Successful replacement
produces exactly the reviewed source* — the claim the whole slice rests on — was passing without
being able to fail. Both tests now take the generation line out first and assert the two differ
before relying on it.

The third was a widened lookup: `routineWithID` also matching a name or a slug is invisible until
something passes one, and nothing did. A name two routines answer to is not an address, and
choosing one is how a person approves one routine's source and has another one's file replaced.
Something passes a name now, and the refusal is the assertion.

**And the gate's own first run was wrong about three of them.** The `-run` filter I used matched
none of the tests whose names do not contain the word "Generation", so three mutations reported as
survivors that the package kills outright. The false-survivor trap from the other direction: a
filter narrow enough to miss the test that holds the line.

Four gaps were closed before the gate could speak at all, because the exported readers were only
tested through their internals and Studio's refusals only through a page stub: the exported readers
under a synthetic future, a context-scoped replacement where the scope changes what is GENERATED,
the Acts' four refusals, and a routine that TYPES the generation line not being a routine that
states one.

One mutant survives on purpose: `Registry.Replace`'s atomicity cannot be told from `Save` by any
test here — they differ only when a write fails PART WAY, and a disk full is not something a test
stages portably. The comment beside it says so rather than letting it look held.

#### The desktop scenario, and what it can see

`generation/a reviewed rebuild replaces the routine` **ran green on 2026-10-27, 1 of 1, exit 0,
first run** (artifacts `20261027-162205`). It is the only desktop scenario here that sends **no
input at all**: it presses Studio's own controls through UI Automation and reads a file. The
fixture is open because the routine has to be a real one, generated by the production path from
real window measurements; its window is never pressed.

The oracle is the **file**. Studio's words are read too and reported as evidence, but three of the
four steps are claims that nothing was written and only the bytes settle those. Its strength is not
"the file changed": the harness takes the generation line out of a routine this build just wrote, so
the same generator wrote both versions and the only difference a correct replacement can have is
that line restored. On the first run the replaced file differed from the stripped one by **exactly
that line** and was **byte-identical** to what the generator had written before.

What the arrangement establishes is bounded and the boundary is stated where it happens: it
exercises the flow against a routine stating no generation. It is not a claim that any particular
older routine in anybody's library looks like this, and nothing in it reconstructs a generation for
an unstamped file. **Physical recording capture remains NOT EXERCISED.**

A second run was made only to read the exit code, and also passed. The first run is the one
reported above and its artifacts are preserved.

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### §41w — four findings against §41v: one slot, one overwrite, one stale read, one promise

The review of the versioning slice found four, and two of them were about the same kind of
mistake: a check that was correct about something other than the thing being acted on.

**[P1] A replacement identified the routine, not the review on the screen.** Studio held one
preparation per routine — `a.reviews[id]` — so a second review of one routine overwrote the first.
These calls are asynchronous, so the page could be displaying A's two sources while the window
held B. Press Replace and B is written: every check in `Deps.Replace` passes against B, the
success names the routine the person was looking at, and the bytes are ones they never saw.

A preparation now carries an immutable **token** and a replacement must present it. The token is
derived from the CONTENT — the two files it was built from, what it would write, and why it would
write nothing — rather than counted. Two reasons. What has to be identified is the *review*, and
the content is that: two preparations sharing a token are byte-for-byte identical in what was
displayed and in what a replacement would write, so resolving one to the other cannot write
anything nobody saw. And a counter would need somewhere to live, which is the shared state this
defect came out of. `Deps.Replace` checks a `Prepared` against its own token too, so one edited on
its way back — by a caller, by a bug, by a struct assembled from parts of two — says so. Studio
keys its held reviews by token now, bounded at sixteen, oldest out first; an evicted review is a
refusal with a reason rather than a wrong write.

**[P1] A failed replacement could change the original's images.** I wrote the images first and
called them additive. They are not. They are named after the routine — `<base>-anchor-1.png` — so
a rebuild produces the SAME paths and `WriteAssets` overwrites what is there. Write the new
images, fail to write the source, and the old source is still on disk pointing at paths that now
hold different pictures: a routine that presses what a different image matched, having been left
alone. The comment calling that order safe was the tell, and it was wrong in exactly the direction
the ordering was chosen to protect.

A conflicting image is now a **refusal** and nothing is written at all. An identical one is no
conflict, because writing a file the bytes it already holds changes nothing — which matters, or
every re-review of an anchored routine would refuse. An absent one is no conflict either: a file
nobody references is not a change to anything that runs. What a failed SOURCE write can still
leave is new image files beside an unchanged old source that does not reference them, which is
unreferenced files and a routine behaving exactly as it did.

Content-addressed image names would make the paths immutable and were rejected on scope: that
changes what every newly generated routine's files are CALLED, reaching every asset writer and
every saved file's embedded paths, and would have arrived inside a provenance slice unreviewed.

**[P2] The unknown-generation refusal checked an earlier read.** `ReviewRebuilt` read the
LISTING's eligibility; `Prepare` then read the source again and never asked what that snapshot
stated. Replacing the file between the two reads with an unknown-generation version bypassed the
one restriction where this build refuses to say what a replacement would change. The rule is now
applied where the bytes are — `Prepare` reads the generation off the source it actually read and
refuses there — and Studio's check survives only as the greying of the button.

**[P2] "Current" was presented as proof of identical regeneration.** The sentence promised "it
will be the same routine". The stamp cannot support that: it says which rules WROTE the source,
not that the source is still what they wrote. A routine somebody edited by hand keeps its stamp
and rebuilds differently — and that person is exactly who would have been told their comparison
was pointless. It offers the comparison as the question it is, and only `Prepared.Changes()`
answers it, after the rebuild.

**The gate against the four: 14 mutations, 13 killed, one real hole.** Nothing exercised Replace
from the PAGE — the rendered test reviewed and cancelled, which is all it needed before — so the
page sending a routine's identity in place of the token survived. The page now reviews, cancels,
reviews again and replaces, and asserts which term travelled in both directions.

Two tests had to stop using a routine this build just wrote, for the reason §41v's own gate found:
such a routine rebuilds byte for byte, so "the file holds the second review's source rather than
the first's" is true however the window behaves. The generation line comes out first, and the
tests say so.

**The gate was re-run rather than taken on trust**, since both the Studio page and the backend
changed: 1 of 1, exit 0 (artifacts `20261027-171820`), with the same evidence as the first run —
the replaced file differing from the stripped one by exactly the generation line and
byte-identical to what the generator had written before. **Physical recording capture remains NOT
EXERCISED.**

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### §41x — two of §41w's four were not actually closed

The review reopened two P1s, and both were cases of a fix that was right about a narrower thing
than the finding.

**The token identified a content, not a target.** I derived it from the two files, what would be
written, and why nothing would, and argued that two preparations sharing a token are identical in
what was displayed and in what a replacement would write. The argument said nothing about WHERE.

Two routines with identical sources and identical demonstrations are a thing this product makes
readily, and I had the example in front of me: the same demonstration saved IN-APP for two
applications is byte-for-byte identical, because an in-app routine switches nothing, so the
generator is told no application and the source never names one. Those two would have shared a
token — and a window holding one and handed the other's token writes the right bytes to the wrong
file, with a success naming the routine the person was looking at. The `settings` case this
product keeps tripping over, arriving through a door I had just built.

So the route's whole identity and the library are in the digest, each part separately so no two
routes can run together into one string, with `Focus` among them because it is the difference
between a routine that brings its application forward and one that does not. A `Deps` pointed at
another library is refused on its own too, with its own sentence: "not the one that was prepared"
would send somebody to look at a review that is perfectly fine when it is this `Deps` that is
pointed elsewhere.

My first test for this FAILED ITS OWN PRECONDITION and said so, which is what the precondition was
for: I reached for two scopes, and `app-anywhere` prepends an `Activate` while `app` strips it, so
those two differ in their first step and a content digest would already have separated them.

**A refusal was not the whole of the image rule.** Deciding that no image conflicted and then
writing all of them left two of the four states wrong. A path that already holds the right picture
is now SKIPPED rather than rewritten — writing a file the bytes it already holds changes nothing
that runs and is still a write: it touches the file, it can fail, and it is one more thing between
a person's decision and the one file this is meant to replace. And a path that cannot be READ is
now a refusal rather than an empty space, which is the correction that matters: I had it
`continue` on a read error, and treating unreadable as "nothing there" is the one reading that
licenses writing over it. A path this cannot read is precisely a path it cannot say is safe to
replace.

So `imagesToWrite` classifies every path first — missing, the same, different, unreadable — and
writes only the missing ones. Deciding all of them before writing any is what keeps a refusal
found on the second image from leaving a routine beside pictures from two different rebuilds.

**Two tests here are worth remembering for their method.** The skip is asserted by making the write
IMPOSSIBLE: the matching file is left read-only, so a replacement that writes it fails and one
that skips it succeeds — comparing timestamps would not do, since the two writes are the same
bytes microseconds apart. And the unreadable path is a DIRECTORY where the image belongs, which is
the one unreadable path a test can arrange on any machine.

**10 mutations, 9 killed, one found a line nobody could test.** Dropping the library from the
token's digest changed nothing observable, because the separate library check already caught the
end-to-end case and one `Acts` only ever holds one library. Rather than keep an untestable line or
delete a correctness property, the token's own claim is asserted directly: two preparations of one
routine in two libraries must not share a token. That is a contract about a public value with a
documented meaning, and the next thing to key a map on it would otherwise be what found out.

The desktop scenario was NOT re-run for this: nothing it exercises changed — it reviews, cancels
and replaces one routine in one library with no images — and the two cases fixed here are ones it
has never been able to reach, which is what the review said. **Physical recording capture remains
NOT EXERCISED.**

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### §41y — classifying is not writing

One P1 left from §41x, and it is the same shape as the three before it: a check that was correct
about an instant other than the one the act happens in.

`Replace` classified every image path — missing, the same, different, unreadable — and then wrote
the missing ones through `Registry.WriteAssets`, which is `os.WriteFile`: create **or truncate**.
Between the classification and the write, another save — another Studio, a `marco learn`, a sync —
can create one of those paths, and the write would then destroy what they had just written on the
strength of a reading taken beforehand.

Not a collision between unrelated files. These paths are reusable by design —
`<base>-anchor-N.png` — so it is the same routine's own image name, written by whatever else is
working in that library.

So the creation **is** the check. `Registry.CreateAssets` opens each path with
`O_CREATE|O_EXCL`, which closes the gap rather than narrowing it: there is no instant between the
test and the write because they are one operation, and the path existing is the failure with
nothing written to it. `WriteAssets` keeps create-or-truncate, which is right for a save — a fresh
save owns the paths it names, and a collision on the routine's name is refused long before any of
this. Two functions rather than one with a flag, because the two callers want opposite things for
good reasons, and a flag would make the next caller choose.

Files created before such a refusal are **left**. They are paths that did not exist, nothing
references them, and removing them would mean deleting a file between creating it and tidying it
up — a narrower version of the hazard being closed.

**And the moment is tested rather than the mechanism.** `Deps.afterClassifying` is an unexported
seam occupying the instant between the two steps, and what it does is what the other save would
have done — so the test is deterministic rather than a race hoped for. Nothing in production can
set it, since a `Deps` is built from its exported fields everywhere, and nothing has a reason to
be notified of that instant. That is the point: the only way to show a gap is closed is to put
something in it. The same discipline as the stop checks in `internal/oshost`, and the fourth time
in this slice that the defect was about WHEN rather than WHAT.

The mechanism underneath is asked directly too: `CreateAssets` answers `fs.ErrExist` and leaves
the file as it was, creates a path that is free, and `WriteAssets` still truncates — which is the
assertion that keeps the two from being quietly merged.

3 mutations, 3 killed: the call reverted to `WriteAssets`, the flag changed to `O_TRUNC`, and the
seam never fired.

No desktop re-run, and the review said so: the scenario replaces one routine with no images, so
nothing it exercises changed and this case is one it cannot reach. **Physical recording capture
remains NOT EXERCISED.**

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### §41z — Studio Simplify, and four more things that were already ruled on

`dca63da` → `0ce3673` → `27c6d09`, on `feat/demonstrated-acts`. The Architect's order after
generation-contract versioning closed: Studio Simplify, through the shared review flow, with no
separate panel or replacement mechanism.

**Nothing new was built, and that was the finding.** Studio had no simplify at all, so the question
looked like "what surface does it need" — and simplifying a routine is *select, render its own
recording again, review, replace or cancel*, which is the flow the previous slice had just had
reviewed to death. Only the rules differ.

So the rules are a **bounded kind**: `RebuildCurrent` is today's generator rules and nothing else,
`RebuildSimplifyTyping` is those plus folding a typed run across longer pauses. Not
caller-supplied simplifier settings — a caller handing those over is deciding what somebody's
routine becomes from outside the one place that knows what the choices MEAN — and a word this
build cannot name is **refused** rather than defaulted. Defaulting is the worst answer available:
the review would be of the ordinary rebuild under the simplified one's name, with a timing caution
attached to a routine whose timing did not change. The kind is in the token, because two reviews
of one routine differing only in the choice that made them are two different things to replace it
with.

**And "simplify" must not imply nothing changed.** Folding a typed run removes the waits BETWEEN
those keystrokes — up to a second of them — so a routine that typed into a box needing that second
will now type into it faster than it did when somebody watched it work. The choice carries that in
a sentence rather than leaving it in a diff, and the sentence also says what does NOT change,
because the first half invites the question: a wait outside a typed run, which button each press
used, which keys were held across what, and the routine stopping at the first action that did not
happen. The CLI prints it before the prompt, asserted by POSITION rather than by presence.

#### What `SimplifyRoute` was doing

Four things, each already ruled on somewhere else, three of which I would have been
re-introducing:

- **It simplified under the ENVIRONMENT's mode.** `d.learnOptions()` reads this process's idea of
  what a recording captures, and the simplification depends on it: with no anchor to wait on, the
  recorded gaps are the only waiting a routine has. A routine whose file states `coordinates`,
  simplified by a process launched with `$MARCO_CV=on`, had every gap capped to 50ms and nothing
  put in its place — a menu pressed before it had drawn. **ADR-254's defect, fixed in `Resave`,
  still here.** `learnOptionsFor`'s own doc comment describes the damage.
- **It deleted the `// Recorded as:` line**, writing through `codegen.Route` rather than `RouteAs`.
  The other half of the same defect: a claim about somebody's afternoon edited out of their file by
  a tidy-up.
- **It wrote with `Reg.Save` and `Reg.WriteAssets`** — create-or-truncate on the source, overwriting
  on the images. The two P1s the Architect had just had me close.
- **And nothing checked the file was still the one it had printed**, across a gap containing a human
  answering a prompt — the longest such window in this product.

#### The lookup, local to Simplify

`Registry.Resolve` is built for PERFORMING: the foreground application's routes, then ANY other
application's focus route with that slug. Reaching into another application is exactly right for
"do the thing I named", because a focus route brings its application forward. It is exactly wrong
for "rewrite the thing I named": `settings` saved for two applications, simplified from a third,
overwrote whichever sorted first.

`marco simplify` already guarded one half of this and the comment above it says why at length — it
resolves exactly rather than by fuzzy match, because a guess that rewrites a file destroys
somebody's work. That guard was about the **matching**, not the **selection**, and an exact slug
shared by two routines is still two routines. So every routine with that slug is found, more than
one is a refusal naming where each lives, and the command prints which routine it is about before
asking. `Resolve` is unchanged for every caller that performs.

**`SimplifySaves` skips the QUESTION and nothing else.** It is a statement by whoever built the
`Deps`, not evidence of who the caller is — the Architect's ruling, and right: an environment
variable cannot establish that, and a surface able to show a comparison must not be excused from
showing one by an environment it inherited. Studio never reads it.

#### Three things the gate found, and one I found by committing

**My own `-run` filter hid a failing test, again.** `TestASaveWithNoPreviewStillRefuses…` used
`afterClassifying` to change the file mid-flight, and that seam fires INSIDE `Replace`, after the
digests have been read — a seam in the wrong gap, asserting nothing. It failed and said so, and I
committed anyway, because the filter I verified with matched none of the words in its name. The
same false-negative as the generation gate's first pass, from the same cause. Rewritten around a
check only `Replace` makes: a conflicting anchor image, which nothing else in that command looks
at.

**The page harness called the handler instead of pressing the button**, so the second button's
wiring was untested and a mutation swapping its argument for the first's survived. It presses the
controls now — Review, Simplify, Cancel and Replace — which is what the desktop harness has always
done and what the page harness should have.

**Two guards nobody could test**, both closed by asking the predicate directly rather than deleting
it: a reading of the whole library that found NOTHING reported as a quiet pass (and the case that
matters is both readings empty — an empty `before` against a real store already reports every file
as created), and the seeding precondition's second half, which refuses a source holding the runs
separately AND the whole of it together. That is not a thing `codegen` writes, which is exactly why
nothing else would notice if it started.

#### The desktop scenario, and the first one here to play what a review wrote

`generation/a simplified rebuild is reviewed, replaced and performed` **ran green on 2026-10-27, 1
of 1, exit 0, first run** (artifacts `20261027-191020`). The saved routine typed `marco typed`,
slept 550ms and typed ` this 42`; the simplified one types the whole of it in one step with the
sleep gone, byte evidence in the artifacts. No file in the library changed until Replace was
pressed, measured over every file in the store. And then it **performed** — the application's own
Name field holding `marco typed this 42` with Save pressed after it, which is the question the
previous scenario never asked: a review that produced a routine nobody could run would be a review
of a file rather than of a routine.

The 540ms pause in the demonstration is between the two thresholds on purpose, so it is rhythm to
one rebuild and a deliberate wait to the other. Without one, both choices produce the same routine
and every assertion stays true while establishing nothing — which the seeding precondition now
refuses.

The first name tried for the scenario contained `play`, and the family-name test caught it: `-only
play` would have swept it up. **Physical recording capture remains NOT EXERCISED.**

One unexplained observation, recorded rather than claimed away: a single `plugins/studio` suite run
FAILED at 94s immediately after a mutation-and-restore of `frontend/index.html`, and four forced
runs since have been clean at ~65s. I could not reproduce it and have no account of it.

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### §42 — release hardening, which turned out to start with "Studio is not shipped"

`b3f9a6e` → `b386fa7` → `09dfc17` → `bcd58f0` → `1b524b8` → `c848176` → `1d002ce` → `d281ece` →
`9f619a6`, on `feat/demonstrated-acts`, after `c043a9f` wrote down the window-bounded distinction
the Architect asked for.

I proposed release hardening expecting a check-over pass. The first thing I looked at ended it.

**`pack.ps1` did not build Studio.** It staged `marco`, `marco-macros`, `director`, `overlay` and
`uia`; the launcher ran the overlay serve stack. No root script mentioned Studio and the README
never did. So five slices — the guarded press, the guarded drag, the button, typed text, and the
generation stamp with its whole review flow — shipped in NOTHING. Studio existed only as a
development build.

And the consequence is sharper than a missing feature, which is the part worth remembering: the
shipped overlay *can* record and *can* simplify, and it sets the no-preview option because it has
no way to render two sources. **An installed Marco could write a routine and could not show
anybody what it was about to write.** Every guard that depends on a review was unreachable to
whoever installed it — which is what the previous two slices were almost entirely about.

#### What shipping it took

Studio is staged and opened, with the Wails tags, and `pack.ps1` reads the built bytes back
looking for the guard's own message box — the only way to tell a windowless build from a working
one without opening a window on somebody's desktop. The overlay is still shipped and starts with
`--overlay`: it is a different product surface rather than an older version of this one.
`marco.exe` and `director.exe` land beside `studio.exe`, because Studio spawns the ones beside
ITSELF, and `bin\` satisfies that and `marco`'s own two lookups at once.

**An incomplete bundle is now a refusal.** `marcoBin`/`directorBin` answer a bare name when there
is no sibling, which `exec.Command` resolves against PATH — so an installed Studio with a missing
sibling performed routines through a binary nobody chose, of whatever age, with every window
looking current. Readiness had reported that for a while and blocked nothing. Play, Run and Learn
refuse it.

**A bundle carries a manifest**: every file's hash, every Go binary's revision asked of the binary
itself, and a binary with no revision fails the pack rather than shipping. The provider has no
stamp — .NET, built by the compiler Windows ships — so it is identified by its bytes. Studio
verifies the hashes against disk, because a manifest is a claim, and prints it WITHOUT opening a
window: a bundle that will not start is exactly when the report has to be reachable without
starting it.

And an upgrade blocked by a running Studio says to close it rather than killing a window that may
hold a draft existing nowhere else.

#### Two claims the product was making

The window now **discloses global recording before Record** — every press and key anywhere, Studio
excepted — and says the other half: recording only a chosen window is not something Marco can do
yet. Rendered rather than asserted in source, because a sentence nobody draws is not a disclosure.

And the **README led with the wrong product**: "nothing to record", and `performed` meaning Marco
looked again and confirmed. `internal/outcome`'s definition is conditional — "for a Play whose
arrival could be checked, it means checked" — and the README was reading the condition off. For a
routine, `performed` means every action was submitted and none refused. What a routine *does*
promise is stated instead, and the semantic path keeps its promise as a DIFFERENT one.

#### Running the artifact found three things no test here could

From an isolated directory, with `LOCALAPPDATA` pointed away from the owner's install, no
repository, PATH cut to the system directories:

- a manifest written with a **byte-order mark**, which Go's JSON refuses — so the first packed
  bundle said "this installation's manifest cannot be read — reinstall Marco" about a manifest
  that was correct, on a fresh install;
- a manifest listing **`version.txt`** as a component, which is the embed's own label and is never
  extracted — a correct installation reporting a missing file;
- the launcher's closing line still saying it launches the overlay.

Then the harness itself learned to test an installation (`-installed <bin>`), and the bundle passed
`play` 1 of 1, `runs` 3 of 3, `generation` 2 of 2 — with every run record naming the INSTALLED
binary's revision rather than the tree's, which is the evidence that the bundle was what answered.

One scenario failed on the first installed run — `runs/two runs are two records`, at the Done
button, "no element with that id is in the window any more". Diagnosed rather than re-run for
green: it passes against both the bundle and a tree build, so it is the staleness the harness's own
comment names. A press is a snapshot plus an Invoke naming that snapshot's runtime id, and the page
repaints on a poll. Only that one error is retried, once.

#### The thirty-second hang, which I had recorded as unexplained

§41z ends with an observation I could not account for: the Studio suite failing once at 94s. It
came back, and it is a real defect on the shipped Play path.

Caught with the product's own diagnostic — I made the failure read the run record the child
claimed, which is the artifact that exists for exactly this. The child ran the program; its last
transcript line was the routine's own log; then nothing for thirty seconds, no outcome, slot still
claimed. One run in six under load, half a second in isolation.

**Deductively it could only be one of two things.** The pump wake has been bounded at 5 seconds
since the 600-second timeouts that found it, so a 30-second silence cannot be the wake — it had to
be one of the two unbounded channel receives either side of it. Both are bounded now, each with
its own sentence, so the next occurrence names which expired. The evidence points at the
app-switch poller, whose one blocking call is a foreground-window read, which blocks when the
window's owning thread is not pumping — and a desktop full of browser windows is that desktop. A
pointer, not a proof; nothing claims the cause is resolved. What is fixed is that a run which
finished its program reports that it finished.

**And the rule is generalised.** `TestAPumpWakeupIsRetriedRatherThanPostedOnce` demanded the
wakeup be retried and put the BOUND in its own failure message — prose, not a check. A site could
satisfy every test and still wedge its caller two lines from the one that was checked, which is
what happened. The tree-walking test now refuses any bare receive in a hook site's shutdown, scoped
to the function that wakes the pump, and skipping a `select`'s comm clauses — `case <-ch:` wraps
its receive exactly like a bare one, and a walk that missed that flagged every bounded wait in the
tree.

#### The parked items

- **The stop that could not be raised** is explained. Reproduced deterministically: `os.Rename`
  over the generation file fails with *Access is denied* whenever ANY process holds it open, and a
  watcher polling the generation holds it open every 100ms — so the stop is most likely to fail at
  precisely the moment it matters. `FILE_SHARE_DELETE` on the reader was tried and ruled out:
  `MoveFileEx` with REPLACE_EXISTING still refuses an open target. Nothing is retried — that is a
  policy about how hard to try to stop somebody's computer. The sentence is fixed, and the three
  scenario notes that recorded it as unexplained now point at the explanation.
- **The Director's provisional record** claimed `actions_sent: 0`, `live: false` and three
  verification zeroes about an attempt nobody can speak for. Five pointers and an omitempty later
  it claims none of them, with every reader going through `SaidSent`/`SaidLive` so that silence
  cannot be read as "none". Studio's history reader was already using pointers for those fields —
  the reader was honest and the writer was not.
- **`failed` versus `refused`** keeps the wire word, with the limitation documented where the
  words live.
- **`anchorArmed`'s comment** said the arming survives "even when MARCO_ANCHORS is off", true when
  written and not now: a recording that STATES coordinate mode takes the picture away.
- **The encoding damage** was worse than I had parked it as — I had recorded one occurrence in
  E2E.md, because the `Ã` I noticed was the only mojibake starting with a letter I recognised. A
  scan found 264 sequences in E2E.md and 50 in ADR-252, across eleven characters. Repaired with a
  tool that refuses unless the result is valid UTF-8, the line endings are unchanged and the file
  got shorter.

**Physical recording capture remains NOT EXERCISED.**

See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

### §42b — three P1s against §42, and the first was mine

`0d0306f`. The review of the release-hardening work found three, and the one that mattered most
was introduced by that work.

**The launcher decided where the data was.** I had it set `MARCO_HOME` and `MARCO_ROUTES` to its
own installation directory, with a comment explaining that the launcher states which store this
installation uses. The reasoning was wrong twice: it overrode an explicit `$MARCO_HOME` somebody
had set, and it pointed away from the shared default `%APPDATA%\marco` that every other way into
Marco resolves. **An existing Studio user would have installed the alpha and been shown an empty
library** — routines exactly where they left them, nothing saying so.

The lesson is the one ADR-252 is about and I walked straight into it from the other side: there is
one resolver and the way to honour it is to ASK NOTHING. `plugins/marco-app` cannot even import
`marcopaths` — it is its own module — which should have been the signal that resolving there was
wrong rather than an obstacle to work around. The launcher sets neither now and seeds no starter
routine, which needed the library resolved and wrote into somebody's data from an installer.
`MARCO_BIN` stays, because that names an executable.

The guard is an assertion about an ABSENCE, by name, ignoring comments — because the note that
records this fix necessarily names what it must not do, and a check that matched a comment would
fail on its own explanation.

**And the conflict is named by the window, which ADR-252 promised and never shipped.**
`marcopaths.Elsewhere` was written for *"the window names any library-shaped directory beside its
launch location that holds routines and says what to do about it; nothing is adopted or moved"* —
and **nothing ever called it**. A complete mechanism with no caller, in a repository whose own
memory note is "prove wiring by deleting it". It is wired into readiness now, and the case is real
because the old launcher created it: every overlay-era bundle kept its routines under
`%LOCALAPPDATA%\Marco\routes`.

**An interrupted upgrade left mixed binaries.** Components were written into `bin\` in turn, and
Windows refuses to overwrite an executing file — so an upgrade failed PART WAY with the earlier
files already replaced, while the error said "nothing has been changed". That sentence was the only
untrue thing in it: a new Studio beside an old marco and an old director is exactly the mixed
installation the sibling discipline exists to prevent, and it looks current from every direction.

Each version stages into `bin-<version>` and `bin\` is replaced only once every file is written.
Staging can fail halfway and cost nothing. The activation is a directory swap — the one step that
can still fail against running binaries — so the old `bin\` is put back on failure. Either the
whole new version is in use or the whole old one is.

**Recorder workers could reach a later recording.** The bound I added in §42 made `Stop` return,
and returning is exactly what let the poller and the capture worker outlive it. Both publish
through package state: a late foreground read appended an app switch to somebody else's
demonstration, and an old capture could write a picture into whatever event was at its index. So
the fix for one finding opened the next, which is worth remembering about bounded waits — the
thing you stopped waiting for is still running.

Both are fenced by session, and for the poller the check is AFTER the sample, because
`winctx.Active()` is the call that blocks and the instant that matters is the one it returns in.
The decision the poller makes twice is one function now: the fence went into one of the two call
sites first and a mutation found it.

**And a further recording is refused outright.** The hooks cannot be fenced from Go at all — C
callbacks that gate on `recActive`, which the next `Start` sets true again, so an abandoned
recording's still-installed hooks would feed the new one. A session number cannot reach a C
callback, so recording is over until the process restarts. That is the honest consequence of hooks
that are still installed, and it is what the shutdown's own message already said.

**Verified against the packaged bundle from isolation**, which is the only place these are
visible: a populated store in the shared default is found (`theirs` listed), an explicit
`MARCO_HOME` wins (`mine over here` listed), and an older bundled library is named with both paths
and its routine still on disk.

**Physical recording capture remains NOT EXERCISED.** And the Architect's note stands: the stop
investigation explains the sharing conflict and does not establish reliable stop delivery during
it.

See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

### §42c — three boundaries, and one of them was the previous fix's own edge

`cec9e65`. The review of §42b found three, and the first is the same lesson one layer in: a
check and a write are two moments unless they are one step.

**The recorder fence was asked before publication rather than with it.** Each worker asked whether
the global state was still its own and then wrote into it. A worker could pass that question, be
descheduled, and publish after a later `Start` had emptied the buffer — and the session was raised
BESIDE the lock rather than under it, so the replacement itself was two steps and a worker could
be let through between those as well. The fence I added in §42b was correct in both halves and
still not a fence.

`recMu` guards the session now as well as the state it describes. `beginRecording` raises the
session and empties the buffer under one acquisition; `recordAppSwitch` and `attach` each ask the
fence and make their writes under that same lock. Two things fell out of doing it properly: the
poller's `last` advances only when publication succeeded, because a sample remembered as recorded
when it was dropped would silence the next real switch to that application; and the foreground the
hooks tag every press with is published inside the fence, or an abandoned poller decides which
application a LATER recording was in.

**And WHERE the fence is read cannot be tested behaviourally.** Move `mineUnderLock` one line up,
above `recMu.Lock()`, and every behavioural test still passes while the gap is open again — no test
can pause a goroutine between two adjacent lines. So the rule is checked where it is written, by
name, with a deferred unlock not counted as an unlock. The mutation that found this was my own
first draft's shape.

**Activation ignored a failed recovery.** It ignored the clearing of the recovery directory and the
rollback's own error, so an activation that failed AND a rollback that failed reported itself as
the case where the rollback worked — *"the version you had is still the one in use"* — about an
installation that was not there. That is the one sentence a person in that position must not be
told: it sends them to close a Marco that is not running instead of looking on their own disk,
where their previous version is sitting under another name.

Both are handled, and a failed rollback names the recovery directory and what to do with it. The
recovery directory is cleared only when there IS an installation to move into it, so a rollback
that failed leaves the one copy of their previous version alone until something is activated over
it. **The failed rollback needed a seam** — it is an ORDER of two renames, not a state on disk:
whatever makes a directory unmovable makes it unmovable in both directions. The other three
outcomes need none, and the test that was there exercised `stage`, which is the half that can fail
for free.

**A relative data override resolved against the installation.** The children start from `bin\` so
that a sibling they spawn by a bare name cannot be resolved against whatever directory somebody
double-clicked the launcher in — and that moved the meaning of a relative `$MARCO_HOME` or
`$MARCO_ROUTES` with it. `MARCO_ROUTES=routes` in a terminal means `.\routes` there; the same words
through the launcher meant `bin\routes`. ADR-252's defect arriving by the back door, through the
fix for the launcher's previous one.

An override somebody SET is made absolute against the directory this process was started in,
before any child is started from anywhere else. **It answers nothing**, which is the line that
keeps it from being a second resolver: an override nobody set stays unset, an absolute one is
untouched, and `marcopaths` decides what any of it means as it always did. The only thing done is
to spell a path somebody else wrote against the directory they wrote it in, which is the one fact
the launcher has and the child does not.

**Verified against the packaged bundle from isolation**, which is where these are visible.
`MARCO_ROUTES=routes` typed in an unrelated directory resolved to THAT directory's library,
against the counterfactual — the same words given to the child started in `bin\` the way the
launcher starts it, which resolved to `bin\routes`. The shared default is still found with
nothing set. And an upgrade attempted while a file in `bin\` was held open was refused with
`bin\` whole at the version it had and the new one left staged beside it; released, it activated
whole with the previous version kept. That path is the FIRST rename failing, which is the common
real case; the rollback is the second one failing after the first succeeded, which is why it has
a seam and a test rather than a demonstration.

**And the installed bundle passed all eleven desktop scenarios again** — `play` 1 of 1, `runs`
3 of 3, `generation` 2 of 2, `text` 1 of 1, `button` 4 of 4 — with nothing left running
afterwards.

**Eight hung `marco do` processes were on this desktop**, from test runs between 18:57 and 21:30,
each with the recorder's global hooks still installed. They are what the thirty-second hang looks
like when it never ends, and they were invisible: the child's `MARCO_HOME` is the test's temporary
directory, so the run record it reserved went away with the test that made it. They also LOCK the
repo-root `marco.exe`, which is how they were found — `withAMarcoBeside` had been skipping every
Play test in Studio's suite because the binary could not be rebuilt. Stopped, rebuilt, and the
suite then ran with no skips at all and left no child behind. One clean run is not a fix and
nothing here claims the cause; what is new is that the symptom has a name and a way to look for
it.

**Physical recording capture remains NOT EXERCISED.**

See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

### §43 — the gate answers for what did not run

`cec9e65` closed the three boundaries; this is the gate the Architect asked for before sign-off,
and it starts from the thing I found while running the last one.

**Eight `marco do` processes were still running on this desktop**, from test runs between 18:57
and 21:30, each with the recorder's global low-level hooks installed. They are the thirty-second
hang with no end to it. And they were invisible twice over: the child's `$MARCO_HOME` is the test's
temporary directory, so the run record it reserved went away with the test that made it — there was
nothing left to read — and the only reason they surfaced at all is that a running binary cannot be
overwritten on Windows, so a build failed. That lock was the other silence: `withAMarcoBeside`
refuses to spawn a stale `marco.exe`, so **Studio's suite had been skipping every test that presses
Play**, printing `ok`.

Each silence explained the other, and `go test ./...` was green through both. That is the whole
argument for this slice: a skip is green, and nothing looks for a child.

#### Three questions, and `go test` asks one

`cmd/gate` runs the same suites and then asks the other two.

**A skip is tolerated only when `gate-skips.txt` declares it**, by package and test, with a reason
somebody wrote. An undeclared skip fails. So does a STALE declaration — one whose test ran and did
not skip — because the entries are read as the gate's coverage, and an entry naming a renamed test
is an exemption doing nothing while the renamed test's next skip arrives undeclared. A reason is
required: a line that names a test without one is an error, because that is the shape a silent
exemption takes. And a reason beginning `BROKEN:` marks a declared skip that is a DEFECT rather
than a decision — counted and printed apart on every run, green or not.

**A surviving product binary is a finding**, established by comparison and identified by PATH. A
gate that failed on "a `marco.exe` exists" would fail for the owner having their own Marco open; a
gate that failed on nothing is the one that let eight of them accumulate. Everything about a
survivor is written down BEFORE anything is done about it, which is the lesson of the eight. And
whether it may be ENDED is a second question: below this process or an orphan, yes; `marco do`
somebody started in their own shell is reported and left alone, with the sentence naming the
parent that is still running.

The harness asks after EVERY scenario rather than once at the end — attribution and containment —
and copies that scenario's run records out of the store it is about to delete. A scenario that
passed and left a child running becomes a finding, with its own evidence kept beside it.

The settling bound is twenty seconds, which is Marco's own number: three bounded waits of five
seconds each in a recording's shutdown, and the process still has to exit after them.

#### What the first run found, which is what it is for

- **Three helper-process skips nobody had declared** in `plugins/overlay` — the branch that IS the
  spawned child, declining when it is the parent. Structural and now written down.
- **A real test three files from its cause.**
  `TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` was skipping because the two
  bounded-shutdown tests I added in §42 abandon a recording — which sets `recWedged` for the life
  of the process — and neither put it back. `Start` then refuses, and the arm test declines on the
  refusal. Fixed by giving both the package-restoring helper; the recorder package now skips
  nothing.
- **Three gates that no longer gate**, parked as Roadmap 37. Two in `internal/director/observe`
  and one in `cmd/director` build a scenario, find it did not produce the condition they are
  about, and decline — one of them under a doc comment reading *"deleting `g.unsettled++` must
  fail this"*, which it cannot. Declared `BROKEN:` rather than fixed: deciding whether the
  behaviour still exists to be gated is perception work, not alpha work.

#### And the stages have an order

`gate.ps1`: the suites, the packed bundle, the bundle INSTALLED from an isolated directory, then
every registered scenario against those installed binaries. Each stage is worth something only if
the one before it passed, and the desktop half is last because it is the only part that takes
somebody's screen. `-Only` prints as a partial run rather than the gate.

Stage 1 builds `marco.exe` and `director.exe` first. Not a convenience: it is the reason Studio's
Play tests decline, so a gate that did not build them would be a gate whose most load-bearing
tests were silent.

**The first `gate.ps1` never ran a stage**: a box-drawing character in a BOM-less `.ps1`, which
Windows PowerShell 5.1 reads as ANSI. ASCII only, now, and the trap is in CLAUDE.md.

#### What it reports, run twice

Once from the working tree and once from the committed one, so the result is reproducible from a
clean checkout rather than from whatever was on disk:

```
1/4  suites    0 failures, 0 findings, 30 tolerated skips, every one declared        4m59s
2/4  pack      dist\Marco.exe  version 88de884-20261028005038
3/4  install   from an isolated directory: no problems, every Go binary at 88de884
4/4  harness   desktop 23 of 23, recording 1 of 1 -- all 24 registered scenarios
                                                                          took 06:52
```

The second run's bundle says `88de884` with no `+dirty` against any of the five Go binaries,
which is the thing that makes the number worth quoting: the binaries that answered are the ones
this commit builds. **No scenario left a process running**, and nothing was on the desktop
afterwards.

The first eleven-scenario run reported in §42c was a selection. This is every registered
scenario, and the twelve it adds are the guarded drag in four of its forms, the chord, the three
holds, the right and middle press with its moved and resized refusals, and the recording path's
twenty-four scripted-recorder tests.

**The eight orphans' cause remains unresolved.** One clean run does not close it and nothing here
diagnoses it. What changed is that a recurrence is exposed and contained rather than silent.

**Physical recording capture remains NOT EXERCISED.**

See [[ADR-264-the-gate-answers-for-what-did-not-run]].

## 44. Four gaps in the gate's own failure paths

The gate passed. The second review read its failure paths instead of its result, and found four
places where the gate would be wrong about something it had never been asked to be right about.
All four are closed, each with a bounded automated case rather than another walkthrough.

### An orphan is not necessarily yours

`OursToEnd` authorised a kill because the parent had disappeared. That is not ownership. It is the
absence of somebody else's claim, and the two come apart in a case anybody can produce by
accident: start `marco do` in a shell while the gate is running, then close the shell. The process
is orphaned, it is at a watched path, it appeared during the run, and it is **not the gate's to
kill**. Nothing about it distinguishes it from a child of this run that lost its parent, which is
exactly why absence cannot be the test.

So ownership is positive only, in a new `internal/gate/ownership.go`: a pid is this run's when
this run started it, or when it was **seen below something this run started while that chain was
still intact**. Everything else is reported, preserved and left running.

**Which forced the harder half.** The reason the orphan clause was there at all is that the chain
breaks: a Studio exits and the `marco` it spawned carries a parent id naming a process nobody can
ask about any more, with the test binary above it gone too. Afterwards there is no path from the
gate to the child — that is precisely the shape of the eight — so nothing can be deduced from the
wreckage. The lineage is therefore **sampled while the run is happening**: `Ownership.Note` is
given a snapshot every two seconds beside each suite and beside the whole harness run, and
remembers whatever is then below a pid already known. A process whose entire ancestry both
appeared and died between two samples cannot be established and is left running with the report
saying why. That limit is stated rather than papered over.

`Descendants` is gone, replaced by a walk UP from the survivor, which is the direction the
question is actually asked in.

### Packaging could fail while the gate carried on

`pack.ps1` sets its own `Stop` preference and reports a bad bundle by **throwing** — a missing
`director.exe`, a Studio built without the Wails tags, a binary carrying no VCS revision.
`$LASTEXITCODE` then holds whatever the last native command that SUCCEEDED left, which is zero.

I probed the mechanism before changing anything, and it is one step over from the report: a plain
`throw` does terminate the caller, even under `Continue` — but it terminates it with a raw
exception and **no stage table and no verdict at all**, so a run that died in packing left nothing
behind saying where it got to. The hole the report describes is real for the shapes either side of
it, both reproduced: a `pack.ps1` that is **missing** (`CommandNotFoundException` is
non-terminating, `$LASTEXITCODE` keeps its stale zero, and the script carries straight on), and a
`pack.ps1` that swallows a failure of its own with `-ErrorAction SilentlyContinue` and exits 0. In
both, stage 3 installs and stage 4 gates **whatever `dist\Marco.exe` was already there** — which
could be from any build at all, while stages 3 and 4 are the entire provenance claim of the alpha.

Packing is now asked three ways, because each catches a different shape of the same failure: the
exception is caught, the exit code is checked, and — the only one of the three that is actually
evidence — the bundle on disk must be one **this run** produced. Two arms there too: written
during the packing, or at least not the file that was there before it began, because `go build -o`
dates its output now while `Copy-Item` PRESERVES a write time. Writing that check found my first
version of it rejecting a legitimately new bundle, which is what the control case is for.

And there is now exactly **one place the script can end**, so a failing stage cannot exit past the
stage table, and it exits with that stage's own code.

### Evidence is persisted before anything is terminated

`lookForSurvivors` accumulated the description in memory, killed the processes, and wrote the file
afterwards — with the write's error discarded. So the one run that mattered was the one that lost
everything: a gate whose diagnostics directory could not be written would have killed every
survivor and left exactly the silence the eight were found in.

The description is now committed and the write **checked** before anything is ended; what was
ended or left running is appended to the same file afterwards. A failed write means nothing is
touched at all, and the finding says so. The same rule was wrong the same way in the harness,
where `artifacts.keep` returns `""` on failure and the sweep went on regardless.

### A partial run no longer gets the full-success sentence

`-SuitesOnly`, `-Installed` and `-Only` all ended with "the suites, the packaged bundle, its
installation, and every registered scenario" — a claim about a bundle the run never packed and
scenarios it never selected. Each switch now adds a line to the verdict naming **what the run did
not ask**, and a narrowed run ends with `PASSED WHAT IT RAN, WHICH WAS NOT THE GATE`. The pass is
real; what it was a pass OF is the part that was being overstated.

### What holds it

Nine new or rewritten tests, and **15 of 15 mutations killed**:

- `internal/gate` — ownership established while the chain is alive and not after; an unrelated
  orphan left alone; the same orphan claimed once the run can show it started what started it; a
  sweep with no ownership ending nothing.
- `cmd/gate` — nothing ended when the diagnostics cannot be written, asserted against a **real**
  child that is still running afterwards; an orphan nobody can account for written down as
  `LEFT RUNNING`; and the wiring, by walking the command.
- `cmd/gate/script_windows_test.go` — the **real `gate.ps1`**, run against a stand-in tree with a
  `pack.ps1` written per case: one that throws after a successful native command, one that reports
  success and produces nothing, a narrowed run, a suites-only run, and a whole run as the control
  without which "not the gate" could be printed unconditionally forever. No window, no screen,
  about a second each.
- `plugins/e2e` — nothing ended when the artifacts cannot be kept; `main` starts the lineage
  watcher exactly once.

A sixteenth was removed rather than killed: seeding the walk with the survivor's own parent id
is unfalsifiable, because a survivor always comes out of the snapshot it is compared against. It
is now a stated contract instead of an untestable line.

**What is unchanged.** The eight orphans' cause remains unresolved, and this makes containment
narrower rather than wider: a recurrence whose ancestry this run cannot establish is now reported
and **left running** instead of killed. That is the safe direction, and it is a change in what the
gate does, not only in what it says. Physical recording capture remains NOT EXERCISED, and
reliable Stop delivery during the sharing conflict remains unestablished.

See [[ADR-264-the-gate-answers-for-what-did-not-run]] and [[Desktop-Harness]].

## 45. A pid is not a process

The fifth finding, and the same mistake as the fourth one level down. Ownership had just been made
positive only — but what it remembered was a **bare pid, indefinitely**. Windows hands a pid back
out once the process holding it has gone, soon and from a small table, so during a gate that
spawns a Studio, a director and a `marco` per scenario the same number comes round again. The
label outlived the thing it was attached to. A run that started pid 7384, watched it exit and
later met pid 7384 would have claimed somebody else's process — **and everything below it**,
because the walk upwards stopped at the remembered number and called the whole line its. Walking
up rather than down makes no difference to this: it is the number that is wrong, not the
direction.

So what is remembered is an **identity**: the pid *and* the moment that pid started running, read
through `GetProcessTimes` by the same path for every use of it, so two readings of one process
compare exactly equal and two processes sharing a pid never do. An identity with no start time is
not an identity and is never equal to anything, including another unknown one.

Three consequences, every one of them a narrowing:

- **A process whose start time cannot be read is not identified**, so it is not claimed and not
  ended. There is no fallback to the bare pid; that fallback is the defect.
- **Every step of an ancestry walk is checked, not just its ends.** A parent has to be in the
  snapshot with the identity this run established, and a child cannot have started before its
  parent — which is what rules out a parent id naming a *replacement* for the process that really
  did the starting. And a line through a parent that has **already exited** is no longer followed
  at all, because there is nothing left to identify it by. What carries ownership across that gap
  is the watcher, which claims the child in its own right while its parent is still alive; by the
  time the parent has gone, the child's own identity is what answers. That case has a test of its
  own, because it is the one the whole sampling design exists for.
- **The kill checks the handle it is about to use.** The pid came out of a snapshot, and a
  snapshot is a photograph: between the photograph and the kill the process can exit and its
  number be handed on. `gate.End` now opens the process **once**, reads the start time from that
  handle, compares it to the identity written down, and terminates **that same handle** — a handle
  names a kernel object rather than a number, so nothing can be swapped underneath it.

And it takes a `gate.Identity` rather than a `uint32`, so the old call cannot be written any more.
That is the strongest form this could take: the compiler refuses a kill by number, in this package
and in the harness, rather than a test noticing one.

### What holds it

Three new tests and two rewritten, and **11 of 12 mutations killed**:

- `internal/gate` `TestAReusedPidIsNotTheProcessThisRunStarted` — a pid this run started and saw,
  then exited and given to somebody else's shell: neither the replacement nor the `marco do` under
  it is claimed, watching learns nothing from the snapshot it appears in, and a process *older*
  than the process its parent id names is refused as well — the same reuse in the middle of a
  line, and what a walk that checked only its ends would have claimed.
- `internal/gate` `TestAProcessWithNoStartTimeIsNotEnded` — reported, left running, and the
  sentence says it cannot be told apart from another process given the same pid.
- `internal/gate` `TestTheProcessEndedIsTheOneThatWasWrittenDown` — against a **real** process,
  because the handle is the whole question. There is no way to make Windows reuse a particular pid
  on demand, so the condition is produced from the other side, which is the same condition as the
  code sees it: an identity whose start time is not the one on the handle is refused and the
  process is still running afterwards; a bare pid is refused too; and then the identity that *was*
  written down is ended — without which every refusal above is satisfied by an End that ends
  nothing at all.
- `plugins/e2e` `TestThisRunRemembersWhatItStarted` — rewritten against a real process this run
  starts: its identity is remembered, the same pid with a different start time is **not** it, and
  a child seen below it stays this run's once its parent has gone.

One mutation survived and was recorded rather than contrived against: dropping the "no start time"
arm of the watcher's own guard changes no behaviour, because an unidentifiable process has a zero
start time, which is before every parent time there can be, so the ordering test refuses it one
line later. It stays, with a comment saying exactly that — the same treatment `Watching.has`
already gives its own unfalsifiable check, for the same reason.

**What is unchanged.** Containment is narrower again, deliberately: a survivor this run cannot
identify, or whose ancestry runs through a process that has already gone, is reported and **left
running**. The eight orphans' cause remains unresolved, physical recording capture remains NOT
EXERCISED, and full packaged acceptance of this tree has not been run.

See [[ADR-264-the-gate-answers-for-what-did-not-run]] and [[Desktop-Harness]].

## 46. Say what you want, and the resolver that was never being called

The first product slice after the release-gate work, and the correction in its title is the part
worth reading.

### The resolver was complete, installed, and dead

`internal/dispatch` has had all of this for a long time: a `Dispatcher` with a documented policy,
an `Advisor` interface, a `PluginAdvisor` that shells out over a tiny JSON protocol, and a
`validate` that accepts a `run` only when the proposed route is one the caller offered.
`plugins/llama` answers that protocol against a local Ollama. `setup.ps1` pins `$MARCO_RESOLVER`
and `$MARCO_ASSISTANT` at the built plugin. On a set-up machine it was configured and ready.

Its callers were `marco dispatch "<phrase>" --json` and `converseTurn` in the `marco assistant`
REPL — a developer surface the file itself says nothing launches. **The widget never called it.**
The overlay's one door is `marco do --source=typed <phrase>`, whose play lookup is exact and
deliberately not fuzzy; anything else goes to Director. The legacy audit had already filed
`$MARCO_RESOLVER` as a zombie and nobody had acted on it.

So this slice reuses an existing resolver **that the widget was not actually calling** — the
Architect's own correction to the brief, kept here because the shape is this repository's most
repeated defect: complete code with no caller, invisible from the outside.

### Which is why a decision now says who answered

Two of `Decide`'s three arms need no model at all. An exact name runs offline; a near miss asks
*did you mean*. Both are indistinguishable, in their answers, from the Advisor having decided — so
a window built on this could work, look right, be demonstrated, and never once reach Ollama.

`Decision.Source` is now `exact` / `advisor` / `fallback` / `none`, and `Decision.Advisor` is
`absent` / `answered` / **`silent`**. The third is the state that used to be invisible: configured,
asked, and with nothing to say, which on a machine where Ollama is down looks exactly like a
machine where nobody configured anything. Studio renders both beside the request. It does not
claim *which* kind of silence, because `Advise` reports "I have nothing" for a declining plugin, a
missing plugin, a dead server and an unparseable reply alike, and inventing a distinction would be
a guess about somebody's machine.

### The target application is stated, never inferred

`Acts.SuggestRoutine(request, target)` reads a request against the three scopes: a global routine
always eligible, an anywhere-in-application routine eligible from wherever you are because it
brings its own application forward, and an in-place routine offered **only** when its application
is the stated target.

Two inferences were available and both are wrong. Studio owns the foreground whenever somebody
types into it, so reading the foreground answers "Studio". And treating *this application has a
window* as eligibility would make every running application's in-place commands reachable from
anywhere — which is what anywhere-in-application already means, so it would erase the difference
between the two app scopes. With no target stated, no in-place routine is offered and the answer
says so. Whether a target is running is a **hint and not a filter**: Play raises it, and the
choice is the person's.

### A label is offered and an identity comes back

The resolver is handed slug-shaped **labels**, not identities. An identity is path-shaped and the
deterministic matcher strips non-alphanumerics *without inserting a space*, so
`ark/context/settings` reduces to `arkcontextsettings` and matches nothing anybody would type.

Uniqueness is checked on `nlu.Key` — the function `nlu.Resolve` itself calls, now exported for
exactly this — because two labels that differ as strings can be **one candidate** to the matcher: a
hyphen is a space and `in` is a stop word, so a derived `settings-in-ark` collides with a routine
genuinely slugged `settings-ark`. A second copy of a normalization is how a surface comes to
believe two labels are distinct while the matcher cannot tell them apart.

And every routine claims its own slug **in a pass of its own** before anything derives a label.
One pass gave whoever the listing put first whatever it asked for, so a routine really called
`settings-in-ark` could lose its own name to a label derived for `settings` saved in Ark. Both
stayed distinct and both mapped correctly — it was arbitrary on the listing's order, which is the
kind of thing that is right in the test and wrong on somebody's machine. The mutation gate found
that: two label mutations survived the first run only because of ordering, and the fix was the
code rather than the test.

The returned label is a key into a map and is **never resolved again**. The whole identity is what
Play is given, and Play is unchanged.

### What holds it

Eleven new tests, and **18 of 18 mutations killed** across the scope boundaries, the labels, the
observability and the page.

The page ones render the real `index.html` in headless Edge and assert what the window does with
an answer: the request and the target arrive together, both targets are offered with "not running"
shown, the named routine's card is **selected**, `PlayRoutine` is called **zero** times, and the
window says which arm answered — including the silence, and including *not* reporting a silence
when nothing was configured.

And one test asks the real plugin and the real model server, because a stand-in Advisor
establishes nothing about the connection and a suite of stand-ins is exactly what the dead
resolver already had. *adjust ark video quality* → `arkascended/focus/ark-graphics`, in 0.8s warm.
Declared in `gate-skips.txt`, so a machine where nothing was asked says so instead of printing
`ok`.

### What the probe found, including what it could not explain

The label-echo probe ran before any of this was written, because converse mode matches a route
name **exactly** with no token fallback: a model that paraphrases the label it was offered is
discarded safely and silently, and silently for exactly the ambiguous case the labels exist for.

Disambiguated slug-shaped labels echo exactly, 6/6 warm, ~0.5s each. Of seven phrasings, six
resolved identically three times each. The seventh — *make the visuals in the dinosaur game look
nicer* — is read by `llama3.2:3b` as a request to teach a new command, 3/3. That reached Ollama
and came back; it is the model's reach and not a defect, and it is why the live test is a
demonstration that the connection works rather than a claim about comprehension.

One thing is **not explained**. The very first converse call of the session returned nothing at
all, which degraded to the deterministic arm exactly as designed. A cold load after
`ollama stop` measured 2.6s and worked, and it did not recur in sixteen later calls, so the cause
is unestablished and recorded rather than guessed at.

### Two corrections before it closed

The review read the two things a passing suite could not see.

**An exact match said the resolver had answered it.** `advisorState` returned `AdvisorAnswered`
whenever an Advisor existed, so a free offline match on an exact name printed *the configured
resolver answered this* about a resolver that was never consulted. That is a fact about what is
INSTALLED standing in for a fact about what HAPPENED — the same class of mistake as the gate's
orphan — and it is the one direction nobody would have noticed, because it flatters the feature.
There is now a fourth state, `not-asked`, `advisorState` never answers `answered`, and each arm
says what actually became of the Advisor from there. The exact-name test asserts the sentence a
person reads, not only that the Advisor was asked zero times.

**A shared name still had an arbitrary winner.** Unique labels stop two routines being confused
for one; they are not an answer to the PERSON's ambiguity, and the first version mistook the one
for the other. The first eligible `settings` kept the bare name, so a request for "settings" was
an exact match on that label: answered offline, selecting whichever routine the listing happened
to put first, with neither the resolver nor the person asked which of three was meant. An
ambiguity resolved by listing order and reported as an exact match.

Now every routine sharing a normalized name is qualified and the bare name belongs to nobody. A
request for it matches no label, so the resolver is asked with the target stated; if nothing
resolves it, what would otherwise be a near miss pointing at an arbitrary one — or an offer to
RECORD a third routine of the same name — becomes a question naming the alternatives, in
`plays.Scope.Says`, the product's own words for the three scopes. Held in both listing orders, and
through the real surface with two namesakes saved by the product's own path.

**23 of 23 mutations killed** after the corrections. One more survivor turned up on the way: no
case exercised two labels that differ as strings and are ONE candidate to the matcher
(`settings-in-ark` beside a routine genuinely slugged `settings-ark`), which is the only thing the
`nlu.Key` check is for. That case is now a test.

### And the resolver is now told which application

One wiring omission outlived both corrections: `SuggestRoutine` passed `""` as the Advisor's
context, so the target filtered the candidates and appeared in the window and **Ollama never
received it**. Studio knew which application was meant and the resolver did not — and the
ambiguity answer added in the same round says a shared name is "asked about with the target
stated", which was not true of the asking.

The reasoning for passing nothing had been written for a different question. An application Studio
worked out for *itself* would be a fiction, because Studio owns the foreground whenever somebody
types into it. But this one is not worked out: it is the one the person chose, and it is exactly
what the resolver is best placed to use. It is context for the question, not another filter on the
answer — eligibility is unchanged, so a global routine and an anywhere-in-application one are
still offered whatever is chosen.

Held by a test that captures the Advisor's `app` argument: it equals the chosen target, and with no
target stated it stays empty rather than being filled in with something invented. Two mutations
gate it — passing nothing, and passing something that is not the target. The live test against the
real model was re-run with the target now in the prompt and still returns the whole identity.

### Not in this slice

Overlay wiring — the widget still has its exact-only door, and giving it this is a separate
change with its own question, since the overlay does not own the foreground. Also out: any change
to `internal/invoke`, a second execution path in Studio, a new scope, and argument extraction.

See [[ADR-265-say-what-you-want-and-studio-offers-the-routine]], [[Plays]] and [[Invocation]].

## 47. One set of rules for keeping a recording

The consolidation that should have happened when Studio gained drafts, done now because the two
things it was holding apart were both measured.

### What the probes found

`marco learn` and the widget never used Studio's draft workflow. They had a second implementation
of the same pipeline — start the recorder, wait, simplify, write — and only Studio's copy had the
rules a save has to obey. Two consequences, both reproduced before anything was changed:

**A collision overwrote, silently.** `marco learn "the same name"` twice with different
recordings: the second replaced the file, the library still held one routine, and the output said
`Learned …` as though it were a fresh save. `saveTaught` went straight to `Reg.Save`, which
truncates. Re-recording is what somebody testing a routine does all afternoon, which makes the
likeliest casualty the routine they were comparing against.

**Two of the three scope answers produced a routine that refuses every press.** With a
window-checking click recorded, `c` (only here) and `g` (anywhere) both saved, and
`Deps.Unrunnable()` — the same reader Studio greys the Play button with — reported the routine
that had just been written.

One expectation was wrong and is recorded as checked: `LearnAuto` **does** refuse an
unrepresentable gesture. Not a gap.

### The shape of the fix

The enforcement moved **down** into `orchestrator.Keep`: whether a scope word means anything,
whether this draft can be saved under the scope asked for, what is already where it would go, and
whether a replacement is the one that was agreed to. `Learn` and `LearnAuto` are thin adapters
over `Begin` → `Finish` → `Draft` → `Keep`, and Studio's `keep` now calls the same function
instead of holding the checks itself.

Each surface supplies only the interaction. Studio shows a card. `marco learn` names the exact
application, scope and route — `notepad/context/the-same-name` — and asks, with **Enter meaning
no**, because the two answers do not cost the same: no costs a second attempt and yes costs a
routine and the demonstration beside it. The widget's unattended path **refuses** a collision
outright, names what is there and preserves it, because it saves with nobody having looked.

A replacement is re-derived and re-checked inside `Keep` at the instant of writing, so a routine
deleted or saved again while the question was open is refused rather than overwritten. The test
for that interleaves for real: the reader removes the routine as the `yes` is read.

### What was deliberately NOT shared

**The capture policy.** Studio states coordinate mode; the terminal and the widget state nothing
and leave the hooks to the environment, which is what every caller before Studio did. Routing them
through `Begin` preserves that exactly, so their routines still state no mode — truthfully. This
is shared orchestration and shared save rules, and not an identical capture configuration.

**The owner strip, which the suite caught me nearly flipping.** A window's rule is that the owner
comes out wherever it is, because a window is glanced at mid-demonstration; the terminal's is the
tail of a button-ended recording only, and `TestAGestureEndedRecordingIsNotStripped` exists
precisely to record that changing that would be a separate decision. Putting both paths on one
pipeline changed it in passing and three tests failed. It is now a stated field,
`Deps.OwnerTailOnly`, defaulting to the window's rule because that is what `Finish` has always
done — I had the default the wrong way round on the first attempt, and the draft tests said so.

**`saveTaught` and `chooseScope` are retained** for `LearnVoice`, which is out of this slice.

### And one loop bounded before it shipped

The terminal's first save re-asked for a scope when the one it got was impossible. `readLine` on a
closed stdin answers `""` for ever, so a terminal whose input had gone would have sat asking
itself the same question until somebody killed it. It refuses and returns instead.

### What holds it

**13 of 13 mutations killed**, and the tests enter through `Deps.Learn` and `Deps.LearnAuto` — the
production entrances — rather than through `Keep`. That matters here more than usual: both
surfaces had `Save` underneath them already, so a test that exercised the rule without entering
where a person does would have passed for the whole time the rule was missing.

Seven cases: the collision through all three answers with the library counted; the unattended
refusal; the interleaved replacement; the scope for all three terminal answers and the unattended
path, with an ordinary recording as the control so none of it is satisfied by a surface that has
stopped honouring anybody's choice; the mode stated to the recorder and read back off the file;
and Cancel leaving nothing in the library at all.

### Two overwrite protections the consolidation still needed

The rules were shared and both of them were still short of what a save contract has to promise.

**Consent identified a path, not the version approved.** A routine's identity is its place in the
library, and a place does not change when its contents do — so `Keep`'s "same route, same id"
check was true of a routine that had been saved again while the question was open, and that newer
version was overwritten. The deletion case was covered; this one was not. Consent now carries
digests of the source **and** the stored demonstration as they were when somebody was shown them,
and both are re-read at the instant of writing. `Deps.Replace` already worked this way for a
regeneration; this is the same rule for the other way a routine gets written over.

The demonstration is half of it, not an extra: a routine re-recorded from the same steps has the
same source and a different recording, and the recording is the only thing a rebuild can be made
from. A mutation that checked the source alone survived the first gate run, which is how that
became a case.

**A fresh save could still overwrite a concurrent one.** `Keep` asked whether the path was free
and then wrote with create-or-truncate, so another writer landing in the gap was overwritten by a
caller that had been told the path was free. The gap is as long as whatever happens in between,
which on an interactive surface is as long as somebody takes to answer. `routes.Registry.Create`
is new and uses `O_CREATE|O_EXCL`, which closes the gap rather than narrowing it: the test and the
write are one operation. A replacement goes through `Registry.Replace` instead, which is atomic —
the one case where there is an original to lose.

And `Collision` treated every `Stat` error as absence, so a path that could not be read answered
"free". Only not-found is absence now.

### What the gate said about the collision check afterwards

Deleting the collision refusal from `Keep` now **changes no behaviour**: the exclusive create
refuses the same case and reports it with the same sentence. So that arm is recorded as redundant
rather than as a kill, and the mutations that bite are the ones on the write. It stays because it
is where the sentence comes from one syscall earlier, and because a surface offering Replace needs
the refusal before it has tried to write. Said in the code, so nobody reads it as the thing
holding the line.

### And one thing that is NOT covered, with the probe behind it

The non-not-found `Stat` branch cannot be reached by anything a test can arrange on Windows. Both
arrangements were measured: a directory in the routine's place makes `Stat` **succeed**, and a
file where a parent directory should be reports path-not-found, which `os.IsNotExist` answers true
for. So the guard is right and its mutation is unfalsifiable here — the same standing as
`Watching.has`'s empty-path check — and the test that was named for it is renamed for what it
actually arranges: a directory where the routine should be, which is occupied, and not written
through.

### What holds it

**20 of 20 mutations killed** across the two gates, after re-anchoring the first one onto the new
signatures.

The fresh-save case is tested twice on purpose, because the two gaps are different sizes. The
production-path test drives the realistic window — another writer creating the target while the
scope answer is being read — and the collision check catches it there. The gap between `Keep`
looking and `Keep` writing has no prompt inside it and no seam to stand in, so that one is
**raced**: 240 iterations with another writer creating the target, 193 of which it won, asserting
in every one of those that its bytes and its demonstration are untouched. It cannot fail wrongly,
because the assertion is only made about an iteration where their create succeeded.

Studio's page now hands back the version it was shown, which is what the digests on `Collision`
are for, and there is a Studio-level case for a routine that changed while the card sat on screen.

One thing the suite caught me breaking on the way: `Keep` had grown its own `Target` call without
`Save`'s `app == "" → dr.App` fallback, so a focus routine recorded from inside its own
application came out with no Activate — a routine that types into whatever happens to be in
front. `TestAFocusPlayActivatesEvenWhenTheDemonstrationDidNot` said so, which is what it is for.
There is one preamble now, shared by `Save` and `Keep`, and a mutation on it.

See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].
