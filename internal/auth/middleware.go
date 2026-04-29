package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// contextKey is an unexported type to prevent collisions in context values.
type contextKey string

const (
	// CtxUserID is the context key for the authenticated user's UUID.
	CtxUserID contextKey = "user_id"

	// CtxUsername is the context key for the authenticated user's username.
	CtxUsername contextKey = "username"

	// CtxJTI is the context key for the JWT ID (needed for logout).
	CtxJTI contextKey = "jti"

	// cookieName is the HttpOnly cookie that carries the JWT.
	cookieName = "vault_token"
)

// Middleware wraps an http.Handler with JWT, blacklist, and account-status checks.
type Middleware struct {
	tm *TokenManager
	db *sqlx.DB
}

// NewMiddleware returns a Middleware configured with the given token manager and database.
func NewMiddleware(tm *TokenManager, db *sqlx.DB) *Middleware {
	return &Middleware{tm: tm, db: db}
}

// Protect returns a handler that enforces authentication on every request.
//
// Pipeline:
//  1. Extract JWT from the HttpOnly cookie.
//  2. Validate signature and expiry.
//  3. Check Redis blacklist — reject if token has been revoked.
//  4. Load account status from Postgres — reject if Frozen.
//  5. Inject user_id, username, and jti into the request context.
func (m *Middleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie(cookieName)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "missing authentication cookie")
			return
		}

		claims, err := m.tm.ValidateToken(cookie.Value)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		blacklisted, err := m.tm.IsBlacklisted(r.Context(), claims.ID)
		if err != nil {
			log.Printf("[ERROR] blacklist check: %v", err)
			writeAuthError(w, http.StatusInternalServerError, "authentication error")
			return
		}
		if blacklisted {
			writeAuthError(w, http.StatusUnauthorized, "token has been revoked")
			return
		}

		frozen, err := m.isAccountFrozen(r.Context(), claims.UserID)
		if err != nil {
			log.Printf("[ERROR] account status check: %v", err)
			writeAuthError(w, http.StatusInternalServerError, "authentication error")
			return
		}
		if frozen {
			writeAuthError(w, http.StatusForbidden, "account is frozen")
			return
		}

		ctx := context.WithValue(r.Context(), CtxUserID, claims.UserID)
		ctx = context.WithValue(ctx, CtxUsername, claims.Username)
		ctx = context.WithValue(ctx, CtxJTI, claims.ID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isAccountFrozen reports whether any account owned by userID has status 'Frozen'.
func (m *Middleware) isAccountFrozen(ctx context.Context, userID uuid.UUID) (bool, error) {
	var status string
	query := `SELECT status FROM accounts WHERE user_id = $1 AND status = 'Frozen' LIMIT 1`
	err := m.db.GetContext(ctx, &status, query, userID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check frozen: %w", err)
	}
	return true, nil
}

// UserIDFromCtx extracts the authenticated user's UUID from the context.
func UserIDFromCtx(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(CtxUserID).(uuid.UUID)
	return v
}

// UsernameFromCtx extracts the authenticated username from the context.
func UsernameFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(CtxUsername).(string)
	return v
}

// JTIFromCtx extracts the JWT ID from the context.
func JTIFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(CtxJTI).(string)
	return v
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
