"""Read one watch-interface.cmd run and answer the seven questions asked of it.

    py -I readrun.py <run directory>

The questions, in the words they were set in:

    the actual selected-entry sequence, timestamps, missed transitions, false
    transitions, observation cadence, and what Director believed throughout the
    session -- and in particular, distinguish a selection that Marco never saw
    from one it saw but declined to confirm.

THE ONE JOIN THAT MATTERS, AND THE TRAP IN IT. A press carries `inference` (how many valid
inferences had run when it was banked) and a belief moment carries `sample` (the sample
sequence it was folded at). Those are two different counters that happened to be EQUAL in
run 0001, because the inference blocked the observation cycle so every sample carried one.
ADR-152's fast tracking decouples them on purpose, so joining the two numbers would
silently mis-attribute every press in the next run. Everything here joins on TIME:
`started_at + at_ms` against a moment's absolute `at`.

Measured against run 0001 (Experiment-024), whose numbers are published in the experiment
record, so the reader can be checked rather than trusted.
"""

import io
import json
import os
import re
import sys
from datetime import datetime, timedelta

# How long after a press a selection move still counts as that press's effect. Two looks at
# the measured 2.6s period, rounded up: a press whose change needed a second agreeing look
# cannot show up sooner, and anything later is not attributable to one press.
ATTRIBUTION_WINDOW = timedelta(seconds=6)


def iso(s):
    """Parse a Go RFC3339Nano timestamp. Python wants at most 6 fractional digits."""
    if not s:
        return None
    s = re.sub(r"\.(\d{6})\d+", r".\1", s)
    try:
        return datetime.fromisoformat(s)
    except ValueError:
        return None


def hhmmss(t, t0):
    if t is None:
        return "    -   "
    return "%8.1f" % (t - t0).total_seconds()


def load(path):
    return json.load(io.open(path, encoding="utf-8"))


def stream(path):
    if not os.path.exists(path):
        return
    with io.open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    yield json.loads(line)
                except ValueError:
                    continue


def phases(path):
    """phases.txt -> [(name, wall clock)], the script's own record of what it asked for."""
    out = []
    if not os.path.exists(path):
        return out
    for line in io.open(path, encoding="utf-8"):
        parts = line.strip().split(" -- ")
        if len(parts) < 2:
            continue
        name = parts[0].strip()
        m = re.search(r"(\d+)/(\d+)/(\d+)\s+(\d+):(\d+):(\d+)\.(\d+)", parts[1])
        if not m:
            continue
        mo, d, y, h, mi, s, cs = (int(x) for x in m.groups())
        out.append((name, datetime(y, mo, d, h, mi, s, cs * 10000)))
    return out


def phase_of(t, ph):
    """Which phase a wall-clock time fell in."""
    if t is None:
        return "-"
    name = "before"
    for n, start in ph:
        if t >= start:
            name = n
    return name


def main(run):
    final_path = os.path.join(run, "final.json")
    if not os.path.exists(final_path):
        print("no final.json in %s" % run)
        return 2
    final = load(final_path)
    started = iso(final.get("started_at"))
    # phases.txt is the script's wall clock and carries no zone; started_at does. They are
    # the same machine's local time, so the session's zone is attached rather than guessed.
    ph = [(n, t.replace(tzinfo=started.tzinfo) if started else t)
          for n, t in phases(os.path.join(run, "phases.txt"))
          if re.match(r"^\d of \d", n)]
    # at_ms is measured from the producer's start, which this script's shape puts within a
    # few tens of milliseconds of the session's (serve-ready and session are back to back in
    # phases.txt; 40ms apart in run 0001). Stated rather than assumed, because the field's
    # own comment says "since the session began" and that is not where its zero comes from.
    anchor = started

    print("=" * 78)
    print("RUN %s   %s" % (os.path.basename(run), final.get("application", "?")))
    print("started %s   state %s" % (started, final.get("state")))
    if ph:
        print("phases: " + ", ".join(n for n, _ in ph))
    print("=" * 78)

    sh = final.get("stats", {}).get("shadow", {})

    # ── 5. observation cadence ────────────────────────────────────────────────
    st = final.get("stats", {})
    elapsed_s = st.get("elapsed", 0) / 1e9
    infs = sh.get("inferences", 0)
    print("\nCADENCE")
    print("  elapsed            %.1fs of %.0fs configured, interval %.0fms"
          % (elapsed_s, final.get("duration", 0) / 1e9, final.get("interval", 0) / 1e6))
    print("  samples            %d taken, %d skipped, %d late"
          % (final.get("samples", 0), final.get("skipped", 0), final.get("late", 0)))
    print("  inferences         %d (%d provenance failures, %d unproven)"
          % (infs, st.get("provenance_refusals", 0), sh.get("provenance_failures", 0)))
    print("  inference latency  median %dms  p95 %dms  max %dms"
          % (sh.get("median_ms", 0), sh.get("p95_ms", 0), sh.get("max_ms", 0)))
    if infs:
        print("  EFFECTIVE PERIOD   %.2fs per inference  <-- the number that decides whether"
              % (elapsed_s / infs))
        print("                     a selection can be followed at all")

    # ── which window the frames were of ────────────────────────
    #
    # Before anything derived from the pixels, because it changes how every number below
    # should be read. Experiment-025 captured the Windows taskbar on one look of 107 with
    # provenance clean and target_losses 0: window capture can return the covering
    # window's pixels under the target's name, and no identity guard sees it.
    behind, unclear = sh.get("behind", 0), sh.get("front_unclear", 0)
    print("\nWHICH WINDOW THE FRAMES WERE OF")
    if behind or unclear:
        print("  %d look(s) taken while ANOTHER window was in front - refused"
              % behind)
        print("  %d look(s) could not be attributed either way" % unclear)
        kept = infs - behind - unclear
        print("  %d of %d looks are evidence about the target" % (kept, infs))
        if infs and behind + unclear > infs // 4:
            print("  ^^ more than a quarter of the run. Read every number below as")
            print("     being about the looks that survived, not about the session")
    else:
        print("  every look was of the target, or the question could not be asked")
        print("  (a run from before this was measured reports nothing here)")

    # ── why each candidate list was refused ────────────────────
    #
    # ADR-156. The belief's `because` says how many candidates there were; these are the
    # tracker's own counters for each, and the requirements it fell short of. Five
    # unrelated defects produce the same refusal sentence and call for different responses.
    cands = sh.get("interface_candidates") or []
    print("\nWHY EACH CANDIDATE LIST WAS REFUSED  (%d watched)" % len(cands))
    if not cands:
        print("  the session reports no per-candidate evidence.")
        print("  Either no list was ever grouped, or this run predates ADR-156 - the")
        print("  belief's `because` below says which: it names the candidate count.")
    else:
        print("  %-5s %-6s %-6s %-5s %-6s %-5s %-6s %-5s %-7s %s"
              % ("rows", "frames", "judged", "excl", "shared", "quiet", "unread",
                 "moves", "holders", "verdict"))
        for c in cands:
            print("  %-5d %-6d %-6d %-5d %-6d %-5d %-6d %-5d %-7d %s"
                  % (c.get("members", 0), c.get("frames", 0), c.get("judged", 0),
                     c.get("exclusive", 0), c.get("shared", 0), c.get("quiet", 0),
                     c.get("unreadable", 0), c.get("transfers", 0),
                     c.get("holders", 0),
                     "IDENTIFIED" if c.get("qualifies") else "refused"))
            for u in c.get("unmet") or []:
                print("        needs %s %s, has %s - %s"
                      % (u.get("requirement"), u.get("want"), u.get("have"),
                         u.get("why")))
        # AND WHAT THAT MEANS, named once rather than left to the reader. The mapping
        # is the one in ADR-156 and it is the whole point of having the counters.
        keys = {u.get("requirement") for c in cands for u in (c.get("unmet") or [])}
        if any(c.get("qualifies") for c in cands):
            print("  -> a list was identified. The sequence below is about it")
        elif "judged_frames" in keys and all(
                c.get("judged", 0) == 0 for c in cands):
            print("  -> nothing could be MEASURED. A capture or detector finding; the")
            print("     selection rule was never reached")
        elif "exclusive_marks" in keys:
            print("  -> no mark was ever VISIBLE in the shading. Either the screen")
            print("     marks its selection in a way this measurement does not see,")
            print("     or the screen is not a list - Experiment-025's pause menu")
        elif "transfers" in keys:
            print("  -> marked, but the mark never MOVED. The protocol, or member")
            print("     matching")
        elif "exclusive_over_shared" in keys:
            print("  -> several members marked at once. Correctly refused: that is a")
            print("     heads-up display, not one thing being chosen")
    # ── the slot trace: what each look brought ────────────────────────────────
    slots = list(stream(os.path.join(run, "slots.jsonl")))
    print("\nWHAT EACH LOOK BROUGHT  (%d slots in the trace)" % len(slots))
    if slots:
        rows = {}
        for s in slots:
            t = anchor + timedelta(milliseconds=s.get("at_ms", 0)) if anchor else None
            p = phase_of(t, ph)
            r = rows.setdefault(p, dict(n=0, valid=0, shaded=0, bare=0, shapes=0,
                                        regions=0, shapen=0))
            r["n"] += 1
            if s.get("outcome") != "valid":
                continue
            r["valid"] += 1
            regions = [x for x in (s.get("regions") or []) if x.get("shade")]
            shapes = [x for x in (s.get("shapes") or []) if x.get("shade")]
            r["regions"] += len(regions)
            r["shapen"] += len(shapes)
            if shapes:
                r["shapes"] += 1
            if regions or shapes:
                r["shaded"] += 1
            else:
                r["bare"] += 1
        print("  %-22s %6s %6s %8s %8s %9s %8s"
              % ("phase", "slots", "valid", "shaded", "bare", "w/shapes", "regions"))
        order = [n for n, _ in ph] + ["before"]
        for p in order:
            if p not in rows:
                continue
            r = rows[p]
            print("  %-22s %6d %6d %8d %8d %9d %8d"
                  % (p[:22], r["n"], r["valid"], r["shaded"], r["bare"], r["shapes"],
                     r["regions"] + r["shapen"]))
        total_shapes = sum(r["shapen"] for r in rows.values())
        print("\n  unnamed geometry preserved across the run: %d rectangles" % total_shapes)
        if total_shapes == 0:
            print("  ^^ ZERO. On a run after ADR-154 that is the finding, not a formatting")
            print("     detail: the selection reader needs these and got none.")

    # ── 6 + 7. what Director believed, and the four ways it saw nothing ───────
    ticks = list(stream(os.path.join(run, "ticks.jsonl")))
    print("\nWHAT DIRECTOR BELIEVED  (%d ticks)" % len(ticks))
    seen = []
    kinds = dict(sat_out=0, bare=0, no_list=0, declined=0, selected=0, ambiguous=0)
    for t in ticks:
        b = t.get("stats", {}).get("shadow", {}).get("interface")
        if not b:
            continue
        key = (b.get("status"), b.get("reading"), b.get("selected"),
               b.get("looks_unconfirmed", 0) > 0,
               b.get("looks_with_nothing_on_screen", 0) > 0)
        if not seen or seen[-1][0] != key:
            seen.append((key, b, iso(b.get("at"))))
        # The four states kept apart, which is the distinction asked for by name.
        if b.get("looks_unconfirmed", 0) > 0:
            kinds["sat_out"] += 1
        elif b.get("looks_with_nothing_on_screen", 0) > 0:
            kinds["bare"] += 1
        elif b.get("status") == "unknown":
            kinds["no_list"] += 1
        elif b.get("status") == "ambiguous":
            kinds["ambiguous"] += 1
        elif b.get("reading") == "selected":
            kinds["selected"] += 1
        else:
            kinds["declined"] += 1
    for key, b, at in seen:
        print("  %-22s %-12s %-22s sel %-3d  %s"
              % (phase_of(at, ph)[:22], b.get("status"), b.get("reading"),
                 b.get("selected", -1), (b.get("because") or "")[:70]))

    print("\n  WHY NOTHING WAS SELECTED, kept apart  (ticks, not looks)")
    print("    the detector sat out the slot      %4d   no evidence about the screen"
          % kinds["sat_out"])
    print("    looked, nothing shaded on screen   %4d   evidence the screen was bare"
          % kinds["bare"])
    print("    saw geometry, no list qualified     %4d   NEVER SAW a selection"
          % kinds["no_list"])
    print("    several lists qualified at once     %4d   declined between them"
          % kinds["ambiguous"])
    print("    a list was identified, reader said  %4d   SAW IT AND DECLINED"
          % kinds["declined"])
    print("    a list was identified and read      %4d   an entry was selected"
          % kinds["selected"])

    # ── 1 + 2. the selected-entry sequence, with times ────────────────────────
    # Merged across every tick, because the live journal is capped at 64 and an earlier
    # tick holds moments a later one has already trimmed.
    moments = {}
    for t in [final] + ticks:
        for m in t.get("stats", {}).get("shadow", {}).get("interface_moments", []) or []:
            moments[(m.get("sample"), m.get("at"), m.get("why"))] = m
    seq = sorted(moments.values(), key=lambda m: (iso(m.get("at")) or started, m.get("sample", 0)))
    print("\nTHE SELECTED-ENTRY SEQUENCE  (%d moments, merged across ticks)" % len(seq))
    if not seq:
        print("  none recorded")
    for m in seq:
        at = iso(m.get("at"))
        print("  %s  %-22s %-12s sel %-3d %s"
              % (hhmmss(at, started), phase_of(at, ph)[:22], m.get("status"),
                 m.get("selected", -1), m.get("why", "")))
    sel = [m for m in seq if m.get("selected", -1) >= 0]
    print("\n  entries actually reported as selected: %s"
          % (" -> ".join(str(m["selected"]) for m in sel) if sel else "NONE"))

    # ── 3 + 4. missed and false transitions ───────────────────────────────────
    events = []
    for e in sh.get("input_log", {}).get("events", []) or []:
        ev = e.get("event", {})
        t = anchor + timedelta(milliseconds=ev.get("at_ms", 0)) if anchor else None
        events.append((t, ev, e))
    nav = [(t, ev) for t, ev in [(t, ev) for t, ev, _ in events]
           if ev.get("intent") in ("up", "down", "left", "right")]
    print("\nPRESSES  (%d admitted, %d of them a direction; %d dropped by the bound)"
          % (len(events), len(nav), sh.get("input_log", {}).get("dropped", 0)))
    by_phase = {}
    for t, ev, _ in events:
        p = phase_of(t, ph)
        by_phase.setdefault(p, {}).setdefault(ev.get("intent"), 0)
        by_phase[p][ev.get("intent")] += 1
    for p in [n for n, _ in ph] + ["before"]:
        if p in by_phase:
            print("  %-22s %s" % (p[:22], dict(sorted(by_phase[p].items()))))

    # A MOVE IS A MOMENT THAT LANDED ON AN ENTRY, computed from `selected` rather than read
    # off a field. InterfaceMoment carries no "selection_moved" — settle() puts that in the
    # prose, and matching on prose would also catch "the selection can no longer be read",
    # which is a move to -1 and the opposite of a selection being followed.
    moves = []
    prev = None
    for m in seq:
        cur = m.get("selected", -1)
        if cur >= 0 and cur != prev:
            moves.append((iso(m.get("at")), m))
        prev = cur
    missed = [(t, ev) for t, ev in nav
              if not any(mt and t and t <= mt <= t + ATTRIBUTION_WINDOW for mt, _ in moves)]
    false = [(mt, m) for mt, m in moves
             if not any(t and mt and t <= mt <= t + ATTRIBUTION_WINDOW for t, _ in nav)]
    print("\nMISSED AND FALSE  (window %ds after a press)" % ATTRIBUTION_WINDOW.seconds)
    print("  directional presses with NO selection move after them   %d of %d"
          % (len(missed), len(nav)))
    print("  selection moves with NO press before them               %d of %d"
          % (len(false), len(moves)))
    if nav and not moves:
        print("  ^^ every press is 'missed' because no selection was ever read. That is one")
        print("     finding, not %d: read the belief table above for which of the four it is."
              % len(nav))
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(__doc__)
        sys.exit(2)
    sys.exit(main(sys.argv[1]))
