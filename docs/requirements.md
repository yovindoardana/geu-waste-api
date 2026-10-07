# Kebutuhan Project

## 1. Sistem yang ingin saya buat

GEU Waste API adalah backend untuk layanan pengumpulan sampah komunitas. Satu household dapat mengajukan pickup beberapa kali. Setiap pickup menyimpan jenis sampah, jadwal, dan status pengerjaannya. Ketika pickup selesai, sistem membuat tagihan. Pembayaran dikonfirmasi melalui upload bukti, kemudian hasilnya muncul dalam laporan.

Yang paling penting bagi saya adalah data tetap konsisten saat request gagal atau dikirim bersamaan. Tidak boleh ada pickup yang dianggap selesai sementara tagihannya gagal dibuat, tagihan ganda untuk satu pickup, atau pembayaran yang dianggap lunas tanpa bukti tersimpan.

Istilah `payment` pada project ini adalah record tagihan sekaligus status pembayarannya. Karena itu payment sudah ada saat masih pending; record tersebut tidak baru dibuat ketika uang diterima. `invoice` di dokumen lain mengacu pada record payment yang sama, bukan tabel tambahan.

## 2. Batas pekerjaan

Project harus menggunakan Go, Gin, PostgreSQL, pgx/pgxpool, local file storage, Docker, dan Docker Compose. Query ditulis eksplisit tanpa ORM. Aplikasi dipisahkan menjadi handler, service, dan repository.

Scope utama mencakup 14 endpoint bisnis, input validation, response yang konsisten, database migration, seed, collection Postman, README aplikasi, dan dokumentasi API yang bisa diakses reviewer. Tambahkan graceful shutdown, unit test aturan bisnis, dan filter rentang tanggal payment. Ketiga tambahan itu tetap masuk target akhir project, tetapi fitur utama dikerjakan lebih dahulu.

Tidak ada frontend, login, role, payment gateway, webhook, notifikasi, multi-tenant, pengaturan rute petugas, cloud storage, atau antrean pekerjaan. Tidak ada data berat/volume sampah, diskon, pajak, pembayaran sebagian, cicilan, refund, dan perubahan tarif lewat API. Semua contoh dan seed memakai data fiktif. Aplikasi dipakai sebagai lingkungan demonstrasi, bukan layanan publik dengan data rumah tangga sungguhan.

## 3. Requirement fungsional

| ID | Requirement wajib | Acceptance criteria |
| --- | --- | --- |
| FR-H01 | POST /api/households | UUID dibuat server; owner_name dan address wajib; 201 |
| FR-H02 | GET /api/households | List dengan pagination; kosong tetap 200 dan array kosong |
| FR-H03 | GET /api/households/:id | Detail 200; UUID valid yang tidak ditemukan 404 |
| FR-H04 | DELETE /api/households/:id | Household dapat dihapus sesuai kebijakan referensi D06 |
| FR-W01 | POST /api/pickups | Household valid, type wajib, cek pending payment; status awal pending |
| FR-W02 | GET /api/pickups | Mendukung filter status dan household_id bersama-sama |
| FR-W03 | PUT /api/pickups/:id/schedule | Mengisi pickup_date; pending menjadi scheduled; safety elektronik diperiksa |
| FR-W04 | PUT /api/pickups/:id/complete | Pickup menjadi completed dan payment tarif benar tersedia secara atomik |
| FR-W05 | PUT /api/pickups/:id/cancel | Pickup menjadi canceled sesuai transisi D02 |
| FR-P01 | POST /api/payments | Payment terkait household dan pickup; interpretasi rinci pada D01 |
| FR-P02 | GET /api/payments | Filter status dan household_id; pagination tambahan rancangan |
| FR-P03 | PUT /api/payments/:id/confirm | Multipart proof wajib; file lokal; status paid dan URL tersimpan |
| FR-R01 | GET /api/reports/waste-summary | Jumlah pickup per kombinasi type/status |
| FR-R02 | GET /api/reports/payment-summary | Jumlah payment per status dan total revenue |

Total: **14 endpoint bisnis**. `/health` dan akses file bukti merupakan tambahan infrastruktur, bukan bagian dari hitungan 14.

## 4. Aturan bisnis wajib

| ID | Aturan | Hasil yang harus dibuktikan |
| --- | --- | --- |
| BR01 | Household dengan payment pending tidak boleh membuat pickup baru | Reject tanpa insert pickup |
| BR02 | Hanya pickup pending dapat dijadwalkan | Status lain ditolak dan tidak berubah |
| BR03 | Electronic hanya dapat dijadwalkan jika safety_check=true | False ditolak; true diterima bila syarat lain terpenuhi |
| BR04 | Completion otomatis membuat payment | Organic/plastic/paper 50000; electronic 100000 |
| BR05 | Confirmation wajib upload proof lokal | Tanpa file ditolak; file valid dan path tersimpan saat sukses |

BR04 diwujudkan melalui transaksi yang mengubah status pickup sekaligus membuat payment pending. Request payment manual tidak boleh membuat invoice kedua untuk pickup yang sama; lihat D01.

## 5. Requirement data

Semua entitas memakai UUID primary key dan `created_at`, `updated_at`. Field asli dipertahankan, termasuk `payments.waste_id`; jangan menggantinya diam-diam menjadi `pickup_id` dalam API.

| Entitas | Field bisnis wajib tersedia |
| --- | --- |
| Household | owner_name, address |
| Waste Pickup | household_id, type, status, pickup_date nullable, safety_check wajib untuk electronic |
| Payment | household_id, waste_id, amount decimal wajib, payment_date nullable, status, proof_file_url nullable |

Nilai yang diizinkan: type `organic/plastic/paper/electronic`; pickup status `pending/scheduled/completed/canceled`; payment status `pending/paid/failed`.

## 6. Non-functional requirements

| ID | Kategori | Target |
| --- | --- | --- |
| NFR01 | Utama | `docker compose up --build` menjalankan aplikasi dan PostgreSQL |
| NFR02 | Utama | Handler, service, repository terpisah dan dapat dijelaskan |
| NFR03 | Utama | Input invalid tidak menyebabkan panic; HTTP code konsisten |
| NFR04 | Utama | Migration berurutan dan langkah seeding terdokumentasi |
| NFR05 | Desain | Completion dan invoice atomic; database melarang invoice ganda |
| NFR06 | Desain | Error publik tidak membocorkan SQL, credential, atau absolute path |
| NFR07 | Desain | Parameter SQL terikat; daftar memakai limit dan urutan deterministik |
| NFR08 | Desain | Startup gagal bila config, migration, atau DB tidak siap |
| NFR09 | Desain | Upload memiliki batas ukuran, MIME verification, nama aman, volume persisten |
| NFR10 | Desain | Tidak mengklaim benchmark/SLA yang belum diukur |

## 7. Deliverables dan Definition of Done

- [ ] Source Go dengan semua endpoint dan aturan wajib.
- [ ] Compose menjalankan app + DB; migration selesai sebelum app menerima traffic.
- [ ] Migration dan seed tersedia; cara menjalankannya jelas.
- [ ] `.env.example`; tidak ada secret nyata dalam repository.
- [ ] README aplikasi: setup/run, migration, seed, env, struktur, contoh API, keputusan/asumsi, test.
- [ ] Postman/Insomnia JSON mencakup 14 endpoint, health, dan contoh upload.
- [ ] Published Postman docs bisa diakses publik untuk reviewer.
- [ ] Link repository GitHub/GitLab dapat diakses reviewer.
- [ ] Alur end-to-end dan negative cases lolos.
- [ ] Graceful shutdown, unit test aturan bisnis, dan filter tanggal payment sudah diuji.
- [ ] Fresh-clone verification dilakukan di environment kosong khusus tes.

Alamat pengumpulan: `it@greenenergiutama.co.id`, CC `wiwin.tri.akhdiana@greenenergiutama.co.id`. Pembuatan source dan dokumen boleh diselesaikan sampai siap dikumpulkan. Publikasi repository, publikasi dokumentasi, dan pengiriman email dilakukan hanya saat saya meminta tindakan tersebut.


## 8. Perilaku yang perlu tetap jelas

- Household yang memiliki payment pending tidak boleh **membuat** pickup baru. Pickup yang sudah telanjur ada tetap boleh dijadwalkan, diselesaikan, atau dibatalkan sesuai state-nya.
- Tidak ada batas satu pickup aktif per household. Beberapa pickup pending/scheduled diperbolehkan selama syarat create terpenuhi pada saat transaksi berjalan.
- Payment paid dan failed tidak memblokir create pickup. Failed bukan pembayaran sukses dan tidak dihitung sebagai revenue.
- Safety check hanya berlaku untuk electronic. Nilai false berarti pemeriksaan belum lolos, bukan field yang hilang.
- Tarif dihitung per pickup, tidak bergantung pada berat, jumlah barang, household, atau tanggal.
- Hanya pickup scheduled yang bisa completed. Pending harus dijadwalkan lebih dahulu; canceled dan completed tidak dibuka kembali.
- Household yang masih memiliki histori pickup atau payment tidak dapat dihapus.
- Tidak ada endpoint edit household, detail pickup, detail payment, reschedule, atau delete pickup/payment dalam scope. Jangan menambahnya hanya untuk mempermudah test.

## 9. Contoh alur yang dianggap berhasil

Saya membuat household baru, kemudian membuat pickup organic. Statusnya pending. Setelah dijadwalkan, status berubah menjadi scheduled. Ketika diselesaikan, status berubah menjadi completed dan satu payment pending sebesar `50000.00` tersedia dalam transaksi yang sama.

Jika saya langsung membuat pickup lain untuk household tersebut, sistem menolak dengan 409. Setelah bukti pembayaran valid diupload dan payment menjadi paid, permintaan pickup baru boleh dibuat. Laporan pembayaran kemudian menunjukkan tambahan revenue sebesar `50000.00`.

Untuk electronic, alurnya sama dengan dua perbedaan: schedule memerlukan safety_check=true dan tarifnya `100000.00`.

## 10. Penyerahan dan penilaian

Hasil akhir berupa link repository GitHub/GitLab, README, collection Postman JSON, serta link published Postman documentation yang dapat diakses publik. README menjelaskan setup/run, migration, seeding, seluruh environment variable, struktur project, keputusan penting, dan contoh penggunaan API.

Kualitas yang ingin saya tunjukkan adalah aturan bisnis yang benar, kode Go yang idiomatis, pemisahan layer yang jelas, error handling yang rapi, dan project yang mudah dijalankan. Saya harus bisa menjelaskan keputusan dan implementasinya sendiri.

Waktu pengerjaan yang dialokasikan adalah lima hari kalender; pengumpulan lebih cepat menjadi nilai tambah. Tanggal dan jam pengiriman saya tentukan sendiri, bukan dihitung dari waktu file dibuat. Ketentuan orisinalitas menilai penggunaan AI minimal atau di bawah 50%, termasuk pemeriksaan kode dan dokumentasi; jangan membuat klaim persentase atau klaim kepengarangan yang tidak bisa dibuktikan.
