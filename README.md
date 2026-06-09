# marco

**Marco is a sentence-driven automation language and a self-teaching computer
assistant.** You name a command in plain words — *"log into Facebook"*, *"start
Sea of Thieves"* — and Marco runs it. If it doesn't know how, it asks you to
show it once, records what you do, and remembers it as a clean, editable program
so next time it just does it.

Underneath is a small language where every line reads like English and the
runtime drives the real machine (keystrokes, mouse, screen) through a
host-FFI boundary that plays nicely with other languages.

```
$ marco assistant
> log into facebook
I don't know "log into facebook" yet.
Show me how to "log into facebook". Do it now, then press Esc when finished.
(For a password, type {{name}} instead of the real value, then set it with: marco secret set <name>)
…you click through the login, typing {{fb-password}} for the password, press Esc…
Learned "log into facebook" → routes/log-into-facebook.marco (7 steps)
Run it now? [y]es / [n]o: y
```

---

## Quickstart

Build the CLI (Go 1.22+):

```sh
go build -o marco ./cmd/marco
```

Teach a route by demonstration, then run it by name:

```sh
marco teach "open inventory"     # record clicks/keys; press Esc to finish
marco do   "open inventory"      # replay it (real input on Windows)
marco routes                     # list what it knows
```

Or just talk to it — the assistant fuzzily matches what you type to a saved
route, or teaches a new one:

```sh
marco assistant
> open the inventory     # resolves to "open inventory" and runs it
```

Matching is deterministic and offline by default. For loosely-phrased requests
("fire up the pirate game") you can plug in a model-backed resolver: build the
reference one and point `$MARCO_RESOLVER` at it —

```sh
go -C plugins/claude-resolver build -o claude-resolver .
export MARCO_RESOLVER=$PWD/plugins/claude-resolver/claude-resolver
export ANTHROPIC_API_KEY=sk-...
```

The resolver is an **external plugin** (a separate Go module), so **Marco itself
has zero dependencies**. It's consulted only when the local matcher is unsure,
and the tool works fully without it.

Press **Esc** at any time to abort a running route.

### Context-aware routes

A route remembers the app you taught it in and **brings that app to the front
before running** — so "log into Facebook" focuses Chrome first. A route is
either **global** (works anywhere) or **scoped to an app**, chosen when you save
it. Scoped routes let the *same phrase* mean different things per app: "switch to
sword" can resolve to your Sea of Thieves route when it's focused and a different
game's route when that is. Resolution prefers the route for the foreground app,
then a global one. `marco routes` shows each route's scope.

### Passwords never get recorded

While teaching, type a **placeholder** `{{name}}` where a secret goes. The
recorder only ever sees the placeholder; the route file stores only the *name*.
The real value lives in your OS credential manager:

```sh
marco secret set fb-password     # Windows Credential Manager / Keychain / secret-service
```

At run time `do OS's Secret with "fb-password"` types it. The password is never
in the recording, never in the route, never logged. (Or declare it as a
secret-named argument — `with username, password` — and pass it once; see
[Commands with arguments](#commands-with-arguments).)

### Clicks that follow the window

Recorded clicks are **window-relative by default** — they're stored as an offset
from the active window's top-left, so a route keeps working when you move the
window or run it on another monitor or machine, without any monitor/DPI math.

For cases where even that isn't enough (fullscreen, heavy re-layout), turn on
**image anchors** — set `MARCO_ANCHORS=1` while teaching and each click also
captures a picture of its target, so the route finds it on screen instead of
trusting a coordinate:

```marco
do OS's Find with button...
    when ok?  do OS's Click with that.
    or?       do OS's Click with p1.   // falls back to the recorded point
```

Anchors are opt-in because the window-relative default covers ordinary windowed
apps; you can also crop the captured image, or let a vision plugin crop it.

### Commands with arguments

A route can take **named arguments**. Declare them with `with` when you teach,
and a value goes wherever you tap the **arg key (F9)** during the demo (no typing
`{{…}}` into the app):

```sh
marco teach "say hello with name"    # demo: tap F9 where the name goes
marco do   "say hello name:chris"    # fills it in
marco do   "dm person:sam message:hi there"   # several; values may have spaces
```

An argument named like a secret (**password**, `pin`, `token`, …) is special: it's
resolved from the credential store, **never written into the route**, and
**remembered** — pass it once, then omit it next time:

```sh
marco teach "login to facebook with username, password"
marco do   "login to facebook username:me password:hunter2"   # remembers both
marco do   "login to facebook"                                 # reuses them
```

You can also still type a `{{name}}` placeholder during the demo for a globally
named secret (set it with `marco secret set <name>`).

### Teach by talking

Instead of demonstrating, you can **narrate** a route — typed or spoken — and each
phrase becomes a step:

```sh
marco teach --narrate "open chest"
# then, one phrase per line (or via the voice plugin's mic):
#   activate sea of thieves
#   click this
#   wait for this screen
#   type the first argument
#   done
```

In the overlay this is hands-free: say (or type) **"narrate teach open chest"**,
then narrate. The vocabulary is forgiving — `click this`, `anchor this`, `wait for
this screen`, `wait 2 seconds`, `type …`, `press enter`, `activate <app>`, plus
`undo` / `done` / `cancel`.

---

## How it works

A route is a small **Marco program** on the `OS` act — the stable, cross-platform
automation API:

```marco
use os.

the OpenInventory is an actor.
this can Run.
this's Run does...
    the slot is a Point with X 680, Y 400.
    do OS's Click with slot.
    do OS's Sleep with 150.
    do OS's Key with "i".
    this is ok!

the App is a script.
do OpenInventory's Run...
    when ok?  log "open inventory: done".
    or?       log that's error.
```

- **Marco** owns the choreography — sequences, branches, loops (`repeat N
  times...`), messaging.
- A **host** performs the real OS effects behind the `OS` act. Pick one with
  `--host`:
  - `dryrun` (default) — logs each call, does nothing real; deterministic.
  - `windows` — native `SendInput` keystrokes/mouse, screen capture, credential store.
  - `bridge:<exe>` — delegates to an external program in any language (e.g.
    AutoHotkey) over a tiny JSON protocol.

The **record → simplify → codegen** pipeline turns a demonstration into that
program: it drops mouse jitter, rounds waits, coalesces key-spam, folds repeated
cycles into loops, records clicks window-relative, turns arg-key taps into
named-argument placeholders, and converts `{{name}}` placeholders into secret
lookups.

Everything is **cross-platform by construction**: input capture, input
synthesis, screen reading, and the credential store each sit behind a small
interface with per-OS backends (Windows implemented; macOS/Linux additive).

See [`spec/Hosts.md`](spec/Hosts.md) for the host-FFI design and `spec/` for the
language reference.

### Alpha install (Windows)

One script builds the layers you choose and writes a launcher. Use `setup.cmd`
(it bypasses PowerShell's execution policy so you don't hit a "running scripts is
disabled" error):

```powershell
.\setup.cmd            # core: engine + macros + native overlay (needs only Go)
.\setup.cmd -Voice     # + offline voice (downloads libvosk + a Vosk model, builds with cgo)
.\setup.cmd -WebUI -Resolver   # also the web panel / Claude resolver
```

(`setup.cmd` just runs `setup.ps1` with `-ExecutionPolicy Bypass`; call the `.ps1`
directly if your policy already allows it.) Voice uses a wake word (default
**marco**, say "marco, <command>"); change it with `-Wake "<word>"`.

Natural-language route matching is **offline and key-free by default** (a
deterministic matcher maps what you say to your route names). For loose phrasing
("fire up the pirate game") add the cloud Claude resolver: `setup.cmd -Resolver
-ApiKey sk-...` (stores `ANTHROPIC_API_KEY` in your user env and points the
launcher at it; it's only consulted when the local matcher is unsure).

Then run it: `.\overlay.cmd`. `-Voice` needs a C compiler (mingw-w64 `gcc`) on
PATH; without one it builds the demo-only voice and tells you how to get gcc.
Re-run any time (downloads are cached; `-Force` refetches).

### The engine is headless; UIs and macros are layers

`marco` itself is a CLI — there's no built-in GUI, and the OS effects can run as
their own process too. Everything beyond the engine is an **external layer** that
plugs in over the host boundary:

- **Macros layer** — [`cmd/marco-macros`](cmd/marco-macros) fulfils the `OS` act
  (real keystrokes, clicks, spam, find …) as a separable bridge process. Select
  it with `--host OS=bridge:marco-macros` (or keep it in-process with
  `--host windows`).
- **UI layers** — drive the engine over the CLI seam (`marco routes --json`,
  `marco active`, `marco do "<name>"`):
  - [`plugins/overlay`](plugins/overlay) — a **native, cross-platform gamer HUD**
    (transparent, click-through, always-on-top). A leader key (`` ` ``) opens a
    command line (`` `m <command>``); it teaches in-place (record → F12 → save),
    narrates by voice or typing, auto-pops `name:` labels for routes that take
    arguments (Tab to accept), and answers prompts in the HUD. Its behaviour lives
    in Marco ([`programs/overlay.marco`](programs/overlay.marco)); it fulfils a
    clean `Overlay` act and pushes input events back over a bidirectional bridge.
  - [`plugins/web-ui`](plugins/web-ui) — a reference local web control panel.
  - [`plugins/ahk-overlay`](plugins/ahk-overlay) — the original MacroMarco
    AutoHotkey overlay (run `D:\Macros\MacroMarco\run.bat`).

Run all three layers together:

```sh
marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay programs/overlay.marco
```

---

## Command reference

```
marco do "<name>"          run a route; teach it once if unknown
                           ("<name> arg:value …" passes named arguments)
marco teach "<name>"       (re)record a route by demonstration
marco teach --narrate "<name>"   build a route by narration (typed or voice)
marco simplify "<name>"    re-simplify a saved route as far as it goes
marco assistant            interactive loop — say what you want
marco routes [--json]      list known routes
marco args "<name>"        print a route's argument labels (used by the overlay)
marco forget "<name>"      delete a route ("forget all" wipes them, after a confirm)
marco secret set|list|rm <name>   manage stored passwords

marco run   [--host …] <file.marco>     run a Marco program
marco serve [--host …] <file.marco>     run persistently, react to host events
marco check [--json] <file.marco>       static check + diagnostics
marco test <file.marco>                 run test blocks
marco contracts <file.marco>            print inferred action contracts
```

Routes live in `./routes` (override with `$MARCO_ROUTES`).

### Environment variables

| Variable | Effect |
|---|---|
| `MARCO_ROUTES` | route directory (default `./routes`) |
| `MARCO_STOP_KEY` | key that ends a recording / aborts a run (default `f12`) |
| `MARCO_ARG_KEY` | key that drops an argument placeholder while teaching (default `f9`; `off` disables) |
| `MARCO_ANCHORS` | `1` captures an image anchor per click while teaching |
| `MARCO_RESOLVER` | path to a resolver plugin for loosely-phrased commands |
| `MARCO_BIN` | engine binary the overlay shells out to |
| `MARCO_OVERLAY_IDLE` | overlay idle opacity (0–1) |
| `MARCO_VOICE_WAKE` | voice wake word (default `marco`; `off` = always listen) |

---

## Development

```sh
go build ./...
go test ./...      # deterministic; the recorder/host/screen backends are
                   # build-tagged, with cross-platform stubs
```

CI builds, vets, and tests (including `-race`) on Linux. The platform backends
(Windows hooks, GDI capture, Credential Manager) are exercised by build-tagged
tests on Windows.
