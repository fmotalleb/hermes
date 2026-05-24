package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr"
)

const (
	defaultAdminUser = "admin"
	defaultAdminPass = "admin"

	adminUserConfig = "ADMIN_USER"
	adminPassConfig = "ADMIN_PASS"

	adminDisableAuthConfig = "ADMIN_NO_AUTH"
)

func Register(app *gofr.App) {
	disableAdminStr := app.Config.GetOrDefault(adminDisableAuthConfig, "false")

	enable := true
	switch strings.ToLower(disableAdminStr) {
	case "y", "yes", "t", "true", "1":
		enable = false
	}

	if !enable {
		return
	}

	user := app.Config.GetOrDefault(adminUserConfig, defaultAdminUser)
	pass := app.Config.GetOrDefault(adminPassConfig, defaultAdminPass)

	app.UseMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()

			validUser := subtle.ConstantTimeCompare(
				[]byte(username),
				[]byte(user),
			) == 1

			validPass := subtle.ConstantTimeCompare(
				[]byte(password),
				[]byte(pass),
			) == 1

			if !ok || !validUser || !validPass {
				w.Header().Set(
					"WWW-Authenticate",
					`Basic realm="admin", charset="UTF-8"`,
				)

				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	})
}
