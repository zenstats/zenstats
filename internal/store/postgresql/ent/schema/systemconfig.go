package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type SystemConfig struct {
	ent.Schema
}

func (SystemConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			MaxLen(100).
			Unique().
			Comment("配置键，如 smtp.host"),
		field.Text("value").
			Optional().
			Comment("配置值"),
		field.String("description").
			MaxLen(255).
			Optional().
			Comment("配置描述"),
		field.String("group_name").
			MaxLen(50).
			Default("general").
			Comment("配置分组：general, smtp"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (SystemConfig) Indexes() []ent.Index {
	return nil
}
