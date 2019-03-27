package rrl

import (
	"fmt"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupZones(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl`,
			shouldErr: false,
			expected: RRL{
				Zones: []string{},
			},
		},
		{input: `rrl {}`,
			shouldErr: false,
			expected: RRL{
				Zones: []string{},
			},
		},
		{input: `rrl example.com {}`,
			shouldErr: false,
			expected: RRL{
				Zones: []string{"example.com."},
			},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		rrl, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if fmt.Sprintf("%v", rrl.Zones) != fmt.Sprintf("%v", test.expected.Zones) {
			t.Errorf("Test %v: Expected Zones '%v' but found: '%v'", i, test.expected.Zones, rrl.Zones)
		}
	}
}

func TestSetupAllowances(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl`,
			shouldErr: false,
			expected:  defaultRRL(),
		},
		{input: `rrl {
                   responses-per-second 10
                 }`,
			shouldErr: false,
			expected: RRL{
				responsesInterval: second / 10,
				nodataInterval:    second / 10,
				nxdomainsInterval: second / 10,
				referralsInterval: second / 10,
				errorsInterval:    second / 10,
			},
		},
		{input: `rrl {
                   responses-per-second 10
                   nodata-per-second 5
                   nxdomains-per-second 6
                   referrals-per-second 7
                   errors-per-second 8
                 }`,
			shouldErr: false,
			expected: RRL{
				responsesInterval: second / 10,
				nodataInterval:    second / 5,
				nxdomainsInterval: second / 6,
				referralsInterval: second / 7,
				errorsInterval:    second / 8,
			},
		},
		{input: `rrl {
                   responses-per-second 10 11
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nodata-per-second 10 11
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nxdomains-per-second 10 11
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   referrals-per-second 10 11
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   errors-per-second 10 11
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   responses-per-second -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nodata-per-second -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nxdomains-per-second -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   referrals-per-second -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   errors-per-second -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   responses-per-second abc
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nodata-per-second abc
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   nxdomains-per-second abc
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   referrals-per-second abc
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   errors-per-second abc
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		rrl, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if rrl.responsesInterval != test.expected.responsesInterval {
			t.Errorf("Test %v: Expected responsesInterval %v but found: %v", i, test.expected.responsesInterval, rrl.responsesInterval)
		}
		if rrl.nodataInterval != test.expected.nodataInterval {
			t.Errorf("Test %v: Expected nodataInterval %v but found: %v", i, test.expected.nodataInterval, rrl.nodataInterval)
		}
		if rrl.nxdomainsInterval != test.expected.nxdomainsInterval {
			t.Errorf("Test %v: Expected nxdomainsInterval %v but found: %v", i, test.expected.nxdomainsInterval, rrl.nxdomainsInterval)
		}
		if rrl.referralsInterval != test.expected.referralsInterval {
			t.Errorf("Test %v: Expected referralsInterval %v but found: %v", i, test.expected.referralsInterval, rrl.referralsInterval)
		}
		if rrl.errorsInterval != test.expected.errorsInterval {
			t.Errorf("Test %v: Expected errorsInterval %v but found: %v", i, test.expected.errorsInterval, rrl.errorsInterval)
		}
	}
}

func TestSetupWindow(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl`,
			shouldErr: false,
			expected:  defaultRRL(),
		},
		{input: `rrl {
                   window 10
                 }`,
			shouldErr: false,
			expected: RRL{
				window: 10 * second,
			},
		},
		{input: `rrl {
                   window 0
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   window five
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   window 1 2
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		rrl, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if rrl.window != test.expected.window {
			t.Errorf("Test %v: Expected window %v but found: %v", i, test.expected.window, rrl.window)
		}
	}
}

func TestSetupPrefixes(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl`,
			shouldErr: false,
			expected:  defaultRRL(),
		},
		{input: `rrl {
                   ipv4-prefix-length 25
                   ipv6-prefix-length 57
                 }`,
			shouldErr: false,
			expected: RRL{
				ipv4PrefixLength: 25,
				ipv6PrefixLength: 57,
			},
		},
		{input: `rrl {
                   ipv4-prefix-length 33
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv6-prefix-length 129
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv4-prefix-length -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv6-prefix-length -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv4-prefix-length 1 2
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv6-prefix-length 3 4
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv4-prefix-length orange
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   ipv6-prefix-length banana
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		rrl, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if string(rrl.ipv4PrefixLength) != string(test.expected.ipv4PrefixLength) {
			t.Errorf("Test %v: Expected ipv4PrefixLength %v but found: %v", i, string(test.expected.ipv4PrefixLength), string(rrl.ipv4PrefixLength))
		}
		if string(rrl.ipv6PrefixLength) != string(test.expected.ipv6PrefixLength) {
			t.Errorf("Test %v: Expected ipv6PrefixLength %v but found: %v", i, string(test.expected.ipv6PrefixLength), string(rrl.ipv6PrefixLength))
		}
	}
}

func TestSetupTableSize(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl`,
			shouldErr: false,
			expected:  defaultRRL(),
		},
		{input: `rrl {
                   max-table-size 500000
                 }`,
			shouldErr: false,
			expected:  RRL{maxTableSize: 500000},
		},
		{input: `rrl {
                   max-table-size -1
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   max-table-size 1 3
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
		{input: `rrl {
                   max-table-size ginormous
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		rrl, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if rrl.maxTableSize != test.expected.maxTableSize {
			t.Errorf("Test %v: Expected maxTableSize %v but found: %v", i, test.expected.maxTableSize, rrl.maxTableSize)
		}
	}
}

func TestSetupInvalidOption(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  RRL
	}{
		{input: `rrl {
                   blah
                 }`,
			shouldErr: true,
			expected:  RRL{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		_, err := rrlParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}
	}
}
