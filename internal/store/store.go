// Package store provides the data-access layer for OmniLedger.
// All queries run through sqlx and operate on the PostgreSQL schema.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
)

// Store wraps the database connection and exposes repository methods.
type Store struct {
	db *sqlx.DB
}

// New returns a Store backed by the given database connection.
func New(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// TxFn is a function that executes within a database transaction.
type TxFn func(tx *sqlx.Tx) error

// WithTx starts a serializable transaction, executes fn, and commits or
// rolls back depending on the returned error.
func (s *Store) WithTx(ctx context.Context, fn TxFn) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetAccountForUpdate fetches a single account using SELECT FOR UPDATE,
// acquiring a row-level exclusive lock within the current transaction.
func (s *Store) GetAccountForUpdate(ctx context.Context, tx *sqlx.Tx, accountID uuid.UUID) (*model.Account, error) {
	var acct model.Account
	query := `SELECT id, user_id, currency, status, created_at
	           FROM accounts
	           WHERE id = $1
	           FOR UPDATE`

	if err := tx.GetContext(ctx, &acct, query, accountID); err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account for update: %w", err)
	}
	return &acct, nil
}

// GetBalance dynamically computes the current balance of an account from the
// ledger as sum(credits) - sum(debits). This is the single source of truth.
func (s *Store) GetBalance(ctx context.Context, tx *sqlx.Tx, accountID uuid.UUID) (decimal.Decimal, error) {
	var balance decimal.Decimal
	query := `SELECT COALESCE(
	              SUM(CASE
	                  WHEN entry_type = 'Credit' THEN  amount
	                  WHEN entry_type = 'Debit'  THEN -amount
	              END),
	              0
	          )
	          FROM ledger
	          WHERE account_id = $1`

	if err := tx.GetContext(ctx, &balance, query, accountID); err != nil {
		return decimal.Zero, fmt.Errorf("get balance: %w", err)
	}
	return balance, nil
}

// CreateTransaction inserts a new transaction row.
func (s *Store) CreateTransaction(ctx context.Context, tx *sqlx.Tx, t *model.Transaction) error {
	query := `INSERT INTO transactions (id, from_account_id, to_account_id, amount, status, idempotency_key)
	          VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := tx.ExecContext(ctx, query,
		t.ID, t.FromAccountID, t.ToAccountID, t.Amount, t.Status, t.IdempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

// UpdateTransactionStatus sets the status on an existing transaction row.
func (s *Store) UpdateTransactionStatus(ctx context.Context, tx *sqlx.Tx, txnID uuid.UUID, status model.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1 WHERE id = $2`
	res, err := tx.ExecContext(ctx, query, status, txnID)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("transaction %s not found", txnID)
	}
	return nil
}

// InsertLedgerEntry appends an immutable ledger row.
func (s *Store) InsertLedgerEntry(ctx context.Context, tx *sqlx.Tx, entry *model.LedgerEntry) error {
	query := `INSERT INTO ledger (id, account_id, transaction_id, amount, entry_type)
	          VALUES ($1, $2, $3, $4, $5)`

	_, err := tx.ExecContext(ctx, query,
		entry.ID, entry.AccountID, entry.TransactionID, entry.Amount, entry.EntryType,
	)
	if err != nil {
		return fmt.Errorf("insert ledger entry: %w", err)
	}
	return nil
}

// GetUserByUsername returns the user with the given username, or an error.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	query := `SELECT id, username, email, password_hash, full_name,
	                 is_system_user, is_active, created_at, updated_at
	          FROM users WHERE username = $1`
	if err := s.db.GetContext(ctx, &user, query, username); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &user, nil
}

// GetUserByEmail returns the user with the given email, or an error.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	query := `SELECT id, username, email, password_hash, full_name,
	                 is_system_user, is_active, created_at, updated_at
	          FROM users WHERE email = $1`
	if err := s.db.GetContext(ctx, &user, query, email); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

// IsFirstUser reports whether the users table is empty.
func (s *Store) IsFirstUser(ctx context.Context) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users`
	if err := s.db.GetContext(ctx, &count, query); err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count == 0, nil
}

// CreateUser inserts a new user row.
func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users (id, username, email, password_hash, full_name, is_system_user, is_active)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.ExecContext(ctx, query,
		u.ID, u.Username, u.Email, u.PasswordHash, u.FullName, u.IsSystemUser, u.IsActive,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// CreateAccount inserts a new account row.
func (s *Store) CreateAccount(ctx context.Context, a *model.Account) error {
	query := `INSERT INTO accounts (id, user_id, currency, status)
	          VALUES ($1, $2, $3, $4)`
	_, err := s.db.ExecContext(ctx, query, a.ID, a.UserID, a.Currency, a.Status)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

// GetAccountByUserID returns the first active account for a given user.
func (s *Store) GetAccountByUserID(ctx context.Context, userID uuid.UUID) (*model.Account, error) {
	var acct model.Account
	query := `SELECT id, user_id, currency, status, created_at
	          FROM accounts
	          WHERE user_id = $1 AND status = 'Active'
	          LIMIT 1`
	if err := s.db.GetContext(ctx, &acct, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account by user: %w", err)
	}
	return &acct, nil
}

// GetAccountsByUserID returns all accounts belonging to a user.
func (s *Store) GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	var accounts []model.Account
	query := `SELECT id, user_id, currency, status, created_at
	          FROM accounts
	          WHERE user_id = $1
	          ORDER BY created_at ASC`
	if err := s.db.SelectContext(ctx, &accounts, query, userID); err != nil {
		return nil, fmt.Errorf("get accounts by user: %w", err)
	}
	return accounts, nil
}

// GetBalanceReadOnly computes the current balance without acquiring a row-level
// lock. Safe for read-only display endpoints. COALESCE handles the zero-entry
// edge case, returning 0 instead of NULL.
func (s *Store) GetBalanceReadOnly(ctx context.Context, accountID uuid.UUID) (decimal.Decimal, error) {
	var balance decimal.Decimal
	query := `SELECT COALESCE(
	              SUM(CASE
	                  WHEN entry_type = 'Credit' THEN  amount
	                  WHEN entry_type = 'Debit'  THEN -amount
	              END),
	              0
	          )
	          FROM ledger
	          WHERE account_id = $1`

	if err := s.db.GetContext(ctx, &balance, query, accountID); err != nil {
		return decimal.Zero, fmt.Errorf("get balance read-only: %w", err)
	}
	return balance, nil
}

// GetTransactionHistory returns a paginated list of transactions involving
// the given account (both sent and received), ordered by created_at DESC.
func (s *Store) GetTransactionHistory(
	ctx context.Context,
	accountID uuid.UUID,
	limit, offset int,
) ([]model.TransactionHistoryItem, int, error) {

	var total int
	countQuery := `SELECT COUNT(*)
	               FROM transactions
	               WHERE from_account_id = $1 OR to_account_id = $1`
	if err := s.db.GetContext(ctx, &total, countQuery, accountID); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	var items []model.TransactionHistoryItem
	query := `SELECT
	              t.id,
	              t.from_account_id,
	              t.to_account_id,
	              t.amount,
	              t.status,
	              t.idempotency_key,
	              t.created_at,
	              t.updated_at,
	              CASE
	                  WHEN t.from_account_id = $1 THEN 'sent'
	                  ELSE 'received'
	              END AS direction
	          FROM transactions t
	          WHERE t.from_account_id = $1 OR t.to_account_id = $1
	          ORDER BY t.created_at DESC
	          LIMIT $2 OFFSET $3`

	if err := s.db.SelectContext(ctx, &items, query, accountID, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("get transaction history: %w", err)
	}

	return items, total, nil
}
