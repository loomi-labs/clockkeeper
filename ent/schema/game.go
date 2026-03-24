package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
)

// TravellerAlignment represents the good/evil alignment of a traveller in a game.
type TravellerAlignment string

// GrimoirePosition stores the x/y position of a token on the grimoire canvas.
type GrimoirePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// GameBagSubstitution represents an extra token needed in the physical bag.
type GameBagSubstitution struct {
	CausedByID    string `json:"caused_by_id"`
	CausedByName  string `json:"caused_by_name"`
	CharacterID   string `json:"character_id"`
	CharacterName string `json:"character_name"`
}

const (
	AlignmentUnset TravellerAlignment = ""
	AlignmentGood  TravellerAlignment = "good"
	AlignmentEvil  TravellerAlignment = "evil"
)

// Game holds the schema definition for a game session.
type Game struct {
	ent.Schema
}

func (Game) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (Game) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Default(""),
		field.Int("user_id"),
		field.Int("script_id"),
		field.Int("player_count").Min(5).Max(15),
		field.Int("traveller_count").Min(0).Max(20).Default(0),
		field.JSON("selected_roles", []string{}),
		field.JSON("selected_travellers", []string{}),
		field.JSON("extra_characters", []string{}).Optional().Default([]string{}),
		field.JSON("selected_bluffs", []string{}).Optional().Default([]string{}),
		field.JSON("traveller_alignments", map[string]TravellerAlignment{}).Optional().Default(map[string]TravellerAlignment{}),
		field.JSON("grimoire_positions", map[string]GrimoirePosition{}).Optional().Default(map[string]GrimoirePosition{}),
		field.JSON("grimoire_player_names", map[string]string{}).Optional().Default(map[string]string{}),
		field.JSON("grimoire_game_notes", map[string]string{}).Optional().Default(map[string]string{}),
		field.JSON("grimoire_round_notes", map[string]string{}).Optional().Default(map[string]string{}),
		field.JSON("bag_substitutions", []GameBagSubstitution{}).Optional().Default([]GameBagSubstitution{}),
		field.Enum("state").
			Values("setup", "in_progress", "completed").
			Default("setup"),
	}
}

func (Game) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("games").Field("user_id").Required().Unique(),
		edge.From("script", Script.Type).Ref("games").Field("script_id").Required().Unique(),
		edge.To("phases", Phase.Type),
	}
}
