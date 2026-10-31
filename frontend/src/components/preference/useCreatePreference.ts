import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreatePreference } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { preferenceKeys } from "@/lib/queryKeys";

// useCreatePreference: create preference entry mutation.
// mutationFn directly imports the wailsjs CreatePreference (no useApp);
// returns preference.PreferenceItem.
// Parameter order: CreatePreference(novelId, input) (same as reader's
// CreateReaderPerspective).
// Consumer: PreferenceView.handleSave (mutateAsync errors are caught by the
// handler's try/catch).
// The handler is responsible for setEditMode(null) + setForm(EMPTY_FORM) +
// the error toast (side effects are not put in the mutation).
// onSuccess invalidates preferences: the new entry is reflected in the list,
// keeping PreferenceView / PreferenceList in sync.
export function useCreatePreference(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreatePreferenceInput) =>
      CreatePreference(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: preferenceKeys.list(novelId) });
    },
  });
}
