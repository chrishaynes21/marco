# ahk-overlay — the MacroMarco UI plugin

The original **MacroMarco** AutoHotkey overlay — a full on-screen terminal with a
log panel, chip picker, builder window, config panel, widgets and themes — used
as a **UI plugin** for the headless `marco` engine. Like `web-ui`, it lives
**outside** marco core and drives it over the CLI, so marco stays headless and
zero-dependency. It's the rich, full-featured front-end; `web-ui` is the minimal
reference one. Both speak the same seam.

The code lives in its own repo at **`D:\Macros\MacroMarco`** (AutoHotkey v2, not
Go), so it isn't vendored here — this directory just registers it as a
first-class marco plugin and documents how it connects.

## How it drives marco

`core/marco_bridge.ahk` is the integration point:

| It runs | When | Shows |
|---|---|---|
| `marco routes --json` | startup / `marco reload` | every route + scope (in the log panel) |
| `marco do "<name>"`   | you type a route's name | runs it, streams output live |

It's wired in at `Engine.Init()` → `MarcoBridge.Init()`, and
`SentenceRuntime.Run(text)` calls `MarcoBridge.TryRun(text)` **first** — if the
text matches a marco route it runs there; otherwise it falls through to the
AHK engine. Typed commands:

- `marco` / `marco routes` — list known routes
- `marco reload` — refresh the route list after teaching new ones
- *(any route name)* — run it via `marco do`

## Run it

```
D:\Macros\MacroMarco\run.bat
```

Needs **AutoHotkey v2** (`%ProgramFiles%\AutoHotkey\v2\AutoHotkey.exe`). The bat
launches `core\startup.ahk`, which brings up the overlay.

## Configure

In `D:\Macros\MacroMarco\config\config.ahk`:

| Key | Points at |
|---|---|
| `marcoExePath`   | the built engine — `D:\Macros\marco\marco.exe` |
| `marcoRoutesDir` | the routes tree — `D:\Macros\marco\routes` |

Rebuild the engine with `go build -o marco.exe ./cmd/marco` (from
`D:\Macros\marco`); the overlay picks it up on next launch / `marco reload`.

## Any front-end works the same way

This and `web-ui` are just two front-ends over the same three commands
(`marco routes --json`, `marco active`, `marco do "<name>"`). marco stays the
headless brain; the UI is always a plugin.
