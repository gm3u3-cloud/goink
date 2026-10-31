import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveMCPServerSettings } from "@/lib/wailsjs/go/app/App";
import type { servercfg } from "@/lib/wailsjs/go/models";
import { mcpServerKeys } from "@/lib/queryKeys";

export function useSaveMCPServerSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: servercfg.Local) => SaveMCPServerSettings(input),
    onMutate: () => qc.cancelQueries({ queryKey: mcpServerKeys.all }),
    onSuccess: (view) => qc.setQueryData(mcpServerKeys.all, view),
    // The config may have been saved while start/stop failed; re-query the
    // backend's real state after both success and failure.
    onSettled: () => qc.invalidateQueries({ queryKey: mcpServerKeys.all }),
  });
}
