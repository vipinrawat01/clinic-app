package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/omnia/clinic-service/internal/auth"
)

type contextKey string

const (
	claimsContextKey        contextKey = "claims"
	serviceCallerContextKey contextKey = "service_caller"
)

// RequireStaffAuth authenticates a real staff member's own session — the
// browser UI's own login, not the main chatbot platform calling in (see
// RequireServiceAuth for that path). Every handler that reads/writes
// clinical data a human is responsible for (creating a patient record,
// cancelling an appointment, recording a consult) goes through this one,
// never the service-key path, so there's always a real accountable
// staff_user_id behind the action.
func RequireStaffAuth(issuer *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			claims, err := issuer.Parse(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireServiceAuth authenticates the main chatbot platform's own
// Action-node calls (checking availability, booking a slot) via a single
// shared secret, not a per-staff login — there is no human staff session
// behind a conversational booking request, just the main platform acting
// on a patient's behalf. Deliberately a single static key, not a full API
// key management system: this app only ever has one caller (the main
// platform), unlike the main platform's own multi-tenant API keys which
// serve arbitrary third-party integrations.
func RequireServiceAuth(serviceKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			provided := strings.TrimPrefix(header, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(serviceKey)) != 1 {
				http.Error(w, "invalid service key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), serviceCallerContextKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdminRole gates staff-management routes (creating, editing, or
// deactivating other staff logins) to the "admin" role only — must run
// after RequireStaffAuth, which is what actually populates the claims this
// reads. Without this, any logged-in receptionist or doctor account could
// create new logins or deactivate someone else's, since every other
// clinical route (patients, appointments) is deliberately open to any
// authenticated staff member and carries no role check of its own.
func RequireAdminRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			http.Error(w, "admin role required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*auth.Claims)
	return claims, ok
}
