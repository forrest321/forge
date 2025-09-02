package handlers

import (
	"github.com/typicalfo/forge/backend/internal/config"
)

type ConfigProvider interface {
	GetAll() (config.Values, error)
}

func (h *APIHandlers) WithConfigStore(store ConfigProvider) *APIHandlers {
	_h := *h
	_h.configStore = store
	return &_h
}
