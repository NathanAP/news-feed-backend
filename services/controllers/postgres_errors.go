package controllers

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolation is SQLSTATE 23505, the code PostgreSQL returns for a UNIQUE constraint
// violation. It is part of the SQL standard's class 23 (integrity constraint violation), so it is
// stable across PostgreSQL versions and locales.
const pgUniqueViolation = "23505"

// isUniqueViolation reports whether err is a PostgreSQL UNIQUE constraint violation. It inspects the
// typed driver error instead of matching the error message text, which is brittle across driver
// versions and locales. pgx surfaces *pgconn.PgError through database/sql wrapping, so errors.As
// reaches it even though the app talks to the database through *sql.DB.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
