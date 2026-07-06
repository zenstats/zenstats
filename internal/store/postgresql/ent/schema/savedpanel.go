package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SavedPanel holds the schema definition for the SavedPanel entity.
// 它保存一次自定义查询的完整配置（指标 / 维度 / 过滤器 / 排序 / limit 等），
// 便于后续复用，或固定到主看板首页展示。
type SavedPanel struct {
	ent.Schema
}

// Fields of the SavedPanel.
func (SavedPanel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("site_id"),
		field.String("name").
			MaxLen(255).
			NotEmpty(),
		field.String("description").
			MaxLen(500).
			Optional(),
		// config 存储自定义查询的完整配置 JSON（前端 CustomQuery 表单状态）。
		field.Text("config").
			Default("{}"),
		// pinned 表示是否固定到主看板首页展示。
		field.Bool("pinned").
			Default(false),
		// layout 存储该面板在自定义看板网格中的位置与大小 JSON（{x,y,w,h}）。
		field.Text("layout").
			Default("{}"),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the SavedPanel.
func (SavedPanel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).
			Ref("saved_panels").
			Field("site_id").
			Unique().
			Required().
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

// Indexes of the SavedPanel.
func (SavedPanel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("site_id"),
	}
}
