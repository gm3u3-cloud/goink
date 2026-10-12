//go:build cgo

package mcp_tools_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sigpanic/goink/internal/agentcfg"
	"github.com/sigpanic/goink/internal/mcp_tools"
	"github.com/sigpanic/goink/internal/storage"
	"github.com/sigpanic/goink/internal/volume"
	"github.com/stretchr/testify/require"
)

func TestVolumeToolsDiscoverReadAndRun(t *testing.T) {
	db, tc, ctx := setupRWEnv(t)
	r := newTestRegistry(t)
	allowed := agentcfg.Allowlist(agentcfg.MainAgent)
	call := func(name, raw string) *mcp_tools.ToolResult {
		return r.Execute(ctx, name, json.RawMessage(raw), tc, allowed)
	}
	for _, name := range []string{"get_volumes", "create_volume"} {
		found := call("search_mcp", fmt.Sprintf(`{"queries":[%q]}`, name))
		require.True(t, found.Success, found.Error)
		require.Contains(t, found.Data["content"], "`"+name+"`")
		schema := call("read", fmt.Sprintf(`{"path":"mcp/schemas/%s.json"}`, name))
		require.True(t, schema.Success, schema.Error)
		var document struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		}
		require.NoError(t, json.Unmarshal([]byte(schema.Data["content"].(string)), &document))
		require.Equal(t, name, document.Name)
		require.NotContains(t, string(document.InputSchema), "novel_id")
		require.NotContains(t, string(document.InputSchema), "sort_order")
		for _, tool := range r.OpenAI(nil) {
			require.NotEqual(t, name, tool["function"].(map[string]any)["name"])
		}
	}
	empty := call("run_mcp", `{"tool_name":"get_volumes","args":{}}`)
	require.True(t, empty.Success, empty.Error)
	require.Contains(t, empty.Data["content"], "暂无分卷")
	foreignID := seedVolume(t, db, 2, "其他小说", 1)
	created := call("run_mcp", `{"tool_name":"create_volume","args":{"volumes":[{"name":"起点"},{"name":"终点"}]}}`)
	require.True(t, created.Success, created.Error)
	ids := created.Data["ids"].([]int64)
	require.Len(t, ids, 2)
	require.Equal(t, 2, created.Data["count"])
	inserted := call("run_mcp", fmt.Sprintf(`{"tool_name":"create_volume","args":{"volumes":[{"name":"转折","before_volume_id":%d}]}}`, ids[1]))
	require.True(t, inserted.Success, inserted.Error)
	volumes, err := volume.NewStore(db, nil).ListByNovel(ctx, nil, tc.NovelID)
	require.NoError(t, err)
	require.Len(t, volumes, 3)
	require.Equal(t, []string{"起点", "转折", "终点"}, []string{volumes[0].Name, volumes[1].Name, volumes[2].Name})
	result := call("run_mcp", `{"tool_name":"get_volumes","args":{}}`)
	require.True(t, result.Success, result.Error)
	content := result.Data["content"].(string)
	for i, v := range volumes {
		require.Contains(t, content, fmt.Sprintf("第%d卷 %s [volume_id:%d]", i+1, v.Name, v.ID))
		require.Contains(t, content, fmt.Sprintf("volumes/id_%d.md", v.ID))
	}
	require.NotContains(t, content, "sort_order")
	require.NotContains(t, content, "其他小说")
	foreign, err := volume.NewStore(db, nil).GetByID(ctx, nil, 2, foreignID)
	require.NoError(t, err)
	require.Equal(t, 1, foreign.SortOrder)
}

func TestCreateVolumeRejectsInvalidBatch(t *testing.T) {
	for _, tt := range []struct {
		name string
		args string
	}{
		{"empty", `{"volumes":[]}`},
		{"blank name", `{"volumes":[{"name":"新卷"},{"name":"  "}]}`},
		{"duplicate in batch", `{"volumes":[{"name":"新卷"},{"name":"新卷"}]}`},
		{"existing name", `{"volumes":[{"name":"新卷"},{"name":"已有卷"}]}`},
		{"foreign anchor", `{"volumes":[{"name":"新卷"},{"name":"另一卷","before_volume_id":20}]}`},
		{"missing anchor", `{"volumes":[{"name":"新卷","before_volume_id":99}]}`},
		{"zero anchor", `{"volumes":[{"name":"新卷","before_volume_id":0}]}`},
		{"novel injection", `{"volumes":[{"name":"新卷"}],"novel_id":2}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, tc, ctx := setupRWEnv(t)
			require.NoError(t, db.Create(&volume.Volume{ID: 10, NovelID: 1, Name: "已有卷", SortOrder: 1}).Error)
			require.NoError(t, db.Create(&volume.Volume{ID: 20, NovelID: 2, Name: "外部卷", SortOrder: 1}).Error)
			before, err := volume.NewStore(db, nil).ListByNovel(ctx, nil, 1)
			require.NoError(t, err)
			result := newTestRegistry(t).Execute(ctx, "run_mcp", json.RawMessage(`{"tool_name":"create_volume","args":`+tt.args+`}`), tc, nil)
			require.False(t, result.Success)
			after, err := volume.NewStore(db, nil).ListByNovel(ctx, nil, 1)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestCreateVolumeTransactionAndOperationLog(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail=%t", fail), func(t *testing.T) {
			db, tc, ctx := setupRWEnv(t)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			anchor := seedVolume(t, db, tc.NovelID, "末卷", 1)
			require.NoError(t, db.AutoMigrate(&storage.OperationLogRecord{}))
			require.NoError(t, storage.RegisterOplogHooks(db))
			ctx = storage.WithTurn(ctx, "volume-test", 1)
			if fail {
				require.NoError(t, db.Exec(`CREATE TRIGGER reject_volume BEFORE INSERT ON volumes WHEN NEW.name = '第二卷' BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Error)
			}
			result := newTestRegistry(t).Execute(ctx, "run_mcp", json.RawMessage(fmt.Sprintf(`{"tool_name":"create_volume","args":{"volumes":[{"name":"第一卷","before_volume_id":%d},{"name":"第二卷","before_volume_id":%d}]}}`, anchor, anchor)), tc, nil)
			volumes, err := volume.NewStore(db, nil).ListByNovel(ctx, nil, tc.NovelID)
			require.NoError(t, err)
			var logs []storage.OperationLogRecord
			require.NoError(t, db.Where("session_id = ?", "volume-test").Find(&logs).Error)
			if fail {
				require.False(t, result.Success)
				require.Contains(t, result.Error, "事务已回滚")
				require.Contains(t, result.Error, "第2项")
				require.Len(t, volumes, 1)
				require.Equal(t, 1, volumes[0].SortOrder)
				require.Empty(t, logs)
				return
			}
			require.True(t, result.Success, result.Error)
			require.Len(t, volumes, 3)
			var createdIDs []int64
			for _, log := range logs {
				if log.Operation == "create" && log.Table == "volumes" {
					var key struct {
						ID int64 `json:"id"`
					}
					require.NoError(t, json.Unmarshal([]byte(log.EntityID), &key))
					createdIDs = append(createdIDs, key.ID)
				}
			}
			require.Equal(t, result.Data["ids"], createdIDs)
		})
	}
}
