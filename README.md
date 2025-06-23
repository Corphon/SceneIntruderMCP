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
- **OpenAI GPT**: GPT-4.1/4o series
- **Anthropic Claude**: Claude-4/3.5 series
- **DeepSeek**: Chinese-optimized models
- **Google Gemini**: Gemini-2.5/2.0 series
- **Open Source Models**: Support via OpenRouter/GitHub Models

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

### 🔗 Core Endpoints

#### Scene Management
```http
POST   /api/scenes                    # Create scene
GET    /api/scenes                    # Get scene list
GET    /api/scenes/{id}               # Get scene details
DELETE /api/scenes/{id}               # Delete scene
```

#### Character Interaction
```http
POST   /api/scenes/{id}/characters/{cid}/chat    # Chat with character
GET    /api/scenes/{id}/characters               # Get scene characters
POST   /api/characters/interaction               # Character interactions
```

#### Story System
```http
GET    /api/scenes/{id}/story                    # Get story data
POST   /api/scenes/{id}/story/choice             # Make story choice
POST   /api/scenes/{id}/story/branch             # Create story branch
```

#### User System
```http
GET    /api/users/{id}                           # Get user info
PUT    /api/users/{id}/preferences               # Update user preferences
POST   /api/users/{id}/items                     # Add user items
POST   /api/users/{id}/skills                    # Add user skills
```

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
- **Email Contact**: [songkf@foxmail.com](mailto:songkf@foxmail.com)

---

<div align="center">

**🌟 If this project helps you, please consider giving it a Star! 🌟**

Made with ❤️ by SceneIntruderMCP Team

</div>
