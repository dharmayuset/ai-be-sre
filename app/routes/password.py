"""
Endpoint API untuk reset password self-service.

Untuk menambah endpoint baru di file ini:
1. Buat schema baru di app/schemas/
2. Tambahkan @router.post(...) atau @router.get(...) di sini
3. Otomatis muncul di Swagger UI di /docs
"""

import logging

from fastapi import APIRouter, HTTPException, status

from app.config import settings
from app.schemas.password import ResetPasswordRequest, ResetPasswordResponse
from app.services.email_service import EmailError, EmailService
from app.services.ldap_service import LDAPError, LDAPService, UserNotFoundError
from app.utils.password import generate_temporary_password

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/password", tags=["password"])


def _mask_email(email: str) -> str:
    """Mask email user, e.g. 'john.doe@example.com' -> 'jo***@example.com'."""
    try:
        local, domain = email.split("@", 1)
        if len(local) <= 2:
            masked_local = local[0] + "***"
        else:
            masked_local = local[:2] + "***"
        return f"{masked_local}@{domain}"
    except ValueError:
        return "***"


@router.post(
    "/reset",
    response_model=ResetPasswordResponse,
    status_code=status.HTTP_200_OK,
    summary="Self-service reset password",
    description=(
        "Generate temporary password baru, set ke akun FreeIPA user, "
        "lalu kirim ke email user. User wajib mengganti password "
        "saat login pertama kali."
    ),
)
def reset_password(payload: ResetPasswordRequest) -> ResetPasswordResponse:
    ldap = LDAPService()
    email_service = EmailService()

    username = payload.username.strip()

    # 1) Generate password temporary
    temp_password = generate_temporary_password(settings.TEMP_PASSWORD_LENGTH)

    # 2) Reset di FreeIPA (sekaligus dapatkan email user)
    try:
        user_email = ldap.reset_password(username, temp_password)
    except UserNotFoundError:
        # SECURITY NOTE:
        # Beberapa organisasi pilih return 200 generik supaya tidak
        # membocorkan apakah username valid atau tidak (anti user enumeration).
        # Untuk pemula, kita pakai 404 yang lebih jelas. Ganti sesuai kebutuhan.
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="User tidak ditemukan",
        )
    except LDAPError as e:
        logger.error("LDAP error: %s", e)
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail="Gagal berkomunikasi dengan server LDAP",
        )

    # 3) Kirim password ke email user
    try:
        email_service.send_temporary_password(user_email, username, temp_password)
    except EmailError as e:
        # NOTE: password sudah ter-reset di LDAP. User tetap bisa diberi
        # tahu admin untuk dapat password manual, atau retry endpoint ini.
        logger.error("Email error: %s", e)
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail="Password sudah di-reset, tapi gagal kirim email. "
                   "Silakan hubungi admin IT.",
        )

    return ResetPasswordResponse(
        message=(
            "Temporary password sudah dikirim ke email Anda. "
            "Silakan login dan segera ganti password."
        ),
        masked_email=_mask_email(user_email),
    )
