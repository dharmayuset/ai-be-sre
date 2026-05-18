"""
Pydantic schemas untuk request & response API password reset.
Pisahkan validasi data dari logic bisnis.
"""

from pydantic import BaseModel, Field


class ResetPasswordRequest(BaseModel):
    """Body request untuk POST /api/password/reset"""

    username: str = Field(
        ...,
        min_length=1,
        max_length=64,
        description="Username (uid) di FreeIPA",
        examples=["john.doe"],
    )


class ResetPasswordResponse(BaseModel):
    """Response sukses reset password.

    Email sengaja di-mask (contoh: j***@example.com) supaya tidak
    membocorkan email lengkap user lain ke siapa pun yang bisa POST.
    """

    message: str
    masked_email: str
