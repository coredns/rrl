package rrl

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Variables declared for monitoring.
var (
	RequestsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "rrl",
		Name:      "requests_dropped_total",
		Help:      "Counter of requests dropped due to QPS limit.",
	}, []string{"client_ip"})

	ResponsesDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "rrl",
		Name:      "responses_dropped_total",
		Help:      "Counter of responses dropped due to QPS limit.",
	}, []string{"client_ip"})
)
