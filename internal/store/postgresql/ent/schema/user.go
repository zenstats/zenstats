package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("email").Unique(),
		field.Bool("email_verified").Default(false),
		field.String("name").Optional(),
		field.Time("last_seen").Default(time.Now),
		field.String("password_hash").Optional(),
		field.String("previous_email").Optional(),
		field.Bytes("totp_secret").Optional(),
		field.Bool("totp_enabled").Default(false),
		field.Time("totp_last_used_at").Optional(),
		field.String("totp_token").Optional(),
		field.Text("notes").Optional(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("api_keys", APIKey.Type),

		edge.To("site_memberships", SiteMembership.Type),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email").Unique(),
	}
}
