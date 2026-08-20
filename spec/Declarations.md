---
status: reference
---

---
# Declarations {#declarations}
---

Declarations are written in **declaration mode** — `it` refers to the current declared subject, and only `the` updates `this`. See [[Reference Modes#Declaration Mode]].

---
## Canonical Form {#canonical-form}
---

Canonical declaration form:

`the <Name> is a <Type>.`

Examples:

Canonical:
```marco
the Game is an actor.
the Saveable is a contract.
the Saved is a status.
the Save is an error.
the Preferences is a set.
```

---
## Field Declarations {#field-declarations}
---

Field/member declaration form:

`this's <Member> is a <Type>.`

Examples:

Canonical:
```marco
this's SaveFile is a File.
this's Preferences is a set.
this's Password is a text Secret.
```

---
## Declaration Frames {#declaration-frames}
---

Declarations create [[Frames|Frames]].

Declaration Frames are implicitly `ok` and usually not inspected.

---
## Declaration Focus {#declaration-focus}
---

`the <Name> is a <Type>.`

Rules:

- creates or declares `<Name>`
- binds `<Name>`
- sets `this` to `<Name>`
- produces a Frame
- sets `that` to `<Name>`

### Example

Canonical:
```marco
the Saveable is a contract.
```

After this line:

- `Saveable` is declared
- `this` is `Saveable`
- `that` is `Saveable`

---
## Field Declaration Focus {#field-declaration-focus}
---

`this's <Member> is a <Type>.`

Rules:

- declares a member on `this`
- does not change `this`
- produces a Frame
- sets `that` to the declared member

### Example

Canonical:
```marco
this's SaveFile is a File.
```

After this line:

- `this` is unchanged
- `that` refers to `SaveFile`

---
## Open Questions {#open-questions}
---

- Whether declaration Frames are always `ok` or may be surfaced as non-`ok` in validation contexts.
- Whether duplicate declarations of the same name are errors or rebinding.
