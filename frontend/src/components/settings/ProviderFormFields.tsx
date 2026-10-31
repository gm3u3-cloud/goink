import { X, Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { llm } from "@/lib/wailsjs/go/models";
import TemperatureInfo from "./TemperatureInfo";
import TestResultWithHint from "./TestResultWithHint";

interface Props {
  provider: llm.ProviderView;
  selectedKey: string;
  onUpdate: (key: string, patch: Partial<llm.ProviderView>) => void;
  onTest: (
    providerKey: string,
  ) => Promise<{ resolvedUrl?: string; error?: string }>;
  onRemoveCustomModel: (providerKey: string, modelId: string) => void;
  testResults: Record<
    string,
    { ok: boolean; msg?: string; warning?: string } | undefined
  >;
  testing: Record<string, boolean>;
  // Extra className for the custom model list container (Custom uses
  // "mb-2" to separate it from the ModelDiscoveryPanel below; Builtin does
  // not)
  customModelListClassName?: string;
}

/**
 * Shared Provider form fields component: ChatURL + APIKey + TestResult +
 * Temperature + CustomModelList.
 * Used by both BuiltinProviderPane and CustomProviderPane to avoid
 * duplicated code.
 *
 * Field order is unified as ChatURL → APIKey → TestResult → Temperature →
 * CustomModelList (Builtin was originally APIKey → ChatURL; it was unified
 * to ChatURL → APIKey, which is more reasonable: fill in the endpoint first,
 * then the key).
 *
 * Label width is unified to w-16 (Builtin was originally w-14; unified to
 * w-16 as more general).
 * The Chat URL hint padding is unified to pl-[5rem] (matching w-16 + gap-3).
 *
 * Builtin-only registration link/guide/built-in models and Custom-only
 * name/new form/delete button are inserted by their respective Panes before
 * and after the ProviderFormFields call, not handled in this component.
 */
export default function ProviderFormFields({
  provider,
  selectedKey,
  onUpdate,
  onTest,
  onRemoveCustomModel,
  testResults,
  testing,
  customModelListClassName = "",
}: Props) {
  const { t } = useTranslation();
  const isTesting = testing[selectedKey];
  const testResult = testResults[selectedKey];

  return (
    <>
      {/* Chat URL */}
      <div className="flex items-center gap-3">
        <label className="text-xs text-muted-foreground w-16 shrink-0">
          {t("settings.chatUrl")}
        </label>
        <input
          value={provider.chat_url}
          onChange={(e) => onUpdate(selectedKey, { chat_url: e.target.value })}
          className="flex-1 h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        />
      </div>
      <p className="text-xs text-muted-foreground pl-[5rem]">
        {t("settings.urlAutoDetectHint")}
      </p>

      {/* API Key + test button */}
      <div className="flex items-center gap-2">
        <label className="text-xs text-muted-foreground w-16 shrink-0">
          {t("settings.apiKey")}
        </label>
        <input
          type="password"
          value={provider.api_key}
          onChange={(e) => onUpdate(selectedKey, { api_key: e.target.value })}
          placeholder={t("settings.enterApiKey")}
          className="flex-1 h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        />
        <button
          onClick={() => onTest(selectedKey)}
          disabled={!provider.api_key || isTesting}
          className="h-8 px-2.5 rounded-md border text-xs shrink-0 hover:bg-muted/50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {isTesting ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            t("settings.test")
          )}
        </button>
      </div>

      {/* Test result */}
      <TestResultWithHint testResult={testResult} />

      {/* Temperature slider */}
      <div className="flex items-center gap-3">
        <label className="text-xs text-muted-foreground w-16 shrink-0 flex items-center gap-1">
          {t("settings.creativity")}
          <TemperatureInfo />
        </label>
        <input
          type="range"
          min="0"
          max="2"
          step="0.1"
          value={provider.temperature}
          onChange={(e) =>
            onUpdate(selectedKey, { temperature: parseFloat(e.target.value) })
          }
          className="flex-1 h-8"
        />
        <span className="text-xs text-muted-foreground w-8 text-right">
          {(provider.temperature ?? 0.7).toFixed(1)}
        </span>
      </div>

      {/* Custom model list */}
      {provider.custom_models && provider.custom_models.length > 0 && (
        <div>
          <span className="text-xs text-muted-foreground mb-2 block">
            {t("settings.customModels")}
          </span>
          <div
            className={`rounded-md border divide-y ${customModelListClassName}`}
          >
            {provider.custom_models.map((m) => (
              <div
                key={m.id}
                className="flex items-center justify-between px-3 py-2"
              >
                <div>
                  <span className="text-sm">{m.name || m.id}</span>
                  {(m.context_window > 0 || m.max_output_tokens > 0) && (
                    <span className="text-xs text-muted-foreground ml-2">
                      {m.context_window > 0 &&
                        (m.context_window >= 1_000_000
                          ? (m.context_window / 1_000_000).toFixed(0) + "M"
                          : (m.context_window / 1_000).toFixed(0) + "K")}
                      {m.max_output_tokens > 0 && (
                        <>
                          {" "}
                          · {(m.max_output_tokens / 1_000).toFixed(0)}K{" "}
                          {t("settings.output")}
                        </>
                      )}
                      {m.supports_thinking ? (
                        <> · {t("settings.thinking")}</>
                      ) : null}
                      {m.reasoning_levels?.length ? (
                        <>
                          {" "}
                          · {t("settings.level")}:{" "}
                          {m.reasoning_levels.join(",")}
                        </>
                      ) : null}
                      {m.supports_vision ? (
                        <> · {t("settings.vision")}</>
                      ) : null}
                    </span>
                  )}
                </div>
                <button
                  onClick={() => onRemoveCustomModel(selectedKey, m.id)}
                  className="text-muted-foreground hover:text-destructive transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
