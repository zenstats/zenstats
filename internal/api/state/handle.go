package state

import "github.com/zenstats/zenstats/internal/service"

type StateHandle struct {
	service *service.StateService
}

func NewStateHandle() *StateHandle {
	return &StateHandle{
		service: service.GetStateService(),
	}
}
