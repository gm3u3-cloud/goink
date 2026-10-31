import { toast } from "sonner";
import i18n from "@/i18n";

/** Shows an error toast with a copy button so the user can report the
 * specific error message. */
export function toastError(msg: string) {
  return toast.error(msg, {
    action: {
      label: i18n.t("common.copy"),
      onClick: () => navigator.clipboard.writeText(msg),
    },
    actionButtonStyle: {
      backgroundColor: "var(--primary)",
      color: "var(--primary-foreground)",
      border: "none",
      padding: "2px 10px",
      borderRadius: "4px",
      fontSize: "12px",
    },
  });
}

/** Shows a success toast for positive, non-error feedback (e.g. automatic
 * URL completion). */
export function toastSuccess(msg: string) {
  return toast.success(msg);
}
