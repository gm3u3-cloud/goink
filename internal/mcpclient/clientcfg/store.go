package clientcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/utils"
)

// Store 将 MCP 配置与凭据作为一个加密快照提交，避免跨文件更新留下半份授权。
// 与 LLM 配置共用应用固定密钥；调用方负责串行化 Load/Save。
type Store struct {
	path string
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		state := State{}
		state.normalize()
		return state, state.Validate()
	}
	if err != nil {
		return State{}, fmt.Errorf("Не удалось прочитать конфигурацию MCP: %w", err)
	}
	plain, err := config.Decrypt(data)
	if err != nil {
		return State{}, fmt.Errorf("Не удалось расшифровать конфигурацию MCP: %w", err)
	}
	var state State
	if err := json.Unmarshal(plain, &state); err != nil {
		return State{}, fmt.Errorf("Не удалось разобрать конфигурацию MCP: %w", err)
	}
	state.normalize()
	if err := state.Validate(); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Store) Save(state State) error {
	state.normalize()
	if err := state.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("Не удалось создать каталог конфигурации MCP: %w", err)
	}
	plain, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("Не удалось закодировать конфигурацию MCP: %w", err)
	}
	data, err := config.Encrypt(plain)
	if err != nil {
		return fmt.Errorf("Не удалось зашифровать конфигурацию MCP: %w", err)
	}
	if err := utils.AtomicWrite(s.path, data); err != nil {
		return fmt.Errorf("Не удалось сохранить конфигурацию MCP: %w", err)
	}
	return nil
}
