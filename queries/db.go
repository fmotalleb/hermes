// Package queries provides SQL query functions for all domain entities:
// zones, DNS records, forward zones, hijacks, and settings. It defines a
// minimal DB interface compatible with both *sql.DB and transactional executors.
package queries

import (
	"context"
	"database/sql"
)

// DB is the minimal interface required for executing SQL queries.
// Both *sql.DB and transactional executors satisfy this interface,
// enabling query reuse in both direct and transactional contexts.
type DB interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}
