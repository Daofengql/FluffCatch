# API 参考

本文档描述当前实现中的 HTTP API。所有 API 均位于 `/api/v1` 下，媒体访问接口位于 `/media` 下。

## 通用约定

### 响应格式

成功响应按接口返回不同 JSON 对象。错误响应统一为：

```json
{
  "error": "错误说明"
}
```

前端也兼容部分接口返回的 `message` 字段。

### 认证

管理员登录成功后，后端设置 Cookie：

```http
Set-Cookie: fluffcatch_session=<session id>; HttpOnly; SameSite=Lax
```

所有 `/api/v1/admin/*` 接口都需要这个 Cookie。

### 验证码头

以下高风险接口需要额外验证码头：

- `DELETE /api/v1/admin/events/{id}`
- `POST /api/v1/admin/submissions/batch-delete`
- `POST /api/v1/admin/photos/batch-delete`

请求头：

```http
X-Captcha-Id: <captcha id>
X-Captcha-Answer: <captcha answer>
```

验证码通过 `GET /api/v1/auth/captcha` 获取。

### 分页

分页接口通常支持：

| 参数 | 说明 |
| --- | --- |
| `page` | 页码，从 1 开始；非法值按 1 处理。 |
| `pageSize` | 每页数量；非法值使用默认值；最大 100。 |

分页响应通常包含：

```json
{
  "page": 1,
  "pageSize": 24,
  "total": 120,
  "totalPages": 5
}
```

### 时间格式

响应时间一般为 RFC3339 字符串。

活动创建和更新的 `startTime`、`endTime` 支持：

- RFC3339
- `YYYY-MM-DDTHH:mm`
- `YYYY-MM-DD HH:mm:ss`
- `YYYY-MM-DD`

图片 `takenAt` 支持同样格式。

## 数据结构

### Event

```json
{
  "id": 1,
  "title": "活动标题",
  "description": "活动简介",
  "location": "详细地点",
  "provinceCode": "310000",
  "provinceName": "上海市",
  "cityCode": "310100",
  "cityName": "上海市",
  "startTime": "2026-05-18T10:00:00+08:00",
  "endTime": "2026-05-18T18:00:00+08:00",
  "coverPolicyId": "default-local",
  "coverObjectKey": "events/1/cover/hash.jpg",
  "coverUrl": "/media/default-local/events/1/cover/hash.jpg",
  "isPublic": true,
  "submissionEnabled": true,
  "privatePassword": "仅管理员列表可能返回",
  "photoCount": 42,
  "createdAt": "2026-05-18T10:00:00+08:00",
  "updatedAt": "2026-05-18T10:00:00+08:00"
}
```

### Photo

```json
{
  "id": 1,
  "eventId": 1,
  "storagePolicyId": "default-local",
  "objectKey": "events/1/photos/file.jpg",
  "url": "/media/photos/1/original",
  "thumbnailKey": "events/1/thumbs/file.jpg",
  "thumbnailUrl": "/media/photos/1/thumbnail",
  "accessGranted": true,
  "contentHash": "sha256...",
  "contentType": "image/jpeg",
  "sizeBytes": 123456,
  "likeCount": 5,
  "liked": false,
  "photographerName": "摄影师",
  "visibility": "public",
  "tags": [
    {
      "id": 1,
      "name": "#舞台",
      "createdAt": "2026-05-18T10:00:00+08:00"
    }
  ],
  "exif": {},
  "takenAt": "2026-05-18T10:00:00+08:00",
  "createdAt": "2026-05-18T10:00:00+08:00",
  "updatedAt": "2026-05-18T10:00:00+08:00"
}
```

### Submission

```json
{
  "id": 1,
  "eventId": 1,
  "storagePolicyId": "default-local",
  "objectKey": "events/1/submissions/file.jpg",
  "url": "/media/default-local/events/1/submissions/file.jpg",
  "thumbnailKey": "events/1/submissions/thumb.jpg",
  "thumbnailUrl": "/media/default-local/events/1/submissions/thumb.jpg",
  "contentHash": "sha256...",
  "contentType": "image/jpeg",
  "sizeBytes": 123456,
  "photographerName": "摄影师",
  "tags": ["#舞台"],
  "status": "pending",
  "exif": {},
  "takenAt": "2026-05-18T10:00:00+08:00",
  "createdAt": "2026-05-18T10:00:00+08:00"
}
```

### SubmissionLink

```json
{
  "id": 1,
  "eventId": 1,
  "label": "摄影师 A",
  "photographerName": "摄影师 A",
  "token": "仅创建时返回",
  "expiresAt": "2026-05-19T10:00:00+08:00",
  "maxUses": 20,
  "useCount": 3,
  "revokedAt": null,
  "createdAt": "2026-05-18T10:00:00+08:00",
  "updatedAt": "2026-05-18T10:00:00+08:00"
}
```

`maxUses=0` 表示不限次数。

## 健康检查

### GET /health

返回服务、环境、存储策略和数据库状态。

响应：

```json
{
  "status": "ok",
  "service": "FluffCatch",
  "env": "development",
  "storagePolicyId": "default-local",
  "database": "ok"
}
```

`database` 可能为 `ok`、`unhealthy` 或 `not-connected`。

## 认证

### GET /auth/captcha

获取登录或高风险操作验证码。

响应：

```json
{
  "id": "captcha-id",
  "imageSvg": "<svg>...</svg>",
  "expiresAt": "2026-05-18T10:05:00+08:00"
}
```

可能返回：

- `429`：请求过于频繁。

### POST /auth/login

管理员登录。

请求：

```json
{
  "username": "admin",
  "password": "password",
  "captchaId": "captcha-id",
  "captchaAnswer": "abcd"
}
```

成功响应：

```json
{
  "authenticated": true,
  "message": "登录成功",
  "username": "admin"
}
```

失败时可能返回：

- `400`：参数无效、验证码错误或过期。
- `401`：用户名或密码错误。
- `429`：登录尝试过于频繁。

### POST /auth/logout

退出登录并清除会话 Cookie。

响应：

```json
{
  "message": "logged out"
}
```

### GET /auth/me

查询当前会话。

未登录响应：

```json
{
  "authenticated": false
}
```

已登录响应：

```json
{
  "authenticated": true,
  "username": "admin"
}
```

### GET /auth/oidc

查询公开 OIDC 配置。

响应：

```json
{
  "enabled": true,
  "providerName": "Keycloak"
}
```

### GET /auth/oidc/login

创建 OIDC 登录流程并返回授权 URL。

响应：

```json
{
  "url": "https://issuer.example/..."
}
```

### GET /auth/oidc/callback

OIDC 提供商回调接口。该接口不返回 JSON，会重定向到前端：

- 登录流程成功：`/login?oidc_success=...`
- 登录流程失败：`/login?oidc_error=...`
- 绑定流程成功：`/admin/settings/security?oidc_success=...`
- 绑定流程失败：`/admin/settings/security?oidc_error=...`

## 公开站点和活动

### GET /site

获取公开站点设置。

响应是 `SiteSettings`：

```json
{
  "name": "FluffCatch",
  "subtitle": "活动返图收集与画廊",
  "logoUrl": "",
  "homeMarkdown": "",
  "themeMode": "system",
  "themePreset": "blue",
  "themePrimaryColor": "#2563eb",
  "publicBackgroundDesktopUrl": "",
  "publicBackgroundMobileUrl": "",
  "footerSections": [],
  "contactWidgetEnabled": false,
  "contactWidgetTitle": "联系我",
  "contactWidgetHtml": ""
}
```

### GET /settings/upload

获取公开上传限制。

响应：

```json
{
  "maxFileSizeMb": 20,
  "maxVideoSizeMb": 500,
  "maxFilesPerUpload": 20,
  "defaultPageSize": 24,
  "maxConcurrentUploads": 2
}
```

### GET /events

列出公开活动。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 标题关键字，大小写不敏感。 |
| `provinceCode` | 省份代码。 |
| `cityCode` | 城市代码；存在时优先于 `provinceCode`。 |
| `startDate` | 起始日期，格式 `YYYY-MM-DD`。 |
| `endDate` | 结束日期，格式 `YYYY-MM-DD`。 |
| `sort` | `start_desc` 默认倒序；`start_asc`、`time_asc`、`date_asc`、`oldest` 为升序。 |
| `page` | 页码。 |
| `pageSize` | 每页数量，最大 100。 |

响应：

```json
{
  "events": [],
  "page": 1,
  "pageSize": 24,
  "total": 0,
  "totalPages": 0
}
```

### GET /events/{id}

获取公开活动详情。管理员已登录时可以读取非公开活动和私密口令明文字段。

响应：

```json
{
  "event": {}
}
```

可能返回：

- `404`：活动不存在或访客访问非公开活动。

## 公开画廊

### GET /events/{id}/photos

列出活动图片。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `visibility` | `all`、`public`、`private`；默认 `all`。 |
| `tag` | 标签，可传 `#tag` 或 `tag`。 |
| `photographer` | 摄影师名模糊搜索。 |
| `mediaType` | `image` 或 `video`。 |
| `sort` | `latest` 默认最新；`oldest`、`taken_desc`、`taken_asc`、`likes`。 |
| `page` | 页码。 |
| `pageSize` | 每页数量，最大 100。 |

访客只能访问公开活动。私密图片未解锁时 `accessGranted=false`，缩略图会返回受限版本。

响应：

```json
{
  "photos": [],
  "total": 0,
  "page": 1,
  "pageSize": 24,
  "totalPages": 0
}
```

### POST /events/{id}/private-access

用活动私密口令解锁私密图片。

请求：

```json
{
  "password": "private-password"
}
```

成功响应：

```json
{
  "unlocked": true
}
```

口令错误响应状态为 `401`：

```json
{
  "unlocked": false
}
```

成功后设置 `fluffcatch_private_<eventId>` Cookie，有效期约 24 小时。

### POST /photos/{id}/like

点赞公开图片。

响应：

```json
{
  "photoId": 1,
  "likeCount": 6,
  "liked": true,
  "justLiked": true
}
```

可能返回：

- `404`：图片不存在。
- `400`：图片不是公开图片，或所属活动不公开。

## 投稿

### GET /events/{id}/submission-token

校验投稿 token。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `token` | 投稿链接 token。 |

响应：

```json
{
  "valid": true,
  "link": {
    "id": 1,
    "eventId": 1,
    "label": "摄影师 A",
    "photographerName": "摄影师 A",
    "maxUses": 20,
    "useCount": 3,
    "createdAt": "2026-05-18T10:00:00+08:00",
    "updatedAt": "2026-05-18T10:00:00+08:00"
  }
}
```

无效、过期、已撤销、达到使用次数或活动未开启投稿时：

```json
{
  "valid": false,
  "link": {}
}
```

### POST /events/{id}/submissions

上传投稿文件。请求类型为 `multipart/form-data`。

表单字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 单文件模式必填 | 单个上传文件。 |
| `files` | 多文件模式必填 | 多个上传文件，可重复字段名。 |
| `submissionToken` | 访客必填 | 投稿链接 token。管理员登录时可省略。 |
| `photographerName` | 否 | 摄影师名；如果链接绑定了摄影师名，会以后端链接值为准。 |
| `tags` | 否 | 标签字符串，可用逗号、空格、换行分隔。 |
| `visibility` | 管理员可用 | 管理员直接上传正式图片时可指定 `public` 或 `private`。 |

访客上传成功响应：

```json
{
  "submissions": []
}
```

管理员登录状态下上传成功响应：

```json
{
  "photos": []
}
```

限制：

- 图片大小由 `maxFileSizeMb` 控制。
- 视频大小由 `maxVideoSizeMb` 控制。
- 单批文件数由 `maxFilesPerUpload` 控制。
- 请求体有 2GB 硬上限保护。
- 后端并发上传请求有 8 个硬上限。

## 管理后台概览

### GET /admin/dashboard

需要管理员登录。

响应：

```json
{
  "stats": {
    "events": 1,
    "photos": 42,
    "pendingSubmissions": 3,
    "photoBytes": 123456789
  }
}
```

## 管理活动

### GET /admin/events

列出全部活动，包括非公开活动，并包含管理员可见字段。

响应：

```json
{
  "events": []
}
```

### POST /admin/events

创建活动。

请求：

```json
{
  "title": "活动标题",
  "description": "活动简介",
  "location": "详细地点",
  "provinceCode": "310000",
  "provinceName": "上海市",
  "cityCode": "310100",
  "cityName": "上海市",
  "startTime": "2026-05-18T10:00",
  "endTime": "2026-05-18T18:00",
  "coverPolicyId": "",
  "coverObjectKey": "",
  "coverUrl": "",
  "removeCover": false,
  "isPublic": true,
  "submissionEnabled": true,
  "privatePassword": "private-password",
  "clearPrivatePassword": false
}
```

响应状态 `201`：

```json
{
  "event": {}
}
```

校验规则：

- `title` 必填，最长 200 字符。
- `description` 最长 10000 字符。
- `location` 最长 500 字符。
- 省市代码最长 20 字符。

### PUT /admin/events/{id}

更新活动。请求体同创建活动。

额外行为：

- `privatePassword` 为空时默认不修改原私密口令。
- `clearPrivatePassword=true` 会清除私密口令。
- 如果同时传入新的 `privatePassword`，新口令会覆盖清除操作。
- `removeCover=true` 或传入新的 `coverPolicyId`、`coverObjectKey` 会更新封面引用。

响应：

```json
{
  "event": {}
}
```

### DELETE /admin/events/{id}

删除活动。需要管理员登录和验证码头。

响应：

```json
{
  "message": "event deleted",
  "deletedObjects": 10
}
```

删除活动会删除数据库记录，并尝试删除活动封面、正式图片、投稿及缩略图对象。

### POST /admin/events/{id}/cover

上传活动封面。请求类型为 `multipart/form-data`。

表单字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 图片文件。 |

响应：

```json
{
  "policyId": "default-local",
  "objectKey": "events/1/cover/hash.jpg",
  "url": "/media/default-local/events/1/cover/hash.jpg"
}
```

### POST /admin/events/{id}/cover-from-photo

从活动内公开图片设置封面。

请求：

```json
{
  "photoId": 123
}
```

响应：

```json
{
  "policyId": "default-local",
  "objectKey": "events/1/thumbs/hash.jpg",
  "url": "/media/default-local/events/1/thumbs/hash.jpg"
}
```

只有公开图片可作为封面。

## 管理投稿链接

### GET /admin/events/{id}/submission-links

列出活动投稿链接。

响应：

```json
{
  "links": []
}
```

注意：列表不会返回明文 token。

### POST /admin/events/{id}/submission-links

创建投稿链接。

请求：

```json
{
  "label": "摄影师 A",
  "photographerName": "摄影师 A",
  "expiresInHours": 24,
  "maxUses": 20
}
```

字段规则：

- `label` 为空时，优先使用 `photographerName`，否则默认为“投稿链接”。
- `label` 最长 191 字符。
- `photographerName` 最长 191 字符。
- `expiresInHours` 小于 0 会按 0 处理，大于 8760 会被限制为 8760。
- `maxUses` 小于 0 会按 0 处理，大于 100000 会被限制为 100000。

响应状态 `201`：

```json
{
  "link": {
    "id": 1,
    "eventId": 1,
    "label": "摄影师 A",
    "photographerName": "摄影师 A",
    "token": "明文 token 仅此一次返回",
    "expiresAt": "2026-05-19T10:00:00+08:00",
    "maxUses": 20,
    "useCount": 0,
    "createdAt": "2026-05-18T10:00:00+08:00",
    "updatedAt": "2026-05-18T10:00:00+08:00"
  }
}
```

### DELETE /admin/events/{id}/submission-links/{linkID}

撤销投稿链接。

响应：

```json
{
  "message": "submission link revoked"
}
```

### DELETE /admin/events/{id}/submission-links/{linkID}/record

删除已撤销投稿链接记录。只有 `revokedAt` 不为空的链接可以删除。

响应：

```json
{
  "message": "submission link deleted"
}
```

## 管理投稿审核

### GET /admin/submissions

列出所有待审核投稿。

响应：

```json
{
  "submissions": []
}
```

### GET /admin/events/{id}/submissions

列出某个活动的待审核投稿。

响应：

```json
{
  "submissions": []
}
```

### POST /admin/submissions/batch-approve

批量通过投稿。

请求：

```json
{
  "submissionIds": [1, 2, 3],
  "visibility": "public"
}
```

`visibility` 可为 `public` 或 `private`，为空时使用公开。

响应：

```json
{
  "processed": 3,
  "message": "submissions approved"
}
```

### POST /admin/submissions/batch-delete

批量删除待审核投稿。需要验证码头。

请求：

```json
{
  "submissionIds": [1, 2, 3]
}
```

响应：

```json
{
  "processed": 3,
  "message": "submissions deleted"
}
```

## 管理正式图片

### GET /admin/events/{id}/photos

管理员列出活动图片。查询参数同公开画廊接口，但可以看到非公开活动里的全部图片。

响应：

```json
{
  "photos": [],
  "total": 0,
  "page": 1,
  "pageSize": 24,
  "totalPages": 0
}
```

### PUT /admin/photos/{id}

更新单张图片。

请求：

```json
{
  "photographerName": "摄影师",
  "visibility": "public",
  "tags": ["#舞台", "合影"],
  "takenAt": "2026-05-18T10:00"
}
```

说明：

- `visibility` 可为 `public` 或 `private`。
- `tags` 传 `null` 或省略表示不修改标签；传数组表示替换标签。
- 标签会自动补 `#` 并去重。

响应：

```json
{
  "photo": {}
}
```

### DELETE /admin/photos/{id}

删除单张正式图片。

响应：

```json
{
  "message": "photo deleted",
  "deletedObjects": 2
}
```

### POST /admin/photos/batch-delete

批量删除正式图片。需要验证码头。

请求：

```json
{
  "photoIds": [1, 2, 3]
}
```

响应：

```json
{
  "deleted": 3,
  "deletedObjects": 6
}
```

### POST /admin/photos/batch-update

批量更新正式图片。

请求：

```json
{
  "photoIds": [1, 2, 3],
  "visibility": "private",
  "photographerName": "摄影师",
  "tags": ["#舞台"],
  "replaceTags": false
}
```

说明：

- `visibility` 为空表示不修改可见性。
- `photographerName` 省略表示不修改；传空字符串表示清空。
- `replaceTags=true` 会先清空原标签。
- `tags` 非空时会追加到每张图片。

响应：

```json
{
  "affected": 3,
  "message": "updated 3 photos"
}
```

## 管理员安全

### POST /admin/change-password

修改管理员密码。

请求：

```json
{
  "currentPassword": "old-password",
  "newPassword": "new-password"
}
```

响应：

```json
{
  "message": "password changed successfully"
}
```

### GET /admin/oidc/status

查询 OIDC 绑定状态。

响应：

```json
{
  "enabled": true,
  "bound": true,
  "subject": "oidc-subject",
  "providerName": "Keycloak"
}
```

### POST /admin/oidc/bind

创建 OIDC 绑定流程。

响应：

```json
{
  "url": "https://issuer.example/..."
}
```

### DELETE /admin/oidc/bind

解绑 OIDC 账号。

响应：

```json
{
  "message": "oidc account unbound"
}
```

## 管理设置

### GET /admin/settings

获取后台设置和存储策略使用量。

响应：

```json
{
  "settings": {
    "site": {},
    "upload": {},
    "storagePolicies": {
      "activePolicyId": "default-local",
      "policies": []
    }
  },
  "usage": {
    "default-local": {
      "policyId": "default-local",
      "objectCount": 42,
      "sizeBytes": 123456789
    }
  }
}
```

存储策略中的 `secretKey` 会被脱敏为 `***`。

### PUT /admin/settings/site

更新站点设置。Logo 和背景图 URL 会保留当前值，不能通过 JSON 直接覆盖。

请求：

```json
{
  "name": "FluffCatch",
  "subtitle": "活动返图收集与画廊",
  "logoUrl": "",
  "homeMarkdown": "",
  "themeMode": "system",
  "themePreset": "blue",
  "themePrimaryColor": "#2563eb",
  "publicBackgroundDesktopUrl": "",
  "publicBackgroundMobileUrl": "",
  "footerSections": [
    {
      "title": "关于站点",
      "html": "<p>...</p>"
    }
  ],
  "contactWidgetEnabled": false,
  "contactWidgetTitle": "联系我",
  "contactWidgetHtml": ""
}
```

响应：

```json
{
  "site": {},
  "message": "site settings updated"
}
```

### POST /admin/settings/site/logo

上传站点 Logo。请求类型为 `multipart/form-data`。

表单字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 图片文件。 |

响应：

```json
{
  "site": {},
  "url": "/media/default-local/site/logo/hash.png",
  "message": "site logo uploaded"
}
```

### DELETE /admin/settings/site/logo

清除站点 Logo。

响应：

```json
{
  "site": {},
  "message": "site logo cleared"
}
```

### POST /admin/settings/site/background/{variant}

上传公开站点背景图。`variant` 为 `desktop` 或 `mobile`。

请求类型为 `multipart/form-data`，字段 `file` 为图片文件。

响应：

```json
{
  "site": {},
  "url": "/media/default-local/site/backgrounds/desktop/hash.jpg",
  "message": "site background uploaded",
  "width": 1920,
  "height": 1080
}
```

桌面背景会处理为 `1920x1080`，移动背景会处理为 `1080x1920`。

### DELETE /admin/settings/site/background/{variant}

清除公开站点背景图。

响应：

```json
{
  "site": {},
  "message": "site background cleared"
}
```

### PUT /admin/settings/upload

更新上传设置。

请求：

```json
{
  "maxFileSizeMb": 20,
  "maxVideoSizeMb": 500,
  "maxFilesPerUpload": 20,
  "defaultPageSize": 24,
  "maxConcurrentUploads": 2
}
```

响应：

```json
{
  "upload": {},
  "message": "upload settings updated"
}
```

### PUT /admin/settings/storage

更新存储策略。

请求：

```json
{
  "activePolicyId": "default-local",
  "policies": [
    {
      "id": "default-local",
      "name": "默认本地存储",
      "driver": "local",
      "localPath": "data/uploads",
      "publicPrefix": "/media",
      "publicBaseUrl": ""
    }
  ]
}
```

响应：

```json
{
  "storagePolicies": {
    "activePolicyId": "default-local",
    "policies": []
  },
  "usage": {},
  "message": "storage policies updated"
}
```

### POST /admin/settings/storage/test

测试单个存储策略连接。该接口始终返回 `200`，通过 `success` 表示测试结果。

请求体是单个 `StoragePolicy`。

成功响应：

```json
{
  "success": true
}
```

失败响应：

```json
{
  "success": false,
  "error": "upload test failed: ..."
}
```

## 维护接口

### GET /admin/maintenance/storage/orphans

扫描本地存储中没有数据库引用的对象。

响应：

```json
{
  "items": [
    {
      "policyId": "default-local",
      "key": "old/file.jpg",
      "size": 123
    }
  ],
  "total": 1,
  "totalSizeBytes": 123,
  "scannedPolicies": 1,
  "skippedPolicies": 0,
  "truncated": false
}
```

只扫描 `local` 策略；非本地策略计入 `skippedPolicies`。

### GET /admin/maintenance/storage/missing-thumbnails

扫描缺失缩略图的图片和投稿。

响应：

```json
{
  "items": [
    {
      "id": 1,
      "eventId": 1,
      "kind": "photo",
      "storagePolicyId": "default-local",
      "objectKey": "events/1/photos/file.jpg",
      "contentType": "image/jpeg"
    }
  ],
  "total": 1,
  "truncated": false
}
```

`kind` 可能是 `photo` 或 `submission`。最多返回前 200 条，超过时 `truncated=true`。

## 媒体访问

### GET /media/photos/{id}/{variant}

受控访问正式图片。

`variant` 可用值：

- `original`
- `thumbnail`
- `blur`

访问规则：

- 管理员可访问所有图片。
- 访客可访问公开活动里的公开图片。
- 访客输入私密口令后可访问该活动私密图片。
- 未解锁的私密图片缩略图通常返回 `blur` 版本。

### GET /media/{policyID}/{key}

访问本地或后端托管的对象。

说明：

- 本地存储会通过该接口提供对象。
- 外部公开对象存储通常会直接返回 `publicBaseUrl` 拼出的 URL，不一定经过该接口。
