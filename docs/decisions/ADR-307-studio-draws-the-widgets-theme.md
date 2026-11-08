---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — STUDIO DRAWS THE WIDGET'S THEME. The widget's ten palettes moved, unchanged, into internal/widgetconfig as data. The widget draws from them, and Studio's page tokens are derived from them by one Go mapping (StudioTokens). The one `theme` setting picks the palette for both, `default` included, so the widget's default dark palette replaces Studio's OS-selected light or dark; `light` stays available, and there is no separate Studio preference. An unknown name resolves to the default, as the widget always resolved it. Every text colour is held to 4.5:1 on every surface it can sit on (the ground, a card, a field and each tinted banner), the accent included, because it is ordinary text in outlined Play labels, links and hints; the ink is held to 7:1 on the plain surfaces. Warning stays Studio's own amber, lifted for readability and kept amber. The accent falls back to the palette's listening colour when its own is within 30° of hue of ok, warning or refusal, a heuristic about meaning and not an accessibility guarantee. Studio applies the tokens at start-up and on its ten-second readiness poll. The widget still reads its settings only at launch, so a theme changed outside its own panel reaches the widget only on restart."
supersedes: []
affects:
  - interface
source_paths:
  - internal/widgetconfig/palettes.go
  - internal/widgetconfig/studiotokens.go
  - internal/widgetconfig/palettes_test.go
  - plugins/overlay/view.go
  - plugins/overlay/config.go
  - plugins/overlay/sharedtheme_test.go
  - plugins/studio/theme.go
  - plugins/studio/themepage_test.go
  - plugins/studio/frontend/index.html
  - cmd/marco/edittheme_test.go
---

# Studio draws the widget's theme

The shared styling and themes step after S0–S9 of [[Surfaces-alignment-proposal-2026-11-02]].
The mapping was rendered before it was built, and the owner's rulings are in
[[S10-shared-themes-review-2026-11-04]].

## What was decided

- **One set of palettes** (`internal/widgetconfig/palettes.go`).
  - The ten palettes moved from `plugins/overlay/view.go` as hex strings, with the same values.
  - The widget builds its colours from them (`themeFrom`) and cycles `PaletteNames`.
  - `PaletteFor` resolves a name: case-insensitive, and `default` for anything else. The widget,
    Studio and the control centre's list all use these.
- **One mapping** (`StudioTokens`). It turns a palette into Studio's CSS tokens.
  - Light or dark is the palette's (its background's luminance), not the desktop's.
  - **Text is measured where it is drawn.** Each text colour (ink, secondary, faint, accent, ok,
    warning, refusal) is lifted toward the ink until it reads at 4.5:1 on the ground, a raised
    card, a sunken field and all four tinted banners. The tints are recomputed until nothing moves.
  - The ink reads at 7:1 on the ground, a card and a field.
  - The accent is held to the text floor, not 3:1, because it labels outlined Play buttons and
    links.
  - The primary button's label is the ground, which reads on the accent by the rule above, and on
    the ink the button turns to under the pointer.
  - Dividers are held to 1.6:1.
- **The accent.** When the palette's `name` colour is within 30° of hue of ok, warning or refusal,
  its `listen` colour is the accent instead. In the current catalogue that is gruvbox-dark and
  monokai. The final, lifted accent is then measured against the final semantic colours for every
  palette, fallback included. This is a heuristic about what a colour is taken to mean. It is not
  an accessibility guarantee, and the labels ("Won't run", "Recording", "Unchecked") carry the
  meaning.
- **Warning is Studio's amber** (`#e3a73c` on a dark palette, `#8f5f07` on a light one), lifted
  like any text colour and checked to stay within 12° of that hue.
- **Studio asks and applies.** `Acts.Theme` returns the resolved theme and its tokens from
  `widgetconfig.Load().Theme`. The page sets them on the root at start-up and on its ten-second
  readiness poll, ahead of that poll's busy check, because it is only a read. It also sets
  `color-scheme`, so the host's own controls follow the palette. If there is no answer, the
  stylesheet's tokens stay.
- **Two fixed whites became tokens**: the text selection (now the accent with the ground on it)
  and the Stop button's hover label (now the ground on the refusal colour). Neither fixed white
  read on every palette.

## Not changed, and not claimed

- **Live reload.** The widget reads `overlay.json` once, at launch, and applies at once only a
  change made in its own panel. A theme changed in the control centre or in the file reaches an
  open Studio within ten seconds and reaches the widget only when it restarts. This does not make
  both surfaces update live. *Superseded by [[ADR-331-widget-settings-in-studio]]: the widget now follows the file.*
- **Semantic colours among themselves.** In gruvbox-dark and solarized-dark, the palette's own
  ok or refusal colour is within 30° of hue of the warning amber (gruvbox: ok 23°, refusal 25°;
  solarized: ok 29°). The palette values are the widget's, and the rulings keep both them and the
  amber. The labels distinguish these states; the colour alone does not.
- **Disabled controls** are drawn at reduced opacity, as before. A disabled Play measured about
  2.1–2.4:1 across palettes, against 2.11–2.14:1 for Studio's own dark tokens before this change.
  Its label and the reason beside it are unchanged.
- **No desktop.** These were rendered in headless Edge. WebView2 is expected to match, but that is
  not established.

## Enforced by

`internal/widgetconfig`:
- `TestThePalettesAreTheWidgetsOwn`: every value, as shipped.
- `TestAThemeNameResolvesAsTheWidgetResolvesIt`: case, unknown and empty names.
- `TestEveryPaletteGivesStudioReadablePairs`: every text token on every plain and tinted surface,
  the button label on the accent and on the ink, the focus ring and dividers.
- `TestTheAccentStaysClearOfOkWarningAndRefusal`: the final accent, fallback included, and which
  palettes fall back.
- `TestTheWarningIsStillAmber`.

`plugins/overlay`: `TestTheWidgetDrawsTheSharedPalettes`.

`plugins/studio`:
- `TestStudioDrawsTheWidgetsThemeAndFollowsAChange`: the real page, rendered, with the real
  token sets.
- `TestStudioKeepsItsOwnTokensWhenThereIsNoTheme`.
- `TestEveryMethodThePageCallsIsOnTheBackend`.

`cmd/marco`: `TestTheControlCentreOffersTheSharedThemes`.

Each guard was mutated and its test failed, with one exception. Removing the page's `.catch` on
a failed theme answer survives: it only silences an unhandled rejection, and the poll continues
without it.
