import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdatePreference } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { preferenceKeys } from "@/lib/queryKeys";

// useUpdatePreference: update preference entry mutation.
// mutationFn directly imports the wailsjs UpdatePreference (no useApp);
// returns preference.PreferenceItem.
// Parameter order: UpdatePreference(novelId, id, input) (3 parameters,
// novelId first — the reverse of reader's
// UpdateReaderPerspective(id, novelId, input); the preference backend
// signature is like this).
// Input {id, input}: input uses app.UpdatePreferenceInput (PUT semantics,
// full payload sent back).
// Consumer: PreferenceView.handleSave (mutateAsync errors are caught by the
// handler's try/catch).
// The handler is responsible for setEditMode(null) + setForm(EMPTY_FORM) +
// the error toast (side effects are not put in the mutation).
// onSuccess invalidates preferences: the entry's field changes are reflected
// in the list.
export function useUpdatePreference(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdatePreferenceInput;
    }) => UpdatePreference(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: preferenceKeys.list(novelId) });
    },
  });
}
