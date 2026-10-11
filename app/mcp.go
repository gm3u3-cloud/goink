package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sigpanic/goink/internal/mcpclient"
	"github.com/sigpanic/goink/internal/mcpserver"
	"github.com/sigpanic/goink/internal/mcpserver/servercfg"
	"github.com/sigpanic/goink/internal/novel"
	"gorm.io/gorm"
)

func (a *App) serverMCPService() (*mcpserver.Service, error) {
	service := a.mcpServerService.Load()
	if service == nil {
		return nil, fmt.Errorf("MCP 管理服务未初始化")
	}
	return service, nil
}

func (a *App) clientMCPService() (*mcpclient.Service, error) {
	service := a.mcpClientService.Load()
	if service == nil {
		return nil, fmt.Errorf("MCP 管理服务未初始化")
	}
	return service, nil
}

func (a *App) stopMCP(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var serverErr, clientErr error
	if service := a.mcpServerService.Load(); service != nil {
		serverErr = service.Close(stopCtx)
	} else if a.mcpServer != nil {
		serverErr = a.mcpServer.Stop(stopCtx)
	}
	if service := a.mcpClientService.Load(); service != nil {
		clientErr = service.Close(stopCtx)
	}
	return errors.Join(serverErr, clientErr)
}

func (a *App) ListMCPConnections() ([]mcpclient.ConnectionView, error) {
	service, err := a.clientMCPService()
	if err != nil {
		return nil, err
	}
	return service.ListConnections(), nil
}

func (a *App) SaveMCPConnection(input mcpclient.ConnectionInput) (mcpclient.ConnectionView, error) {
	service, err := a.clientMCPService()
	if err != nil {
		return mcpclient.ConnectionView{}, err
	}
	return service.SaveConnection(input)
}

func (a *App) DeleteMCPConnection(id string) error {
	service, err := a.clientMCPService()
	if err != nil {
		return err
	}
	return service.DeleteConnection(id)
}

func (a *App) SetMCPConnectionEnabled(id string, enabled bool) error {
	service, err := a.clientMCPService()
	if err != nil {
		return err
	}
	return service.SetConnectionEnabled(id, enabled)
}

func (a *App) SetMCPToolAllowed(id, tool string, allowed bool) error {
	service, err := a.clientMCPService()
	if err != nil {
		return err
	}
	return service.SetToolAllowed(id, tool, allowed)
}

func (a *App) GetMCPTools(id string) ([]mcpclient.ToolView, error) {
	service, err := a.clientMCPService()
	if err != nil {
		return nil, err
	}
	return service.Tools(id)
}

func (a *App) RefreshMCPTools(id string) ([]mcpclient.ToolView, error) {
	service, err := a.clientMCPService()
	if err != nil {
		return nil, err
	}
	return service.Refresh(a.ctx, id)
}

func (a *App) TestMCPConnection(id string) ([]mcpclient.ToolView, error) {
	return a.RefreshMCPTools(id)
}

func (a *App) GetMCPServerSettings() (mcpserver.LocalView, error) {
	service, err := a.serverMCPService()
	if err != nil {
		return mcpserver.LocalView{}, err
	}
	return service.LocalSettings(), nil
}

func (a *App) SaveMCPServerSettings(input servercfg.Local) (mcpserver.LocalView, error) {
	service, err := a.serverMCPService()
	if err != nil {
		return mcpserver.LocalView{}, err
	}
	return service.SaveLocal(a.ctx, input)
}

func (a *App) RotateMCPServerCredential() (mcpserver.LocalView, error) {
	service, err := a.serverMCPService()
	if err != nil {
		return mcpserver.LocalView{}, err
	}
	return service.RotateCredential(a.ctx)
}

func (a *App) RevokeMCPServerCredential() error {
	service, err := a.serverMCPService()
	if err != nil {
		return err
	}
	return service.RevokeCredential(a.ctx)
}

// currentNovel 读取界面确认过的当前小说，供每次 MCP 调用取得独立快照。
func (a *App) currentNovel(ctx context.Context) (mcpserver.CurrentNovel, error) {
	id := a.activeNovelID.Load()
	if id <= 0 {
		return mcpserver.CurrentNovel{}, mcpserver.ErrNoCurrentNovel
	}
	var item novel.Novel
	if err := a.novel.DB.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return mcpserver.CurrentNovel{}, mcpserver.ErrNoCurrentNovel
		}
		return mcpserver.CurrentNovel{}, fmt.Errorf("读取当前小说: %w", err)
	}
	return mcpserver.CurrentNovel{ID: item.ID, Title: item.Title}, nil
}
