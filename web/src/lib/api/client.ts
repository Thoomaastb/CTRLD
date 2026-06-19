/**
 * CTRLD API-Client
 *
 * Zentraler HTTP-Client für alle Requests an das Go-Backend.
 * - Base-URL via NEXT_PUBLIC_API_URL oder /api (Proxy via next.config.ts)
 * - Automatisches JWT-Handling (kommt mit US-001)
 * - Einheitliches Error-Handling
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "/api/v1";

export interface ApiError {
  error: string;
  code: string;
  request_id: string;
}

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: ApiError
  ) {
    super(`API error ${status}: ${body.error}`);
    this.name = "ApiRequestError";
  }
}

/**
 * Basis-Fetch-Wrapper mit einheitlichem Error-Handling.
 * Gibt den geparsten JSON-Body zurück oder wirft ApiRequestError.
 */
async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE}${path}`;

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  const response = await fetch(url, {
    ...options,
    headers,
    // Credentials für Cookie-basierte Sessions (kommt mit US-001)
    credentials: "include",
  });

  if (!response.ok) {
    let errorBody: ApiError;
    try {
      errorBody = (await response.json()) as ApiError;
    } catch {
      errorBody = {
        error: response.statusText,
        code: "UNKNOWN",
        request_id: "",
      };
    }
    throw new ApiRequestError(response.status, errorBody);
  }

  // 204 No Content: kein Body
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

// ─── Convenience-Methoden ──────────────────────────────────────────────────

export const apiClient = {
  get: <T>(path: string, options?: RequestInit) =>
    apiFetch<T>(path, { ...options, method: "GET" }),

  post: <T>(path: string, body?: unknown, options?: RequestInit) =>
    apiFetch<T>(path, {
      ...options,
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  put: <T>(path: string, body?: unknown, options?: RequestInit) =>
    apiFetch<T>(path, {
      ...options,
      method: "PUT",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  delete: <T>(path: string, options?: RequestInit) =>
    apiFetch<T>(path, { ...options, method: "DELETE" }),
};

// ─── Health-API ───────────────────────────────────────────────────────────

export interface HealthResponse {
  status: string;
  version: string;
  timestamp: string;
}

export const healthApi = {
  get: () => apiClient.get<HealthResponse>("/health"),
};
