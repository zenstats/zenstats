package bootstrap

import (
	"context"

	"github.com/zenstats/zenstats/internal/event"
)

// InitBlocklist 初始化垃圾 referrer 域名黑名单。
// 优先从 Matomo 远程拉取最新列表，失败则使用编译时嵌入的 fallback。
func InitBlocklist() {
	event.InitSpamBlocklist(context.Background())
}
