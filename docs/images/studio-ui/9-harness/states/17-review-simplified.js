F.Recording = {active: false, review: true, name: 'type a note', application: 'notepad', stopped_by: 'the stop key',
  mode: 'coordinates', captures: CAPTURES, everywhere: EVERYWHERE, arg_key: 'F9', stop_key: 'F12', pressable: 2, basis: 'b3',
  steps: ['bring notepad to the front', 'wait 250ms', 'left-click at 400, 300 in the window (1280\u00d7720, 96 dpi, notepad)', 'wait 250ms', 'type "draft notes"', 'press enter'],
  said: '6 step(s), 3 of which press something.', simplified: true, original: true, unmarked: ['note'],
  default_scope: 'app-anywhere', unavailable: {everywhere: 'Every press here checks the window it was made in, but "From anywhere → current foreground" names no application, so each would refuse. Save it "Only in notepad" or "From anywhere → notepad".'},
  supply: [{at: 4, label: 'step 5 \u2014 types "draft notes"'}],
  supply_says: 'Pick a typed step and name it, and Play will ask for that value instead of typing it.'}
__at(1500, () => __scrollTo('capReview', 60))
