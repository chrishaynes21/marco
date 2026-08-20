# marco

**Marco is a sentence-driven automation language and a computer assistant that
learns what you show it.** You name a command in plain words — *"log into
Facebook"*, *"start Sea of Thieves"* — and Marco runs it. If it doesn't know
how, it asks you to show it once, watches what you do, and keeps it as a clean,
editable program so next time it just does it.

Underneath is a small language where every line reads like English and the
runtime drives the real machine (keystrokes, mouse, screen) through a
host-FFI boundary that plays nicely with other languages.

```
$ marco learn "log into facebook"
Show me how to "log into facebook". Do it now, then press F12 when finished.
(For a password, type {{name}} and set it with: marco secret set <name>.)
…you click through the login, typing {{fb-password}} for the password, press F12…
Learned "log into facebook" (anywhere) → routes/global/log-into-facebook.marco

$ marco plays
Known plays:
  log into facebook            Recorded · Ready · anywhere

$ marco do "log into facebook"
[route] log into facebook
[result] performed
```

Or say it: the overlay (`.\overlay.cmd`) takes the same words typed or spoken,
and one **stop** — said, typed, or the leader key — ends whatever is running.

---

## Quickstart

Build the CLI (Go 1.26+):

```sh
go build -o marco ./cmd/marco
```

Show Marco a **play** once, then run it by name:

```sh
marco learn "open inventory"     # record clicks/keys; press F12 to finish
marco do   "open inventory"      # replay it (real input on Windows)
marco plays                      # list every play you have
marco routes                     # list the plays you can ask for (unchanged)
```

A **play** is one durable behaviour: a small, readable Marco program on disk with
a record of where it came from. It's the noun the product uses everywhere — the
`marco routes` command, the `routes/` directory and `$MARCO_ROUTES` keep their
old names, and nothing you already type stops working.

Press the **stop key** at any time to abort a running play — **F12** by default, so
Esc stays free to record as an ordinary key (`$MARCO_STOP_KEY` changes it). From
anywhere else, including another window, `marco stop` does the same thing.

### One way in

Typing, speaking, a hotkey, a button in the control centre and the command line
are **entrances, not different Marcos**. Whichever you use, one rule decides what
happens:

> **If Marco already knows exactly what you mean, it does the thing it knows.
> Otherwise the Director works out what you mean from what's on screen.**

So a play called *"open inventory"* runs when you type it, when you say it, when
you press the key you bound to it and when you click Run — the same play, every
time. And *"turn the bluetooth off"*, which you never taught, goes to the
Director whichever way you asked. Speaking is not a different vocabulary from
typing; it's a different microphone.

Matching against your saved plays is **exact** and offline: case, punctuation,
quotes and extra spaces are folded, and nothing else. A near miss is a miss, and
a miss is a question about the screen — which is the Director's job, because it
can look, and it can ask you.

**"stop"** always means stop. It reaches whatever is running before anything else
looks at it, from any entrance.

Every command tells you what became of it, in one of six words:

| | |
|---|---|
| **performed** | it ran, and Marco confirmed you ended up where you asked |
| **clarify** | Marco asked you something and is waiting |
| **refused** | Marco declined, or wasn't allowed — a question you answered no to, a screen it didn't recognise |
| **unavailable** | nothing took the request at all. This is the only time Marco offers to learn it |
| **cancelled** | you stopped it |
| **failed** | it was tried and it went wrong |

*"Refused" and "failed" are different on purpose, and neither is "performed".* A
play that finished every step but didn't get you where you asked is not a success.

### The interactive loop

`marco assistant` is a developer surface with one extra habit: it will offer a
close match rather than requiring the exact words. It **asks first** —

```sh
marco assistant
> open the inventory
Did you mean "open inventory"? [y]es / [n]o: y
```

and whatever you confirm takes the same path everything else does. For loosely
phrased requests ("fire up the pirate game") you can plug a model-backed resolver
into that prompt: build the reference one and point `$MARCO_RESOLVER` at it —

```sh
go -C plugins/claude-resolver build -o claude-resolver .
export MARCO_RESOLVER=$PWD/plugins/claude-resolver/claude-resolver
export ANTHROPIC_API_KEY=sk-...
```

The resolver is an **external plugin** (a separate Go module), so **Marco itself
has zero dependencies**. It is consulted only by that prompt, never on the way to
running a play, and the tool works fully without it.

### Where a play works (scope)

When you save a play you choose one of three scopes:

- **Only here (context)** — works *only* while the app you taught it in is in front,
  and never switches windows. This is what lets the *same phrase* mean different
  things per app ("leave game" in Rocket League vs. Sea of Thieves) — name
  overloading. Foreground-only, so it won't fire from another app.
- **Anywhere, focus first** — works from anywhere and **brings the app to the front
  before running** (so "log into Facebook" focuses Chrome first). Focusing only
  un-minimizes a *minimized* window; a maximized/fullscreen app (a game, the Xbox
  app) is left as-is, so activation never knocks it out of fullscreen.
- **Anywhere** — works in place with no window switch, for app-agnostic actions
  (type some text, Ctrl+A then Delete).

Resolution prefers the foreground app's context play, then a global one.
`marco plays` and `marco routes` both show each play's scope, and a
focus-scoped row says which app it brings forward.

### Saved, then askable — `marco plays` and `marco register`

Saving a play and making it *askable* are two different things, and Marco keeps
them apart on purpose: a saved play is a file you can read and edit, and until
it is **registered** nothing can ask for it by name. Learning a play does both
for you. Registration can also legitimately be refused — a name already taken is
the usual cause — and then you're left with a real file that nothing mentions.

So there are two listings:

```sh
marco plays        # everything you have, in two groups: askable, and saved-not-yet-askable
marco routes       # only the plays you can ask for (what a UI plugin may offer)
```

`marco plays` prints each play's kind (**Authored**, **Recorded**, **Learned**),
its standing, and where it answers from:

```
Known plays:
  open inventory               Recorded · Ready · only in seaofthieves
  open mouse settings          Learned · Ready · from anywhere (brings settings forward)

Saved, not askable yet:
  mute volume                  Learned · Saved — not askable yet   (marco register "mute volume")
```

Register one by name, and it becomes askable:

```sh
marco register "mute volume"
```

The same two groups appear in the **Plays** tab of `marco ui`, with a Register
button beside every saved row. `marco routes` is unchanged — same result set,
same JSON keys (`name`, `slug`, `app`, `scope`, plus new ones) — so anything
already scripted against it keeps working.

### Passwords never get recorded

While Marco is learning, type a **placeholder** `{{name}}` where a secret goes.
The recorder only ever sees the placeholder; the play file stores only the *name*.
The real value lives in your OS credential manager:

```sh
marco secret set fb-password     # Windows Credential Manager / Keychain / secret-service
```

At run time `do OS's Secret with "fb-password"` types it. The password is never
in the recording, never in the play, never logged. (Or declare it as a
secret-named argument — `with username, password` — and pass it once; see
[Commands with arguments](#commands-with-arguments).)

### Clicks that follow the window

Recorded clicks are **window-relative by default** — they're stored as an offset
from the active window's top-left, so a play keeps working when you move the
window or run it on another monitor or machine, without any monitor/DPI math.

For a target that isn't there yet (a menu, a loading screen) or that **moves**
between runs, attach an **anchor** — a first-class on-screen target the engine
**recognises** rather than trusting a blind coordinate. `Find` scores several
in-engine signals into one **confidence**; when confident it clicks the **located**
point (following a target that drifted), and when unsure it falls back to the
recorded coordinate. All of it is pure Go in the engine — **no OpenCV, no
dependency**:

- **Image** — template match of the button, made robust: **scale-invariant** (a
  DPI/resolution change), **brightness- and contrast-invariant** (night mode, a
  re-theme, a different monitor), plus a **colour-palette** check.
- **Shape (edges)** — matches the button's *outline* (orientation-aware), so it
  still resolves when a theme **recolours** the button and the pixels no longer match.
- **Colour** — the pixel under the recorded click.
- **Position** — how near a candidate is to where you clicked *and* to where the
  anchor **resolved last time**, so the search follows a UI that drifts across runs.
- **Window** — the **title** of the window you taught it in, so a perfect match in
  the *wrong* window of a multi-window app (Steam's library vs. friends vs. store)
  can't produce a confident click.

When you anchor a click the captured patch is **auto-cropped to the button** it
detects, and every signal above is captured automatically — nothing extra to do.
A taught anchor reads like this; `when ok?` clicks the located point, `or?` is the
safe fallback:

```marco
do OS's Find with menu...                // score the signals into a confidence
    when ok?  do OS's Click with that.   // confident: click where it IS (follows a move)
    or?       do OS's Click with p1.      // unsure: click the recorded coordinate
```

For a target that moves *within* a reflowing UI (Discord's mute button), add the
**text** resolver: it OCRs the screen, finds the word, and **snaps to the button
containing it**. Narrate it with the cursor over the target: *"click the text Mute"*.

```marco
do OS's Find with menu...
    when ok?  do OS's Click with that.
    or?
        do Text's Find with mute...      // OCR: locate the word, snap to its button
            when ok?  do OS's Click with that.
            or?       do OS's Click with p1.
```

Text is a **plugin** (OCR needs a dependency; the engine stays zero-dep). `setup.cmd
-OCR` builds it **and installs the `tesseract` engine** it uses (via winget), then
wires it up. Without it, a text anchor falls back to its recorded coordinate. See
`plugins/ocr/README.md`. (Plays learned before this keep working; learn one again
to pick up the newer signals and the button-cropped template.)

Anchor a click **per click** by **tapping the anchor key, then clicking** that
target during the demo — the tap starts the anchor, the click ends it. Only that
click matches by image; everything else stays an exact, fast coordinate. It's
one-handed (no holding). (Narration has the same gesture: say *"anchor this"* before
a click.) To anchor *every* click for a session instead, set `MARCO_ANCHORS=1`. The
anchor key is `$MARCO_ANCHOR_KEY` (default **F12**; `off` disables) and is swallowed
during recording so it never reaches the app. In the overlay the leader stops the
demo, leaving F12 free to anchor; in the CLI F12 is the stop key, so set
`MARCO_ANCHOR_KEY` to another key to anchor there. You can also crop the captured
image, or let a vision plugin crop it.

### Holding a key

Some actions need a key **held down** while you do something else — hold a movement
key, hold Q while you click. Just do it during the demo: a key you **hold for ≥ ½
second, or hold across a click or another key**, is recorded as an explicit hold that
**persists** across the steps in between and releases where you released it. A quick
tap stays an ordinary keystroke. The real hold *duration* is kept (a charge-hold
stays long; a hold-and-click spans exactly your clicks), and a held key is always
released when the run ends — even on error or **Esc** — so nothing is ever left stuck
down. (Normal taps also get a small key-down→up linger so fast games register them;
tune it with `$MARCO_KEY_HOLD_MS`.)

### Commands with arguments

A play can take **named arguments**. Declare them with `with` in the name Marco
learns, and a value goes wherever you tap the **arg key (F9)** during the demo
(no typing `{{…}}` into the app). Pass values two ways — by name, or positionally
with the same `with` word, where each value fills the declared arg in order:

```sh
marco learn "say hello with name"    # demo: tap F9 where the name goes
marco do   "say hello name:chris"    # by name
marco do   "say hello with chris"    # positional — "chris" fills `name`
marco do   "dm person:sam message:hi there"   # several; values may have spaces
marco do   "dm with sam, hi there"            # positional, comma-separated
```

In the overlay you type the positional form (`say hello with chris`) and it
shows the arg name as a colored hint in front of each value
(`say hello with `**`name:`**`chris`). The `name:` is a display-only label — the
command stays `say hello with chris`; press **Tab** to step to the next arg slot.

An argument named like a secret (**password**, `pin`, `token`, …) is special: it's
resolved from the credential store, **never written into the play**, and
**remembered** — pass it once, then omit it next time:

```sh
marco learn "login to facebook with username, password"
marco do   "login to facebook username:me password:hunter2"   # remembers both
marco do   "login to facebook"                                 # reuses them
```

You can also still type a `{{name}}` placeholder during the demo for a globally
named secret (set it with `marco secret set <name>`).

### Chaining commands

Run several commands as one with **`then`** — each step keeps its own args and runs
in order (stopping at the first failure):

```sh
marco do "say hello with chris then delete all then say hi"
marco bind e "say hello with chris then delete all then say hi"   # one key, three steps
```

`bind <key> <command>` ties a leader hotkey (`` `e ``) to a command — a single play
or a `then`-chain — scoped to the foreground app; `unbind <key>` removes it.

### Learn by talking

Instead of demonstrating, you can **narrate** a play — typed or spoken — and each
phrase becomes a step:

```sh
marco learn --narrate "open chest"
# then, one phrase per line (or via the voice plugin's mic):
#   activate sea of thieves
#   click this
#   wait for this screen
#   type the first argument
#   done
```

In the overlay this is hands-free: say (or type) **"narrate learn open chest"**,
then narrate. The vocabulary is forgiving — `click this`, `anchor this`, `wait for
this screen`, `wait 2 seconds`, `type …`, `press enter`, `activate <app>`, menu
navigation (`down`, `down arrow`, `tab`, `escape`, `select`), plus `undo` / `done` /
`cancel`.

---

## How it works

A play is a small **Marco program** on the `OS` act — the stable, cross-platform
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
cycles into loops, records clicks window-relative, **detects held keys** (press/hold
that persists across clicks), **auto-crops anchors to the button** and captures their
match signals, turns arg-key taps into named-argument placeholders, and converts
`{{name}}` placeholders into secret lookups.

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
**computer**, say "computer, <command>"); change it with `-Wake "<word>"`,
or set it to `off` to listen to every phrase without arming.

Natural-language play matching is **offline and key-free by default** (a
deterministic matcher maps what you say to your play names). For loose phrasing
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
    command line (`` `m <command>``); it learns in-place (record → F12 → save),
    narrates by voice or typing, shows `name:` arg-name hints inline as you fill a
    play's arguments (Tab to step between slots), and answers prompts in the HUD.
    `` `m config `` opens an in-HUD editor (theme, opacity, monitor/corner, width,
    mini mode, the CPU/RAM widget, and a live **screen-coords tooltip** — handy when
    a demonstration crosses monitors). Its behaviour lives
    in Marco ([`plugins/overlay/overlay.marco`](plugins/overlay/overlay.marco)); it fulfils a
    clean `Overlay` act and pushes input events back over a bidirectional bridge.
  - [`plugins/web-ui`](plugins/web-ui) — a reference local web control panel.
  - [`plugins/ahk-overlay`](plugins/ahk-overlay) — the original MacroMarco
    AutoHotkey overlay (run `D:\Macros\MacroMarco\run.bat`).

Run all three layers together:

```sh
marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay plugins/overlay/overlay.marco
```

---

## Command reference

```
marco do "<name>"          ask for something: runs the play of that name, or hands
                           the words to the Director if there isn't one
                           ("<name> arg:value …" passes named arguments)
marco learn "<name>"       (re)record a play by demonstration
marco learn --narrate "<name>"   build a play by narration (typed or voice)
marco simplify "<name>"    re-simplify a saved play as far as it goes
marco assistant            interactive loop — say what you want
marco ui                   open the control center (all plays, bindings, help)
marco edit "<name>"        open the control center on one play's visual editor
marco plays [--json]       list every play, the saved-not-yet-askable ones included
marco routes [--json]      list the plays you can ask for (what a UI plugin may offer)
marco register "<name>"    make a saved play askable
marco args "<name>"        print a play's argument labels (used by the overlay)
marco rename "<old>" to "<new>"   rename a play (keeps its recording + anchors)
marco forget "<name>"      delete a play ("forget all" wipes them, after a confirm)
marco bind <key> "<cmd>"   bind a leader hotkey to a command or `then`-chain
marco unbind <key>         remove a hotkey binding
marco secret set|list|rm <name>   manage stored passwords

marco run   [--host …] <file.marco>     run a Marco program
marco serve [--host …] <file.marco>     run persistently, react to host events
marco check [--json] <file.marco>       static check + diagnostics
marco test <file.marco>                 run test blocks
marco contracts <file.marco>            print inferred action contracts
```

Plays live in `./routes` — the directory keeps its old name (override with
`$MARCO_ROUTES`).

**One compatibility alias.** Acquisition used to be spelled `teach`, and the old
spelling still answers everywhere it ever did — `marco teach`, `director teach`,
and the overlay's `teach <name>` / `narrate teach <name>`. It is undocumented
elsewhere on purpose and it is retiring, so type `learn`. **Teach** is reserved
for the other direction: Marco guiding you through something, which is not built
yet.

`marco do` also takes three flags that exist for the surfaces rather than for you:
`--source=<typed|spoken|hotkey|control-centre|cli|web>` records how a request
arrived (it is written to the trace and **never** used to decide anything), and
`--play=<slug>` with `--app=` / `--focus` names a play directly, so a clicked
button or a bound hotkey performs the play it already identified instead of
handing its name back to be looked up a second time. You can use them, but the
product does not need you to.

### Environment variables

| Variable | Effect |
|---|---|
| `MARCO_ROUTES` | where plays live (default `./routes` — the directory keeps its old name) |
| `MARCO_HOME` | where the Director keeps its action graph and semantic memory (default `%APPDATA%\marco`) |
| `MARCO_MEMORY` | the semantic-memory file itself (default `$MARCO_HOME/semantic-memory.json`); `marco` and `director` share one store and both honour this |
| `MARCO_STOP_KEY` | key that ends a recording / aborts a run (default `f12`) |
| `MARCO_ARG_KEY` | key that drops an argument placeholder while Marco is learning (default `f9`; `off` disables) |
| `MARCO_ANCHOR_KEY` | key you tap, then click, to anchor that click by image (default `f12`; `off` disables) |
| `MARCO_ANCHORS` | `1` captures an image anchor for *every* click while Marco is learning |
| `MARCO_CLICK_RADIUS` | half-size (px) of the captured anchor patch; default ≈ ¼ screen width, scaled to resolution |
| `MARCO_LOG` | log level; `debug` shows the full per-click anchor-scoring / coordinate trace (default `info`) |
| `MARCO_FIND_SCALES` | template scales tried when matching, for DPI/resolution changes (e.g. `0.8,1.0,1.25`) |
| `MARCO_EDGE_TOLERANCE` | edge-detection sensitivity for shape matching and button recognition |
| `MARCO_FIND_CONFIDENCE` / `MARCO_FIND_TOLERANCE` / `MARCO_FIND_THRESHOLD` | anchor confidence / colour-tolerance / image-match thresholds |
| `MARCO_ANCHOR_CACHE` | `0` disables the last-known-location cache that follows a drifting target |
| `MARCO_KEY_HOLD_MS` | linger between a tap's key-down and key-up so fast apps register it (default 25) |
| `MARCO_RESOLVER` | path to a resolver plugin for loosely-phrased commands, offered as a confirmation by `marco assistant` (never consulted on the way to running a play) |
| `MARCO_TRACE_INTAKE` | `1` prints one `[intake] …` line per command saying how the request was routed: its source, the decision, the play or phrase, and which rule fired. Off by default; it keeps no store and writes no file |
| `MARCO_OCR` | path to the OCR text-resolver plugin (`plugins/ocr/ocr.exe`); fulfils a play's text anchor |
| `MARCO_TESSERACT` | path to the `tesseract` binary if it isn't on `PATH` (used by the OCR plugin) |
| `MARCO_BIN` | engine binary the overlay shells out to |
| `MARCO_OVERLAY_IDLE` | overlay idle opacity (0–1) |
| `MARCO_VOICE_WAKE` | voice wake word (default `computer`; `off` = always listen) |

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
