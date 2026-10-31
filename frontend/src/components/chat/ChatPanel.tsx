import { useState, useCallback, useRef, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { MessageSquare, Loader2, History, Plus } from "lucide-react";
import { EventsOn } from "@/lib/wailsjs/runtime/runtime";
import { useQueryClient } from "@tanstack/react-query";
import { GetSession, Chat } from "@/lib/wailsjs/go/app/App";
import { cachedFileDescription } from "@/components/content/fileDescription";
import { modelKeys, sessionKeys, slashCommandKeys } from "@/lib/queryKeys";
import { useModels } from "@/components/settings/useModels";
import { useSettings } from "@/components/settings/useSettings";
import { useSessions } from "./useSessions";
import { useSession } from "./useSession";
import { useSessionMessages } from "./useSessionMessages";
import { useSlashCommands } from "./useSlashCommands";
import type { AgentEvent, Turn } from "./types";
import {
  AgentEventType,
  emptySegment,
  filterSegmentsBySeq,
  rebuildTurns,
} from "./types";
import ChatInput from "./ChatInput";
import ChatControls from "./ChatControls";
import DeleteSessionDialog from "./DeleteSessionDialog";
import MessageBubble from "./MessageBubble";
import ThinkingBlock from "./ThinkingBlock";
import ToolCallCard from "./ToolCallCard";
import WebSearchCard from "./WebSearchCard";
import WebFetchCard from "./WebFetchCard";
import SubagentCard from "./SubagentCard";
import CompressionBlock from "./CompressionBlock";
import type { UsageInfo } from "./ContextRing";
import SettingsDialog from "@/components/settings/SettingsDialog";
import RecentSessions from "./RecentSessions";
import SessionHistory from "./SessionHistory";
import { toastError } from "@/utils/toast";
import { toErrorMessage } from "@/utils/error";
import { useChatStore } from "./useChatStore";
import { useCompressContext } from "./useCompressContext";
import { useCancelChat } from "./useCancelChat";
import { useSetLastSession } from "./useSetLastSession";
import { useSetSelectedModel } from "./useSetSelectedModel";
import { useSetReasoningEffort } from "./useSetReasoningEffort";
import { useSetApprovalMode } from "./useSetApprovalMode";

interface Props {
  novelId: number;
  onApprove: (toolId: string, feedback: string) => Promise<void>;
  onReject: (toolId: string, feedback: string) => Promise<void>;
  onApprovalFileEdit?: (payload: {
    path: string;
    title: string;
    diff: string;
    original: string;
    modified: string;
    changeType: string;
    reason: string;
    toolId: string;
  }) => void;
  onFileEditFinished?: (toolId: string, path?: string) => void;
  chatPanelWidth: number;
  onChatPanelResize: (w: number) => void;
}
const EVENT_REORDER_TIMEOUT = 120;

interface EventQueue {
  nextSeq: number;
  pending: Map<number, AgentEvent>;
  flushTimer: ReturnType<typeof setTimeout> | null;
}

interface ChatStartedEvent {
  session_id?: string;
  turn_id: number;
}

export default function ChatPanel({
  novelId,
  onApprove,
  onReject,
  onApprovalFileEdit,
  onFileEditFinished,
  chatPanelWidth,
  onChatPanelResize,
}: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const compressMutation = useCompressContext();
  const cancelChatMutation = useCancelChat();
  const setLastSessionMutation = useSetLastSession();
  const setSelectedModelMutation = useSetSelectedModel();
  const setReasoningEffortMutation = useSetReasoningEffort();
  const setApprovalModeMutation = useSetApprovalMode();
  const modelsQuery = useModels();
  const settingsQuery = useSettings();
  const sessionsQuery = useSessions({
    novelId,
    page: 1,
    size: 5,
    search: "",
  });
  const slashCommandsQuery = useSlashCommands(novelId);
  const [isDragging, setIsDragging] = useState(false);
  const startXRef = useRef(0);
  const startWidthRef = useRef(chatPanelWidth);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [sessionId, setSessionId] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  // selectedModel/reasoningEffort/approvalMode are subscribed from
  // useChatStore (shared across components; the concatenated key is
  // deprecated).
  const selectedModel = useChatStore((s) => s.selectedModel);
  const setSelectedModel = useChatStore((s) => s.setSelectedModel);
  const reasoningEffort = useChatStore((s) => s.reasoningEffort);
  const setReasoningEffort = useChatStore((s) => s.setReasoningEffort);
  const approvalMode = useChatStore((s) => s.approvalMode);
  const setApprovalMode = useChatStore((s) => s.setApprovalMode);
  const selectedKey = selectedModel?.Key ?? "";
  const [lastUsage, setLastUsage] = useState<UsageInfo | null>(null);
  const [isCompressing, setIsCompressing] = useState(false);
  const compressingRef = useRef(false);
  const activeCountRef = useRef(0);
  const [showSettings, setShowSettings] = useState(false);
  const [activeSessionId, setActiveSessionId] = useState<
    string | null | undefined
  >(undefined);
  const [showHistoryPanel, setShowHistoryPanel] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const isNearBottomRef = useRef(true);
  const counterRef = useRef(0);
  const startedUnsubRef = useRef<(() => void) | null>(null);
  const agentUnsubRef = useRef<(() => void) | null>(null);
  const eventQueuesRef = useRef<Map<number, EventQueue>>(new Map());
  const onApprovalFileEditRef = useRef(onApprovalFileEdit);
  useEffect(() => {
    onApprovalFileEditRef.current = onApprovalFileEdit;
  }, [onApprovalFileEdit]);
  const onFileEditFinishedRef = useRef(onFileEditFinished);
  useEffect(() => {
    onFileEditFinishedRef.current = onFileEditFinished;
  }, [onFileEditFinished]);
  const restoredRef = useRef(false);

  // Selected-state restoration: depends on both the useModels and
  // useSettings query data being ready.
  // restoredRef guarantees restoration happens only once (avoids resetting
  // the user's manual selection when a query refetches).
  // Replaces the old Promise.all([GetModels, GetSettings]) +
  // setInitLoadError logic.
  // selectedModel stores a structured object (with ModelID/ProviderName);
  // the concatenated key + splitModelKey are deprecated.
  useEffect(() => {
    if (restoredRef.current) return;
    const modelList = modelsQuery.data;
    const settings = settingsQuery.data;
    if (!modelList || modelList.length === 0 || !settings) return;
    if (!novelId) return;
    restoredRef.current = true;

    // Restore the model selection (verify the key still exists) → set the
    // structured selectedModel
    const key = settings.selected_model_key || "";
    let model = modelList.find((m) => m.Key === key);
    if (!model) {
      model = modelList[0];
    }
    setSelectedModel(model);

    // Restore the reasoning effort (verify the level is still valid)
    let effort = settings.reasoning_effort || "";
    if (!effort || !model.ReasoningLevels?.includes(effort)) {
      effort = model.ReasoningLevels?.[0] || "";
    }
    setReasoningEffort(effort);

    // Restore the approval mode
    const mode = settings.approval_mode;
    if (mode === "manual" || mode === "auto") {
      setApprovalMode(mode);
    }

    // Directly restore the last active session (novelId already has a value
    // here; useLayoutEffect runs before useEffect)
    if (settings.last_session_id) {
      GetSession(settings.last_session_id)
        .then((detail) => {
          if (detail && detail.novel_id === novelId) {
            setActiveSessionId(settings.last_session_id);
          }
        })
        .catch(() => {
          setLastSessionMutation
            .mutateAsync("")
            .catch((err) =>
              toastError(toErrorMessage(err, t("chat.setLastSessionFailed"))),
            );
        });
    }
  }, [
    modelsQuery.data,
    settingsQuery.data,
    novelId,
    setSelectedModel,
    setReasoningEffort,
    setApprovalMode,
    setLastSessionMutation.mutateAsync,
    t,
  ]);

  // After the models query refetches (e.g. a SettingsDialog save triggers
  // invalidation):
  // check whether the current selectedModel is still in the new list; if not,
  // select the first one (replaces the old onSaved callback).
  useEffect(() => {
    if (!restoredRef.current) return;
    const modelList = modelsQuery.data;
    if (!modelList || modelList.length === 0) return;
    const currentKey = selectedModel?.Key;
    if (!currentKey || !modelList.find((m) => m.Key === currentKey)) {
      const first = modelList[0];
      setSelectedModel(first);
      if (first.ReasoningLevels?.length) {
        setReasoningEffort(first.ReasoningLevels[0]);
      }
    }
  }, [modelsQuery.data, selectedModel, setSelectedModel, setReasoningEffort]);

  // Active session details + history messages: useSession /
  // useSessionMessages queries.
  // When activeSid is empty (new conversation / nothing selected), enabled
  // is false and no fetch happens.
  const activeSid = activeSessionId ?? "";
  const sessionQuery = useSession(activeSid);
  const messagesQuery = useSessionMessages(activeSid);

  // Reset the session view when novelId changes.
  // Session restoration is handled by the restoredRef effect above
  // (restored directly on first load).
  // The session list is auto-fetched by the useSessions query (novelId
  // changes → queryKey changes → refetch).
  useEffect(() => {
    if (!novelId) return;
    setActiveSessionId(undefined);
    setTurns([]);
    setSessionId("");
  }, [novelId]);

  // Sync sessionId when activeSessionId changes; rebuildTurns when
  // messagesQuery.data is ready.
  // During streaming, activeSessionId does not change and
  // messagesQuery.data does not change; turns are updated by agent events.
  useEffect(() => {
    if (!activeSessionId) {
      setSessionId("");
      return;
    }
    setSessionId(activeSessionId);
    const msgs = messagesQuery.data;
    if (msgs) {
      setTurns(rebuildTurns(msgs));
    }
  }, [activeSessionId, messagesQuery.data]);

  // lastUsage is restored from the usage field of the useSession data
  // (replaces the old GetSession inside handleSelectSession).
  useEffect(() => {
    if (!activeSessionId) return;
    if (sessionQuery.data?.usage) {
      setLastUsage(sessionQuery.data.usage as unknown as UsageInfo);
    } else if (sessionQuery.isError) {
      setLastUsage(null);
    }
  }, [sessionQuery.data, sessionQuery.isError, activeSessionId]);

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      setIsDragging(true);
      startXRef.current = e.clientX;
      startWidthRef.current = chatPanelWidth;
    },
    [chatPanelWidth],
  );

  useEffect(() => {
    if (!isDragging) return;
    const handleMouseMove = (e: MouseEvent) => {
      const delta = e.clientX - startXRef.current;
      onChatPanelResize(startWidthRef.current - delta);
    };
    const handleMouseUp = () => setIsDragging(false);
    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isDragging, onChatPanelResize]);

  // Clean up event listeners
  useEffect(() => {
    const eventQueues = eventQueuesRef.current;
    return () => {
      startedUnsubRef.current?.();
      agentUnsubRef.current?.();
      eventQueues.forEach((queue) => {
        if (queue.flushTimer) clearTimeout(queue.flushTimer);
      });
      eventQueues.clear();
    };
  }, []);

  // Listen for chat:turn_outcome: chat.go emits it after Run returns,
  // carrying the authoritative session.EventType.
  // The frontend sets the turn status by the event_type string (the same
  // mapping used by rebuildTurns restoration).
  // A top-level listener avoids a microtask race with the Chat() promise
  // reject (turn_outcome is the last event emitted before Run returns).
  useEffect(() => {
    const cleanup = EventsOn(
      "chat:turn_outcome",
      (data: { turn_id: number; event_type: string; reason: string }) => {
        setTurns((prev) =>
          prev.map((turn) => {
            if (turn.turnId !== data.turn_id) return turn;
            let status: Turn["status"] = "interrupted";
            if (data.event_type === "error") status = "failed";
            else if (data.event_type === "user_stopped") status = "stopped";
            return {
              ...turn,
              status,
              errorMessage: data.reason,
              retrying: null,
            };
          }),
        );
      },
    );
    return () => cleanup();
  }, []);

  // Auto-scroll to the bottom during streaming, but only when the user has
  // not actively scrolled up
  useEffect(() => {
    if (isNearBottomRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: "instant" });
    }
  }, [turns]);

  const handleMessagesScroll = useCallback(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    isNearBottomRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight < 60;
  }, []);

  const handleSelectSession = useCallback(
    (sid: string) => {
      setActiveSessionId(sid);
      setLastSessionMutation
        .mutateAsync(sid)
        .catch((err) =>
          toastError(toErrorMessage(err, t("chat.setLastSessionFailed"))),
        );
      // usage is auto-fetched by the useSession query + restored by the
      // lastUsage effect; no manual fetch anymore
    },
    [setLastSessionMutation.mutateAsync, t],
  );

  const handleNewChat = useCallback(() => {
    setActiveSessionId(null);
    setTurns([]);
    setSessionId("");
    setLastUsage(null);
    setLastSessionMutation
      .mutateAsync("")
      .catch((err) =>
        toastError(toErrorMessage(err, t("chat.setLastSessionFailed"))),
      );
    qc.invalidateQueries({ queryKey: sessionKeys.list(novelId, 1, 5, "") });
  }, [novelId, qc, setLastSessionMutation.mutateAsync, t]);

  const handleOpenHistory = useCallback(() => {
    setShowHistoryPanel(true);
  }, []);

  const handleCloseHistory = useCallback(() => {
    setShowHistoryPanel(false);
  }, []);

  // Slash commands are auto-fetched by the useSlashCommands query; onListSlash
  // invalidates to trigger a refetch.
  const refreshSlashCommands = useCallback(() => {
    qc.invalidateQueries({ queryKey: slashCommandKeys.list(novelId) });
  }, [qc, novelId]);

  const applyAgentEvent = useCallback(
    (turnId: number, event: AgentEvent) => {
      // File edit approval → notify ContentPanel to open the diff tab
      if (
        event.type === AgentEventType.ToolCall &&
        !event.sub_task_id &&
        event.phase === "awaiting_approval" &&
        event.metadata?.approval_type === "file_edit" &&
        event.metadata.payload
      ) {
        const p = event.metadata.payload as Record<string, unknown>;
        const path = (p.path as string) || "";
        const title = cachedFileDescription(qc, novelId, path, t).diffTitle;
        onApprovalFileEditRef.current?.({
          path,
          title,
          diff: "",
          original: (p.original as string) || "",
          modified: (p.modified as string) || "",
          changeType: (p.change_type as string) || "",
          reason: (p.reason as string) || "",
          toolId: (event.tool_id as string) || "",
        });
      }
      if (
        event.type === AgentEventType.ToolCall &&
        !event.sub_task_id &&
        event.tool_name === "edit" &&
        event.tool_id &&
        (event.phase === "completed" || event.phase === "failed")
      ) {
        const path = event.metadata?.path;
        onFileEditFinishedRef.current?.(
          event.tool_id,
          event.phase === "completed" && typeof path === "string"
            ? path
            : undefined,
        );
      }
          // P2: stream-resume event clears the retrying state (the agent's
          // LLM call retry succeeded)
      if (
        event.type === AgentEventType.Thinking ||
        event.type === AgentEventType.ThinkingDone ||
        event.type === AgentEventType.Content ||
        event.type === AgentEventType.ToolCall
      ) {
        setTurns((prev) =>
          prev.map((turn) => {
            if (turn.turnId !== turnId) return turn;
            // P1: after a sub-agent retry, the stream resumes; clear the
            // retrying state of the corresponding subagent segment
            if (event.sub_task_id) {
              const subIdx = turn.segments.findIndex(
                (s) => s.type === "subagent" && s.taskId === event.sub_task_id,
              );
              if (subIdx >= 0 && turn.segments[subIdx].retrying) {
                const newSegs = [...turn.segments];
                newSegs[subIdx] = { ...turn.segments[subIdx], retrying: null };
                return { ...turn, segments: newSegs };
              }
              return turn;
            }
            // Stream resumed after the main turn's retry
            return turn.retrying ? { ...turn, retrying: null } : turn;
          }),
        );
      }
      switch (event.type) {
        case AgentEventType.Usage: {
          if (event.usage) {
            setLastUsage(event.usage as unknown as UsageInfo);
          }
          return;
        }
        case AgentEventType.Error: {
          // P1: when a sub-agent fails, route to the corresponding subagent
          // segment by sub_task_id
          // The backend RunSubAgent reuses the parent turn's TurnID+EventSeq,
          // so an EventError triggered by a sub-agent failure bubbles up to
          // the main turn. The frontend must distinguish by sub_task_id,
          // otherwise the main turn would be wrongly set to failed.
          if (event.sub_task_id) {
            setTurns((prev) =>
              prev.map((turn) => {
                if (turn.turnId !== turnId) return turn;
                const subIdx = turn.segments.findIndex(
                  (s) =>
                    s.type === "subagent" && s.taskId === event.sub_task_id,
                );
                if (subIdx < 0) return turn;
                const subSeg = { ...turn.segments[subIdx] };
                subSeg.status = "failed";
                subSeg.errorMessage = event.error || t("chat.chatError");
                subSeg.retrying = null;
                const newSegs = [...turn.segments];
                newSegs[subIdx] = subSeg;
                return { ...turn, segments: newSegs };
              }),
            );
            return;
          }
          // Main turn failed
          setTurns((prev) =>
            prev.map((turn) =>
              turn.turnId === turnId
                ? {
                    ...turn,
                    status: "failed" as const,
                    errorMessage: event.error || t("chat.chatError"),
                    retrying: null,
                  }
                : turn,
            ),
          );
          return;
        }
        case AgentEventType.Retrying: {
          // P2: the agent layer is retrying the LLM call; show "retry x/y"
          // Also clear the partial segments already rendered by this
          // streamLoop iteration (historical segments with firstSeq=0 are
          // kept)
          const clearFromSeq = event.clear_from_seq ?? 0;
          // P1: when a sub-agent is retrying, route to the corresponding
          // subagent segment by sub_task_id
          // The backend RunSubAgent reuses the parent turn's TurnID+EventSeq,
          // so an EventRetrying triggered by a sub-agent retry bubbles up to
          // the main turn. The frontend must distinguish by sub_task_id,
          // otherwise the main turn would wrongly show the banner, and the
          // partials inside the sub-agent would not be cleared (bug
          // reproduction).
          if (event.sub_task_id) {
            setTurns((prev) =>
              prev.map((turn) => {
                if (turn.turnId !== turnId) return turn;
                const subIdx = turn.segments.findIndex(
                  (s) =>
                    s.type === "subagent" && s.taskId === event.sub_task_id,
                );
                if (subIdx < 0) return turn;
                const subSeg = { ...turn.segments[subIdx] };
                subSeg.segments = filterSegmentsBySeq(
                  subSeg.segments || [],
                  clearFromSeq,
                );
                subSeg.retrying = {
                  attempt: event.attempt || 0,
                  maxRetries: event.max_retries || 0,
                  errorMessage: event.error || "",
                };
                const newSegs = [...turn.segments];
                newSegs[subIdx] = subSeg;
                return { ...turn, segments: newSegs };
              }),
            );
            return;
          }
          setTurns((prev) =>
            prev.map((turn) =>
              turn.turnId === turnId
                ? {
                    ...turn,
                    status: "streaming" as const,
                    segments: filterSegmentsBySeq(turn.segments, clearFromSeq),
                    retrying: {
                      attempt: event.attempt || 0,
                      maxRetries: event.max_retries || 0,
                      errorMessage: event.error || "",
                    },
                  }
                : turn,
            ),
          );
          return;
        }
        case AgentEventType.Compression: {
          const phase = (event.compression_phase || "started") as
            "compressing" | "done";
          if (event.sub_task_id) {
            setTurns((prev) =>
              prev.map((turn) => {
                if (turn.turnId !== turnId) return turn;
                const subIdx = turn.segments.findIndex(
                  (s) =>
                    s.type === "subagent" && s.taskId === event.sub_task_id,
                );
                if (subIdx < 0) {
                  return {
                    ...turn,
                    segments: [
                      ...turn.segments,
                      {
                        ...emptySegment(`subagent_${event.sub_task_id}`),
                        type: "subagent",
                        status: "streaming",
                        agentType: "review" as const,
                        taskId: event.sub_task_id,
                        segments: [
                          {
                            ...emptySegment(`comp_${++counterRef.current}`),
                            type: "compression",
                            compressionPhase: phase,
                          },
                        ],
                      },
                    ],
                  };
                }
                const subSeg = { ...turn.segments[subIdx] };
                if (!subSeg.segments) subSeg.segments = [];
                const subSegs = [...subSeg.segments];
                const compIdx = subSegs.findIndex(
                  (s) => s.type === "compression",
                );
                if (compIdx >= 0) {
                  subSegs[compIdx] = {
                    ...subSegs[compIdx],
                    compressionPhase: phase,
                  };
                } else {
                  subSegs.push({
                    ...emptySegment(`comp_${++counterRef.current}`),
                    type: "compression",
                    compressionPhase: phase,
                  });
                }
                subSeg.segments = subSegs;
                const newSegs = [...turn.segments];
                newSegs[subIdx] = subSeg;
                return { ...turn, segments: newSegs };
              }),
            );
            return;
          }
          setTurns((prev) =>
            prev.map((turn) => {
              if (turn.turnId !== turnId) return turn;
              const compIdx = turn.segments.findIndex(
                (s) => s.type === "compression",
              );
              if (compIdx >= 0) {
                const segs = [...turn.segments];
                segs[compIdx] = { ...segs[compIdx], compressionPhase: phase };
                return { ...turn, segments: segs };
              }
              return {
                ...turn,
                segments: [
                  ...turn.segments,
                  {
                    ...emptySegment(`comp_${++counterRef.current}`),
                    type: "compression" as const,
                    compressionPhase: phase,
                  },
                ],
              };
            }),
          );
          return;
        }
      }

      setTurns((prev) =>
        prev.map((turn) => {
          if (turn.turnId !== turnId) return turn;

          // Sub-Agent event: route to the corresponding SubagentSegment by
          // sub_task_id
          if (event.sub_task_id) {
            let subIdx = turn.segments.findIndex(
              (s) => s.type === "subagent" && s.taskId === event.sub_task_id,
            );
            let updatedSegments = turn.segments;
            if (subIdx < 0) {
              // The run_subagent ToolCall event has not been applied yet, but
              // the sub-Agent event arrived first — create it in place
              const newSeg = {
                ...emptySegment(`subagent_${event.sub_task_id}`),
                type: "subagent" as const,
                status: "streaming" as const,
                agentType: "memory" as const,
                taskId: event.sub_task_id,
                segments: [],
                finalText: "",
                toolStatus: "executing" as const,
                firstSeq: event.seq ?? 0,
              };
              updatedSegments = [...turn.segments, newSeg];
              subIdx = updatedSegments.length - 1;
            }
            const subSeg = { ...updatedSegments[subIdx] };
            if (!subSeg.segments) subSeg.segments = [];
            const subSegs = [...subSeg.segments];
            const subSegId = `subseg_${++counterRef.current}`;

            switch (event.type) {
              case AgentEventType.Thinking: {
                const chunk = event.data || "";
                const last = subSegs[subSegs.length - 1];
                if (last && last.type === "text" && last.isStreaming) {
                  subSegs[subSegs.length - 1] = {
                    ...last,
                    thinkingContent: last.thinkingContent + chunk,
                  };
                } else {
                  subSegs.push({
                    ...emptySegment(subSegId),
                    thinkingContent: chunk,
                    thinkingDone: false,
                    isStreaming: true,
                    firstSeq: event.seq ?? 0,
                  });
                }
                break;
              }
              case AgentEventType.ThinkingDone: {
                for (let i = 0; i < subSegs.length; i++) {
                  if (subSegs[i].type === "text" && !subSegs[i].thinkingDone) {
                    subSegs[i] = {
                      ...subSegs[i],
                      thinkingDone: true,
                      isStreaming: false,
                    };
                  }
                }
                break;
              }
              case AgentEventType.Content: {
                const chunk = event.data || "";
                const last = subSegs[subSegs.length - 1];
                if (last && last.type === "text" && last.isStreaming) {
                  subSegs[subSegs.length - 1] = {
                    ...last,
                    content: last.content + chunk,
                    thinkingDone: true,
                  };
                } else {
                  subSegs.push({
                    ...emptySegment(subSegId),
                    content: chunk,
                    thinkingDone: true,
                    isStreaming: true,
                    firstSeq: event.seq ?? 0,
                  });
                }
                break;
              }
              case AgentEventType.ToolCall: {
                const subToolStatus =
                  event.phase === "completed"
                    ? ("completed" as const)
                    : event.phase === "failed"
                      ? ("failed" as const)
                      : ("executing" as const);
                const stIdx = subSegs.findIndex(
                  (s) => s.type === "tool" && s.toolId === event.tool_id,
                );
                if (stIdx >= 0) {
                  subSegs[stIdx] = {
                    ...subSegs[stIdx],
                    toolStatus: subToolStatus,
                    displayText:
                      event.display_text || subSegs[stIdx].displayText,
                    activityKind: event.activity_kind || "",
                    error: event.error || "",
                  };
                } else {
                  subSegs.push({
                    ...emptySegment(subSegId),
                    type: "tool",
                    toolName: event.tool_name || "",
                    toolId: event.tool_id || "",
                    toolStatus: subToolStatus,
                    displayText: event.display_text || event.tool_name || "",
                    activityKind: event.activity_kind || "",
                    error: event.error || "",
                    firstSeq: event.seq ?? 0,
                  });
                }
                break;
              }
              default:
                break;
            }

            subSeg.segments = subSegs;
            const newSegs = [...updatedSegments];
            newSegs[subIdx] = subSeg;
            return { ...turn, segments: newSegs };
          }

          const segments = [...turn.segments];
          const segId = `seg_${++counterRef.current}`;

          switch (event.type) {
            case AgentEventType.Thinking: {
              const chunk = event.data || "";
              const lastSeg = segments[segments.length - 1];
              if (lastSeg && lastSeg.type === "text" && lastSeg.isStreaming) {
                segments[segments.length - 1] = {
                  ...lastSeg,
                  thinkingContent: lastSeg.thinkingContent + chunk,
                };
              } else {
                segments.push({
                  ...emptySegment(segId),
                  thinkingContent: chunk,
                  thinkingDone: false,
                  isStreaming: true,
                  firstSeq: event.seq ?? 0,
                });
              }
              return { ...turn, segments };
            }

            case AgentEventType.ThinkingDone: {
              return {
                ...turn,
                segments: segments.map((seg) =>
                  seg.type === "text" && !seg.thinkingDone
                    ? { ...seg, thinkingDone: true, isStreaming: false }
                    : seg,
                ),
              };
            }

            case AgentEventType.Content: {
              const chunk = event.data || "";
              const lastSeg = segments[segments.length - 1];
              if (lastSeg && lastSeg.type === "text" && lastSeg.isStreaming) {
                segments[segments.length - 1] = {
                  ...lastSeg,
                  content: lastSeg.content + chunk,
                  thinkingDone: true,
                };
              } else {
                segments.push({
                  ...emptySegment(segId),
                  content: chunk,
                  thinkingDone: true,
                  isStreaming: true,
                  firstSeq: event.seq ?? 0,
                });
              }
              return { ...turn, segments };
            }

            case AgentEventType.ToolCall: {
              const isSubagent = event.tool_name === "run_subagent";
              const toolStatus =
                event.phase === "awaiting_approval"
                  ? ("awaiting_approval" as const)
                  : event.phase === "completed"
                    ? ("completed" as const)
                    : event.phase === "failed"
                      ? ("failed" as const)
                      : ("executing" as const);

              // run_subagent: maintain the corresponding subagent segment
              if (isSubagent) {
                const agentType =
                  (event.metadata?.agent_type as "memory" | "review") ||
                  "memory";
                const toolId = event.tool_id || "";
                const subIdx = segments.findIndex(
                  (seg) => seg.type === "subagent" && seg.taskId === toolId,
                );
                if (subIdx >= 0) {
                  segments[subIdx] = {
                    ...segments[subIdx],
                    agentType,
                    status:
                      toolStatus === "executing"
                        ? "streaming"
                        : toolStatus === "failed"
                          ? "failed"
                          : "done",
                    toolStatus,
                    errorMessage:
                      toolStatus === "failed"
                        ? event.error || ""
                        : segments[subIdx].errorMessage,
                  };
                } else {
                  segments.push({
                    ...emptySegment(`subagent_${toolId || segId}`),
                    type: "subagent",
                    status: "streaming",
                    agentType,
                    taskId: toolId,
                    segments: [],
                    finalText: "",
                    toolStatus: "executing",
                    firstSeq: event.seq ?? 0,
                  });
                }
                // Remove the tool segment with the same toolId (it may have
                // been created by mistake from an early event with an empty
                // toolName)
                const cleanSegs = toolId
                  ? segments.filter(
                      (seg) => !(seg.type === "tool" && seg.toolId === toolId),
                    )
                  : segments;
                return { ...turn, segments: cleanSegs };
              }

              const idx = segments.findIndex(
                (seg) =>
                  seg.type === "tool" &&
                  event.tool_id &&
                  seg.toolId === event.tool_id,
              );

              const approvalType =
                toolStatus === "awaiting_approval"
                  ? (event.metadata?.approval_type as string | undefined)
                  : undefined;
              const approvalPayload =
                toolStatus === "awaiting_approval"
                  ? (event.metadata?.payload as
                      Record<string, unknown> | undefined)
                  : undefined;

              if (idx >= 0) {
                segments[idx] = {
                  ...segments[idx],
                  toolName: event.tool_name || segments[idx].toolName,
                  toolId: event.tool_id || segments[idx].toolId,
                  toolStatus,
                  displayText: event.display_text || segments[idx].displayText,
                  activityKind:
                    event.activity_kind || segments[idx].activityKind || "",
                  error: event.error || "",
                  approvalType: approvalType ?? segments[idx].approvalType,
                  approvalPayload:
                    approvalPayload ?? segments[idx].approvalPayload,
                  result:
                    toolStatus === "completed"
                      ? event.metadata || segments[idx].result
                      : segments[idx].result,
                };
              } else {
                segments.push({
                  ...emptySegment(segId),
                  type: "tool",
                  toolName: event.tool_name || "",
                  toolId: event.tool_id || "",
                  toolStatus,
                  displayText: event.display_text || event.tool_name || "",
                  activityKind: event.activity_kind || "",
                  error: event.error || "",
                  approvalType,
                  approvalPayload,
                  result:
                    toolStatus === "completed" ? event.metadata : undefined,
                  firstSeq: event.seq ?? 0,
                });
              }

              return { ...turn, segments };
            }

            default:
              return turn;
          }
        }),
      );
    },
    [t, qc, novelId],
  );

  const flushEventQueue = useCallback(
    (turnId: number, force = false) => {
      const queue = eventQueuesRef.current.get(turnId);
      if (!queue) return;

      let event = queue.pending.get(queue.nextSeq);
      while (event) {
        queue.pending.delete(queue.nextSeq);
        queue.nextSeq += 1;
        applyAgentEvent(turnId, event);
        event = queue.pending.get(queue.nextSeq);
      }

      if (force && queue.pending.size > 0) {
        const orderedEvents = [...queue.pending.entries()].sort(
          ([a], [b]) => a - b,
        );
        queue.pending.clear();

        for (const [seq, queuedEvent] of orderedEvents) {
          if (seq >= queue.nextSeq) {
            queue.nextSeq = seq + 1;
            applyAgentEvent(turnId, queuedEvent);
          }
        }
      }

      if (queue.pending.size === 0 && queue.flushTimer) {
        clearTimeout(queue.flushTimer);
        queue.flushTimer = null;
      }
    },
    [applyAgentEvent],
  );

  const handleAgentEvent = useCallback(
    (turnId: number) => (event: AgentEvent) => {
      if (!event.seq) {
        applyAgentEvent(turnId, event);
        return;
      }

      let queue = eventQueuesRef.current.get(turnId);
      if (!queue) {
        queue = {
          nextSeq: 1,
          pending: new Map<number, AgentEvent>(),
          flushTimer: null,
        };
        eventQueuesRef.current.set(turnId, queue);
      }

      if (event.seq < queue.nextSeq) return;

      queue.pending.set(event.seq, event);
      flushEventQueue(turnId);

      if (queue.pending.size > 0 && !queue.flushTimer) {
        queue.flushTimer = setTimeout(() => {
          queue.flushTimer = null;
          flushEventQueue(turnId, true);
        }, EVENT_REORDER_TIMEOUT);
      }
    },
    [applyAgentEvent, flushEventQueue],
  );

  const handleConfigModel = useCallback(() => setShowSettings(true), []);

  // Model refresh: invalidation triggers a useModels refetch; the selected
  // state is auto-corrected by the effect below.
  const refreshModels = useCallback(() => {
    qc.invalidateQueries({ queryKey: modelKeys.all });
  }, [qc]);

  const handleSelectModel = useCallback(
    (key: string) => {
      const m = modelsQuery.data?.find((x) => x.Key === key);
      if (!m) return;
      setSelectedModel(m);
      let effort = "";
      if (m.ReasoningLevels?.length) {
        effort = m.ReasoningLevels[0];
        setReasoningEffort(effort);
      }
      setSelectedModelMutation
        .mutateAsync({ key, effort })
        .catch((err) =>
          toastError(toErrorMessage(err, t("chat.setSelectedModelFailed"))),
        );
    },
    [
      modelsQuery.data,
      setSelectedModel,
      setReasoningEffort,
      setSelectedModelMutation.mutateAsync,
      t,
    ],
  );

  const handleSelectEffort = useCallback(
    (effort: string) => {
      setReasoningEffort(effort);
      setReasoningEffortMutation
        .mutateAsync(effort)
        .catch((err) =>
          toastError(toErrorMessage(err, t("chat.setReasoningEffortFailed"))),
        );
    },
    [setReasoningEffort, setReasoningEffortMutation.mutateAsync, t],
  );

  const handleToggleApproval = useCallback(() => {
    const next = approvalMode === "manual" ? "auto" : "manual";
    setApprovalMode(next);
    setApprovalModeMutation
      .mutateAsync(next)
      .catch((err) =>
        toastError(toErrorMessage(err, t("chat.setApprovalModeFailed"))),
      );
  }, [approvalMode, setApprovalMode, setApprovalModeMutation.mutateAsync, t]);

  const handleCompress = useCallback(async () => {
    if (!sessionId || !selectedModel || compressingRef.current) return;
    const providerName = selectedModel.ProviderName;
    const modelID = selectedModel.ModelID;

    compressingRef.current = true;
    setIsCompressing(true);
    // Create the compressing turn (for the animation display)
    const compTurnId = `comp_${++counterRef.current}`;
    const compressingTurn: Turn = {
      id: compTurnId,
      turnId: 0,
      userMessage: "",
      segments: [
        {
          ...emptySegment(compTurnId),
          type: "compression" as const,
          compressionPhase: "compressing" as const,
        },
      ],
      status: "done" as const,
      compressionOnly: true,
    };
    setTurns((prev) => [...prev, compressingTurn]);

    try {
      const result = await compressMutation.mutateAsync({
        session_id: sessionId,
        provider_name: providerName,
        model_id: modelID,
      });
      // Update: back-fill the real turnId + the completed state
      setTurns((prev) =>
        prev.map((t) => {
          if (t.id === compTurnId) {
            return {
              ...t,
              turnId: result.turn_id,
              segments: t.segments.map((s) =>
                s.type === "compression"
                  ? { ...s, compressionPhase: "done" as const }
                  : s,
              ),
            };
          }
          return t;
        }),
      );
    } catch (err) {
      // Compression failed; remove the compressing turn
      setTurns((prev) => prev.filter((t) => t.id !== compTurnId));
      toastError(toErrorMessage(err, t("chat.compressFailed")));
    } finally {
      setIsCompressing(false);
      compressingRef.current = false;
    }
  }, [sessionId, selectedModel, compressMutation.mutateAsync, t]);

  const handleSend = useCallback(
    async (content: string) => {
      if (!selectedModel) return;
      const p = selectedModel.ProviderName;
      const m = selectedModel.ModelID;
      activeCountRef.current++;
      if (activeCountRef.current > 1) {
        cancelChatMutation
          .mutateAsync(sessionId)
          .catch((err) =>
            toastError(toErrorMessage(err, t("chat.cancelFailed"))),
          );
      }
      setIsLoading(true);

      const turnId = `turn_${++counterRef.current}`;
      const newTurn: Turn = {
        id: turnId,
        turnId: 0,
        userMessage: content,
        segments: [],
        status: "streaming",
      };

      // If this is a new conversation, clear the history marker
      if (activeSessionId === null || activeSessionId === undefined) {
        setActiveSessionId(null);
      }

      setTurns((prev) => [...prev, newTurn]);

      // Listen for chat:started; once the turnId is obtained, subscribe to
      // the agent event stream
      startedUnsubRef.current?.();
      const startedCleanup = EventsOn(
        "chat:started",
        (data: ChatStartedEvent) => {
          if (data.session_id) {
            setSessionId(data.session_id);
            setActiveSessionId(data.session_id);
            setLastSessionMutation
              .mutateAsync(data.session_id)
              .catch((err) =>
                toastError(toErrorMessage(err, t("chat.setLastSessionFailed"))),
              );
          }

          // Update the turn's turnId to the real value assigned by the
          // backend
          setTurns((prev) =>
            prev.map((t) =>
              t.id === turnId ? { ...t, turnId: data.turn_id } : t,
            ),
          );

          agentUnsubRef.current?.();
          const agentCleanup = EventsOn(
            `agent:${data.turn_id}`,
            handleAgentEvent(data.turn_id),
          );
          agentUnsubRef.current = agentCleanup;
        },
      );
      startedUnsubRef.current = startedCleanup;

      try {
        await Chat({
          session_id: sessionId,
          novel_id: novelId,
          message: content,
          provider_name: p,
          model_id: m,
          reasoning_effort: reasoningEffort,
        });
        // Refresh the session list (invalidation triggers a useSessions
        // refetch)
        qc.invalidateQueries({ queryKey: sessionKeys.list(novelId, 1, 5, "") });
      } catch (err) {
        setTurns((prev) =>
          prev.map((t) => {
            if (t.id !== turnId) return t;
            if (
              t.status === "stopped" ||
              t.status === "failed" ||
              t.status === "interrupted"
            )
              return t;
            return {
              ...t,
              status: "interrupted" as const,
              errorMessage: String(err),
            };
          }),
        );
      } finally {
        eventQueuesRef.current.forEach((queue, queuedTurnId) => {
          if (queue.flushTimer) clearTimeout(queue.flushTimer);
          const orderedEvents = [...queue.pending.entries()].sort(
            ([a], [b]) => a - b,
          );
          queue.pending.clear();
          for (const [seq, queuedEvent] of orderedEvents) {
            if (seq >= queue.nextSeq) {
              queue.nextSeq = seq + 1;
              applyAgentEvent(queuedTurnId, queuedEvent);
            }
          }
        });
        eventQueuesRef.current.clear();
        setTurns((prev) =>
          prev.map((t) =>
            t.id === turnId && t.status === "streaming"
              ? {
                  ...t,
                  status: "done" as const,
                  segments: t.segments.map((seg) =>
                    seg.type === "text" ? { ...seg, isStreaming: false } : seg,
                  ),
                }
              : t,
          ),
        );
        activeCountRef.current--;
        if (activeCountRef.current === 0) {
          setIsLoading(false);
        }
        startedUnsubRef.current?.();
        startedUnsubRef.current = null;
        agentUnsubRef.current?.();
        agentUnsubRef.current = null;
      }
    },
    [
      sessionId,
      novelId,
      selectedModel,
      reasoningEffort,
      handleAgentEvent,
      applyAgentEvent,
      activeSessionId,
      qc,
      cancelChatMutation.mutateAsync,
      setLastSessionMutation.mutateAsync,
      t,
    ],
  );

  const hasNovel = novelId > 0;
  const hasTurns = turns.length > 0;
  const hasActiveSession =
    activeSessionId !== undefined && activeSessionId !== null;
  const showRecent = !hasActiveSession && !hasTurns && !isLoading;

  const inputPlaceholder = !hasNovel
    ? t("chat.selectNovelFirst")
    : !selectedKey
      ? t("chat.configureModelFirst")
      : t("chat.inputPlaceholder");

  // Draft key: existing sessions are keyed by sessionId; new conversations
  // (unselected / newly opened) are slotted per novel
  const draftKey = sessionId ? sessionId : `new_${novelId}`;

  return (
    <aside
      className="shrink-0 flex flex-col bg-sidebar border-l relative overflow-hidden"
      style={{ width: chatPanelWidth }}
    >
      <div
        className="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-primary/30 transition-colors z-10 select-none"
        style={{ marginLeft: -2 }}
        onMouseDown={handleMouseDown}
      />

      <div className="px-4 py-2.5 border-b shrink-0 flex items-center justify-between select-none">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {t("chat.aiChat")}
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={handleOpenHistory}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          >
            <History className="w-3.5 h-3.5" /> {t("chat.history")}
          </button>
          <button
            onClick={handleNewChat}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          >
            <Plus className="w-3.5 h-3.5" /> {t("chat.newChat")}
          </button>
        </div>
      </div>

      <div className="absolute left-0 right-0 top-[41px] bottom-0 pointer-events-none z-30">
        <SessionHistory
          open={showHistoryPanel}
          novelId={novelId}
          onClose={handleCloseHistory}
          onSelectSession={handleSelectSession}
        />
      </div>

      <div
        ref={scrollContainerRef}
        onScroll={handleMessagesScroll}
        className="flex-1 overflow-y-auto overscroll-contain px-3 py-3 relative"
      >
        {!hasNovel ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center">
              <MessageSquare className="w-10 h-10 text-muted-foreground/20 mx-auto mb-3" />
              <p className="text-sm text-muted-foreground">
                {t("chat.selectNovel")}
              </p>
            </div>
          </div>
        ) : showRecent ? (
          <RecentSessions
            sessions={sessionsQuery.data?.items ?? []}
            total={sessionsQuery.data?.total ?? 0}
            onSelectSession={handleSelectSession}
            onViewAll={handleOpenHistory}
          />
        ) : messagesQuery.isLoading ? (
          <div className="flex items-center justify-center h-full">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <>
            {/* Message list */}
            {messagesQuery.isError ? (
              <div className="flex items-center justify-center h-full">
                <div className="text-center">
                  <p className="text-sm text-red-500 mb-2">
                    {t("chat.loadMessagesFailed")}
                  </p>
                  <button
                    onClick={() => messagesQuery.refetch()}
                    className="text-xs text-primary underline cursor-pointer"
                  >
                    {t("chat.retry")}
                  </button>
                </div>
              </div>
            ) : !hasTurns && !isLoading ? (
              <div className="flex items-center justify-center h-full">
                <div className="text-center">
                  <MessageSquare className="w-10 h-10 text-muted-foreground/20 mx-auto mb-3" />
                  <p className="text-sm text-muted-foreground">
                    {t("chat.startConversation")}
                  </p>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                {turns.map((turn) => (
                  <div key={turn.id} className="space-y-2">
                    {turn.userMessage && (
                      <MessageBubble role="user" content={turn.userMessage} />
                    )}

                    {turn.segments.map((seg) => {
                      if (seg.type === "subagent" && seg.agentType) {
                        return (
                          <SubagentCard
                            key={seg.id}
                            agentType={seg.agentType}
                            segments={seg.segments || []}
                            status={seg.status || "done"}
                            retrying={seg.retrying}
                            errorMessage={seg.errorMessage}
                          />
                        );
                      }

                      if (seg.type === "tool") {
                        // run_subagent is already rendered by the subagent
                        // segment; skip the plain tool card
                        if (seg.toolName === "run_subagent") return null;

                        if (
                          seg.toolName === "web_search" &&
                          seg.toolStatus === "completed" &&
                          seg.result
                        ) {
                          return (
                            <WebSearchCard key={seg.id} result={seg.result} />
                          );
                        }
                        if (
                          seg.toolName === "web_fetch" &&
                          seg.toolStatus === "completed" &&
                          seg.result
                        ) {
                          return (
                            <WebFetchCard
                              key={seg.id}
                              result={seg.result}
                              displayText={seg.displayText}
                            />
                          );
                        }

                        return (
                          <ToolCallCard
                            key={seg.id}
                            toolName={seg.toolName}
                            displayText={seg.displayText}
                            status={seg.toolStatus}
                            activityKind={seg.activityKind}
                            error={seg.error}
                            approvalType={seg.approvalType}
                            approvalPayload={seg.approvalPayload}
                            result={seg.result}
                            onApprove={
                              seg.toolStatus === "awaiting_approval"
                                ? (feedback: string) =>
                                    onApprove(seg.toolId, feedback)
                                : undefined
                            }
                            onReject={
                              seg.toolStatus === "awaiting_approval"
                                ? (feedback: string) =>
                                    onReject(seg.toolId, feedback)
                                : undefined
                            }
                          />
                        );
                      }

                      if (seg.type === "compression") {
                        return (
                          <CompressionBlock
                            key={seg.id}
                            phase={seg.compressionPhase || "compressing"}
                          />
                        );
                      }

                      return (
                        <div key={seg.id}>
                          {seg.thinkingContent && (
                            <div className="max-w-[85%]">
                              <ThinkingBlock
                                content={seg.thinkingContent}
                                isStreaming={
                                  !seg.thinkingDone && seg.isStreaming
                                }
                              />
                            </div>
                          )}
                          {seg.content && (
                            <MessageBubble
                              role="assistant"
                              content={seg.content}
                            />
                          )}
                        </div>
                      );
                    })}

                    {turn.status === "failed" && turn.errorMessage && (
                      <div className="flex justify-start">
                        <div className="bg-danger-bg border border-danger-border rounded-lg px-3 py-2 text-xs text-red-600 max-w-[80%]">
                          {turn.errorMessage}
                        </div>
                      </div>
                    )}
                    {turn.status === "interrupted" && (
                      <div className="flex justify-center">
                        <div className="bg-danger-bg border border-danger-border rounded-lg px-3 py-2 text-xs text-red-500 max-w-[80%]">
                          {turn.errorMessage || t("chat.chatInterrupted")}
                        </div>
                      </div>
                    )}
                    {turn.status === "stopped" && (
                      <div className="flex justify-center">
                        <div className="bg-muted/50 border rounded-lg px-3 py-2 text-xs text-muted-foreground max-w-[80%]">
                          {t("chat.chatStopped")}
                        </div>
                      </div>
                    )}
                    {turn.retrying && (
                      <div className="flex justify-center">
                        <div className="bg-warning border border-warning-border rounded-lg px-3 py-2 text-xs text-warning-foreground max-w-[80%] flex items-center gap-2">
                          <Loader2 className="w-3 h-3 animate-spin" />
                          <span>
                            {t("chat.retrying", {
                              attempt: turn.retrying.attempt,
                              max: turn.retrying.maxRetries,
                            })}
                          </span>
                          {turn.retrying.errorMessage && (
                            <span className="opacity-70">
                              · {turn.retrying.errorMessage}
                            </span>
                          )}
                        </div>
                      </div>
                    )}
                    {turn.status === "streaming" &&
                      turn.segments.length === 0 &&
                      !turn.retrying && (
                        <div className="flex justify-start">
                          <div className="bg-muted rounded-lg rounded-bl-sm px-3 py-2">
                            <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
                          </div>
                        </div>
                      )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}

        <div ref={messagesEndRef} />
      </div>

      <ChatInput
        disabled={!hasNovel || !selectedKey}
        isLoading={isLoading}
        isCancelling={cancelChatMutation.isPending}
        placeholder={inputPlaceholder}
        draftKey={draftKey}
        slashItems={slashCommandsQuery.data ?? []}
        onSend={handleSend}
        onListSlash={refreshSlashCommands}
        onStop={() => {
          setTurns((prev) =>
            prev.map((t) =>
              t.status === "streaming"
                ? { ...t, status: "stopped" as const }
                : t,
            ),
          );
          cancelChatMutation
            .mutateAsync(sessionId)
            .catch((err) =>
              toastError(toErrorMessage(err, t("chat.cancelFailed"))),
            );
        }}
      />

      <div className="border-t mx-4" />

      <ChatControls
        onSelectModel={handleSelectModel}
        onRefreshModels={refreshModels}
        onSelectEffort={handleSelectEffort}
        onToggleApproval={handleToggleApproval}
        onConfigModel={handleConfigModel}
        usage={lastUsage}
        onCompress={handleCompress}
        isTurnRunning={isLoading}
        isCompressing={isCompressing}
      />

      {isDragging && (
        <div className="fixed inset-0 z-50 cursor-col-resize select-none" />
      )}

      <SettingsDialog
        open={showSettings}
        onClose={() => setShowSettings(false)}
        initialTab="model"
      />

      <DeleteSessionDialog
        activeSessionId={activeSessionId}
        onActiveSessionDeleted={() => {
          setActiveSessionId(null);
          setTurns([]);
          setSessionId("");
        }}
      />
    </aside>
  );
}
