package dns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/pubsub"
	"github.com/fmotalleb/hermes/registry"
	"github.com/fmotalleb/hermes/runtime"
)

func Serve(ctx context.Context, app *runtime.App, bus pubsub.Bus, opts ...ServerOption) error {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("dns server option: %w", err)
		}
	}

	store := &store{
		db:       app.DB,
		logger:   app.Logger,
		registry: app.ServiceRegistry,
	}
	tr := otel.GetTracerProvider().Tracer("dns-server")

	var c cache.Cache
	switch app.Config.DNSCacheBackend {
	case "redis":
		c = cache.NewRedisCache(app.Redis, dnsResponseCacheRedisKeyNamespace)
	case "memory":
		c = cache.NewMemoryCache(ctx)
	case "none":
		c = cache.NewNoneCache()
	default:
		return fmt.Errorf("%w: %s", dnsCacheInvalidBackend, app.Config.DNSCacheBackend)
	}

	cacheTypes := make([]uint16, len(app.Config.DNSCacheTypes))

	for i, v := range app.Config.DNSCacheTypes {
		if tv, ok := dns.StringToType[v]; ok {
			cacheTypes[i] = tv
		} else {
			return fmt.Errorf("type defined in the DNS_CACHE_TYPES: %s, is undefined", v)
		}
	}

	h := &handler{
		dnsStore:   store,
		logger:     app.Logger,
		tracer:     tr,
		cache:      c,
		cacheTypes: cacheTypes,
	}

	eg, groupCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return bus.Subscribe(ctx, DNSCacheInvalidTopic, func(_ context.Context, _ []byte) error {
			app.Logger.Info("received invalidation notice", slog.String("id", app.ID().String()))
			if err := h.cache.Clear(ctx); err != nil {
				app.Logger.Warn("failed to invalidate cache", slog.String("err", err.Error()))
			}
			return nil
		})
	})
	eg.Go(func() error {
		return app.ServiceRegistry.OnDelete(ctx, app.Config.RedisDB, registry.ServiceKindProxy, func(_ string) {
			app.Logger.Info("received invalidation notice, proxy disconnected", slog.String("id", app.ID().String()))
			if err := h.cache.Clear(ctx); err != nil {
				app.Logger.Warn("failed to invalidate cache", slog.String("err", err.Error()))
			}
		})
	})

	switch cfg.protocol {
	case ProtocolUDP:
		serveUDP(eg, groupCtx, cfg, h)
	case ProtocolTCP:
		serveTCP(eg, groupCtx, cfg, h)
	case ProtocolBoth:
		serveUDP(eg, groupCtx, cfg, h)
		serveTCP(eg, groupCtx, cfg, h)
	case ProtocolTLS:
		if cfg.tlsConfig == nil {
			return errors.New("ProtocolTLS requires TLS configuration (use WithTLSFiles or WithTLSConfig)")
		}
		serveTLS(eg, groupCtx, cfg, h)
	case ProtocolHTTPS:
		if cfg.tlsConfig == nil {
			return errors.New("ProtocolHTTPS requires TLS configuration (use WithTLSFiles or WithTLSConfig)")
		}
		serveDoH(eg, groupCtx, cfg, h)
	default:
		return fmt.Errorf("unknown protocol: %d", cfg.protocol)
	}

	app.Logger.Info("dns server started", "addr", cfg.listenAddr, "protocol", cfg.protocol)
	return eg.Wait()
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
