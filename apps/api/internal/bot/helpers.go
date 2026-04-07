package bot

import "database/sql"

// toNullString converts a plain string to sql.NullString.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
