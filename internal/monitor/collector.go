package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bitsbeats/openshift-route-monitor/internal/kube"
)

// Collector implements prometheus.Collector
type Collector struct {
	mw        *kube.MultiWatcher
	descCache descMap
}

// NewCollector creates a prometheus.Collector that montors all Routes from mw
func NewCollector(mw *kube.MultiWatcher) *Collector {
	descCache := newDescMap(mapBuilder{
		"resolved_seconds": {
			"time to resolve hostname",
			func(m *kube.RequestMetrics) (float64, []string) {
				return m.Resolved.Seconds(), []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"connected_seconds": {
			"time to open the connection",
			func(m *kube.RequestMetrics) (float64, []string) { return m.Connected.Seconds(), []string{} },
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"wrote_request_seconds": {
			"time until the full request was sent",
			func(m *kube.RequestMetrics) (float64, []string) {
				return m.WroteRequest.Seconds(), []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"read_first_byte_seconds": {
			"time until first byte was read",
			func(m *kube.RequestMetrics) (float64, []string) {
				return m.ReadFirstByte.Seconds(), []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"read_body_seconds": {
			"time until full body was read",
			func(m *kube.RequestMetrics) (float64, []string) {
				return m.ReadBody.Seconds(), []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"ssl_exires_seconds": {
			"seconds until the ssl expires",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.Expires.IsZero() {
					// no certificate
					return -1, []string{}
				}
				untilExpire := time.Until(m.Expires).Seconds()
				return untilExpire, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"redirect_count": {
			"number of http redirects",
			func(m *kube.RequestMetrics) (float64, []string) {
				return float64(m.RedirectCount), []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"invalid_request_error": {
			"errors during request",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.BodyDownloadErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"connection_error": {
			"errors during connection opening",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.ConnectionErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"body_download_error": {
			"errors during body download",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.BodyDownloadErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"invalid_statuscode_error": {
			"invalid statuscode",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.InvalidStatusCodeErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"invalid_body_regex_error": {
			"invalid regex",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.InvalidBodyRegexErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
		"invalid_body_error": {
			"invalid body",
			func(m *kube.RequestMetrics) (float64, []string) {
				if m.InvalidBodyErr {
					return 1, []string{}
				}
				return 0, []string{}
			},
			[]string{"host", "path", "ssl", "cluster", "uid", "namespace", "name"},
		},
	})
	return &Collector{
		mw:        mw,
		descCache: descCache,
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, v := range c.descCache {
		ch <- v.desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	routes := c.mw.List()
	wg.Add(len(routes))
	for _, r := range routes {
		go func(r *kube.Route) {
			defer wg.Done()
			c.check(r, ch)
		}(r)
	}
	wg.Wait()
}

func (c *Collector) check(r *kube.Route, ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()

	rm := r.Probe(ctx)
	if rm == nil {
		return
	}
	for _, v := range c.descCache {
		pm, err := v.getPromConstMetric(rm)
		if err != nil {
			continue
		}
		ch <- pm
	}
}
