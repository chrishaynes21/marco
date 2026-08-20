# Marco — end-to-end test guide

A manual walkthrough of every shipped feature. Each step lists **what to do** and
**what you should see (PASS)**. Work top to bottom; later sections build on earlier
ones. Windows, with the overlay stack running unless noted.

```powershell
.\setup.cmd            # build engine + macros + overlay (add -Voice for the mic)
.\overlay.cmd          # launch the HUD stack (leave it running)
```

> Tip: to avoid touching your real routes, run CLI checks with a throwaway store:
> `$env:MARCO_ROUTES="$env:TEMP\marco-e2e"`. Unset it (`Remove-Item Env:MARCO_ROUTES`)
> to go back to the real `.\routes`.

In the overlay, **`` ` ``** (backtick) is the leader; **`` `m ``** opens the command
line. Commands below written as `` `m <thing>`` mean: press backtick, then `m`, then
type `<thing>`, then Enter.

---

## A. Teach & run basics

1. **Teach by demonstration**
   `` `m teach open notepad`` → Notepad-or-anything demo, then press **F12**.
   - PASS: the HUD shows "recording … press F12", then a **save prompt drops in
     below the command line** with a live **`● rec M:SS` timer** while you record.

2. **Answer the prompts (type-then-Enter)**
   At `Save this as "open notepad"?` press **y** then **Enter**; at `Make it
   available only in <app>?` press **y** then **Enter**.
   - PASS: each prompt shows your selection as `› y` on its own line, the prompts
     stack as a transcript, and it saves scoped to the app. **Esc = no.**

3. **Run it**
   `` `m open notepad`` (or `marco do "open notepad"` in a terminal).
   - PASS: the route replays.

4. **Scope happy-path** — re-teach something and just press **Enter, Enter** at the
   two prompts.
   - PASS: saved, scoped to the app (app-only is the default; "no" would make it
     global).

5. **List / forget**
   `marco routes` lists them. `marco forget "open notepad"` asks `Forget …?` →
   **y**.
   - PASS: gone from `marco routes`.

6. **Forget all** — `marco forget all` (or `` `m delete all`` in the HUD).
   - PASS: `Forget ALL N routes? This can't be undone.` → **y** wipes them; **n**
     keeps them. (Earlier this wrongly said "no route named delete all".)

---

## B. Arguments

7. **Named argument (demonstrate)**
   `` `m teach say hello with name`` → in the demo, open a text field and **tap F9
   where the name should go** (don't type a name), then **F12**, save.
   - PASS: the recording hint reads `… tap F9 where each arg goes, in order: name`.

8. **Run with a value**
   `` `m say hello name:chris`` (or `marco do "say hello name:chris"`).
   - PASS: it types **chris** where you tapped F9.
   - Check the engine sees it: `marco args "say hello"` → prints `name`.

9. **Several args / spaces**
   Teach `dm with person, message` (tap F9 twice, in order). Run
   `marco do "dm person:sam message:hi there"`.
   - PASS: `person` = `sam`, `message` = `hi there` (value runs to the next `key:`).

10. **Secret argument — provide once, then remembered**
    Teach `login to facebook with username, password` (tap F9 for each).
    - `marco do "login to facebook username:me password:hunter2"` → types both.
    - `marco do "login to facebook"` (omit them) → reuses the remembered values.
    - PASS: the second run works without re-entering. Open the saved
      `routes/.../login-to-facebook.marco`: it contains
      `do OS's Secret with "login-to-facebook:password"` and **no plaintext
      password** (username may appear as `{{username}}`).

11. **Global `{{name}}` secret still works** — teach a route where you type
    `{{token}}` literally, then `marco secret set token`.
    - PASS: route stores only the name; the value comes from the credential store.

---

## C. Teach by talking (typed or voice)

12. **Typed narration (no mic needed)**
    ```
    "activate notepad`ntype hello`ndone" | marco teach --narrate "quick note"
    ```
    - PASS: prints each parsed step (`type "hello"`, `done`) and `Learned "quick
      note"`.

13. **Narration vocabulary** — try phrases: `click this`, `anchor this`, `wait for
    this screen`, `wait 2 seconds`, `press enter`, `activate <app>`, `undo`,
    `cancel`, `done`.
    - PASS: each becomes the right step; `undo` drops the last; `cancel` saves
      nothing.

14. **Hands-free in the overlay** — `` `m narrate teach open chest``, then say or
    type each phrase, finish with `done`.
    - PASS: per-step status streams into the HUD; `done` saves. (With `-Voice` set
      up, speak them; otherwise type each `` `m <phrase>``.)
    - **Continuous-listen (voice only):** say `"marco narrate teach open chest"` to
      start, then say `"click this"`, `"type hello"`, etc. — the wake word is only
      needed once. The overlay creates `$TEMP\marco-narrate.lock` while the session
      is live; the voice plugin re-arms automatically after each phrase. The file is
      removed when `done`/`cancel` exits the session.

---

## D. Overlay UX

15. **Auto-popped argument labels**
    Start typing a route that takes args, e.g. `` `m login to facebook`` and pause.
    - PASS: after a beat, **highlighted `username: password:  (tab)`** appears after
      your text. **Tab** appends `username:` to the line so you just type the value;
      it then offers `password:`.

16. **Opacity & focus**
    - PASS: idle HUD is clearly visible (not nearly invisible); when you open the
      command line (**focus**) the panel goes **fully solid**.
    - Open config (`` `m config``), select **opacity**, press ←/→.
      - PASS: the panel opacity **changes live as you drag** (it used to look inert).

17. **Prompt handshake on any command** — `` `m forget open notepad`` in the HUD.
    - PASS: the `Forget …?` confirm appears below the command and takes **y/Enter**
      or **n/Esc**, same as teach.

18. **Leader / command line basics** — `` `m help``, `` `m exit``.
    - PASS: help menu shows; `exit` closes the overlay.

---

## E. Robustness

19. **Window-relative clicks (the default)**
    Teach a click in a windowed app. **Move the window** (or drag it to another
    monitor). Run the route.
    - PASS: it clicks the right spot relative to the window, no monitor/DPI setup.

20. **Image anchors (opt-in)**
    Re-teach with anchors on: set `$env:MARCO_ANCHORS="1"` before launching, teach a
    click, then resize/move things and run.
    - PASS: the route finds the target and clicks it; the saved route has an `Anchor`
      with an `Image` (and `Color`/`Window`), and falls back to the recorded point when
      it can't resolve. The saved `*-anchor-*.png` is cropped tight to the **button**
      (auto-crop), not a big square of background. (More anchor checks in section F.)

21. **Stop key** — start a long route, press **F12**.
    - PASS: it aborts mid-run. (F12 also ends recordings; change with
      `$env:MARCO_STOP_KEY`.)

22. **Diagnostics** — `marco diag`.
    - PASS: prints DPI awareness, virtual-desktop bounds, cursor position, and a
      SetCursorPos round-trip that lands exactly (incl. negative-origin monitors).

---

## F. Anchors (recognition) & key holds

> Turn on the scoring trace first: the overlay console (`.\overlay.cmd`) prints
> `find: candidate signals=image=…,edge=… colour=… …` and `find: resolved …` once
> `MARCO_LOG=debug`. The default launcher is `info` (quiet); set `MARCO_LOG=debug`
> in the environment before `.\overlay.cmd` to watch the decisions.

23. **Anchor follows a moved / re-themed target**
    Anchor a click on a button (tap **F12**, then click it). Then change the scene so
    the button is in a **different place** (scroll/reflow) or **recoloured** (switch the
    app's light/dark theme), and run the route.
    - PASS: it clicks the button where it **now** is, not the old coordinate — the
      `when ok?` branch fires (`do OS's Click with that`). At `MARCO_LOG=debug` you see
      `edge=` carry the recolour and `image=` survive a brightness/scale change. A
      target it genuinely can't find → low-confidence **Warn** and the recorded-point
      fallback (it never clicks the wrong place confidently).

24. **Window-name context (multi-window app)**
    In an app that opens several windows (Steam: Library vs. Friends), anchor a click in
    **one** window via a **focus** route (so the route activates the app). Bring a
    **different** window of the same app to the front, then run the route.
    - PASS: with the wrong window in front the anchor won't confidently click (a
      `find: foreground window mismatch — penalising confidence` line at debug); with the
      right window it resolves normally. The saved anchor carries `Window "<title>"`.

25. **Text resolver snaps to the button (needs `-OCR`)**
    `setup.cmd -OCR` once. Narrate an anchor over a labelled button: `` `m narrate teach
    mute me``, say `click the text Mute`, `done`. Run it.
    - PASS: it OCRs "Mute" and clicks the **centre of the button**, not the glyphs; the
      route has a `do Text's Find with mute…` branch. Without OCR built it falls back to
      the recorded coordinate.

26. **Hold a key across a click**
    Teach a route where you **press and hold Q**, click something, then **release Q**
    (a quick tap won't count — hold it past ½ a second or across the click). Open the
    saved `.marco`.
    - PASS: it contains `do OS's KeyDown with "q"` … `do OS's Click …` … `do OS's KeyUp
      with "q"` (the hold brackets the click). Run it against an app that shows held keys
      (e.g. a game, or a key-state tester) — Q reads as **held** during the click.

27. **A hold never gets stuck (safety)**
    Run the hold route from step 26 but press **Esc** *between* the KeyDown and KeyUp
    (start a long wait in the middle if needed).
    - PASS: Q is **released** even though the route was aborted — check the key-state
      tester shows Q up afterwards. (The driver releases any held key at route end on
      success, error, or Esc.)

## Quick regression sweep (terminal only, ~1 min)

```powershell
$env:MARCO_ROUTES="$env:TEMP\marco-e2e"; Remove-Item -Recurse -Force $env:MARCO_ROUTES -EA 0
"activate notepad`ntype the first argument`ndone" | marco teach --narrate "echo"
marco args "echo"                     # -> 1   (a positional arg placeholder)
marco do "echo with hi there"         # types "hi there"
marco forget all                      # y -> wipes
Remove-Item Env:MARCO_ROUTES
```
- PASS: teaches, reports the arg, runs with the value, and clears.

> **Narration makes named args too:** `marco teach --narrate "say hello with
> person"` then narrate `type person` (or `type the first argument`) → the route
> gets a `{{person}}` arg, runnable as `say hello person:bob`. Declaring no `with`
> clause and narrating `type arg 1` gives a positional `{{1}}` instead.
>
> URLs in route phrases work fine (`go to http://…` stays intact — the `//` guard
> in `ParseInvocation` prevents it from being treated as a `key:value` arg).

## Realistic Learn acceptance (Roadmap 34, goal-centric — needs a human, ~5 min)

The acceptance criterion: **you use the computer normally while Marco learns.** You should
never need to know when START fingerprints, when capture arms, when a session samples, when
to wait for boxes, or when to return anywhere. If any step below makes you feel like you're
operating Marco's protocol instead of your computer, that is a FAIL of the milestone even if
the mechanics pass.

Setup: `director serve` running with the UIA bridge; Windows **Settings** open on its home
page (Settings grounds cleanly; Explorer cannot be pointed at — see Experiment-012).

> Already confirmed on a cold service (2026-08-17), so if any of these fail it is a
> regression rather than an unknown: the phrase is accepted and the play name it derives is
> printed up front (`do MouseSettings's Open`); a cold Settings — never established, no
> question ever answered about it — still reaches "go ahead and show me" and grounds its
> start; and with nobody demonstrating, the pass refuses honestly with *"I didn't see
> anything change"* and saves nothing. What needs your hands starts at the click.

1. **Learn with one ordinary click.**
   `director teach "open mouse settings"` — if Settings is not the foreground window, add
   `--window-title Settings`. Wait for "go ahead and show me", then simply click
   **Bluetooth & devices → Mouse** (or navigate Home → Mouse however you normally would,
   clicks included). Stop when you're there. Do nothing else.
   - PASS: the pass ends a couple of seconds after you stop, on its own. No second
     demonstration is requested for a clean example. `--watch` shows the click attributed
     WITH its target (a `list_item` and, under the Learn licence, its name).
   - PASS: `director reach` lists `"open mouse settings"`.
   - PASS (capture-first): even if recognition failed somewhere, `director observation-session`
     shows every click/keypress you made in the session's input log — nothing discarded.

2. **The rehearsal reaches the right window.**
   Answer the "want me to try?" question (`director answer … yes`) from the terminal — and
   deliberately leave the terminal in front for a few seconds.
   - PASS: nothing is typed into the terminal. Teach reports it is waiting
     (`window_not_in_front` in `--watch`); the moment you click into Settings (standing on
     the start), the attempt fires ONCE, and each step classifies against the right screens
     (no unconditional `unrecognised`).

3. **Reuse from somewhere other than the demonstrated start.**
   Walk Settings to wherever the route's start was (e.g. Bluetooth & devices) by hand, then
   `director reach "open mouse settings"`.
   - PASS: the plan starts from where you ARE. If you're already on Mouse, it says you're
     already there. From a page with no known chain, it says "I know what you want to
     reach, but I don't yet know how to get there from here" — never "go back to Home".

4. **No stale boxes.**
   During and after all of the above: run a bare `director teach` status read mid-session
   and again after it settles.
   - PASS: highlights appear only at the moment their decision is made, are gone when the
     phase moves on or the session ends, and a status read after the moment prints the
     sentence with no box. Nothing lingers waiting for a timer.
