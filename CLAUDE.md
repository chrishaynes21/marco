# CLAUDE.md — start here

**Marco** is a sentence-driven automation language + self-teaching assistant: you
name a command in plain words, demonstrate (or narrate) it once, and it's saved as a
small editable Marco program that drives real input via a host-FFI boundary.

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
  `WM_QUIT`) and unhook from the installing thread. Enforced by
  `internal/platform/navsource/pump_test.go`, which walks the whole tree for hook sites
  rather than naming them. **This is a correctness rule, not a measured latency win**: it
  was found while chasing a desktop hitch whose root cause was never established, and no
  clean measurement has yet attributed any perceived input latency to it.
- **Secrets never land in a route or a recording.** Passwords resolve via
  `do OS's Secret …` at run time from the credential store. Don't text-substitute a
  secret value into route source.
- **Cross-platform by construction.** OS surfaces (`winctx`, `screen`, `recorder`,
  `secrets`, `oshost` backend) sit behind interfaces with Windows backends + stubs;
  keep the stub side compiling.

## Build / test / run
```sh
go build ./... && go test ./...        # 73 packages, deterministic
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
