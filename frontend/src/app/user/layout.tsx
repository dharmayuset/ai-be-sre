import { Header } from '@/components/Header';
import { getCurrentUser } from '@/lib/server';

export default async function UserLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();
  return (
    <>
      <Header user={user} />
      <main className="mx-auto max-w-3xl px-4 py-8">{children}</main>
    </>
  );
}
