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

var InvalidPaginatorParams = errors.New("invalid paginator parameters")

type Paginator struct {
	Offset uint32
	Limit  uint32
}

type queryParamer interface {
	QueryParam(string) string
}

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
