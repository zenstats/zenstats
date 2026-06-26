package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SharedLink holds the schema definition for the SharedLink entity.
type SharedLink struct {
	ent.Schema
}

// Fields of the SharedLink.
func (SharedLink) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.String("name").
			MaxLen(255).
			NotEmpty(),
		field.String("slug").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.String("password_hash").
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

// Edges of the SharedLink.
func (SharedLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("shared_links").
			Field("site_id").
			Unique().
			Required().
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

// Indexes of the SharedLink.
func (SharedLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id"),
		index.Fields("slug").Unique(),
	}
}
