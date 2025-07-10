package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ShieldRulesHostname holds the schema definition for the ShieldRulesHostname entity.
type ShieldRulesHostname struct {
	ent.Schema
}

// Fields of the ShieldRulesHostname.
func (ShieldRulesHostname) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.Text("hostname"),
		field.Text("hostname_pattern"),
		field.String("action").Default("allow"),
		field.String("added_by").Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the ShieldRulesHostname.
func (ShieldRulesHostname) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("shield_rules_hostname").
			Field("site_id").
			Unique().
			Required(),
	}
}

// Indexes of the ShieldRulesHostname.
func (ShieldRulesHostname) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "hostname_pattern").Unique(),
	}
}
