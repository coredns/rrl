package rrl

import (
	"testing"
	"time"

	"github.com/coredns/rrl/plugins/rrl/cache"

	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

func TestDebit(t *testing.T) {

	rrl := defaultRRL()
	rrl.window = 5
	rrl.responsesPerSecond = 0
	rrl.nxdomainsPerSecond = 100
	rrl.table = cache.New(rrl.maxTableSize)

	_, err := rrl.debit(rrl.allowanceForRtype(rTypeResponse), "token1")
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	ra, _ := rrl.table.Get("token1")
	bal := ra.(*ResponseAccount).balance
	if bal != rrl.window-1 {
		t.Errorf("expected balance of %v, got %v", rrl.window-1, bal)
	}

	bal, err = rrl.debit(rrl.allowanceForRtype(rTypeResponse), "token1")
	if bal != rrl.window-2 {
		t.Errorf("expected balance of %v, got %v", rrl.window-2, bal)
	}

	_, err = rrl.debit(rrl.allowanceForRtype(rTypeNxdomain), "token2")
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	time.Sleep(time.Second) // sleep 1 second, balance should max out
	bal, err = rrl.debit(rrl.allowanceForRtype(rTypeNxdomain), "token2")
	if bal != rrl.window-1 {
		t.Errorf("expected balance of %v, got %v", rrl.window-1, bal)
	}

}

func TestResponseType(t *testing.T) {
	tests := []struct {
		msg      dns.Msg
		expected byte
	}{
		{
			msg: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
				Answer: []dns.RR{
					test.A("example.com. 5 IN A 1.2.3.4"),
				}},
			expected: rTypeResponse,
		},
		{
			msg: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
				Answer: []dns.RR{},
			},
			expected: rTypeNodata,
		},
		{
			msg: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeNameError},
				Answer: []dns.RR{},
			},
			expected: rTypeNxdomain,
		},
		{
			msg: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeFormatError},
				Answer: []dns.RR{},
			},
			expected: rTypeError,
		},
	}
	for _, c := range tests {
		got := responseType(c.msg)
		if got != c.expected {
			t.Errorf("expected '%v', got '%v'", c.expected, got)
		}
	}
}

func TestAllowanceForRtype(t *testing.T) {
	rtypes := []uint8{rTypeResponse, rTypeNodata, rTypeError, rTypeNxdomain, rTypeReferral}

	rrl := defaultRRL()
	rrl.responsesPerSecond = 100
	rrl.nodataPerSecond = 100
	rrl.nxdomainsPerSecond = 100
	rrl.referralsPerSecond = 100
	rrl.errorsPerSecond = 100

	for _, rtype := range rtypes {
		got := rrl.allowanceForRtype(rtype)
		if got != 100 {
			t.Errorf("expected '%v', got '%v'", 100, got)
		}
	}
}

func TestResponseToToken(t *testing.T) {
	tests := []struct {
		rtype      uint8
		qtype      uint16
		name       string
		remoteAddr string
		expected   string
	}{
		{
			rtype:      rTypeResponse,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/0/1/example.com",
		},
		{
			rtype:      rTypeError,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/4/1/",
		},
	}
	rrl := defaultRRL()
	for _, c := range tests {
		got := rrl.responseToToken(c.rtype, c.qtype, c.name, c.remoteAddr)
		if got != c.expected {
			t.Errorf("expected '%v', got '%v'", c.expected, got)
		}
	}
}

func TestAddrPrefix(t *testing.T) {
	tests := []struct {
		ipv4Prefix, ipv6Prefix int
		remoteAddr, expected   string
	}{
		{
			ipv4Prefix: 24,
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0",
		},
		{
			ipv6Prefix: 56,
			remoteAddr: "[1234:5678::1]:80",
			expected:   "1234:5678::",
		},
		{
			ipv4Prefix: 8,
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.0.0.0",
		},
		{
			ipv6Prefix: 16,
			remoteAddr: "[1234:5678::1]:80",
			expected:   "1234::",
		},
		{
			ipv4Prefix: 32,
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.4",
		},
		{
			ipv6Prefix: 128,
			remoteAddr: "[1234:5678::1]:80",
			expected:   "1234:5678::1",
		},
	}

	rrl := defaultRRL()
	for _, c := range tests {
		rrl.ipv4PrefixLength = c.ipv4Prefix
		rrl.ipv6PrefixLength = c.ipv6Prefix
		got := rrl.addrPrefix(c.remoteAddr)
		if got != c.expected {
			t.Errorf("expected '%v', got '%v'", c.expected, got)
		}
	}
}
