# CLAUDE.md — start here

**Marco** is a sentence-driven automation language + an assistant that learns what you
show it: you name a command in plain words, demonstrate (or narrate) it once, and it's
saved as a small editable Marco program that drives real input via a host-FFI boundary.

## Read first (in order)
1. **`docs/AI-CONTEXT.md`** — how to navigate the knowledge base. Then `docs/Director.md`
   (system map) → the relevant `docs/subsystems/*.md` → its linked ADRs in
   `docs/decisions/`. **Don't load the whole vault**; read the map, one or two subsystem
   notes, and their ADRs.
2. `HANDOFF.md` — the chronological narrative of what was built when. Treat it as
   **navigation, not architectural truth**: where it disagrees with a subsystem note or an
   ADR, the note and the ADR win.
3. `README.md` — user-facing features and commands.
4. `E2E.md` — manual test walkthrough (run this on the live overlay to catch what the
   Go suite can't).
5. `spec/` — the language reference; `spec/Hosts.md` — the host-FFI design.

## The docs vault
`docs/` is an Obsidian-compatible vault of plain Markdown — greppable, diffable, no
dependency on Obsidian. Canonical notes carry frontmatter (`type`, `status`,
`source_paths`) and deliberate `[[wiki links]]`. The existing `docs/director-*.md`
milestone records were not moved; subsystem notes index them.
- Validate with `go run ./cmd/docscheck` — broken links, duplicate note names, stale
  `source_paths`, missing frontmatter. Read-only; it never rewrites a note.
- After changing behavior, update the subsystem note, the relevant ADR, and the
  experiment record. An ADR needs an **Enforced by** entry naming a real test.

## The language governance rule

**Director may grow intelligence. Marco may grow expressiveness only when Director needs a new
legal effect or orchestration primitive that cannot already be expressed clearly.** Perception,
semantic memory, hypotheses, evidence, learning, planning and verification are Director's
business and are not Marco nouns — not because they could not be represented, but because a
Marco program is a play somebody reads.

- `spec/Core.md` is **the** language. Every other page in `spec/` declares `status:` —
  `normative`, `reference`, `historical` or `experimental` — so a future session can tell "this
  is Marco" from "this was an idea for Marco" without reading the code.
- Generated Marco must stay inside Core and must not name a backstage concept. Director may
  know WHY it chose an action; Marco says WHAT it intends to do.
- New syntax needs a demonstrated language-level need, not an implementation concept that could
  be represented in the language. Every permanent word is a cost paid by every future reader.
- `this's` / `that's` are settled. Do not redesign them and do not add another pronoun.
- `act`, `scene` and `actor` are settled and distinct: an act is the way in, a scene is where
  things happen, an actor is a thing in the play. They share one representation on purpose;
  only an act offers capabilities outwards, and `internal/spectest` holds that line.

**Language work is closed.** Reopen it only when a concrete Director requirement proves Core v1
insufficient — not to make the implementation more elegant.

Enforced by `internal/spectest`: normative examples are compiled, spec pages must be classified,
and generated routes are checked against Core's vocabulary.

## Load-bearing invariants (don't break these)
- **The engine has ZERO external deps.** `cmd/marco` + `internal/*` import only the
  stdlib. Anything needing deps (overlay/ebiten, voice/vosk, resolver/Claude) is a
  separate module under `plugins/`. Don't add a dep to the engine.
- **Low-level hook callbacks must return fast.** The keyboard/mouse hooks
  (`plugins/overlay/controller_windows.go`, `internal/recorder`,
  `internal/platform/navsource`) share the install thread; do screen capture / pipe
  writes / anything slow OFF the hook thread, or Windows silently drops the hooks
  (kills F12). See the memory note of the same name.
- **A hook's message pump must BLOCK, never poll.** A separate invariant from the one
  above, and the reason it needs saying: Windows delivers a low-level hook callback only
  while the installing thread is in a message wait. `GetMessage` is that wait;
  `PeekMessage` + `Sleep(1)` leaves the thread asleep instead, which charges every
  keystroke and every mouse move **on the whole desktop** up to a ~15.6ms quantum. It
  never looks like a bug — the hooks keep working, Windows only unhooks past its 300ms
  timeout — so it survived in two of three hook sites. Shutdown must WAKE the pump (post
  `WM_QUIT`) and unhook from the installing thread, **and the wake must be RETRIED and the
  wait BOUNDED**: a thread has no message queue until it first calls a message function, so
  a post into that window is refused and lost, `GetMessage` then waits for a message that
  will never come, and whoever waits for the pump waits forever. That cost 495s in
  `navsource` and two 600s timeouts in `cmd/marco`, the second only because the fix was
  made at one site and not carried to the other. Enforced by
  `internal/platform/navsource/pump_test.go`, which walks the whole tree for hook sites
  rather than naming them — including the shutdown half, which named one file and is why
  the second site kept the bug. **This is a correctness rule, not a measured latency win**: it
  was found while chasing a desktop hitch whose root cause was never established, and no
  clean measurement has yet attributed any perceived input latency to it.
- **Secrets never land in a route or a recording.** Passwords resolve via
  `do OS's Secret …` at run time from the credential store. Don't text-substitute a
  secret value into route source.
- **LEARN, TEACH and DO are three different words, vertically.** LEARN is the person
  acting while Marco watches and acquires — the whole demonstrate/narrate flow, and the
  only one of the three that is built. TEACH is RESERVED for the mirror image: Marco
  guiding a person through a thing they do themselves. DO is Marco acting on the
  person's behalf. So live acquisition code is spelled Learn everywhere
  (`internal/director/learn`, `internal/voicelearn`, `Runtime.LearnSession`,
  `IntentLearn`, `marco learn`, `director learn`, the overlay's `learn <name>`), and
  `teach` survives only as an undocumented CLI/overlay alias for the muscle memory the
  product shipped with. Enforced by `cmd/marco` `TestNoLiveAcquisitionCodeIsNamedTeach`,
  which names the alias explicitly so it cannot grow. Dated ADR bodies and quoted
  transcripts keep the old word on purpose — they are the record of what it meant then.
- **Cross-platform by construction.** OS surfaces (`winctx`, `screen`, `recorder`,
  `secrets`, `oshost` backend) sit behind interfaces with Windows backends + stubs;
  keep the stub side compiling.

## Build / test / run
```sh
go build ./... && go test ./...        # 73 packages, deterministic
go build -o marco.exe ./cmd/marco
go build -o director.exe ./cmd/director   # REQUIRED beside marco.exe: see below
go build -o marco-macros.exe ./cmd/marco-macros
go -C plugins/overlay build -o overlay.exe .
plugins/studio/build.cmd                 # Studio: NEVER a plain `go build` — see below
plugins/vision/build.cmd                 # Windows: the ONNX backend (needs gcc)
.\overlay.cmd                          # Windows: launches the overlay stack
```
- **`director.exe` must sit beside `marco.exe`.** `marco` finds it by looking next to its own
  executable (`$DIRECTOR_BIN` overrides), and it is not optional: it is what starts the Director
  service, and it is what performs a routine somebody demonstrated — typing the name of a pixel
  routine into the overlay spawns `director perform-workflow`, so a tree built without it reports
  `unavailable` for the half of the product a person made with their own hands. The installer
  already ships both (`cmd/marco` `TestThePackagedLayoutIsWhatMarcoLooksFor`); this line is for a
  dev build, which used not to mention it.
- **A plain `go build` of `plugins/studio` produces a binary that opens NO WINDOW.** Wails
  guards on its own build tags: without `production` it compiles a `CreateApp` that shows a
  message box reading *Wails applications will not build without the correct build tags* and
  exits. The binary is otherwise ordinary — it prints usage, and `-acts` and `-proof` both
  work, because they return before the app is created — so a broken one sat in the tree
  looking current until somebody double-clicked it. Use `plugins/studio/build.cmd`, which
  builds the windowed and console binaries and puts the windowed one beside `marco.exe`.
  **And Studio spawns the `director.exe` AND the `marco.exe` beside ITSELF**, not the ones
  beside each other (`directorBin`, `marcoBin` — the director records, and marco performs a
  saved routine when somebody presses Play on it). With a copy of Studio in `plugins/studio`
  and another at the repo root, that makes a stale sibling invisible: every binary looks
  freshly built, the window is current, and the recordings come from whenever that director
  was last made. It cost the owner a day of recordings. `build.cmd` now makes all of them and
  puts them in both places; `TestTheBuildScriptMakesTheDirectorStudioWillSpawn` holds it.
  **And close Studio before building it.** Studio installs the input hooks and makes the draft
  in its OWN process, so a window left open across a build records through the old recorder
  while every binary on disk is current — the same trap one step further in, and this one no
  check of the files can see. A fix to the recorder or to the exclusion is not on the desktop
  until the window has been reopened; it cost a live trial that reported its own fixed defect.
  To check an existing binary without opening a window on somebody's desktop:
  `grep -a "will not build without the correct build tags" studio.exe` — a correct build does
  not contain that code at all. And to SEE the page without a window, render
  `frontend/index.html` in headless Edge with `window.go` stubbed; the page waits for the
  Wails bindings and paints nothing until they exist.
- **`director capture-game-session` is the L0 recorder** (frames + observed inputs on one
  clock, nothing injected) and `go run ./cmd/perceptionreplay -frames -stride N <dir>` replays
  the pixels through the production perception path. `-frames` is a BOOL and the directory is
  POSITIONAL; putting the directory first silently ignores `-stride`.
- **The full-frame perception corpus test is behind `-tags corpus`** and is REQUIRED for a
  perception change: `go test -tags corpus ./cmd/perceptionreplay/` (about 145s). It is out of the
  default suite because it was starving other packages — `cmd/marco` past its 600s limit, and a
  latency assertion from 962µs to 60ms. `go test ./...` still gates perception through the
  every-tenth-frame test in the same package. See
  [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]].
- **`go build` in `plugins/vision` produces a detector that detects NOTHING.** The ONNX
  backend is behind `-tags onnxvision` and the default build answers "no model loaded" to
  everything, which reads as a missing subsystem rather than an unbuilt one. Use
  `plugins/vision/build.cmd` — it needs gcc
  (`winget install BrechtSanders.WinLibs.POSIX.UCRT`). The runtime and model are
  vendored under `tools/` and **Director's own wiring finds them**:
  `visionChildEnv` hands each child `$MARCO_VISION_MODEL`, `$MARCO_VISION_SIZE=1280`
  and `$MARCO_ONNXRUNTIME`. A bare `vision.exe detect` finds NONE of them and needs
  all three by hand; without the size it refuses with `Got: 640 Expected: 1280`,
  because the vendored model is 1280 and the default is 640.
  `$MARCO_SCREENPARSER_MODEL` only chooses which model path Director passes — the
  plugin itself reads `$MARCO_VISION_MODEL`.
- **Restart `overlay.cmd`** to pick up a new `overlay.exe`; `marco.exe` is spawned
  fresh per command so it takes effect immediately (`$MARCO_BIN` overrides it).
- `marco do` performs **real input** (types/clicks for real). For behavior checks use
  the Go tests or `marco run --host dryrun`, and a temp `$MARCO_ROUTES`. Never run
  `marco do` blind in a test.
- **One library, and it is a place.** `internal/marcopaths` answers where the route store and
  the Marco home are, for `marco`, `director` and Studio alike. Don't derive either in a
  command: each of the four that did was `$MARCO_ROUTES, else "routes"` — a RELATIVE path, so
  the library was whichever directory that process was started from, and a routine saved by a
  double-clicked Studio was invisible to `marco do` run from a terminal. The default is now
  `routes` inside the store and the answer is absolute, including a relative `$MARCO_ROUTES`.

## Working agreements
- Branch is `feat/host-ffi` (unmerged). **Commit/push only when the user asks.**
- After edits, run `go build ./... && go test ./...` and `gofmt -w` touched files.
- Prefer extending the existing patterns (`internal/routes/args.go`,
  `internal/voicelearn`, `plugins/overlay` MVC) over new abstractions.
