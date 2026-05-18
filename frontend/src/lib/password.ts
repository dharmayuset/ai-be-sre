// Password strength validation di sisi client.
// HARUS sinkron dengan policy server (yang juga validate di FreeIPA).

export interface PasswordCheck {
  ok: boolean;
  errors: string[];
  strength: 0 | 1 | 2 | 3 | 4; // 0..4 (visual indicator)
}

export function checkPassword(p: string): PasswordCheck {
  const errors: string[] = [];
  if (p.length < 12) errors.push('Minimal 12 karakter');
  if (!/[A-Z]/.test(p)) errors.push('Harus ada huruf besar');
  if (!/[a-z]/.test(p)) errors.push('Harus ada huruf kecil');
  if (!/[0-9]/.test(p)) errors.push('Harus ada angka');
  if (!/[!@#$%^&*\-_=+]/.test(p)) errors.push('Harus ada karakter spesial (!@#$%^&*-_=+)');

  let strength: 0 | 1 | 2 | 3 | 4 = 0;
  if (p.length >= 12) strength++;
  if (p.length >= 16) strength++;
  if (/[A-Z]/.test(p) && /[a-z]/.test(p)) strength++;
  if (/[0-9]/.test(p) && /[!@#$%^&*\-_=+]/.test(p)) strength++;
  strength = Math.min(strength, 4) as PasswordCheck['strength'];

  return { ok: errors.length === 0, errors, strength };
}
