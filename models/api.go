package models

import "errors"

const maximumLimit = 100

var InvalidPaginatorParams = errors.New("invalid paginator parameters")

type Paginator struct {
	Offset uint32
	Limit  uint32
}

func (p Paginator) Error() error {
	if p.Limit > maximumLimit {
		return InvalidPaginatorParams
	}
	return nil
}
