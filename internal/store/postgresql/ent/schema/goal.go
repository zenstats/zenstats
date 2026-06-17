package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Goal holds the schema definition for the Goal entity.
type Goal struct {
	ent.Schema
}

// Fields of the Goal.
func (Goal) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.Text("event_name").Optional(),
		field.Text("page_path").Optional(),
		field.Text("display_name"),
		field.JSON("custom_props", map[string]string{}).Optional(),
	}
}

// Edges of the Goal.
func (Goal) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("goals").
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

// Indexes of the Goal.
func (Goal) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "display_name").Unique(),
	}
}
