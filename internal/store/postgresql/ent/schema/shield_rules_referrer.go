package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ShieldRulesReferrer holds the schema definition for the ShieldRulesReferrer entity.
// 管理员自定义的垃圾 referrer 域名黑名单。
type ShieldRulesReferrer struct {
	ent.Schema
}

func (ShieldRulesReferrer) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("site_id"),
		field.String("hostname").MaxLen(255).Comment("要屏蔽的 referrer 域名"),
		field.String("action").MaxLen(10).Default("deny").Comment("deny=屏蔽"),
		field.String("description").MaxLen(255).Optional().Comment("备注说明"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (ShieldRulesReferrer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("shield_rules_referrer").
			Field("site_id").
			Unique().
			Required().Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			},
		),
	}
}
