# rrl

## Name

*rrl* - provides BIND-like Response Rate Limiting to help mitigate DNS
amplification attacks. *rrl* also allows request rate limiting. 

## Description

The *rrl* plugin tracks response rates per category of response. 
The category of a given response consists of the following:

* Prefix of the client IP (per the ipv4/6-prefix-length)
* Requested name (qname) excluding response type of error (see response type below)
* Requested type (qtype) excluding response type of error (see response type below)
* Response type (each corresponding to the configurable per-second allowances)
  * response - for positive responses that contain answers
  * nodata - for NODATA responses
  * nxdomain - for NXDOMAIN responses
  * referrals - for referrals or delegations
  * error - for all DNS errors (except NXDOMAIN)


To better protect against attacks using invalid requests, requested name
and type are not categorized separately for error type requests. In other
words, all error responses are limited collectively per client, regardless
of qname or qtype.

Each category has an account balance which is credited at a rate of the
configured *per-second* allowance for that response type, and debited each
time a response in that category would be sent to a client.  When an account
balance is negative, responses in the category are dropped until the balance
goes non-negative.  Account balances cannot be more positive than  *per-second*
allowance, and cannot be more negative than *window* * *per-second* allowance.

The response rate limiting implementation intends to replicate the behavior
of BIND 9's response rate limiting feature.

When limiting requests, the category of each request is determined by the
prefix of the client IP (per the ipv4/6-prefix-length).


## Syntax

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
    slip-ratio N
    requests-per-second ALLOWANCE
    max-table-size SIZE
    report-only
}
```

* `window SECONDS` - the rolling window in **SECONDS** during which response rates are tracked. Default 15.

* `ipv4-prefix-length LENGTH` - the prefix **LENGTH** in bits to use for identifying a ipv4 client. Default 24.

* `ipv6-prefix-length LENGTH` - the prefix **LENGTH** in bits to use for identifying a ipv6 client. Default 56.

* `responses-per-second ALLOWANCE` - the number of positive responses allowed per second. An **ALLOWANCE** of 0 disables rate limiting of positive responses. Default 0.

* `nodata-per-second ALLOWANCE` - the number of `NODATA` responses allowed per second. An **ALLOWANCE** of 0 disables rate limiting of NODATA responses. Defaults to responses-per-second.

* `nxdomains-per-second ALLOWANCE` - the number of `NXDOMAIN` responses allowed per second. An **ALLOWANCE** of 0 disables rate limiting of NXDOMAIN responses. Defaults to responses-per-second.

* `referrals-per-second ALLOWANCE` - the number of referral responses allowed per second. An **ALLOWANCE** of 0 disables rate limiting of referral responses. Defaults to responses-per-second.

* `errors-per-second ALLOWANCE` - the number of error responses allowed per second (excluding NXDOMAIN). An **ALLOWANCE** of 0 disables rate limiting of error responses. Defaults to responses-per-second.

* `slip-ratio N` - Let every **N**th dropped response slip through truncated. Responses that slip through are marked 
  truncated and have all sections emptied before being relayed. A client receiving a truncated response will retry using TCP,
  which is not subject to response rate limiting.  This provides a way for clients making legitimate requests to get an 
  answer while their IP prefix is being blocked by response rate limiting. For **N** = 1 slip every dropped response through;
  **N** = 4 slip every 4th dropped response through; etc. The default is **N** = 0, don't slip any responses through.

* `requests-per-second ALLOWANCE` - the number of requests allowed per second. An **ALLOWANCE** of 0 disables rate limiting of requests. Default 0.

* `max-table-size SIZE` - the maximum number of responses to be tracked at one time. When exceeded, rrl stops rate limiting new responses. Defaults to 100000.

* `report-only` -  Do not drop requests/responses when rates are exceeded, only log metrics. Defaults to false.

## Mitigate Wildcard Flooding with the metadata Plugin

An attacker can evade _rrl_ rate limits when launching a reflection attack if they know of the existence of a wildcard record.
In a nutshell, the attacker can spread the reflection attack across an unlimited number of unique query names synthesized by
a wildcard keeping the rate of responses for each individual name under limits.
To protect against this, enable the _metadata_ plugin. When the _metadata_ plugin is enabled, _rrl_ will account for all
responses synthesized by known wildcards under the parent domain of the wildcard. e.g. Both `a.example.org.` and
`a.example.org.` would be accounted for as `example.org.`, if they are synthesized from the wildcard record `*.example.org.`
This approach follows BIND9's solution to the same problem.

*Important:*
* The _metadata_ plugin MUST be enabled for this to work.
* CoreDNS MUST be >= *TBD*. Plugins in CoreDNS do not produce the required metadata until this version.
* This cannot protect against attacks leveraging wildcard records hosted by upstream nameservers.
* External plugins that can synthesize wildcard responses must be updated produce the metadata `zone/wildcard` in order
  to protect against flooding with wildcards it serves.
* Some plugins such as `rewrite` and `template` can emulate wildcard-like behavior in such a way that they can be leveraged
  in the same way by an attacker to launch an undetected reflection attack. This is possible if the plugin produces a
  positive answer for an unbounded set of questions.  `rewrite` and `template` do not produce the metadata required to 
  mitigate wildcard flooding.

## Metrics

If monitoring is enabled (via the *prometheus* plugin) then the following metrics are exported:

* `coredns_rrl_responses_exceeded_total{client_ip}` - Counter of responses exceeding QPS limit.
* `coredns_rrl_requests_exceeded_total{client_ip}` - Counter of requests exceeding QPS limit.

## External Plugin

*RRL* is an *external* plugin, which means it is not included in CoreDNS releases.  To use *rrl*, you'll need to build a CoreDNS image with *rrl* included (near the top of the plugin list). In a nutshell you'll need to:
* Clone https://github.com/coredns/coredns
* Add this plugin to [plugin.cfg](https://github.com/coredns/coredns/blob/master/plugin.cfg) per instructions therein.
* `make -f Makefile.release DOCKER=your-docker-repo release`
* `make -f Makefile.release DOCKER=your-docker-repo docker`
* `make -f Makefile.release DOCKER=your-docker-repo docker-push`

## Examples

Example 1

~~~ corefile

. {
  rrl . {
    responses-per-second 10
  }
}

~~~

## Known Issues

*rrl* is vulnerable to wildcard flooding. See the section above for mitigating this vulnerability: **Mitigate Wildcard Flooding with the metadata Plugin**

## Additional References

[A Quick Introduction to Response Rate Limiting](https://kb.isc.org/docs/aa-01000)

[This Plugin's Design Spec](./README-DEV.md)
