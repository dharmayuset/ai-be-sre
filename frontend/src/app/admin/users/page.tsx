'use client';

import { useEffect, useState, useCallback } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';
import type { User } from '@/types/api';

type StatusFilter = '' | 'active' | 'inactive';

export default function AdminUsersPage() {
  const [q, setQ] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('');
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [busyUser, setBusyUser] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [batchBusy, setBatchBusy] = useState(false);

  const load = useCallback(async (query?: string, status?: StatusFilter) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.adminListUsers({
        q: query ?? q,
        status: status ?? statusFilter,
      });
      setUsers(res.users);
      setSelected(new Set()); // clear selection on reload
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal memuat users');
    } finally {
      setLoading(false);
    }
  }, [q, statusFilter]);

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ---------- Actions ----------

  async function onResetPassword(username: string) {
    if (!confirm(`Reset password untuk "${username}"? User akan menerima temporary password via email.`)) return;
    setBusyUser(username);
    setActionMsg(null);
    setError(null);
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
    setError(null);
    try {
      await api.adminLockUser(u.username, !u.locked);
      setActionMsg(`Akun ${u.username} di-${action}.`);
      await load();
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : `Gagal ${action} user`);
    } finally {
      setBusyUser(null);
    }
  }

  async function onDeleteUser(username: string) {
    if (!confirm(`HAPUS PERMANEN akun "${username}"? Operasi ini TIDAK BISA di-undo!`)) return;
    setBusyUser(username);
    setActionMsg(null);
    setError(null);
    try {
      const res = await api.adminDeleteUser(username);
      setActionMsg(res.message);
      await load();
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal menghapus user');
    } finally {
      setBusyUser(null);
    }
  }

  async function onBatchDelete() {
    const count = selected.size;
    if (count === 0) return;
    if (!confirm(
      `HAPUS PERMANEN ${count} user yang dipilih?\n\n` +
      `Usernames: ${Array.from(selected).join(', ')}\n\n` +
      `Operasi ini TIDAK BISA di-undo!`
    )) return;
    setBatchBusy(true);
    setActionMsg(null);
    setError(null);
    try {
      const res = await api.adminBatchDeleteUsers(Array.from(selected));
      if (res.failCount === 0) {
        setActionMsg(`${res.successCount} user berhasil dihapus.`);
      } else {
        setActionMsg(
          `${res.successCount} berhasil, ${res.failCount} gagal. ` +
          `Gagal: ${res.results.filter(r => !r.success).map(r => `${r.username} (${r.error})`).join(', ')}`
        );
      }
      await load();
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal batch delete');
    } finally {
      setBatchBusy(false);
    }
  }

  function onExportCSV() {
    // Trigger download via direct navigation (cookie auth diperlukan)
    const url = api.adminExportUsersCSVUrl({ q, status: statusFilter });
    window.open(url, '_blank');
  }

  // ---------- Selection ----------

  function toggleSelect(username: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(username)) next.delete(username);
      else next.add(username);
      return next;
    });
  }

  function toggleSelectAll() {
    if (selected.size === users.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(users.map((u) => u.username)));
    }
  }

  // ---------- Filter submit ----------

  function onFilterSubmit(e: React.FormEvent) {
    e.preventDefault();
    load(q, statusFilter);
  }

  function onStatusChange(s: StatusFilter) {
    setStatusFilter(s);
    load(q, s);
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-slate-900">Users</h1>
        <p className="mt-1 text-sm text-slate-500">
          Kelola user FreeIPA: filter, reset password, lock/unlock, hapus, export CSV.
        </p>
      </header>

      {/* Search & Filter Bar */}
      <form className="flex flex-wrap gap-2" onSubmit={onFilterSubmit}>
        <input
          type="search"
          placeholder="Cari username / nama / email..."
          className="input flex-1 min-w-[200px]"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <select
          className="input w-auto"
          value={statusFilter}
          onChange={(e) => onStatusChange(e.target.value as StatusFilter)}
        >
          <option value="">Semua Status</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive / Locked</option>
        </select>
        <button type="submit" className="btn-primary" disabled={loading}>
          {loading ? 'Mencari...' : 'Cari'}
        </button>
      </form>

      {/* Action Bar */}
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="btn-secondary text-xs"
          onClick={onExportCSV}
          disabled={loading}
        >
          Export CSV
        </button>
        {selected.size > 0 && (
          <button
            type="button"
            className="btn-danger text-xs"
            onClick={onBatchDelete}
            disabled={batchBusy}
          >
            {batchBusy
              ? 'Menghapus...'
              : `Hapus ${selected.size} user terpilih`}
          </button>
        )}
        <span className="ml-auto text-xs text-slate-500">
          {users.length} user ditampilkan
          {selected.size > 0 && ` · ${selected.size} dipilih`}
        </span>
      </div>

      {error && <Alert tone="error">{error}</Alert>}
      {actionMsg && <Alert tone="success">{actionMsg}</Alert>}

      {/* Users Table */}
      <div className="card overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-3 py-3 w-10">
                <input
                  type="checkbox"
                  checked={users.length > 0 && selected.size === users.length}
                  onChange={toggleSelectAll}
                  className="rounded border-slate-300"
                  title="Pilih semua"
                />
              </th>
              <th className="px-3 py-3">Username</th>
              <th className="px-3 py-3">Nama</th>
              <th className="px-3 py-3">Email</th>
              <th className="px-3 py-3">Status</th>
              <th className="px-3 py-3 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {users.length === 0 && !loading && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-slate-500">
                  Tidak ada user yang cocok.
                </td>
              </tr>
            )}
            {users.map((u) => (
              <tr
                key={u.username}
                className={`border-t border-slate-100 ${
                  selected.has(u.username) ? 'bg-blue-50' : ''
                }`}
              >
                <td className="px-3 py-3">
                  <input
                    type="checkbox"
                    checked={selected.has(u.username)}
                    onChange={() => toggleSelect(u.username)}
                    className="rounded border-slate-300"
                  />
                </td>
                <td className="px-3 py-3 font-medium text-slate-900">{u.username}</td>
                <td className="px-3 py-3 text-slate-700">{u.displayName || '-'}</td>
                <td className="px-3 py-3 text-slate-700">{u.email || '-'}</td>
                <td className="px-3 py-3">
                  <div className="flex flex-wrap gap-1">
                    {u.isAdmin && (
                      <span className="badge bg-purple-100 text-purple-800">admin</span>
                    )}
                    {u.locked ? (
                      <span className="badge bg-red-100 text-red-800">locked</span>
                    ) : (
                      <span className="badge bg-green-100 text-green-800">active</span>
                    )}
                  </div>
                </td>
                <td className="px-3 py-3 text-right">
                  <div className="flex justify-end gap-1.5">
                    <button
                      type="button"
                      className="btn-secondary text-xs"
                      onClick={() => onResetPassword(u.username)}
                      disabled={busyUser === u.username || !u.email}
                      title={!u.email ? 'User belum punya email terdaftar' : 'Reset password'}
                    >
                      Reset
                    </button>
                    <button
                      type="button"
                      className={u.locked ? 'btn-primary text-xs' : 'btn-secondary text-xs'}
                      onClick={() => onToggleLock(u)}
                      disabled={busyUser === u.username}
                    >
                      {u.locked ? 'Unlock' : 'Lock'}
                    </button>
                    <button
                      type="button"
                      className="btn-danger text-xs"
                      onClick={() => onDeleteUser(u.username)}
                      disabled={busyUser === u.username}
                      title="Hapus permanen"
                    >
                      Hapus
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
