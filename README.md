# SceneIntruderMCP

<div align="center">

![SceneIntruderMCP Logo](temp/logo.png)

**AI-native storytelling workspace for scenes, comics, scripts, and video**

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-apache-green.svg)](LICENSE)

English | [简体中文](README_CN.md)

</div>

## What this project is now

SceneIntruderMCP is no longer just a lightweight scene-analysis demo. The current product is a unified creative workspace built around one Go backend and one React SPA, with four connected modules:

1. **Interactive scenes** — analyze text into scene data, characters, items, context, and branching story flows.
2. **Comics Studio** — a 5-step workflow for analysis, prompts, key elements, references, image generation, and export.
3. **Video Studio** — build timeline data from comics results, generate clip assets asynchronously, inspect recovery state, and export bundles.
4. **New Script** — create writing projects, generate initial drafts, revise chapters, and export manuscripts.

The system also centralizes **LLM / Vision / Video provider configuration**, long-running task tracking via **SSE**, and file-based persistence under `data/`.

## Current capability map

### Backend and runtime

- Go + Gin server
- SPA hosting from the same binary
- Unified config in `data/config.json`
- Encrypted API key storage via `data/.encryption_key` or `CONFIG_ENCRYPTION_KEY`
- SSE progress endpoint: `GET /api/progress/:taskID`
- Plain WebSocket endpoints for scene/user realtime channels
- File-based storage for scenes, stories, comics, scripts, exports, and users

### Frontend workspaces

- `/` — scenes home
- `/settings` — LLM, Vision, Video settings
- `/scenes/:id` — scene detail
- `/scenes/:id/story` — story mode
- `/scenes/:id/comic` — Comics Studio
- `/scenes/:id/comic/video` — Video Studio
- `/scripts` / `/scripts/:id` — script workspace

### LLM providers officially wired in backend

The backend currently registers these providers:

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

Notes:

- Reasoning / thinking mode is now **default-off** across the LLM layer for structured analysis safety.
- Provider-specific default suppression is applied where supported, including Google, Qwen, and NVIDIA.
- The frontend still contains an `ollama` option in Settings UI, but it is **not** part of the current backend-supported provider matrix and is therefore not documented here as an officially available path.

### Vision providers

- `placeholder`
- `sdwebui`
- `dashscope`
- `gemini`
- `ark`
- `openai`
- `glm`

Default model and model catalog are delivered through `GET /api/settings` via `vision_default_model`, `vision_models`, and `vision_model_providers`.

### Video providers

- `dashscope`
- `kling`
- `google`
- `vertex`
- `ark`
- `mock`

Default video model routing is also exposed through `GET /api/settings` via `video_default_model`, `video_models`, and `video_model_providers`.

## Architecture overview

```text
frontend (React + Vite + MUI)
                │
                ├─ REST (/api/*)
                ├─ SSE  (/api/progress/:taskID)
                └─ WS   (/ws/*)
                                │
backend (Go + Gin)
                │
                ├─ config / auth / rate limit / API handlers
                ├─ LLM / Vision / Video / Story / Script / Comic services
                └─ file storage under data/
```

Core source directories:

- `cmd/server` — server entry
- `internal/api` — router, handlers, middleware
- `internal/app` — application bootstrapping and provider registration
- `internal/config` — runtime config and encryption handling
- `internal/llm` — provider abstraction and reasoning control
- `internal/services` — business logic
- `internal/vision` — vision providers
- `frontend` — SPA client

## Quick start

### Prerequisites

- Go 1.21+
- Node.js 18+
- npm 9+
- At least one usable provider credential

### 1. Install dependencies

```bash
go mod download
cd frontend
npm install
cd ..
```

### 2. Build frontend assets

```bash
cd frontend
npm run build
cd ..
```

### 3. Start the server

```bash
go run ./cmd/server
```

Default address: `http://localhost:8080`

### 4. Open the app

- Home: `http://localhost:8080/`
- Settings: `http://localhost:8080/settings`

## Configuration model

The runtime config is persisted to `data/config.json`.

Important top-level fields:

- `llm_provider`, `llm_config`
- `vision_provider`, `vision_default_model`, `vision_config`
- `video_provider`, `video_default_model`, `video_config`
- `vision_models`, `vision_model_providers`
- `video_models`, `video_model_providers`

Minimal example:

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

### Encryption notes

- If `CONFIG_ENCRYPTION_KEY` is absent in development mode, the app creates `data/.encryption_key` automatically.
- Keep `data/.encryption_key` together with `data/config.json`.
- Deleting the key file invalidates previously encrypted credentials.

## Recommended first-run path

1. Build frontend assets.
2. Start backend.
3. Open Settings and configure one working LLM provider.
4. Optionally configure Vision and Video providers.
5. Create a scene or a standalone comic workspace.

## Key operational behaviors

### Long-running jobs

Analysis, prompt generation, image generation, script generation, and video generation are asynchronous. The usual pattern is:

1. Start a job and receive `task_id`
2. Subscribe to `GET /api/progress/:taskID`
3. Fetch final result from the corresponding `GET` endpoint

### Guest vs authenticated usage

- Many scene-oriented routes degrade to `console_user` when auth is missing or invalid.
- User-scoped routes under `/api/users/:user_id/...` require authenticated ownership.
- Scripts routes require authenticated access.

### Video provider note

Some video providers need a publicly reachable reference image URL. In practice, that means `video_config.public_base_url` should usually be configured for deployed environments.

## Development commands

Backend:

```bash
go test ./...
go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm run dev
npm test
npm run lint
npm run build
```

## Documentation index

- [API reference](docs/api.md)
- [Deployment guide](docs/deployment.md)
- [Frontend developer guide](docs/frontend_dev.md)
- [中文 API 文档](docs/api_cn.md)
- [中文部署文档](docs/deployment_cn.md)

## Current scope boundary

This repository already contains significantly more than an initial prototype. The maintained documentation now treats it as:

- a **multi-workspace creative platform**,
- with **provider-configurable AI services**,
- **job-based asynchronous generation**,
- and **documented operational deployment requirements**.

That is the baseline future changes should preserve.

<!--
## 🛠️ API Documentation

### 🔗 Actually Available API Endpoints

#### Scene Management
```http
GET    /api/scenes                      # Get scene list
POST   /api/scenes                      # Create scene  
POST   /api/scenes/shell                # Create a standalone comic workspace shell
GET    /api/scenes/{id}                 # Get scene details
GET    /api/scenes/{id}/characters      # Get scene characters
GET    /api/scenes/{id}/conversations   # Get scene conversations
GET    /api/scenes/{id}/aggregate       # Get scene aggregate data
```

#### Comics v2
```http
POST   /api/scenes/{id}/comic/analysis                 # Start comic analysis (supports source_text)
GET    /api/scenes/{id}/comic/analysis                 # Get analysis result
POST   /api/scenes/{id}/comic/prompts                  # Start frame prompt generation
GET    /api/scenes/{id}/comic/prompts                  # Get all frame prompts
POST   /api/scenes/{id}/comic/key_elements             # Start key element extraction
GET    /api/scenes/{id}/comic/key_elements             # Get key elements
POST   /api/scenes/{id}/comic/references               # Upload reference images
POST   /api/scenes/{id}/comic/generate                 # Start image generation
POST   /api/scenes/{id}/comic/frames/{frame_id}/regenerate # Regenerate a frame
GET    /api/scenes/{id}/comic/images/{frame_id}        # Get generated PNG
GET    /api/scenes/{id}/comic                          # Get comic overview
GET    /api/scenes/{id}/comic/export?format=zip|html   # Export comic
```

#### Scripts
```http
GET    /api/scripts                    # List script projects
POST   /api/scripts                    # Create script project
GET    /api/scripts/{id}               # Get script details
POST   /api/scripts/{id}/generate      # Start initial generation
POST   /api/scripts/{id}/command       # Execute assist command
PUT    /api/scripts/{id}/chapter_draft # Save chapter draft
PUT    /api/scripts/{id}/draft         # Save/replace active draft
POST   /api/scripts/{id}/rewind        # Rewind to a previous draft
GET    /api/scripts/{id}/export        # Export script
```

#### Story System
```http
GET    /api/scenes/{id}/story           # Get story data
POST   /api/scenes/{id}/story/choice    # Make story choice
POST   /api/scenes/{id}/story/advance   # Advance story
POST   /api/scenes/{id}/story/rewind    # Rewind story
GET    /api/scenes/{id}/story/branches  # Get story branches
POST   /api/scenes/{id}/story/rewind    # Rewind story to specific node
```

#### Export Functions
```http
GET    /api/scenes/{id}/export/scene        # Export scene data
GET    /api/scenes/{id}/export/interactions # Export interactions
GET    /api/scenes/{id}/export/story        # Export story document
```

#### Interaction Aggregation
```http
POST   /api/interactions/aggregate         # Process aggregated interactions
GET    /api/interactions/{scene_id}        # Get character interactions
GET    /api/interactions/{scene_id}/{character1_id}/{character2_id} # Get character-to-character interactions
```

#### Scene Aggregation
```http
GET    /api/scenes/{id}/aggregate          # Get comprehensive scene data with options
```

#### Batch Operations
```http
POST   /api/scenes/{id}/story/batch        # Batch story operations
```

#### User Management
```http
GET    /api/users/{user_id}                # Get user profile
PUT    /api/users/{user_id}                # Update user profile
GET    /api/users/{user_id}/preferences    # Get user preferences
PUT    /api/users/{user_id}/preferences    # Update user preferences
```

#### User Items and Skills Management
```http
# User Items
GET    /api/users/{user_id}/items           # Get user items
POST   /api/users/{user_id}/items           # Add user item
GET    /api/users/{user_id}/items/{item_id} # Get specific item
PUT    /api/users/{user_id}/items/{item_id} # Update user item
DELETE /api/users/{user_id}/items/{item_id} # Delete user item

# User Skills
GET    /api/users/{user_id}/skills           # Get user skills
POST   /api/users/{user_id}/skills           # Add user skill
GET    /api/users/{user_id}/skills/{skill_id} # Get specific skill
PUT    /api/users/{user_id}/skills/{skill_id} # Update user skill
DELETE /api/users/{user_id}/skills/{skill_id} # Delete user skill
```

#### Configuration and Health Checks
```http
GET    /api/config/health                   # Get configuration health status
GET    /api/config/metrics                  # Get configuration metrics
GET    /api/settings                        # Get system settings
POST   /api/settings                        # Update system settings
POST   /api/settings/test-connection        # Test connection
```

#### WebSocket Management
```http
GET    /api/ws/status                       # Get WebSocket connection status
POST   /api/ws/cleanup                      # Clean up expired WebSocket connections
```

#### Text Analysis & File Upload
```http
POST   /api/analyze                     # Analyze text content
GET    /api/progress/{taskID}           # Get analysis progress
POST   /api/cancel/{taskID}             # Cancel analysis task
POST   /api/upload                      # Upload file
```

#### Character Interaction & Chat
```http
POST   /api/chat                        # Basic chat with characters
POST   /api/chat/emotion                # Chat with emotion analysis
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

GET    /api/llm/status                  # Get LLM service status
GET    /api/llm/models                  # Get available models
PUT    /api/llm/config                  # Update LLM configuration
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

### 📋 **API Usage Examples**

#### Story Interaction Flow
```javascript
// 1. Get story data
const storyData = await fetch('/api/scenes/scene123/story');

// 2. Make a choice
const choiceResult = await fetch('/api/scenes/scene123/story/choice', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        node_id: 'node_1',
        choice_id: 'choice_a'
    })
});

// 3. Export story
const storyExport = await fetch('/api/scenes/scene123/export/story?format=markdown');
```

#### Character Interaction
```javascript
// 1. Basic chat
const chatResponse = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        scene_id: 'scene123',
        character_id: 'char456',
        message: 'Hello, how are you?'
    })
});

// 2. Trigger character interaction
const interaction = await fetch('/api/interactions/trigger', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        scene_id: 'scene123',
        character_ids: ['char1', 'char2'],
        topic: 'Discussing the mysterious artifact'
    })
});
```

#### User Customization
```javascript
// 1. Add custom item
const newItem = await fetch('/api/users/user123/items', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        name: 'Magic Sword',
        description: 'A legendary sword with mystical powers',
        type: 'weapon',
        properties: { attack: 50, magic: 30 }
    })
});

// 2. Add skill
const newSkill = await fetch('/api/users/user123/skills', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        name: 'Fireball',
        description: 'Cast a powerful fireball spell',
        type: 'magic',
        level: 3
    })
});
```

### 🔗 **WebSocket Integration**

#### Scene WebSocket Connection
```javascript
// Connect to scene WebSocket
const sceneWs = new WebSocket(`ws://localhost:8080/ws/scene/scene123?user_id=user456`);

sceneWs.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Scene update:', data);
};

// Send character interaction
sceneWs.send(JSON.stringify({
    type: 'character_interaction',
    character_id: 'char123',
    message: 'Hello everyone!'
}));

// Send story choice
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

#### User Status WebSocket
```javascript
// Connect to user status WebSocket
const statusWs = new WebSocket(`ws://localhost:8080/ws/user/status?user_id=user456`);

statusWs.onmessage = (event) => {
    const data = JSON.parse(event.data);
    switch(data.type) {
        case 'heartbeat':
            console.log('Connection alive');
            break;
        case 'user_status_update':
            console.log('User status changed:', data.status);
            break;
        case 'error':
            console.error('WebSocket error:', data.error);
            break;
        default:
            console.log('Received:', data);
    }
};
```

#### Supported WebSocket Message Types
- **character_interaction**: Character-to-character interactions
- **story_choice**: Story decision-making events
- **user_status_update**: User presence and status updates
- **conversation:new**: New conversation events
- **heartbeat**: Connection health checks
- **pong**: Heartbeat response messages
- **error**: Error notifications

#### Client-Side Realtime Management
The application uses RealtimeManager class for handling WebSocket communications:
```javascript
// Initialize scene realtime functionality
await window.realtimeManager.initSceneRealtime('scene_123');

// Send character interaction
window.realtimeManager.sendCharacterInteraction('scene_123', 'character_456', 'Hello!');

// Subscribe to story events
window.realtimeManager.on('story:event', (data) => {
    // Handle story updates
    console.log('Story event:', data);
});

// Get connection status
const status = window.realtimeManager.getConnectionStatus();
console.log('WebSocket status:', status);
```

### 📊 **Response Formats**

#### Standard Success Response
```json
{
    "success": true,
    "data": {
        // Response data
    },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

#### Error Response
```json
{
    "success": false,
    "error": "Error message description",
    "code": "ERROR_CODE",
    "timestamp": "2024-01-01T12:00:00Z"
}
```

#### Export Response
```json
{
    "file_path": "/exports/story_20240101_120000.md",
    "content": "# Story Export\n\n...",
    "format": "markdown",
    "size": 1024,
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### 🛡️ **Authentication & Security**

Currently, the API uses session-based authentication for user management. For production deployment, consider implementing:

- **JWT Authentication**: Token-based authentication for API access
- **Rate Limiting**: API call frequency limits
- **Input Validation**: Strict parameter validation and sanitization
- **HTTPS Only**: Force HTTPS for all production traffic

For detailed API documentation, see: [API Documentation](docs/api.md)

## 🧪 Development Guide

### 🏃‍♂️ Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/services/...
```

### 🔧 Adding New LLM Providers

1. **Implement Interface**: Create new provider in `internal/llm/providers/`
2. **Register Provider**: Register in `init()` function
3. **Add Configuration**: Update configuration file template
4. **Write Tests**: Add corresponding unit tests

### 📁 Code Structure Explanation

- **models/**: Data models defining core entities in the system
- **services/**: Business logic layer handling core functionality
- **api/**: HTTP handlers exposing RESTful APIs
- **llm/**: LLM abstraction layer supporting multiple AI providers

## 📈 Performance Optimization

### 🚀 System Performance

- **Concurrent Processing**: Support multiple simultaneous users
- **Caching Mechanism**: Intelligent caching of LLM responses
- **Memory Optimization**: Load on demand, prevent memory leaks
- **File Compression**: Automatic compression of historical data

### 📊 Monitoring Metrics

- **API Usage Statistics**: Request count and token consumption
- **Response Time**: AI model response speed monitoring
- **Error Rate**: System and API error tracking
- **Resource Usage**: CPU and memory usage monitoring

## 🔐 Security Considerations

### 🛡️ Data Security

- **API Keys**: Secure storage with environment variable support
- **User Data**: Local storage with complete privacy control
- **Access Control**: User session and permission management support
- **Data Backup**: Automatic backup of important data

### 🔒 Network Security

- **HTTPS Support**: HTTPS recommended for production environments
- **CORS Configuration**: Secure cross-origin resource sharing configuration
- **Input Validation**: Strict user input validation and sanitization

### 🔐 Data Security & API Key Encryption

- **AES-GCM Encryption**: API keys are securely encrypted using AES-GCM algorithm before storage
- **Environment Variable Priority**: API keys are primarily loaded from environment variables (e.g., `OPENAI_API_KEY`) 
- **Encrypted Storage**: When stored in configuration files, API keys are kept in encrypted form in `EncryptedLLMConfig` field
- **Runtime Decryption**: API keys are decrypted only when needed for API calls
- **Automatic Migration**: Legacy unencrypted API keys are automatically migrated to encrypted storage
- **Secure Backward Compatibility**: The system handles transition from unencrypted to encrypted API key storage
- **Configuration Security**: The encryption key should be set as `CONFIG_ENCRYPTION_KEY` environment variable for optimal security
- **Fallback Protection**: Includes fallback mechanisms to prevent storing API keys as plain text
- **Key Derivation**: In absence of environment-provided encryption keys, the system safely derives encryption keys from multiple entropy sources

## 🤝 Contributing

We welcome all forms of contributions!

### 📝 Ways to Contribute

1. **Bug Reports**: Use GitHub Issues to report problems
2. **Feature Suggestions**: Propose ideas and suggestions for new features
3. **Code Contributions**: Submit Pull Requests
4. **Documentation Improvements**: Help improve documentation and examples

### 🔧 Development Process

1. Fork the project repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push branch: `git push origin feature/amazing-feature`
5. Create Pull Request

### 📋 Code Standards

- Follow official Go coding style
- Add necessary comments and documentation
- Write unit tests covering new features
- Ensure all tests pass

## 📄 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details

## 🙏 Acknowledgments

### 🎯 Core Technologies

- [Go](https://golang.org/) - High-performance programming language
- [Gin](https://gin-gonic.com/) - Lightweight web framework
- [OpenAI](https://openai.com/) - GPT series models
- [Anthropic](https://anthropic.com/) - Claude series models

### 👥 Community Support

Thanks to all developers and users who have contributed to this project!

## 📞 Contact Us

- **Project Homepage**: [GitHub Repository](https://github.com/Corphon/SceneIntruderMCP)
- **Issue Reports**: [GitHub Issues](https://github.com/Corphon/SceneIntruderMCP/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/Corphon/SceneIntruderMCP/discussions)
- **Email Contact**: [project@sceneintruder.dev](mailto:songkf@foxmail.com)

---

<div align="center">

**🌟 If this project helps you, please consider giving it a Star! 🌟**

Made with ❤️ by SceneIntruderMCP Team

</div>
-->
