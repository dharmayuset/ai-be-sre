"""
Password generator untuk temporary password.

Dijamin mengandung:
- minimal 1 huruf besar
- minimal 1 huruf kecil
- minimal 1 angka
- minimal 1 karakter spesial

Sehingga lulus password policy default FreeIPA.
"""

import secrets
import string


def generate_temporary_password(length: int = 16) -> str:
    """
    Generate password random yang aman secara kriptografis.

    Pakai `secrets` (bukan `random`) karena cocok untuk hal sensitif
    seperti password / token.
    """
    if length < 8:
        raise ValueError("Panjang password minimal 8 karakter")

    upper = string.ascii_uppercase
    lower = string.ascii_lowercase
    digits = string.digits
    # Hindari karakter yang ambigu / bermasalah di shell, email, dsb.
    specials = "!@#$%^&*-_=+"

    # Pastikan minimal 1 karakter dari setiap kategori
    required = [
        secrets.choice(upper),
        secrets.choice(lower),
        secrets.choice(digits),
        secrets.choice(specials),
    ]

    # Sisanya random dari semua kategori
    all_chars = upper + lower + digits + specials
    remaining = [secrets.choice(all_chars) for _ in range(length - len(required))]

    # Acak posisinya supaya karakter wajib tidak selalu di awal
    password_chars = required + remaining
    secrets.SystemRandom().shuffle(password_chars)

    return "".join(password_chars)
