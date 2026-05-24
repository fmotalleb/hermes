package dns

import (
	"errors"
	"net"
	"os"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr"

	"github.com/fmotalleb/hermes/cache"

	"golang.org/x/sync/errgroup"
)

const defaultListenAddr = "0.0.0.0:5354"

func Serve(ctx *gofr.Context) error {
	db := ctx.SQL
	logger := ctx.Logger

	store := &store{
		db:     db,
		logger: logger,
	}

	tr := otel.GetTracerProvider().Tracer("dns-server")

	h := &handler{
		store:  store,
		logger: logger,
		tracer: tr,
		cache:  cache.NewMemoryCache(ctx),
	}
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
