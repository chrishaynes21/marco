package service

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
)

// Serving the game diagnostics.
//
// The service TRANSPORTS what the framework concluded. It does not re-run detection, does
// not summarise a pack's contribution and has no opinion about either — the same rule the
// perception and demonstration requests follow, and for the same reason: a second reading
// of the same question is how two answers to it come about.

// gameDiagnostics answers one GAME request.
func (s *Server) gameDiagnostics(p GamePayload) (GameResponse, error) {
	out := GameResponse{
		Active: s.runtime.DetectedGame(),
		Packs:  len(s.runtime.GameCapabilities().Packs),
	}
	switch p.Action {
	case GameDetected, "":
		return out, nil
	case GameCapabilities:
		out.Report = s.runtime.GameCapabilities()
		return out, nil
	case GameInventory:
		out.Inventory = s.runtime.GameInventory(p.Container)
		return out, nil
	}
	return GameResponse{}, fmt.Errorf(
		"%q is not something that can be asked about a game", p.Action)
}

// unused keeps the game import meaningful to a reader of this file, which is entirely
// about game types even though it names none directly.
var _ = game.Active{}
