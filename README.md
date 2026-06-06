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
Run it now? [Y/n] y
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

### Passwords never get recorded

While teaching, type a **placeholder** `{{name}}` where a secret goes. The
recorder only ever sees the placeholder; the route file stores only the *name*.
The real value lives in your OS credential manager:

```sh
marco secret set fb-password     # Windows Credential Manager / Keychain / secret-service
```

At run time `do OS's Secret with "fb-password"` types it. The password is never
in the recording, never in the route, never logged.

### Robust clicks

Routes can click **by image** instead of by fixed coordinate, so they survive
moved windows:

```marco
do OS's Find with button...
    when ok?  do OS's Click with that.
    or?       this is failed with error "button not found"!
```

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
cycles into loops, and converts `{{name}}` placeholders into secret lookups.

Everything is **cross-platform by construction**: input capture, input
synthesis, screen reading, and the credential store each sit behind a small
interface with per-OS backends (Windows implemented; macOS/Linux additive).

See [`spec/Hosts.md`](spec/Hosts.md) for the host-FFI design and `spec/` for the
language reference.

---

## Command reference

```
marco do "<name>"        run a route; teach it once if unknown
marco teach "<name>"     (re)record a route by demonstration
marco assistant          interactive loop — say what you want
marco routes             list known routes
marco forget "<name>"    delete a route
marco secret set|list|rm <name>   manage stored passwords

marco run   [--host …] <file.marco>     run a Marco program
marco serve [--host …] <file.marco>     run persistently, react to host events
marco check [--json] <file.marco>       static check + diagnostics
marco test <file.marco>                 run test blocks
marco contracts <file.marco>            print inferred action contracts
```

Routes live in `./routes` (override with `$MARCO_ROUTES`).

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
