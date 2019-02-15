# CoreDNS RRL Plugin Design Spec

This spec defines a CoreDNS plugin intended to replicate the behavior of
the rate-limit feature in BIND.

In the interest of keeping PRs as small as possible, RRL will first
implement the following minimal set of sub-functions.

* Parsing of Corefile
* Categorization of responses, and accounts debits
* Periodic account credits
* Always block response when account is negative (no slip, i.e. slip = 0)

The following functions, if added, would bring RRL into feature parity
with BIND’s implementation of  rate-limit.  These can be added in
separate PRs, to keep PRs small and easily digestible.

* configurable slip ratio (slipping = send truncated response instead of dropping)
* all-per-second (upper limit RRL at which we stop slipping)
* expose metrics
* exempt-clients (client list / cidrs to exempt from RRL)
* qps-scale (scale down allowances proportionally to current qps load)


## Initial Features Spec

RRL will be delivered as a plugin that accepts a list of zones for which
it will track and enforce rate limiting for UDP responses. e.g.

```
rrl example.org {
    responses-per-second 10
}
```

As a plugin, RRL should take an incoming response, pass it through the
remaining plugins in the plugin chain, and then track/process the result
before responding to the client. For this reason, it needs to be near the
top of the plugin list.

### Plugin Directives

Available configuration options (following naming convention of BIND)…

```
rrl [ZONES...] {
    window SECONDS
    ipv4-prefix-length LENGTH
    ipv6-prefix-length LENGTH
    responses-per-second ALLOWANCE
    nodata-per-second ALLOWANCE
    nxdomains-per-second ALLOWANCE
    referrals-per-second ALLOWANCE
    errors-per-second ALLOWANCE
    max-table-size SIZE
}
```

* `window SECONDS` - defines a rolling window in SECONDS during which response rates are tracked. Default 15

* `ipv4-prefix-length LENGTH` - the prefix LENGTH in bits to use for identifying a ipv4 client. Default 24

* `ipv6-prefix-length LENGTH` - the prefix LENGTH in bits to use for identifying a ipv6 client. Default 56

* `responses-per-second ALLOWANCE` - the number of positive responses allowed per second. Default 0

* `nodata-per-second ALLOWANCE` - the number of empty (NODATA) responses allowed per second. Defaults to responses-per-second.

* `nxdomains-per-second ALLOWANCE` - the number of negative (NXDOMAIN) responses allowed per second. Defaults to responses-per-second.

* `referrals-per-second ALLOWANCE` - the number of negative (NXDOMAIN) responses allowed per second. Defaults to responses-per-second.

* `errors-per-second ALLOWANCE` - the number of error responses allowed per second (excluding NXDOMAIN). Defaults to responses-per-second.

* `max-table-size SIZE` - the maximum number of responses to be tracked at one time. When exceeded, rrl stops rate limiting new responses.


### Record Keeping: ResponseAccounts

RRL tracks responses rates using a table of *ResponseAccounts*.  A
*ResponseAccount* consists of a *token*, and a *balance*.

The *ResponseAccount* *token* uniquely identifies a category of responses and is
comprised the following data extracted from a response:

* Prefix of the client IP (per the  ipv4/6-prefix-length)
* Requested name (qname) excluding response type of error (see response type below)
* Requested type (qtype) excluding response type of error (see response type below)
* Response type (each corresponding to the configurable per-second allowances)
  * response - for positive responses that contain answers
  * nodata - for NODATA responses
  * nxdomain - for NXDOMAIN responses
  * referrals - for referrals or delegations
  * error - for all DNS errors (except NXDOMAIN)

To better protect against attacks using invalid requests, requested name and type are not used in the *token* for error type requests. In other words, all error responses are limited collectively per client, regardless of qname or qtype.

The *ResponseAccount balance* is an integer. When the *balance* becomes negative
for a *ResponseAccount*, any responses that match its *token* are dropped until
the *balance* becomes positive again.
The *ResponseAccount balance* cannot become more positive than `window` and
cannot become more negative than `window` * the `per-second` allowance of the
response type.

*ResponseAccount* balances are credited and debited as outlined below.


### ResponseAccount Credits

Once per second, RRL will credit each existing *ResponseAccount balance* by an amount equal to `per-second` allowance of the the corresponding response type.
If a *ResponseAccount balance* exceeds window, then the *ResponseAccount* should be removed to keep the *ResponseAccount* table from running out of space (should prune less often than every second to reduce thrashing).


### ResponseAccount Debits

*ResponseAccount balances* are debited at the time of sending a UDP response to a client, using the following logic ...

Calculate the *ResponseAccount token* for the response
If the *token* doesn’t exist in the to the *ResponseAccount* table, add the token as follows…
If the `max-table-size` is reached/exceeded, log an error/warning, and send response to client (done)
Add the *token* to the *ResponseAccount* table
Credit the *balance* to maximum - 1.  I.e. `window`  - 1
If the *token* does exist, debit the balance by 1
If *balance* is >= 0, send response to client. (done)
If *balance* is < 0, then drop the response, sending nothing to the client (done)

### ResponseAccount Concurrency

Since the *ResponseAccount* table will be read and written to from parallel threads, locking should be used to ensure data integrity is maintained for all reads/writes.

## Follow up features

### Slip ratio

With a slip ratio defined, responses will not always be dropped when a balance goes negative - we let some slip through.
However, instead of sending the whole response to the client, we send a small truncated response `TC=1`. This truncated response is intended to prompt the client to retry on TCP.
Slipping in this way lets legitimate clients know to use TCP instead without flooding.

Implementing slip ratios will introduce a new plugin directive: slip

```
rrl [ZONES...] {
    slip RATE
}
```

Slip `RATE` is an integer between 0 and 10.  0 means never slip.  1 means always slip. Other numbers mean slip every nth time.
Furthermore, a new property should be added to the *ResponseAccount*, *slipCount*, which defaults to `slip`.  This value should be decremented every time we block a response matching the ResponseAccount token.
When the value hits zero, we should slip a truncated response to the client, and reset the slipCount to slip.
Response types that cannot be truncated such as `REFUSED` and `SERVFAIL` should be leaked in their entirety at the slip rate.

### All-per-second

TBD

### Exposing metrics

TBD

### Exempt-clients

TBD (client list / cidrs to exempt from RRL)

### Qps-scale

TBD (scale down allowances proportionally to current qps load)
