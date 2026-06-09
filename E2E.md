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
    - PASS: the route finds the target by image; the saved route has an `Anchor`
      with an `Image`, and falls back to the recorded point.

21. **Stop key** — start a long route, press **F12**.
    - PASS: it aborts mid-run. (F12 also ends recordings; change with
      `$env:MARCO_STOP_KEY`.)

22. **Diagnostics** — `marco diag`.
    - PASS: prints DPI awareness, virtual-desktop bounds, cursor position, and a
      SetCursorPos round-trip that lands exactly (incl. negative-origin monitors).

---

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
> Known edge: a literal URL in a command (`go to http://…`) is mis-read as a
> `key:value` arg — avoid colons in route phrases for now.
