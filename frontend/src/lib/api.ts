// Wrapper fetch ke backend.
//
// Strategi:
//   - Path API selalu /api/v1/... (di-rewrite ke backend Go via next.config.mjs).
//   - credentials: 'include' supaya HttpOnly cookie ikut.
//   - Sentralisasi error handling -> throw ApiClientError dengan info yang
//     berguna untuk UI.

import type { ApiError } from '@/types/api';

export class ApiClientError extends Error {
  status: number;
  code?: string;
  details?: unknown;

  constructor(status: number, body: ApiError) {
    super(body.error || `HTTP ${status}`);
    this.name = 'ApiClientError';
    this.status = status;
    this.code = body.code;
    this.details = body.details;
  }
}

const API_BASE = '/api/v1';
const DEFAULT_TIMEOUT_MS = Number(process.env.NEXT_PUBLIC_API_TIMEOUT_MS ?? 20000);

interface RequestOpts extends RequestInit {
  timeoutMs?: number;
}

export async function apiFetch<T = unknown>(path: string, opts: RequestOpts = {}): Promise<T> {
  const { timeoutMs = DEFAULT_TIMEOUT_MS, headers, ...rest } = opts;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const res = await fetch(`${API_BASE}${path}`, {
      ...rest,
      credentials: 'include',
      signal: controller.signal,
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
        ...(headers ?? {}),
      },
    });

    // 204 No Content
    if (res.status === 204) return undefined as T;

    const text = await res.text();
    const data = text ? JSON.parse(text) : undefined;

    if (!res.ok) {
      throw new ApiClientError(res.status, (data as ApiError) ?? { error: 'request failed' });
    }
    return data as T;
  } catch (err) {
    if (err instanceof ApiClientError) throw err;
    if ((err as Error).name === 'AbortError') {
      throw new ApiClientError(408, { error: 'request timeout' });
    }
    throw new ApiClientError(0, {
      error: (err as Error).message || 'network error',
    });
  } finally {
    clearTimeout(timer);
  }
}

// ---------- API helpers ----------

import type {
  User,
  AuditListResponse,
  DashboardStats,
} from '@/types/api';

export const api = {
  // Auth
  login: (username: string, password: string) =>
    apiFetch<{ user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => apiFetch<{ message: string }>('/auth/logout', { method: 'POST' }),
  me: () => apiFetch<{ user: User }>('/auth/me'),
  refresh: () => apiFetch<{ user: User }>('/auth/refresh', { method: 'POST' }),

  // Password (user)
  changePassword: (oldPassword: string, newPassword: string) =>
    apiFetch<{ message: string }>('/password/change', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),
  resetPasswordRequest: (username: string) =>
    apiFetch<{ message: string }>('/password/reset-request', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),

  // Admin
  adminStats: () => apiFetch<DashboardStats>('/admin/stats'),
  adminListUsers: (q?: string) => {
    const qs = q ? `?q=${encodeURIComponent(q)}` : '';
    return apiFetch<{ users: User[]; count: number }>(`/admin/users${qs}`);
  },
  adminGetUser: (username: string) =>
    apiFetch<{ user: User }>(`/admin/users/${encodeURIComponent(username)}`),
  adminResetPassword: (username: string) =>
    apiFetch<{ message: string; maskedEmail: string }>(
      `/admin/users/${encodeURIComponent(username)}/reset-password`,
      { method: 'POST' },
    ),
  adminLockUser: (username: string, lock: boolean) =>
    apiFetch<{ message: string }>(
      `/admin/users/${encodeURIComponent(username)}/lock`,
      { method: 'POST', body: JSON.stringify({ lock }) },
    ),
  adminAudit: (params: {
    actor?: string;
    target?: string;
    action?: string;
    status?: string;
    limit?: number;
    offset?: number;
  } = {}) => {
    const usp = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') usp.set(k, String(v));
    });
    const qs = usp.toString();
    return apiFetch<AuditListResponse>(`/admin/audit${qs ? `?${qs}` : ''}`);
  },
};
