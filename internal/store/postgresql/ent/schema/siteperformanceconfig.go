package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"time"
)

type SitePerformanceConfig struct{ ent.Schema }

func (SitePerformanceConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(), field.Int64("site_id"),
		field.Bool("enabled").Default(false),
		field.JSON("paths", []string{}).Default([]string{"/*"}).Annotations(entsql.Default(`["/*"]`)),
		field.Float("sample_rate").Default(.1), field.Int("raw_days").Default(7),
		field.String("timezone").Default("UTC").MaxLen(50), field.Uint64("config_version").Default(1),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (SitePerformanceConfig) Edges() []ent.Edge {
	return []ent.Edge{edge.From("site", Site.Type).Ref("performance_config").Field("site_id").Unique().Required().Annotations(entsql.Annotation{OnDelete: entsql.Cascade})}
}
func (SitePerformanceConfig) Indexes() []ent.Index {
	return []ent.Index{index.Fields("site_id").Unique()}
}
