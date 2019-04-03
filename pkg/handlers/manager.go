package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sapcc/wham/pkg/api"
	"github.com/sapcc/wham/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	opts config.Options
	ctx  context.Context
	log  *log.Entry
}
type HandlerFactory func(ctx context.Context) (Handler, error)

var handlerFactories = make(map[string]HandlerFactory)

//NewManager creates new manager object
func New(ctx context.Context, c config.Options) *Manager {
	contextLogger := log.WithFields(log.Fields{
		"component": "manager",
	})
	manager := &Manager{
		c,
		ctx,
		contextLogger,
	}

	return manager
}

func Register(name string, factory HandlerFactory) {
	if factory == nil {
		log.Panicf("Handler factory %s does not exist.", name)
	}
	_, registered := handlerFactories[name]
	if registered {
		log.Errorf("Handler factory %s already registered. Ignoring.", name)
	}
	handlerFactories[name] = factory
}

func (m Manager) CreateHandler(handlerName string) (Handler, error) {

	handlerFactory, ok := handlerFactories[handlerName]
	if !ok {
		availableHandlers := m.getHandlers()
		return nil, fmt.Errorf(fmt.Sprintf("Invalid Handler name. Must be one of: %s", strings.Join(availableHandlers, ", ")))
	}

	// Run the factory with the configuration.
	return handlerFactory(m.ctx)
}

func (m Manager) getHandlers() []string {
	availableHandlers := make([]string, len(handlerFactories))
	for k := range handlerFactories {
		availableHandlers = append(availableHandlers, k)
	}
	return availableHandlers
}

// Run starts the manager and its handlers
func (m Manager) Start(wg *sync.WaitGroup, handlers []string) {
	defer wg.Done()
	wg.Add(1)
	api := api.NewAPI(m.opts)

	for _, name := range handlers {
		log.Info("loading handler: ", name)
		h, err := m.CreateHandler(name)

		if err != nil {
			log.Error(err)
		}
		go h.Run(api, wg)
	}

	go func() {
		if err := api.Serve(); err != nil {
			log.Fatal("API failed with", err)
		}
	}()
}
