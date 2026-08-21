---
type: decision
status: accepted
date: 2026-08-21
supersedes: []
affects:
  - service
  - execution
  - passive-observation
source_paths:
  - internal/platform/homelock/homelock.go
  - internal/platform/homelock/homelock_windows.go
  - internal/platform/homelock/homelock_stub.go
  - cmd/director/serve.go
  - cmd/director/rehearserun.go
  - internal/director/rehearse/live.go
---

# ADR-092 — one Director per home, one hand on the keyboard

## Three Directors, two of them on one home

Found while diagnosing something else entirely, by listing what was actually running:

```
pid=10672  …\marco-35b\director.exe
pid=38436  …\marco-35c\director.exe
pid=41716  …\marco-35c\director.exe     ← two, on the same home
```

Two processes serving one `$MARCO_HOME`: two observation loops on one desktop, two writers to one
semantic store, either able to cancel while the other kept acting, and each answering "where is
the Audience standing" from its own private world.

Nothing prevented it. The CLIENT had a startup lock — `Connect` takes a file lock before
autostarting, precisely so two clients do not spawn two services. `director serve` had none, so
running it twice always made two Directors, and one of the two above was spawned by a harness
that had been *fixed* to always start one.

## Three ownerships, and collapsing any two would be wrong

| | says | scope |
|---|---|---|
| **home ownership** | "I am the Director for this Marco world." | one per canonical home |
| **desktop lease** | "I am the only runtime driving this screen." | one per machine, per production |
| **Audience authority** | "Marco may perform this action." | one per invocation |

They are genuinely different and this ADR adds the first two. The third is unchanged and still
required: a lease that granted permission would be exactly the shortcut
[[ADR-029-resolution-is-not-permission]] exists to refuse.

**Home ownership cannot cover the desktop**, and that is the part worth stating. Two homes are
two legitimate worlds — an acceptance sandbox beside the real store is how every harness in this
repository runs — and they share one keyboard. A lease scoped by home would let them type at the
same time, each holding something it genuinely owned.

## A file cannot be the primitive

`$MARCO_HOME` is not merely a directory; it names one semantic world. But nothing about a file
can say who owns it:

- it survives the process that wrote it, so a crash leaves a claim nobody holds
- a PID inside it can be reused by an unrelated process
- a staleness timeout is a guess about how long a start takes

What is needed is a claim whose lifetime is the PROCESS's. On Windows that is a named kernel
object: `CreateMutexW` returns a handle, the object lives while a handle is open, and Windows
closes every handle a process holds when it ends — cleanly, by crash, or by being killed. There is
no stale mutex, no timeout, and no PID to distrust.

**The endpoint file stays, as discovery.** It answers where and with what token, and it is
validated by connecting rather than believed. It does not prove ownership and never did — that it
was the closest thing to a check is how three Directors happened.

**`ERROR_ALREADY_EXISTS` still returns a handle**, and that handle is closed rather than kept.
Holding it would keep the object alive past the real owner's exit and turn a released claim into a
permanent one — a crash that bricked the home.

## Canonical home identity

Ownership is keyed on the home, so two spellings of one directory must be one key or two
Directors both claim, both succeed, and every guarantee here is gone with no error anywhere.

Handled: relative against absolute, slash direction, trailing separators, `.` and `..`
components, symbolic links where the path exists, and **case only where the platform's own
filesystem folds it** — folding everywhere would merge two genuinely different homes on a
case-sensitive filesystem, which is the opposite mistake and the worse one.

The key is a hash rather than the path: a kernel object name cannot contain a separator, and this
string reaches diagnostics, where a filesystem path is more than a reader needs and more than
Marco should volunteer about somebody's machine.

**The fixture uses a home that does not exist yet**, and that is not a detail.
`filepath.EvalSymlinks` resolves an existing path and on Windows hands back the real spelling —
right case, right separators, no dot components. A test against an existing directory is testing
EvalSymlinks, and three of the normalisations below it survived deletion exactly that way. A home
that has not been created is the ordinary case anyway: the first `director serve` for a sandbox
claims it before anything makes the directory.

## The order is the property

The claim is taken before `NewRuntime`, before `NewServer`, before `Listen`. Every one of those
is an act of the runtime owner — opening the semantic store, building perception, registering
commands that can drive the desktop, publishing an endpoint clients will connect to — and a
second Director that performed them and then discovered it was not the owner would already have
half-owned the world.

A claim taken afterwards is a check that has already lost.

A refused Director says so and stops. It does not start anyway, and it does not take the claim
away: a second Director that killed the first would make starting one a hidden restart mechanism,
and a play running at that moment would be cut off mid-route.

## The desktop lease is held around a production

Not for the life of the Director. Held from startup, one Director would stop every other on the
machine from ever acting, including the sandbox ones this repository's own harnesses run; held
around the walk, it stops only what actually conflicts.

Around the WALK rather than each step: a route is a sequence, and another runtime typing between
step two and step three is exactly as wrong as typing during one.

It sits **beside the foreground gate and before the grant is claimed**, for the same reason:
somebody else using the keyboard is a reason to wait, and waiting costs nothing only while the
permission is unspent. `RehearsalGrant.BeginAttempt` sets `GrantConsumed` and `Attempt.Cancel`
does not undo it, so a lease checked one line later would make every busy desktop cost the person
a permission they would have to give again.

**Observation needs no lease at all**, which is what makes a debug Director useful while the real
one works, and what keeps the sandboxes running.

**No Actor competes for the desktop.** The lease is installed once, at the composition root that
builds the walker, next to the foreground answer — the same place, for the same reason there is
only one of it.

## What this does not claim

- **Not distributed anything.** No election, no failover, no coordination service. One machine,
  one login session, `Local\` object names.
- **NOT "one Director per machine".** That would break every acceptance sandbox. One per
  canonical home, and different homes coexist by design.
- **Not a fix for concurrent writers generally.** Play files and route registries are still
  written by `marco` processes according to the existing architecture; this ADR's authority is
  the Director runtime, the semantic world and desktop actuation.
- **The stub enforces nothing**, and says so in its own file rather than pretending. A build on a
  platform with no backend can run two Directors against one home. Refusing to claim there would
  make the Director unstartable to guard against a race that platform's users would have to
  create deliberately, and a file-based stand-in would be the very thing this ADR rejects with
  the added harm of looking authoritative.

## Consequences, including the costs

- **A crash cannot brick a home.** The operating system releases the claim, the next Director
  acquires it, and stale discovery metadata is replaced by use rather than by hand.
- **PID reuse is irrelevant to authority.** The PID is reported in diagnostics because a person
  reading it wants it, and nothing decides anything from it.
- **A home mismatch is refused by the TOKEN, not by a home check.** The endpoint file lives in
  the home directory, so a client reads its own home's metadata; the risk is a stale file naming
  a port some other home's Director now holds, and the per-start random token refuses that. No
  home identity was added to the handshake, because the protection already exists and a second
  one would be a second answer to the same question. Stated rather than claimed as new work.
- **`desktopObject` takes no home**, which is the guarantee and is untestable from inside one
  process: a mutation appending an identity would append the same one in every caller. It is
  stated at the function, and what it protects is held one layer up.
- **A dry walker is handed no lease, and that is belt and braces.** `Live.Perform` guards on
  `l.real` too, so the mutation that hands a live claim to a dry walker survives the suite. Kept
  because it makes the wiring honest on its own terms; the enforcement is one layer down where it
  can be tested.

## KNOWN FOLLOW-ONS

1. **The `Connect` startup lock is now a second mechanism.** It is still a file lock with a 30
   second staleness window, and it is now redundant with the real claim — a losing child refuses
   at startup instead of racing. Worth simplifying, and not worth changing blind.
2. **No live acceptance.** Whether two `director serve` invocations on one home refuse correctly
   on a real desktop is **UNMEASURED**.
3. **Multi-process writers are not audited exhaustively.** This ADR removes the Director-vs-
   Director case; `marco` writing routes and bindings is unchanged and legitimate.
4. **`setup.ps1` and friends still report a raw file-lock error** when a binary is replaced while
   the stack is running. Ownership could produce a better sentence; not attempted here.

## Ready for ambient Observe?

For the ownership half, yes, and this is the shape it needs:

```
marco observe
  → resolve canonical home
  → find the one Director owner (start one if none)
  → tell that owner to enable observation
  → the same Director owns Stage
  → no second observer runtime
  → observation takes no desktop lease
```

Every line of that is now available. Observe is not implemented here.

## Enforced by

- `internal/platform/homelock` — `TestOneHomeSpelledSeveralWaysIsOneHome` (six spellings of a
  home that does not exist yet, plus case on the platform that folds it);
  `TestTwoHomesAreNotOneHome`; `TestAHomeHasOneOwner` (and releasing gives it back);
  `TestOwningOneHomeLeavesAnotherFree`; `TestTheDesktopIsOneWhateverTheHome`;
  `TestReleasingAClaimTwiceIsHarmless`.
- `cmd/director` — `TestADirectorClaimsItsHomeBeforeItOwnsAnything` (the ORDER, read from the
  source, because `runServe` cannot be entered); `TestEveryLiveWalkerClaimsTheDesktop`;
  `TestALiveWalkerRefusesWhenTheDesktopIsBusy` and
  `TestALiveWalkerProceedsWhenTheDesktopIsFree`, which enter through the production request with
  the machine's answer replaced.
- `internal/director/rehearse` — `TestTwoRuntimesDoNotInterleaveRealInput`;
  `TestADesktopRefusalSpendsNoPermission`; `TestADesktopLeaseIsAlwaysGivenBack`.

## Related

[[ADR-029-resolution-is-not-permission]] ·
[[ADR-087-one-stop-and-it-crosses-a-process-boundary]] ·
[[ADR-085-a-performance-is-a-registry-command]] ·
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]] ·
[[Service]]
