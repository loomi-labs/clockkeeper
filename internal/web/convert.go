package web

import (
	"github.com/loomi-labs/clockkeeper/ent"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

func characterToProto(c *botc.Character) *clockkeeperv1.Character {
	return &clockkeeperv1.Character{
		Id:                 c.ID,
		Name:               c.Name,
		Team:               string(c.Team),
		Edition:            c.Edition,
		Ability:            c.Ability,
		Setup:              c.Setup,
		Reminders:          c.Reminders,
		RemindersGlobal:    c.RemindersGlobal,
		FirstNightReminder: c.FirstNightReminder,
		OtherNightReminder: c.OtherNightReminder,
		FirstNight:         int32(c.FirstNight),
		OtherNight:         int32(c.OtherNight),
	}
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

func entGameToProto(g *ent.Game, registry *botc.Registry) *clockkeeperv1.Game {
	chars := registry.Characters(g.SelectedRoles)
	travellerChars := registry.Characters(g.SelectedTravellers)

	var dist *clockkeeperv1.RoleDistribution
	if d, err := botc.DistributionForPlayerCount(g.PlayerCount); err == nil {
		adjusted, _ := botc.ApplySetupModifiers(d, chars)
		dist = &clockkeeperv1.RoleDistribution{
			Townsfolk: int32(adjusted.Townsfolk),
			Outsiders: int32(adjusted.Outsiders),
			Minions:   int32(adjusted.Minions),
			Demons:    int32(adjusted.Demons),
		}
	}

	// Collect reminder tokens from both regular and traveller characters.
	var tokens []*clockkeeperv1.ReminderToken
	allChars := append(chars, travellerChars...)
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

	return &clockkeeperv1.Game{
		Id:                          int64(g.ID),
		ScriptId:                    int64(g.ScriptID),
		PlayerCount:                 int32(g.PlayerCount),
		TravellerCount:              int32(g.TravellerCount),
		SelectedRoleIds:             g.SelectedRoles,
		SelectedTravellerIds:        g.SelectedTravellers,
		State:                       string(g.State),
		Distribution:                dist,
		SelectedCharacters:          charactersToProto(chars),
		SelectedTravellerCharacters: charactersToProto(travellerChars),
		ReminderTokens:              tokens,
	}
}
