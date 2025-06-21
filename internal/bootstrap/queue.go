package bootstrap

import (
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/pkg/generic"
	"github.com/zenstats/zenstats/pkg/globals"
)

func InitWorkQueue() {
	queue := generic.NewQueue[*common.EventRequest](100, 1000)

	globals.SetQueue(queue)
}
