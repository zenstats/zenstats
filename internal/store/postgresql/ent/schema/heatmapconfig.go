package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/zenstats/zenstats/internal/heatmap"
	"time"
)

type HeatmapConfig struct{ ent.Schema }

func (HeatmapConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(), field.Int64("site_id"),
		field.Bool("enabled").Default(false),
		field.JSON("paths", []string{}).Default([]string{}),
		field.JSON("groups", []heatmap.GroupRule{}).Default([]heatmap.GroupRule{}).Annotations(entsql.Default("[]")),
		field.Float("sample_rate").Default(.1), field.Int("max_clicks").Default(100),
		field.Int("raw_days").Default(7), field.Int("aggregate_days").Default(90),
		field.String("timezone").Default("UTC").MaxLen(50),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (HeatmapConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).Ref("heatmap_config").Field("site_id").Unique().Required().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}
func (HeatmapConfig) Indexes() []ent.Index { return []ent.Index{index.Fields("site_id").Unique()} }
