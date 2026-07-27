package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/loomi-labs/clockkeeper/ent/schema/mixin"
)

// SpotifyPlaylistSlot is a playlist configured for one game phase.
// The name and image are snapshotted at config time so the panel can render
// without an extra Spotify API round-trip.
type SpotifyPlaylistSlot struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url,omitempty"`
}

// SpotifyConnection holds the schema definition for a user's linked Spotify
// account. At most one connection per user.
type SpotifyConnection struct {
	ent.Schema
}

func (SpotifyConnection) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.TimestampMixin{},
	}
}

func (SpotifyConnection) Fields() []ent.Field {
	return []ent.Field{
		field.String("spotify_user_id").NotEmpty(),
		field.String("display_name").Optional(),
		field.Bool("premium").Default(false),
		// Stored in plaintext: this is a self-hosted single-binary app and the
		// operator already controls the database. App-level AES-GCM encryption
		// with a key from the environment is a possible follow-up.
		field.String("refresh_token").Sensitive().NotEmpty(),
		field.String("access_token").Sensitive().Optional(),
		field.Time("access_token_expires_at").Optional().Nillable(),
		field.JSON("day_playlist", &SpotifyPlaylistSlot{}).Optional(),
		field.JSON("night_playlist", &SpotifyPlaylistSlot{}).Optional(),
		field.JSON("nominations_playlist", &SpotifyPlaylistSlot{}).Optional(),
		field.Int("user_id"),
	}
}

func (SpotifyConnection) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("spotify_connection").Unique().Required().Field("user_id"),
	}
}
