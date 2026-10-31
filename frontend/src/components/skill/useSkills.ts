import { useQuery } from "@tanstack/react-query";
import { ListSkills } from "@/lib/wailsjs/go/app/App";
import { skillKeys } from "@/lib/queryKeys";

// useSkills: skill list query.
// queryFn directly imports the wailsjs ListSkills (no useApp); no staleTime
// is set (inherits the global 30s).
// enabled: guarded by !!novelId; no fetch when novelId=0 (data falls back to
// an empty array).
// Consumers: SkillList / SkillMarketplace (installed index, moved in commit
// 3) share the cache; after CRUD, the mutation's invalidateQueries keeps
// them in sync (useDeleteSkill moved in commit 2).
// ListSkills is the old API (returns an array directly, not apperr.Result),
// so no unwrapResult is needed.
export function useSkills(novelId: number) {
  return useQuery({
    queryKey: skillKeys.list(novelId),
    queryFn: async () => {
      const list = await ListSkills({ novel_id: novelId });
      return list ?? [];
    },
    enabled: !!novelId,
  });
}
