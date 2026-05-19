package request

import (
	"errors"

	"gofr.dev/pkg/gofr"

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

func PaginatorOf(ctx *gofr.Context) Paginator {
	offsetStr := ctx.Param("offset")
	limitStr := ctx.Param("limit")
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
