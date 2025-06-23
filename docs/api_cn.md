# SceneIntruderMCP API 文档

<div align="center">

**🎭 AI驱动的沉浸式互动叙事平台 API 参考**

版本: v1.0.0 | 更新时间: 2025-06-20

[返回主页](../README_CN.md) | [English Version](api_en.md)

</div>

## 📋 目录

- [概述](#概述)
- [认证](#认证)
- [通用响应格式](#通用响应格式)
- [错误处理](#错误处理)
- [场景管理 API](#场景管理-api)
- [角色互动 API](#角色互动-api)
- [故事系统 API](#故事系统-api)
- [用户系统 API](#用户系统-api)
- [设置管理 API](#设置管理-api)
- [统计分析 API](#统计分析-api)
- [导出功能 API](#导出功能-api)
- [SDK 示例](#sdk-示例)

## 🌟 概述

SceneIntruderMCP API 提供了完整的RESTful接口，支持：
- 场景创建和管理
- AI角色互动
- 动态故事分支
- 用户定制化
- 数据导出分析

### 基础信息

- **Base URL**: `http://localhost:8080/api`
- **API版本**: v1
- **内容类型**: `application/json`
- **字符编码**: UTF-8

## 🔐 认证

当前版本使用简单的会话认证，未来版本将支持：
- JWT Token 认证
- API Key 认证
- OAuth 2.0

```http
# 当前版本无需特殊认证头
Content-Type: application/json
```

## 📊 通用响应格式

### 成功响应
```json
{
  "success": true,
  "data": {},
  "message": "操作成功",
  "timestamp": "2025-06-20T10:30:00Z"
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
  "timestamp": "2025-06-20T10:30:00Z"
}
```

## ⚠️ 错误处理

### HTTP状态码

| 状态码 | 含义 | 描述 |
|-------|------|------|
| 200 | OK | 请求成功 |
| 201 | Created | 资源创建成功 |
| 400 | Bad Request | 请求参数错误 |
| 404 | Not Found | 资源不存在 |
| 500 | Internal Server Error | 服务器内部错误 |

### 错误代码

| 错误代码 | 描述 |
|---------|------|
| `SCENE_NOT_FOUND` | 场景不存在 |
| `CHARACTER_NOT_FOUND` | 角色不存在 |
| `INVALID_TEXT_FORMAT` | 文本格式无效 |
| `LLM_SERVICE_ERROR` | AI服务错误 |
| `STORAGE_ERROR` | 存储服务错误 |

---
## 🛠️ API文档

### 🔗 可用的API端点

#### 场景管理
```http
GET    /api/scenes                    # 获取场景列表
POST   /api/scenes                    # 创建场景  
GET    /api/scenes/{id}               # 获取场景详情
GET    /api/scenes/{id}/characters    # 获取场景角色
GET    /api/scenes/{id}/aggregate     # 获取场景聚合数据
```

#### 文本分析
```http
POST   /api/analyze                   # 分析文本内容
GET    /api/progress/{taskID}         # 获取分析进度
POST   /api/cancel/{taskID}           # 取消分析任务
POST   /api/upload                    # 上传文件
```

#### 角色互动
```http
POST   /api/chat                      # 与角色对话
POST   /api/interactions/trigger      # 触发角色互动
POST   /api/interactions/simulate     # 模拟角色对话
POST   /api/interactions/aggregate    # 聚合交互处理
GET    /api/interactions/{scene_id}   # 获取互动历史
GET    /api/conversations/{scene_id}  # 获取对话历史
```

#### 系统配置
```http
GET    /api/settings                  # 获取系统设置
POST   /api/settings                  # 更新系统设置
POST   /api/settings/test-connection  # 测试连接
GET    /api/llm/models               # 获取可用模型
```

#### 用户系统
```http
GET    /api/users/{user_id}/items           # 获取用户道具
POST   /api/users/{user_id}/items          # 添加用户道具
GET    /api/users/{user_id}/items/{item_id} # 获取特定道具
PUT    /api/users/{user_id}/items/{item_id} # 更新用户道具
DELETE /api/users/{user_id}/items/{item_id} # 删除用户道具
```
#### 用户技能系统
```http
GET    /api/users/{user_id}/skills           # 获取用户技能
POST   /api/users/{user_id}/skills          # 添加用户技能
GET    /api/users/{user_id}/skills/{skill_id} # 获取特定技能
PUT    /api/users/{user_id}/skills/{skill_id} # 更新用户技能
DELETE /api/users/{user_id}/skills/{skill_id} # 删除用户技能
```

## 🎬 场景管理 API

### 获取所有场景

获取用户的所有场景列表。

```http
GET /api/scenes
```

**响应示例:**
```json
{
  "success": true,
  "data": [
    {
      "id": "scene_001",
      "name": "神秘的古堡",
      "description": "一个充满魔法的古老城堡",
      "type": "fantasy",
      "created_at": "2025-06-20T10:00:00Z",
      "last_updated": "2025-06-20T10:30:00Z",
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

**请求体:**
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

**参数说明:**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 场景名称 |
| `text_content` | string | 是 | 源文本内容 |
| `analysis_type` | string | 否 | 分析类型: `auto`, `novel`, `script`, `story` |
| `creativity_level` | string | 否 | 创意级别: `STRICT`, `BALANCED`, `EXPANSIVE` |
| `allow_plot_twists` | boolean | 否 | 是否允许剧情转折 |
| `preferred_model` | string | 否 | 首选AI模型 |

**响应示例:**
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
          "personality": "正义、坚强、有同情心"
        }
      ],
      "items": [
        {
          "id": "item_001", 
          "name": "魔法剑",
          "description": "散发着蓝色光芒的古老魔剑"
        }
      ],
      "locations": [
        {
          "name": "大厅",
          "description": "宽敞的古堡大厅",
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

**响应示例:**
```json
{
  "success": true,
  "data": {
    "scene": {
      "id": "scene_001",
      "name": "神秘的古堡",
      "description": "一个充满魔法的古老城堡",
      "created_at": "2025-06-20T10:00:00Z"
    },
    "characters": [...],
    "items": [...],
    "locations": [...],
    "story_data": {...}
  }
}
```

### 删除场景

删除指定的场景及其所有相关数据。

```http
DELETE /api/scenes/{scene_id}
```

**响应示例:**
```json
{
  "success": true,
  "message": "场景删除成功"
}
```

### 获取场景聚合数据

获取场景的完整聚合数据，包含对话历史、故事状态等。

```http
GET /api/scenes/{scene_id}/aggregate?conversation_limit=50&include_story=true&include_ui_state=true
```

**查询参数:**

| 参数 | 类型 | 描述 |
|------|------|------|
| `conversation_limit` | integer | 对话历史条数限制 |
| `include_story` | boolean | 是否包含故事数据 |
| `include_ui_state` | boolean | 是否包含UI状态 |

---

## 🎭 角色互动 API

### 与角色对话

与指定角色进行对话交互。

```http
POST /api/chat
```

**请求体:**
```json
{
  "scene_id": "scene_001",
  "character_id": "char_001", 
  "message": "你好，艾莉亚！你在这里做什么？",
  "include_emotion": true,
  "response_format": "structured"
}
```

**参数说明:**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `scene_id` | string | 是 | 场景ID |
| `character_id` | string | 是 | 角色ID |
| `message` | string | 是 | 用户消息 |
| `include_emotion` | boolean | 否 | 是否包含情感分析 |
| `response_format` | string | 否 | 响应格式: `simple`, `structured` |

**响应示例:**
```json
{
  "success": true,
  "data": {
    "character_id": "char_001",
    "character_name": "艾莉亚",
    "message": "你好！我正在探索这座古堡的秘密。这里似乎隐藏着古老的魔法...",
    "emotion": "好奇",
    "action": "警惕地环顾四周",
    "emotion_data": {
      "emotion": "curious",
      "intensity": 7,
      "body_language": "slightly tensed, alert posture",
      "facial_expression": "furrowed brow, focused eyes",
      "voice_tone": "cautious but intrigued",
      "secondary_emotions": ["alert", "determined"]
    },
    "timestamp": "2025-06-20T10:30:00Z"
  }
}
```

### 获取场景角色列表

获取指定场景中的所有角色。

```http
GET /api/scenes/{scene_id}/characters
```

**响应示例:**
```json
{
  "success": true,
  "data": [
    {
      "id": "char_001",
      "name": "艾莉亚",
      "description": "勇敢的女骑士",
      "personality": "正义、坚强、有同情心",
      "current_mood": "alert",
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

**请求体:**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002"],
  "topic": "探索古堡的计划",
  "context": "两人在大厅中相遇",
  "interaction_type": "discussion"
}
```

**响应示例:**
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
        "message": "我们应该小心探索这座古堡...",
        "emotion": "谨慎",
        "action": "握紧剑柄"
      },
      {
        "character_id": "char_002", 
        "character_name": "托马斯",
        "message": "同意，我感觉到了强大的魔法气息。",
        "emotion": "警觉",
        "action": "点头同意"
      }
    ]
  }
}
```

### 模拟角色对话

模拟多轮角色间的自动对话。

```http
POST /api/interactions/simulate
```

**请求体:**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002", "char_003"],
  "topic": "制定探索策略",
  "rounds": 5,
  "style": "collaborative"
}
```

### 获取对话历史

获取场景的对话历史记录。

```http
GET /api/conversations/{scene_id}?limit=50&offset=0&character_id=char_001
```

**查询参数:**

| 参数 | 类型 | 描述 |
|------|------|------|
| `limit` | integer | 返回条数限制 |
| `offset` | integer | 偏移量 |
| `character_id` | string | 筛选特定角色 |

---

## 📖 故事系统 API

### 获取故事数据

获取场景的完整故事数据。

```http
GET /api/scenes/{scene_id}/story
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "story_id": "story_001",
    "scene_id": "scene_001",
    "intro": "在这座神秘的古堡中，冒险即将开始...",
    "main_objective": "找到古堡的核心秘密",
    "current_state": "exploring_main_hall",
    "progress": 35,
    "nodes": [
      {
        "id": "node_001",
        "content": "你站在古堡的大厅中...",
        "type": "main",
        "choices": [
          {
            "id": "choice_001",
            "text": "调查左侧的楼梯",
            "consequence": "发现隐藏的密室"
          }
        ]
      }
    ],
    "tasks": [
      {
        "id": "task_001",
        "title": "探索大厅",
        "description": "仔细搜查古堡的主大厅",
        "status": "completed",
        "objectives": [
          {
            "id": "obj_001",
            "description": "检查大门",
            "completed": true
          }
        ]
      }
    ]
  }
}
```

### 做出故事选择

在故事分支点做出选择，推进剧情发展。

```http
POST /api/scenes/{scene_id}/story/choice
```

**请求体:**
```json
{
  "node_id": "node_001",
  "choice_id": "choice_001",
  "user_preferences": {
    "creativity_level": "BALANCED",
    "allow_plot_twists": true
  }
}
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "next_node": {
      "id": "node_002",
      "content": "你小心翼翼地走上左侧楼梯，突然发现墙上有一个隐藏的开关...",
      "type": "main",
      "choices": [...]
    },
    "new_items": [
      {
        "id": "item_002",
        "name": "古老的钥匙",
        "description": "在楼梯下发现的神秘钥匙"
      }
    ],
    "story_progress": 40
  }
}
```

### 创建故事分支

基于特定触发条件创建新的故事分支。

```http
POST /api/scenes/{scene_id}/story/branch
```

**请求体:**
```json
{
  "trigger_type": "item",
  "trigger_id": "item_001", 
  "branch_name": "魔法剑路线",
  "context": "使用魔法剑后的剧情发展"
}
```

### 故事回溯

回溯到指定的故事节点。

```http
POST /api/scenes/{scene_id}/story/rewind
```

**请求体:**
```json
{
  "target_node_id": "node_005",
  "preserve_choices": true
}
```

---

## 👤 用户系统 API

### 获取用户信息

获取用户的完整信息和偏好设置。

```http
GET /api/users/{user_id}
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "id": "user_001",
    "username": "player1",
    "display_name": "冒险者",
    "preferences": {
      "creativity_level": "BALANCED",
      "allow_plot_twists": true,
      "response_length": "medium",
      "language_style": "casual",
      "preferred_model": "gpt-4"
    },
    "items": [
      {
        "id": "custom_item_001",
        "name": "幸运护符",
        "description": "增加运气的神奇护符",
        "effects": [
          {
            "target": "self",
            "type": "luck",
            "value": 5,
            "probability": 1.0
          }
        ]
      }
    ],
    "skills": [
      {
        "id": "custom_skill_001",
        "name": "洞察术",
        "description": "看穿隐藏的秘密",
        "cooldown": 300,
        "effects": [
          {
            "target": "environment",
            "type": "discovery",
            "value": 10,
            "probability": 0.8
          }
        ]
      }
    ],
    "saved_scenes": ["scene_001", "scene_002"]
  }
}
```

### 更新用户偏好

更新用户的个人偏好设置。

```http
PUT /api/users/{user_id}/preferences
```

**请求体:**
```json
{
  "creativity_level": "EXPANSIVE",
  "allow_plot_twists": true,
  "response_length": "long",
  "language_style": "literary", 
  "preferred_model": "claude-3-5-sonnet-20241022",
  "dark_mode": true
}
```

### 添加用户道具

为用户添加自定义道具。

```http
POST /api/users/{user_id}/items
```

**请求体:**
```json
{
  "name": "魔法水晶",
  "description": "蕴含古老魔法的水晶",
  "rarity": "rare",
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
  "usage_conditions": ["in_combat", "during_spell_casting"],
  "cooldown": 1800
}
```

### 添加用户技能

为用户添加自定义技能。

```http
POST /api/users/{user_id}/skills
```

**请求体:**
```json
{
  "name": "心灵感应",
  "description": "读取他人思想的能力",
  "category": "mental",
  "effects": [
    {
      "target": "other",
      "type": "emotion_reveal",
      "value": 100,
      "probability": 0.9
    }
  ],
  "requirements": ["mana >= 10", "target_distance <= 5"],
  "cooldown": 600,
  "mana_cost": 15
}
```

### 获取用户道具列表

```http
GET /api/users/{user_id}/items?category=weapon&rarity=rare
```

### 获取用户技能列表

```http
GET /api/users/{user_id}/skills?category=magic&available_only=true
```

---

## ⚙️ 设置管理 API

### 获取系统设置

获取当前的系统配置。

```http
GET /api/settings
```

**响应示例:**
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
      "version": "1.0.0",
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

**请求体:**
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

测试AI服务提供商的连接状态。

```http
POST /api/settings/test-connection
```

**请求体:**
```json
{
  "provider": "openai",
  "model": "gpt-4"
}
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "provider": "openai",
    "model": "gpt-4",
    "status": "connected",
    "response_time": 1250,
    "test_message": "连接测试成功"
  }
}
```

---

## 📊 统计分析 API

### 获取场景统计

获取场景的详细统计信息。

```http
GET /api/scenes/{scene_id}/stats
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_001",
    "basic_stats": {
      "total_conversations": 156,
      "total_messages": 1247,
      "active_characters": 5,
      "story_completion": 65,
      "average_session_time": 1800
    },
    "character_stats": [
      {
        "character_id": "char_001",
        "name": "艾莉亚",
        "interaction_count": 89,
        "average_response_length": 45,
        "emotion_distribution": {
          "happy": 25,
          "curious": 30,
          "concerned": 20,
          "determined": 25
        },
        "player_relationship": 75
      }
    ],
    "story_stats": {
      "nodes_visited": 23,
      "choices_made": 18,
      "branches_explored": 3,
      "items_discovered": 12,
      "tasks_completed": 8
    },
    "time_stats": {
      "total_playtime": 14400,
      "average_session": 1800,
      "last_activity": "2025-06-20T10:30:00Z"
    }
  }
}
```

### 获取用户统计

获取用户的全局统计信息。

```http
GET /api/users/{user_id}/stats
```

**响应示例:**
```json
{
  "success": true,
  "data": {
    "user_id": "user_001",
    "global_stats": {
      "total_scenes": 5,
      "total_playtime": 36000,
      "total_conversations": 423,
      "favorite_genre": "fantasy",
      "preferred_creativity": "BALANCED"
    },
    "achievement_progress": {
      "story_master": 70,
      "character_whisperer": 85,
      "world_explorer": 45
    }
  }
}
```

---

## 📤 导出功能 API

### 导出场景数据

导出完整的场景数据为多种格式。

```http
POST /api/scenes/{scene_id}/export
```

**请求体:**
```json
{
  "format": "html",
  "include_conversations": true,
  "include_story": true,
  "include_stats": true,
  "date_range": {
    "start": "2025-06-01T00:00:00Z",
    "end": "2025-06-20T23:59:59Z"
  }
}
```

**支持的格式:**
- `html` - 完整的HTML报告
- `markdown` - Markdown格式文档  
- `json` - 结构化JSON数据
- `csv` - 表格数据（对话记录）
- `pdf` - PDF文档（需要额外配置）

**响应示例:**
```json
{
  "success": true,
  "data": {
    "export_id": "export_001",
    "format": "html", 
    "file_url": "/api/exports/download/export_001.html",
    "file_size": 2048576,
    "created_at": "2025-06-20T10:30:00Z",
    "expires_at": "2025-06-27T10:30:00Z"
  }
}
```

### 下载导出文件

下载生成的导出文件。

```http
GET /api/exports/download/{filename}
```

### 获取导出历史

获取用户的导出历史记录。

```http
GET /api/users/{user_id}/exports?limit=20
```

---

## 🛠️ SDK 示例

### JavaScript SDK

```javascript
// 初始化API客户端
class SceneIntruderAPI {
    constructor(baseURL = 'http://localhost:8080/api') {
        this.baseURL = baseURL;
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        const response = await fetch(url, config);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }

        return await response.json();
    }

    // 场景管理
    async createScene(data) {
        return this.request('/scenes', {
            method: 'POST',
            body: data
        });
    }

    async getScene(sceneId) {
        return this.request(`/scenes/${sceneId}`);
    }

    // 角色互动
    async chatWithCharacter(sceneId, characterId, message) {
        return this.request('/chat', {
            method: 'POST',
            body: {
                scene_id: sceneId,
                character_id: characterId,
                message: message,
                include_emotion: true
            }
        });
    }

    // 故事系统
    async makeStoryChoice(sceneId, nodeId, choiceId) {
        return this.request(`/scenes/${sceneId}/story/choice`, {
            method: 'POST',
            body: {
                node_id: nodeId,
                choice_id: choiceId
            }
        });
    }
}

// 使用示例
const api = new SceneIntruderAPI();

// 创建场景
const scene = await api.createScene({
    name: "测试场景",
    text_content: "从前有一个勇敢的骑士...",
    creativity_level: "BALANCED"
});

// 与角色对话
const response = await api.chatWithCharacter(
    scene.data.scene_id,
    "char_001", 
    "你好！"
);

console.log('角色回复:', response.data.message);
console.log('情感状态:', response.data.emotion);
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

# 使用示例
api = SceneIntruderAPI()

# 创建场景
scene = api.create_scene(
    name="Python测试场景",
    text_content="这是一个测试故事...",
    creativity_level="BALANCED"
)

# 与角色对话
response = api.chat_with_character(
    scene["data"]["scene_id"],
    "char_001",
    "你好，很高兴见到你！"
)

print(f"角色回复: {response['data']['message']}")
print(f"情感状态: {response['data']['emotion']}")
```

---

## 📋 API变更日志

### v1.0.0 (2025-06-20)
- 初始API版本发布
- 支持场景创建和管理
- 实现角色互动系统
- 添加故事分支功能
- 集成多LLM提供商支持

### 计划中的功能
- v1.1.0: WebSocket实时通信
- v1.2.0: 批量操作API
- v1.3.0: GraphQL支持
- v2.0.0: 微服务架构重构

---

## 🔗 相关链接

- [主要文档](../README_CN.md)
- [部署指南](deployment_cn.md)
- [GitHub仓库](https://github.com/Corphon/SceneIntruderMCP)
- [问题反馈](https://github.com/Corphon/SceneIntruderMCP/issues)

---

<div align="center">

**📚 需要帮助？欢迎查阅我们的文档或提交issue！**

Made with ❤️ by SceneIntruderMCP Team

</div>
