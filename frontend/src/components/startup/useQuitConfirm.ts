import { useCallback, useEffect, useState } from "react";
import { CancelQuit, ConfirmQuit } from "@/lib/wailsjs/go/app/App";
import { EventsOn } from "@/lib/wailsjs/runtime/runtime";

export interface QuitConfirmStatus {
  /** The backend requested quit confirmation; the close-window confirm
   * dialog should be shown. */
  open: boolean;
  /** The user confirmed quitting: notify the backend to close the window. */
  confirm: () => void;
  /** The user chose to keep waiting: clear the backend's pending-confirmation
   * flag so the next close attempt is dissuaded again. */
  cancel: () => void;
}

/**
 * Subscribes to the backend's close-window confirmation request, keeping
 * "dialog + sending the user's choice back" inside the hook.
 *
 * The confirmation is initiated by the backend: OnBeforeClose intercepts
 * closing during initialization and emits app:quit-confirm, because Wails v2's
 * MessageDialog ignores custom buttons on Windows/Linux and cannot express
 * the "keep waiting / exit" option pair.
 *
 * Both choices must be sent back to the backend; it uses a flag to decide
 * whether the next close attempt should be dissuaded once more or allowed
 * through directly:
 * closing the dialog without cancelling means the user's next close attempt
 * is treated as "closing again during confirmation" and is no longer dissuaded.
 */
export function useQuitConfirm(): QuitConfirmStatus {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const unsubscribe = EventsOn("app:quit-confirm", () => setOpen(true));
    return () => unsubscribe();
  }, []);

  const confirm = useCallback(() => {
    setOpen(false);
    void ConfirmQuit();
  }, []);

  const cancel = useCallback(() => {
    setOpen(false);
    void CancelQuit();
  }, []);

  return { open, confirm, cancel };
}
