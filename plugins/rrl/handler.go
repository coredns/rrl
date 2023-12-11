package rrl

import (
	"context"
	"errors"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"
)

// Name implements the Handler interface.
func (rrl *RRL) Name() string { return "rrl" }

// ServeDNS implements the Handler interface.
func (rrl *RRL) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	// only limit rates for applied zones
	zone := plugin.Zones(rrl.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, w, r)
	}

	// Limit request rate
	if rrl.requestsInterval != 0 {
		t := rrl.addrPrefix(state.RemoteAddr())
		b, _, err := rrl.debit(rrl.requestsInterval, t) // ignore slip when request limit is exceeded (there is no response to slip)
		// if the balance is negative, drop the request (don't write response to client)
		if b < 0 && err == nil {
			log.Debugf("request rate exceeded from %v (token='%v', balance=%.1f)", state.IP(), t, float64(b)/float64(rrl.requestsInterval))
			RequestsExceeded.WithLabelValues(state.IP()).Add(1)
			// always return success, to prevent writing of error statuses to client
			if !rrl.reportOnly {
				return dns.RcodeSuccess, errReqRateLimit
			}
		}
	}

	// Limit response rate
	// dont limit responses rates for tcp requests
	if state.Proto() == "tcp" {
		return plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, w, r)
	}

	// create a non-writer, because we need to look at the response before writing to the client
	nw := nonwriter.New(w)
	rcode, err := plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, nw, r)
	if !plugin.ClientWrite(rcode) {
		return rcode, err
	}

	// get token for response and debit the balance
	rtype := responseType(nw.Msg)
	t := rrl.responseToToken(ctx, nw, rtype)
	allowance := rrl.allowanceForRtype(rtype)
	// a zero allowance indicates that no RRL should be performed for the response type, so write the response to client
	if allowance == 0 {
		err = w.WriteMsg(nw.Msg)
		return rcode, err
	}
	b, slip, err := rrl.debit(allowance, t)

	// if the balance is negative, drop the response (don't write response to client)
	if b < 0 && err == nil {
		log.Debugf("response rate exceeded to %v for \"%v\" %v (token='%v', balance=%.1f)", nw.RemoteAddr().String(), nw.Msg.Question[0].String(), dns.RcodeToString[nw.Msg.Rcode], t, float64(b)/float64(allowance))
		// always return success, to prevent writing of error statuses to client
		ResponsesExceeded.WithLabelValues(state.IP()).Add(1)
		if !rrl.reportOnly {
			if !slip {
				// drop the response.  Return success, otherwise server will return an error response to client.
				return dns.RcodeSuccess, errRespRateLimit
			}
			// truncate the response to just the header and let it slip through
			nw.Msg.Ns = []dns.RR{}
			nw.Msg.Answer = []dns.RR{}
			nw.Msg.Extra = []dns.RR{}
			nw.Msg.Truncated = true
		}
	}

	if err != nil {
		log.Warningf("%v", err)
	}

	// write response to client
	err = w.WriteMsg(nw.Msg)
	return rcode, err
}

var (
	errReqRateLimit  = errors.New("query rate exceeded the limit")
	errRespRateLimit = errors.New("response rate exceeded the limit")
)
