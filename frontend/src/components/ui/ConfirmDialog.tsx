import { useEffect } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
  onClose: () => void;
}

// ConfirmDialog is a generic centered confirmation dialog used to replace the
// browser-native confirm(). Native confirm sits at the top of the window, its
// styling is system-determined and uncontrollable; this component is a real
// DOM element, centered and styled to follow the app theme. Delete
// confirmations in other domains can be migrated here uniformly later.
//
// Rendered to document.body via createPortal, detached from the caller's DOM
// tree, to avoid an ancestor's pointer-events:none / transform / z-index
// affecting the dialog's interaction and positioning.
export default function ConfirmDialog({
  open,
  title,
  message,
  confirmText,
  cancelText,
  danger = false,
  loading = false,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  const { t } = useTranslation();

  // Escape closes, Enter confirms. The callback and loading go into the deps
  // so the binding always sees the latest values; registered only when open.
  // Callers that want fewer rebindings can stabilize the callback with
  // useCallback.
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (e.key === "Enter" && !loading) {
        e.preventDefault();
        void onConfirm();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose, loading, onConfirm]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-background rounded-xl shadow-2xl border w-[420px] max-w-[90vw] p-6">
        <button
          onClick={onClose}
          disabled={loading}
          className="absolute top-3 right-3 w-7 h-7 flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors disabled:opacity-50"
        >
          ✕
        </button>

        <h2
          className={`text-base font-semibold mb-3 ${
            danger ? "text-destructive" : ""
          }`}
        >
          {title}
        </h2>

        {message && (
          <p className="text-sm text-muted-foreground mb-5 whitespace-pre-line">
            {message}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            disabled={loading}
            className="h-9 px-4 rounded-md text-sm border hover:bg-muted transition-colors disabled:opacity-50"
          >
            {cancelText ?? t("common.cancel")}
          </button>
          <button
            onClick={() => void onConfirm()}
            disabled={loading}
            className={`h-9 px-4 rounded-md text-sm transition-colors disabled:opacity-50 flex items-center gap-1.5 ${
              danger
                ? "bg-destructive text-destructive-foreground hover:bg-destructive/85"
                : "bg-primary text-primary-foreground hover:bg-primary/90"
            }`}
          >
            {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            {confirmText ?? t("common.confirm")}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
