import { useState, useEffect } from "react";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { llm } from "@/lib/wailsjs/go/models";
import ModelDiscoveryPanel from "./ModelDiscoveryPanel";
import ProviderFormFields from "./ProviderFormFields";
import ProviderDropdown from "./ProviderDropdown";
import ProviderStatusBadge from "./ProviderStatusBadge";
import ConfirmDialog from "@/components/ui/ConfirmDialog";

interface Props {
  providers: llm.ProviderView[];
  onAdd: (provider: llm.ProviderView) => void;
  onUpdate: (key: string, patch: Partial<llm.ProviderView>) => void;
  onRemove: (key: string) => void;
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

export default function CustomProviderPane({
  providers,
  onAdd,
  onUpdate,
  onRemove,
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
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [showNewForm, setShowNewForm] = useState(false);

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

  // Delete a custom provider: clicking the button only records the target
  // and shows the confirm dialog; the actual removal runs in
  // confirmDeleteProvider after confirmation
  const confirmDeleteProvider = () => {
    if (!deleteTarget) return;
    onRemove(deleteTarget);
    setSelectedKey(
      providers.filter((p) => p.key !== deleteTarget)[0]?.key || "",
    );
    setDeleteTarget(null);
  };
  const [newName, setNewName] = useState("");
  const [newChatURL, setNewChatURL] = useState("");
  const [newApiKey, setNewApiKey] = useState("");
  const provider = providers.find((p) => p.key === selectedKey);

  const handleAdd = () => {
    if (!newName.trim() || !newChatURL.trim()) return;
    onAdd({
      key: newName.trim().toLowerCase().replace(/\s+/g, "-"),
      name: newName.trim(),
      chat_url: newChatURL.trim(),
      api_key: newApiKey,
      source: "custom",
      builtin_models: [],
      custom_models: [],
      temperature: 0.7,
    } as unknown as llm.ProviderView);
    setNewName("");
    setNewChatURL("");
    setNewApiKey("");
    setShowNewForm(false);
    // Select the newly added provider
    setSelectedKey(newName.trim().toLowerCase().replace(/\s+/g, "-"));
  };

  // No custom providers and the new-provider form is not expanded
  if (providers.length === 0 && !showNewForm) {
    return (
      <div className="flex flex-col items-center gap-3 py-8">
        <p className="text-sm text-muted-foreground">
          {t("settings.noCustomProviders")}
        </p>
        <button
          onClick={() => setShowNewForm(true)}
          className="flex items-center gap-1 text-xs text-primary hover:underline"
        >
          <Plus className="w-3 h-3" /> {t("settings.addCustomProvider")}
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Provider selection + status + add */}
      <div className="flex items-center gap-3">
        <label className="text-xs text-muted-foreground w-14 shrink-0">
          {t("settings.provider")}
        </label>
        <ProviderDropdown
          providers={providers}
          selectedKey={selectedKey}
          onSelect={setSelectedKey}
          testResults={testResults}
          renderIcon={(_key, name) => {
            // Custom has no built-in icon; use a circular initial fallback
            // Take the first character of name (or "?" if empty),
            // uppercase it, and place it in a bg-primary/15 circle
            const initial = (name || "?").charAt(0).toUpperCase();
            return (
              <div className="w-4 h-4 rounded-full bg-primary/15 text-primary text-[10px] flex items-center justify-center font-medium shrink-0">
                {initial}
              </div>
            );
          }}
        />
        <ProviderStatusBadge
          hasKey={!!provider?.api_key}
          testResult={provider ? testResults[provider.key] : undefined}
        />
        <button
          onClick={() => setShowNewForm(!showNewForm)}
          className="text-xs text-primary flex items-center gap-0.5 hover:underline shrink-0"
        >
          <Plus className="w-3 h-3" /> {t("settings.add")}
        </button>
      </div>

      {/* New provider form */}
      {showNewForm && (
        <div className="border rounded-md p-3 space-y-3">
          <div className="text-xs font-medium">
            {t("settings.newCustomProvider")}
          </div>

          <div className="flex items-center gap-3">
            <label className="text-xs text-muted-foreground w-16 shrink-0">
              {t("common.name")}
            </label>
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={t("settings.providerName")}
              className="flex-1 h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
          </div>

          <div className="flex items-center gap-3">
            <label className="text-xs text-muted-foreground w-16 shrink-0">
              {t("settings.chatUrl")}
            </label>
            <input
              value={newChatURL}
              onChange={(e) => setNewChatURL(e.target.value)}
              placeholder="https://api.example.com/v1/chat/completions"
              className="flex-1 h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
          </div>

          <div className="flex items-center gap-3">
            <label className="text-xs text-muted-foreground w-16 shrink-0">
              {t("settings.apiKey")}
            </label>
            <input
              type="password"
              value={newApiKey}
              onChange={(e) => setNewApiKey(e.target.value)}
              placeholder={t("settings.enterApiKey")}
              className="flex-1 h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
          </div>

          <div className="flex items-center gap-2 pt-1">
            <div className="flex-1" />
            <button
              onClick={() => setShowNewForm(false)}
              className="h-8 px-3 rounded-md border text-xs text-muted-foreground"
            >
              {t("common.cancel")}
            </button>
            <button
              onClick={handleAdd}
              className="h-8 px-3 rounded-md bg-primary text-primary-foreground text-xs"
            >
              {t("settings.add")}
            </button>
          </div>
        </div>
      )}

      {/* Edit area for a selected existing provider */}
      {provider && !showNewForm && (
        <>
          <div className="flex items-center gap-3">
            <label className="text-xs text-muted-foreground w-16 shrink-0">
              {t("common.name")}
            </label>
            <input
              value={provider.name}
              disabled
              className="flex-1 h-8 rounded-md border bg-muted/50 px-2.5 text-sm text-muted-foreground"
            />
          </div>

          {/* Shared form fields: ChatURL → APIKey → TestResult →
           * Temperature → CustomModelList */}
          <ProviderFormFields
            provider={provider}
            selectedKey={selectedKey}
            onUpdate={onUpdate}
            onTest={onTest}
            onRemoveCustomModel={onRemoveCustomModel}
            testResults={testResults}
            testing={testing}
            customModelListClassName="mb-2"
          />

          <ModelDiscoveryPanel
            key={selectedKey}
            providerName={provider.name}
            chatUrl={provider.chat_url}
            apiKey={provider.api_key}
            existingIds={
              new Set((provider?.custom_models || []).map((m) => m.id))
            }
            onAddModel={(m) => onAddCustomModel(selectedKey, m)}
          />

          {/* Delete */}
          <div className="flex pt-1">
            <button
              onClick={() => setDeleteTarget(selectedKey)}
              className="h-8 px-3 rounded-md border border-danger-border text-destructive text-xs hover:bg-danger-bg transition-colors"
            >
              {t("settings.deleteProvider")}
            </button>
          </div>
        </>
      )}
      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("settings.confirmDeleteProvider")}
        message={
          deleteTarget
            ? `"${providers.find((p) => p.key === deleteTarget)?.name || deleteTarget}"? ${t("common.irreversible")}`
            : ""
        }
        danger
        confirmText={t("common.delete")}
        onClose={() => setDeleteTarget(null)}
        onConfirm={confirmDeleteProvider}
      />
    </div>
  );
}
