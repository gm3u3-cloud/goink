import { useMutation } from "@tanstack/react-query";
import { CancelChat } from "@/lib/wailsjs/go/app/App";

// useCancelChat: mutation to cancel an in-progress chat streaming response.
// The backend CancelChat only cancels the context + clears the cancelMgr
// map, writing no persisted data, so no settings invalidation is needed
// (unlike setter mutations, which do change settings fields).
// Error handling is done by the caller with try/catch + toastError (keeps
// the component-level toast).
// 5.1 commit 4 addendum.
export function useCancelChat() {
  return useMutation({
    mutationFn: (sessionId: string) => CancelChat(sessionId),
  });
}
