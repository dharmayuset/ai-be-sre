// Root page: redirect berdasarkan status auth.
// Auth dikenali dari ada/tidaknya cookie (lihat middleware.ts).
import { redirect } from 'next/navigation';

export default function HomePage() {
  redirect('/user');
}
