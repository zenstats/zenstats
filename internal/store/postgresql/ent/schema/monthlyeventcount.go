package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MonthlyEventCount struct {
	ent.Schema
}

func (MonthlyEventCount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.Int64("user_id").
			Comment("关联用户ID"),
		field.Int("year").
			Comment("年份"),
		field.Int("month").
			Comment("月份"),
		field.Int64("count").
			Default(0).
			Comment("事件数"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (MonthlyEventCount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("monthly_event_counts").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (MonthlyEventCount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "year", "month").Unique(),
		index.Fields("year", "month"),
	}
}
