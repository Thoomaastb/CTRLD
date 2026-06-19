import { apiClient } from "./client";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface LoginResponse {
  access_token?: string;
  refresh_token?: string;
  access_expires_at?: string;
  refresh_expires_at?: string;
  role?: string;
  mfa_required?: boolean;
  mfa_session_token?: string;
}

export interface MFAVerifyResponse {
  access_token: string;
  refresh_token: string;
  access_expires_at: string;
  role: string;
}

export interface SetupStatus {
  is_completed: boolean;
  has_admin: boolean;
  current_step: number;
  admin_id?: string;
}

export interface TOTPSetup {
  secret: string;
  qr_code: string;
  manual_entry_key: string;
}

export interface TOTPConfirmResult {
  credential_id: string;
  backup_codes: string[];
  message: string;
}

// ── Token-Verwaltung (localStorage) ──────────────────────────────────────────

const TOKEN_KEY = "ctrld_access_token";
const REFRESH_KEY = "ctrld_refresh_token";
const ROLE_KEY = "ctrld_role";

export const tokenStore = {
  set(accessToken: string, refreshToken: string, role: string) {
    localStorage.setItem(TOKEN_KEY, accessToken);
    localStorage.setItem(REFRESH_KEY, refreshToken);
    localStorage.setItem(ROLE_KEY, role);
  },
  getAccess(): string | null {
    return localStorage.getItem(TOKEN_KEY);
  },
  getRefresh(): string | null {
    return localStorage.getItem(REFRESH_KEY);
  },
  getRole(): string | null {
    return localStorage.getItem(ROLE_KEY);
  },
  clear() {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(REFRESH_KEY);
    localStorage.removeItem(ROLE_KEY);
  },
  isLoggedIn(): boolean {
    return !!localStorage.getItem(TOKEN_KEY);
  },
};

// ── API-Calls ─────────────────────────────────────────────────────────────────

export const authApi = {
  login: (email: string, password: string) =>
    apiClient.post<LoginResponse>("/auth/login", { email, password }),

  verifyMFA: (mfaSessionToken: string, code: string, type = "totp") =>
    apiClient.post<MFAVerifyResponse>("/auth/mfa/verify", {
      mfa_session_token: mfaSessionToken,
      code,
      type,
    }),

  logout: () =>
    apiClient.post<void>("/auth/logout"),

  refresh: (refreshToken: string) =>
    apiClient.post<LoginResponse>("/auth/refresh", { refresh_token: refreshToken }),

  setupStatus: () =>
    apiClient.get<SetupStatus>("/setup/status"),

  createAdmin: (email: string, password: string) =>
    apiClient.post<{ user_id: string; email: string }>("/setup/admin", { email, password }),

  completeSetup: (adminId: string) =>
    apiClient.post<{ message: string }>("/setup/complete", { admin_id: adminId }),

  initiateTOTP: () =>
    apiClient.post<TOTPSetup>("/auth/mfa/credentials/totp/initiate", {}),

  confirmTOTP: (secret: string, code: string, deviceName?: string) =>
    apiClient.post<TOTPConfirmResult>("/auth/mfa/credentials/totp/confirm", {
      secret,
      code,
      device_name: deviceName ?? "Authenticator App",
    }),
};
