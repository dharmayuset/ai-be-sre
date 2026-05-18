'use client';

import { useEffect, useState } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';
import type { User } from '@/types/api';

export default function AdminUsersPage() {
  const [q, setQ] = useState('');
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [busyUser, setBusyUser] = useState<string | null>(null);

  async function load(query?: string) {
    setLoading(true);
    setError(null);
    try {
      const res = await api.adminListUsers(query);
      setUsers(res.users);
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal memuat users');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function onResetPassword(username: string) {
    if (!confirm(`Reset password untuk "${username}"? User akan menerima temporary password via email.`)) return;
    setBusyUser(username);
    setActionMsg(null);
    try {
      const res = await api.adminResetPassword(username);
      setActionMsg(`${res.message} (email: ${res.maskedEmail})`);
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal reset password');
    } finally {
      setBusyUser(null);
    }
  }

  async function onToggleLock(u: User) {
    const action = u.locked ? 'unlock' : 'lock';
    if (!confirm(`${action.toUpperCase()} akun "${u.username}"?`)) return;
    setBusyUser(u.username);
    setActionMsg(null);
    try {
      await api.adminLockUser(u.username, !u.locked);
      setActionMsg(`Akun ${u.username} di-${action}.`);
      await load(q);
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : `Gagal ${action} user`);
    } finally {
      setBusyUser(null);
    }
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-slate-900">Users</h1>
        <p className="mt-1 text-sm text-slate-500">
          Kelola user FreeIPA: reset password atau lock/unlock akun.
        </p>
      </header>

      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          load(q);
        }}
      >
        <input
          type="search"
          placeholder="Cari username / nama / email..."
          className="input"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <button type="submit" className="btn-primary" disabled={loading}>
          {loading ? 'Mencari...' : 'Cari'}
        </button>
      </form>

      {error && <Alert tone="error">{error}</Alert>}
      {actionMsg && <Alert tone="success">{actionMsg}</Alert>}

      <div className="card overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-4 py-3">Username</th>
              <th className="px-4 py-3">Nama</th>
              <th className="px-4 py-3">Email</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {users.length === 0 && !loading && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-slate-500">
                  Tidak ada user yang cocok.
                </td>
              </tr>
            )}
            {users.map((u) => (
              <tr key={u.username} className="border-t border-slate-100">
                <td className="px-4 py-3 font-medium text-slate-900">{u.username}</td>
                <td className="px-4 py-3 text-slate-700">{u.displayName || '-'}</td>
                <td className="px-4 py-3 text-slate-700">{u.email || '-'}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {u.isAdmin && <span className="badge bg-purple-100 text-purple-800">admin</span>}
                    {u.locked ? (
                      <span className="badge bg-red-100 text-red-800">locked</span>
                    ) : (
                      <span className="badge bg-green-100 text-green-800">active</span>
                    )}
                  </div>
                </td>
                <td className="px-4 py-3 text-right">
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      className="btn-secondary text-xs"
                      onClick={() => onResetPassword(u.username)}
                      disabled={busyUser === u.username || !u.email}
                      title={!u.email ? 'User belum punya email terdaftar' : ''}
                    >
                      Reset Password
                    </button>
                    <button
                      type="button"
                      className={u.locked ? 'btn-primary text-xs' : 'btn-danger text-xs'}
                      onClick={() => onToggleLock(u)}
                      disabled={busyUser === u.username}
                    >
                      {u.locked ? 'Unlock' : 'Lock'}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
