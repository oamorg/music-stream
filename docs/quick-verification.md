# 快速验证

本文件用于手工验证当前 MVP 的最小闭环。假设本机已安装 `go`、`docker`、`ffmpeg`、`curl`。

## 0. 自动化本地回归

如果你已经有可用的 `go` 和真实 `ffmpeg` 二进制，优先跑自动化集成回归：

```bash
FFMPEG_BINARY=/absolute/path/to/ffmpeg make test-integration
```

说明：
- 这条回归会在测试进程内启动真实 PostgreSQL，并完成 migration、导入、转码、播放和历史校验
- 当前 MVP 对 Redis / MinIO 还没有真实业务读写，测试会使用本地 TCP listener 满足 `/health/ready`
- 真正的容器联调仍按下面的手工步骤执行

已验证结果：
- 2026-04-06 已在本机 Colima/containerd 环境完成一轮真实容器联调
- 验证覆盖注册、登录、刷新、登出、导入、FFmpeg 转码、播放、事件、历史和 `/metrics`

## 1. 启动依赖与服务

```bash
cp .env.example .env
make docker-infra-up
make migrate-up
make run-api
make run-worker
```

如果你想容器化运行 API/Worker，可以改用：

```bash
make docker-up
make docker-migrate-up
```

补充说明：
- 当前镜像会在启动时自动修正 `/srv/media` 卷权限，避免 Worker 无法创建 `hls/` 输出目录
- 如果你使用的是 Colima / `nerdctl compose`，避免手工执行 `compose run migrate`
- 一旦出现额外的 `deploy-postgres-run-*` 临时容器，先清理它们，再重启 API / Worker，避免服务发现落到错误的 PostgreSQL 实例

## 2. 注册并登录

注册：

```bash
curl -s http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"super-secret-password"}'
```

登录并记录 `accessToken`：

```bash
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"super-secret-password"}'
```

## 3. 导入一首测试歌曲

先把源音频放到本地媒体目录：

```bash
make stage-media ARGS='--source /absolute/path/to/song.mp3 --key raw/song.mp3'
```

再写入曲库和转码任务：

```bash
make import-track ARGS='--title "Song A" --artist "Artist A" --album "Album A" --duration-sec 215 --source-object-key raw/song.mp3'
```

等待 Worker 完成转码后，确认数据库中歌曲和素材均为 `READY`：

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
  psql postgres://music:music@postgres:5432/music -c \
  "select id, title, status from tracks order by id desc limit 5;"
```

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
  psql postgres://music:music@postgres:5432/music -c \
  "select id, track_id, status, hls_manifest_key, error_message from track_assets order by id desc limit 5;"
```

## 4. 授予播放权限

当前 MVP 还没有后台管理接口，开发时直接插入 `user_entitlements`：

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
  psql postgres://music:music@postgres:5432/music -c \
  "insert into user_entitlements (user_id, track_id, access_type)
   select u.id, t.id, 'STREAM'
   from users u
   join tracks t on t.title = 'Song A'
   where u.email = 'demo@example.com'
   on conflict (user_id, track_id) do nothing;"
```

## 5. 创建播放会话

先查询刚导入歌曲的实际 `track_id`：

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
  psql postgres://music:music@postgres:5432/music -c \
  "select id, title, status from tracks where title = 'Song A' order by id desc limit 1;"
```

再将登录返回的 access token 和查询到的 `track_id` 填入：

```bash
curl -s http://localhost:8080/api/v1/playback/sessions \
  -H 'Authorization: Bearer ACCESS_TOKEN_HERE' \
  -H 'Content-Type: application/json' \
  -d '{"trackId":TRACK_ID_HERE}'
```

预期返回 `manifestUrl`，地址类似：

```text
http://localhost:8080/media/hls/asset-1/index.m3u8?expires=...
```

## 6. 上报播放事件

将创建播放会话返回的 `sessionId` 填入：

```bash
curl -s http://localhost:8080/api/v1/playback/events \
  -H 'Authorization: Bearer ACCESS_TOKEN_HERE' \
  -H 'Content-Type: application/json' \
  -d '{"sessionId":SESSION_ID_HERE,"eventType":"START","positionSec":0,"clientTimestamp":"2026-04-06T12:00:00Z"}'
```

```bash
curl -s http://localhost:8080/api/v1/playback/events \
  -H 'Authorization: Bearer ACCESS_TOKEN_HERE' \
  -H 'Content-Type: application/json' \
  -d '{"sessionId":SESSION_ID_HERE,"eventType":"HEARTBEAT","positionSec":30,"clientTimestamp":"2026-04-06T12:00:30Z"}'
```

## 7. 查询最近历史

```bash
curl -s 'http://localhost:8080/api/v1/me/history?limit=10' \
  -H 'Authorization: Bearer ACCESS_TOKEN_HERE'
```

预期历史中能看到刚才的歌曲和最近一次事件位置。

## 8. 重点检查项

- `/health/live` 返回 `200`
- `/health/ready` 能看到 PostgreSQL、Redis、MinIO 的依赖检查结果
- `/metrics` 能看到 `http_requests_total`、`http_request_duration_seconds`、`dependency_up`
- 登录接口在同一窗口内超限后返回 `429`
- 无播放权限时 `/api/v1/playback/sessions` 返回 `403`
- `play_events` 入库后，`/api/v1/me/history` 只返回当前用户的数据
