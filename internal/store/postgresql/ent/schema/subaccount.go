package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubAccount holds the schema definition for the SubAccount entity.
type SubAccount struct {
	ent.Schema
}

// Fields of the SubAccount.
func (SubAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("parent_user_id").
			Comment("父用户ID"),
		field.String("email").
			MaxLen(255).
			Unique().
			NotEmpty().
			Comment("子账号邮箱"),
		field.String("password_hash").
			MaxLen(255).
			NotEmpty().
			Comment("密码哈希"),
		field.String("name").
			MaxLen(100).
			Optional().
			Comment("名称"),
		field.String("role").
			Default("viewer").
			MaxLen(20).
			Comment("角色：viewer（只读）"),
		field.String("status").
			Default("active").
			MaxLen(20).
			Comment("状态：active, suspended"),
		field.Time("last_seen").
			Optional().
			Comment("最后登录时间"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the SubAccount.
func (SubAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("parent_user", User.Type).
			Ref("sub_accounts").
			Field("parent_user_id").
			Unique().
			Required(),
	}
}

// Indexes of the SubAccount.
func (SubAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_user_id"),
		index.Fields("email").Unique(),
	}
}
