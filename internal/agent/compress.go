package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/agentcfg"
	"github.com/sigpanic/goink/internal/session"
)

const compressionPrompt = `<system-reminder>
You are a context compression assistant. Based on the full conversation history, generate a structured summary for context recovery in subsequent turns.

## Completed items
(One sentence each, at most 15, starting from the most recent. Do not repeat items that were already executed.)

## In progress (breakpoint)
(The most detailed part: what is currently being done, at which step, and what is planned next. This is the most important part — be as thorough as possible.)

## User preferences and requirements
(The core writing style, constraints, and repeatedly emphasized points distilled from the user's messages.)

## Key decisions and setting changes
(Confirmed plot directions, character settings, worldbuilding rules, naming, and other decisions.)

## To-do items
(The list of planned tasks that have not started yet.)
</system-reminder>`

const compressionReminder = "<system-reminder>\nThe context has been compressed. Please continue working based on the summary below.\n</system-reminder>"

const maxUserRetain = 15
const minConversationTurns = 4

// generateSummary 调用 LLM 生成压缩摘要，返回摘要文本和应保留的历史消息。
func (a *Agent) generateSummary(ctx context.Context, opts *RunOptions) (string, []map[string]any, error) {
	msgs := make([]map[string]any, len(opts.Messages)+1)
	copy(msgs, opts.Messages)
	msgs[len(opts.Messages)] = map[string]any{"role": "user", "content": compressionPrompt}

	summary, err := a.llm.GenerateText(ctx, opts.ProviderName, msgs, opts.Model.ID)
	if err != nil {
		a.logger.Warn("summary generation failed, keeping original context", "err", err)
		return "", nil, fmt.Errorf("compress: LLM summary failed: %w", err)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		a.logger.Warn("summary is empty, keeping original context")
		return "", nil, fmt.Errorf("compress: empty summary")
	}

	a.logger.Debug("summary generated", "summary_len", len(summary))
	retained := retainMessages(msgs[:len(opts.Messages)])
	return summary, retained, nil
}

// Compress 执行主 Agent 上下文压缩：调 LLM 生成摘要，重建 SkillCatalog/NovelState，保留近期关键消息。
// opts 不会被修改，成功后才赋值新的 Messages / ActiveVersion / runningTokens。
func (a *Agent) Compress(ctx context.Context, opts *RunOptions, runningTokens map[string]int) error {
	a.logger.Info("starting context compression",
		"session_id", opts.SessionID,
		"turn_id", opts.TurnID,
		"estimated_tokens", sumRunningTokens(runningTokens),
		"msg_count", len(opts.Messages),
	)

	a.emitCompression(ctx, opts.TurnID, "compressing", "", "")

	summary, retained, err := a.generateSummary(ctx, opts)
	if err != nil {
		return err
	}

	// 重建系统消息（与 chat 走同一条路径 BuildSystemMessages，事务外构建）
	sysMsgs, buildErr := agentcfg.BuildSystemMessages(ctx, a.db, opts.NovelID, a.skillStore)
	if buildErr != nil {
		a.logger.Warn("partial failure building system messages during compression", "novel_id", opts.NovelID, "err", buildErr)
	}

	// 在事务中完成版本递增 + 全部 DB 写入
	newVersion, err := a.persistCompression(ctx, opts, sysMsgs, summary, retained)
	if err != nil {
		return fmt.Errorf("compress: persist failed: %w", err)
	}

	// 从 DB 加载新版本消息，与 Chat() 走同一条路径
	apiMsgs, err := a.session.GetMessagesForAPI(ctx, opts.SessionID, newVersion)
	if err != nil {
		return fmt.Errorf("compress: load messages after persist: %w", err)
	}
	opts.ActiveVersion = newVersion
	opts.Messages = make([]map[string]any, len(apiMsgs))
	for i, m := range apiMsgs {
		opts.Messages[i] = m.ToAPIFormat(a.logger)
	}

	newTokens := a.InitRunningTokens(opts.Messages)
	clear(runningTokens)
	maps.Copy(runningTokens, newTokens)

	a.emitCompression(ctx, opts.TurnID, "done", summary, "")

	a.logger.Info("context compression complete",
		"session_id", opts.SessionID,
		"new_version", newVersion,
		"retained", len(retained),
		"new_msg_count", len(opts.Messages),
	)
	return nil
}

// compressInMemory 执行子 Agent 上下文压缩：纯内存操作，AgentIdentity/NovelState 不动，仅写边界标记到 DB。
func (a *Agent) compressInMemory(ctx context.Context, opts *RunOptions, runningTokens map[string]int) error {
	a.logger.Info("sub-agent context compression",
		"session_id", opts.SessionID,
		"turn_id", opts.TurnID,
		"agent_type", opts.AgentType,
		"sub_task_id", opts.SubTaskID,
		"estimated_tokens", sumRunningTokens(runningTokens),
		"msg_count", len(opts.Messages),
	)

	a.emitCompression(ctx, opts.TurnID, "compressing", "", opts.SubTaskID)

	summary, retained, err := a.generateSummary(ctx, opts)
	if err != nil {
		return err
	}

	// 提取头部 system 消息，不动
	sysEnd := 0
	for i, m := range opts.Messages {
		role, _ := m["role"].(string)
		if role == "system" {
			sysEnd = i + 1
		} else {
			break
		}
	}
	sysMsgs := make([]map[string]any, sysEnd)
	copy(sysMsgs, opts.Messages[:sysEnd])

	// 内存重建 opts.Messages
	newMsgs := make([]map[string]any, 0, sysEnd+2+len(retained))
	newMsgs = append(newMsgs, sysMsgs...)
	newMsgs = append(newMsgs,
		map[string]any{"role": "user", "content": compressionReminder},
		map[string]any{"role": "user", "content": "<system-reminder>\n" + summary + "\n</system-reminder>"},
	)
	newMsgs = append(newMsgs, retained...)
	opts.Messages = newMsgs

	// 边界标记
	if err := a.db.Create(&session.Message{
		SessionID:  opts.SessionID,
		TurnID:     opts.TurnID,
		Role:       "system",
		Content:    "",
		Version:    opts.ActiveVersion,
		ToAPI:      false,
		ToFrontend: true,
		EventType:  session.EventCompression,
		AgentType:  opts.AgentType,
		SubTaskID:  opts.SubTaskID,
	}).Error; err != nil {
		a.logger.Warn("failed to write sub-agent compression boundary marker", "err", err)
	}

	newTokens := a.InitRunningTokens(opts.Messages)
	clear(runningTokens)
	maps.Copy(runningTokens, newTokens)

	opts.SubAgentVersion++

	a.emitCompression(ctx, opts.TurnID, "done", summary, opts.SubTaskID)

	a.logger.Info("sub-agent context compression complete",
		"session_id", opts.SessionID,
		"sub_agent_version", opts.SubAgentVersion,
		"retained", len(retained),
		"new_msg_count", len(opts.Messages),
	)
	return nil
}

// persistCompression 在事务中递增 active_version 并写入所有压缩消息。
func (a *Agent) persistCompression(ctx context.Context, opts *RunOptions, msgs *agentcfg.SystemMessages, summary string, retained []map[string]any) (int, error) {
	var newVersion int

	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sess session.Session
		if err := tx.First(&sess, "session_id = ?", opts.SessionID).Error; err != nil {
			return fmt.Errorf("failed to query session: %w", err)
		}
		sess.ActiveVersion++
		if err := tx.Save(&sess).Error; err != nil {
			return fmt.Errorf("failed to increment active_version: %w", err)
		}
		newVersion = sess.ActiveVersion

		msg := func(role, content string, toAPI, toFE bool, eventType session.EventType) error {
			return tx.Create(&session.Message{
				SessionID:  opts.SessionID,
				TurnID:     opts.TurnID,
				Role:       role,
				Content:    content,
				Version:    newVersion,
				ToAPI:      toAPI,
				ToFrontend: toFE,
				EventType:  eventType,
				AgentType:  "main",
			}).Error
		}

		// system 消息按 Identity/Always/Catalog/Profile/State 顺序写入
		sysContents := []struct {
			name    string
			content string
		}{
			{"AgentIdentity", msgs.Identity},
			{"AlwaysSkills", msgs.Always},
			{"SkillCatalog", msgs.Catalog},
			{"NovelProfile", msgs.Profile},
			{"NovelState", msgs.State},
		}
		for _, sc := range sysContents {
			if sc.content == "" {
				continue
			}
			if err := msg("system", sc.content, true, false, ""); err != nil {
				return fmt.Errorf("write %s: %w", sc.name, err)
			}
		}
		// 提醒语
		if err := msg("user", compressionReminder, true, false, ""); err != nil {
			return fmt.Errorf("write compression reminder: %w", err)
		}
		// 摘要
		if err := msg("user", "<system-reminder>\n"+summary+"\n</system-reminder>", true, false, ""); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
		// 保留消息副本
		for _, m := range retained {
			rm := apiMsgToMessage(m, opts.SessionID, opts.TurnID, newVersion)
			if err := tx.Create(rm).Error; err != nil {
				return fmt.Errorf("failed to write retained messages: %w", err)
			}
		}
		// 边界标记
		if err := msg("system", "", false, true, session.EventCompression); err != nil {
			return fmt.Errorf("write compression marker: %w", err)
		}

		return nil
	})

	return newVersion, err
}

// apiMsgToMessage 将 API 格式的 map 反向转换为 session.Message，提取 ExtraMetadata。
func apiMsgToMessage(m map[string]any, sessionID string, turnID, version int) *session.Message {
	role, _ := m["role"].(string)
	content, _ := m["content"].(string)

	msg := &session.Message{
		SessionID:  sessionID,
		TurnID:     turnID,
		Role:       role,
		Content:    content,
		Version:    version,
		ToAPI:      true,
		ToFrontend: false,
		AgentType:  "main",
	}

	if tc, ok := m["reasoning_content"].(string); ok {
		msg.ThinkingContent = tc
	}

	meta := make(map[string]any)
	switch role {
	case "assistant":
		if tc, ok := m["tool_calls"]; ok && tc != nil {
			meta["tool_calls"] = tc
		}
	case "tool":
		if id, ok := m["tool_call_id"]; ok && id != nil {
			meta["tool_call_id"] = id
		}
		if name, ok := m["name"]; ok && name != nil {
			meta["tool_name"] = name
		}
	}
	if len(meta) > 0 {
		b, _ := json.Marshal(meta)
		msg.ExtraMetadata = string(b)
	}

	return msg
}

// emitCompression 推送压缩事件到前端。
func (a *Agent) emitCompression(ctx context.Context, turnID int, phase, summary, subTaskID string) {
	wails.EventsEmit(ctx, "agent:"+strconv.Itoa(turnID), AgentEvent{
		TurnID:           turnID,
		Type:             EventCompression,
		CompressionPhase: phase,
		Summary:          summary,
		SubTaskID:        subTaskID,
		Timestamp:        time.Now(),
	})
}

// retainMessages 筛选应保留的历史消息。
// 跳过前 N 条 system 消息，后续应用保留规则。
func retainMessages(messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return nil
	}

	// 跳过前部 system 消息
	sysEnd := 0
	for _, m := range messages {
		role, _ := m["role"].(string)
		if role == "system" {
			sysEnd++
		} else {
			break
		}
	}

	history := messages[sysEnd:]
	if len(history) == 0 {
		return nil
	}

	// 找到所有 user 消息的位置
	userIdx := make([]int, 0)
	for i, m := range history {
		role, _ := m["role"].(string)
		if role == "user" {
			userIdx = append(userIdx, i)
		}
	}

	if len(userIdx) == 0 {
		return nil
	}

	// 保留最近 maxUserRetain 条 user 消息
	keepFrom := 0
	if len(userIdx) > maxUserRetain {
		keepFrom = userIdx[len(userIdx)-maxUserRetain]
	}

	// 确保至少保留 minConversationTurns 轮对话
	if len(userIdx) >= minConversationTurns {
		minKeep := userIdx[len(userIdx)-minConversationTurns]
		if minKeep < keepFrom {
			keepFrom = minKeep
		}
	}

	retained := make([]map[string]any, 0, len(history)-keepFrom)
	for _, m := range history[keepFrom:] {
		dup := make(map[string]any, len(m))
		maps.Copy(dup, m)
		retained = append(retained, dup)
	}

	return retained
}

// IsRunning 检查指定 session 是否有正在进行的 turn。
func (a *Agent) IsRunning(sessionID string) bool {
	return a.cancelMgr.IsRegistered(CancelPrefixChat + sessionID)
}
