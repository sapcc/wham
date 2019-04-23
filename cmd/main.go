package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sapcc/wham/pkg/config"
	"github.com/sapcc/wham/pkg/handlers"
	"github.com/sapcc/wham/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

type (
	// just an example alert store. in a real hook, you would do something useful
	alertStore struct {
		sync.Mutex
	}

	responseJSON struct {
		Status  int
		Message string
	}
)

var opts config.Options

func init() {
	flag.StringVar(&opts.DebugLevel, "debug-level", "info", "To set Log Level: development or production")
	flag.IntVar(&opts.MetricPort, "metric-port", 9090, "Prometheus metric port")
	flag.IntVar(&opts.ListenPort, "listen-port", 8080, "Webhook listen port")
	flag.StringVar(&opts.ConfigFilePath, "config-file", "/etc/config/wham.yaml", "Path to the config file")
	flag.Parse()

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	lvl, err := log.ParseLevel(opts.DebugLevel)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}

	manager := handlers.New(ctx, opts)

	cfg, err := config.GetConfig(opts)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	go manager.Start(wg, cfg)
	go metrics.Serve(opts)

	defer func() {
		signal.Stop(sigs)
		cancel()
	}()

	select {
	case <-sigs:
		cancel()
	case <-ctx.Done():
	}

	wg.Wait()
}
