import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  RevokeMCPServerCredential,
  RotateMCPServerCredential,
} from "@/lib/wailsjs/go/app/App";
import { mcpServerKeys } from "@/lib/queryKeys";

export function useMCPServerCredentialActions() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: mcpServerKeys.all });
  const cancelQuery = () => qc.cancelQueries({ queryKey: mcpServerKeys.all });
  const rotate = useMutation({
    mutationFn: RotateMCPServerCredential,
    onMutate: cancelQuery,
    onSuccess: (view) => qc.setQueryData(mcpServerKeys.all, view),
    onSettled: refresh,
  });
  const revoke = useMutation({
    mutationFn: RevokeMCPServerCredential,
    onMutate: cancelQuery,
    onSettled: refresh,
  });
  return { rotate, revoke };
}
