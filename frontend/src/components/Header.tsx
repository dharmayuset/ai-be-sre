'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { api, ApiClientError } from '@/lib/api';
import type { User } from '@/types/api';

export function Header({ user }: { user: User }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function handleLogout() {
    setBusy(true);
    try {
      await api.logout();
    } catch (e) {
      // log only - tetap arahkan ke login
      if (e instanceof ApiClientError) console.warn('logout error:', e.message);
    } finally {
      router.push('/login');
      router.refresh();
    }
  }

  return (
    <header className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
        <Link href={user.isAdmin ? '/admin' : '/user'} className="text-sm font-semibold text-slate-900">
          FreeIPA Self-Service
        </Link>
        <div className="flex items-center gap-3 text-sm">
          <span className="hidden sm:inline text-slate-600">
            {user.displayName || user.username}
          </span>
          <span
            className={`badge ${
              user.isAdmin ? 'bg-purple-100 text-purple-800' : 'bg-slate-100 text-slate-700'
            }`}
          >
            {user.isAdmin ? 'admin' : 'user'}
          </span>
          <button onClick={handleLogout} disabled={busy} className="btn-secondary">
            {busy ? 'Keluar...' : 'Logout'}
          </button>
        </div>
      </div>
    </header>
  );
}
