package proxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"

	"github.com/fmotalleb/junction/crypto/tls"
	jproxy "github.com/fmotalleb/junction/proxy"
)

var errSNIMissing = errors.New("SNI missing in ClientHello")

func (p *Proxy) serveSNIRouter(ctx context.Context) error {
	logger := log.FromContext(ctx).Named("proxy.sni_router").
		With(
			zap.String("router", "sni"),
			zap.String("listen", p.ListenTLS),
		)

	addrPort, err := netip.ParseAddrPort(p.ListenTLS)
	if err != nil {
		return err
	}

	addr := net.TCPAddrFromAddrPort(addrPort)
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logger.Error("listen failed", zap.Error(err))
		return err
	}
	defer listener.Close()

	logger.Info("SNI router started")

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("router exit due to context cancellation")
				return nil
			}
			logger.Warn("accept failed", zap.Error(err))
			continue
		}

		go p.handleClient(ctx, conn, logger)
	}
}

func (p *Proxy) handleClient(ctx context.Context, conn net.Conn, logger *zap.Logger) {
	serverName, buf, n, err := readSNI(conn, logger)
	if err != nil {
		_ = conn.Close()
		return
	}

	sni := string(serverName)
	l := logger.With(zap.String("sni", sni))

	if !p.AllowedHost(ctx, sni) {
		l.Warn("SNI rejected")
		_ = conn.Close()
		return
	}

	go p.proxyToTarget(ctx, conn, sni, buf, n, l)
}

func (p *Proxy) proxyToTarget(parentCtx context.Context, client net.Conn, sni string, buf []byte, n int, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(parentCtx, p.Timeout)
	defer cancel()

	dialer, err := jproxy.NewDialer([]*url.URL{p.ProxyAddr})
	if err != nil {
		logger.Error("failed to create SOCKS5 dialer", zap.Error(err))
		_ = client.Close()
		return
	}

	server, err := dialer.Dial("tcp", net.JoinHostPort(sni, "443"))
	if err != nil {
		logger.Debug("failed to connect to target", zap.String("sni", sni), zap.Error(err))
		_ = client.Close()
		return
	}
	defer server.Close()

	if _, err := server.Write(buf[:n]); err != nil {
		logger.Error("initial write failed", zap.Error(err))
		_ = client.Close()
		return
	}

	relayTraffic(ctx, client, server, logger)
}

func readSNI(conn net.Conn, logger *zap.Logger) ([]byte, []byte, int, error) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		logger.Error("client read failed", zap.Error(err))
		return nil, nil, 0, err
	}

	name := tls.ExtractSNI(buf[:n])
	if name == nil {
		logger.Debug("SNI missing",
			zap.String("client", conn.RemoteAddr().String()),
		)
		return nil, nil, 0, errSNIMissing
	}

	return name, buf, n, nil
}
