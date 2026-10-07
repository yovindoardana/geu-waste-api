# GEU Waste API

Saya ingin membangun API pengumpulan sampah komunitas yang sederhana, rapi, dan bisa dijalankan dari clone baru tanpa setup database manual. Project ini mengelola rumah tangga, permintaan pickup, pembayaran layanan, dan ringkasan operasional.

Repository: `github.com/yovindoardana/geu-waste-api`.

Dokumen dalam paket ini menjadi acuan pengerjaan project. Semua aturan bisnis, kontrak API, struktur data, keputusan teknis, dan kriteria selesai sudah dijelaskan di sini. Gunakan spesifikasi ini langsung; tidak ada requirement tambahan yang perlu dicari di lampiran atau riwayat percakapan.

## Mulai dari sini

| Dokumen | Kapan dibaca |
| --- | --- |
| [requirements.md](docs/requirements.md) | Pertama, untuk memahami sistem yang harus dibuat |
| [decisions.md](docs/decisions.md) | Sebelum coding, untuk memahami alasan di balik perilaku sistem |
| [design.md](docs/design.md) | Saat menyusun layer, transaksi, dan alur request |
| [database.md](docs/database.md) | Saat membuat migration, query, dan seed |
| [api-contract.md](docs/api-contract.md) | Saat membuat handler, response, dan collection Postman |
| [testing.md](docs/testing.md) | Sejak implementasi dimulai, bukan hanya menjelang selesai |
| [implementation-plan.md](docs/implementation-plan.md) | Untuk menentukan urutan kerja dan memeriksa hasil tiap tahap |
| [antigravity-guide.md](docs/antigravity-guide.md) | Sebagai instruksi kerja Antigravity selama project dikerjakan |

Keputusan di `decisions.md` adalah keputusan yang dipakai project ini. Jangan membiarkannya sebagai pilihan yang belum ditentukan atau mengganti perilakunya tanpa memperbarui bagian terkait.

## Cara membaca aturan

`requirements.md` menentukan hasil yang harus dicapai. `api-contract.md` menentukan perilaku yang terlihat oleh client. `database.md` menentukan bentuk data dan constraint. `design.md` menjelaskan cara menyusun implementasinya. `testing.md` membuktikan bahwa semuanya berjalan sesuai kontrak.

Jika ada perbedaan penulisan, utamakan aturan bisnis di requirements dan keputusan bernomor di decisions, lalu selaraskan kontrak, schema, dan test. Jangan memilih versi yang paling mudah diimplementasikan. Konflik yang mengubah perilaku bisnis perlu disampaikan kepada saya dengan usulan penyelesaian yang konkret.

## Cara saya ingin project ini dikerjakan

Mulai dengan membaca source yang sudah ada. Pertahankan Go module dan endpoint health jika keduanya sudah berjalan. Lanjutkan per tahap sampai seluruh scope selesai; jangan membuat ulang project atau menambahkan fitur di luar kebutuhan.

Saya ingin setiap tahap menghasilkan perubahan yang bisa dijalankan dan diperiksa. Bila suatu tahap belum berhasil, selesaikan penyebabnya sebelum masuk fitur berikutnya. Laporkan apa yang berubah, hasil test yang benar-benar dijalankan, dan hal yang masih menghalangi. Jangan menandai fitur selesai hanya karena file kodenya sudah dibuat.

## Menaruh dokumen di repository

Salin direktori `docs/` ke root project. README paket ini adalah petunjuk penggunaan spesifikasi. README aplikasi tetap dibuat di root repository berdasarkan implementasi akhir: cara menjalankan, environment, migration, seed, test, contoh API, dan link dokumentasi Postman.

Spesifikasi ini menjelaskan target sistem, bukan laporan bahwa semua target sudah selesai. Catatan progres dikelola terpisah dari aturan yang berlaku agar Antigravity tidak salah membaca rencana sebagai hasil implementasi.
