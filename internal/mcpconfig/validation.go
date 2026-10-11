package mcpconfig

import (
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
)

// Transport 表示外部 MCP server 的连接方式。
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportStreamableHTTP Transport = "streamable_http"
	DefaultPort                       = 5108
)

func (c Connection) Validate(secret Secrets) error {
	if c.ID == "" {
		return errors.New("MCP 连接 ID 不能为空")
	}
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 {
		return errors.New("MCP server 名称不能为空或超过 200 字节")
	}
	return c.ValidateTransport(secret)
}

// ValidateTransport 校验传输参数和凭据，不要求持久化连接的 ID、名称。
// Client 可复用此规则检查直接传入的运行参数。
func (c ConnectionSettings) ValidateTransport(secret Secrets) error {
	for key, value := range secret.Env {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return errors.New("MCP 环境变量格式无效")
		}
	}
	if strings.ContainsAny(secret.BearerToken, "\r\n\x00") {
		return errors.New("MCP 认证令牌格式无效")
	}
	switch c.Transport {
	case TransportStdio:
		if strings.TrimSpace(c.Command) == "" || strings.ContainsRune(c.Command, '\x00') || c.Endpoint != "" {
			return errors.New("stdio MCP 连接需要命令，不能包含 HTTP 地址")
		}
		for _, arg := range c.Args {
			if strings.ContainsRune(arg, '\x00') {
				return errors.New("MCP 命令参数格式无效")
			}
		}
		if secret.BearerToken != "" {
			return errors.New("stdio 连接不能包含 HTTP 令牌")
		}
	case TransportStreamableHTTP:
		endpoint, err := url.Parse(c.Endpoint)
		if err != nil || endpoint.Hostname() == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return errors.New("HTTP MCP 连接需要有效的 http(s) 地址")
		}
		if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return errors.New("HTTP MCP 连接需要有效的 http(s) 地址；认证信息请使用独立凭据字段，不要放入 URL")
		}
		if c.Command != "" || len(c.Args) > 0 || len(secret.Env) > 0 {
			return errors.New("HTTP MCP 连接不能包含 stdio 命令或环境变量")
		}
	default:
		return errors.New("不支持的 MCP 传输方式")
	}
	return nil
}

func (s *State) normalize() {
	if s.Credentials == nil {
		s.Credentials = map[string]Secrets{}
	}
	if s.Local.Port == 0 {
		s.Local.Port = DefaultPort
	}
}

func (s State) Validate() error {
	seen := make(map[string]bool, len(s.Connections))
	for _, c := range s.Connections {
		if err := c.Validate(s.Credentials[c.ID]); err != nil {
			return err
		}
		if seen[c.ID] {
			return errors.New("MCP 连接 ID 重复")
		}
		seen[c.ID] = true
	}
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

func (c Local) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("MCP 端口必须在 1 到 65535 之间")
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
