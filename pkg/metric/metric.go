package metric

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Counter:   A counter metric always increases
// Gauge:     A gauge metric can increase or decrease
// Histogram: A histogram metric can increase or descrease to track sampled
//            observations over time

const (
	namespace = "helloworld"
	subsystem = "microservice_x"
)

var (
	labels = []string{
		"tenant_id",
		"foo_id",
	}
)

var (
	FooCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "foo_counter",
		Help:      "The number of foo that have been created",
		Namespace: namespace,
		Subsystem: subsystem,
	}, labels)

	FooGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "foo_gauge",
		Help:      "The number of foo that are current",
		Namespace: namespace,
		Subsystem: subsystem,
	}, labels)

	FooHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "foo_histogram",
		Help:      "The number of foo seen",
		Namespace: namespace,
		Subsystem: subsystem,
		Buckets:   []float64{0, 1, 10, 100, 1000, 10000},
	}, labels)
)

func Handler() http.Handler {
	return promhttp.Handler()
}
