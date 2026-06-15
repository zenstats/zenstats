package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserConfig holds the schema definition for the UserConfig entity.
type UserConfig struct {
	ent.Schema
}

// Fields of the UserConfig.
func (UserConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("user_id").
			Unique().
			Comment("关联用户ID"),
		field.Int64("group_id").
			Comment("关联用户组/套餐ID"),
		field.String("status").
			Default("active").
			MaxLen(20).
			Comment("用户状态：active, suspended, expired"),
		field.Time("expires_at").
			Optional().
			Nillable().
			Comment("过期时间（可选，用于付费套餐）"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the UserConfig.
func (UserConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_config").
			Field("user_id").
			Unique().
			Required(),
		edge.From("group", UserGroup.Type).
			Ref("user_configs").
			Field("group_id").
			Unique().
			Required(),
	}
}

// Indexes of the UserConfig.
func (UserConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("group_id"),
	}
}
