package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/mcpclient"
	"github.com/sigpanic/goink/internal/mcpclient/clientcfg"
	"github.com/sigpanic/goink/internal/mcpserver"
	"github.com/sigpanic/goink/internal/search"
	"github.com/stretchr/testify/require"
)

type mcpContextKey string

func TestMCPToolContextPreservesRequestLifetimeAndLoadsDependencies(t *testing.T) {
	a := setupTestApp(t)
	key := mcpContextKey("runtime-value")
	a.ctx = context.WithValue(context.Background(), key, "desktop")
	request, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx, tc := a.mcpToolContext(request)
	require.Equal(t, "desktop", ctx.Value(key))
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	requestDeadline, _ := request.Deadline()
	require.Equal(t, requestDeadline, deadline)
	require.Same(t, a.skill, tc.SkillStore)
	require.Nil(t, tc.Approver)
	require.Nil(t, tc.EmitApproval)
	require.Nil(t, tc.RunSubAgent)
	svc := search.NewService(a.logger, a.character, a.location, a.timeline, a.storyarc, a.chapter, a.reader, a.preference, a.setting, nil)
	a.searchService.Store(svc)
	ctx, tc = a.mcpToolContext(context.WithValue(request, key, "request"))
	require.Equal(t, "request", ctx.Value(key))
	require.Same(t, svc, tc.SearchService)
	_, err := tc.WebSearch(ctx, "查询")
	require.ErrorContains(t, err, "DeepSeek API key not configured")
	cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	select {
	case <-ctx.Done():
	default:
		t.Fatal("MCP 请求取消没有传递到工具上下文")
	}
}

func TestMCPReadEditAndBuiltinSkillWithInjectedDependencies(t *testing.T) {
	a := setupTestApp(t)
	sqlDB, err := a.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	novel, err := a.CreateNovel(CreateNovelInput{Title: "MCP 读写"})
	require.NoError(t, err)
	require.NoError(t, a.SetActiveNovel(SetActiveNovelInput{NovelID: novel.ID}))
	svc := search.NewService(a.logger, a.character, a.location, a.timeline, a.storyarc, a.chapter, a.reader, a.preference, a.setting, nil)
	a.searchService.Store(svc)
	server := mcpserver.New(a.registry, a.db, a.currentNovel, a.logger)
	require.NoError(t, server.SetAllowedTools([]string{"read", "edit"}))
	require.NoError(t, server.SetToolContextProvider(a.mcpToolContext))
	require.NoError(t, server.SetPort(0))
	endpoint, err := server.Start(strings.Repeat("a", 64))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Stop(ctx))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mcpclient.Connect(ctx, mcpclient.Config{
		TransportSettings: clientcfg.TransportSettings{Transport: clientcfg.TransportStreamableHTTP, Endpoint: endpoint.URL},
		HTTPClient:        &http.Client{Transport: mcpTestAuthTransport{token: endpoint.Token}},
	})
	require.NoError(t, err)
	defer client.Close()
	result, err := client.CallTool(ctx, "edit", map[string]any{
		"path": "chapters/new.md", "change_type": "full_replace", "new_content": "旧关键字", "reason": "创建章节",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.StructuredContent)
	id := int64(result.StructuredContent.(map[string]any)["chapter_id"].(float64))
	path := git.ChapterPath(id)
	hits, err := svc.SearchAll(ctx, novel.ID, "旧关键字")
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	result, err = client.CallTool(ctx, "edit", map[string]any{
		"path": path, "change_type": "full_replace", "new_content": "新关键字", "reason": "更新章节",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.StructuredContent)
	hits, err = svc.SearchAll(ctx, novel.ID, "旧关键字")
	require.NoError(t, err)
	require.Empty(t, hits, "编辑后不能继续命中旧缓存")
	hits, err = svc.SearchAll(ctx, novel.ID, "新关键字")
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	result, err = client.CallTool(ctx, "read", map[string]any{"path": path, "include_lines": false})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Equal(t, "新关键字", result.StructuredContent.(map[string]any)["content"])
	metadata := a.skill.ListMeta(novel.ID)
	for _, sk := range metadata {
		if sk.Source != "builtin" {
			continue
		}
		result, err = client.CallTool(ctx, "read", map[string]any{"path": fmt.Sprintf("/builtin/skills/%s.md", sk.Name)})
		require.NoError(t, err)
		require.False(t, result.IsError, result.StructuredContent)
		return
	}
	t.Fatal("缺少用于验证读取依赖的内置技能")
}
