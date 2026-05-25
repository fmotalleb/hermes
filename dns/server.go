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

const defaultListenAddr = "0.0.0.0:5354"

func Serve(ctx *gofr.Context, app *gofr.App) error {
	db := app.GetSQL()
	logger := app.Logger()

	store := &store{
		db:     db,
		logger: logger,
	}
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

	listenAddr := listenAddr(app)
	udpServer := &dns.Server{
		Addr:    listenAddr,
		Net:     "udp",
		Handler: h,
	}

	tcpServer := &dns.Server{
		Addr:    listenAddr,
		Net:     "tcp",
		Handler: h,
	}

	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		<-groupCtx.Done()
		_ = udpServer.Shutdown()
		_ = tcpServer.Shutdown()
		return nil
	})

	group.Go(func() error {
		err := udpServer.ListenAndServe()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	})

	group.Go(func() error {
		err := tcpServer.ListenAndServe()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	})
	logger.Infof("dns server started at: %s", listenAddr)
	return group.Wait()
}

func listenAddr(app *gofr.App) string {
	return app.Config.GetOrDefault("DNS_LISTEN_ADDR", defaultListenAddr)
}
