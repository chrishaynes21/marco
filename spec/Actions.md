---
status: reference
---

---
# Actions {#actions}
---

---
## Core Rule {#core-rule}
---

`does` opens a phrase that defines behavior.

`does` is the only keyword required to open a function/action body.

One-liner:

`does` is the universal behavior opener.

---
## Canonical Behavior Opener {#canonical-behavior-opener}
---

Canonical behavior opener:

```marco
does...
```

Examples:

Canonical:
```marco
Save does...
this does...
this's Save does...
```

Meaning:

- opens a phrase
- creates a Frame
- defines behavior for the subject

---
## Action Declaration Forms {#action-declaration-forms}
---

Marco supports multiple equivalent ways to declare behavior.

All of the following are legal:

```marco
X does...
this does...
this's Save does...
this can Save.
that does...
this can Save.
this's Save does...
```

---
## Semantics {#semantics}
---

`X does...`

Defines behavior for `X` directly.

Example:

Canonical:
```marco
Save does...
    this ok!
```

`this does...`

Defines behavior for the current context.

Example:

Canonical:
```marco
the Save is an action.
this does...
    this ok!
```

`this's Save does...`

Defines behavior for a named member on `this`.

Example:

Canonical:
```marco
the Game is an actor.
this's Save does...
    this ok!
```

`this can Save.` + `that does...`

Declares a capability and defines its behavior inline.

Example:

Canonical:
```marco
the Game is an actor.
this can Save.
that does...
    this ok!
```

Meaning:

- declares `Save` as a callable action
- the declaration produces a Frame representing `Save`
- `that` refers to that Frame as the most recent result

`this can Save.` + `this's Save does...`

Equivalent to the previous form, but split.

Example:

Canonical:
```marco
the Game is an actor.
this can Save.
this's Save does...
    this ok!
```

---
## Naming Rule {#naming-rule}
---

All behavior definitions reduce to:

```marco
does...
```

Naming:

- `this can X` declares the action
- `does` defines the implementation
- `that` inside `can` follows the same global rule as everywhere else

---
## `that` Inside `can` {#that-inside-can}
---

Core rule:

- `that` always refers to the result of the most recent Frame

This does not change inside `can`.

Correct interpretation:

Canonical:
```marco
this can Save.
```

Meaning:

- the current context (`this`) declares a capability named `Save`
- a new Frame representing the `Save` action is introduced
- that Frame becomes the most recent result
- `that` now refers to the `Save` action Frame

Therefore:

Canonical:
```marco
this can Save.
that does...
```

Meaning:

- `Save` is declared
- the declaration produces a Frame (the `Save` action)
- `that` refers to that Frame
- `does` defines behavior for that Frame

Equivalent forms:

```marco
this can Save.
that does...
```

is equivalent to:

```marco
this can Save.
this's Save does...
```

Important clarification:

- `that` does not mean "previous result" in a vague sense
- it always means the result of the immediately preceding Frame
- in this case, `this can Save.` produces a Frame representing the `Save` action, and `that` refers to that Frame

One-liner:

- `that` always refers to the most recent Frame result, including action declarations

---
## Constraint {#constraint}
---

Do not introduce additional keywords for defining functions or actions.

All callable behavior must ultimately be defined using `does`.

Do not redefine `that` behavior inside `can`.

It must remain consistent with all other uses of `that`.
