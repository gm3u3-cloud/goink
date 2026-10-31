import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { imp, llm } from "@/lib/wailsjs/go/models";
import {
  ImportNovel,
  PickAndImportNovel,
  ImportWithLLM,
} from "@/lib/wailsjs/go/app/App";
import { EventsOn } from "@/lib/wailsjs/runtime/runtime";
import { toErrorMessage } from "@/utils/error";
import { useModels } from "@/components/settings/useModels";
import { useSettings } from "@/components/settings/useSettings";

export type ImportProgressStage =
  | "idle"
  | "select_file"
  | "parse"
  | "create_novel"
  | "write_chapters"
  | "commit"
  | "done"
  | "error"
  | "needs_llm"
  | "analyzing";

export interface ImportProgressState {
  stage: ImportProgressStage;
  message: string;
  current: number;
  total: number;
  percent: number;
  novel_id?: number;
}

const INITIAL_IMPORT_PROGRESS: ImportProgressState = {
  stage: "idle",
  message: "",
  current: 0,
  total: 0,
  percent: 0,
};

interface UseImportNovelOptions {
  onImported: (result: imp.ImportResult) => Promise<void>;
}

export function useImportNovel({ onImported }: UseImportNovelOptions) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [progress, setProgress] = useState<ImportProgressState>({
    ...INITIAL_IMPORT_PROGRESS,
    message: t("novel.importPreparing2"),
  });
  const [error, setError] = useState("");
  const [skippedCount, setskipped_count] = useState(0);
  const [skippedChapters, setskipped_chapters] = useState<
    { title: string; reason: string }[]
  >([]);

  // LLM fallback related state
  const [filePath, setFilePath] = useState("");
  // selectedModel stores a structured AvailableModel object (with
  // ProviderName/ModelID), replacing the old modelKey string + splitModelKey
  // split. The models list is read-only via query data.
  const [selectedModel, setSelectedModel] = useState<llm.AvailableModel | null>(
    null,
  );

  // 5.9: model list + currently selected model go through the query hooks
  // (useModels 5.1 / useSettings 5.8), replacing the old imperative
  // useEffect app.GetModels + app.GetSettings. After removing the app param
  // we no longer depend on useApp.
  const { data: modelsData = [] } = useModels();
  const { data: settingsData } = useSettings();

  useEffect(() => {
    const unsubscribe = EventsOn(
      "import:progress",
      (data: ImportProgressState) => {
        setProgress({
          stage: data.stage,
          message: data.message,
          current: data.current ?? 0,
          total: data.total ?? 0,
          percent: data.percent ?? 0,
          novel_id: data.novel_id,
        });
        if (data.stage === "error") {
          setError(data.message);
        }
      },
    );
    return unsubscribe;
  }, []);

  // selectedModel selection state: backfilled from settings once query data
  // is ready. The old imperative app.GetModels().then + app.GetSettings().then
  // now uses the query cache; within the 30s staleTime a cache hit avoids a
  // redundant fetch.
  useEffect(() => {
    if (!modelsData?.length) return;
    const key = settingsData?.selected_model_key || "";
    setSelectedModel(
      modelsData.find((m) => m.Key === key) ?? modelsData[0] ?? null,
    );
  }, [modelsData, settingsData]);

  const reset = useCallback(() => {
    setOpen(false);
    setError("");
    setskipped_count(0);
    setskipped_chapters([]);
    setFilePath("");
    setProgress({
      ...INITIAL_IMPORT_PROGRESS,
      message: t("novel.importPreparing2"),
    });
  }, [t]);

  const startImport = useCallback(
    async (fp?: string) => {
      setError("");
      setProgress({
        ...INITIAL_IMPORT_PROGRESS,
        stage: fp ? "parse" : "select_file",
        message: fp ? t("novel.importParsing2") : t("novel.importSelectFile2"),
      });
      setOpen(true);

      let result: imp.ImportResult | null;
      try {
        result = fp
          ? await ImportNovel({ file_path: fp })
          : await PickAndImportNovel();
      } catch (err: unknown) {
        setProgress((prev) => ({
          ...prev,
          stage: "error",
          message: t("novel.importRollbackDone"),
          percent: 100,
        }));
        setError(toErrorMessage(err, t("novel.importFailedRetry")));
        return;
      }

      if (!result) {
        reset();
        return;
      }

      // Regex splitting failed; prompt the user to use AI analysis.
      // Prefer the file_path returned by the backend (under the
      // PickAndImportNovel path fp is undefined).
      if (result.needs_llm) {
        setFilePath(result.file_path || fp || "");
        setProgress((prev) => ({
          ...prev,
          stage: "needs_llm",
          message: t("novel.importNeedsLLM"),
          percent: 0,
        }));
        return;
      }

      setskipped_count(result.skipped_count ?? 0);
      setskipped_chapters(
        (result.skipped_chapters ?? []) as { title: string; reason: string }[],
      );

      try {
        await onImported(result);
      } catch (err: unknown) {
        setError(toErrorMessage(err, t("novel.importFailedRetry")));
      }
    },
    [onImported, reset, t],
  );

  // The user clicks "AI analyze" -> call ImportWithLLM; after the LLM
  // analysis we import directly. Structured args: take
  // selectedModel.ProviderName/ModelID directly, no more splitModelKey split.
  const startLLMImport = useCallback(async () => {
    if (!filePath || !selectedModel) return;
    const { ProviderName: providerName, ModelID: modelID } = selectedModel;
    if (!providerName || !modelID) return;

    setError("");
    setProgress((prev) => ({
      ...prev,
      stage: "analyzing",
      message: t("novel.importAnalyzing"),
      percent: 30,
    }));

    try {
      const result = await ImportWithLLM({
        file_path: filePath,
        provider_name: providerName,
        model_id: modelID,
      });

      setskipped_count(result.skipped_count ?? 0);
      setskipped_chapters(
        (result.skipped_chapters ?? []) as { title: string; reason: string }[],
      );

      await onImported(result);
    } catch (err: unknown) {
      setProgress((prev) => ({
        ...prev,
        stage: "error",
        message: t("novel.importRollbackDone"),
        percent: 100,
      }));
      setError(toErrorMessage(err, t("novel.importFailedRetry")));
    }
  }, [filePath, selectedModel, onImported, t]);

  // Expose the structured selectedModel + models list + onModelChange.
  // ImportProgressDialog builds the dropdown from models, and onChange
  // internally finds the model and calls onModelChange.
  return {
    startImport,
    startLLMImport,
    selectedModel,
    models: modelsData,
    onModelChange: (model: llm.AvailableModel) => setSelectedModel(model),
    dialogProps: {
      open,
      progress,
      error,
      skippedCount,
      skippedChapters,
      onClose: reset,
    },
  };
}
