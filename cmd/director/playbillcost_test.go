package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// What watching costs.
//
// An observability surface that changes the system it observes is worse than none, so the
// cost of a poll is measured rather than asserted. The number that matters is per-read:
// the overlay refreshes at 1.5s while a panel is open, and the perception loop it sits
// beside samples every 500ms and spends 170–730ms of that on a detection pass.
//
// Run with:
//
//	go test ./cmd/director -run xxx -bench Playbill -benchmem
//
// A read is expected in the tens of microseconds. If one ever approaches a millisecond,
// something in the assembly has started doing work rather than reading it — which is the
// failure this benchmark exists to catch, not the absolute number.

func BenchmarkPlaybillRead(b *testing.B) {
	g := benchRegistry(b)
	rt := benchRuntime(b, g)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = rt.Playbill(service.PlaybillPayload{})
	}
}

// The diagnostics read is the expensive one, and it is opt-in for exactly that reason.
func BenchmarkPlaybillReadWithDiagnostics(b *testing.B) {
	g := benchRegistry(b)
	rt := benchRuntime(b, g)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = rt.Playbill(service.PlaybillPayload{Diagnostics: true})
	}
}

// Rendering is what a presentation does sixty times a second, so it is measured apart
// from the read a presentation does once a second and a half.
func BenchmarkPlaybillRender(b *testing.B) {
	g := benchRegistry(b)
	rt := benchRuntime(b, g)
	v := rt.Playbill(service.PlaybillPayload{}).Normalise()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = v.Watch()
	}
}

// The digest is computed on every publish, so it has to be cheap enough not to matter.
func BenchmarkPlaybillDigest(b *testing.B) {
	g := benchRegistry(b)
	rt := benchRuntime(b, g)
	v := rt.Playbill(service.PlaybillPayload{}).Normalise()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = v.WithDigest()
	}
}

func benchRegistry(b *testing.B) *observationRegistry {
	b.Helper()
	t := &testing.T{}
	return observedRegistryFor(t)
}

func benchRuntime(b *testing.B, g *observationRegistry) *Runtime {
	b.Helper()
	rt := testRuntime(&testing.T{})
	rt.observations = g
	return rt
}
