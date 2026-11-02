F.ReviewRename = (id, name) => ({id, from: id, to: 'arkascended/focus/open-settings', renaming: name + ' with item', clause: 'with item', movable: false, token: '',
  refused: '"open settings" is already a routine in arkascended. Nothing has been renamed \u2014 choose another name, or delete that one first',
  was: '// Recorded as: coordinates.\nuse os.\n', now: '// Recorded as: coordinates.\nuse os.\n'})
__at(400, () => __click('routineOpen-arkascended-focus-cloud-pull-stone'))
__at(1500, () => __click('routineRenameBtn'))
__at(1800, () => { __type('routineRenameBox', 'open settings') })
__at(2100, () => __click('routineRenameGo'))
__at(2800, () => __scrollTo('routine-arkascended-focus-cloud-pull-stone', 20))
