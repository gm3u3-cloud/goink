# MCP Server 配置与运行管理

当前以 Goink 本地 MCP Server 为交付范围，不提供远程 Server 部署，也没有单小说级别的 MCP 配置。Client 的职责见 [Client 设计说明](../mcpclient/design.md)。

## 各部分职责

```mermaid
flowchart TD
    App[app：界面 API 与应用生命周期] --> Service[mcpserver.Service：本地 Server 配置和运行状态协调]
    Service --> Store[servercfg.Store：Server 配置存储]
    Service --> Server[mcpserver.Server：Goink 本地服务]
    Server --> Tools[mcp_tools.Registry：内部工具]
```

| 部分 | 负责内容 |
| --- | --- |
| `app` | Wails API、DTO 和错误适配；在应用初始化与退出时接入管理服务。 |
| `mcpserver.Service` | 协调配置存储和本地 Server；管理启停、工具开放策略、令牌生成、轮换及撤销。 |
| `mcpserver/servercfg` | 配置结构及其静态校验放在 types.go；Store 读取和原子写入全局 Server 配置，复用应用加密实现，不管理网络连接。 |
| `mcpserver.Server` | 本地监听、Token 鉴权、工具白名单、当前小说上下文和在途调用管理。 |
| `mcp_tools.Registry` | 注册内部领域工具，供本地调用和 MCP Server 选用。 |

Goink 当前只有一个本地 Server 实例，其运行生命周期由 Server 自己管理，无需增加 Server Manager。Client 与 Server 分别由各自 Service 管理，应用退出时分别关闭。

`servercfg` 定义 Server 静态配置规则：本地端口、令牌格式，以及启用状态与 Token 的一致性。Store 加载、保存时补齐默认端口，并调用 State.Validate；Load 成功返回的结果已经通过配置包全部校验。无效快照不会写入磁盘，加载失败保留原文件。Service 检查当前工具开放策略并编排配置与运行状态；Server 检查工具是否实际注册以及是否完全停止，负责执行运行操作。

App 通过 Server 管理服务关闭 MCP Server；管理服务尚未创建时才直接关闭 Server，避免重复关闭同一实例。

## 查询 DTO

`LocalView.config` 复用 `servercfg.Local`，其余字段只提供可开放工具、Token、运行状态和错误。凭据是否存在由 `token` 是否为空判断，不重复返回 `has_credential`。

本地设置查询、保存和轮换均直接返回 `LocalView`，Token 只出现在其 `token` 字段，不再额外包装重复的 Token。当前接口尚未发行，同步生成绑定和调用方，不保留旧 DTO 兼容层。

## 存储

Server 配置保存在 `~/.goink/mcp_config.enc`，快照包含 local 和 local_token，即本地 Server 开关、端口、工具权限和 Token。Client 使用独立文件，两个 Store 不会覆盖对方配置。

配置与 LLM 共用原有 AES-256-GCM 密钥。写入通过同目录临时文件、同步、关闭和重命名完成，配置文件权限为 `0600`。当前格式尚未发行，不提供旧 MCP 格式迁移。

## 配置异常与核心应用启动

读取、解密或配置校验失败只停用本地 MCP Server，不阻止小说编辑等核心功能初始化或 Client 配置加载。Service 构造失败时仍返回一个停用的管理实例和加载错误；调用方保留该实例供设置查询和统一关闭使用。

异常文件原样保留，不自动覆盖或恢复为默认文件。设置查询返回加载错误；配置恢复前拒绝写入和启动 MCP，避免用空状态覆盖原配置。修复配置文件后重新启动应用即可重新加载，当前不增加配置重置或迁移机制。

## Token

Service 在首次启用且没有 Token 时生成并保存 Token，然后交给 Server 鉴权。普通启停和应用重启沿用已保存的 Token；主动轮换生成新 Token，撤销则清除 Token 并关闭服务。

这是本地桌面管理接口，不采用一次性展示策略。Server 设置查询与设置保存结果均返回当前已保存的 Token，包括配置保存成功但后续启停失败的情况。前端即使因 Wails 错误返回而丢弃操作结果，也可重新查询设置获得 Token。落盘失败时不返回未保存的新 Token。

可重复读取的凭据仅指 Goink 自己的 Server Token。外部连接的凭据由 Client 方向独立管理。

## 地址与端口

Goink 仅监听 `127.0.0.1`，默认端口为 `5108`，端点为 `http://127.0.0.1:5108/mcp`。允许保存自定义端口，合法范围为 `1..65535`；配置未指定端口时使用默认值，保存设置未指定端口时沿用当前值。

重启、修改工具权限和轮换 Token 均沿用配置中的端口。端口被占用时返回启动错误，不回退到随机端口，避免客户端地址悄悄改变。底层 Server 允许测试显式使用端口 `0`，应用管理层不会将 `0` 用作实际监听端口。

## 验证

- 异常配置下核心应用继续可用，MCP 停用、错误可查询，原文件不被覆盖。
- Token 可重复查询；首次启用和轮换在启停失败后仍可恢复凭据。
- 重启和轮换不改变端口；端口冲突明确报错，释放端口后可重试。
- 运行相关 Go 测试、竞态检查和应用构建，并通过 Wails 生成器更新绑定。
