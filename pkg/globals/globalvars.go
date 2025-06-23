package globals

import (
	"sync"

	"github.com/aarondl/authboss/v3"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/generic"
)

var (
	mu sync.Mutex

	global_queue *generic.DynamicQueue[*common.EventRequest]

	ab *authboss.Authboss

	db *postgresql.Client
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

func SetAuthboss(authboss *authboss.Authboss) {
	ab = authboss
}

func GetAuthboss() *authboss.Authboss {
	return ab
}

func SetDB(client *postgresql.Client) {
	mu.Lock()
	defer mu.Unlock()

	db = client
}

func GetDB() *postgresql.Client {
	return db
}
