# FreeIPA Self-Service Portal

Self-service portal untuk akun FreeIPA / LDAP, dengan **dashboard admin**
dan UI **user** yang fokus pada 2 fungsi: **Ganti Password** & **Reset Password**.

- **Backend**: Go 1.25 + chi router + ldap/v3 + JWT + SQLite (audit log).
- **Frontend**: Next.js 14 (App Router) + TypeScript + Tailwind.
- **Email**: SMTP relay (mendukung relay tanpa auth / STARTTLS / SMTPS).

---

## Fitur

### User
- **Login** dengan kredensial FreeIPA.
- **Ganti Password** — masukkan password lama, set password baru.
- **Reset Password** — kirim *temporary password* ke email user; setelah
  reset, FreeIPA otomatis menandai password expired sehingga user wajib
  ganti password saat login berikutnya.
- **Lupa Password** (publik, tanpa login).

### Admin (user yang anggota grup `LDAP_ADMIN_GROUP`)
- **Dashboard** — statistik event 24 jam terakhir, total sukses/gagal.
- **Users Management** — cari user, reset password user, lock/unlock akun.
- **Audit Log** — semua aktivitas sensitif tercatat dengan filter.

---

## Arsitektur

```
ai-be-sre/
├── backend/                    # Go API
│   ├── cmd/server/             # Entry point
│   ├── internal/
│   │   ├── config/             # Load env, validasi
│   │   ├── ldap/               # FreeIPA client
│   │   ├── email/              # SMTP relay sender
│   │   ├── audit/              # SQLite audit log
│   │   ├── auth/               # JWT manager + cookies
│   │   ├── middleware/         # Auth, role, security, logging
│   │   ├── handlers/           # HTTP handlers
│   │   ├── models/             # Domain types
│   │   └── utils/              # Password gen, response helpers
│   ├── Dockerfile
│   ├── Makefile
│   └── .env.example
│
├── frontend/                   # Next.js UI
│   ├── src/
│   │   ├── app/
│   │   │   ├── login/          # /login
│   │   │   ├── forgot-password/
│   │   │   ├── user/           # User area (2 menu)
│   │   │   └── admin/          # Admin dashboard, users, audit
│   │   ├── components/         # Alert, Header, PasswordInput, ...
│   │   ├── lib/                # api client, server-side helpers
│   │   ├── types/              # Tipe data API
│   │   └── middleware.ts       # Edge auth check
│   └── Dockerfile
│
└── docker-compose.yml
```

---

## Persiapan FreeIPA

### 1. Buat service account khusus (recommended)
Daripada pakai `admin`, buat akun terpisah supaya mudah dicabut & di-audit:

1. Login ke FreeIPA Web UI sebagai `admin`.
2. Identity → Users → Add user, mis. `pwreset-svc`.
3. Identity → Roles → Add role: `Password Reset Service`.
4. Tambah privilege ke role tersebut:
   - `Modify Users password` — untuk reset password user.
   - `Modify Users` — untuk lock/unlock akun.
5. Assign role ke user `pwreset-svc`.
6. Catat DN-nya: `uid=pwreset-svc,cn=users,cn=accounts,dc=...`

### 2. Tentukan grup admin app
Buat grup baru atau pakai yang sudah ada (mis. `admins`). Setiap user di
grup ini akan otomatis dapat akses ke `/admin/...` di portal.

```
LDAP_ADMIN_GROUP=admins
```

---

## SMTP Relay

App ini didesain untuk **SMTP relay** internal (skenario yang Anda
sebutkan). Mendukung 3 mode:

| Mode | Port | Setting |
|---|---|---|
| Plain (relay internal trust by IP) | 25 | `SMTP_USE_STARTTLS=false`, `SMTP_USE_TLS=false` |
| STARTTLS | 587 | `SMTP_USE_STARTTLS=true`, `SMTP_USE_TLS=false` |
| Implicit TLS / SMTPS | 465 | `SMTP_USE_STARTTLS=false`, `SMTP_USE_TLS=true` |

Auth opsional — kosongkan `SMTP_USERNAME`/`SMTP_PASSWORD` kalau relay
trust by IP.

---

## Setup Lokal (tanpa Docker)

### Prerequisite
- **Go 1.25+** (untuk backend)
- **Node.js 22+** (untuk frontend)
- C compiler (gcc / clang) — untuk go-sqlite3 (cgo)

### Backend
```bash
cd backend
cp .env.example .env
# Edit .env, isi LDAP_*, SMTP_*, dan JWT_SECRET (generate: openssl rand -base64 64)

go mod tidy
go run ./cmd/server
# Server up di http://localhost:8080
```

Test:
```bash
curl http://localhost:8080/health
```

### Frontend
```bash
cd frontend
cp .env.example .env
# BACKEND_URL=http://localhost:8080

npm install
npm run dev
# UI up di http://localhost:3000
```

Buka <http://localhost:3000/login>, login dengan kredensial FreeIPA.

---

## Setup dengan Docker Compose

```bash
# Siapkan env
cp backend/.env.example backend/.env
# Edit backend/.env

# Build & run
docker compose up --build
# Backend: http://localhost:8080
# Frontend: http://localhost:3000
```

Audit log SQLite di-persist di volume `backend-data`.

---

## Endpoint API

Base path: `/api/v1`

### Public
| Method | Path | Deskripsi |
|---|---|---|
| POST | `/auth/login` | Login (rate limit 10/min/IP) |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/password/reset-request` | Request reset password (rate limit 5/min/IP, anti enumeration) |

### Authenticated
| Method | Path | Deskripsi |
|---|---|---|
| POST | `/auth/logout` | Logout, clear cookies |
| GET | `/auth/me` | Info user yang login |
| POST | `/password/change` | Ganti password sendiri (perlu password lama) |

### Admin only
| Method | Path | Deskripsi |
|---|---|---|
| GET | `/admin/stats` | Statistik dashboard |
| GET | `/admin/users` | List users (`?q=` untuk search) |
| GET | `/admin/users/{username}` | Detail user |
| POST | `/admin/users/{username}/reset-password` | Reset password user |
| POST | `/admin/users/{username}/lock` | Body `{"lock":true/false}` |
| GET | `/admin/audit` | Audit log dengan filter |

---

## Security Considerations

Aplikasi ini sudah mengimplementasikan banyak best practice keamanan:

| Aspek | Implementasi |
|---|---|
| **Auth** | Bind LDAP (verifikasi password). JWT HS256 dengan claims di-validate WithValidMethods (anti alg-confusion). |
| **Cookies** | HttpOnly + Secure (di prod) + SameSite=Lax — anti XSS & CSRF dasar. |
| **Refresh** | Re-fetch user dari LDAP setiap refresh — perubahan grup langsung berlaku. |
| **Rate limit** | Per-IP: login 10/min, reset 5/min, API umum 60/min. |
| **Anti enumeration** | Reset password selalu return generic message. |
| **LDAP injection** | Semua input di-escape dengan `ldap.EscapeFilter` & `ldap.EscapeDN`. |
| **Body size** | `MaxBytesReader` 1MB, `DisallowUnknownFields`. |
| **Security headers** | X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, CSP, HSTS (prod). |
| **Password generator** | `crypto/rand`, min 12 char, 4 kategori karakter. |
| **Password policy** | Server-side wajib (min 12 char + LDAP policy FreeIPA), client-side preview. |
| **Audit trail** | Setiap aksi sensitif (sukses + gagal) tercatat di SQLite dengan IP, UA, actor, target. |
| **Logout** | Clear kedua cookies. |
| **Self-protection** | Admin tidak bisa lock akun sendiri. |
| **Email notifikasi** | User dapat email saat password berubah (alert security). |
| **Email masking** | Response API mask email user lain (anti info leak). |
| **Open redirect** | Login `?next=` hanya boleh path relative. |
| **TLS** | LDAPS + STARTTLS/SMTPS. Bisa di-disable verify hanya untuk dev. |
| **Graceful shutdown** | SIGTERM → 15s drain. |
| **Panic safety** | Recoverer middleware. |
| **Timeouts** | ReadHeaderTimeout 10s, ReadTimeout 30s, WriteTimeout 60s, request 30s. |

### Yang masih perlu Anda pertimbangkan untuk produksi
- **Reverse proxy + HTTPS** (Nginx / Caddy / Traefik). Frontend dan
  backend wajib di belakang TLS termination.
- **CAPTCHA** di endpoint reset/login (mis. Cloudflare Turnstile, hCaptcha)
  untuk mencegah bot.
- **Token revocation list** kalau butuh logout server-side (sekarang
  refresh token tetap valid sampai expired).
- **Centralized logging** (ELK / Loki). Backend sudah pakai slog
  structured logging.
- **Backup audit DB** secara reguler.
- **Secret management** — jangan commit `.env`. Pakai HashiCorp Vault /
  AWS Secrets Manager / k8s Secrets.
- **Email DKIM/SPF/DMARC** dari relay supaya tidak masuk spam.
- **Network policy** — backend tidak perlu expose ke internet, cukup
  frontend yang public.

---

## Development Tips

### Menambah endpoint baru di backend
1. Tambah handler method di `backend/internal/handlers/`.
2. Daftarkan route di `backend/cmd/server/router.go`, pilih grup
   (public / authenticated / admin).
3. Untuk endpoint admin: pasang `middleware.RequireRole(models.RoleAdmin)`.

### Menambah halaman baru di frontend
1. Buat folder di `frontend/src/app/<area>/<page>/page.tsx`.
2. Untuk page yang butuh user info, pakai `getCurrentUser()` /
   `requireAdmin()` dari `lib/server.ts`.
3. Untuk fetch API, pakai helper di `lib/api.ts`.

### Generate JWT secret
```bash
openssl rand -base64 64
```

### Run tests backend
```bash
cd backend
go test -count=1 ./...
```

### Format & lint
```bash
# backend
cd backend && gofmt -w . && go vet ./...

# frontend
cd frontend && npm run typecheck && npm run lint
```

---

## Catatan tentang sandbox build
Repo ini disusun di sandbox terbatas yang **tidak punya akses internet**,
sehingga `go mod tidy` & `npm install` belum dijalankan. Sebelum run
pertama kali di environment Anda:

```bash
cd backend && go mod tidy && go build ./cmd/server
cd ../frontend && npm install && npm run build
```

Tools yang sudah diverifikasi di sandbox:
- Backend Go: `gofmt -w` PASS, unit test untuk utils PASS.
- Frontend TS: syntax check PASS (kecuali missing-module errors yang
  terselesaikan setelah `npm install`).

---

## Lisensi
Tentukan sesuai kebutuhan organisasi Anda.
