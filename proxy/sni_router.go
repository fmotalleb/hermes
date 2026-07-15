package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"

	"github.com/fmotalleb/junction/crypto/tls"
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

	go func() {
		<-ctx.Done()
		_ = client.Close()
	}()
	// TODO: handle with p.ProxyAddr
	server, err := net.DialTimeout("tcp", net.JoinHostPort(sni, "443"), 10*time.Second)
	if err != nil {
		_ = client.Close()
		return
	}
	defer server.Close()

	if _, err := server.Write(buf[:n]); err != nil {
		logger.Error("initial write failed", zap.Error(err))
		_ = client.Close()
		return
	}

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(server, client)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(client, server)
		errCh <- err
	}()

	<-ctx.Done()
	_ = client.Close()
	_ = server.Close()
	<-errCh
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
