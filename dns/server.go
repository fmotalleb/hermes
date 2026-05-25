package dns

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/cache"

	"golang.org/x/sync/errgroup"
)

const defaultListenAddr = "0.0.0.0:5354"

func Serve(ctx *gofr.Context, app *gofr.App) error {
	db := ctx.SQL
	logger := ctx.Logger

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
		c = cache.NewMemoryCache(ctx)
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
	// TODO this does not receive event submitted
	app.Subscribe(DNSCacheInvalidTopic, func(c *gofr.Context) error {
		return h.cache.Clear(c)
	})

	listenAddr := listenAddr()
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

func listenAddr() string {
	if v := os.Getenv("DNS_LISTEN_ADDR"); v != "" {
		return v
	}

	return defaultListenAddr
}
