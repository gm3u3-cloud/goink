import { useState, useEffect } from "react";
import { Globe, ExternalLink, ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";
import { BrowserOpenURL } from "@/lib/wailsjs/runtime/runtime";
import type { llm } from "@/lib/wailsjs/go/models";
import ModelDiscoveryPanel from "./ModelDiscoveryPanel";
import ProviderIcon from "./ProviderIcon";
import ProviderFormFields from "./ProviderFormFields";
import ProviderDropdown from "./ProviderDropdown";
import ProviderStatusBadge from "./ProviderStatusBadge";

interface Props {
  providers: llm.ProviderView[];
  onUpdate: (key: string, patch: Partial<llm.ProviderView>) => void;
  onAddCustomModel: (providerKey: string, model: llm.ModelInfo) => void;
  onRemoveCustomModel: (providerKey: string, modelId: string) => void;
  onTest: (
    providerKey: string,
  ) => Promise<{ resolvedUrl?: string; error?: string }>;
  testResults: Record<
    string,
    { ok: boolean; msg?: string; warning?: string } | undefined
  >;
  testing: Record<string, boolean>;
  // Parent external directive: when focusNonce increments, setSelectedKey
  // (focusKey) is called; used to switch the view to the failed provider
  // when a save test fails, so the user can locate it
  focusKey?: string;
  focusNonce?: number;
}

export default function BuiltinProviderPane({
  providers,
  onUpdate,
  onAddCustomModel,
  onRemoveCustomModel,
  onTest,
  testResults,
  testing,
  focusKey,
  focusNonce,
}: Props) {
  const { t } = useTranslation();
  const [selectedKey, setSelectedKey] = useState(providers[0]?.key || "");
  const [helpOpen, setHelpOpen] = useState(false);

  // Reset the collapse state when the provider changes (the dropdown is
  // already managed internally by ProviderDropdown)
  useEffect(() => {
    setHelpOpen(false);
  }, [selectedKey]);

  // Sync selectedKey after providers load asynchronously: the parent's
  // providers initial value is [], so on first mount selectedKey is locked
  // to ""; once providers are ready it must self-heal to the first item's
  // key
  useEffect(() => {
    if (providers.length > 0 && !providers.find((p) => p.key === selectedKey)) {
      setSelectedKey(providers[0].key);
    }
  }, [providers, selectedKey]);

  // Parent external directive: when focusNonce increments, force-select
  // focusKey (the focusNonce increment guarantees the trigger even when
  // focusKey is unchanged, e.g. consecutive failures of the same provider)
  useEffect(() => {
    if (focusKey && providers.find((p) => p.key === focusKey)) {
      setSelectedKey(focusKey);
    }
    // Deliberately depends only on focusNonce, not focusKey/providers
    // (when providers change, the effect above handles it; here we only
    // respond to external directives)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusNonce]);

  const provider = providers.find((p) => p.key === selectedKey);
  if (!provider) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        {t("settings.noBuiltinProviders")}
      </div>
    );
  }

  const allExistingIds = new Set([
    ...(provider?.builtin_models || []).map((m) => m.id),
    ...(provider?.custom_models || []).map((m) => m.id),
  ]);

  return (
    <div className="flex flex-col gap-4">
      {/* Provider selection + status */}
      <div className="flex items-center gap-3">
        <label className="text-xs text-muted-foreground w-14 shrink-0">
          {t("settings.provider")}
        </label>
        <ProviderDropdown
          providers={providers}
          selectedKey={selectedKey}
          onSelect={setSelectedKey}
          testResults={testResults}
          renderIcon={(key) => (
            <ProviderIcon
              provider={key}
              className="w-4 h-4 shrink-0 text-muted-foreground"
            />
          )}
        />
        <ProviderStatusBadge
          hasKey={!!provider.api_key}
          testResult={testResults[selectedKey]}
        />
      </div>

      {/* Shared form fields: ChatURL → APIKey → TestResult → Temperature →
       * CustomModelList */}
      <ProviderFormFields
        provider={provider}
        selectedKey={selectedKey}
        onUpdate={onUpdate}
        onTest={onTest}
        onRemoveCustomModel={onRemoveCustomModel}
        testResults={testResults}
        testing={testing}
      />

      {/* Registration link */}
      {provider.platform_url && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-muted-foreground w-14 shrink-0">
            {t("settings.register")}
          </label>
          <button
            onClick={() => BrowserOpenURL(provider.platform_url!)}
            className="flex items-center gap-1.5 h-8 px-2.5 rounded-md border text-xs hover:bg-muted/50 transition-colors max-w-full"
          >
            <Globe className="w-3.5 h-3.5 shrink-0" />
            <span className="truncate">{provider.platform_url}</span>
            <ExternalLink className="w-3 h-3 shrink-0 text-muted-foreground" />
          </button>
        </div>
      )}

      {/* Registration guide */}
      {provider.help_text && (
        <div className="border rounded-md overflow-hidden">
          <button
            onClick={() => setHelpOpen(!helpOpen)}
            className="flex items-center gap-1.5 w-full px-3 py-2 text-xs text-muted-foreground hover:bg-muted/30 transition-colors"
          >
            <ChevronDown
              className={`w-3 h-3 transition-transform duration-200 ${helpOpen ? "rotate-180" : ""}`}
            />
            {t("settings.registerGuide")}
          </button>
          <div
            className={`grid transition-all duration-300 ease-out ${
              helpOpen
                ? "grid-rows-[1fr] opacity-100"
                : "grid-rows-[0fr] opacity-0"
            }`}
          >
            <div className="overflow-hidden">
              <div className="px-3 pb-2 text-xs text-muted-foreground leading-relaxed">
                {provider.help_text}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Built-in models */}
      {provider.builtin_models && provider.builtin_models.length > 0 && (
        <div>
          <div className="text-xs text-muted-foreground mb-2">
            {t("settings.builtinModels")}
          </div>
          <div className="rounded-md border divide-y">
            {provider.builtin_models.map((m) => (
              <div
                key={m.id}
                className="flex items-center justify-between px-3 py-2 bg-muted/30"
              >
                <span className="text-sm">{m.name}</span>
                <span className="text-xs text-muted-foreground">
                  {m.context_window >= 1_000_000
                    ? (m.context_window / 1_000_000).toFixed(0) + "M"
                    : (m.context_window / 1_000).toFixed(0) + "K"}{" "}
                  {t("settings.context")}
                  {m.max_output_tokens > 0 && (
                    <>
                      {" "}
                      · {(m.max_output_tokens / 1_000).toFixed(0)}K{" "}
                      {t("settings.output")}
                    </>
                  )}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <ModelDiscoveryPanel
        key={selectedKey}
        providerName={selectedKey}
        chatUrl={provider.chat_url}
        apiKey={provider.api_key}
        existingIds={allExistingIds}
        onAddModel={(m) => onAddCustomModel(selectedKey, m)}
      />
    </div>
  );
}
