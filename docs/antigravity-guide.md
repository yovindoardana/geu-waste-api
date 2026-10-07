# Instruksi Pengerjaan untuk Antigravity

Saya ingin kamu menyelesaikan GEU Waste API berdasarkan spesifikasi dalam folder ini. Seluruh requirement pengerjaan ada di sini. Jangan mencari aturan tambahan dari lampiran, percakapan lama, atau project lain.

## Sebelum mengubah kode

Baca requirements.md, decisions.md, design.md, database.md, api-contract.md, testing.md, dan implementation-plan.md. Setelah itu periksa source yang sudah ada: struktur folder, module path, dependency, route, Docker, migration, dan test. Gunakan implementasi yang sudah benar; jangan memulai ulang project hanya karena struktur foldernya tidak persis seperti contoh.

Module project adalah `github.com/yovindoardana/geu-waste-api`. Stack yang dipakai: Go, Gin, PostgreSQL 17, pgx/pgxpool, golang-migrate/v4, dan penyimpanan file lokal.

## Cara bekerja

Kerjakan tahap P0 sampai P8 secara berurutan. Setelah satu tahap lolos pemeriksaan, lanjutkan ke tahap berikutnya tanpa meminta persetujuan ulang untuk pekerjaan yang masih berada dalam scope ini. Kalau saya memberi batas tahap tertentu pada instruksi yang lebih baru, ikuti batas tersebut.

Jangan menghasilkan seluruh project sekaligus tanpa checkpoint. Saya ingin perubahan tetap mudah ditinjau: satu bagian selesai, diverifikasi, lalu lanjut. Kirim update singkat saat checkpoint tercapai. Bila ada test gagal, cari penyebabnya dan perbaiki sebelum menambah fitur.

Kamu boleh menentukan nama fungsi, pembagian file kecil, helper internal, dan detail idiomatik Go. Perubahan endpoint, field, business rule, tarif, format nominal, state transition, atau kebijakan delete harus dibicarakan karena memengaruhi kontrak.

Dokumen ini adalah acuan requirement. Dokumentasi teknis resmi library boleh dipakai untuk memahami API library, tetapi jangan mengambil requirement bisnis lain dari sana. Tidak perlu menanyakan aturan yang jawabannya sudah tertulis di dokumen ini.

## Hal yang harus dijaga

- Handler mengurus HTTP; service menjalankan aturan bisnis; repository mengurus query dan transaksi database.
- Semua query dalam satu transaksi harus memakai transaksi yang sama, termasuk insert payment saat complete.
- Payment tidak boleh ganda. Household payment harus cocok dengan household pickup.
- Amount dan revenue tidak boleh memakai floating point.
- Safety_check false harus bisa dibedakan dari field yang tidak dikirim.
- Request concurrent harus mengikuti urutan lock yang dijelaskan di design.md.
- Proof harus lolos batas ukuran dan pemeriksaan isi; path client tidak pernah dipakai langsung.
- Jangan menghapus proof jika status commit database masih tidak pasti.
- Jangan menambahkan auth, frontend, payment gateway, endpoint tambahan, atau abstraksi besar di luar scope.
- Jangan menambahkan dependency hanya untuk pekerjaan kecil yang sudah tercakup standard library atau package existing.
- Secret, .env lokal, dan upload runtime tidak masuk Git.
- Tidak boleh ada success response palsu, dummy repository, TODO pada alur inti, atau test yang dilewati untuk menyembunyikan bug.

## Menangani hal yang belum berjalan

Jika Go/image Docker tidak cocok, buktikan mismatch-nya dan pilih perbaikan minimum. Jika Docker atau network tidak tersedia, tetap selesaikan kode dan pemeriksaan yang bisa dijalankan, lalu jelaskan verifikasi yang terhalang. Jangan menulis bahwa integration test atau fresh-clone test lolos bila belum dijalankan.

Jika menemukan konflik nyata antarspesifikasi, tunjukkan dua aturan yang bertentangan, usulkan satu penyelesaian, dan jelaskan dampaknya. Lanjutkan pekerjaan lain yang tidak bergantung pada keputusan tersebut. Jangan mengubah kontrak secara diam-diam.

## Checkpoint yang saya perlukan

Setiap tahap cukup dilaporkan dengan empat hal: apa yang sudah berubah, file utama yang disentuh, hasil verifikasi, dan pekerjaan berikutnya. Jangan menjelaskan ulang seluruh requirement setiap kali selesai satu file.

Gunakan testing.md sebagai checklist perilaku. Unit test cukup untuk kalkulasi atau keputusan service yang terisolasi. Untuk transaksi, constraint, lock, dan rollback, gunakan PostgreSQL nyata. Jangan menganggap mock sudah membuktikan perilaku database.

Saat satu tahap sudah layak di-commit, berikan saran commit message sesuai perubahan nyata. Jangan menjalankan commit, push, mengubah visibilitas repository, menerbitkan dokumentasi, atau mengirim email tanpa instruksi saya.

## Definition of done

Project baru dianggap selesai ketika seluruh endpoint, business rule, schema, seed, Docker, upload, laporan, test, dan dokumentasi dapat digunakan sesuai kontrak. Semua file yang dibutuhkan reviewer harus tersedia, termasuk .env.example, collection Postman, environment Postman tanpa secret, dan sample proof fiktif.

README aplikasi ditulis berdasarkan cara menjalankan project yang benar-benar sudah diverifikasi. Gunakan bahasa langsung dan contoh konkret. Jangan membuat klaim pengalaman, hasil benchmark, jumlah test lolos, atau kepengarangan yang tidak didukung pekerjaan nyata.

Handoff akhir memuat cara menjalankan, cara seed/test, hasil verifikasi, keputusan penting yang diterapkan, dan blocker yang masih ada. Jika publikasi belum diminta, siapkan seluruh file untuk publikasi dan jelaskan bahwa tindakan publikasinya belum dilakukan.
