package dns

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	dnsResponseCacheVersionKey   = "dns:response:version"
	dnsResponseCacheDefaultTTL   = 30 * time.Second
	dnsResponseCacheNegativeTTL  = 10 * time.Second
	dnsResponseCacheMaximumTTL   = 60 * time.Second
	dnsResponseCacheKeyNamespace = "dns:response:v1"
)

func dnsResponseCacheKey(version uint64, qname string, qtype, qclass uint16) string {
	return fmt.Sprintf(
		"%s:%d:%s:%d:%d",
		dnsResponseCacheKeyNamespace,
		version,
		normalizeDNSName(qname),
		qtype,
		qclass,
	)
}

func (h *handler) dnsResponseCacheVersion(ctx context.Context) uint64 {
	if h.cache == nil {
		return 0
	}

	version, err := h.cache.Get(ctx, dnsResponseCacheVersionKey).Uint64()
	if err != nil {
		return 0
	}

	return version
}

func (h *handler) cachedResponse(ctx context.Context, req *dns.Msg) (*dns.Msg, bool) {
	if h.cache == nil || req == nil || len(req.Question) == 0 {
		return nil, false
	}

	q := req.Question[0]
	key := dnsResponseCacheKey(h.dnsResponseCacheVersion(ctx), q.Name, q.Qtype, q.Qclass)
	data, err := h.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		return nil, false
	}

	msg.Id = req.Id
	msg.Question = append([]dns.Question(nil), req.Question...)

	return msg, true
}

func (h *handler) cacheResponse(ctx context.Context, req, resp *dns.Msg) {
	if h.cache == nil || req == nil || resp == nil || len(req.Question) == 0 {
		return
	}

	ttl, ok := responseCacheTTL(resp)
	if !ok {
		return
	}

	data, err := resp.Pack()
	if err != nil {
		return
	}

	q := req.Question[0]
	key := dnsResponseCacheKey(h.dnsResponseCacheVersion(ctx), q.Name, q.Qtype, q.Qclass)
	_ = h.cache.Set(ctx, key, data, ttl).Err()
}

func responseCacheTTL(resp *dns.Msg) (time.Duration, bool) {
	if resp == nil || resp.Truncated {
		return 0, false
	}

	switch resp.Rcode {
	case dns.RcodeServerFailure, dns.RcodeRefused:
		return 0, false
	case dns.RcodeNameError:
		return dnsResponseCacheNegativeTTL, true
	case dns.RcodeSuccess:
		if len(resp.Answer) == 0 {
			return dnsResponseCacheNegativeTTL, true
		}

		ttl := minRRTTL(resp.Answer)
		if ttl == 0 {
			ttl = dnsResponseCacheDefaultTTL
		}
		if ttl > dnsResponseCacheMaximumTTL {
			ttl = dnsResponseCacheMaximumTTL
		}
		return ttl, true
	default:
		return 0, false
	}
}

func minRRTTL(rrs []dns.RR) time.Duration {
	if len(rrs) == 0 {
		return 0
	}

	minTTL := rrs[0].Header().Ttl
	for _, rr := range rrs[1:] {
		if ttl := rr.Header().Ttl; ttl < minTTL {
			minTTL = ttl
		}
	}

	return time.Duration(minTTL) * time.Second
}
