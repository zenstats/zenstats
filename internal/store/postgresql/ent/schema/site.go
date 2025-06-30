package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Site holds the schema definition for the Site entity.
type Site struct {
	ent.Schema
}

// Fields of the Site.
func (Site) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("domain").
			Unique().
			NotEmpty().
			MaxLen(255).
			Comment("The domain of the site"),
		field.String("remark").
			MaxLen(255).
			Optional().
			Comment("The remark of the site"),

		field.String("timezone").
			Default("UTC").
			MaxLen(50).
			Comment("The timezone of the site"),
		field.Bool("public").
			Default(false),
		field.Time("stats_start_date").
			Optional().
			Nillable().
			Comment("The stats start date of the site"),
		field.Int("ingest_rate_limit_scale_seconds").
			Default(60),
		field.Int("ingest_limit_per_minute").
			Default(1000),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Indexes of the Site.
func (Site) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain"),
	}
}

// Edges of the Site.
func (Site) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("funnels", Funnel.Type),
		edge.To("members", User.Type),
		edge.To("goals", Goal.Type),
		edge.To("site_memberships", SiteMembership.Type),
	}
}
