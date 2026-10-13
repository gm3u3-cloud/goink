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
    // 配置可能已保存而启停失败，成功和失败后都重新查询后端真实状态。
    onSettled: () => qc.invalidateQueries({ queryKey: mcpServerKeys.all }),
  });
}
