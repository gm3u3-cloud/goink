import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteNovelSetting } from "@/lib/wailsjs/go/app/App";
import { novelSettingKeys } from "@/lib/queryKeys";

// useDeleteNovelSetting: delete novel-setting entry mutation.
// mutationFn directly imports the wailsjs DeleteNovelSetting (no useApp).
// Single-parameter API: DeleteNovelSetting(settingId) (same as preference's
// DeletePreference(id), unlike reader's two-parameter version).
// Consumer: NovelSettingView.confirmDelete (mutateAsync errors are caught by
// the handler's try/catch).
// The handler is responsible for setDeleteTarget(null) + the error toast
// (side effects are not put in the mutation).
// onSuccess invalidates novel-settings: the list syncs after the entry is
// deleted (NovelSettingView + NovelSettingList share the cache).
export function useDeleteNovelSetting(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settingId: number) => DeleteNovelSetting(settingId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelSettingKeys.list(novelId) });
    },
  });
}
