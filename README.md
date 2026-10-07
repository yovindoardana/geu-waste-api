# GEU Waste API

Production-ready backend API service for the **GEU Waste Management System**, built with Go (Golang) and PostgreSQL. The system manages household waste registration, multi-stage pickup lifecycles, automated tariff calculation and invoice generation, payment proof upload verification, and aggregation reporting.

> 🌐 **Language Options**: [English (README.md)](README.md) | [Bahasa Indonesia (README.id.md)](README.id.md)

---

## 🏗️ Architecture & Technical Stack

The service adheres to Clean 3-Layer Architecture principles with strict separation of concerns:

- **Transport / Handlers (`internal/handler/`)**: Gin HTTP routing, strict JSON validation (`DisallowUnknownFields`, 1 MiB body limit for JSON, 6 MiB for multipart file uploads), multipart file parsing, and standardized JSON envelopes (`response.SuccessResponse`, `response.ListSuccessResponse`, `response.ErrorResponse`).
- **Domain & Services (`internal/domain/`, `internal/service/`)**: Pure business logic, state machine validation, business rule enforcement (BR01–BR05), and transactional workflows.
- **Persistence / Repositories (`internal/repository/postgres/`)**: High-performance PostgreSQL operations using `jackc/pgx/v5` connection pools (`pgxpool`), parameterized queries, transactional execution with `SELECT ... FOR UPDATE` row locking.
- **Monetary Arithmetic**: Exact fixed-point arithmetic using `github.com/shopspring/decimal`. Monetary values are stored as `NUMERIC(12,2)` in PostgreSQL and serialized as two-decimal JSON strings (e.g. `"50000.00"`, `"100000.00"`).
- **Runtime & Deployment**: Multi-stage `Dockerfile` producing a minimal, secure runner container, orchestrated with `docker-compose.yml` and graceful shutdown (`SIGTERM`/`SIGINT`).

```
geu-waste-api/
├── cmd/
│   ├── api/          # Main HTTP server entrypoint
│   ├── migrate/      # Standalone schema migration CLI tool
│   └── seed/         # Deterministic seed runner CLI tool
├── internal/
│   ├── config/       # Environment & configuration loader
│   ├── domain/       # Core business entities, enums, DTOs & custom serializers
│   ├── handler/      # HTTP handlers, middleware, and routers
│   ├── repository/   # Database access layer (PostgreSQL pgxpool)
│   ├── response/     # Standardized JSON API response envelopes
│   ├── seed/         # Seed execution logic & fixtures
│   ├── service/      # Domain business logic & transaction orchestration
│   └── validator/    # Strict JSON decoding & image upload validation
├── migrations/       # Versioned SQL migration files (.up.sql / .down.sql)
├── postman/          # Automated Postman collection and environment
├── seeds/assets/     # Seed binary assets (sample payment proofs)
├── uploads/          # Runtime uploaded file storage
├── Dockerfile        # Production multi-stage container build
├── docker-compose.yml# Multi-container local stack (PostgreSQL + API)
├── Makefile          # Developer tooling and automation tasks
├── README.md         # English Documentation
└── README.id.md      # Indonesian Documentation
```

---

## 📋 Business Rules & Domain Logic

| ID | Rule Name | Description |
|---|---|---|
| **BR01** | **Pending Payment Block** | A household with any pending unpaid payment for a completed pickup cannot create a new pickup (`POST /api/pickups`). Returns `409 Conflict` (`HOUSEHOLD_PENDING_PAYMENT`). Protected by strict hierarchical lock ordering (`households` $\rightarrow$ `waste_pickups`). |
| **BR02** | **Pending State for Scheduling** | Only pickups with status `pending` can be scheduled (`PUT /api/pickups/:id/schedule`). Other states return `409 Conflict` (`INVALID_STATE_TRANSITION`). |
| **BR03** | **Electronic Safety Check** | Electronic waste pickups (`type: electronic`) can only be scheduled if effective `safety_check` is `true`. (Creating an electronic pickup allows `safety_check: false`; scheduling requires `safety_check: true`). Returns `409 Conflict` (`SAFETY_CHECK_REQUIRED`) if false. |
| **BR04** | **Atomic Completion & Invoicing** | Completing a pickup (`PUT /api/pickups/:id/complete`) atomically marks the pickup as `completed` and creates a pending payment invoice in a single database transaction. Standard waste (`organic`, `plastic`, `paper`) = **Rp 50,000.00**; Electronic waste (`electronic`) = **Rp 100,000.00**. |
| **BR05** | **Proof Upload for Confirmation** | Confirming a payment (`PUT /api/payments/:id/confirm`) strictly requires a valid local proof image (JPEG/PNG, $\le 5\text{MB}$, $\le 10,000\text{px}$). Promoted from staging to final path before DB update, and preserved upon `ErrCommitUncertain`. |
| **D01** | **Idempotent Invoice Endpoint** | `POST /api/payments` ensures an invoice exists for a completed pickup (returns `201 Created` if newly generated, or `200 OK` if already exists). |
| **D08** | **Revenue Aggregation** | Payment summary report calculates `total_revenue` by strictly summing amounts from payments with `status = 'paid'`. |

---

## ⚙️ Environment Variables

Copy `.env.example` to `.env` to customize your environment:

```bash
cp .env.example .env
```

| Variable | Default Value | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | Port on which the HTTP server listens |
| `SERVER_HOST` | `0.0.0.0` | Host IP address binding |
| `DB_HOST` | `localhost` | PostgreSQL host (`postgres` in Docker Compose) |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL database username |
| `DB_PASSWORD` | `postgres` | PostgreSQL database password |
| `DB_NAME` | `geu_waste` | PostgreSQL application database name |
| `TEST_DB_NAME` | `geu_waste_test` | Dedicated isolated database for integration test suite |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL connection mode (`disable`, `require`) |
| `UPLOAD_DIR` | `./uploads/payment-proofs` | Directory where uploaded payment proofs are stored |
| `MAX_UPLOAD_SIZE_MB` | `5` | Maximum upload file size in megabytes |

---

## 🚀 Quick Start (Docker Compose)

To start both the PostgreSQL database and the API service in containers:

```bash
# Build and run container stack
docker compose up -d --build

# View container logs
docker compose logs -f app

# Stop containers
docker compose down
```

The API will be accessible at `http://localhost:8080`.

---

## 💻 Local Development Setup

### Prerequisites
- **Go**: 1.24+ (tested on Go 1.24 and 1.27)
- **PostgreSQL**: 16+ or Docker
- **Make**: Standard GNU make

### Step-by-Step Setup

1. **Start PostgreSQL Database** (via Docker or local service):
   ```bash
   docker-compose up -d postgres
   ```

2. **Run Database Migrations**:
   ```bash
   make migrate-up
   # Or using go run:
   go run ./cmd/migrate up
   ```

3. **Seed Initial Deterministic Test Data**:
   ```bash
   make seed
   # Or using go run:
   go run ./cmd/seed
   ```

4. **Run the API Server**:
   ```bash
   make run
   # Or using go run:
   go run ./cmd/api
   ```

5. **Run the Automated Test Suite**:
   ```bash
   make test
   ```

6. **Build Production Binaries**:
   ```bash
   make build
   ```

---

## 📚 API Endpoints Catalog (14 Business Endpoints + Health + Proof Serving)

All responses follow a standard envelope:

**Success Single**:
```json
{
  "success": true,
  "message": "resource created successfully",
  "data": { ... }
}
```

**Success List**:
```json
{
  "success": true,
  "message": "list retrieved successfully",
  "data": [ ... ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 25,
    "total_pages": 2
  }
}
```

**Error**:
```json
{
  "success": false,
  "code": "VALIDATION_ERROR",
  "message": "validation failed",
  "errors": [
    { "field": "owner_name", "message": "owner_name is required" }
  ]
}
```

---

### 1. Health Check
- `GET /health` - Returns `{"success":true,"message":"service is healthy"}`.

---

### 2. Households (`/api/households`)

- `POST /api/households` - Register a new household.
  ```json
  // Request
  {
    "owner_name": "Budi Santoso",
    "address": "Jl. Merdeka No. 45, Bandung"
  }
  ```
- `GET /api/households` - List households (query parameters: `page`, `limit`).
- `GET /api/households/:id` - Retrieve household by UUID.
- `DELETE /api/households/:id` - Delete household (fails with `409 Conflict` if dependent pickups/payments exist).

---

### 3. Waste Pickups (`/api/pickups`)

- `POST /api/pickups` - Request a waste pickup.
  - Allowed `type`: `organic`, `plastic`, `paper`, `electronic`.
  - For non-electronic: `safety_check` must be omitted.
  - For `electronic`: `safety_check` is required (`false` or `true`).
  ```json
  // Request Non-Electronic
  {
    "household_id": "11111111-1111-4111-8111-111111111111",
    "type": "organic"
  }

  // Request Electronic
  {
    "household_id": "11111111-1111-4111-8111-111111111111",
    "type": "electronic",
    "safety_check": false
  }
  ```
- `GET /api/pickups` - List pickups (query parameters: `page`, `limit`, `status`, `household_id`).
- `PUT /api/pickups/:id/schedule` - Schedule a pending pickup.
  - For `electronic`: `safety_check` can be updated to `true`.
  ```json
  // Request
  {
    "pickup_date": "2026-10-15T09:00:00+07:00",
    "safety_check": true
  }
  ```
- `PUT /api/pickups/:id/cancel` - Cancel a pending or scheduled pickup.
- `PUT /api/pickups/:id/complete` - Mark pickup as completed and atomically generate invoice.
  ```json
  // Response 200 OK
  {
    "success": true,
    "message": "pickup completed successfully",
    "data": {
      "pickup": {
        "id": "22222222-2222-4222-8222-222222222222",
        "household_id": "11111111-1111-4111-8111-111111111111",
        "type": "organic",
        "status": "completed",
        "pickup_date": "2026-10-08T02:00:00Z",
        "safety_check": null,
        "created_at": "2026-10-07T08:00:00Z",
        "updated_at": "2026-10-08T03:00:00Z"
      },
      "payment": {
        "id": "33333333-3333-4333-8333-333333333333",
        "household_id": "11111111-1111-4111-8111-111111111111",
        "waste_id": "22222222-2222-4222-8222-222222222222",
        "amount": "50000.00",
        "payment_date": null,
        "status": "pending",
        "proof_file_url": null,
        "created_at": "2026-10-08T03:00:00Z",
        "updated_at": "2026-10-08T03:00:00Z"
      }
    }
  }
  ```

---

### 4. Payments & Proofs (`/api/payments`)

- `POST /api/payments` - Ensure payment invoice exists for a completed pickup (returns `201 Created` if newly created, or `200 OK` if already exists).
  ```json
  // Request
  {
    "household_id": "11111111-1111-4111-8111-111111111111",
    "waste_id": "22222222-2222-4222-8222-222222222222",
    "amount": "50000.00"
  }
  ```
- `GET /api/payments` - List payments (query parameters: `page`, `limit`, `status`, `household_id`, `start_date`, `end_date`).
- `PUT /api/payments/:id/confirm` - Confirm payment by uploading proof of payment (`multipart/form-data` with field `proof`). Validates file size ($\le 5\text{MB}$), MIME type (`image/jpeg`, `image/png`), and image dimensions ($\le 10,000\text{px}$, $\le 20\text{MP}$).
- `GET /uploads/payment-proofs/:filename` - Static route to view uploaded payment proof image.

---

### 5. Reports & Analytics (`/api/reports`)

- `GET /api/reports/waste-summary` - Returns 16 canonical buckets for all waste types and statuses in strict fixed order:
  - `organic` (`pending`, `scheduled`, `completed`, `canceled`)
  - `plastic` (`pending`, `scheduled`, `completed`, `canceled`)
  - `paper` (`pending`, `scheduled`, `completed`, `canceled`)
  - `electronic` (`pending`, `scheduled`, `completed`, `canceled`)
- `GET /api/reports/payment-summary` - Returns breakdown of payments across all 3 statuses (`pending`, `paid`, `failed`) and `total_revenue` (summing strictly `paid` payments).

---

## 🧪 Postman & Newman Verification

Comprehensive Postman assets are located in the `postman/` directory:

- **Collection**: [postman/geu-waste-api.postman_collection.json](file:///Users/toothless/Documents/Coding/project/geu-waste-api/postman/geu-waste-api.postman_collection.json)
- **Environment**: [postman/local.postman_environment.json](file:///Users/toothless/Documents/Coding/project/geu-waste-api/postman/local.postman_environment.json)
- **Test Asset**: [postman/assets/sample-proof.png](file:///Users/toothless/Documents/Coding/project/geu-waste-api/postman/assets/sample-proof.png)

### Running with Newman:
```bash
npx newman run postman/geu-waste-api.postman_collection.json \
  -e postman/local.postman_environment.json
```

---

## 🛠️ Makefile Commands

| Command | Action |
|---|---|
| `make build` | Build all Go binaries (`bin/api`, `bin/migrate`, `bin/seed`) |
| `make run` | Run API server locally with `go run ./cmd/api` |
| `make test` | Run all unit tests with race detector |
| `make migrate-up` | Run all pending database migrations |
| `make migrate-down` | Rollback the latest migration batch |
| `make seed` | Execute deterministic database seed runner |
| `make docker-up` | Build and start containers in background |
| `make docker-down` | Stop and remove container stack |
| `make clean` | Clean build artifacts and binary outputs |

---

## 📄 License
This project is proprietary and intended for the GEU Waste Management System.
