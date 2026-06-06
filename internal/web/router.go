package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type (
	HandlerFunc func(*Context) (any, error)
	Middleware  func(http.Handler) http.Handler
)

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

type File struct {
	Content     []byte
	ContentType string
}

type Responded struct{}

type HTTPError interface {
	error
	StatusCode() int
	Body() any
}

func NewRouter() *Router {
	return &Router{
		notFound: http.NotFound,
	}
}

func (r *Router) Use(middlewares ...Middleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

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

func (r *Router) GET(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodGet, pattern, handler)
}

func (r *Router) POST(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodPost, pattern, handler)
}

func (r *Router) DELETE(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodDelete, pattern, handler)
}

func (r *Router) OPTIONS(pattern string, handler HandlerFunc) {
	r.Handle(http.MethodOptions, pattern, handler)
}

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
	for _, rt := range r.routes {
		params, ok := matchRoute(rt.parts, req.URL.Path)
		if !ok || rt.method != req.Method {
			continue
		}

		ctx := &Context{
			Context:        req.Context(),
			Request:        req,
			ResponseWriter: w,
			Params:         params,
		}

		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			value, err := rt.handler(ctx)
			writeResult(w, req, value, err)
		})

		for i := len(r.middlewares) - 1; i >= 0; i-- {
			handler = r.middlewares[i](handler)
		}

		handler.ServeHTTP(w, req)
		return
	}

	if r.notFound != nil {
		r.notFound(w, req)
		return
	}

	http.NotFound(w, req)
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

func writeResult(w http.ResponseWriter, req *http.Request, value any, err error) {
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
