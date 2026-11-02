F.StepEditing = id => ({id, name: 'save the file', basis: 'sb1', editable: true, rows: [
  {at: 0, says: 'bring notepad to the front', x: 0, y: 0},
  {at: 1, says: 'wait 640ms', wait: true, ms: 640, x: 0, y: 0},
  {at: 2, says: 'hold ctrl', pairs: true, pairs_at: 4, pair_says: 'removing this removes step 5 as well, because they are the two halves of holding "ctrl"', x: 0, y: 0},
  {at: 3, says: 'press s', x: 0, y: 0},
  {at: 4, says: 'let go of ctrl', pairs: true, pairs_at: 2, pair_says: 'removing this removes step 3 as well, because they are the two halves of holding "ctrl"', x: 0, y: 0},
  {at: 5, says: 'wait 1210ms', wait: true, ms: 1210, x: 0, y: 0},
  {at: 6, says: 'left-click at 32, 18 in the window (1280\u00d7720, 96 dpi, notepad)', point: true, movable: true, in_window: true, x: 32, y: 18},
  {at: 7, says: 'left-click at 1510, 22', point: true, movable: true, x: 1510, y: 22},
  {at: 8, says: 'type "draft notes"', types: true, text: 'draft notes', x: 0, y: 0},
  {at: 9, says: 'left-click at 210, 340 in the window (1280\u00d7720, 96 dpi, notepad)', point: true, in_window: true, x: 210, y: 340,
   point_why: 'it is found by its picture, so the position is a place inside that picture rather than a place on the screen'},
]})
__at(400, () => __click('routineOpen-notepad-context-save-the-file'))
__at(1500, () => __click('routineStepsBtn'))
__at(2200, () => __click('routineStepCut-1'))
__at(2600, () => __scrollTo('routineSteps', 40))
