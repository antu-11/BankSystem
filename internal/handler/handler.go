// Package handler contains the HTTP transport layer for the OmniLedger API.
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/service"
)

// TransferHandler exposes HTTP endpoints for the transfer service.
type TransferHandler struct {
	svc *service.TransferService
}

// NewTransferHandler returns a handler wired to the given transfer service.
func NewTransferHandler(svc *service.TransferService) *TransferHandler {
	return &TransferHandler{svc: svc}
}

// RegisterRoutes mounts the handler's endpoints on the given mux.
func (h *TransferHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/transfers", h.handleTransfer)
	mux.HandleFunc("GET /api/v1/health", handleHealth)
}

// ServeTransfer is the public entry point for the transfer handler,
// allowing it to be wrapped by middleware in main.go.
func (h *TransferHandler) ServeTransfer(w http.ResponseWriter, r *http.Request) {
	h.handleTransfer(w, r)
}

func (h *TransferHandler) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req model.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.svc.TransferFunds(r.Context(), req)
	if err != nil {
		code := mapErrorToHTTP(err)
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "omniledger"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[ERROR] writeJSON: %v", err)
	}
}

func mapErrorToHTTP(err error) int {
	switch {
	case errors.Is(err, errs.ErrInsufficientFunds):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errs.ErrAccountNotFound):
		return http.StatusNotFound
	case errors.Is(err, errs.ErrAccountNotActive):
		return http.StatusForbidden
	case errors.Is(err, errs.ErrDuplicateTransaction):
		return http.StatusConflict
	case errors.Is(err, errs.ErrSameAccount):
		return http.StatusBadRequest
	case errors.Is(err, errs.ErrInvalidAmount):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
