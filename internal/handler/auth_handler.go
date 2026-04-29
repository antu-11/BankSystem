// Package handler — auth_handler.go
// HTTP endpoints for registration, login, and logout.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/akdandapat/OmniLedger/internal/auth"
	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/service"
)

// AuthHandler exposes HTTP endpoints for the auth service.
type AuthHandler struct {
	svc    *service.AuthService
	secure bool // true in production → sets Secure flag on cookies
}

// NewAuthHandler creates a new handler wired to the auth service.
func NewAuthHandler(svc *service.AuthService, secure bool) *AuthHandler {
	return &AuthHandler{svc: svc, secure: secure}
}

// RegisterAuthRoutes mounts auth endpoints that are NOT rate-limited.
// Login and Register are registered separately in main.go with rate limiting.
func (h *AuthHandler) RegisterAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/logout", h.handleLogout)
}

// HandleRegister is the public entry point for registration (used by rate limiter in main.go).
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	h.handleRegister(w, r)
}

// HandleLogin is the public entry point for login (used by rate limiter in main.go).
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	h.handleLogin(w, r)
}

// ──────────────────────────────────────────────────────────────────────────────
// POST /api/v1/auth/register
// ──────────────────────────────────────────────────────────────────────────────

func (h *AuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.svc.Register(r.Context(), req)
	if err != nil {
		code := mapAuthErrorToHTTP(err)
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	// Set JWT in HttpOnly cookie
	service.SetTokenCookie(w, resp.Token, h.secure)

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "registration successful",
		"user":    resp.User,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// POST /api/v1/auth/login
// ──────────────────────────────────────────────────────────────────────────────

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		code := mapAuthErrorToHTTP(err)
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	service.SetTokenCookie(w, resp.Token, h.secure)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "login successful",
		"user":    resp.User,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// POST /api/v1/auth/logout
// ──────────────────────────────────────────────────────────────────────────────

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Extract JTI from context (set by auth middleware or from cookie)
	jti := auth.JTIFromCtx(r.Context())
	if jti == "" {
		// Fallback: parse the cookie directly for logout-without-middleware
		cookie, err := r.Cookie("vault_token")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}
		// We need a token manager reference — use a minimal parse
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token context, cookie: " + cookie.Value[:8] + "..."})
		return
	}

	// Blacklist the token for its remaining TTL
	if err := h.svc.Logout(r.Context(), jti, time.Now().Add(24*time.Hour)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "logout failed"})
		return
	}

	service.ClearTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// ──────────────────────────────────────────────────────────────────────────────
// Error mapping
// ──────────────────────────────────────────────────────────────────────────────

func mapAuthErrorToHTTP(err error) int {
	switch {
	case errors.Is(err, errs.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, errs.ErrUsernameTaken), errors.Is(err, errs.ErrEmailTaken):
		return http.StatusConflict
	case errors.Is(err, errs.ErrMissingFields):
		return http.StatusBadRequest
	case errors.Is(err, errs.ErrAccountNotActive):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
