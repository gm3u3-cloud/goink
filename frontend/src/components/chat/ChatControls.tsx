import { useTranslation } from "react-i18next";
import ModelPicker from "@/components/model/ModelPicker";
import ContextRing from "./ContextRing";
import type { UsageInfo } from "./ContextRing";
import { useModels } from "@/components/settings/useModels";
import { useChatStore } from "./useChatStore";

interface Props {
  onSelectModel: (key: string) => void;
  onRefreshModels?: () => void;
  onSelectEffort: (effort: string) => void;
  onToggleApproval: () => void;
  onConfigModel: () => void;
  usage: UsageInfo | null;
  onCompress?: () => void;
  isTurnRunning?: boolean;
  isCompressing?: boolean;
}

// ChatControls: model/reasoning/approval controls.
// The models list is subscribed via the useModels query;
// selectedModel/reasoningEffort/approvalMode are subscribed from useChatStore
// (shared across components; the concatenated key is deprecated). Callbacks
// are still passed via props (migrated in mutation commit 4).
// 5.x: model + reasoning are merged into ModelPicker (clustered by provider
// + a trailing reasoning section at the bottom).
export default function ChatControls({
  onSelectModel,
  onRefreshModels,
  onSelectEffort,
  onToggleApproval,
  onConfigModel,
  usage,
  onCompress,
  isTurnRunning,
  isCompressing,
}: Props) {
  const { t } = useTranslation();
  const modelsQuery = useModels();
  const models = modelsQuery.data ?? [];
  const selectedModel = useChatStore((s) => s.selectedModel);
  const reasoningEffort = useChatStore((s) => s.reasoningEffort);
  const approvalMode = useChatStore((s) => s.approvalMode);

  const selectedKey = selectedModel?.Key ?? "";

  return (
    <div className="flex items-center gap-1.5 px-4 py-2 text-xs shrink-0 select-none">
      <ModelPicker
        models={models}
        selectedKey={selectedKey}
        reasoningEffort={reasoningEffort}
        onSelectModel={onSelectModel}
        onSelectEffort={onSelectEffort}
        onOpen={onRefreshModels}
        footerAction={{
          label: t("chat.configureModel"),
          onClick: onConfigModel,
        }}
      />

      <div className="flex-1" />

      <button
        onClick={onToggleApproval}
        className={`h-[30px] rounded-lg border px-2.5 text-xs transition-colors shrink-0 ${
          approvalMode === "auto"
            ? "bg-primary/10 text-primary border-primary/30"
            : "text-muted-foreground"
        }`}
      >
        {t("chat.auto")}
      </button>

      <ContextRing
        usage={usage}
        onCompress={onCompress}
        isTurnRunning={isTurnRunning}
        isCompressing={isCompressing}
      />
    </div>
  );
}
