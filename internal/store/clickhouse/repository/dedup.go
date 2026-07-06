package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// WithInsertDedupToken 返回携带随机 insert_deduplication_token 的 context。
// 该 token 每次调用唯一，但调用方应在一次批次的所有重试中复用同一个 ctx，
// 使 flush 重试携带相同 token —— ClickHouse 在去重窗口内识别到相同 token 会忽略重复批次，
// 从而让"已提交但 ack 超时后重试"的场景不再重复计数（配合表设置 non_replicated_deduplication_window）。
// 若随机源不可用，回退为不带 token 的原始 ctx（退化为原有行为，不影响写入）。
func WithInsertDedupToken(ctx context.Context) context.Context {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ctx
	}
	return clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"insert_deduplication_token": hex.EncodeToString(b),
	}))
}
