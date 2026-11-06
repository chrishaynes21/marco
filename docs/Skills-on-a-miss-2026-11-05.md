---
type: reference
status: proposal
date: 2026-11-05
supersedes: []
source_paths:
  - plugins/overlay/intake.go
  - plugins/overlay/resolve.go
  - plugins/overlay/line.go
  - internal/resolveroute/resolveroute.go
  - cmd/marco/intake.go
  - cmd/marco/routinerecord.go
  - internal/routes/args.go
---

# Skills on a miss: what the widget can resolve when exact matching fails

The widget and Studio share one library, one resolution
([[ADR-267-director-owns-route-resolution]]) and one set of run guards. Exact names and the
deterministic grammar stay first and fast. On a miss, a model proposes a known **skill** with
structured inputs, and Marco resolves and validates the identities, values and scope, asks when
the request is ambiguous, and runs it through the review and execution paths it already has.
Voice reaches all of this through the same intake: [[ADR-318-the-widget-starts-its-own-microphone]].

## Where each skill stands

**Status:** **works** means a person can do it from the widget today. **wire** means the operation
exists and the miss path doesn't reach it. **new** means it needs execution behaviour that doesn't
exist.

| Skill | Example | Status | What exists | What is missing |
|---|---|---|---|---|
| Find by meaning | "bring up the social menu" → `open social` | **works** | Miss → `marco resolve --json` → `resolveroute.Resolve`, exact, then the advisor (`resolveroute.go:224,247`) → offered (`plugins/overlay/resolve.go:183`) | Meaning needs `$MARCO_RESOLVER`; without one only exact names and aliases match |
| Fill arguments | "pull some stone from the cloud" → `resource: stone` | **works** | `proposals()` asks for word spans and copies values out of Marco's copy of the request (`resolveroute.go:340-379`); the widget reviews them editably (`line.go` `reviewProposal`) | Needs a model that answers the extract question |
| Chain | "open social, then mute Discord" | **wire** for exact names, **new** by meaning | `routes.SplitChain` + `grammar()` run a chain when every step is an exact name (`cmd/marco/intake.go:347-370`) | A step that is not exact sends the WHOLE phrase to Director. No per-step resolution, no ordered review, no plan |
| Explain a refusal | "why didn't that work?" | **works** (deterministic) — [[ADR-320-why-didnt-that-work]] | The intake writes one record of the latest attempt, refusals before the run included (`internal/lastattempt`); `marco last`; the widget answers a fixed set of phrasings from it, typed or spoken | Director's own sentence for a Director-run request is not carried back; a phrasing outside the fixed set is an ordinary request |
| Discover | "what can I run in this game?" | **works** as the word `plays`; **wire** by meaning | `plays` groups `marco routes --json` by scope for the app in front; `marco bindings --here` lists shortcuts | The resolver answers only run/learn/chat/clarify, so the question is not mapped to the listing |
| Recall / rerun | "what did I just run?", "run that again" | **wire** for recall, **new** for rerun | The widget's history is in memory; routine records are durable | No rerun of a routine. "do that again" is Director's `IntentRepeat`, which replays a Director action, not a saved routine |
| Prepare a shortcut | "put mute Discord on E, only in this game" | **works** as `bind`; **wire** by meaning | `bind <key> <routine>` → `marco bind`, scoped to the app in front by default, stored by identity | The sentence is not turned into a bind proposal for review |
| Navigate Studio | "show me where to edit cloud pull" | **new** | `studio -view <view>` and the one-window hand-off ([[ADR-317-the-widgets-help-opens-studio]]) open a rail view | Nothing opens Studio on ONE routine |
| Repeat a routine N times | "repeat open social five times" | **new** (design only) | Director's count grammar takes only bare "repeat that N times" | No count on a routine invocation. Design: the plan carries `count`, the review shows it, and execution waits for a later slice |

## One plan, whoever produced it

The resolver answers one routine pick today (`ResolveRouteResponse`: `kind`, `id/app/slug/focus`,
`values`, `sourceAt`, `candidates`). The design keeps it and adds steps:

```
plan {
  skill:   find | fill | chain | explain | discover | recall | rerun | shortcut | navigate | repeat
  steps:   [ { id, app, slug, focus, values:[{name, value, from}], sourceAt } ]   // in order
  count:   n               // repeat only; reviewed, not executed yet
  target:  { key, scope }  // shortcut only
  ask:     "<question>"    // when it is ambiguous; nothing runs
  source:  exact | grammar | advisor
}
```

- **The grammar and the model produce the same plan.** `SplitChain` on exact names is a plan with
  `source: grammar`. A model's proposal is the same shape with `source: advisor`.
- **A model proposes a skill and words, never an identity.** Every step's routine is resolved by
  `resolveroute` against the library and scope in front. Every value is a span copied out of
  Marco's own copy of the request, as `proposals()` does today. Anything that fails to resolve
  makes the plan an `ask`.
- **A plan is reviewed before anything runs**, on the widget's command line (or Studio's card):
  each step's read-back, in order. Each step then runs through the existing explicit-identity door
  (`--play/--app/--focus/--expect/--arg`), so every run guard is the one already there.
- **Credential values never reach a model.** A secret argument is not offered to the extract
  question (`resolveroute.go:326-330`), and a span cannot carry text. **Supplied text stays out of
  durable logs**: routine records hold no argument values, and a reviewed run logs the routine id
  and an argument count. The widget's log now carries a phrase's length, never its words, and
  redacts a child's echo of the request and its values; voice.exe logs a word count; the latest-
  attempt record is scrubbed ([[ADR-319-the-installer-ships-voice]], [[ADR-320-why-didnt-that-work]]).
  The engine's own `intake: routed` line and an unknown command's error still name the phrase on
  `marco`'s stderr.

## Explain is built; Chain is next, and brings the plan

**Explain landed** as a deterministic lookup ([[ADR-320-why-didnt-that-work]]): items 1–3 below,
with one record of the latest attempt rather than a refusal written into the routine run records
(a refusal pressed nothing, so it has no run to cite). Item 4, the plan type, was deliberately NOT
built: Explain is one step and needed none. **Chain is the next slice and introduces the plan
format**, because it is the first real multi-step operation the format has to represent. Repeat
stays on the list for later.

The original Explain plan, for the record:

1. **Record the early refusal.** A run refused before it is claimed (a missing value, a stale
   version) writes the same routine record with `Outcome: refused` and its `Because`. It carries no
   argument values.
2. **`marco last --json`**: the most recent routine record for this library, in a person's words,
   with the remedy when the refusal is one Marco knows (a value missing → "say it with
   `resource:…`"; saved since the read-back → "read it again"; another application in front →
   which application).
3. **The resolver's `explain` skill.** "why didn't that work?" returns `plan{skill: explain}`, and
   the widget shows `marco last`. Its exact phrasings (`why`, `what happened`) are in the
   deterministic grammar, so the common case asks no model.
4. **The plan type in `ResolveRouteResponse`**, with the existing single pick as a one-step
   `find`/`fill` plan, so the widget and Studio read one shape.

**Then Chain:** split with `SplitChain`, resolve each segment through the same `Resolve`, review the
ordered steps, and run them one after another through the identity door, stopping at the first
step that does not perform and saying which.
