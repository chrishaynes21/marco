package main

// The Director's face, in a browser.
//
// # Why this exists beside the overlay
//
// The overlay is a strip on top of whatever you are doing, and it is deliberately small — it has
// to be, because it sits over the application being watched. That makes it the wrong place to
// read an account with six sections and a checklist in it. This is the same account with room to
// breathe: cards, headings, whitespace, and one accent colour reserved for the one thing that
// needs a person.
//
// # It renders; it decides nothing
//
// Every sentence below came from `pkg/playbill` through /api/playbill. There is no wording here,
// no threshold, no interpretation and no inference from timing. The page chooses colour, order
// and layout — which is the whole of what a presentation is allowed to choose.
//
// # Switching views changes nothing
//
// Normal, Watch and Debug are three renderings of one value. Moving between them re-renders what
// the page already has and asks for the next poll at a different depth; it starts no observation,
// resets no teaching, grants nothing, and creates no session.

const accountPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Marco</title>
<style>
:root{
  --bg:#0f1115; --card:#171a21; --edge:#242833; --ink:#e6e9ef; --dim:#8b93a4;
  --plain:#e6e9ef; --muted:#8b93a4; --good:#5fd08a; --doubt:#e3b341; --alarm:#f2686a;
  --accent:#6ea8fe; --accent-bg:#16233a;
}
@media (prefers-color-scheme:light){
  :root{--bg:#f6f7f9;--card:#fff;--edge:#e3e6ec;--ink:#12151b;--dim:#5d6577;
        --plain:#12151b;--muted:#5d6577;--good:#1a7f4b;--doubt:#8a6100;--alarm:#c02a2c;
        --accent:#1c5fd8;--accent-bg:#e8f0ff;}
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);
  font:15px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;}
header{position:sticky;top:0;z-index:5;background:var(--bg);
  border-bottom:1px solid var(--edge);padding:18px 24px 14px;}
.brand{font-size:13px;letter-spacing:.14em;text-transform:uppercase;color:var(--dim);
  margin-bottom:10px}
.headline{display:flex;align-items:baseline;gap:14px;flex-wrap:wrap}
.word{font-size:30px;font-weight:650;letter-spacing:-.01em}
.detail{color:var(--dim);font-size:15px}
.word.accent,.detail.accent{color:var(--accent)}
.word.good{color:var(--good)} .word.doubt{color:var(--doubt)} .word.alarm{color:var(--alarm)}
.word.muted{color:var(--dim)}
.attention{border-left:4px solid var(--accent);padding-left:14px;margin-left:-18px}
nav{display:flex;gap:6px;margin-top:14px}
nav button{background:transparent;border:1px solid var(--edge);color:var(--dim);
  border-radius:999px;padding:5px 15px;font:inherit;font-size:13px;cursor:pointer}
nav button[aria-pressed=true]{background:var(--accent-bg);border-color:var(--accent);
  color:var(--accent);font-weight:600}
nav button:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
main{padding:22px 24px 60px;max-width:1500px}
.grid{display:flex;flex-direction:column;gap:16px}
/* THINKING is the long one — it gets a column of its own so a reading three paragraphs
   deep does not push everything else off the bottom of the screen. */
.layout{display:grid;gap:16px;grid-template-columns:minmax(0,1fr) minmax(0,1fr);align-items:start}
@media (max-width:900px){.layout{grid-template-columns:1fr}}
.col{display:flex;flex-direction:column;gap:16px;min-width:0}
.col.main .card{max-height:calc(100vh - 230px);overflow-y:auto}
.card{background:var(--card);border:1px solid var(--edge);border-radius:12px;padding:16px 18px}
.card h2{margin:0 0 10px;font-size:11px;letter-spacing:.14em;text-transform:uppercase;
  color:var(--dim);font-weight:650}
.row{margin:3px 0;white-space:pre-wrap;overflow-wrap:anywhere}
.row.i1{margin-left:16px;padding-left:12px;border-left:2px solid var(--edge);color:var(--dim);
  font-size:14px}
.row.i2{margin-left:32px;padding-left:12px;border-left:2px solid var(--edge);color:var(--dim);
  font-size:14px}
.plain{color:var(--plain)} .muted{color:var(--muted)} .good{color:var(--good)}
.doubt{color:var(--doubt)} .alarm{color:var(--alarm)}
.accent{color:var(--accent);font-weight:600}
.card.teach{grid-column:1/-1;border-color:var(--accent);background:var(--accent-bg)}
.cue{font-size:22px;font-weight:700;color:var(--accent);margin:4px 0 12px}
.did{display:flex;gap:8px;flex-wrap:wrap;margin:10px 0}
.chip{background:var(--card);border:1px solid var(--edge);border-radius:8px;
  padding:4px 12px;font-size:14px;font-weight:600}
.chip.unknown{color:var(--doubt);border-color:var(--doubt)}
ol.steps{list-style:none;margin:12px 0 0;padding:0;display:flex;flex-wrap:wrap;gap:8px}
ol.steps li{display:flex;align-items:center;gap:7px;font-size:14px;color:var(--dim);
  background:var(--card);border:1px solid var(--edge);border-radius:999px;padding:4px 13px}
ol.steps li .mark{font-weight:700}
ol.steps li.done{color:var(--good)} ol.steps li.done .mark{color:var(--good)}
ol.steps li.current{color:var(--accent);border-color:var(--accent);font-weight:600}
ol.steps li.skipped{opacity:.55;text-decoration:line-through}
.card.question{grid-column:1/-1;border-color:var(--accent)}
.question .asks{font-size:18px;font-weight:600;margin-bottom:6px}
.known{border-top:1px solid var(--edge);padding-top:12px;margin-top:12px}
.answers{display:flex;gap:8px;margin-top:12px;flex-wrap:wrap}
.answers button,.namer button{background:var(--accent-bg);border:1px solid var(--accent);
  color:var(--accent);border-radius:8px;padding:8px 20px;font:inherit;font-weight:600;
  cursor:pointer}
.answers button.plainbtn,.namer button.plainbtn{background:transparent;border-color:var(--edge);
  color:var(--dim);font-weight:400}
.answers button:focus-visible,.namer button:focus-visible,.namer input:focus-visible{
  outline:2px solid var(--accent);outline-offset:2px}
.answers button[disabled],.namer button[disabled]{opacity:.45;cursor:progress}
.namer{display:flex;gap:8px;margin-top:12px;flex-wrap:wrap}
.namer input{flex:1;min-width:220px;background:var(--bg);border:1px solid var(--edge);
  border-radius:8px;padding:8px 12px;color:var(--ink);font:inherit}
.said{margin-top:10px;font-size:13px}
.stopbtn{margin-left:auto;background:transparent;border:1px solid var(--edge);
  color:var(--dim);border-radius:999px;padding:5px 15px;font:inherit;font-size:13px;cursor:pointer}
.stopbtn:hover{border-color:var(--alarm);color:var(--alarm)}
.offline{grid-column:1/-1;text-align:center;padding:44px 20px;color:var(--dim)}
footer{padding:0 24px 40px;color:var(--dim);font-size:12px}
footer code{background:var(--card);border:1px solid var(--edge);border-radius:5px;padding:1px 6px}
.stale{opacity:.5}
</style></head>
<body>
<header>
  <div class="brand">Marco</div>
  <div id="head" class="headline" role="status" aria-live="polite">
    <span class="word muted">Connecting…</span>
  </div>
  <nav aria-label="How much detail to show">
    <button class="stopbtn" id="stop" title="Stop whatever Marco is doing">Stop</button>
    <button id="v-sight" aria-pressed="false" title="Show what Marco can see">Sight</button>
    <button id="v-normal" aria-pressed="false">Normal</button>
    <button id="v-watch"  aria-pressed="true">Watch</button>
    <button id="v-debug"  aria-pressed="false">Debug</button>
  </nav>
</header>
<main>
  <div id="top" class="grid"></div>
  <div class="layout">
    <div id="col-main" class="col main"></div>
    <div id="col-side" class="col"></div>
  </div>
  <div id="sightbox"></div>
  <div id="knowsbox"></div>
  <div style="display:none">
  </div>
</main>
<footer id="foot"></footer>

<script>
// The page holds ONE piece of state: which reading is being shown. Everything else is
// whatever the last poll returned. There is no local model of what Marco is doing, because a
// second model is a model that can disagree with the Director.
let view = "watch";
let lastDigest = null, lastEpoch = null, failures = 0;

const el = id => document.getElementById(id);
const tone = t => t || "plain";

function setView(v){
  view = v;
  for (const b of ["normal","watch","debug"])
    el("v-"+b).setAttribute("aria-pressed", String(b === v));
  lastDigest = null;            // force one repaint at the new depth
  poll();
}
el("v-normal").onclick = () => setView("normal");
el("v-watch").onclick  = () => setView("watch");
el("v-debug").onclick  = () => setView("debug");

// Keyboard: 1/2/3 switch depth. Deliberately plain digits and no modifiers — a global
// shortcut with a modifier is one that could reach the application being taught.
addEventListener("keydown", e => {
  if (e.target.tagName === "INPUT" || e.metaKey || e.ctrlKey || e.altKey) return;
  if (e.key === "1") setView("normal");
  if (e.key === "2") setView("watch");
  if (e.key === "3") setView("debug");
});

function headline(a){
  const h = a.headline || {};
  const box = el("head");
  box.className = "headline" + (h.attention ? " attention" : "");
  box.innerHTML = "";
  const w = document.createElement("span");
  w.className = "word " + tone(h.tone);
  w.textContent = h.word || "";
  box.appendChild(w);
  if (h.detail){
    const d = document.createElement("span");
    d.className = "detail" + (h.attention ? " accent" : "");
    d.textContent = h.detail;
    box.appendChild(d);
  }
}

function card(title, build){
  const c = document.createElement("section");
  c.className = "card";
  if (title){
    const h = document.createElement("h2");
    h.textContent = title;
    c.appendChild(h);
  }
  build(c);
  return c;
}

function lineRows(c, lines){
  for (const l of lines){
    const p = document.createElement("div");
    p.className = "row " + tone(l.tone) + (l.indent ? " i" + Math.min(l.indent,2) : "");
    p.textContent = l.text;
    c.appendChild(p);
  }
}

// The teaching panel. Given its own card, at full width, above everything else — a person
// who asked to teach something is waiting for a cue, and the cue must not be a line in a list.
function teachingCard(t){
  const c = card("Teaching" + (t.asked ? " — " + t.asked : ""), c => {
    if (t.armed){
      const cue = document.createElement("div");
      cue.className = "cue";
      cue.textContent = "SHOW ME — I'm watching this example now.";
      c.appendChild(cue);
    }
    if (t.because){
      const b = document.createElement("div");
      b.className = "row " + (t.armed ? "accent" : "plain");
      b.textContent = t.because;
      c.appendChild(b);
    }
    // What Marco believes you did. The empty case is SPLIT: nothing yet, versus a change
    // nobody could attribute. Only the second draws anything.
    if ((t.did && t.did.length) || t.unattributed){
      const label = document.createElement("div");
      label.className = "row muted";
      label.textContent = "You did";
      c.appendChild(label);
      const box = document.createElement("div");
      box.className = "did";
      if (t.did && t.did.length){
        for (const d of t.did){
          const s = document.createElement("span");
          s.className = "chip";
          s.textContent = d;
          box.appendChild(s);
        }
      } else {
        const s = document.createElement("span");
        s.className = "chip unknown";
        s.textContent = "?";
        box.appendChild(s);
      }
      c.appendChild(box);
      if (t.unattributed){
        const w = document.createElement("div");
        w.className = "row doubt i1";
        w.textContent = "I saw the screen change, but I couldn't tell what you did.";
        c.appendChild(w);
      }
    }
    if (t.progress && t.progress.length){
      const ol = document.createElement("ol");
      ol.className = "steps";
      const mark = {done:"✓", current:"●", skipped:"–", pending:"○"};
      for (const s of t.progress){
        const li = document.createElement("li");
        li.className = s.state;
        if (s.state === "current") li.setAttribute("aria-current","step");
        const m = document.createElement("span");
        m.className = "mark";
        m.textContent = mark[s.state] || "○";
        li.appendChild(m);
        li.appendChild(document.createTextNode(s.name));
        ol.appendChild(li);
      }
      c.appendChild(ol);
    }
    if (t.learned){
      const d = document.createElement("div");
      d.className = "row good";
      d.textContent = "Learned “" + t.learned + "”.";
      c.appendChild(d);
      if (!t.registered){
        const n = document.createElement("div");
        n.className = "row muted i1";
        n.textContent = "Nothing can ask for it yet.";
        c.appendChild(n);
      }
    } else if (t.stopped){
      const d = document.createElement("div");
      d.className = "row doubt";
      d.textContent = "Stopped. Nothing was written down.";
      c.appendChild(d);
    }
  });
  c.classList.add("teach");
  return c;
}

// The question. The page shows the ordinary command that answers it — it does not answer
// anything itself, because the answer paths are Director's and there is no fourth one.
function questionCard(q){
  const c = card("Marco asks", c => {
    const a = document.createElement("div");
    a.className = "asks accent";
    a.textContent = q.asks;
    c.appendChild(a);
    const b = document.createElement("div");
    b.className = "row muted";
    if (q.about) {
      b.textContent = "about " + q.about;
    } else {
      // Marco genuinely cannot point at this one. A question about a group of controls
      // has no screen and no route to name, and coordinates deliberately never reach a
      // presentation — so saying nothing would leave a person guessing what "they" are.
      // The evidence for the claim is in THINKING, below, and it says where it saw them.
      b.textContent = "Marco can2019t point at this one 2014 see THINKING below for what "
        + "it saw and where. If you can2019t tell, 201cNot now201d is the honest answer.";
    }
    c.appendChild(b);
    // POINTING at what the question is about. The one thing that turns "do these controls
    // belong together?" from unanswerable into answerable: "these" is a session-local id
    // nobody can see, and a person who cannot inspect the referent cannot supervise the
    // judgement — which is how a wrong answer became durable in the first place.
    //
    // Offered whatever the answer turns out to be. When Marco cannot point, the sentence that
    // comes back says so, which is more honest than hiding the button and leaving somebody to
    // wonder whether they missed a highlight.
    const pointed = document.createElement("div");
    pointed.className = "said muted";
    const showBtn = document.createElement("button");
    showBtn.textContent = "Show me what this refers to";
    showBtn.onclick = async () => {
      showBtn.disabled = true;
      // BY NAME. The generic path answers "what is Marco referring to" and deliberately
      // skips a screen-shaped subject so it does not consume the choice — so under a
      // question about a screen it would highlight some other group beneath this sentence.
      // Two subjects, one box, no way to tell. Found live on Windows Settings.
      const r = await sendRead("/api/point", {question: q.id});
      pointed.textContent = r.shown
        ? "Pointing at it on your screen now."
        : (r.said || "I can't point at that right now.");
      showBtn.disabled = false;
    };
    const showRow = document.createElement("div");
    showRow.className = "row";
    showRow.appendChild(showBtn);
    c.appendChild(showRow);
    c.appendChild(pointed);
    // The answer controls. They POST to /api/answer and /api/name, which shell out to the
    // ordinary 'director answer' and 'director name-screen' commands — the same journey a
    // person's typing makes at a terminal. There is no shorter path from this page, and a
    // button that had one would be a second authority route.
    const said = document.createElement("div");
    said.className = "said muted";

    if (q.wants === "name"){
      const form = document.createElement("form");
      form.className = "namer";
      const input = document.createElement("input");
      input.type = "text";
      input.placeholder = "what you call this screen";
      input.setAttribute("aria-label", "What you call this screen");
      input.maxLength = 60;
      const go = document.createElement("button");
      go.type = "submit";
      go.textContent = "Save name";
      form.append(input, go);
      form.onsubmit = async e => {
        e.preventDefault();
        const name = input.value.trim();
        if (!name) return;
        go.disabled = true;
        said.textContent = await send("/api/name", {id: q.id, name});
        go.disabled = false;
      };
      c.appendChild(form);
    } else {
      const box = document.createElement("div");
      box.className = "answers";
      // Exactly the answers the account offered, in its own words. A presentation offers
      // these and nothing else — there is no fourth button to invent.
      // "Not sure" and not "Not now". They are different sentences: one is "I cannot tell",
      // the other is "I am busy", and only the first is what a person means when they
      // cannot see what is being asked about. Mislabelling it is how an honest abstention
      // becomes a durable "no".
      const label = {confirmed:"Yes", contradicted:"No", declined:"Not sure"};
      for (const ans of (q.answers||[])){
        const b = document.createElement("button");
        b.textContent = label[ans] || ans;
        if (ans !== "confirmed") b.className = "plainbtn";
        b.onclick = async () => {
          for (const other of box.querySelectorAll("button")) other.disabled = true;
          said.textContent = await send("/api/answer", {id: q.id, response: ans});
          for (const other of box.querySelectorAll("button")) other.disabled = false;
        };
        box.appendChild(b);
      }
      c.appendChild(box);
    }
    c.appendChild(said);
  });
  c.classList.add("question");
  return c;
}

// send posts an answer and reports what the Director said back.
//
// The reply is shown verbatim. An answer that the service refused — because it was already
// answered, or the question has gone — must read as a refusal rather than as a silent success,
// and the page is in no position to interpret which.
// sendRead posts and returns the parsed answer.
//
// Separate from send(), which is for things that CHANGE something: that one repaints the whole page
// and flattens the reply to a short confirmation. A read must do neither — repainting on every
// "show me" would make the list jump under the pointer, and the reply here is a sentence Marco
// wrote rather than a receipt.
async function sendRead(path, body){
  try {
    const r = await fetch(path, {method:"POST", headers:{"Content-Type":"application/json"},
                                body: JSON.stringify(body)});
    if (!r.ok) return null;
    return await r.json();
  } catch (e) {
    return null;
  }
}

async function send(path, body){
  try {
    const r = await fetch(path, {method:"POST", headers:{"Content-Type":"application/json"},
                                body: JSON.stringify(body)});
    if (!r.ok) return "Marco didn't accept that: " + (await r.text()).trim();
    const out = await r.json();
    lastDigest = null;                       // the answer changed something; repaint
    poll();
    return out.ok ? "Noted." : ("Marco didn't accept that: " + (out.output||"").trim());
  } catch (e) {
    return "Couldn't reach Marco.";
  }
}

// THINKING gets a column to itself.
//
// It is the section that grows without bound — one reading carries a claim, its evidence, every
// contradiction and what would settle it — and in one flow it pushed everything a person actually
// needed off the bottom of the screen. The rest goes beside it, where a glance finds it.
const OWN_COLUMN = "THINKING";

function render(a){
  headline(a);
  const top = el("top"), main = el("col-main"), side = el("col-side");
  top.innerHTML = main.innerHTML = side.innerHTML = "";

  if (a.reach !== "present"){
    const d = document.createElement("div");
    d.className = "offline";
    d.textContent = a.why || "The Director isn't running.";
    top.appendChild(d);
    return;
  }
  // Anything that needs a person goes across the top, above both columns. Nobody should have
  // to find a question in a column.
  if (a.teaching) top.appendChild(teachingCard(a.teaching));
  if (a.question) top.appendChild(questionCard(a.question));

  let any = top.children.length > 0;
  for (const s of (a.sections||[])){
    // A section is dropped when a panel above is already showing the same facts. One or
    // the other, never the same thing twice in two shapes — a question rendered once as a
    // button and again as a sentence is a person wondering which one is real.
    if (a.teaching && s.title === "TEACHING") continue;
    if (a.question && s.title === "MARCO ASKS") continue;
    const c = card(s.title || "Right now", c => lineRows(c, s.lines));
    (s.title === OWN_COLUMN ? main : side).appendChild(c);
    any = true;
  }
  if (!any){
    const d = document.createElement("div");
    d.className = "offline";
    d.textContent = a.why || "Nothing to report yet.";
    top.appendChild(d);
  }
}

async function poll(){
  try {
    const r = await fetch("/api/playbill?view=" + view, {cache:"no-store"});
    const a = await r.json();
    failures = 0;
    document.body.classList.remove("stale");
    // A restart voids everything. Distinct from a quiet moment, and a page that confused
    // them would show stale certainty across it.
    if (a.epoch && lastEpoch && a.epoch !== lastEpoch) lastDigest = null;
    lastEpoch = a.epoch || lastEpoch;
    // Hold still while nothing a person would notice has changed. The digest deliberately
    // excludes clocks and sample counts, so an unchanging screen does not repaint.
    if (a.digest && a.digest === lastDigest) { foot(a); return; }
    lastDigest = a.digest || null;
    render(a);
    foot(a);
  } catch (e) {
    if (++failures > 2) document.body.classList.add("stale");
  }
}

function foot(a){
  el("foot").textContent =
    "Reading state the Director already holds — this starts no observation and takes no sample. " +
    "Keys 1 / 2 / 3 switch depth." + (a.epoch ? "  ·  " + a.epoch : "");
}

// ── What Marco Knows ──────────────────────────────────────────────────────────
//
// Its own box, its own poll, and deliberately outside render(): the account above repaints only
// when the Director's digest changes, and a person's own judgements are not part of that digest.
// Hanging this off it would mean a correction sat on screen uncorrected until something else moved.
//
// This box decides nothing. Every sentence in it — what you said, what it was about, whether Marco
// can still locate it — arrived from the Director through /api/knows.
let known = [];
let knownKey = "";
let knownNote = "";

function knowsRow(k){
  const d = document.createElement("div");
  d.className = "known";

  const said = document.createElement("div");
  said.className = "row accent";
  said.textContent = k.said;
  d.appendChild(said);

  if (k.called){
    const n = document.createElement("div");
    n.className = "row plain";
    n.textContent = "You call this screen “" + k.called + "”.";
    d.appendChild(n);
  }
  const about = document.createElement("div");
  about.className = "row muted";
  about.textContent = k.about;
  d.appendChild(about);

  // The distinction this whole surface exists for. Marco remembering what you said and Marco
  // being able to point at what you said it about are different things, and a page that quietly
  // conflated them would offer to show you something nothing can see.
  if (!k.locatable){
    const w = document.createElement("div");
    w.className = "row muted";
    w.textContent = "Marco remembers your answer but can't currently locate what it referred to.";
    d.appendChild(w);
  }

  const bar = document.createElement("div");
  bar.className = "answers";

  // Offered ONLY when a live session currently recognises the subject. The button is absent
  // rather than disabled when it is not: a greyed-out "show me" invites somebody to keep
  // clicking at something that is never going to appear.
  if (k.locatable){
    const s = document.createElement("button");
    s.textContent = "Show me what this refers to";
    s.onclick = async () => {
      const r = await sendRead("/api/showme", {subject: k.subject});
      knownNote = (r && r.said) ? r.said : "";
      renderKnows();
    };
    bar.appendChild(s);
  }

  const opposite = (k.judgement === "contradicted") ? "yes" : "no";
  const label = (k.judgement === "contradicted")
    ? "Change to yes" : "Change to no";
  for (const [text, change] of [[label, opposite],
                                ["I'm not sure", "not-sure"],
                                ["Withdraw", "withdraw"]]){
    const b = document.createElement("button");
    b.textContent = text;
    b.onclick = async () => {
      knownNote = await send("/api/correct",
        {subject: k.subject, kind: k.kind, change: change});
      knownKey = "";                          // whatever it is now, re-read it
      pollKnows();
    };
    bar.appendChild(b);
  }
  d.appendChild(bar);
  return d;
}

function renderKnows(){
  const box = el("knowsbox");
  box.innerHTML = "";
  if (!known.length && !knownNote) return;
  box.appendChild(card("What Marco Knows", c => {
    if (!known.length){
      const p = document.createElement("div");
      p.className = "row muted";
      p.textContent = "Nothing here. Marco records something when you answer a question " +
                      "about what part of an application means.";
      c.appendChild(p);
    }
    let app = "";
    for (const k of known){
      if (k.application !== app){
        app = k.application;
        const h = document.createElement("div");
        h.className = "row plain";
        h.textContent = app.toUpperCase();
        c.appendChild(h);
      }
      c.appendChild(knowsRow(k));
    }
    if (knownNote){
      const n = document.createElement("div");
      n.className = "row muted";
      n.textContent = knownNote;
      c.appendChild(n);
    }
  }));
}

// ── Show Sight ────────────────────────────────────────────────────────────────
//
// A toggle over a READ. Turning it on starts nothing, answers nothing and writes nothing, so a
// person can leave it on while Marco is asking them something — which is exactly when knowing
// what Marco can see is worth having.
//
// Normal stays calm: four lines a person can act on. Debug adds the provenance, because "which
// evidence was this geometry segmented from" is a developer's question and putting it in front of
// everybody teaches people to distrust the calm reading.
let sightOn = false, sight = null;

function sightRow(label, value, dim){
  const r = document.createElement("div");
  r.className = "row" + (dim ? " muted" : "");
  const l = document.createElement("span");
  l.className = "word muted";
  l.textContent = label;
  const v = document.createElement("span");
  v.textContent = value;
  r.appendChild(l); r.appendChild(v);
  return r;
}

function renderSight(){
  const box = el("sightbox");
  box.innerHTML = "";
  if (!sightOn) return;
  box.appendChild(card("What I'm seeing", c => {
    if (!sight || sight.reach !== "present"){
      c.appendChild(sightRow("", "Marco isn't reachable, so it can't tell you what it sees.",
        true));
      return;
    }
    c.appendChild(sightRow("watching", sight.watching || "the application in front"));
    // Named sources with their real state, never a summary word. "Perceiving: yes" would be
    // true of a Director reading a tree and of one reading pixels, and those are different
    // claims about how much to trust a highlight.
    const parts = (sight.sources||[]).map(s => s.name + " " + (s.on ? "on" : "off"));
    c.appendChild(sightRow("perceiving", parts.join(", ") || "unknown"));
    if (view === "debug"){
      for (const s of (sight.sources||[])){
        if (!s.on && s.reason) c.appendChild(sightRow("", s.name + " is off: " + s.reason, true));
      }
    }
    c.appendChild(sightRow("the place",
      sight.place || "I haven't settled on what this screen is"));
    // What is AVAILABLE and what this referent was actually read FROM are different facts.
    // A panel showing only the first lets "ocr on" be read as "Marco looked at the pixels".
    if (sight.grounding && sight.grounding !== "none"){
      c.appendChild(sightRow("grounding on", sight.grounding));
    }
    c.appendChild(sightRow("referring to", sight.about || sight.say));
    if (sight.question) c.appendChild(sightRow("the question", sight.question));
    else if (sight.interpretation) c.appendChild(sightRow("what I think", sight.interpretation));

    // Pointing is offered only when Marco can actually do it. A button that produced nothing
    // would teach people that the highlight means less than it does.
    if (!sight.locatable) return;
    const said = document.createElement("div");
    said.className = "said muted";
    const b = document.createElement("button");
    b.textContent = "Show me what you mean";
    b.onclick = async () => {
      b.disabled = true;
      const r = await sendRead("/api/point", {});
      said.textContent = r.shown
        ? "Pointing at it on your screen now."
        : (r.said || "I can't point at that right now.");
      b.disabled = false;
    };
    const row = document.createElement("div");
    row.className = "row";
    row.appendChild(b);
    c.appendChild(row);
    c.appendChild(said);
  }));
}

async function pollSight(){
  if (!sightOn) return;
  try {
    const r = await fetch("/api/sight?view=" + view, {cache:"no-store"});
    sight = await r.json();
  } catch (e) { sight = {reach:"absent"}; }
  renderSight();
}

async function pollKnows(){
  try {
    const r = await fetch("/api/knows", {cache:"no-store"});
    const v = await r.json();
    const next = v.known || [];
    const key = JSON.stringify(next);
    if (key === knownKey) return;             // hold still while nothing has changed
    knownKey = key;
    known = next;
    renderKnows();
  } catch (e) { /* the account above already says the Director is unreachable */ }
}

el("v-sight").onclick = () => {
  sightOn = !sightOn;
  el("v-sight").setAttribute("aria-pressed", sightOn ? "true" : "false");
  renderSight();
  pollSight();
};

setInterval(poll, 700);
poll();
setInterval(pollKnows, 2000);
pollKnows();
setInterval(pollSight, 2000);
</script>
</body></html>`
