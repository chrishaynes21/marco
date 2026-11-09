package main

import "testing"

// THE KEY BOX, WITH "NO LEADER" TICKED, TAKES THE COMBO PRESSED IN IT and sets it as a hotkey of
// its own: Ctrl+Shift+F9 is sent as `direct:ctrl+shift+f9`, a Shift+1 is 1 with Shift and not
// "!", and a modifier pressed alone waits for its key. Unticked, the box is one key after the
// leader, as it was. See ADR-341.
func TestTheKeyBoxTakesAComboPressedWithoutTheLeader(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
window.__set = ''
Object.assign(window.go.main.Acts, {
  RoutineShortcuts: (id) => Promise.resolve({id: id, name: 'x', leader: '`+"`"+`',
    leader_says: 'Press the leader.', keys: [], legacy: [], askable: true,
    places: [{listen_in: 'arkascended', label: 'in arkascended', allowed: true, default: true}]}),
  SetShortcut: (id, key, where, consent) => { window.__set = key + '@' + where
    return Promise.resolve({id: id, keys: [], legacy: [], places: [], askable: true}) },
})
let r = {}
const down = (o) => {
  const box = document.getElementById('routineKeyBox')
  box.dispatchEvent(new KeyboardEvent('keydown', Object.assign({bubbles: true, cancelable: true}, o)))
  return box.value
}
setTimeout(() => { press('routineOpen-' + K1) }, 1100)
setTimeout(() => { press('routineKeysTab') }, 1500)
setTimeout(() => {
  r.leaderBox = down({key: 'e', code: 'KeyE'}) === 'e' ? 1 : 0
  const tick = document.getElementById('routineKeyDirect')
  tick.checked = true
  tick.dispatchEvent(new Event('change'))
  r.cleared = document.getElementById('routineKeyBox').value === '' ? 1 : 0
  r.modWaits = down({key: 'Control', code: 'ControlLeft', ctrlKey: true}) === '' ? 1 : 0
  r.combo = down({key: 'F9', code: 'F9', ctrlKey: true, shiftKey: true}) === 'ctrl+shift+f9' ? 1 : 0
  r.shifted = down({key: '!', code: 'Digit1', shiftKey: true}) === 'shift+1' ? 1 : 0
  r.numpad = down({key: '5', code: 'Numpad5'}) === 'num5' ? 1 : 0
  down({key: 'F9', code: 'F9', ctrlKey: true, shiftKey: true})
  press('routineKeySetBtn')
}, 2100)
setTimeout(() => {
  r.sent = window.__set === 'direct:ctrl+shift+f9@arkascended' ? 1 : 0
  document.title = 'probe ' + Object.entries(r).map(([k, v]) => k + '=' + v).join(' ') +
    ' set=' + window.__set
}, 2500)
`)
	n := counters(t, title, "probe")
	for key, says := range map[string]string{
		"leaderBox": "unticked, the box no longer takes one key after the leader",
		"cleared":   "ticking no leader left the leader's key in the box",
		"modWaits":  "a modifier on its own was taken as the hotkey",
		"combo":     "Ctrl+Shift+F9 is not written ctrl+shift+f9",
		"shifted":   "Shift+1 is not written shift+1",
		"numpad":    "numpad 5 is not written num5",
		"sent":      "Set did not send direct:ctrl+shift+f9 for arkascended",
	} {
		if n(key) != 1 {
			t.Errorf("%s: %q", says, title)
		}
	}
}
