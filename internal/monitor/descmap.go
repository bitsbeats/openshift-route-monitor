package monitor

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bitsbeats/openshift-route-monitor/internal/kube"
)

type (
	// mapBuilder is a
	mapBuilder map[string]struct {
		descStr     string
		valueGetter valueGetter
		labels      []string
	}

	valueGetter func(*kube.RequestMetrics) (value float64, additionalLabels []string)
	descValue   struct {
		desc        *prometheus.Desc
		valueGetter valueGetter
	}

	// descMap is a prometheus desc storage and metric generator
	descMap map[string]*descValue
)

// newDescMap generates a new descMap
func newDescMap(mb mapBuilder) (dm descMap) {
	dm = descMap{}
	for k, v := range mb {
		pk := fmt.Sprintf("ormon_%s", k)
		dm[k] = &descValue{
			desc:        prometheus.NewDesc(pk, v.descStr, v.labels, prometheus.Labels{}),
			valueGetter: v.valueGetter,
		}
	}
	return
}

// getPromConstMetric generates the matching prometheus.Metric using
// the confgured valueGetter and the stored *prometheus.Desc
func (dv descValue) getPromConstMetric(rm *kube.RequestMetrics) (pm prometheus.Metric, err error) {
	value, labels := dv.valueGetter(rm)
	labels = append([]string{
		rm.Host,
		rm.Path,
		fmt.Sprintf("%t", rm.SSL),
		rm.Cluster,
		rm.UID,
		rm.Namespace,
		rm.Name,
	}, labels...)
	return prometheus.NewConstMetric(dv.desc, prometheus.GaugeValue, value, labels...)
}
