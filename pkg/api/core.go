package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sapcc/wham/pkg/config"
)

type API struct {
	*mux.Router
	opts config.Options
}

func NewAPI(opts config.Options) *API {
	api := &API{
		mux.NewRouter().StrictSlash(false),
		opts,
	}

	return api
}

func (a *API) Serve() error {
	host := fmt.Sprintf("0.0.0.0:%d", a.opts.ListenPort)
	return http.ListenAndServe(host, a)
}

func (a *API) addRoute(pathPrefix, method, path string, handleFunc http.HandlerFunc) {
	a.PathPrefix(pathPrefix).Methods(method, http.MethodOptions).Path(path).HandlerFunc(handleFunc)
}

// AddRoute adds a new route to the v1 API
func (a *API) AddRoute(method, path string, handleFunc http.HandlerFunc) {
	a.addRoute("/alerts", method, path, handleFunc)
}
