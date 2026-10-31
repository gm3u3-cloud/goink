import { useEffect, useState } from "react";
import { CheckUpdate } from "@/lib/wailsjs/go/app/App";
import type { update } from "@/lib/wailsjs/go/models";

// The auto-update check cycle: first check 30s after startup, then periodic
// rechecks.
// The backend CheckUpdate(false) has a separate 12h throttle as a safety net
// (cross-startup dedup + preventing duplicate calls from HMR/StrictMode).
const UPDATE_FIRST_DELAY_MS = 30_000;
// The polling interval is deliberately slightly larger than the backend's
// 12h throttle: the first check writes last_check 30s after startup, then
// setInterval triggers periodically;
// if it were exactly 12h, the first cycle would be only 11h59m30s after the
// first check and would be blocked by the backend (the first recheck would
// be delayed to 24h).
// The 5-minute margin both eliminates the 30s start offset and absorbs
// setInterval timer drift. Product-wise, 12h vs 12h5m is imperceptible.
const UPDATE_INTERVAL_MS = (12 * 60 + 5) * 60 * 1000; // 12h5m

/**
 * useUpdateCheck encapsulates the scheduled dispatch and result state of the
 * auto-update check.
 *
 * - First check 30s after startup, then a recheck every 12h;
 * - updateResult is written and the dialog opened only when a new version is
 *   found;
 * - timers are cleaned up on component unmount, and a cancelled flag avoids
 *   setState after unmount.
 *
 * The backend CheckUpdate(false) handles the 12h throttle (cross-startup
 * dedup); the frontend only handles periodic triggering.
 */
export function useUpdateCheck() {
  const [showUpdate, setShowUpdate] = useState(false);
  const [updateResult, setUpdateResult] = useState<update.CheckResult | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    const doCheck = async () => {
      if (cancelled) return;
      try {
        const result = await CheckUpdate(false);
        if (cancelled) return;
        if (result && result.hasUpdate) {
          setUpdateResult(result);
          setShowUpdate(true);
        }
      } catch {
        /* Silent failure */
      }
    };
    const firstTimer = setTimeout(doCheck, UPDATE_FIRST_DELAY_MS);
    const interval = setInterval(doCheck, UPDATE_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearTimeout(firstTimer);
      clearInterval(interval);
    };
  }, []);

  return { updateResult, showUpdate, setShowUpdate };
}
