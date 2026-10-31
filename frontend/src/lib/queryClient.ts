import { QueryClient } from "@tanstack/react-query";
import { installQueryErrorToast } from "./queryErrorToast";

// staleTime 30s: avoid repeated fetches when switching panels in a short
// time.
// The project has no URL; queryKeys are designed by hand, so staleTime acts
// as a fallback debounce.
// refetchOnWindowFocus: false — a desktop app should not refetch on window
// focus changes.
// retry: 1 — retry once on occasional backend hiccups, avoid infinite
// retries.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

// 4a: install the global query error toast middleware.
// Mounted once at module top level to avoid the StrictMode double call.
// Design details: docs/feat/v1.4.0/refactor-plan/04a-query-error-toast.md.
installQueryErrorToast(queryClient);
