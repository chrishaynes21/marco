#Requires AutoHotkey v2.0
; ahk-bridge.ahk — a reference Marco host bridge in AutoHotkey (see spec/Hosts.md).
;
; This is the "keep AHK as a bridge host" path: Marco describes the macro, this
; script performs the real keystrokes/mouse via AHK's Send/Click while Marco
; orchestrates. It speaks the bridge protocol over stdio — one JSON object per
; line in, one per line out:
;
;   in : {"act":"OS","action":"Key","input":"e"}
;   out: {"status":"ok","data":null}
;
; Run as a Marco host:
;   marco run --host bridge:"AutoHotkey.exe ahk-bridge.ahk" program.marco
;   (or compile this script to ahk-bridge.exe and use bridge:ahk-bridge.exe)
;
; Scope: handles flat inputs (string / number) and a {X,Y} point object — enough
; for Key, Type, Click, Move, Sleep. It is a starting point, not a full parser.

stdin  := FileOpen("*", "r `n")
stdout := FileOpen("*", "w `n")

Loop {
    line := stdin.ReadLine()
    if (line == "")
        break
    line := Trim(line, " `t`r`n")
    if (line == "")
        continue

    action := JsonField(line, "action")
    status := "ok"
    global gData := "null"      ; set by actions that return data (e.g. find)
    try {
        Dispatch(action, line)
    } catch as e {
        Respond(stdout, "failed", e.Message, "null")
        continue
    }
    Respond(stdout, status, "", gData)
}

Dispatch(action, line) {
    global gData
    switch StrLower(action) {
        case "key":
            Send("{" StrInput(line) "}")
        case "type":
            SendText(StrInput(line))
        case "click":
            px := NumField(line, "X"), py := NumField(line, "Y")
            if (px != "" && py != "")
                Click(px, py)
            else
                Click()
        case "move":
            MouseMove(NumField(line, "X"), NumField(line, "Y"), 0)
        case "sleep":
            Sleep(NumInput(line))
        case "find":
            ; The optional AHK Find backend: ImageSearch returns the match's
            ; top-left; we report the center as {X,Y} so it lines up with the
            ; native host. Region defaults to the whole screen.
            img := JsonField(line, "Image")
            if (img == "")
                throw Error("find needs an Image path")
            x1 := NumField(line, "X1"), y1 := NumField(line, "Y1")
            x2 := NumField(line, "X2"), y2 := NumField(line, "Y2")
            if (x2 == "" || x2 = 0) {
                x1 := 0, y1 := 0
                x2 := A_ScreenWidth, y2 := A_ScreenHeight
            }
            tol := NumField(line, "Tolerance")
            if (tol == "")
                tol := 10
            ; ImageSearch reports the match's top-left; report it as {X,Y}.
            ; (Good enough for most click targets; the native host reports the
            ; center.)
            if ImageSearch(&fx, &fy, x1, y1, x2, y2, "*" tol " " img)
                gData := '{"X":' fx ',"Y":' fy '}'
            else
                throw Error("image not found on screen")
        default:
            throw Error("unknown action: " action)
    }
}

; Respond writes a JSON response line. On ok, data is a raw JSON fragment.
Respond(stdout, status, errMsg, data) {
    if (status == "failed")
        stdout.Write('{"status":"failed","error":"' JsonEsc(errMsg) '"}`n')
    else
        stdout.Write('{"status":"ok","data":' data '}`n')
}

; --- tiny JSON field extractors (flat objects only) ---

; JsonField pulls a string value: "key":"value"
JsonField(json, key) {
    if RegExMatch(json, '"' key '"\s*:\s*"([^"]*)"', &m)
        return m[1]
    return ""
}

; StrInput / NumInput pull the top-level "input" value.
StrInput(json) {
    if RegExMatch(json, '"input"\s*:\s*"([^"]*)"', &m)
        return m[1]
    return ""
}
NumInput(json) {
    if RegExMatch(json, '"input"\s*:\s*([-0-9.]+)', &m)
        return m[1] + 0
    return 0
}

; NumField pulls a nested number, e.g. input.X / input.Y.
NumField(json, key) {
    if RegExMatch(json, '"' key '"\s*:\s*([-0-9.]+)', &m)
        return m[1] + 0
    return ""
}

JsonEsc(s) {
    s := StrReplace(s, "\", "\\")
    s := StrReplace(s, '"', '\"')
    return s
}
