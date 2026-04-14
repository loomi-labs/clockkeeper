package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shifty11/clockkeeper/ent/schema/mixin"
)

// Death tracks when a role dies during a game phase.
type Death struct {
	ent.Schema
}

func (Death) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (Death) Fields() []ent.Field {
	return []ent.Field{
		field.Int("phase_id"),
		field.String("role_id").NotEmpty(),
		field.Bool("ghost_vote").Default(true),
	}
}

func (Death) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("phase", Phase.Type).Ref("deaths").Field("phase_id").Required().Unique(),
	}
}

func (Death) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id").Edges("phase").Unique(),
	}
}
