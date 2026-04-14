package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/shifty11/clockkeeper/ent/schema/mixin"
)

// Script holds the schema definition for a saved BotC script.
type Script struct {
	ent.Schema
}

func (Script) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (Script) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("edition").Default("custom"),
		field.JSON("character_ids", []string{}),
		field.Bool("is_system").Default(false),
		field.Int("user_id").Optional().Nillable(),
		field.Time("deleted_at").Optional().Nillable(),
	}
}

func (Script) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("scripts").Field("user_id").Unique(),
		edge.To("games", Game.Type),
	}
}
