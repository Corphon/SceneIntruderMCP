# SceneIntruderMCP

<div align="center">

![SceneIntruderMCP Logo](static/images/logo.png)

**🎭 AI-Powered Immersive Interactive Storytelling Platform**

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/Corphon/SceneIntruderMCP)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow.svg)](https://codecov.io)

English | [简体中文](README_CN.md)

</div>

## 🌟 Project Overview

SceneIntruderMCP is a revolutionary AI-driven interactive storytelling platform that combines traditional text analysis with modern AI technology, providing users with an unprecedented immersive role-playing and story creation experience.

### ✨ Core Features

#### 🧠 **Intelligent Text Analysis**
- **Multi-dimensional Parsing**: Automatically extract scenes, characters, items, and plot elements
- **Bilingual Support**: Perfect support for intelligent recognition and processing of Chinese and English content
- **Deep Analysis**: Professional-grade text type identification based on literary theory

#### 🎭 **AI Character System**
- **Emotional Intelligence**: 8-dimensional emotional analysis (emotion, action, expression, tone, etc.)
- **Character Consistency**: Maintain long-term memory and personality traits
- **Dynamic Interaction**: Intelligently triggered automatic dialogues between characters

#### 📖 **Dynamic Story Engine**
- **Non-linear Narrative**: Support complex story branching and timeline management
- **Intelligent Choice Generation**: AI dynamically creates 4 types of choices based on context (Action/Dialogue/Investigation/Strategy)
- **Story Rewind**: Complete timeline rollback and state management

#### 🎮 **Gamified Experience**
- **User Customization**: Custom items and skills system
- **Creativity Control**: 3-level creativity control (Strict/Balanced/Expansive)
- **Progress Tracking**: Real-time story completion and statistical analysis

#### 🔗 **Multi-LLM Support**
- **OpenAI GPT**: GPT-3.5/4/4o series
- **Anthropic Claude**: Claude-3/3.5 series
- **DeepSeek**: Chinese-optimized models
- **Google Gemini**: Gemini-2.0 series
- **Grok**: xAI's Grok models
- **Mistral**: Mistral series models
- **Qwen**: Alibaba Cloud Qwen series
- **GitHub Models**: Via GitHub Models platform
- **OpenRouter**: Open source model aggregation platform
- **GLM**: Zhipu AI's GLM series

## 🏗️ Technical Architecture

### 📁 Project Structure

```
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
├── static/
│   ├── css/              # Style files
│   ├── js/               # Frontend JavaScript
│   └── images/           # Static images
├── web/
│   └── templates/        # HTML templates
├── data/                 # Data storage directory
│   ├── scenes/           # Scene data
│   ├── stories/          # Story data
│   ├── users/            # User data
│   └── exports/          # Export files
└── logs/                 # Application logs
```

### 🔧 Core Technology Stack

- **Backend**: Go 1.21+, Gin Web Framework
- **AI Integration**: Multi-LLM provider support with unified abstraction interface
- **Storage**: File system-based JSON storage with database extension support
- **Frontend**: Vanilla JavaScript + HTML/CSS, responsive design
- **Deployment**: Containerization support, cloud-native architecture

## 🚀 Quick Start

### 📋 System Requirements

- Go 1.21 or higher
- At least one LLM API key (OpenAI/Claude/DeepSeek, etc.)
- 2GB+ available memory
- Operating System: Windows/Linux/macOS

### 📦 Installation Steps

1. **Clone the Project**
```bash
git clone https://github.com/Corphon/SceneIntruderMCP.git
cd SceneIntruderMCP
```

2. **Install Dependencies**
```bash
go mod download
```

3. **Configure Environment**
```bash
# Copy configuration template
cp data/config.json.example data/config.json

# Edit configuration file and add API keys
nano data/config.json
```

4. **Start Service**
```bash
# Development mode
go run cmd/server/main.go

# Production mode
go build -o sceneintruder cmd/server/main.go
./sceneintruder
```

5. **Access Application**
```
Open browser: http://localhost:8080
```

### ⚙️ Configuration Guide

#### `data/config.json` Configuration Example

```json
{
  "llm": {
    "default_provider": "openai",
    "providers": {
      "openai": {
        "api_key": "your-openai-api-key",
        "base_url": "https://api.openai.com/v1",
        "default_model": "gpt-4"
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

## 📖 User Guide

### 🎬 Creating Scenes

1. **Upload Text**: Support various text formats including novels, scripts, stories
2. **AI Analysis**: System automatically extracts characters, scenes, items, and other elements
3. **Scene Generation**: Create interactive scene environments

### 🎭 Character Interaction

1. **Select Character**: Choose interaction targets from analyzed characters
2. **Natural Dialogue**: Engage in natural language conversations with AI characters
3. **Emotional Feedback**: Observe character emotions, actions, and expression changes

### 📚 Story Branching

1. **Dynamic Choices**: AI generates 4 types of choices based on current situation
2. **Story Development**: Advance non-linear story plots based on choices
3. **Branch Management**: Support story rewind and multi-branch exploration

### 📊 Data Export

1. **Interaction Records**: Export complete dialogue history
2. **Story Documents**: Generate structured story documents
3. **Statistical Analysis**: Character interaction and story progress statistics

## 🛠️ API Documentation

### 🔗 Actually Available API Endpoints

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
POST   /api/scenes/{id}/story/rewind    # Rewind story to specific node
```

#### Export Functions
```http
GET    /api/scenes/{id}/export/scene        # Export scene data
GET    /api/scenes/{id}/export/interactions # Export interactions
GET    /api/scenes/{id}/export/story        # Export story document
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
```

#### User Status WebSocket
```javascript
// Connect to user status WebSocket
const statusWs = new WebSocket(`ws://localhost:8080/ws/user/status?user_id=user456`);

statusWs.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.type === 'heartbeat') {
        console.log('Connection alive');
    }
};
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

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details

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
