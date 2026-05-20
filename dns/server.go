package dns

import (
	"context"
	"errors"
	"net"

	"github.com/miekg/dns"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"

	"golang.org/x/sync/errgroup"
)

const defaultListenAddr = "0.0.0.0:5354"

func Serve(ctx context.Context, cfg config.Config, logger logging.Logger, metrics metrics.Manager) error {
	store, err := newStore(cfg, logger, metrics)
	if err != nil {
		return err
	}
	defer store.Close()

	h := &handler{store: store, logger: logger}

	udpServer := &dns.Server{
		Addr:    listenAddr(cfg),
		Net:     "udp",
		Handler: h,
	}

	tcpServer := &dns.Server{
		Addr:    listenAddr(cfg),
		Net:     "tcp",
		Handler: h,
	}

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		<-ctx.Done()
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

	return group.Wait()
}

func listenAddr(cfg config.Config) string {
	if v := cfg.Get("DNS_LISTEN_ADDR"); v != "" {
		return v
	}

	return defaultListenAddr
}
