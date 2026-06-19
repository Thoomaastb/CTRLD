"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { useState } from "react";

interface ProvidersProps {
  children: React.ReactNode;
}

/**
 * Providers-Wrapper für alle Client-seitigen Provider.
 * QueryClient wird pro Session instanziiert (kein globaler Singleton).
 */
export function Providers({ children }: ProvidersProps) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            // Daten 30 Sekunden als frisch betrachten
            staleTime: 30_000,
            // Bei Fehler max. 2 Wiederholungsversuche
            retry: 2,
            // Kein Refetch beim Fokus-Wechsel (Server-Panel, kein Browser-Tab-Sprung)
            refetchOnWindowFocus: false,
          },
          mutations: {
            retry: 0,
          },
        },
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      {/* DevTools nur im Development-Build */}
      {process.env.NODE_ENV === "development" && (
        <ReactQueryDevtools initialIsOpen={false} />
      )}
    </QueryClientProvider>
  );
}
