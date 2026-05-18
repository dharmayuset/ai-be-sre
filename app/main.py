"""
Entry point FastAPI.

Jalankan dengan:
    uvicorn app.main:app --reload

Lalu buka:
    http://localhost:8000/docs   -> Swagger UI (interactive)
    http://localhost:8000/health -> Health check

Cara menambah modul/endpoint baru:
    1. Buat file di app/routes/<nama>.py
    2. Definisikan `router = APIRouter(...)`
    3. Daftarkan dengan `app.include_router(...)` di bawah ini
"""

import logging

from fastapi import FastAPI

from app.config import settings
from app.routes import password as password_routes

# Setup logging dasar (silakan disesuaikan / pakai library yang lebih advanced)
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)

app = FastAPI(
    title=settings.APP_NAME,
    description="Backend self-service untuk reset password akun FreeIPA / LDAP.",
    version="0.1.0",
)


@app.get("/health", tags=["meta"])
def health_check() -> dict[str, str]:
    """Endpoint untuk health check (dipakai load balancer / monitoring)."""
    return {"status": "ok"}


# Daftarkan semua router di sini
app.include_router(password_routes.router)
