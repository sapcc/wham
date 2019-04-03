package handlers

import (
	"context"
	"sync"

	"github.com/sapcc/wham/pkg/api"
	"github.com/sapcc/wham/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	handlers []Handler
	config   config.Config
}

//NewManager creates new manager object
func NewManager(ctx context.Context, c config.Config) (*Manager, error) {
	var handlers []Handler
	bm, err := NewBaremetalHandler(ctx)
	if err != nil {
		log.Fatal("Failed creating handler", err)
		return nil, err
	}
	handlers = append(handlers, bm)
	manager := &Manager{
		handlers,
		c,
	}

	return manager, nil
}

// Run starts the manager and its handlers
func (m Manager) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)
	api := api.NewAPI(m.config)

	for _, handler := range m.handlers {
		go handler.Run(api, wg)
	}

	go func() {
		if err := api.Serve(); err != nil {
			log.Fatal("API failed with", err)
		}
	}()
}
