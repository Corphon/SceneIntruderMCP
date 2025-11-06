# SceneIntruderMCP API 文档

<div align="center">

**🎭 AI 驱动的沉浸式互动叙事平台 API 参考**

版本: v1.1.0 | 更新日期: 2025-06-27

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
```

#### 故事系统
```http
GET    /api/scenes/{id}/story           # 获取故事数据
POST   /api/scenes/{id}/story/choice    # 进行故事选择
POST   /api/scenes/{id}/story/advance   # 推进故事
POST   /api/scenes/{id}/story/rewind    # 回溯故事
GET    /api/scenes/{id}/story/branches  # 获取故事分支
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
    "llm": {
      "default_provider": "openai",
      "available_providers": ["openai", "anthropic", "deepseek"],
      "models": {
        "openai": ["gpt-3.5-turbo", "gpt-4", "gpt-4-turbo"],
        "anthropic": ["claude-3-sonnet-20240229", "claude-3-5-sonnet-20241022"]
      }
    },
    "server": {
      "version": "1.1.0",
      "uptime": 3600,
      "debug_mode": false
    },
    "features": {
      "story_branching": true,
      "character_interaction": true,
      "data_export": true,
      "user_customization": true
    }
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
  "llm": {
    "default_provider": "openai",
    "providers": {
      "openai": {
        "api_key": "your-openai-api-key",
        "base_url": "https://api.openai.com/v1",
        "default_model": "gpt-4o"
      },
      "anthropic": {
        "api_key": "your-claude-api-key", 
        "default_model": "claude-3-5-sonnet-20241022"
      },
      "deepseek": {
        "api_key": "your-deepseek-api-key",
        "default_model": "deepseek-chat"
      }
    }
  },
  "server": {
    "port": 8080,
    "debug": false
  },
  "storage": {
    "data_path": "./data"
  }
}
```

### 测试连接

测试 AI 服务提供商的连接状态。

```http
POST /api/settings/test-connection
```

**请求体：**
```json
{
  "provider": "openai",
  "model": "gpt-4"
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
| `format` | string | 导出格式：`json`, `markdown`, `html` |
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

```http
GET /api/scenes/{scene_id}/export/interactions?format=markdown
```

### 导出故事文档

将故事导出为可读文档。

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

### v1.1.0 (2025-06-27) - 当前版本
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
