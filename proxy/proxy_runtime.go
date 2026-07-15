package proxy

import (
	"context"
	"net/url"
	"time"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/queries"
)

type Proxy struct {
	ListenHTTP string        `env:"HERMES_HTTP_LISTEN" default:"0.0.0.0:80"`
	ListenTLS  string        `env:"HERMES_TLS_LISTEN" default:"0.0.0.0:443"`
	Timeout    time.Duration `env:"HERMES_PROXY_TIMEOUT" default:"1m"`
	ProxyAddr  *url.URL

	cache cache.Cache
	db    queries.DB
}

const allowedHostCachePrefix = "proxy:allowed-host:"

func (p *Proxy) AllowedHost(ctx context.Context, host string) bool {
	if host == "" {
		return false
	}

	if p.cache != nil {
		if _, err := p.cache.GetBytes(ctx, allowedHostCachePrefix+host); err == nil {
			return true
		}
	}

	if p.db == nil {
		return false
	}

	var exists int
	if err := p.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM zones WHERE name = $1 LIMIT 1;`,
		host,
	).Scan(&exists); err != nil {
		return false
	}

	if p.cache != nil {
		_ = p.cache.Set(ctx, allowedHostCachePrefix+host, []byte{1}, 5*time.Minute)
	}

	return true
}
