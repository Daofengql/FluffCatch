# 配置与部署

本文档说明 FluffCatch 的配置文件、环境变量、数据库迁移、构建方式和运行时设置。

## 快速启动

```bash
go mod tidy
go run ./cmd/fluffcatch
```

这会使用默认 SQLite 数据库 `data/fluffcatch.db`。如果数据库文件不存在或为空，正常启动会自动创建表结构；首次启动如果还没有管理员密码，程序会把生成的密码哈希写入 `config.yaml` 并在终端显示一次明文密码。

如果要使用 MySQL：

```bash
cp config.example.yaml config.yaml
go run ./cmd/fluffcatch --migrate
go run ./cmd/fluffcatch
```

开发时另开终端启动前端：

```bash
cd www
npm install
npm run dev
```

默认地址：

- 后端：`http://localhost:8080`
- 前端开发服务器：`http://localhost:5173`
- 健康检查：`http://localhost:8080/api/v1/health`

## 配置优先级

配置来源优先级为：

1. 内置默认值。
2. YAML 配置文件。
3. 环境变量。

默认配置文件是 `config.yaml`。可以通过 `--config` 指定其他文件：

```bash
go run ./cmd/fluffcatch --config config.production.yaml
```

迁移和重置密码也支持同样的 `--config` 参数。

## app

```yaml
app:
  name: FluffCatch
  env: development
```

| 字段 | 说明 |
| --- | --- |
| `name` | 应用名称，也会作为站点默认名称。 |
| `env` | 运行环境。`production` 或 `release` 会启用更严格的 Cookie Secure 判断，并配合 release 构建关闭部分框架 info 日志。 |

环境变量：

- `APP_ENV`

## http

```yaml
http:
  addr: :8080
  read_timeout: 10s
  write_timeout: 30s
```

| 字段 | 说明 |
| --- | --- |
| `addr` | HTTP 监听地址。 |
| `read_timeout` | 请求读取超时。 |
| `write_timeout` | 响应写入超时。 |

环境变量：

- `HTTP_ADDR`
- `HTTP_READ_TIMEOUT`
- `HTTP_WRITE_TIMEOUT`

## database

FluffCatch 支持 SQLite 和 MySQL。默认 `database.driver=auto`：检测到 MySQL 连接字段时使用 MySQL；没有 MySQL 配置时使用内置 SQLite。MySQL 配置采用分项字段，不需要手写 DSN。

```yaml
database:
  driver: auto
  sqlite_path: data/fluffcatch.db
  host: 127.0.0.1
  port: 3306
  user: fluffcatch
  password: fluffcatch
  database: fluffcatch
  charset: utf8mb4
  location: Local
  parse_time: true
  connect_on_start: true
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime: 25m
  conn_max_idle_time: 5m
  timeout: 5s
  read_timeout: 30s
  write_timeout: 30s
  connect_retries: 5
  connect_retry_delay: 2s
```

| 字段 | 说明 |
| --- | --- |
| `driver` | `auto`、`sqlite` 或 `mysql`。默认 `auto`。 |
| `sqlite_path` | SQLite 数据库文件路径，默认 `data/fluffcatch.db`。 |
| `host` | MySQL 主机。 |
| `port` | MySQL 端口。 |
| `user` | MySQL 用户名。 |
| `password` | MySQL 密码。 |
| `database` | 数据库名。 |
| `charset` | 字符集，建议 `utf8mb4`。 |
| `location` | Go 时区名称，例如 `Local` 或 `Asia/Shanghai`。 |
| `parse_time` | 是否解析 MySQL 时间字段。 |
| `connect_on_start` | 启动时是否立刻连接数据库。 |
| `max_open_conns` | 最大打开连接数。 |
| `max_idle_conns` | 最大空闲连接数。 |
| `conn_max_lifetime` | 连接最大生命周期。 |
| `conn_max_idle_time` | 连接最大空闲时间。 |
| `timeout` | 建立连接超时。 |
| `read_timeout` | 读超时。 |
| `write_timeout` | 写超时。 |
| `connect_retries` | 启动连接失败后的重试次数。 |
| `connect_retry_delay` | 启动连接重试间隔。 |

环境变量：

- `DATABASE_DRIVER`
- `DB_DRIVER`
- `SQLITE_PATH`
- `DATABASE_SQLITE_PATH`
- `MYSQL_HOST`
- `MYSQL_PORT`
- `MYSQL_USER`
- `MYSQL_PASSWORD`
- `MYSQL_DATABASE`
- `MYSQL_CHARSET`
- `MYSQL_LOCATION`
- `MYSQL_PARSE_TIME`
- `MYSQL_CONNECT_ON_START`
- `MYSQL_MAX_OPEN_CONNS`
- `MYSQL_MAX_IDLE_CONNS`
- `MYSQL_CONN_MAX_LIFETIME`
- `MYSQL_CONN_MAX_IDLE_TIME`
- `MYSQL_TIMEOUT`
- `MYSQL_READ_TIMEOUT`
- `MYSQL_WRITE_TIMEOUT`
- `MYSQL_CONNECT_RETRIES`
- `MYSQL_CONNECT_RETRY_DELAY`

## 数据库迁移

迁移 SQL 位于 `migrations/`，并按数据库方言分开执行。迁移文件会编译进二进制文件，使用 release 二进制时不需要额外携带迁移目录。

SQLite：

- 默认数据库文件是 `data/fluffcatch.db`。
- 数据库文件不存在或为空时，正常启动会自动执行初始迁移。
- 已存在的 SQLite 数据库不会在每次启动时自动迁移；需要升级维护时可以显式执行迁移后退出：

```bash
go run ./cmd/fluffcatch --migrate
```

MySQL：

- 首次部署或升级时建议显式执行迁移。
- 迁移完成后进程会退出，再正常启动服务。

```bash
go run ./cmd/fluffcatch --config config.production.yaml --migrate
go run ./cmd/fluffcatch --config config.production.yaml
```

## auth

```yaml
auth:
  admin_username: admin
  admin_password_hash: ""
  session_secret: ""
```

| 字段 | 说明 |
| --- | --- |
| `admin_username` | 管理员用户名。 |
| `admin_password_hash` | 管理员密码哈希。首次为空时程序会生成随机密码并回写配置。 |
| `session_secret` | 私密访问签名密钥。生产环境建议设置随机长字符串。 |

环境变量：

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD_HASH`
- `SESSION_SECRET`

管理员登录会话保存在后端进程内存中，并通过 `fluffcatch_session` Cookie 关联。服务重启后管理员需要重新登录；如果部署多实例，需要让同一浏览器会话固定访问同一个后端实例。

首次迁移或首次启动时，如果 `admin_password_hash` 为空，程序会自动生成管理员密码并在终端输出一次：

```text
username=admin password=...
```

忘记密码时可以重置：

```bash
go run ./cmd/fluffcatch --reset-admin-password
go run ./cmd/fluffcatch --reset-admin-password --admin-password "new-password"
```

## oidc

```yaml
oidc:
  enabled: false
  provider: Keycloak
  issuer_url: ""
  client_id: ""
  client_secret: ""
  bound_subject: ""
```

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用 OIDC。 |
| `provider` | 前端展示的提供商名称，默认 `Keycloak`。 |
| `issuer_url` | OIDC Issuer URL。 |
| `client_id` | OIDC Client ID。 |
| `client_secret` | OIDC Client Secret。 |
| `bound_subject` | 已绑定的管理员 OIDC subject，通常由后台绑定流程写入。 |

环境变量：

- `OIDC_ENABLED`
- `OIDC_PROVIDER`
- `OIDC_ISSUER_URL`
- `OIDC_CLIENT_ID`
- `OIDC_CLIENT_SECRET`
- `OIDC_BOUND_SUBJECT`

OIDC 回调地址由请求信息推导：

```text
<scheme>://<host><forwarded-prefix>/api/v1/auth/oidc/callback
```

反向代理部署时应正确传递：

- `X-Forwarded-Proto`
- `X-Forwarded-Host`
- `X-Forwarded-Prefix`，如果应用挂在子路径下。

## storage

启动配置中的 `storage` 会成为默认运行时存储策略。

```yaml
storage:
  driver: local
  local_path: data/uploads
  public_prefix: /media
  public_base_url: ""
  s3:
    endpoint: ""
    bucket: ""
    region: us-east-1
    access_key: ""
    secret_key: ""
    use_ssl: true
    account_id: ""
```

| 字段 | 说明 |
| --- | --- |
| `driver` | 存储驱动：`local`、`s3`、`minio`、`aws-s3`、`aliyun-oss`、`tencent-cos`、`cf-r2`。 |
| `local_path` | 本地存储根目录。 |
| `public_prefix` | 本地媒体访问前缀，默认 `/media`。 |
| `public_base_url` | 外部公开访问基础 URL，用于 CDN 或公开桶。 |
| `s3.*` | S3 兼容存储配置。 |

环境变量：

- `STORAGE_DRIVER`
- `STORAGE_LOCAL_PATH`
- `STORAGE_PUBLIC_PREFIX`
- `STORAGE_PUBLIC_BASE_URL`
- `S3_ENDPOINT`
- `S3_BUCKET`
- `S3_REGION`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- `S3_USE_SSL`
- `S3_ACCOUNT_ID`

后台设置页可以维护多个存储策略。新上传文件写入当前激活策略；旧文件按数据库里记录的策略 ID 读取。

## upload

```yaml
upload:
  max_size_mb: 20
  max_video_size_mb: 500
  max_files_per_upload: 20
  default_page_size: 24
  max_concurrent_uploads: 2
```

| 字段 | 说明 |
| --- | --- |
| `max_size_mb` | 图片等普通文件最大大小，单位 MB。 |
| `max_video_size_mb` | 视频最大大小，单位 MB。 |
| `max_files_per_upload` | 单批最多文件数。 |
| `default_page_size` | 画廊默认分页大小。 |
| `max_concurrent_uploads` | 后端允许的并发上传请求数，硬上限为 8。 |

环境变量：

- `UPLOAD_MAX_SIZE_MB`
- `UPLOAD_MAX_VIDEO_SIZE_MB`
- `UPLOAD_MAX_FILES_PER_UPLOAD`
- `UPLOAD_DEFAULT_PAGE_SIZE`
- `UPLOAD_MAX_CONCURRENT_UPLOADS`

这些设置启动后也会进入数据库运行时设置，之后可在后台更新。

## frontend

前端服务模式不建议写入 `config.yaml`。开发调试时通过命令行参数 `--frontend-mode` 指定；部署时如需覆盖，可使用环境变量。

| 模式 | 说明 |
| --- | --- |
| `auto` | 优先使用内嵌前端，没有内嵌时读取磁盘静态目录。 |
| `embedded` | 只使用内嵌前端，需要 `embed_frontend` 构建标签。 |
| `disk` | 只读取 `static_root` 指向的静态文件目录。 |
| `external` | Go 服务不提供前端，适合 Vite 独立开发。 |
| `disabled` | 关闭前端静态服务，只提供 API。 |

命令行参数：

- `--frontend-mode`

环境变量：

- `FRONTEND_MODE`
- `FRONTEND_STATIC_ROOT`
- `STATIC_ROOT`，兼容旧命名。

开发时常用：

```bash
go run ./cmd/fluffcatch --frontend-mode external
cd www
npm run dev
```

## 生产构建

```bash
cd www
npm install
npm run build
cd ..
go build -trimpath -buildvcs=false -tags embed_frontend -ldflags "-s -w -X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch ./cmd/fluffcatch
```

Windows：

```bash
go build -trimpath -buildvcs=false -tags embed_frontend -ldflags "-s -w -X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch.exe ./cmd/fluffcatch
```

构建产物会内嵌前端静态文件。默认 SQLite 数据库、上传对象、Logo 和背景图会落在本地 `data/` 目录；如果配置 MySQL 或外部对象存储，这些运行时数据仍在对应外部服务中。当前 SQLite 驱动为纯 Go，发布构建也可以设置 `CGO_ENABLED=0` 来获得更好的跨机器可移植性。

## 单二进制 SQLite 部署

最小部署可以只发布二进制文件：

```bash
./fluffcatch
```

首次启动会：

- 使用 `data/fluffcatch.db` 创建 SQLite 数据库。
- 在数据库文件不存在或为空时自动执行 SQLite 初始迁移。
- 使用 `data/uploads` 作为本地对象存储。
- 如果没有 `config.yaml`，在生成管理员密码和会话密钥时创建一个只包含安全配置的 `config.yaml`。

这种模式适合个人站点、轻量 NAS、内网返图站或低并发部署。需要更强的并发写入、集中备份或已有数据库运维体系时，可以切换到 MySQL。

## 常用命令

```bash
make api
make web
make build
make test
make migrate
make reset-admin-password
```
