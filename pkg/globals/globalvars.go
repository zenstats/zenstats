package globals

import (
	"sync"

	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"
)

var (
	mu sync.Mutex

	global_queue *generic.DynamicQueue[*model.EventRequest]

	db *postgresql.Client
)

func SetQueue(queue *generic.DynamicQueue[*model.EventRequest]) {
	mu.Lock()
	defer mu.Unlock()

	global_queue = queue
}

func GetQueue() *generic.DynamicQueue[*model.EventRequest] {
	mu.Lock()
	defer mu.Unlock()
	return global_queue
}

func SetDB(client *postgresql.Client) {
	mu.Lock()
	defer mu.Unlock()

	db = client
}

func GetDB() *postgresql.Client {
	return db
}
