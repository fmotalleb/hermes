package web

import (
	"net/http"
	"strconv"
	"strings"
)

// CorsOptions holds the configuration for CORS middleware.
type CorsOptions struct {
	Origins []string
	Methods []string
	Headers []string
	MaxAge  uint
}

// CorsOption configures a CorsOptions struct.
type CorsOption func(*CorsOptions)

// WithOrigins sets the allowed origins for CORS. Use "*" to allow all origins.
func WithOrigins(origins ...string) CorsOption {
	return func(o *CorsOptions) {
		o.Origins = origins
	}
}

// WithMethods sets the allowed HTTP methods for CORS.
func WithMethods(methods ...string) CorsOption {
	return func(o *CorsOptions) {
		o.Methods = methods
	}
}

// WithHeaders sets the allowed request headers for CORS.
func WithHeaders(headers ...string) CorsOption {
	return func(o *CorsOptions) {
		o.Headers = headers
	}
}

// WithMaxAge sets the CORS preflight cache duration in seconds.
func WithMaxAge(seconds uint) CorsOption {
	return func(o *CorsOptions) {
		o.MaxAge = seconds
	}
}

func defaultCorsOptions() CorsOptions {
	return CorsOptions{
		Origins: []string{},
		Methods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		Headers: []string{
			"Authorization",
			"Content-Type",
			"Accept",
		},
		MaxAge: 86400,
	}
}

// AllowCors enables CORS middleware on the router with the given options.
func (r *Router) AllowCors(opts ...CorsOption) {
	cfg := defaultCorsOptions()

	for _, opt := range opts {
		opt(&cfg)
	}

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")

			switch {
			case len(cfg.Origins) == 0:
				// Do not set header.
			case len(cfg.Origins) == 1 && cfg.Origins[0] == "*":
				w.Header().Set("Access-Control-Allow-Origin", "*")
			default:
				for _, allowed := range cfg.Origins {
					if allowed == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
						break
					}
				}
			}

			w.Header().Set(
				"Access-Control-Allow-Methods",
				strings.Join(cfg.Methods, ", "),
			)

			w.Header().Set(
				"Access-Control-Allow-Headers",
				strings.Join(cfg.Headers, ", "),
			)

			w.Header().Set(
				"Access-Control-Max-Age",
				strconv.FormatUint(uint64(cfg.MaxAge), 10),
			)

			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	})
}
