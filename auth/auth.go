package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/fmotalleb/hermes/runtime"
	"github.com/fmotalleb/hermes/web"
)

const (
	defaultAdminUser = "admin"
	defaultAdminPass = "admin"
)

func Middleware(cfg runtime.Config) web.Middleware {
	if cfg.AdminNoAuth {
		return func(next http.Handler) http.Handler { return next }
	}

	user := cfg.AdminUser
	if user == "" {
		user = defaultAdminUser
	}
	pass := cfg.AdminPass
	if pass == "" {
		pass = defaultAdminPass
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()

			validUser := subtle.ConstantTimeCompare([]byte(username), []byte(user)) == 1
			validPass := subtle.ConstantTimeCompare([]byte(password), []byte(pass)) == 1

			if !ok || !validUser || !validPass {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin", charset="UTF-8"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func DisabledFromEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "y", "yes", "t", "true", "1":
		return true
	default:
		return false
	}
}
