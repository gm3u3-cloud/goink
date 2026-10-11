import { Radio, Settings2 } from "lucide-react";
import { useTranslation } from "react-i18next";

interface Props {
  enabled: boolean;
  running: boolean;
  stopping: boolean;
  port: string;
  disabled: boolean;
  onEnabledChange: (enabled: boolean) => void;
  onPortChange: (port: string) => void;
}

export default function MCPServerConnection({
  enabled,
  running,
  stopping,
  port,
  disabled,
  onEnabledChange,
  onPortChange,
}: Props) {
  const { t } = useTranslation();
  return (
    <section
      className="overflow-hidden rounded-xl border bg-card"
      aria-label={t("settings.mcpServer.connection")}
    >
      <div className="space-y-3 p-4">
        <div className="flex items-center justify-between gap-3">
          <label
            htmlFor="mcp-server-enabled"
            className="flex items-center gap-2 text-sm font-medium"
          >
            <Radio className="h-4 w-4 text-primary" />
            {t("settings.mcpServer.enabled")}
          </label>
          <input
            id="mcp-server-enabled"
            type="checkbox"
            role="switch"
            checked={enabled}
            disabled={disabled}
            onChange={(event) => onEnabledChange(event.target.checked)}
            className="peer sr-only"
          />
          <label
            htmlFor="mcp-server-enabled"
            aria-hidden="true"
            className="relative h-5 w-9 shrink-0 cursor-pointer rounded-full bg-muted-foreground/25 transition-colors after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-background after:shadow-sm after:transition-transform peer-checked:bg-primary peer-checked:after:translate-x-4 peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 peer-disabled:cursor-default peer-disabled:opacity-50"
          />
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {t("settings.mcpServer.enabledHint")}
        </p>
        {enabled && !running && !stopping && (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            {t("settings.mcpServer.enabledNotRunning")}
          </p>
        )}
      </div>
      <div className="space-y-3 border-t bg-muted/20 p-4">
        <div className="flex items-center justify-between gap-3">
          <label
            htmlFor="mcp-server-port"
            className="flex items-center gap-2 text-xs font-medium"
          >
            <Settings2 className="h-4 w-4 text-muted-foreground" />
            {t("settings.mcpServer.port")}
          </label>
          <input
            id="mcp-server-port"
            type="number"
            min={1024}
            max={32767}
            step={1}
            value={port}
            disabled={disabled}
            onChange={(event) => onPortChange(event.target.value)}
            className="h-8 w-28 rounded-lg border bg-background px-3 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
          />
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {t("settings.mcpServer.portHint")}
        </p>
      </div>
    </section>
  );
}
