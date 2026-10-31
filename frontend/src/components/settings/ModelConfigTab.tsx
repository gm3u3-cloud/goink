import { useState, useEffect, useCallback, useRef } from "react";
import { Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { llm } from "@/lib/wailsjs/go/models";
import { toastSuccess } from "@/utils/toast";
import { useSaveLLMConfig } from "./useSaveLLMConfig";
import { useLLMConfig } from "./useLLMConfig";
import { useTestConnection } from "./useTestConnection";
import BuiltinProviderPane from "./BuiltinProviderPane";
import CustomProviderPane from "./CustomProviderPane";
import ConfirmDialog from "@/components/ui/ConfirmDialog";

type SubNav = "builtin" | "custom";

export default function ModelConfigTab() {
  const { t } = useTranslation();
  const saveMutation = useSaveLLMConfig();
  const llmConfigQuery = useLLMConfig();
  const testConnectionMutation = useTestConnection();
  const [providers, setProviders] = useState<llm.ProviderView[]>([]);
  const [subNav, setSubNav] = useState<SubNav>("builtin");
  const [isSaving, setIsSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState("");
  // saveMsg kind drives the color (replaces the old text-matching on the
  // localized message, which broke once saveMsg is no longer Chinese)
  const [saveMsgKind, setSaveMsgKind] = useState<
    "success" | "error" | "testing" | "info"
  >("info");

  // Test state: { providerKey: { ok, msg, warning, apiKey } }
  // warning is for "passed with a note" scenarios like 429 rate limiting; it
  // does not affect the ok decision
  const [testResults, setTestResults] = useState<
    Record<
      string,
      | { ok: boolean; msg?: string; warning?: string; keySnapshot: string }
      | undefined
    >
  >({});
  const [testing, setTesting] = useState<Record<string, boolean>>({});
  // Set of provider keys the user has acknowledged as failed.
  // Once a provider test fails during save it joins this set, and the next
  // save skips retesting it (unless the user changed that provider's api_key
  // or chat_url, which handleUpdateProvider clears).
  const [acknowledgedFailures, setAcknowledgedFailures] = useState<Set<string>>(
    new Set(),
  );
  // Saved config hashes, used to detect whether a key was modified
  const savedKeysRef = useRef<Record<string, string>>({});

  // "External directive" from the parent to the child's selectedKey: focusKey
  // names the provider key to select; focusNonce increments on every focusOn
  // to trigger the child's useEffect (forcing selection even when focusKey is
  // unchanged).
  // The child keeps sovereignty over selectedKey (user manual switching,
  // providers async load sync, etc. are unchanged); the parent only
  // focusOn's the failed provider when a save test fails, to help the user
  // locate it.
  const [focusKey, setFocusKey] = useState("");
  const [focusNonce, setFocusNonce] = useState(0);
  const focusOn = useCallback((key: string) => {
    setFocusKey(key);
    setFocusNonce((n) => n + 1);
  }, []);

  // Failure confirmation dialog: if a provider test fails during save, show
  // this dialog asking whether to keep the config.
  // pendingFailure is the info shown to the user (providerKey/name/error);
  // resolverRef holds the Promise's resolve; after the user clicks, true
  // (keep) / false (cancel) is injected.
  // handleSave pauses at await confirmFailure and continues the remaining
  // provider tests (or returns) once the user chooses.
  const [pendingFailure, setPendingFailure] = useState<{
    providerKey: string;
    name: string;
    error: string;
  } | null>(null);
  const resolverRef = useRef<((ok: boolean) => void) | null>(null);
  const confirmFailure = useCallback(
    (providerKey: string, name: string, error: string): Promise<boolean> => {
      return new Promise((resolve) => {
        resolverRef.current = resolve;
        setPendingFailure({ providerKey, name, error });
      });
    },
    [],
  );
  const resolveFailure = useCallback((ok: boolean) => {
    if (resolverRef.current) {
      resolverRef.current(ok);
      resolverRef.current = null;
    }
    setPendingFailure(null);
  }, []);

  // One-shot initialization of providers/savedKeysRef/testResults once the
  // query data is ready (providers is local edit state; the later
  // handleUpdateProvider/handleTest/handleSave all mutate local state and
  // never write back to the query cache).
  // GET errors are handled by the global middleware (llm-config prefix →
  // settings.llmConfigLoadFailed), not here.
  //
  // testResults init: mark every provider with a key as ok=true by default
  // (keySnapshot=api_key).
  // Semantics: backend-saved config is assumed valid by default (it passed
  // the test at last save); providers whose url/key were not changed after
  // opening are not retested; changing api_key/chat_url makes
  // handleUpdateProvider clear testResults to trigger a retest.
  // Cost: if the backend URL suddenly dies or the user edits the config
  // file externally, it is assumed passing by default; the user can still
  // find out by clicking test manually.
  useEffect(() => {
    const config = llmConfigQuery.data;
    if (config?.providers) {
      setProviders(config.providers);
      const keys: Record<string, string> = {};
      const results: Record<
        string,
        | { ok: boolean; msg?: string; warning?: string; keySnapshot: string }
        | undefined
      > = {};
      for (const p of config.providers) {
        if (p.api_key) {
          keys[p.key] = p.api_key;
          results[p.key] = { ok: true, keySnapshot: p.api_key };
        }
      }
      savedKeysRef.current = keys;
      setTestResults(results);
    }
  }, [llmConfigQuery.data]);

  const builtinProviders = providers.filter((p) => p.source === "builtin");
  const customProviders = providers.filter((p) => p.source === "custom");

  const handleUpdateProvider = useCallback(
    (key: string, patch: Partial<llm.ProviderView>) => {
      setProviders((prev) =>
        prev.map((p) =>
          p.key === key
            ? ({ ...p, ...patch } as unknown as llm.ProviderView)
            : p,
        ),
      );
      // Any api_key or chat_url change invalidates the old testResults
      // (dirty check: untested / key changed / url changed all require a
      // retest on next save, otherwise an unverified new URL would be saved)
      if ("api_key" in patch || "chat_url" in patch) {
        setTestResults((prev) => {
          // Optimization: only api_key changed and the value is unchanged
          // (onChange fired but the content is identical) — skip
          if (
            "api_key" in patch &&
            !("chat_url" in patch) &&
            prev[key]?.keySnapshot === patch.api_key
          ) {
            return prev;
          }
          const next = { ...prev };
          delete next[key];
          return next;
        });
        // After changing api_key/chat_url, the old "acknowledged failure"
        // state is also invalidated: next save should retest
        setAcknowledgedFailures((prev) => {
          if (!prev.has(key)) return prev;
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
      }
    },
    [],
  );

  const handleAddCustomProvider = useCallback((provider: llm.ProviderView) => {
    setProviders((prev) => [...prev, provider]);
  }, []);

  const handleRemoveCustomProvider = useCallback((key: string) => {
    setProviders((prev) => prev.filter((p) => p.key !== key));
  }, []);

  const handleAddCustomModel = useCallback(
    (providerKey: string, model: llm.ModelInfo) => {
      setProviders((prev) =>
        prev.map((p) => {
          if (p.key !== providerKey) return p;
          const models = [...(p.custom_models || []), model];
          return { ...p, custom_models: models } as unknown as llm.ProviderView;
        }),
      );
    },
    [],
  );

  const handleRemoveCustomModel = useCallback(
    (providerKey: string, modelId: string) => {
      setProviders((prev) =>
        prev.map((p) => {
          if (p.key !== providerKey) return p;
          const models = (p.custom_models || []).filter(
            (m) => m.id !== modelId,
          );
          return { ...p, custom_models: models } as unknown as llm.ProviderView;
        }),
      );
    },
    [],
  );

  // Test connectivity; returns { resolvedUrl?, warning?, error? }.
  // The backend performs a real test with multi-layer fallback and returns
  // the verified URL (which may differ from the input).
  // On success, resolvedUrl is written back to provider.chat_url so the
  // saved URL matches the one that was tested.
  // A non-empty warning means "passed with a note" (e.g. 429 rate limit); it
  // does not affect the ok decision but is shown to the user.
  const handleTest = useCallback(
    async (
      providerKey: string,
    ): Promise<{ resolvedUrl?: string; warning?: string; error?: string }> => {
      const provider = providers.find((p) => p.key === providerKey);
      if (!provider || !provider.api_key) {
        const msg = t("settings.apiKeyNotConfigured");
        setTestResults((prev) => ({
          ...prev,
          [providerKey]: { ok: false, msg, keySnapshot: "" },
        }));
        return { error: msg };
      }

      const models = provider.builtin_models?.length
        ? provider.builtin_models
        : provider.custom_models;
      const modelId = models?.[0]?.id;
      if (!modelId) {
        const msg = t("settings.pleaseAddModel");
        setTestResults((prev) => ({
          ...prev,
          [providerKey]: { ok: false, msg, keySnapshot: provider.api_key },
        }));
        return { error: msg };
      }

      setTesting((prev) => ({ ...prev, [providerKey]: true }));
      try {
        // Backend expandChatURLCandidates completes https:// and applies
        // multi-layer fallback; the frontend passes the original value
        // directly and no longer normalizes on its own.
        // The backend returns { url, warning }: url is the verified actual
        // endpoint, warning is a non-fatal note
        const result = await testConnectionMutation.mutateAsync({
          provider_name: providerKey,
          chat_url: provider.chat_url || "",
          api_key: provider.api_key,
          model_id: modelId,
        });
        const resolvedUrl = result.url;
        const warning = result.warning;
        // Write back the probed correct URL (multi-layer fallback may differ
        // from the original value)
        if (resolvedUrl && resolvedUrl !== provider.chat_url) {
          setProviders((prev) =>
            prev.map((p) =>
              p.key === providerKey
                ? ({
                    ...p,
                    chat_url: resolvedUrl,
                  } as unknown as llm.ProviderView)
                : p,
            ),
          );
          // Tell the user the URL was auto-completed (avoids surprising
          // changes to the input field)
          toastSuccess(t("settings.urlAutoCompleted", { url: resolvedUrl }));
        }
        setTestResults((prev) => ({
          ...prev,
          [providerKey]: {
            ok: true,
            warning,
            keySnapshot: provider.api_key,
          },
        }));
        // Test passed: clear the old "acknowledged failure" state (if any)
        setAcknowledgedFailures((prev) => {
          if (!prev.has(providerKey)) return prev;
          const next = new Set(prev);
          next.delete(providerKey);
          return next;
        });
        return { resolvedUrl, warning };
      } catch (err: any) {
        const msg = String(err).replace(/^app: test connection: /, "");
        setTestResults((prev) => ({
          ...prev,
          [providerKey]: { ok: false, msg, keySnapshot: provider.api_key },
        }));
        // Do not auto-add to acknowledgedFailures here: whether to keep a
        // failed config is decided by the user via the confirm dialog in
        // handleSave (after the user changes api_key/chat_url,
        // handleUpdateProvider clears it, triggering a retest)
        return { error: msg };
      } finally {
        setTesting((prev) => ({ ...prev, [providerKey]: false }));
      }
    },
    [providers, testConnectionMutation.mutateAsync, t],
  );

  const handleSave = useCallback(async () => {
    // Collect providers that have a key
    const withKey = providers.filter((p) => p.api_key);
    if (withKey.length === 0) {
      // The error message is not cleared (kept until the next successful save
      // or a new error overwrites it), so the user can read the error
      setSaveMsgKind("error");
      setSaveMsg(t("settings.pleaseConfigureApiKey"));
      return;
    }

    // Find the ones that need testing: never tested / key changed / last
    // failed but not acknowledged by the user
    const needTest = withKey.filter((p) => {
      const tr = testResults[p.key];
      if (!tr) return true; // never tested
      if (!tr.ok) {
        // Last time failed: skip if the user acknowledged it (avoids
        // retesting a known failure on every save); after the user changes
        // api_key/chat_url, handleUpdateProvider clears it, triggering a
        // retest
        return !acknowledgedFailures.has(p.key);
      }
      if (tr.keySnapshot !== p.api_key) return true; // key changed
      return false;
    });

    // The correct URLs probed during testing, used to override the save data
    // (the closure providers is stale; resolvedUrls is the fallback)
    const resolvedUrls: Record<string, string> = {};
    // Collect the names of failed providers the user chose to "keep" in this
    // save, for a final summary message
    const keptFailures: string[] = [];
    if (needTest.length > 0) {
      setSaveMsgKind("testing");
      setSaveMsg(t("settings.testingConnection"));
      for (const p of needTest) {
        const result = await handleTest(p.key);
        if (result.error) {
          // Show the confirm dialog: the user decides whether to keep this
          // failed config
          // Choose "keep" → add to acknowledgedFailures, skip this provider on
          // next save, continue testing the next one
          // Choose "cancel" → do not save; switch to the failed provider view
          // so the user can fix it first
          const keep = await confirmFailure(p.key, p.name, result.error);
          if (!keep) {
            // The error message is not cleared (kept until the next successful
            // save or a new error overwrites it), so the user can read the
            // error
            // The ✗ prefix makes it obvious at a glance that this is a
            // failure message (the color is already red; the symbol
            // reinforces recognition)
            setSaveMsgKind("error");
            setSaveMsg(
              `✗ ${p.name} ${t("settings.connectionTestFailed")}: ${result.error}`,
            );
            // Switch to the view containing the failed provider and select
            // that provider, so the user can locate it easily
            setSubNav(p.source === "builtin" ? "builtin" : "custom");
            focusOn(p.key);
            return;
          }
          // The user chose "keep": mark as acknowledged failure; next save
          // skips this provider's test automatically (unless the user changed
          // api_key/chat_url, in which case handleUpdateProvider clears it
          // and triggers a retest)
          setAcknowledgedFailures((prev) => {
            const next = new Set(prev);
            next.add(p.key);
            return next;
          });
          // Record the kept failed provider name for the final summary
          keptFailures.push(p.name);
          // This provider is kept as a known failure; do not modify the URL in
          // the save data (do not write to resolvedUrls; use the original
          // value from providers)
          continue;
        }
        if (result.resolvedUrl) {
          resolvedUrls[p.key] = result.resolvedUrl;
        }
      }
    }

    setIsSaving(true);
    setSaveMsgKind("info");
    setSaveMsg("");
    try {
      // Override chat_url with resolvedUrls to build the save data
      // (handleTest already wrote the UI back via setProviders; here we sync
      // the save data)
      const providersToSave = providers.map((p) => {
        const resolved = resolvedUrls[p.key];
        return resolved && resolved !== p.chat_url
          ? ({ ...p, chat_url: resolved } as unknown as llm.ProviderView)
          : p;
      });
      await saveMutation.mutateAsync({
        providers: providersToSave,
      } as unknown as llm.LLMConfigView);
      const keys: Record<string, string> = {};
      for (const p of providers) {
        if (p.api_key) keys[p.key] = p.api_key;
      }
      savedKeysRef.current = keys;
      // Build the success message based on keptFailures: show the full
      // notice when there are kept failures, otherwise just "config saved"
      const successMsg =
        keptFailures.length > 0
          ? t("settings.configSavedWithKept", {
              names: keptFailures.join(t("common.listSeparator")),
            })
          : t("settings.configSaved");
      // The ✓ prefix makes it obvious at a glance that this is a success
      // message (the color is already green; the symbol reinforces
      // recognition)
      // saveMsg is not actively cleared; it stays displayed until the next
      // save (success/failure/testing) result overwrites it
      setSaveMsgKind("success");
      setSaveMsg(`✓ ${successMsg}`);
      // Additionally notify save success via a toast (project toast style)
      toastSuccess(successMsg);
    } catch (err) {
      setSaveMsgKind("error");
      setSaveMsg(`✗ ${t("settings.saveFailed")}: ${String(err)}`);
    } finally {
      setIsSaving(false);
    }
  }, [
    providers,
    saveMutation.mutateAsync,
    testResults,
    acknowledgedFailures,
    handleTest,
    focusOn,
    confirmFailure,
    t,
  ]);

  if (llmConfigQuery.isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      {/* Sub navigation */}
      <div className="flex gap-6 px-1 mb-4">
        <button
          onClick={() => setSubNav("builtin")}
          className={`text-sm pb-1 transition-colors ${
            subNav === "builtin"
              ? "text-foreground border-b-2 border-primary font-medium"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {t("settings.builtinProviders")}
        </button>
        <button
          onClick={() => setSubNav("custom")}
          className={`text-sm pb-1 transition-colors ${
            subNav === "custom"
              ? "text-foreground border-b-2 border-primary font-medium"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {t("settings.customProviders")}
        </button>
      </div>

      {/* Content area */}
      <div className="flex-1 overflow-y-auto">
        {subNav === "builtin" ? (
          <BuiltinProviderPane
            providers={builtinProviders}
            onUpdate={handleUpdateProvider}
            onAddCustomModel={handleAddCustomModel}
            onRemoveCustomModel={handleRemoveCustomModel}
            onTest={handleTest}
            testResults={testResults}
            testing={testing}
            focusKey={focusKey}
            focusNonce={focusNonce}
          />
        ) : (
          <CustomProviderPane
            providers={customProviders}
            onAdd={handleAddCustomProvider}
            onUpdate={handleUpdateProvider}
            onRemove={handleRemoveCustomProvider}
            onAddCustomModel={handleAddCustomModel}
            onRemoveCustomModel={handleRemoveCustomModel}
            onTest={handleTest}
            testResults={testResults}
            testing={testing}
            focusKey={focusKey}
            focusNonce={focusNonce}
          />
        )}
      </div>

      {/* Bottom save bar */}
      <div className="flex items-end justify-between gap-3 pt-4 border-t mt-4">
        <span
          className={`flex-1 text-xs whitespace-pre-line max-w-[60ch] text-left ${saveMsgKind === "error" || saveMsgKind === "testing" ? "text-destructive" : "text-success-foreground"}`}
        >
          {saveMsg}
        </span>
        <button
          onClick={handleSave}
          disabled={isSaving}
          className="h-8 px-4 rounded-md bg-primary text-primary-foreground text-sm disabled:opacity-50 shrink-0"
        >
          {isSaving ? t("common.saving") : t("settings.saveConfig")}
        </button>
      </div>

      {/* Confirmation dialog shown when a save test fails: the user decides
       * whether to keep the failed config */}
      <ConfirmDialog
        open={pendingFailure !== null}
        title={t("settings.confirmKeepFailureTitle")}
        message={
          pendingFailure
            ? t("settings.confirmKeepFailureMessage", {
                name: pendingFailure.name,
                error: pendingFailure.error,
              })
            : ""
        }
        confirmText={t("settings.keepAnyway")}
        cancelText={t("common.cancel")}
        onConfirm={() => resolveFailure(true)}
        onClose={() => resolveFailure(false)}
      />
    </div>
  );
}
