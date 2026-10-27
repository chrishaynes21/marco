// Package fixture makes the harness's test surface, Fixture.cs, a Go build input.
//
// There is no Go code in the fixture and there is not meant to be: it is a C# WinForms
// application, built by `build.ps1`, and the rules about what it may and may not do live as tests
// in `plugins/e2e`. One of those tests reads this file, and that is the whole reason this package
// exists.
//
// # Why reading it from the other module was not enough
//
// `plugins/e2e` is a module of its own, so `Fixture.cs` is outside it — and `go test` CACHES a
// result across a change to a file like that. Measured, not assumed: the gate that keeps the
// fixture from logging every idle pointer movement was deleted, the suite was run again, and it
// said `ok (cached)`. A test that goes green against a broken fixture on every run but the first
// is worse than no test, because the one run that would have caught it is the one nobody repeats.
//
// Embedding it here puts the CONTENT into the root module's build graph, so changing the C#
// changes this package's hash and every test that reads it runs again.
package fixture

import _ "embed"

// Source is the fixture's own code, as the tests about its rules read it.
//
//go:embed Fixture.cs
var Source string
