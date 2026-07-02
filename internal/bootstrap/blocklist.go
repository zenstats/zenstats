package bootstrap

import (
	"context"

	"github.com/zenstats/zenstats/internal/event"
)

// InitBlocklist initializes the spam referrer domain blocklist.
// Pulls the latest list from Matomo first, falls back to compile-time embedded list on failure.
func InitBlocklist() {
	event.InitSpamBlocklist(context.Background())
}
