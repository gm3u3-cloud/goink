import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { GetSettings } from "@/lib/wailsjs/go/app/App";
import { toastError } from "@/utils/toast";
import { toErrorMessage } from "@/utils/error";

/**
 * The final step before the main view is ready: read the "last opened book".
 *
 * Uses phase/version as input, not just phase: every startup-state change
 * (including retries) must refetch; and the main view is not released before
 * the fetch completes, otherwise a bookshelf with novelId=0 would flash first.
 */
export function useWorkspaceBoot(phase: string | null, version: number | null) {
  const { t } = useTranslation();
  const [novelId, setNovelId] = useState(0);
  const [readyVersion, setReadyVersion] = useState<number | null>(null);

  useEffect(() => {
    if (phase !== "ready" || version === null) return;

    let active = true;
    GetSettings()
      .then((settings) => {
        if (active) setNovelId(settings?.last_novel_id ?? 0);
      })
      .catch((error) => {
        if (active) {
          toastError(toErrorMessage(error, t("chat.settingsLoadFailed")));
        }
      })
      .finally(() => {
        if (active) setReadyVersion(version);
      });
    return () => {
      active = false;
    };
  }, [phase, version, t]);

  return {
    novelId,
    ready: phase === "ready" && readyVersion === version,
  };
}
