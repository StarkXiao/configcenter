基于 Go 实现的配置中心 Web 项目，一款后端服务与原生管理界面，支持多应用多环境配置管理、版本发布回滚、SSE 长连接订阅与热更新。

# configcenter

## 项目说明

这是一个单节点配置中心服务。管理员可以创建应用和运行环境，维护配置草稿并发布不可变版本；客户端通过访问令牌读取配置，通过 SSE 长连接接收版本变化通知，并在重新拉取后热更新本地配置。

## 构建

```bash
go mod download
go build ./...
```

## 运行

```bash
go run ./cmd/server
```

服务默认监听 `127.0.0.1:8081`，管理令牌默认为 `local-admin-token`。生产环境请通过 `CONFIG_CENTER_ADDR`、`CONFIG_CENTER_ADMIN_TOKEN` 和 `CONFIG_CENTER_DB` 设置监听地址、强管理令牌及数据库路径。

## 测试与检查

```bash
go test ./...
go vet ./...
make check
```

## Docker 评测镜像

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh configcenter linux/amd64
./build_benzhi_docker.sh configcenter linux/arm64
```

构建脚本使用 `benzhi.Dockerfile`，镜像保留完整 Go 工具链，便于在容器内继续编译和测试。
