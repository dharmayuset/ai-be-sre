'use client';

// User yang sudah login juga boleh "request reset password" — tapi
// biasanya skenarionya: user tahu password lama, lebih disarankan pakai
// "Ganti Password". Di sini kita kirim reset ke email user yang login.

import Link from 'next/link';
import { useEffect, useState, type FormEvent } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';
import type { User } from '@/types/api';

export default function ResetPasswordSelfPage() {
  const [me, setMe] = useState<User | null>(null);
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmStep, setConfirmStep] = useState(false);

  useEffect(() => {
    api.me().then((r) => setMe(r.user)).catch(() => setMe(null));
  }, []);

  async function onConfirm(e: FormEvent) {
    e.preventDefault();
    if (!me) return;
    setBusy(true);
    setError(null);
    try {
      await api.resetPasswordRequest(me.username);
      setSuccess(true);
    } catch (e) {
      if (e instanceof ApiClientError) {
        if (e.status === 429) {
          setError('Terlalu banyak permintaan. Coba lagi beberapa saat.');
        } else {
          setError(e.message || 'Permintaan gagal.');
        }
      } else {
        setError('Terjadi kesalahan tak terduga.');
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <Link href="/user" className="text-sm text-brand-700 hover:underline">
        &larr; Kembali
      </Link>

      <div className="card">
        <h1 className="text-xl font-semibold text-slate-900">Reset Password</h1>
        <p className="mt-1 text-sm text-slate-500">
          Kami akan mengirim <em>temporary password</em> ke email Anda yang
          terdaftar di FreeIPA. Anda akan diminta mengganti password saat
          login berikutnya.
        </p>

        {success ? (
          <div className="mt-6 space-y-4">
            <Alert tone="success">
              Jika username terdaftar dan punya email, instruksi reset
              password sudah dikirim. Cek inbox (juga folder spam).
              <br />
              Setelah login dengan password sementara, Anda harus mengganti
              password.
            </Alert>
            <Link href="/user" className="btn-primary inline-block">
              Selesai
            </Link>
          </div>
        ) : me ? (
          <form onSubmit={onConfirm} className="mt-6 space-y-4">
            <div className="rounded-md bg-slate-50 p-3 text-sm">
              <div className="text-slate-500">Akun:</div>
              <div className="font-medium">{me.username}</div>
              <div className="text-slate-500 mt-1">Email tujuan:</div>
              <div className="font-medium">{me.email || '(tidak terdaftar)'}</div>
            </div>

            {!me.email && (
              <Alert tone="warning">
                Anda belum punya email terdaftar. Hubungi administrator
                untuk menambahkan email akun Anda.
              </Alert>
            )}

            {!confirmStep ? (
              <button
                type="button"
                className="btn-primary w-full"
                onClick={() => setConfirmStep(true)}
                disabled={!me.email}
              >
                Lanjutkan
              </button>
            ) : (
              <>
                <Alert tone="warning">
                  Konfirmasi: setelah klik tombol di bawah, password lama Anda
                  akan langsung tidak berlaku. Anda harus login ulang dengan
                  temporary password yang dikirim ke email.
                </Alert>
                {error && <Alert tone="error">{error}</Alert>}
                <div className="flex gap-2">
                  <button
                    type="button"
                    className="btn-secondary flex-1"
                    onClick={() => setConfirmStep(false)}
                    disabled={busy}
                  >
                    Batal
                  </button>
                  <button type="submit" className="btn-danger flex-1" disabled={busy}>
                    {busy ? 'Mengirim...' : 'Ya, Reset & Kirim Email'}
                  </button>
                </div>
              </>
            )}
          </form>
        ) : (
          <div className="mt-6 text-sm text-slate-500">Memuat...</div>
        )}
      </div>
    </div>
  );
}
