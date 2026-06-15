package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type EmailVerificationToken struct {
	ent.Schema
}

func (EmailVerificationToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("user_id").
			Comment("关联用户ID"),
		field.String("token").
			MaxLen(128).
			Unique().
			Comment("验证 token"),
		field.String("email").
			MaxLen(255).
			Comment("用户邮箱"),
		field.Time("expires_at").
			Comment("过期时间"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (EmailVerificationToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("email_verification_tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (EmailVerificationToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("token").Unique(),
		index.Fields("expires_at"),
	}
}
