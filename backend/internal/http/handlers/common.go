package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/omnia/clinic-service/internal/auth"
	"github.com/omnia/clinic-service/internal/http/middleware"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func claimsFromRequest(r *http.Request) (*auth.Claims, bool) {
	return middleware.ClaimsFromContext(r.Context())
}

// orEmptySlice returns [] instead of a nil slice in JSON — so a caller
// always gets a real array to iterate, never "null", regardless of
// whether anything matched.
func orEmptySlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// formatPgTime/parsePgTime translate pgtype.Time (microseconds since
// midnight, Postgres's own wire format for a TIME column) to/from a
// plain "HH:MM" string — the shape a shift-editing UI actually works
// with, so callers never have to know pgtype's internal representation.
func formatPgTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	d := time.Duration(t.Microseconds) * time.Microsecond
	return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

func parsePgTime(s string) (pgtype.Time, error) {
	parsed, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	micros := (parsed.Hour()*3600 + parsed.Minute()*60) * 1_000_000
	return pgtype.Time{Microseconds: int64(micros), Valid: true}, nil
}

// isUniqueViolation is true only for Postgres error code 23505 — a UNIQUE
// constraint failing (e.g. staff_users.email already taken). Lets a
// handler return a real 409 Conflict with a human-readable message
// instead of a raw 500 with Postgres's own constraint-name error text.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
