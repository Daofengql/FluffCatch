# FluffCatch - 活动返图收集与画廊系统

FluffCatch 是一个自部署的活动返图收集与画廊系统，适用于漫展、兽聚、摄影活动、社群聚会等场景。管理员可以按活动创建独立页面，生成限时投稿链接，接收匿名或署名的图片和视频投稿，审核后发布到公开或私密画廊，并支持标签、点赞、下载、站点设置和多存储策略。

## 仓库地址

- GitHub: https://github.com/Daofengql/FluffCatch
- SSH: `git@github.com:Daofengql/FluffCatch.git`
- HTTPS: `https://github.com/Daofengql/FluffCatch.git`

```bash
git clone git@github.com:Daofengql/FluffCatch.git
```

## 典型流程

1. 管理员创建活动页，配置时间、地点、封面、公开状态和私密访问口令。
2. 管理员为活动生成限时投稿链接，可限制有效期、使用次数并绑定摄影师名称。
3. 投稿者通过链接上传图片或视频，文件进入待审核池。
4. 管理员审核投稿，选择公开或私密可见性，通过后进入正式画廊。
5. 访客浏览活动画廊，筛选、点赞、下载公开返图；输入口令后可访问私密返图。

## 功能概览

- 公开站点：活动列表、活动详情、公开画廊、统一投稿入口和管理员登录页。
- 活动管理：标题、简介、地点、省市筛选、时间、公开状态、投稿开关、封面和私密访问口令。
- 投稿链接：可为单个活动创建多个限时链接，支持过期时间、使用次数和摄影师绑定。
- 投稿审核：访客投稿先进入待审核池，管理员可批量通过或删除，并选择公开或私密可见性。
- 画廊管理：支持图片和视频、标签、摄影师、拍摄时间、排序筛选、点赞、下载和批量操作。
- 站点设置：站点名称、副标题、Logo、首页 Markdown、主题、背景图、页脚和联系组件。
- 存储策略：支持本地、S3/MinIO/AWS S3、阿里云 OSS、腾讯云 COS、Cloudflare R2 等策略。
- 安全边界：管理员密码哈希、登录验证码、高风险操作验证码、投稿 token 哈希、私密访问签名 Cookie 和上传限制。

更完整的产品说明见 [docs/features.md](docs/features.md)。

## 技术栈

- 后端：Go，入口位于 `cmd/fluffcatch`。
- 前端：React + TypeScript + Vite + MUI，位于 `www/`。
- 数据库：默认 SQLite，也支持 MySQL；迁移文件位于 `migrations/`。
- 对象存储：默认本地文件系统，也支持多种 S3 兼容或云厂商对象存储。

## 快速开始

不提供 `config.yaml` 时，FluffCatch 会使用内置 SQLite，数据库默认位于 `data/fluffcatch.db`。首次启动会自动创建表结构；如果管理员密码哈希为空，会生成随机密码，写入配置文件并在终端输出一次。

```bash
go mod tidy
go run ./cmd/fluffcatch
```

默认地址：

- 后端和内置前端静态服务：`http://localhost:8080`
- 健康检查：`http://localhost:8080/api/v1/health`

开发时通常让 Go 后端不接管前端，再另开一个终端运行 Vite：

```bash
go run ./cmd/fluffcatch --frontend-mode external
```

```bash
cd www
npm install
npm run dev
```

- 前端开发服务器：`http://localhost:5173`
- API 前缀：`/api/v1`

## 配置

复制示例配置后可以切换数据库、存储、端口和 OIDC 等配置：

```bash
cp config.example.yaml config.yaml
```

默认配置文件是 `config.yaml`，也可以通过 `--config` 指定：

```bash
go run ./cmd/fluffcatch --config config.production.yaml
```

配置优先级为：内置默认值 < YAML 文件 < 环境变量。详细字段和环境变量见 [docs/configuration.md](docs/configuration.md)。

## 数据库迁移

SQLite 数据库文件不存在或为空时，正常启动会自动执行初始迁移。MySQL 首次部署或升级时建议显式执行迁移：

```bash
go run ./cmd/fluffcatch --migrate
```

指定配置文件时：

```bash
go run ./cmd/fluffcatch --config config.production.yaml --migrate
```

迁移 SQL 会编译进二进制文件，发布包不需要额外携带 `migrations/` 目录。

## 管理员密码

首次迁移或首次启动时，如果 `auth.admin_password_hash` 为空，程序会自动生成管理员密码并在终端输出一次：

```text
username=admin password=...
```

请立刻保存该密码；后续不会再次显示。

忘记密码时可以重置：

```bash
go run ./cmd/fluffcatch --reset-admin-password
```

也可以手动指定新密码：

```bash
go run ./cmd/fluffcatch --reset-admin-password --admin-password "new-password"
```

## 生产构建

```bash
cd www
npm install
npm run build
cd ..
go build -trimpath -buildvcs=false -tags embed_frontend -ldflags "-s -w -X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch ./cmd/fluffcatch
```

Windows 下可以构建为：

```bash
go build -trimpath -buildvcs=false -tags embed_frontend -ldflags "-s -w -X fluffcatch/internal/buildinfo.Mode=release" -o bin/fluffcatch.exe ./cmd/fluffcatch
```

构建完成后，运行 `bin/fluffcatch` 或 `bin/fluffcatch.exe` 即可用一个二进制文件同时提供 API 和内嵌 React 前端。默认 SQLite 数据库、上传文件、Logo 和背景图会落在 `data/` 下；如果改用 MySQL 或外部对象存储，对应数据仍保存在外部服务中。

当前 SQLite 驱动为纯 Go，发布构建也可以设置 `CGO_ENABLED=0` 来获得更好的跨机器可移植性。

## 前端服务模式

开发调试时建议通过命令行参数 `--frontend-mode` 指定；部署时也可以通过环境变量 `FRONTEND_MODE` 覆盖。

| 模式 | 说明 |
| --- | --- |
| `auto` | 优先使用内嵌前端，没有内嵌时读取磁盘静态目录。 |
| `embedded` | 只使用内嵌前端，需要使用 `embed_frontend` 构建标签。 |
| `disk` | 只读取 `FRONTEND_STATIC_ROOT` 指向的静态文件目录。 |
| `external` | 不由 Go 服务提供前端，适合开发时独立运行 Vite。 |
| `disabled` | 完全关闭前端静态服务，只提供 API。 |

## 常用命令

```bash
make api
make web
make build
make test
make migrate
make reset-admin-password
```

`make build` 会先构建前端，再使用 `embed_frontend` 构建标签编译包含内嵌前端的 Go 二进制文件。

## 项目结构

```text
cmd/fluffcatch/      后端启动入口
internal/            后端业务模块
migrations/          数据库迁移
www/                 React 前端
docs/                详细文档
data/                本地运行时数据，默认不提交
```

## 文档

- [docs/README.md](docs/README.md)：文档索引和系统边界。
- [docs/features.md](docs/features.md)：功能说明。
- [docs/configuration.md](docs/configuration.md)：配置、环境变量和部署说明。
- [docs/api.md](docs/api.md)：HTTP API 参考。

## 许可证

本项目采用 [PolyForm Noncommercial License 1.0.0](LICENSE)。允许非商业使用、学习、修改和分发；不得将本项目或其衍生版本用于商业用途或收费服务。

分发原项目、修改版或衍生版本时，必须同时保留许可证文本以及 [NOTICE](NOTICE) 中的 Required Notice，包括原始仓库地址：

```text
https://github.com/Daofengql/FluffCatch
```
