F.Recording = {active: false, review: true, name: 'type a note', application: 'notepad', stopped_by: 'the stop key',
  mode: 'coordinates', captures: CAPTURES, everywhere: EVERYWHERE, arg_key: 'F9', stop_key: 'F12', pressable: 2, basis: 'b3',
  steps: ['bring notepad to the front', 'wait 250ms', 'left-click at 400, 300 in the window (1280\u00d7720, 96 dpi, notepad)', 'wait 250ms', 'type "draft notes"', 'press enter'],
  said: '6 step(s), 3 of which press something.', simplified: true, original: true, unmarked: ['note'],
  default_scope: 'app-anywhere', unavailable: {everywhere: 'every press in this recording checks the window it was made in, and "From anywhere → current foreground" names no application, so there would be no window for those presses to be checked against and every one of them would refuse. Save it "Only in notepad" or "From anywhere → notepad".'},
  supply: [{at: 4, label: 'step 5 \u2014 types "draft notes"'}],
  supply_says: 'Ask for a value instead of typing it: pick a step, name the value, and the routine will ask for it when you play it.'}
__at(1500, () => __scrollTo('capReview', 60))
