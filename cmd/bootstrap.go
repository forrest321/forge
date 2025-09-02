package main

import (
	"fmt"
	"path/filepath"

	"github.com/typicalfo/forge/backend/internal/config"
)

type bootstrap struct {
	ConfigStore *config.Store
}

func initConfig() (*bootstrap, error) {
	cfgPath := filepath.Join("backend", "config.db")
	store, err := config.Ensure(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	return &bootstrap{ConfigStore: store}, nil
}
