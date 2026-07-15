package api

import (
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type entityAlreadyExistsError struct{}

func (entityAlreadyExistsError) Error() string   { return "entity already exists" }
func (entityAlreadyExistsError) StatusCode() int { return 409 }
func (entityAlreadyExistsError) Body() any {
	return map[string]string{"error": "entity already exists"}
}

type entityNotFoundError struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (e entityNotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Name, e.Value)
}

func (e entityNotFoundError) StatusCode() int { return 404 }
func (e entityNotFoundError) Body() any       { return e }

// ErrEntityAlreadyExist is returned when attempting to create a resource that conflicts with an existing one.
var ErrEntityAlreadyExist = entityAlreadyExistsError{}

func normalizeCreateError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return ErrEntityAlreadyExist
	}

	return err
}
