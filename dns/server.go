package dns

import (
	"errors"
	"fmt"
	"net"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/cache"

	"golang.org/x/sync/errgroup"
)

func Serve(ctx *gofr.Context, app *gofr.App, opts ...ServerOption) error {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("dns server option: %w", err)
		}
	}

	db := app.GetSQL()
	logger := app.Logger()
	store := &store{db: db, logger: logger}
	tr := otel.GetTracerProvider().Tracer("dns-server")

	var c cache.Cache
	switch app.Config.GetOrDefault("DNS_CACHE_BACKEND", dnsDefaultCacheBackend) {
	case "redis":
		c = cache.NewRedisCache(ctx.Redis, dnsResponseCacheRedisKeyNamespace)
	case "memory":
		c = cache.NewMemoryCache(ctx, app.Metrics())
	case "none":
		c = cache.NewNoneCache()
	default:
		return fmt.Errorf("%w: %s", dnsCacheInvalidBackend, app.Config.Get("DNS_CACHE_BACKEND"))
	}

	h := &handler{
		store:  store,
		logger: logger,
		tracer: tr,
		cache:  c,
	}

	app.Subscribe(DNSCacheInvalidTopic, func(c *gofr.Context) error {
		return h.cache.Clear(c)
	})

	group, groupCtx := errgroup.WithContext(ctx)

	switch cfg.protocol {
	case ProtocolUDP:
		serveUDP(group, groupCtx, cfg, h)

	case ProtocolTCP:
		serveTCP(group, groupCtx, cfg, h)

	case ProtocolBoth:
		serveUDP(group, groupCtx, cfg, h)
		serveTCP(group, groupCtx, cfg, h)

	case ProtocolTLS:
		if cfg.tlsConfig == nil {
			return fmt.Errorf("ProtocolTLS requires TLS configuration (use WithTLSFiles or WithTLSConfig)")
		}
		serveTLS(group, groupCtx, cfg, h)

	case ProtocolHTTPS:
		if cfg.tlsConfig == nil {
			return fmt.Errorf("ProtocolHTTPS requires TLS configuration (use WithTLSFiles or WithTLSConfig)")
		}
		serveDoH(group, groupCtx, cfg, h)

	default:
		return fmt.Errorf("unknown protocol: %d", cfg.protocol)
	}

	logger.Infof("dns server started at %s (protocol=%d)", cfg.listenAddr, cfg.protocol)
	return group.Wait()
}

// --- helpers that register goroutines into the errgroup ---

func serveUDP(g *errgroup.Group, ctx interface{ Done() <-chan struct{} }, cfg *ServerConfig, h dns.Handler) {
	srv := &dns.Server{Addr: cfg.listenAddr, Net: "udp", Handler: h}
	g.Go(shutdownOn(ctx, srv))
	g.Go(listenAndServe(srv))
}

func serveTCP(g *errgroup.Group, ctx interface{ Done() <-chan struct{} }, cfg *ServerConfig, h dns.Handler) {
	srv := &dns.Server{Addr: cfg.listenAddr, Net: "tcp", Handler: h}
	g.Go(shutdownOn(ctx, srv))
	g.Go(listenAndServe(srv))
}

func serveTLS(g *errgroup.Group, ctx interface{ Done() <-chan struct{} }, cfg *ServerConfig, h dns.Handler) {
	srv := &dns.Server{
		Addr:      cfg.listenAddr,
		Net:       "tcp-tls",
		Handler:   h,
		TLSConfig: cfg.tlsConfig,
	}
	g.Go(shutdownOn(ctx, srv))
	g.Go(listenAndServe(srv))
}

// shutdownOn shuts down a dns.Server when the errgroup context is cancelled.
func shutdownOn(ctx interface{ Done() <-chan struct{} }, srv *dns.Server) func() error {
	return func() error {
		<-ctx.Done()
		_ = srv.Shutdown()
		return nil
	}
}

// listenAndServe starts a dns.Server and maps net.ErrClosed to nil (clean shutdown).
func listenAndServe(srv *dns.Server) func() error {
	return func() error {
		if err := srv.ListenAndServe(); !errors.Is(err, net.ErrClosed) {
			return err
		}
		return nil
	}
}
