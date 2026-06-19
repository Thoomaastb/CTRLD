/**
 * Shared TypeScript-Typen — spiegeln die Go-Structs 1:1 wider.
 * Werden mit jedem API-Endpunkt erweitert.
 */

// ─── Auth ─────────────────────────────────────────────────────────────────

export type UserRole = "admin" | "viewer";

export interface User {
  id: string;
  email: string;
  role: UserRole;
  created_at: string;
  last_login_at: string | null;
  is_active: boolean;
}

export interface Session {
  id: string;
  user_id: string;
  ip_address: string;
  user_agent: string | null;
  created_at: string;
  expires_at: string;
}

// ─── PIM ──────────────────────────────────────────────────────────────────

export interface PimSession {
  id: string;
  user_id: string;
  reason: string;
  requested_duration_min: number;
  started_at: string;
  expires_at: string;
  ended_at: string | null;
  is_break_glass: boolean;
  action_count: number;
}

// ─── System ───────────────────────────────────────────────────────────────

export interface SystemMetrics {
  cpu_usage_percent: number;
  ram_used_bytes: number;
  ram_total_bytes: number;
  disk_used_bytes: number;
  disk_total_bytes: number;
  uptime_seconds: number;
  load_avg_1: number;
  load_avg_5: number;
  load_avg_15: number;
}

// ─── Services ─────────────────────────────────────────────────────────────

export type ServiceStatus = "active" | "inactive" | "failed" | "unknown";

export interface Service {
  name: string;
  display_name: string;
  status: ServiceStatus;
  description: string;
  active_since: string | null;
}

// ─── Audit ────────────────────────────────────────────────────────────────

export type AuditSeverity = "info" | "warning" | "critical";
export type AuditResult = "success" | "failure";

export interface AuditEntry {
  id: string;
  user_id: string | null;
  session_id: string | null;
  pim_session_id: string | null;
  action_type: string;
  resource: string | null;
  result: AuditResult;
  ip_address: string | null;
  metadata: Record<string, unknown> | null;
  severity: AuditSeverity;
  created_at: string;
}

// ─── Alerts ───────────────────────────────────────────────────────────────

export type AlertType = "cpu" | "ram" | "disk";
export type AlertSeverity = "warning" | "critical";

export interface Alert {
  id: string;
  type: AlertType;
  threshold: number;
  current_value: number;
  severity: AlertSeverity;
  triggered_at: string;
  resolved_at: string | null;
  notified: boolean;
}

// ─── Pagination ───────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}
