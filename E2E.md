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
demonstration recorded against a backgrounded game is a demonstration of your terminal.

```sh
# 1. find the window title
./director.exe windows | grep -i <game>

# 2. start watching, then switch to the game within the 5s delay
./director.exe demonstrate-visual "enter free play" --window-title "<title>" --seconds 120
```

Then, with your own mouse:

1. click one menu entry, then **wait, and wait longer than feels necessary** — several seconds,
   until the new panel has plainly been sitting there doing nothing;
2. click a second entry, and wait the same way again;
3. click a third;
4. come back to the terminal and press **ENTER**.

**A second is not enough, and that is what a demonstration with no outcomes looks like.** For a
step to remember an OUTCOME — words that appeared somewhere other than the control you pressed and
STAYED — the readings after your click have to carry it: the first two are dropped as mid-response
(`observe.OutcomeSkipReadings`) and five more have to agree (`OutcomeSettleLooks`), and a reading
is an OCR pass that finishes when it finishes. Click again before that and the window is cut short
— the step is still learned, it still arrives somewhere, and it has nothing to verify against. So
the pause between the clicks is not politeness, it is the evidence. And see
[[ADR-242-the-second-action-waits-for-the-first-ones-answer]] for what it buys: a two-action Act
can only hold the second press back if the first press has an outcome to be waited on.

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



## Two actions, and the second waiting for the first (CLI, needs your hands, ~4 min)

The step that makes a WORKFLOW out of a single guarded action. Nothing new to demonstrate: a
routine with two presses in it already has everything this needs, as long as each step carries an
outcome expectation.

Which is the thing to settle BEFORE running anything. If `sequences` says a step cannot be
verified, it is the demonstration that needs redoing and not the run: a two-action trial over a
step with no outcome does not test the waiting, it tests the honest shrug for a file that cannot
be checked. The pause between the two clicks is what records it — see the teaching walkthrough
above.

```sh
./director.exe sequences "<name>"      # read this FIRST: it says which steps can be verified
./director.exe perform-workflow --name "<name>" --live --reading --hotkeys
```

`--reading` is not optional here and the reason is the whole point: a step's outcome IS words, so
without OCR there is no evidence either way, the verdict is `unconfirmed`, and the run stops
before the second action. That is the correct refusal, and it is also the most likely way to see
this behave as though it were broken.

What should happen, in this order:

1. **F7** to start, then name the screen you are on (**F7** if Marco's candidate is right);
2. step 1: agree (**F7**), then authorise the press (**F7**) — the control is pressed;
3. the panel changes. **The run does not move on yet.** The transcript says the destination
   arrived and the outcome is still being decided — *a screen that changed is not the same
   evidence as a screen that changed the way it was demonstrated to*;
4. the words of step 1's outcome appear and hold for a full settled window: `step 0: outcome
   verified`;
5. **only now** is step 2 put to you. Agree, authorise, and the second press goes out.

What to check, beyond "both presses happened":

- the run reference's record (`%MARCO_HOME%\runs\<id>.json`) says `verified` with `actions_sent` 2;
- **the negative half, which matters more.** Run it again and, when step 1's panel appears, make
  the outcome false — go somewhere else in the menu, or start from a screen whose panel does not
  contain step 1's expected words. The run must refuse at step 1 with `contradicted` or
  `unconfirmed`, `actions_sent` must be 1, and **step 2 must never be put to you at all**. A
  workflow that asked you to authorise an action whose predecessor did not work would be a
  workflow whose verification is a log line;
- a step saved with no outcome expectation (`sequences` tells you which) still advances on its
  destination arriving, and the report says its action could not be verified. That is the honest
  end for a file that cannot be checked, not a regression.

See [[ADR-242-the-second-action-waits-for-the-first-ones-answer]].
## When it does not work: asking where the words went (~1 min, no hands)

Three things, in the order to reach for them.

```sh
marco do "<name>" --explain     # where those words would go, performing NOTHING
set MARCO_LOG=debug             # before overlay.cmd: the engine's and the overlay's own logs
set MARCO_TRACE_INTAKE=1        # one line per invocation: source, decision, why
```

`--explain` is the one to start with, and it is the only one that is safe to type when you do not
know what is about to happen. It prints which half of the product would take the words, why, and
what would carry them out — and it performs nothing at all, which is held by
`TestExplainingNeverPerforms` and `TestTheEntranceNeverPutsAFlagInThePhrase`.

**The failure it was written for.** One name, two artifacts:

```
words        "ark graphics live"
application  arkascended (what is in front now)
decision     play
because      an exact match for a play Marco has
would run    the play ark-graphics-live (scoped to arkascended)
but note     "ark graphics live" is ALSO saved as something you demonstrated, and the
             play above answers first. To perform what you demonstrated instead:
             director.exe perform-workflow --name "ark graphics live" ...
```

A recorded play of raw coordinates and a pixel demonstration under one name. The play arm answers
first (see [[Invocation]]), so typing the name ran the coordinates — and `learn <name>` recreates
that play every time it is used, because the two acquisition paths write to two different stores.
Until that is settled, a demonstrated routine wants a name no play answers to.

And in the log, at INFO, both sides of the process boundary now say how it ended:

```
[INFO] overlay - intake source=typed phrase=ark graphics live
[INFO] intake - routed trace=source=typed decision=demonstration ...
[INFO] intake - the supervised run ended name=... run=run-20261019-121500-abcd ran=verified
[INFO] intake - finished decision=demonstration outcome=performed words=ark graphics live
[INFO] overlay - finished name=ark graphics live outcome=performed took=12.4s
```

The log used to carry the request and nothing about the answer, so a command that failed and a
command that was never dispatched looked the same in it.
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

## Running an Act you composed, by name (CLI, needs your hands, ~5 min)

An **Act** is a `.marco` play whose own sentences are the order. Each one asks for an interaction
you demonstrated, by the name you gave it, and the next one sits inside the previous one's
`when ok?` — so nothing follows an action that was not verified. The order lives in the file you
can read, not in Go.

Two commands, and the first one presses nothing.

```sh
./director.exe sequences                       # which demonstrations you have
./director.exe sequences "<demonstration>"     # and which interactions are in this one

./director.exe compose-act --from "<demonstration>" --name "graphics preset" \
    --interaction '0=the graphics tab' \
    --interaction '1=apply changes'
```

`compose-act` prints the play it saved. **Read it.** That is the whole reason an Act is legal
Marco rather than a data structure: you should recognise what it says, and the sentences should be
the ones you meant, in the order you meant them.

The `--interaction` flags are `<number>=<what you call it>`. The number is which interaction of
the demonstration — `sequences "<name>"` lists them — and the name is yours; the play says your
words, never `control_1`. **The flag order is the run order**, so an Act may be a subset of a
longer demonstration, or its interactions the other way round. Composing over a name you already
have is refused rather than replacing it.

Then run it, one authorised press at a time:

```sh
./director.exe perform-workflow --play "graphics preset" \
    --application <exe> --live --reading --hotkeys
```

`--reading` is not optional, for the reason the two-action section above gives: an outcome IS
words, and without OCR the verdict is `unconfirmed` and the run stops before the next
interaction. That refusal is correct and is the likeliest way to see this look broken.

What should happen, in this order:

1. `compose-act` prints a play with one scene per interaction and the second `do` **indented
   inside** the first one's `when ok?`. If the two sentences are at the same depth, stop — a flat
   play presses the second control after the first one failed.
2. The run names the Act and the demonstration it came from, then asks you to press F7 to start.
3. It asks which screen you are on. F7.
4. It asks whether the first interaction belongs in this run. F7.
5. It asks to press the first control, naming where. F7 — and with `--live` it is pressed.
6. It waits, reads the screen, and reports the first interaction `verified`.
7. **Only then** does it ask about the second. Nothing is asked about it before step 6 lands.
8. At the end: one line per interaction, a run reference, and a record naming the Act and the
   demonstration both.

Three refusals worth producing on purpose, because each one is a guarantee:

- `--play "<name>" --step 1` — refused on usage. An Act already says which interactions it
  performs; `--step` would renumber the demonstration underneath it and the first sentence would
  press whichever interaction moved into its place.
- Answer the press question **after** `--seconds` has passed (use `--seconds 10 --answer 30s` and
  wait). Nothing is authorised, nothing is pressed, and the record says the time limit — not that
  you cancelled.
- Edit the saved `.marco` by hand and run it again. Refused: the play is rebuilt from its bindings
  and compared, so only a play Marco composed runs, and only while its bindings still describe it.
  Compose it again to change it.

An Act whose interactions all verify exits 0 and records `verified`, even when it performs only
some of a longer demonstration — the completion that matters is the play's, not the artifact's.

---

## 60. Pressing Play (needs your hands, ~2 min)

The flow a person uses. Nothing here asks you to manage a window, race a countdown, read JSON or
kill a process — if any of that turns out to be necessary, that is the finding.

```sh
go build -o director.exe ./cmd/director
go -C plugins/studio build -tags "desktop,production" -ldflags "-H windowsgui" -o studio.exe .
copy director.exe plugins\studio\director.exe
plugins\studio\studio.exe
```

Have the game running, on the screen your Act starts on. You do not need it in front — Marco
brings it forward, because Windows only lets the window you just clicked in do that.

1. **Pick the interaction you want performed.** Under *1 · What you have demonstrated*, each one
   is described by the words the demonstration read on it — `the one that reads “audio · camera ·
   cosmetics · graphics · video”` — with what can be checked afterwards beside it: `afterwards: 61
   words appeared and stayed (advanced, bloom, brightness…)`, or `nothing was recorded to check
   afterwards`. If it says the second thing, pick something else or demonstrate it again: an
   interaction with no outcome can be performed and can never be verified.
2. **Name the Act and save it.** Give it a name you would say out loud — and NOT the same words as
   one of its interactions, which generates two declarations of one name and is refused. The play
   it saved is printed; it is meant to be readable.
3. **Press Play**, or **Try it without clicking** for a pass where every click is written down
   instead of pressed. That press IS the authorisation: Marco will perform the interactions this
   Act names, on the application it was demonstrated against, and will not ask you anything.
4. **Watch the run panel.** `1 of 1 · the audio tab`, then `verifying · 1 of 1 · the audio tab`
   while it checks, then the same line with the verdict. **Stop** is live the whole time.
5. **Read the result.** A sentence, not a word: *It worked. Marco performed "the audio page" and
   the screen changed the way it did when you showed it.* The outcome word is beside it for the
   record.

What to check, and what each failure would mean:

- **Nothing is ever asked.** No countdown, no "which screen are you looking at", no "may Marco
  press this". If any question appears, the run is not under the authorisation it was given.
- **Marco brings the game forward by itself.** If it does not, and the panel says it cannot see
  the application, Windows refused the raise — note what was in front at the time.
- **A pause is a sentence you can act on.** If Marco cannot tell which saved screen you are on it
  says so and keeps looking: *what is on screen agrees with screen_c at 0.41, below 0.55 — this
  does not look like the screen "the audio page" starts on.* Navigate to that screen; it should
  proceed by itself, with nobody pressing anything. **A pause that nobody could act on is the
  defect to report**, and so is one whose sentence names something only an engineer would
  recognise.
- **`control_2` must not appear anywhere you are choosing or authorising.** It is a name this
  machine gave itself.
- **The two words that matter.** If it ends `refused`, nothing was pressed. If it ends
  `contradicted`, something WAS pressed and the screen did not change the way you showed it. The
  sentence says which; if it does not, that is the defect.

Then, for the record: the run reference is in the panel, and *Inspect the record* shows the JSON
with `authority` naming the press of Play that permitted it. That is engineering evidence and
nothing in the flow above needs it.

## 61. The engineering path: watching the same run, where the state is the thing being checked

**This is the supervised contract, not the product one.** Every press asked for and answered by
key, the screen named by key, the application typed by hand — behind *the engineering path* in
Studio's run section. It is kept because it is the only way to exercise the consent desk from a
window, and because the account it produces is the thing being checked here. What a person does
is §60.

`plugins/studio` spawns exactly the run above — `director perform-workflow --play …` — and shows
what it says about itself rather than reading its prose. So this step is not a second test of the
Act; it is a test of the account, and the only way to check an account is to watch it beside the
thing it describes.

```sh
go build -o director.exe ./cmd/director
go -C plugins/studio build -tags "desktop,production" -ldflags "-H windowsgui" -o studio.exe .
copy director.exe plugins\studio\director.exe
plugins\studio\studio.exe
```

**The copy is not optional.** Studio spawns the supervised runner from `director.exe` beside its
own executable, falling back to whatever the PATH answers; a Studio without one reports every run
as failing to start, which reads as a broken Act rather than a missing binary. And launch it with
no `-home` and no `-isolate`: the store then resolves the way `marco` and `director` resolve it, so
the demonstrations listed are the ones you made. `-isolate` is for the shell proof, and a window
on an empty temporary store lists nothing and looks like lost work.

Pick the Act, leave **live** off for the first pass, and press Run. Then watch the top line of the
run panel, not the transcript, and check each of these AS IT HAPPENS — a screenshot afterwards
cannot tell you any of them:

1. **The countdown counts down, and stops when you say so.** `preparing · Ns left to get ready`.
   Press F7 before it reaches zero: the countdown must VANISH in the same beat, not run on. If it
   keeps counting after you have pressed start, the run's end of preparation is not reaching the
   window and everything below is suspect.
2. **The key offered is a key that works.** At the start prompt the panel offers two — yes and
   cancel. There must be no "no" on offer there: `started` ignores it, and a key that does nothing
   reads as a run that is not listening.

   **And every question reads as a question about something you recognise.** This is the one
   check only a person can make, and it is not about the keys: read each prompt as it arrives and
   ask whether somebody who did not compose this Act could tell what they were agreeing to. The
   words are the runner's own, verbatim — Studio does not paraphrase them, deliberately, because
   a window's summary of a consent prompt is a different question from the one being answered. If
   a prompt names a control instead of the thing you called it, or asks about a screen in terms
   you did not choose, write the sentence down: the fix belongs in the runner's words.
3. **The phase moves through `starting` before anything is asked.** Acquisition and bringing up
   perception take seconds, and during them the window must say `starting` and show no question.
4. **The interaction is named the way you named it, and numbered the way the Act is.** `1 of 2 ·
   the graphics tab`. If it says the demonstration's step number, or the control's identifier
   (`control_1`), stop — that is the window telling you something false about the press you are
   about to authorise.
5. **`verifying` stands while Marco is watching.** After the press the panel must read
   `verifying · 1 of 2 · the graphics tab` with no verdict, for as long as the check takes, and
   then the same line WITH the verdict. If it says `acting` through that interval, the wait is
   being shown as a press.
6. **The second interaction is not mentioned until the first has a verdict.** Same guarantee as
   step 7 of the walkthrough above, read off the panel instead of the transcript.
7. **The mode the panel claims is the mode the run is in.** The tag beside the Act reads `dry`
   or a red `LIVE`, and it comes from the run's own account rather than from the checkbox. Tick
   the live box WHILE the dry run is in flight: the tag must stay `dry`, because the run in front
   of you is a dry one and a window that re-labelled it would be lying about what the next F7
   does. In the dry pass nothing moves on screen and the record ends up saying `"live": false`
   with `"actions_sent"` above zero — the whole run happened and every click was written down.
8. **Cancelling ends the run, and the run says it was cancelled.** Press **F9** (or the Cancel
   button, which raises the same one stop broadcast) while a question is up. The outcome tag must
   become `cancelled` and not `refused` — a run somebody stopped and a run that would not proceed
   are different endings — the Cancel button must go dark, and the panel must NOT say INCOMPLETE,
   because a cancelled run ends by saying so. Cancel once AFTER authorising a press, too: the stop
   is read at the last moment it could matter, so the record's `"actions_sent"` must leave no doubt
   which side of the press the stop landed on.
9. **The final display and the saved record agree.** When it has ended, press **Inspect the
   record** and read the JSON against the tags above, field by field: `run` against the reference,
   `outcome` against the outcome tag, `live` against the mode tag, `act` against the Act, and
   `verified` / `contradicted` / `unconfirmed` against what the phase line said as it went. The
   file is `%MARCO_HOME%\runs\<reference>.json`, and it is the artefact a bug report cites — so a
   panel that showed `verified` over a record that says `unconfirmed` means the thing you can cite
   disagrees with the thing you watched, and the record wins.

Then produce the one case no test can produce on a real screen: while a question is on the panel,
**kill THAT runner** — the one Studio spawned, by its own process id. Studio starts it as a child
of itself, so that is how to find it:

```powershell
# the Studio run's own director.exe, and nothing else wearing that name
Get-CimInstance Win32_Process -Filter "Name='director.exe'" |
  Select-Object ProcessId, ParentProcessId, CommandLine
taskkill /PID <the one whose CommandLine says perform-workflow> /F
```

**Not `/IM director.exe`.** That kills every Director process on the machine, including a daemon
somebody else is depending on and anything else mid-run; what this case needs is one supervised
runner gone, with the rest of the machine untouched. And not `director stop` either: that raises
the cooperative stop, which is check 8 above, and a stopped run ends by saying it was cancelled —
you would be shown a complete run and this case would go untested. A hard kill of the one child is
the only way to leave a stream with no ending in it.

Three things must then hold, and the third is why the kill is targeted:

- the panel says INCOMPLETE;
- it STOPS offering the keys — the question stays visible as what it was ("it was asking this when
  it last said anything"), and nothing invites you to press F7 at a process that has gone. If the
  keys are still offered, a person can answer a question nobody is listening to and watch nothing
  happen;
- every OTHER `director.exe` is still running. Re-run the `Get-CimInstance` line above: a daemon
  that went down with the runner means the kill was not the one this case is about, and whatever
  the panel then said is evidence about a different event.

## 62. Recording a routine in Studio's window, and running it (needs your hands, ~6 min)

The recorder-first flow, end to end: Record → Stop → Review → Save → reload → Play. Nothing here
is a demonstration of a *control* — this is the other half of the product, where you show Marco a
sequence of presses and it saves a routine that performs them.

**Everything about it is automated evidence so far and none of it has been done on a real
desktop.** This section is what turns that into acceptance.

```sh
plugins\studio\build.cmd
plugins\studio\studio.exe
```

Have the game running. You do **not** need it in front: you will switch to it yourself, and that
switch is recorded as the step that performs it.

1. **Name it and press Record.** Under *Record a routine*, type a name you would say out loud —
   `ark graphics` — and press Record. The card becomes `Recording "ark graphics"…`, with `or press
   <key>` beside it if a stop key is set.
2. **Switch to the game and do the thing.** Ordinary left-clicks and the keyboard. Open a menu,
   press two or three controls, nothing clever.
3. **Visit Studio's window in the middle of it.** This is the case worth going out of your way
   for: switch back to Studio, look at it for a few seconds, then switch to the game again and
   **press another control**. Then finish.
4. **Press Stop** — in Studio's window, with the mouse.
5. **Read the review.** Every step in plain words, in order. Then press **Save**.
6. **Run it.** `marco do "ark graphics"` from a terminal, with the game on the screen you recorded
   it on.

What to check, and what each failure would mean:

- **Every press you made is in the review, including the one after your visit to Studio.** That
  last one is the whole reason for step 3: excluding Studio's own window used to take the press
  beside it as well. A review missing it is the defect, and the step count is the quickest tell —
  count your presses before you look.
- **Nothing you did in Studio's window is in the review.** No click on Record, no click on Stop,
  no switch to Studio and no switch back. If `studio` appears as a step, or there is a press at
  the Stop button's coordinates, the recorder's own window reached the routine.
- **Each press says what it measured**: `left-click at 722, 210 in the window (1920×1080, 144 dpi,
  arkascended.exe)`. A press that says `cannot be checked — <something> was not measured` is a
  real answer, not a bug — but note which measurement and whether the sentence beside the draft
  says whether recording it again would help.
- **The scope is not a question.** It should read *Runs in <app>. Marco brings it forward before
  playback.* with no dropdown. A three-way choice appearing here means the recording's presses
  are not being checked at all.
- **Save says where it went**, and the routine is on disk under `<app>\focus\`. Under
  `<app>\context\` is the defect: that scope brings no window forward and every press in it would
  refuse. The tree is `$MARCO_ROUTES`, or `routes\` beside **wherever you launched Studio from** —
  so launch it from the repo root and run `marco do` from there too, or the two look at different
  trees and the routine you just saved will not be found.
- **Read the saved Marco.** It should be a chain — `this can Run.` wrapping the Activate, then
  `this can Step2.` and so on, each calling the next only inside `when ok?`. One flat block with
  bare `do OS's Click` lines means a refusal would not stop it.
- **Running it presses where you pressed**, in the window wherever it now is. Move the game window
  before running it: that is the point of the whole contract, and a routine that presses the old
  desktop coordinates is the defect.
- **And it refuses rather than guessing.** Resize the game window and run it again: it should stop
  at the first press and say the window is a different size now. `marco do` should exit non-zero
  and print the sentence. If it reports success having pressed nothing, the result is not reaching
  the command.
- **Right-clicks and drags are refused at Save** in this version, with a sentence naming which.
  That is deliberate; a recording containing one that saves anyway is the defect.

Then, for the record: `<routes>\<app>\focus\<slug>.rec.json` holds the raw demonstration beside the
routine. Nothing executes it, and Studio reads it to save the routine again if it ever needs to.
