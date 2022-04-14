package rrl

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/rrl/plugins/rrl/cache"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
)

// RRL performs response rate limiting
type RRL struct {
	Next  plugin.Handler
	Zones []string

	window int64

	ipv4PrefixLength int
	ipv6PrefixLength int

	responsesInterval int64
	nodataInterval    int64
	nxdomainsInterval int64
	referralsInterval int64
	errorsInterval    int64

	requestsInterval int64

	slipRatio uint

	reportOnly bool

	maxTableSize int

	table *cache.Cache
}

// ResponseAccount holds accounting for a category of response
type ResponseAccount struct {
	allowTime     int64 // Next response is allowed if current time >= allowTime
	slipCountdown uint  // When at 1, a dropped response slips through instead of being dropped
}

// Theses constants are categories of response types
const (
	rTypeResponse = 0
	rTypeNodata   = 1
	rTypeNxdomain = 2
	rTypeReferral = 3
	rTypeError    = 4
)

// responseType returns the RRL response type for a response
func responseType(m *dns.Msg) byte {
	if len(m.Answer) > 0 {
		return rTypeResponse
	} else if m.Rcode == dns.RcodeNameError {
		return rTypeNxdomain
	} else if m.Rcode == dns.RcodeSuccess {
		if len(m.Ns) > 0 && m.Ns[0].Header().Rrtype == dns.TypeNS {
			return rTypeReferral
		}
		return rTypeNodata
	} else {
		return rTypeError
	}
}

// allowanceForRtype returns allowed response interval for the given rtype
func (rrl *RRL) allowanceForRtype(rtype uint8) int64 {
	switch rtype {
	case rTypeResponse:
		return rrl.responsesInterval
	case rTypeNodata:
		return rrl.nodataInterval
	case rTypeNxdomain:
		return rrl.nxdomainsInterval
	case rTypeReferral:
		return rrl.referralsInterval
	case rTypeError:
		return rrl.errorsInterval
	}
	return -1
}

// initTable creates a new cache table and sets the cache eviction function
func (rrl *RRL) initTable() {
	rrl.table = cache.New(rrl.maxTableSize)
	// This eviction function returns true if the allowance is >= max value (window)
	rrl.table.SetEvict(func(el *interface{}) bool {
		ra, ok := (*el).(ResponseAccount)
		if !ok {
			return true
		}
		return time.Now().UnixNano()-ra.allowTime >= rrl.window
	})
}

// responseToToken returns a token string for the response in writer
func (rrl *RRL) responseToToken(nw *nonwriter.Writer, rtype byte) string {
	var name string
	if rtype == rTypeNxdomain || rtype == rTypeReferral {
		// for these types we index on the authoritative domain, not the full qname
		// if there is no auth section, dont index on name at all (treat all identical)
		if len(nw.Msg.Ns) > 0 {
			name = nw.Msg.Ns[0].Header().Name
		}
	} else {
		name = nw.Msg.Question[0].Name
	}
	return rrl.buildToken(rtype, nw.Msg.Question[0].Qtype, name, nw.RemoteAddr().String())
}

// buildToken returns a token string for the given inputs
func (rrl *RRL) buildToken(rtype uint8, qtype uint16, name, remoteAddr string) string {
	// "Per BIND" references below are copied from the BIND 9.11 Manual
	// https://ftp.isc.org/isc/bind9/cur/9.11/doc/arm/Bv9ARM.pdf
	prefix := rrl.addrPrefix(remoteAddr)
	rtypestr := strconv.FormatUint(uint64(rtype), 10)
	switch rtype {
	case rTypeResponse:
		// Per BIND: All non-empty responses for a valid domain name (qname) and record type (qtype) are identical
		qtypeStr := strconv.FormatUint(uint64(qtype), 10)
		return strings.Join([]string{prefix, rtypestr, qtypeStr, name}, "/")
	case rTypeNodata:
		// Per BIND: All empty (NODATA) responses for a valid domain, regardless of query type, are identical.
		return strings.Join([]string{prefix, rtypestr, "", name}, "/")
	case rTypeNxdomain:
		// Per BIND: Requests for any and all undefined subdomains of a given valid domain result in NXDOMAIN errors
		// and are identical regardless of query type.
		return strings.Join([]string{prefix, rtypestr, "", name}, "/")
	case rTypeReferral:
		// Per BIND: Referrals or delegations to the server of a given domain are identical.
		qtypeStr := strconv.FormatUint(uint64(qtype), 10)
		return strings.Join([]string{prefix, rtypestr, qtypeStr, name}, "/")
	case rTypeError:
		// Per BIND: All requests that result in DNS errors other than NXDOMAIN, such as SERVFAIL and FORMERR, are
		// identical regardless of requested name (qname) or record type (qtype).
		return strings.Join([]string{prefix, rtypestr, "", ""}, "/")
	}
	return ""
}

// debit will update an existing response account in the rrl table and recalculate the current balance,
// or if the response account does not exist, it will add it.
func (rrl *RRL) debit(allowance int64, t string) (int64, bool, error) {

	type balances struct {
		balance int64
		slip    bool
	}
	result := rrl.table.UpdateAdd(t,
		// the 'update' function updates the account and returns the new balance
		func(el *interface{}) interface{} {
			ra := (*el).(*ResponseAccount)
			if ra == nil {
				return nil
			}
			now := time.Now().UnixNano()
			balance := now - ra.allowTime - allowance
			if balance >= second {
				// positive balance can't exceed 1 second
				balance = second - allowance
			} else if balance < -rrl.window {
				// balance can't be more negative than window
				balance = -rrl.window
			}
			ra.allowTime = now - balance
			if balance > 0 || ra.slipCountdown == 0 {
				return balances{balance, false}
			}
			if ra.slipCountdown == 1 {
				ra.slipCountdown = rrl.slipRatio
				return balances{balance, true}
			}
			ra.slipCountdown -= 1
			return balances{balance, false}

		},
		// the 'add' function returns a new ResponseAccount for the response type
		func() interface{} {
			ra := &ResponseAccount{
				allowTime:     time.Now().UnixNano() - second + allowance,
				slipCountdown: rrl.slipRatio,
			}
			return ra
		})

	if result == nil {
		return 0, false, nil
	}
	if err, ok := result.(error); ok {
		return 0, false, err
	}
	if b, ok := result.(balances); ok {
		return b.balance, b.slip, nil
	}
	return 0, false, errors.New("unexpected result type")
}

// addrPrefix returns the address prefix of the net.Addr style address string (e.g. 1.2.3.4:1234 or [1:2::3:4]:1234)
func (rrl *RRL) addrPrefix(addr string) string {
	i := strings.LastIndex(addr, ":")
	ip := net.ParseIP(addr[:i])
	if ip.To4() != nil {
		ip = ip.Mask(net.CIDRMask(rrl.ipv4PrefixLength, 32))
		return ip.String()
	}
	ip = net.ParseIP(addr[1 : i-1]) // strip brackets from ipv6 e.g. [2001:db8::1]
	ip = ip.Mask(net.CIDRMask(rrl.ipv6PrefixLength, 128))

	return ip.String()
}
