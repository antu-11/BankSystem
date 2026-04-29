// Package errs defines domain-specific error types for OmniLedger.
package errs

import "errors"

var (
	// ErrInsufficientFunds is returned when a debit would cause a negative balance.
	ErrInsufficientFunds = errors.New("insufficient funds")

	// ErrAccountNotFound is returned when the target account does not exist.
	ErrAccountNotFound = errors.New("account not found")

	// ErrAccountNotActive is returned when an account is Frozen or Closed.
	ErrAccountNotActive = errors.New("account is not active")

	// ErrDuplicateTransaction is returned when an idempotency key has already been processed.
	ErrDuplicateTransaction = errors.New("duplicate transaction: idempotency key already processed")

	// ErrSameAccount is returned when sender and receiver are the same account.
	ErrSameAccount = errors.New("cannot transfer to the same account")

	// ErrInvalidAmount is returned when the amount is zero or negative.
	ErrInvalidAmount = errors.New("transfer amount must be positive")

	// ── Auth Errors ─────────────────────────────────────────────────────

	// ErrInvalidCredentials is returned when username or password is wrong.
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrUsernameTaken is returned when a username is already registered.
	ErrUsernameTaken = errors.New("username already taken")

	// ErrEmailTaken is returned when an email is already registered.
	ErrEmailTaken = errors.New("email already taken")

	// ErrMissingFields is returned when required fields are empty.
	ErrMissingFields = errors.New("missing required fields")

	// ── Funding Errors ──────────────────────────────────────────────────

	// ErrNotSystemUser is returned when a non-system user tries to mint funds.
	ErrNotSystemUser = errors.New("forbidden: system user access required")
)
