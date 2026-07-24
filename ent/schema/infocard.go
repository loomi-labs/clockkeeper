package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
)

// InfoCard holds the schema definition for a user-created custom info card.
// Standard info cards are computed on the frontend from game state and are
// never stored; only reusable custom cards live here.
type InfoCard struct {
	ent.Schema
}

func (InfoCard) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (InfoCard) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").NotEmpty(),
		field.String("body").Default(""),
		field.JSON("character_ids", []string{}).Optional().Default([]string{}),
		field.Int("user_id"),
		field.Int("sort_order").Default(0),
	}
}

func (InfoCard) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("info_cards").Field("user_id").Required().Unique(),
	}
}
