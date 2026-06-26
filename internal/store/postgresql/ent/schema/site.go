package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Site holds the schema definition for the Site entity.
type Site struct {
	ent.Schema
}

// Fields of the Site.
func (Site) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("domain").
			NotEmpty().
			MaxLen(255).
			Comment("The domain of the site"),
		field.String("remark").
			MaxLen(255).
			Optional().
			Comment("The remark of the site"),

		field.String("timezone").
			Default("UTC").
			MaxLen(50).
			Comment("The timezone of the site"),
		field.Bool("public").
			Default(false),
		field.Time("stats_start_date").
			Optional().
			Nillable().
			Comment("The stats start date of the site"),
		field.Int("ingest_rate_limit_scale_seconds").
			Default(60),
		field.Int("ingest_limit_per_minute").
			Default(1000),
		field.String("allowed_origins").
			MaxLen(2048).
			Optional().
			Comment("Comma-separated allowed origins for event ingestion"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.String("verification_token").
			MaxLen(64).
			Optional().
			Nillable().
			Immutable().
			Comment("Random token for domain verification"),
		field.Bool("is_verified").
			Default(false).
			Comment("Whether the site domain is verified"),
		field.Bool("email_report_weekly").
			Default(true).
			Comment("Whether to send weekly email reports"),
		field.Bool("email_report_monthly").
			Default(true).
			Comment("Whether to send monthly email reports"),
		field.Bool("traffic_alert_enabled").
			Default(false).
			Comment("Whether traffic anomaly alerts are enabled"),
		field.Int("traffic_alert_threshold").
			Default(50).
			Min(10).
			Max(500).
			Comment("Alert threshold percentage (e.g. 50 = 50%% change)"),
		field.String("traffic_alert_recipients").
			MaxLen(1024).
			Optional().
			Nillable().
			Comment("Comma-separated additional alert recipients"),
		field.String("traffic_alert_interval").
			Default("hourly").
			MaxLen(10).
			Comment("Alert comparison interval: hourly or daily"),
		field.Time("verified_at").
			Optional().
			Nillable().
			Comment("When the site was verified"),
	}
}

// Indexes of the Site.
func (Site) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain"),
	}
}

// Edges of the Site.
func (Site) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("funnels", Funnel.Type),
		edge.To("members", User.Type),
		edge.To("goals", Goal.Type),
		edge.To("site_memberships", SiteMembership.Type),
		edge.To("shield_rules_ip", ShieldRulesIp.Type),
		edge.To("shield_rules_hostname", ShieldRulesHostname.Type),
		edge.To("shield_rules_country", ShieldRulesCountry.Type),

		edge.To("shield_rules_referrer", ShieldRulesReferrer.Type),
		edge.To("shared_links", SharedLink.Type),
		edge.To("segments", Segment.Type),
	}
}
