// FIXTURES — production wording, quoted from the Go backend (file:line in the audit doc).
const CAPTURES = 'Where you press and how long you pause, and nothing about how the screen looked. It does not take a picture of anything.'
const EVERYWHERE = 'Records actions in the window you choose. Recording pauses outside it. Return to Studio and press Stop, or press F12.'
const ARG_SAYS = 'Press F9 during the demonstration where the routine should ask for a value instead of typing it. Add \u201cwith item\u201d to the name above and the value is called item.'
const WROTE = "This routine states that generation 1 of Marco's rules wrote it, which is the generation this build writes. You can review a version rebuilt from its stored recording, which will show whether it differs from what is saved."
const LEGACY = 'Its presses are not checked: each one happens at the point it was recorded at, whatever is there now.'
const NP = 'notepad \u2014 Untitled - Notepad'
const ARK = 'arkascended \u2014 ARK: Survival Ascended'

window.LIB = [
  {id: 'notepad/context/save-the-file', name: 'save the file', slug: 'save-the-file', app: 'notepad',
   scope: 'app', contract: 'guarded', mode: 'coordinates', captures: CAPTURES, runnable: true,
   saved_at: '2026-10-30 11:02', generation: 'current', declared_generation: '1', wrote: WROTE,
   rebuildable: true, offer: true,
   source: '// Recorded as: coordinates.\nuse os.\nthe p1 is a Point with X 352, Y 48, RelX 32, RelY 18, RelTo "window", WinW 1280, WinH 720, Dpi 96, Process "notepad.exe".\nsave the file is:\n  do OS\'s Key with "ctrl+s".\n  do OS\'s Click with p1.\n'},
  {id: 'arkascended/focus/cloud-pull-stone', name: 'cloud pull stone', slug: 'cloud-pull-stone', app: 'arkascended',
   scope: 'app-anywhere', contract: 'guarded', mode: 'coordinates', captures: CAPTURES, runnable: true, clause: 'with item',
   saved_at: '2026-10-31 23:09', generation: 'current', declared_generation: '1', wrote: WROTE, rebuildable: true, offer: true,
   source: '// Recorded as: coordinates.\nuse os.\n'},
  {id: 'arkascended/focus/open-settings', name: 'open settings', slug: 'open-settings', app: 'arkascended',
   scope: 'app-anywhere', contract: 'guarded', mode: 'coordinates', captures: CAPTURES, runnable: true,
   saved_at: '2026-10-29 21:40', generation: 'current', declared_generation: '1', wrote: WROTE, rebuildable: true, offer: true,
   source: '// Recorded as: coordinates.\nuse os.\n'},
  {id: 'global/old-launcher', name: 'old launcher', slug: 'old-launcher',
   scope: 'everywhere', contract: 'legacy', checks: LEGACY, runnable: true,
   saved_at: '2026-09-14 08:12', generation: 'unstated', wrote: 'This routine does not state which generation rules wrote it. You can review a version rebuilt from its stored recording.',
   rebuildable: true, offer: true, source: 'use os.\n'},
  {id: 'arkascended/focus/graphics-low', name: 'graphics low', slug: 'graphics-low', app: 'arkascended',
   scope: 'app-anywhere', contract: 'incomplete', mode: 'coordinates', captures: CAPTURES, runnable: false,
   why: 'The recording is missing how wide that window was and what scale it was drawn at, so every press refuses. Record it again.',
   saved_at: '2026-10-28 19:55', generation: 'current', declared_generation: '1', wrote: WROTE, rebuildable: true, offer: true,
   source: '// Recorded as: coordinates.\nuse os.\n'},
]

const HELP = {sections: [
  {key: 'recording', title: 'Recording a routine', lines: [
    'Press Record, give the routine a name, and choose the window to record in \u2014 Marco asks which one before it starts, and records that window only.',
    EVERYWHERE,
    'When you stop, you see what was recorded before anything is saved.']},
  {key: 'arguments', title: 'Asking for a value each time', lines: [
    'Name it \u201ccloud pull stone with item\u201d and the routine asks for item each time you play it.',
    'Or press F9 during the demonstration, at the moment the value goes in.',
    'Or afterwards: open \u201cEdit its steps\u201d, find the step that types the value, and say what to call it.',
    'A routine that asks for a value shows a box on its card, and Play waits until it is filled in.']},
  {key: 'editing', title: 'Editing what a routine does', lines: [
    'Change a wait, remove a step, or say a run of steps is text you typed.',
    'Make the text a step types something you supply each time \u2014 all of that step\u2019s text, not part of it.',
    'Nothing is written until you review the change and save it \u2014 the version you would get is shown beside the one you have.',
    'The routine is written again from the list, so removing one half of a held key removes the other half with it, and the waits around a removed step stay as they are until you change them too.']},
  {key: 'measured', title: 'Where a press is measured', lines: [
    'A press recorded inside a window is written down twice: as an offset inside that window, and as a point on the screen. Both were measured at the same instant.',
    'The offset is the durable half \u2014 it still means the same place after the window has moved \u2014 so it is the one you edit, and the point on the screen moves with it from that same measurement.',
    'A press recorded with no window to measure against has only the point on the screen, and that is what you edit instead.',
    'A press Marco finds by its picture is not moved at all: the coordinate is a position inside that picture, so moving it would aim the picture somewhere else.',
    'Changing a press keeps everything else about it: which button it uses, any keys held down with it, and the window, size and scale it is checked against.']},
  {key: 'checks', title: 'What a routine checks before it presses', lines: [
    'A routine recorded in a window checks three things before every press: that the program is the one it was recorded in, that the window is the size it was recorded at, and that it is drawn at the same scale.',
    'If any of them has changed it stops, rather than pressing somewhere it was never shown.',
    'Beside Play you are told which window the run will bring forward, and whether its presses are checked against it.',
    'A routine saved before Marco measured windows checks nothing: every press happens at the point it was recorded at, whatever is there now. Its card says so.',
    'A recording that measured only some of those terms refuses every press, and its card says which measurement is missing. Record it again.']},
  {key: 'scope', title: 'Where a routine works', lines: [
    'In one application only \u2014 it runs when that application is in front.',
    'In that application wherever it is \u2014 it brings the window forward first.',
    'Everywhere \u2014 no application is checked before it runs.']},
]}

const winFor = id => id.startsWith('notepad') ? {id: '4660:77', label: NP, app: 'notepad'}
  : id.startsWith('arkascended') ? {id: '9120:3304', label: ARK, app: 'arkascended'} : {id: '4660:77', label: NP}
const GUARDED_SAYS = l => 'Play brings ' + l + ' to the front and presses there. Its presses are checked against that window and stop if something else comes forward.'
const LOOSE_SAYS = l => 'Play brings ' + l + ' to the front and presses there. Nothing after that is confined to it: its presses are not checked, so whatever is in front at the time receives them.'

Object.assign(F, {
  Help: HELP,
  Ready: {build: 'ce296c0', installation: 'a tree build, with no manifest of what these binaries are', problems: [],
    limits: 'Checked just now, without looking at a screen: this cannot say the input hooks will install, or that a press will land where you put it.'},
  SuggestTargets: [{app: 'notepad', open: true, context: 1}, {app: 'arkascended', open: true, context: 0}],
  Library: {dir: 'C:\\Users\\alex\\AppData\\Local\\Marco\\routes'},
  Windows: [{id: '4660', label: NP}, {id: '9120', label: ARK}, {id: '7712', label: 'msedge \u2014 Marco docs - Microsoft Edge'}],
  Routines: () => window.LIB,
  PlaybackTarget: id => { const r = window.LIB.find(x => x.id === id) || {}; const w = winFor(id)
    return {chosen: w.id, windows: [{id: w.id, label: w.label, says: r.contract === 'legacy' ? LOOSE_SAYS(w.label) : GUARDED_SAYS(w.label)}]} },
  RoutineAnswers: id => ({id, name: (window.LIB.find(x => x.id === id) || {}).name, phrases: id.includes('cloud') ? ['grab a stone'] : [], askable: true}),
  PlayNeeds: id => id.includes('cloud')
    ? {id, name: 'cloud pull stone', needs: true, fields: [{name: 'item', secret: false, says: 'typed into the routine where you tapped during the demonstration'}]}
    : {id, name: '', needs: false, fields: []},
})
F.Recording = {active: false, mode: 'coordinates', stop_key: 'F12', captures: CAPTURES, everywhere: EVERYWHERE, arg_key: 'F9', arg_says: ARG_SAYS}
