import Link from 'next/link';
import { Header } from '@/components/Header';
import { requireAdmin } from '@/lib/server';

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  // Server-side guard: kalau bukan admin -> redirect ke /user.
  // Backend juga akan reject dengan 403 untuk endpoint /admin/*.
  const user = await requireAdmin();

  return (
    <>
      <Header user={user} />
      <div className="mx-auto flex max-w-6xl gap-6 px-4 py-8">
        <aside className="hidden w-56 shrink-0 sm:block">
          <nav className="space-y-1 text-sm">
            <NavItem href="/admin" label="Dashboard" />
            <NavItem href="/admin/users" label="Users" />
            <NavItem href="/admin/audit" label="Audit Log" />
          </nav>
        </aside>
        <main className="flex-1 min-w-0">{children}</main>
      </div>
    </>
  );
}

function NavItem({ href, label }: { href: string; label: string }) {
  return (
    <Link
      href={href}
      className="block rounded-md px-3 py-2 text-slate-700 hover:bg-slate-100 hover:text-slate-900"
    >
      {label}
    </Link>
  );
}
