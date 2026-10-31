import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateNovelSetting } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { novelSettingKeys } from "@/lib/queryKeys";

// useUpdateNovelSetting: update novel-setting entry mutation.
// mutationFn directly imports the wailsjs UpdateNovelSetting (no useApp);
// returns setting.SettingItem.
// Parameter order: UpdateNovelSetting(novelId, id, input) (3 parameters,
// novelId first — same as preference; the reverse of reader's
// UpdateReaderPerspective(id, novelId, input)).
// The input has no is_global field (unlike preference; novel-setting has no
// global/novel-level distinction).
// Input {id, input}: input uses app.UpdateNovelSettingInput (PUT semantics,
// full payload sent back).
// Consumer: NovelSettingView.handleSave (mutateAsync errors are caught by
// the handler's try/catch).
// The handler is responsible for setEditMode(null) + setForm(EMPTY_FORM) +
// the error toast (side effects are not put in the mutation).
// onSuccess invalidates novel-settings: the entry's field changes are
// reflected in the list.
export function useUpdateNovelSetting(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateNovelSettingInput;
    }) => UpdateNovelSetting(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: novelSettingKeys.list(novelId) });
    },
  });
}
