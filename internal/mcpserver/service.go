package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/sigpanic/goink/internal/mcpserver/servercfg"
)

type Repository interface {
	Load() (servercfg.State, error)
	Save(servercfg.State) error
}

type LocalView struct {
	Config         servercfg.Local `json:"config"`
	AvailableTools []string        `json:"available_tools"`
	Token          string          `json:"token"`
	Status         Status          `json:"status"`
	LastError      string          `json:"last_error"`
}

var localTools = []string{"get_chapter_list"}

// Service 编排本地 Server 配置、令牌与运行状态。
type Service struct {
	mu       sync.Mutex
	store    Repository
	state    servercfg.State
	server   *Server
	loadErr  error
	localErr string
	closed   bool
}

// New 加载失败时仍返回停用的 Service，供应用展示错误并统一关闭；异常配置不会被覆盖。
func NewService(store Repository, server *Server) (*Service, error) {
	s := &Service{
		store: store, server: server,
		state: servercfg.State{Local: servercfg.Local{Port: servercfg.DefaultPort}},
	}
	state, err := store.Load()
	if err == nil {
		err = checkLocalTools(state.Local.AllowedTools)
	}
	if err != nil {
		s.loadErr, s.localErr = err, err.Error()
		return s, err
	}
	s.state = state
	return s, nil
}

// Restore 只在应用核心模块初始化完成后调用。
func (s *Service) Restore() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("MCP 管理服务已关闭")
	}
	if s.loadErr != nil {
		return s.loadErr
	}
	return s.startLocal()
}

func (s *Service) LocalSettings() LocalView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.localView()
}

// SaveLocal 返回当前已保存的令牌；启停失败后也可通过 LocalSettings 再次读取。
func (s *Service) SaveLocal(ctx context.Context, config servercfg.Local) (LocalView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if config.Port == 0 {
		config.Port = s.state.Local.Port
	}
	if err := checkLocalTools(config.AllowedTools); err != nil {
		return s.localView(), err
	}
	next := cloneState(s.state)
	next.Local = config
	if config.Enabled && next.LocalToken == "" {
		var err error
		next.LocalToken, err = newToken()
		if err != nil {
			return s.localView(), err
		}
	}
	return s.applyLocal(ctx, next)
}

func (s *Service) RotateCredential(ctx context.Context) (LocalView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := newToken()
	if err != nil {
		return s.localView(), err
	}
	next := cloneState(s.state)
	next.LocalToken = token
	return s.applyLocal(ctx, next)
}

// RevokeCredential 同时持久化关闭状态，重启应用也不能使旧凭据重新生效。
func (s *Service) RevokeCredential(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	next.LocalToken = ""
	next.Local.Enabled = false
	_, err := s.applyLocal(ctx, next)
	return err
}

func (s *Service) applyLocal(ctx context.Context, next servercfg.State) (LocalView, error) {
	// 先保存期望状态，失败不动监听器；成功后关闭旧服务以撤销旧白名单和令牌。
	// 若关停或重启失败，查询同时返回期望开关、真实运行状态及错误，可重试同一设置。
	if err := s.save(next); err != nil {
		return s.localView(), err
	}
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.server.Stop(stopCtx); err != nil {
		s.localErr = "MCP 服务仍在关闭，请稍后重试"
		return s.localView(), fmt.Errorf("%s: %w", s.localErr, err)
	}
	if err := s.startLocal(); err != nil {
		return s.localView(), err
	}
	return s.localView(), nil
}

func (s *Service) startLocal() error {
	s.localErr = ""
	if err := s.server.SetPort(s.state.Local.Port); err != nil {
		s.localErr = "无法设置 MCP 端口，请先停止服务后重试"
		return fmt.Errorf("%s: %w", s.localErr, err)
	}
	if err := s.server.SetAllowedTools(s.state.Local.AllowedTools); err != nil {
		s.localErr = "无法设置 MCP 对外工具，请先停止服务后重试"
		return fmt.Errorf("%s: %w", s.localErr, err)
	}
	if s.state.Local.Enabled {
		if _, err := s.server.Start(s.state.LocalToken); err != nil {
			s.localErr = "启动 MCP 服务失败，请检查端口是否被占用或本机监听权限后重试"
			return fmt.Errorf("%s: %w", s.localErr, err)
		}
	}
	return nil
}

func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.server.Stop(ctx)
}

func (s *Service) save(next servercfg.State) error {
	if s.closed {
		return errors.New("MCP 管理服务已关闭")
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

func (s *Service) localView() LocalView {
	config := s.state.Local
	config.AllowedTools = append([]string{}, config.AllowedTools...)
	return LocalView{
		Config: config, AvailableTools: slices.Clone(localTools),
		Token: s.state.LocalToken, Status: s.server.Status(), LastError: s.localErr,
	}
}

func checkLocalTools(names []string) error {
	for _, name := range names {
		if !slices.Contains(localTools, name) {
			return errors.New("选择了当前不允许对外开放的 MCP 工具；get_current_novel 始终内置")
		}
	}
	return nil
}

func newToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("生成 MCP 凭据失败: %w", err)
	}
	return hex.EncodeToString(secret), nil
}

func cloneState(state servercfg.State) servercfg.State {
	data, _ := json.Marshal(state)
	var result servercfg.State
	_ = json.Unmarshal(data, &result)
	return result
}
