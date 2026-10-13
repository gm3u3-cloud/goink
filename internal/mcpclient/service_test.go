package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/mcpclient/clientcfg"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, store Repository) (*Service, *Manager) {
	t.Helper()
	manager := NewManager()
	service, err := NewService(store, manager)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, service.Close(ctx))
	})
	return service, manager
}

func TestConnectionPersistenceStableIDAndSecrets(t *testing.T) {
	store := clientcfg.NewStore(filepath.Join(t.TempDir(), "mcp_client_config.enc"))
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
	input.Credentials = &clientcfg.Secrets{}
	_, err = other.SaveConnection(input)
	require.NoError(t, err)
	require.Empty(t, other.ListConnections()[0].EnvKeys)
	require.NoError(t, other.DeleteConnection(created.Config.ID))
	loaded, err = store.Load()
	require.NoError(t, err)
	require.Empty(t, loaded.Connections)
	require.Empty(t, loaded.Credentials)
}

type failingRepository struct {
	Repository
	fail bool
}

func (r *failingRepository) Save(state clientcfg.State) error {
	if r.fail {
		return errors.New("模拟磁盘写入失败")
	}
	return r.Repository.Save(state)
}

func TestFailedSavePreservesRuntimeAndRejectsUnsafeConfig(t *testing.T) {
	store := &failingRepository{Repository: clientcfg.NewStore(filepath.Join(t.TempDir(), "mcp_client_config.enc"))}
	service, _ := newTestService(t, store)
	created, err := service.SaveConnection(ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "server", TransportSettings: clientcfg.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.NoError(t, err)
	store.fail = true
	require.Error(t, service.DeleteConnection(created.Config.ID))
	require.Len(t, service.ListConnections(), 1)
	store.fail = false
	beforeConnections := service.ListConnections()
	beforeState, err := store.Load()
	require.NoError(t, err)
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{ID: created.Config.ID, Name: "bad", TransportSettings: clientcfg.TransportSettings{Transport: "stdio"}}})
	require.Error(t, err)
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "bad", TransportSettings: clientcfg.TransportSettings{Transport: "streamable_http", Endpoint: "https://user:private@example.com/mcp"}}})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "bad", TransportSettings: clientcfg.TransportSettings{Transport: "streamable_http", Endpoint: "https://example.com/mcp?token=private"}}})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
	require.Equal(t, beforeConnections, service.ListConnections(), "静态校验失败不能发布新连接配置")
	afterState, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, beforeState, afterState, "非法输入不能改变持久化快照")
}

func TestLoadFailurePreservesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_client_config.enc")
	store := clientcfg.NewStore(path)
	plain, err := json.Marshal(clientcfg.State{
		Connections: []clientcfg.Connection{{ConnectionSettings: clientcfg.ConnectionSettings{ID: "invalid", Name: "server"}}},
	})
	require.NoError(t, err)
	corrupted, err := config.Encrypt(plain)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, corrupted, 0600))
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	service, err := NewService(store, NewManager())
	require.Error(t, err)
	require.NotNil(t, service)
	defer service.Close(context.Background())
	_, err = service.SaveConnection(ConnectionInput{ConnectionSettings: clientcfg.ConnectionSettings{Name: "test", TransportSettings: clientcfg.TransportSettings{Transport: "stdio", Command: "test"}}})
	require.Error(t, err)
	remaining, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, remaining)
}
