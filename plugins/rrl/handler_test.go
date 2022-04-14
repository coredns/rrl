package rrl

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/plugin/test"
)

func TestServeDNSRateLimit(t *testing.T) {
	tc := test.Case{Qname: "example.com", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess}

	rrl := defaultRRL()
	rrl.Next = test.HandlerFunc(fixedAnswer)
	rrl.Zones = []string{"example.com."}
	rrl.window = 2 * second
	rrl.responsesInterval = second
	rrl.initTable()

	ctx := context.TODO()

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := rrl.ServeDNS(ctx, w, tc.Msg())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	// ensure that the message was written to the client
	if w.Len == 0 {
		t.Errorf("expected message to be written to client")
	}

	// redo the query a couple of times to deplete balance to negative
	for i := 0; i < 2; i++ {
		w = dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := rrl.ServeDNS(ctx, w, tc.Msg())
		if err == nil {
			t.Error("expected rate limit error, got no error")
		}
	}

	// ensure that the last message was not written to the client
	if w.Len != 0 {
		t.Errorf("expected message to be dropped")
	}
}

func TestServeDNStcp(t *testing.T) {
	tc := test.Case{Qname: "example.com", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess}

	rrl := defaultRRL()
	rrl.Next = test.HandlerFunc(fixedAnswer)
	rrl.Zones = []string{"example.com."}
	rrl.window = 2 * second
	rrl.responsesInterval = second
	rrl.initTable()

	ctx := context.TODO()

	var w *dnstest.Recorder

	// deplete balance to what would be negative if we were rate limiting
	for i := 0; i < 3; i++ {
		w = dnstest.NewRecorder(&test.ResponseWriter{TCP: true})
		_, err := rrl.ServeDNS(ctx, w, tc.Msg())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	}

	// ensure that the last message was written to the client
	if w.Len == 0 {
		t.Errorf("expected message to be written to client")
	}

}

func TestServeDNSForeignZone(t *testing.T) {
	tc := test.Case{Qname: "example.com", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess}

	rrl := defaultRRL()
	rrl.Next = test.HandlerFunc(fixedAnswer)
	rrl.Zones = []string{"not.example.com."}
	rrl.window = 2 * second
	rrl.responsesInterval = second
	rrl.initTable()

	ctx := context.TODO()

	var w *dnstest.Recorder

	// deplete balance to what would be negative if we were rate limiting
	for i := 0; i < 3; i++ {
		w = dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := rrl.ServeDNS(ctx, w, tc.Msg())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	}

	// ensure that the last message was written to the client
	if w.Len == 0 {
		t.Errorf("expected message to be written to client")
	}

}
func TestServeDNSZeroAllowance(t *testing.T) {
	tc := test.Case{Qname: "example.com", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess}

	rrl := defaultRRL()
	rrl.Next = test.HandlerFunc(fixedAnswer)
	rrl.Zones = []string{"example.com."}
	rrl.window = 2 * second
	rrl.responsesInterval = 0 // zero allowance should disable rate limiting
	rrl.initTable()

	ctx := context.TODO()

	var w *dnstest.Recorder

	// deplete balance to what would be negative if we were rate limiting
	for i := 0; i < 3; i++ {
		w = dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := rrl.ServeDNS(ctx, w, tc.Msg())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	}

	// ensure that the last message was written to the client
	if w.Len == 0 {
		t.Errorf("expected message to be written to client")
	}

}

func TestServeDNSRateLimitSlips(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		testServeDNSRateLimitSlip(t, 0, 0)
	})
	t.Run("1", func(t *testing.T) {
		testServeDNSRateLimitSlip(t, 1, 10)
	})
	t.Run("2", func(t *testing.T) {
		testServeDNSRateLimitSlip(t, 2, 5)
	})
	t.Run("5", func(t *testing.T) {
		testServeDNSRateLimitSlip(t, 5, 2)
	})
	t.Run("10", func(t *testing.T) {
		testServeDNSRateLimitSlip(t, 10, 1)
	})
}

func testServeDNSRateLimitSlip(t *testing.T, slipRatio uint, expectedSlips int) {
	tc := test.Case{Qname: "example.com", Qtype: dns.TypeA, Rcode: dns.RcodeSuccess}

	rrl := defaultRRL()
	rrl.Next = test.HandlerFunc(fixedAnswer)
	rrl.Zones = []string{"example.com."}
	rrl.window = 2 * second
	rrl.responsesInterval = second
	rrl.slipRatio = slipRatio
	rrl.initTable()

	ctx := context.TODO()

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := rrl.ServeDNS(ctx, w, tc.Msg())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	// ensure that the message was written to the client
	if w.Len == 0 {
		t.Errorf("expected message to be written to client")
	}

	var responses []*dns.Msg
	for i := 0; i < 10; i++ {
		w = dnstest.NewRecorder(&test.ResponseWriter{})
		rrl.ServeDNS(ctx, w, tc.Msg())
		if w.Msg == nil {
			continue
		}
		responses = append(responses, w.Msg)
	}

	for _, r := range responses {
		if !r.Truncated {
			t.Errorf("Slipped message not marked truncated")
		}
		if len(r.Answer) != 0 {
			t.Errorf("Slipped message Answer section is not empty")
		}
	}

	if len(responses) != expectedSlips {
		t.Errorf("expected %v messages to slip, got %v", expectedSlips, len(responses))
	}
}

func fixedAnswer(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	r.Answer = []dns.RR{test.A("example.com.	5	IN	A	1.2.3.4")}
	w.WriteMsg(r)
	return dns.RcodeSuccess, nil
}
