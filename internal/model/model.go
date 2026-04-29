// Package model defines the core domain types for OmniLedger.
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AccountStatus represents the lifecycle state of an account.
type AccountStatus string

const (
	AccountStatusActive AccountStatus = "Active"
	AccountStatusFrozen AccountStatus = "Frozen"
	AccountStatusClosed AccountStatus = "Closed"
)

// TransactionStatus represents the lifecycle state of a transaction.
type TransactionStatus string

const (
	TxnStatusPending   TransactionStatus = "Pending"
	TxnStatusCompleted TransactionStatus = "Completed"
	TxnStatusFailed    TransactionStatus = "Failed"
	TxnStatusReversed  TransactionStatus = "Reversed"
)

// EntryType distinguishes credit and debit ledger entries.
type EntryType string

const (
	EntryTypeCredit EntryType = "Credit"
	EntryTypeDebit  EntryType = "Debit"
)

// User represents a registered user or the internal system account.
type User struct {
	ID           uuid.UUID `db:"id"             json:"id"`
	Username     string    `db:"username"        json:"username"`
	Email        string    `db:"email"           json:"email"`
	PasswordHash string    `db:"password_hash"   json:"-"`
	FullName     string    `db:"full_name"       json:"full_name"`
	IsSystemUser bool      `db:"is_system_user"  json:"is_system_user"`
	IsActive     bool      `db:"is_active"       json:"is_active"`
	CreatedAt    time.Time `db:"created_at"      json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"      json:"updated_at"`
}

// Account represents a financial account. Balance is never stored here;
// it is always derived from the ledger.
type Account struct {
	ID        uuid.UUID     `db:"id"         json:"id"`
	UserID    uuid.UUID     `db:"user_id"    json:"user_id"`
	Currency  string        `db:"currency"   json:"currency"`
	Status    AccountStatus `db:"status"     json:"status"`
	CreatedAt time.Time     `db:"created_at" json:"created_at"`
}

// Transaction records a fund movement between two accounts.
type Transaction struct {
	ID             uuid.UUID         `db:"id"               json:"id"`
	FromAccountID  uuid.UUID         `db:"from_account_id"  json:"from_account_id"`
	ToAccountID    uuid.UUID         `db:"to_account_id"    json:"to_account_id"`
	Amount         decimal.Decimal   `db:"amount"           json:"amount"`
	Status         TransactionStatus `db:"status"           json:"status"`
	IdempotencyKey uuid.UUID         `db:"idempotency_key"  json:"idempotency_key"`
	CreatedAt      time.Time         `db:"created_at"       json:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at"       json:"updated_at"`
}

// LedgerEntry is an immutable, append-only record of a credit or debit
// against a single account. Two entries are created per completed transaction.
type LedgerEntry struct {
	ID            uuid.UUID       `db:"id"             json:"id"`
	AccountID     uuid.UUID       `db:"account_id"     json:"account_id"`
	TransactionID uuid.UUID       `db:"transaction_id" json:"transaction_id"`
	Amount        decimal.Decimal `db:"amount"         json:"amount"`
	EntryType     EntryType       `db:"entry_type"     json:"entry_type"`
	CreatedAt     time.Time       `db:"created_at"     json:"created_at"`
}

// TransferRequest is the inbound payload for initiating a fund transfer.
type TransferRequest struct {
	FromAccountID  uuid.UUID       `json:"from_account_id"  validate:"required"`
	ToAccountID    uuid.UUID       `json:"to_account_id"    validate:"required"`
	Amount         decimal.Decimal `json:"amount"           validate:"required,gt=0"`
	IdempotencyKey uuid.UUID       `json:"idempotency_key"  validate:"required"`
}

// TransferResponse is returned after a successful transfer.
type TransferResponse struct {
	TransactionID uuid.UUID         `json:"transaction_id"`
	Status        TransactionStatus `json:"status"`
	Message       string            `json:"message"`
}

// RegisterRequest is the inbound payload for user registration.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// LoginRequest is the inbound payload for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse is returned after successful registration or login.
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"-"` // set via HttpOnly cookie, not in response body
}

// FundRequest is the inbound payload for system funding (minting money).
type FundRequest struct {
	ToAccountID uuid.UUID       `json:"to_account_id"`
	Amount      decimal.Decimal `json:"amount"`
}

// FundResponse is returned after a successful funding operation.
type FundResponse struct {
	TransactionID uuid.UUID         `json:"transaction_id"`
	ToAccountID   uuid.UUID         `json:"to_account_id"`
	Amount        decimal.Decimal   `json:"amount"`
	Status        TransactionStatus `json:"status"`
	Message       string            `json:"message"`
}

// TransactionHistoryItem is a single row returned by the history endpoint.
// Direction is computed as "sent" or "received" relative to the queried account.
type TransactionHistoryItem struct {
	ID             uuid.UUID         `db:"id"               json:"id"`
	FromAccountID  uuid.UUID         `db:"from_account_id"  json:"from_account_id"`
	ToAccountID    uuid.UUID         `db:"to_account_id"    json:"to_account_id"`
	Amount         decimal.Decimal   `db:"amount"           json:"amount"`
	Status         TransactionStatus `db:"status"           json:"status"`
	IdempotencyKey uuid.UUID         `db:"idempotency_key"  json:"idempotency_key"`
	CreatedAt      time.Time         `db:"created_at"       json:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at"       json:"updated_at"`
	Direction      string            `db:"direction"        json:"direction"`
}

// PaginatedHistory wraps a page of transaction history items with metadata.
type PaginatedHistory struct {
	Items      []TransactionHistoryItem `json:"items"`
	Total      int                      `json:"total"`
	Page       int                      `json:"page"`
	PerPage    int                      `json:"per_page"`
	TotalPages int                      `json:"total_pages"`
}

// BalanceResponse is the response for the balance endpoint.
type BalanceResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Currency  string    `json:"currency"`
	Balance   string    `json:"balance"`
}
