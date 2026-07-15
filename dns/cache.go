package dns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	// DNSCacheInvalidTopic is the pubsub topic used to signal DNS cache invalidation.
	DNSCacheInvalidTopic = "dns:cache:invalidate"

	dnsResponseCacheDefaultTTL        = 30 * time.Second
	dnsResponseCacheNegativeTTL       = 10 * time.Second
	dnsResponseCacheMaximumTTL        = 60 * time.Second
	dnsResponseCacheRedisKeyNamespace = "dns:response:v1"
)

var errDNSCacheInvalidBackend = errors.New("cache backend is invalid")

func dnsResponseCacheKey(qname string, qtype, qclass uint16) string {
	return fmt.Sprintf(
		"%s:%d:%d",
		normalizeDNSName(qname),
		qtype,
		qclass,
	)
}

func (h *handler) cachedResponse(ctx context.Context, req *dns.Msg) (*dns.Msg, bool) {
	if h.cache == nil || req == nil || len(req.Question) == 0 {
		return nil, false
	}

	q := req.Question[0]
	if !h.shouldCache(q) {
		return nil, false
	}
	key := dnsResponseCacheKey(q.Name, q.Qtype, q.Qclass)
	data, err := h.cache.GetBytes(ctx, key)
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

	q := req.Question[0]
	if !h.shouldCache(q) {
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

	key := dnsResponseCacheKey(q.Name, q.Qtype, q.Qclass)
	_ = h.cache.Set(ctx, key, data, ttl)
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
