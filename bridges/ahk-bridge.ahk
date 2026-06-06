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
    try {
        Dispatch(action, line)
    } catch as e {
        status := "failed"
        Respond(stdout, status, e.Message)
        continue
    }
    Respond(stdout, status, "")
}

Dispatch(action, line) {
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
        default:
            throw Error("unknown action: " action)
    }
}

; Respond writes a JSON response line. data is an optional error message.
Respond(stdout, status, errMsg) {
    if (status == "failed")
        stdout.Write('{"status":"failed","error":"' JsonEsc(errMsg) '"}`n')
    else
        stdout.Write('{"status":"ok","data":null}`n')
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
