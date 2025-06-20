# SceneIntruderMCP

<div align="center">

![SceneIntruderMCP Logo](static/images/logo.png)

**🎭 AI驱动的沉浸式互动叙事平台**

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/Corphon/SceneIntruderMCP)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow.svg)](https://codecov.io)

[English](README.md) | 简体中文

</div>

## 🌟 项目简介

SceneIntruderMCP 是一个革命性的AI驱动互动叙事平台，它将传统的文本分析与现代AI技术相结合，为用户提供前所未有的沉浸式角色扮演和故事创作体验。

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

#### 🎮 **游戏化体验**
- **用户定制**: 自定义道具和技能系统
- **创意控制**: 3级创意程度控制 (严格/平衡/扩展)
- **进度追踪**: 实时故事完成度和统计分析

#### 🔗 **多LLM支持**
- **OpenAI GPT**: GPT-3.5/4/4o 系列
- **Anthropic Claude**: Claude-3/3.5 系列
- **DeepSeek**: 中文优化模型
- **Google Gemini**: Gemini-2.0 系列
- **开源模型**: 通过 OpenRouter/GitHub Models 支持

## 🏗️ 技术架构

### 📁 项目结构

```
SceneIntruderMCP/
├── cmd/
│   └── server/           # 应用程序入口
│       └── main.go
├── internal/
│   ├── api/              # HTTP API 路由和处理器
│   ├── app/              # 应用程序核心逻辑
│   ├── config/           # 配置管理
│   ├── di/               # 依赖注入
│   ├── llm/              # LLM提供商抽象层
│   │   └── providers/    # 各种LLM提供商实现
│   ├── models/           # 数据模型定义
│   ├── services/         # 业务逻辑服务
│   └── storage/          # 存储抽象层
├── static/
│   ├── css/              # 样式文件
│   ├── js/               # 前端JavaScript
│   └── images/           # 静态图片
├── web/
│   └── templates/        # HTML模板
├── data/                 # 数据存储目录
│   ├── scenes/           # 场景数据
│   ├── stories/          # 故事数据
│   ├── users/            # 用户数据
│   └── exports/          # 导出文件
└── logs/                 # 应用日志
```

### 🔧 核心技术栈

- **后端**: Go 1.21+, Gin Web Framework
- **AI集成**: 多LLM提供商支持，统一抽象接口
- **存储**: 基于文件系统的JSON存储，支持扩展到数据库
- **前端**: 原生JavaScript + HTML/CSS，响应式设计
- **部署**: 容器化支持，云原生架构

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
```bash
# 复制配置模板
cp data/config.json.example data/config.json

# 编辑配置文件，添加API密钥
nano data/config.json
```

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

## 📖 使用指南

### 🎬 创建场景

1. **上传文本**: 支持小说、剧本、故事等多种文本格式
2. **AI分析**: 系统自动提取角色、场景、物品等要素
3. **场景生成**: 创建可交互的场景环境

### 🎭 角色互动

1. **选择角色**: 从分析出的角色中选择互动对象
2. **自然对话**: 与AI角色进行自然语言对话
3. **情感反馈**: 观察角色的情绪、动作和表情变化

### 📚 故事分支

1. **动态选择**: AI根据当前情况生成4种类型的选择
2. **故事发展**: 基于选择推进非线性故事情节
3. **分支管理**: 支持故事回溯和多分支探索

### 📊 数据导出

1. **交互记录**: 导出完整的对话历史
2. **故事文档**: 生成结构化的故事文档
3. **统计分析**: 角色互动和故事进展统计

## 🛠️ API文档

### 🔗 核心接口

#### 场景管理
```http
POST   /api/scenes                    # 创建场景
GET    /api/scenes                    # 获取场景列表
GET    /api/scenes/{id}               # 获取场景详情
DELETE /api/scenes/{id}               # 删除场景
```

#### 角色互动
```http
POST   /api/scenes/{id}/characters/{cid}/chat    # 与角色对话
GET    /api/scenes/{id}/characters               # 获取场景角色
POST   /api/characters/interaction               # 角色间互动
```

#### 故事系统
```http
GET    /api/scenes/{id}/story                    # 获取故事数据
POST   /api/scenes/{id}/story/choice             # 做出故事选择
POST   /api/scenes/{id}/story/branch             # 创建故事分支
```

#### 用户系统
```http
GET    /api/users/{id}                           # 获取用户信息
PUT    /api/users/{id}/preferences               # 更新用户偏好
POST   /api/users/{id}/items                     # 添加用户道具
POST   /api/users/{id}/skills                    # 添加用户技能
```

详细API文档请参考: [API Documentation](docs/api.md)

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

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

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
