import { useQuery } from "@tanstack/react-query";
import { GetStyleSample } from "@/lib/wailsjs/go/app/App";
import { styleSampleKeys } from "@/lib/queryKeys";

// useStyleSample: style sample detail query (StyleView's edit dialog
// fetches a single sample's full content on demand).
// The items.content returned by ListStyleSamples is a truncated preview;
// editing needs the full content, so the detail is fetched independently.
// enabled: id != null (no fetch when detailId is null); queryKey includes
// the id.
// The caller (StyleView openDetail) sets detailId to trigger the query; a
// useEffect watches for the query.data to become ready and back-fills the
// edit fields.
// The update mutation's onSuccess invalidates the detail (commit 2).
// 5.3 commit 1: GET errors are handled by the global middleware; the
// component does not mount toastError.
export function useStyleSample(id: number | null) {
  return useQuery({
    queryKey: styleSampleKeys.detail(id ?? 0),
    queryFn: async () => {
      return await GetStyleSample(id as number);
    },
    enabled: id != null,
  });
}
