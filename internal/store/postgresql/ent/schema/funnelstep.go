package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FunnelStep holds the schema definition for the FunnelStep entity.
type FunnelStep struct {
	ent.Schema
}

// Fields of the FunnelStep.
func (FunnelStep) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("funnel_id"),
		field.Int64("goal_id"),
		field.Int("step_order"),
	}
}

// Edge of the FunnelStep.
func (FunnelStep) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("goals", Goal.Type).
			Ref("funnel_steps").
			Field("goal_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
		edge.From("funnels", Funnel.Type).
			Ref("funnel_steps").
			Field("funnel_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
	}
}

// Indexes of the FunnelStep.
func (FunnelStep) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("funnel_id", "goal_id").Unique(),
	}
}
