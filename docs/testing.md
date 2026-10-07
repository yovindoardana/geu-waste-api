# Rencana Pengujian

Saya ingin test membuktikan perilaku sistem, terutama saat request gagal atau datang bersamaan. Daftar di bawah menjadi kriteria penerimaan; status lulus diisi hanya setelah test benar-benar dijalankan.

## 1. Lapisan pengujian

| Level | Target | Alat/strategi |
| --- | --- | --- |
| Unit | Business rule, tarif, transisi, validasi service | Go testing; stub/fake interface kecil; table-driven |
| HTTP | Binding, status, envelope, query, upload | httptest + router aplikasi |
| Integration | SQL, FK, CHECK, rollback, lock, aggregation | PostgreSQL disposable dengan versi sama seperti Compose |
| End-to-end | Alur reviewer dari startup sampai report | Docker Compose dan Postman/curl |

Mock tidak cukup membuktikan transaksi, PostgreSQL constraint, atau concurrency. Test tersebut wajib memakai DB nyata jika implementasi mengklaim perlindungannya. Tidak perlu mengejar persentase coverage arbitrer; utamakan cabang rule dan kegagalan yang berdampak pada data.

## 2. Business-rule matrix

| Test ID | Source | Scenario | Expected |
| --- | --- | --- | --- |
| BR-T01 | BR01 | Household punya satu pending payment | Create pickup 409; count tetap |
| BR-T02 | BR01 | Household hanya punya paid/failed | Create pickup 201 |
| BR-T03 | BR01 | Household tidak punya payment | Create pickup 201 |
| BR-T04 | BR02 | Schedule pending non-electronic | 200, date terisi, scheduled |
| BR-T05 | BR02 | Schedule scheduled/completed/canceled | Masing-masing 409; row tidak berubah |
| BR-T06 | BR03 | Electronic false, schedule tanpa override | 409; tetap pending |
| BR-T07 | BR03 | Electronic false, schedule override true | 200; safety tersimpan true |
| BR-T08 | BR03 | Electronic true, schedule tanpa override | 200 |
| BR-T09 | BR04 | Complete organic/plastic/paper | Masing-masing satu invoice pending 50000.00 |
| BR-T10 | BR04 | Complete electronic | Satu invoice pending 100000.00 |
| BR-T11 | BR04 | Insert invoice dipaksa gagal | Pickup tetap scheduled; tidak ada partial success |
| BR-T12 | BR05 | Confirm tanpa proof | 400; status tetap pending |
| BR-T13 | BR05 | Confirm proof valid | 200; paid, date, URL, file ada |

## 3. HTTP dan validation cases

| Area | Cases | Assertion |
| --- | --- | --- |
| Household | owner/address omitted, null, blank, terlalu panjang | 400, tanpa insert |
| Household | trim Unicode whitespace dan batas panjang karakter | Nilai valid tersimpan rapi; multibyte tidak salah hitung |
| UUID | malformed/nil vs valid missing | 400 vs 404 |
| Delete | tanpa referensi, ada pickup, missing, ulang delete | 200, 409, 404, 404 |
| Pickup create | missing/invalid type, unknown household | 400/404 |
| Safety | electronic omitted/null/false/true | 400/400/201/201 bila rule lain lolos |
| Safety | non-electronic mengirim safety_check | 400 |
| Schedule | missing date, invalid date, tanpa offset, null | 400 |
| Schedule | offset +07 dikonversi UTC, tanggal lampau | Instant sama; tanggal lampau diterima |
| Complete | pending, canceled, completed | 409; tidak ada mutation |
| Cancel | pending, scheduled, completed, canceled | 200/200/409/409 |
| Server fields | client mengirim status/id/created_at/proof URL | 400 |
| Body | malformed JSON, trailing JSON, unknown field | 400 |
| Query | unknown key, duplicate key, invalid enum/UUID | 400 |
| Pagination | default, page 0, limit 0/101, noninteger, overflow | Default benar; invalid 400 |
| Pagination | page melewati akhir, DB kosong | 200, data [], total_pages benar |
| Filtering | household/status bersama, UUID valid tanpa data | AND; hasil kosong bukan 404 |
| Content-Type | JSON endpoint mendapat form, confirm mendapat JSON | 415 |
| Limits | JSON >1 MiB; multipart >6 MiB | 413 tanpa partial write |
| Errors | DB/storage error terkontrol | 500 generik; tidak ada DSN/SQL di body |
| Routing | unknown route | 404 dengan response JSON yang konsisten |

## 4. Payment cases

- POST completed tanpa invoice pada fixture recovery: 201, amount benar.
- POST invoice existing pending/paid/failed: 200, ID sama; status/date/proof/updated_at tidak berubah.
- POST ownership beda: 400; missing related entity: 404; pickup belum completed: 409.
- Amount berbeda tarif, negatif, nol, JSON number, exponent, >2 pecahan, NaN: 400.
- Normalisasi `"50000"` dan `"50000.0"` menjadi `"50000.00"`.
- Confirmation payment paid/failed: 409; tidak menimpa bukti.
- File kosong atau korup: 400; format bukan JPEG/PNG: 415; ukuran >5 MiB: 413.
- Extension JPG dengan isi executable tidak lolos; filename traversal dari client tidak menjadi path disk.
- Tepat 5 MiB valid diterima selama body multipart tidak melampaui 6 MiB.
- Lebih dari satu proof / unexpected multipart field: 400.
- Storage gagal: payment tetap pending; DB gagal setelah file final: cleanup aman dan tidak ada paid tanpa file.
- Commit outcome tidak pasti: file tidak dihapus sebelum cek referensi; kondisi tercatat untuk rekonsiliasi.

## 5. Database dan concurrency

| Scenario | Hasil yang dibuktikan |
| --- | --- |
| Dua complete bersamaan untuk pickup sama | Tepat satu 200, satu 409, satu invoice |
| Dua EnsurePayment bersamaan pada recovery fixture | Satu 201, satu 200, satu ID invoice |
| Complete berlomba dengan cancel | Salah satu menang; jika completed ada invoice; jika canceled tidak ada invoice |
| Create pickup berlomba dengan complete pada household sama | Terurut oleh lock: create lebih dulu boleh sukses; completion lebih dulu menyebabkan create 409 |
| Confirm berlomba dengan create pickup | Create sebelum paid melihat pending dan 409; setelah paid boleh 201 |
| Dua confirmation bersamaan | Satu paid, satu 409; hanya proof pemenang direferensikan |
| Delete household berlomba dengan create pickup | Tidak ada orphan; hasil sesuai urutan lock |
| Insert invoice dengan waste_id duplicate | Unique violation; service mapping konsisten |
| Household payment berbeda dari pickup | Composite FK menolak |
| Invalid enum, empty name, nonpositive amount | CHECK/NOT NULL menolak |
| Electronic scheduled tanpa true | CHECK menolak |
| Payment paid tanpa date/proof | CHECK menolak |

Koordinasikan test race dengan barrier/channel dan transaksi eksplisit agar urutan bisa dikontrol. Hindari mengandalkan sleep acak. Gunakan database/schema terisolasi untuk parallel test.

## 6. Date filter dan reporting

- start_date saja, end_date saja, keduanya; format salah dan start>end → 400.
- Baris tepat start midnight UTC termasuk; tepat midnight setelah end tidak termasuk.
- payment_date null tidak termasuk ketika date filter aktif.
- status=pending + date filter menghasilkan kosong pada versi ini, bukan validation error.
- Empty report: 16 waste bucket nol; 3 payment bucket nol; revenue `0.00`.
- Seed report: total_pickups=7, total_payments=3, revenue `100000.00`.
- Semua jenis/type/status masuk bucket benar; failed/pending amount tidak masuk revenue.
- Setelah confirmation, pending count turun, paid naik, revenue bertambah tepat amount.
- Agregasi amount tetap decimal exact; tidak ada floating-point artifact.

## 7. End-to-end reviewer flow

1. Clone pada environment disposable; copy `.env.example`; jalankan `docker compose up --build`.
2. Pastikan database healthy, migration sukses, `/health` 200.
3. Jalankan seed dua kali; count tetap sama dan sample proof tersedia.
4. Create household baru → create organic → schedule → complete.
5. Ambil payment_id dari completion; GET payment filter household menampilkan invoice yang sama.
6. Create pickup berikutnya ditolak 409 karena invoice pending.
7. POST payments untuk invoice yang sama → 200, ID tetap.
8. Confirm tanpa file ditolak; confirm file valid → paid.
9. Create pickup baru berhasil; jalankan electronic false lalu schedule true.
10. Verifikasi report dan akses proof URL.
11. Restart container; data DB dan proof tetap tersedia.
12. Stop DB; health 503; restart DB dan verifikasi recovery sesuai pool behavior.
13. SIGTERM app: request aktif diberi kesempatan selesai; pool ditutup.

Gunakan UUID hasil API, bukan UUID contoh dokumen. Simpan hasil pass/fail dan kendala nyata. Jangan menandai checklist hanya karena kode test sudah ditulis.

## 8. Gates

Tahap foundation: build, health, DB, migration, persistence. Tahap fitur: unit dan HTTP pada use case yang ditambahkan. Tahap completion/upload: integration rollback dan concurrency. Final: `go test ./...`, `go vet ./...`, serta integration command yang benar-benar diimplementasikan dan ditulis di README. Race detector digunakan bila didukung toolchain/platform dan test concurrency relevan; jangan mengklaim hasil sebelum dijalankan.

## 9. Struktur dan fixture test

Unit dan HTTP test ditempatkan dekat package yang diuji. Gunakan fake repository/storage hanya untuk mengisolasi keputusan service. Integration test berada di tests/integration dan memakai build tag `integration`. Dengan begitu `go test ./...` dapat dijalankan tanpa PostgreSQL, sedangkan `go test -tags=integration ./...` mencakup pengujian DB ketika environment test tersedia.

Service Compose test menyediakan connection string ke postgres-test dengan database geu_waste_test, menjalankan migration yang sama, lalu menjalankan test bertag integration. Jika database test tidak tersedia, perintah integration harus gagal dengan pesan yang jelas, bukan sukses karena seluruh test di-skip.

Gunakan clock tetap untuk menguji timestamp, temp directory untuk proof, dan UUID terpisah per test. Jangan mengandalkan urutan test atau fixture seed development untuk unit test. Test laporan seed dapat memakai fixture deterministik di database.md, tetapi test lain sebaiknya menyiapkan data minimum sendiri.

## 10. Matriks lengkap state pickup

| State awal | Schedule | Complete | Cancel |
| --- | --- | --- | --- |
| pending | 200 jika date/safety valid | 409 | 200 |
| scheduled | 409 | 200 dan payment tercipta | 200 |
| completed | 409 | 409 | 409 |
| canceled | 409 | 409 | 409 |

Setiap 409 harus mempertahankan status, updated_at, pickup_date, dan jumlah payment. Untuk schedule electronic false, response SAFETY_CHECK_REQUIRED hanya jika state masih pending; state lain mengembalikan INVALID_STATE_TRANSITION.

## 11. Kasus tambahan untuk kontrak yang ketat

- JSON duplicate key, case field berbeda, unknown field, scalar/array/null, dan trailing object ditolak 400.
- Content-Type application/json dengan charset UTF-8 diterima; body nonkosong pada GET/DELETE ditolak.
- Complete/cancel dengan body kosong diterima; null atau object berfield ditolak.
- Query kosong, query duplicate, leading/trailing whitespace, dan overflow offset ditolak.
- UUID uppercase diterima dan output lowercase; UUID tanpa hyphen/nil ditolak.
- safety_check=null pada schedule ditolak, bukan dianggap omitted.
- Method salah pada route dikenal mengembalikan 405 dan Allow; route tidak dikenal 404.
- Dimensi melebihi 10000 piksel pada salah satu sisi atau 20 juta piksel total ditolak 400 sebelum decode penuh.
- Unsupported proof tidak lolos walaupun filename .jpg; truncated PNG/JPEG harus gagal pemeriksaan isi.
- GET/HEAD proof hanya melayani filename yang valid; path traversal, staging path, dan directory root tidak terbuka.
- POST payment existing tidak mengubah updated_at; timestamp completion/confirmation sesuai satu clock yang digunakan.
- Aggregate nominal besar tidak overflow atau melewati float64.

## 12. Cara membuktikan rollback

Siapkan pickup scheduled, lalu paksa repository insert payment gagal di jalur transaksi. Pada integration test, kegagalan dapat dibuat lewat constraint/trigger sementara yang hanya hidup dalam schema test atau test adapter yang mengembalikan error pada query insert. Sesudah request gagal, baca ulang menggunakan koneksi lain: status tetap scheduled dan tidak ada invoice baru.

Mock yang hanya memastikan Rollback dipanggil tetap berguna untuk unit test, tetapi tidak menggantikan pembacaan state akhir pada PostgreSQL. Jangan menyisipkan flag kegagalan melalui endpoint publik aplikasi.

Untuk upload, buat storage gagal sebelum rename dan DB gagal setelah rename secara terpisah. Pastikan cleanup tidak menghapus file request lain. Uji jalur commit tidak pasti secara terkontrol; jika sulit dipicu pada network nyata, gunakan fault injection pada adapter commit dan buktikan bahwa file final dipertahankan sampai status DB dapat diperiksa.

## 13. Traceability hasil

| Bagian | Requirement | Bukti minimum |
| --- | --- | --- |
| Household | FR-H01–FR-H04 | HTTP success/error, pagination, FK delete |
| Pickup | FR-W01–FR-W05, BR01–BR04 | State matrix, safety, tariff, transaksi, race |
| Payment | FR-P01–FR-P03, BR05 | Manual ensure, filter, file valid/invalid, duplicate confirmation |
| Reports | FR-R01–FR-R02 | Empty buckets, seed counts, revenue setelah paid |
| Runtime | NFR01–NFR09 | Docker startup, migration, persistence, DB outage, error sanitization |
| Tambahan | Graceful shutdown, date filter, unit test | SIGTERM, batas UTC, hasil test runner |

Pada handoff akhir tulis command yang dijalankan, ringkasan hasilnya, dan blocker jika ada. Saya tidak membutuhkan daftar panjang test yang dibacakan ulang, tetapi harus jelas mana yang benar-benar sudah diverifikasi. Coverage boleh dilaporkan bila diukur; jangan menjadikannya pengganti pembuktian aturan bisnis.
