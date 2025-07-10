package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserSession holds the schema definition for the UserSession entity.
type UserSession struct {
	ent.Schema
}

// Fields of the UserSession.
func (UserSession) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("user_id"),
		field.Bytes("token"),
		field.String("device").MaxLen(255),
		field.Time("last_used_at"),
		field.Time("timeout_at"),
		field.Time("created_at").Default(time.Now),
	}
}

// Indexes of the UserSession.
func (UserSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token"),
	}
}
