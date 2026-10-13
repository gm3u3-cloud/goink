import { useState } from "react";
import { Eye, EyeOff, KeyRound, RefreshCw, ShieldOff } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import MCPServerCopyButton from "./MCPServerCopyButton";

export interface MCPServerAccess {
  url: string;
  token: string;
  disabled: boolean;
}

interface Props extends MCPServerAccess {
  onRotate: () => void;
  onRevoke: () => void;
}

export default function MCPServerCredentials({
  url,
  token,
  disabled,
  onRotate,
  onRevoke,
}: Props) {
  const { t } = useTranslation();
  const [revealed, setRevealed] = useState(false);
  return (
    <section
      className="space-y-4 rounded-xl border bg-card p-4"
      aria-labelledby="mcp-server-credentials-title"
    >
      <h4
        id="mcp-server-credentials-title"
        className="flex items-center gap-2 text-sm font-semibold"
      >
        <KeyRound className="h-4 w-4 text-primary" />
        {t("settings.mcpServer.credentials")}
      </h4>
      <div className="space-y-2">
        <label htmlFor="mcp-server-url" className="text-xs font-medium">
          {t("settings.mcpServer.url")}
        </label>
        <div className="flex flex-wrap items-start gap-2">
          <input
            id="mcp-server-url"
            readOnly
            value={url}
            className="h-8 min-w-0 flex-1 rounded-lg border bg-muted/30 px-3 font-mono text-xs"
          />
          <MCPServerCopyButton
            text={url}
            label={t("settings.mcpServer.copyURL")}
            disabled={disabled}
          />
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {t("settings.mcpServer.urlHint")}
        </p>
      </div>
      <div className="space-y-2">
        <label htmlFor="mcp-server-token" className="text-xs font-medium">
          {t("settings.mcpServer.token")}
        </label>
        <div className="flex flex-wrap items-start gap-2">
          <input
            id="mcp-server-token"
            type={revealed ? "text" : "password"}
            readOnly
            value={token}
            autoComplete="off"
            spellCheck={false}
            placeholder={t("settings.mcpServer.noToken")}
            className="h-8 min-w-0 flex-1 rounded-lg border bg-muted/30 px-3 font-mono text-xs"
          />
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(
              revealed
                ? "settings.mcpServer.hideToken"
                : "settings.mcpServer.showToken",
            )}
            disabled={disabled || !token}
            onClick={() => setRevealed(!revealed)}
          >
            {revealed ? <EyeOff /> : <Eye />}
          </Button>
          <MCPServerCopyButton
            text={token}
            label={t("settings.mcpServer.copyToken")}
            disabled={disabled || !token}
          />
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {t("settings.mcpServer.tokenHint")}
        </p>
      </div>
      <div className="flex flex-wrap gap-2 border-t pt-3">
        <Button
          variant="outline"
          size="sm"
          disabled={disabled}
          onClick={onRotate}
        >
          <RefreshCw />
          {t(
            token
              ? "settings.mcpServer.rotateToken"
              : "settings.mcpServer.generateToken",
          )}
        </Button>
        <Button
          variant="destructive"
          size="sm"
          disabled={disabled || !token}
          onClick={onRevoke}
        >
          <ShieldOff />
          {t("settings.mcpServer.revokeToken")}
        </Button>
      </div>
    </section>
  );
}
