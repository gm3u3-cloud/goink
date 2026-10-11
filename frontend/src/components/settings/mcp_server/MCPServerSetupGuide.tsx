import { useState } from "react";
import { BookOpen, ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { BrowserOpenURL } from "@/lib/wailsjs/runtime/runtime";
import MCPServerCopyButton from "./MCPServerCopyButton";
import type { MCPServerAccess } from "./MCPServerCredentials";
import {
  createMCPClientConfig,
  mcpClientExamples,
  type MCPExampleClient,
} from "./mcpClientExamples";

export default function MCPServerSetupGuide({
  url,
  token,
  disabled,
}: MCPServerAccess) {
  const { t } = useTranslation();
  const [client, setClient] = useState<MCPExampleClient>("cursor");
  const selected = mcpClientExamples[client];
  return (
    <section
      className="space-y-4 rounded-xl border bg-card p-4"
      aria-labelledby="mcp-server-guide-title"
    >
      <h4
        id="mcp-server-guide-title"
        className="flex items-center gap-2 text-sm font-semibold"
      >
        <BookOpen className="h-4 w-4 text-primary" />
        {t("settings.mcpServer.setupGuide")}
      </h4>
      <ol className="list-decimal space-y-2 pl-4 text-xs leading-relaxed text-muted-foreground">
        <li>{t("settings.mcpServer.setupStep1")}</li>
        <li>{t("settings.mcpServer.setupStep2")}</li>
        <li>{t("settings.mcpServer.setupStep3")}</li>
      </ol>
      <div
        className="flex flex-wrap items-center gap-2"
        role="group"
        aria-label={t("settings.mcpServer.client")}
      >
        {Object.entries(mcpClientExamples).map(([id, item]) => (
          <Button
            key={id}
            size="sm"
            variant={id === client ? "secondary" : "ghost"}
            aria-pressed={id === client}
            onClick={() => setClient(id as MCPExampleClient)}
          >
            {item.name}
          </Button>
        ))}
      </div>
      <p className="text-xs leading-relaxed text-muted-foreground">
        {t(`settings.mcpServer.${selected.hintKey}`, { path: selected.path })}
      </p>
      <div className="overflow-hidden rounded-lg border bg-muted/30">
        <div className="flex items-center justify-between gap-2 border-b px-3 py-2 text-[11px] text-muted-foreground">
          <span className="min-w-0 break-all font-mono">{selected.path}</span>
          <span className="shrink-0 font-medium">{selected.format}</span>
        </div>
        <pre className="overflow-x-auto p-3 text-[11px] leading-relaxed">
          <code>{createMCPClientConfig(client, url, "YOUR_TOKEN")}</code>
        </pre>
      </div>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => BrowserOpenURL(selected.docs)}
        >
          <ExternalLink />
          {t("settings.mcpServer.clientDocs")}
        </Button>
        <MCPServerCopyButton
          text={createMCPClientConfig(client, url, token)}
          label={t("settings.mcpServer.copyConfig")}
          disabled={disabled || !token}
        />
      </div>
      <p className="text-xs leading-relaxed text-muted-foreground">
        {t("settings.mcpServer.configHint")}
      </p>
    </section>
  );
}
