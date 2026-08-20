---
status: experimental
---

---
# Inference {#inference}
---

---
## Core Rule {#core-rule}
---

Marco should infer names, input, and result mappings whenever there is exactly one valid interpretation.

If inference is ambiguous, the compiler must require explicit syntax.

---
## Frame Set Model {#frame-set-model}
---

- `it` = current Frame
- `it's` = current Frame status
- `its` = current Frame set
- `input` = current Frame input set
- `that` = most recent completed child or observed Frame
- `that's` = access to data on `that`'s result set
- in data-flow contexts (`with that`), `that` is unwrapped to its result

---
## Input Inference {#input-inference}
---

Input values passed with `with` become the current Frame's input set.

### Example

Canonical:
```marco
do Save with SaveArgs...
```

Inside `Save`, these are equivalent when unambiguous:

Canonical:
```marco
input's File
File

input's User
User
```

If a name exists in multiple accessible sets, explicit access is required.

---
## Result Inference {#result-inference}
---

Returned values are matched to the expected result shape when possible.

### Example

Canonical:
```marco
this is ok with that!
```

If `that` already matches the expected result type, no explicit cast is required.

If the returned value must change type, use:

`as a`

### Example

Canonical:
```marco
this is ok with that as a CustomUser!
```

Rules:

- if the result already matches the expected type, no cast is required
- if the type does not match, `as a <Type>` is required
- Marco may infer field mapping when unambiguous
- if mapping is ambiguous, a translator or [[Modules#Scenes|scene]] must be defined

---
## Error Inference {#error-inference}
---

This is legal shorthand:

Canonical:
```marco
this is failed with error "save failed"!
```

It creates an error whose `Message` is `"save failed"` on the Frame's `error` slot. See [[Lifecycles#Errors]] for the full error model and the canonical expanded form.

This is also legal:

Canonical:
```marco
this is failed with that!
```

If the `failed` status expects an error, and `that` can resolve to an error, Marco maps it to `its error`.

Canonical expanded meaning:

Canonical:
```marco
this is failed with its error!
```

Rule:

Status data may be inferred from context when the contract defines exactly one valid target.

---
## Ambiguity Rule {#ambiguity-rule}
---

Inference is allowed only when exactly one valid mapping exists.

If multiple mappings exist, compiler error.

### Example

Canonical:
```marco
this is ok with that!
```

Invalid if the contract expects multiple result values and `that` cannot be unambiguously mapped.

The author must write:

Canonical:
```marco
this is ok with that as a User, and Metadata as Meta!
```

---
## Guiding Principle {#guiding-principle}
---

Marco should feel magical when the graph makes the answer obvious.

Marco must require explicit syntax when the graph cannot prove the answer.
