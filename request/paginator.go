// Package request provides HTTP request parsing utilities, including a
// paginator that extracts limit/offset query parameters with sensible defaults
// and maximum bounds.
package request

import (
	"errors"

	"github.com/fmotalleb/hermes/convert"
)

const (
	defaultLimit  = uint32(50)
	defaultOffset = uint32(0)
	maximumLimit  = uint32(100)
)

// ErrInvalidPaginatorParams is returned when paginator query parameters are invalid.
var ErrInvalidPaginatorParams = errors.New("invalid paginator parameters")

// Paginator holds the pagination offset and limit extracted from query parameters.
type Paginator struct {
	Offset uint32
	Limit  uint32
}

type queryParamer interface {
	QueryParam(string) string
}

// PaginatorOf extracts and validates pagination parameters (offset, limit)
// from the request query string, applying sensible defaults and maximum bounds.
func PaginatorOf(ctx queryParamer) Paginator {
	offsetStr := ctx.QueryParam("offset")
	limitStr := ctx.QueryParam("limit")
	offset := convert.Uint32Or(offsetStr, defaultOffset)
	limit := convert.Uint32Or(limitStr, defaultLimit)
	if limit > maximumLimit {
		limit = maximumLimit
	}
	return Paginator{
		Offset: offset,
		Limit:  limit,
	}
}
