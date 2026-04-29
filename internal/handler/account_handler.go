package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/akdandapat/OmniLedger/internal/auth"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/store"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// AccountHandler exposes data-fetching endpoints for accounts.
type AccountHandler struct {
	store *store.Store
}

// NewAccountHandler returns a handler wired to the given store.
func NewAccountHandler(s *store.Store) *AccountHandler {
	return &AccountHandler{store: s}
}

// RegisterAccountRoutes mounts account endpoints on the mux, protected by auth middleware.
func (h *AccountHandler) RegisterAccountRoutes(mux *http.ServeMux, authMw *auth.Middleware) {
	mux.Handle("GET /api/v1/accounts", authMw.Protect(http.HandlerFunc(h.handleGetAccounts)))
	mux.Handle("GET /api/v1/accounts/{id}/balance", authMw.Protect(http.HandlerFunc(h.handleGetBalance)))
	mux.Handle("GET /api/v1/accounts/{id}/history", authMw.Protect(http.HandlerFunc(h.handleGetHistory)))
}

func (h *AccountHandler) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromCtx(r.Context())
	if userID == uuid.Nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	accounts, err := h.store.GetAccountsByUserID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch accounts"})
		return
	}

	if accounts == nil {
		accounts = []model.Account{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

func (h *AccountHandler) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromCtx(r.Context())
	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}

	account, ok := h.verifyOwnership(w, r, userID, accountID)
	if !ok {
		return
	}

	balance, err := h.store.GetBalanceReadOnly(r.Context(), accountID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute balance"})
		return
	}

	writeJSON(w, http.StatusOK, model.BalanceResponse{
		AccountID: accountID,
		Currency:  account.Currency,
		Balance:   balance.StringFixed(4),
	})
}

func (h *AccountHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromCtx(r.Context())
	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}

	if _, ok := h.verifyOwnership(w, r, userID, accountID); !ok {
		return
	}

	page := parseIntParam(r, "page", defaultPage)
	perPage := parseIntParam(r, "per_page", defaultPerPage)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (page - 1) * perPage

	items, total, err := h.store.GetTransactionHistory(r.Context(), accountID, perPage, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch history"})
		return
	}

	if items == nil {
		items = []model.TransactionHistoryItem{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	writeJSON(w, http.StatusOK, model.PaginatedHistory{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

// verifyOwnership ensures the requested account belongs to the authenticated user.
// Returns the account and true if valid, or writes an error response and returns false.
func (h *AccountHandler) verifyOwnership(
	w http.ResponseWriter,
	r *http.Request,
	userID, accountID uuid.UUID,
) (*model.Account, bool) {
	accounts, err := h.store.GetAccountsByUserID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify account ownership"})
		return nil, false
	}

	for i := range accounts {
		if accounts[i].ID == accountID {
			return &accounts[i], true
		}
	}

	writeJSON(w, http.StatusForbidden, map[string]string{"error": "account does not belong to authenticated user"})
	return nil, false
}

func parseIntParam(r *http.Request, key string, fallback int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}
