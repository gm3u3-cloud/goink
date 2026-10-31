import { useQuery } from "@tanstack/react-query";
import { GetRemoteSkillContent } from "@/lib/wailsjs/go/app/App";
import { skillKeys } from "@/lib/queryKeys";
import { unwrapResult } from "@/utils/wailsResult";

// fetchRemoteSkillContent: fetch remote skill content (new apperr API).
// Exported for SkillMarketplace.handleInstall to reuse in the
// confirm_overwrite flow:
// the detail phase has already cached it via the useRemoteSkillContent
// query; handleInstall uses qc.fetchQuery with the same queryKey to reuse
// the cache and avoid a duplicate fetch.
export async function fetchRemoteSkillContent(name: string): Promise<string> {
  const res = await GetRemoteSkillContent(name);
  return unwrapResult(res);
}

// useRemoteSkillContent: remote skill content query (new apperr API).
// queryFn uses unwrapResult to unwrap Result[string]; throws AppErr when
// err_code is non-empty.
// enabled: !!name && phase === "detail" (passed by the caller); the
// confirm_overwrite phase does not refetch (it uses the cached value copied
// to remoteContentForConfirm during handleInstall).
export function useRemoteSkillContent(name: string, enabled: boolean) {
  return useQuery({
    queryKey: skillKeys.remoteContent(name),
    queryFn: () => fetchRemoteSkillContent(name),
    enabled,
  });
}
