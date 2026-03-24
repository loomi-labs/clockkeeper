package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
)

// Phase holds the schema definition for a game phase (night or day).
type Phase struct {
	ent.Schema
}

func (Phase) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (Phase) Fields() []ent.Field {
	return []ent.Field{
		field.Int("game_id"),
		field.Int("round_number").Min(1),
		field.Enum("type").Values("night", "day"),
		field.Bool("is_active").Default(true),
		field.JSON("completed_actions", []string{}).Optional().Default([]string{}),
		field.JSON("character_alignments", map[string]string{}).Optional().Default(map[string]string{}),
	}
}

func (Phase) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("game", Game.Type).Ref("phases").Field("game_id").Required().Unique(),
		edge.To("deaths", Death.Type),
	}
}
