import { CheckCircle2, XCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

interface Props {
  hasKey: boolean;
  testResult?: { ok: boolean; msg?: string; warning?: string } | undefined;
}

/**
 * Five-state provider status badge: not configured / not tested / passed /
 * passed (rate limited) / failed.
 * Shared by Builtin and Custom; replaces the old "configured/not configured"
 * two-state badge (which only reflected the weak hasKey semantics).
 *
 * State priority:
 * 1. Not configured (hasKey=false) → gray
 * 2. Configured but not tested (testResult=undefined) → gray
 * 3. Tested and passed (ok=true, warning empty) → green
 * 4. Tested and passed with warning (ok=true, warning non-empty, e.g. 429)
 *    → yellow
 * 5. Tested and failed (ok=false) → red
 */
export default function ProviderStatusBadge({ hasKey, testResult }: Props) {
  const { t } = useTranslation();

  if (!hasKey) {
    return (
      <span className="flex items-center gap-1 text-xs shrink-0 text-muted-foreground">
        {t("settings.notConfigured")}
      </span>
    );
  }
  if (!testResult) {
    return (
      <span className="flex items-center gap-1 text-xs shrink-0 text-muted-foreground">
        {t("settings.notTested")}
      </span>
    );
  }
  if (testResult.ok) {
    if (testResult.warning) {
      return (
        <span className="flex items-center gap-1 text-xs shrink-0 text-warning-foreground">
          <CheckCircle2 className="w-3.5 h-3.5" />
          {t("settings.passedWithWarning")}
        </span>
      );
    }
    return (
      <span className="flex items-center gap-1 text-xs shrink-0 text-success-foreground">
        <CheckCircle2 className="w-3.5 h-3.5" />
        {t("settings.passed")}
      </span>
    );
  }
  return (
    <span className="flex items-center gap-1 text-xs shrink-0 text-destructive">
      <XCircle className="w-3.5 h-3.5" />
      {t("settings.failed")}
    </span>
  );
}
