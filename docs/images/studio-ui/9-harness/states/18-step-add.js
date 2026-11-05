F.StepEditing = id => ({id, name: 'save the file', basis: 'sb1', editable: true, rows: [
  {at: 0, says: 'bring notepad to the front', x: 0, y: 0},
  {at: 1, says: 'wait 640ms', wait: true, ms: 640, x: 0, y: 0},
  {at: 2, says: 'hold ctrl', keyed: true, key: 'ctrl', holds: true, pairs: true, pairs_at: 4, pair_says: 'removes step 5 too: they are the two halves of holding "ctrl"', x: 0, y: 0},
  {at: 3, says: 'press s', keyed: true, key: 's', x: 0, y: 0},
  {at: 4, says: 'let go of ctrl', keyed: true, key: 'ctrl', holds: true, pairs: true, pairs_at: 2, pair_says: 'removes step 3 too: they are the two halves of holding "ctrl"', x: 0, y: 0},
  {at: 5, says: 'wait 1210ms', wait: true, ms: 1210, x: 0, y: 0},
  {at: 6, says: 'left-click at 32, 18 in the window (1280\u00d7720, 96 dpi, notepad)', point: true, movable: true, in_window: true, x: 32, y: 18},
  {at: 7, says: 'left-click at 1510, 22', point: true, movable: true, x: 1510, y: 22},
  {at: 8, says: 'type "draft notes"', types: true, text: 'draft notes', x: 0, y: 0},
  {at: 9, says: 'left-click at 210, 340 in the window (1280\u00d7720, 96 dpi, notepad)', point: true, in_window: true, x: 210, y: 340,
   point_why: 'found by its picture: the position is inside that picture, not on the screen'},
]})
__at(400, () => __click('routineOpen-notepad-context-save-the-file'))
__at(1500, () => __click('routineStepsBtn'))
__at(2200, () => __click('routineStepAdd-5'))
__at(2400, () => { const k = document.getElementById('routineStepNewKind-1'); k.value = 'key'; k.dispatchEvent(new Event('change')) })
__at(2600, () => { const v = document.getElementById('routineStepNewVal-1'); v.value = 'f5'; v.dispatchEvent(new Event('input')) })
__at(2800, () => __scrollTo('routineSteps', 40))
