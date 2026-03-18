package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
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
		field.Int("user_id"),
		field.Int("script_id"),
		field.Int("player_count").Min(5).Max(15),
		field.Int("traveller_count").Min(0).Max(20).Default(0),
		field.JSON("selected_roles", []string{}),
		field.JSON("selected_travellers", []string{}),
		field.Enum("state").
			Values("setup", "in_progress", "completed").
			Default("setup"),
	}
}

func (Game) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("games").Field("user_id").Required().Unique(),
		edge.From("script", Script.Type).Ref("games").Field("script_id").Required().Unique(),
	}
}
