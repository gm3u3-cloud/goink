package clientcfg

import (
	"errors"
	"net/url"
	"strings"
)

// Transport 表示外部 MCP server 的连接方式。
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportStreamableHTTP Transport = "streamable_http"
)

type TransportSettings struct {
	Transport Transport `json:"transport"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	Endpoint  string    `json:"endpoint"`
}

// Validate 校验建立连接所需的传输参数和环境变量；不检查持久化字段或凭据存储策略。
func (c TransportSettings) Validate(env map[string]string) error {
	for key, value := range env {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return errors.New("Неверный формат переменной окружения MCP")
		}
	}
	switch c.Transport {
	case TransportStdio:
		if strings.TrimSpace(c.Command) == "" || strings.ContainsRune(c.Command, '\x00') || c.Endpoint != "" {
			return errors.New("stdio MCP-соединение требует команду и не может содержать HTTP-адрес")
		}
		for _, arg := range c.Args {
			if strings.ContainsRune(arg, '\x00') {
				return errors.New("Неверный формат аргументов команды MCP")
			}
		}
	case TransportStreamableHTTP:
		endpoint, err := url.Parse(c.Endpoint)
		if err != nil || endpoint.Hostname() == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return errors.New("HTTP MCP-соединение требует валидный http(s)-адрес")
		}
		if c.Command != "" || len(c.Args) > 0 || len(env) > 0 {
			return errors.New("HTTP MCP-соединение не может содержать stdio-команду или переменные окружения")
		}
	default:
		return errors.New("Неподдерживаемый способ передачи MCP")
	}
	return nil
}

type ConnectionSettings struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	TransportSettings
	Enabled bool `json:"enabled"`
}

type Connection struct {
	ConnectionSettings
	AllowedTools map[string]bool `json:"allowed_tools"`
}

func (c Connection) Validate(secret Secrets) error {
	if c.ID == "" {
		return errors.New("ID MCP-соединения не может быть пустым")
	}
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 {
		return errors.New("Имя MCP-сервера не может быть пустым или превышать 200 байт")
	}
	if err := c.TransportSettings.Validate(secret.Env); err != nil {
		return err
	}
	if strings.ContainsAny(secret.BearerToken, "\r\n\x00") {
		return errors.New("Неверный формат токена аутентификации MCP")
	}
	switch c.Transport {
	case TransportStdio:
		if secret.BearerToken != "" {
			return errors.New("stdio-соединение не может содержать HTTP-токен")
		}
	case TransportStreamableHTTP:
		endpoint, err := url.Parse(c.Endpoint)
		if err != nil || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return errors.New("HTTP MCP-соединение требует валидный http(s)-адрес; для данных аутентификации используйте отдельное поле, а не URL")
		}
	}
	return nil
}

// Secrets 不用于设置查询；环境变量也按凭据处理，只返回已配置的变量名。
type Secrets struct {
	Env         map[string]string `json:"env"`
	BearerToken string            `json:"bearer_token"`
}

type State struct {
	Connections []Connection       `json:"connections"`
	Credentials map[string]Secrets `json:"credentials"`
}

func (s *State) normalize() {
	if s.Credentials == nil {
		s.Credentials = map[string]Secrets{}
	}
}

func (s State) Validate() error {
	seen := make(map[string]bool, len(s.Connections))
	for _, c := range s.Connections {
		if err := c.Validate(s.Credentials[c.ID]); err != nil {
			return err
		}
		if seen[c.ID] {
			return errors.New("Дублированный ID MCP-соединения")
		}
		seen[c.ID] = true
	}
	return nil
}
