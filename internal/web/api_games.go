package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

func (h *ClockKeeperServiceHandler) CreateGame(ctx context.Context, req *connect.Request[clockkeeperv1.CreateGameRequest]) (*connect.Response[clockkeeperv1.CreateGameResponse], error) {
	// Validate player count.
	if _, err := botc.DistributionForPlayerCount(int(req.Msg.PlayerCount)); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Validate traveller count.
	travellerCount := int(req.Msg.TravellerCount)
	if travellerCount < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("traveller count must be non-negative"))
	}
	if int(req.Msg.PlayerCount)+travellerCount > 25 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("total player count (players + travellers) must not exceed 25"))
	}

	// Verify script exists.
	if _, err := h.db.Script.Get(ctx, int(req.Msg.ScriptId)); err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
		}
		slog.Error("get script for game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err := h.db.Game.Create().
		SetScriptID(int(req.Msg.ScriptId)).
		SetPlayerCount(int(req.Msg.PlayerCount)).
		SetTravellerCount(travellerCount).
		SetSelectedRoles([]string{}).
		SetSelectedTravellers([]string{}).
		SetState(game.StateSetup).
		Save(ctx)
	if err != nil {
		slog.Error("create game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.CreateGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) GetGame(ctx context.Context, req *connect.Request[clockkeeperv1.GetGameRequest]) (*connect.Response[clockkeeperv1.GetGameResponse], error) {
	g, err := h.db.Game.Get(ctx, int(req.Msg.Id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.GetGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) RandomizeRoles(ctx context.Context, req *connect.Request[clockkeeperv1.RandomizeRolesRequest]) (*connect.Response[clockkeeperv1.RandomizeRolesResponse], error) {
	g, err := h.db.Game.Get(ctx, int(req.Msg.GameId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game for randomize failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Get the script's character pool.
	script, err := h.db.Script.Get(ctx, g.ScriptID)
	if err != nil {
		slog.Error("get script for randomize failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	chars := h.registry.Characters(script.CharacterIds)
	selected, err := botc.RandomizeRoles(chars, g.PlayerCount)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	// Pick random travellers if traveller_count > 0.
	var selectedTravellers []string
	if g.TravellerCount > 0 {
		var travellers []*botc.Character
		for _, c := range chars {
			if c.Team == botc.TeamTraveller {
				travellers = append(travellers, c)
			}
		}
		rand.Shuffle(len(travellers), func(i, j int) {
			travellers[i], travellers[j] = travellers[j], travellers[i]
		})
		pick := min(g.TravellerCount, len(travellers))
		for i := range pick {
			selectedTravellers = append(selectedTravellers, travellers[i].ID)
		}
	}
	if selectedTravellers == nil {
		selectedTravellers = []string{}
	}

	g, err = g.Update().
		SetSelectedRoles(selected).
		SetSelectedTravellers(selectedTravellers).
		SetTravellerCount(len(selectedTravellers)).
		Save(ctx)
	if err != nil {
		slog.Error("save randomized roles failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.RandomizeRolesResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGameRoles(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGameRolesRequest]) (*connect.Response[clockkeeperv1.UpdateGameRolesResponse], error) {
	g, err := h.db.Game.Get(ctx, int(req.Msg.GameId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game for update roles failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = g.Update().SetSelectedRoles(req.Msg.SelectedRoleIds).Save(ctx)
	if err != nil {
		slog.Error("save updated roles failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameRolesResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGameTravellers(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGameTravellersRequest]) (*connect.Response[clockkeeperv1.UpdateGameTravellersResponse], error) {
	g, err := h.db.Game.Get(ctx, int(req.Msg.GameId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game for update travellers failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Validate all IDs are traveller-team characters.
	for _, id := range req.Msg.SelectedTravellerIds {
		c, ok := h.registry.Character(id)
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", id))
		}
		if c.Team != botc.TeamTraveller {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not a traveller", c.Name))
		}
	}

	// Validate total doesn't exceed 25.
	total := g.PlayerCount + len(req.Msg.SelectedTravellerIds)
	if total > 25 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("total players (%d) exceeds maximum of 25", total))
	}

	// Auto-sync traveller_count to match the list.
	g, err = g.Update().
		SetSelectedTravellers(req.Msg.SelectedTravellerIds).
		SetTravellerCount(len(req.Msg.SelectedTravellerIds)).
		Save(ctx)
	if err != nil {
		slog.Error("save updated travellers failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameTravellersResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) GetSetupChecklist(ctx context.Context, req *connect.Request[clockkeeperv1.GetSetupChecklistRequest]) (*connect.Response[clockkeeperv1.GetSetupChecklistResponse], error) {
	g, err := h.db.Game.Get(ctx, int(req.Msg.GameId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game for checklist failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	chars := h.registry.Characters(g.SelectedRoles)
	steps := botc.GenerateSetupChecklist(chars, h.registry)

	protoSteps := make([]*clockkeeperv1.SetupStep, len(steps))
	for i, s := range steps {
		protoSteps[i] = &clockkeeperv1.SetupStep{
			Id:             s.ID,
			Title:          s.Title,
			Description:    s.Description,
			RequiresAction: s.RequiresAction,
		}
	}

	return connect.NewResponse(&clockkeeperv1.GetSetupChecklistResponse{
		Steps: protoSteps,
	}), nil
}
