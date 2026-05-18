'use client';

import { useState } from 'react';

interface Props extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

// Input password dengan toggle show/hide.
export function PasswordInput({ label, error, id, ...rest }: Props) {
  const [shown, setShown] = useState(false);
  return (
    <div>
      {label && <label htmlFor={id} className="label">{label}</label>}
      <div className="relative">
        <input
          {...rest}
          id={id}
          type={shown ? 'text' : 'password'}
          autoComplete="off"
          className={`input pr-20 ${error ? 'border-red-400' : ''}`}
        />
        <button
          type="button"
          onClick={() => setShown((v) => !v)}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-slate-500 hover:text-slate-800 px-2"
          tabIndex={-1}
          aria-label={shown ? 'Sembunyikan password' : 'Tampilkan password'}
        >
          {shown ? 'Hide' : 'Show'}
        </button>
      </div>
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
    </div>
  );
}
