<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis&logoColor=white" alt="Redis" />
  <img src="https://img.shields.io/badge/Next.js-15-000000?style=for-the-badge&logo=next.js&logoColor=white" alt="Next.js" />
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker" />
</p>

<h1 align="center">🏦 OmniLedger</h1>

<p align="center">
  <strong>A high-concurrency, distributed banking ledger built for correctness under pressure.</strong><br />
  Implements institutional-grade double-entry bookkeeping, row-level locking, and idempotent transaction processing — battle-tested with 100 concurrent goroutines.
</p>

<p align="center">
  <a href="#-system-architecture">Architecture</a> •
  <a href="#-local-development-setup">Setup</a> •
  <a href="#-api-documentation">API</a> •
  <a href="#-running-the-chaos-test">Chaos Test</a>
</p>

---

![Dashboard Screenshot](docs/screenshots/dashboard.png)

---

## 🧠 System Architecture

> **Why build a ledger this way?**
>
> Financial systems don't get second chances. A single lost update or phantom read under concurrency means real money vanishes — or is created from nothing. Every design decision in OmniLedger optimises for **correctness first**, performance second.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Next.js Frontend (:3000)                     │
│                   App Router · HttpOnly Cookie Auth                 │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP / JSON
┌──────────────────────────────▼──────────────────────────────────────┐
│                         Go API Server (:8080)                       │
│  ┌──────────┐  ┌──────────────┐  ┌───────────┐  ┌───────────────┐  │
│  │  Auth    │  │  Transfers   │  │  Funding  │  │  Accounts     │  │
│  │  Handler │  │  Handler     │  │  Handler  │  │  Handler      │  │
│  └────┬─────┘  └──────┬───────┘  └─────┬─────┘  └───────┬───────┘  │
│       │               │               │                │           │
│  ┌────▼───────────────▼───────────────▼────────────────▼────────┐  │
│  │                      Service Layer                           │  │
│  │  AuthService · TransferService · FundingService              │  │
│  └──────────────────────────┬───────────────────────────────────┘  │
│                             │                                      │
│  ┌──────────────────────────▼───────────────────────────────────┐  │
│  │                       Store (DAL)                            │  │
│  │  WithTx · GetAccountForUpdate · GetBalance · InsertLedger   │  │
│  └──────────┬───────────────────────────────────┬───────────────┘  │
│             │                                   │                  │
└─────────────┼───────────────────────────────────┼──────────────────┘
              │                                   │
   ┌──────────▼──────────┐             ┌──────────▼──────────┐
   │   PostgreSQL 16     │             │     Redis 7         │
   │  ─────────────────  │             │  ─────────────────  │
   │  users              │             │  Idempotency keys   │
   │  accounts           │             │  JWT blacklist      │
   │  transactions       │             │  Rate-limit state   │
   │  ledger (immutable) │             │  (Sorted Sets)      │
   └─────────────────────┘             └─────────────────────┘
```

---

### 📒 Immutable Ledger & Double-Entry Bookkeeping

Traditional systems store a mutable `balance` column and increment/decrement it on every transaction. This is fundamentally unsafe — a single lost `UPDATE` under concurrency silently corrupts account state, and there is no audit trail to detect or recover from the inconsistency.

**OmniLedger takes a different approach.** The `accounts` table has no balance column. Instead, every completed transfer atomically appends exactly **two immutable rows** to the `ledger` table — a `Debit` entry on the sender and a `Credit` entry on the receiver:

```sql
-- Balance is ALWAYS derived dynamically — never stored.
SELECT COALESCE(
    SUM(CASE
        WHEN entry_type = 'Credit' THEN  amount
        WHEN entry_type = 'Debit'  THEN -amount
    END),
    0
) FROM ledger WHERE account_id = $1;
```

The ledger is **append-only by database trigger** — `UPDATE` and `DELETE` operations on the `ledger` table raise an `IMMUTABILITY VIOLATION` exception at the PostgreSQL level, enforced via `trg_ledger_no_update` and `trg_ledger_no_delete`.

**Why this matters:**
- ✅ Complete, tamper-evident audit trail from genesis
- ✅ Balance can be recomputed at any point in time
- ✅ No mutable state to corrupt under concurrent writes
- ✅ Regulatory compliance by design (SOX, PCI-DSS audit readiness)

---

### 🔒 Concurrency Control

The core challenge: what happens when 100 users simultaneously transfer from the same account?

Without protection, two transactions could both read the same balance, both conclude there are sufficient funds, and both succeed — creating money from nothing. This is the classic **double-spending problem**.

OmniLedger solves this with **PostgreSQL row-level exclusive locks** via `SELECT ... FOR UPDATE`:

```go
// 1. Acquire exclusive locks in deterministic UUID order (prevents deadlocks)
firstLock, secondLock := req.FromAccountID, req.ToAccountID
if req.FromAccountID.String() > req.ToAccountID.String() {
    firstLock, secondLock = req.ToAccountID, req.FromAccountID
}

// 2. SELECT FOR UPDATE — blocks concurrent transactions on these rows
_, err := store.GetAccountForUpdate(ctx, tx, firstLock)
_, err = store.GetAccountForUpdate(ctx, tx, secondLock)

// 3. Now safe: read balance, validate, write ledger entries — all serialized
```

**Key details:**
- Locks are acquired in **deterministic UUID order** to prevent deadlocks when transfers flow in opposite directions (A→B and B→A simultaneously)
- Transactions run at `sql.LevelSerializable` isolation
- The lock scope is a **single row**, not a table — uninvolved accounts process in full parallel

---

### 🔁 Idempotency & Resilience

Network retries are inevitable. A mobile client submits a ₹500 transfer, the server processes it, but the response is lost. The client retries. Without idempotency, the customer is charged ₹1,000.

OmniLedger prevents this with a **Redis-backed idempotency gate**:

```go
// Redis SET NX — atomic "set if not exists" with 24h TTL
wasSet, err := ts.rdb.SetNX(ctx, "vault:idempotency:"+key, "processing", 24*time.Hour).Result()
if !wasSet {
    return nil, ErrDuplicateTransaction  // 409 Conflict
}
```

**Resilience guarantees:**
- ✅ If the transfer **succeeds**, the key persists — replays return `409 Conflict`
- ✅ If the transfer **fails**, the key is automatically cleaned up via `defer` — the client can safely retry with the same idempotency key
- ✅ Keys expire after 24 hours, preventing unbounded memory growth

---

### 🛡️ Security

| Layer | Mechanism | Implementation |
|---|---|---|
| **Authentication** | JWT via `HttpOnly` + `Secure` + `SameSite=Strict` cookies | XSS-proof — JavaScript cannot access the token |
| **Token Revocation** | Redis-backed JWT blacklist by `jti` claim | Immediate logout; revoked tokens fail the middleware pipeline |
| **Rate Limiting** | Redis Sorted Set sliding-window (5 req/min per IP) | Prevents brute-force on `/auth/login` and `/auth/register` |
| **Password Storage** | `bcrypt` with cost factor 12 | Adaptive hashing; resistant to GPU/ASIC attacks |
| **Account Freeze** | Middleware-level account status gate | Frozen accounts are blocked at the HTTP layer, not just the service layer |
| **System Funding** | DB-verified `is_system_user` flag (never trusts JWT claims) | Privilege escalation via JWT manipulation is impossible |

---

## 📐 Project Structure

```
.
├── cmd/api/
│   └── main.go                 # Bootstrap: config → logger → DB → Redis → routes → server
├── internal/
│   ├── auth/                   # JWT token manager + auth middleware + Redis blacklist
│   ├── config/                 # Environment loader (.env / os.Getenv)
│   ├── email/                  # Async email worker pool (SMTP)
│   ├── errs/                   # Domain-specific sentinel errors
│   ├── handler/                # HTTP handlers (auth, account, transfer, funding)
│   ├── logger/                 # Structured slog configuration
│   ├── middleware/             # Rate limiter (Redis sliding window)
│   ├── model/                  # Domain models + request/response DTOs
│   ├── service/                # Business logic (auth, transfer, funding)
│   └── store/                  # Data-access layer (sqlx, PostgreSQL)
├── sql/
│   └── V1__init_schema.sql     # Full DDL: tables, triggers, views, functions, seeds
├── frontend/                   # Next.js 15 (App Router) dashboard
├── docker-compose.yml          # PostgreSQL 16 + Redis 7 + Go API + Next.js
├── Dockerfile                  # Multi-stage build → ~15 MB scratch image
└── .env.example                # Template for all required environment variables
```

---

## 🚀 Local Development Setup

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [Go 1.22+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/) (for the frontend)

### 1️⃣ Clone & Configure

```bash
git clone https://github.com/akdandapat/OmniLedger.git
cd OmniLedger
cp .env.example .env
```

### 2️⃣ Start Infrastructure

```bash
# Launch PostgreSQL 16 + Redis 7 (schema auto-migrates via docker-entrypoint)
docker-compose up -d
```

The `V1__init_schema.sql` migration is mounted into the Postgres container's init directory — the database schema, triggers, views, and seed data are applied automatically on first boot.

### 3️⃣ Verify Services

```bash
docker-compose ps                          # Both should be "healthy"
docker exec vault-postgres pg_isready      # PostgreSQL
docker exec vault-redis redis-cli ping     # Redis → PONG
```

### 4️⃣ Start the Go API

```bash
go run ./cmd/api
# ✅ OmniLedger API started on :8080
```

### 5️⃣ Start the Frontend (Optional)

```bash
cd frontend
npm install
npm run dev
# ✅ Next.js dashboard on :3000
```

### 6️⃣ Quick Smoke Test

```bash
curl -s http://localhost:8080/api/v1/health | jq
# → { "status": "ok", "service": "omniledger" }
```

---

## 📡 API Documentation

All endpoints are prefixed with `/api/v1`. Authentication is handled via `HttpOnly` cookies — include `credentials: 'include'` in frontend fetch calls.

### Auth

| Method | Endpoint | Auth | Rate Limited | Description |
|--------|----------|------|--------------|-------------|
| `POST` | `/auth/register` | ❌ | ✅ 5/min | Create a new user + default INR account |
| `POST` | `/auth/login` | ❌ | ✅ 5/min | Authenticate and receive a session cookie |
| `POST` | `/auth/logout` | 🔐 | ❌ | Blacklist the current JWT and clear the cookie |

### Accounts

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/accounts` | 🔐 | List all accounts for the authenticated user |
| `GET` | `/accounts/{id}/balance` | 🔐 | Dynamically computed balance (∑ Credits − ∑ Debits) |
| `GET` | `/accounts/{id}/history?page=1&per_page=20` | 🔐 | Paginated transaction history (sent + received) |

### Transfers

| Method | Endpoint | Auth | Rate Limited | Description |
|--------|----------|------|--------------|-------------|
| `POST` | `/transfers` | 🔐 | ✅ 5/min | Execute an atomic fund transfer between accounts |

<details>
<summary><strong>📦 Transfer Request Body</strong></summary>

```json
{
  "from_account_id": "uuid",
  "to_account_id": "uuid",
  "amount": "250.0000",
  "idempotency_key": "uuid"
}
```
</details>

<details>
<summary><strong>📦 Transfer Response</strong></summary>

```json
{
  "transaction_id": "uuid",
  "status": "Completed",
  "message": "Transfer completed successfully"
}
```
</details>

### System Operations

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/transactions/system/fund` | 🔐 System User | Mint funds into a target account (DB-verified privilege) |

### Health

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/health` | ❌ | Liveness probe |

---

## 🔥 Running the Chaos Test

The integration test suite proves correctness under extreme concurrency. **100 goroutines simultaneously transfer from a single account**, and the test mathematically verifies that:

- ✅ **No double-spending** — final balance matches the exact mathematical expectation
- ✅ **No overdraft** — sender balance never goes negative
- ✅ **Money conservation** — total money in the system is constant (nothing created or destroyed)
- ✅ **No deadlocks** — deterministic lock ordering prevents circular waits

### Run the Test

```bash
# Requires running PostgreSQL + Redis (docker-compose up -d)
go test -v -count=1 ./internal/service/
```

### Expected Output

```
══════════════════════════════════════════════════════════════════════
  CHAOS TEST RESULTS
══════════════════════════════════════════════════════════════════════
  Concurrent goroutines:  100
  Transfer amount each:   10.0000
  Duration:               897ms
  ────────────────────────────────────────────────────────────────────
  Succeeded:              6
  Failed (expected):      94
  ────────────────────────────────────────────────────────────────────
  Sender initial:         5000.0000
  Sender final:           4940.0000
  Receiver final:         60.0000
══════════════════════════════════════════════════════════════════════
  ✅ Sender balance matches expectation: 4940.0000
  ✅ Receiver balance matches expectation: 60.0000
  ✅ Money conservation verified: 5000.0000 = 5000.0000
  ✅ No overdraft: sender balance is non-negative
```

> **Wait — why did 94 transfers fail?**
>
> This is a feature, not a bug. The system uses `Serializable` isolation with row-level locking, so PostgreSQL recognized that 100 goroutines were mutating the exact same balance simultaneously. Instead of allowing a race condition (which creates money from thin air), the database acted as a bouncer — it cleanly processed 6 transactions in serial order and threw a `pq: could not serialize access` error for the remaining 94 collisions. The math is exact: 6 × ₹10 = ₹60 moved, ₹4940 remains. Money conservation holds perfectly.
>
> In a production system, the client retries with the same idempotency key and the request succeeds on the next attempt. The point of the test is to prove that **no amount of concurrency can corrupt the ledger**.

---

## 🐳 Production Deployment

The multi-stage Dockerfile produces a **~15 MB scratch image** with zero OS surface area:

```bash
docker-compose up -d --build
```

| Container | Image | Port | Purpose |
|-----------|-------|------|---------|
| `vault-api` | Custom (scratch) | `:8080` | Go API server |
| `vault-postgres` | `postgres:16-alpine` | `:5432` | Primary data store |
| `vault-redis` | `redis:7-alpine` | `:6379` | Idempotency, blacklist, rate limiting |
| `vault-frontend` | Custom (Next.js) | `:3000` | Dashboard UI (optional profile) |

---

## 📄 License

This project is provided for educational and portfolio demonstration purposes.

---

<p align="center">
  <sub>Built with correctness as a feature, not an afterthought.</sub>
</p>
