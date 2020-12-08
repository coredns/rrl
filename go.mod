module github.com/coredns/rrl

go 1.12

require (
	github.com/coredns/caddy v1.1.0
	github.com/coredns/coredns v1.8.0
	github.com/miekg/dns v1.1.34
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
)

replace github.com/DataDog/dd-trace-go v0.6.1 => github.com/datadog/dd-trace-go v0.6.1
