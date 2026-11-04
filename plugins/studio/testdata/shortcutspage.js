// THE SHORTCUT TAB'S HARNESS: a stubbed backend for rendering frontend/index.html without a window.
//
// Used two ways, so both see the same stub: shortcutspage_test.go renders it and reads counters off
// the title, and a screenshot of the same render is how the layout was reviewed. The scenario is
// chosen by the query string (?scenario=taken|global|onlyin|test); the default is test.
//
// The backend's answers are written here the way plugins/studio/shortcuts.go writes them; the rules
// behind them are S2's owner and are tested there and in shortcutsacts_test.go, not here.
window.addEventListener('error', (e) => {
  window.bad = (window.bad || '') + '[' + e.message + ']'
  document.title = 'the page threw: ' + window.bad
})
const scenario = (new URLSearchParams(location.search)).get('scenario') || 'test'
window.calls = []
const call = (name, args) => { window.calls.push(name + '(' + args.join('|') + ')') }

const draftPlaces = [
  {listen_in: 'notepad', label: 'while notepad is in front', allowed: true, default: true},
  {listen_in: '', label: 'everywhere', allowed: true},
  {named: true, listen_in: '', label: 'while another app you name is in front',
   allowed: true},
]
const copyPlaces = [
  {listen_in: '', label: 'everywhere', allowed: true},
  {named: true, listen_in: '', label: 'while an app you name is in front',
   allowed: true},
]
const pdfPlaces = [
  {listen_in: 'notepad', label: 'while notepad is in front', allowed: true, default: true},
  {listen_in: '', label: 'everywhere', allowed: false,
   why: '"save as pdf" works only while notepad is in front, so its shortcut can only be ' +
   'listened for there.'},
  {named: true, listen_in: '', label: 'while another app you name is in front',
   allowed: false, why: '"save as pdf" works only while notepad is in front, so its shortcut ' +
   'can only be listened for there.'},
]
const leader = {leader: '`', leader_says: "Press the widget's leader (`), then the key."}
const eKey = {key: 'e', listen_in: 'notepad', pressed: '`e', where: 'in notepad',
  version: 'v-e', runs: '"save draft"', cmd: 'save draft', state: 'ok'}
const sLegacy = {key: 's', listen_in: 'notepad', pressed: '`s', where: 'in notepad',
  version: 'v-s', runs: '"save draft" (set up by name)', cmd: 'save draft',
  state: 'needs-reassigning', plan: 'notepad/focus/save-draft', reassignable: true,
  plan_says: 'Set up by name as "save draft". Reassigning makes it run exactly ' +
    'notepad/focus/save-draft, keeping those words.'}
const cKey = {key: 'c', listen_in: '', pressed: '`c', where: 'everywhere', version: 'v-c',
  runs: '"copy"', cmd: 'copy', state: 'ok', shadows: 'In paint, `c runs "paint copy" instead.'}

let draftKeys = [eKey], draftLegacy = [sLegacy]
const answer = (id, extra) => {
  if (id === 'global/copy') {
    return Object.assign({id: id, name: 'copy', keys: [cKey], legacy: [], places: copyPlaces,
      askable: true}, leader, extra || {})
  }
  if (id === 'notepad/context/save-as-pdf') {
    return Object.assign({id: id, name: 'save as pdf', keys: [], legacy: [], places: pdfPlaces,
      askable: true}, leader, extra || {})
  }
  return Object.assign({id: id, name: 'save draft', keys: draftKeys, legacy: draftLegacy,
    places: draftPlaces, askable: true}, leader, extra || {})
}

window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: []}),
    Recording: () => Promise.resolve({active: false}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: () => Promise.resolve([]),
    CancelMove: () => {},
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    StepEditing: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ReviewStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    CancelStepEdit: () => {},
    SaveStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    RoutineAnswers: (id) => Promise.resolve({id: id, name: 'x', askable: true, phrases: []}),
    AddRoutineAnswer: () => Promise.resolve({}),
    RemoveRoutineAnswer: () => Promise.resolve({}),
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},

    // ── THE SHORTCUT TAB ──
    RoutineShortcuts: (...a) => { call('RoutineShortcuts', a); return Promise.resolve(answer(a[0])) },
    SetShortcut: (...a) => {
      call('SetShortcut', a)
      const [id, key, listenIn, consent] = a
      if (key === 'f' && consent === '') {
        // A KEY IN USE: nothing is written, and what is there comes back with its version.
        return Promise.resolve(answer(id, {taken: {key: 'f', listen_in: listenIn,
          pressed: '`f', where: listenIn ? 'in ' + listenIn : 'everywhere',
          version: 'v-f', runs: '"find"'}}))
      }
      if (key === 'm') {
        return Promise.resolve(answer(id, {failed: true,
          said: "`m is the widget's command line, so a shortcut on it could never run."}))
      }
      const k = {key: key, listen_in: listenIn, pressed: '`' + key,
        where: listenIn ? 'in ' + listenIn : 'everywhere', version: 'v-' + key,
        runs: '"save draft"', cmd: 'save draft', state: 'ok'}
      draftKeys = draftKeys.filter(x => !(x.key === key && x.listen_in === listenIn)).concat(k)
      return Promise.resolve(answer(id, {said: k.pressed + ' ' + k.where +
        ' now runs "save draft".'}))
    },
    ChangeShortcut: (...a) => {
      call('ChangeShortcut', a)
      const [id, fromIn, fromKey, , key, listenIn] = a
      draftKeys = draftKeys.filter(x => !(x.key === fromKey && x.listen_in === fromIn))
        .concat({key: key, listen_in: listenIn, pressed: '`' + key,
          where: listenIn ? 'in ' + listenIn : 'everywhere', version: 'v-' + key,
          runs: '"save draft"', cmd: 'save draft', state: 'ok'})
      return Promise.resolve(answer(id, {said: '`' + key + ' now runs "save draft".'}))
    },
    RemoveShortcut: (...a) => {
      call('RemoveShortcut', a)
      const [id, listenIn, key] = a
      draftKeys = draftKeys.filter(x => !(x.key === key && x.listen_in === listenIn))
      draftLegacy = draftLegacy.filter(x => !(x.key === key && x.listen_in === listenIn))
      return Promise.resolve(answer(id, {said: '`' + key + ' no longer runs anything.'}))
    },
    ReassignShortcut: (...a) => {
      call('ReassignShortcut', a)
      const [id, listenIn, key] = a
      draftLegacy = draftLegacy.filter(x => !(x.key === key && x.listen_in === listenIn))
      draftKeys = draftKeys.concat({key: key, listen_in: listenIn, pressed: '`' + key,
        where: 'in ' + listenIn, version: 'v2-' + key, runs: '"save draft"', state: 'ok'})
      return Promise.resolve(answer(id, {said: '`' + key + ' in ' + listenIn +
        ' now runs exactly notepad/focus/save-draft.'}))
    },
    Routines: () => {
      const row = (id, slug, app, scope, keys) => ({id: id, name: slug, slug: slug, app: app,
        scope: scope, contract: 'guarded', runnable: true, source: 'use os.',
        generation: 'current', rebuildable: true, offer: false, wrote: 'x', keys: keys,
        saved_at: '2026-11-03 09:12'})
      return Promise.resolve([
        row('global/copy', 'copy', '', 'everywhere', ['`c everywhere']),
        row('notepad/focus/save-draft', 'save draft', 'notepad', 'app-anywhere',
          draftKeys.map(k => k.pressed + ' ' + (k.listen_in || 'everywhere'))),
        row('notepad/context/save-as-pdf', 'save as pdf', 'notepad', 'app', []),
      ])
    },
  },
}}

const $id = (id) => document.getElementById(id)
const press = (id) => { const el = $id(id); if (el) { el.click() } }
const shown = (id) => {
  const el = $id(id)
  return (!el || el.classList.contains('hide')) ? '' : el.textContent
}
const open = (id) => press('routineOpen-' + id.replace(/[^a-z0-9]+/gi, '-'))
const typeKey = (k) => {
  const box = $id('routineKeyBox')
  box.dispatchEvent(new KeyboardEvent('keydown', {key: k, bubbles: true, cancelable: true}))
}
const at = (ms, fn) => setTimeout(() => { try { fn() } catch (e) {
  window.bad = (window.bad || '') + '[' + e.message + ']' } }, ms)

if (scenario === 'taken') {
  at(900, () => open('notepad/focus/save-draft'))
  at(1100, () => press('routineKeysTab'))
  at(1300, () => { typeKey('f'); press('routineKeySetBtn') })
} else if (scenario === 'global') {
  at(900, () => open('global/copy'))
  at(1100, () => press('routineKeysTab'))
} else if (scenario === 'onlyin') {
  at(900, () => open('notepad/context/save-as-pdf'))
  at(1100, () => press('routineKeysTab'))
} else {
  // ── THE TEST SCENARIO: every claim the page makes, read off the DOM ──
  const got = {}
  at(900, () => open('notepad/focus/save-draft'))
  at(1100, () => {
    got.chips = ($id('routineKeys-notepad-focus-save-draft') || {textContent: ''}).textContent
    press('routineKeysTab')
  })
  at(1400, () => {
    got.rows = $id('routineKeysList').children.length
    got.rowText = $id('routineKeysList').textContent
    got.legacyText = $id('routineKeysLegacy').textContent
    got.leader = shown('routineKeysLeader')
    got.defaultWhere = $id('routineKeyWhere').value
    got.setDisabledEmpty = $id('routineKeySetBtn').disabled
    typeKey('F')
    got.boxLower = $id('routineKeyBox').value
    press('routineKeySetBtn')
  })
  at(1700, () => {
    // A KEY IN USE: shown, nothing replaced, the typed key kept.
    got.taken = shown('routineKeyTaken')
    got.kept = $id('routineKeyBox').value
    // The person changes the form; Replace must still send what the banner showed.
    $id('routineKeyBox').value = 'g'
    press('routineKeyReplaceBtn')
  })
  at(2000, () => {
    got.takenGone = shown('routineKeyTaken') === '' ? 1 : 0
    got.saidAfterReplace = shown('routineKeysSaid')
    // CHANGE: started on `e, moved to `h... h is reserved by the owner; use j.
    press('routineKey-notepad-e-change')
  })
  at(2200, () => {
    got.changeLabel = $id('routineKeyFormLabel').textContent
    got.cancelShown = shown('routineKeyCancelBtn') ? 1 : 0
    typeKey('j')
    press('routineKeySetBtn')
  })
  at(2500, () => {
    press('routineKeyLegacy-notepad-s-reassign')
  })
  at(2800, () => {
    got.reassigned = shown('routineKeysSaid')
    press('routineKey-notepad-j-remove')
  })
  at(3100, () => {
    got.removed = shown('routineKeysSaid')
    typeKey('m')
    press('routineKeySetBtn')
  })
  at(3400, () => {
    got.refused = shown('routineKeysSaid')
    got.refusedBad = $id('routineKeysSaid').className === 'no' ? 1 : 0
    got.refusedKept = $id('routineKeyBox').value
    open('global/copy')
  })
  at(3700, () => {
    // ANOTHER ROUTINE: none of the first one's keys, and no place chosen for it.
    press('routineKeysTab')
  })
  at(4000, () => {
    got.copyText = $id('routineKeysList').textContent
    got.copyWhere = $id('routineKeyWhere').value
    got.copyShadow = $id('routineKeysList').textContent
    typeKey('x')
    got.copySetDisabled = $id('routineKeySetBtn').disabled
    open('notepad/context/save-as-pdf')
  })
  at(4300, () => press('routineKeysTab'))
  at(4600, () => {
    const sel = $id('routineKeyWhere')
    got.pdfEverywhereDisabled = Array.from(sel.options)
      .some(o => o.textContent.startsWith('everywhere') && o.disabled) ? 1 : 0
    got.pdfWhy = shown('routineKeyWhereWhy')
  })
  at(4900, () => {
    const c = window.calls
    const has = (s) => c.includes(s) ? 1 : 0
    document.title = 'err=' + (window.bad ? 1 : 0) +
      ' chips=' + (got.chips.includes('`e notepad') ? 1 : 0) +
      ' rows=' + got.rows +
      ' rowsays=' + (got.rowText.includes('`e') && got.rowText.includes('in notepad') ? 1 : 0) +
      ' legacy=' + (got.legacyText.includes('set up by name') &&
                    got.legacyText.includes('Reassign') ? 1 : 0) +
      ' leader=' + (got.leader.includes('leader (`)') ? 1 : 0) +
      ' defaultwhere=' + (got.defaultWhere === '0' ? 1 : 0) +
      ' setdisabledempty=' + (got.setDisabledEmpty ? 1 : 0) +
      ' boxlower=' + (got.boxLower === 'f' ? 1 : 0) +
      ' taken=' + (got.taken.includes('already runs "find"') ? 1 : 0) +
      ' kept=' + (got.kept === 'f' ? 1 : 0) +
      ' firstset=' + has('SetShortcut(notepad/focus/save-draft|f|notepad|)') +
      ' replacesent=' + has('SetShortcut(notepad/focus/save-draft|f|notepad|v-f)') +
      ' replacenotform=' + (c.some(s => s.startsWith('SetShortcut(notepad/focus/save-draft|g'))
        ? 0 : 1) +
      ' takengone=' + got.takenGone +
      ' changelabel=' + (got.changeLabel === 'change `e to' ? 1 : 0) +
      ' cancelshown=' + got.cancelShown +
      ' changesent=' + has('ChangeShortcut(notepad/focus/save-draft|notepad|e|v-e|j|notepad|)') +
      ' reassignsent=' + has('ReassignShortcut(notepad/focus/save-draft|notepad|s|v-s|' +
        'notepad/focus/save-draft)') +
      ' reassignsaid=' + (got.reassigned.includes('now runs exactly') ? 1 : 0) +
      ' removesent=' + has('RemoveShortcut(notepad/focus/save-draft|notepad|j|v-j)') +
      ' removesaid=' + (got.removed.includes('no longer runs') ? 1 : 0) +
      ' refused=' + (got.refused.includes('could never run') ? 1 : 0) +
      ' refusedbad=' + got.refusedBad +
      ' refusedkept=' + (got.refusedKept === 'm' ? 1 : 0) +
      ' copyclean=' + (got.copyText.includes('`e') ? 0 : 1) +
      ' copynodefault=' + (got.copyWhere === '' ? 1 : 0) +
      ' copyshadow=' + (got.copyShadow.includes('In paint') ? 1 : 0) +
      ' copysetdisabled=' + (got.copySetDisabled ? 1 : 0) +
      ' pdfeverywhere=' + got.pdfEverywhereDisabled +
      ' pdfwhy=' + (got.pdfWhy.includes('can only be listened for there') && got.pdfWhy.split('works only').length === 2 ? 1 : 0) +
      ' calls=' + c.length
  })
}
