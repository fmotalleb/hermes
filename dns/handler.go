package dns

import (
	"gofr.dev/pkg/gofr/logging"
)

type handler struct {
	store  *store
	logger logging.Logger
}
