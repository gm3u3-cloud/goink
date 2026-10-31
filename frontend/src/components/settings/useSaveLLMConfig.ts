import { useMutation, useQueryClient } from "@tanstack/react-query";
import { SaveLLMConfig } from "@/lib/wailsjs/go/app/App";
import type { llm } from "@/lib/wailsjs/go/models";
import { modelKeys, llmConfigKeys } from "@/lib/queryKeys";

// useSaveLLMConfig: mutation to save the LLM config.
// onSuccess invalidates modelKeys.all (ChatPanel's useModels auto-refetches;
// the selected state is corrected by the modelsQuery effect)
// + llmConfigKeys.all (ModelConfigTab's useLLMConfig auto-refetches,
// replacing the old onSaved callback chain).
// Error handling is done by the caller with try/catch + setSaveMsg (keeps
// the component-level inline error display).
export function useSaveLLMConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: llm.LLMConfigView) => SaveLLMConfig(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: modelKeys.all });
      qc.invalidateQueries({ queryKey: llmConfigKeys.all });
    },
  });
}
