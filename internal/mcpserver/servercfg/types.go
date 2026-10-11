package servercfg

import (
	"encoding/hex"
	"errors"
)

const DefaultPort = 5108

type Local struct {
	Enabled      bool     `json:"enabled"`
	Port         int      `json:"port"`
	AllowedTools []string `json:"allowed_tools"`
}

func (c Local) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("MCP 端口必须在 1 到 65535 之间")
	}
	return nil
}

type State struct {
	Local      Local  `json:"local"`
	LocalToken string `json:"local_token"`
}

func (s *State) normalize() {
	if s.Local.Port == 0 {
		s.Local.Port = DefaultPort
	}
}

func (s State) Validate() error {
	if err := s.Local.Validate(); err != nil {
		return err
	}
	if s.Local.Enabled && s.LocalToken == "" {
		return errors.New("已启用的 MCP 服务缺少凭据")
	}
	if s.LocalToken != "" {
		return ValidateToken(s.LocalToken)
	}
	return nil
}

func ValidateToken(token string) error {
	secret, err := hex.DecodeString(token)
	if err != nil || len(secret) != 32 {
		return errors.New("MCP 访问令牌无效")
	}
	return nil
}
