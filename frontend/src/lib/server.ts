// Helper untuk Server Components: ambil current user dengan
// meneruskan cookie request -> backend.
//
// Ini DUA kali validasi yang aman:
//   1. Edge middleware sudah cek keberadaan cookie (cepat, UX).
//   2. Server fetch ke /auth/me memvalidasi JWT signature di backend.
import { cookies, headers } from 'next/headers';
import { redirect } from 'next/navigation';
import type { User } from '@/types/api';

const BACKEND = process.env.BACKEND_URL || 'http://localhost:8080';

export async function getCurrentUser(): Promise<User> {
  const ck = cookies().toString();
  // Forward minimal headers untuk audit
  const ua = headers().get('user-agent') ?? '';

  let res: Response;
  try {
    res = await fetch(`${BACKEND}/api/v1/auth/me`, {
      headers: {
        Cookie: ck,
        'User-Agent': ua,
      },
      // Server-side fetch — disable Next.js cache supaya selalu fresh
      cache: 'no-store',
    });
  } catch {
    redirect('/login');
  }

  if (!res.ok) {
    // 401/403 -> belum auth. Lainnya (5xx) -> service down. Sama-sama
    // redirect ke login supaya user tahu mereka harus login ulang.
    redirect('/login');
  }
  const data = (await res.json()) as { user: User };
  return data.user;
}

export async function requireAdmin(): Promise<User> {
  const u = await getCurrentUser();
  if (!u.isAdmin) redirect('/user');
  return u;
}
