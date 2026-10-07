# GEU Waste API

Layanan REST API backend siap produksi untuk **GEU Waste Management System** (Sistem Pengelolaan Sampah Rumah Tangga), dibangun menggunakan Go (Golang) dan PostgreSQL. Sistem ini mengelola pendaftaran rumah tangga (*household*), siklus hidup penjemputan sampah (*pickup lifecycle*), kalkulasi tarif otomatis dan pembuatan faktur/tagihan tagihan (*invoice*), verifikasi unggah bukti pembayaran, serta laporan agregasi ringkasan sampah dan pendapatan.

---

## 🏗️ Arsitektur & Teknologi Stack

Layanan ini mengadopsi prinsip **Clean 3-Layer Architecture** dengan pemisahan tanggung jawab (*separation of concerns*) yang ketat:

- **Transport / Handlers (`internal/handler/`)**: Routing HTTP berbasis Gin, validasi JSON ketat (`DisallowUnknownFields`, batas *body* 1 MiB untuk JSON, 6 MiB untuk multipart file upload), pemrosesan unggah file gambar, serta standardisasi amplop respons JSON (`response.SuccessResponse`, `response.ListSuccessResponse`, `response.ErrorResponse`).
- **Domain & Services (`internal/domain/`, `internal/service/`)**: Logika bisnis murni, validasi *state machine*, penegakan aturan bisnis (BR01–BR05), serta orkestrasi alur transaksi.
- **Persistence / Repositories (`internal/repository/postgres/`)**: Operasi PostgreSQL performa tinggi memanfaatkan connection pooling `jackc/pgx/v5` (`pgxpool`), *parameterized queries*, eksekusi transaksi dengan *row-level locking* (`SELECT ... FOR UPDATE`) berurutan (*household* $\rightarrow$ *pickup*) untuk mencegah *race condition*.
- **Aritmatika Finansial**: Aritmatika *fixed-point* presisi tinggi menggunakan `github.com/shopspring/decimal`. Nilai moneter disimpan sebagai kolom `NUMERIC(12,2)` di PostgreSQL dan diserialisasi sebagai string desimal 2 digit di JSON (contoh: `"50000.00"`, `"100000.00"`).
- **Runtime & Deployment**: *Multi-stage* `Dockerfile` dengan image builder Go 1.27 Alpine dan image runner Alpine minimalis, terorkestrasi melalui `docker-compose.yml` serta mendukung *graceful shutdown* (`SIGTERM`/`SIGINT`).

```
geu-waste-api/
├── cmd/
│   ├── api/          # Entrypoint utama server HTTP
│   ├── migrate/      # CLI tool migrasi skema database (up/down/version)
│   └── seed/         # CLI tool seed database deterministik
├── internal/
│   ├── config/       # Pemroses konfigurasi & environment variables
│   ├── domain/       # Entitas inti, enum, DTO, dan custom serializer
│   ├── handler/      # HTTP handler, middleware limit, dan router
│   ├── repository/   # Lapisan akses data (PostgreSQL pgxpool)
│   ├── response/     # Standar amplop format respons JSON
│   ├── seed/         # Logika eksekusi data awal (seed fixtures)
│   ├── service/      # Logika bisnis domain & transaksi atomik
│   └── validator/    # Validator JSON ketat & validasi file gambar
├── migrations/       # File migrasi skema SQL terversi (.up.sql / .down.sql)
├── postman/          # Postman collection & environment pengujian otomatis
├── seeds/assets/     # Aset biner seed (contoh bukti transfer)
├── uploads/          # Direktori penyimpanan file bukti pembayaran
├── Dockerfile        # Dockerfile build multi-stage
├── docker-compose.yml# Konfigurasi container lokal (PostgreSQL + Migrasi + API)
├── Makefile          # Otomasi task & build tooling
├── README.md         # Dokumentasi resmi (Bahasa Inggris)
└── README.id.md      # Dokumentasi resmi (Bahasa Indonesia)
```

---

## 📋 Aturan Bisnis & Logika Domain

| ID | Nama Aturan | Deskripsi & Implementasi |
|---|---|---|
| **BR01** | **Pending Payment Block** | Rumah tangga yang masih memiliki tagihan yang belum dibayar (`status: pending`) untuk penjemputan yang sudah selesai dilarang membuat permintaan penjemputan baru (`POST /api/pickups`). Mengembalikan `409 Conflict` (`HOUSEHOLD_PENDING_PAYMENT`). Pengecekan dilindungi oleh *hierarchical locking* berurutan (`households` $\rightarrow$ `waste_pickups`). |
| **BR02** | **Pending State for Scheduling** | Hanya penjemputan dengan status `pending` yang dapat dijadwalkan (`PUT /api/pickups/:id/schedule`). Status lain akan mengembalikan `409 Conflict` (`INVALID_STATE_TRANSITION`). |
| **BR03** | **Electronic Safety Check** | Sampah elektronik (`type: electronic`) hanya dapat dijadwalkan jika nilai efektif `safety_check` bernilai `true`. Saat pembuatan penjemputan baru, `safety_check: false` diperbolehkan, namun wajib diubah/dipastikan `true` saat penjadwalan. Mengembalikan `409 Conflict` (`SAFETY_CHECK_REQUIRED`) jika bernilai `false`. |
| **BR04** | **Atomic Completion & Invoicing** | Penyelesaian penjemputan (`PUT /api/pickups/:id/complete`) secara atomik mengubah status penjemputan menjadi `completed` dan menerbitkan tagihan pembayaran berstatus `pending` dalam **satu transaksi database**. Tarif sampah standar (`organic`, `plastic`, `paper`) = **Rp 50.000,00**; Sampah elektronik (`electronic`) = **Rp 100.000,00**. |
| **BR05** | **Proof Upload for Confirmation** | Konfirmasi pembayaran (`PUT /api/payments/:id/confirm`) wajib mengunggah file bukti transfer lokal yang valid (JPEG/PNG, $\le 5\text{MB}$, resolusi $\le 10.000\text{px}$). File dipindahkan dari *staging* ke *final* sebelum mutasi DB, dan file dipertahankan jika terjadi ketidakpastian commit (`ErrCommitUncertain`). |
| **D01** | **Idempotent Invoice Endpoint** | `POST /api/payments` menjamin keberadaan 1 faktur untuk pickup yang sudah selesai (mengembalikan `201 Created` jika baru dibuat, atau `200 OK` jika sudah ada). |
| **D08** | **Revenue Aggregation** | Laporan ringkasan pembayaran menghitung `total_revenue` secara ketat hanya dari pembayaran dengan status `paid`. |

---

## ⚙️ Variabel Lingkungan (*Environment Variables*)

Salin file `.env.example` menjadi `.env` untuk konfigurasi lokal:

```bash
cp .env.example .env
```

| Variabel | Nilai Default | Deskripsi |
|---|---|---|
| `SERVER_PORT` | `8080` | Port tempat server HTTP mendengarkan *request* |
| `SERVER_HOST` | `0.0.0.0` | Binding IP address host |
| `DB_HOST` | `localhost` | Host PostgreSQL (`postgres` di Docker Compose) |
| `DB_PORT` | `5432` | Port database PostgreSQL |
| `DB_USER` | `postgres` | Username database PostgreSQL |
| `DB_PASSWORD` | `postgres` | Password database PostgreSQL |
| `DB_NAME` | `geu_waste` | Nama database utama aplikasi |
| `TEST_DB_NAME` | `geu_waste_test` | Nama database terisolasi khusus pengujian integrasi |
| `DB_SSLMODE` | `disable` | Mode SSL PostgreSQL (`disable`, `require`) |
| `UPLOAD_DIR` | `./uploads/payment-proofs` | Direktori lokal penyimpanan file bukti pembayaran |
| `MAX_UPLOAD_SIZE_MB` | `5` | Batas maksimum ukuran unggah file bukti (MB) |

---

## 🚀 Menjalankan Aplikasi via Docker Compose

Untuk menjalankan seluruh stack (PostgreSQL, migrasi otomatis, dan API server):

```bash
# Build dan jalankan seluruh container di background
docker compose up -d --build

# Pantau log container aplikasi
docker compose logs -f app

# Periksa status seluruh container
docker compose ps -a

# Menghentikan container (data volume PostgreSQL tetap aman)
docker compose down
```

Setelah berjalan, API dapat diakses di `http://localhost:8080`.

---

## 💻 Panduan Pengembangan Lokal (*Local Development*)

### Prasyarat:
- **Go**: versi 1.24+ (dikompilasi & diuji pada Go 1.25 dan Go 1.27)
- **PostgreSQL**: versi 16+ atau via Docker container
- **Make**: Standard GNU make

### Langkah-langkah Setup:

1. **Jalankan Database PostgreSQL**:
   ```bash
   docker compose up -d postgres
   ```

2. **Jalankan Migrasi Database**:
   ```bash
   make migrate-up
   # Atau langsung via Go:
   go run ./cmd/migrate up
   ```

3. **Jalankan Seed Data Uji Awal (Deterministik)**:
   ```bash
   make seed
   # Atau langsung via Go:
   go run ./cmd/seed
   ```

4. **Jalankan Server API**:
   ```bash
   make run
   # Atau langsung via Go:
   go run ./cmd/api
   ```

5. **Jalankan Seluruh Suite Pengujian (Unit + Integration + Race Detector)**:
   ```bash
   make test
   # Atau secara eksplisit:
   go test -race -count=1 ./...
   ```

6. **Kompilasi Biner Produksi**:
   ```bash
   make build
   ```

---

## 📚 Katalog Endpoint API

Format respons mengikuti amplop JSON terstandarisasi:

**Respons Sukses Tunggal**:
```json
{
  "success": true,
  "message": "household created successfully",
  "data": { ... }
}
```

**Respons Sukses Berupa Daftar (Pagination)**:
```json
{
  "success": true,
  "message": "pickups retrieved successfully",
  "data": [ ... ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 25,
    "total_pages": 3
  }
}
```

**Respons Error / Validasi**:
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
- `GET /health` - Memeriksa status kesehatan server. Mengembalikan `{"success":true,"message":"service is healthy"}`.

---

### 2. Rumah Tangga (*Households*) (`/api/households`)

- `POST /api/households` - Mendaftarkan rumah tangga baru.
  ```json
  // Request Body
  {
    "owner_name": "Budi Santoso",
    "address": "Jl. Melati No. 12, Bandung"
  }
  ```
- `GET /api/households` - Mendapatkan daftar rumah tangga (parameter query: `page`, `limit`).
- `GET /api/households/:id` - Mendapatkan detail satu rumah tangga berdasarkan UUID.
- `DELETE /api/households/:id` - Menghapus rumah tangga (ditolak dengan `409 Conflict` jika masih ada riwayat penjemputan/pembayaran).

---

### 3. Penjemputan Sampah (*Waste Pickups*) (`/api/pickups`)

- `POST /api/pickups` - Membuat permintaan penjemputan sampah baru.
  - Jenis sampah yang valid (`type`): `organic`, `plastic`, `paper`, `electronic`.
  - Untuk sampah non-elektronik: field `safety_check` **tidak boleh** disertakan.
  - Untuk sampah elektronik (`electronic`): field `safety_check` **wajib** disertakan (nilai `false` diperbolehkan saat registrasi awal).
  ```json
  // Request Sampah Standar (Non-Electronic)
  {
    "household_id": "10000000-0000-4000-8000-000000000001",
    "type": "organic"
  }

  // Request Sampah Elektronik
  {
    "household_id": "10000000-0000-4000-8000-000000000001",
    "type": "electronic",
    "safety_check": false
  }
  ```
- `GET /api/pickups` - Mendapatkan daftar penjemputan sampah (parameter query: `page`, `limit`, `status`, `household_id`).
- `PUT /api/pickups/:id/schedule` - Menjadwalkan penjemputan yang berstatus `pending`.
  - Untuk sampah elektronik, `safety_check` wajib bernilai `true` (dapat diperbarui pada endpoint ini).
  ```json
  // Request Body
  {
    "pickup_date": "2026-10-15T09:00:00Z",
    "safety_check": true
  }
  ```
- `PUT /api/pickups/:id/cancel` - Membatalkan penjemputan yang berstatus `pending` atau `scheduled`.
- `PUT /api/pickups/:id/complete` - Menyelesaikan penjemputan sampah dan secara atomik menerbitkan faktur tagihan berstatus `pending`.
  ```json
  // Response 200 OK
  {
    "success": true,
    "message": "pickup completed successfully",
    "data": {
      "pickup": {
        "id": "20000000-0000-4000-8000-000000000001",
        "household_id": "10000000-0000-4000-8000-000000000001",
        "type": "organic",
        "status": "completed",
        "pickup_date": "2026-10-08T02:00:00Z",
        "safety_check": null,
        "created_at": "2026-10-07T08:00:00Z",
        "updated_at": "2026-10-08T03:00:00Z"
      },
      "payment": {
        "id": "30000000-0000-4000-8000-000000000001",
        "household_id": "10000000-0000-4000-8000-000000000001",
        "waste_id": "20000000-0000-4000-8000-000000000001",
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

### 4. Pembayaran & Bukti Transfer (*Payments*) (`/api/payments`)

- `POST /api/payments` - Menjamin ketersediaan tagihan untuk penjemputan yang sudah selesai (idempoten: `201 Created` jika baru dibuat, `200 OK` jika sudah tersedia).
  ```json
  // Request Body
  {
    "household_id": "10000000-0000-4000-8000-000000000001",
    "waste_id": "20000000-0000-4000-8000-000000000001",
    "amount": "50000.00"
  }
  ```
- `GET /api/payments` - Mendapatkan daftar pembayaran (parameter query: `page`, `limit`, `status`, `household_id`, `start_date`, `end_date`).
- `PUT /api/payments/:id/confirm` - Mengonfirmasi pembayaran dengan mengunggah bukti transfer (`multipart/form-data` dengan field `proof`).
  - Mendukung file `image/jpeg` dan `image/png` dengan ukuran $\le 5\text{MB}$ dan resolusi $\le 10.000\text{px}$.
- `GET /uploads/payment-proofs/:filename` - Endpoint statis untuk melihat/mengunduh file bukti pembayaran yang telah diunggah.

---

### 5. Laporan & Agregasi (*Reports*) (`/api/reports`)

- `GET /api/reports/waste-summary` - Mengembalikan 16 kombinasi kanonikal (*buckets*) tipe sampah dan status dalam urutan tetap:
  1. `organic` (`pending`, `scheduled`, `completed`, `canceled`)
  2. `plastic` (`pending`, `scheduled`, `completed`, `canceled`)
  3. `paper` (`pending`, `scheduled`, `completed`, `canceled`)
  4. `electronic` (`pending`, `scheduled`, `completed`, `canceled`)
- `GET /api/reports/payment-summary` - Mengembalikan ringkasan status pembayaran (`pending`, `paid`, `failed`) dan `total_revenue` yang dijumlahkan khusus dari pembayaran berstatus `paid`.

---

## 🧪 Pengujian Otomatis via Postman & Newman

Koleksi Postman terintegrasi tersedia di folder `postman/`:

- **Collection**: [`postman/geu-waste-api.postman_collection.json`](file:///Users/toothless/Documents/Coding/project/geu-waste-api/postman/geu-waste-api.postman_collection.json)
- **Environment**: [`postman/local.postman_environment.json`](file:///Users/toothless/Documents/Coding/project/geu-waste-api/postman/local.postman_environment.json)
- **Aset Bukti Uji**: [`seeds/assets/sample-proof.png`](file:///Users/toothless/Documents/Coding/project/geu-waste-api/seeds/assets/sample-proof.png)

Jalankan pengujian end-to-end secara otomatis menggunakan Newman:

```bash
npx -y newman run postman/geu-waste-api.postman_collection.json \
  --env-var base_url=http://localhost:8080
```

---

## 🛠️ Perintah Makefile

| Perintah | Fungsi / Aksi |
|---|---|
| `make build` | Mengompilasi seluruh biner Go (`bin/api`, `bin/migrate`, `bin/seed`) |
| `make run` | Menjalankan server API lokal dengan `go run ./cmd/api` |
| `make test` | Menjalankan seluruh unit & integration test dengan race detector (`-race -count=1`) |
| `make migrate-up` | Menjalankan seluruh file migrasi database pending |
| `make migrate-down` | Melakukan rollback batch migrasi database terakhir |
| `make seed` | Menjalankan seed data awal deterministik ke database |
| `make docker-up` | Membangun dan menjalankan seluruh service di Docker Compose |
| `make docker-down` | Menghentikan service Docker Compose tanpa menghapus volume data |
| `make clean` | Menghapus file biner dan artefak kompilasi |

---

## 📄 Lisensi
Proyek ini bersifat *proprietary* dan dikembangkan khusus untuk GEU Waste Management System.
