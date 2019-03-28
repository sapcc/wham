package metrics

import (
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sapcc/wham/pkg/config"
	log "github.com/sirupsen/logrus"
)

// Serve ...
func Serve(opts config.Config) {
	logger := log.WithFields(log.Fields{
		"component": "metrics",
	})
	host := "0.0.0.0"

	logger.Infof("exposing prometheus metrics on port: %d", opts.MetricPort)

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%v", host, opts.MetricPort))
	defer listener.Close()

	if err == nil {
		http.Serve(listener, promhttp.Handler())
	} else {
		logger.Error("exposing prometheus metrics failed", err)
	}
}
