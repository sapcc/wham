package handlers

import (
	"sync"

	"github.com/sapcc/wham/pkg/api"
)

type Handler interface {
	Run(a *api.API, wg *sync.WaitGroup) error
}
