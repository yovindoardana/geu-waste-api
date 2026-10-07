# Kontrak API

Kontrak ini dipakai oleh handler, test, dan collection Postman. Perilaku yang dijelaskan di sini harus sama pada ketiganya. Base URL development: `http://localhost:8080`. Prefix bisnis: `/api`. Autentikasi tidak termasuk scope.

## 1. Conventions

- JSON memakai snake_case; error/message berbahasa Inggris.
- UUID invalid/nil UUID: 400. UUID valid tetapi target resource tidak ada: 404.
- Semua timestamp output RFC3339 UTC, contoh `2026-10-08T02:00:00Z`.
- Nullable field tetap muncul sebagai `null`; array kosong memakai `[]`.
- Unknown body/query field, duplicate query key, body JSON rusak, tipe salah, dan trailing JSON ditolak 400. Content-Type tidak didukung: 415.
- Body JSON maksimal 1 MiB; multipart maksimal 6 MiB. Melebihi batas: 413.
- Semua input ID/timestamp/status server-managed ditolak jika dikirim client.
- Amount JSON berupa string decimal, output tepat dua angka pecahan, contoh `"50000.00"`.

### Response sukses

```json
{
  "success": true,
  "message": "household created successfully",
  "data": {
    "id": "11111111-1111-4111-8111-111111111111",
    "owner_name": "Demo Household",
    "address": "Jalan Contoh 1",
    "created_at": "2026-10-07T08:00:00Z",
    "updated_at": "2026-10-07T08:00:00Z"
  }
}
```

`data` adalah object resource untuk create/detail/mutation, kecuali complete mengembalikan object `{pickup, payment}`, delete mengembalikan null, dan health mempertahankan response awal tanpa data.

### Response list

```json
{
  "success": true,
  "message": "households retrieved successfully",
  "data": [],
  "meta": {"page": 1, "limit": 20, "total": 0, "total_pages": 0}
}
```

Pagination `page >= 1`, `1 <= limit <= 100`; default page=1, limit=20. Nilai non-integer/out-of-range/overflow ditolak 400, bukan di-clamp diam-diam. Page melebihi total menghasilkan array kosong dan meta aktual. Urutan `created_at DESC, id DESC`.

### Response error

```json
{
  "success": false,
  "message": "household has pending payment",
  "code": "HOUSEHOLD_PENDING_PAYMENT",
  "errors": null
}
```

Validasi field menggunakan `errors` sebagai array `{field, message}`; error non-field memakai null. Jangan mengirim raw database error.

| Status | Code contoh | Penggunaan |
| --- | --- | --- |
| 400 | VALIDATION_ERROR | Field, UUID, date, query, amount invalid |
| 404 | RESOURCE_NOT_FOUND | Target resource tidak ditemukan |
| 409 | HOUSEHOLD_PENDING_PAYMENT | BR01 |
| 409 | INVALID_STATE_TRANSITION | State tidak mengizinkan aksi |
| 409 | SAFETY_CHECK_REQUIRED | Electronic belum lolos safety |
| 409 | HOUSEHOLD_HAS_DEPENDENCIES | Delete household yang direferensikan |
| 409 | PAYMENT_CONFLICT | Invoice existing tidak konsisten |
| 413 | PAYLOAD_TOO_LARGE | Batas body/file terlampaui |
| 415 | UNSUPPORTED_MEDIA_TYPE | Content-Type/jenis proof tidak didukung |
| 500 | INTERNAL_ERROR | Kegagalan internal tak terduga |
| 503 | SERVICE_UNAVAILABLE | Health gagal ping DB |

Tidak ada jaminan urutan error apabila beberapa field sekaligus invalid, tetapi validasi format dilakukan sebelum business mutation. Cross-household mismatch valid secara sintaks menghasilkan 400, bukan 404 palsu.

## 2. Resource representation

| Resource | Field output |
| --- | --- |
| Household | id, owner_name, address, created_at, updated_at |
| Pickup | id, household_id, type, status, pickup_date, safety_check, created_at, updated_at |
| Payment | id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at |

Tidak melakukan nested household expansion dalam list versi ini. Response resource penuh setelah mutation harus konsisten dengan data committed.

## 3. Household endpoints

### POST /api/households

Content-Type `application/json`.

```json
{"owner_name":"Demo Household","address":"Jalan Contoh 1"}
```

Trim kedua field, wajib string tidak kosong; owner_name maksimal 150 karakter, address 1000. Return **201** dengan Household. Nilai nama/alamat duplicate diperbolehkan.

### GET /api/households

Query yang diterima: `page`, `limit`. Return **200**, array Household dan meta.

### GET /api/households/:id

Tanpa body/query. Return **200** Household; malformed id 400; tidak ditemukan 404.

### DELETE /api/households/:id

Tanpa body/query. Hanya tanpa referensi pickup/payment. Return **200**:

```json
{"success":true,"message":"household deleted successfully","data":null}
```

Referenced household: 409. Tidak ada: 404. Tidak memakai 204 karena kontrak ini mempertahankan JSON envelope.

## 4. Pickup endpoints

### POST /api/pickups

Body non-electronic:

```json
{"household_id":"11111111-1111-4111-8111-111111111111","type":"organic"}
```

Body electronic:

```json
{"household_id":"11111111-1111-4111-8111-111111111111","type":"electronic","safety_check":false}
```

type exact lowercase: organic/plastic/paper/electronic. Electronic wajib boolean non-null; false valid saat create. Non-electronic harus menghilangkan safety_check; nilai apa pun yang dikirim ditolak 400. Server menetapkan pending, pickup_date=null. Household tidak ada 404; pending payment 409. Return **201** Pickup.

### GET /api/pickups

Query: `page`, `limit`, `status`, `household_id`. Filter opsional menggunakan AND. Status harus enum pickup; UUID filter valid tetapi tidak punya row menghasilkan list kosong 200. Return array Pickup dan meta.

Contoh: `/api/pickups?status=pending&household_id=11111111-1111-4111-8111-111111111111&page=1&limit=20`.

### PUT /api/pickups/:id/schedule

```json
{"pickup_date":"2026-10-08T09:00:00+07:00","safety_check":true}
```

pickup_date wajib RFC3339 dengan offset, dinormalisasi UTC. safety_check opsional hanya untuk electronic; bila dihilangkan gunakan nilai existing. Non-electronic hanya mengirim pickup_date. Nilai efektif electronic harus true. Current state harus pending. Return **200** Pickup scheduled. Missing/invalid field 400; non-pending 409; effective safety false 409; not found 404.

### PUT /api/pickups/:id/complete

Tanpa body atau object JSON kosong `{}`. Field lain ditolak. Hanya scheduled. Return **200**:

```json
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

Tarif ditentukan type: organic/plastic/paper=50000.00; electronic=100000.00. Client tidak mengirim amount. Repeated completion 409; kegagalan insert payment membuat seluruh completion rollback.

### PUT /api/pickups/:id/cancel

Tanpa body atau `{}`. pending/scheduled → canceled. Return **200** Pickup. completed/canceled → 409. Tidak membuat payment dan tidak mengubah payment existing. Tidak ada cancellation reason dalam scope.

## 5. Payment endpoints

### POST /api/payments

Lihat keputusan **D01**: endpoint memastikan invoice untuk pickup completed, bukan invoice tambahan.

```json
{
  "household_id":"11111111-1111-4111-8111-111111111111",
  "waste_id":"22222222-2222-4222-8222-222222222222",
  "amount":"50000.00"
}
```

Ketiga field wajib. Amount menerima digit positif dengan nol sampai dua angka pecahan dalam string; normalize ke dua digit. Tidak menerima number JSON, exponent, separator ribuan, whitespace, NaN, atau Infinity. Nilai harus sama dengan tarif pickup; mismatch 400.

Household/pickup tidak ada 404; ownership mismatch 400; pickup belum completed 409. Tidak ada invoice: **201**, Payment pending baru. Invoice sudah ada dan konsisten: **200**, Payment existing tanpa mutation, termasuk bila sudah paid/failed. Invoice inconsistent: 409. Tidak boleh mengganti proof atau mereset status melalui endpoint ini.

### GET /api/payments

Query: `page`, `limit`, `status`, `household_id`; `start_date`, `end_date`. Return **200**, array Payment dan meta.

Tanggal `YYYY-MM-DD`, menggunakan payment_date. Batas boleh diberikan sendiri; start ≤ end jika keduanya ada. Start pada 00:00 UTC inklusif, end sampai sebelum 00:00 UTC hari berikutnya. Null payment_date terbuang ketika salah satu filter tanggal ada. Tanpa filter tanggal, semua status tetap bisa muncul.

Contoh: `/api/payments?status=paid&start_date=2026-10-01&end_date=2026-10-07&page=1&limit=20`.

### PUT /api/payments/:id/confirm

Content-Type `multipart/form-data`. Tepat satu file `proof`; tidak menerima field status/payment_date/proof_file_url. File JPEG/PNG valid, tidak kosong, maksimal 5 MiB. Lebar/tinggi masing-masing maksimal 10000 piksel dan total maksimal 20 juta piksel; dimensi di luar batas ditolak 400. Total multipart maksimal 6 MiB.

```bash
curl -X PUT http://localhost:8080/api/payments/33333333-3333-4333-8333-333333333333/confirm \
  -F 'proof=@./sample-proof.jpg'
```

UUID di contoh adalah ilustrasi; gunakan ID hasil request aktual. Return **200**, Payment paid dengan payment_date server dan URL relatif. Missing/empty/malformed image 400; unsupported detected type 415; terlalu besar 413; paid/failed 409; unknown payment 404. File gagal tidak membuat payment paid.

## 6. Reports

### GET /api/reports/waste-summary

Tanpa parameter. Return **200**, data berisi `items` dan `total_pickups`. Selalu 16 bucket: type berurutan organic, plastic, paper, electronic; setiap type status pending, scheduled, completed, canceled. Bucket kosong bernilai 0.

Setiap item mempunyai bentuk `{"type":"organic","status":"pending","count":0}`. `total_pickups` adalah jumlah seluruh count. Contract ini tidak menambahkan total berat karena project ini tidak mencatat berat sampah.

### GET /api/reports/payment-summary

Tanpa parameter. Contoh database kosong:

```json
{
  "success": true,
  "message": "payment summary retrieved successfully",
  "data": {
    "currency": "IDR",
    "items": [
      {"status":"pending","count":0,"total_amount":"0.00"},
      {"status":"paid","count":0,"total_amount":"0.00"},
      {"status":"failed","count":0,"total_amount":"0.00"}
    ],
    "total_payments": 0,
    "total_revenue": "0.00"
  }
}
```

Urutan pending, paid, failed. Revenue hanya paid. Jangan menyamakan total invoice semua status dengan revenue.

## 7. Infrastructure endpoints

`GET /health` memeriksa DB dengan timeout 2 detik. Healthy **200** mempertahankan kontrak awal:

```json
{"success":true,"message":"service is healthy"}
```

Unhealthy **503** menggunakan envelope error dengan SERVICE_UNAVAILABLE tanpa detail koneksi. Ini readiness check sederhana, bukan monitoring lengkap.

`GET /uploads/payment-proofs/<server-generated-filename>` mengembalikan file image saat ada, 404 jika tidak ada. Tidak membuka arbitrary filesystem path atau directory listing. Response binary ini dikecualikan dari JSON envelope.

## 8. Detail parsing yang tidak boleh berbeda antar-handler

Saya ingin input yang salah ditolak dengan jelas, bukan dikoreksi diam-diam. Aturan berikut berlaku untuk seluruh endpoint yang relevan.

| Input | Aturan |
| --- | --- |
| JSON object | Body create/schedule wajib object non-null; array, scalar, body kosong, dan null ditolak 400 |
| JSON keys | Nama case-sensitive sesuai kontrak; unknown key dan duplicate key ditolak 400 |
| String | owner_name/address ditrim; ID, enum, amount, dan date tidak menerima whitespace tambahan |
| UUID | Bentuk berhyphen 8-4-4-4-12; hex upper/lower diterima; output lowercase; nil ditolak |
| Boolean | Hanya true/false JSON; string, angka, dan null ditolak |
| Schedule safety | Omitted berarti gunakan existing; explicit null tetap 400 |
| Query kosong | `?status=` atau `?page=` ditolak; optional berarti key tidak dikirim |
| Pagination | Hanya digit desimal; tanda plus/minus, pecahan, dan whitespace ditolak |
| Date | Tanggal kalender valid YYYY-MM-DD; tidak menerima timestamp untuk start/end |
| Timestamp | RFC3339 dengan Z atau offset ±HH:MM; normalisasi UTC dan mikrodetik |
| Amount | Pola `^[0-9]+(\\.[0-9]{1,2})?$`, nilai positif dan maksimal 9999999999.99; maksimum 13 karakter |
| Content-Type JSON | application/json dengan parameter charset UTF-8 boleh; media type lain 415 |

Untuk amount, leading zero diperbolehkan selama panjang/nominal valid; response selalu bentuk canonical tanpa leading zero berlebih dan tepat dua angka pecahan. Input `"00050000.0"` menjadi `"50000.00"`. Request terlalu panjang ditolak sebelum parsing nominal.

Complete/cancel menerima body kosong tanpa Content-Type atau `{}` dengan Content-Type JSON. Literal null dan field apa pun ditolak 400. Endpoint GET/DELETE tidak menerima body nonkosong. Semua endpoint hanya menerima query yang tercantum; mutation tidak menerima query tambahan. Multipart confirmation dengan Content-Type multipart tetapi boundary invalid adalah 400.

Unknown route mendapat 404 RESOURCE_NOT_FOUND. Method yang salah pada route dikenal mendapat 405 METHOD_NOT_ALLOWED dengan header Allow yang tepat. Tidak ada redirect trailing slash otomatis untuk mutation. GET file mendukung HEAD untuk metadata dan tetap tidak membuka directory listing.

## 9. Urutan pemeriksaan

Pemeriksaan dasar: ukuran/Content-Type → parsing dan validasi format → lookup resource → relasi → state → aturan bisnis → mutation. Tidak perlu menjamin urutan antar-error field yang sama-sama invalid, tetapi tidak boleh ada mutation sebelum seluruh syarat lolos.

Khusus schedule: periksa status pending sebelum effective safety. Pickup completed dengan safety false tetap mendapat INVALID_STATE_TRANSITION. Khusus POST payment: validasi sintaks amount lebih dahulu, kemudian resource/kepemilikan/state, baru kesesuaian tarif dan invoice existing.

Konfirmasi boleh memeriksa payment lebih dahulu agar tidak menyimpan file untuk ID yang tidak ada. Tetap lakukan pengecekan ulang setelah lock sebelum status diubah. File staging yang dibuat sebelum pemeriksaan final harus dibersihkan bila request ditolak.

## 10. Message response

Gunakan message berikut untuk hasil normal. Client sebaiknya membaca success, HTTP status, dan code untuk keputusan program; message ditujukan untuk manusia.

| Operasi | Message |
| --- | --- |
| Household create / list / detail / delete | household created successfully / households retrieved successfully / household retrieved successfully / household deleted successfully |
| Pickup create / list | pickup created successfully / pickups retrieved successfully |
| Pickup schedule / complete / cancel | pickup scheduled successfully / pickup completed successfully / pickup canceled successfully |
| Payment POST 201 / POST 200 | payment created successfully / payment already exists |
| Payment list / confirm | payments retrieved successfully / payment confirmed successfully |
| Waste / payment report | waste summary retrieved successfully / payment summary retrieved successfully |

Contoh error validasi:

```json
{
  "success": false,
  "message": "validation failed",
  "code": "VALIDATION_ERROR",
  "errors": [
    {"field": "owner_name", "message": "owner_name is required"}
  ]
}
```

Untuk body yang rusak gunakan field `body`; UUID route invalid memakai `id`; invalid query memakai nama query. Semua response JSON memiliki Content-Type application/json. Error internal memakai message `internal server error`, tanpa stack trace.

## 11. Waste report untuk seed awal

Response berikut berlaku setelah seed pertama pada DB kosong. Jika reviewer mengubah data, hasilnya mengikuti data aktual.

```json
{
  "success": true,
  "message": "waste summary retrieved successfully",
  "data": {
    "items": [
      {"type":"organic","status":"pending","count":1},
      {"type":"organic","status":"scheduled","count":0},
      {"type":"organic","status":"completed","count":0},
      {"type":"organic","status":"canceled","count":1},
      {"type":"plastic","status":"pending","count":0},
      {"type":"plastic","status":"scheduled","count":0},
      {"type":"plastic","status":"completed","count":1},
      {"type":"plastic","status":"canceled","count":0},
      {"type":"paper","status":"pending","count":0},
      {"type":"paper","status":"scheduled","count":1},
      {"type":"paper","status":"completed","count":1},
      {"type":"paper","status":"canceled","count":0},
      {"type":"electronic","status":"pending","count":1},
      {"type":"electronic","status":"scheduled","count":0},
      {"type":"electronic","status":"completed","count":1},
      {"type":"electronic","status":"canceled","count":0}
    ],
    "total_pickups": 7
  }
}
```

Payment report seed mempunyai tiga item: pending count 1/total_amount `50000.00`, paid count 1/total_amount `100000.00`, failed count 1/total_amount `50000.00`. total_payments=3 dan total_revenue=`100000.00`.
