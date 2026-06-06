#Requires AutoHotkey v2.0
; ahk-hotkeys.ahk — the MacroMarco leader-key engine as a Marco event source.
;
; It captures the backtick (`) leader key followed by a command key (and Esc to
; stop), and emits one JSON Hotkeys event per command on stdout. Pipe it into a
; served Marco program so the existing AHK front-end drives the Marco macros:
;
;   AutoHotkey.exe ahk-hotkeys.ahk | marco serve --host windows programs/globals.marco
;
; Leader chords (press backtick, then the key):
;   `e -> E    `c -> C    `1.. -> K1..K4    `r -> Roll   `8 -> EightBall   `n -> Name
;   Esc        -> Stop   (no leader needed; mirrors MacroMarco's stop key)
;
; This is the "keep AHK as the front-end" path: AHK owns hotkey capture and the
; UI; Marco owns the macro logic. Stop the original engine's own hotkeys (or run
; this standalone) so they don't double-fire.

stdout := FileOpen("*", "w `n")

Emit(event) {
    global stdout
    stdout.Write('{"feed":"Hotkeys","event":"' event '"}`n')
    stdout.Read(0)   ; nudge the buffer to flush to the pipe
}

; Map a command key (the key pressed after the leader) to a Hotkeys event.
cmdMap := Map(
    "e", "E", "c", "C",
    "1", "K1", "2", "K2", "3", "K3", "4", "K4",
    "r", "Roll", "8", "EightBall", "n", "Name"
)

; Backtick leader (VK_OEM_3 = 0xC0). `$` makes it non-recursive; capturing it
; means the leader itself is swallowed, not typed.
$vkC0:: {
    global cmdMap
    ih := InputHook("L1 T2")        ; one character, 2-second timeout
    ih.Start()
    ih.Wait()
    key := StrLower(ih.Input)
    if cmdMap.Has(key)
        Emit(cmdMap[key])
}

; Stop key — ends any running spam, no leader needed.
Esc::Emit("Stop")

; F12 quits the bridge (and, via EOF on the pipe, shuts down `marco serve`).
F12::ExitApp()
