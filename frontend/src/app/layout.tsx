import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'FreeIPA Self-Service Portal',
  description: 'Portal self-service untuk reset & ganti password akun FreeIPA',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>{children}</body>
    </html>
  );
}
