/**
 * Converts any unknown error into a readable string.
 *
 * The Wails backend rejects with a plain string (Go err.Error()), not a JS
 * Error object, so instanceof Error is always false and e?.message is always
 * undefined. This function tries in priority order: string → Error →
 * {message} → fallback → String(err).
 *
 * @param err the unknown error from a catch block
 * @param fallback fallback text (usually an i18n t("xxx.saveFailed"))
 */
export function toErrorMessage(err: unknown, fallback?: string): string {
  if (typeof err === "string") return err;
  if (err instanceof Error) return err.message;
  if (err && typeof err === "object" && "message" in err) {
    const m = (err as { message: unknown }).message;
    if (typeof m === "string") return m;
  }
  return fallback ?? String(err);
}
