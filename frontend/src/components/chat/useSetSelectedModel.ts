import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SetSelectedModel } from "@/lib/wailsjs/go/app/App";
import { settingsKeys } from "@/lib/queryKeys";

// useSetSelectedModel: mutation to set the selected model (persists
// selected_model_key + reasoning_effort to AppSettings).
// The backend API signature is SetSelectedModel(key, effort); mutationFn
// wraps it into an object { key, effort }.
// onSuccess invalidates settingsKeys.all — useSettings refetches the new
// selected_model_key/reasoning_effort.
// Error handling is done by the caller with try/catch + toastError.
// 5.1 commit 4 addendum.
export function useSetSelectedModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { key: string; effort: string }) =>
      SetSelectedModel(input.key, input.effort),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
