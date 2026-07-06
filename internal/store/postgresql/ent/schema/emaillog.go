package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EmailLog holds the schema definition for the EmailLog entity.
// 记录系统发送的每一封邮件及其发送状态（仅元数据，不含正文）。
type EmailLog struct {
	ent.Schema
}

// Fields of the EmailLog.
func (EmailLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Immutable(),
		field.String("recipient").
			MaxLen(320).
			Comment("收件人邮箱"),
		field.String("subject").
			MaxLen(500).
			Comment("邮件主题"),
		// email_type: verification / password_reset / email_report / traffic_alert
		field.String("email_type").
			MaxLen(50).
			Default("other").
			Comment("邮件类型"),
		// status: sent / failed / skipped
		field.String("status").
			MaxLen(20).
			Comment("发送状态：sent / failed / skipped"),
		field.Text("error").
			Optional().
			Comment("失败时的错误信息"),
		field.Int64("site_id").
			Optional().
			Nillable().
			Comment("关联站点（报告/告警类邮件）"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Indexes of the EmailLog.
func (EmailLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("status"),
		index.Fields("email_type"),
	}
}
