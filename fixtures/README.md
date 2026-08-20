# fixtures — recorded desktops for the Director's tests

These are real accessibility trees, captured from real windows, checked in as JSON.

They exist so that Director development **stops needing a live desktop**. Everything
downstream of perception — observation fusion, stable element identity, target
ranking, planning, verification — is developed and tested against these files. That
makes the test suite deterministic, runnable in CI, and incapable of clicking
anything on the author's machine.

Testing a desktop agent only by driving a live desktop is how you get a suite that is
slow, flaky, unrunnable on a build server, and occasionally destructive.

## Layout

```
fixtures/
└── save-dialog/
    ├── accessibility.json     the UIA snapshot (recorded)
    ├── windows.json           window/monitor metadata      (not yet recorded)
    ├── ocr.json               text observations            (not yet recorded)
    ├── screenshot.png         optional frame               (not yet recorded)
    ├── request.txt            the user's words
    ├── expected-world.json    the World Model this should produce
    └── expected-plan.json     the plan it should produce
```

Only `accessibility.json` exists today — the vertical slice is accessibility-only by
design (see `ANALYSIS.md` §7). The other files arrive with the phases that need them.

## The three recorded cases

| Fixture | What it is for |
|---|---|
| `save-dialog` | The vertical slice's target. **Four** elements match "Save": an inert text label, the real button, a menu, and a "Save As…" menu item. Ranking must pick the button — on role and exactness, not on text matching. |
| `duplicate-labels` | Two identical "Apply" buttons, one in an "Audio" group and one in "Video". Label alone cannot decide. The Director must either disambiguate through parent structure or **ask** — never silently pick one. |
| `disabled-button` | A greyed-out "Save". The correct answer is "Save is disabled" — not a click that reports success and does nothing, and not "there is no Save button". |

## Recording

```sh
powershell -File plugins/uia/record-fixtures.ps1
```

The three cases above are **built** rather than borrowed from real applications, for
two reasons: a real app's tree changes with its version and theme, so a fixture
recorded today starts failing next month for no useful reason; and the cases that
matter most (duplicate labels, a disabled control) are awkward to produce on demand.
The windows are genuine and so are their UI Automation trees — only the content is
under our control.

Real-application captures are complementary, not a replacement, and are worth adding
for anything with a surprising tree:

```sh
plugins/uia/uia.exe snapshot fixtures/<name>/accessibility.json --delay 5000
# then bring the window you want to the front
```

## Rules

- **Fixtures are checked in.** A test that regenerates its own input tests nothing.
- **Re-record deliberately.** A fixture changing should be a reviewed diff, not a
  side effect of running the suite.
- **No sensitive content.** These are readable by anyone with repo access. Never
  record a window containing credentials, personal data, or private messages.
