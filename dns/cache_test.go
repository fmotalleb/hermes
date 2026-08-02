package dns

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/cache"
)

func TestDNSResponseCacheRoundTrip(t *testing.T) {
	h := &handler{
		cache:      cache.NewMemoryCache(t.Context()),
		cacheTypes: []uint16{dns.TypeA},
	}

	req := new(dns.Msg)
	req.Id = 42
	req.SetQuestion("WWW.Example.COM.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true
	resp.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "www.example.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			A: net.ParseIP("192.0.2.10").To4(),
		},
	}

	h.cacheResponse(context.Background(), req, resp)

	got, ok := h.cachedResponse(context.Background(), req)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if got.Id != req.Id {
		t.Fatalf("expected cached response id %d, got %d", req.Id, got.Id)
	}

	if len(got.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(got.Answer))
	}

	if got.Answer[0].Header().Name != "www.example.com." {
		t.Fatalf("expected cached answer owner to be preserved, got %q", got.Answer[0].Header().Name)
	}
}

func TestDNSResponseCacheKeyNormalizesName(t *testing.T) {
	got := dnsResponseCacheKey("WWW.Example.COM.", dns.TypeA, dns.ClassINET)
	want := "www.example.com:1:1"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
