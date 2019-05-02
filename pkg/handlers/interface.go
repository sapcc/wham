package handlers

import (
	"sync"

	"github.com/sapcc/wham/pkg/api"
)

//Handler interface for handlers to implement
type Handler interface {
	Run(a *api.API, wg *sync.WaitGroup) error
}
