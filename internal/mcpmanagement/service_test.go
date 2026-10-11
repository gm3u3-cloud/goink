package mcpmanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/mcp_tools"
	"github.com/sigpanic/goink/internal/mcpclient"
	"github.com/sigpanic/goink/internal/mcpconfig"
	"github.com/sigpanic/goink/internal/mcpserver"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, store Repository) (*Service, *mcpclient.Manager) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := mcp_tools.NewRegistry(logger)
	mcp_tools.RegisterAllTools(registry)
	server := mcpserver.New(registry, nil, func(context.Context) (mcpserver.CurrentNovel, error) {
		return mcpserver.CurrentNovel{ID: 7, Title: "测试小说"}, nil
	}, logger, nil)
	manager := mcpclient.NewManager()
	service, err := New(store, manager, server)
	require.NoError(t, err)
	if service.state.Local.Port == mcpconfig.DefaultPort && service.state.LocalToken == "" {
		service.state.Local.Port = freeLocalPort(t)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, service.Close(ctx))
	})
	return service, manager
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func TestConnectionPersistenceStableIDAndSecrets(t *testing.T) {
	store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
	service, _ := newTestService(t, store)
	var input ConnectionInput
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Original","transport":"stdio","command":"example","args":["--stdio"],"credentials":{"env":{"API_KEY":"private-env"}}}`), &input))
	created, err := service.SaveConnection(input)
	require.NoError(t, err)
	require.NotEmpty(t, created.Config.ID)
	require.False(t, created.Config.Enabled)
	require.Empty(t, created.Config.AllowedTools)
	require.Equal(t, []string{"API_KEY"}, created.EnvKeys)
	input.ID, input.Name, input.Command, input.Credentials = created.Config.ID, "Renamed", "new-command", nil
	updated, err := service.SaveConnection(input)
	require.NoError(t, err)
	require.Equal(t, created.Config.ID, updated.Config.ID)
	require.Equal(t, []string{"API_KEY"}, updated.EnvKeys)
	require.NoError(t, service.SetConnectionEnabled(created.Config.ID, true))
	require.NoError(t, service.SetToolAllowed(created.Config.ID, "echo", true))
	encoded, err := json.Marshal(service.ListConnections())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-env")
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "private-env", loaded.Credentials[created.Config.ID].Env["API_KEY"])
	other, _ := newTestService(t, store)
	views := other.ListConnections()
	require.Len(t, views, 1)
	require.Equal(t, created.Config.ID, views[0].Config.ID)
	require.True(t, views[0].Config.Enabled)
	require.True(t, views[0].Config.AllowedTools["echo"])
	require.False(t, views[0].Status.Connected)
	views[0].Config.AllowedTools["injected"] = true
	require.False(t, other.ListConnections()[0].Config.AllowedTools["injected"])
	views[0].Config.Args[0] = "changed"
	require.Equal(t, "--stdio", other.ListConnections()[0].Config.Args[0])
	settings := other.LocalSettings()
	settings.AvailableTools[0] = "edit"
	require.Equal(t, []string{"get_chapter_list"}, other.LocalSettings().AvailableTools)
	input.Credentials = &mcpconfig.Secrets{}
	_, err = other.SaveConnection(input)
	require.NoError(t, err)
	require.Empty(t, other.ListConnections()[0].EnvKeys)
	require.NoError(t, other.DeleteConnection(created.Config.ID))
	loaded, err = store.Load()
	require.NoError(t, err)
	require.Empty(t, loaded.Connections)
	require.Empty(t, loaded.Credentials)
}

type testBearerTransport struct{ token string }

func (b testBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(copy)
}

func localClient(ctx context.Context, endpoint, token string) (*mcpclient.Client, error) {
	return mcpclient.Connect(ctx, mcpclient.Config{
		TransportSettings: mcpconfig.TransportSettings{Transport: mcpconfig.TransportStreamableHTTP, Endpoint: endpoint},
		HTTPClient:        &http.Client{Transport: testBearerTransport{token: token}},
	})
}

func TestLocalLifecyclePersistenceRotationAndRevocation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
	service, _ := newTestService(t, store)
	require.NoError(t, service.Restore())
	require.False(t, service.LocalSettings().Status.Running)
	result, err := service.SaveLocal(ctx, mcpconfig.Local{Enabled: true, AllowedTools: []string{"get_chapter_list"}})
	require.NoError(t, err)
	require.Len(t, result.Token, 64)
	firstToken := result.Token
	viewJSON, err := json.Marshal(result)
	require.NoError(t, err)
	var viewFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(viewJSON, &viewFields))
	require.Contains(t, viewFields, "config")
	require.NotContains(t, viewFields, "enabled")
	require.NotContains(t, viewFields, "has_credential")
	result.Config.AllowedTools[0] = "edit"
	require.Equal(t, []string{"get_chapter_list"}, service.LocalSettings().Config.AllowedTools)
	client, err := localClient(ctx, result.Status.URL, result.Token)
	require.NoError(t, err)
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	current, err := client.CallTool(ctx, "get_current_novel", nil)
	require.NoError(t, err)
	require.Equal(t, float64(7), current.StructuredContent.(map[string]any)["novel_id"])
	client.Close()
	encoded, err := json.Marshal(service.LocalSettings())
	require.NoError(t, err)
	require.Contains(t, string(encoded), firstToken)
	require.Equal(t, firstToken, service.LocalSettings().Token)
	firstURL := result.Status.URL
	require.NoError(t, service.Close(ctx))

	restarted, _ := newTestService(t, store)
	require.NoError(t, restarted.Restore())
	require.Equal(t, firstToken, restarted.LocalSettings().Token)
	require.Equal(t, firstURL, restarted.LocalSettings().Status.URL)
	client, err = localClient(ctx, restarted.LocalSettings().Status.URL, firstToken)
	require.NoError(t, err, "重启后持久令牌应有效")
	client.Close()
	rotated, err := restarted.RotateCredential(ctx)
	require.NoError(t, err)
	require.NotEqual(t, firstToken, rotated.Token)
	require.Equal(t, firstURL, rotated.Status.URL)
	old, err := localClient(ctx, rotated.Status.URL, firstToken)
	if old != nil {
		old.Close()
	}
	require.Error(t, err, "旧令牌不能访问轮换后的服务")
	client, err = localClient(ctx, rotated.Status.URL, rotated.Token)
	require.NoError(t, err)
	client.Close()
	result, err = restarted.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
	require.NoError(t, err)
	require.Equal(t, rotated.Token, result.Token, "重复保存应返回当前令牌")
	require.Equal(t, result, restarted.LocalSettings())
	require.Equal(t, firstURL, result.Status.URL)
	client, err = localClient(ctx, result.Status.URL, rotated.Token)
	require.NoError(t, err)
	tools, err = client.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	blocked, err := client.CallTool(ctx, "get_chapter_list", nil)
	require.True(t, err != nil || blocked.IsError)
	client.Close()
	require.NoError(t, restarted.RevokeCredential(ctx))
	require.False(t, restarted.LocalSettings().Config.Enabled)
	require.False(t, restarted.LocalSettings().Status.Running)
	require.Empty(t, restarted.LocalSettings().Token)
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, loaded.LocalToken)
	require.False(t, loaded.Local.Enabled)
}

type failingRepository struct {
	Repository
	fail bool
}

func (r *failingRepository) Save(state mcpconfig.State) error {
	if r.fail {
		return errors.New("模拟磁盘写入失败")
	}
	return r.Repository.Save(state)
}

func TestFailedSavePreservesRuntimeAndRejectsUnsafeConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store := &failingRepository{Repository: mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))}
	service, _ := newTestService(t, store)
	created, err := service.SaveConnection(ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{Name: "server", TransportSettings: mcpconfig.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.NoError(t, err)
	local, err := service.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
	require.NoError(t, err)
	store.fail = true
	failedRotation, err := service.RotateCredential(ctx)
	require.Error(t, err)
	require.Equal(t, local.Token, failedRotation.Token, "保存失败不能返回未保存的新令牌")
	require.Error(t, service.DeleteConnection(created.Config.ID))
	require.Len(t, service.ListConnections(), 1)
	_, err = service.SaveLocal(ctx, mcpconfig.Local{})
	require.Error(t, err)
	require.Equal(t, local, service.LocalSettings())
	client, err := localClient(ctx, local.Status.URL, local.Token)
	require.NoError(t, err)
	client.Close()
	store.fail = false
	beforeConnections := service.ListConnections()
	beforeState, err := store.Load()
	require.NoError(t, err)
	_, err = service.SaveLocal(ctx, mcpconfig.Local{Enabled: true, AllowedTools: []string{"edit"}})
	require.Error(t, err)
	_, err = service.SaveLocal(ctx, mcpconfig.Local{Port: -1})
	require.Error(t, err)
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{ID: created.Config.ID, Name: "bad", TransportSettings: mcpconfig.TransportSettings{Transport: "stdio"}}})
	require.Error(t, err)
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{Name: "bad", TransportSettings: mcpconfig.TransportSettings{Transport: "streamable_http", Endpoint: "https://user:private@example.com/mcp"}}})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{Name: "bad", TransportSettings: mcpconfig.TransportSettings{Transport: "streamable_http", Endpoint: "https://example.com/mcp?token=private"}}})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
	require.Equal(t, beforeConnections, service.ListConnections(), "静态校验失败不能发布新连接配置")
	require.Equal(t, local, service.LocalSettings(), "静态校验失败不能停止当前本地服务")
	afterState, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, beforeState, afterState, "非法输入不能改变持久化快照")
}

func TestSavedTokenSurvivesListenFailure(t *testing.T) {
	for _, rotate := range []bool{false, true} {
		t.Run(fmt.Sprintf("rotate=%t", rotate), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
			service, _ := newTestService(t, store)
			port := service.LocalSettings().Config.Port
			previousToken := ""
			if rotate {
				result, err := service.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
				require.NoError(t, err)
				previousToken = result.Token
				require.NoError(t, service.server.Stop(ctx))
			}
			occupied, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			require.NoError(t, err)
			defer occupied.Close()
			var result LocalView
			if rotate {
				result, err = service.RotateCredential(ctx)
			} else {
				result, err = service.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
			}
			require.ErrorContains(t, err, "端口")
			require.False(t, result.Status.Running)
			require.Empty(t, result.Status.URL, "端口占用不能回退到随机地址")
			require.Len(t, result.Token, 64)
			require.NotEqual(t, previousToken, result.Token)
			token := result.Token
			require.Equal(t, token, service.LocalSettings().Token)
			stored, err := store.Load()
			require.NoError(t, err)
			require.Equal(t, token, stored.LocalToken)
			require.Equal(t, port, stored.Local.Port)
			require.NoError(t, occupied.Close())
			result, err = service.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
			require.NoError(t, err)
			require.Equal(t, token, result.Token)
			require.Equal(t, fmt.Sprintf("http://127.0.0.1:%d/mcp", port), result.Status.URL)
			client, err := localClient(ctx, result.Status.URL, token)
			require.NoError(t, err)
			require.NoError(t, client.Close())
		})
	}
}

func TestSavedTokenSurvivesStopTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mcpserver.New(mcp_tools.NewRegistry(logger), nil, func(ctx context.Context) (mcpserver.CurrentNovel, error) {
		close(started)
		<-ctx.Done()
		<-release
		return mcpserver.CurrentNovel{}, ctx.Err()
	}, logger, nil)
	store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
	service, err := New(store, mcpclient.NewManager(), server)
	require.NoError(t, err)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, service.Close(closeCtx))
	})
	initial, err := service.SaveLocal(ctx, mcpconfig.Local{Enabled: true, Port: freeLocalPort(t)})
	require.NoError(t, err)
	client, err := localClient(ctx, initial.Status.URL, initial.Token)
	require.NoError(t, err)
	defer client.Close()
	done := make(chan struct{})
	go func() {
		_, _ = client.CallTool(ctx, "get_current_novel", nil)
		close(done)
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("工具调用未开始")
	}
	shortCtx, cancelShort := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancelShort()
	rotated, err := service.RotateCredential(shortCtx)
	require.Error(t, err)
	require.NotEmpty(t, rotated.Token)
	require.NotEqual(t, initial.Token, rotated.Token)
	require.Equal(t, rotated.Token, service.LocalSettings().Token)
	stored, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, rotated.Token, stored.LocalToken)
	close(release)
	require.NoError(t, server.Stop(ctx))
	recovered, err := service.SaveLocal(ctx, mcpconfig.Local{Enabled: true})
	require.NoError(t, err)
	require.Equal(t, rotated.Token, recovered.Token)
	require.Equal(t, initial.Status.URL, recovered.Status.URL)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("旧调用未退出")
	}
}

func TestLocalPortDefaultsAndValidation(t *testing.T) {
	store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mcpserver.New(mcp_tools.NewRegistry(logger), nil, nil, logger, nil)
	service, err := New(store, mcpclient.NewManager(), server)
	require.NoError(t, err)
	require.Equal(t, mcpconfig.DefaultPort, service.LocalSettings().Config.Port)
	require.Equal(t, 5108, service.LocalSettings().Config.Port)
	defer service.Close(context.Background())
	for _, port := range []int{-1, 65536} {
		_, err := service.SaveLocal(context.Background(), mcpconfig.Local{Port: port})
		require.Error(t, err)
	}
	result, err := service.SaveLocal(context.Background(), mcpconfig.Local{Port: 23456})
	require.NoError(t, err)
	require.Equal(t, 23456, result.Config.Port)
	result, err = service.SaveLocal(context.Background(), mcpconfig.Local{})
	require.NoError(t, err)
	require.Equal(t, 23456, result.Config.Port)
	stored, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, 23456, stored.Local.Port)
}

func TestLoadFailurePreservesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_config.enc")
	store := mcpconfig.NewStore(path)
	plain, err := json.Marshal(mcpconfig.State{
		Local: mcpconfig.Local{Enabled: true}, LocalToken: strings.Repeat("x", 64),
	})
	require.NoError(t, err)
	corrupted, err := config.Encrypt(plain)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, corrupted, 0600))
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mcpserver.New(mcp_tools.NewRegistry(logger), nil, nil, logger, nil)
	service, err := New(store, mcpclient.NewManager(), server)
	require.Error(t, err)
	require.NotNil(t, service)
	defer service.Close(context.Background())
	require.Error(t, service.Restore())
	settings := service.LocalSettings()
	require.False(t, settings.Config.Enabled)
	require.NotEmpty(t, settings.LastError)
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{Name: "test", TransportSettings: mcpconfig.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.Error(t, err)
	require.Error(t, service.RevokeCredential(context.Background()))
	remaining, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, remaining)
}

func TestRuntimePolicyValidationOnLoad(t *testing.T) {
	store := mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_config.enc"))
	require.NoError(t, store.Save(mcpconfig.State{Local: mcpconfig.Local{AllowedTools: []string{"edit"}}}), "Store 不负责工具开放策略")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mcpserver.New(mcp_tools.NewRegistry(logger), nil, nil, logger, nil)
	service, err := New(store, mcpclient.NewManager(), server)
	require.Error(t, err)
	require.False(t, service.LocalSettings().Status.Running)
	require.NoError(t, service.Close(context.Background()))
}
