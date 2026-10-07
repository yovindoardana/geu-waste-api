# GEU Waste API

Production-ready backend API service for the **GEU Waste Management System**, built with Go (Golang) and PostgreSQL. The system manages household waste registration, multi-stage pickup lifecycles, automated tariff calculation and invoice generation, payment proof upload verification, and aggregation reporting.

---

## 🏗️ Architecture & Technical Stack

The service adheres to Clean 3-Layer Architecture principles with strict separation of concerns:

- **Transport / Handlers (`internal/handler/`)**: Gin HTTP routing, strict JSON validation (`DisallowUnknownFields`, 1 MiB body limits), multipart file parsing, and standardized JSON envelopes (`response.SuccessResponse`, `response.ListSuccessResponse`, `response.ErrorResponse`).
- **Domain & Services (`internal/domain/`, `internal/service/`)**: Pure business logic, state machine validation, business rule enforcement (BR01, BR02, BR03), and transactional workflows.
- **Persistence / Repositories (`internal/repository/postgres/`)**: High-performance PostgreSQL operations using `jackc/pgx/v5` connection pools (`pgxpool`), parameterized queries, transactional execution with `SELECT ... FOR UPDATE` row locking.
- **Monetary Arithmetic**: Exact fixed-point arithmetic using `github.com/shopspring/decimal`. Monetary values are stored as `NUMERIC(12,2)` in PostgreSQL and serialized as two-decimal JSON strings (e.g. `"50000.00"`, `"100000.00"`).
- **Runtime & Deployment**: Multi-stage `Dockerfile` producing a minimal, secure scratch/distroless runner container, orchestrated with `docker-compose.yml` and graceful shutdown (`SIGTERM`/`SIGINT`).

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
└── README.md
```

---

## 📋 Business Rules & Domain Logic

| ID | Rule Name | Description |
|---|---|---|
| **BR01** | **Pending Payment Lock** | A household cannot request a new pickup (`POST /api/pickups`) if they have any pending unpaid payment for a previously completed pickup. |
| **BR02** | **E-Waste Safety Confirmation** | Pickups with waste type `electronic` strictly require explicit safety confirmation (`"e_waste_safety_confirmed": true`). Returns `422 Unprocessable Entity` if omitted or false. |
| **BR03** | **Pickup State Machine** | Strict state transition workflow: `pending` $\rightarrow$ `scheduled` $\rightarrow$ `completed` or `canceled`. Canceled pickups cannot be scheduled or completed. Completed pickups cannot be modified. |
| **Tariffs** | **Waste Categorization** | Standard waste (`organic`, `plastic`, `paper`) is billed at **Rp 50,000.00**. Electronic waste (`electronic`) is billed at **Rp 100,000.00**. |
| **D01** | **Invoice Generation** | Completing a pickup (`PUT /api/pickups/:id/complete`) atomically marks the pickup as `completed` and creates a pending payment invoice with the corresponding tariff. `POST /api/payments` idempotently returns existing invoice if already generated. |
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
| `DB_USER` | `geu_user` | PostgreSQL database username |
| `DB_PASSWORD` | `geu_password` | PostgreSQL database password |
| `DB_NAME` | `geu_waste_db` | PostgreSQL database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL connection mode (`disable`, `require`) |
| `UPLOAD_DIR` | `./uploads/payment-proofs` | Directory where uploaded payment proofs are stored |
| `MAX_UPLOAD_SIZE_MB` | `5` | Maximum upload file size in megabytes |
| `ALLOWED_FILE_TYPES` | `image/jpeg,image/png` | Comma-separated allowed MIME content types |

---

## 🚀 Quick Start (Docker Compose)

To start both the PostgreSQL database and the API service in containers:

```bash
# Build and run container stack
docker-compose up -d --build

# View container logs
docker-compose logs -f app

# Stop containers
docker-compose down
```

The API will be accessible at `http://localhost:8080`.

---

## 💻 Local Development Setup

### Prerequisites
- **Go**: 1.24+ (tested on Go 1.24 and 1.27)
- **PostgreSQL**: 16+
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

## 📚 API Endpoints Catalog

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
    "limit": 10,
    "total": 25,
    "total_pages": 3
  }
}
```

**Error**:
```json
{
  "success": false,
  "code": "VALIDATION_ERROR",
  "message": "invalid request body",
  "errors": [
    { "field": "owner_name", "message": "owner_name is required" }
  ]
}
```

---

### 1. Health Check
- `GET /healthz` - Returns service health status.

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
- `DELETE /api/households/:id` - Delete household (fails with `409 Conflict` if dependent pickups exist).

---

### 3. Waste Pickups (`/api/pickups`)

- `POST /api/pickups` - Request a waste pickup.
  - Allowed `waste_type`: `organic`, `plastic`, `paper`, `electronic`.
  - For `electronic`, `"e_waste_safety_confirmed": true` is required.
  ```json
  // Request
  {
    "household_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "waste_type": "electronic",
    "notes": "Old computer monitor",
    "e_waste_safety_confirmed": true
  }
  ```
- `GET /api/pickups` - List pickups (query parameters: `page`, `limit`, `status`, `waste_type`, `household_id`).
- `PUT /api/pickups/:id/schedule` - Schedule a pickup.
  ```json
  // Request
  {
    "scheduled_date": "2026-10-15T09:00:00Z"
  }
  ```
- `PUT /api/pickups/:id/cancel` - Cancel a pickup (allowed only from `pending` or `scheduled`).
- `PUT /api/pickups/:id/complete` - Mark pickup as completed and atomically generate invoice.
  ```json
  // Response 200 OK
  {
    "success": true,
    "message": "pickup completed successfully",
    "data": {
      "pickup": {
        "id": "...",
        "status": "completed"
      },
      "payment": {
        "id": "...",
        "amount": "50000.00",
        "status": "pending"
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
    "pickup_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
  }
  ```
- `GET /api/payments` - List payments (query parameters: `page`, `limit`, `status`).
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
| `make test` | Run all unit & integration tests with race detector |
| `make migrate-up` | Run all pending database migrations |
| `make migrate-down` | Rollback the latest migration batch |
| `make seed` | Execute deterministic database seed runner |
| `make docker-up` | Build and start containers in background |
| `make docker-down` | Stop and remove container stack |
| `make clean` | Clean build artifacts and binary outputs |

---

## 📄 License
This project is proprietary and intended for the GEU Waste Management System.
