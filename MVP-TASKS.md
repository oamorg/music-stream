# 音乐流媒体系统 MVP 任务清单

本清单直接对应《[必看.md](/Users/hjie/数据/编程项目/音乐/必看.md)》中的 Phase 0 到 Phase 5。

状态说明：
- `[x]` 已完成
- `[ ]` 未开始

## Phase 0：工程初始化

交付目标：建立统一工程骨架，确保后续功能开发不会偏离规范。

- [x] 创建目录结构：`cmd/`、`internal/`、`db/`、`deploy/`、`docs/`、`scripts/`
- [x] 创建 API 与 Worker 程序入口
- [x] 提供基础环境变量模板 `.env.example`
- [x] 提供 `Makefile`
- [x] 提供 `README.md`
- [x] 提供 `deploy/docker-compose.yml`
- [x] 提供健康检查接口 `/health/live` 和 `/health/ready`
- [x] 提供基础请求日志中间件
- [x] 创建迁移目录和首版初始化迁移
- [x] 创建 OpenAPI 占位文档
- [x] 确定数据库迁移工具并补充执行命令
- [x] 为 `/health/ready` 接入真实依赖探测
- [x] 接入标准 Prometheus 指标采集
- [x] 初始化 Git 仓库
- [ ] 建立首个提交

## Phase 1：认证闭环

交付目标：用户可以完成注册、登录、刷新令牌与登出。

当前说明：
- 当前仓库已提供 PostgreSQL 版认证模块实现，后续需补数据库集成测试和迁移执行方式。

- [x] 实现 `users` 与 `refresh_tokens` 仓储层
- [x] 实现密码哈希与校验逻辑
- [x] 实现 JWT access token 生成
- [x] 实现 refresh token 持久化、撤销和轮换
- [x] 实现 `POST /api/v1/auth/register`
- [x] 实现 `POST /api/v1/auth/login`
- [x] 实现 `POST /api/v1/auth/refresh`
- [x] 实现 `POST /api/v1/auth/logout`
- [x] 编写认证模块单元测试
- [x] 更新 OpenAPI 文档
- [x] 用 PostgreSQL 仓储替换当前内存实现

验收检查：
- [x] 正常注册返回用户标识
- [x] 正常登录返回 access token 与 refresh token
- [x] 错误密码返回 401
- [x] 已撤销 refresh token 无法继续刷新

## Phase 2：曲库与媒体处理

交付目标：导入歌曲、上传原始音频、转码出 HLS。

- [x] 实现 `tracks` 与 `track_assets` 仓储层
- [x] 提供内部 CLI 或内部管理接口用于导入歌曲元数据
- [x] 提供开发模式的本地媒体 staging 流程
- [x] 实现 FFmpeg HLS 转码任务
- [x] 实现 `outbox_events` 任务写入与 Worker 消费
- [x] 实现 `outbox_events` 任务写入
- [x] 实现 Worker 消费
- [x] 完成导入阶段的 `tracks.status` 与 `track_assets.status` 初始状态流转
- [x] 为转码失败记录明确错误信息
- [x] 编写固定测试音频的媒体处理验证
- [x] 更新 OpenAPI 或内部接口文档

验收检查：
- [x] 测试音频可成功生成 HLS 清单和分片
- [x] 成功转码后 `tracks.status = READY`
- [x] 失败时 `track_assets.status = FAILED`

## Phase 3：搜索与播放权限

交付目标：用户可以搜索歌曲、查看详情、申请播放会话。

- [x] 实现 `GET /api/v1/tracks`
- [x] 实现 `GET /api/v1/tracks/{trackId}`
- [x] 实现 `GET /api/v1/search?q=`
- [x] 建立 `pg_trgm` 与搜索索引
- [x] 实现 `user_entitlements` 仓储层
- [x] 实现播放权限校验逻辑
- [x] 实现 `POST /api/v1/playback/sessions`
- [x] 生成短时效签名 `manifestUrl`
- [x] 编写搜索与播放会话集成测试
- [x] 更新 OpenAPI 文档

验收检查：
- [x] 仅 `READY` 状态歌曲可搜索
- [x] 无权限播放返回 403
- [x] 有权限播放返回可用的 `manifestUrl`

## Phase 4：事件与历史

交付目标：记录播放行为并让用户查看自己的最近历史。

- [x] 实现 `playback_sessions` 与 `play_events` 仓储层
- [x] 实现 `POST /api/v1/playback/events`
- [x] 校验 `START`、`HEARTBEAT`、`COMPLETE` 事件格式
- [x] 为播放事件加入基础限流
- [x] 实现最近播放历史查询
- [x] 实现 `GET /api/v1/me/history`
- [x] 为历史接口添加鉴权
- [x] 编写事件上报与历史查询测试
- [x] 更新 OpenAPI 文档

验收检查：
- [x] 播放事件可成功落库
- [x] 历史接口只返回当前用户数据
- [x] 最近播放历史在 1 分钟内可见

## Phase 5：稳定性与交付

交付目标：满足联调、演示和回归要求。

- [x] 为登录接口加入限流
- [x] 为播放事件接口加入限流
- [x] 完成统一错误码结构
- [x] 补齐关键路径集成测试
- [x] 补齐失败路径测试
- [x] 完成 Prometheus 指标埋点
- [x] 完成部署与运行说明
- [x] 整理已知限制与后续升级路径

当前说明：
- 已新增 `e2e` 集成回归入口，覆盖认证、导入、转码、搜索、播放、事件、历史、健康检查与指标
- 该回归依赖本机可用的 `go` 和真实 `ffmpeg`，执行时会启动真实 PostgreSQL；Redis / MinIO 目前仅作为 readiness TCP 探测
- 已于 2026-04-06 完成一轮真实容器联调，覆盖 Colima/containerd、PostgreSQL、FFmpeg、API/Worker、播放事件、历史查询与 `/metrics`

验收检查：
- [x] 登录、搜索、播放、上报全链路可回归
- [x] README 能指导新人完成环境启动
- [x] 无阻塞上线的高危 TODO

## 当前建议的执行顺序

1. 先补真实依赖探测和迁移工具
2. 再做认证模块
3. 然后做曲库导入与 HLS 转码
4. 再做搜索与播放权限
5. 最后补事件历史、限流、指标和文档
