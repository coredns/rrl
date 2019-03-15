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
func (rrl *RRL) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (rcode int, err error) {
	state := request.Request{W: w, Req: r}

	// immediately pass to next plugin if the request is over tcp
	if state.Proto() == "tcp" {
		return plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, w, r)
	}

	// immediately pass to next plugin if the request not in the rrl zones
	zone := plugin.Zones(rrl.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, w, r)
	}

	// create a non-writer, because we need to look at the response before writing to the client
	nw := nonwriter.New(w)
	rcode, err = plugin.NextOrFailure(rrl.Name(), rrl.Next, ctx, nw, r)

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
		log.Debugf("dropped response to %v for \"%v\" %v (token='%v', balance=%f.1)", nw.RemoteAddr().String(), nw.Msg.Question[0].String(), dns.RcodeToString[nw.Msg.Rcode], t, b)
		// always return success, to prevent writing of error statuses to client
		return dns.RcodeSuccess, nil
	}

	if err != nil {
		log.Warningf("%v", err)
	}

	// write response to client
	err = w.WriteMsg(nw.Msg)
	return rcode, err
}
