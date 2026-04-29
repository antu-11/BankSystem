package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/store"
)

const (
	concurrentWorkers  = 100
	transferAmountEach = "10.0000"
	initialFunding     = "5000.0000"
)

func testEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := testEnv("TEST_DATABASE_URL",
		"postgres://vault_user:vault_pass@localhost:5432/the_vault?sslmode=disable")

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Skipf("Skipping integration test — cannot connect to Postgres: %v", err)
	}

	db.SetMaxOpenConns(concurrentWorkers + 10)
	db.SetMaxIdleConns(concurrentWorkers + 10)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db
}

func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr: testEnv("TEST_REDIS_URL", "localhost:6379"),
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Skipping integration test — cannot connect to Redis: %v", err)
	}
	return rdb
}

func createTestUser(t *testing.T, db *sqlx.DB, username string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO users (id, username, email, password_hash, full_name, is_system_user, is_active)
		 VALUES ($1, $2, $3, 'test-hash', 'Test User', false, true)`,
		userID, username, username+"@test.vault",
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return userID
}

func createTestAccount(t *testing.T, db *sqlx.DB, userID uuid.UUID) uuid.UUID {
	t.Helper()
	acctID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO accounts (id, user_id, currency, status) VALUES ($1, $2, 'INR', 'Active')`,
		acctID, userID,
	)
	if err != nil {
		t.Fatalf("create test account: %v", err)
	}
	return acctID
}

func seedBalance(t *testing.T, db *sqlx.DB, acctID uuid.UUID, amount string) {
	t.Helper()

	txnID := uuid.New()
	systemAcctID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	_, err := db.Exec(
		`INSERT INTO transactions (id, from_account_id, to_account_id, amount, status, idempotency_key)
		 VALUES ($1, $2, $3, $4, 'Completed', $5)`,
		txnID, systemAcctID, acctID, amount, uuid.New(),
	)
	if err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO ledger (id, account_id, transaction_id, amount, entry_type)
		 VALUES ($1, $2, $3, $4, 'Credit')`,
		uuid.New(), acctID, txnID, amount,
	)
	if err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
}

func getBalance(t *testing.T, db *sqlx.DB, acctID uuid.UUID) decimal.Decimal {
	t.Helper()
	var bal decimal.Decimal
	err := db.Get(&bal,
		`SELECT COALESCE(SUM(CASE WHEN entry_type='Credit' THEN amount ELSE -amount END), 0)
		 FROM ledger WHERE account_id = $1`, acctID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	return bal
}

func cleanupTestData(db *sqlx.DB, userIDs ...uuid.UUID) {
	for _, uid := range userIDs {
		db.Exec(`DELETE FROM ledger WHERE account_id IN (SELECT id FROM accounts WHERE user_id = $1)`, uid)
		db.Exec(`DELETE FROM transactions WHERE from_account_id IN (SELECT id FROM accounts WHERE user_id = $1)
		          OR to_account_id IN (SELECT id FROM accounts WHERE user_id = $1)`, uid)
		db.Exec(`DELETE FROM accounts WHERE user_id = $1`, uid)
		db.Exec(`DELETE FROM users WHERE id = $1`, uid)
	}
}

// TestConcurrentTransfers_100Goroutines is a chaos-engineering integration test
// that proves SELECT FOR UPDATE row-level locking prevents race conditions.
//
// 100 goroutines concurrently transfer from a single account; the final balance
// must match the exact mathematical expectation.
//
// Requires a live PostgreSQL + Redis.
// Run with: go test -v -tags=integration -count=1 ./internal/service/
func TestConcurrentTransfers_100Goroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()
	rdb := setupTestRedis(t)
	defer rdb.Close()

	ctx := context.Background()

	senderUser := createTestUser(t, db, fmt.Sprintf("sender_%s", uuid.New().String()[:8]))
	receiverUser := createTestUser(t, db, fmt.Sprintf("receiver_%s", uuid.New().String()[:8]))
	defer cleanupTestData(db, senderUser, receiverUser)

	senderAcct := createTestAccount(t, db, senderUser)
	receiverAcct := createTestAccount(t, db, receiverUser)

	seedBalance(t, db, senderAcct, initialFunding)

	initialBal := getBalance(t, db, senderAcct)
	t.Logf("Sender initial balance: %s", initialBal.StringFixed(4))

	dataStore := store.New(db)
	transferSvc := NewTransferService(dataStore, rdb)

	var wg sync.WaitGroup
	var successCount int64
	var failCount int64
	amount := decimal.RequireFromString(transferAmountEach)

	start := time.Now()

	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			req := model.TransferRequest{
				FromAccountID:  senderAcct,
				ToAccountID:    receiverAcct,
				Amount:         amount,
				IdempotencyKey: uuid.New(),
			}

			_, err := transferSvc.TransferFunds(ctx, req)
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				t.Logf("   Worker %3d: %v", workerID, err)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	finalSenderBal := getBalance(t, db, senderAcct)
	finalReceiverBal := getBalance(t, db, receiverAcct)

	succeeded := atomic.LoadInt64(&successCount)
	failed := atomic.LoadInt64(&failCount)

	t.Logf("")
	t.Logf("==========================================================")
	t.Logf("  CHAOS TEST RESULTS")
	t.Logf("==========================================================")
	t.Logf("  Concurrent goroutines:  %d", concurrentWorkers)
	t.Logf("  Transfer amount each:   %s", transferAmountEach)
	t.Logf("  Duration:               %s", elapsed.Round(time.Millisecond))
	t.Logf("  ----------------------------------------------------------")
	t.Logf("  Succeeded:              %d", succeeded)
	t.Logf("  Failed (expected):      %d", failed)
	t.Logf("  ----------------------------------------------------------")
	t.Logf("  Sender initial:         %s", initialBal.StringFixed(4))
	t.Logf("  Sender final:           %s", finalSenderBal.StringFixed(4))
	t.Logf("  Receiver final:         %s", finalReceiverBal.StringFixed(4))
	t.Logf("==========================================================")

	expectedSenderBal := initialBal.Sub(amount.Mul(decimal.NewFromInt(succeeded)))
	expectedReceiverBal := amount.Mul(decimal.NewFromInt(succeeded))

	if !finalSenderBal.Equal(expectedSenderBal) {
		t.Errorf("FAIL: RACE CONDITION — Sender balance mismatch: expected %s, got %s",
			expectedSenderBal.StringFixed(4), finalSenderBal.StringFixed(4))
	} else {
		t.Logf("PASS: Sender balance matches expectation: %s", expectedSenderBal.StringFixed(4))
	}

	if !finalReceiverBal.Equal(expectedReceiverBal) {
		t.Errorf("FAIL: RACE CONDITION — Receiver balance mismatch: expected %s, got %s",
			expectedReceiverBal.StringFixed(4), finalReceiverBal.StringFixed(4))
	} else {
		t.Logf("PASS: Receiver balance matches expectation: %s", expectedReceiverBal.StringFixed(4))
	}

	totalMoney := finalSenderBal.Add(finalReceiverBal)
	if !totalMoney.Equal(initialBal) {
		t.Errorf("FAIL: MONEY CONSERVATION VIOLATED — initial: %s, final total: %s",
			initialBal.StringFixed(4), totalMoney.StringFixed(4))
	} else {
		t.Logf("PASS: Money conservation verified: %s = %s", totalMoney.StringFixed(4), initialBal.StringFixed(4))
	}

	if finalSenderBal.IsNegative() {
		t.Errorf("FAIL: OVERDRAFT — Sender balance went negative: %s",
			finalSenderBal.StringFixed(4))
	} else {
		t.Logf("PASS: No overdraft detected")
	}

	if succeeded+failed != int64(concurrentWorkers) {
		t.Errorf("FAIL: Worker count mismatch: %d + %d != %d",
			succeeded, failed, concurrentWorkers)
	}

	_ = sql.ErrNoRows
}
