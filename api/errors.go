package api

import (
	"errors"

	"github.com/lib/pq"

	"gofr.dev/pkg/gofr/http"
)

func normalizeCreateError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return http.ErrorEntityAlreadyExist{}
	}

	return err
}
