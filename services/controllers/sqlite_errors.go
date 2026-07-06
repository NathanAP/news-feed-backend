package controllers

import (
	"errors"

	sqlitedriver "modernc.org/sqlite"
)

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE, the extended result code SQLite returns for a
// UNIQUE constraint violation. modernc.org/sqlite enables extended result codes on connection open,
// so a unique conflict always surfaces with exactly this code.
const sqliteConstraintUnique = 2067

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint violation. It inspects the
// typed driver result code instead of matching the error message text, which is brittle across
// driver versions and locales.
func isUniqueViolation(err error) bool {
	var serr *sqlitedriver.Error
	return errors.As(err, &serr) && serr.Code() == sqliteConstraintUnique
}
