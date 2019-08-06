module github.com/coredns/rrl

go 1.12

require (
	github.com/caddyserver/caddy v1.0.1
	github.com/coredns/coredns v1.6.1
	github.com/miekg/dns v1.1.15
	github.com/pkg/errors v0.8.1
)

replace github.com/DataDog/dd-trace-go v0.6.1 => github.com/datadog/dd-trace-go v0.6.1
