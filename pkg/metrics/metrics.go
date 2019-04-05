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
func Serve(opts config.Options) {
	ctxLog := log.WithFields(log.Fields{
		"component": "metrics",
	})
	host := "0.0.0.0"

	ctxLog.Infof("exposing prometheus metrics on port: %d", opts.MetricPort)

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%v", host, opts.MetricPort))
	defer listener.Close()

	if err == nil {
		http.Serve(listener, promhttp.Handler())
	} else {
		ctxLog.Error("exposing prometheus metrics failed", err)
	}
}
