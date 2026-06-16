package repository

import "sync"

var (
	locationOnce sync.Once
	eventOnce    sync.Once
	sessionOnce  sync.Once
	importOnce   sync.Once
)
