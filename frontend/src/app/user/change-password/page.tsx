'use client';

import Link from 'next/link';
import { useState, type FormEvent } from 'react';
import { Alert } from '@/components/Alert';
import { PasswordInput } from '@/components/PasswordInput';
import { PasswordStrength } from '@/components/PasswordStrength';
import { api, ApiClientError } from '@/lib/api';
import { checkPassword } from '@/lib/password';

export default function ChangePasswordPage() {
  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const policy = checkPassword(newPwd);
  const matchesConfirm = confirm.length > 0 && newPwd === confirm;
  const canSubmit = oldPwd && policy.ok && matchesConfirm && oldPwd !== newPwd && !busy;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!policy.ok) return;
    if (newPwd !== confirm) {
      setError('Konfirmasi password tidak cocok.');
      return;
    }
    if (oldPwd === newPwd) {
      setError('Password baru tidak boleh sama dengan password lama.');
      return;
    }
    setBusy(true);
    try {
      await api.changePassword(oldPwd, newPwd);
      setSuccess(true);
      setOldPwd('');
      setNewPwd('');
      setConfirm('');
    } catch (e) {
      if (e instanceof ApiClientError) {
        switch (e.code) {
          case 'INVALID_OLD_PASSWORD':
            setError('Password lama yang Anda masukkan salah.');
            break;
          case 'WEAK_PASSWORD':
            setError('Password baru tidak memenuhi kebijakan keamanan FreeIPA.');
            break;
          case 'SAME_PASSWORD':
            setError('Password baru tidak boleh sama dengan password lama.');
            break;
          case 'ACCOUNT_LOCKED':
            setError('Akun Anda terkunci. Hubungi administrator.');
            break;
          case 'LDAP_UNAVAILABLE':
            setError('Server tidak tersedia. Coba lagi nanti.');
            break;
          default:
            setError(e.message || 'Gagal mengganti password.');
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
        <h1 className="text-xl font-semibold text-slate-900">Ganti Password</h1>
        <p className="mt-1 text-sm text-slate-500">
          Masukkan password lama dan password baru Anda.
        </p>

        {success ? (
          <div className="mt-6 space-y-4">
            <Alert tone="success">
              Password Anda berhasil diubah. Notifikasi telah dikirim ke email Anda.
            </Alert>
            <Link href="/user" className="btn-primary inline-block">
              Selesai
            </Link>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="mt-6 space-y-4" autoComplete="off">
            <PasswordInput
              id="old-password"
              label="Password Lama"
              required
              value={oldPwd}
              onChange={(e) => setOldPwd(e.target.value)}
              disabled={busy}
              autoComplete="current-password"
            />

            <div>
              <PasswordInput
                id="new-password"
                label="Password Baru"
                required
                value={newPwd}
                onChange={(e) => setNewPwd(e.target.value)}
                disabled={busy}
                autoComplete="new-password"
              />
              <PasswordStrength value={newPwd} />
            </div>

            <PasswordInput
              id="confirm-password"
              label="Konfirmasi Password Baru"
              required
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              disabled={busy}
              autoComplete="new-password"
              error={
                confirm.length > 0 && !matchesConfirm
                  ? 'Konfirmasi tidak cocok'
                  : undefined
              }
            />

            {error && <Alert tone="error">{error}</Alert>}

            <button type="submit" className="btn-primary w-full" disabled={!canSubmit}>
              {busy ? 'Mengubah...' : 'Ubah Password'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
