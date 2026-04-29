package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/akdandapat/OmniLedger/internal/auth"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/service"
)

// FundingHandler exposes the system funding endpoint.
type FundingHandler struct {
	svc *service.FundingService
	db  *sqlx.DB
}

// NewFundingHandler returns a handler wired to the given funding service.
func NewFundingHandler(svc *service.FundingService, db *sqlx.DB) *FundingHandler {
	return &FundingHandler{svc: svc, db: db}
}

// RegisterFundingRoutes mounts the funding endpoint behind auth middleware
// and an additional system-user gate that verifies privilege against the database.
func (h *FundingHandler) RegisterFundingRoutes(mux *http.ServeMux, authMw *auth.Middleware) {
	protected := authMw.Protect(http.HandlerFunc(h.handleFund))
	systemOnly := h.requireSystemUser(protected)
	mux.Handle("POST /api/v1/transactions/system/fund", systemOnly)
}

func (h *FundingHandler) handleFund(w http.ResponseWriter, r *http.Request) {
	var req model.FundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	systemUserID := auth.UserIDFromCtx(r.Context())

	resp, err := h.svc.FundAccount(r.Context(), req, systemUserID)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// requireSystemUser is a secondary middleware that runs after auth.Protect.
// It verifies is_system_user directly from the database — never trusting
// the JWT claim alone for privilege escalation.
func (h *FundingHandler) requireSystemUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := auth.UserIDFromCtx(r.Context())
		if userID == uuid.Nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}

		isSystem, err := h.isSystemUserInDB(r.Context(), userID)
		if err != nil {
			log.Printf("[ERROR] system user check: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "authorization check failed"})
			return
		}
		if !isSystem {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden: system user access required"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *FundingHandler) isSystemUserInDB(ctx context.Context, userID uuid.UUID) (bool, error) {
	var isSystem bool
	query := `SELECT is_system_user FROM users WHERE id = $1`
	err := h.db.GetContext(ctx, &isSystem, query, userID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check system user: %w", err)
	}
	return isSystem, nil
}
