package main

// mlog is a minimal inline structured logger for the voice plugin.
// Mirrors internal/mlog but lives here because plugins are separate modules.
//
// Usage: mlogD("msg", "key", val) — reads $MARCO_LOG at first call.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type mlogLevel int

const (
	mlogDebug mlogLevel = iota
	mlogInfo
	mlogWarn
	mlogError
	mlogOff
)

var (
	mlogOnce sync.Once
	mlogLvl  mlogLevel = mlogOff
	mlogW    io.Writer = os.Stderr
)

func mlogInit() {
	mlogOnce.Do(func() {
		mlogLvl = mlogParseLevel(os.Getenv("MARCO_LOG"))
	})
}

func mlogParseLevel(s string) mlogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return mlogDebug
	case "info":
		return mlogInfo
	case "warn", "warning":
		return mlogWarn
	case "error", "err":
		return mlogError
	case "off", "none", "silent":
		return mlogOff
	default:
		return mlogInfo
	}
}

func mlogWrite(lvl mlogLevel, msg string, kv ...any) {
	mlogInit()
	if lvl < mlogLvl {
		return
	}
	names := [...]string{"DEBUG", "INFO", "WARN", "ERROR"}
	name := "OFF"
	if int(lvl) < len(names) {
		name = names[lvl]
	}
	s := time.Now().Format("[15:04:05]") + "[" + name + "] " + strings.Replace(msg, ": ", " - ", 1)
	for i := 0; i+1 < len(kv); i += 2 {
		s += fmt.Sprintf(" %v=%v", kv[i], kv[i+1])
	}
	fmt.Fprintln(mlogW, s)
}

func mlogD(msg string, kv ...any)  { mlogWrite(mlogDebug, msg, kv...) }
func mlogI(msg string, kv ...any)  { mlogWrite(mlogInfo, msg, kv...) }
func mlogW2(msg string, kv ...any) { mlogWrite(mlogWarn, msg, kv...) }
func mlogE(msg string, kv ...any)  { mlogWrite(mlogError, msg, kv...) }
