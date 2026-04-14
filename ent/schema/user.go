package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"github.com/shifty11/clockkeeper/ent/schema/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("uuid").Unique().NotEmpty().DefaultFunc(uuid.NewString),
		field.String("discord_id").Optional().Nillable().Unique(),
		field.String("discord_username").Optional().Nillable(),
		field.String("discord_avatar").Optional().Nillable(),
		field.Bool("is_anonymous").Default(false),
		field.Time("last_active_at").Default(time.Now),
		field.JSON("player_presets", []string{}).Optional().Default([]string{}),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("scripts", Script.Type),
		edge.To("games", Game.Type),
	}
}
