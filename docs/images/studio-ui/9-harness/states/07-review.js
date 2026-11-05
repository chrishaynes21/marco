F.Recording = {active: false, review: true, name: 'save the file', application: 'notepad', stopped_by: 'the stop key',
  mode: 'coordinates', captures: CAPTURES, everywhere: EVERYWHERE, arg_key: 'F9', stop_key: 'F12', pressable: 2, basis: 'b1',
  steps: ['bring notepad to the front', 'wait 640ms', 'hold ctrl', 'press s', 'let go of ctrl', 'wait 1210ms',
          'left-click at 32, 18 in the window (1280\u00d7720, 96 dpi, notepad)'],
  said: '7 step(s), 2 of which press something.',
  exit: 'The alt+tab you used to leave the window and come back here are how the recording was ended, not part of it, so they were left out.',
  default_scope: 'app-anywhere',
  unavailable: {everywhere: 'every press in this recording checks the window it was made in, and "From anywhere → current foreground" names no application, so there would be no window for those presses to be checked against and every one of them would refuse. Save it "Only in notepad" or "From anywhere → notepad".'}}
__at(1500, () => __scrollTo('capReview', 60))
