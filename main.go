package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	_ "github.com/coredns/coredns/core/plugin"
	"github.com/coredns/coredns/coremain"

	_ "github.com/coredns/rrl/plugins/rrl"

)

func init() {
	dnsserver.Directives = append([]string{"rrl"}, dnsserver.Directives...)
}

func main() {
	coremain.Run()
}
