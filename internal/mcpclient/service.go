package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sigpanic/goink/internal/mcpclient/clientcfg"
)

type Repository interface {
	Load() (clientcfg.State, error)
	Save(clientcfg.State) error
}

type ConnectionInput struct {
	clientcfg.ConnectionSettings
	// nil 保留现有凭据；非 nil 整体替换，空对象显式清除凭据。
	Credentials *clientcfg.Secrets `json:"credentials"`
}

type ConnectionView struct {
	Config         clientcfg.Connection `json:"config"`
	EnvKeys        []string             `json:"env_keys"`
	HasBearerToken bool                 `json:"has_bearer_token"`
	Status         ConnectionStatus     `json:"status"`
}

type ToolView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Allowed     bool   `json:"allowed"`
}

// Service 编排持久授权和运行时连接；磁盘更新失败不会发布新外部配置。
// 网络请求在 client Manager 中执行，不占用配置锁或数据库事务。
type Service struct {
	mu      sync.Mutex
	store   Repository
	state   clientcfg.State
	client  *Manager
	loadErr error
	closed  bool
}

// New 加载失败时仍返回停用的 Service，供应用展示错误并统一关闭；异常配置不会被覆盖。
func NewService(store Repository, client *Manager) (*Service, error) {
	s := &Service{
		store: store, client: client,
		state: clientcfg.State{Credentials: map[string]clientcfg.Secrets{}},
	}
	state, err := store.Load()
	if err != nil {
		s.loadErr = err
		return s, err
	}
	s.state = state
	for _, c := range state.Connections {
		client.Configure(c, state.Credentials[c.ID])
	}
	return s, nil
}

func (s *Service) ListConnections() []ConnectionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	views := make([]ConnectionView, 0, len(s.state.Connections))
	for _, c := range s.state.Connections {
		views = append(views, s.connectionView(c))
	}
	return views
}

func (s *Service) SaveConnection(input ConnectionInput) (ConnectionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ConnectionView{}, errors.New("Служба управления MCP остановлена")
	}
	next := cloneState(s.state)
	index := -1
	if input.ID == "" {
		input.ID = uuid.NewString()
	} else {
		index = connectionIndex(next, input.ID)
		if index < 0 {
			return ConnectionView{}, errors.New("MCP-соединение не найдено")
		}
	}
	settings := input.ConnectionSettings
	settings.Name = strings.TrimSpace(settings.Name)
	settings.Args = slices.Clone(settings.Args)
	connection := clientcfg.Connection{ConnectionSettings: settings, AllowedTools: map[string]bool{}}
	if index >= 0 {
		connection.AllowedTools = next.Connections[index].AllowedTools
	}
	if input.Credentials != nil {
		next.Credentials[input.ID] = *input.Credentials
	}
	if index < 0 {
		next.Connections = append(next.Connections, connection)
	} else {
		next.Connections[index] = connection
	}
	if err := s.save(next); err != nil {
		return ConnectionView{}, err
	}
	s.configure(input.ID)
	return s.connectionView(connection), nil
}

func (s *Service) DeleteConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	index := connectionIndex(next, id)
	if index < 0 {
		return errors.New("MCP-соединение не найдено")
	}
	next.Connections = append(next.Connections[:index], next.Connections[index+1:]...)
	delete(next.Credentials, id)
	if err := s.save(next); err != nil {
		return err
	}
	s.client.Remove(id)
	return nil
}

func (s *Service) SetConnectionEnabled(id string, enabled bool) error {
	return s.updateConnection(id, func(c *clientcfg.Connection) { c.Enabled = enabled })
}

func (s *Service) SetToolAllowed(id, tool string, allowed bool) error {
	if strings.TrimSpace(tool) == "" {
		return errors.New("Имя инструмента не может быть пустым")
	}
	return s.updateConnection(id, func(c *clientcfg.Connection) {
		if c.AllowedTools == nil {
			c.AllowedTools = map[string]bool{}
		}
		if allowed {
			c.AllowedTools[tool] = true
		} else {
			delete(c.AllowedTools, tool)
		}
	})
}

func (s *Service) updateConnection(id string, update func(*clientcfg.Connection)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	index := connectionIndex(next, id)
	if index < 0 {
		return errors.New("MCP-соединение не найдено")
	}
	update(&next.Connections[index])
	if err := s.save(next); err != nil {
		return err
	}
	s.client.SetAccess(s.state.Connections[index])
	return nil
}

func (s *Service) Refresh(ctx context.Context, id string) ([]ToolView, error) {
	if _, err := s.client.Refresh(ctx, id); err != nil {
		return nil, err
	}
	return s.Tools(id)
}

func (s *Service) Tools(id string) ([]ToolView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := connectionIndex(s.state, id)
	if index < 0 {
		return nil, errors.New("MCP-соединение не найдено")
	}
	views := []ToolView{}
	for _, tool := range s.client.Tools(id) {
		views = append(views, ToolView{Name: tool.Name, Description: tool.Description, Allowed: s.state.Connections[index].AllowedTools[tool.Name]})
	}
	return views, nil
}

func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.client.Close(ctx)
}

func (s *Service) save(next clientcfg.State) error {
	if s.closed {
		return errors.New("Служба управления MCP остановлена")
	}
	if s.loadErr != nil {
		return s.loadErr
	}
	next = cloneState(next)
	if err := s.store.Save(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Service) configure(id string) {
	index := connectionIndex(s.state, id)
	s.client.Configure(s.state.Connections[index], s.state.Credentials[id])
}

func (s *Service) connectionView(c clientcfg.Connection) ConnectionView {
	secret := s.state.Credentials[c.ID]
	keys := make([]string, 0, len(secret.Env))
	for key := range secret.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	allowed := map[string]bool{}
	for name, value := range c.AllowedTools {
		allowed[name] = value
	}
	c.Args = append([]string{}, c.Args...)
	c.AllowedTools = allowed
	return ConnectionView{
		Config: c, EnvKeys: keys, HasBearerToken: secret.BearerToken != "",
		Status: s.client.Status(c.ID),
	}
}

func connectionIndex(state clientcfg.State, id string) int {
	for i, c := range state.Connections {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func cloneState(state clientcfg.State) clientcfg.State {
	data, _ := json.Marshal(state)
	var result clientcfg.State
	_ = json.Unmarshal(data, &result)
	return result
}
