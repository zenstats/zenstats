package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CustomSearchEngine holds the schema definition for the CustomSearchEngine entity.
type CustomSearchEngine struct {
	ent.Schema
}

// Fields of the CustomSearchEngine.
func (CustomSearchEngine) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("user_id").
			Comment("所属用户ID"),
		field.String("domain").
			MaxLen(255).
			NotEmpty().
			Comment("搜索引擎域名，如 google.com"),
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("显示名称，如 Google"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the CustomSearchEngine.
func (CustomSearchEngine) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("custom_search_engines").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the CustomSearchEngine.
func (CustomSearchEngine) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "domain").Unique(),
		index.Fields("user_id"),
	}
}
