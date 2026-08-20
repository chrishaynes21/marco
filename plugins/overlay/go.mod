module github.com/chaynes-simpleclouds/marco/plugins/overlay

go 1.26.2

require (
	github.com/chaynes-simpleclouds/marco v0.0.0
	github.com/hajimehoshi/ebiten/v2 v2.9.9
	github.com/shirou/gopsutil/v3 v3.24.5
	golang.org/x/image v0.31.0
)

// The overlay decodes the Director's playbill into the DIRECTOR'S OWN TYPE rather than
// into a hand-mirrored struct. That is the whole architectural point of this milestone:
// one representation, three presentations. A mirrored copy would drift the first time a
// field's meaning changed, and it would drift silently, in the one surface whose job is
// to tell somebody the truth.
//
// It costs the overlay nothing it was avoiding: pkg/playbill is standard library only,
// as the whole engine is. The same pattern plugins/ocr and plugins/vision already use.
replace github.com/chaynes-simpleclouds/marco => ../..

require (
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-text/typesetting v0.3.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
