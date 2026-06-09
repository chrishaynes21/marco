# Handoff — Marco (feat/host-ffi)

State as of this handoff: **all green** (19 Go packages pass), gofmt clean, branch
`feat/host-ffi` (unmerged). Binaries built and current: `marco.exe`,
`marco-macros.exe` (repo root), `plugins/overlay/overlay.exe`. The engine has **zero
external deps**; the overlay (ebiten) and voice (vosk) plugins are separate modules.

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
  spawning). Behavior lives in `programs/overlay.marco`.
- **Run stack:** `marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay programs/overlay.marco`,
  with `voice.exe | …` piping Voice events into serve's stdin.

## Pending / known issues / open decisions

- **URL/colon edge:** `routes.ParseInvocation` reads a leading `word:` as a named
  arg, so `go to http://…` mis-splits. Not yet guarded. (Flagged in README/E2E.)
- **Auto-pop needs a saved route:** `marco args` resolves against saved routes, so
  labels don't appear until a route exists. Fine for normal use.
- **Voice wake word:** the vosk plugin may require the wake word per phrase; a
  continuous-listen mode during narrate-teach (so phrases stream until "done") is NOT
  built — for hands-free, that's the next voice-plugin change.
- **macOS/Linux:** `winctx`/`screen`/`recorder`/`secrets` have stubs only; backends
  are additive work.
- **Claude-vision anchor auto-crop:** deferred opt-in plugin (anchors are
  `MARCO_ANCHORS=1`, whole-region capture; user crops manually for now).
- **Arg key / secret-name list** are conventions (F9; password/pin/token/…) —
  overridable later; `MARCO_ARG_KEY` exists, the secret-name list is hardcoded in
  `codegen.isSecretArg`.

## Env vars

`MARCO_ROUTES`, `MARCO_STOP_KEY` (f12), `MARCO_ARG_KEY` (f9; `off`), `MARCO_ANCHORS`,
`MARCO_RESOLVER`, `MARCO_BIN`, `MARCO_OVERLAY_IDLE`, `MARCO_VOICE_WAKE`,
`MARCO_NO_PANIC_STOP`/`MARCO_NO_TEACH`/`MARCO_SIMPLIFY_SAVES` (set by the overlay).

## Not done / explicitly out of scope this session

No commit/push was made (commit only when the user asks). No merge of
`feat/host-ffi`. No new dependencies added to the engine.
