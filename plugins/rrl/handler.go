package rrl

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
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
		b, err := rrl.debit(rrl.requestsInterval, t)
		// if the balance is negative, drop the request (don't write response to client)
		if b < 0 && err == nil {
			log.Debugf("request rate exceeded from %v (token='%v', balance=%.1f)", state.IP(), t, float64(b)/float64(rrl.requestsInterval))
			RequestsExceeded.WithLabelValues(state.IP()).Add(1)
			// always return success, to prevent writing of error statuses to client
			if !rrl.reportOnly {
				return dns.RcodeSuccess, nil
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
	if err != nil || nw.Msg == nil {
		return rcode, err
	}

	// get token for response and debit the balance
	rtype := responseType(nw.Msg)
	t := rrl.responseToToken(nw, rtype)
	allowance := rrl.allowanceForRtype(rtype)
	// a zero allowance indicates that no RRL should be performed for the response type, so write the response to client
	if allowance == 0 {
		err = w.WriteMsg(nw.Msg)
		return rcode, err
	}
	b, err := rrl.debit(allowance, t)

	// if the balance is negative, drop the response (don't write response to client)
	if b < 0 && err == nil {
		log.Debugf("response rate exceeded to %v for \"%v\" %v (token='%v', balance=%.1f)", nw.RemoteAddr().String(), nw.Msg.Question[0].String(), dns.RcodeToString[nw.Msg.Rcode], t, float64(b)/float64(allowance))
		// always return success, to prevent writing of error statuses to client
		ResponsesExceeded.WithLabelValues(state.IP()).Add(1)
		return dns.RcodeSuccess, nil
	}

	if err != nil {
		log.Warningf("%v", err)
	}

	// write response to client
	err = w.WriteMsg(nw.Msg)
	return rcode, err
}
