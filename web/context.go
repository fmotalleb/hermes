package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Context wraps an HTTP request with path parameters, request body binding,
// and JSON response writing utilities.
type Context struct {
	context.Context

	Request        *http.Request
	ResponseWriter http.ResponseWriter
	Params         map[string]string
}

func (c *Context) PathParam(name string) string {
	if c == nil {
		return ""
	}
	return c.Params[name]
}

func (c *Context) QueryParam(name string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.URL.Query().Get(name)
}

func (c *Context) Bind(v any) error {
	if c == nil || c.Request == nil {
		return errors.New("missing request")
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}

	return nil
}

func (c *Context) JSON(status int, value any) {
	writeJSON(c.ResponseWriter, status, value)
}
