import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateNovelSetting } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { novelSettingKeys } from "@/lib/queryKeys";

// useCreateNovelSetting: create novel-setting entry mutation.
// mutationFn directly imports the wailsjs CreateNovelSetting (no useApp);
// returns setting.SettingItem.
// Parameter order: CreateNovelSetting(novelId, input) (same as preference's
// CreatePreference).
// The input has no is_global field (unlike preference; novel-setting has no
// global/novel-level distinction).
// Consumer: NovelSettingView.handleSave (mutateAsync errors are caught by
// the handler's try/catch).
// The handler is responsible for setEditMode(null) + setForm(EMPTY_FORM) +
// the error toast (side effects are not put in the mutation).
// onSuccess invalidates novel-settings: the new entry is reflected in the
// list, keeping NovelSettingView / NovelSettingList in sync.
export function useCreateNovelSetting(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateNovelSettingInput) =>
      CreateNovelSetting(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelSettingKeys.list(novelId) });
    },
  });
}
