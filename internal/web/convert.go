package web

import (
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	"github.com/loomi-labs/clockkeeper/ent/schema"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

var teamToProto = map[botc.Team]clockkeeperv1.Team{
	botc.TeamTownsfolk: clockkeeperv1.Team_TEAM_TOWNSFOLK,
	botc.TeamOutsider:  clockkeeperv1.Team_TEAM_OUTSIDER,
	botc.TeamMinion:    clockkeeperv1.Team_TEAM_MINION,
	botc.TeamDemon:     clockkeeperv1.Team_TEAM_DEMON,
	botc.TeamTraveller: clockkeeperv1.Team_TEAM_TRAVELLER,
	botc.TeamFabled:    clockkeeperv1.Team_TEAM_FABLED,
	botc.TeamLoric:     clockkeeperv1.Team_TEAM_LORIC,
}

var protoToTeam = map[clockkeeperv1.Team]botc.Team{
	clockkeeperv1.Team_TEAM_TOWNSFOLK: botc.TeamTownsfolk,
	clockkeeperv1.Team_TEAM_OUTSIDER:  botc.TeamOutsider,
	clockkeeperv1.Team_TEAM_MINION:    botc.TeamMinion,
	clockkeeperv1.Team_TEAM_DEMON:     botc.TeamDemon,
	clockkeeperv1.Team_TEAM_TRAVELLER: botc.TeamTraveller,
	clockkeeperv1.Team_TEAM_FABLED:    botc.TeamFabled,
	clockkeeperv1.Team_TEAM_LORIC:     botc.TeamLoric,
}

var gameStateToProto = map[game.State]clockkeeperv1.GameState{
	game.StateSetup:      clockkeeperv1.GameState_GAME_STATE_SETUP,
	game.StateInProgress: clockkeeperv1.GameState_GAME_STATE_IN_PROGRESS,
	game.StateCompleted:  clockkeeperv1.GameState_GAME_STATE_COMPLETED,
}

var phaseTypeToProto = map[phase.Type]clockkeeperv1.PhaseType{
	phase.TypeNight: clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT,
	phase.TypeDay:   clockkeeperv1.PhaseType_PHASE_TYPE_DAY,
}

func entDeathToProto(d *ent.Death) *clockkeeperv1.Death {
	return &clockkeeperv1.Death{
		Id:        int64(d.ID),
		RoleId:    d.RoleID,
		PhaseId:   int64(d.PhaseID),
		GhostVote: d.GhostVote,
	}
}

func entPhaseToProto(p *ent.Phase) *clockkeeperv1.Phase {
	proto := &clockkeeperv1.Phase{
		Id:               int64(p.ID),
		RoundNumber:      int32(p.RoundNumber),
		Type:             phaseTypeToProto[p.Type],
		IsActive:         p.IsActive,
		CompletedActions: p.CompletedActions,
	}
	if len(p.CharacterAlignments) > 0 {
		proto.CharacterAlignments = p.CharacterAlignments
	}
	for _, d := range p.Edges.Deaths {
		proto.Deaths = append(proto.Deaths, entDeathToProto(d))
	}
	return proto
}

func characterToProto(c *botc.Character) *clockkeeperv1.Character {
	return &clockkeeperv1.Character{
		Id:                 c.ID,
		Name:               c.Name,
		Team:               teamToProto[c.Team],
		Edition:            c.Edition,
		Ability:            c.Ability,
		Flavor:             c.Flavor,
		Setup:              c.Setup,
		Reminders:          c.Reminders,
		RemindersGlobal:    c.RemindersGlobal,
		FirstNightReminder: c.FirstNightReminder,
		OtherNightReminder: c.OtherNightReminder,
		FirstNight:         int32(c.FirstNight),
		OtherNight:         int32(c.OtherNight),
	}
}

func characterToProtoWithJinxes(c *botc.Character, registry *botc.Registry) *clockkeeperv1.Character {
	proto := characterToProto(c)
	jinxes := registry.Jinxes(c.ID)
	if len(jinxes) > 0 {
		proto.Jinxes = make([]*clockkeeperv1.CharacterJinx, len(jinxes))
		for i, j := range jinxes {
			name := j.ID
			if target, ok := registry.Character(j.ID); ok {
				name = target.Name
			}
			proto.Jinxes[i] = &clockkeeperv1.CharacterJinx{
				CharacterId:   j.ID,
				CharacterName: name,
				Reason:        j.Reason,
			}
		}
	}
	return proto
}

func bagSubstitutionsToProto(subs []schema.GameBagSubstitution) []*clockkeeperv1.BagSubstitution {
	if len(subs) == 0 {
		return nil
	}
	result := make([]*clockkeeperv1.BagSubstitution, len(subs))
	for i, s := range subs {
		result[i] = &clockkeeperv1.BagSubstitution{
			CausedById:    s.CausedByID,
			CausedByName:  s.CausedByName,
			CharacterId:   s.CharacterID,
			CharacterName: s.CharacterName,
			Team:          s.Team,
		}
	}
	return result
}

func charactersToProto(chars []*botc.Character) []*clockkeeperv1.Character {
	result := make([]*clockkeeperv1.Character, len(chars))
	for i, c := range chars {
		result[i] = characterToProto(c)
	}
	return result
}

func entScriptToProto(s *ent.Script, registry *botc.Registry) *clockkeeperv1.Script {
	proto := &clockkeeperv1.Script{
		Id:           int64(s.ID),
		Name:         s.Name,
		Edition:      s.Edition,
		CharacterIds: s.CharacterIds,
		IsSystem:     s.IsSystem,
	}
	if registry != nil {
		proto.Characters = charactersToProto(registry.Characters(s.CharacterIds))
	}
	return proto
}

func entGameToSummary(g *ent.Game) *clockkeeperv1.GameSummary {
	summary := &clockkeeperv1.GameSummary{
		Id:             int64(g.ID),
		Name:           g.Name,
		PlayerCount:    int32(g.PlayerCount),
		TravellerCount: int32(g.TravellerCount),
		State:          gameStateToProto[g.State],
	}

	// Script name from eager-loaded edge.
	if s := g.Edges.Script; s != nil {
		summary.ScriptName = s.Name
	}

	// Phase and death info from eager-loaded phases.
	// Collect unique role IDs to avoid double-counting deaths that propagate across phases.
	deadRoles := make(map[string]struct{})
	for _, p := range g.Edges.Phases {
		if p.IsActive {
			summary.CurrentRound = int32(p.RoundNumber)
			summary.CurrentPhaseType = phaseTypeToProto[p.Type]
		}
		for _, d := range p.Edges.Deaths {
			deadRoles[d.RoleID] = struct{}{}
		}
	}
	summary.DeathCount = int32(len(deadRoles))

	return summary
}

func entGameToProto(g *ent.Game, registry *botc.Registry) *clockkeeperv1.Game {
	chars := registry.Characters(g.SelectedRoles)
	travellerChars := registry.Characters(g.SelectedTravellers)
	extraChars := registry.Characters(g.ExtraCharacters)
	bluffChars := registry.Characters(g.SelectedBluffs)

	var dist *clockkeeperv1.RoleDistribution
	if d, err := botc.DistributionForPlayerCount(g.PlayerCount); err == nil {
		adjusted := botc.ApplySetupModifiers(d, chars).Distribution
		dist = &clockkeeperv1.RoleDistribution{
			Townsfolk: int32(adjusted.Townsfolk),
			Outsiders: int32(adjusted.Outsiders),
			Minions:   int32(adjusted.Minions),
			Demons:    int32(adjusted.Demons),
		}
	}

	// Collect reminder tokens from regular, traveller, and extra characters.
	var tokens []*clockkeeperv1.ReminderToken
	allChars := make([]*botc.Character, 0, len(chars)+len(travellerChars)+len(extraChars))
	allChars = append(allChars, chars...)
	allChars = append(allChars, travellerChars...)
	allChars = append(allChars, extraChars...)
	for _, c := range allChars {
		for _, r := range c.Reminders {
			tokens = append(tokens, &clockkeeperv1.ReminderToken{
				CharacterId:   c.ID,
				CharacterName: c.Name,
				Text:          r,
			})
		}
		for _, r := range c.RemindersGlobal {
			tokens = append(tokens, &clockkeeperv1.ReminderToken{
				CharacterId:   c.ID,
				CharacterName: c.Name,
				Text:          r,
			})
		}
	}

	proto := &clockkeeperv1.Game{
		Id:                          int64(g.ID),
		Name:                        g.Name,
		ScriptId:                    int64(g.ScriptID),
		PlayerCount:                 int32(g.PlayerCount),
		TravellerCount:              int32(g.TravellerCount),
		SelectedRoleIds:             g.SelectedRoles,
		SelectedTravellerIds:        g.SelectedTravellers,
		ExtraCharacterIds:           g.ExtraCharacters,
		State:                       gameStateToProto[g.State],
		Distribution:                dist,
		SelectedCharacters:          charactersToProto(chars),
		SelectedTravellerCharacters: charactersToProto(travellerChars),
		ExtraCharacterDetails:       charactersToProto(extraChars),
		ReminderTokens:              tokens,
		SelectedBluffIds:            g.SelectedBluffs,
		SelectedBluffCharacters:     charactersToProto(bluffChars),
		BagSubstitutions:            bagSubstitutionsToProto(g.BagSubstitutions),
	}

	// Populate traveller alignments.
	if len(g.TravellerAlignments) > 0 {
		proto.TravellerAlignments = make(map[string]clockkeeperv1.TravellerAlignment)
		for id, align := range g.TravellerAlignments {
			switch align {
			case schema.AlignmentGood:
				proto.TravellerAlignments[id] = clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD
			case schema.AlignmentEvil:
				proto.TravellerAlignments[id] = clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_EVIL
			}
		}
	}

	// Populate grimoire state.
	if len(g.GrimoirePositions) > 0 {
		proto.GrimoirePositions = make(map[string]*clockkeeperv1.Position, len(g.GrimoirePositions))
		for id, pos := range g.GrimoirePositions {
			proto.GrimoirePositions[id] = &clockkeeperv1.Position{X: float32(pos.X), Y: float32(pos.Y)}
		}
	}
	if len(g.GrimoirePlayerNames) > 0 {
		proto.GrimoirePlayerNames = g.GrimoirePlayerNames
	}
	if len(g.GrimoireGameNotes) > 0 {
		proto.GrimoireGameNotes = g.GrimoireGameNotes
	}
	if len(g.GrimoireRoundNotes) > 0 {
		proto.GrimoireRoundNotes = g.GrimoireRoundNotes
	}
	if len(g.GrimoireReminderAttachments) > 0 {
		proto.GrimoireReminderAttachments = g.GrimoireReminderAttachments
	}

	// Populate play_state from eager-loaded phases+deaths.
	if phases := g.Edges.Phases; len(phases) > 0 {
		playState := &clockkeeperv1.GamePlayState{}
		for _, p := range phases {
			pp := entPhaseToProto(p)
			playState.Phases = append(playState.Phases, pp)
			if p.IsActive {
				playState.CurrentPhase = pp
				playState.CurrentRound = int32(p.RoundNumber)
			}
			for _, d := range p.Edges.Deaths {
				playState.AllDeaths = append(playState.AllDeaths, entDeathToProto(d))
			}
		}
		proto.PlayState = playState
	}

	return proto
}
