package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/mcpclient"
	"github.com/sigpanic/goink/internal/mcpclient/clientcfg"
	"github.com/sigpanic/goink/internal/mcpserver"
	"github.com/sigpanic/goink/internal/mcpserver/servercfg"
	"github.com/sigpanic/goink/internal/testsupport"
	"github.com/stretchr/testify/require"
)

func TestMCPManagementAPIAndShutdown(t *testing.T) {
	a := setupTestApp(t)
	server := mcpserver.New(a.registry, a.db, a.currentNovel, a.logger)
	dir := t.TempDir()
	serverStore := servercfg.NewStore(filepath.Join(dir, "mcp_server_config.enc"))
	clientStore := clientcfg.NewStore(filepath.Join(dir, "mcp_client_config.enc"))
	service, err := mcpserver.NewService(serverStore, server)
	require.NoError(t, err)
	a.mcpServerService.Store(service)
	clientService, err := mcpclient.NewService(clientStore, mcpclient.NewManager())
	require.NoError(t, err)
	a.mcpClientService.Store(clientService)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, service.Close(ctx))
		require.NoError(t, clientService.Close(ctx))
	})
	settings, err := a.GetMCPServerSettings()
	require.NoError(t, err)
	require.False(t, settings.Config.Enabled)
	require.False(t, settings.Status.Running)
	connection, err := a.SaveMCPConnection(mcpclient.ConnectionInput{
		ConnectionSettings: clientcfg.ConnectionSettings{Name: "test", TransportSettings: clientcfg.TransportSettings{Transport: "streamable_http", Endpoint: "https://example.com/mcp"}},
		Credentials:        &clientcfg.Secrets{BearerToken: "external-private-token"},
	})
	require.NoError(t, err)
	require.True(t, connection.HasBearerToken)
	list, err := a.ListMCPConnections()
	require.NoError(t, err)
	encoded, err := json.Marshal(list)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "external-private-token")
	require.NoError(t, a.SetMCPConnectionEnabled(connection.Config.ID, true))
	require.NoError(t, a.SetMCPToolAllowed(connection.Config.ID, "echo", true))
	port := testsupport.FreeLocalPort(t, servercfg.MinPort, servercfg.MaxPort)
	local, err := a.SaveMCPServerSettings(servercfg.Local{Enabled: true, Port: port})
	require.NoError(t, err)
	require.NotEmpty(t, local.Token)
	settings, err = a.GetMCPServerSettings()
	require.NoError(t, err)
	require.Equal(t, local.Token, settings.Token)
	require.Equal(t, port, settings.Config.Port)
	general, err := a.GetSettings()
	require.NoError(t, err)
	encoded, err = json.Marshal(general)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), local.Token)
	require.NotContains(t, string(encoded), "external-private-token")
	rotated, err := a.RotateMCPServerCredential()
	require.NoError(t, err)
	require.NotEqual(t, local.Token, rotated.Token)
	savedClient, err := clientStore.Load()
	require.NoError(t, err)
	require.Len(t, savedClient.Connections, 1, "Server 写入不能覆盖 Client 配置")
	require.Equal(t, connection.Config.ID, savedClient.Connections[0].ID)
	require.True(t, savedClient.Connections[0].Enabled)
	require.True(t, savedClient.Connections[0].AllowedTools["echo"])
	require.Equal(t, "external-private-token", savedClient.Credentials[connection.Config.ID].BearerToken)
	require.NoError(t, a.DeleteMCPConnection(connection.Config.ID))
	savedServer, err := serverStore.Load()
	require.NoError(t, err)
	require.Equal(t, rotated.Token, savedServer.LocalToken, "Client 写入不能覆盖 Server 配置")
	require.Equal(t, port, savedServer.Local.Port)
	a.OnShutdown(context.Background())
	require.False(t, service.LocalSettings().Status.Running)
	sqlDB, err := a.db.DB()
	require.NoError(t, err)
	require.Error(t, sqlDB.Ping())
	_, err = a.SaveMCPConnection(mcpclient.ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "closed", TransportSettings: clientcfg.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.Error(t, err)
}

func TestMCPAPIBeforeInitialization(t *testing.T) {
	a := &App{}
	_, err := a.ListMCPConnections()
	require.ErrorContains(t, err, "не инициализирован")
	_, err = a.GetMCPServerSettings()
	require.ErrorContains(t, err, "не инициализирован")
}

func TestMCPConfigFailureDoesNotBlockStartup(t *testing.T) {
	for _, direction := range []string{"server", "client"} {
		for _, mode := range []string{"unreadable", "corrupted", "invalid"} {
			t.Run(direction+"/"+mode, func(t *testing.T) {
				dataDir := testsupport.Isolate(t)
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("USERPROFILE", home)
				path := config.MCPServerConfigPath()
				if direction == "client" {
					path = config.MCPClientConfigPath()
				}
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
				var original []byte
				switch mode {
				case "unreadable":
					require.NoError(t, os.Mkdir(path, 0700))
				case "corrupted":
					original = []byte("broken MCP ciphertext")
					require.NoError(t, os.WriteFile(path, original, 0600))
				case "invalid":
					var invalid any = servercfg.State{Local: servercfg.Local{Port: -1}}
					if direction == "client" {
						invalid = clientcfg.State{Connections: []clientcfg.Connection{{ConnectionSettings: clientcfg.ConnectionSettings{ID: "bad", Name: "server"}}}}
					}
					plain, err := json.Marshal(invalid)
					require.NoError(t, err)
					ciphertext, err := config.Encrypt(plain)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(path, ciphertext, 0600))
					original, err = os.ReadFile(path)
					require.NoError(t, err)
				}
				a := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
				a.ctx, a.cancel = context.WithCancel(context.Background())
				t.Cleanup(func() { a.OnShutdown(context.Background()) })
				require.NoError(t, a.runInit(&config.AppConfig{DataDir: dataDir}))
				require.Equal(t, PhaseReady, a.GetStartupState().Phase)
				require.True(t, a.IsInitialized())
				_, err := a.GetNovels()
				require.NoError(t, err)
				_, err = a.GetSettings()
				require.NoError(t, err)
				settings, err := a.GetMCPServerSettings()
				require.NoError(t, err)
				require.False(t, settings.Config.Enabled)
				require.False(t, settings.Status.Running)
				_, clientErr := a.SaveMCPConnection(mcpclient.ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "test", TransportSettings: clientcfg.TransportSettings{Transport: "stdio", Command: "test"}}})
				if direction == "server" {
					require.NotEmpty(t, settings.LastError)
					_, err = a.SaveMCPServerSettings(servercfg.Local{Enabled: true})
					require.Error(t, err)
					_, err = a.RotateMCPServerCredential()
					require.Error(t, err)
					require.NoError(t, clientErr, "Server 配置异常不能阻止 Client 配置保存")
				} else {
					require.Error(t, clientErr)
					require.Empty(t, settings.LastError)
					_, err = a.SaveMCPServerSettings(servercfg.Local{})
					require.NoError(t, err, "Client 配置异常不能阻止 Server 配置保存")
				}
				if mode == "unreadable" {
					require.DirExists(t, path)
				} else {
					remaining, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, original, remaining)
				}
				require.Eventually(t, func() bool { return a.searchService.Load() != nil }, time.Second, time.Millisecond)
			})
		}
	}
}
