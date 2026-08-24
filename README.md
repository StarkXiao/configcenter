# Go Config Center

一个单节点 Go 配置中心，支持多应用、多环境、配置草稿、不可变发布版本、差异比较、版本回滚、SSE 长连接通知、条件拉取和原生 Web 管理界面。

## 启动

需要 Go 1.24 或更高版本。

```bash
go run ./cmd/server
```

浏览器打开 `http://127.0.0.1:8081`，输入开发管理令牌 `local-admin-token` 后连接。SQLite 数据默认写入 `./data/config-center.db`。默认地址仅监听本机；生产部署必须通过环境变量设置强管理令牌和实际监听地址。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `CONFIG_CENTER_ADDR` | `127.0.0.1:8081` | HTTP 监听地址 |
| `CONFIG_CENTER_DB` | `./data/config-center.db` | SQLite 数据库文件 |
| `CONFIG_CENTER_ADMIN_TOKEN` | `local-admin-token` | 管理 API 令牌，生产环境必须覆盖 |
| `CONFIG_CENTER_LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |
| `CONFIG_CENTER_HEARTBEAT` | `20s` | SSE 心跳间隔 |
| `CONFIG_CENTER_SHUTDOWN_TIMEOUT` | `10s` | 优雅关闭超时 |
| `CONFIG_CENTER_EVENT_HISTORY` | `1000` | 单节点事件重放缓存数量 |

## 客户端接入

创建应用后，接口只返回一次客户端访问令牌。客户端先读取配置，再订阅变更：

```text
GET /client/v1/apps/{app}/envs/{env}/config
GET /client/v1/apps/{app}/envs/{env}/subscribe
Authorization: Bearer <client-access-token>
```

订阅事件只携带版本和校验和。客户端收到事件后应再次调用配置接口，解析成功后原子替换本地配置；断线时携带 `Last-Event-ID` 重连，服务端会重放缓存事件或要求全量同步。

## 开发检查

```bash
make check
```

项目规模约束由 `make size` 检查：非测试 Go 文件必须为 21–24 个，非测试 Go 代码必须为 2001–2199 行。
