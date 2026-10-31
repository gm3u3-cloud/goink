import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { GetContent } from "@/lib/wailsjs/go/app/App";
import { contentKeys } from "@/lib/queryKeys";

// useFileContent: the fetch cache channel for GetContent.
//
// Design rationale (5.2 special point 4): ContentPanel is a multi-tab
// editor; tabs are local state (useEditorTabs). GetContent's role is to
// "fetch on demand and stuff it into tab.content" — the query is a fetch
// cache channel, not a direct UI driver.
// The fetched data still needs updateTab(tabId, { content }) to back-fill
// the tab.
//
// Because all call sites are inside callbacks/effects (doOpenFile /
// handleSetViewMode / handleDiffApprove / restore-tab effect), the useQuery
// hook cannot be used (hooks must be at the component top level);
// queryClient.fetchQuery is used instead:
// - The same path uses the cache within 30s (inherits the global
//   staleTime); shared across tabs to avoid duplicate fetches
// - queryFn imports wailsjs GetContent directly (not useApp)
// - Query errors go through the global middleware (fetchQuery creates the
//   query and adds it to the QueryCache; the error state triggers the
//   toast)
// - The caller's catch does the fallback (e.g. updateTab stuffing
//   content.loadFailedCloseTab), preserving the original behavior
//
// The forced refresh triggered by the file:changed event does not go through
// this hook (commit 1 imported GetContent directly; commit 3 switched to
// qc.invalidateQueries).
export function useFileContent() {
  const qc = useQueryClient();
  const fetchContent = useCallback(
    async (novelId: number, filePath: string): Promise<string> => {
      return qc.fetchQuery({
        queryKey: contentKeys.detail(novelId, filePath),
        queryFn: () => GetContent(novelId, filePath),
      });
    },
    [qc],
  );
  return { fetchContent };
}
