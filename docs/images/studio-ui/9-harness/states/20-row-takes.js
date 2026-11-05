// A ROUTINE'S ROW SAYS WHAT IT ASKS FOR: `cloud pull ‹item›`. ADR-317.
window.LIB = window.LIB.map(r => r.id === 'arkascended/focus/cloud-pull-stone'
  ? Object.assign({}, r, {name: 'cloud pull', takes: '\u2039item\u203a'}) : r)
