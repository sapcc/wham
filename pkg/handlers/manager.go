package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sapcc/wham/pkg/api"
	"github.com/sapcc/wham/pkg/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Manager struct {
	opts config.Options
	ctx  context.Context
	log  *log.Entry
}
type HandlerFactory func(ctx context.Context, config interface{}) (Handler, error)

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

func (m Manager) CreateHandler(name string, handler interface{}) (Handler, error) {

	handlerFactory, ok := handlerFactories[name]
	if !ok {
		availableHandlers := m.getHandlers()
		return nil, fmt.Errorf(fmt.Sprintf("Invalid Handler name. Must be one of: %s", strings.Join(availableHandlers, ", ")))
	}

	// Run the factory with the configuration.
	return handlerFactory(m.ctx, handler)
}

func (m Manager) getHandlers() []string {
	availableHandlers := make([]string, len(handlerFactories))
	for k := range handlerFactories {
		availableHandlers = append(availableHandlers, k)
	}
	return availableHandlers
}

// Run starts the manager and its handlers
func (m Manager) Start(wg *sync.WaitGroup, cfg config.Config) {
	defer wg.Done()
	wg.Add(1)
	api := api.NewAPI(m.opts)

	for name, handler := range cfg.Handlers {
		log.Info("loading handler: ", name)
		h, err := m.CreateHandler(name, handler)

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

func UnmarshalHandler(handlerIn, handlerOut interface{}) error {

	h, err := yaml.Marshal(handlerIn)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(h, handlerOut)
}
