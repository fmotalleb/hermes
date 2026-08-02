// Package web provides a lightweight HTTP router with path parameters,
// middleware support, CORS handling, and JSON response utilities.
package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/fmotalleb/hermes/otellog"
)

type (
	// HandlerFunc is a route handler that returns a value or an error.
	HandlerFunc func(*Context) (any, error)
	// Middleware wraps an http.Handler to add cross-cutting behavior such as logging or auth.
	Middleware func(http.Handler) http.Handler
)

// Router is a lightweight HTTP router with path parameters and middleware support.
type Router struct {
	routes       []route
	middlewares  []Middleware
	notFound     http.HandlerFunc
	methodPrefix string
}

type route struct {
	method  string
	pattern string
	parts   []routePart
	handler HandlerFunc
}

type routePart struct {
	literal  string
	param    string
	catchAll bool
}

// File represents a static file response with its content and content type.
type File struct {
	Content     []byte
	ContentType string
}

// Responded is a sentinel value that tells the router the response has already been written.
type Responded struct{}

// HTTPError is an error with an associated HTTP status code and response body.
type HTTPError interface {
	error
	StatusCode() int
	Body() any
}

// NewRouter creates a new router with no routes or middlewares.
func NewRouter() *Router {
	return &Router{
		notFound: http.NotFound,
	}
}

// Use appends middlewares to the router's middleware chain.
func (r *Router) Use(middlewares ...Middleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

// RequestLogger registers middleware that logs every HTTP request with its
// method, path, status code, duration, and remote address, plus the
// request-scoped request id and trace id. The request id comes from the
// X-Request-ID header when present, otherwise a UUIDv7 is generated and echoed
// back in the response header. The ids are attached to the context logger (see
// log.FromContext), so every log emitted while handling the request carries
// them. Register it before auth or other middlewares to also cover rejected
// requests.
func (r *Router) RequestLogger() {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			requestID := strings.TrimSpace(req.Header.Get("X-Request-ID"))
			if requestID == "" {
				requestID = newRequestID()
			}

			// Derive the http child logger and attach it to the context so every
			// log emitted while handling the request carries the request and
			// trace ids and the http component name.
			ctx, logger := log.AsNamedChild(req.Context(), "http")
			fields := []zap.Field{zap.String("request_id", requestID)}
			if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
				fields = append(fields, zap.String("trace_id", sc.TraceID().String()))
				// Carry the span context so OTLP log records are correlated with
				// the request's trace natively (trace id + span id on the record).
				fields = append(fields, otellog.TraceContextField(ctx))
			}
			ctx = log.WithLogger(ctx, logger.With(fields...))
			req = req.WithContext(ctx)

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			rec.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(rec, req)

			log.FromContext(req.Context()).Info("request",
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", rec.status),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote", req.RemoteAddr),
			)
		})
	})
}

// newRequestID generates a UUIDv7 request id, falling back to a UUIDv4 if the
// v7 generator is unavailable.
func newRequestID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.New().String()
}

// statusRecorder captures the response status code for request logging while
// forwarding writes to the underlying response writer.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush implements http.Flusher so streaming handlers keep their flush
// capability when wrapped by the request logger.
func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker so the wrapped writer can be hijacked if a
// handler ever needs raw connection access.
func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Handle registers a route with the given HTTP method, URL pattern, and handler.
// Patterns support path parameters: /users/{id}, /files/{path...}.
func (r *Router) Handle(method, pattern string, handler HandlerFunc) {
	if method != http.MethodOptions {
		r.OPTIONS(pattern, nil)
	}
	r.routes = append(r.routes, route{
		method:  strings.ToUpper(method),
		pattern: pattern,
		parts:   parsePattern(pattern),
		handler: handler,
	})
}

// GET registers a handler for HTTP GET requests at the given pattern.
func (r *Router) GET(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodGet, pattern, handler)
}

// POST registers a handler for HTTP POST requests at the given pattern.
func (r *Router) POST(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodPost, pattern, handler)
}

// DELETE registers a handler for HTTP DELETE requests at the given pattern.
func (r *Router) DELETE(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodDelete, pattern, handler)
}

// OPTIONS registers a handler for HTTP OPTIONS requests at the given pattern.
func (r *Router) OPTIONS(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodOptions, pattern, handler)
}

// Group creates a sub-router with a path prefix and shared middlewares.
func (r *Router) Group(prefix string, fn func(*Router)) {
	child := &Router{
		notFound:     r.notFound,
		methodPrefix: strings.TrimSuffix(prefix, "/"),
	}
	child.middlewares = append(child.middlewares, r.middlewares...)
	child.routes = append(child.routes, r.routes...)
	fn(child)
	r.routes = child.routes
	r.middlewares = child.middlewares
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Route matching and the 404 fallback run inside the middleware chain so
	// middlewares (request logging, auth, CORS) apply to every request, and so
	// the handler context reflects any values attached by middlewares.
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, rt := range r.routes {
			params, ok := matchRoute(rt.parts, req.URL.Path)
			if !ok || rt.method != req.Method {
				continue
			}

			if rt.handler == nil {
				continue
			}

			ctx := &Context{
				Context:        req.Context(),
				Request:        req,
				ResponseWriter: w,
				Params:         params,
			}
			value, err := rt.handler(ctx)
			writeResult(w, req, value, err)
			return
		}

		if r.notFound != nil {
			r.notFound(w, req)
			return
		}

		http.NotFound(w, req)
	})

	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	handler.ServeHTTP(w, req)
}

func parsePattern(pattern string) []routePart {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}

	segments := strings.Split(trimmed, "/")
	parts := make([]routePart, 0, len(segments))
	for _, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if cut, _, found := strings.Cut(name, ":"); found {
				if cut != "" {
					name = cut
				}
				if strings.Contains(segment, ":.*") {
					parts = append(parts, routePart{param: name, catchAll: true})
					continue
				}
			}
			if strings.HasSuffix(name, "...") {
				parts = append(parts, routePart{param: strings.TrimSuffix(name, "..."), catchAll: true})
				continue
			}
			parts = append(parts, routePart{param: name})
			continue
		}
		parts = append(parts, routePart{literal: segment})
	}
	return parts
}

func matchRoute(parts []routePart, requestPath string) (map[string]string, bool) {
	trimmed := strings.Trim(requestPath, "/")
	var segments []string
	if trimmed != "" {
		segments = strings.Split(trimmed, "/")
	}

	params := make(map[string]string)
	i := 0
	for ; i < len(parts); i++ {
		part := parts[i]
		if part.catchAll {
			params[part.param] = strings.Join(segments[i:], "/")
			return params, true
		}
		if i >= len(segments) {
			return nil, false
		}
		if part.literal != "" {
			if part.literal != segments[i] {
				return nil, false
			}
			continue
		}
		params[part.param] = segments[i]
	}

	if len(segments) != len(parts) {
		return nil, false
	}

	return params, true
}

func writeResult(w http.ResponseWriter, _ *http.Request, value any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}

	switch v := value.(type) {
	case nil:
		w.WriteHeader(http.StatusNoContent)
	case Responded:
		return
	case File:
		if v.ContentType != "" {
			w.Header().Set("Content-Type", v.ContentType)
		}
		if len(v.Content) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(v.Content)
	case io.Reader:
		_, _ = io.Copy(w, v)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(value)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.StatusCode()
		if status == 0 {
			status = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(httpErr.Body())
		return
	}

	var body any = map[string]string{"error": err.Error()}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, io.EOF):
		status = http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		status = http.StatusRequestTimeout
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}
