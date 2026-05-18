import Link from 'next/link';
import { getCurrentUser } from '@/lib/server';

// Home user: hanya 2 menu sesuai requirement.
export default async function UserHomePage() {
  const user = await getCurrentUser();

  return (
    <div className="space-y-6">
      <section className="card">
        <h1 className="text-xl font-semibold text-slate-900">
          Halo, {user.displayName || user.username}
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Pilih layanan yang ingin Anda gunakan.
        </p>
      </section>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link
          href="/user/change-password"
          className="card flex flex-col gap-2 transition hover:border-brand-500 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          <div className="text-3xl">🔑</div>
          <h2 className="text-base font-semibold text-slate-900">Ganti Password</h2>
          <p className="text-sm text-slate-500">
            Ubah password Anda. Anda perlu memasukkan password lama.
          </p>
        </Link>

        <Link
          href="/user/reset-password"
          className="card flex flex-col gap-2 transition hover:border-brand-500 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          <div className="text-3xl">📧</div>
          <h2 className="text-base font-semibold text-slate-900">Reset Password</h2>
          <p className="text-sm text-slate-500">
            Lupa password lama? Kirim temporary password ke email Anda.
          </p>
        </Link>
      </div>
    </div>
  );
}
