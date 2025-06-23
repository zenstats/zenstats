package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Funnel holds the schema definition for the Funnel entity.
type Funnel struct {
	ent.Schema
}

// Fields of the Funnel.
func (Funnel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.String("name").MaxLen(255),
	}
}

// Edges of the Funnel.
func (Funnel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("funnels").
			Field("site_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
		edge.To("funnel_steps", FunnelStep.Type),
	}
}

// Indexes of the Funnel.
func (Funnel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "name").Unique(),
	}
}
