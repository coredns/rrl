package rrl

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Variables declared for monitoring.
var (
	RequestsExceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "rrl",
		Name:      "requests_exceeded_total",
		Help:      "Counter of requests exceeding QPS limit.",
	}, []string{"client_ip"})

	ResponsesExceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "rrl",
		Name:      "responses_exceeded_total",
		Help:      "Counter of responses exceeding QPS limit.",
	}, []string{"client_ip"})
)
