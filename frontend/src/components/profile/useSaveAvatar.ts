import { useMutation } from "@tanstack/react-query";
import { SaveAvatar } from "@/lib/wailsjs/go/app/App";

// useSaveAvatar: save-avatar-file mutation.
// mutationFn directly imports the wailsjs SaveAvatar (no useApp).
// No onSuccess invalidate — the avatar is a file (not query-cached data);
// it relies on the caller's setAvatarKey(prev => prev + 1) to break the
// <img key> and force a refresh (mechanism kept).
// Error handling is done by the caller with try/catch + setAvatarError
// (keeping the component-level inline error display).
// 5.7 commit 2.
export function useSaveAvatar() {
  return useMutation({
    mutationFn: (bytes: number[]) => SaveAvatar(bytes),
  });
}
