# SceneIntruderMCP Deployment Guide

This document provides detailed deployment instructions for SceneIntruderMCP in various environments.

## 📋 Table of Contents

- [System Requirements](#system-requirements)
- [Development Environment Deployment](#development-environment-deployment)
- [Production Environment Deployment](#production-environment-deployment)
- [Docker Deployment](#docker-deployment)
- [Cloud Platform Deployment](#cloud-platform-deployment)
- [Configuration Management](#configuration-management)
- [Security Configuration](#security-configuration)
- [Monitoring and Logging](#monitoring-and-logging)
- [Troubleshooting](#troubleshooting)

## 🖥️ System Requirements

### Minimum Configuration
- **CPU**: 2 cores
- **Memory**: 4GB RAM
- **Storage**: 10GB available space
- **Operating System**: Linux/Windows/macOS
- **Go Version**: 1.21+

### Recommended Configuration
- **CPU**: 4+ cores
- **Memory**: 8GB+ RAM
- **Storage**: 50GB+ SSD
- **Network**: 100Mbps+ bandwidth

### Software Dependencies
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install git curl build-essential

# CentOS/RHEL
sudo yum update
sudo yum install git curl gcc make

# macOS (using Homebrew)
brew install git go
```

## 🔧 Development Environment Deployment

### 1. Clone and Build

```bash
# Clone project
git clone https://github.com/Corphon/SceneIntruderMCP.git
cd SceneIntruderMCP

# Download dependencies
go mod download

# Verify build
go build -o sceneintruder cmd/server/main.go
```

### 2. Environment Configuration

```bash
# Application will automatically create configuration file, no manual copying needed
# Default configuration will be generated in data/config.json on first run

# Configure via environment variables
export PORT=8080
export OPENAI_API_KEY=your-openai-api-key
export DEBUG_MODE=true
```

**Basic Configuration Example**:
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
    "api_key": "<encrypted_api_key_here>"  // API Key will be encrypted when stored
  }
}
```

**Note**: API keys are encrypted when stored in the configuration file for security. During runtime, they are decrypted only when needed for API calls.

### 3. Start Development Server

```bash
# Method 1: Direct run
go run cmd/server/main.go

# Method 2: Build then run
go build -o sceneintruder cmd/server/main.go
./sceneintruder

# Access application (default port is 8081)
open http://localhost:8081
```

## 🚀 Production Environment Deployment

### 1. Build Optimized Version

```bash
# Build production version
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o sceneintruder-linux-amd64 \
  cmd/server/main.go

# Verify binary
file sceneintruder-linux-amd64
```

### 2. System Service Configuration

**Create systemd service file** (`/etc/systemd/system/sceneintruder.service`):

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

# Security configuration
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/sceneintruder/data /opt/sceneintruder/logs

# Environment variables
Environment=GIN_MODE=release
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

### 3. Deployment Steps

```bash
# 1. Create dedicated user
sudo useradd -r -s /bin/false sceneintruder

# 2. Create directory structure
sudo mkdir -p /opt/sceneintruder/{data,logs,static,web}
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder

# 3. Copy files
sudo cp sceneintruder-linux-amd64 /opt/sceneintruder/sceneintruder
sudo cp -r static/* /opt/sceneintruder/static/
sudo cp -r web/* /opt/sceneintruder/web/

# 4. Set permissions
sudo chmod +x /opt/sceneintruder/sceneintruder
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder

# 5. Start service
sudo systemctl daemon-reload
sudo systemctl enable sceneintruder
sudo systemctl start sceneintruder

# 6. Check status
sudo systemctl status sceneintruder
```

## 🐳 Docker Deployment

### 1. Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o sceneintruder \
    cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -s /bin/sh sceneintruder

WORKDIR /app

# Copy binary and static resources
COPY --from=builder /app/sceneintruder .
COPY --from=builder /app/static ./static
COPY --from=builder /app/web ./web

# Create necessary directories
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
      - PORT=8080
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    restart: unless-stopped

  # Optional: Nginx reverse proxy
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

  # Optional: Log aggregation
  fluentd:
    image: fluent/fluentd:latest
    volumes:
      - ./logs:/fluentd/log
      - ./fluentd.conf:/fluentd/etc/fluent.conf
    depends_on:
      - sceneintruder
```

### 3. Deployment Commands

```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f sceneintruder

# Update deployment
docker-compose pull
docker-compose up -d --force-recreate

# Cleanup
docker-compose down
docker system prune -f
```

## ☁️ Cloud Platform Deployment

### AWS EC2 Deployment

```bash
#!/bin/bash
# AWS EC2 User Data Script

# Update system
yum update -y

# Install Go
wget https://golang.org/dl/go1.21.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
source /etc/profile

# Create application user
useradd -r -s /bin/false sceneintruder

# Deploy application
mkdir -p /opt/sceneintruder
cd /opt/sceneintruder

# Download release version
wget https://github.com/Corphon/SceneIntruderMCP/releases/latest/download/sceneintruder-linux-amd64
chmod +x sceneintruder-linux-amd64
mv sceneintruder-linux-amd64 sceneintruder

# Create configuration
mkdir -p data
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

# Set permissions
chown -R sceneintruder:sceneintruder /opt/sceneintruder

# Start service
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
  PORT: "8080"

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

## ⚙️ Configuration Management

### Environment Variable Configuration

```bash
# .env file
PORT=8080
DEBUG_MODE=false
DATA_DIR=/opt/sceneintruder/data
LOG_DIR=/opt/sceneintruder/logs

# LLM Configuration
LLM_PROVIDER=openai
OPENAI_API_KEY=your-openai-key
ANTHROPIC_API_KEY=your-claude-key
DEEPSEEK_API_KEY=your-deepseek-key

# Security Configuration
ALLOWED_ORIGINS=https://yourdomain.com
CORS_ENABLED=true
RATE_LIMIT_ENABLED=true
```

### Persistent Encryption Key (`data/.encryption_key`)

- When no `CONFIG_ENCRYPTION_KEY` environment variable is supplied, the server creates a random 32-byte key in `data/.encryption_key` so encrypted API credentials survive restarts.
- This file must travel with `data/config.json`; if you delete it, every encrypted key becomes unreadable until you re-enter the values through the settings API/UI.
- To rotate the key intentionally, remove the file, restart the service, and immediately supply fresh API keys—the new key will be persisted automatically.


### Multi-Environment Configuration

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

## 🔐 Security Configuration

### 1. API Key Encryption

The application implements AES-GCM encryption to protect API keys both in transit and at rest:
- **AES-GCM Encryption**: API keys are securely encrypted using AES-GCM algorithm before storage
- **Environment Priority**: API keys are primarily loaded from environment variables (e.g., `OPENAI_API_KEY`) 
- **Encrypted Storage**: When stored in configuration files, API keys are kept in encrypted form in `encrypted_llm_config` field
- **Runtime Decryption**: API keys are decrypted only when needed for API calls
- **Automatic Migration**: Legacy unencrypted API keys are automatically migrated to encrypted storage
- **Configuration Security**: The encryption key should be set as `CONFIG_ENCRYPTION_KEY` environment variable for optimal security

### 2. Nginx Reverse Proxy

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

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers off;

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload";

    # Request size limit
    client_max_body_size 10M;

    # Static files
    location /static/ {
        alias /opt/sceneintruder/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API rate limiting
    location /api/ {
        limit_req zone=api burst=10 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Application service
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

# Rate limiting configuration
http {
    limit_req_zone $binary_remote_addr zone=api:10m rate=60r/m;
    limit_req_zone $binary_remote_addr zone=general:10m rate=10r/s;
}
```

### 2. Firewall Configuration

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

### 3. SSL/TLS Configuration

```bash
# Let's Encrypt certificate
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d yourdomain.com

# Auto-renewal
echo "0 12 * * * /usr/bin/certbot renew --quiet" | sudo crontab -
```

## 📊 Monitoring and Logging

### 1. Log Configuration

```bash
# Log rotation configuration /etc/logrotate.d/sceneintruder
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

### 2. Prometheus Monitoring

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

### 3. Health Check

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


## 🔧 Troubleshooting

### Common Issues

#### 1. Service Startup Failure

```bash
# Check logs
sudo journalctl -u sceneintruder -f

# Check configuration file
sudo -u sceneintruder test -f /opt/sceneintruder/data/config.json && echo "Configuration file exists"

# Check port usage (default 8081, or configured PORT)
sudo netstat -tlnp | grep :8081
```

#### 2. API Key Issues

```bash
# Test API connection
curl -H "Authorization: Bearer your-api-key" \
     https://api.openai.com/v1/models

# Check configuration file
sudo -u sceneintruder cat /opt/sceneintruder/data/config.json
```

#### 3. Permission Issues

```bash
# Check file permissions
ls -la /opt/sceneintruder/
sudo chown -R sceneintruder:sceneintruder /opt/sceneintruder/

# Check SELinux (if enabled)
sudo setsebool -P httpd_can_network_connect 1
```

#### 4. Memory Issues

```bash
# Check memory usage
free -h
ps aux | grep sceneintruder
```

### Backup and Recovery

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/sceneintruder"
DATA_DIR="/opt/sceneintruder/data"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup
mkdir -p "$BACKUP_DIR"
tar -czf "$BACKUP_DIR/sceneintruder_$DATE.tar.gz" \
    -C "/opt/sceneintruder" \
    data logs

# Keep backups for last 30 days
find "$BACKUP_DIR" -name "sceneintruder_*.tar.gz" -mtime +30 -delete

echo "Backup completed: sceneintruder_$DATE.tar.gz"
```

### Performance Tuning

```bash
# System optimization
echo 'vm.swappiness=10' >> /etc/sysctl.conf
echo 'net.core.somaxconn=65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog=65535' >> /etc/sysctl.conf
sysctl -p

# File descriptor limits
echo 'sceneintruder soft nofile 65535' >> /etc/security/limits.conf
echo 'sceneintruder hard nofile 65535' >> /etc/security/limits.conf
```

## 📞 Support and Help

If you encounter issues during deployment, please:

1. Check the [FAQ Documentation](docs/faq.md)
2. Browse [GitHub Issues](https://github.com/Corphon/SceneIntruderMCP/issues)
3. Ask questions in [GitHub Discussions](https://github.com/Corphon/SceneIntruderMCP/discussions)
4. Send email to [support@sceneintruder.dev](mailto:songkf@foxmail.com)

---

**Note**: This deployment guide is written based on the latest version. For specific version deployments, please refer to the corresponding version documentation.
