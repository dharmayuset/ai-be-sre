import { checkPassword } from '@/lib/password';

const labels = ['Sangat lemah', 'Lemah', 'Cukup', 'Kuat', 'Sangat kuat'];
const colors = ['bg-red-500', 'bg-orange-500', 'bg-yellow-500', 'bg-green-500', 'bg-emerald-600'];

export function PasswordStrength({ value }: { value: string }) {
  if (!value) return null;
  const { strength, errors, ok } = checkPassword(value);
  return (
    <div className="mt-2">
      <div className="flex h-1.5 w-full overflow-hidden rounded bg-slate-200">
        <div
          className={`h-full transition-all ${colors[strength]}`}
          style={{ width: `${(strength / 4) * 100}%` }}
        />
      </div>
      <p className={`mt-1 text-xs ${ok ? 'text-green-700' : 'text-slate-600'}`}>
        Kekuatan: {labels[strength]}
      </p>
      {errors.length > 0 && (
        <ul className="mt-1 space-y-0.5 text-xs text-red-600">
          {errors.map((e) => (
            <li key={e}>- {e}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
