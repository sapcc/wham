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

var conf config.Config

func init() {
	flag.StringVar(&conf.AppEnv, "APP_ENV", "development", "To set Log Level: development or production")
	flag.StringVar(&conf.Version, "OS_VERSION", "v0.0.1", "Wham Version")
	flag.IntVar(&conf.MetricPort, "metric-port", 9090, "Prometheus metric port")
	flag.Parse()

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}

	manager, err := handlers.NewManager(ctx, conf)
	if err != nil {
		log.Error(err)
		cancel()
		os.Exit(1)
	}
	go manager.Run(wg)
	go metrics.Serve(conf)

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
