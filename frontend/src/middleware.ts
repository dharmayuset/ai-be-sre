// Next.js Edge middleware: route protection berdasarkan cookie auth.
//
// Catatan: middleware ini HANYA mengecek keberadaan cookie. Verifikasi
// JWT signature & role tetap dilakukan oleh backend Go di setiap request API.
// Tujuan middleware ini cuma UX (redirect cepat ke login kalau belum auth).

import { NextResponse, type NextRequest } from 'next/server';

const ACCESS_COOKIE = 'ai_be_sre_access';

const PUBLIC_PATHS = ['/login', '/forgot-password', '/vpn-profile'];

function isPublic(pathname: string): boolean {
  return PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'));
}

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;

  const hasAccess = req.cookies.has(ACCESS_COOKIE);

  // Jika sudah login, tidak boleh kembali ke /login
  if (hasAccess && pathname === '/login') {
    return NextResponse.redirect(new URL('/user', req.url));
  }

  // Halaman publik: lewatkan
  if (isPublic(pathname)) return NextResponse.next();

  // Halaman protected & belum login -> redirect ke /login
  if (!hasAccess) {
    const loginUrl = new URL('/login', req.url);
    loginUrl.searchParams.set('next', pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

// Jangan jalankan middleware di asset statis & API rewrites
export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)'],
};
