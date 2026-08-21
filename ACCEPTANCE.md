# Minimum live acceptance

`E2E.md` is the full manual walkthrough. This is the short one: **twenty checks, about half an
hour**, covering the things a Go test structurally cannot — real hooks, a real desktop, two real
processes, a browser, and a person demonstrating something.

Everything here is safe. Nothing asks you to type into a document you care about, and the one Play
you make performs no input of its own.

> **If a check fails, that is the point.** Write down what you saw and stop; a failure here is worth
> more than the other nineteen passing.

---

## Setup — a sandbox, so nothing touches your real plays

```powershell
$env:MARCO_ROUTES = "$env:TEMP\marco-accept\routes"
$env:MARCO_HOME   = "$env:TEMP\marco-accept\home"
New-Item -ItemType Directory -Force "$env:MARCO_ROUTES\global", $env:MARCO_HOME | Out-Null
go build -o marco.exe ./cmd/marco
go build -o director.exe ./cmd/director
go -C plugins/overlay build -o overlay.exe .
```

Make one Play that does nothing but wait, so you can stop it:

```powershell
@'
use os.

the App is a script.

log "STARTED".
repeat 40000000 times...
    do OS's Sleep with 1...
        or?
log "FINISHED THE WHOLE LOOP".
finally...
    log "CLEANUP RAN".
'@ | Out-File -Encoding utf8 "$env:MARCO_ROUTES\global\wait-for-me.marco"
```

---

## A. One stop — the headline of Phase 3

**1. A running Play stops, and its cleanup runs.**
In terminal 1: `.\marco.exe run --host dryrun "$env:MARCO_ROUTES\global\wait-for-me.marco"`
In terminal 2, after a second or two: `.\marco.exe stop`

- [ ] Terminal 1 stops within about a second.
- [ ] It printed `CLEANUP RAN`.
- [ ] It did **not** print `FINISHED THE WHOLE LOOP`.

*Why it matters:* before this campaign the cleanup never ran on any cancellation, so a Play holding
a key down left it held. `CLEANUP RAN` is the key being released.

**2. The same, for a Play started the way the overlay starts one.**
Terminal 1: `$env:MARCO_NO_PANIC_STOP="1"; .\marco.exe run --host dryrun "$env:MARCO_ROUTES\global\wait-for-me.marco"`
Terminal 2: `.\marco.exe stop`

- [ ] It still stops, and still prints `CLEANUP RAN`.

*Why it matters:* this is most Plays anybody runs. That variable used to make them uncancellable,
and the overlay's only remaining option was to kill the process — which runs no cleanup at all.
Unset it afterwards: `Remove-Item Env:\MARCO_NO_PANIC_STOP`.

**3. Stop with nothing running is harmless, and says so without backstage words.**
`.\marco.exe stop`

- [ ] It answers `stop sent — anything Marco was running will stop`.
- [ ] It does **not** name the Director. (`.\marco.exe director stop` is the developer verb and
      *does* say "the Director is not running" — that one is correct, because you asked about it.)

---

## B. One way in

Start the stack: `.\overlay.cmd` (leave it running for the rest of this).

**4. Typed and spoken reach the same Play.**
Press the leader key (`` ` `` by default), then `m`, and type `wait for me`. Stop it with the leader
key. Then say the same words, if voice is set up.

- [ ] Both start the same Play.
- [ ] The leader key stops it.

**5. A request Marco does not know goes to the Director.**
`` `m `` then `make me a sandwich`

- [ ] Marco answers about the screen or says it cannot — it does **not** offer to record a Play
      called "make me a sandwich".

**6. The trace agrees, if you want to see it.**
In a terminal with `$env:MARCO_TRACE_INTAKE="1"`, run `.\marco.exe do "wait for me"` and
`.\marco.exe do "make me a sandwich"`.

- [ ] The first says `decision=play`. The second says `decision=director`.

---

## C. The product surface — Phase 4

**7. The control centre has a normal half and an Advanced half.**
`` `m ui ``

- [ ] The drawer shows **Here · Learn · Plays · Activity · Bindings · Settings · Help**, and
      **Advanced** separately below.
- [ ] The step editor is under Advanced, **not** in the normal list.

**8. Every view opens by name.**
`` `m ui here ``, then `` `m ui activity ``, `` `m ui advanced ``, `` `m ui plays ``, `` `m ui settings ``

- [ ] Each opens that view. None of them says `No play named "…"`.

**9. Here answers "what does Marco see?" on its own.**

- [ ] Here shows what Marco makes of the screen without you starting a Learn session.

**10. A clicked Run reports what actually happened.**
In **Plays**, press Run beside `wait for me`, then stop it with the leader key.

- [ ] The page ends up saying it was **stopped / cancelled** — not "running…" for ever, and not
      that it ran.

*Why it matters:* this surface used to answer "ok" the instant the process started, so a refusal, a
cancellation and a success looked identical.

**11. Activity says what happened.**
Open **Activity**.

- [ ] The run you just stopped is there, with an outcome word.

**12. No backstage words on a normal page.**
Look over Here, Learn, Plays, Activity, Settings.

- [ ] You do not see: `DIRECTOR NOT RUNNING`, `Transition:`, `Target locked:`, or a raw id like
      `subj_a1b2c3`.

**13. Cast says who can act, and why not.**
Open **Advanced** → the Cast table.

- [ ] It names the Accessibility Actor, where its provider is, and whether it can act.
- [ ] Now try it broken: in a terminal, `$env:MARCO_UIA_BRIDGE="C:\nope\uia.exe"; .\marco.exe director diagnose`
      — it should say the Actor cannot act **and why**, naming that path. Unset it afterwards.

---

## D. Nothing was lost — the earlier phases

**14. Learn still works, both ways.**
`` `m learn open the calculator `` — demonstrate opening Calculator, then press the leader key.

- [ ] It saves, and **Plays** lists it as **Recorded**.
- [ ] `.\marco.exe do "open the calculator"` runs it.

**15. A learned Play still runs cold.**
If you have a learned (not recorded) Play from earlier work, close everything, start fresh, and ask
for it by name.

- [ ] It brings its app forward, recognises where it is, performs, and confirms arrival.

> **The one behaviour change to watch here.** A performance now checks that the window it is about
> to type into is actually in front, which it never did before. If a Play refuses with
> *"the watched window is not in front"* where it used to type, that is the new gate working — but
> note it, because it may mean the app was brought forward ambiguously (several windows sharing one
> process is a known open issue, ADR-078's follow-on 2). It fails safe: it refuses rather than
> typing into the wrong window, and no permission is spent.

---

## E. Fast Learn — watch once, understand, remember

Roadmap 35B. This is the one that needs your hands: Marco has to watch a real person do a real
thing. Everything below runs against the sandbox from Setup, so your own plays are untouched.

Start the stack (`.\overlay.cmd`) and open **Windows Settings** on its **Home** page.

**16. Learn it in one demonstration.**
In the overlay: `` `m learn open mouse settings ``
Then, in Settings, click **Bluetooth & devices**, then click **Mouse**. Press the leader key to
finish.

- [ ] Marco does **not** ask you to name Home, Bluetooth & devices, or Mouse.
- [ ] Marco does **not** ask "Can I try that?" for either step.
- [ ] Marco does **not** move the mouse or press anything itself.
- [ ] It finishes concisely — the sense of *"Got it."*

*Why it matters:* before this, learning that route required Marco to REPLAY both steps on your
desktop, after asking permission twice, to learn something you had just shown it.

**17. It kept both steps, and one Play.**
`.\marco.exe plays`

- [ ] `open mouse settings` is listed, as **Learned**.
- [ ] Exactly one Play was created — not one per step.

**18. It does not claim it performed anything.**
`.\director.exe reach "open mouse settings"`, or the control centre's Advanced view.

- [ ] Marco reports it knows a way there.
- [ ] Nothing claims Marco executed or verified those steps — it watched you, and only that.

*This is the distinction the whole roadmap turns on. If any surface says an edge Marco never ran is
"verified", that is a bug and worth stopping for.*

**19. Demonstrate the same route again.**
Go back to Settings Home and repeat step 16 exactly.

- [ ] `.\marco.exe plays` still shows **one** `open mouse settings`.
- [ ] No duplicate Places appear — Here still shows three, not six.

**20. Now run it — the first time Marco actually performs it.**
Put an unrelated window in front. Then: `` `m open mouse settings ``

- [ ] Settings comes forward.
- [ ] Marco navigates to **Mouse** and reports success only after arriving.
- [ ] If it refuses instead, note the reason — a refusal here is honest, and is data rather than a
      failed acceptance.

*This is where observation becomes proof: Marco learned the route by watching, and proves it now,
under the ordinary authority and verification it has always used.*

## Tidy up

```powershell
Remove-Item -Recurse -Force "$env:TEMP\marco-accept"
Remove-Item Env:\MARCO_ROUTES, Env:\MARCO_HOME
```

Your real plays were never touched: everything above ran against a temporary `$MARCO_ROUTES`.
