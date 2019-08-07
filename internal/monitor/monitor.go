package monitor

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/bitsbeats/openshift-route-monitor/internal/kube"
)

type (
	// Config holds the Monitor configuration
	Config struct {
		Listen string `yaml:"listen"`
	}

	// Monitor periodically checks routes
	Monitor struct {
		config Config
	}
)

// Create a new Monitor from Config
func New(c Config, mw *kube.MultiWatcher) (m *Monitor, err error) {
	if c.Listen == "" {
		c.Listen = ":9142"
	}
	err = prometheus.Register(NewCollector(mw))
	if err != nil {
		return nil, err
	}
	return &Monitor{config: c}, nil
}

// Run starts the metric server
func (m *Monitor) Run(ctx context.Context, errs chan<- error) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		logrus.Infof("listening on %s", m.config.Listen)
		s := http.Server{Addr: m.config.Listen}
		go func() { errs <- s.ListenAndServe() }()
		<-ctx.Done()
		errs <- s.Shutdown(context.Background())
	}()
}
