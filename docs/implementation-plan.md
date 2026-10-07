# Rencana Implementasi

## 1. Titik mulai

Mulai dengan memeriksa repository. Pertahankan module dan route health yang sudah ada jika implementasinya sesuai. Status pekerjaan ditentukan dari source dan hasil menjalankan project, bukan dari asumsi bahwa tahap tertentu sudah selesai.

Saya ingin pekerjaan berlanjut sampai seluruh scope terpenuhi, dengan pemeriksaan pada akhir setiap tahap. Selesaikan fitur utama terlebih dahulu, kemudian lengkapi graceful shutdown, unit test business rule, dan date filter sebelum final handoff.

## 2. Work packages

| Tahap | Pekerjaan | Exit criteria |
| --- | --- | --- |
| P0: design review | Review requirement dan D01–D17; cocokkan folder existing | Keputusan dipahami; kontrak/schema tidak saling bertentangan |
| P1: runtime foundation | Config, pgxpool, Dockerfile, Compose, migration runner, health DB, graceful shutdown | Satu perintah menjalankan app+DB; migration selesai; health 200; restart mempertahankan data |
| P2: schema and seed | Tiga migration lengkap, constraints/index, seed idempotent | DB kosong bisa migrate; seed dua kali tanpa duplikasi; down/up diuji hanya pada DB disposable |
| P3: household | Create/list/detail/delete, DTO, errors, pagination | HTTP dan repository behavior sesuai contract |
| P4: pickup lifecycle | Create/list/schedule/cancel, BR01–BR03, lock household | Semua state dan safety scenarios lolos |
| P5: completion and invoice | Complete transactional, tariff, EnsurePayment | BR04, rollback, unique invoice, concurrency teruji |
| P6: proof confirmation | Local storage, upload limits, payment list/confirm | BR05, file/DB failure, persistence dan cleanup teruji |
| P7: laporan dan pelengkap | SQL aggregation, payment date filter, unit tests tambahan | Bucket/revenue akurat; seluruh fitur tambahan teruji |
| P8: submission readiness | Fresh-clone test, README aplikasi, collection, published docs, repo access | Semua deliverable dapat digunakan reviewer |

P1 menyiapkan runner dan tiga schema migration yang sudah ditentukan di database.md agar startup dapat diverifikasi. P2 mengaudit constraint/index dan melengkapi seed; tidak membuat migration placeholder. Graceful shutdown boleh ditunda ke P7 jika foundation terhambat, tetapi tetap selesai sebelum final handoff.

## 3. Configuration contract

Contoh nilai berikut **hanya credential development**, bukan credential produksi.

| Variable | Example/default | Aturan |
| --- | --- | --- |
| APP_ENV | development | development/test/production |
| APP_PORT | 8080 | Integer port 1–65535 |
| DB_HOST | postgres dalam Compose; localhost dari host | Required; hostname service Compose hanya untuk network container |
| DB_PORT | 5432 | Integer port 1–65535 |
| DB_NAME | geu_waste | Required |
| DB_USER | postgres | Required |
| DB_PASSWORD | postgres | Required melalui env; tidak hardcoded di source |
| DB_SSLMODE | disable | Mode eksplisit; disable hanya untuk DB lokal |
| UPLOAD_DIR | /app/uploads/payment-proofs | Writable; mount persisten |
| DB_CONNECT_TIMEOUT | 5s | Durasi positif |
| SHUTDOWN_TIMEOUT | 10s | Durasi positif |

Compose dapat menyediakan default development yang sama dengan `.env.example`. Runtime Go membaca environment; **os.Getenv tidak membaca file .env sendiri**. Compose memuat `.env` untuk substitusi konfigurasi, lalu `environment`/`env_file` harus meneruskan variabel yang dibutuhkan ke app. Menjalankan Go langsung dari host memerlukan environment terset atau loader yang sengaja dipilih dan didokumentasikan.

Bangun DSN dengan URL encoding atau konfigurasi pgx terstruktur agar password yang mengandung `@`, `:`, atau `/` tidak merusak koneksi. Jangan memakai string interpolation mentah untuk credential. Startup menolak konfigurasi invalid dan tidak mencetak password.

## 4. Docker contract

| Komponen | Ketentuan |
| --- | --- |
| postgres | PostgreSQL major 17 dengan versi patch/digest dikunci saat build; named volume DB; healthcheck kesiapan |
| migrate | golang-migrate/v4; one-shot; menunggu postgres healthy; migration up lalu exit |
| app | Multi-stage build; menunggu migrate sukses; bind port konfigurasi; runtime non-root bila permission upload sudah disiapkan |
| seed | Service profile tools; menjalankan seed binary dari image app; tidak ikut startup normal |
| upload volume | Mounted pada directory upload dan writable oleh runtime user |

Pin toolchain Go dan image PostgreSQL/migration tool setelah ketersediaan serta kompatibilitas diperiksa. Gunakan versi Go di module yang kompatibel dengan dependency dan tersedia sebagai builder; perbaiki mismatch dengan perubahan minimum dan catat alasannya. Commit `go.mod` dan `go.sum`; jalankan tidy setelah package diimport. Hindari `go get` semua dependency spekulatif.

`Dockerfile` harus meng-copy module files sebelum source agar cache dependency berguna. `.dockerignore` mengecualikan `.git`, `.env`, file upload runtime, dan artifact lokal. Build tidak memerlukan credential repository privat untuk modul publik.

## 5. Command target

Perintah yang harus didukung hasil implementasi:

```bash
cp .env.example .env
docker compose up --build
```

Terminal kedua:

```bash
curl http://localhost:8080/health
docker compose run --rm seed
go test ./...
go vet ./...
```

Perintah di atas adalah kontrak operasional yang perlu dibuat. Nama service seed harus tersedia setelah P2. README aplikasi final menggunakan perintah yang sama setelah keberhasilannya diverifikasi.

Untuk fresh-run gunakan Compose project/volume terpisah. `docker compose down -v` menghapus database dan volume upload project tersebut; hanya gunakan bila environment tersebut memang disposable. Fresh-clone verification tidak memerlukan penghapusan data development utama.

## 6. README aplikasi final

README aplikasi ditulis berdasarkan implementasi yang benar-benar berjalan, dengan struktur:

1. Overview dan scope.
2. Stack beserta versi yang dipakai.
3. Architecture dan project structure.
4. Prerequisites dan langkah run satu perintah.
5. Environment variables dan perbedaan host/container.
6. Migration, seed, dan cara reset environment disposable.
7. API usage serta public Postman documentation link.
8. Business rules, state transitions, dan asumsi penting D01.
9. Upload format, limit, URL, serta volume persistence.
10. Unit/integration test commands dan hasil yang benar-benar diperoleh.
11. Design tradeoffs, limitation, dan remaining work bila ada.

Jangan menyatakan seluruh checklist selesai sebelum diverifikasi. Paket perancangan ini bisa dilampirkan, tetapi bukan pengganti petunjuk run aktual.

## 7. Postman collection

Folder: Health, Household (4), Pickup (5), Payment (3), Report (2). Environment: `base_url`, `household_id`, `pickup_id`, `payment_id`; tanpa credential pribadi. Script menyimpan ID dari response agar contoh bisa dijalankan berurutan.

Sertakan success dan negative examples, jumlah/status assertion, upload `proof`, serta filter/pagination. File upload pada collection sering membutuhkan pemilihan file lokal oleh reviewer; sertakan sample fiktif dan instruksi path. Jangan menyimpan absolute path komputer saya dalam collection.

Export collection/environment JSON, lalu publish dokumentasi dan verifikasi link publik secara terpisah. Tidak ada link publik yang dibuat oleh paket dokumen ini.

## 8. Review dan commit boundary

Satu tahap layak di-commit ketika perubahannya utuh, bisa dibangun, dan pemeriksaan yang relevan sudah lolos. Berikan saran commit message berdasarkan perubahan nyata pada checkpoint tersebut. Commit dan push tetap menunggu instruksi saya.

Final gate: seluruh FR-H/W/P/R, BR01–BR05, migration/seed, Compose, `.env.example`, README, collection, docs publik, akses repo, serta bukti fresh-clone run. Pastikan bisa menjelaskan transaksi, constraint, HTTP errors, dan asumsi dengan mengacu pada kode yang benar-benar dibuat.

## 9. File environment yang perlu dibuat

Gunakan contoh berikut untuk `.env.example`. Nilai ini sengaja untuk development lokal.

```dotenv
APP_ENV=development
APP_PORT=8080
DB_HOST=postgres
DB_PORT=5432
DB_NAME=geu_waste
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSLMODE=disable
UPLOAD_DIR=/app/uploads/payment-proofs
DB_CONNECT_TIMEOUT=5s
SHUTDOWN_TIMEOUT=10s
```

DB variable wajib ada ketika app dijalankan, termasuk password pada konfigurasi lokal ini. APP_ENV, APP_PORT, UPLOAD_DIR, dan timeout boleh memakai default pada tabel konfigurasi. Jangan memperlakukan environment yang diisi whitespace sebagai nilai valid. SSL mode aplikasi menerima disable, require, verify-ca, atau verify-full; mode lainnya ditolak untuk menjaga konfigurasi tetap eksplisit.

Compose menyediakan default lokal dari nilai di atas sehingga `docker compose up --build` tetap bekerja walau file .env belum disalin. `.env` dapat dipakai untuk override. App dan migrator menerima database environment yang sama, sedangkan postgres menerima POSTGRES_DB/POSTGRES_USER/POSTGRES_PASSWORD yang dipetakan dari nilai tersebut.

Jika Go dijalankan dari host, pakai DB_HOST=localhost, UPLOAD_DIR=./uploads/payment-proofs, serta environment DB lain yang sama. Publish PostgreSQL hanya pada `127.0.0.1:5432`; port app menggunakan APP_PORT. Bila port development bentrok, dokumentasikan override Compose lokal tanpa mengubah kontrak endpoint.

Sediakan `.gitignore` untuk `.env`, `.DS_Store`, hasil build/test, dan isi upload runtime. Pertahankan `uploads/.gitkeep` jika diperlukan. Asset contoh di `seeds/assets/` dan `postman/assets/` boleh masuk Git karena berupa data fiktif.

## 10. Runner migration dan service Compose

Buat binary `cmd/migrate` yang menggunakan golang-migrate/v4 dan config DB yang sama dengan aplikasi. Wrapper menyusun DSN dengan encoding yang benar; jangan menempatkan password mentah pada command shell. Binary mendukung subcommand `up`, `down 1`, dan `version`. Tanpa argumen jalankan `up`. Keadaan tidak ada migration baru dianggap sukses. Dirty migration atau error koneksi menghasilkan exit nonzero.

Service `migrate` menggunakan binary tersebut dari image project. Dengan cara ini tidak diperlukan image migration terpisah atau instalasi CLI pada komputer reviewer. App, migrate, dan seed memiliki entrypoint yang jelas agar argumen tidak tertukar.

| Service | Profile | Prasyarat | Perintah utama |
| --- | --- | --- | --- |
| postgres | Default | Volume database | PostgreSQL server |
| migrate | Default | postgres healthy | /app/migrate up |
| app | Default | migrate completed successfully | /app/api |
| seed | tools | postgres healthy dan schema siap | /app/seed |
| postgres-test | test | Volume test terpisah | PostgreSQL dengan DB geu_waste_test |
| test | test | postgres-test healthy | Unit + HTTP + integration tests |

Seed boleh bergantung pada migration service atau melakukan pemeriksaan schema sebelum bekerja; tidak boleh membuat schema alternatif. `seed` tidak ikut `up` default karena berada pada profile tools. Service test menggunakan build target yang masih memiliki Go toolchain. Jangan memasukkan toolchain ke image runtime hanya untuk menjalankan test.

Pastikan Dockerfile membuat directory upload milik runtime UID dan volume kosong memperoleh permission yang sesuai. Seed dan app memakai UID serta mount upload yang sama. Staging berada di dalam volume itu agar rename tidak melintasi filesystem. Healthcheck app harus memakai cara yang tersedia dalam image runtime; jangan menambahkan curl pada healthcheck bila curl tidak diinstal.

Perintah setelah implementasi lengkap:

```bash
# Jalankan aplikasi dan migration yang belum diterapkan.
docker compose up --build

# Seed eksplisit, boleh diulang tanpa mereset histori.
docker compose run --rm seed

# Lihat versi migration.
docker compose run --rm migrate version

# Terapkan migration baru secara eksplisit.
docker compose run --rm migrate up

# Unit dan HTTP tests tanpa database eksternal.
go test ./...
go vet ./...

# Semua tests termasuk PostgreSQL integration pada environment terpisah.
docker compose --profile test run --rm test
```

Rollback satu versi memakai `docker compose run --rm migrate down 1`, hanya pada environment disposable yang sudah dihentikan aplikasinya. Jangan menjalankan rollback bersamaan dengan app yang masih menerima request. Jangan otomatis menggunakan force untuk menghilangkan status migration dirty.

## 11. Isi minimum collection Postman

Saya ingin reviewer bisa mencoba alur normal tanpa meng-copy UUID manual. Script setelah create household menyimpan data.id ke household_id. Create pickup menyimpan data.id ke pickup_id. Complete menyimpan data.payment.id ke payment_id. POST payment juga boleh memperbarui payment_id dari data.id. Script hanya mengubah variable setelah response success sesuai harapan.

Request manual payment menggunakan amount sebagai string sesuai type pickup. Confirmation menggunakan form-data key proof dengan tipe File. Collection tidak menyimpan absolute path komputer saya; file contoh tersedia di postman/assets/sample-proof.png dan cara memilihnya dijelaskan.

Simpan collection sebagai `postman/geu-waste-api.postman_collection.json` dan environment sebagai `postman/local.postman_environment.json`. Variable base_url default `http://localhost:8080`; UUID awal kosong. Folder negative cases terpisah agar request penolakan tidak merusak alur utama. Setiap request mencantumkan status sukses, field wajib, serta contoh error yang relevan.

## 12. Pemeriksaan terakhir

- Build dari source bersih berhasil tanpa file dari komputer developer.
- Startup pertama membuat schema; startup kedua tidak error karena schema sudah ada.
- Stop/start mempertahankan database dan upload.
- Seed pertama menghasilkan fixture tepat; seed kedua tidak menimpa perubahan API.
- Semua endpoint memakai URL, method, payload, dan response yang sama dengan dokumentasi.
- Tidak ada migration dirty, error yang disembunyikan, placeholder core logic, atau test yang dipaksa skip.
- README menjelaskan credential development, host/container, profile test, seed, dan upload.
- Collection, environment, sample file, dan source sudah siap diserahkan.
- Publikasi repo/docs dan email hanya dilakukan saat saya meminta; laporkan dengan jelas bila file sudah siap tetapi belum dipublikasikan.
