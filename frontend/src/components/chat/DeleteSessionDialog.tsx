import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import ConfirmDialog from "@/components/ui/ConfirmDialog";
import { toastError } from "@/utils/toast";
import { toErrorMessage } from "@/utils/error";
import { sessionMessagesKeys } from "@/lib/queryKeys";
import { useChatStore } from "./useChatStore";
import { useDeleteSession } from "./useDeleteSession";

interface Props {
  activeSessionId: string | null | undefined;
  onActiveSessionDeleted: () => void;
}

// DeleteSessionDialog: the centralized session-deletion confirm dialog
// (the only entry point in ChatPanel).
// Reads deletingSession from the store and mounts the single ConfirmDialog;
// SessionHistory/RecentSessions delete clicks only dispatch to the store —
// no execution, no ConfirmDialog mounted.
// After a successful deletion: invalidate the full sessions prefix
// (mutation onSuccess) +
//   if the deleted session was active, invalidate messages + clear the view
//   via onActiveSessionDeleted.
// On deletion failure: the caller does try/catch + toastError (the mutation
// has no onError).
export default function DeleteSessionDialog({
  activeSessionId,
  onActiveSessionDeleted,
}: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const deletingSession = useChatStore((s) => s.deletingSession);
  const setDeletingSession = useChatStore((s) => s.setDeletingSession);
  const deleteMutation = useDeleteSession();

  const handleConfirm = async () => {
    if (!deletingSession) return;
    try {
      await deleteMutation.mutateAsync(deletingSession.session_id);
      // If the deleted session was active, invalidate messages + clear the
      // active session view
      if (deletingSession.session_id === activeSessionId) {
        qc.invalidateQueries({
          queryKey: sessionMessagesKeys.detail(deletingSession.session_id),
        });
        onActiveSessionDeleted();
      }
      setDeletingSession(null);
    } catch (err) {
      toastError(t("chat.deleteSessionFailed") + ": " + toErrorMessage(err));
    }
  };

  return (
    <ConfirmDialog
      open={deletingSession !== null}
      title={t("chat.confirmDeleteSession")}
      message={
        deletingSession
          ? t("chat.confirmDeleteSessionMessage", {
              title: deletingSession.title || t("chat.newChat"),
            })
          : ""
      }
      danger
      loading={deleteMutation.isPending}
      confirmText={t("common.delete")}
      onClose={() => setDeletingSession(null)}
      onConfirm={handleConfirm}
    />
  );
}
