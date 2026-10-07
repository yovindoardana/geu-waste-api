# Keputusan Teknis

Saya mencatat keputusan di sini supaya implementasi tidak berubah-ubah di tengah jalan. Gunakan keputusan ini sebagai perilaku project, termasuk untuk test dan contoh API. Kalau ada alasan kuat untuk mengubahnya, jelaskan dampaknya lebih dahulu dan perbarui seluruh bagian yang terkait.

## Ringkasan

| ID | Keputusan | Alasan |
| --- | --- | --- |
| D01 | POST payment memastikan satu invoice untuk pickup completed; 201 jika dibuat, 200 jika sudah ada | Completion otomatis sudah membuat invoice, jadi request manual tidak boleh membuat invoice kedua |
| D02 | pending→scheduled/canceled; scheduled→completed/canceled; completed/canceled terminal | Alur kerja mudah ditelusuri dan tidak mengubah histori yang sudah selesai |
| D03 | Electronic wajib membawa safety_check saat create; false valid. Schedule dapat memperbaruinya menjadi true | Field hilang berbeda dengan pemeriksaan yang belum lolos |
| D04 | pickup_date wajib RFC3339 ber-offset saat schedule; tanggal lampau diperbolehkan | Mendukung pencatatan jadwal historis tanpa aturan tanggal tambahan |
| D05 | Satu payment per pickup dan household keduanya harus sama | Mencegah tagihan ganda dan relasi salah |
| D06 | Household hanya dihapus jika tidak memiliki referensi pickup/payment | Histori tidak ikut hilang karena delete household |
| D07 | NUMERIC(12,2), IDR, JSON decimal string; tarif ditentukan server | Nominal tidak mengalami pembulatan float atau berubah dari input client |
| D08 | Revenue hanya berasal dari payment paid | Tagihan yang belum dibayar atau gagal bukan pendapatan |
| D09 | Semua list memakai page/limit; default 1/20; limit maksimal 100 | Client menggunakan pola yang sama pada setiap list |
| D10 | Filter tanggal memakai payment_date dalam kalender UTC | Arti tanggal tidak bergantung pada timezone server |
| D11 | Confirmation hanya pending→paid; failed tetap terminal di API ini | Belum ada alur retry pembayaran gagal atau payment gateway |
| D12 | Proof JPEG/PNG maksimal 5 MiB/20 juta piksel; sisi maksimal 10000 piksel; multipart maksimal 6 MiB | Cukup untuk bukti pembayaran dan mudah diverifikasi |
| D13 | Modular monolith, Gin, pgx, SQL eksplisit, PostgreSQL TEXT+CHECK | Cukup untuk ukuran project tanpa lapisan abstraksi berlebihan |
| D14 | Timestamp disimpan/output UTC; input menerima offset RFC3339 | Satu aturan waktu untuk API, database, dan test |
| D15 | Seed dijalankan eksplisit dan idempotent | Restart tidak mengubah data yang sedang dicoba reviewer |
| D16 | Go module existing dipertahankan; PostgreSQL major 17; migration memakai golang-migrate/v4 | Lingkungan dan tooling punya arah yang jelas |
| D17 | Tidak ada retry otomatis untuk mutation setelah hasil commit tidak pasti | Mencegah duplikasi atau penghapusan file yang ternyata sudah direferensikan |

## D01 — Payment otomatis dan endpoint manual

Saya memakai satu record payment untuk satu pickup. Record itu dibuat otomatis saat pickup selesai. Endpoint `POST /api/payments` tetap tersedia sebagai operasi untuk memastikan invoice bagi pickup completed, termasuk pemulihan data lama yang belum mempunyai invoice.

Client wajib mengirim household_id, waste_id, dan amount. Service memastikan pickup benar-benar milik household tersebut, statusnya completed, dan amount sesuai tarif. Jika record belum ada, buat payment pending dan kembalikan 201. Jika sudah ada dengan data yang konsisten, kembalikan record yang sama dengan 200. Statusnya bisa pending, paid, atau failed; jangan mengubah status atau proof yang sudah tersimpan.

Pilihan ini sengaja membuat request manual aman saat diulang. Pada alur normal, POST akan mengembalikan 200 karena completion sudah menghasilkan invoice. Jalur 201 diuji memakai fixture pemulihan pada database test khusus. Jangan membuat celah pada transaksi completion hanya supaya jalur POST 201 mudah dicoba.

Jika amount request salah, response 400. Jika invoice existing sendiri tidak konsisten dengan tarif atau relasi pickup, response 409 PAYMENT_CONFLICT dan jangan memperbaiki datanya secara diam-diam. Tidak ada prebilling, invoice bebas tanpa pickup, atau invoice tambahan.

## D02–D04 — Siklus pickup

Saya ingin pickup melalui pending, scheduled, lalu completed. Pending dan scheduled masih bisa canceled. Completed dan canceled tidak boleh diproses kembali. Mengulang schedule, complete, cancel, atau confirm mendapat 409; client dapat melihat state terakhir dari endpoint list.

Electronic boleh dibuat dengan safety_check=false, tetapi belum boleh dijadwalkan. Saat hasil pemeriksaan sudah ada, request schedule mengirim safety_check=true sekaligus pickup_date. Jika schedule tidak mengirim safety_check, gunakan nilai yang sudah tersimpan. Nilai null tetap invalid. Untuk non-electronic, field safety_check harus tidak dikirim dan nilainya NULL di database.

Canceled dari scheduled tetap menyimpan pickup_date terakhir untuk histori. Canceled dari pending memiliki pickup_date null. Tidak ada endpoint terpisah untuk mengganti type, household, atau jadwal yang sudah scheduled.

## D05–D08 — Integritas dan nominal

Database menolak waste_id duplicate dan payment dengan household yang berbeda dari pickup. Service menjaga agar perubahan status completed dan pembuatan payment terjadi dalam satu transaksi.

Tarif organic, plastic, dan paper adalah Rp50.000; electronic Rp100.000. Semua nominal menggunakan dua angka desimal meskipun tarifnya bulat. Saya memilih string decimal di JSON agar nominal tidak melewati floating-point parser. Di Go gunakan representasi exact; jangan menghitung amount atau revenue dengan float64.

Household tanpa histori boleh dihapus secara permanen. Bila sudah memiliki satu pickup saja, termasuk canceled, delete mendapat 409. Tidak ada cascade deletion atau soft delete.

## D09–D12 — List, pembayaran, dan file

List diurutkan created_at DESC lalu id DESC. Offset pagination cukup untuk project ini; hasil antarhalaman bisa bergeser jika data baru masuk di antara dua request. Satu response harus memakai filter dan snapshot yang konsisten untuk count serta items.

Filter tanggal payment adalah tanggal pembayaran, bukan tanggal invoice dibuat. Karena pending dan failed memiliki payment_date null, keduanya tidak muncul saat filter tanggal digunakan. Failed didukung oleh schema, seed, filter, dan laporan, tetapi tidak ada endpoint untuk membuat payment gagal atau mencoba ulang payment failed.

Proof disimpan pada disk lokal dengan filename buatan server. Nama file dari client tidak digunakan sebagai path. File runtime disimpan dalam Docker volume; contoh file untuk seed berada dalam source sebagai asset fiktif. Akses file dalam lingkungan demonstrasi tidak memakai autentikasi.

## D13–D17 — Implementasi dan operasional

Saya memilih SQL langsung agar query, transaksi, dan lock terlihat jelas. TEXT+CHECK menyediakan daftar nilai yang valid tanpa perlu mengelola PostgreSQL native enum. Tidak perlu ORM, event bus, background worker, atau cache untuk memenuhi scope ini.

Versi patch dependency Go yang sudah cocok dipertahankan melalui go.mod/go.sum. Pada tahap foundation, cocokkan toolchain dengan image builder yang benar-benar bisa dibangun. Jika keduanya tidak cocok, laporkan mismatch beserta perubahan minimum yang diperlukan. Ini pemeriksaan lingkungan, bukan alasan mengganti requirement atau meminta dokumen lain. PostgreSQL menggunakan major 17; patch image dan versi golang-migrate/v4 dikunci ketika build awal berhasil, lalu dicatat pada README aplikasi.

Seed tidak menjadi bagian dari startup normal. Migration menjadi prasyarat startup, dan error migration harus membuat aplikasi gagal mulai. API tidak mencoba memperbaiki schema secara otomatis.

Request yang kehilangan koneksi saat COMMIT belum tentu rollback. Jangan mengulang mutation begitu saja. Untuk upload, jangan menghapus file final sebelum memastikan bahwa database tidak mereferensikannya. Untuk complete, client dapat membaca pickup/payment terakhir sebelum memutuskan request berikutnya.
