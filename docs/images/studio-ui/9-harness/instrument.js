// AUDIT INSTRUMENT — injected before the stub. Captures page errors and measures
// visible text when the state script calls window.__measure().
window.__errors = []
window.addEventListener('error', e => window.__errors.push('error: ' + (e.message || e)))
window.addEventListener('unhandledrejection', e => window.__errors.push('rejection: ' + (e.reason && (e.reason.message || e.reason))))
;(function(){
  const ce = console.error
  console.error = function(){ window.__errors.push('console.error: ' + Array.from(arguments).join(' ')); return ce.apply(console, arguments) }
})()
window.__click = (id) => { const el = document.getElementById(id); if (!el) { window.__errors.push('audit: no element #' + id); return } el.click() }
window.__type = (id, v) => { const el = document.getElementById(id); if (!el) { window.__errors.push('audit: no element #' + id); return } el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); el.dispatchEvent(new Event('change', {bubbles: true})) }
window.__scrollTo = (id, offset) => { const el = document.getElementById(id); const m = document.querySelector('main'); if (el && m) { m.scrollTop = Math.max(0, el.getBoundingClientRect().top + m.scrollTop - m.getBoundingClientRect().top - (offset || 0)) } }
window.__measure = function(){
  const visible = el => {
    for (let e = el; e && e !== document.body; e = e.parentElement) {
      const cs = getComputedStyle(e)
      if (cs.display === 'none' || cs.visibility === 'hidden') return false
      if (e.tagName === 'DETAILS' && !e.open) {
        // only the summary of a closed details is visible
        if (!el.closest('summary') || el.closest('details') !== e) return false
      }
    }
    return el.getClientRects().length > 0
  }
  const scope = document.querySelector('main')
  const rail = document.getElementById('rail')
  const blocks = []
  const walk = (root, where) => {
    const tw = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
    let n
    while ((n = tw.nextNode())) {
      const t = n.textContent.replace(/\s+/g, ' ').trim()
      if (!t) continue
      const p = n.parentElement
      if (!p || p.closest('script,style')) continue
      if (!visible(p)) continue
      const owner = p.closest('[id]')
      let kind = 'text'
      if (p.closest('button,summary,select,option,label')) kind = 'control'
      if (p.closest('h1,h2,h3')) kind = 'heading'
      if (p.closest('.banner')) kind = 'banner'
      if (p.closest('.muted') && kind === 'text') kind = 'muted'
      if (p.closest('.no,.wontrun') && kind === 'text') kind = 'refusal'
      if (p.closest('pre,.mach,.val,.what,.act,.lbl')) kind = kind === 'control' ? kind : 'value'
      blocks.push({where, id: owner ? owner.id : '', kind, t})
    }
  }
  walk(rail, 'rail'); walk(scope, 'main')
  const words = s => s.split(/\s+/).filter(w => /[A-Za-z0-9]/.test(w)).length
  const byKind = {}, byId = {}
  let total = 0
  blocks.forEach(b => { const w = words(b.t); total += w; byKind[b.kind] = (byKind[b.kind] || 0) + w; byId[b.id] = (byId[b.id] || 0) + w })
  const count = sel => Array.from(document.querySelectorAll(sel)).filter(visible).length
  const ctlNoName = Array.from(document.querySelectorAll('main button, main input, main select')).filter(visible).filter(el => {
    const name = (el.getAttribute('aria-label') || el.textContent || '').trim() || (el.id && document.querySelector('label[for="' + el.id + '"]')) || el.closest('label') || el.placeholder
    return !name
  }).map(el => el.id || el.outerHTML.slice(0, 60))
  const doc = document.documentElement
  const m = document.querySelector('main')
  const out = {
    total, byKind, byId, blocks,
    buttons: count('main button'), primary: count('main button.go'), inputs: count('main input,main select'),
    cards: count('main .card'), nestedCards: count('main .card .card'), banners: count('main .banner'),
    headings: count('main h1,main h2,main h3'), hints: count('main .hint'), details: count('main details'),
    unnamedControls: ctlNoName,
    hscroll: m ? (m.scrollWidth > m.clientWidth + 1) : false,
    mainScrollHeight: m ? m.scrollHeight : 0, viewport: innerWidth + 'x' + innerHeight,
    errors: window.__errors,
  }
  doc.setAttribute('data-audit', JSON.stringify(out))
}
