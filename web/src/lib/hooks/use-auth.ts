"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { authApi, tokenStore } from "@/lib/api/auth";

// ── Setup Status ──────────────────────────────────────────────────────────────

export function useSetupStatus() {
  return useQuery({
    queryKey: ["setup-status"],
    queryFn: () => authApi.setupStatus(),
    staleTime: 60_000,
    retry: 1,
  });
}

// ── Login ─────────────────────────────────────────────────────────────────────

export function useLogin() {
  const router = useRouter();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) =>
      authApi.login(email, password),
    onSuccess: (data) => {
      if (data.mfa_required && data.mfa_session_token) {
        // MFA erforderlich — Token im sessionStorage für nächsten Schritt
        sessionStorage.setItem("mfa_session_token", data.mfa_session_token);
        router.push("/login/mfa");
        return;
      }
      if (data.access_token && data.refresh_token && data.role) {
        tokenStore.set(data.access_token, data.refresh_token, data.role);
        qc.invalidateQueries({ queryKey: ["auth"] });
        router.push("/dashboard");
      }
    },
  });
}

// ── MFA Verify ────────────────────────────────────────────────────────────────

export function useMFAVerify() {
  const router = useRouter();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ code, type }: { code: string; type?: string }) => {
      const mfaToken = sessionStorage.getItem("mfa_session_token") ?? "";
      return authApi.verifyMFA(mfaToken, code, type);
    },
    onSuccess: (data) => {
      sessionStorage.removeItem("mfa_session_token");
      tokenStore.set(data.access_token, data.refresh_token, data.role);
      qc.invalidateQueries({ queryKey: ["auth"] });
      router.push("/dashboard");
    },
  });
}

// ── Logout ────────────────────────────────────────────────────────────────────

export function useLogout() {
  const router = useRouter();
  const qc = useQueryClient();

  return useMutation({
    mutationFn: () => authApi.logout(),
    onSettled: () => {
      tokenStore.clear();
      qc.clear();
      router.push("/login");
    },
  });
}

// ── Auth-State ────────────────────────────────────────────────────────────────

export function useIsLoggedIn(): boolean {
  if (typeof window === "undefined") return false;
  return tokenStore.isLoggedIn();
}

export function useRole(): string | null {
  if (typeof window === "undefined") return null;
  return tokenStore.getRole();
}
