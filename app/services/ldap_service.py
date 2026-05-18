"""
LDAP Service: berkomunikasi dengan FreeIPA via protokol LDAP.

Alur reset password di FreeIPA:
1. Bind ke LDAP pakai service account (yang punya hak reset password).
2. Cari user berdasarkan username (uid).
3. Ambil email user dari atribut 'mail'.
4. Ganti password user dengan operasi LDAP modify.
5. FreeIPA otomatis menandai password sebagai EXPIRED ketika di-reset
   oleh akun lain (bukan oleh user itu sendiri). Jadi user WAJIB
   ganti password saat login pertama kali. Inilah "temporary password".
"""

import logging
from typing import Optional

from ldap3 import Connection, Server, Tls, ALL, MODIFY_REPLACE
from ldap3.core.exceptions import LDAPException
import ssl

from app.config import settings

logger = logging.getLogger(__name__)


class LDAPError(Exception):
    """Custom exception agar mudah di-handle di route layer."""


class UserNotFoundError(LDAPError):
    """User tidak ditemukan di FreeIPA."""


class LDAPService:
    """
    Wrapper sederhana untuk operasi LDAP yang kita butuhkan.

    Cara pakai:
        ldap = LDAPService()
        email = ldap.get_user_email("john.doe")
        ldap.reset_password("john.doe", "TempPass123!")
    """

    def __init__(self) -> None:
        self.server_url = settings.LDAP_SERVER_URL
        self.bind_dn = settings.LDAP_BIND_DN
        self.bind_password = settings.LDAP_BIND_PASSWORD
        self.user_base_dn = settings.LDAP_USER_BASE_DN
        self.use_tls = settings.LDAP_USE_TLS
        self.verify_cert = settings.LDAP_VERIFY_CERT

    # ---------- internal helpers ----------

    def _build_server(self) -> Server:
        """Buat objek Server ldap3 dengan opsi TLS sesuai config."""
        tls_config = None
        if self.use_tls:
            tls_config = Tls(
                validate=ssl.CERT_REQUIRED if self.verify_cert else ssl.CERT_NONE
            )
        return Server(self.server_url, use_ssl=self.use_tls, tls=tls_config, get_info=ALL)

    def _connect(self) -> Connection:
        """
        Buka koneksi ke FreeIPA dan bind sebagai service account.
        Return: Connection yang sudah authenticated.
        """
        try:
            server = self._build_server()
            conn = Connection(
                server,
                user=self.bind_dn,
                password=self.bind_password,
                auto_bind=True,
            )
            return conn
        except LDAPException as e:
            logger.exception("Gagal connect / bind ke LDAP")
            raise LDAPError(f"Tidak bisa connect ke LDAP server: {e}") from e

    def _find_user_dn_and_email(
        self, conn: Connection, username: str
    ) -> tuple[str, Optional[str]]:
        """
        Cari user berdasarkan uid. Return (user_dn, email).
        Raise UserNotFoundError kalau tidak ketemu.
        """
        # Pakai escape untuk safety (cegah LDAP injection)
        from ldap3.utils.conv import escape_filter_chars

        safe_username = escape_filter_chars(username)
        search_filter = f"(uid={safe_username})"

        conn.search(
            search_base=self.user_base_dn,
            search_filter=search_filter,
            attributes=["mail"],
        )

        if not conn.entries:
            raise UserNotFoundError(f"User '{username}' tidak ditemukan")

        entry = conn.entries[0]
        user_dn = entry.entry_dn
        email = str(entry.mail) if entry.mail else None
        return user_dn, email

    # ---------- public API ----------

    def get_user_email(self, username: str) -> str:
        """
        Ambil email user. Raise UserNotFoundError kalau user tidak ada,
        atau LDAPError kalau email tidak terdaftar.
        """
        with self._connect() as conn:
            _, email = self._find_user_dn_and_email(conn, username)
        if not email:
            raise LDAPError(f"User '{username}' tidak punya email terdaftar")
        return email

    def reset_password(self, username: str, new_password: str) -> str:
        """
        Reset password user di FreeIPA.

        Karena reset dilakukan oleh service account (bukan user sendiri),
        FreeIPA akan otomatis menandai password sebagai expired,
        sehingga user wajib ganti password saat login pertama.

        Return: email user (untuk dipakai mengirim notifikasi).
        """
        with self._connect() as conn:
            user_dn, email = self._find_user_dn_and_email(conn, username)

            if not email:
                raise LDAPError(f"User '{username}' tidak punya email terdaftar")

            success = conn.modify(
                user_dn,
                {"userPassword": [(MODIFY_REPLACE, [new_password])]},
            )

            if not success:
                logger.error("Gagal reset password: %s", conn.result)
                raise LDAPError(
                    f"Gagal reset password: {conn.result.get('description')}"
                )

            logger.info("Password berhasil di-reset untuk user '%s'", username)
            return email
