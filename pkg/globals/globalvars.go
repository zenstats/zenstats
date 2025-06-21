package globals

import (
	"sync"

	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/pkg/generic"
)

var (
	mu sync.Mutex

	global_queue *generic.DynamicQueue[*common.EventRequest]
)

func SetQueue(queue *generic.DynamicQueue[*common.EventRequest]) {
	mu.Lock()
	defer mu.Unlock()

	global_queue = queue
}

func GetQueue() *generic.DynamicQueue[*common.EventRequest] {
	mu.Lock()
	defer mu.Unlock()
	return global_queue
}
