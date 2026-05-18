'use client';

import { useEffect, useState } from 'react';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';
import type { AuditEntry, AuditListResponse } from '@/types/api';

const ACTION_OPTIONS = [
  '', 'login', 'login_failed', 'logout', 'change_password',
  'reset_password', 'admin_reset_password', 'admin_lock_user', 'admin_unlock_user',
];

const PAGE_SIZE = 50;

export default function AuditLogPage() {
  const [items, setItems] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState({ actor: '', target: '', action: '', status: '' });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load(off = 0) {
    setLoading(true);
    setError(null);
    try {
      const res: AuditListResponse = await api.adminAudit({
        ...filters,
        limit: PAGE_SIZE,
        offset: off,
      });
      setItems(res.items);
      setTotal(res.total);
      setOffset(res.offset);
    } catch (e) {
      setError(e instanceof ApiClientError ? e.message : 'Gagal memuat audit log');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-slate-900">Audit Log</h1>
        <p className="mt-1 text-sm text-slate-500">
          Catatan semua aktivitas sensitif di sistem.
        </p>
      </header>

      <form
        className="grid gap-3 sm:grid-cols-5"
        onSubmit={(e) => {
          e.preventDefault();
          load(0);
        }}
      >
        <input
          className="input"
          placeholder="Actor"
          value={filters.actor}
          onChange={(e) => setFilters({ ...filters, actor: e.target.value })}
        />
        <input
          className="input"
          placeholder="Target"
          value={filters.target}
          onChange={(e) => setFilters({ ...filters, target: e.target.value })}
        />
        <select
          className="input"
          value={filters.action}
          onChange={(e) => setFilters({ ...filters, action: e.target.value })}
        >
          {ACTION_OPTIONS.map((a) => (
            <option key={a} value={a}>{a || 'Semua aksi'}</option>
          ))}
        </select>
        <select
          className="input"
          value={filters.status}
          onChange={(e) => setFilters({ ...filters, status: e.target.value })}
        >
          <option value="">Semua status</option>
          <option value="success">Sukses</option>
          <option value="failure">Gagal</option>
        </select>
        <button type="submit" className="btn-primary" disabled={loading}>
          {loading ? 'Memuat...' : 'Filter'}
        </button>
      </form>

      {error && <Alert tone="error">{error}</Alert>}

      <div className="card overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-3 py-2">Waktu</th>
              <th className="px-3 py-2">Actor</th>
              <th className="px-3 py-2">Target</th>
              <th className="px-3 py-2">Action</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">IP</th>
              <th className="px-3 py-2">Pesan</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 && !loading && (
              <tr>
                <td colSpan={7} className="px-3 py-8 text-center text-slate-500">
                  Tidak ada data.
                </td>
              </tr>
            )}
            {items.map((e) => (
              <tr key={e.id} className="border-t border-slate-100">
                <td className="px-3 py-2 whitespace-nowrap text-slate-600">
                  {new Date(e.timestamp).toLocaleString()}
                </td>
                <td className="px-3 py-2 font-medium">{e.actor}</td>
                <td className="px-3 py-2">{e.target}</td>
                <td className="px-3 py-2">{e.action}</td>
                <td className="px-3 py-2">
                  <span
                    className={`badge ${
                      e.status === 'success'
                        ? 'bg-green-100 text-green-800'
                        : 'bg-red-100 text-red-800'
                    }`}
                  >
                    {e.status}
                  </span>
                </td>
                <td className="px-3 py-2 text-slate-600">{e.ipAddress}</td>
                <td className="px-3 py-2 text-slate-500">{e.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-sm">
        <p className="text-slate-500">
          {total === 0
            ? '0 hasil'
            : `Menampilkan ${offset + 1} - ${Math.min(offset + items.length, total)} dari ${total}`}
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            className="btn-secondary"
            onClick={() => load(Math.max(0, offset - PAGE_SIZE))}
            disabled={loading || offset === 0}
          >
            &larr; Sebelumnya
          </button>
          <button
            type="button"
            className="btn-secondary"
            onClick={() => load(offset + PAGE_SIZE)}
            disabled={loading || offset + items.length >= total}
          >
            Berikutnya &rarr;
          </button>
        </div>
      </div>
    </div>
  );
}
