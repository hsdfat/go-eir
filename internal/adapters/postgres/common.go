package postgres

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// dbExecutor is an interface that both *sqlx.DB and *sqlx.Tx implement
// This allows repositories to work with either a database connection or a transaction
type dbExecutor interface {
	sqlx.Queryer
	sqlx.Execer
	sqlx.Preparer
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}
