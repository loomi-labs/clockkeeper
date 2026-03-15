package web

import (
	"context"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

func (h *ClockKeeperServiceHandler) ListCharacters(ctx context.Context, req *connect.Request[clockkeeperv1.ListCharactersRequest]) (*connect.Response[clockkeeperv1.ListCharactersResponse], error) {
	var chars []*botc.Character

	switch {
	case req.Msg.Edition != "" && req.Msg.Team != "":
		// Filter by both edition and team.
		for _, c := range h.registry.CharactersByEdition(req.Msg.Edition) {
			if string(c.Team) == req.Msg.Team {
				chars = append(chars, c)
			}
		}
	case req.Msg.Edition != "":
		chars = h.registry.CharactersByEdition(req.Msg.Edition)
	case req.Msg.Team != "":
		chars = h.registry.CharactersByTeam(botc.Team(req.Msg.Team))
	default:
		chars = h.registry.AllCharacters()
	}

	return connect.NewResponse(&clockkeeperv1.ListCharactersResponse{
		Characters: charactersToProto(chars),
	}), nil
}

func (h *ClockKeeperServiceHandler) ListEditions(ctx context.Context, req *connect.Request[clockkeeperv1.ListEditionsRequest]) (*connect.Response[clockkeeperv1.ListEditionsResponse], error) {
	editions := h.registry.Editions()
	result := make([]*clockkeeperv1.Edition, len(editions))
	for i, e := range editions {
		charIDs := make([]string, len(e.Characters))
		for j, c := range e.Characters {
			charIDs[j] = c.ID
		}
		result[i] = &clockkeeperv1.Edition{
			Id:           e.ID,
			Name:         e.Name,
			CharacterIds: charIDs,
		}
	}

	return connect.NewResponse(&clockkeeperv1.ListEditionsResponse{
		Editions: result,
	}), nil
}
