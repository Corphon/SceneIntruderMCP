# SceneIntruderMCP API 文档

本文档描述后端提供的 HTTP API 与 WebSocket 接口（以代码实现为准）。

- REST Base URL：`http://localhost:8080/api`
- WebSocket：`ws://localhost:8080/ws`

## 身份验证

### 登录

通过 `POST /api/auth/login` 获取 Bearer Token。

请求：

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin"
}
```

响应（200）：

```json
{
  "success": true,
  "data": {
    "token": "...",
    "user_id": "admin"
  },
  "message": "登录成功",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

### 使用 Token

请求头携带：

```http
Authorization: Bearer <token>
```

### 访客模式（guest）降级

当未携带或携带无效 `Authorization` 时，多数接口会以访客模式继续执行，并将 `user_id` 视为 `console_user`。

例外：

- `/api/users/:user_id/...` 用户资源接口要求已登录且 `:user_id` 必须与 Token 中的用户一致（访客仅允许访问 `console_user`）。
- 一些敏感操作显式使用 `AuthMiddleware()` 保护。

### 生产环境安全建议

- 生产环境请设置 `AUTH_SECRET_KEY`（至少 32 字节；更长会截断为 32 字节）。
- Token 有效期为 24 小时。

## 限流

`/api` 下启用限流，并会返回以下响应头：

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`（unix 时间戳）

限流策略：

- 默认（所有 `/api/*`）：100 次/分钟/IP
- Chat + interactions：30 次/分钟/用户键（优先取 `X-User-ID`；缺失则回退到 IP）
- upload + analyze：10 次/小时/用户键（优先取 `X-User-ID`；缺失则回退到 IP）

## 通用响应格式

多数 REST 接口使用统一结构：

```json
{
  "success": true,
  "data": {},
  "message": "...",
  "timestamp": "2025-01-01T00:00:00Z",
  "request_id": "..."
}
```

错误响应通常为：

```json
{
  "success": false,
  "error": {
    "code": "BAD_REQUEST",
    "message": "...",
    "details": "..."
  },
  "timestamp": "2025-01-01T00:00:00Z",
  "request_id": "..."
}
```

少数旧路径可能直接返回 `{"error": "..."}`。该行为正在逐步收敛到统一 `APIResponse`。

## REST 接口清单

### Auth

- `POST /api/auth/login`
- `POST /api/auth/logout`

### Settings

- `GET /api/settings`
- `POST /api/settings`
- `POST /api/settings/test-connection`

### LLM

- `GET /api/llm/status`
- `GET /api/llm/models?provider=<provider>`
- `PUT /api/llm/config`

### Scenes

- `GET /api/scenes`
- `POST /api/scenes`
- `POST /api/scenes/shell`
- `GET /api/scenes/:id`
- `DELETE /api/scenes/:id`
- `GET /api/scenes/:id/characters`
- `GET /api/scenes/:id/conversations`
- `GET /api/scenes/:id/nodes/:node_id/content`
- `GET /api/scenes/:id/aggregate`

#### v2 Comics（分镜 / 提示词 / 关键元素）

这些接口用于启动异步任务（返回 202 + `task_id`），并在任务完成后读取落盘结果。进度请订阅 `GET /api/progress/:taskID`（SSE）。

v2.0.0 新增的 standalone comics 典型流程为：

1. 先通过 `POST /api/scenes/shell` 创建一个空的 comics 工作区
2. 前端跳转到 `/scenes/:id/comic?entry=comic_standalone`
3. 再通过 `source_text` 启动分镜分析，而不是依赖已有 story node

- `POST /api/scenes/:id/comic/analysis` — 启动分镜分析（落盘 `data/comics/scene_<id>/analysis.json`）
- `GET /api/scenes/:id/comic/analysis` — 读取分镜分析结果
- `POST /api/scenes/:id/comic/prompts` — 启动每帧提示词生成（落盘 `data/comics/scene_<id>/prompts/*.json`）
- `GET /api/scenes/:id/comic/prompts` — 读取所有帧提示词
- `POST /api/scenes/:id/comic/key_elements` — 启动关键元素提取（落盘 `data/comics/scene_<id>/key_elements.json`）
- `GET /api/scenes/:id/comic/key_elements` — 读取关键元素
- `POST /api/scenes/:id/comic/references` — 上传参考图（multipart；落盘 `data/comics/scene_<id>/references/*` + `references/index.json`）
- `POST /api/scenes/:id/comic/generate` — 启动图片生成（异步；落盘 `data/comics/scene_<id>/images/*.png`）
- `POST /api/scenes/:id/comic/frames/:frameID/regenerate` — 单帧重绘（异步）
- `GET /api/scenes/:id/comic/images/:frameID` — 获取单帧 PNG 图片（用于前端预览；读取 `data/comics/scene_<id>/images/<frameID>.png`）
- `GET /api/scenes/:id/comic` — comics 概览（是否有分析/提示词/关键元素，参考图数量、图片数量等）
- `GET /api/scenes/:id/comic/export?format=zip|html` — 导出下载（ZIP/HTML）

说明：

- 这些路由使用 `RequireAuthForScene()` 保护。
  - 未携带/携带无效 `Authorization` 时，通常会按 guest 模式继续执行。
  - 若携带有效 token，中间件可能会额外校验场景是否存在（未来也可扩展为校验归属/权限）。
- 若希望以“已登录用户”身份调用，可额外添加请求头：`-H "Authorization: Bearer <token>"`

可复制 curl 示例（替换占位符）：

启动分镜分析（返回 202 + `task_id`）：

```bash
curl -sS -X POST \
  -H "Content-Type: application/json" \
  http://localhost:8080/api/scenes/<scene_id>/comic/analysis \
  -d '{}'
```

订阅进度（SSE），直到 `status=completed/failed`（然后服务端会关闭连接）：

```bash
curl -N \
  -H "Accept: text/event-stream" \
  http://localhost:8080/api/progress/<task_id>
```

读取落盘结果：

```bash
curl -sS http://localhost:8080/api/scenes/<scene_id>/comic/analysis

curl -sS http://localhost:8080/api/scenes/<scene_id>/comic/prompts

curl -sS http://localhost:8080/api/scenes/<scene_id>/comic/key_elements
```

启动提示词生成 / 关键元素提取（都返回 202 + `task_id`，进度订阅同上）：

```bash
curl -sS -X POST -H "Content-Type: application/json" \
  http://localhost:8080/api/scenes/<scene_id>/comic/prompts -d '{}'

curl -sS -X POST -H "Content-Type: application/json" \
  http://localhost:8080/api/scenes/<scene_id>/comic/key_elements -d '{}'
```

上传参考图（multipart；仅 PNG/JPEG/WEBP；单文件 <= 5MB）：

单个上传（`element_id` + `file`）：

```bash
curl -sS -X POST \
  -F "element_id=<element_id>" \
  -F "file=@./reference.png" \
  http://localhost:8080/api/scenes/<scene_id>/comic/references
```

批量上传（多个 `file_<element_id>` part）：

```bash
curl -sS -X POST \
  -F "file_<element_id_1>=@./ref1.png" \
  -F "file_<element_id_2>=@./ref2.jpg" \
  http://localhost:8080/api/scenes/<scene_id>/comic/references
```

启动图片生成 / 单帧重绘（都返回 202 + `task_id`）：

```bash
curl -sS -X POST -H "Content-Type: application/json" \
  http://localhost:8080/api/scenes/<scene_id>/comic/generate -d '{}'

curl -sS -X POST -H "Content-Type: application/json" \
  http://localhost:8080/api/scenes/<scene_id>/comic/frames/<frame_id>/regenerate -d '{}'
```

comics 概览：

```bash
curl -sS http://localhost:8080/api/scenes/<scene_id>/comic
```

导出（ZIP/HTML）：

```bash
curl -fSL -o comic_<scene_id>.zip \
  "http://localhost:8080/api/scenes/<scene_id>/comic/export?format=zip"

curl -fSL -o comic_<scene_id>.html \
  "http://localhost:8080/api/scenes/<scene_id>/comic/export?format=html"
```

启动分镜分析：

```http
POST /api/scenes/<scene_id>/comic/analysis
Content-Type: application/json

{}
```

Standalone 文本分析示例：

```http
POST /api/scenes/<scene_id>/comic/analysis
Content-Type: application/json

{
  "source_text": "侦探走进废弃车站，在停摆的时钟下发现了第一条线索……",
  "target_frames": 6
}
```

创建独立 comics scene shell：

```http
POST /api/scenes/shell
Content-Type: application/json
Authorization: Bearer <token>

{
  "title": "My Comic",
  "description": "Standalone comic workspace"
}
```

响应（202）：

```json
{
  "success": true,
  "data": { "task_id": "comic_analyze_<scene_id>_..." },
  "message": "分镜分析任务已受理",
  "timestamp": "..."
}
```

读取分镜分析结果：

```http
GET /api/scenes/<scene_id>/comic/analysis
```

响应（200）：

```json
{
  "success": true,
  "data": {
    "scene_id": "<scene_id>",
    "target_frames": 8,
    "frames": [
      { "id": "frame_1", "order": 1, "description": "..." }
    ]
  },
  "message": "分镜分析结果获取成功",
  "timestamp": "..."
}
```

常见错误码：

- `LLM_NOT_READY`：LLM 未配置或未就绪（启动任务时会直接返回 503）
- `COMIC_SERVICE_NOT_READY`：ComicService/Repo 未初始化或依赖缺失（503）
- `COMIC_ANALYSIS_NOT_FOUND`：分镜分析结果不存在（404）
- `COMIC_PROMPTS_NOT_FOUND`：提示词不存在（404）
- `COMIC_KEY_ELEMENTS_NOT_FOUND`：关键元素不存在（404）

导出 comics：

```http
GET /api/scenes/<scene_id>/comic/export?format=zip
```

响应（200）：文件下载

- `Content-Type`：`application/zip` / `text/html; charset=utf-8`
- `Content-Disposition`：`attachment; filename="comic_<scene_id>_<timestamp>.<ext>"`

### 场景物品

- `GET /api/scenes/:id/items`
- `POST /api/scenes/:id/items`
- `GET /api/scenes/:id/items/:item_id`
- `PUT /api/scenes/:id/items/:item_id`
- `DELETE /api/scenes/:id/items/:item_id`

### Story

- `GET /api/scenes/:id/story`
- `POST /api/scenes/:id/story/choice`
- `POST /api/scenes/:id/story/advance`
- `POST /api/scenes/:id/story/command`
- `POST /api/scenes/:id/story/nodes/:node_id/insert`
- `POST /api/scenes/:id/story/rewind`
- `GET /api/scenes/:id/story/branches`
- `GET /api/scenes/:id/story/choices`
- `POST /api/scenes/:id/story/batch`
- `POST /api/scenes/:id/story/tasks/:task_id/objectives/:objective_id/complete`
- `POST /api/scenes/:id/story/locations/:location_id/unlock`
- `POST /api/scenes/:id/story/locations/:location_id/explore`

### Export

导出接口支持 `?format=`（当前统一为：`json`、`markdown`、`txt`、`html`；comics 导出为 `zip|html`）。

- `GET /api/scenes/:id/export/scene`
- `GET /api/scenes/:id/export/interactions`
- `GET /api/scenes/:id/export/story`

### Chat

- `POST /api/chat`
- `POST /api/chat/emotion`

### Interactions

- `POST /api/interactions/trigger`
- `POST /api/interactions/simulate`
- `POST /api/interactions/aggregate`
- `GET /api/interactions/:scene_id`
- `GET /api/interactions/:scene_id/:character1_id/:character2_id`

### Upload / Analyze / Progress

- `POST /api/upload`
- `POST /api/analyze`
- `GET /api/progress/:taskID`（SSE）
- `POST /api/cancel/:taskID`

#### 进度订阅（SSE）

通过 Server-Sent Events 订阅任务进度：

```http
GET /api/progress/<taskID>
Accept: text/event-stream
```

SSE 事件类型：

- `event: connected`（连接建立时发送一次）
- `event: progress`（JSON payload）
- `event: heartbeat`（保活）

`progress` 事件 payload 结构：

```json
{
  "progress": 42,
  "message": "...",
  "status": "running"
}
```

`status` 取值：`running` / `completed` / `failed`。当状态变为 `completed` 或 `failed` 时，服务端会关闭 SSE 连接。

当任务不存在时，接口会返回普通 JSON 错误（非 SSE）：

```json
{
  "success": false,
  "error": { "code": "TASK_NOT_FOUND", "message": "任务不存在" },
  "timestamp": "..."
}
```

若客户端请求 SSE（例如设置 `Accept: text/event-stream`），服务端会发送一次 `event: progress`（`status="failed"`）后关闭连接，便于前端展示原因并收敛。

常见原因：

- 服务端重启（进度 tracker 目前是内存态）。
- 任务完成/失败后被自动清理。
- taskID 无效。

#### 取消任务

按 taskID 取消任务：

```http
POST /api/cancel/<taskID>
Authorization: Bearer <token>
```

- 若 JobQueue 中存在同 taskID 的运行任务，会尝试向底层任务传播取消。
- ProgressTracker 会标记为失败（"用户取消"），确保 SSE 订阅端能及时收敛。

### Config / Metrics

- `GET /api/config/health`
- `GET /api/config/metrics`

### Users

所有用户接口均位于 `/api/users/:user_id`，并要求 `:user_id` 与已登录用户一致。

- `GET /api/users/:user_id`
- `PUT /api/users/:user_id`
- `GET /api/users/:user_id/preferences`
- `PUT /api/users/:user_id/preferences`
- `GET /api/users/:user_id/items`
- `POST /api/users/:user_id/items`
- `GET /api/users/:user_id/items/:item_id`
- `PUT /api/users/:user_id/items/:item_id`
- `DELETE /api/users/:user_id/items/:item_id`
- `GET /api/users/:user_id/skills`
- `POST /api/users/:user_id/skills`
- `GET /api/users/:user_id/skills/:skill_id`
- `PUT /api/users/:user_id/skills/:skill_id`
- `DELETE /api/users/:user_id/skills/:skill_id`

### WebSocket 管理

- `GET /api/ws/status`
- `POST /api/ws/cleanup`

## 关键请求示例

### 创建场景

```http
POST /api/scenes
Content-Type: application/json
Authorization: Bearer <token>

{
  "title": "My Scene",
  "text": "...source text..."
}
```

### 分析并订阅 SSE 进度

1) 发起分析：

```http
POST /api/analyze
Content-Type: application/json
Authorization: Bearer <token>

{
  "title": "My Scene",
  "text": "...source text..."
}
```

2) 订阅进度（Server-Sent Events）：

```http
GET /api/progress/<taskID>
Accept: text/event-stream
```

SSE 事件类型：

- `event: connected`
- `event: progress`
- `event: heartbeat`

## WebSocket

### 端点

- `GET /ws/scene/:id?user_id=<可选>`
- `GET /ws/user/status?user_id=<必填>`

### 协议说明

后端使用 **原生 WebSocket（Gorilla WebSocket）**，不是 Socket.IO。

消息为 JSON 对象，包含 `type` 字段。

客户端 → 服务端支持的 `type`：

- `character_interaction`
- `story_choice`
- `user_status_update`
- `ping`

服务端 → 客户端常见消息：

- `connected`
- `conversation:new`
- `story:choice_confirmed`
- `user:presence`
- `pong`
- `heartbeat`
- `error`

示例发送（客户端 → 服务端）：
```json
{
  "type": "ping"
}
```

<!--

# SceneIntruderMCP API 文档

<div align="center">

**🎭 AI 驱动的沉浸式互动叙事平台 API 参考**

版本: v1.2.2 | 更新日期: 2025-11-27

[返回主页](../README.md) | [English Version](api.md)

</div>

## 📋 目录

- [概览](#概览)
- [身份验证](#身份验证)
- [通用响应格式](#通用响应格式)
- [错误处理](#错误处理)
- [场景管理 API](#场景管理-api)
- [角色互动 API](#角色互动-api)
- [故事系统 API](#故事系统-api)
- [用户系统 API](#用户系统-api)
- [设置管理 API](#设置管理-api)
- [分析统计 API](#分析统计-api)
- [导出功能 API](#导出功能-api)
- [WebSocket API](#websocket-api)
- [SDK 示例](#sdk-示例)

## 🌟 概览

SceneIntruderMCP API 提供完整的 RESTful 接口，支持：
- 场景创建和管理
- AI 角色互动
- 动态故事分支
- 用户自定义
- 数据导出和分析

### 基本信息

- **Base URL**: `http://localhost:8080/api`
- **API 版本**: v1.1
- **内容类型**: `application/json`
- **字符编码**: UTF-8

## 🔐 身份验证

当前版本使用简单的会话认证。未来版本将支持：
- JWT Token 认证
- API Key 认证
- OAuth 2.0

```http
# 当前版本不需要特殊的认证头
Content-Type: application/json
```

## 📊 通用响应格式

### 成功响应
```json
{
  "success": true,
  "data": {},
  "message": "操作成功",
  "timestamp": "2025-06-27T10:30:00Z"
}
```

### 错误响应
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "请求参数无效",
    "details": "场景名称不能为空"
  },
  "timestamp": "2025-06-27T10:30:00Z"
}
```

## ⚠️ 错误处理

### HTTP 状态码

| 状态码 | 含义 | 描述 |
|--------|------|------|
| 200 | OK | 请求成功 |
| 201 | Created | 资源创建成功 |
| 400 | Bad Request | 请求参数无效 |
| 404 | Not Found | 资源未找到 |
| 500 | Internal Server Error | 服务器内部错误 |

### 错误代码

| 错误代码 | 描述 |
|----------|------|
| `SCENE_NOT_FOUND` | 场景未找到 |
| `CHARACTER_NOT_FOUND` | 角色未找到 |
| `INVALID_TEXT_FORMAT` | 无效的文本格式 |
| `LLM_SERVICE_ERROR` | AI 服务错误 |
| `STORAGE_ERROR` | 存储服务错误 |

---

## 🛠️ API 接口文档

### 🔗 可用的 API 端点

#### 场景管理
```http
GET    /api/scenes                      # 获取场景列表
POST   /api/scenes                      # 创建场景
GET    /api/scenes/{id}                 # 获取场景详情
GET    /api/scenes/{id}/characters      # 获取场景角色
GET    /api/scenes/{id}/conversations   # 获取场景对话
GET    /api/scenes/{id}/aggregate       # 获取场景聚合数据

# 场景物品管理 (新增)
GET    /api/scenes/{id}/items           # 获取场景物品列表
POST   /api/scenes/{id}/items           # 添加物品到场景
GET    /api/scenes/{id}/items/{item_id} # 获取指定物品
PUT    /api/scenes/{id}/items/{item_id} # 更新场景物品
DELETE /api/scenes/{id}/items/{item_id} # 删除场景物品
```

#### 故事系统
```http
GET    /api/scenes/{id}/story           # 获取故事数据
POST   /api/scenes/{id}/story/choice    # 进行故事选择
POST   /api/scenes/{id}/story/advance   # 推进故事
POST   /api/scenes/{id}/story/rewind    # 回溯故事
GET    /api/scenes/{id}/story/branches  # 获取故事分支
GET    /api/scenes/{id}/story/choices   # 获取可用选择 (新增)

# 任务与目标管理 (新增)
POST   /api/scenes/{id}/story/tasks/{task_id}/objectives/{objective_id}/complete
       # 完成任务目标

# 地点管理 (新增)
POST   /api/scenes/{id}/story/locations/{location_id}/unlock
       # 解锁故事地点
POST   /api/scenes/{id}/story/locations/{location_id}/explore
       # 探索故事地点（触发事件、发现物品）
```

#### 导出功能
```http
GET    /api/scenes/{id}/export/scene        # 导出场景数据
GET    /api/scenes/{id}/export/interactions # 导出互动记录
GET    /api/scenes/{id}/export/story        # 导出故事文档
GET    /api/scenes/{id}/export/conversations # 导出对话历史 (新增)
GET    /api/scenes/{id}/export/characters   # 导出角色数据 (新增)
GET    /api/scenes/{id}/export/aggregate    # 导出所有场景数据 (新增)
```

#### 角色互动
```http
POST   /api/chat                        # 基础角色聊天
POST   /api/chat/emotion                # 带情绪分析的聊天
POST   /api/interactions/trigger        # 触发角色互动
POST   /api/interactions/simulate       # 模拟角色对话
POST   /api/interactions/aggregate      # 聚合互动处理
GET    /api/interactions/{scene_id}     # 获取互动历史
GET    /api/interactions/{scene_id}/{character1_id}/{character2_id} # 获取特定角色间互动
```

#### 系统配置与 LLM 管理
```http
GET    /api/settings                    # 获取系统设置
POST   /api/settings                    # 更新系统设置
POST   /api/settings/test-connection    # 测试连接

GET    /api/llm/status                  # 获取 LLM 服务状态
GET    /api/llm/models                  # 获取可用模型
PUT    /api/llm/config                  # 更新 LLM 配置
```

#### 互动聚合
```http
POST   /api/interactions/aggregate      # 处理聚合互动
GET    /api/interactions/{scene_id}     # 获取角色互动
GET    /api/interactions/{scene_id}/{character1_id}/{character2_id} # 获取特定角色间互动
```

#### 场景聚合
```http
GET    /api/scenes/{id}/aggregate       # 获取综合场景数据（含选项）
```

#### 批量操作
```http
POST   /api/scenes/{id}/story/batch      # 批量故事操作
```

#### 配置与健康检查
```http
GET    /api/config/health                # 获取配置健康状态
GET    /api/config/metrics               # 获取配置指标
GET    /api/config/models                # 获取提供商的可用模型
POST   /api/config/test-connection       # 测试提供商连接
```

#### WebSocket 管理
```http
GET    /api/ws/status                    # 获取 WebSocket 连接状态
POST   /api/ws/cleanup                   # 清理过期 WebSocket 连接
```

#### 用户管理系统
```http
# 用户档案
GET    /api/users/{user_id}             # 获取用户档案
PUT    /api/users/{user_id}             # 更新用户档案
GET    /api/users/{user_id}/preferences # 获取用户偏好
PUT    /api/users/{user_id}/preferences # 更新用户偏好

# 用户道具管理
GET    /api/users/{user_id}/items           # 获取用户道具
POST   /api/users/{user_id}/items           # 添加用户道具
GET    /api/users/{user_id}/items/{item_id} # 获取特定道具
PUT    /api/users/{user_id}/items/{item_id} # 更新用户道具
DELETE /api/users/{user_id}/items/{item_id} # 删除用户道具

# 用户技能管理
GET    /api/users/{user_id}/skills           # 获取用户技能
POST   /api/users/{user_id}/skills           # 添加用户技能
GET    /api/users/{user_id}/skills/{skill_id} # 获取特定技能
PUT    /api/users/{user_id}/skills/{skill_id} # 更新用户技能
DELETE /api/users/{user_id}/skills/{skill_id} # 删除用户技能
```

#### WebSocket 支持
```http
WS     /ws/scene/{id}                   # 场景 WebSocket 连接
WS     /ws/user/status                  # 用户状态 WebSocket 连接
```

#### 调试与开发
```http
GET    /api/ws/status                   # 获取 WebSocket 连接状态
```

## 🎬 场景管理 API

### 获取所有场景

获取用户的所有场景列表。

```http
GET /api/scenes
```

**响应示例：**
```json
{
  "success": true,
  "data": [
    {
      "id": "scene_001",
      "name": "神秘城堡",
      "description": "一座充满魔法的古老城堡",
      "type": "fantasy",
      "created_at": "2025-06-27T10:00:00Z",
      "last_updated": "2025-06-27T10:30:00Z",
      "character_count": 5,
      "item_count": 12,
      "story_progress": 65
    }
  ]
}
```

### 创建新场景

通过上传文本创建新的互动场景。

```http
POST /api/scenes
```

**请求体：**
```json
{
  "name": "场景名称",
  "text_content": "小说或故事文本内容...",
  "analysis_type": "auto",
  "creativity_level": "BALANCED",
  "allow_plot_twists": true,
  "preferred_model": "gpt-4"
}
```

### 新建剧本（New Script 写作助手）

```http
POST /api/scripts
Content-Type: application/json
Authorization: Bearer <token>

{
  "title": "我的悬疑小说",
  "type": "novel",
  "framework": {
    "genre": "mystery",
    "chapter_count": 12,
    "notes": "慢热悬疑，共12章"
  }
}
```

**响应示例（201）**

```json
{
  "success": true,
  "data": {
    "id": "script_12345",
    "title": "我的悬疑小说",
    "state": { "active_draft_id": "", "cursor": { "chapter": 1, "scene": 1 } }
  },
  "message": "Script created"
}
```

**显式启动生成**

```http
POST /api/scripts/{id}/generate
Content-Type: application/json
Authorization: Bearer <token>

{}
```

**响应（202）**

```json
{
  "success": true,
  "data": { "task_id": "task_abc123" },
  "message": "Generation started; subscribe to /api/progress/{task_id} for progress (SSE)"
}
```

> 说明：生成过程会向 `/api/progress/:taskID` 发送进度事件（SSE），使用 `GET /api/progress/<taskID>` 并设置 `Accept: text/event-stream` 以接收 `progress` 事件。

**参数描述：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 场景名称 |
| `text_content` | string | 是 | 源文本内容 |
| `analysis_type` | string | 否 | 分析类型：`auto`, `novel`, `script`, `story` |
| `creativity_level` | string | 否 | 创意程度：`STRICT`, `BALANCED`, `EXPANSIVE` |
| `allow_plot_twists` | boolean | 否 | 是否允许剧情转折 |
| `preferred_model` | string | 否 | 首选 AI 模型 |

**响应示例：**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_12345",
    "analysis_result": {
      "characters": [
        {
          "id": "char_001",
          "name": "艾莉亚",
          "description": "勇敢的女骑士",
          "personality": "正义、坚强、富有同情心"
        }
      ],
      "items": [
        {
          "id": "item_001", 
          "name": "魔法剑",
          "description": "散发蓝光的古老魔法剑"
        }
      ],
      "locations": [
        {
          "name": "大厅",
          "description": "宽敞的城堡大厅",
          "accessible": true
        }
      ]
    }
  }
}
```

### 获取场景详情

获取指定场景的详细信息。

```http
GET /api/scenes/{scene_id}
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "scene": {
      "id": "scene_001",
      "name": "神秘城堡",
      "description": "一座充满魔法的古老城堡",
      "created_at": "2025-06-27T10:00:00Z"
    },
    "characters": [...],
    "items": [...],
    "locations": [...],
    "story_data": {...}
  }
}
```

### 获取场景聚合数据

获取场景的完整聚合数据，包括对话历史、故事状态等。

```http
GET /api/scenes/{scene_id}/aggregate?conversation_limit=50&include_story=true&include_ui_state=true
```

**查询参数：**

| 参数 | 类型 | 描述 |
|------|------|------|
| `conversation_limit` | integer | 对话历史限制数量 |
| `include_story` | boolean | 是否包含故事数据 |
| `include_ui_state` | boolean | 是否包含 UI 状态 |

### 删除场景

删除指定场景及其关联的缓存和故事数据。

```http
DELETE /api/scenes/{scene_id}
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_001"
  },
  "message": "场景删除成功"
}
```

> **v1.2.2 更新**：接口会同步删除 `data/stories/<scene_id>` 目录并刷新故事缓存，不再产生残留文件。

---

## 🎭 角色互动 API

### 与角色聊天

与指定角色进行互动聊天。

```http
POST /api/chat
```

**请求体：**
```json
{
  "scene_id": "scene_001",
  "character_id": "char_001", 
  "message": "你好，艾莉亚！你在这里做什么？",
  "include_emotion": true,
  "response_format": "structured"
}
```

**参数描述：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `scene_id` | string | 是 | 场景 ID |
| `character_id` | string | 是 | 角色 ID |
| `message` | string | 是 | 用户消息 |
| `include_emotion` | boolean | 否 | 是否包含情绪分析 |
| `response_format` | string | 否 | 响应格式：`simple`, `structured` |

**响应示例：**
```json
{
  "success": true,
  "data": {
    "character_id": "char_001",
    "character_name": "艾莉亚",
    "message": "你好！我正在探索这座城堡的秘密。这里似乎隐藏着古老的魔法...",
    "emotion": "好奇",
    "action": "警觉地环顾四周",
    "emotion_data": {
      "emotion": "好奇",
      "intensity": 7,
      "body_language": "略微紧张，警觉的姿态",
      "facial_expression": "皱眉，专注的眼神",
      "voice_tone": "谨慎但充满兴趣",
      "secondary_emotions": ["警觉", "坚定"]
    },
    "timestamp": "2025-06-27T10:30:00Z"
  }
}
```

### 获取场景角色列表

获取指定场景中的所有角色。

```http
GET /api/scenes/{scene_id}/characters
```

**响应示例：**
```json
{
  "success": true,
  "data": [
    {
      "id": "char_001",
      "name": "艾莉亚",
      "description": "勇敢的女骑士",
      "personality": "正义、坚强、富有同情心",
      "current_mood": "警觉",
      "energy_level": 85,
      "relationship_status": {
        "player": 50,
        "char_002": 70
      }
    }
  ]
}
```

### 触发角色互动

触发两个或多个角色之间的自动互动。

```http
POST /api/interactions/trigger
```

**请求体：**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002"],
  "topic": "城堡探索计划",
  "context": "两个角色在大厅相遇",
  "interaction_type": "讨论"
}
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "interaction_id": "int_001",
    "participants": ["char_001", "char_002"],
    "dialogue": [
      {
        "character_id": "char_001",
        "character_name": "艾莉亚",
        "message": "我们应该小心地探索这座城堡...",
        "emotion": "谨慎",
        "action": "握紧剑柄"
      },
      {
        "character_id": "char_002", 
        "character_name": "托马斯",
        "message": "同意，我感受到了强大的魔法气息。",
        "emotion": "警觉",
        "action": "赞同地点头"
      }
    ]
  }
}
```

### 模拟角色对话

模拟角色之间的多轮自动对话。

```http
POST /api/interactions/simulate
```

**请求体：**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002", "char_003"],
  "topic": "制定探索策略",
  "rounds": 5,
  "style": "合作"
}
```

### 获取对话历史

获取场景的对话历史记录。

```http
GET /api/scenes/{scene_id}/conversations?limit=50&offset=0&character_id=char_001
```

**查询参数：**

| 参数 | 类型 | 描述 |
|------|------|------|
| `limit` | integer | 返回数量限制 |
| `offset` | integer | 偏移量 |
| `character_id` | string | 筛选特定角色 |

---

## 📖 故事系统 API

### 获取故事数据

获取特定场景的故事数据。

```http
GET /api/scenes/{scene_id}/story
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_001",
    "intro": "欢迎来到神秘的城堡...",
    "main_objective": "探索城堡并揭开其秘密",
    "current_state": {
      "current_node_id": "node_001",
      "current_location": "城堡入口"
    },
    "progress": {
      "completion_percentage": 25,
      "nodes_visited": 5,
      "choices_made": 3
    },
    "nodes": [
      {
        "id": "node_001",
        "title": "入口",
        "content": "你站在巨大的城堡门前...",
        "choices": [
          {
            "id": "choice_001",
            "text": "推开门",
            "consequences": "你进入了主厅"
          }
        ]
      }
    ]
  }
}
```

### 进行故事选择

在故事进程中做出选择。

```http
POST /api/scenes/{scene_id}/story/choice
```

**请求体：**
```json
{
  "node_id": "node_001",
  "choice_id": "choice_001"
}
```

**响应示例：**
```json
{
  "success": true,
  "message": "选择执行成功",
  "next_node": {
    "id": "node_002",
    "title": "主厅",
    "content": "你发现自己身处一个巨大的大厅..."
  },
  "story_data": {
    "current_state": {...},
    "progress": {...}
  }
}
```

### 推进故事

根据当前上下文自动推进故事。

```http
POST /api/scenes/{scene_id}/story/advance
```

**响应示例：**
```json
{
  "success": true,
  "message": "故事已推进",
  "story_update": {
    "title": "新的发现",
    "content": "当你进一步探索时，你发现了...",
    "new_characters": [...],
    "new_items": [...]
  }
}
```

### 回溯故事

将故事回溯到之前的节点。

```http
POST /api/scenes/{scene_id}/story/rewind
```

**请求体：**
```json
{
  "node_id": "node_001"
}
```

### 获取故事分支

获取所有故事分支和路径。

```http
GET /api/scenes/{scene_id}/story/branches
```

---

## ⚙️ 设置管理 API

### 获取系统设置

获取当前系统配置。

```http
GET /api/settings
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "llm_provider": "openai",
    "llm_config": {
      "model": "gpt-4o",
      "has_api_key": true
    },
    "debug_mode": false,
    "port": "8080",

    "vision_provider": "sdwebui",
    "vision_default_model": "sd",
    "vision_config": {
      "endpoint": "http://localhost:7860"
    },
    "vision_model_providers": {
      "sd": "sdwebui",
      "placeholder": "placeholder"
    },
    "vision_models": [
      {
        "key": "sd",
        "label": "Stable Diffusion",
        "provider": "sdwebui",
        "supports_reference_image": true
      },
      {
        "key": "gpt-image-1.5",
        "label": "GPT Image",
        "provider": "openai",
        "supports_reference_image": false
      },
      {
        "key": "glm-image",
        "label": "GLM Image",
        "provider": "glm",
        "supports_reference_image": false
      }
    ]
  }
}
```

### 更新系统设置

更新系统配置（需要管理员权限）。

```http
POST /api/settings
```

**请求体：**
```json
{
  "llm_provider": "openai",
  "llm_config": {
    "api_key": "your-openai-api-key",
    "base_url": "https://api.openai.com/v1",
    "model": "gpt-4o"
  },

  "debug_mode": false,

  "vision_provider": "openai",
  "vision_default_model": "gpt-image-1.5",
  "vision_config": {
    "endpoint": "https://api.openai.com/v1",
    "api_key": "your-openai-api-key",
    "model_gpt-image-1.5": "gpt-image-1",
    "size_gpt-image-1.5": "1024x1024"
  },
  "vision_model_providers": {
    "gpt-image-1.5": "openai"
  },
  "vision_models": [
    {
      "key": "gpt-image-1.5",
      "label": "GPT Image 1.5",
      "provider": "openai",
      "supports_reference_image": false
    }
  ]
}
```

**GLM Image 示例：**
```json
{
  "vision_provider": "glm",
  "vision_default_model": "glm-image",
  "vision_config": {
    "endpoint": "https://open.bigmodel.cn/api/paas/v4",
    "api_key": "your-glm-api-key",
    "model_glm-image": "glm-image",
    "size_glm-image": "1280x1280"
  },
  "vision_model_providers": {
    "glm-image": "glm"
  }
}
```

说明：

- `GET /api/settings` 的 `llm_config` 为摘要形态（`model` + `has_api_key`）。
- `POST /api/settings` 可提交完整的 `llm_config` / `vision_config` 映射；具体 key 随 provider 不同而不同，常见字段包括：
  - `vision_config.endpoint`（或 `base_url`）与 `vision_config.api_key`
  - per-model 覆盖：`vision_config.model_<modelKey>` / `vision_config.size_<modelKey>`
  - 重试开关：`vision_config.max_attempts`（整数字符串，默认 1）
  - PNG 重压缩阈值：`vision_config.png_recompress_threshold_bytes`（整数字符串，默认 262144；设为 0 可禁用）
- 当前支持的 `vision_provider` 包括：`placeholder`、`sdwebui`、`dashscope`、`gemini`、`ark`、`openai`、`glm`。
- 前端会在两个位置消费 `GET /api/settings` 返回的 `vision_models`：comics Step4 模型选择器，以及 Settings 页的 Vision 配置区。
- Settings 页现在会在切换 Vision provider 时自动带入推荐的 `vision_default_model` 与 `vision_config.endpoint`；该行为属于前端交互增强，底层仍对应上述 API 字段。

### 测试连接

测试 AI 服务提供商的连接状态。

```http
POST /api/settings/test-connection
```

该接口用于测试 **LLM** 连通性。请求体可选：

- 不传请求体：测试当前已保存的 LLM 配置是否可用。
- 传入临时配置：服务端会先校验配置，再发起一次小的测试请求。

**请求体（临时配置）：**
```json
{
  "provider": "openai",
  "llm_config": {
    "api_key": "your-openai-api-key",
    "base_url": "https://api.openai.com/v1",
    "model": "gpt-4o"
  }
}
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "provider": "openai",
    "model": "gpt-4",
    "status": "已连接",
    "response_time": 1250,
    "test_message": "连接测试成功"
  }
}
```

---

## 👤 用户系统 API

### 获取用户档案

获取完整的用户档案信息。

```http
GET /api/users/{user_id}
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "id": "user_123",
    "username": "player01",
    "display_name": "冒险玩家",
    "bio": "热爱奇幻冒险",
    "avatar": "avatar_url",
    "created_at": "2025-06-27T10:00:00Z",
    "preferences": {
      "creativity_level": "balanced",
      "language": "zh-cn",
      "auto_save": true
    },
    "items_count": 15,
    "skills_count": 8,
    "saved_scenes": ["scene_001", "scene_002"]
  }
}
```

### 更新用户档案

更新用户档案信息。

```http
PUT /api/users/{user_id}
```

**请求体：**
```json
{
  "display_name": "新的显示名称",
  "bio": "更新的个人简介",
  "preferences": {
    "creativity_level": "expansive",
    "auto_save": false
  }
}
```

### 获取用户偏好

获取用户偏好设置。

```http
GET /api/users/{user_id}/preferences
```

### 更新用户偏好

更新用户偏好设置。

```http
PUT /api/users/{user_id}/preferences
```

**请求体：**
```json
{
  "creativity_level": "balanced",
  "language": "zh-cn",
  "auto_save": true,
  "notification_enabled": true,
  "theme": "dark"
}
```

### 添加用户道具

为用户添加自定义道具。

```http
POST /api/users/{user_id}/items
```

**请求体：**
```json
{
  "name": "魔法水晶",
  "description": "蕴含古老魔法的水晶",
  "rarity": "稀有",
  "effects": [
    {
      "target": "self",
      "type": "mana",
      "value": 20,
      "probability": 1.0,
      "duration": 3600
    },
    {
      "target": "other",
      "type": "mood", 
      "value": 5,
      "probability": 0.6
    }
  ],
  "usage_conditions": ["战斗中", "施法时"],
  "cooldown": 1800
}
```

### 获取用户技能

获取用户的自定义技能。

```http
GET /api/users/{user_id}/skills?category=magic&available_only=true
```

### 添加用户技能

为用户添加自定义技能。

```http
POST /api/users/{user_id}/skills
```

**请求体：**
```json
{
  "name": "心灵感应",
  "description": "读取他人思想的能力",
  "category": "精神",
  "effects": [
    {
      "target": "other",
      "type": "emotion_reveal",
      "value": 100,
      "probability": 0.9
    }
  ],
  "requirements": ["法力值 >= 10", "目标距离 <= 5"],
  "cooldown": 600,
  "mana_cost": 15
}
```

### 更新用户道具

更新指定的用户道具。

```http
PUT /api/users/{user_id}/items/{item_id}
```

### 删除用户道具

删除指定的用户道具。

```http
DELETE /api/users/{user_id}/items/{item_id}
```

## 📤 导出功能 API

### 导出场景数据

以各种格式导出完整的场景数据。

```http
GET /api/scenes/{scene_id}/export/scene?format=json&include_conversations=true
```

**查询参数：**

| 参数 | 类型 | 描述 |
|------|------|------|
| `format` | string | 导出格式：`json`, `markdown`, `txt`, `html` |
| `include_conversations` | boolean | 是否包含对话历史 |

**响应示例：**
```json
{
  "file_path": "/exports/scene_001_20250627.json",
  "content": "...",
  "format": "json",
  "size": 2048,
  "timestamp": "2025-06-27T10:30:00Z"
}
```

### 导出互动记录

导出互动摘要。

支持格式：`json`, `markdown`, `txt`, `html`。

```http
GET /api/scenes/{scene_id}/export/interactions?format=markdown
```

### 导出故事文档

将故事导出为可读文档。

支持格式：`json`, `markdown`, `txt`, `html`。

```http
GET /api/scenes/{scene_id}/export/story?format=html
```

## 🔄 WebSocket API

### 场景 WebSocket 连接

连接到场景以获取实时更新。

```javascript
// 连接到场景 WebSocket
const ws = new WebSocket('ws://localhost:8080/ws/scene/scene_001?user_id=user123');

// 监听消息
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('收到消息:', data);
};

// 发送角色互动
ws.send(JSON.stringify({
    type: 'character_interaction',
    character_id: 'char_001',
    message: '大家好！'
}));
```

**消息类型：**

| 类型 | 描述 | 方向 |
|------|------|------|
| `character_interaction` | 角色消息 | 客户端 → 服务器 |
| `story_choice` | 故事选择 | 客户端 → 服务器 |
| `user_status_update` | 用户状态更新 | 客户端 → 服务器 |
| `conversation:new` | 新对话 | 服务器 → 客户端 |
| `story:choice_made` | 故事选择结果 | 服务器 → 客户端 |
| `user:presence` | 用户在线状态更新 | 服务器 → 客户端 |

### 用户状态 WebSocket

连接以获取用户状态更新。

```javascript
const statusWs = new WebSocket('ws://localhost:8080/ws/user/status?user_id=user123');
```

---

## 🛠️ SDK 示例

### JavaScript SDK（增强版）

```javascript
class SceneIntruderAPI {
    constructor(baseUrl = 'http://localhost:8080/api') {
        this.baseUrl = baseUrl;
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseUrl}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        if (options.body && typeof options.body === 'object') {
            config.body = JSON.stringify(options.body);
        }

        const response = await fetch(url, config);
        return response.json();
    }

    // 故事系统 API
    async getStoryData(sceneId) {
        return this.request(`/scenes/${sceneId}/story`);
    }

    async makeStoryChoice(sceneId, nodeId, choiceId) {
        return this.request(`/scenes/${sceneId}/story/choice`, {
            method: 'POST',
            body: { node_id: nodeId, choice_id: choiceId }
        });
    }

    async advanceStory(sceneId) {
        return this.request(`/scenes/${sceneId}/story/advance`, {
            method: 'POST'
        });
    }

    // 导出 API
    async exportSceneData(sceneId, format = 'json', includeConversations = false) {
        return this.request(`/scenes/${sceneId}/export/scene?format=${format}&include_conversations=${includeConversations}`);
    }

    // 用户管理 API
    async getUserProfile(userId) {
        return this.request(`/users/${userId}`);
    }

    async updateUserProfile(userId, profileData) {
        return this.request(`/users/${userId}`, {
            method: 'PUT',
            body: profileData
        });
    }

    async getUserPreferences(userId) {
        return this.request(`/users/${userId}/preferences`);
    }

    // WebSocket 连接助手
    connectToScene(sceneId, userId) {
        const ws = new WebSocket(`ws://localhost:8080/ws/scene/${sceneId}?user_id=${userId}`);
        
        ws.onopen = () => console.log('已连接到场景 WebSocket');
        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            this.handleWebSocketMessage(data);
        };
        
        return ws;
    }

    handleWebSocketMessage(data) {
        switch (data.type) {
            case 'conversation:new':
                this.onNewConversation(data);
                break;
            case 'story:choice_made':
                this.onStoryChoiceMade(data);
                break;
            case 'user:presence':
                this.onUserPresenceUpdate(data);
                break;
        }
    }

    // 事件处理器（由使用者实现）
    onNewConversation(data) {
        console.log('新对话:', data);
    }

    onStoryChoiceMade(data) {
        console.log('故事选择已做出:', data);
    }

    onUserPresenceUpdate(data) {
        console.log('用户在线状态更新:', data);
    }
}

// 使用示例
const api = new SceneIntruderAPI();

// 创建场景
const scene = await api.request('/scenes', {
    method: 'POST',
    body: {
        name: "测试场景",
        text_content: "这是一个测试故事...",
        creativity_level: "BALANCED"
    }
});

// 与角色聊天
const chatResponse = await api.request('/chat', {
    method: 'POST',
    body: {
        scene_id: scene.data.scene_id,
        character_id: "char_001",
        message: "你好，很高兴见到你！",
        include_emotion: true
    }
});

console.log(`角色回复: ${chatResponse.data.message}`);
console.log(`情绪状态: ${chatResponse.data.emotion}`);
```

### Python SDK

```python
import requests
import json
from typing import Dict, Any, Optional

class SceneIntruderAPI:
    def __init__(self, base_url: str = "http://localhost:8080/api"):
        self.base_url = base_url
        self.session = requests.Session()
        
    def request(self, endpoint: str, method: str = "GET", 
                data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        url = f"{self.base_url}{endpoint}"
        headers = {"Content-Type": "application/json"}
        
        kwargs = {"headers": headers}
        if data:
            kwargs["json"] = data
            
        response = self.session.request(method, url, **kwargs)
        response.raise_for_status()
        
        return response.json()
    
    # 场景管理
    def create_scene(self, name: str, text_content: str, **kwargs) -> Dict[str, Any]:
        data = {
            "name": name,
            "text_content": text_content,
            **kwargs
        }
        return self.request("/scenes", "POST", data)
    
    def get_scene(self, scene_id: str) -> Dict[str, Any]:
        return self.request(f"/scenes/{scene_id}")
    
    # 角色互动
    def chat_with_character(self, scene_id: str, character_id: str, 
                          message: str) -> Dict[str, Any]:
        data = {
            "scene_id": scene_id,
            "character_id": character_id,
            "message": message,
            "include_emotion": True
        }
        return self.request("/chat", "POST", data)

    # 用户道具
    def get_user_items(self, user_id: str) -> Dict[str, Any]:
        return self.request(f"/users/{user_id}/items")
    
    def add_user_item(self, user_id: str, item_data: Dict[str, Any]) -> Dict[str, Any]:
        return self.request(f"/users/{user_id}/items", "POST", item_data)

    # 故事系统
    def get_story_data(self, scene_id: str) -> Dict[str, Any]:
        return self.request(f"/scenes/{scene_id}/story")
    
    def make_story_choice(self, scene_id: str, node_id: str, choice_id: str) -> Dict[str, Any]:
        data = {
            "node_id": node_id,
            "choice_id": choice_id
        }
        return self.request(f"/scenes/{scene_id}/story/choice", "POST", data)

# 使用示例
api = SceneIntruderAPI()

# 创建场景
scene = api.create_scene(
    name="Python 测试场景",
    text_content="这是一个测试故事...",
    creativity_level="BALANCED"
)

# 与角色聊天
response = api.chat_with_character(
    scene["data"]["scene_id"],
    "char_001",
    "你好，很高兴见到你！"
)

print(f"角色回复: {response['data']['message']}")
print(f"情绪状态: {response['data']['emotion']}")

# 获取故事数据
story_data = api.get_story_data(scene["data"]["scene_id"])
print(f"故事介绍: {story_data['data']['intro']}")
```

### cURL 示例

```bash
# 创建场景
curl -X POST http://localhost:8080/api/scenes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "奇幻冒险",
    "text_content": "在一座古老的城堡里，年轻的巫师们正在学习魔法...",
    "creativity_level": "EXPANSIVE"
  }'

# 与角色聊天
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "scene_id": "scene_123",
    "character_id": "char_001",
    "message": "教授，今天的魔法课是什么内容？",
    "include_emotion": true
  }'

# 获取用户道具
curl -X GET "http://localhost:8080/api/users/user_123/items?category=weapon"

# 添加用户道具
curl -X POST http://localhost:8080/api/users/user_123/items \
  -H "Content-Type: application/json" \
  -d '{
    "name": "烈焰之剑",
    "description": "燃烧着永恒火焰的剑",
    "category": "weapon",
    "rarity": "传说"
  }'

# 故事系统示例
# 获取故事数据
curl -X GET http://localhost:8080/api/scenes/scene_001/story

# 进行故事选择
curl -X POST http://localhost:8080/api/scenes/scene_001/story/choice \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node_001",
    "choice_id": "choice_a"
  }'

# 导出场景数据
curl -X GET "http://localhost:8080/api/scenes/scene_001/export/scene?format=markdown&include_conversations=true"

# 用户档案管理
# 获取用户档案
curl -X GET http://localhost:8080/api/users/user_123

# 更新用户偏好
curl -X PUT http://localhost:8080/api/users/user_123/preferences \
  -H "Content-Type: application/json" \
  -d '{
    "creativity_level": "balanced",
    "language": "zh-cn",
    "auto_save": true
  }'

# LLM 配置
# 获取 LLM 状态
curl -X GET http://localhost:8080/api/llm/status

# 更新 LLM 配置
curl -X PUT http://localhost:8080/api/llm/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "config": {
      "api_key": "your-api-key",
      "model": "gpt-4"
    }
  }'
```

---

## 📋 API 更新日志

### v1.2.2 (2025-11-27) - 当前版本
- **改进**: `DELETE /api/scenes/{id}` 会自动删除 `data/stories/<scene_id>` 并刷新缓存，避免残留数据
- **修复**: GitHub Models 等提供商严格遵循配置中的 `default_model`，彻底解决 “connection failed” 问题
- **文档**: 新增 `data/.encryption_key` 说明与预发布数据清理指引，提升上线准备效率

### v1.1.0 (2025-06-27)
- **新增**: 故事系统 API，支持交互式叙事
- **新增**: 导出功能，支持场景、互动和故事导出
- **新增**: WebSocket 支持，实现实时通信
- **新增**: 完整的用户档案和偏好管理
- **新增**: LLM 配置和状态管理 API
- **增强**: 用户道具和技能管理系统
- **改进**: 错误处理和响应格式标准化

### v1.0.0 (2025-06-20)
- 初始 API 版本发布
- 支持场景创建和管理
- 角色互动系统实现
- 故事分支功能添加
- 多 LLM 提供商支持集成

---

## 🔗 相关链接

- [主要文档](../README.md)
- [部署指南](deployment.md)
- [GitHub 仓库](https://github.com/Corphon/SceneIntruderMCP)
- [问题报告](https://github.com/Corphon/SceneIntruderMCP/issues)

---

<div align="center">

**📚 需要帮助？欢迎查看我们的文档或提交问题！**

由 SceneIntruderMCP 团队用 ❤️ 制作

</div>

-->
