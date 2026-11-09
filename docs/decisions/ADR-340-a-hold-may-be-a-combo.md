---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — A HOLD MAY BE A COMBO. `do OS's KeyDown with \"shift+w\"` holds Shift and then W, and the KeyUp of the same combo releases W and then Shift. Each key is written down as it goes, so the route-end sweep releases whatever is still down; a key refused part way through a combo's press lets go of the keys that call pressed before it reports; a combo's release attempts every key even when one is refused, and a refused one stays for the sweep. The step editor accepts a combo for a hold, and every key box accepts the spaces people write around a `+` (\"Shift + W\" is `shift+w`)."
supersedes: []
affects:
  - execution
source_paths:
  - internal/oshost/oshost.go
  - internal/orchestrator/stepedit.go
---

# A hold may be a combo

Follows [[ADR-339-a-modifier-pressed-alone-is-a-key]].

## Why

The owner typed `shift + w` into a hold in Studio's step editor and was told a hold is one key.
For a game, holding Shift and W together is how you sprint forward, and two holds in a row were
the only way to say it.

## What did not change

- The recorder still writes a hold per key, at each key's own press and release. A combo hold is
  what a person writes, not what a recording becomes.
- `do OS's Key with "ctrl+shift"` was already a chord: Ctrl held while Shift is tapped.

## Not established

- Not tried on a desktop or in a game.
- A combo hold sharing a key with another open hold (hold Shift, then hold `shift+w`, then release
  `shift+w`) lets go of the Shift the first hold is still holding.

## Enforced by

- `internal/oshost`: `TestAComboIsHeldKeyByKeyAndReleasedInReverse`, `TestAComboRefusedPartWayLetsGoOfWhatWentDown`, `TestAComboKeyThatWillNotComeUpIsLeftForTheSweep`.
- `internal/orchestrator`: `TestAHoldCanBeChangedToACombo`, `TestAPressWrittenWithSpacesIsTheCombo`, `TestAKeyMarcoCannotPressIsRefusedAtReview`.
