import { useMutation, useQueryClient } from "@tanstack/react-query";
import { InstallRemoteSkill } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { contentKeys, skillKeys } from "@/lib/queryKeys";
import { unwrapResult } from "@/utils/wailsResult";
import { reportAIFileChange } from "@/components/content/aiFileChanges";

// useInstallRemoteSkill: install-remote-skill mutation (new apperr API).
// mutationFn uses unwrapResult to unwrap Result[struct]; throws AppErr when
// err_code is non-empty (data is an empty struct and meaningless; only
// err_code/err_msg are consumed).
// Input is app.InstallRemoteSkillInput (with name/target/novel_id); the
// caller assembles all fields (aligned with the useDeleteSkill pattern:
// novelId is known from the hook closure, but the InstallRemoteSkillInput
// backend binding requires the novel_id field, which the caller fills in).
// onSuccess invalidations:
//   - skillKeys.list(novelId): refreshes the installed index in
//     SkillList/SkillMarketplace; the installedVersions Map is recomputed
//     (determines the card installed/updatable markers)
//   - ["remote-skills"]: invalidates the remote list as a whole, refreshing
//     card markers
//   - contentKeys.detail: clears the empty content cached by probeLocal's
//     GetContent for the same path before install (issue #47: after install,
//     opening the skill preview showed no content until a restart)
// The mutation has no onError; the caller does try/catch + toastError
// (aligned with the useDeleteSkill pattern).
// AppErr.errCode is read by the caller and mapped to short-code messages via
// classifyError (e.g. rate_limited → errorRateLimited).
export function useInstallRemoteSkill(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: app.InstallRemoteSkillInput) => {
      const res = await InstallRemoteSkill(input);
      return unwrapResult(res);
    },
    onSuccess: (_data, input) => {
      qc.invalidateQueries({ queryKey: skillKeys.list(novelId) });
      qc.invalidateQueries({ queryKey: ["remote-skills"] });
      // Keep the install path consistent with SkillList.skillPath /
      // SkillMarketplace.pathForSource
      const path =
        input.target === "novel"
          ? `skills/${input.name}.md`
          : `~/.goink/skills/${input.name}.md`;
      if (input.target === "user") {
        void qc.invalidateQueries({ queryKey: skillKeys.all });
        void qc.invalidateQueries({
          predicate: ({ queryKey }) =>
            queryKey[0] === contentKeys.all[0] && queryKey[2] === path,
        });
      } else {
        void qc.invalidateQueries({
          queryKey: contentKeys.detail(input.novel_id, path),
        });
      }
      reportAIFileChange({ novelId: input.novel_id, path });
    },
  });
}
