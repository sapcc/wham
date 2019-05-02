package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sapcc/wham/pkg/api"
	"github.com/sapcc/wham/pkg/config"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

//Manager struct
type Manager struct {
	opts config.Options
	ctx  context.Context
	log  *log.Entry
}

//HandlerFactory function to create handlers
type HandlerFactory func(ctx context.Context, config interface{}) (Handler, error)

var handlerFactories = make(map[string]HandlerFactory)

//New creates new manager object
func New(ctx context.Context, cfg config.Options) *Manager {
	contextLogger := log.WithFields(log.Fields{
		"component": "manager",
	})
	manager := &Manager{
		cfg,
		ctx,
		contextLogger,
	}

	return manager
}

//Register registers a handler to the HandlerFactory
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

//CreateHandler creates handler with name and handler config interface
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

//Start starts the manager and its handlers
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

//UnmarshalHandlerConfig unmarshals handler configs
func UnmarshalHandlerConfig(handlerConfigIn, handlerConfigOut interface{}) error {

	h, err := yaml.Marshal(handlerConfigIn)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(h, handlerConfigOut)
}
