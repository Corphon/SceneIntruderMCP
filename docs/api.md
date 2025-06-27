# SceneIntruderMCP API Documentation

<div align="center">

**🎭 AI-Powered Immersive Interactive Storytelling Platform API Reference**

Version: v1.0.0 | Updated: 2025-06-20

[Back to Home](../README.md) | [中文版本](api_CN.md)

</div>

## 📋 Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Common Response Format](#common-response-format)
- [Error Handling](#error-handling)
- [Scene Management API](#scene-management-api)
- [Character Interaction API](#character-interaction-api)
- [Story System API](#story-system-api)
- [User System API](#user-system-api)
- [Settings Management API](#settings-management-api)
- [Analytics API](#analytics-api)
- [Export Features API](#export-features-api)
- [SDK Examples](#sdk-examples)

## 🌟 Overview

SceneIntruderMCP API provides complete RESTful interfaces, supporting:
- Scene creation and management
- AI character interaction
- Dynamic story branching
- User customization
- Data export and analysis

### Basic Information

- **Base URL**: `http://localhost:8080/api`
- **API Version**: v1
- **Content Type**: `application/json`
- **Character Encoding**: UTF-8

## 🔐 Authentication

Current version uses simple session authentication. Future versions will support:
- JWT Token authentication
- API Key authentication
- OAuth 2.0

```http
# Current version requires no special authentication headers
Content-Type: application/json
```

## 📊 Common Response Format

### Success Response
```json
{
  "success": true,
  "data": {},
  "message": "Operation successful",
  "timestamp": "2025-06-20T10:30:00Z"
}
```

### Error Response
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": "Scene name cannot be empty"
  },
  "timestamp": "2025-06-20T10:30:00Z"
}
```

## ⚠️ Error Handling

### HTTP Status Codes

| Status Code | Meaning | Description |
|------------|---------|-------------|
| 200 | OK | Request successful |
| 201 | Created | Resource created successfully |
| 400 | Bad Request | Invalid request parameters |
| 404 | Not Found | Resource not found |
| 500 | Internal Server Error | Internal server error |

### Error Codes

| Error Code | Description |
|-----------|-------------|
| `SCENE_NOT_FOUND` | Scene not found |
| `CHARACTER_NOT_FOUND` | Character not found |
| `INVALID_TEXT_FORMAT` | Invalid text format |
| `LLM_SERVICE_ERROR` | AI service error |
| `STORAGE_ERROR` | Storage service error |

---
## 🛠️ API Documentation

### 🔗 Available API Endpoints

#### Scene Management
```http
GET    /api/scenes                      # Get scene list
POST   /api/scenes                      # Create scene  
GET    /api/scenes/{id}                 # Get scene details
GET    /api/scenes/{id}/characters      # Get scene characters
GET    /api/scenes/{id}/conversations   # Get scene conversations
GET    /api/scenes/{id}/aggregate       # Get scene aggregate data
```

#### Story System
```http
GET    /api/scenes/{id}/story           # Get story data
POST   /api/scenes/{id}/story/choice    # Make story choice
POST   /api/scenes/{id}/story/advance   # Advance story
POST   /api/scenes/{id}/story/rewind    # Rewind story
GET    /api/scenes/{id}/story/branches  # Get story branches
```

#### Export Features
```http
GET    /api/scenes/{id}/export/scene        # Export scene data
GET    /api/scenes/{id}/export/interactions # Export interactions
GET    /api/scenes/{id}/export/story        # Export story document
```

#### Character Interaction
```http
POST   /api/chat                        # Basic chat with characters
POST   /api/chat/emotion                # Chat with emotion analysis (NEW)
POST   /api/interactions/trigger        # Trigger character interactions
POST   /api/interactions/simulate       # Simulate character dialogue
POST   /api/interactions/aggregate      # Aggregate interaction processing
GET    /api/interactions/{scene_id}     # Get interaction history
GET    /api/interactions/{scene_id}/{character1_id}/{character2_id} # Get specific character interactions
```

#### System Configuration & LLM Management
```http
GET    /api/settings                    # Get system settings
POST   /api/settings                    # Update system settings
POST   /api/settings/test-connection    # Test connection

GET    /api/llm/status                  # Get LLM service status (NEW)
GET    /api/llm/models                  # Get available models (NEW)
PUT    /api/llm/config                  # Update LLM configuration (NEW)
```

#### User Management System
```http
# User Profile
GET    /api/users/{user_id}             # Get user profile
PUT    /api/users/{user_id}             # Update user profile
GET    /api/users/{user_id}/preferences # Get user preferences
PUT    /api/users/{user_id}/preferences # Update user preferences

# User Items Management
GET    /api/users/{user_id}/items           # Get user items
POST   /api/users/{user_id}/items           # Add user item
GET    /api/users/{user_id}/items/{item_id} # Get specific item
PUT    /api/users/{user_id}/items/{item_id} # Update user item
DELETE /api/users/{user_id}/items/{item_id} # Delete user item

# User Skills Management
GET    /api/users/{user_id}/skills           # Get user skills
POST   /api/users/{user_id}/skills           # Add user skill
GET    /api/users/{user_id}/skills/{skill_id} # Get specific skill
PUT    /api/users/{user_id}/skills/{skill_id} # Update user skill
DELETE /api/users/{user_id}/skills/{skill_id} # Delete user skill
```

#### WebSocket Support
```http
WS     /ws/scene/{id}                   # Scene WebSocket connection
WS     /ws/user/status                  # User status WebSocket connection
```

#### Debug & Development
```http
GET    /api/ws/status                   # Get WebSocket connection status
```

## 🎬 Scene Management API

### Get All Scenes

Get all scenes list for the user.

```http
GET /api/scenes
```

**Response Example:**
```json
{
  "success": true,
  "data": [
    {
      "id": "scene_001",
      "name": "Mysterious Castle",
      "description": "An ancient castle filled with magic",
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

### Create New Scene

Create a new interactive scene by uploading text.

```http
POST /api/scenes
```

**Request Body:**
```json
{
  "name": "Scene Name",
  "text_content": "Novel or story text content...",
  "analysis_type": "auto",
  "creativity_level": "BALANCED",
  "allow_plot_twists": true,
  "preferred_model": "gpt-4"
}
```

**Parameter Description:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Scene name |
| `text_content` | string | Yes | Source text content |
| `analysis_type` | string | No | Analysis type: `auto`, `novel`, `script`, `story` |
| `creativity_level` | string | No | Creativity level: `STRICT`, `BALANCED`, `EXPANSIVE` |
| `allow_plot_twists` | boolean | No | Whether to allow plot twists |
| `preferred_model` | string | No | Preferred AI model |

**Response Example:**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_12345",
    "analysis_result": {
      "characters": [
        {
          "id": "char_001",
          "name": "Aria",
          "description": "Brave female knight",
          "personality": "Just, strong, compassionate"
        }
      ],
      "items": [
        {
          "id": "item_001", 
          "name": "Magic Sword",
          "description": "Ancient magic sword glowing with blue light"
        }
      ],
      "locations": [
        {
          "name": "Great Hall",
          "description": "Spacious castle hall",
          "accessible": true
        }
      ]
    }
  }
}
```

### Get Scene Details

Get detailed information of a specified scene.

```http
GET /api/scenes/{scene_id}
```

**Response Example:**
```json
{
  "success": true,
  "data": {
    "scene": {
      "id": "scene_001",
      "name": "Mysterious Castle",
      "description": "An ancient castle filled with magic",
      "created_at": "2025-06-20T10:00:00Z"
    },
    "characters": [...],
    "items": [...],
    "locations": [...],
    "story_data": {...}
  }
}
```

### Get Scene Aggregate Data

Get complete aggregate data of a scene, including conversation history, story state, etc.

```http
GET /api/scenes/{scene_id}/aggregate?conversation_limit=50&include_story=true&include_ui_state=true
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `conversation_limit` | integer | Conversation history limit |
| `include_story` | boolean | Whether to include story data |
| `include_ui_state` | boolean | Whether to include UI state |

---

## 🎭 Character Interaction API

### Chat with Character

Interact and chat with a specified character.

```http
POST /api/chat
```

**Request Body:**
```json
{
  "scene_id": "scene_001",
  "character_id": "char_001", 
  "message": "Hello, Aria! What are you doing here?",
  "include_emotion": true,
  "response_format": "structured"
}
```

**Parameter Description:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `scene_id` | string | Yes | Scene ID |
| `character_id` | string | Yes | Character ID |
| `message` | string | Yes | User message |
| `include_emotion` | boolean | No | Whether to include emotion analysis |
| `response_format` | string | No | Response format: `simple`, `structured` |

**Response Example:**
```json
{
  "success": true,
  "data": {
    "character_id": "char_001",
    "character_name": "Aria",
    "message": "Hello! I'm exploring the secrets of this castle. There seems to be ancient magic hidden here...",
    "emotion": "curious",
    "action": "looking around alertly",
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
## 📖 Story System API

### Get Story Data

Get story data for a specific scene.

```http
GET /api/scenes/{scene_id}/story
```

**Response Example:**
```json
{
  "success": true,
  "data": {
    "scene_id": "scene_001",
    "intro": "Welcome to the mysterious castle...",
    "main_objective": "Explore the castle and uncover its secrets",
    "current_state": {
      "current_node_id": "node_001",
      "current_location": "castle_entrance"
    },
    "progress": {
      "completion_percentage": 25,
      "nodes_visited": 5,
      "choices_made": 3
    },
    "nodes": [
      {
        "id": "node_001",
        "title": "The Entrance",
        "content": "You stand before the massive castle doors...",
        "choices": [
          {
            "id": "choice_001",
            "text": "Push open the doors",
            "consequences": "You enter the main hall"
          }
        ]
      }
    ]
  }
}
```

### Make Story Choice

Make a choice in the story progression.

```http
POST /api/scenes/{scene_id}/story/choice
```

**Request Body:**
```json
{
  "node_id": "node_001",
  "choice_id": "choice_001"
}
```

**Response Example:**
```json
{
  "success": true,
  "message": "Choice executed successfully",
  "next_node": {
    "id": "node_002",
    "title": "The Main Hall",
    "content": "You find yourself in a vast hall..."
  },
  "story_data": {
    "current_state": {...},
    "progress": {...}
  }
}
```

### Advance Story

Automatically advance the story based on current context.

```http
POST /api/scenes/{scene_id}/story/advance
```

**Response Example:**
```json
{
  "success": true,
  "message": "Story advanced",
  "story_update": {
    "title": "A New Discovery",
    "content": "As you explore further, you discover...",
    "new_characters": [...],
    "new_items": [...]
  }
}
```

### Rewind Story

Rewind the story to a previous node.

```http
POST /api/scenes/{scene_id}/story/rewind
```

**Request Body:**
```json
{
  "node_id": "node_001"
}
```

### Get Story Branches

Get all story branches and paths.

```http
GET /api/scenes/{scene_id}/story/branches
```

### Get Scene Character List

Get all characters in a specified scene.

```http
GET /api/scenes/{scene_id}/characters
```

**Response Example:**
```json
{
  "success": true,
  "data": [
    {
      "id": "char_001",
      "name": "Aria",
      "description": "Brave female knight",
      "personality": "Just, strong, compassionate",
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

### Trigger Character Interaction

Trigger automatic interaction between two or more characters.

```http
POST /api/interactions/trigger
```

**Request Body:**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002"],
  "topic": "Castle exploration plan",
  "context": "Two characters meet in the hall",
  "interaction_type": "discussion"
}
```

**Response Example:**
```json
{
  "success": true,
  "data": {
    "interaction_id": "int_001",
    "participants": ["char_001", "char_002"],
    "dialogue": [
      {
        "character_id": "char_001",
        "character_name": "Aria",
        "message": "We should explore this castle carefully...",
        "emotion": "cautious",
        "action": "gripping sword hilt"
      },
      {
        "character_id": "char_002", 
        "character_name": "Thomas",
        "message": "Agreed, I sense powerful magical auras.",
        "emotion": "alert",
        "action": "nodding in agreement"
      }
    ]
  }
}
```

### Simulate Character Dialogue

Simulate multi-round automatic dialogue between characters.

```http
POST /api/interactions/simulate
```

**Request Body:**
```json
{
  "scene_id": "scene_001",
  "character_ids": ["char_001", "char_002", "char_003"],
  "topic": "Formulating exploration strategy",
  "rounds": 5,
  "style": "collaborative"
}
```

### Get Conversation History

Get conversation history records of a scene.

```http
GET /api/conversations/{scene_id}?limit=50&offset=0&character_id=char_001
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Return count limit |
| `offset` | integer | Offset |
| `character_id` | string | Filter specific character |

---

## ⚙️ Settings Management API

### Get System Settings

Get current system configuration.

```http
GET /api/settings
```

**Response Example:**
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

### Update System Settings

Update system configuration (requires admin privileges).

```http
POST /api/settings
```

**Request Body:**
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

### Test Connection

Test connection status of AI service providers.

```http
POST /api/settings/test-connection
```

**Request Body:**
```json
{
  "provider": "openai",
  "model": "gpt-4"
}
```

**Response Example:**
```json
{
  "success": true,
  "data": {
    "provider": "openai",
    "model": "gpt-4",
    "status": "connected",
    "response_time": 1250,
    "test_message": "Connection test successful"
  }
}
```

---

## 👤 User System API

### Get User Profile

Get complete user profile information.

```http
GET /api/users/{user_id}
```

**Response Example:**
```json
{
  "success": true,
  "data": {
    "id": "user_123",
    "username": "player01",
    "display_name": "Adventure Player",
    "bio": "Love fantasy adventures",
    "avatar": "avatar_url",
    "created_at": "2025-06-20T10:00:00Z",
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

### Update User Profile

Update user profile information.

```http
PUT /api/users/{user_id}
```

**Request Body:**
```json
{
  "display_name": "New Display Name",
  "bio": "Updated bio",
  "preferences": {
    "creativity_level": "expansive",
    "auto_save": false
  }
}
```

### Get User Preferences

Get user preferences settings.

```http
GET /api/users/{user_id}/preferences
```

### Update User Preferences

Update user preferences.

```http
PUT /api/users/{user_id}/preferences
```

**Request Body:**
```json
{
  "creativity_level": "balanced",
  "language": "zh-cn",
  "auto_save": true,
  "notification_enabled": true,
  "theme": "dark"
}
```

### Add User Item

Add a custom item for the user.

```http
POST /api/users/{user_id}/items
```

**Request Body:**
```json
{
  "name": "Magic Crystal",
  "description": "Crystal containing ancient magic",
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

### Get User Skills

Get user's custom skills.

```http
GET /api/users/{user_id}/skills?category=magic&available_only=true
```

### Add User Skill

Add a custom skill for the user.

```http
POST /api/users/{user_id}/skills
```

**Request Body:**
```json
{
  "name": "Telepathy",
  "description": "Ability to read others' thoughts",
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

### Update User Item

Update a specific user item.

```http
PUT /api/users/{user_id}/items/{item_id}
```

### Delete User Item

Delete a specific user item.

```http
DELETE /api/users/{user_id}/items/{item_id}
```

## 📤 Export Features API

### Export Scene Data

Export complete scene data in various formats.

```http
GET /api/scenes/{scene_id}/export/scene?format=json&include_conversations=true
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `format` | string | Export format: `json`, `markdown`, `html` |
| `include_conversations` | boolean | Whether to include conversation history |

**Response Example:**
```json
{
  "file_path": "/exports/scene_001_20250620.json",
  "content": "...",
  "format": "json",
  "size": 2048,
  "timestamp": "2025-06-20T10:30:00Z"
}
```

### Export Interactions

Export interaction summary.

```http
GET /api/scenes/{scene_id}/export/interactions?format=markdown
```

### Export Story Document

Export story as a readable document.

```http
GET /api/scenes/{scene_id}/export/story?format=html
```

## 🔄 WebSocket API

### Scene WebSocket Connection

Connect to a scene for real-time updates.

```javascript
// Connect to scene WebSocket
const ws = new WebSocket('ws://localhost:8080/ws/scene/scene_001?user_id=user123');

// Listen for messages
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

// Send character interaction
ws.send(JSON.stringify({
    type: 'character_interaction',
    character_id: 'char_001',
    message: 'Hello everyone!'
}));
```

**Message Types:**

| Type | Description | Direction |
|------|-------------|-----------|
| `character_interaction` | Character message | Client → Server |
| `story_choice` | Story choice selection | Client → Server |
| `user_status_update` | User status update | Client → Server |
| `conversation:new` | New conversation | Server → Client |
| `story:choice_made` | Story choice result | Server → Client |
| `user:presence` | User presence update | Server → Client |

### User Status WebSocket

Connect for user status updates.

```javascript
const statusWs = new WebSocket('ws://localhost:8080/ws/user/status?user_id=user123');
```

---
## 🛠️ SDK Examples

### JavaScript SDK (Enhanced)

```javascript
class SceneIntruderAPI {
    // ... existing methods ...

    // Story System APIs
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

    // Export APIs
    async exportSceneData(sceneId, format = 'json', includeConversations = false) {
        return this.request(`/scenes/${sceneId}/export/scene?format=${format}&include_conversations=${includeConversations}`);
    }

    // User Management APIs
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

    // WebSocket connection helper
    connectToScene(sceneId, userId) {
        const ws = new WebSocket(`ws://localhost:8080/ws/scene/${sceneId}?user_id=${userId}`);
        
        ws.onopen = () => console.log('Connected to scene WebSocket');
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
}
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
    
    # Scene management
    def create_scene(self, name: str, text_content: str, **kwargs) -> Dict[str, Any]:
        data = {
            "name": name,
            "text_content": text_content,
            **kwargs
        }
        return self.request("/scenes", "POST", data)
    
    def get_scene(self, scene_id: str) -> Dict[str, Any]:
        return self.request(f"/scenes/{scene_id}")
    
    # Character interaction
    def chat_with_character(self, scene_id: str, character_id: str, 
                          message: str) -> Dict[str, Any]:
        data = {
            "scene_id": scene_id,
            "character_id": character_id,
            "message": message,
            "include_emotion": True
        }
        return self.request("/chat", "POST", data)

    # User items
    def get_user_items(self, user_id: str) -> Dict[str, Any]:
        return self.request(f"/users/{user_id}/items")
    
    def add_user_item(self, user_id: str, item_data: Dict[str, Any]) -> Dict[str, Any]:
        return self.request(f"/users/{user_id}/items", "POST", item_data)

# Usage example
api = SceneIntruderAPI()

# Create scene
scene = api.create_scene(
    name="Python Test Scene",
    text_content="This is a test story...",
    creativity_level="BALANCED"
)

# Chat with character
response = api.chat_with_character(
    scene["data"]["scene_id"],
    "char_001",
    "Hello, nice to meet you!"
)

print(f"Character reply: {response['data']['message']}")
print(f"Emotion state: {response['data']['emotion']}")
```

### cURL Examples

```bash
# Create scene
curl -X POST http://localhost:8080/api/scenes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fantasy Adventure",
    "text_content": "In an ancient castle, young wizards are learning magic...",
    "creativity_level": "EXPANSIVE"
  }'

# Chat with character
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "scene_id": "scene_123",
    "character_id": "char_001",
    "message": "Professor, what is today'\''s magic lesson?",
    "include_emotion": true
  }'

# Get user items
curl -X GET "http://localhost:8080/api/users/user_123/items?category=weapon"

# Add user item
curl -X POST http://localhost:8080/api/users/user_123/items \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fire Sword",
    "description": "A sword that burns with eternal flame",
    "category": "weapon",
    "rarity": "legendary"
  }'

# Story System Examples
# Get story data
curl -X GET http://localhost:8080/api/scenes/scene_001/story

# Make story choice
curl -X POST http://localhost:8080/api/scenes/scene_001/story/choice \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node_001",
    "choice_id": "choice_a"
  }'

# Export scene data
curl -X GET "http://localhost:8080/api/scenes/scene_001/export/scene?format=markdown&include_conversations=true"

# User Profile Management
# Get user profile
curl -X GET http://localhost:8080/api/users/user_123

# Update user preferences
curl -X PUT http://localhost:8080/api/users/user_123/preferences \
  -H "Content-Type: application/json" \
  -d '{
    "creativity_level": "balanced",
    "language": "zh-cn",
    "auto_save": true
  }'

# LLM Configuration
# Get LLM status
curl -X GET http://localhost:8080/api/llm/status

# Update LLM config
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

## 📋 API Changelog

### v1.0.0 (2025-06-20)
- Initial API version release
- Support for scene creation and management
- Character interaction system implementation
- Story branching functionality added
- Multi-LLM provider support integrated

---

## 🔗 Related Links

- [Main Documentation](../README.md)
- [Deployment Guide](deployment.md)
- [GitHub Repository](https://github.com/Corphon/SceneIntruderMCP)
- [Issue Reporting](https://github.com/Corphon/SceneIntruderMCP/issues)

---

<div align="center">

**📚 Need Help? Feel free to check our documentation or submit an issue!**

Made with ❤️ by SceneIntruderMCP Team

</div>
