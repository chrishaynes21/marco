// S10 CONTRAST AUDIT — injected into a rendered Studio page. Measures every visible piece of
// text against the background it is actually drawn on: the backgrounds of its ancestors
// composited in order, and a parent's opacity (a disabled button is drawn at .42) applied to both
// the text and what is behind it. Results go on <html data-contrast>. A render, not a desktop.
window.__contrast = function(){
  const parse = s => { const m = s.match(/rgba?\(([^)]+)\)/); if (!m) return null; const p = m[1].split(/[ ,\/]+/).filter(Boolean).map(Number); return {r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1} }
  const over = (top, under) => ({r: top.r * top.a + under.r * (1 - top.a), g: top.g * top.a + under.g * (1 - top.a), b: top.b * top.a + under.b * (1 - top.a), a: 1})
  const mix = (a, b, t) => ({r: a.r + (b.r - a.r) * t, g: a.g + (b.g - a.g) * t, b: a.b + (b.b - a.b) * t, a: 1})
  const lum = c => { const f = v => { v /= 255; return v <= .03928 ? v / 12.92 : Math.pow((v + .055) / 1.055, 2.4) }; return .2126 * f(c.r) + .7152 * f(c.g) + .0722 * f(c.b) }
  const ratio = (x, y) => { const a = lum(x), b = lum(y); return (Math.max(a, b) + .05) / (Math.min(a, b) + .05) }
  const hex = c => '#' + [c.r, c.g, c.b].map(v => Math.round(v).toString(16).padStart(2, '0')).join('')
  const visible = el => { for (let e = el; e; e = e.parentElement) { const cs = getComputedStyle(e); if (cs.display === 'none' || cs.visibility === 'hidden') return false; if (e.tagName === 'DETAILS' && !e.open && !el.closest('summary')) return false } return el.getClientRects().length > 0 }
  const rows = []
  const tw = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  let n
  while ((n = tw.nextNode())) {
    const t = n.textContent.replace(/\s+/g, ' ').trim()
    if (!t || !/[A-Za-z0-9]/.test(t)) continue
    const p = n.parentElement
    if (!p || p.closest('script,style,svg')) continue
    if (!visible(p)) continue
    const r = p.getBoundingClientRect()
    if (r.bottom < 0 || r.top > innerHeight || r.right < 0 || r.left > innerWidth) continue
    const chain = []; for (let e = p; e; e = e.parentElement) chain.unshift(e)
    let B = {r: 255, g: 255, b: 255, a: 1}, base = null, op = 1
    for (const e of chain) {
      const cs = getComputedStyle(e)
      const o = Number(cs.opacity)
      if (o < 1 && !base) { base = B; op = o }
      const bg = parse(cs.backgroundColor); if (bg && bg.a > 0) B = over(bg, B)
    }
    let fg = over(parse(getComputedStyle(p).color), B), bgv = B
    if (base) { fg = mix(base, fg, op); bgv = mix(base, B, op) }
    const disabled = !!p.closest(':disabled,[aria-disabled="true"]') || !!base
    rows.push({t: t.slice(0, 48), id: (p.closest('[id]') || {}).id || '', fg: hex(fg), bg: hex(bgv), r: Math.round(ratio(fg, bgv) * 100) / 100, disabled})
  }
  const enabled = rows.filter(x => !x.disabled), dis = rows.filter(x => x.disabled)
  const min = a => a.reduce((m, x) => x.r < m.r ? x : m, {r: 99})
  const out = {texts: rows.length, minEnabled: min(enabled), minDisabled: dis.length ? min(dis) : null,
    under45: enabled.filter(x => x.r < 4.5), palette: document.documentElement.dataset.palette || '-'}
  document.documentElement.setAttribute('data-contrast', JSON.stringify(out))
}
