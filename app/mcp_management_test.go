package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/mcpclient"
	"github.com/sigpanic/goink/internal/mcpconfig"
	"github.com/sigpanic/goink/internal/mcpmanagement"
	"github.com/sigpanic/goink/internal/mcpserver"
	"github.com/sigpanic/goink/internal/testsupport"
	"github.com/stretchr/testify/require"
)

func TestMCPManagementAPIAndShutdown(t *testing.T) {
	a := setupTestApp(t)
	a.mcpServer = mcpserver.New(a.registry, a.db, a.currentNovel, a.logger, nil)
	service, err := mcpmanagement.New(mcpconfig.NewStore(filepath.Join(t.TempDir(), "mcp_server_config.enc")), mcpclient.NewManager(), a.mcpServer)
	require.NoError(t, err)
	a.mcpManagement.Store(service)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, service.Close(ctx))
	})
	settings, err := a.GetMCPServerSettings()
	require.NoError(t, err)
	require.False(t, settings.Config.Enabled)
	require.False(t, settings.Status.Running)
	connection, err := a.SaveMCPConnection(mcpmanagement.ConnectionInput{
		ConnectionSettings: mcpconfig.ConnectionSettings{Name: "test", TransportSettings: mcpconfig.TransportSettings{Transport: "streamable_http", Endpoint: "https://example.com/mcp"}},
		Credentials:        &mcpconfig.Secrets{BearerToken: "external-private-token"},
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	local, err := a.SaveMCPServerSettings(mcpconfig.Local{Enabled: true, Port: port})
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
	require.NoError(t, a.DeleteMCPConnection(connection.Config.ID))
	a.OnShutdown(context.Background())
	require.False(t, a.mcpServer.Status().Running)
	sqlDB, err := a.db.DB()
	require.NoError(t, err)
	require.Error(t, sqlDB.Ping())
	_, err = a.SaveMCPConnection(mcpmanagement.ConnectionInput{ConnectionSettings: mcpconfig.ConnectionSettings{Name: "closed", TransportSettings: mcpconfig.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.Error(t, err)
}

func TestMCPAPIBeforeInitialization(t *testing.T) {
	a := &App{}
	_, err := a.ListMCPConnections()
	require.ErrorContains(t, err, "未初始化")
	_, err = a.GetMCPServerSettings()
	require.ErrorContains(t, err, "未初始化")
}

func TestMCPConfigFailureDoesNotBlockStartup(t *testing.T) {
	for _, mode := range []string{"unreadable", "corrupted", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			dataDir := testsupport.Isolate(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			path := config.MCPServerConfigPath()
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
			var original []byte
			switch mode {
			case "unreadable":
				require.NoError(t, os.Mkdir(path, 0700))
			case "corrupted":
				original = []byte("broken MCP ciphertext")
				require.NoError(t, os.WriteFile(path, original, 0600))
			case "invalid":
				plain, err := json.Marshal(mcpconfig.State{Local: mcpconfig.Local{Port: -1}})
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
			require.NotEmpty(t, settings.LastError)
			_, err = a.SaveMCPServerSettings(mcpconfig.Local{Enabled: true})
			require.Error(t, err)
			_, err = a.RotateMCPServerCredential()
			require.Error(t, err)
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
