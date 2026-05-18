'use client';

import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';

// Halaman publik untuk user yang lupa password.
// Backend selalu return generic message (anti user enumeration).
export default function ForgotPasswordPage() {
  const [username, setUsername] = useState('');
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.resetPasswordRequest(username.trim());
      setSuccess(true);
    } catch (e) {
      if (e instanceof ApiClientError) {
        if (e.status === 429) {
          setError('Terlalu banyak permintaan. Coba lagi beberapa saat.');
        } else if (e.code === 'LDAP_UNAVAILABLE') {
          setError('Server tidak tersedia. Coba lagi nanti.');
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
    <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4 py-12">
      <div className="w-full max-w-md">
        <div className="card">
          <h1 className="text-xl font-semibold text-slate-900">Lupa Password</h1>
          <p className="mt-1 text-sm text-slate-500">
            Masukkan username Anda. Jika terdaftar, kami akan mengirim
            <em> temporary password</em> ke email Anda.
          </p>

          {success ? (
            <div className="mt-6 space-y-4">
              <Alert tone="success">
                Jika username terdaftar dan punya email, instruksi reset password
                sudah dikirim. Silakan cek inbox Anda (juga folder spam).
              </Alert>
              <Link href="/login" className="btn-primary w-full">
                Kembali ke Login
              </Link>
            </div>
          ) : (
            <form onSubmit={onSubmit} className="mt-6 space-y-4">
              <div>
                <label htmlFor="username" className="label">Username</label>
                <input
                  id="username"
                  type="text"
                  required
                  maxLength={64}
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="input"
                  placeholder="contoh: john.doe"
                  disabled={busy}
                />
              </div>

              {error && <Alert tone="error">{error}</Alert>}

              <button type="submit" className="btn-primary w-full" disabled={busy}>
                {busy ? 'Mengirim...' : 'Kirim Instruksi Reset'}
              </button>

              <div className="text-center text-sm">
                <Link href="/login" className="text-brand-700 hover:text-brand-900 hover:underline">
                  Kembali ke Login
                </Link>
              </div>
            </form>
          )}
        </div>
      </div>
    </main>
  );
}
