package directorapi

// Marco's own windows, and why Marco must not see them.
//
// # The failure this prevents
//
// A presentation surface that draws over an application is, to the window system, another window
// in front of that application. Left alone, it enumerates as a targetable program, it can be
// chosen as an observation target, it adds a candidate to every fuzzy window resolution, and — if
// it is ever on screen while a session is running — it becomes structure Marco believes belongs to
// the world it is watching.
//
// That last one is the serious case. A pointer Marco can see is not a pointer: it is part of the
// scene, and the thing it points at is now described partly in terms of the pointing.
//
// # Ownership, not naming
//
// The marker is a WINDOW PROPERTY set by the surface on its own handle. Deliberately not a title
// match, not a substring, and not the process name: titles are user-visible text that changes,
// substrings collide with any application that happens to mention Marco (a browser tab reading
// "Marco - Director" already does), and a process may legitimately own both a presentation surface
// and an ordinary window. A property is attached to the one handle that is actually ours.
//
// It also disappears correctly. A window property lives and dies with its window, so a surface
// that crashes leaves nothing behind to keep excluding — there is no registry to go stale, and no
// cleanup step that can be skipped.
//
// Set it with SetPropW on the surface's own HWND; the engine reads it with GetPropW while
// enumerating. Both sides name it from here so there is one spelling.
const OwnedSurfaceProperty = "MarcoOwnedPresentationSurface"
