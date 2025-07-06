package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// SearchEngines holds the schema definition for the SearchEngines entity.
type SearchEngines struct {
	ent.Schema
}

// Fields of the SearchEngines.
func (SearchEngines) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("domain").
			Unique().
			NotEmpty().
			MaxLen(255),
		field.String("name").
			MaxLen(255).
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
