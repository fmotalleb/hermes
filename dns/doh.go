package dns

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

func serveDoH(g *errgroup.Group, ctx interface{ Done() <-chan struct{} }, cfg *ServerConfig, h dns.Handler) {
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.httpPath, dohHandler(h))

	httpSrv := &http.Server{
		Addr:      cfg.listenAddr,
		Handler:   mux,
		TLSConfig: cfg.tlsConfig,
	}

	g.Go(func() error {
		<-ctx.Done()
		return httpSrv.Close()
	})
	g.Go(func() error {
		if err := httpSrv.ListenAndServeTLS("", ""); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
}

// dohHandler adapts a dns.Handler to a net/http handler following RFC 8484.
// It supports both GET (?dns=<base64url>) and POST (application/dns-message).
func dohHandler(h dns.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rawMsg []byte
		var err error

		switch r.Method {
		case http.MethodGet:
			param := r.URL.Query().Get("dns")
			if param == "" {
				http.Error(w, "missing dns parameter", http.StatusBadRequest)
				return
			}
			rawMsg, err = base64.RawURLEncoding.DecodeString(param)
			if err != nil {
				http.Error(w, "invalid base64url encoding", http.StatusBadRequest)
				return
			}

		case http.MethodPost:
			if ct := r.Header.Get("Content-Type"); ct != "application/dns-message" {
				http.Error(w, "Content-Type must be application/dns-message", http.StatusUnsupportedMediaType)
				return
			}
			rawMsg, err = io.ReadAll(io.LimitReader(r.Body, maxDNSMessageSize))
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		req := new(dns.Msg)
		if err = req.Unpack(rawMsg); err != nil {
			http.Error(w, "failed to unpack DNS message", http.StatusBadRequest)
			return
		}

		// dns.ResponseWriter adapter that captures the response written by the handler.
		rw := &dohResponseWriter{localAddr: r.Host}
		h.ServeDNS(rw, req)

		if rw.msg == nil {
			http.Error(w, "no response from DNS handler", http.StatusInternalServerError)
			return
		}

		packed, err := rw.msg.Pack()
		if err != nil {
			http.Error(w, "failed to pack DNS response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/dns-message")
		w.Header().Set("Cache-Control", dohCacheControl(rw.msg))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(packed)
	}
}

const maxDNSMessageSize = 65535

// dohResponseWriter implements dns.ResponseWriter over an in-memory buffer.
type dohResponseWriter struct {
	localAddr string
	msg       *dns.Msg
}

func (w *dohResponseWriter) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", w.localAddr)
	return addr
}
func (w *dohResponseWriter) RemoteAddr() net.Addr      { return &net.TCPAddr{} }
func (w *dohResponseWriter) WriteMsg(m *dns.Msg) error { w.msg = m; return nil }
func (w *dohResponseWriter) Write(b []byte) (int, error) {
	m := new(dns.Msg)
	if err := m.Unpack(b); err != nil {
		return 0, err
	}
	w.msg = m
	return len(b), nil
}
func (w *dohResponseWriter) Close() error        { return nil }
func (w *dohResponseWriter) TsigStatus() error   { return nil }
func (w *dohResponseWriter) TsigTimersOnly(bool) {}
func (w *dohResponseWriter) Hijack()             {}

// dohCacheControl computes a Cache-Control max-age from the minimum TTL
// in the response, as recommended by RFC 8484 §5.1.
func dohCacheControl(m *dns.Msg) string {
	minTTL := uint32(300) // sensible default
	for _, rr := range append(m.Answer, append(m.Ns, m.Extra...)...) {
		if h := rr.Header(); h.Ttl < minTTL {
			minTTL = h.Ttl
		}
	}
	return fmt.Sprintf("max-age=%d", minTTL)
}
