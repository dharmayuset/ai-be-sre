# FreeIPA Self-Service Password Reset

Backend sederhana berbasis **FastAPI** + **ldap3** untuk self-service password reset
akun FreeIPA / LDAP. Saat user request reset, sistem akan:

1. Generate **temporary password** acak yang aman.
2. Reset password user di FreeIPA via service account.
3. Kirim temporary password ke email user.
4. FreeIPA otomatis menandai password sebagai expired, sehingga user
   **wajib mengganti password** saat login berikutnya.

---

## Arsitektur

```
app/
├── main.py              # Entry point FastAPI, daftarkan router
├── config.py            # Load environment variable
├── routes/
│   └── password.py      # Endpoint POST /api/password/reset
├── services/
│   ├── ldap_service.py  # Komunikasi dengan FreeIPA via LDAP
│   └── email_service.py # Kirim email via SMTP
├── schemas/
│   └── password.py      # Pydantic models (request/response)
└── utils/
    └── password.py      # Generator password random
```

Prinsip pembagiannya:
- **routes/** — handle HTTP (request/response, status code).
- **services/** — logic bisnis & komunikasi dengan sistem eksternal.
- **schemas/** — validasi data (Pydantic).
- **utils/** — fungsi pembantu yang reusable.

---

## Persiapan FreeIPA

Anda butuh akun di FreeIPA yang punya hak **reset password user lain**.
Ada 2 opsi:

### Opsi A — Pakai akun admin (cepat, tidak direkomendasikan untuk produksi)
Langsung pakai `admin` di `LDAP_BIND_DN` dan password admin di `LDAP_BIND_PASSWORD`.

### Opsi B — Buat service account khusus (recommended)
1. Login sebagai admin ke web UI FreeIPA.
2. Buat user baru, misal `pwreset-svc`.
3. Buat **role** baru (Identity → Roles), misal `Password Reset Service`.
4. Tambahkan **privilege** `User Administrators` (atau yang lebih spesifik
   seperti `Modify Users password`).
5. Assign role tersebut ke user `pwreset-svc`.

Lalu di `.env`:
```
LDAP_BIND_DN=uid=pwreset-svc,cn=users,cn=accounts,dc=example,dc=com
LDAP_BIND_PASSWORD=<password-service-account>
```

---

## Setup Lokal

### 1. Buat virtual environment & install dependencies
```bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

### 2. Salin & isi file environment
```bash
cp .env.example .env
# edit .env, isi semua config sesuai environment Anda
```

### 3. Jalankan server
```bash
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

### 4. Test endpoint
Buka Swagger UI: <http://localhost:8000/docs>

Atau via `curl`:
```bash
curl -X POST http://localhost:8000/api/password/reset \
  -H "Content-Type: application/json" \
  -d '{"username": "john.doe"}'
```

Response sukses:
```json
{
  "message": "Temporary password sudah dikirim ke email Anda. Silakan login dan segera ganti password.",
  "masked_email": "jo***@example.com"
}
```

---

## Cara Menambah Fitur Baru

Aplikasi ini dirancang agar mudah di-extend. Contoh skenario:

### Tambah endpoint baru (misal: cek status akun)
1. Buat schema di `app/schemas/account.py`.
2. Buat route di `app/routes/account.py`:
   ```python
   from fastapi import APIRouter
   router = APIRouter(prefix="/api/account", tags=["account"])

   @router.get("/status/{username}")
   def status(username: str): ...
   ```
3. Daftarkan di `app/main.py`:
   ```python
   from app.routes import account as account_routes
   app.include_router(account_routes.router)
   ```

### Tambah operasi LDAP baru (misal: unlock akun)
Tambahkan method di `LDAPService` class (`app/services/ldap_service.py`).

### Ganti template email
Edit method `send_temporary_password` di `app/services/email_service.py`.

---

## Saran Hardening untuk Produksi

Aplikasi ini sengaja dibuat sederhana agar mudah dipelajari. Sebelum
deploy ke produksi, pertimbangkan:

- **Rate limiting** — cegah abuse / brute force user enumeration
  (pakai `slowapi` atau reverse proxy seperti Nginx).
- **CAPTCHA** — supaya tidak bisa di-script.
- **Audit log** — catat siapa minta reset, kapan, IP-nya.
- **MFA tambahan** — misal kirim OTP ke email/SMS sebelum reset.
- **Anti user enumeration** — selalu return response generik "jika user
  ada, email akan dikirim" (lihat catatan di `app/routes/password.py`).
- **HTTPS** — selalu jalankan di belakang reverse proxy dengan TLS.
- **LDAP TLS** — set `LDAP_USE_TLS=true` dan `LDAP_VERIFY_CERT=true`.
- **Secret management** — jangan commit `.env`, pakai vault / k8s secrets.

---

## Struktur Endpoint Saat Ini

| Method | Path                    | Deskripsi                       |
|--------|-------------------------|---------------------------------|
| GET    | `/health`               | Health check                    |
| POST   | `/api/password/reset`   | Reset password & kirim email    |
| GET    | `/docs`                 | Swagger UI (auto-generated)     |
| GET    | `/redoc`                | ReDoc UI (auto-generated)       |
