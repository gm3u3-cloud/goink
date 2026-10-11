package servercfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sigpanic/goink/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStoreEncryptedRoundTripAndCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".goink", "mcp_server_config.enc")
	store := NewStore(path)
	empty, err := store.Load()
	require.NoError(t, err)
	require.False(t, empty.Local.Enabled)
	require.Equal(t, DefaultPort, empty.Local.Port)
	require.NoFileExists(t, path, "读取默认设置不能创建配置文件")
	state := testState()
	require.NoError(t, store.Save(state))
	loaded, err := NewStore(path).Load()
	require.NoError(t, err)
	require.Equal(t, state, loaded)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, secret := range []string{state.LocalToken} {
		require.NotContains(t, string(data), secret)
	}
	plain, err := config.Decrypt(data)
	require.NoError(t, err, "MCP 必须使用共用加密格式和固定密钥")
	var decoded State
	require.NoError(t, json.Unmarshal(plain, &decoded))
	require.Equal(t, state, decoded)
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1, "新存储不能产生独立密钥或残留临时文件")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		info, err = os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	}
	require.NoError(t, store.Save(state))
	next, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEqual(t, data, next, "重复保存应使用新 nonce")
	data[len(data)-1] ^= 1
	require.NoError(t, os.WriteFile(path, data, 0600))
	_, err = store.Load()
	require.Error(t, err)
	remaining, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, remaining)
}

func testState() State {
	return State{
		Local: Local{Enabled: true, Port: DefaultPort, AllowedTools: []string{"get_chapter_list"}}, LocalToken: strings.Repeat("a", 64),
	}
}

func TestStoreFailedAtomicReplacePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_server_config.enc")
	require.NoError(t, os.Mkdir(path, 0700))
	marker := filepath.Join(path, "existing")
	require.NoError(t, os.WriteFile(marker, []byte("preserve"), 0600))
	err := NewStore(path).Save(testState())
	var renameErr *os.LinkError
	require.ErrorAs(t, err, &renameErr)
	_, err = NewStore(path).Load()
	var readErr *os.PathError
	require.ErrorAs(t, err, &readErr)
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "preserve", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "替换失败也要清理临时文件")
}

func TestStorePreservesJSONError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_server_config.enc")
	data, err := config.Encrypt([]byte("{"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	_, err = NewStore(path).Load()
	var parseErr *json.SyntaxError
	require.ErrorAs(t, err, &parseErr)
	require.ErrorContains(t, err, "解析 MCP 配置失败")
}

func TestStoreNormalizesDefaultsWithoutRewritingOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_server_config.enc")
	data, err := config.Encrypt([]byte(`{}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	store := NewStore(path)
	state, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, DefaultPort, state.Local.Port)
	remaining, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, remaining, "加载补默认值不能自动改写文件")
	require.NoError(t, store.Save(State{}))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	plain, err := config.Decrypt(data)
	require.NoError(t, err)
	var saved State
	require.NoError(t, json.Unmarshal(plain, &saved))
	require.Equal(t, state, saved, "保存也应补齐同一套默认值")
}

func TestStoreRejectsInvalidSnapshotsAndPreservesFiles(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*State)
	}{
		{"negative_port", func(s *State) { s.Local.Port = -1 }},
		{"oversized_port", func(s *State) { s.Local.Port = 65536 }},
		{"enabled_without_token", func(s *State) { s.LocalToken = "" }},
		{"malformed_token", func(s *State) { s.LocalToken = strings.Repeat("x", 64) }},
		{"short_token", func(s *State) { s.LocalToken = strings.Repeat("a", 62) }},
		{"disabled_with_invalid_token", func(s *State) { s.Local.Enabled = false; s.LocalToken = "invalid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp_server_config.enc")
			store := NewStore(path)
			require.NoError(t, store.Save(testState()))
			original, err := os.ReadFile(path)
			require.NoError(t, err)
			invalid := testState()
			tc.mutate(&invalid)
			err = store.Save(invalid)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private", "校验错误不能暴露凭据")
			remaining, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, original, remaining, "非法保存不能覆盖原配置")
			plain, err := json.Marshal(invalid)
			require.NoError(t, err)
			corrupted, err := config.Encrypt(plain)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, corrupted, 0600))
			_, err = store.Load()
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private", "加载错误不能暴露凭据")
			remaining, err = os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, corrupted, remaining, "非法加载不能覆盖异常配置")
		})
	}
}
