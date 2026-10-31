import { create } from "zustand";
import type { llm, app } from "@/lib/wailsjs/go/models";

// useChatStore: UI state shared across components in the chat domain.
// selectedModel: the structured selected model (the concatenated key +
// splitModelKey are deprecated); shared by ChatPanel/ChatControls.
//   handleSend/handleCompress read ProviderName + ModelID directly, no more
//   splitModelKey string splitting.
// reasoningEffort/approvalMode: strongly related to selectedModel (effort is
// reset when the model changes); more cohesive to keep them in the store
// together.
// deletingSession: consolidated deletion (SessionHistory + RecentSessions
// only dispatch; DeleteSessionDialog executes centrally).
//   Stores the full SessionMeta object rather than just the id, since the
//   ConfirmDialog needs the title (in the paginated case ChatPanel cannot
//   look up all sessions from the query data).
// Not in the store: turns/sessionId/isLoading (streaming local state),
//   activeSessionId (coordinated inside ChatPanel),
//   showSettings/showHistoryPanel (pure UI toggles), drag/scroll refs
//   (5.1 special point 1 + rule 10).
interface ChatUIState {
  selectedModel: llm.AvailableModel | null;
  setSelectedModel: (model: llm.AvailableModel | null) => void;
  reasoningEffort: string;
  setReasoningEffort: (effort: string) => void;
  approvalMode: "manual" | "auto";
  setApprovalMode: (mode: "manual" | "auto") => void;
  deletingSession: app.SessionMeta | null;
  setDeletingSession: (session: app.SessionMeta | null) => void;
}

export const useChatStore = create<ChatUIState>((set) => ({
  selectedModel: null,
  setSelectedModel: (model) => set({ selectedModel: model }),
  reasoningEffort: "",
  setReasoningEffort: (effort) => set({ reasoningEffort: effort }),
  approvalMode: "manual",
  setApprovalMode: (mode) => set({ approvalMode: mode }),
  deletingSession: null,
  setDeletingSession: (session) => set({ deletingSession: session }),
}));
