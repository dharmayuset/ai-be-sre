"""
Konfigurasi aplikasi.

Semua setting dibaca dari file .env (lihat .env.example untuk template).
Gunakan `from app.config import settings` di file lain untuk mengakses config.
"""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Semua environment variable terpusat di sini."""

    # ---- LDAP / FreeIPA ----
    LDAP_SERVER_URL: str
    LDAP_USER_BASE_DN: str
    LDAP_BIND_DN: str
    LDAP_BIND_PASSWORD: str
    LDAP_USE_TLS: bool = True
    LDAP_VERIFY_CERT: bool = True

    # ---- SMTP / Email ----
    SMTP_HOST: str
    SMTP_PORT: int = 587
    SMTP_USERNAME: str
    SMTP_PASSWORD: str
    SMTP_USE_TLS: bool = True
    SMTP_FROM_EMAIL: str
    SMTP_FROM_NAME: str = "IT Support"

    # ---- App ----
    APP_NAME: str = "FreeIPA Self-Service Password Reset"
    APP_HOST: str = "0.0.0.0"
    APP_PORT: int = 8000
    TEMP_PASSWORD_LENGTH: int = 16

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=True,
    )


# Singleton instance: import `settings` di mana saja
settings = Settings()
