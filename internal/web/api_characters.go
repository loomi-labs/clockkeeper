package web

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/shifty11/clockkeeper/gen/clockkeeper/v1"
	"github.com/shifty11/clockkeeper/internal/botc"
)

func (h *ClockKeeperServiceHandler) GetCharacter(ctx context.Context, req *connect.Request[clockkeeperv1.GetCharacterRequest]) (*connect.Response[clockkeeperv1.GetCharacterResponse], error) {
	c, ok := h.registry.Character(req.Msg.Id)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("character %q not found", req.Msg.Id))
	}
	return connect.NewResponse(&clockkeeperv1.GetCharacterResponse{
		Character: characterToProtoWithJinxes(c, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) ListCharacters(ctx context.Context, req *connect.Request[clockkeeperv1.ListCharactersRequest]) (*connect.Response[clockkeeperv1.ListCharactersResponse], error) {
	var chars []*botc.Character

	teamFilter := protoToTeam[req.Msg.Team] // TEAM_UNSPECIFIED maps to zero value ""

	switch {
	case req.Msg.Edition != "" && teamFilter != "":
		// Filter by both edition and team.
		for _, c := range h.registry.CharactersByEdition(req.Msg.Edition) {
			if c.Team == teamFilter {
				chars = append(chars, c)
			}
		}
	case req.Msg.Edition != "":
		chars = h.registry.CharactersByEdition(req.Msg.Edition)
	case teamFilter != "":
		chars = h.registry.CharactersByTeam(teamFilter)
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
