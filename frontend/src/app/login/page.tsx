'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { Alert } from '@/components/Alert';
import { PasswordInput } from '@/components/PasswordInput';
import { api, ApiClientError } from '@/lib/api';

// Allowlist of safe redirect targets to mitigate open-redirect.
function safeNext(next: string | null, fallback: string): string {
  if (!next) return fallback;
  // Only allow same-origin relative paths starting with "/"
  if (!next.startsWith('/') || next.startsWith('//')) return fallback;
  return next;
}

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const next = searchParams.get('next');

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const res = await api.login(username.trim(), password);
      // Tujuan default berdasarkan role
      const fallback = res.user.isAdmin ? '/admin' : '/user';
      router.push(safeNext(next, fallback));
      router.refresh();
    } catch (e) {
      if (e instanceof ApiClientError) {
        switch (e.code) {
          case 'INVALID_CREDENTIALS':
            setError('Username atau password salah.');
            break;
          case 'ACCOUNT_LOCKED':
            setError('Akun Anda terkunci. Hubungi administrator.');
            break;
          case 'LDAP_UNAVAILABLE':
            setError('Server otentikasi tidak tersedia. Coba lagi nanti.');
            break;
          default:
            setError(e.message || 'Login gagal.');
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
          <h1 className="text-xl font-semibold text-slate-900">FreeIPA Self-Service Portal</h1>
          <p className="mt-1 text-sm text-slate-500">
            Masuk dengan kredensial direktori Anda.
          </p>

          <form onSubmit={onSubmit} className="mt-6 space-y-4">
            <div>
              <label htmlFor="username" className="label">Username</label>
              <input
                id="username"
                name="username"
                type="text"
                autoComplete="username"
                required
                maxLength={64}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                placeholder="contoh: john.doe"
                disabled={busy}
              />
            </div>

            <PasswordInput
              id="password"
              name="password"
              label="Password"
              autoComplete="current-password"
              required
              maxLength={256}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={busy}
              placeholder="Masukkan password Anda"
            />

            {error && <Alert tone="error">{error}</Alert>}

            <button type="submit" className="btn-primary w-full" disabled={busy}>
              {busy ? 'Masuk...' : 'Masuk'}
            </button>
          </form>

          <div className="mt-6 border-t border-slate-200 pt-4 text-center text-sm">
            <Link
              href="/forgot-password"
              className="text-brand-700 hover:text-brand-900 hover:underline"
            >
              Lupa password?
            </Link>
            <span className="mx-2 text-slate-300">|</span>
            <Link
              href="/vpn-profile"
              className="text-brand-700 hover:text-brand-900 hover:underline"
            >
              Kirim VPN Profile
            </Link>
          </div>
        </div>

        <p className="mt-6 text-center text-xs text-slate-400">
          &copy; {new Date().getFullYear()} FreeIPA Self-Service Portal
        </p>
      </div>
    </main>
  );
}
