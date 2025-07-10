package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ShieldRulesCountry holds the schema definition for the ShieldRulesCountry entity.
type ShieldRulesCountry struct {
	ent.Schema
}

// Fields of the ShieldRulesCountry.
func (ShieldRulesCountry) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.Text("country_code"),
		field.String("action").Default("deny"),
		field.String("added_by").Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the ShieldRulesCountry.
func (ShieldRulesCountry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("shield_rules_country").
			Field("site_id").
			Unique().
			Required(),
	}
}

// Indexes of the ShieldRulesCountry.
func (ShieldRulesCountry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "country_code").Unique(),
	}
}
