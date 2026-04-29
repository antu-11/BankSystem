-- ============================================================================
-- THE VAULT — Banking System Schema
-- PostgreSQL 15+
-- Migration: V1 — Initial Schema
-- ============================================================================

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- ENUM TYPES
-- ============================================================================

CREATE TYPE account_status  AS ENUM ('Active', 'Frozen', 'Closed');
CREATE TYPE txn_status      AS ENUM ('Pending', 'Completed', 'Failed', 'Reversed');
CREATE TYPE entry_type      AS ENUM ('Credit', 'Debit');

-- ============================================================================
-- 1. USERS
-- ============================================================================

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username        VARCHAR(50)  NOT NULL UNIQUE,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    full_name       VARCHAR(100) NOT NULL,
    is_system_user  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON COLUMN users.is_system_user
    IS 'Marks the internal funding / settlement account. Exactly one row should have this set to TRUE.';

-- ============================================================================
-- 2. ACCOUNTS
-- ============================================================================
-- NOTE: Balance is intentionally omitted.
-- The authoritative balance is always derived from the ledger via
--   SUM(CASE WHEN entry_type = 'Credit' THEN amount ELSE -amount END)
-- ============================================================================

CREATE TABLE accounts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    currency    VARCHAR(3)      NOT NULL DEFAULT 'INR',
    status      account_status  NOT NULL DEFAULT 'Active',
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_user_id ON accounts(user_id);

-- ============================================================================
-- 3. TRANSACTIONS
-- ============================================================================

CREATE TABLE transactions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_account_id   UUID           NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    to_account_id     UUID           NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount            NUMERIC(18,4)  NOT NULL CHECK (amount > 0),
    status            txn_status     NOT NULL DEFAULT 'Pending',
    idempotency_key   UUID           NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT now(),

    CONSTRAINT chk_different_accounts CHECK (from_account_id <> to_account_id)
);

COMMENT ON COLUMN transactions.idempotency_key
    IS 'Client-supplied key to guarantee exactly-once processing. Must be globally unique.';

CREATE INDEX idx_txn_from_account  ON transactions(from_account_id);
CREATE INDEX idx_txn_to_account    ON transactions(to_account_id);
CREATE INDEX idx_txn_idempotency   ON transactions(idempotency_key);
CREATE INDEX idx_txn_status        ON transactions(status);

-- ============================================================================
-- 4. LEDGER  (Immutable — append-only)
-- ============================================================================
-- Every completed transaction produces exactly TWO ledger rows:
--   • Debit  on from_account
--   • Credit on to_account
-- This table must NEVER be updated or deleted — enforced by trigger below.
-- ============================================================================

CREATE TABLE ledger (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id      UUID           NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    transaction_id  UUID           NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    amount          NUMERIC(18,4)  NOT NULL CHECK (amount > 0),
    entry_type      entry_type     NOT NULL,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- ── Composite indexes for optimised balance calculation ─────────────────────
CREATE INDEX idx_ledger_account_created
    ON ledger(account_id, created_at);

CREATE INDEX idx_ledger_account_entry_type
    ON ledger(account_id, entry_type);

CREATE INDEX idx_ledger_transaction
    ON ledger(transaction_id);

-- ============================================================================
-- 5. IMMUTABILITY ENFORCEMENT — Ledger
-- ============================================================================
-- Reject any UPDATE or DELETE on the ledger table at the database level.

CREATE OR REPLACE FUNCTION fn_ledger_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION
        'IMMUTABILITY VIOLATION: The ledger table is append-only. '
        'UPDATE and DELETE operations are strictly prohibited.';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_no_update
    BEFORE UPDATE ON ledger
    FOR EACH ROW
    EXECUTE FUNCTION fn_ledger_immutable();

CREATE TRIGGER trg_ledger_no_delete
    BEFORE DELETE ON ledger
    FOR EACH ROW
    EXECUTE FUNCTION fn_ledger_immutable();

-- ============================================================================
-- 6. AUTO-UPDATE TIMESTAMP TRIGGER
-- ============================================================================

CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION fn_set_updated_at();

-- ============================================================================
-- 7. BALANCE CALCULATION — View & Function
-- ============================================================================
-- Since balance is never stored, we derive it from ledger entries.

CREATE VIEW v_account_balances AS
SELECT
    a.id            AS account_id,
    a.user_id,
    a.currency,
    a.status,
    COALESCE(
        SUM(
            CASE
                WHEN l.entry_type = 'Credit' THEN  l.amount
                WHEN l.entry_type = 'Debit'  THEN -l.amount
            END
        ),
        0
    ) AS balance
FROM accounts a
LEFT JOIN ledger l ON l.account_id = a.id
GROUP BY a.id, a.user_id, a.currency, a.status;

-- Scalar function for a single account's balance
CREATE OR REPLACE FUNCTION fn_get_balance(p_account_id UUID)
RETURNS NUMERIC(18,4) AS $$
DECLARE
    v_balance NUMERIC(18,4);
BEGIN
    SELECT COALESCE(
        SUM(
            CASE
                WHEN entry_type = 'Credit' THEN  amount
                WHEN entry_type = 'Debit'  THEN -amount
            END
        ),
        0
    )
    INTO v_balance
    FROM ledger
    WHERE account_id = p_account_id;

    RETURN v_balance;
END;
$$ LANGUAGE plpgsql STABLE;

-- ============================================================================
-- 8. SEED: System User & Funding Account
-- ============================================================================

INSERT INTO users (id, username, email, password_hash, full_name, is_system_user)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'the_vault_system',
    'system@thevault.internal',
    '--- SYSTEM ACCOUNT — NO LOGIN ---',
    'The Vault System',
    TRUE
);

INSERT INTO accounts (id, user_id, currency, status)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    'INR',
    'Active'
);

-- ============================================================================
-- SCHEMA COMPLETE
-- ============================================================================
