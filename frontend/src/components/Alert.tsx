// Reusable alert component.
type Tone = 'success' | 'error' | 'info' | 'warning';

const toneClass: Record<Tone, string> = {
  success: 'bg-green-50 border-green-200 text-green-800',
  error: 'bg-red-50 border-red-200 text-red-800',
  info: 'bg-blue-50 border-blue-200 text-blue-800',
  warning: 'bg-amber-50 border-amber-200 text-amber-800',
};

export function Alert({ tone = 'info', children }: { tone?: Tone; children: React.ReactNode }) {
  return (
    <div role="alert" className={`rounded-md border px-4 py-3 text-sm ${toneClass[tone]}`}>
      {children}
    </div>
  );
}
