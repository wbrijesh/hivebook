"use client"

import { useState, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { TransportProvider } from "@connectrpc/connect-query"

import { transport } from "@/lib/api/client"
import { isAuthError } from "@/lib/session"

// One QueryClient and one Connect transport for the whole app. The query
// defaults live here: server state stays briefly fresh, and auth failures are
// never retried — the session policy signs the user out instead of looping
// (design-doc 0006, standards/frontend/data-fetching).
function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) => !isAuthError(error) && failureCount < 3,
      },
    },
  })
}

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(makeQueryClient)
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </TransportProvider>
  )
}
