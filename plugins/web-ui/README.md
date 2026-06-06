# web-ui — a Marco UI plugin

A reference **UI plugin**: a tiny local web control panel for the headless
`marco` engine. Like the resolver and bridge plugins, the UI lives **outside**
marco — it's a separate program (its own module, **stdlib only**) that drives
the marco CLI, so marco core stays headless and zero-dependency.

It shells out to:

| Endpoint | Runs | Shows |
|---|---|---|
| `GET /api/routes` | `marco routes --json` | every route + its scope |
| `GET /api/status` | `marco active` + running set | foreground app + routes currently running |
| `POST /api/do` | `marco do "<name>"` | runs a route, returns its output |

The page lists **all routes**, **highlights the one running**, and takes commands
by **typing or voice** (the mic uses the browser's Web Speech API in Chrome/Edge;
it's hidden where unsupported).

## Build & run

```sh
go build -o marco ./cmd/marco                 # the engine
go -C plugins/web-ui build -o web-ui .         # the UI plugin
MARCO_BIN=$PWD/marco web-ui                     # open http://localhost:8765
```

`MARCO_BIN` points at the marco binary (default `marco` on PATH); `MARCO_ROUTES`
(read by marco) selects the routes dir; `MARCO_UI_ADDR` overrides the listen
address.

## Any front-end works the same way

This web UI is just one front-end. **The MacroMarco AutoHotkey overlay can be a
UI plugin too** — its command box / leader-key chords call `marco do "<name>"`,
and it can populate its list from `marco routes --json` and show context from
`marco active`. That's the same seam, in whatever UI toolkit you like; marco
stays the headless brain.
