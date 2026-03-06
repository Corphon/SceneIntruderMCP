# SceneIntruderMCP

<div align="center">

![SceneIntruderMCP Logo](temp/logo.png)

**🎭 AI驱动的沉浸式互动叙事绘画平台**

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/Corphon/SceneIntruderMCP)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow.svg)](https://codecov.io)

[English](README.md) | 简体中文

</div>

## 🌟 项目简介

SceneIntruderMCP 现已演进为一个 AI 原生叙事工作台：同时覆盖互动场景、连环画 comics 分镜工作流，以及 New Script 写作助手三条完整创作链路，并将结构化分析、异步生成和可配置 LLM/Vision provider 收口到同一个应用中。

### ✨ 核心特性

#### 🧠 **智能文本分析**
- **多维度解析**: 自动提取场景、角色、物品、情节要素
- **双语支持**: 完美支持中英文内容的智能识别和处理
- **深度分析**: 基于文学理论的专业级文本类型识别

#### 🎭 **AI角色系统**
- **情感智能**: 8维度情感分析 (情绪、动作、表情、语调等)
- **角色一致性**: 维护长期记忆和个性特征
- **动态互动**: 角色间自动触发的智能对话

#### 📖 **动态故事引擎**
- **非线性叙事**: 支持复杂的故事分支和时间线管理
- **智能选择生成**: AI基于上下文动态创建4类选择 (行动/对话/探索/策略)
- **故事回溯**: 完整的时间线回滚和状态管理

#### 🎬 **v2 Comics 工作台**
- **首页独立入口**: 可从 Home 直接通过 `POST /api/scenes/shell` 创建独立 comic 工作区
- **5 步工作流**: 分镜分析 → 提示词 → 关键元素 → Prompt Review → 出图/导出
- **SSE 进度流**: 长任务统一通过 `GET /api/progress/:task_id` 推送进度
- **参考图链路**: 支持 element 级参考图上传与重绘复用
- **模型驱动配置**: Step4 从 Settings 的 `vision_models` 获取可选模型

#### ✍️ **New Script 写作空间**
- **项目式创作**: 创建剧本项目、初始化大纲/首稿，并持续迭代章节
- **辅助模式**: 通过 `/api/scripts/:id/command` 暴露 inspiration / completion / polish 等写作辅助
- **草稿版本化**: 支持回滚、导出以及基于 `chapter_draft.json` 的章节级持久化编辑

#### 🎮 **游戏化体验**
- **用户定制**: 自定义道具和技能系统
- **创意控制**: 3级创意程度控制 (严格/平衡/扩展)
- **进度追踪**: 实时故事完成度和统计分析

#### 🎒 **用户道具与技能管理**
- **自定义道具**: 用户可以定义具有可自定义属性的独特道具
- **自定义技能**: 用户可以创建和管理具有不同效果和等级的技能
- **属性系统**: 遏品可以有多个属性（攻击力、防御力、魔力、耐久度等）
- **稀有度等级**: 遏品支持不同的稀有度等级：普通、罕见、史诗、传说
- **技能树**: 层级化技能系统，带有前置要求和条件
- **角色互动**: 遏品和技能可以影响角色互动和故事情节
- **API集成**: 通过API提供完整的CRUD操作来管理用户定义的内容

#### 🔗 **多LLM支持**
- **OpenAI GPT**: GPT-3.5/4/4o/5-chat 系列
- **Anthropic Claude**: Claude-3/3.5/3.7 系列
- **DeepSeek**: DeepSeek-R1/Coder 系列
- **Google Gemini**: Gemini-2.0/1.5 系列 (包含思维模型)
- **Grok**: xAI 的 Grok-2/2-mini/3 系列
- **Mistral**: Mistral-large/small 系列
- **Qwen**: 阿里云 Qwen2.5/32b 系列 (包含 qwq 模型)
- **GitHub Models**: 通过 GitHub Models 平台 (GPT-4o, o1 系列, Phi-4 等)
- **OpenRouter**: 开源模型聚合平台，提供免费层级
- **GLM**: 智谱AI的 GLM-4/4-plus 系列

#### 🖼️ **多 Vision Provider 支持**
- **内置 provider**: `sdwebui`、`dashscope`、`gemini`、`ark`、`openai`、`glm`、`placeholder`
- **推荐智谱图像模型**: `glm-image`
- **统一 Vision 设置**: 可在 Settings 页集中配置 `vision_provider`、`vision_default_model`、`vision_config.endpoint`、`vision_config.api_key`

## 🏗️ 技术架构

### 📁 项目结构

```

> ℹ️ **提示**：当前激活的提供商会读取其 `default_model` 配置。凡是未明确指定模型的 AI 请求都会自动回退到该值，因此只需在配置文件里修改一次即可全局切换模型，无需改动代码。
SceneIntruderMCP/
├── cmd/
│   └── server/           # Application entry point
│       └── main.go
├── internal/
│   ├── api/              # HTTP API routes and handlers
│   ├── app/              # Application core logic
│   ├── config/           # Configuration management
│   ├── di/               # Dependency injection
│   ├── llm/              # LLM provider abstraction layer
│   │   └── providers/    # Various LLM provider implementations
│   ├── models/           # Data model definitions
│   ├── services/         # Business logic services
│   └── storage/          # Storage abstraction layer
├── frontend/
│   └── dist/             # assets
├── data/                 # Data storage directory
│   ├── scenes/           # Scene data
│   ├── stories/          # Story data
│   ├── users/            # User data
│   └── exports/          # Export files
└── logs/                 # Application logs
```

### 🔧 核心技术栈

- **后端**: Go 1.21+, Gin Web Framework
- **AI集成**: 多LLM提供商支持，统一抽象接口
- **存储**: 基于文件系统的JSON存储，支持扩展到数据库
- **前端**: React，响应式设计
- **部署**: 容器化支持，云原生架构

## 🆕 版本亮点（v2.0.0 · 2026-03-06）

- **Comics v2 已完成**：首页新增独立 **New Comic** 入口，创建 scene shell 后可直接进入 comics 5-step 工作流。
- **Standalone 文本分析**：comics Step1 已支持 `source_text`，无需先有 story node 也能直接从故事文本生成分镜。
- **Vision provider 完整接入**：Settings 现同时管理 LLM + Vision，并支持 `glm` / `glm-image`、provider 自动填充 endpoint/默认模型。
- **文档与接口对齐**：`GET /api/settings` 已成为前端 Vision 模型列表的事实来源，comics、scripts、导出与部署说明均已同步到当前实现。

---

## 🆕 版本亮点（v1.4.0 · 2025-12-25）

- **新增“新建剧本”写作助手（核心功能）** — 新增“一键新建剧本”写作流程，会创建剧本项目并自动生成初始章节大纲与第1章第1场草稿（封装 CreateProject + GenerateInitial）。使用方法：在“剧本”页面点击 **“新建剧本”** 即可开始，系统会初始化 `chapter_draft.json`，生成初始草稿与工作流项，项目随即可编辑与继续生成。

## 🚀 快速开始

### 📋 系统要求

- Go 1.21 或更高版本
- 至少一个LLM API密钥 (OpenAI/Claude/DeepSeek等)
- 2GB+ 可用内存
- 操作系统: Windows/Linux/macOS

### 📦 安装步骤

1. **克隆项目**
```bash
git clone https://github.com/Corphon/SceneIntruderMCP.git
cd SceneIntruderMCP
```

2. **安装依赖**
```bash
go mod download
```

3. **配置环境**

首次启动时，服务会在 `data/config.json`（或 `${DATA_DIR}/config.json`）生成配置文件。
你可以通过以下方式配置 LLM 与 Vision 提供商：

- 打开设置页面：`http://localhost:8080/settings`，或
- 直接编辑 `data/config.json`。

v2.0.0 中常见的 Vision 配置字段包括：

- `vision_provider`
- `vision_default_model`
- `vision_config.endpoint`
- `vision_config.api_key`

若使用 GLM Image，推荐值为：

- provider：`glm`
- 默认模型：`glm-image`
- endpoint：`https://open.bigmodel.cn/api/paas/v4`

4. **启动服务**
```bash
# 开发模式
go run cmd/server/main.go

# 生产模式
go build -o sceneintruder cmd/server/main.go
./sceneintruder
```

5. **访问应用**
```
浏览器打开: http://localhost:8080
```

### ⚙️ 配置说明

#### `data/config.json` 配置示例

```json
{
    "port": "8080",
    "data_dir": "data",
    "static_dir": "frontend\\dist\\assets",
    "templates_dir": "frontend\\dist",
    "log_dir": "logs",
    "debug_mode": true,
    "llm_provider": "openrouter",
    "llm_config": {
        "default_model": "mistralai/devstral-2512:free",
        "base_url": "",
        "api_key": ""
    },
    "encrypted_llm_config": {
        "api_key": "<encrypted_api_key_here>"
    }
}
```

#### 🔐 配置加密与 `.encryption_key`

- 如果未设置 `CONFIG_ENCRYPTION_KEY`，系统会自动生成 32 字节随机密钥并写入 `data/.encryption_key`，用于长期加密 API 凭据。
- 该文件必须与 `data/config.json` 同步保留，否则所有已加密的密钥都将无法解密，需要重新在设置页填写。
- 如需轮换密钥，可删除该文件并重启服务，然后立即更新新的 API 密钥，系统会自动生成并持久化新的密钥。

## 📖 使用指南

### 🎬 创建场景

1. **上传文本**: 支持小说、剧本、故事等多种文本格式
2. **AI分析**: 系统自动提取角色、场景、物品等要素
3. **场景生成**: 创建可交互的场景环境

### 🎭 角色互动

1. **选择角色**: 从分析出的角色中选择互动对象
2. **自然对话**: 与AI角色进行自然语言对话
3. **情感反馈**: 观察角色的情绪、动作和表情变化

### 🖥️ 控制台 CLI 快速体验

`cmd/demo` 提供了一套无需前端即可跑通完整流程的多语言控制台界面，便于快速联调和压测：

- **开箱即选语言**: 启动后即可在中文/英文界面间切换，所有提示与指令同步更新。
- **LLM 一键配置**: 菜单默认优先引导配置 LLM，可直接从 `config.json` 读取，无需重复输入密钥。
- **首节点自动推送**: 进入场景后系统会主动推进首个剧情节点，并用方框高亮展示最新内容。
- **HUD 式互动看板**: 每轮交互前都会以方框列出可用的角色(@)、地点(@)、物品(/)、技能(/)与系统指令(!)。
- **符号快捷指令**: `@角色/地点` 聚焦特定对象，`/物品/技能` 引入装备与能力，`!status/!tasks/!advance/...` 可随时查看状态或强制推进剧情。
- **空输入继续剧情**: 直接回车即可让 AI 自动续写节点，方便快速冒烟测试故事引擎。

### 📚 故事分支

1. **动态选择**: AI根据当前情况生成4种类型的选择
2. **故事发展**: 基于选择推进非线性故事情节
3. **分支管理**: 支持故事回溯和多分支探索

### 📊 数据导出

1. **交互记录**: 导出完整的对话历史
2. **故事文档**: 生成结构化的故事文档
3. **统计分析**: 角色互动和故事进展统计

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
