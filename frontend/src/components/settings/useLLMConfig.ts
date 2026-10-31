import { useQuery } from "@tanstack/react-query";
import { GetLLMConfig } from "@/lib/wailsjs/go/app/App";
import { llmConfigKeys } from "@/lib/queryKeys";
import type { llm } from "@/lib/wailsjs/go/models";

// useLLMConfig: LLM config query (includes the providers list).
// queryFn imports wailsjs GetLLMConfig directly (not useApp).
// ModelConfigTab's startup initialization depends on this query data being
// ready.
// useSaveLLMConfig onSuccess invalidates this key to trigger a refetch.
// GET errors are handled by the global middleware (llm-config prefix →
// settings.llmConfigLoadFailed).
// 5.8 commit 1.
export type LLMConfig = llm.LLMConfigView;

export function useLLMConfig() {
  return useQuery({
    queryKey: llmConfigKeys.all,
    queryFn: async () => {
      const config = await GetLLMConfig();
      return config;
    },
  });
}
