# SceneIntruderMCP

<div align="center">

![SceneIntruderMCP Logo](temp/logo.png)

**面向场景、漫画、视频与剧本的一体化 AI 原生创作工作台**

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-apache-green.svg)](LICENSE)

[English](README.md) | 简体中文

</div>

## 项目现在的定位

SceneIntruderMCP 已不再是最初那种“简单分析文本并搭个页面”的轻量样板，而是一个统一的创作平台。当前版本由一个 Go 后端和一个 React SPA 组成，串起四条主链路：

1. **互动场景**：文本分析、角色/物品/上下文抽取、剧情推进与回溯。
2. **Comics Studio**：5 步工作流，覆盖分镜、提示词、关键元素、参考图、出图与导出。
3. **Video Studio**：基于 comics 结果构建 timeline、异步生成 clip、查看恢复态并导出。
4. **New Script**：创建剧本项目、生成首稿、继续改写章节并导出。

此外，系统已经把 **LLM / Vision / Video provider 配置**、**SSE 长任务进度**、**文件系统持久化** 收敛到统一运行时里。

## 当前能力全景

### 后端与运行时

- Go + Gin 服务端
- 同一二进制同时提供 API 与前端 SPA
- 统一配置文件 `data/config.json`
- API 凭据加密存储（`data/.encryption_key` 或 `CONFIG_ENCRYPTION_KEY`）
- SSE 进度订阅：`GET /api/progress/:taskID`
- 原生 WebSocket 场景/用户通道
- 基于文件系统的 scenes / stories / comics / scripts / exports / users 存储

### 前端工作区

- `/` —— scenes 首页
- `/settings` —— LLM / Vision / Video 设置
- `/scenes/:id` —— scene 详情
- `/scenes/:id/story` —— story 模式
- `/scenes/:id/comic` —— Comics Studio
- `/scenes/:id/comic/video` —— Video Studio
- `/scripts` / `/scripts/:id` —— Script 工作区

### 当前后端已正式接线的 LLM provider

后端当前已注册：

- `openai`
- `anthropic`
- `google`
- `deepseek`
- `qwen`
- `mistral`
- `grok`
- `glm`
- `githubmodels`
- `openrouter`
- `nvidia`

说明：

- 为避免结构化分析被 think / reasoning 污染，LLM 层现在默认关闭推理模式。
- 支持 provider 原生关闭时会显式关闭；不支持时会回退到更安全的非 reasoning 模型。
- Google、Qwen、NVIDIA 已做 provider 级默认抑制。
- 前端 Settings 仍保留 `ollama` 选项，但它 **不在当前后端正式支持矩阵内**，因此本文档不把它列为可用 provider。

### Vision provider

- `placeholder`
- `sdwebui`
- `dashscope`
- `gemini`
- `ark`
- `openai`
- `glm`

前端实际使用的模型列表来自 `GET /api/settings` 返回的：

- `vision_default_model`
- `vision_models`
- `vision_model_providers`

### Video provider

- `dashscope`
- `kling`
- `google`
- `vertex`
- `ark`
- `mock`

对应默认模型与模型路由同样通过 `GET /api/settings` 下发：

- `video_default_model`
- `video_models`
- `video_model_providers`

## 架构概览

```text
frontend (React + Vite + MUI)
                │
                ├─ REST (/api/*)
                ├─ SSE  (/api/progress/:taskID)
                └─ WS   (/ws/*)
                                │
backend (Go + Gin)
                │
                ├─ config / auth / rate limit / handlers
                ├─ LLM / Vision / Video / Story / Script / Comic services
                └─ file storage under data/
```

核心目录：

- `cmd/server` —— 启动入口
- `internal/api` —— 路由、handler、中间件
- `internal/app` —— 应用装配与 provider 注册
- `internal/config` —— 配置与加密逻辑
- `internal/llm` —— LLM 抽象与 reasoning 控制
- `internal/services` —— 业务服务
- `internal/vision` —— Vision provider
- `frontend` —— SPA 前端

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+
- npm 9+
- 至少一组可用 provider 凭据

### 1. 安装依赖

```bash
go mod download
cd frontend
npm install
cd ..
```

### 2. 构建前端资源

```bash
cd frontend
npm run build
cd ..
```

### 3. 启动服务

```bash
go run ./cmd/server
```

默认地址：`http://localhost:8080`

### 4. 打开应用

- 首页：`http://localhost:8080/`
- 设置页：`http://localhost:8080/settings`

## 配置模型

运行时配置持久化在 `data/config.json`。

最关键的字段：

- `llm_provider`、`llm_config`
- `vision_provider`、`vision_default_model`、`vision_config`
- `video_provider`、`video_default_model`、`video_config`
- `vision_models`、`vision_model_providers`
- `video_models`、`video_model_providers`

示例：

```json
{
    "port": "8080",
    "data_dir": "data",
    "static_dir": "frontend/dist/assets",
    "templates_dir": "frontend/dist",
    "log_dir": "logs",
    "debug_mode": true,
    "llm_provider": "nvidia",
    "llm_config": {
        "api_key": "",
        "base_url": "https://integrate.api.nvidia.com/v1",
        "default_model": "moonshotai/kimi-k2.5"
    },
    "vision_provider": "glm",
    "vision_default_model": "glm-image",
    "vision_config": {
        "endpoint": "https://open.bigmodel.cn/api/paas/v4",
        "api_key": ""
    },
    "video_provider": "dashscope",
    "video_default_model": "wan2.6-i2v-flash",
    "video_config": {
        "endpoint": "https://dashscope.aliyuncs.com/api/v1",
        "api_key": "",
        "public_base_url": "https://your-domain.example"
    }
}
```

### 加密说明

- 开发模式下如果未设置 `CONFIG_ENCRYPTION_KEY`，系统会自动生成 `data/.encryption_key`。
- 该文件必须和 `data/config.json` 一起保留。
- 删除密钥文件后，旧的加密凭据将无法解密。

## 推荐首次使用路径

1. 先构建前端资源。
2. 启动后端。
3. 在 Settings 里先配好一个可用的 LLM provider。
4. 按需配置 Vision / Video provider。
5. 创建 scene 或直接创建 standalone comic 工作区。

## 关键运行特性

### 长任务模式

分析、提示词生成、出图、剧本生成、视频生成均为异步任务，通常流程为：

1. 发起任务并拿到 `task_id`
2. 订阅 `GET /api/progress/:taskID`
3. 再读取对应结果接口

### 访客与登录用户

- 许多 scene 相关接口在缺少有效授权时会降级到 `console_user`。
- `/api/users/:user_id/...` 这类用户资源接口要求登录并校验归属。
- scripts 路由要求登录。

### Video provider 注意事项

部分视频 provider 需要公网可访问的参考图 URL，因此部署环境通常需要正确配置 `video_config.public_base_url`。

## 常用开发命令

后端：

```bash
go test ./...
go run ./cmd/server
```

前端：

```bash
cd frontend
npm run dev
npm test
npm run lint
npm run build
```

## 文档索引

- [API 文档](docs/api_cn.md)
- [部署文档](docs/deployment_cn.md)
- [前端开发文档](docs/frontend_dev.md)
- [English API](docs/api.md)
- [English deployment](docs/deployment.md)

## 当前项目边界

本仓库现在应被视为：

- 一个**多工作区创作平台**，
- 带有**可配置 AI provider**，
- 拥有**异步任务 / SSE / 恢复态**，
- 并且具备**可部署、可运维、可持续扩展**的文档与运行约束。

后续任何改动，都应以这个真实产品形态为基线，而不是回退到最初的简化想象。

<!--
## 🛠️ API 接口文档

### 🔗 实际可用的 API 端点

#### 场景管理
```http
GET    /api/scenes                      # 获取场景列表
POST   /api/scenes                      # 创建场景  
POST   /api/scenes/shell                # 创建独立 comics 工作区 shell
GET    /api/scenes/{id}                 # 获取场景详情
GET    /api/scenes/{id}/characters      # 获取场景角色
GET    /api/scenes/{id}/conversations   # 获取场景对话
GET    /api/scenes/{id}/aggregate       # 获取场景聚合数据
```

#### Comics v2
```http
POST   /api/scenes/{id}/comic/analysis                 # 启动分镜分析（支持 source_text）
GET    /api/scenes/{id}/comic/analysis                 # 获取分镜分析结果
POST   /api/scenes/{id}/comic/prompts                  # 启动逐帧提示词生成
GET    /api/scenes/{id}/comic/prompts                  # 获取所有帧提示词
POST   /api/scenes/{id}/comic/key_elements             # 启动关键元素提取
GET    /api/scenes/{id}/comic/key_elements             # 获取关键元素
POST   /api/scenes/{id}/comic/references               # 上传参考图
POST   /api/scenes/{id}/comic/generate                 # 启动图片生成
POST   /api/scenes/{id}/comic/frames/{frame_id}/regenerate # 单帧重绘
GET    /api/scenes/{id}/comic/images/{frame_id}        # 获取生成 PNG
GET    /api/scenes/{id}/comic                          # 获取 comics 概览
GET    /api/scenes/{id}/comic/export?format=zip|html   # 导出 comics
```

#### 剧本（Scripts）
```http
GET    /api/scripts                    # 列出剧本项目
POST   /api/scripts                    # 创建剧本项目
GET    /api/scripts/{id}               # 获取剧本详情
POST   /api/scripts/{id}/generate      # 启动初始生成
POST   /api/scripts/{id}/command       # 执行辅助指令
PUT    /api/scripts/{id}/chapter_draft # 保存章节草稿
PUT    /api/scripts/{id}/draft         # 保存/替换活动草稿
POST   /api/scripts/{id}/rewind        # 回滚到历史草稿
GET    /api/scripts/{id}/export        # 导出剧本
```

#### 故事系统
```http
GET    /api/scenes/{id}/story           # 获取故事数据
POST   /api/scenes/{id}/story/choice    # 进行故事选择
POST   /api/scenes/{id}/story/advance   # 推进故事情节
POST   /api/scenes/{id}/story/rewind    # 回溯故事
GET    /api/scenes/{id}/story/branches  # 获取故事分支
POST   /api/scenes/{id}/story/rewind    # 回溯到指定故事节点
```

#### 导出功能
```http
GET    /api/scenes/{id}/export/scene        # 导出场景数据
GET    /api/scenes/{id}/export/interactions # 导出互动记录
GET    /api/scenes/{id}/export/story        # 导出故事文档
```

#### 互动聚合
```http
POST   /api/interactions/aggregate         # 处理聚合互动
GET    /api/interactions/{scene_id}        # 获取角色互动
GET    /api/interactions/{scene_id}/{character1_id}/{character2_id} # 获取角色间互动
```

#### 场景聚合
```http
GET    /api/scenes/{id}/aggregate          # 获取综合场景数据（含选项）
```

#### 批量操作
```http
POST   /api/scenes/{id}/story/batch        # 批量故事操作
```

#### 用户管理
```http
GET    /api/users/{user_id}                # 获取用户档案
PUT    /api/users/{user_id}                # 更新用户档案
GET    /api/users/{user_id}/preferences    # 获取用户偏好
PUT    /api/users/{user_id}/preferences    # 更新用户偏好
```

#### 用户道具和技能管理
```http
# 用户道具
GET    /api/users/{user_id}/items           # 获取用户道具
POST   /api/users/{user_id}/items           # 添加用户道具
GET    /api/users/{user_id}/items/{item_id} # 获取特定道具
PUT    /api/users/{user_id}/items/{item_id} # 更新用户道具
DELETE /api/users/{user_id}/items/{item_id} # 删除用户道具

# 用户技能
GET    /api/users/{user_id}/skills           # 获取用户技能
POST   /api/users/{user_id}/skills           # 添加用户技能
GET    /api/users/{user_id}/skills/{skill_id} # 获取特定技能
PUT    /api/users/{user_id}/skills/{skill_id} # 更新用户技能
DELETE /api/users/{user_id}/skills/{skill_id} # 删除用户技能
```

#### 配置和健康检查
```http
GET    /api/config/health                   # 获取配置健康状态
GET    /api/config/metrics                  # 获取配置指标
GET    /api/settings                        # 获取系统设置
POST   /api/settings                        # 更新系统设置
POST   /api/settings/test-connection        # 测试连接
```

#### WebSocket 管理
```http
GET    /api/ws/status                       # 获取 WebSocket 连接状态
POST   /api/ws/cleanup                      # 清理过期 WebSocket 连接
```

#### 文本分析与文件上传
```http
POST   /api/analyze                     # 分析文本内容
GET    /api/progress/{taskID}           # 获取分析进度
POST   /api/cancel/{taskID}             # 取消分析任务
POST   /api/upload                      # 上传文件
```

#### 角色互动与聊天
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

### 📋 **API 使用示例**

#### 故事互动流程
```javascript
// 1. 获取故事数据
const storyData = await fetch('/api/scenes/scene123/story');

// 2. 进行故事选择
const choiceResult = await fetch('/api/scenes/scene123/story/choice', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        node_id: 'node_1',
        choice_id: 'choice_a'
    })
});

// 3. 导出故事
const storyExport = await fetch('/api/scenes/scene123/export/story?format=markdown');
```

#### 角色互动
```javascript
// 1. 基础聊天
const chatResponse = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        scene_id: 'scene123',
        character_id: 'char456',
        message: '你好，最近怎么样？'
    })
});

// 2. 触发角色互动
const interaction = await fetch('/api/interactions/trigger', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        scene_id: 'scene123',
        character_ids: ['char1', 'char2'],
        topic: '讨论神秘的古老文物'
    })
});
```

#### 用户自定义
```javascript
// 1. 添加自定义道具
const newItem = await fetch('/api/users/user123/items', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        name: '魔法剑',
        description: '一把拥有神秘力量的传奇之剑',
        type: 'weapon',
        properties: { attack: 50, magic: 30 }
    })
});

// 2. 添加技能
const newSkill = await fetch('/api/users/user123/skills', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        name: '火球术',
        description: '释放强力火球魔法',
        type: 'magic',
        level: 3
    })
});
```

### 🔗 **WebSocket 集成**

#### 场景 WebSocket 连接
```javascript
// 连接到场景 WebSocket
const sceneWs = new WebSocket(`ws://localhost:8080/ws/scene/scene123?user_id=user456`);

sceneWs.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('场景更新:', data);
};

// 发送角色互动
sceneWs.send(JSON.stringify({
    type: 'character_interaction',
    character_id: 'char123',
    message: '大家好！'
}));

// 发送故事选择
sceneWs.send(JSON.stringify({
    type: 'story_choice',
    node_id: 'story_node_1',
    choice_id: 'choice_a',
    user_preferences: {
        creativity_level: 'balanced',
        allow_plot_twists: true
    }
}));
```

#### 用户状态 WebSocket
```javascript
// 连接到用户状态 WebSocket
const statusWs = new WebSocket(`ws://localhost:8080/ws/user/status?user_id=user456`);

statusWs.onmessage = (event) => {
    const data = JSON.parse(event.data);
    switch(data.type) {
        case 'heartbeat':
            console.log('连接保持活跃');
            break;
        case 'user_status_update':
            console.log('用户状态改变:', data.status);
            break;
        case 'error':
            console.error('WebSocket错误:', data.error);
            break;
        default:
            console.log('接收到:', data);
    }
};
```

#### 支持的 WebSocket 消息类型
- **character_interaction**: 角色间互动
- **story_choice**: 故事决策事件
- **user_status_update**: 用户在线状态和状态更新
- **conversation:new**: 新对话事件
- **heartbeat**: 连接健康检查
- **pong**: 心跳响应消息
- **error**: 错误通知

#### 前端实时管理
应用程序使用 RealtimeManager 类处理 WebSocket 通信：
```javascript
// 初始化场景实时功能
await window.realtimeManager.initSceneRealtime('scene_123');

// 发送角色互动
window.realtimeManager.sendCharacterInteraction('scene_123', 'character_456', '你好！');

// 订阅故事事件
window.realtimeManager.on('story:event', (data) => {
    // 处理故事更新
    console.log('故事事件:', data);
});

// 获取连接状态
const status = window.realtimeManager.getConnectionStatus();
console.log('WebSocket状态:', status);
```

### 📊 **响应格式**

#### 标准成功响应
```json
{
    "success": true,
    "data": {
        // 响应数据
    },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

#### 错误响应
```json
{
    "success": false,
    "error": "错误信息描述",
    "code": "ERROR_CODE",
    "timestamp": "2024-01-01T12:00:00Z"
}
```

#### 导出响应
```json
{
    "file_path": "/exports/story_20240101_120000.md",
    "content": "# 故事导出\n\n...",
    "format": "markdown",
    "size": 1024,
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### 🛡️ **身份验证与安全**

当前 API 使用基于会话的身份验证进行用户管理。对于生产环境部署，建议实施：

- **JWT 身份验证**：基于令牌的 API 访问认证
- **频率限制**：API 调用频次限制
- **输入验证**：严格的参数验证和清理
- **仅 HTTPS**：生产环境强制使用 HTTPS

详细的 API 文档，请参见：[API 文档](docs/api.md)

### 🎯 **请求参数说明**

#### 故事选择参数
```javascript
{
    "node_id": "string",      // 当前故事节点ID
    "choice_id": "string",    // 选择的选项ID
    "user_preferences": {     // 可选：用户偏好设置
        "creativity": "balanced",  // 创意度：strict|balanced|expansive
        "language": "zh-cn"        // 语言偏好
    }
}
```

#### 角色互动参数
```javascript
{
    "scene_id": "string",          // 场景ID
    "character_ids": ["string"],   // 参与互动的角色ID列表
    "topic": "string",             // 互动主题
    "context": "string",           // 可选：互动背景
    "interaction_type": "string"   // 互动类型：dialogue|action|conflict
}
```

#### 用户道具/技能参数
```javascript
// 道具参数
{
    "name": "string",           // 道具名称
    "description": "string",    // 道具描述
    "type": "string",          // 道具类型：weapon|armor|tool|consumable
    "properties": {            // 道具属性
        "attack": 0,           // 攻击力
        "defense": 0,          // 防御力
        "magic": 0,            // 魔法力
        "durability": 100      // 耐久度
    },
    "rarity": "common"         // 稀有度：common|rare|epic|legendary
}

// 技能参数
{
    "name": "string",           // 技能名称
    "description": "string",    // 技能描述
    "type": "string",          // 技能类型：combat|magic|social|crafting
    "level": 1,                // 技能等级
    "requirements": {          // 技能需求
        "min_level": 1,        // 最低等级
        "prerequisites": []    // 前置技能
    },
    "effects": {               // 技能效果
        "damage": 0,           // 伤害值
        "heal": 0,             // 治疗值
        "duration": 0          // 持续时间（秒）
    }
}
```

### 📈 **API 限制与配额**

#### 频率限制
- **聊天 API**：每分钟最多 30 次请求
- **分析 API**：每小时最多 10 次请求  
- **导出 API**：每小时最多 50 次请求
- **其他 API**：每分钟最多 100 次请求

#### 内容限制
- **文本长度**：单次分析最大 50,000 字符
- **文件大小**：上传文件最大 10MB
- **并发连接**：每用户最多 5 个 WebSocket 连接

#### 响应时间
- **一般 API**：< 2 秒
- **AI 聊天**：< 10 秒
- **文本分析**：< 30 秒
- **导出功能**：< 60 秒

更多详细信息，请查看：[开发者文档](docs/developer.md)

## 🧪 开发指南

### 🏃‍♂️ 运行测试

```bash
# 运行所有测试
go test ./...

# 运行测试并生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行特定包的测试
go test ./internal/services/...
```

### 🔧 添加新的LLM提供商

1. **实现接口**: 在 `internal/llm/providers/` 创建新提供商
2. **注册提供商**: 在 `init()` 函数中注册
3. **添加配置**: 更新配置文件模板
4. **编写测试**: 添加对应的单元测试

### 📁 代码结构说明

- **models/**: 数据模型，定义系统中的核心实体
- **services/**: 业务逻辑层，处理核心功能
- **api/**: HTTP处理器，暴露RESTful API
- **llm/**: LLM抽象层，支持多个AI提供商

## 📈 性能优化

### 🚀 系统性能

- **并发处理**: 支持多用户同时访问
- **缓存机制**: LLM响应智能缓存
- **内存优化**: 按需加载，避免内存泄漏
- **文件压缩**: 自动压缩历史数据

### 📊 监控指标

- **API使用统计**: 请求次数和Token消耗
- **响应时间**: AI模型响应速度监控
- **错误率**: 系统错误和API错误追踪
- **资源使用**: CPU和内存使用监控

## 🔐 安全考虑

### 🛡️ 数据安全

- **API密钥**: 安全存储，支持环境变量
- **用户数据**: 本地存储，完全隐私控制
- **访问控制**: 支持用户会话和权限管理
- **数据备份**: 自动备份重要数据

### 🔒 网络安全

- **HTTPS支持**: 生产环境推荐使用HTTPS
- **CORS配置**: 跨域资源共享安全配置
- **输入验证**: 严格的用户输入验证和清理

### 🔐 数据安全与API密钥加密

- **AES-GCM加密**: API密钥在存储前使用AES-GCM算法安全加密
- **环境变量优先**: API密钥主要从环境变量加载（例如，`OPENAI_API_KEY`）
- **加密存储**: 存储在配置文件中的API密钥以加密形式保存在`EncryptedLLMConfig`字段中
- **运行时解密**: 仅在需要进行API调用时才解密API密钥
- **自动迁移**: 旧的未加密API密钥自动迁移到加密存储
- **向后兼容性**: 系统处理从未加密到加密API密钥存储的转换
- **配置安全**: 加密密钥应设置为`CONFIG_ENCRYPTION_KEY`环境变量以获得最佳安全性
- **降级保护**: 包含降级机制以防止以明文形式存储API密钥
- **密钥派生**: 在缺少环境提供的加密密钥时，系统安全地从多个熵源派生加密密钥

## 🤝 贡献指南

我们欢迎各种形式的贡献！

### 📝 贡献方式

1. **Bug报告**: 使用 GitHub Issues 报告问题
2. **功能建议**: 提出新功能的想法和建议
3. **代码贡献**: 提交 Pull Request
4. **文档改进**: 帮助改进文档和示例

### 🔧 开发流程

1. Fork 项目仓库
2. 创建功能分支: `git checkout -b feature/amazing-feature`
3. 提交更改: `git commit -m 'Add amazing feature'`
4. 推送分支: `git push origin feature/amazing-feature`
5. 创建 Pull Request

### 📋 代码规范

- 遵循 Go 官方代码风格
- 添加必要的注释和文档
- 编写单元测试覆盖新功能
- 确保所有测试通过

## 📄 许可证

本项目采用 Apache 2.0 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

### 🎯 核心技术

- [Go](https://golang.org/) - 高性能编程语言
- [Gin](https://gin-gonic.com/) - 轻量级Web框架
- [OpenAI](https://openai.com/) - GPT系列模型
- [Anthropic](https://anthropic.com/) - Claude系列模型

### 👥 社区支持

感谢所有为本项目做出贡献的开发者和用户！

## 📞 联系我们

- **项目主页**: [GitHub Repository](https://github.com/Corphon/SceneIntruderMCP)
- **问题反馈**: [GitHub Issues](https://github.com/Corphon/SceneIntruderMCP/issues)
- **功能建议**: [GitHub Discussions](https://github.com/Corphon/SceneIntruderMCP/discussions)
- **邮件联系**: [project@sceneintruder.dev](mailto:songkf@foxmail.com)

---

<div align="center">

**🌟 如果这个项目对您有帮助，请考虑给它一个Star！ 🌟**

Made with ❤️ by SceneIntruderMCP Team

</div>
-->
