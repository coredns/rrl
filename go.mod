module github.com/coredns/rrl

go 1.16

require (
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.8.6
	github.com/miekg/dns v1.1.43
	github.com/prometheus/client_golang v1.11.0
)

replace github.com/DataDog/dd-trace-go v0.6.1 => github.com/datadog/dd-trace-go v0.6.1
