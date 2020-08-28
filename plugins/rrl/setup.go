package rrl

import (
	"strconv"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("rrl")

func init() {
	caddy.RegisterPlugin("rrl", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	e, err := rrlParse(c)
	if err != nil {
		return plugin.Error("rrl", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	c.OnStartup(func() error {
		metrics.MustRegister(c, RequestsExceeded, ResponsesExceeded)
		return nil
	})

	return nil
}

func defaultRRL() RRL {
	return RRL{
		window:           15 * second,
		ipv4PrefixLength: 24,
		ipv6PrefixLength: 56,
		maxTableSize:     100000,
	}
}

func rrlParse(c *caddy.Controller) (*RRL, error) {
	rrl := defaultRRL()

	var (
		nodataIntervalSet    bool
		nxdomainsIntervalSet bool
		referralsIntervalSet bool
		errorsIntervalSet    bool
	)

	for c.Next() {
		for _, z := range c.RemainingArgs() {
			if strings.Contains(z, "{") {
				continue
			}
			rrl.Zones = append(rrl.Zones, z)
		}
		if len(rrl.Zones) == 0 {
			rrl.Zones = make([]string, len(c.ServerBlockKeys))
			copy(rrl.Zones, c.ServerBlockKeys)
		}
		for i, str := range rrl.Zones {
			rrl.Zones[i] = plugin.Host(str).Normalize()
		}

		if c.NextBlock() {
			for {
				switch c.Val() {
				case "window":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					w, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if w <= 0 {
						return nil, c.Err("window must be greater than zero")
					}
					rrl.window = int64(w * second)
				case "ipv4-prefix-length":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					i, err := strconv.Atoi(c.Val())
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if i <= 0 || i > 32 {
						return nil, c.Errf("%v must be between 1 and 32", c.Val())
					}
					rrl.ipv4PrefixLength = i
				case "ipv6-prefix-length":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					i, err := strconv.Atoi(c.Val())
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if i <= 0 || i > 128 {
						return nil, c.Errf("%v must be between 1 and 128", c.Val())
					}
					rrl.ipv6PrefixLength = i
				case "responses-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.responsesInterval = i
				case "nodata-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.nodataInterval = i
					nodataIntervalSet = true
				case "nxdomains-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.nxdomainsInterval = i
					nxdomainsIntervalSet = true
				case "referrals-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.referralsInterval = i
					referralsIntervalSet = true
				case "errors-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.errorsInterval = i
					errorsIntervalSet = true
				case "requests-per-second":
					i, err := getIntervalArg(c)
					if err != nil {
						return nil, err
					}
					rrl.requestsInterval = i
				case "max-table-size":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					i, err := strconv.Atoi(args[0])
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if i < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.maxTableSize = i
				case "report-only":
					args := c.RemainingArgs()
					if len(args) > 0 {
						return nil, c.ArgErr()
					}
					rrl.reportOnly = true
				default:
					if c.Val() != "}" {
						return nil, c.Errf("unknown property '%s'", c.Val())
					}
				}

				if !c.Next() {
					break
				}
			}
		}

		// If any allowance intervals were not set, default them to responsesInterval
		if !nodataIntervalSet {
			rrl.nodataInterval = rrl.responsesInterval
		}
		if !nxdomainsIntervalSet {
			rrl.nxdomainsInterval = rrl.responsesInterval
		}
		if !referralsIntervalSet {
			rrl.referralsInterval = rrl.responsesInterval
		}
		if !errorsIntervalSet {
			rrl.errorsInterval = rrl.responsesInterval
		}

		// initialize table
		rrl.initTable()

		return &rrl, nil
	}
	return nil, nil
}

func getIntervalArg(c *caddy.Controller) (int64, error) {
	args := c.RemainingArgs()
	if len(args) != 1 {
		return 0, c.ArgErr()
	}
	rps, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, c.Errf("%v invalid value. %v", c.Val(), err)
	}
	if rps < 0 {
		return 0, c.Errf("%v cannot be negative", c.Val())
	}
	if rps == 0.0 {
		return 0, nil
	} else {
		return int64(second / rps), nil
	}
}

const second = 1000000000
