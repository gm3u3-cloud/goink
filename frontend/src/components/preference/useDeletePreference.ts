import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeletePreference } from "@/lib/wailsjs/go/app/App";
import { preferenceKeys } from "@/lib/queryKeys";

// useDeletePreference: delete preference entry mutation.
// mutationFn directly imports the wailsjs DeletePreference (no useApp).
// Single-parameter API: DeletePreference(preferenceId) (unlike reader's
// two-parameter entryId+novelId).
// Consumer: PreferenceView.confirmDelete (mutateAsync errors are caught by
// the handler's try/catch).
// The handler is responsible for setDeleteTarget(null) + the error toast
// (side effects are not put in the mutation).
// onSuccess invalidates preferences: the list syncs after the entry is
// deleted (PreferenceView + PreferenceList share the cache).
export function useDeletePreference(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (preferenceId: number) => DeletePreference(preferenceId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: preferenceKeys.list(novelId) });
    },
  });
}
