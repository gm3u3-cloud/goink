package clientcfg

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
	path := filepath.Join(dir, ".goink", "mcp_client_config.enc")
	store := NewStore(path)
	empty, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, empty.Credentials)
	require.NoFileExists(t, path, "读取默认设置不能创建配置文件")
	state := testState()
	require.NoError(t, store.Save(state))
	loaded, err := NewStore(path).Load()
	require.NoError(t, err)
	require.Equal(t, state, loaded)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, secret := range []string{"env-secret", "http-secret"} {
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
	require.JSONEq(t, `"stdio"`, string(fields["transport"]))
	require.JSONEq(t, `"test"`, string(fields["command"]))
	require.JSONEq(t, `{"echo":true}`, string(fields["allowed_tools"]))
	require.NotContains(t, fields, "ConnectionSettings", "匿名嵌入应保持配置字段平铺")
	require.NotContains(t, fields, "TransportSettings", "两层匿名嵌入应保持传输字段平铺")
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
			{ConnectionSettings: ConnectionSettings{ID: "stable-id", Name: "server", TransportSettings: TransportSettings{Transport: TransportStdio, Command: "test"}, Enabled: true}, AllowedTools: map[string]bool{"echo": true}},
			{ConnectionSettings: ConnectionSettings{ID: "http-id", Name: "http-server", TransportSettings: TransportSettings{Transport: TransportStreamableHTTP, Endpoint: "https://example.com/mcp"}}},
		},
		Credentials: map[string]Secrets{
			"stable-id": {Env: map[string]string{"KEY": "env-secret"}},
			"http-id":   {BearerToken: "http-secret"},
		},
	}
}

func TestStoreFailedAtomicReplacePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_client_config.enc")
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
	path := filepath.Join(t.TempDir(), "mcp_client_config.enc")
	data, err := config.Encrypt([]byte(`{}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	store := NewStore(path)
	state, err := store.Load()
	require.NoError(t, err)
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

func TestStoreLoadsFlatConnectionSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_client_config.enc")
	plain := []byte(`{"connections":[{"id":"flat-id","name":"server","transport":"stdio","command":"test","args":["--stdio"],"endpoint":"","enabled":true,"allowed_tools":{"echo":true}}],"credentials":{"flat-id":{"env":{"KEY":"value"},"bearer_token":""}}}`)
	data, err := config.Encrypt(plain)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	store := NewStore(path)
	state, err := store.Load()
	require.NoError(t, err)
	require.Len(t, state.Connections, 1)
	connection := state.Connections[0]
	require.Equal(t, "flat-id", connection.ID)
	require.Equal(t, TransportSettings{Transport: TransportStdio, Command: "test", Args: []string{"--stdio"}}, connection.TransportSettings)
	require.NoError(t, store.Save(state))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	saved, err := config.Decrypt(data)
	require.NoError(t, err)
	require.JSONEq(t, string(plain), string(saved), "抽出公共传输结构不能改变持久化字段布局")
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp_client_config.enc")
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
