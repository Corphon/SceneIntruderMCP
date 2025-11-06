# SceneIntruderMCP 部署指南

本文档提供了在不同环境中部署 SceneIntruderMCP 的详细指南。

## 📋 目录

- [系统要求](#系统要求)
- [开发环境部署](#开发环境部署)
- [生产环境部署](#生产环境部署)
- [Docker 部署](#docker-部署)
- [云平台部署](#云平台部署)
- [配置管理](#配置管理)
- [安全配置](#安全配置)
- [监控和日志](#监控和日志)
- [故障排除](#故障排除)

## 🖥️ 系统要求

### 最低配置
- **CPU**: 2核心
- **内存**: 4GB RAM
- **存储**: 10GB 可用空间
- **操作系统**: Linux/Windows/macOS
- **Go版本**: 1.21+

### 推荐配置
- **CPU**: 4核心+
- **内存**: 8GB+ RAM
- **存储**: 50GB+ SSD
- **网络**: 100Mbps+ 带宽

### 软件依赖
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install git curl build-essential

# CentOS/RHEL
sudo yum update
sudo yum install git curl gcc make

# macOS (使用 Homebrew)
brew install git go
```

## 🔧 开发环境部署

### 1. 克隆和构建

```bash
# 克隆项目
git clone https://github.com/Corphon/SceneIntruderMCP.git
cd SceneIntruderMCP

# 下载依赖
go mod download

# 验证构建
go build -o sceneintruder cmd/server/main.go
```

### 2. 环境配置

```bash
# 应用会自动创建配置文件，无需手动复制
# 首次运行时会在 data/config.json 中生成默认配置

# 可以通过环境变量配置
export PORT=8080
export OPENAI_API_KEY=your-openai-api-key
export DEBUG_MODE=true
```

**基础配置示例**:
```json
{
  "port": "8080",
  "data_dir": "data",
  "static_dir": "static",
  "templates_dir": "web/templates",
  "log_dir": "logs",
  "debug_mode": true,
  "llm_provider": "openai",
  "llm_config": {
    "default_model": "gpt-4o"
  },
  "encrypted_llm_config": {
    "api_key": "<encrypted_api_key_here>"  // API 密钥在存储时会被加密
  }
}
```

**注意**: API 密钥在存储时会被加密，运行时仅在需要进行 API 调用时解密。

### 3. 启动开发服务器

```bash
# 方式一：直接运行
go run cmd/server/main.go

# 方式二：构建后运行
go build -o sceneintruder cmd/server/main.go
./sceneintruder

# 访问应用
open http://localhost:8080
```

## 🚀 生产环境部署

### 1. 编译优化版本

```bash
# 编译生产版本
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o sceneintruder-linux-amd64 \
  cmd/server/main.go

# 验证二进制文件
./sceneintruder-linux-amd64
```

### 2. 系统服务配置

**创建 systemd 服务文件** (`/etc/systemd/system/sceneintruder.service`):

```ini
[Unit]
Description=SceneIntruderMCP AI Interactive Storytelling Platform
After=network.target
Wants=network.target

[Service]
Type=simple
User=sceneintruder
Group=sceneintruder
WorkingDirectory=/opt/sceneintruder
ExecStart=/opt/sceneintruder/sceneintruder
ExecReload=/bin/kill -HUP $MAINPID
KillMode=mixed
KillSignal=SIGTERM
TimeoutSec=30
RestartSec=5
Restart=always

# 安全配置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/sceneintruder/data /opt/sceneintruder/logs

# 环境变量
Environment=GIN_MODE=release
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

### 3. 部署步骤

```bash
# 1. 创建专用用户
sudo useradd -r -s /bin/false sceneintruder

# 2. 创建目录结构
sudo mkdir -p /opt/sceneintruder/{data,logs,static,web}
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder

# 3. 复制文件
sudo cp sceneintruder-linux-amd64 /opt/sceneintruder/sceneintruder
sudo cp -r static/* /opt/sceneintruder/static/
sudo cp -r web/* /opt/sceneintruder/web/

# 4. 设置权限
sudo chmod +x /opt/sceneintruder/sceneintruder
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder

# 5. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable sceneintruder
sudo systemctl start sceneintruder

# 6. 检查状态
sudo systemctl status sceneintruder
```

## 🐳 Docker 部署

### 1. Dockerfile

```dockerfile
# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git ca-certificates

# 复制源码
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o sceneintruder \
    cmd/server/main.go

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -s /bin/sh sceneintruder

WORKDIR /app

# 复制二进制文件和静态资源
COPY --from=builder /app/sceneintruder .
COPY --from=builder /app/static ./static
COPY --from=builder /app/web ./web

# 创建必要目录
RUN mkdir -p data data/scenes temp logs static static/css static/js static/images

USER sceneintruder

EXPOSE 8080

CMD ["./sceneintruder"]
```

### 2. docker-compose.yml

```yaml
version: '3.8'

services:
  sceneintruder:
    build: .
    ports:
      - "8080:8080"
    environment:
      - GIN_MODE=release
      - LOG_LEVEL=info
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    restart: unless-stopped

  # 可选：Nginx 反向代理
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - sceneintruder
    restart: unless-stopped

  # 可选：日志聚合
  fluentd:
    image: fluent/fluentd:latest
    volumes:
      - ./logs:/fluentd/log
      - ./fluentd.conf:/fluentd/etc/fluent.conf
    depends_on:
      - sceneintruder
```

### 3. 部署命令

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f sceneintruder

# 更新部署
docker-compose pull
docker-compose up -d --force-recreate

# 清理
docker-compose down
docker system prune -f
```

## ☁️ 云平台部署

### AWS EC2 部署

```bash
#!/bin/bash
# AWS EC2 用户数据脚本

# 更新系统
yum update -y

# 安装 Go
wget https://golang.org/dl/go1.21.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
source /etc/profile

# 创建应用用户
useradd -r -s /bin/false sceneintruder

# 部署应用
mkdir -p /opt/sceneintruder
cd /opt/sceneintruder

# 下载发布版本
wget https://github.com/Corphon/SceneIntruderMCP/releases/latest/download/sceneintruder-linux-amd64
chmod +x sceneintruder-linux-amd64
mv sceneintruder-linux-amd64 sceneintruder

# 创建配置
cat > data/config.json << EOF
{
  "port": "8080",
  "debug_mode": false,
  "llm_provider": "openai",
  "llm_config": {
    "api_key": "${OPENAI_API_KEY}",
    "default_model": "gpt-4o"
  }
}
EOF

# 设置权限
chown -R sceneintruder:sceneintruder /opt/sceneintruder

# 启动服务
systemctl enable sceneintruder
systemctl start sceneintruder
```

### Google Cloud Platform

```yaml
# app.yaml (App Engine)
runtime: go121

env_variables:
  GIN_MODE: release
  OPENAI_API_KEY: "your-api-key"

automatic_scaling:
  min_instances: 1
  max_instances: 10
  target_cpu_utilization: 0.6

resources:
  cpu: 1
  memory_gb: 2
  disk_size_gb: 10

handlers:
- url: /static
  static_dir: static
  secure: always

- url: /.*
  script: auto
  secure: always
```

### Azure App Service

```yaml
# azure-pipelines.yml
trigger:
- main

pool:
  vmImage: 'ubuntu-latest'

variables:
  buildConfiguration: 'Release'
  azureSubscription: 'your-subscription'
  appName: 'sceneintruder-app'

steps:
- task: GoTool@0
  inputs:
    version: '1.21'

- task: Go@0
  inputs:
    command: 'build'
    arguments: '-o $(Build.ArtifactStagingDirectory)/sceneintruder cmd/server/main.go'

- task: AzureWebApp@1
  inputs:
    azureSubscription: '$(azureSubscription)'
    appType: 'webAppLinux'
    appName: '$(appName)'
    package: '$(Build.ArtifactStagingDirectory)'
```

## ⚙️ 配置管理

### 环境变量配置

```bash
# .env 文件
PORT=8080
DEBUG_MODE=false
DATA_DIR=/opt/sceneintruder/data
LOG_DIR=/opt/sceneintruder/logs

# LLM 配置
LLM_PROVIDER=openai
OPENAI_API_KEY=your-openai-key
ANTHROPIC_API_KEY=your-claude-key
DEEPSEEK_API_KEY=your-deepseek-key

# 安全配置
ALLOWED_ORIGINS=https://yourdomain.com
CORS_ENABLED=true
RATE_LIMIT_ENABLED=true
```

### 多环境配置

```json
// config/production.json
{
  "port": "8080",
  "debug_mode": false,
  "llm_provider": "openai",
  "llm_config": {
    "api_key": "${OPENAI_API_KEY}",
    "default_model": "gpt-4o",
    "timeout": 30,
    "max_retries": 3
  },
  "security": {
    "cors_enabled": true,
    "allowed_origins": ["https://yourdomain.com"],
    "rate_limit": {
      "enabled": true,
      "requests_per_minute": 60
    }
  },
  "monitoring": {
    "metrics_enabled": true,
    "health_check_enabled": true
  }
}
```

## 🔐 安全配置

### 1. API 密钥加密

该应用程序实现了 AES-GCM 加密来保护 API 密钥，确保传输和存储安全：
- **AES-GCM 加密**: API 密钥在存储前使用 AES-GCM 算法安全加密
- **环境变量优先**: API 密钥主要从环境变量加载（例如，`OPENAI_API_KEY`）
- **加密存储**: 在配置文件中存储时，API 密钥保存在 `encrypted_llm_config` 字段的加密形式
- **运行时解密**: API 密钥仅在需要进行 API 调用时解密
- **自动迁移**: 遗留的未加密 API 密钥自动迁移到加密存储
- **配置安全**: 加密密钥应设为 `CONFIG_ENCRYPTION_KEY` 环境变量以获得最佳安全性

### 2. Nginx 反向代理

```nginx
# /etc/nginx/sites-available/sceneintruder
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    # SSL 配置
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers off;

    # 安全头
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload";

    # 限制请求大小
    client_max_body_size 10M;

    # 静态文件
    location /static/ {
        alias /opt/sceneintruder/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API 限流
    location /api/ {
        limit_req zone=api burst=10 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 应用服务
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

# 限流配置
http {
    limit_req_zone $binary_remote_addr zone=api:10m rate=60r/m;
    limit_req_zone $binary_remote_addr zone=general:10m rate=10r/s;
}
```

### 2. 防火墙配置

```bash
# UFW (Ubuntu)
sudo ufw allow ssh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable

# iptables
iptables -A INPUT -p tcp --dport 22 -j ACCEPT
iptables -A INPUT -p tcp --dport 80 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -j ACCEPT
iptables -A INPUT -j DROP
```

### 3. SSL/TLS 配置

```bash
# Let's Encrypt 证书
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d yourdomain.com

# 自动续期
echo "0 12 * * * /usr/bin/certbot renew --quiet" | sudo crontab -
```

## 📊 监控和日志

### 1. 日志配置

```bash
# 日志轮转配置 /etc/logrotate.d/sceneintruder
/opt/sceneintruder/logs/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 sceneintruder sceneintruder
    postrotate
        systemctl reload sceneintruder
    endscript
}
```

### 2. Prometheus 监控

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'sceneintruder'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

### 3. 健康检查

```bash
#!/bin/bash
# health-check.sh

ENDPOINT="http://localhost:8080/"
TIMEOUT=5

response=$(curl -s -w "%{http_code}" -o /dev/null --connect-timeout $TIMEOUT $ENDPOINT)

if [ "$response" = "200" ]; then
    echo "Service is healthy"
    exit 0
else
    echo "Service is unhealthy (HTTP $response)"
    exit 1
fi
```

## 🔧 故障排除

### 常见问题

#### 1. 服务启动失败

```bash
# 检查日志
sudo journalctl -u sceneintruder -f

# 检查配置文件
sudo -u sceneintruder test -f /opt/sceneintruder/data/config.json && echo "配置文件存在"

# 检查端口占用
sudo netstat -tlnp | grep :8080
```

#### 2. API 密钥问题

```bash
# 测试 API 连接
curl -H "Authorization: Bearer your-api-key" \
     https://api.openai.com/v1/models

# 检查配置文件
sudo -u sceneintruder cat /opt/sceneintruder/data/config.json
```

#### 3. 权限问题

```bash
# 检查文件权限
ls -la /opt/sceneintruder/
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder/

# 检查 SELinux (如果启用)
sudo setsebool -P httpd_can_network_connect 1
```

#### 4. 内存不足

```bash
# 检查内存使用
free -h
ps aux | grep sceneintruder
```

### 备份和恢复

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/sceneintruder"
DATA_DIR="/opt/sceneintruder/data"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份
mkdir -p "$BACKUP_DIR"
tar -czf "$BACKUP_DIR/sceneintruder_$DATE.tar.gz" \
    -C "/opt/sceneintruder" \
    data logs

# 保留最近30天的备份
find "$BACKUP_DIR" -name "sceneintruder_*.tar.gz" -mtime +30 -delete

echo "Backup completed: sceneintruder_$DATE.tar.gz"
```

## 📞 支持和帮助

如果在部署过程中遇到问题，请：

1. 查看 [FAQ 文档](docs/faq.md)
2. 检查 [GitHub Issues](https://github.com/Corphon/SceneIntruderMCP/issues)
3. 在 [GitHub Discussions](https://github.com/Corphon/SceneIntruderMCP/discussions) 提问
4. 发送邮件至 [support@sceneintruder.dev](mailto:songkf@foxmail.com)

---

**注意**: 本部署指南基于最新版本编写。对于特定版本的部署，请参考对应版本的文档。
