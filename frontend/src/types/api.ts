// Tipe data API yang dipakai antar komponen.
// Selalu sinkronkan dengan struct Go di backend/internal/models.

export type Role = 'user' | 'admin';

export interface User {
  username: string;
  email: string;
  displayName: string;
  firstName: string;
  lastName: string;
  groups: string[];
  isAdmin: boolean;
  locked: boolean;
  lastLogin?: string;
}

export interface ApiError {
  error: string;
  code?: string;
  details?: unknown;
}

export interface AuditEntry {
  id: number;
  timestamp: string;
  actor: string;
  target: string;
  action: string;
  status: 'success' | 'failure';
  ipAddress: string;
  userAgent: string;
  message?: string;
}

export interface AuditListResponse {
  items: AuditEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface DashboardStats {
  totalEvents: number;
  successCount: number;
  failureCount: number;
  byAction: Record<string, number>;
  recentFailures: number;
  resetCount: number;
}

export interface UserStats {
  active: number;
  inactive: number;
  total: number;
}

export interface BatchDeleteResult {
  username: string;
  success: boolean;
  error?: string;
}

export interface BatchDeleteResponse {
  results: BatchDeleteResult[];
  successCount: number;
  failCount: number;
  total: number;
}
