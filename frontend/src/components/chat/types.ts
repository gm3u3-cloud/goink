import type { session } from "@/lib/wailsjs/go/models";

// Corresponds one-to-one with the AgentEventType enum in Go's
// internal/agent/events.go
export enum AgentEventType {
  Thinking = 0,
  ThinkingDone = 1,
  Content = 2,
  ToolCall = 3,
  Usage = 4,
  Error = 5,
  Retrying = 6, // Retrying a recoverable error
  Compression = 7,
}

// AgentEvent corresponds one-to-one with the JSON serialization of Go's
// AgentEvent
export interface AgentEvent {
  turn_id: number;
  sub_task_id?: string;
  seq?: number;
  type: AgentEventType;
  data?: string;
  tool_name?: string;
  tool_id?: string;
  phase?: string; // "selected" | "executing" | "awaiting_approval" | "completed" | "failed" | "loop_detected"
  tool_args?: Record<string, unknown>;
  success?: boolean;
  error?: string;
  display_text?: string;
  activity_kind?: string;
  metadata?: Record<string, unknown>;
  usage?: Record<string, unknown>;
  compression_phase?: string; // "compressing" | "done"
  summary?: string;
  attempt?: number; // On EventRetrying: which retry attempt (1-indexed)
  max_retries?: number; // On EventRetrying: max retry count
  backoff_ms?: number; // On EventRetrying: backoff for this attempt in ms
  clear_from_seq?: number; // On EventRetrying: the seq of the start event of this streamLoop iteration (the frontend clears this iteration's partial segments based on it)
  timestamp: string;
}

// TurnSegment is a fragment within a turn: a text block, tool call, sub
// Agent, or compression marker
export interface TurnSegment {
  id: string;
  type: "text" | "tool" | "subagent" | "compression";
  content: string;
  thinkingContent: string;
  thinkingDone: boolean;
  isStreaming: boolean;
  // tool
  toolName: string;
  toolId: string;
  toolStatus: "executing" | "awaiting_approval" | "completed" | "failed";
  displayText: string;
  activityKind: string;
  error: string;
  // approval
  approvalType?: string;
  approvalPayload?: Record<string, unknown>;
  // subagent
  status?: "streaming" | "done" | "failed";
  agentType?: "memory" | "review";
  taskId?: string;
  segments?: TurnSegment[];
  finalText?: string;
  // compression
  compressionPhase?: "compressing" | "done";
  // Rich-text result for web_search / web_fetch
  result?: Record<string, unknown>;
  // P2: the event seq when this segment was created (for live streaming,
  // >= 1)
  // Historical segments created by rebuildTurns default to 0 and are never
  // cleared by EventRetrying (clear_from_seq >= 1)
  firstSeq: number;
  // P1: sub-agent retry state (the main turn uses Turn.retrying; subagent
  // segments use this field)
  // Only segments with type='subagent' use this; set when EventRetrying
  // carries a sub_task_id
  retrying?: {
    attempt: number;
    maxRetries: number;
    errorMessage: string;
  } | null;
  // P1: the sub-agent's final failure reason (set when EventError carries a
  // sub_task_id)
  // Difference from retrying.errorMessage: retrying is lost once set to
  // null; errorMessage persists
  errorMessage?: string;
}

export function emptySegment(id: string): TurnSegment {
  return {
    id,
    type: "text",
    content: "",
    thinkingContent: "",
    thinkingDone: false,
    isStreaming: false,
    toolName: "",
    toolId: "",
    toolStatus: "executing",
    displayText: "",
    activityKind: "",
    error: "",
    firstSeq: 0,
  };
}

// filterSegmentsBySeq clears the partial segments already rendered by this
// streamLoop iteration.
// Rule: keep segments with firstSeq < clearFromSeq (history + previous
// turns).
// - Historical segments (created by rebuildTurns) have firstSeq=0 and
// clearFromSeq >= 1, so they are always kept
// - This iteration's segments have firstSeq >= clearFromSeq and are cleared
// The backend emits EventRetrying with clear_from_seq = streamStartSeq =
// *eventSeq + 1
export function filterSegmentsBySeq(
  segments: TurnSegment[],
  clearFromSeq: number,
): TurnSegment[] {
  return segments.filter((s) => s.firstSeq < clearFromSeq);
}

// Turn is one conversation turn: the user message + the AI reply's segments
export interface Turn {
  id: string;
  turnId: number;
  userMessage: string;
  segments: TurnSegment[];
  status: "streaming" | "done" | "failed" | "interrupted" | "stopped";
  errorMessage?: string;
  compressionOnly?: boolean; // Pure compression turn (manual compression), no user message
  retrying?: {
    // P2: recoverable error retry state
    attempt: number; // which retry attempt (1-indexed)
    maxRetries: number; // max retry count
    errorMessage: string; // the error message that triggered the retry
  } | null;
}

export function rebuildTurns(messages: session.Message[]): Turn[] {
  const turns: Turn[] = [];
  let current: Turn | null = null;
  let segCounter = 0;
  const subagentCache = new Map<string, TurnSegment>();

  for (const msg of messages) {
    // Interruption marker: distinguish user stop from system interruption by
    // event_type
    if (
      msg.event_type === "user_stopped" ||
      msg.event_type === "system_interrupted" ||
      msg.event_type === "error"
    ) {
      const target = turns.find((t) => t.turnId === msg.turn_id);
      if (target) {
        if (msg.event_type === "user_stopped") {
          target.status = "stopped";
        } else if (msg.event_type === "error") {
          target.status = "failed";
          target.errorMessage = msg.content;
        } else {
          target.status = "interrupted";
          target.errorMessage = msg.content;
        }
      }
      continue;
    }

    // Compression boundary marker: becomes a standalone pure compression turn
    if (msg.event_type === "compression") {
      if (msg.agent_type !== "main" && msg.sub_task_id) {
        const cached = subagentCache.get(msg.sub_task_id);
        if (cached && cached.segments) {
          cached.segments.push({
            ...emptySegment(`seg_${segCounter++}`),
            type: "compression",
            compressionPhase: "done",
          });
        }
        continue;
      }
      if (current && current.turnId === msg.turn_id) {
        current.segments.push({
          ...emptySegment(`seg_${segCounter++}`),
          type: "compression",
          compressionPhase: "done",
        });
        continue;
      }
      turns.push({
        id: `comp_${msg.turn_id}`,
        turnId: msg.turn_id,
        userMessage: "",
        segments: [
          {
            ...emptySegment(`comp_${msg.turn_id}`),
            type: "compression",
            compressionPhase: "done",
          },
        ],
        status: "done",
        compressionOnly: true,
      });
      continue;
    }

    if (msg.role === "user") {
      current = {
        id: `hist_${msg.turn_id}`,
        turnId: msg.turn_id,
        userMessage: msg.content,
        segments: [],
        status: "done",
      };
      turns.push(current);
    } else if (msg.role === "assistant") {
      // Sub-Agent message: agent_type !== 'main' and has a sub_task_id
      if (msg.agent_type !== "main" && msg.sub_task_id && current) {
        const subTaskId = msg.sub_task_id;
        const cached = subagentCache.get(subTaskId);
        const subSeg: TurnSegment =
          cached ??
          (() => {
            const seg: TurnSegment = {
              ...emptySegment(`seg_${segCounter++}`),
              type: "subagent",
              status: "done",
              agentType: (msg.agent_type as "memory" | "review") || "memory",
              taskId: subTaskId,
              segments: [],
              finalText: "",
            };
            // Do not push directly — wait until the run_subagent
            // tool_display is encountered, then insert at the correct
            // position
            subagentCache.set(subTaskId, seg);
            return seg;
          })();

        // Append the sub-agent's text content
        if ((msg.content || msg.thinking_content) && subSeg.segments) {
          subSeg.segments.push({
            ...emptySegment(`seg_${segCounter++}`),
            type: "text",
            content: msg.content || "",
            thinkingContent: msg.thinking_content || "",
            thinkingDone: true,
            isStreaming: false,
          });
          if (msg.content) {
            subSeg.finalText =
              subSeg.finalText || ""
                ? subSeg.finalText + "\n" + msg.content
                : msg.content;
          }
        }

        // The sub-agent's tool calls
        const toolDisplays = parseToolDisplays(msg.extra_metadata);
        if (toolDisplays.length > 0 && subSeg.segments) {
          for (const td of toolDisplays) {
            const phase = td.phase as
              "completed" | "failed" | "executing" | undefined;
            subSeg.segments.push({
              ...emptySegment(`seg_${segCounter++}`),
              type: "tool",
              toolName: td.tool_name,
              toolId: td.tool_id,
              toolStatus:
                phase &&
                (phase === "executing" ||
                  phase === "completed" ||
                  phase === "failed")
                  ? phase
                  : "completed",
              displayText: td.display_text,
              activityKind: td.activity_kind,
              error: td.error || "",
            });
          }
        }
        continue;
      }

      // Main Agent message
      if (!current) continue;

      const thinkingContent = msg.thinking_content || "";

      // Text content
      if (msg.content || thinkingContent) {
        current.segments.push({
          ...emptySegment(`seg_${segCounter++}`),
          type: "text",
          content: msg.content || "",
          thinkingContent,
          thinkingDone: true,
          isStreaming: false,
        });
      }

      // Tool display info
      const toolDisplays = parseToolDisplays(msg.extra_metadata);
      for (const td of toolDisplays) {
        // run_subagent: insert the sub-Agent card at this position
        if (td.tool_name === "run_subagent") {
          const cached = subagentCache.get(td.tool_id);
          if (cached) {
            // P1: correct the subagent segment's status from the
            // tool_display's phase
            // rebuildTurns creates the subagent segment with status='done'
            // by default; here it is corrected to failed based on the
            // run_subagent phase (history playback preserves the failed
            // state)
            if (td.phase === "failed") {
              cached.status = "failed";
            }
            // Restore errorMessage from tool_display.error (history
            // playback)
            if (td.error) {
              cached.errorMessage = td.error;
            }
            current.segments.push(cached);
          }
          continue;
        }
        current.segments.push({
          ...emptySegment(`seg_${segCounter++}`),
          type: "tool",
          toolName: td.tool_name,
          toolId: td.tool_id,
          toolStatus:
            td.phase === "completed" ||
            td.phase === "failed" ||
            td.phase === "executing"
              ? td.phase
              : "completed",
          displayText: td.display_text || td.tool_name,
          activityKind: td.activity_kind || "",
          error: td.error || "",
          result: td.result,
        });
      }
    }
  }

  return turns;
}

interface ToolDisplay {
  tool_id: string;
  tool_name: string;
  display_text: string;
  activity_kind: string;
  phase: string;
  result?: Record<string, unknown>;
  error?: string;
}

function parseToolDisplays(extraMetadata?: string): ToolDisplay[] {
  if (!extraMetadata) return [];
  try {
    const meta = JSON.parse(extraMetadata);
    if (meta.tool_displays && Array.isArray(meta.tool_displays)) {
      return meta.tool_displays as ToolDisplay[];
    }
    return [];
  } catch {
    return [];
  }
}
