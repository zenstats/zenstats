package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SiteMembership holds the schema definition for the SiteMembership entity.
type SiteMembership struct {
	ent.Schema
}

// Fields of the SiteMembership.
func (SiteMembership) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("site_id"),
		field.Int64("user_id"),
		field.Enum("role").
			Values("owner", "admin", "viewer").
			Default("owner"),
	}
}

// Edge of the SiteMembership.
func (SiteMembership) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("site_memberships").
			Field("site_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
		edge.From("user", User.Type).
			Ref("site_memberships").
			Field("user_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
	}
}

// Indexes of the SiteMembership.
func (SiteMembership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "user_id").Unique(),
	}
}
