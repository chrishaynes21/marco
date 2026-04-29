---
# Modules {#modules}
---

Marco programs are organized into scripts and modules.

---
## Scripts {#scripts}
---

A script is an entry point.

### Example

Canonical:
```marco
the App is a script. do MacroMarco's Start...
```

---
## Module Contents {#module-contents}
---

Modules may contain:

- actors
- acts
- scenes
- sets
- contracts

### Example Layout

```text
script.play
acts/
actors/
scenes/
sets/
```

---
## Acts {#acts}
---

Acts represent external systems or integrations.

### Example

Canonical:
```marco
the Ahk is an act.
```

Acts expose callable behavior:

Canonical:
```marco
this exports Click.
this exports SendText.
```

Acts may emit events:

Canonical:
```marco
when Ahk hears HotkeyPressed?
```

---
## Scenes {#scenes}
---

Scenes represent composed flows or UI/state groupings.

### Example

Canonical:
```marco
the SaveScene is a scene.
this can Render.
this can HandleInput.
```

Scenes behave like structured actors focused on orchestration.

---
## Show — UI Rendering {#show-ui-rendering}
---

`show` is a built-in action that renders a UI component or scene element.

### Canonical Form

```marco
show <Component>.
show <Component> with <Data>.
```

### Examples

Canonical:
```marco
show ErrorBanner with that's error.
show SaveButton.
```

Rules:

- `show` is for UI rendering, not for debugging or inspection
- for inspection at runtime, see [[Observability#Debugging]]
- for execution observation, see [[Observability#Logging]] and [[Observability#Observation]]
- `show` is a system built-in (lowercase). A user-defined `Show` may override it — see [[Core Concepts#Naming Convention]].

---
## Actors {#actors}
---

Actors are long-lived entities that own state, hear and say messages, and coordinate execution. They are miniature state machines.

See [[Actors]] for the actor model, [[Actors#Messaging Syntax|messaging syntax]] (`say`/`hears`), [[Actors#Group Routing Via Sets|group routing]], and [[Actors#Built-In Actors|built-in actors]] (`Frame`).

---
## Imports {#imports}
---

Imports are lazy and selective. `use` exposes a module without eagerly pulling every symbol into the local namespace — only **referenced** names become visible.

### Canonical Behavior

```marco
use Ahk.

do Ahk's Click with Button.
```

Effect:

- `Click` is now available
- nothing else from `Ahk` is imported unless used

### After `use`

Qualification may be omitted when unambiguous:

```marco
do Click with Button.
```

Allowed only if:

- `Click` exists in exactly one imported module
- no local symbol conflicts with `Click`

Qualified access (`<Module>'s <Action>`) always works.

### Collision Rule

If multiple modules export the same name:

```marco
use FileSystem.
use CloudStorage.

do Read.   // ambiguous
```

Compiler error:

```
Read is ambiguous.
Possible sources:
  - FileSystem's Read
  - CloudStorage's Read
```

Fix:

```marco
do FileSystem's Read.
```

### Rules

- `use` exposes a module, not all of its names eagerly
- names become available only when referenced
- unqualified usage is allowed only when unambiguous
- qualification always works (`<Module>'s <Name>`)
- the compiler must prevent accidental collisions
- imports add nodes to the [[Graph]]
- imports do not automatically change `this`
- name resolution follows [[Scoping]]; imported modules sit at the global tier

### Explicit Import (Future, Not v1)

A symbol-level `use` form may be considered later:

```marco
use Ahk's Click.
use FileSystem's Read.
```

Not required for v1.

### Principle — Minimal Namespace

Marco keeps the namespace minimal. You only pull in what you actually use.

### One-Line Lock

**Imports are lazy and selective; only referenced names enter the namespace.**

---
## One-Line Lock {#one-line-lock}
---

Marco programs are composed of modules where actors, acts, and scenes coordinate execution through Frames, sets, and events.

---
## Open Questions {#open-questions}
---

- Visibility and export rules across modules.
- How acts surface event subscription versus polling.
- Lifecycle of actors relative to the owning script.
