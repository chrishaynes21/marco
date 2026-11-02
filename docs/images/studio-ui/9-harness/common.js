// COMMON STUB — every binding the page calls, answering from window.F (the fixture),
// which each state script fills in. Wording in F is taken from the Go backend; see
// fixtures.js. A binding the page calls that is missing here throws, and the instrument
// records it, which is the point.
window.F = window.F || {}
const P = v => Promise.resolve(typeof v === 'function' ? v() : v)
const call = (name, dflt) => (...a) => {
  const f = window.F[name]
  if (f === undefined) return P(dflt)
  return P(typeof f === 'function' ? f(...a) : f)
}
window.go = {main: {
  Backend: {
    Loaded: () => P(),
    SuggestRoutine: call('SuggestRoutine', {answer: {kind: 'none'}}),
  },
  Acts: {
    SuggestTargets: call('SuggestTargets', []),
    Demonstrations: call('Demonstrations', []),
    Saved: call('Saved', []),
    PlaybackTarget: call('PlaybackTarget', {chosen: '', windows: []}),
    RoutineAnswers: call('RoutineAnswers', null),
    PlayNeeds: call('PlayNeeds', null),
    ReviewPlay: call('ReviewPlay', null),
    ConfirmPlay: call('ConfirmPlay', ''),
    CancelPlay: () => P(),
    Routines: call('Routines', []),
    Applications: call('Applications', []),
    Windows: call('Windows', []),
    Pictures: call('Pictures', []),
    Picture: call('Picture', ''),
    Learning: call('Learning', {active: false}),
    Watch: call('Watch', {act: '', active: false, transcript: null}),
    History: call('History', []),
    Unrunnable: call('Unrunnable', []),
    Library: call('Library', {dir: 'C:\\Users\\alex\\AppData\\Local\\Marco\\routes'}),
    Features: call('Features', {acts: false, scenes: false, views: ['routines', 'history', 'help']}),
    BuildStatus: call('BuildStatus', {build: 'ce296c0'}),
    Help: call('Help', {sections: []}),
    Ready: call('Ready', {build: 'ce296c0', problems: [], limits: ''}),
    Recording: call('Recording', {active: false, review: false}),
    StartRecording: call('StartRecording', ''),
    StopRecording: call('StopRecording', ''),
    CancelRecording: call('CancelRecording', ''),
    SimplifyDraft: call('SimplifyDraft', ''),
    UseOriginalDraft: call('UseOriginalDraft', ''),
    SupplyDraftText: call('SupplyDraftText', ''),
    TypedDraftText: call('TypedDraftText', ''),
    SaveDraft: call('SaveDraft', ''),
    SaveDraftAs: call('SaveDraftAs', ''),
    ReplaceDraft: call('ReplaceDraft', ''),
    Collision: call('Collision', null),
    StepEditing: call('StepEditing', null),
    ReviewStepEdit: call('ReviewStepEdit', null),
    SaveStepEdit: call('SaveStepEdit', ''),
    CancelStepEdit: call('CancelStepEdit', ''),
    ScopeChoices: call('ScopeChoices', null),
    ReviewRescope: call('ReviewRescope', null),
    ReviewRename: call('ReviewRename', null),
    RenameRoutine: call('RenameRoutine', ''),
    MoveRoutine: call('MoveRoutine', ''),
    CancelMove: call('CancelMove', ''),
    ReviewRebuilt: call('ReviewRebuilt', null),
    ReplaceWithRebuilt: call('ReplaceWithRebuilt', ''),
    CancelReview: call('CancelReview', ''),
    RecordAgain: call('RecordAgain', ''),
    AddRoutineAnswer: call('AddRoutineAnswer', ''),
    RemoveRoutineAnswer: call('RemoveRoutineAnswer', ''),
    PlayRoutine: call('PlayRoutine', ''),
    Resave: call('Resave', ''),
    Run: call('Run', ''), Play: call('Play', ''), Learn: call('Learn', ''), Done: call('Done', ''),
    StopLearning: call('StopLearning', ''), Compose: call('Compose', ''), Report: call('Report', ''),
    Inspect: call('Inspect', ''), ForgetEverything: call('ForgetEverything', ''),
    Cancel: () => P(),
  },
}}
// THE SCRIPT: a list of [ms, fn] the state runs, then measures.
window.__steps = window.__steps || []
window.__at = (ms, fn) => window.__steps.push([ms, fn])
window.addEventListener('load', () => {
  window.__steps.forEach(([ms, fn]) => setTimeout(() => { try { fn() } catch (e) { window.__errors.push('step: ' + e.message) } }, ms))
  const last = Math.max(0, ...window.__steps.map(s => s[0]))
  setTimeout(() => window.__measure(), last + 1500)
})
