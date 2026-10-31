import { HelpCircle } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { explainErrorKey } from "@/utils/errorExplain";
import { cn } from "@/utils/cn";

interface Props {
  testResult?: { ok: boolean; msg?: string; warning?: string } | undefined;
  className?: string;
}

/**
 * Test result display component: error/success message + a "?" icon Tooltip
 * hint on errors.
 * On success with a warning (e.g. 429 rate limit), an additional yellow ⚠
 * note is shown, without affecting the ok decision.
 * Shared by BuiltinProviderPane, CustomProviderPane, and ModelDiscoveryPanel
 * to avoid duplicated code.
 * className can override the default pl-[4rem] left padding (e.g.
 * ModelDiscoveryPanel does not need to align with a label).
 */
export default function TestResultWithHint({ testResult, className }: Props) {
  const { t } = useTranslation();
  if (!testResult) return null;

  return (
    <div
      className={cn(
        "text-xs pl-[4rem] flex items-start gap-1.5",
        testResult.ok ? "text-success-foreground" : "text-destructive",
        className,
      )}
    >
      {!testResult.ok && testResult.msg && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex items-center justify-center w-5 h-5 mt-0.5 rounded-full bg-primary/10 text-primary cursor-help hover:bg-primary/20 transition-colors shrink-0">
              <HelpCircle className="w-3 h-3" />
            </span>
          </TooltipTrigger>
          <TooltipContent side="top" className="max-w-xs">
            <p>{t(explainErrorKey(testResult.msg))}</p>
          </TooltipContent>
        </Tooltip>
      )}
      <span className="whitespace-pre-line">
        {testResult.ok
          ? t("settings.connectionSuccess")
          : `✗ ${testResult.msg || t("settings.connectionFailed")}`}
      </span>
      {testResult.ok && testResult.warning && (
        <span className="whitespace-pre-line text-warning-foreground shrink-0">
          ⚠ {testResult.warning}
        </span>
      )}
    </div>
  );
}
