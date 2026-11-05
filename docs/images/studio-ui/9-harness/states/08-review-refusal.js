F.Recording = {active: false, review: true, name: 'graphics low', application: 'arkascended', stopped_by: 'the Stop button',
  mode: 'coordinates', captures: CAPTURES, everywhere: EVERYWHERE, arg_key: 'F9', stop_key: 'F12', pressable: 3, basis: 'b2',
  steps: ['bring arkascended to the front', 'press escape', 'wait 900ms',
          'left-click at 960, 540 in the window (1920\u00d71080, cannot be checked \u2014 what scale it was drawn at was not measured)',
          'wait 700ms', 'left-click at 412, 655 in the window (1920\u00d71080, cannot be checked \u2014 what scale it was drawn at was not measured)'],
  said: '6 step(s), 3 of which press something.',
  away: 'Left out: 8s outside the window (once), and anything you did there.',
  incomplete: true,
  missing: '2 presses are missing measurements: what scale it was drawn at. Record it again with the program in front of you.',
  default_scope: 'app-anywhere',
  unavailable: {everywhere: 'Every press here checks the window it was made in, but "From anywhere → current foreground" names no application, so each would refuse. Save it "Only in arkascended" or "From anywhere → arkascended".'}}
__at(1500, () => __scrollTo('capReview', 60))
