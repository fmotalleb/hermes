package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/dns"
	"github.com/fmotalleb/hermes/pubsub"
	"github.com/fmotalleb/hermes/queries"
)

// Proxy handles transparent proxying through HTTP and TLS/SNI routers,
// with optional SOCKS5 upstream, zone-based access control, and cache invalidation.
type Proxy struct {
	ListenAddr string
	HTTPPorts  []string
	TLSPorts   []string
	Timeout    time.Duration
	ProxyAddr  *url.URL

	logger *zap.Logger
	cache  cache.Cache
	db     queries.DB
	bus    pubsub.Bus
}

const allowedHostCachePrefix = "proxy:allowed-host:"

// NewProxy creates a new Proxy with the given configuration and dependencies.
// proxyURL is parsed from its string representation; an empty string leaves ProxyAddr nil.
func NewProxy(listenAddr string, httpPorts, tlsPorts []string, timeout time.Duration, proxyURL string, c cache.Cache, db queries.DB, bus pubsub.Bus, logger *zap.Logger) (*Proxy, error) {
	var parsedURL *url.URL
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy URL: %w", err)
		}
		parsedURL = u
	}
	if len(httpPorts) == 0 && len(tlsPorts) == 0 {
		return nil, errors.New("at least one HTTP or TLS port must be configured")
	}
	return &Proxy{
		ListenAddr: listenAddr,
		HTTPPorts:  httpPorts,
		TLSPorts:   tlsPorts,
		Timeout:    timeout,
		ProxyAddr:  parsedURL,
		logger:     logger,
		cache:      c,
		db:         db,
		bus:        bus,
	}, nil
}

// Serve runs HTTP and TLS/SNI proxy listeners for every configured port concurrently,
// and subscribes to cache invalidation events from the pubsub bus.
// The method blocks until all listeners exit, typically due to context cancellation.
func (p *Proxy) Serve(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	// Subscribe to cache invalidation so zone changes propagate to the proxy.
	if p.bus != nil && p.cache != nil {
		eg.Go(func() error {
			p.logger.Info("subscribing to cache invalidation",
				zap.String("topic", dns.DNSCacheInvalidTopic),
			)
			return p.bus.Subscribe(ctx, dns.DNSCacheInvalidTopic, func(_ context.Context, _ []byte) error {
				if err := p.cache.Clear(ctx); err != nil {
					p.logger.Warn("failed to clear proxy cache", zap.Error(err))
				}
				p.logger.Debug("proxy cache invalidated")
				return nil
			})
		})
	}

	for _, port := range p.HTTPPorts {
		addr := net.JoinHostPort(p.ListenAddr, port)
		eg.Go(func() error {
			return p.serveHTTPRouter(ctx, addr)
		})
	}
	for _, port := range p.TLSPorts {
		addr := net.JoinHostPort(p.ListenAddr, port)
		eg.Go(func() error {
			return p.serveSNIRouter(ctx, addr)
		})
	}

	return eg.Wait()
}

func (p *Proxy) AllowedHost(ctx context.Context, host string) bool {
	if host == "" {
		return false
	}

	if p.cache != nil {
		if _, err := p.cache.GetBytes(ctx, allowedHostCachePrefix+host); err == nil {
			return true
		}
	}

	if p.db == nil {
		return false
	}

	var exists int
	if err := p.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM zones WHERE name = $1 LIMIT 1;`,
		host,
	).Scan(&exists); err != nil {
		return false
	}

	if p.cache != nil {
		_ = p.cache.Set(ctx, allowedHostCachePrefix+host, []byte{1}, 5*time.Minute)
	}

	return true
}
