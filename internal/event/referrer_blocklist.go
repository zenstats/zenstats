package event

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 远程数据源：Matomo referrer spam list（每周自动更新）
const remoteBlocklistURL = "https://raw.githubusercontent.com/matomo-org/referrer-spam-list/master/spammers.txt"

//go:embed spammers.txt
var fallbackSpammersData string

var (
	mu          sync.RWMutex
	spammersSet map[string]struct{}
	lastUpdated time.Time
	initOnce    sync.Once
)

// InitSpamBlocklist 初始化垃圾 referrer 列表，启动时优先从远程拉取，失败则用嵌入版本。
func InitSpamBlocklist(ctx context.Context) {
	initOnce.Do(func() {
		loadEmbedded()
		if err := fetchAndReplace(ctx); err != nil {
			slog.Warn("failed to fetch remote spam blocklist, using embedded fallback", "error", err)
		}
	})
}

// RefreshSpamBlocklist 从远程拉取最新列表并热替换（供 cron 定时调用）。
func RefreshSpamBlocklist(ctx context.Context) error {
	return fetchAndReplace(ctx)
}

// IsSpamReferrer 检查 hostname 是否在已知垃圾 referrer 列表中。线程安全。
func IsSpamReferrer(hostname string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if hostname == "" || spammersSet == nil {
		return false
	}
	_, ok := spammersSet[strings.ToLower(hostname)]
	return ok
}

// GetBlocklistStats 返回当前 blocklist 状态（条目数、最后更新时间）。
func GetBlocklistStats() (count int, updated time.Time) {
	mu.RLock()
	defer mu.RUnlock()
	return len(spammersSet), lastUpdated
}

// loadEmbedded 加载嵌入的 spammers.txt 到内存（初始 fallback）。
func loadEmbedded() {
	set := make(map[string]struct{}, 2000)
	scanner := bufio.NewScanner(strings.NewReader(fallbackSpammersData))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[line] = struct{}{}
		count++
	}
	mu.Lock()
	spammersSet = set
	lastUpdated = time.Time{} // 嵌入版本无时间戳
	mu.Unlock()
	slog.Info("loaded embedded spam referrer blocklist", "count", count)
}

// fetchAndReplace 从远程 URL 拉取并热替换内存 Set。
func fetchAndReplace(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteBlocklistURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "ZenStats/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB 上限
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	newSet := make(map[string]struct{}, 2000)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		newSet[line] = struct{}{}
		count++
	}

	mu.Lock()
	spammersSet = newSet
	lastUpdated = time.Now()
	mu.Unlock()

	slog.Info("updated spam referrer blocklist from remote", "count", count, "source", remoteBlocklistURL)
	return nil
}
