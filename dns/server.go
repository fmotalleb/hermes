package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/internal/pubsub"
	"github.com/fmotalleb/hermes/internal/runtime"
	"golang.org/x/sync/errgroup"
)

func Serve(ctx context.Context, app *runtime.App, opts ...ServerOption) error {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("dns server option: %w", err)
		}
	}

	store := &store{db: app.DB, logger: app.Logger}
	tr := otel.GetTracerProvider().Tracer("dns-server")

	var c cache.Cache
	switch strings.ToLower(app.Config.CacheBackend) {
	case "redis":
		c = cache.NewRedisCache(app.Redis, dnsResponseCacheRedisKeyNamespace)
	case "memory":
		c = cache.NewMemoryCache(ctx)
	case "none":
		c = cache.NewNoneCache()
	default:
		return fmt.Errorf("%w: %s", dnsCacheInvalidBackend, app.Config.CacheBackend)
	}

	h := &handler{
		store:  store,
		logger: app.Logger,
		tracer: tr,
		cache:  c,
	}

	bus := pubsub.New(app.Redis)
	go func() {
		_ = bus.Subscribe(ctx, DNSCacheInvalidTopic, func(_ context.Context, _ []byte) error {
			return h.cache.Clear(ctx)
		})
	}()

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

	app.Logger.Info("dns server started", "addr", cfg.listenAddr, "protocol", cfg.protocol)
	return group.Wait()
}

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

func shutdownOn(ctx interface{ Done() <-chan struct{} }, srv *dns.Server) func() error {
	return func() error {
		<-ctx.Done()
		_ = srv.Shutdown()
		return nil
	}
}

func listenAndServe(srv *dns.Server) func() error {
	return func() error {
		if err := srv.ListenAndServe(); !errors.Is(err, net.ErrClosed) {
			return err
		}
		return nil
	}
}

