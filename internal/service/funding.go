package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/store"
)

// FundingService handles system-level funding (minting) operations.
type FundingService struct {
	store *store.Store
}

// NewFundingService returns a FundingService wired to the given store.
func NewFundingService(s *store.Store) *FundingService {
	return &FundingService{store: s}
}

// FundAccount creates a system transaction that credits funds into a target
// account. This is a single-entry operation — no sender is debited.
//
// Sequence inside a single SQL transaction:
//  1. Validate the target account exists and is Active.
//  2. Create a Transaction record (from = system account, to = target).
//  3. Insert a single Credit ledger entry for the target account.
//  4. Mark the transaction as Completed.
func (fs *FundingService) FundAccount(ctx context.Context, req model.FundRequest, systemUserID uuid.UUID) (*model.FundResponse, error) {

	if !req.Amount.IsPositive() {
		return nil, errs.ErrInvalidAmount
	}

	txnID := uuid.New()

	systemAcct, err := fs.store.GetAccountByUserID(ctx, systemUserID)
	if err != nil {
		return nil, fmt.Errorf("system account lookup: %w", err)
	}

	err = fs.store.WithTx(ctx, func(tx *sqlx.Tx) error {

		targetAcct, err := fs.store.GetAccountForUpdate(ctx, tx, req.ToAccountID)
		if err != nil {
			return err
		}
		if targetAcct.Status != model.AccountStatusActive {
			return fmt.Errorf("target %w", errs.ErrAccountNotActive)
		}

		txn := &model.Transaction{
			ID:             txnID,
			FromAccountID:  systemAcct.ID,
			ToAccountID:    req.ToAccountID,
			Amount:         req.Amount,
			Status:         model.TxnStatusPending,
			IdempotencyKey: uuid.New(),
		}
		if err := fs.store.CreateTransaction(ctx, tx, txn); err != nil {
			return err
		}

		creditEntry := &model.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     req.ToAccountID,
			TransactionID: txnID,
			Amount:        req.Amount,
			EntryType:     model.EntryTypeCredit,
		}
		if err := fs.store.InsertLedgerEntry(ctx, tx, creditEntry); err != nil {
			return fmt.Errorf("credit ledger: %w", err)
		}

		if err := fs.store.UpdateTransactionStatus(ctx, tx, txnID, model.TxnStatusCompleted); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &model.FundResponse{
		TransactionID: txnID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Status:        model.TxnStatusCompleted,
		Message:       "Funds minted successfully",
	}, nil
}
