package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"time"
)

type HeatmapSnapshot struct{ ent.Schema }

func (HeatmapSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable(), field.Int64("site_id"),
		field.String("hostname").MaxLen(253), field.String("path").MaxLen(2000),
		field.String("version").MaxLen(120), field.String("variant").MaxLen(120),
		field.Int("viewport_width"), field.Int("viewport_height"),
		field.Int("document_width"), field.Int("document_height"),
		field.Int("image_width"), field.Int("image_height"),
		field.String("file_key").MaxLen(80), field.String("content_type").MaxLen(30),
		field.Int("file_size"), field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("expires_at"),
	}
}
func (HeatmapSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("site", Site.Type).Ref("heatmap_snapshots").Field("site_id").Unique().Required().Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}
func (HeatmapSnapshot) Indexes() []ent.Index { return []ent.Index{index.Fields("site_id")} }
