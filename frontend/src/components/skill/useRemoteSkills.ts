import { useQuery } from "@tanstack/react-query";
import { ListRemoteSkills } from "@/lib/wailsjs/go/app/App";
import type { app, storage, remote } from "@/lib/wailsjs/go/models";
import { skillKeys } from "@/lib/queryKeys";
import { unwrapResult } from "@/utils/wailsResult";

// useRemoteSkills: remote skill marketplace list query (new apperr API).
// queryFn uses unwrapResult to unwrap Result[PageResult[RemoteSkillMeta]];
// throws AppErr (with errCode) when err_code is non-empty, putting the query
// into the error state and triggering the middleware's fallback toast.
// The component reads query.error's (AppErr) errCode and passes it to
// classifyError to compute the specific inline message.
// enabled is controlled by the caller (SkillMarketplace only fetches when open).
// debounce is controlled by the caller putting debouncedQuery into the
// queryKey; a queryKey change auto-refetches.
//
// T is explicitly the PageResult struct type (unwrapResult's generic
// inference degrades to {}, so a manual annotation is needed).
type RemoteSkillPageResult = {
  items: remote.RemoteSkillMeta[];
  total: number;
  total_pages: number;
};

export function useRemoteSkills(
  input: app.ListRemoteSkillsInput,
  enabled: boolean,
) {
  return useQuery({
    queryKey: skillKeys.remoteList(input),
    queryFn: async (): Promise<RemoteSkillPageResult> => {
      const res = await ListRemoteSkills(input);
      return unwrapResult<storage.PageResult_github_com_sigpanic_goink_internal_skill_remote_RemoteSkillMeta_>(
        res,
      );
    },
    enabled,
  });
}
