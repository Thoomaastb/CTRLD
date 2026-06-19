"use client";

import { useQuery } from "@tanstack/react-query";
import { healthApi, type HealthResponse } from "@/lib/api/client";

/**
 * React Query Hook für den Backend-Health-Endpoint.
 * Prüft alle 30 Sekunden ob das Backend erreichbar ist.
 */
export function useHealth() {
  return useQuery<HealthResponse>({
    queryKey: ["health"],
    queryFn: () => healthApi.get(),
    refetchInterval: 30_000,
    retry: 1,
  });
}
