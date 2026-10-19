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

### Pick the button the game lets you click

Not every entry in a game menu is a mouse target. Ark's pause menu navigates by key — the
owner's account is *there's not a left click for settings, I can click resume to close the
menu* — and a demonstration of an entry the mouse cannot press is a demonstration of nothing,
however carefully it is performed.

**The best button for this test is one that CLOSES the menu**, because returning to the menu is
then the second visit, and the click itself causes the change rather than a key doing it:

| | |
|---|---|
| click | the control being learned, pressed on visit one |
| the key that reopens the menu | how you get back, and it names nothing — which is fine |
| click again | the same control, on visit two |

Resume, Back, Close, Continue: any of them. The word that gets saved is whatever the control
reads, and `"resume"` is as good an identity as `"settings"` — better, for the different-menu
negative, since no other menu carries it.

So demonstrate the SAME button twice, going away and coming back:

1. click the button;
2. come back to that menu, by whatever the game's own way is -- Escape, or a Back button;
3. click the SAME button again;
4. ENTER in the terminal.

What changes in the report when it worked:

```
clicks:    2 reached the observer, 2 given to the correlator
target_1   on visual_1   16%x  5%  2 press(es), read as "resume", led somewhere
```

**The size is the first thing to read, before the words.** A game menu's proposer does not
reliably box one entry, and the shape of a bad answer looks exactly like a good one everywhere
else in the report:

```
target_1   on visual_1   49%x 64%  1 press(es), read as "ark clug discord exit friends
                                   main menu mode on photo settings to toggle", led somewhere
target_2   on visual_2   58%x100%  1 press(es), nothing was read on it, led somewhere
                  NOT A BUTTON: this is the rectangle target_1 was distinguished from,
                  recorded there as a container it is not
```

That is [[Experiment-082-a-control-that-is-half-the-window]], measured: a control a third of the
screen across carrying the menu's whole vocabulary, and a second one that is the first one's
container to the last decimal digit. `[geometric]`, "led somewhere" and the right walk all held.
Thirteen words on a control is not a rich identity — it is a reading of every button beside the
one that was pressed, and at that scale the `reacquire` corroboration becomes a rubber stamp,
because sharing ONE word is all it asks.

So: a two-digit size and one or two words is a button. A size near the window's and a sentence
of words is a panel, and nothing further in the report will say so.

The `clicks:` line first, every time. It is the only line in the report that is about pointer
presses: `input: N of M event(s) classified` counts navigation intents of EVERY kind, including
keys, and read as a count of clicks it says a session of four Escapes made four presses.

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

## Recording a game menu for investigation (needs your hands, ~1 min)

`demonstrate-visual` keeps **no frames and no screenshots**, and keeps every press with its
position in a separate raw record (`director what-i-did`) that nothing executes. That is the right
design for a saved routine and it still makes the ARTIFACT useless for asking *where was the
button?* — the readings are rectangles the proposer offered, not the frame they came from. The L0
recorder keeps the frames too, on one clock, and executes nothing:

```sh
./director.exe capture-game-session --application arkascended --session ark-menu-1 --seconds 45
```

Open the menu and click two or three entries **with the mouse**. Nothing is injected; every input
in the recording is observed from hardware. It stops on its own.

Then ask the recording the two targeting questions, neither of which presses anything:

```sh
go run ./cmd/perceptionreplay -frames -button   .tmp/game-capture-review/ark-menu-1
go run ./cmd/perceptionreplay -frames -temporal .tmp/game-capture-review/ark-menu-1
go run ./cmd/perceptionreplay -frames -outcome  .tmp/game-capture-review/ark-menu-1
```

`-button` asks whether the clicked button can be isolated inside the panel the proposer offered,
from the click position, the proposals and the OCR word boxes. `-temporal` asks the same click of
the looks before and after it. `-outcome` asks whether the press DID anything
that can be verified — the words elsewhere on the window that appeared and stayed, which is the
only evidence a click has when the screen itself does not change. Read the `EVERY SET OF TERMS`
table: a rule is only worth having if it verifies real presses AND reports zero outcomes in the
windows with no input in them. On Rocket League the first found the right button on both presses
with a known answer and the second found one rescue in seventeen —
[[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]] and
[[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]].

Two things to read first, because they decide whether the run says anything at all:

| line | what it means |
|---|---|
| `N press(es) with a position` | zero means a keyboard-only capture. Neither question can be asked |
| `N look(s) carrying word boxes` | zero means `words.jsonl` predates boxes. Delete it and run `-frames -semantic` again |

## Teaching with a mistake in it, and correcting it (needs your hands, ~2 min)

The point of this one is that you are **meant** to get it wrong. Click something that isn't a
control, press a key that does nothing, click one entry you didn't mean to — then fix what Marco
learned without doing the whole thing again.

```sh
./director.exe demonstrate-visual "ark graphics" --application arkascended --seconds 45
```

Click two or three entries with the mouse, include one deliberate mistake, press ENTER when done.
Two files are written, and they answer two different questions:

```sh
./director.exe sequences  "ark graphics"      what Marco will DO        (sequences/*.json)
./director.exe what-i-did "ark graphics"      what you actually DID     (evidence/*.json)
```

`what-i-did` prints every press with its position and the screen it was made on — including the
ones no proposal contained, which say `nothing was proposed there` rather than vanishing — and
every key as a press with a duration, so a tap, a key you leaned on and two keys held together
are three different lines. It reads and writes nothing.

Then take the mistake out of the procedure:

```sh
./director.exe correct "ark graphics" --drop last --because "that click was accidental"
./director.exe correct "ark graphics" --drop 2 --dry-run
```

The step comes out of what Marco will do and stays in the routine's own `corrections` with your
words. **Your demonstration is not touched** — run `what-i-did` again afterwards and the press
is still there with its position. That is the whole arrangement: see
[[ADR-235-three-records-and-only-one-of-them-may-act]].

Three things worth checking by hand:

| check | what you should see |
|---|---|
| `--drop 9` on a three-step routine | refused, naming how many there are — never rounded down |
| a key that changed nothing | counted in `what-i-did`, and no step to correct. That is correct |
| `what-i-did` on a routine taught before today | *no raw demonstration of it is* — the record is new, and old routines have none |

If something else was in front for part of it — an overlay, a notification, Marco's own window —
`what-i-did` says which looks, because a window capture is not occlusion-safe and an input that
arrived on one of those looks may never have reached the game. It is named, not discarded.

## Pointing the wrong routine at the wrong game (no hands, ~10s)

A saved routine belongs to the application it was demonstrated against, and `--application` is
now optional — Marco reads the foreground itself:

```sh
./director.exe perform-workflow --workflow ark-graphics.json            # no --application
./director.exe perform-workflow --application solasta --workflow ark-graphics.json
```

The first discovers whatever is in front. The second is the one worth running deliberately: an
Ark routine aimed at another game is refused **before anything is built**, naming both
applications, and nothing is pressed. See
[[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]].

Three things to check by hand:

| check | what you should see |
|---|---|
| a routine saved before today | *UNSCOPED* — readable, and valid for no application until bound |
| nothing in the foreground (lock the screen) | a refusal naming which of the four cases it is, not a guess |
| alt-tab away mid-run | *the application changed after the plan was made* — the press is refused |

The last one is the important one: the check at startup is not permission for a press that
happens seconds later.

## Deriving a one-step routine from captures, and what it refuses (no hands, ~3 min)

The pixel execution path needs an artifact, and an artifact normally needs a person's hand on
the mouse. `perceptionreplay -fixture` builds one from two captures instead — every rectangle,
label and outcome word measured from the recorded pixels, nothing written down by hand:

```sh
# the two looks come from ONE recording, so the recording itself says what caused the change
go run ./cmd/perceptionreplay -frames \
    -fixture GRAPHICS -fixture-look 245 -fixture-after-look 262 \
    -fixture-after .tmp/game-capture-review/ark-menu-1 \
    -fixture-name "ark graphics tab" -fixture-save "$TMP/ark-fixture.json" \
    .tmp/game-capture-review/ark-menu-1
```

`-frames` is a BOOL and the directory is POSITIONAL. What it should print:

```
control       7.2%x2.8% at 0.410,0.241, holding exactly "GRAPHICS"
containers    1 proposal(s) enclose it, and the outcome may not be read there
caused by     an observed mouse press at 0.414,0.261 24892ms into the recording, inside
              this control's own rectangle and between the two looks measured; that press
              is evidence of what caused the difference and is NOT a replay coordinate
outcome       36 word(s) on every reading of 5 look(s): adjust advanced bloom blur …
discarded     0 transient, 1 reused by the control, 17 vanished
held still    0 of 1 window(s) before the action changed by themselves
scope: true … and it is refused for any other application
```

Six things have to be true before it will write anything, and each refuses on its own:

| check | what it refuses |
|---|---|
| a press was SEEN inside the control between the two looks | a difference no action of this control's caused |
| the after state is after the before state | a window that runs backwards |
| both looks are the same application | a workflow derived from two games |
| the label sits alone in some proposal | the tab bar, mistaken for the tab |
| five words appeared and STOOD on every reading | a half-drawn panel, or OCR flicker |
| the before window held still | a clock, a ping counter, a rotating news panel |

The first one is the one worth running deliberately, because it is the one four perception terms
could not catch. Point it at a keyboard-only recording:

```sh
go run ./cmd/perceptionreplay -frames \
    -fixture SETTINGS -fixture-look 100 -fixture-after-look 542 \
    -fixture-after .tmp/game-capture-review/tm-menu-steps-sanitized \
    .tmp/game-capture-review/tm-menu-steps-sanitized
```

```
tm-menu-steps-sanitized saw no mouse press inside this control between look 100 (9068ms)
and look 542 (38533ms) — 0 press(es) and 31 input(s) in the whole recording. Whatever
differs between those two looks was not this control's doing, and an outcome is only an
outcome OF something
```

Without that check it produced twelve outcome words for a press nobody made. See
[[ADR-237-an-outcome-is-only-an-outcome-of-something]].

**Two separate recordings** share no input timeline, so the question cannot be asked at all —
which is deliberately the controlled test menu's shape, since it serves its before and after
states from a URL and nobody has to click to capture either one. There `-fixture-asserted` is
REQUIRED, and what it says is written onto the step:

```sh
go run ./cmd/perceptionreplay -frames -fixture GRAPHICS \
    -fixture-after "$TMP/menu-graphics" \
    -fixture-asserted "the test menu serves ?panel=graphics, which is where its GRAPHICS tab points" \
    -fixture-save "$TMP/menu-fixture.json" "$TMP/menu-display"
```

Leave `-fixture-asserted` off and it refuses rather than assuming the link. A derivation whose
causal claim rests on somebody's word has to say whose, in the artifact, where whoever reads the
verdict will see it.

**What a derived routine is, and is not.** The step says `ASSEMBLED` — not acquired by the live
correlator while a person demonstrated it. It validates the execution path: ground, authorise,
press, verify. It claims nothing about what a live demonstration would produce, and a live claim
still needs a live demonstration.

## The five guarded pixel cases, on a surface that keeps its own score (~10 min)

The point of this one is that **Marco's report is not the evidence**. Marco is the thing being
tested, so a run that says `pressed` and `verified` is a run agreeing with itself. The test menu
therefore counts the presses it actually receives and publishes them, with the standing panel,
in its own window title:

```
FXM <panel-index> <presses>        0=DISPLAY 1=GRAPHICS 2=AUDIO 3=CONTROLS
```

Read it from outside Marco, before and after every run:

```sh
powershell -c "Get-Process msedge | ? { \$_.MainWindowTitle -like 'FXM*' } | % { \$_.MainWindowTitle }"
```

Serve the menu in an app window with its own profile — a fresh profile each time, or the
previous window's press counter comes back through session restore and the witness lies:

```sh
msedge --user-data-dir=<fresh dir> --no-first-run --window-size=1280,860 \
       --window-position=80,60 --app="file:///<path>/menu.html?panel=display"
```

Then, with that window in front and selected exactly (`--pid`, because one executable can own
two windows and the person's own browser usually does):

| case | command | what you should see |
|---|---|---|
| correct menu | `--from=screen_a --delay=5s` | *outcome verified — 18 of the 18 word(s) … stayed for 5 reading(s)*, exit 0, witness `0 0` → `1 1` |
| wrong application | foreground Notepad, `--foreground` | refused naming both applications, *Nothing was pressed.*, Notepad untouched |
| reset and repeat | kill the window, relaunch, run again | verified again — a repeated action is where the bodies are |
| foreground switch | `--dwell=4s`, then take the foreground ~1s into the dwell | *the pointer was moved to the control and the world changed before the press* — and the witness still says **0 presses** |
| verification timeout | `--verify-for=200ms` | *unconfirmed … whether it worked is unknown — which is not the same as its having failed* |

The last two are the ones worth running deliberately. The foreground case proves a check that
passed at invocation is not permission for a press four seconds later, and the witness is what
turns "Marco refused" into "nothing was clicked". The timeout case is interesting *because* the
press works: the witness goes `0 0` → `1 1` while Marco says unconfirmed, which is the honest
answer when it had 293ms and needed five readings.

**What `--from` buys and what it hides.** Establishing the first screen is a person's claim, not
a pixel match ([[ADR-186-a-person-can-say-where-they-are]]). Run without `--from` and the right
menu and the wrong one refuse identically — *nobody could say which saved screen visual_1 is* —
so a refusal there says nothing about which menu you were on. Run the control before trusting a
refusal.

See [[Experiment-090-the-first-guarded-pixel-action]] for the seven defects this batch found,
five of them in the verifier, and every one of them a false verdict about an action that worked.

## Running what you demonstrated, by name (needs your hands for the first half, ~4 min)

The two halves of the product, joined. Demonstrate a menu action, then run it by the name you
gave it — no file paths, no JSON:

```sh
# 1. you demonstrate it. Switch to the game within the delay and click ONE menu control.
./director.exe demonstrate-visual "graphics tab" --application <app> --seconds 20

# 2. it is saved, scoped to the window that was WATCHED, with the raw demonstration beside it
./director.exe sequences                      # what you have shown it
./director.exe sequences "graphics tab"       # its steps, and what it can do
./director.exe what-i-did "graphics tab"      # what you actually did, coordinates and all

# 3. reset the menu by hand, then run it
./director.exe perform-workflow --name "graphics tab" --live --reading --from screen_a
```

`--name` and `--workflow` are exclusive: one names a demonstration you saved, the other an
artifact file, and given both this refuses rather than choosing. A name that was never saved
lists nothing and tells you the two commands that would help.

**What `sequences` now tells you, and what it used to.** It used to say of every pixel routine
*None of these can be run: they were learned from PIXELS and carry no verbs* and send you to
`reacquire`, which measures and presses nothing. That was written before the pixel execution
path existed. Now each routine is offered what it can actually do:

| what it carries | what you are told |
|---|---|
| verbs | `director execute "run <name>"` |
| visual evidence, scoped | `director perform-workflow --name <name> --live --reading` |
| visual evidence, unscoped | nothing to run — demonstrate it again |
| no outcome expectation | runnable, and *its action can be performed and cannot be verified* |

That last row is worth reading before you run anything: a routine saved before outcomes were
recorded will press the control and then have nothing to check the result against, so it reports
UNCONFIRMED rather than success. Demonstrating it again records the expectation.

See [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]].

## Running it from the widget, by typing its name (needs your hands, ~3 min)

The same thing again with the terminal taken away, which is the version a person actually gets.
Nothing new is being tested about the run itself — this is about whether the product can be
*used*: the only path to a routine you had demonstrated used to be the command above.

Build both binaries from the same tree, or the front end has nothing to spawn:

```sh
go build -o marco.exe ./cmd/marco
go build -o director.exe ./cmd/director     # beside marco.exe, see CLAUDE.md
go -C plugins/overlay build -o overlay.exe .
.\overlay.cmd
```

Then, with the game running and a routine already demonstrated and scoped to it:

1. press the leader key, type the routine's **name** — nothing else, no verb, no flags — and
   press Enter;
2. the status line says **Starting in 5s — bring the application in front**. Do that;
3. it asks **where you are**: *Marco has a question — press F7 if this is the screen you are on.*
   The transcript names the saved screen it thinks you are looking at and how closely the
   arrangement agrees. Press **F7** if that is right. This is the question that used to be
   unanswerable from the widget — a hotkey run had no way to be told which screen it was on and
   refused every first observation, so nothing below this line could ever happen;
4. it asks about the **step**: *press F7 to allow this action*. Press F7, and it asks once more
   about the **press** itself — agreeing that a step belongs and permitting it to happen now are
   two questions on purpose;
5. look at the game before that last F7: the pointer is resting on the control. Press **F7** on
   the keyboard — not through anything Marco can reach, which is the point;
6. the control is pressed, the outcome is checked against what your demonstration saw, and the
   HUD's log tail carries the whole transcript including the `run …` reference;
7. the history row says **performed** (verified), **unverified** (nothing could decide), or
   **refused**.

What to check, beyond "it worked":

- **F8** refuses one action and the run ends having sent nothing.
- **F9**, or typing `stop`, or the leader key — all three end the run, and the record says
  `cancelled` rather than `failed`. Typing `stop` is the one that used not to reach it at all.
- Pressing **F8** at the screen question stops the run instead of guessing: the record says the
  screen was not established and nothing was pressed.
- A name you have NOT demonstrated behaves exactly as it did before: Director reads it against
  the screen. A name that is also a play runs the **play**.
- `./director.exe sequences` after the run, and the `run …` reference from the log tail, retrieve
  the attempt you just made rather than the last one.

If the status line never mentions F7, the run is asking on a terminal nobody has: that is
`--hotkeys` missing from the spawn, and `TestADemonstrationIsPerformedWithTheFlagsItNeeds` is the
test that should have caught it.

See [[ADR-241-a-name-you-demonstrated-is-a-name-you-can-use]].

## Demonstrating in a real game, and what goes wrong (needs your hands, ~5 min)

Four live demonstrations against Ark produced three artifacts and zero presses. What that
walkthrough taught is mostly about the three ways it fails before perception is even reached.

```sh
./director.exe demonstrate-visual "ark graphics tab" --application arkascended \
    --delay 15s --seconds 28
```

**Bring the game to the front yourself, and then look at it.** The session only uses looks where
the target leads the desktop. Two demonstrations were lost to this: the first because the game
lost focus while the recorder ran, the second because the instructions were on another monitor
and six presses landed in the browser. `what-i-did` says so afterwards:

```
Something else was in front for 14 of the 16 moment(s) that carried input
  look(s) 5, 16, 17, 22, 23, 24, 25, 26, and 6 more
```

**Dwell on both sides of the click.** The outcome expectation is measured from readings taken
before the press and after it. OCR answers two or three times a second on a 1920x1080 game, and
the window needs about five readings each side — so wait a few seconds, click once, then leave
the result standing for ten. A demonstration that clicks immediately and moves on records a
step that can be performed and never verified, and `director sequences` will tell you so.

**Check what it kept before trusting it:**

```sh
./director.exe sequences "ark graphics tab"      # the steps, and what it can do
./director.exe what-i-did "ark graphics tab"     # your presses, with coordinates
```

A good one reads `1 press(es)` on a control a few percent of the window across — Ark's GRAPHICS
tab measures 8%x3%. A control at 100%x82% is the whole screen and is reported as `NOT A BUTTON`.

**And the one that is not your fault.** On Ark the fold does not hold still:

```
the screens this run saw   4 screen(s) over 43 look(s)
  visual_1   1 look(s)   nobody read it
  visual_2   2 look(s)   nobody read it
  visual_3  20 look(s)   read, and no word was there twice
  visual_4   3 look(s)   nobody read it
```

That is one static settings menu. A screen established while demonstrating cannot be recognised
again at replay, and `--from` is spent once, on one screen, by design. Everything downstream —
grounding, pressing, verifying — is unreachable there. See
[[Experiment-091-four-demonstrations-and-no-press]].
