package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/death"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	"github.com/loomi-labs/clockkeeper/ent/schema"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

func (h *ClockKeeperServiceHandler) ListGames(ctx context.Context, req *connect.Request[clockkeeperv1.ListGamesRequest]) (*connect.Response[clockkeeperv1.ListGamesResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	games, err := h.db.Game.Query().
		Where(game.UserID(u.ID)).
		WithScript().
		WithPhases(func(q *ent.PhaseQuery) {
			q.WithDeaths()
		}).
		Order(ent.Desc(game.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		slog.Error("list games failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	summaries := make([]*clockkeeperv1.GameSummary, len(games))
	for i, g := range games {
		summaries[i] = entGameToSummary(g)
	}

	return connect.NewResponse(&clockkeeperv1.ListGamesResponse{
		Games: summaries,
	}), nil
}

func (h *ClockKeeperServiceHandler) CreateGame(ctx context.Context, req *connect.Request[clockkeeperv1.CreateGameRequest]) (*connect.Response[clockkeeperv1.CreateGameResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

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

	// Verify script exists and get name for default game name.
	script, err := h.db.Script.Get(ctx, int(req.Msg.ScriptId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
		}
		slog.Error("get script for game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	defaultName := fmt.Sprintf("%s - %s", script.Name, time.Now().Format("Jan 2"))

	// Auto-populate fabled/lorics from the script, and select travellers up to travellerCount.
	var scriptTravellers []string
	var extraCharacters []string
	for _, c := range h.registry.Characters(script.CharacterIds) {
		switch c.Team {
		case botc.TeamTraveller:
			scriptTravellers = append(scriptTravellers, c.ID)
		case botc.TeamFabled, botc.TeamLoric:
			extraCharacters = append(extraCharacters, c.ID)
		}
	}
	// Only select up to travellerCount travellers from the script.
	selectedTravellers := scriptTravellers
	if len(selectedTravellers) > travellerCount {
		selectedTravellers = selectedTravellers[:travellerCount]
	}
	if selectedTravellers == nil {
		selectedTravellers = []string{}
	}
	if extraCharacters == nil {
		extraCharacters = []string{}
	}

	g, err := h.db.Game.Create().
		SetName(defaultName).
		SetUserID(u.ID).
		SetScriptID(int(req.Msg.ScriptId)).
		SetPlayerCount(int(req.Msg.PlayerCount)).
		SetTravellerCount(travellerCount).
		SetSelectedRoles([]string{}).
		SetSelectedTravellers(selectedTravellers).
		SetExtraCharacters(extraCharacters).
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
	g, err := h.getOwnedGame(ctx, int(req.Msg.Id))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.GetGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) RandomizeRoles(ctx context.Context, req *connect.Request[clockkeeperv1.RandomizeRolesRequest]) (*connect.Response[clockkeeperv1.RandomizeRolesResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Get the script's character pool.
	script, err := h.db.Script.Get(ctx, g.ScriptID)
	if err != nil {
		slog.Error("get script for randomize failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	chars := h.registry.Characters(script.CharacterIds)
	result, err := botc.RandomizeRoles(chars, g.PlayerCount)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	// Select travellers from the script, capped to the game's traveller count.
	var scriptTravellers []string
	for _, c := range chars {
		if c.Team == botc.TeamTraveller {
			scriptTravellers = append(scriptTravellers, c.ID)
		}
	}
	rand.Shuffle(len(scriptTravellers), func(i, j int) {
		scriptTravellers[i], scriptTravellers[j] = scriptTravellers[j], scriptTravellers[i]
	})
	selectedTravellers := scriptTravellers
	if len(selectedTravellers) > g.TravellerCount {
		selectedTravellers = selectedTravellers[:g.TravellerCount]
	}
	if selectedTravellers == nil {
		selectedTravellers = []string{}
	}

	// Auto-select 3 demon bluffs from not-in-play good characters.
	bluffs := botc.SelectDemonBluffs(chars, result.SelectedIDs, 3)

	// Convert bag substitutions to schema type for storage.
	bagSubs := make([]schema.GameBagSubstitution, len(result.BagSubstitutions))
	for i, bs := range result.BagSubstitutions {
		bagSubs[i] = schema.GameBagSubstitution{
			CausedByID:    bs.CausedByID,
			CausedByName:  bs.CausedByName,
			CharacterID:   bs.CharacterID,
			CharacterName: bs.CharacterName,
			Team:          string(bs.Team),
		}
	}

	g, err = g.Update().
		SetSelectedRoles(result.SelectedIDs).
		SetSelectedTravellers(selectedTravellers).
		SetSelectedBluffs(bluffs).
		SetBagSubstitutions(bagSubs).
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
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Validate all role IDs exist in the registry.
	for _, id := range req.Msg.SelectedRoleIds {
		if _, ok := h.registry.Character(id); !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", id))
		}
	}

	// Reconcile bag substitutions: create empty ones for newly-selected setup chars,
	// preserve existing ones, remove ones for deselected chars.
	script, err := h.db.Script.Get(ctx, g.ScriptID)
	if err != nil {
		slog.Error("get script for bag sub reconciliation failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	chars := h.registry.Characters(script.CharacterIds)
	existingBagSubs := make([]botc.BagSubstitution, len(g.BagSubstitutions))
	for i, bs := range g.BagSubstitutions {
		existingBagSubs[i] = botc.BagSubstitution{
			CausedByID:    bs.CausedByID,
			CausedByName:  bs.CausedByName,
			Team:          botc.Team(bs.Team),
			CharacterID:   bs.CharacterID,
			CharacterName: bs.CharacterName,
		}
	}
	reconciledBotc := botc.BagSubstitutionsForRoles(req.Msg.SelectedRoleIds, chars, existingBagSubs)
	reconciledSubs := make([]schema.GameBagSubstitution, len(reconciledBotc))
	for i, bs := range reconciledBotc {
		reconciledSubs[i] = schema.GameBagSubstitution{
			CausedByID:    bs.CausedByID,
			CausedByName:  bs.CausedByName,
			CharacterID:   bs.CharacterID,
			CharacterName: bs.CharacterName,
			Team:          string(bs.Team),
		}
	}

	g, err = g.Update().
		SetSelectedRoles(req.Msg.SelectedRoleIds).
		SetBagSubstitutions(reconciledSubs).
		Save(ctx)
	if err != nil {
		slog.Error("save updated roles failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameRolesResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGameTravellers(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGameTravellersRequest]) (*connect.Response[clockkeeperv1.UpdateGameTravellersResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
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

	// Clean up stale alignment entries.
	newTravellerSet := make(map[string]bool)
	for _, id := range req.Msg.SelectedTravellerIds {
		newTravellerSet[id] = true
	}
	cleanedAlignments := make(map[string]schema.TravellerAlignment)
	for k, v := range g.TravellerAlignments {
		if newTravellerSet[k] {
			cleanedAlignments[k] = v
		}
	}

	// Auto-sync traveller_count to match the list.
	g, err = g.Update().
		SetSelectedTravellers(req.Msg.SelectedTravellerIds).
		SetTravellerCount(len(req.Msg.SelectedTravellerIds)).
		SetTravellerAlignments(cleanedAlignments).
		Save(ctx)
	if err != nil {
		slog.Error("save updated travellers failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameTravellersResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGameExtraCharacters(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGameExtraCharactersRequest]) (*connect.Response[clockkeeperv1.UpdateGameExtraCharactersResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Validate all IDs exist.
	for _, id := range req.Msg.ExtraCharacterIds {
		if _, ok := h.registry.Character(id); !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", id))
		}
	}

	g, err = g.Update().
		SetExtraCharacters(req.Msg.ExtraCharacterIds).
		Save(ctx)
	if err != nil {
		slog.Error("save updated extra characters failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameExtraCharactersResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateTravellerAlignment(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateTravellerAlignmentRequest]) (*connect.Response[clockkeeperv1.UpdateTravellerAlignmentResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Validate role_id is in selected_travellers.
	found := false
	for _, id := range g.SelectedTravellers {
		if id == req.Msg.RoleId {
			found = true
			break
		}
	}
	if !found {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("character %s is not a selected traveller", req.Msg.RoleId))
	}

	// Update alignment map.
	alignments := make(map[string]schema.TravellerAlignment, len(g.TravellerAlignments))
	for k, v := range g.TravellerAlignments {
		alignments[k] = v
	}

	switch req.Msg.Alignment {
	case clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD:
		alignments[req.Msg.RoleId] = schema.AlignmentGood
	case clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_EVIL:
		alignments[req.Msg.RoleId] = schema.AlignmentEvil
	default:
		delete(alignments, req.Msg.RoleId) // UNSPECIFIED = remove = unset
	}

	g, err = g.Update().SetTravellerAlignments(alignments).Save(ctx)
	if err != nil {
		slog.Error("update traveller alignment failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Re-fetch with eager-loaded phases.
	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UpdateTravellerAlignmentResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGameName(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGameNameRequest]) (*connect.Response[clockkeeperv1.UpdateGameNameResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	name := req.Msg.Name
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	g, err = g.Update().SetName(name).Save(ctx)
	if err != nil {
		slog.Error("update game name failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGameNameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdatePlayerCount(ctx context.Context, req *connect.Request[clockkeeperv1.UpdatePlayerCountRequest]) (*connect.Response[clockkeeperv1.UpdatePlayerCountResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("can only change player count during setup"))
	}

	count := int(req.Msg.PlayerCount)
	if count < 5 || count > 15 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("player count must be between 5 and 15, got %d", count))
	}

	g, err = g.Update().SetPlayerCount(count).Save(ctx)
	if err != nil {
		slog.Error("update player count failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdatePlayerCountResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateGrimoireState(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateGrimoireStateRequest]) (*connect.Response[clockkeeperv1.UpdateGrimoireStateResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	positions := make(map[string]schema.GrimoirePosition, len(req.Msg.Positions))
	for id, pos := range req.Msg.Positions {
		positions[id] = schema.GrimoirePosition{X: float64(pos.X), Y: float64(pos.Y)}
	}

	g, err = g.Update().
		SetGrimoirePositions(positions).
		SetGrimoirePlayerNames(req.Msg.PlayerNames).
		SetGrimoireGameNotes(req.Msg.GameNotes).
		SetGrimoireRoundNotes(req.Msg.RoundNotes).
		SetGrimoireReminderAttachments(req.Msg.ReminderAttachments).
		Save(ctx)
	if err != nil {
		slog.Error("update grimoire state failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UpdateGrimoireStateResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateCharacterAlignment(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateCharacterAlignmentRequest]) (*connect.Response[clockkeeperv1.UpdateCharacterAlignmentResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	if !isRoleInGame(g, req.Msg.RoleId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("character %s is not in this game", req.Msg.RoleId))
	}

	alignment := req.Msg.Alignment
	if alignment != "" && alignment != "good" && alignment != "evil" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("alignment must be 'good', 'evil', or empty"))
	}

	targetPhases := phasesFromID(g.Edges.Phases, int(req.Msg.PhaseId), req.Msg.Propagate)
	if len(targetPhases) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("phase not found"))
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	for _, p := range targetPhases {
		alignments := make(map[string]string, len(p.CharacterAlignments))
		for k, v := range p.CharacterAlignments {
			alignments[k] = v
		}
		if alignment == "" {
			delete(alignments, req.Msg.RoleId)
		} else {
			alignments[req.Msg.RoleId] = alignment
		}
		_, err = tx.Phase.UpdateOneID(p.ID).SetCharacterAlignments(alignments).Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("update character alignment failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}

	if err = tx.Commit(); err != nil {
		slog.Error("commit transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UpdateCharacterAlignmentResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateDemonBluffs(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateDemonBluffsRequest]) (*connect.Response[clockkeeperv1.UpdateDemonBluffsResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in setup state"))
	}

	// Validate all bluff IDs exist in the registry.
	for _, id := range req.Msg.BluffIds {
		if _, ok := h.registry.Character(id); !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", id))
		}
	}

	g, err = g.Update().SetSelectedBluffs(req.Msg.BluffIds).Save(ctx)
	if err != nil {
		slog.Error("update demon bluffs failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateDemonBluffsResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateBagSubstitutions(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateBagSubstitutionsRequest]) (*connect.Response[clockkeeperv1.UpdateBagSubstitutionsResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in setup state"))
	}

	subs := make([]schema.GameBagSubstitution, len(req.Msg.BagSubstitutions))
	for i, bs := range req.Msg.BagSubstitutions {
		causedBy, ok := h.registry.Character(bs.CausedById)
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", bs.CausedById))
		}
		char, ok := h.registry.Character(bs.CharacterId)
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", bs.CharacterId))
		}
		subs[i] = schema.GameBagSubstitution{
			CausedByID:    bs.CausedById,
			CausedByName:  causedBy.Name,
			CharacterID:   bs.CharacterId,
			CharacterName: char.Name,
			Team:          string(botc.BagTeamForCharacter(bs.CausedById)),
		}
	}

	g, err = g.Update().SetBagSubstitutions(subs).Save(ctx)
	if err != nil {
		slog.Error("update bag substitutions failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateBagSubstitutionsResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) GetSetupChecklist(ctx context.Context, req *connect.Request[clockkeeperv1.GetSetupChecklistRequest]) (*connect.Response[clockkeeperv1.GetSetupChecklistResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	allCharIDs := make([]string, 0, len(g.SelectedRoles)+len(g.ExtraCharacters))
	allCharIDs = append(allCharIDs, g.SelectedRoles...)
	allCharIDs = append(allCharIDs, g.ExtraCharacters...)
	chars := h.registry.Characters(allCharIDs)

	// Convert stored bag substitutions to botc type.
	var bagSubs []botc.BagSubstitution
	for _, bs := range g.BagSubstitutions {
		bagSubs = append(bagSubs, botc.BagSubstitution{
			CausedByID:    bs.CausedByID,
			CausedByName:  bs.CausedByName,
			Team:          botc.Team(bs.Team),
			CharacterID:   bs.CharacterID,
			CharacterName: bs.CharacterName,
		})
	}

	steps := botc.GenerateSetupChecklist(chars, h.registry, bagSubs, g.SelectedBluffs)

	protoSteps := make([]*clockkeeperv1.SetupStep, len(steps))
	for i, s := range steps {
		protoSteps[i] = &clockkeeperv1.SetupStep{
			Id:             s.ID,
			Title:          s.Title,
			Description:    s.Description,
			RequiresAction: s.RequiresAction,
			CharacterIds:   s.CharacterIDs,
			Editions:       s.Editions,
		}
	}

	return connect.NewResponse(&clockkeeperv1.GetSetupChecklistResponse{
		Steps: protoSteps,
	}), nil
}

func (h *ClockKeeperServiceHandler) GetDistribution(ctx context.Context, req *connect.Request[clockkeeperv1.GetDistributionRequest]) (*connect.Response[clockkeeperv1.GetDistributionResponse], error) {
	d, err := botc.DistributionForPlayerCount(int(req.Msg.PlayerCount))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&clockkeeperv1.GetDistributionResponse{
		Distribution: &clockkeeperv1.RoleDistribution{
			Townsfolk: int32(d.Townsfolk),
			Outsiders: int32(d.Outsiders),
			Minions:   int32(d.Minions),
			Demons:    int32(d.Demons),
		},
	}), nil
}

func (h *ClockKeeperServiceHandler) DeleteGame(ctx context.Context, req *connect.Request[clockkeeperv1.DeleteGameRequest]) (*connect.Response[clockkeeperv1.DeleteGameResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.Id))
	if err != nil {
		return nil, err
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Delete deaths for all phases of this game.
	if _, err := tx.Death.Delete().Where(death.HasPhaseWith(phase.HasGameWith(game.ID(g.ID)))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete deaths failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Delete all phases.
	if _, err := tx.Phase.Delete().Where(phase.HasGameWith(game.ID(g.ID))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete phases failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Delete the game.
	if err := tx.Game.DeleteOneID(g.ID).Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.DeleteGameResponse{}), nil
}

func (h *ClockKeeperServiceHandler) DuplicateGame(ctx context.Context, req *connect.Request[clockkeeperv1.DuplicateGameRequest]) (*connect.Response[clockkeeperv1.DuplicateGameResponse], error) {
	src, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Get script name for the default game name.
	script, err := h.db.Script.Get(ctx, src.ScriptID)
	if err != nil {
		slog.Error("get script for duplicate failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	defaultName := fmt.Sprintf("%s - %s", script.Name, time.Now().Format("Jan 2"))

	g, err := h.db.Game.Create().
		SetName(defaultName).
		SetUserID(src.UserID).
		SetScriptID(src.ScriptID).
		SetPlayerCount(src.PlayerCount).
		SetTravellerCount(src.TravellerCount).
		SetSelectedRoles(src.SelectedRoles).
		SetSelectedTravellers(src.SelectedTravellers).
		SetExtraCharacters(src.ExtraCharacters).
		SetSelectedBluffs(src.SelectedBluffs).
		SetTravellerAlignments(src.TravellerAlignments).
		SetBagSubstitutions(src.BagSubstitutions).
		SetState(game.StateSetup).
		Save(ctx)
	if err != nil {
		slog.Error("duplicate game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Reload with script edge for proto conversion.
	g, err = h.db.Game.Query().
		Where(game.ID(g.ID)).
		WithScript().
		WithPhases(func(q *ent.PhaseQuery) {
			q.WithDeaths()
		}).
		Only(ctx)
	if err != nil {
		slog.Error("reload duplicated game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.DuplicateGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

// getOwnedGame fetches a game by ID with eager-loaded phases+deaths and verifies the current user owns it.
func (h *ClockKeeperServiceHandler) getOwnedGame(ctx context.Context, gameID int) (*ent.Game, error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	g, err := h.db.Game.Query().
		Where(game.ID(gameID)).
		WithPhases(func(q *ent.PhaseQuery) {
			q.WithDeaths().
				Order(ent.Asc(phase.FieldID))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
		}
		slog.Error("get game failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if g.UserID != u.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("game not found"))
	}

	return g, nil
}
