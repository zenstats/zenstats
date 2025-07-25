package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ShieldRulesIp holds the schema definition for the ShieldRulesIp entity.
type ShieldRulesIp struct {
	ent.Schema
}

// Fields of the ShieldRulesIp.
func (ShieldRulesIp) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.Other("inet", &Inet{}).Optional().SchemaType(map[string]string{
			"postgres": "inet",
			"mysql":    "varchar(39)",
		}),
		field.String("action").Default("deny"),
		field.String("description").Optional(),
		field.String("added_by").Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the ShieldRulesIp.
func (ShieldRulesIp) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("shield_rules_ip").
			Field("site_id").
			Unique().
			Required(),
	}
}

// Indexes of the ShieldRulesIp.
func (ShieldRulesIp) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id", "inet").Unique(),
	}
}
