package rrl

import (
	"context"
	"strconv"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

func BenchmarkBuildToken(b *testing.B) {
	rrl := RRL{
		ipv4PrefixLength: 24,
		ipv6PrefixLength: 56,
	}
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		rrl.buildToken(rTypeResponse, dns.TypeA, "example.org.", "101.102.103.104:4321")
	}
}

func BenchmarkDebit(b *testing.B) {
	rrl := RRL{
		window:            15 * second,
		responsesInterval: second / 10,
		maxTableSize:      10000,
	}
	rrl.initTable()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		rrl.debit(10, "0/0/101.102.103.0/"+strconv.Itoa(i%10000)+".example.org")
	}
}

func BenchmarkServeDNS(b *testing.B) {
	rrl := RRL{
		Zones:             []string{"example.org."},
		Next:              backendHandler(),
		window:            15 * second,
		ipv4PrefixLength:  24,
		ipv6PrefixLength:  56,
		responsesInterval: second / 10,
		nxdomainsInterval: second / 10,
		errorsInterval:    second / 10,
		maxTableSize:      1000,
	}
	rrl.initTable()

	ctx := context.TODO()

	names := []string{"a", "b", "c", "d", "e", "f", "A", "B", "C", "D", "E", "F", "0", "1", "2", "3", "4", "5"}

	var reqs []*dns.Msg
	for _, q := range names {
		m := new(dns.Msg)
		m.SetQuestion(q+".example.org.", dns.TypeA)
		reqs = append(reqs, m)
		m2 := new(dns.Msg)
		m2.SetQuestion(q+".example.org.", dns.TypeAAAA)
		reqs = append(reqs, m2)
	}

	j := 0
	l := len(reqs)
	b.ReportAllocs()
	b.StartTimer()

	b.Run("with-rrl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rrl.ServeDNS(ctx, &test.ResponseWriter{}, reqs[j])
			j = (j + 1) % l
		}
	})

	b.Run("without-rrl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rrl.Next.ServeDNS(ctx, &test.ResponseWriter{}, reqs[j])
			j = (j + 1) % l
		}
	})
}

// backendHandler returns a response based on the first character of the qname.
//   a-z: NOERROR (NODATA)
//   A-Z: NXDOMAIN
//   0-9: SERVFAIL
// Only rcode matters in these benchmarks, the records in the response are
// not important so we don't need to return any records.
func backendHandler() plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Response = true
		m.RecursionAvailable = true

		qname := m.Question[0].Name
		if byte(qname[0]) >= 65 && byte(qname[0]) < 90 {
			m.Rcode = dns.RcodeNameError
			w.WriteMsg(m)
			return dns.RcodeSuccess, nil
		} else if byte(qname[0]) >= 48 && byte(qname[0]) < 58 {
			m.Rcode = dns.RcodeServerFailure
			w.WriteMsg(m)
			return dns.RcodeSuccess, nil
		}
		m.Rcode = dns.RcodeSuccess
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
}
