import { useQuery } from "@tanstack/react-query";
import { GetNovels } from "@/lib/wailsjs/go/app/App";
import { novelKeys } from "@/lib/queryKeys";

// useNovels: global novel list query.
// queryFn directly imports the wailsjs GetNovels (no useApp); a 30s staleTime
// prevents fetch jitter.
// The return value falls back to an empty array; consumers use
// const { data: novels = [] } = useNovels().
export function useNovels() {
  return useQuery({
    queryKey: novelKeys.all,
    queryFn: async () => {
      const list = await GetNovels();
      return list ?? [];
    },
  });
}
