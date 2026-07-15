package proxy

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/queries"
)

type Proxy struct {
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
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		return false
	}

	if p.cache != nil {
		_ = p.cache.Set(ctx, allowedHostCachePrefix+host, []byte{1}, 5*time.Minute)
	}

	return true
}
