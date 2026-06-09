# CLAUDE.md — start here

**Marco** is a sentence-driven automation language + self-teaching assistant: you
name a command in plain words, demonstrate (or narrate) it once, and it's saved as a
small editable Marco program that drives real input via a host-FFI boundary.

## Read first (in order)
1. **`HANDOFF.md`** — current state, what was just built, pending issues, open
   decisions. The single source of truth for "where things are."
2. `README.md` — user-facing features and commands.
3. `E2E.md` — manual test walkthrough (run this on the live overlay to catch what the
   Go suite can't).
4. `spec/` — the language reference; `spec/Hosts.md` — the host-FFI design.

## Load-bearing invariants (don't break these)
- **The engine has ZERO external deps.** `cmd/marco` + `internal/*` import only the
  stdlib. Anything needing deps (overlay/ebiten, voice/vosk, resolver/Claude) is a
  separate module under `plugins/`. Don't add a dep to the engine.
- **Low-level hook callbacks must return fast.** The overlay's keyboard/mouse hooks
  (`plugins/overlay/controller_windows.go`, `internal/recorder`) share the install
  thread; do screen capture / pipe writes / anything slow OFF the hook thread, or
  Windows silently drops the hooks (kills F12). See the memory note of the same name.
- **Secrets never land in a route or a recording.** Passwords resolve via
  `do OS's Secret …` at run time from the credential store. Don't text-substitute a
  secret value into route source.
- **Cross-platform by construction.** OS surfaces (`winctx`, `screen`, `recorder`,
  `secrets`, `oshost` backend) sit behind interfaces with Windows backends + stubs;
  keep the stub side compiling.

## Build / test / run
```sh
go build ./... && go test ./...        # 19 packages, deterministic
go build -o marco.exe ./cmd/marco
go build -o marco-macros.exe ./cmd/marco-macros
go -C plugins/overlay build -o overlay.exe .
.\overlay.cmd                          # Windows: launches the overlay stack
```
- **Restart `overlay.cmd`** to pick up a new `overlay.exe`; `marco.exe` is spawned
  fresh per command so it takes effect immediately (`$MARCO_BIN` overrides it).
- `marco do` performs **real input** (types/clicks for real). For behavior checks use
  the Go tests or `marco run --host dryrun`, and a temp `$MARCO_ROUTES`. Never run
  `marco do` blind in a test.

## Working agreements
- Branch is `feat/host-ffi` (unmerged). **Commit/push only when the user asks.**
- After edits, run `go build ./... && go test ./...` and `gofmt -w` touched files.
- Prefer extending the existing patterns (`internal/routes/args.go`,
  `internal/voiceteach`, `plugins/overlay` MVC) over new abstractions.
