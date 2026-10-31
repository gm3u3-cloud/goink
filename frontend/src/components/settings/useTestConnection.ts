import { useMutation } from "@tanstack/react-query";
import { TestConnection } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";

// useTestConnection: mutation to test LLM provider connectivity.
// The backend performs a real test with multi-layer fallback and returns the
// verified URL (which may differ from the input).
// No invalidate needed on onSuccess (command operation; it does not change
// cached data; the URL write-back is handled by the caller's setProviders).
// Error handling is done by the caller with try/catch + inline
// testResults.msg (keeps the component-level inline error display).
// 5.8 commit 1.
export function useTestConnection() {
  return useMutation({
    mutationFn: (input: app.TestConnectionInput) => TestConnection(input),
  });
}
