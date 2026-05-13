# FluffCatch

FluffCatch 是一个自部署的兽聚返图收集与画廊应用。它面向单管理员自用场景：按兽聚创建独立卡片，接收匿名或署名投稿，审核后转入正式画廊，并支持标签与隐私访问控制。

## 技术栈

- Go 后端位于仓库根目录。
- React + TypeScript + MUI 前端位于 `www/`。
- MySQL 数据库结构位于 `migrations/`。
- 默认使用本地文件系统存储图片，并预留 MinIO/S3 存储接口。

## 当前状态

当前仓库已经具备可运行的前后端骨架与首版核心闭环：兽聚卡片、公开画廊、图片投稿、后台审核、数据库会话登录、多存储策略读取、站点信息设置与前端内嵌构建。后续仍可继续补强图片缩略图、EXIF 展示、OIDC 实际登录和更完整的隐私访问体验。

## 快速开始

```bash
cp config.example.yaml config.yaml
go mod tidy
go run ./cmd/fluffcatch
```

另开一个终端启动前端开发服务器：

```bash
cd www
npm install
npm run dev
```

- 后端：`http://localhost:8080`
- 前端开发服务器：`http://localhost:5173`
- 健康检查：`http://localhost:8080/api/v1/health`

数据库连接使用分项配置，不需要手写 DSN URL。请在 `config.yaml` 中分别填写：

- `database.host`
- `database.port`
- `database.user`
- `database.password`
- `database.database`

默认情况下，后端启动时会连接 MySQL，但不会自动建表。建表需要显式进入迁移模式。

后端使用 Go 标准库连接池管理 MySQL 连接。默认会启用驱动连接存活检查，并设置连接、读取、写入超时；如果 MySQL 临时断开，后续请求会从池中重新获取可用连接。启动连接失败时会按 `database.connect_retries` 和 `database.connect_retry_delay` 自动重试。

如遇到本机 MySQL、代理或防火墙偶发断开，可以按需在 `config.yaml` 中增加这些可选项：

- `database.max_open_conns`：最大打开连接数，默认 `20`。
- `database.max_idle_conns`：最大空闲连接数，默认 `10`。
- `database.conn_max_lifetime`：连接最长生命周期，默认 `25m`。
- `database.conn_max_idle_time`：连接最大空闲时间，默认 `5m`。
- `database.timeout`：建立连接超时，默认 `5s`。
- `database.read_timeout`：读取超时，默认 `30s`。
- `database.write_timeout`：写入超时，默认 `30s`。
- `database.connect_retries`：启动连接重试次数，默认 `5`。
- `database.connect_retry_delay`：启动连接重试间隔，默认 `2s`。

如果只想执行数据库迁移，不进入主系统，可以运行：

```bash
go run ./cmd/fluffcatch --migrate
```

迁移完成后进程会提示重新启动，然后退出；再次不带 `--migrate` 启动即可进入主系统。

如果需要使用非默认配置文件，可以通过 `--config` 指定：

```bash
go run ./cmd/fluffcatch --config config.production.yaml
go run ./cmd/fluffcatch --config config.production.yaml --migrate
```

不指定时默认读取 `config.yaml`。配置优先级为：默认值 < YAML 文件 < 系统环境变量，便于 Docker 或 systemd 注入密码、端口等部署参数。

首次迁移或首次启动时，如果 `auth.admin_password_hash` 为空，程序会自动生成管理员密码，把哈希写入 `config.yaml`，并在终端输出一次随机密码：

```text
username=admin password=...
```

请立刻保存该密码；后续不会再次显示。

如果忘记管理员密码，可以运行：

```bash
go run ./cmd/fluffcatch --reset-admin-password
```

也可以手动指定新密码：

```bash
go run ./cmd/fluffcatch --reset-admin-password --admin-password "new-password"
```

重置完成后进程会提示重新启动，然后退出。

`config.yaml` 保留启动必需项、管理员账号密码哈希和 OIDC 登录配置。后台仍可修改密码和绑定 OIDC 账号，这些账号安全改动会写回配置文件；OIDC 客户端配置只通过配置文件维护。站点名称、站点副标题、Logo、外部对象存储策略、上传限制、分页默认值与上传并发数会保存到数据库 `settings` 表，运行时可在后台设置中更新。

## 生产构建

```bash
cd www
npm install
npm run build
cd ..
go build -tags embed_frontend -ldflags "-X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch ./cmd/fluffcatch
```

Windows 下也可以构建为：

```bash
go build -tags embed_frontend -ldflags "-X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch.exe ./cmd/fluffcatch
```

构建完成后，运行 `bin/fluffcatch` 或 `bin/fluffcatch.exe` 即可用一个二进制文件同时提供 API 和内嵌的 React 前端。MySQL 和已上传图片仍然是外部运行时数据，不会被打包进二进制文件。

开发时可以单独启动前后端：

```bash
go run ./cmd/fluffcatch --frontend-mode external
cd www
npm run dev
```

前端服务模式不需要写入 `config.yaml`。开发调试时通过命令行参数 `--frontend-mode` 指定即可；环境变量 `FRONTEND_MODE` 仍保留给 Docker 或 systemd 等部署环境覆盖：

- `auto`：优先使用内嵌前端，没有内嵌时读取 `www/dist`。
- `embedded`：只使用内嵌前端，需要使用 `embed_frontend` 构建标签。
- `disk`：只读取 `FRONTEND_STATIC_ROOT` 指向的静态文件目录。
- `external`：不由 Go 服务提供前端，适合开发时独立运行 Vite。
- `disabled`：完全关闭前端静态服务。

## API

所有 API 路由都位于 `/api/v1` 下。

- `GET /health`
- `GET /auth/captcha`
- `POST /auth/login`
- `POST /auth/logout`
- `GET /auth/me`
- `GET /events`
- `GET /site`
- `GET /events/{id}`
- `GET /events/{id}/photos`
- `POST /photos/{id}/like`
- `POST /events/{id}/submissions`
- `GET /admin/dashboard`
- `GET /admin/events`
- `POST /admin/events`
- `PUT /admin/events/{id}`
- `DELETE /admin/events/{id}`
- `POST /admin/events/{id}/cover`
- `GET /admin/events/{id}/photos`
- `GET /admin/submissions`
- `POST /admin/submissions/batch-approve`
- `POST /admin/submissions/batch-delete`
- `PUT /admin/photos/{id}`
- `GET /admin/settings`
- `PUT /admin/settings/storage`
- `PUT /admin/settings/site`
- `POST /admin/settings/site/logo`
- `DELETE /admin/settings/site/logo`

管理员登录使用 `config.yaml` 中的 `auth.admin_username` 与 `auth.admin_password_hash`，登录页包含图片验证码；登录后通过数据库中的 `sessions` 表和 `fluffcatch_session` Cookie 访问后台。OIDC 配置和绑定身份也保存在配置文件中。

## 前后台路由

- 公开页面：`/`、`/submit`、`/events/:id`、`/login`
- 后台页面：`/admin/dashboard`、`/admin/events`、`/admin/submissions`、`/admin/settings`

## 领域模型

- 每个兽聚都是一个独立卡片，包含标题、简介、举办地点、开始/结束时间、公开状态、封面图和投稿口令。
- 兽聚地点分为“行政区”和“详细地点”：行政区使用省/市级联选择，详细地点继续用原 `location` 字段填写酒店、会场或补充说明。
- 首页可按省份或城市筛选兽聚；未补录行政区的旧数据仍会显示详细地点，但不会出现在省市筛选结果中。
- 访客可以进入某个兽聚的独立投稿页上传图片，并可留下摄影师署名与自由 `#标签`。
- 管理员可以查看待审核投稿，并批量通过或批量删除。
- 通过后的图片进入对应兽聚画廊，可设置为公开、密码访问或私有。
- 支持多个存储策略；新上传文件使用当前默认策略，已上传文件按照记录中的 `storage_policy_id` 找回原策略读取。
- 本地存储图片通过后端 `/media/{policyId}/...` 提供；S3/MinIO 等外部策略应配置公开桶或 CDN 的 `publicBaseUrl`，前端直接访问对象 URL，不经过后端代理。
- 管理页可查看每个存储策略的对象数量与占用量；仍有文件引用的策略不能删除，避免幽灵文件。
- 后台系统设置可以修改站点名称、副标题和首页 Markdown 介绍卡片；Logo 只能通过后台上传，强制保存到本地存储并写入内部 URL。
- 公开画廊图片以小卡片展示，会显示摄影师、文件类型、文件大小、时间和标签等基础信息。
- 公开图片支持点赞；访客点赞使用 IP、浏览器 UA 与语言生成哈希指纹去重，不保存原始 IP。
- 后台投稿审核、兽聚图片管理的卡片/列表视图会记忆在当前浏览器本地，刷新后保持上次选择。

## 数据库迁移

迁移 SQL 会编译进二进制文件，发布包不需要额外携带 `migrations/` 目录。首次部署或升级时运行：

```bash
./fluffcatch --migrate
```

## 常用命令

```bash
make api
make web
make build
make test
make migrate
make reset-admin-password
```

`make build` 会先构建前端，再使用 `embed_frontend` 构建标签编译包含内嵌前端的 Go 二进制文件，并写入 release 构建标记以关闭 Gin 和 GORM 的框架 info 日志。
