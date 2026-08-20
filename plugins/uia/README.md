# uia — the accessibility resolver

A Marco bridge host that fulfils an `Accessibility` act by reading Windows' UI
Automation tree. It is the Director's strongest routinely available perception
source: the one that reports an element **is** a button, **is** labelled "Save" and
**is** disabled — rather than that some pixels look that way.

Where the image/colour anchors are geometric, `Text` is OCR and `Vision` is a learned
detector, this is **structured**: it reads what the application itself declares about
its own controls. That is why it sits at the top of the Director's source ladder, and
why a target found here needs no pixel corroboration.

## Protocol

Newline-delimited JSON on stdio (see `spec/Hosts.md`), one object per line.

```
→ {"act":"Accessibility","action":"Snapshot","input":{"Window":"hwnd:1234"}}
← {"status":"ok","data":{"WindowId":"hwnd:1234","App":"notepad","Elements":[…]}}

→ {"act":"Accessibility","action":"Available"}
← {"status":"ok","data":{"Available":true,"Reason":""}}
```

### Snapshot

Every input field is optional:

| Field | Default | Meaning |
|---|---|---|
| `Window` | foreground window | `"hwnd:<handle>"`. A value that isn't a usable handle is an **error**, not a fallback — see below. |
| `MaxNodes` | 1500 | Node ceiling. |
| `MaxDepth` | 40 | Depth ceiling. |
| `TimeoutMs` | 4000 | Wall-clock ceiling for the walk. |

Response fields beyond `Elements`: `WindowId`, `WindowTitle`, `App`, `ProcessId`,
`WindowX/Y/W/H`, `Minimized`, `Maximized`, `WindowVisible`, `ElapsedMs`, and —
importantly — `Partial` and `Reason`.

Each element carries `Id` (UIA RuntimeId), `ParentId`, `Role`, `ControlType`,
`Label`, `Value`, `Description`, `X/Y/W/H`, `Enabled`, `Visible`, `Focused`,
`Selected`, `Offscreen`, `AutomationId`, `ClassName`, `Depth`.

## Three decisions worth knowing about

**`Partial` is load-bearing.** A walk that hit a node, depth or time limit sets
`Partial` with a `Reason`. Without it, a truncated snapshot would let the Director
conclude "the Save button does not exist" when the truth is "I stopped looking" —
and those two lead to very differently dangerous decisions.

**An unusable `Window` is an error.** Falling back to the foreground window would
answer a question about window X with a description of window Y, and the reply would
look perfectly successful. The Director would plan against the wrong application with
full confidence. (This was a real bug, caught by the first live test.)

**Bulk cache, not property-by-property.** Managed UI Automation makes a
cross-process call per property read: a 40-control dialog at 15 properties each is
600 round trips. A single `CacheRequest` over the subtree fetches everything at once.
That is the difference between a snapshot taking ~130ms and taking seconds — and the
Director re-observes after every meaningful step. Applications that refuse a subtree
cache fall back to a live walk, and say so in `Reason`.

## Why C#, and why a subprocess

UI Automation is a COM API. Managed UIA makes the tree walk straightforward where
hand-rolled COM vtable calls from Go would not be. Making it a **subprocess** means
that choice never reaches Marco's zero-dependency engine — the same reason `ocr` and
`vision` are separate.

The Director sees this only through `directorapi.AccessibilityProvider`, so replacing
it with an in-process implementation later is an optimisation, not a rewrite. That
was the explicit trade: ship the tree walk now, revisit latency with measurements.

## Build

```sh
powershell -File plugins/uia/build.ps1
```

No SDK, no NuGet: it compiles with the .NET Framework `csc.exe` that every Windows
machine already has, referencing `UIAutomationClient` / `UIAutomationTypes` /
`WindowsBase` out of the GAC. The result is a single self-contained `uia.exe`, which
is how Marco's other plugin binaries are distributed.

## Recording fixtures

The Go side (`internal/platform/uiaclient`) is tested entirely against recorded
snapshots, so the Director's perception, fusion, ranking and planning can all be
developed with no live desktop.

```sh
powershell -File plugins/uia/record-fixtures.ps1        # the built test dialogs
.\uia.exe snapshot out.json --delay 5000                # a real app, by focus
.\uia.exe snapshot out.json --window <hwnd> --quiet     # a real app, by handle
```

`record-fixtures.ps1` waits for each window's element count to stop growing before
saving. A form's UIA provider publishes its children asynchronously, so without that
wait the first fixture records the window chrome and none of the controls — a file
that looks plausible and is silently wrong.

## Known limits

- **Windows only.** macOS (AX) and Linux (AT-SPI) would be sibling plugins behind the
  same `AccessibilityProvider` interface.
- **Coverage is uneven in the wild.** Electron apps (Discord, VS Code) expose a tree
  only with accessibility enabled, and games typically expose nothing at all — a
  fullscreen game returns exactly one element, its own window. This is expected, and
  it is the entire reason the Director has a source ladder rather than one source.
- **Reads only.** Actions go through Marco's executor
  (`internal/platform/marcohost`); this plugin never touches input.
