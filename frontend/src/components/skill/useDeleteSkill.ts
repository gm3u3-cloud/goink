import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteSkill } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { skillKeys } from "@/lib/queryKeys";

// useDeleteSkill: delete-skill mutation.
// mutationFn directly imports the wailsjs DeleteSkill (no useApp).
// Input is app.DeleteSkillInput (backend Pattern 2: novel_id is a struct
// field, not a standalone function parameter); the caller passes the full
// struct {novel_id, name, source}, and the hook forwards it as-is without
// closure-assembly of novel_id.
// Follows the backend API design (consistent with the useInstallRemoteSkill
// pattern), avoiding artificially creating partial types.
// onSuccess invalidates skillKeys.list(novelId): the list auto-refreshes
// (replaces the manual invalidate inside confirmDelete).
// The mutation has no onError; the caller does try/catch + toastError
// (aligned with the character/style pattern).
export function useDeleteSkill(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.DeleteSkillInput) => DeleteSkill(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: skillKeys.list(novelId) });
    },
  });
}
