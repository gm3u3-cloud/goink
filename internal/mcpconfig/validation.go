package mcpconfig

import (
	"encoding/hex"
	"errors"
)

// Transport 表示外部 MCP server 的连接方式。
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportStreamableHTTP Transport = "streamable_http"
	DefaultPort                       = 5108
)

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
