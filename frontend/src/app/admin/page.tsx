'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Alert } from '@/components/Alert';
import { api, ApiClientError } from '@/lib/api';
import type { DashboardStats, UserStats } from '@/types/api';

export default function AdminDashboard() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [userStats, setUserStats] = useState<UserStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .adminStats()
      .then(setStats)
      .catch((e) => {
        setError(e instanceof ApiClientError ? e.message : 'Gagal memuat statistik');
      });
    api
      .adminUserStats()
      .then(setUserStats)
      .catch((e) => {
        setError(e instanceof ApiClientError ? e.message : 'Gagal memuat statistik user');
      });
  }, []);

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-xl font-semibold text-slate-900">Dashboard</h1>
        <p className="mt-1 text-sm text-slate-500">Ringkasan aktivitas sistem dan statistik user.</p>
      </header>

      {error && <Alert tone="error">{error}</Alert>}

      {/* User Statistics */}
      <section>
        <h2 className="text-base font-semibold text-slate-900 mb-3">Statistik User</h2>
        <div className="grid gap-4 sm:grid-cols-3">
          <StatCard title="Total User" value={userStats?.total} tone="default" />
          <StatCard title="Active" value={userStats?.active} tone="success" />
          <StatCard title="Inactive / Locked" value={userStats?.inactive} tone="failure" />
        </div>
      </section>

      {/* Activity Statistics */}
      <section>
        <h2 className="text-base font-semibold text-slate-900 mb-3">Aktivitas Sistem</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard title="Total Event" value={stats?.totalEvents} tone="default" />
          <StatCard title="Sukses" value={stats?.successCount} tone="success" />
          <StatCard title="Gagal" value={stats?.failureCount} tone="failure" />
          <StatCard
            title="Reset 24h Terakhir"
            value={stats?.resetCount}
            tone="info"
          />
        </div>
      </section>

      <section className="card">
        <h2 className="text-base font-semibold text-slate-900">Event per Tipe</h2>
        {stats ? (
          Object.keys(stats.byAction).length === 0 ? (
            <p className="mt-2 text-sm text-slate-500">Belum ada event tercatat.</p>
          ) : (
            <table className="mt-3 w-full text-sm">
              <tbody>
                {Object.entries(stats.byAction)
                  .sort((a, b) => b[1] - a[1])
                  .map(([action, count]) => (
                    <tr key={action} className="border-t border-slate-100">
                      <td className="py-2 text-slate-700">{action}</td>
                      <td className="py-2 text-right font-medium">{count}</td>
                    </tr>
                  ))}
              </tbody>
            </table>
          )
        ) : (
          <p className="mt-2 text-sm text-slate-500">Memuat...</p>
        )}
      </section>

      <section className="card">
        <h2 className="text-base font-semibold text-slate-900">Aksi Cepat</h2>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <Link href="/admin/users" className="btn-secondary justify-start">
            Kelola Users
          </Link>
          <Link href="/admin/audit" className="btn-secondary justify-start">
            Lihat Audit Log
          </Link>
        </div>
      </section>
    </div>
  );
}

function StatCard({
  title,
  value,
  tone,
}: {
  title: string;
  value: number | undefined;
  tone: 'default' | 'success' | 'failure' | 'info';
}) {
  const toneClass = {
    default: 'text-slate-900',
    success: 'text-green-700',
    failure: 'text-red-700',
    info: 'text-brand-700',
  }[tone];
  return (
    <div className="card">
      <div className="text-xs uppercase tracking-wide text-slate-500">{title}</div>
      <div className={`mt-1 text-2xl font-semibold ${toneClass}`}>
        {value ?? '...'}
      </div>
    </div>
  );
}
