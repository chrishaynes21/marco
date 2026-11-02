// PROPOSAL OVERLAY — applied on top of the production page after it has painted from the
// fixtures. It only restyles, moves, hides and relabels existing elements; it decides
// nothing. Used for the "after" renders in the audit. Not production code.
window.__proposal = function(parts){
  const $ = id => document.getElementById(id)
  const css = document.createElement('style')
  css.textContent = `
  /* P1 — the library is the screen */
  #view-routines > .sub, #view-routines > h2 { display:none !important }
  .p-head { display:flex; align-items:center; gap:12px; margin:0 0 18px }
  .p-head h1 { margin:0; flex:1 }
  .p-find { display:flex; gap:8px; align-items:center; margin:0 0 6px }
  .p-find #askBox { flex:1; min-width:0 }
  #askCard.p-flat { background:none; border:0; box-shadow:none; padding:0; margin:0 0 4px }
  #routinesCard.p-flat { background:none; border:0; box-shadow:none; padding:0 }
  #readyMachine, #readyLimits, #libWhere { display:none !important }
  .pill.ok { display:none !important }
  .p-quiet.go.big { background:none; color:var(--spot); border:1px solid var(--spot-dim);
     font:600 15px var(--text); min-height:42px; padding:8px 22px; border-radius:var(--r) }
  .p-quiet.go.big:hover:not(:disabled) { background:var(--spot-bg); color:var(--spot); border-color:var(--spot) }
  .card.pick { padding:9px 10px }
  .card.pick h3 { font-size:15.5px }
  .p-recagain { white-space:nowrap }
  /* P2 — the open routine: run strip + tabs */
  .p-strip { display:flex; flex-wrap:wrap; gap:8px 14px; align-items:center; margin:2px 0 4px }
  .p-strip .p-f { display:flex; gap:7px; align-items:center }
  .p-strip label { color:var(--dim); font-size:13px }
  .p-strip select { min-width:0; max-width:320px }
  .p-strip input[type=text] { width:14em; min-height:var(--ctl) }
  .p-icon { min-width:var(--ctl); padding:0 9px; font-size:15px }
  .p-says { color:var(--dim); font-size:12.5px; margin:2px 0 0 }
  .p-says .hint { font-size:12.5px }
  .p-tabs { display:flex; flex-wrap:wrap; gap:2px; border-top:1px solid var(--line); margin-top:14px; padding-top:8px }
  .p-tabs button { background:none; border:0; color:var(--dim); font-weight:500; min-height:30px; padding:4px 10px }
  .p-tabs button:hover:not(:disabled) { background:var(--raise); color:var(--ink) }
  .p-tabs .p-more { margin-left:auto }
  /* P3 — review: the steps are the thing being approved */
  .p-meta { color:var(--dim); font-size:13px; margin:0 0 4px }
  .p-alert { border-left:3px solid var(--no); background:var(--no-bg); border-radius:0 var(--r) var(--r) 0;
     padding:10px 14px; margin:6px 0 10px; display:flex; gap:12px; align-items:center; flex-wrap:wrap }
  .p-alert .p-t { flex:1 1 20em; min-width:0 }
  .p-alert b { color:var(--no) }
  .p-foot { display:flex; gap:9px; align-items:center; flex-wrap:wrap; margin-top:14px;
     border-top:1px solid var(--line); padding-top:12px }
  .p-foot .p-sp { flex:1 }
  .p-ghost { background:none; border-color:transparent; color:var(--dim) }
  .p-note { color:var(--dim); font-size:12.5px; margin-top:8px }
  `
  document.head.appendChild(css)

  if (parts.indexOf('library') >= 0) {
    const view = $('view-routines'), h1 = view.querySelector('h1')
    const head = document.createElement('div'); head.className = 'p-head'
    view.insertBefore(head, h1); head.appendChild(h1)
    const rec = $('recordRevealBtn'); rec.textContent = '●  Record'; head.appendChild(rec)
    const help = document.createElement('button'); help.className = 'p-icon'; help.textContent = '?'
    help.setAttribute('aria-label', 'How recording works'); head.appendChild(help)
    $('recordStart').style.display = 'none'
    const ask = $('askCard'); ask.classList.add('p-flat')
    $('askBox').placeholder = 'Find a routine, or say what you want done'
    // the list follows the find box directly
    view.insertBefore($('routinesCard'), $('readyCard'))
    view.insertBefore(ask, $('routinesCard'))
    $('routinesCard').classList.add('p-flat')
    document.querySelectorAll('[id^=routinePlay-]').forEach(b => {
      b.classList.add('p-quiet'); b.textContent = '▶  Play'
      if (b.disabled) {
        const again = document.createElement('button'); again.className = 'p-recagain'
        again.textContent = 'Record again'; b.replaceWith(again)
      }
    })
    document.querySelectorAll('.rowsays.no').forEach(s => {
      s.textContent = s.textContent.replace(/^The recording is missing (.*), so every press refuses\.$/, 'Not measured when recorded: $1.')
    })
  }

  if (parts.indexOf('open') >= 0) {
    const panel = $('routineChosen')
    ;['routineAskedFor', 'routineSecondLook', 'routineStepsRow', 'routineAnswersBox', 'routineWhereRow']
      .forEach(id => { const e = $(id); if (e) e.style.display = 'none' })
    const checksRow = $('routineChecks') && $('routineChecks').parentElement
    if (checksRow) checksRow.style.display = 'none'
    panel.querySelectorAll(':scope > details').forEach(d => d.style.display = 'none')
    // one strip: where it presses, what it needs
    const strip = document.createElement('div'); strip.className = 'p-strip'
    const into = $('routineIntoRow'); into.style.display = 'none'
    const f1 = document.createElement('span'); f1.className = 'p-f'
    const l1 = document.createElement('label'); l1.textContent = 'Presses in'; l1.htmlFor = 'routineInto'
    const ref = $('routineIntoAgainBtn'); ref.textContent = '↻'; ref.classList.add('p-icon')
    ref.setAttribute('aria-label', 'Refresh the window list')
    f1.append(l1, $('routineInto'), ref); strip.appendChild(f1)
    const args = $('routineArgsBox')
    if (args && !args.classList.contains('hide')) {
      args.style.display = 'none'
      args.querySelectorAll('#routineArgsList .row').forEach(row => {
        const f = document.createElement('span'); f.className = 'p-f'
        const lab = row.querySelector('label'); const box = row.querySelector('input')
        if (lab) { lab.style.width = ''; lab.textContent = lab.textContent; f.appendChild(lab) }
        if (box) { box.placeholder = 'required'; f.appendChild(box) }
        strip.appendChild(f)
      })
    }
    panel.insertBefore(strip, into)
    const says = $('routineIntoSays'); says.style.display = 'none'
    const short = document.createElement('div'); short.className = 'p-says'
    short.innerHTML = 'Brings it forward first · every press is checked against this window ' +
      '<button class="hint">What is checked</button>'
    strip.after(short)
    const tabs = document.createElement('div'); tabs.className = 'p-tabs'; tabs.setAttribute('role', 'toolbar')
    ;['Steps', 'Other names (1)', 'Where it works', 'Source'].forEach(t => {
      const b = document.createElement('button'); b.textContent = t; tabs.appendChild(b) })
    const more = document.createElement('button'); more.className = 'p-more'; more.textContent = 'Simplify…  ·  Record again  ▾'
    tabs.appendChild(more)
    panel.appendChild(tabs)
    const wants = $('routineWants'); if (wants) { wants.style.flex = '1 0 100%'; wants.style.fontSize = '12.5px'; short.after(wants) }
  }

  if (parts.indexOf('review') >= 0) {
    const card = $('capReview')
    const said = $('capSaid'); const where = $('capWhere')
    const meta = document.createElement('div'); meta.className = 'p-meta'
    const m = (said.textContent.match(/(\d+) step\(s\), (\d+)/) || [])
    const app = (where.textContent.match(/recorded in (\S+)/) || [])[1] || ''
    meta.textContent = (m[1] || '?') + ' steps · ' + (m[2] || '?') + ' press something · recorded in ' +
      app + ' · runs there, bringing it forward'
    said.replaceWith(meta); where.style.display = 'none'
    $('capBrings').style.display = 'none'
    const wont = $('capWontRunBox')
    if (!wont.classList.contains('hide')) {
      wont.style.display = 'none'
      const alert = document.createElement('div'); alert.className = 'p-alert'
      alert.innerHTML = '<div class="p-t"><b>Won’t run as recorded.</b> ' + $('capMissing').textContent.replace(/ Record it again.*$/, '') +
        '</div><button class="go">Record again</button>'
      meta.after(alert)
    }
    const exit = $('capExit'); const steps = $('capSteps')
    const notes = document.createElement('div'); notes.className = 'p-note'
    notes.textContent = [exit.classList.contains('hide') ? '' : 'The alt+tab you used to come back was left out.',
      window.__away || ''].filter(Boolean).join(' ')
    exit.style.display = 'none'
    steps.after(notes)
    const simp = $('capSimplifyRow'); simp.style.display = 'none'
    const saveRow = $('capSaveBtn').parentElement; saveRow.classList.add('p-foot')
    const save = $('capSaveBtn'); const cancel = saveRow.querySelector('.stop')
    cancel.textContent = 'Discard'; cancel.classList.add('p-ghost'); cancel.classList.remove('stop')
    const sb = $('capSimplifyBtn'); sb.textContent = 'Simplify…'; sb.title = 'Shortens the waits. You see the result before anything is saved.'
    const sp = document.createElement('span'); sp.className = 'p-sp'
    saveRow.insertBefore(sb, save); saveRow.insertBefore(sp, save)
    if (!wont.classList.contains('hide')) { save.className = ''; save.textContent = 'Save anyway' }
    cancel.className = 'p-ghost'; cancel.style.cssText = 'background:none;border-color:transparent;color:var(--dim)'
    // while a recording is under review, the library is not the task
    document.querySelector('#view-routines > h2:last-of-type') // (hidden by P1 css when combined)
    $('routinesCard').style.display = 'none'
    document.querySelectorAll('#view-routines > h2').forEach(h => { if (/routines/i.test(h.textContent)) h.style.display = 'none' })
  }
}
