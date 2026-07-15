package proxy

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"go.uber.org/zap"

	jproxy "github.com/fmotalleb/junction/proxy"
)

// serveHTTPRouter starts an HTTP proxy server on the given addr.
// It initializes the server with a proxy handler that forwards requests through a SOCKS5 proxy chain.
func (p *Proxy) serveHTTPRouter(ctx context.Context, addr string) error {
	logger := log.FromContext(ctx).
		Named("router.http").
		With(
			zap.String("router", "http"),
			zap.String("listen", addr),
		)

	server := &http.Server{
		ReadHeaderTimeout: time.Second * 30,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
		Addr:              addr,
		Handler: &httpProxyHandler{
			Proxy:      p,
			ctx:        ctx,
			logger:     logger,
			targetPort: "80", // currently fixed value
		},
	}

	logger.Info("HTTP proxy booted")

	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP server error", zap.Error(err))
		return errors.Join(
			errors.New("failed to start listener for http proxy"),
			err,
		)
	}

	return nil
}

type httpProxyHandler struct {
	*Proxy
	ctx        context.Context
	logger     *zap.Logger
	targetPort string
}

func (h *httpProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	port := h.targetPort
	targetHost, err := prepareTargetHost(
		cmp.Or(r.Host, r.Header.Get("Host")),
		port,
	)
	if err != nil {
		h.logger.Warn("failed to prepare target host", zap.Error(err))
		http.Error(w, "malformed host value, refusing to process request", http.StatusBadRequest)
		return
	} else if targetHost == "" {
		h.logger.Warn("failed to read target host")
		http.Error(w, "malformed host value, failed to read the value, refusing to process request", http.StatusBadRequest)
		return
	}

	if !h.AllowedHost(h.ctx, targetHost) {
		h.logger.Warn("hostname rejected", zap.String("hostname", targetHost))
		w.WriteHeader(http.StatusForbidden)
		return
	}

	h.logger.Debug("HTTP request received",
		zap.String("method", r.Method),
		zap.String("targetHost", targetHost),
		zap.String("remoteAddr", r.RemoteAddr),
	)

	if r.Method == http.MethodConnect {
		h.handleConnect(w, r, targetHost)
	} else {
		h.handleHTTPRequest(w, r, targetHost)
	}
}

func prepareTargetHost(hostHeader, targetPort string) (string, error) {
	host := strings.TrimSpace(hostHeader)
	if host == "" {
		return "", errors.New("host header is empty")
	}

	// Strip scheme if present
	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("invalid URL in host header: %w", err)
		}
		host = u.Host
	}

	// Strip port, keep just the hostname
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if host == "" {
		return "", errors.New("empty host after parsing")
	}

	if targetPort == "" {
		return host, nil
	}

	return net.JoinHostPort(host, targetPort), nil
}

func (h *httpProxyHandler) handleConnect(w http.ResponseWriter, _ *http.Request, targetHost string) {
	ctx, cancel := context.WithTimeout(h.ctx, h.Timeout)
	defer cancel()
	dialer, err := jproxy.NewDialer([]*url.URL{h.ProxyAddr})
	if err != nil {
		http.Error(w, "SOCKS5 dialer error", http.StatusInternalServerError)
		return
	}

	targetConn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		h.logger.Debug("CONNECT failed", zap.String("target", targetHost), zap.Error(err))
		http.Error(w, "Failed to connect to target", http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		h.logger.Error("Hijacking unsupported")
		http.Error(w, "Hijacking unsupported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		h.logger.Error("Hijack failed", zap.Error(err))
		http.Error(w, "Hijack failed", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	relayTraffic(ctx, clientConn, targetConn, h.logger)
}

func (h *httpProxyHandler) handleHTTPRequest(w http.ResponseWriter, r *http.Request, targetHost string) {
	dialer, err := jproxy.NewDialer([]*url.URL{h.ProxyAddr})
	if err != nil {
		http.Error(w, "SOCKS5 dialer error", http.StatusInternalServerError)
		return
	}

	targetURL := &url.URL{
		Scheme:   "http",
		Host:     targetHost,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	req, err := http.NewRequestWithContext(h.ctx, r.Method, targetURL.String(), r.Body)
	if err != nil {
		h.logger.Error("Request creation failed", zap.Error(err))
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	resp, err := (&http.Client{Transport: &http.Transport{Dial: dialer.Dial}}).Do(req) //nolint:gosec // SSRF is intentional; proxy forwards to user-configured targets
	if err != nil {
		h.logger.Error("Request to target failed", zap.String("url", targetURL.String()), zap.Error(err))
		http.Error(w, "Request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.logger.Error("Response copy failed", zap.Error(err))
	}
}
