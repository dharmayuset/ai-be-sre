"""
Email Service: kirim email via SMTP.

Pakai `smtplib` dari standard library Python (tidak perlu dependency tambahan).
"""

import logging
import smtplib
from email.message import EmailMessage

from app.config import settings

logger = logging.getLogger(__name__)


class EmailError(Exception):
    """Custom exception untuk error pengiriman email."""


class EmailService:
    """
    Cara pakai:
        email = EmailService()
        email.send_temporary_password("user@example.com", "john.doe", "TempPass!23")
    """

    def __init__(self) -> None:
        self.host = settings.SMTP_HOST
        self.port = settings.SMTP_PORT
        self.username = settings.SMTP_USERNAME
        self.password = settings.SMTP_PASSWORD
        self.use_tls = settings.SMTP_USE_TLS
        self.from_email = settings.SMTP_FROM_EMAIL
        self.from_name = settings.SMTP_FROM_NAME

    # ---------- internal ----------

    def _send(self, msg: EmailMessage) -> None:
        """Generic SMTP send. Dipakai oleh semua method send_* di bawah."""
        try:
            with smtplib.SMTP(self.host, self.port, timeout=15) as smtp:
                smtp.ehlo()
                if self.use_tls:
                    smtp.starttls()
                    smtp.ehlo()
                if self.username and self.password:
                    smtp.login(self.username, self.password)
                smtp.send_message(msg)
            logger.info("Email berhasil dikirim ke %s", msg["To"])
        except Exception as e:
            logger.exception("Gagal mengirim email")
            raise EmailError(f"Gagal mengirim email: {e}") from e

    # ---------- public API ----------

    def send_temporary_password(
        self, to_email: str, username: str, temp_password: str
    ) -> None:
        """Kirim email berisi temporary password ke user."""
        msg = EmailMessage()
        msg["Subject"] = "Reset Password - Temporary Password Anda"
        msg["From"] = f"{self.from_name} <{self.from_email}>"
        msg["To"] = to_email

        # Plain text body (selalu sediakan fallback teks)
        text_body = (
            f"Halo {username},\n\n"
            f"Berikut adalah temporary password Anda:\n\n"
            f"    {temp_password}\n\n"
            f"PENTING:\n"
            f"- Password ini hanya untuk login pertama.\n"
            f"- Anda akan diminta mengganti password setelah login.\n"
            f"- Jangan bagikan password ini kepada siapa pun.\n\n"
            f"Jika Anda tidak meminta reset password ini, segera hubungi tim IT.\n\n"
            f"Salam,\n{self.from_name}\n"
        )
        msg.set_content(text_body)

        # HTML body (lebih enak dilihat)
        html_body = f"""\
        <html>
          <body style="font-family: Arial, sans-serif; color: #333;">
            <p>Halo <b>{username}</b>,</p>
            <p>Berikut adalah <b>temporary password</b> Anda:</p>
            <p style="font-size: 18px; background: #f4f4f4; padding: 12px;
                      border-radius: 6px; font-family: monospace;">
              {temp_password}
            </p>
            <p><b>PENTING:</b></p>
            <ul>
              <li>Password ini hanya untuk login pertama.</li>
              <li>Anda akan diminta mengganti password setelah login.</li>
              <li>Jangan bagikan password ini kepada siapa pun.</li>
            </ul>
            <p>Jika Anda tidak meminta reset password ini, segera hubungi tim IT.</p>
            <p>Salam,<br/>{self.from_name}</p>
          </body>
        </html>
        """
        msg.add_alternative(html_body, subtype="html")

        self._send(msg)
