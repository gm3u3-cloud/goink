import { useState } from "react";
import { AlertCircle, Loader2, RefreshCw, Server } from "lucide-react";
import { useTranslation } from "react-i18next";
import { servercfg } from "@/lib/wailsjs/go/models";
import { toErrorMessage } from "@/utils/error";
import { useMCPServerSettings } from "./useMCPServerSettings";
import { useSaveMCPServerSettings } from "./useSaveMCPServerSettings";

export default function MCPServerConfigTab() {
  const { t } = useTranslation();
  const query = useMCPServerSettings();
  const save = useSaveMCPServerSettings();
  const [portDraft, setPortDraft] = useState<string | null>(null);
  const [toolsDraft, setToolsDraft] = useState<string[] | null>(null);
  const [actionError, setActionError] = useState("");
  const [saved, setSaved] = useState(false);
  const view = query.data;

  if (!view) {
    return (
      <div className="flex flex-col items-center justify-center flex-1 gap-3">
        {query.isPending ? (
          <>
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
            <p className="text-sm text-muted-foreground" role="status">
              {t("settings.mcpServer.loading")}
            </p>
          </>
        ) : (
          <>
            <p role="alert" className="text-sm text-destructive">
              {toErrorMessage(query.error, t("settings.mcpServer.loadFailed"))}
            </p>
            <button
              onClick={() => void query.refetch()}
              disabled={query.isFetching}
              className="h-8 px-3 rounded-md border text-xs hover:bg-muted disabled:opacity-50"
            >
              {t("settings.mcpServer.refresh")}
            </button>
          </>
        )}
      </div>
    );
  }

  const config = view.config;
  const portValue = portDraft ?? String(config.port);
  const allowedTools = toolsDraft ?? view.config.allowed_tools ?? [];
  const busy = save.isPending;
  const disabled = busy || query.isError;
  const statusKey = view.status.stopping
    ? "stopping"
    : view.status.running
      ? "running"
      : "stopped";
  const statusColor = view.status.stopping
    ? "bg-amber-500/10 text-amber-600 dark:text-amber-400"
    : view.status.running
      ? "bg-primary/10 text-primary"
      : "bg-muted text-muted-foreground";
  const dirty =
    portValue !== String(view.config.port) ||
    allowedTools.length !== (view.config.allowed_tools ?? []).length ||
    allowedTools.some((name) => !view.config.allowed_tools?.includes(name));
  const error = query.isError
    ? toErrorMessage(query.error, t("settings.mcpServer.loadFailed"))
    : actionError || view.last_error;

  async function apply(input: servercfg.Local, clearDraft: boolean) {
    setActionError("");
    setSaved(false);
    try {
      await save.mutateAsync(input);
      if (clearDraft) {
        setPortDraft(null);
        setToolsDraft(null);
        setSaved(true);
      }
    } catch (err) {
      setActionError(toErrorMessage(err, t("settings.mcpServer.saveFailed")));
    }
  }

  function saveConfig() {
    const port = Number(portValue);
    if (
      !/^\d+$/.test(portValue) ||
      !Number.isInteger(port) ||
      port < 1024 ||
      port > 32767
    ) {
      setSaved(false);
      setActionError(t("settings.mcpServer.invalidPort"));
      return;
    }
    void apply(
      new servercfg.Local({
        enabled: config.enabled,
        port,
        allowed_tools: allowedTools,
      }),
      true,
    );
  }

  function toggleTool(name: string, allowed: boolean) {
    setToolsDraft(
      allowed
        ? [...allowedTools, name]
        : allowedTools.filter((item) => item !== name),
    );
    setSaved(false);
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="flex items-start justify-between gap-3 mb-5 pr-6">
        <div>
          <h3 className="flex items-center gap-2 text-sm font-medium">
            <Server className="w-4 h-4" />
            {t("settings.mcpServer.title")}
          </h3>
          <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
            {t("settings.mcpServer.description")}
          </p>
        </div>
        <span
          role="status"
          className={`shrink-0 px-2 py-1 rounded-md text-xs ${statusColor}`}
        >
          {t(`settings.mcpServer.${statusKey}`)}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto space-y-5">
        {error && (
          <div
            role="alert"
            className="flex gap-2 p-3 rounded-md border border-destructive/30 bg-destructive/5 text-xs text-destructive break-words"
          >
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span className="min-w-0 whitespace-pre-wrap">{error}</span>
          </div>
        )}

        <section className="rounded-lg border p-4 space-y-3">
          <div className="flex items-center justify-between gap-3">
            <label htmlFor="mcp-server-enabled" className="text-sm font-medium">
              {t("settings.mcpServer.enabled")}
            </label>
            <input
              id="mcp-server-enabled"
              type="checkbox"
              role="switch"
              checked={view.config.enabled}
              disabled={disabled}
              onChange={(event) =>
                void apply(
                  new servercfg.Local({
                    ...view.config,
                    enabled: event.target.checked,
                  }),
                  false,
                )
              }
              className="h-4 w-4 accent-primary disabled:opacity-50"
            />
          </div>
          <p className="text-xs text-muted-foreground leading-relaxed">
            {t("settings.mcpServer.enabledHint")}
          </p>
          {view.config.enabled &&
            !view.status.running &&
            !view.status.stopping && (
              <p className="text-xs text-amber-600 dark:text-amber-400">
                {t("settings.mcpServer.enabledNotRunning")}
              </p>
            )}
        </section>

        <section className="rounded-lg border p-4 space-y-3">
          <label
            htmlFor="mcp-server-port"
            className="block text-xs font-medium"
          >
            {t("settings.mcpServer.port")}
          </label>
          <input
            id="mcp-server-port"
            type="number"
            min={1024}
            max={32767}
            step={1}
            value={portValue}
            disabled={disabled}
            onChange={(event) => {
              setPortDraft(event.target.value);
              setSaved(false);
            }}
            className="h-8 w-32 rounded-md border bg-background px-3 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50"
          />
          <p className="text-xs text-muted-foreground leading-relaxed">
            {t("settings.mcpServer.portHint")}
          </p>
        </section>

        <section className="rounded-lg border p-4 space-y-3">
          <h4 className="text-xs font-medium">
            {t("settings.mcpServer.tools")}
          </h4>
          <p className="text-xs text-muted-foreground leading-relaxed">
            {t("settings.mcpServer.toolsHint")}
          </p>
          <div className="space-y-3">
            <label className="flex items-start gap-2 text-xs">
              <input
                type="checkbox"
                checked
                disabled
                className="mt-0.5 h-3.5 w-3.5 accent-primary"
              />
              <span>
                <span className="font-medium">
                  {t("settings.mcpServer.currentNovel")}
                </span>
                <span className="ml-2 text-muted-foreground">
                  {t("settings.mcpServer.builtin")}
                </span>
                <code className="block mt-1 text-[11px] text-muted-foreground">
                  get_current_novel
                </code>
              </span>
            </label>
            {view.available_tools.map((name) => (
              <label key={name} className="flex items-start gap-2 text-xs">
                <input
                  type="checkbox"
                  aria-label={
                    name === "get_chapter_list"
                      ? t("settings.mcpServer.chapterList")
                      : name
                  }
                  checked={allowedTools.includes(name)}
                  disabled={disabled}
                  onChange={(event) => toggleTool(name, event.target.checked)}
                  className="mt-0.5 h-3.5 w-3.5 accent-primary"
                />
                <span>
                  <span className="font-medium">
                    {name === "get_chapter_list"
                      ? t("settings.mcpServer.chapterList")
                      : name}
                  </span>
                  <code className="block mt-1 text-[11px] text-muted-foreground">
                    {name}
                  </code>
                </span>
              </label>
            ))}
          </div>
        </section>
      </div>

      <div className="flex items-center justify-between gap-3 pt-4 border-t mt-4">
        <button
          onClick={() => {
            setActionError("");
            void query.refetch();
          }}
          disabled={busy || query.isFetching}
          className="inline-flex items-center gap-1.5 h-8 px-3 rounded-md border text-xs hover:bg-muted disabled:opacity-50"
        >
          <RefreshCw
            className={`w-3.5 h-3.5 ${query.isFetching ? "animate-spin" : ""}`}
          />
          {t("settings.mcpServer.refresh")}
        </button>
        <div className="flex items-center gap-3">
          {saved && (
            <span role="status" className="text-xs text-primary">
              {t("settings.mcpServer.saved")}
            </span>
          )}
          <button
            onClick={saveConfig}
            disabled={
              disabled || (!dirty && !view.last_error && !view.status.stopping)
            }
            className="h-8 px-4 rounded-md bg-primary text-primary-foreground text-xs disabled:opacity-50"
          >
            {busy ? t("common.saving") : t("settings.mcpServer.save")}
          </button>
        </div>
      </div>
    </div>
  );
}
