# 音乐流媒体系统

本仓库是《[必看.md](/Users/hjie/数据/编程项目/音乐/必看.md)》对应的 MVP 工程骨架。

当前阶段目标：
- 建立统一目录结构
- 提供 API 与 Worker 入口
- 接通基础环境配置
- 预留数据库迁移、对象存储、监控和 OpenAPI 文档位置

当前仓库状态：
- 已完成 repo 脚手架
- 已提供健康检查与基础请求日志
- 已提供 Docker Compose 基础依赖清单
- 已提供数据库 migration CLI
- 已实现 Phase 1 认证接口的最小版本
- 当前认证仓储已切换为 PostgreSQL 实现
- `/health/ready` 已改为真实依赖探测（PostgreSQL / Redis / MinIO）
- 已实现曲库导入 CLI、`tracks` / `track_assets` 仓储和转码任务写入
- 已实现开发模式的本地媒体 staging、Worker 消费和 FFmpeg 转码链路
- 已实现 catalog、playback、history 的最小 API 闭环
- 已实现 Prometheus 文本格式的请求、耗时和依赖指标
- 已为登录和播放事件接入基础限流
- 已统一 API 错误响应结构为 `{"error":{"code","message"}}`
- 已补认证、catalog、playback、history 与限流器的基础单元测试
- 已补媒体 Worker 的转码成功/失败测试与主要失败路径测试
- 已补本地非容器集成回归测试入口（真实 PostgreSQL + 真实 FFmpeg）
- 已完成一轮真实容器联调（Colima/containerd + PostgreSQL + FFmpeg + API/Worker）
- 容器启动时会自动修正 `/srv/media` 卷权限，避免 Worker 转码时写 HLS 目录失败

## 目录说明

```text
.
├── cmd/                  # 程序入口
├── internal/             # 业务与平台代码
├── db/                   # 迁移与种子数据
├── deploy/               # 容器编排与监控配置
├── docs/                 # OpenAPI 与设计文档
├── e2e/                  # 集成回归测试
├── scripts/              # 开发辅助脚本
├── MVP-TASKS.md          # 可执行任务清单
└── 必看.md               # MVP 开发规范
```

## 快速开始

### 方式一：本机运行 API/Worker

1. 创建环境变量文件：

```bash
cp .env.example .env
```

2. 启动基础依赖：

```bash
make docker-infra-up
```

3. 执行数据库迁移：

```bash
make migrate-up
```

4. 本地启动 API：

```bash
make run-api
```

5. 本地启动 Worker：

```bash
make run-worker
```

### 方式二：容器运行完整栈

```bash
cp .env.example .env
make docker-up
make docker-migrate-up
```

如果你在 Colima / `nerdctl compose` 环境中联调：
- 优先使用 `make docker-migrate-up`，不要手工执行 `compose run migrate`
- 如果已经残留额外的 `deploy-postgres-run-*` 临时容器，先清理孤儿容器，再重启 API / Worker

## 开发模式媒体导入

本地开发默认使用 `LOCAL_MEDIA_ROOT` 作为媒体根目录：
- `make stage-media` 将本地音频文件复制到媒体根目录
- `make import-track` 写入数据库和转码任务
- `make run-worker` 或容器 Worker 消费任务并执行 FFmpeg 转码
- API 通过 `/media/*` 暴露生成的 HLS 文件

示例：

```bash
make stage-media ARGS='--source /path/to/song.mp3 --key raw/song.mp3'
make import-track ARGS='--title "Song A" --artist "Artist A" --duration-sec 215 --source-object-key raw/song.mp3'
```

## 导入歌曲元数据

在当前开发模式下，`source_object_key` 指向 `LOCAL_MEDIA_ROOT` 下的相对路径。导入 CLI 负责：
- 写入 `tracks`
- 写入 `track_assets`
- 写入 `outbox_events`
- 将 `tracks.status` 设置为 `PROCESSING`

示例：

```bash
make import-track ARGS='--title "Song A" --artist "Artist A" --album "Album A" --duration-sec 215 --release-date 2024-01-02 --source-object-key tracks/song-a.mp3'
```

## 常用命令

```bash
make fmt
make test
FFMPEG_BINARY=/absolute/path/to/ffmpeg make test-integration
make stage-media ARGS='--source /path/to/song.mp3 --key raw/song.mp3'
make import-track ARGS='--title "Song A" --artist "Artist A" --duration-sec 215 --source-object-key tracks/song-a.mp3'
make migrate-up
make migrate-down
make docker-migrate-up
make docker-migrate-down
make docker-infra-up
make docker-up
make docker-down
```

## 当前已实现接口

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/tracks`
- `GET /api/v1/tracks/{trackId}`
- `GET /api/v1/search?q=`
- `POST /api/v1/playback/sessions`
- `POST /api/v1/playback/events`
- `GET /api/v1/me/history`

## 快速验证

手工联调步骤见 [docs/quick-verification.md](/Users/hjie/数据/编程项目/音乐/docs/quick-verification.md)。文档覆盖了注册、导入音频、授予播放权限、申请播放会话、上报事件和查询历史的最小闭环。

截至 2026-04-06，本项目已经在本机 Colima/containerd 环境完成一轮真实容器回归，实际验证通过：
- 注册、登录、刷新、登出
- 曲库搜索、详情查询、播放会话创建
- FFmpeg HLS 清单与分片生成
- Manifest 与分片文件访问
- 播放事件上报、历史查询和 `/metrics` 抓取

非容器本地回归也已提供自动化入口：

```bash
FFMPEG_BINARY=/absolute/path/to/ffmpeg make test-integration
```

这条回归会在测试进程内启动真实 PostgreSQL，串起注册、登录、导入、转码、搜索、播放、事件、历史、健康检查与指标验证。当前 Redis 和 MinIO 在 MVP 中仍只参与 `/health/ready` 探测，因此该回归使用本地 TCP dummy listener 覆盖 readiness 检查。

如果 Maven Central 下载嵌入式 PostgreSQL 二进制较慢，可以额外指定镜像：

```bash
E2E_POSTGRES_BINARY_REPOSITORY_URL=https://repo.maven.apache.org/maven2 \
FFMPEG_BINARY=/absolute/path/to/ffmpeg \
make test-integration
```

## 监控

- API 暴露 `/metrics`
- `make docker-up` 后可通过 Prometheus 访问 [http://localhost:9090](http://localhost:9090)
- 当前指标包括请求总量、请求耗时直方图和 readiness 依赖状态

## 当前约束

- 主后端语言固定为 Go
- 当前骨架优先满足 MVP，不引入 Kafka、Elasticsearch、Kubernetes
- 播放鉴权目前依赖 `user_entitlements` 表，暂无后台管理接口
- `manifestUrl` 当前是带过期时间戳的开发态 URL，不是正式签名 URL
- 限流目前为单进程内存实现，适合本地开发和单实例演示，不适合多实例生产
- 媒体 Worker 的单元测试仍使用可执行 stub 模拟 FFmpeg；真实二进制联调改由 `make test-integration` 覆盖
- 所有后续任务请先对照 [MVP-TASKS.md](/Users/hjie/数据/编程项目/音乐/MVP-TASKS.md)

## 后续升级方向

- 将限流迁移到 Redis 等共享存储
- 为媒体上传接入真正的 MinIO/S3 SDK，而不是仅依赖本地 staging
- 为播放地址改造成真正签名 URL 或鉴权代理
- 将本地集成回归扩展到真实 Redis / MinIO SDK 与容器编排
