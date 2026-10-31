import { useCallback, useEffect, useRef, useState } from "react";
import { FrontendReady, RetryStartup } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { EventsOn } from "@/lib/wailsjs/runtime/runtime";

/** Debounce window for the migration overlay: the overlay is only shown
 * once the state persists longer than this, avoiding a flash on fast startup. */
const PREPARING_DELAY_MS = 500;

export interface StartupStatus {
  /** Current startup phase; "failed" on handshake failure, null before any
   * snapshot is received. */
  phase: string | null;
  /** Current snapshot version; null means no snapshot has been received. */
  version: number | null;
  /** The startup error to display currently; empty string when none. */
  error: string;
  /** Migrating and past the debounce window; the "preparing data" overlay
   * should be shown. */
  showPreparing: boolean;
  /** Whether this startup was triggered from the first-time config page;
   * used to show onboarding after entering the main view. */
  cameFromInit: boolean;
  /** The "retry" button on the failure page. */
  retry: () => Promise<void>;
  retrying: boolean;
}

/**
 * Subscribes to the backend startup state, keeping the whole startup
 * protocol ("four states + version number + debounce + retry") inside the hook.
 *
 * The backend pushes state via the startup:state event, but events are
 * transient and may arrive out of order:
 * ① Snapshots travel over two independent channels (event push / the
 *    FrontendReady return value), and their order is not guaranteed;
 * ② The frontend registers first and then calls FrontendReady, so the return
 *    value and events arriving afterwards may interleave.
 * Therefore this hook does not compare "spin-up" — it only accepts changes by
 * monotonically increasing Version; older snapshots are always discarded.
 */
export function useStartupState(): StartupStatus {
  const [state, setState] = useState<app.StartupState | null>(null);
  const [bootstrapError, setBootstrapError] = useState("");
  const [retryError, setRetryError] = useState("");
  const [retrying, setRetrying] = useState(false);
  const [cameFromInit, setCameFromInit] = useState(false);
  const [preparingVisibleVersion, setPreparingVisibleVersion] = useState<
    number | null
  >(null);
  const versionRef = useRef(-1);
  const phaseRef = useRef<string | null>(null);

  useEffect(() => {
    let active = true;
    const applyState = (next: app.StartupState) => {
      if (!active || next.version <= versionRef.current) return;

      const previousPhase = phaseRef.current;
      versionRef.current = next.version;
      phaseRef.current = next.phase;
      if (previousPhase === "unconfigured" && next.phase === "initializing") {
        setCameFromInit(true);
      }
      setBootstrapError("");
      setRetryError("");
      setState(next);
    };

    const unsubscribe = EventsOn("startup:state", (data: app.StartupState) => {
      applyState(data);
    });
    // The order cannot be reversed: register the listener first, then notify
    // the backend. FrontendReady returns the snapshot "at this moment";
    // changes after that all arrive via events; changes before that are
    // already included in this return value.
    FrontendReady()
      .then(applyState)
      .catch((error) => {
        if (active) setBootstrapError(String(error));
      });

    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    if (state?.phase !== "initializing") return;

    const version = state.version;
    const timer = window.setTimeout(
      () => setPreparingVisibleVersion(version),
      PREPARING_DELAY_MS,
    );
    return () => window.clearTimeout(timer);
  }, [state?.phase, state?.version]);

  const retry = useCallback(async () => {
    setRetryError("");
    setRetrying(true);
    try {
      await RetryStartup();
    } catch (error) {
      setRetryError(String(error));
    } finally {
      setRetrying(false);
    }
  }, []);

  // A handshake failure (no snapshot received at all) is also normalized to
  // failed, so callers only need to look at the single phase field.
  const phase = bootstrapError ? "failed" : (state?.phase ?? null);
  const error = bootstrapError || retryError || state?.error || "";
  const showPreparing =
    state?.phase === "initializing" &&
    preparingVisibleVersion === state.version;

  return {
    phase,
    version: state?.version ?? null,
    error,
    showPreparing,
    cameFromInit,
    retry,
    retrying,
  };
}
