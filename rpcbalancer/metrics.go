package rpcbalancer

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	TotalRequests    *prometheus.CounterVec
	DeniedRequests   prometheus.Counter
	InvalidRequests  prometheus.Counter
	BlockHeight      prometheus.Gauge
	FailoverRequests *prometheus.CounterVec
	RequestsByMethod *prometheus.CounterVec
	Healthy          prometheus.Gauge
}

func NewMetrics() *Metrics {
	totalRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "num_requests",
			Help: "Total number of requests processed by method",
		},
		[]string{"method"},
	)

	blockHeight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "block_height",
			Help: "Current block height",
		},
	)

	failoverRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "num_failover_requests",
			Help: "Total number of failover requests by endpoint",
		},
		[]string{"endpoint"},
	)

	healthy := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "service_is_healthy",
			Help: "Current status of node health",
		},
	)

	deniedRequests := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_requests_denied",
			Help: "Total number of requests denied",
		},
	)

	invalidRequests := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_requests_invalid",
			Help: "Total number of invalid requests",
		},
	)

	prometheus.MustRegister(totalRequests)
	prometheus.MustRegister(blockHeight)
	prometheus.MustRegister(failoverRequests)
	prometheus.MustRegister(healthy)
	prometheus.MustRegister(deniedRequests)
	prometheus.MustRegister(invalidRequests)

	return &Metrics{
		TotalRequests:    totalRequests,
		BlockHeight:      blockHeight,
		FailoverRequests: failoverRequests,
		Healthy:          healthy,
		DeniedRequests:   deniedRequests,
		InvalidRequests:  invalidRequests,
	}
}

func (m *Metrics) IncrementTotalRequests(method string) {
	m.TotalRequests.WithLabelValues(method).Inc()
}

func (m *Metrics) IncrementDeniedRequests() {
	m.DeniedRequests.Inc()
}

func (m *Metrics) IncrementInvalidRequests() {
	m.InvalidRequests.Inc()
}

func (m *Metrics) IncrementTotalFailoverRequestsByEndpoint(endpoint string) {
	m.FailoverRequests.WithLabelValues(endpoint).Inc()
}

func (m *Metrics) UpdateHealth() {
	if nodeHealth {
		m.Healthy.Set(1)
	} else {
		m.Healthy.Set(0)
	}
}

func CreateMetricsServer(name string, port string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", promhttp.Handler())
	mux.HandleFunc("/favicon.ico", HandleFavicon)
	server := http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	return &server
}

func RunMetricsServer(wg *sync.WaitGroup, port string) {
	defer wg.Done()

	server := CreateMetricsServer("metrics", port)

	//go CheckHealth(2) // update block height every second; also update node health based on success/failure of response

	log.Fatal(server.ListenAndServe())
}
