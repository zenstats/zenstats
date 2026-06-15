package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// UserGroup holds the schema definition for the UserGroup entity.
type UserGroup struct {
	ent.Schema
}

// Fields of the UserGroup.
func (UserGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("套餐名称"),
		field.Text("description").
			Optional().
			Comment("套餐描述"),
		field.Int("max_sites").
			Default(3).
			Comment("最大站点数限制，-1表示无限制"),
		field.Int("max_monthly_events").
			Default(10000).
			Comment("每月最大事件数限制，-1表示无限制"),
		field.Int("max_api_keys").
			Default(2).
			Comment("最大API Key数限制，-1表示无限制"),
		field.Int("max_sub_accounts").
			Default(0).
			Comment("最大子账号数限制，-1表示无限制"),
		field.Bool("custom_search_engines").
			Default(false).
			Comment("是否允许自定义搜索引擎"),
		field.Bool("is_default").
			Default(false).
			Comment("是否为默认套餐"),
		field.Float("price").
			Default(0).
			Comment("价格（可选，用于展示）"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the UserGroup.
func (UserGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_configs", UserConfig.Type),
	}
}
