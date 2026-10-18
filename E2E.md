# Marco — end-to-end test guide

A manual walkthrough of every shipped feature. Each step lists **what to do** and
**what you should see (PASS)**. Work top to bottom; later sections build on earlier
ones. Windows, with the overlay stack running unless noted.

```powershell
.\setup.cmd            # build engine + macros + overlay (add -Voice for the mic)
.\overlay.cmd          # launch the HUD stack (leave it running)
```

Rebuild and restart before you start — the Director wire protocol is at **8**, and a stale
`overlay.exe` or `director.exe` is refused by the version check (see the note in section H).

> Tip: to avoid touching your real plays, run CLI checks with a throwaway store:
> `$env:MARCO_ROUTES="$env:TEMP\marco-e2e"`. Unset it (`Remove-Item Env:MARCO_ROUTES`)
> to go back to the real `.\routes`.

In the overlay, **`` ` ``** (backtick) is the leader; **`` `m ``** opens the command
line. Commands below written as `` `m <thing>`` mean: press backtick, then `m`, then
type `<thing>`, then Enter.

---

## A. Learn & run basics

1. **Learn by demonstration**
   `` `m learn open notepad`` → Notepad-or-anything demo, then press **F12**.
   - PASS: the HUD shows "recording … press F12", then a **save prompt drops in
     below the command line** with a live **`● rec M:SS` timer** while you record.

2. **Answer the prompts (type-then-Enter)**
   At `Save this as "open notepad"?` press **y** then **Enter**; at `Make it
   available only in <app>?` press **y** then **Enter**.
   - PASS: each prompt shows your selection as `› y` on its own line, the prompts
     stack as a transcript, and it saves scoped to the app. **Esc = no.**

3. **Run it**
   `` `m open notepad`` (or `marco do "open notepad"` in a terminal).
   - PASS: the play replays.

4. **Scope happy-path** — learn something again and just press **Enter, Enter** at the
   two prompts.
   - PASS: saved, scoped to the app (app-only is the default; "no" would make it
     global).

5. **List / forget**
   `marco routes` lists them. `marco forget "open notepad"` asks `Forget …?` →
   **y**.
   - PASS: gone from `marco routes`.

6. **Forget all** — `marco forget all` (or `` `m delete all`` in the HUD).
   - PASS: `Forget ALL N plays? This can't be undone.` → **y** wipes them; **n**
     keeps them. (Earlier this wrongly reported no such play.)

> **Step 6a — the old word still answers.** Acquisition is spelled **learn** everywhere
> now; `teach` survives as an undocumented compatibility alias for the muscle memory the
> product shipped with. The HUD is the only place the overlay's alias can be proven — the
> Go suite cannot drive the command line — so prove it here, once.
>
> `` `m teach open notepad`` → demo, then **F12**, and answer the prompts.
>
> - PASS: it behaves exactly like step 1 — same recording line, same save/scope prompts,
>   same saved play. The word is an alias, not a second code path.
> - PASS: `` `m narrate teach open chest`` starts the same narration session as step 14
>   (say or type `done` to finish, or `cancel` to save nothing).
> - PASS: `marco teach "open notepad"` and `director teach` still run from a terminal.
>
> Type `learn` from here on. `teach` is retiring, and it is reserved for the other
> direction — Marco guiding you through something — which is not built yet.

---

## B. Arguments

7. **Named argument (demonstrate)**
   `` `m learn say hello with name`` → in the demo, open a text field and **tap F9
   where the name should go** (don't type a name), then **F12**, save.
   - PASS: the recording hint reads `… tap F9 where each arg goes, in order: name`.

8. **Run with a value**
   `` `m say hello name:chris`` (or `marco do "say hello name:chris"`).
   - PASS: it types **chris** where you tapped F9.
   - Check the engine sees it: `marco args "say hello"` → prints `name`.

9. **Several args / spaces**
   Learn `dm with person, message` (tap F9 twice, in order). Run
   `marco do "dm person:sam message:hi there"`.
   - PASS: `person` = `sam`, `message` = `hi there` (value runs to the next `key:`).

10. **Secret argument — provide once, the remembered**
    Learn `login to facebook with username, password` (tap F9 for each).
    - `marco do "login to facebook username:me password:hunter2"` → types both.
    - `marco do "login to facebook"` (omit them) → reuses the remembered values.
    - PASS: the second run works without re-entering. Open the saved
      `routes/.../login-to-facebook.marco`: it contains
      `do OS's Secret with "login-to-facebook:password"` and **no plaintext
      password** (username may appear as `{{username}}`).

11. **Global `{{name}}` secret still works** — learn a play where you type
    `{{token}}` literally, then `marco secret set token`.
    - PASS: the play stores only the name; the value comes from the credential store.

---

## C. Learn by talking (typed or voice)

12. **Typed narration (no mic needed)**
    ```
    "activate notepad`ntype hello`ndone" | marco learn --narrate "quick note"
    ```
    - PASS: prints each parsed step (`type "hello"`, `done`) and `Learned "quick
      note"`.

13. **Narration vocabulary** — try phrases: `click this`, `anchor this`, `wait for
    this screen`, `wait 2 seconds`, `press enter`, `activate <app>`, `undo`,
    `cancel`, `done`.
    - PASS: each becomes the right step; `undo` drops the last; `cancel` saves
      nothing.

14. **Hands-free in the overlay** — `` `m narrate learn open chest``, then say or
    type each phrase, finish with `done`.
    - PASS: per-step status streams into the HUD; `done` saves. (With `-Voice` set
      up, speak them; otherwise type each `` `m <phrase>``.)
    - **Continuous-listen (voice only):** say `"marco narrate learn open chest"` to
      start, then say `"click this"`, `"type hello"`, etc. — the wake word is only
      needed once. The overlay creates `$TEMP\marco-narrate.lock` while the session
      is live; the voice plugin re-arms automatically after each phrase. The file is
      removed when `done`/`cancel` exits the session.

---

## D. Overlay UX

15. **Auto-popped argument labels**
    Start typing a play that takes args, e.g. `` `m login to facebook`` and pause.
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
      or **n/Esc**, same as learning a play.

18. **Leader / command line basics** — `` `m help``, `` `m exit``.
    - PASS: help menu shows; `exit` closes the overlay.

---

## E. Robustness

19. **Window-relative clicks (the default)**
    Learn a click in a windowed app. **Move the window** (or drag it to another
    monitor). Run the play.
    - PASS: it clicks the right spot relative to the window, no monitor/DPI setup.

20. **Image anchors (opt-in)**
    Learn one again with anchors on: set `$env:MARCO_ANCHORS="1"` before launching,
    learn a click, then resize/move things and run.
    - PASS: the play finds the target and clicks it; the saved play has an `Anchor`
      with an `Image` (and `Color`/`Window`), and falls back to the recorded point when
      it can't resolve. The saved `*-anchor-*.png` is cropped tight to the **button**
      (auto-crop), not a big square of background. (More anchor checks in section F.)

21. **Stop key** — start a long play, press **F12**.
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
    app's light/dark theme), and run the play.
    - PASS: it clicks the button where it **now** is, not the old coordinate — the
      `when ok?` branch fires (`do OS's Click with that`). At `MARCO_LOG=debug` you see
      `edge=` carry the recolour and `image=` survive a brightness/scale change. A
      target it genuinely can't find → low-confidence **Warn** and the recorded-point
      fallback (it never clicks the wrong place confidently).

24. **Window-name context (multi-window app)**
    In an app that opens several windows (Steam: Library vs. Friends), anchor a click in
    **one** window via a **focus** play (so the play activates the app). Bring a
    **different** window of the same app to the front, then run the play.
    - PASS: with the wrong window in front the anchor won't confidently click (a
      `find: foreground window mismatch — penalising confidence` line at debug); with the
      right window it resolves normally. The saved anchor carries `Window "<title>"`.

25. **Text resolver snaps to the button (needs `-OCR`)**
    `setup.cmd -OCR` once. Narrate an anchor over a labelled button: `` `m narrate learn
    mute me``, say `click the text Mute`, `done`. Run it.
    - PASS: it OCRs "Mute" and clicks the **centre of the button**, not the glyphs; the
      play has a `do Text's Find with mute…` branch. Without OCR built it falls back to
      the recorded coordinate.

26. **Hold a key across a click**
    Learn a play where you **press and hold Q**, click something, then **release Q**
    (a quick tap won't count — hold it past ½ a second or across the click). Open the
    saved `.marco`.
    - PASS: it contains `do OS's KeyDown with "q"` … `do OS's Click …` … `do OS's KeyUp
      with "q"` (the hold brackets the click). Run it against an app that shows held keys
      (e.g. a game, or a key-state tester) — Q reads as **held** during the click.

27. **A hold never gets stuck (safety)**
    Run the hold play from step 26 but press **Esc** *between* the KeyDown and KeyUp
    (start a long wait in the middle if needed).
    - PASS: Q is **released** even though the play was aborted — check the key-state
      tester shows Q up afterwards. (The driver releases any held key at play end on
      success, error, or Esc.)

---

## G. Plays — saved, then askable (~3 min)

A **play** is one durable behaviour; **saved** and **askable** are two different states.
This section walks a play across that line and watches every surface agree about it.

You need a **staged** play (saved, not registered). A completed Learn saves *and* registers, so
reach the state on purpose with the developer half of the lifecycle — save without register:

```powershell
$env:MARCO_ROUTES="$env:TEMP\marco-e2e"     # marco and director share this; set it for both
# --save without --register: written down, and nothing can ask for it
director learned --application <app> --name "<what you want to ask for>" --save
```

Read the play's name off `marco plays` and use it below; the steps say `"<name>"`.

28. **A staged play is listed, and says plainly that nothing can ask for it.**
    `marco plays`.
    - PASS: it appears under **`Saved, not askable yet:`** as
      `Learned · Saved — not askable yet`, with `(marco register "<name>")` printed
      beside it — the command that fixes it, named where the problem is shown.
    - PASS: `marco routes` does **not** list it, and neither does `marco routes --json`.
      That listing is what a front end may offer, and a staged play cannot answer.

29. **The control centre shows the same two groups.**
    `marco ui plays` (or `marco ui`, which now lands on Plays with no play named).
    - PASS: the tab reads **Plays**; the staged play is in its own group with a
      **Register** button and the sentence *"Saved. It is a file you can read and edit —
      nothing can ask for it until you register it."*
    - PASS: an askable play has **no** Register button.

30. **Register it, and watch it become ready.**
    `marco register "<name>"` (or the Register button — either one).
    - PASS: `Registered "<name>". You can ask for it now.`
    - PASS: `marco plays` now lists it under **`Known plays:`** as
      `Learned · Ready · from anywhere (brings <app> forward)`, and the
      staged group is gone.
    - PASS: `marco routes` lists it too, and `marco plays --json` shows
      `"life": "ready"`, `"registered": true`, `"activates": "<app>"`.
    - PASS: the Plays tab shows **Ready** and *"You can ask for this play."* — the same
      words, because both surfaces read them from the same place.

31. **It answers from another application.**
    Leave the terminal (or anything that is not the play's app) in front.
    - PASS: `marco dispatch "<name>"` now prints `run: <name>` — the classifier only
      ever offers registered plays. Registering, not the foreground, is what changed.
    - PASS: `marco routes --json` shows `"scope": "focus"` for it. A learned play is
      asked for from anywhere, and Marco brings its application forward itself.
    - Only now, and only if you are ready for **real input**: `marco do "<name>"` from
      that same foreground. Marco asks *"Marco learned … Run it now?"*
      first — answer **n** to confirm the question arrives, or **y** to watch it bring
      the app forward and perform. PASS either way: the question is the door, and a
      staged play never got this far.

32. **The play's past survives a move.**
    In the Plays tab, change the registered play's scope with the dropdown (focus →
    only-here, say).
    - PASS: it still reads **Learned** afterwards, not Authored — the provenance moved
      with the file. If you had edited the `.marco` by hand first, it still reads
      `Ready · edited`: moving a play never re-verifies it.
    - PASS: nothing is left behind at the old scope (`marco plays` lists it once).

## H. One intake — the same request, however you ask (~6 min, needs a person)

The claim this section tests: **typing, speaking and `marco do` are entrances, not different
Marcos.** The same words reach the same play from all three, a request Marco has never heard of
reaches the Director from all three, a bound hotkey performs the play it was bound to, and "stop"
reaches whatever is running.

> **Rebuild everything first.** The Director wire protocol went **6 → 7** when intake was
> unified, and again **7 → 8** when acquisition was renamed to Learn and the two acquisition
> request types were merged into one. A stale `director.exe` beside a fresh `marco.exe` is
> refused by the version check rather than silently dropping the new field, so run
> `.\setup.cmd` and **restart `.\overlay.cmd`** — `overlay.exe` only picks up a new build on a
> restart. If a step below answers *protocol version* anything, that is what happened. This
> applies to the whole guide, not only this section.

> **Real input warning, same as elsewhere in this guide.** Steps that end in `marco do` on a
> registered play type and click for real. Everything up to the routing decision is safe to run
> blind; the performing step is called out each time, and you can stop after the trace line.

**Turn the routing trace on.** This is the trick that makes the whole section visible:

```powershell
$env:MARCO_ROUTES="$env:TEMP\marco-e2e"    # a throwaway store; not your real plays
$env:MARCO_TRACE_INTAKE="1"                # print one [intake] line per command
```

`overlay.cmd` must be restarted with that variable set for the HUD's children to inherit it.
Unset it (`Remove-Item Env:MARCO_TRACE_INTAKE`) when you're done — it is off by default because it
would otherwise land in the HUD log on every command.

Use a play you already have from section A or G; the steps below say `"<name>"`.

33. **The same known play, by typing.**
    In the overlay: `` `m <name> ``.
    - PASS: the HUD log shows `[intake] source=typed decision=play play=<slug> …
      why="an exact match for a play Marco has"`, then `[route] <name>`, then
      `[result] performed` (or `refused` if you answer no at the door — both are fine here,
      the routing is what this step proves).

34. **The same known play, by speaking.** *(Needs a person at a microphone — a spoken phrase
    cannot be scripted. There is no way to inject a Final transcript from a test, which is why
    this section is in E2E and not in the Go suite.)*
    Turn voice on (`` `m voice on ``) and **say the play's name**.
    - PASS: the HUD shows what it heard, and the `[intake]` line is **identical to step 33 except
      for `source=spoken`**. Same decision, same play, same `why`.
    - FAIL (and this is the bug the phase fixed): the line says `decision=director`, or the HUD
      offers to record a demonstration of a play you already have.

35. **The same known play, from a shell.**
    `marco do "<name>"` in a terminal.
    - PASS: `[intake] source=cli decision=play play=<slug> …`, the same slug again.
    - PASS: `echo $LASTEXITCODE` is `0` after `[result] performed`.

36. **A novel request, by typing.** Pick something you have definitely never taught —
    `turn the bluetooth off` will do.
    `` `m turn the bluetooth off ``.
    - PASS: `[intake] source=typed decision=director phrase="turn the bluetooth off"
      why="no play answers to this, so Director reads it against what is on screen"`.
    - PASS: the words reach the Director **unedited** — the phrase in the trace is exactly what
      you typed.
    - PASS: whatever comes back is one of the six (`performed` / `clarify` / `refused` /
      `unavailable` / `failed`) — and the offer to learn appears **only** if it was `unavailable`
      with no `[route]` line, i.e. nothing took the request at all.

37. **The same novel request, by speaking.** Say the same sentence.
    - PASS: identical trace except `source=spoken`. This is the pair that used to differ.

38. **The same novel request, from a shell.** `marco do "turn the bluetooth off"`.
    - PASS: `source=cli`, same decision, same phrase.
    - PASS: with no Director running, `[result] unavailable`, exit code **3**, and stderr names
      the phrase — a recognised request is never silently dropped.

39. **A near miss is a miss.** With `"<name>"` registered, ask for it with an extra word:
    `marco do "<name> please"`.
    - PASS: `decision=director` — not the play. Exact means exact; a guess about which play you
      *probably* meant belongs to the tier that can look at the screen and ask you.
    - PASS: but case, punctuation and spacing still land on the play:
      `marco do "  <NAME>  "` gives `decision=play` with the same slug.

40. **A binding carries an identity, not words.**
    `marco bind s "<name>"` (or `` `m bind s <name> ``), then press `` ` `` then `s`.
    - PASS: `[intake] source=hotkey decision=play play=<slug> explicit=yes
      why="the surface named the play itself"`.
    - PASS: `explicit=yes` is the point — the binding resolved to that play **once**, and the
      words were never read back. A hotkey cannot drift onto a different play as the store fills up.
    - PASS: `` `m unbind s `` afterwards.

41. **A clicked Run does the same.** `marco ui plays`, then Run on a registered row.
    - **Real input:** this performs. Only press it when you're ready.
    - PASS: `source=control-centre`, `explicit=yes`, and the slug is the row's own handle — a
      display name that doesn't round-trip through its slug still runs the right file.

42. **Stop means stop, from every entrance.**
    Start something long (a learned play with several steps, or a Director request), then:
    - type `` `m stop `` → PASS: `[intake] source=typed decision=control
      why="a control phrase — it acts on what is running"`, and the run ends.
    - say **"stop"** → PASS: the same line with `source=spoken`, and it ends *immediately* —
      the overlay recognises the word locally to kill its child at once, and still sends it
      through the intake so the Director (which is what actually drives a learned play) is told.
    - press **Esc** while it runs → PASS: same result, `[result] cancelled`.
    - PASS in all three: `stop` is **never** offered as something to learn. If Marco asks whether
      you'd like to record a demonstration called "stop", that is the old behaviour and a FAIL.
    - PASS: `director status` names the running play *while* it runs (it is a registry command
      now), and says nothing is running afterwards.

43. **An answer belongs to its question.**
    Ask the Director something ambiguous enough that it asks back ("click save" on a screen with
    two Saves). While the question is pending, and **while a play of that name exists**, say or
    type the answer.
    - PASS: `[intake] … decision=director why="Director is waiting for an answer — these words
      are it"` — even if the words happen to name one of your plays. An answer is not an
      invocation.

44. **Clean up.**
    `Remove-Item Env:MARCO_TRACE_INTAKE; Remove-Item Env:MARCO_ROUTES`, and restart
    `.\overlay.cmd` so the HUD stops printing the trace.

## Quick regression sweep (terminal only, ~1 min)

```powershell
$env:MARCO_ROUTES="$env:TEMP\marco-e2e"; Remove-Item -Recurse -Force $env:MARCO_ROUTES -EA 0
"activate notepad`ntype the first argument`ndone" | marco learn --narrate "echo"
marco args "echo"                     # -> 1   (a positional arg placeholder)
marco do "echo with hi there"         # types "hi there"
marco forget all                      # y -> wipes
Remove-Item Env:MARCO_ROUTES
```
- PASS: learns it, reports the arg, runs with the value, and clears.

> **Narration makes named args too:** `marco learn --narrate "say hello with
> person"` then narrate `type person` (or `type the first argument`) → the play
> gets a `{{person}}` arg, runnable as `say hello person:bob`. Declaring no `with`
> clause and narrating `type arg 1` gives a positional `{{1}}` instead.
>
> URLs in play phrases work fine (`go to http://…` stays intact — the `//` guard
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
   `director learn "open mouse settings"` — if Settings is not the foreground window, add
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
   - PASS: nothing is typed into the terminal. The session reports it is waiting
     (`window_not_in_front` in `--watch`); the moment you click into Settings (standing on
     the start), the attempt fires ONCE, and each step classifies against the right screens
     (no unconditional `unrecognised`).

3. **Reuse from somewhere other than the demonstrated start.**
   Walk Settings to wherever the play's start was (e.g. Bluetooth & devices) by hand, then
   `director reach "open mouse settings"`.
   - PASS: the plan starts from where you ARE. If you're already on Mouse, it says you're
     already there. From a page with no known chain, it says "I know what you want to
     reach, but I don't yet know how to get there from here" — never "go back to Home".

4. **No stale boxes.**
   During and after all of the above: run a bare `director learn` status read mid-session
   and again after it settles.
   - PASS: highlights appear only at the moment their decision is made, are gone when the
     phase moves on or the session ends, and a status read after the moment prints the
     sentence with no box. Nothing lingers waiting for a timer.

## Teaching Marco a game-menu routine (needs your hands)

Marco cannot demonstrate to itself. `navsource` discards injected presses at the callback as not
coming from a hand, so a session driven by Marco's own clicks records zero attributed presses.
That separation is deliberate — it is what makes a demonstration mean something — so this one
walkthrough has to be performed by a person. It takes about two minutes.

Use an **offline or single-player** menu. Nothing here injects input, so the game is never driven;
Marco only watches. Pick a menu with a few obvious buttons and no timer.

```sh
go build -o director.exe ./cmd/director

# no daemon needed: this command, and `reacquire` and `sequences` below, all read
# and write the store directly.
./director.exe demonstrate-visual "enter free play" --application <exe-name> --seconds 120
```

The selector is the EXECUTABLE name, which survives a patch -- `arkascended`, not
`Ark: Survival Ascended (94.34.0)`. Use `--window-title "<substring>"` instead when one
executable owns several windows, and exactly one of the two: `Selector.Validate` refuses both
together, because a request naming two has nothing to disambiguate it.

**The game must be in FRONT.** Window capture is not occlusion-safe: `--application` avoids
stealing focus and returns the covering window's pixels under the target's name, so a
demonstration recorded against a backgrounded game is a demonstration of your terminal.| grep -i <game>

# 2. start watching, then switch to the game within the 5s delay
./director.exe demonstrate-visual "enter free play" --window-title "<title>" --seconds 120
```

Then, with your own mouse:

1. click one menu entry, and wait a second for the next screen to settle;
2. click a second entry, wait again;
3. click a third;
4. come back to the terminal and press **ENTER**.

Marco prints what it observed before it saves anything:

```
WHAT THE DEMONSTRATION TAUGHT
  ended because you ended it.
  4 screen(s) seen, 4 visited in order, 3 demonstrated control(s)
  presses: 3 of 3 classified

  Steps
    1 visual_1 -> visual_2 via target_1   [geometric]
    2 visual_2 -> visual_3 via target_2   [geometric]
    3 visual_3 -> visual_4 via target_3   [geometric]

  Controls, and what each press was taken to be
    target_1     1 press(es), read as "PLAY", led somewhere
    target_2     1 press(es), read as "FREE PLAY", led somewhere
    target_3     1 press(es), nothing was read on it, led somewhere
```

**What to check, in this order:**

- `presses: N of N classified`. A press refused as `unplaceable_pointer` was outside the watched
  window. A press counted as `pointer_injected` did not come from a hand.
- **Does each `read as` match the button you actually clicked?** That is the one line only you can
  verify, and it is the strongest identity evidence the artifact carries.
- `nothing was read on it` is honest, not broken — an unlabelled icon reads as nothing.
- `NOTHING CHANGED after it` means the press was seen and no screen change was attributed to it.
- `0 demonstrated control(s)` with presses classified means no press landed on a *settled* screen:
  click more slowly, leaving a second between clicks.

Then inspect and measure what was kept:

```sh
./director.exe sequences                                  # what has been saved
./director.exe sequences "enter free play"                 # its steps, and how each is aimed
./director.exe reacquire "enter free play" --application <exe-name> --seconds 30
```

`reacquire` presses nothing. It reports, per control, how often it could be found again, and how
often two controls claimed one rectangle — which is the only wrongness it can detect without you.

## Clicking the same control twice

One press is one press. The recorder says so on every step it records from one —
*clicked once, so nothing independent confirms that this control can be found again* — and a
second press on a separate visit is the only independent evidence about where the first one's
control was.

**With the MOUSE, every time.** A control is identified from a pointer press landing inside a
region the proposer found. Moving the highlight with the arrow keys and pressing Enter is
navigation: it is watched, it is counted, and it names nothing — the first live attempt at this
was twenty-nine events of which one was a pointer press, and that one was the click onto the
terminal. [[Experiment-081-the-first-real-negative]].

So demonstrate the SAME button twice, going away and coming back:

1. click the button (say Settings);
2. come back out of that menu, by whatever the game's own way is -- Escape, or a Back button;
3. click the SAME button again;
4. ENTER in the terminal.

What changes in the report when it worked:

```
target_1   on visual_1   2 press(es), read as "settings", led somewhere
```

`2 press(es)` is the whole point, and `seen: 2` appears on the saved step. A control pressed
twice has two readings of its rectangle and two chances to be read, and the step's `unresolved`
note about a single click disappears.

And **two** controls instead of one is the interesting failure, because there are two ways to get
there and they need different answers. The screen on the line is what separates them:

```
target_1   on visual_1   1 press(es), read as "settings", led somewhere
target_2   on visual_1   1 press(es), read as "settings", led somewhere   <- the presses did not merge
```

Both on ONE screen: the menu was recognised on the way back and the two presses were not taken
to be the same button. That is `sameControl` — mutual centre containment — and the two
rectangles the proposer offered disagreed too much. Evidence about the proposer.

```
target_1   on visual_1   1 press(es), read as "settings", led somewhere
target_2   on visual_3   1 press(es), read as "settings", led somewhere   <- a new screen
```

On TWO screens: the returned menu was not recognised as the menu, so the second press was never
offered the first one's identity to merge into. A control identity is scoped to a screen on
purpose, so this is correct behaviour on a wrong premise — evidence about screen identity, and
`N screen(s) seen, M visited in order` says it too (seen < visited means something was
revisited).

Then measure it, with the game IN FRONT for the whole run:

```sh
./director.exe reacquire "<name>" --application <exe-name> --seconds 30
```

The `words` column is what to read. `corroborated` means the rectangle it matched reads what the
control was saved as; `contradicted` means it reads something else, over repeated readings; and
`unread` means there was not enough fresh evidence, which is NOT agreement.

The game must be in front for the whole run or every reading is refused as unusable and the
column is honestly `unread`. The `readings:` line says which happened:

```
readings: 35 offered, 31 admitted, 4 stale (epoch 4, window 0)        <- good
readings: 37 offered, 0 admitted, 37 on a look that could host none   <- the window was behind
```

**Expect it to be unreliable, and expect the report to say so.**
[[Experiment-078-the-conjunction-is-two-terms-wearing-three-hats]] measured the grounding rules
on a recorded game menu: the best of them is wrong one time in thirty-one and ready on 1 look in
449. Nothing authorises a pixel replay yet, and `director sequences "<name>"` says in as many
words that a pixel routine cannot be run.

The shape of a bad answer, from verifying these commands against a Rocket League artifact in an
Ark window -- two games that share no menu at all:

```
12 of 13 control(s) were found on at least one look.
colliding matches: 2100
  AT LEAST 2100 WRONG. Two controls matched one rectangle on the same look,
  and they cannot both be the thing in that place.
```

Twelve of thirteen controls "found" in a game they were never demonstrated in. That is
[[Experiment-077-the-screen-was-not-supporting-context]] in one screen of output, and it is why
the collision count is printed before anything else.

### The negative test that needs a real menu

A saved word edited by hand proves the mechanism and not the behaviour. The test that matters is
a DIFFERENT real menu of the same game, carrying rectangles of a similar shape:

```sh
./director.exe reacquire "ark menu 2" --application arkascended --seconds 30
```

Open the other menu BEFORE the delay runs out and stay on it for the whole run. The verdict is
accumulated across every look of the run, so a run that starts on the demonstrated menu and
navigates away folds both menus' readings into one answer and no column can be read.

Three honest outcomes, and all three are informative:

| what the report says | what it means |
|---|---|
| `0% matched`, `unread` | geometry found nothing of that shape here. The abstention, and the best answer |
| `N% matched`, `contradicted` | a rectangle of that shape is here and it reads something else. The refusal working |
| `N% matched`, `unread` | a rectangle of that shape is here and nothing legible is in it. An abstention, NOT agreement |

And one that would be a finding against Marco: `N% matched`, `corroborated` on a menu that has
no Settings button — the saved word read back off a different control. Check the `readings:` line
first, because a stale or unusable reading cannot produce it.
