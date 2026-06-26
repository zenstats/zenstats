package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Segment holds the schema definition for the Segment entity.
type Segment struct {
	ent.Schema
}

// Fields of the Segment.
func (Segment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.String("name").
			MaxLen(255).
			NotEmpty(),
		field.Text("filters").
			Default("[]"),
		field.String("description").
			MaxLen(500).
			Optional(),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Segment.
func (Segment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("segments").
			Field("site_id").
			Unique().
			Required().
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

// Indexes of the Segment.
func (Segment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id"),
	}
}
