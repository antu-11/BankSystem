// Package service contains the core business logic for The Vault.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/store"
)

const (
	// idempotencyTTL is how long an idempotency key is remembered in Redis.
	idempotencyTTL = 24 * time.Hour

	// redisKeyPrefix namespaces idempotency keys in Redis.
	redisKeyPrefix = "vault:idempotency:"
)

// TransferService orchestrates fund transfers using double-entry bookkeeping.
type TransferService struct {
	store *store.Store
	rdb   *redis.Client
}

// NewTransferService returns a TransferService wired to the given store and Redis client.
func NewTransferService(s *store.Store, rdb *redis.Client) *TransferService {
	return &TransferService{store: s, rdb: rdb}
}

// TransferFunds executes an atomic fund transfer between two accounts.
//
// The operation follows this sequence inside a single SQL transaction:
//
//  1. Idempotency check  — Redis SET NX to reject duplicate requests.
//  2. Row-level locking  — SELECT FOR UPDATE on both accounts (ordered by UUID
//     to prevent deadlocks).
//  3. Validation          — Both accounts must be Active.
//  4. Balance check       — Dynamically computed as sum(credits) - sum(debits).
//  5. Transaction record  — Insert with status Pending.
//  6. Double-entry ledger — One Debit row (sender) + one Credit row (receiver).
//  7. Status promotion    — Mark transaction Completed.
//
// If any step fails, the SQL transaction rolls back and the Redis idempotency
// key is removed so the client can safely retry.
func (ts *TransferService) TransferFunds(ctx context.Context, req model.TransferRequest) (*model.TransferResponse, error) {
	start := time.Now()

	if req.FromAccountID == req.ToAccountID {
		return nil, errs.ErrSameAccount
	}
	if !req.Amount.IsPositive() {
		return nil, errs.ErrInvalidAmount
	}

	idempKey := redisKeyPrefix + req.IdempotencyKey.String()
	wasSet, err := ts.rdb.SetNX(ctx, idempKey, "processing", idempotencyTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("redis idempotency check: %w", err)
	}
	if !wasSet {
		return nil, errs.ErrDuplicateTransaction
	}

	// Release the idempotency lock on failure so the caller can retry.
	rollbackIdemKey := true
	defer func() {
		if rollbackIdemKey {
			if delErr := ts.rdb.Del(ctx, idempKey).Err(); delErr != nil {
				slog.Warn("failed to rollback idempotency key",
					"idempotency_key", req.IdempotencyKey,
					"error", delErr,
				)
			}
		}
	}()

	txnID := uuid.New()

	// Lock accounts in UUID order to prevent A->B / B->A deadlocks.
	firstLock, secondLock := req.FromAccountID, req.ToAccountID
	if req.FromAccountID.String() > req.ToAccountID.String() {
		firstLock, secondLock = req.ToAccountID, req.FromAccountID
	}

	err = ts.store.WithTx(ctx, func(tx *sqlx.Tx) error {

		_, err := ts.store.GetAccountForUpdate(ctx, tx, firstLock)
		if err != nil {
			return err
		}
		_, err = ts.store.GetAccountForUpdate(ctx, tx, secondLock)
		if err != nil {
			return err
		}

		// Re-fetch in logical roles; locks are already held.
		fromAcct, err := ts.store.GetAccountForUpdate(ctx, tx, req.FromAccountID)
		if err != nil {
			return fmt.Errorf("sender account: %w", err)
		}
		toAcct, err := ts.store.GetAccountForUpdate(ctx, tx, req.ToAccountID)
		if err != nil {
			return fmt.Errorf("receiver account: %w", err)
		}

		if fromAcct.Status != model.AccountStatusActive {
			return fmt.Errorf("sender %w", errs.ErrAccountNotActive)
		}
		if toAcct.Status != model.AccountStatusActive {
			return fmt.Errorf("receiver %w", errs.ErrAccountNotActive)
		}

		senderBalance, err := ts.store.GetBalance(ctx, tx, req.FromAccountID)
		if err != nil {
			return fmt.Errorf("sender balance: %w", err)
		}

		if senderBalance.LessThan(req.Amount) {
			return errs.ErrInsufficientFunds
		}

		txn := &model.Transaction{
			ID:             txnID,
			FromAccountID:  req.FromAccountID,
			ToAccountID:    req.ToAccountID,
			Amount:         req.Amount,
			Status:         model.TxnStatusPending,
			IdempotencyKey: req.IdempotencyKey,
		}
		if err := ts.store.CreateTransaction(ctx, tx, txn); err != nil {
			return err
		}

		debitEntry := &model.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     req.FromAccountID,
			TransactionID: txnID,
			Amount:        req.Amount,
			EntryType:     model.EntryTypeDebit,
		}
		creditEntry := &model.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     req.ToAccountID,
			TransactionID: txnID,
			Amount:        req.Amount,
			EntryType:     model.EntryTypeCredit,
		}

		if err := ts.store.InsertLedgerEntry(ctx, tx, debitEntry); err != nil {
			return fmt.Errorf("debit ledger: %w", err)
		}
		if err := ts.store.InsertLedgerEntry(ctx, tx, creditEntry); err != nil {
			return fmt.Errorf("credit ledger: %w", err)
		}

		if err := ts.store.UpdateTransactionStatus(ctx, tx, txnID, model.TxnStatusCompleted); err != nil {
			return err
		}

		return nil
	})

	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		slog.Error("transfer failed",
			"transaction_id", txnID,
			"idempotency_key", req.IdempotencyKey,
			"from_account", req.FromAccountID,
			"to_account", req.ToAccountID,
			"amount", req.Amount.StringFixed(4),
			"status", "failed",
			"latency_ms", latencyMs,
			"error", err.Error(),
		)
		return nil, err
	}

	rollbackIdemKey = false
	_ = ts.rdb.Set(ctx, idempKey, txnID.String(), idempotencyTTL).Err()

	slog.Info("transfer completed",
		"transaction_id", txnID,
		"idempotency_key", req.IdempotencyKey,
		"from_account", req.FromAccountID,
		"to_account", req.ToAccountID,
		"amount", req.Amount.StringFixed(4),
		"status", "completed",
		"latency_ms", latencyMs,
	)

	return &model.TransferResponse{
		TransactionID: txnID,
		Status:        model.TxnStatusCompleted,
		Message:       "Transfer completed successfully",
	}, nil
}

// GetBalance returns the dynamically computed balance for an account.
func (ts *TransferService) GetBalance(ctx context.Context, accountID uuid.UUID) (decimal.Decimal, error) {
	var balance decimal.Decimal

	err := ts.store.WithTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := ts.store.GetAccountForUpdate(ctx, tx, accountID); err != nil {
			return err
		}
		b, err := ts.store.GetBalance(ctx, tx, accountID)
		if err != nil {
			return err
		}
		balance = b
		return nil
	})
	if err != nil {
		return decimal.Zero, err
	}
	return balance, nil
}
