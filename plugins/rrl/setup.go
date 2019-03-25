package rrl

import (
	"strconv"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/mholt/caddy"
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
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					rps, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if rps < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.responsesInterval = int64(second / rps)
				case "nodata-per-second":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					rps, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if rps < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.nodataInterval = int64(second / rps)
					nodataIntervalSet = true
				case "nxdomains-per-second":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					rps, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if rps < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.nxdomainsInterval = int64(second / rps)
					nxdomainsIntervalSet = true
				case "referrals-per-second":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					rps, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if rps < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.referralsInterval = int64(second / rps)
					referralsIntervalSet = true
				case "errors-per-second":
					args := c.RemainingArgs()
					if len(args) != 1 {
						return nil, c.ArgErr()
					}
					rps, err := strconv.ParseFloat(args[0], 64)
					if err != nil {
						return nil, c.Errf("%v invalid value. %v", c.Val(), err)
					}
					if rps < 0 {
						return nil, c.Errf("%v cannot be negative", c.Val())
					}
					rrl.errorsInterval = int64(second / rps)
					errorsIntervalSet = true
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

const second = 1000000000
