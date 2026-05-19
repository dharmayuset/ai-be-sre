'use client';

import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';

// Halaman publik (tanpa login) untuk request VPN profile via email.
// Backend akan cari user di Pritunl berdasarkan email, download profile,
// lalu kirim ke email tersebut.
export default function VPNProfilePage() {
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.sendVPNProfile(email.trim());
      setSuccess(true);
    } catch (e) {
      if (e instanceof ApiClientError) {
        if (e.status === 429) {
          setError('Terlalu banyak permintaan. Coba lagi beberapa saat.');
        } else if (e.code === 'VPN_UNAVAILABLE') {
          setError('Server VPN tidak tersedia. Coba lagi nanti.');
        } else if (e.code === 'PROFILE_DOWNLOAD_FAILED') {
          setError('Gagal mengambil VPN profile. Coba lagi nanti.');
        } else if (e.code === 'EMAIL_FAILED') {
          setError('VPN profile berhasil diambil tapi gagal kirim email. Coba lagi nanti.');
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
          <h1 className="text-xl font-semibold text-slate-900">Kirim VPN Profile</h1>
          <p className="mt-1 text-sm text-slate-500">
            Masukkan email Anda yang terdaftar di VPN. Kami akan mengirim
            file VPN profile ke email tersebut.
          </p>

          {success ? (
            <div className="mt-6 space-y-4">
              <Alert tone="success">
                Jika email terdaftar di sistem VPN, profile sudah dikirim.
                Silakan cek inbox Anda (juga folder spam).
              </Alert>
              <div className="space-y-2 rounded-md bg-slate-50 p-4 text-sm text-slate-600">
                <p className="font-medium text-slate-800">Cara pakai VPN profile:</p>
                <ol className="list-decimal list-inside space-y-1">
                  <li>Download dan extract file <code>.tar</code> dari email.</li>
                  <li>Import file <code>.ovpn</code> ke Pritunl Client / OpenVPN.</li>
                  <li>Connect menggunakan profil tersebut.</li>
                </ol>
              </div>
              <button
                type="button"
                className="btn-primary w-full"
                onClick={() => {
                  setSuccess(false);
                  setEmail('');
                }}
              >
                Kirim Lagi
              </button>
            </div>
          ) : (
            <form onSubmit={onSubmit} className="mt-6 space-y-4">
              <div>
                <label htmlFor="email" className="label">Email</label>
                <input
                  id="email"
                  type="email"
                  required
                  maxLength={256}
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="input"
                  placeholder="contoh: john.doe@company.com"
                  disabled={busy}
                />
                <p className="mt-1 text-xs text-slate-500">
                  Gunakan email yang sama dengan yang terdaftar di Pritunl VPN.
                </p>
              </div>

              {error && <Alert tone="error">{error}</Alert>}

              <button type="submit" className="btn-primary w-full" disabled={busy || !email.trim()}>
                {busy ? 'Mengirim...' : 'Kirim VPN Profile ke Email'}
              </button>

              <div className="text-center text-sm">
                <Link href="/login" className="text-brand-700 hover:text-brand-900 hover:underline">
                  Kembali ke Login
                </Link>
              </div>
            </form>
          )}
        </div>

        <p className="mt-6 text-center text-xs text-slate-400">
          &copy; {new Date().getFullYear()} FreeIPA Self-Service Portal
        </p>
      </div>
    </main>
  );
}
