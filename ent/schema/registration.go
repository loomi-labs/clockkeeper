package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
)

// Registration holds the schema definition for a player who joined a game's
// digital token bag. Players register a name from their own device (or via the
// storyteller's shared device) and later see the role assigned to them.
type Registration struct {
	ent.Schema
}

func (Registration) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (Registration) Fields() []ent.Field {
	return []ent.Field{
		field.Int("game_id"),
		// Display form of the name as typed by the player.
		field.String("name").NotEmpty().MaxLen(50),
		// Case/whitespace-folded name, used to enforce uniqueness per game.
		field.String("name_normalized").NotEmpty(),
		field.String("secret_hash").NotEmpty().Sensitive(),
		field.Bool("via_shared_device").Default(false),
		field.Int("left_neighbor_id").Optional(),
		field.Int("right_neighbor_id").Optional(),
		field.String("assigned_role_id").Optional(),
	}
}

func (Registration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("game", Game.Type).Ref("registrations").Field("game_id").Required().Unique(),
	}
}

func (Registration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name_normalized").Edges("game").Unique(),
		index.Fields("secret_hash").Unique(),
	}
}
