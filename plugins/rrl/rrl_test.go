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
	rrl.window = 5 * second
	rrl.responsesInterval = second / 10
	rrl.nxdomainsInterval = second / 100
	rrl.table = cache.New(rrl.maxTableSize)

	_, err := rrl.debit(rrl.allowanceForRtype(rTypeResponse), "token1")
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	ra, _ := rrl.table.Get("token1")
	bal := time.Now().UnixNano() - ra.(*ResponseAccount).allowTime
	if bal < second - rrl.responsesInterval {
		t.Errorf("expected balance not less than %v, got %v", second - rrl.responsesInterval, bal)
	}

	bal, err = rrl.debit(rrl.allowanceForRtype(rTypeResponse), "token1")
	if bal > second - rrl.responsesInterval {
		t.Errorf("expected balance of < %v, got %v", second - rrl.responsesInterval, bal)
	}

	_, err = rrl.debit(rrl.allowanceForRtype(rTypeNxdomain), "token2")
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	time.Sleep(time.Second) // sleep 1 second, balance should max out
	bal, err = rrl.debit(rrl.allowanceForRtype(rTypeNxdomain), "token2")
	if bal != second - rrl.nxdomainsInterval {
		t.Errorf("expected balance of %v, got %v", rrl.window-rrl.nxdomainsInterval, bal)
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
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
				Ns: []dns.RR{
					test.NS("example.com. 12345 IN NS ns1.example.com."),
				},
			},
			expected: rTypeReferral,
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
		got := responseType(&c.msg)
		if got != c.expected {
			t.Errorf("expected '%v', got '%v'", c.expected, got)
		}
	}
}

func TestAllowanceForRtype(t *testing.T) {
	rtypes := []uint8{rTypeResponse, rTypeNodata, rTypeError, rTypeNxdomain, rTypeReferral}

	rrl := defaultRRL()
	rrl.responsesInterval = 100
	rrl.nodataInterval = 100
	rrl.nxdomainsInterval = 100
	rrl.referralsInterval = 100
	rrl.errorsInterval = 100

	for _, rtype := range rtypes {
		got := rrl.allowanceForRtype(rtype)
		if got != 100 {
			t.Errorf("expected '%v', got '%v'", 100, got)
		}
	}
}

func TestBuildToken(t *testing.T) {
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
			rtype:      rTypeNodata,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/1//example.com",
		},
		{
			rtype:      rTypeError,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/4//",
		},
		{
			rtype:      rTypeNxdomain,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/2//example.com",
		},
		{
			rtype:      rTypeReferral,
			qtype:      dns.TypeA,
			name:       "example.com",
			remoteAddr: "1.2.3.4:1234",
			expected:   "1.2.3.0/3/1/example.com",
		},
	}
	rrl := defaultRRL()
	for _, c := range tests {
		got := rrl.buildToken(c.rtype, c.qtype, c.name, c.remoteAddr)
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
