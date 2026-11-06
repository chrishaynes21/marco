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
- **A recording is of ONE WINDOW, THE WINDOW IS CHOSEN, and the gate is at the instant of the
  event.** The window is GIVEN (`Deps.Window`, a handle AND its owner) and `Begin` REFUSES a
  confined recording that was not told which one. Don't infer it: the first version latched the
  first foreground window that was not Marco's, which is whatever came forward first — a
  terminal, a toast, an updater — so the demonstration was admitted from a window nobody picked.
  Membership is decided from ONE reading per event (`whose`) carrying both halves, because a
  second reading is a second instant and the foreground changes between them exactly when
  somebody clicks elsewhere — and the application WRITTEN DOWN comes from the chosen window too
  (`chosenApp`), or a foreground sample taken while somebody was away is recorded as an
  `Activate` once they come back and the routine is filed under their browser. What was already held at Start is scanned and seeded
  (`alreadyHeld`), or a ctrl+click records as a click. The POLLER moves the boundary too, or an
  absence with no input in it keeps its full playback delay. And the pause is SHOWN while it
  happens on both surfaces — silence looks exactly like a recording that is still going.
  `Deps.Watch` states the scope before the hooks go on and its zero value is `OneWindow`, so
  Studio, `Learn` and `LearnAuto` all record only the window the person goes to;
  `orchestrator.WholeDesktop` is the explicit opt-out. Don't turn this into a filter at `Finish`:
  input decided against afterwards is input already in the buffer, in `Draft.Events`, published
  to every consumer of the live channel, and written to disk beside the routine — which is the
  same class of leak as the rule below. The live channel is gated WITH the buffer; the single
  exception is a key in the stop chord, because `StopKey.Triggered` reads nothing else and the
  stop must work from anywhere. And the time spent away comes out of the TIMESTAMPS, never a
  marker beside them: a marker is a kind an older build ignores while keeping the raw gap, so a
  regenerated routine would silently acquire a half-minute pause. `Draft.Events` therefore
  carries no marker at all. `marco learn "<name>" --whole-desktop` is how a person asks for the
  old behaviour, and the scope is printed before recording starts; Studio has no control for it
  yet. See [[ADR-268-record-only-the-chosen-window]].
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
plugins/studio/build.cmd                 # Studio AND the whole widget stack, one commit — NEVER a plain `go build`: see below
plugins/vision/build.cmd                 # Windows: the ONNX backend (needs gcc)
.\overlay.cmd                          # Windows: launches the overlay stack
.\gate.ps1                             # the release gate: suites, a clean checkout, bundle, install, EVERY scenario
go run ./cmd/gate                      # its first stage alone, which needs no screen
go run ./cmd/cleanroom                 # its second: builds the COMMITTED source in a clean clone
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
  puts them in both places. **It builds the widget stack too** (`marco-macros.exe` at the root,
  `overlay.exe` under `plugins\overlay`), because Open widget starts the ROOT stack from either
  `studio.exe` and refuses any piece whose revision is not that Studio's — before, a freshly
  built Studio could never open the widget. It builds the local resolver too
  (`plugins\llama\llama.exe`), which Director finds beside marco, and the microphone
  (`plugins\voice\voice.exe`, with cgo, so it needs gcc and libvosk). It refuses a dirty tree and anything running from a
  destination, stages every build and checks every stamp against HEAD before replacing anything.
  Never copy a piece in to satisfy the check. `TestTheBuildScriptMakesTheWholeStackFromOneCommit`
  holds it; see [[ADR-302-studio-opens-the-widget]].
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
- **`go test ./...` is not the gate, because a skip is green.** Studio’s suite printed `ok`
  while every test that presses Play SKIPPED — the `marco.exe` they spawn could not be rebuilt —
  and the reason it could not be rebuilt was eight `marco do` processes from earlier runs still
  holding the file open, each with the recorder’s global hooks installed. Each silence explained
  the other. So `cmd/gate` asks the two questions `go test` does not: a skip is tolerated only
  when **`gate-skips.txt`** declares it by name with a reason (an undeclared skip fails, and so
  does a stale declaration whose test ran and did not skip), and a product binary running after a
  suite that was not running before it is a finding. **Add a declaration when you add a skip, in
  the same change.**
- **And a survivor is written down before it is touched, and ended only when the run can SHOW it
  started it.** Both halves were wrong once. The description is committed and the write CHECKED
  first, because a process killed with nothing left behind to read is the state the eight were
  found in; a failed write means nothing is ended. And an ORPHAN IS A FINDING, NOT A LICENCE —
  somebody can start `marco do` in a shell during a run and close the shell, and what is left is
  indistinguishable from a child of the run, so ownership is positive only and the lineage is
  sampled while the run is happening. AND A PID IS NOT A PROCESS: Windows hands the number back
  out, so what is remembered is the pid AND when that pid started, every link of the ancestry is
  checked against that, and `gate.End` compares the start time on the HANDLE IT KILLS — it takes a
  `gate.Identity`, so the old kill-by-number cannot be written any more.
- **`gate.ps1` is the whole gate**: the suites, then a CLEAN CHECKOUT of this commit, then the
  bundle packed from this tree, then that bundle installed from an isolated directory, then the
  tests that need the screen, then EVERY registered desktop scenario against the installed
  binaries. Packing is asked three ways, because `pack.ps1` reports a bad bundle by
  THROWING and `$LASTEXITCODE` then holds a stale zero: catch, exit code, and the bundle itself
  must be one this run produced. And `-SuitesOnly`, `-Installed` and `-Only` each end with what
  the run DID NOT ASK, rather than with the sentence a whole run gets. See
  [[ADR-264-the-gate-answers-for-what-did-not-run]].
- **A GREEN WORKING TREE IS NOT EVIDENCE ABOUT A CLONE, so packing is gated on `cmd/cleanroom`.**
  Every instrument here reads the directory in front of you — `go build ./...`, the suite,
  `cmd/gate`, `docscheck`, `wiringcheck` — and for the whole life of
  `cmd/perceptionreplay/coverage.go` that directory held 420 lines of production source no clone
  had, because `.gitignore` carried `coverage.*` and that matches a `.go` file as readily as a
  `.out` file. NOTHING COULD HAVE SAID SO: `git status` does not list an ignored file, so the
  check and the thing it checks shared one blind spot, and it surfaced only when a commit was
  checked out into an empty worktree. So stage 2 clones the commit, REFUSES unless
  `git status --porcelain --ignored` in the checkout is empty, builds it and runs the screen-free
  gate there, and writes a result NAMING THE COMMIT — and packing refuses unless that record
  says PASSED *and* names the commit about to be packed, because an exit code is not a record and
  a record of yesterday's commit is not a record of this one. The working tree must be clean too:
  cleanroom builds the COMMIT while `pack.ps1` packs the WORKING TREE, so an uncommitted change
  would be packed having been checked nowhere. `internal/wiringcheck`
  `TestNoIgnorePatternHidesASourceFile` kills the specific pattern accident and cannot establish
  that every file the build needs was ever added; only a clone can. See
  [[ADR-270-packaging-is-gated-on-a-clean-checkout]].
- **A `.ps1` in this tree must be ASCII, or carry a BOM.** Windows PowerShell 5.1 reads a
  BOM-less file as ANSI, so a box-drawing character or an em dash in a script written as plain
  UTF-8 becomes a parse error with a mangled caret — which is what the first `gate.ps1` did,
  before a single stage ran.
- **Keeping a recording is ONE set of rules, in `orchestrator.Keep`, and every surface obeys
  them.** Studio gained drafts and the consolidation did not come with them: `marco learn` and the
  widget had a second copy of the same pipeline with none of the rules, so learning a name twice
  REPLACED the first routine, and two of the three scope answers saved a routine that refuses
  every press. Both measured. Scope validity, the collision check and the replacement-identity
  re-check live in `Keep` now — and consent is to a VERSION (digests of the source AND the
  stored demonstration, re-read at the write), while a fresh save is EXCLUSIVE
  (`routes.Registry.Create`, `O_CREATE|O_EXCL`) because a check and a write are two instants —
  and the version check and both writes sit inside a CROSS-PROCESS LOCK on that route
  (`routes.Registry.Lock`, `LockFileEx`, released by the OS when the process ends), because atomic
  writing does not make the comparison before it atomic. EVERY WRITER TAKES IT: `Deps.Replace`,
  behind Studio's Simplify and its generation rebuilds, validated and wrote outside the lock and
  has joined — a lock protects the writers that take it, and nobody else taking it protects one
  that does not.
  Each surface supplies only the interaction — Studio a card,
  `marco learn` a question naming the exact application/scope/route with ENTER MEANING NO, and the
  widget's unattended `LearnAuto` a refusal, because it saves with nobody having looked. What is
  NOT shared, on purpose: the capture mode (the terminal states nothing and still does) and the
  owner-strip rule (`Deps.OwnerTailOnly`). `saveTaught`/`chooseScope` are retained for
  `LearnVoice`. See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].
- **DIRECTOR OWNS ROUTE RESOLUTION, and every surface asks it.** `internal/resolveroute` is the
  policy and `cmd/director` is its only caller, reached over the service protocol
  (`RESOLVE_ROUTE`, read-only). Exact versus fuzzy is an implementation detail inside that package
  and NOT an ownership boundary, so there is no local fallback: the ask box, the widget's offer and
  `marco dispatch` go dark when Director does and SAY SO — not running, speaking another
  protocol, or answered and refused are three different sentences. Cards and Play keep working.
  THE FOREGROUND NEEDS PROVENANCE: a world-model observation can be seconds old and already wrong
  (the window provider excludes Marco's own surfaces, so with Studio in front none is produced at
  all) or name a Marco window (accessibility does not exclude them), so a context is granted only
  by a live foreground reading through the excluding path, a stated target is the authoritative
  override, and a basis that cannot be relied on carries NO application. Resolution returns an
  identity and grants nothing; Play is still the person. AN ADAPTER CARRIES THE WHOLE IDENTITY
  (id/app/slug/focus) through confirmation, or the door resolves the words again and performs a
  namesake. And only Director's own `learn` decision becomes a learn offer: with Director
  unavailable the widget offers nothing, which is the boundary rather than a bug —
  `learn <name>` is still one of the overlay's own words. See
  [[ADR-267-director-owns-route-resolution]].
- **Studio can be ASKED for a routine in words, and the resolver it asks was dead.**
  `internal/dispatch` + `plugins/llama` + a local Ollama had a Dispatcher, an Advisor, a
  trust-but-verify `validate` and `$MARCO_RESOLVER` pinned by `setup.ps1` — reached only by
  `marco dispatch` and the `marco assistant` REPL, which nothing launches. The widget never called
  it. So `Acts.SuggestRoutine` reuses it and SELECTS a routine; the thing that runs one is still
  the Play button. Three rules worth keeping straight: the TARGET APPLICATION IS STATED, never
  inferred (Studio owns the foreground, and "it has a window" would erase the difference between
  context and focus); the resolver is offered slug-shaped LABELS whose uniqueness is checked on
  `nlu.Key`, the matcher’s own reduction, and the answer is a key into a map that is NEVER
  resolved again; and a decision says which arm answered and WHAT BECAME OF THE RESOLVER — absent,
  not-asked, answered or asked-and-silent — because two of the three arms need no model, so a
  window that never reached Ollama would look exactly like one that did, and an exact match
  reporting "the resolver answered this" is the same lie the other way up. And a name more than
  one eligible routine answers to is QUALIFIED FOR ALL OF THEM and asked about: unique labels
  stop two routines being confused for one, and are not an answer to the person's ambiguity. See
  [[ADR-265-say-what-you-want-and-studio-offers-the-routine]].
- **`director capture-game-session` is the L0 recorder** (frames + observed inputs on one
  clock, nothing injected) and `go run ./cmd/perceptionreplay -frames -stride N <dir>` replays
  the pixels through the production perception path. `-frames` is a BOOL and the directory is
  POSITIONAL; putting the directory first silently ignores `-stride`.
- **The live resolver test is behind `-tags resolver`**: `go test -tags resolver ./cmd/director/`,
  REQUIRED for a resolver change, because it is the only test that asks the real plugin and a
  real model server rather than a stand-in. It is out of the default suite for a reason about the
  LEDGER and not about the test: a conditional skip is the one thing `gate-skips.txt` cannot
  express — an undeclared skip fails the gate and a declaration whose test ran fails it too, so
  the same honest test was a finding with Ollama running and a finding without it. The ledger is
  for a skip that is always a skip. **Any other test that needs something this repository does
  not contain belongs behind a tag for the same reason, not in the ledger.**
- **AND A TEST WHOSE PRECONDITIONS ARE FACTS ABOUT THE OWNER'S SCREEN IS BEHIND `-tags desktop`**:
  `go test -tags desktop ./cmd/director/`, which `gate.ps1` runs as stage 4 of 5 — before the
  harness, because these only LOOK while stage 5 presses. Stage 1 has to be usable while somebody
  is using their computer and was not: fourteen tests of Director's RUN LOGIC (a deadline
  announced, a question withdrawn, a grant for another Act refusing, a startup failure recorded)
  had to manufacture a performable act first, and the only way to get one was to photograph
  whatever was in front of the owner — so they inherited every precondition that photograph has,
  and two runs of the same tree gave 8 findings and then 6, a different set each time, with a game
  on screen. They state their own desktop now: `winprovider.DesktopForTest` answers the window
  list, the handle lookup, process liveness AND ownership out of ONE list, with no individual
  knobs, because a stated list under a live lookup is a test that passes while its rule does
  nothing. The pixels are DRAWN and production perception still reads them — `structure.Propose`
  finds four control-shaped regions agreeing at 1.000, where a real window in front of somebody
  playing agreed at 0.339 — but the APPLICATION may not be invented, because a run resolves it and
  refuses: *a workflow may only act on the application it was shown in*. Don't tag a file by its
  name: two of `groundbykey_test.go`'s three tests reach no desktop at all, and tagging the file
  would have taken them out of the default suite. And THE GUARD IS NOT TAGGED
  (`TestADesktopTestCanObserveButNeverAct`), because a guard that is only present when the thing
  it guards is running is not a guard. See
  [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]].
- **BOTH perception-corpus tests are behind `-tags corpus`** and are REQUIRED for a perception
  change: `go test -tags corpus ./cmd/perceptionreplay/` (the full-frame one is about 145s). The
  full-frame one went out of the default suite because it was starving other packages — `cmd/marco`
  past its 600s limit, and a latency assertion from 962µs to 60ms. **The every-tenth-frame one
  joined it**, and the sentence that used to be here — "`go test ./...` still gates perception
  through the every-tenth-frame test" — was true on ONE MACHINE: it reads an L0 recording from
  `$MARCO_FRAME_CORPUS` or `.tmp/`, both uncommitted, so in a clean checkout it SKIPPED, and a
  skip is green. Under the tag it refuses instead, because selecting it is asking for the corpus.
  `go test ./...` gates perception through the hundred-odd deterministic tests in that package,
  over synthetic fixtures; what it does not gate is the measurement against a real recording. See
  [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]] and
  [[ADR-270-packaging-is-gated-on-a-clean-checkout]].
- **A TEST THAT NEEDS A PACKED BUNDLE IS BEHIND `-tags packaging`, AND THE GATE RUNS IT.** The
  distinction that decides where a precondition goes: `-tags resolver`, `-tags corpus`,
  `-tags vision` and `-tags desktop` stand for something this repository DOES NOT CONTAIN — a
  model server, a recording, vendored weights, somebody's screen — and each is selected by hand. A
  PACKED BUNDLE is something this repository MAKES, so `plugins/marco-app`'s six installation
  tests are not an external prerequisite: `gate.ps1` stages the bundle and runs them in the same
  stage, REJECTS A SKIP, and refuses to install a bundle whose own installation tests did not run.
  Under the tag the precondition fails rather than skips. Same for the one shadow test that needs
  the vendored weights (`-tags vision`): only THAT TEST moved, because
  `shadowwiring_test.go` holds five others that touch no model at all. See
  [[ADR-270-packaging-is-gated-on-a-clean-checkout]].
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
