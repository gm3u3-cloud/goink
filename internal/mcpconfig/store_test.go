package mcpconfig

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
	path := filepath.Join(dir, ".goink", "mcp_config.enc")
	store := NewStore(path)
	empty, err := store.Load()
	require.NoError(t, err)
	require.False(t, empty.Local.Enabled)
	require.Equal(t, DefaultPort, empty.Local.Port)
	require.NotNil(t, empty.Credentials)
	require.NoFileExists(t, path, "读取默认设置不能创建配置文件")
	state := testState()
	require.NoError(t, store.Save(state))
	loaded, err := NewStore(path).Load()
	require.NoError(t, err)
	require.Equal(t, state, loaded)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, secret := range []string{"env-secret", "http-secret", state.LocalToken} {
		require.NotContains(t, string(data), secret)
	}
	plain, err := config.Decrypt(data)
	require.NoError(t, err, "MCP 必须使用共用加密格式和固定密钥")
	var decoded State
	require.NoError(t, json.Unmarshal(plain, &decoded))
	require.Equal(t, state, decoded)
	var fields map[string]json.RawMessage
	connectionJSON, err := json.Marshal(decoded.Connections[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(connectionJSON, &fields))
	require.JSONEq(t, `"stable-id"`, string(fields["id"]))
	require.JSONEq(t, `{"echo":true}`, string(fields["allowed_tools"]))
	require.NotContains(t, fields, "ConnectionSettings", "匿名嵌入应保持配置字段平铺")
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
		Connections: []Connection{
			{ConnectionSettings: ConnectionSettings{ID: "stable-id", Name: "server", Transport: TransportStdio, Command: "test", Enabled: true}, AllowedTools: map[string]bool{"echo": true}},
			{ConnectionSettings: ConnectionSettings{ID: "http-id", Name: "http-server", Transport: TransportStreamableHTTP, Endpoint: "https://example.com/mcp"}},
		},
		Credentials: map[string]Secrets{
			"stable-id": {Env: map[string]string{"KEY": "env-secret"}},
			"http-id":   {BearerToken: "http-secret"},
		},
		Local: Local{Enabled: true, Port: DefaultPort, AllowedTools: []string{"get_chapter_list"}}, LocalToken: strings.Repeat("a", 64),
	}
}

func TestStoreFailedAtomicReplacePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.enc")
	require.NoError(t, os.Mkdir(path, 0700))
	marker := filepath.Join(path, "existing")
	require.NoError(t, os.WriteFile(marker, []byte("preserve"), 0600))
	require.Error(t, NewStore(path).Save(testState()))
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "preserve", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "替换失败也要清理临时文件")
}

func TestStoreNormalizesDefaultsWithoutRewritingOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_config.enc")
	data, err := config.Encrypt([]byte(`{}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	store := NewStore(path)
	state, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, DefaultPort, state.Local.Port)
	require.NotNil(t, state.Credentials)
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
		{"missing_id", func(s *State) { s.Connections[0].ID = "" }},
		{"duplicate_id", func(s *State) { s.Connections = append(s.Connections, s.Connections[0]) }},
		{"empty_name", func(s *State) { s.Connections[0].Name = " " }},
		{"long_name", func(s *State) { s.Connections[0].Name = strings.Repeat("a", 201) }},
		{"unknown_transport", func(s *State) { s.Connections[0].Transport = Transport("unknown") }},
		{"missing_command", func(s *State) { s.Connections[0].Command = "" }},
		{"invalid_command", func(s *State) { s.Connections[0].Command = "bad\x00command" }},
		{"invalid_argument", func(s *State) { s.Connections[0].Args = []string{"bad\x00arg"} }},
		{"stdio_with_endpoint", func(s *State) { s.Connections[0].Endpoint = "https://example.com/mcp" }},
		{"invalid_env_key", func(s *State) { s.Credentials["stable-id"].Env["BAD=KEY"] = "value" }},
		{"empty_env_key", func(s *State) { s.Credentials["stable-id"].Env[""] = "value" }},
		{"invalid_env_value", func(s *State) { s.Credentials["stable-id"].Env["KEY"] = "bad\x00value" }},
		{"stdio_with_token", func(s *State) { s.Credentials["stable-id"] = Secrets{BearerToken: "private-token"} }},
		{"invalid_bearer", func(s *State) { s.Credentials["http-id"] = Secrets{BearerToken: "private\r\ntoken"} }},
		{"invalid_http_scheme", func(s *State) { s.Connections[1].Endpoint = "file:///tmp/mcp" }},
		{"missing_http_host", func(s *State) { s.Connections[1].Endpoint = "https:///mcp" }},
		{"malformed_http_url", func(s *State) { s.Connections[1].Endpoint = "https://example.com/%zz" }},
		{"http_with_command", func(s *State) { s.Connections[1].Command = "test" }},
		{"http_with_args", func(s *State) { s.Connections[1].Args = []string{"--stdio"} }},
		{"http_with_env", func(s *State) { s.Credentials["http-id"] = Secrets{Env: map[string]string{"KEY": "value"}} }},
		{"http_with_userinfo", func(s *State) { s.Connections[1].Endpoint = "https://user:private@example.com/mcp" }},
		{"http_with_query", func(s *State) { s.Connections[1].Endpoint = "https://example.com/mcp?token=private" }},
		{"http_with_fragment", func(s *State) { s.Connections[1].Endpoint = "https://example.com/mcp#private" }},
		{"disabled_invalid_connection", func(s *State) { s.Connections[0].Enabled = false; s.Connections[0].Command = "" }},
		{"negative_port", func(s *State) { s.Local.Port = -1 }},
		{"oversized_port", func(s *State) { s.Local.Port = 65536 }},
		{"enabled_without_token", func(s *State) { s.LocalToken = "" }},
		{"malformed_token", func(s *State) { s.LocalToken = strings.Repeat("x", 64) }},
		{"short_token", func(s *State) { s.LocalToken = strings.Repeat("a", 62) }},
		{"disabled_with_invalid_token", func(s *State) { s.Local.Enabled = false; s.LocalToken = "invalid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp_config.enc")
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
