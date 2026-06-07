# Golem Release Script Design Specification

**Date**: 2026-06-07
**Status**: Draft
**Author**: Claude Code (Brainstorming)

## 1. Overview

### 1.1 Project Goal

Create a release script and documentation for Golem AI Agent that:
- Builds a single Go binary for macOS arm64
- Generates a complete distribution package with executable, configuration, and documentation
- Provides clear instructions for setup, configuration, and usage
- Supports future expansion to macOS x86, Linux, and Windows

### 1.2 Key Design Principles

1. **Simple Distribution**: Single tar.gz package with everything needed
2. **User-Friendly**: Clear documentation in Chinese
3. **Easy Installation**: One-command install script
4. **Flexible Configuration**: YAML config with environment variable support
5. **Future-Proof**: Extensible to other platforms

### 1.3 Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Build Tool | Go | 1.21+ |
| Configuration | YAML | gopkg.in/yaml.v3 |
| Platform | macOS arm64 | - |
| Distribution | tar.gz | - |

## 2. Release Script Design

### 2.1 release.sh Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    release.sh 流程                           │
├─────────────────────────────────────────────────────────────┤
│  Step 1: 检查前置依赖                                        │
│  └─ 验证 go 命令可用                                         │
├─────────────────────────────────────────────────────────────┤
│  Step 2: 清理旧进程                                          │
│  └─ 停止正在运行的 golem 进程                                 │
├─────────────────────────────────────────────────────────────┤
│  Step 3: 构建 Go 二进制文件                                   │
│  └─ go build -o release/golem ./cmd/golem                   │
├─────────────────────────────────────────────────────────────┤
│  Step 4: 复制配置文件                                        │
│  └─ 复制 golem.example.yaml 到 release/                      │
├─────────────────────────────────────────────────────────────┤
│  Step 5: 生成 README.md                                      │
│  └─ 生成使用说明（中文）                                      │
├─────────────────────────────────────────────────────────────┤
│  Step 6: 生成 install.sh                                     │
│  └─ 生成安装脚本，支持一键安装到 ~/.golem/                     │
├─────────────────────────────────────────────────────────────┤
│  Step 7: 打包                                                │
│  └─ tar -czf golem-darwin-arm64.tar.gz -C release .          │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Output Directory Structure

```
release/
├── golem                        # 可执行文件 (~10MB)
├── golem.example.yaml           # 示例配置文件
├── README.md                    # 使用说明
├── install.sh                   # 安装脚本
└── golem-darwin-arm64.tar.gz    # 压缩包 (~5MB)
```

### 2.3 Script Implementation

```bash
#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Version
VERSION="1.0.0"

echo -e "${GREEN}Building Golem AI Agent v${VERSION}...${NC}"

# Step 1: Check prerequisites
echo -e "${YELLOW}Step 1: Checking prerequisites...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: go is not installed${NC}"
    exit 1
fi

# Step 2: Clean up old processes
echo -e "${YELLOW}Step 2: Cleaning up old processes...${NC}"
pkill -f "golem" 2>/dev/null || true

# Step 3: Build Go binary
echo -e "${YELLOW}Step 3: Building Go binary...${NC}"
rm -rf release
mkdir -p release
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o release/golem ./cmd/golem
chmod +x release/golem

# Step 4: Copy config file
echo -e "${YELLOW}Step 4: Copying config file...${NC}"
cp configs/golem.example.yaml release/golem.example.yaml

# Step 5: Generate README
echo -e "${YELLOW}Step 5: Generating README...${NC}"
cat > release/README.md << 'EOF'
# Golem AI Agent

本地 AI 助手框架，支持飞书机器人集成。

## 快速开始

### 1. 安装

解压安装包：
tar -xzf golem-darwin-arm64.tar.gz
cd release

运行安装脚本：
./install.sh

### 2. 配置大模型

编辑配置文件：
vim ~/.golem/golem.yaml

配置 OpenAI：
llm:
  providers:
    openai:
      api_key: "sk-your-api-key"

或使用环境变量：
export OPENAI_API_KEY="sk-your-api-key"

### 3. 启动服务

golem

或指定配置文件：
golem -config /path/to/golem.yaml

### 4. 停止服务

按 Ctrl+C 停止

或查找进程并终止：
ps aux | grep golem
kill <PID>

## 飞书机器人配置

### 1. 创建飞书应用

1. 访问 https://open.feishu.cn
2. 创建企业自建应用
3. 获取 App ID 和 App Secret

### 2. 配置权限

在飞书开放平台，为应用添加权限：
- im:message
- im:message.group_at_msg
- im:message.p2p_msg

### 3. 启用事件订阅

1. 在"事件订阅"中，添加事件：
   - im.message.receive_v1
2. 配置请求地址（如果使用 Webhook 模式）

### 4. 配置 Golem

编辑 ~/.golem/golem.yaml：
feishu:
  enabled: true
  app_id: "cli_xxxx"
  app_secret: "xxxx"
  verification_token: "xxxx"

或使用环境变量：
export FEISHU_APP_ID="cli_xxxx"
export FEISHU_APP_SECRET="xxxx"
export FEISHU_VERIFICATION_TOKEN="xxxx"

### 5. 重启服务

golem

## 常见问题

### Q: 如何切换模型？

修改 ~/.golem/golem.yaml 中的 default_provider：
llm:
  default_provider: "claude"  # 切换到 Claude

### Q: 如何查看日志？

tail -f ~/.golem/golem.log

### Q: 如何更新版本？

1. 下载新版本
2. 运行 ./install.sh 覆盖安装
EOF

# Step 6: Generate install script
echo -e "${YELLOW}Step 6: Generating install script...${NC}"
cat > release/install.sh << 'EOF'
#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Installing Golem AI Agent...${NC}"

# Create directory
mkdir -p ~/.golem

# Copy binary
cp golem ~/.golem/golem
chmod +x ~/.golem/golem

# Copy config (only if not exists)
if [ ! -f ~/.golem/golem.yaml ]; then
    cp golem.example.yaml ~/.golem/golem.yaml
    echo -e "${YELLOW}Created config file: ~/.golem/golem.yaml${NC}"
else
    echo -e "${YELLOW}Config file already exists, skipping...${NC}"
fi

# Create symlink (optional)
if [ -d /usr/local/bin ]; then
    ln -sf ~/.golem/golem /usr/local/bin/golem
    echo -e "${GREEN}Created symlink: /usr/local/bin/golem${NC}"
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Configuration: ~/.golem/golem.yaml"
echo "Start: golem"
echo "Stop: Ctrl+C"
EOF
chmod +x release/install.sh

# Step 7: Package
echo -e "${YELLOW}Step 7: Packaging...${NC}"
tar -czf release/golem-darwin-arm64.tar.gz -C release golem golem.example.yaml README.md install.sh

echo -e "${GREEN}Build complete!${NC}"
echo ""
echo "Output:"
echo "  release/golem"
echo "  release/golem.example.yaml"
echo "  release/README.md"
echo "  release/install.sh"
echo "  release/golem-darwin-arm64.tar.gz"
```

## 3. Configuration Design

### 3.1 Configuration File Location

```
~/.golem/
├── golem.yaml                   # 主配置文件
├── golem.log                    # 日志文件（运行时生成）
└── sessions/                    # 会话数据（运行时生成）
```

### 3.2 golem.yaml Structure

```yaml
# Golem AI Agent 配置文件
# 位置: ~/.golem/golem.yaml

# 服务器配置
server:
  host: "127.0.0.1"
  port: 9921

# 大模型配置
llm:
  # 默认使用的模型提供商
  default_provider: "openai"

  # 模型提供商配置
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"  # 支持环境变量
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      temperature: 0.7
      max_tokens: 4096

    claude:
      api_key: "${ANTHROPIC_API_KEY}"
      base_url: "https://api.anthropic.com"
      model: "claude-3-opus-20240229"
      temperature: 0.7
      max_tokens: 4096

# 会话配置
session:
  max_history: 50        # 最大历史消息数
  trim_to: 20            # 修剪到这个数量
  idle_timeout: 30m      # 空闲超时时间
  cleanup_interval: 5m   # 清理间隔

# 飞书配置
feishu:
  enabled: false                          # 是否启用飞书
  app_id: "${FEISHU_APP_ID}"             # 飞书应用 ID
  app_secret: "${FEISHU_APP_SECRET}"     # 飞书应用密钥
  verification_token: "${FEISHU_VERIFICATION_TOKEN}"  # 验证令牌
  encrypt_key: "${FEISHU_ENCRYPT_KEY}"   # 加密密钥（可选）

# 日志配置
logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json, text
  output: "file"   # stdout, file
  file: "~/.golem/golem.log"
```

### 3.3 Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENAI_API_KEY` | OpenAI API key | Yes (if using OpenAI) |
| `ANTHROPIC_API_KEY` | Anthropic API key | Yes (if using Claude) |
| `FEISHU_APP_ID` | Feishu app ID | Yes (if enabling Feishu) |
| `FEISHU_APP_SECRET` | Feishu app secret | Yes (if enabling Feishu) |
| `FEISHU_VERIFICATION_TOKEN` | Feishu verification token | Yes (if enabling Feishu) |
| `FEISHU_ENCRYPT_KEY` | Feishu encrypt key | No |

### 3.4 Configuration Priority

```
Environment variables > ~/.golem/golem.yaml > Default values
```

## 4. Installation Design

### 4.1 install.sh Flow

```bash
#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Installing Golem AI Agent...${NC}"

# Create directory
mkdir -p ~/.golem

# Copy binary
cp golem ~/.golem/golem
chmod +x ~/.golem/golem

# Copy config (only if not exists)
if [ ! -f ~/.golem/golem.yaml ]; then
    cp golem.example.yaml ~/.golem/golem.yaml
    echo -e "${YELLOW}Created config file: ~/.golem/golem.yaml${NC}"
else
    echo -e "${YELLOW}Config file already exists, skipping...${NC}"
fi

# Create symlink (optional)
if [ -d /usr/local/bin ]; then
    ln -sf ~/.golem/golem /usr/local/bin/golem
    echo -e "${GREEN}Created symlink: /usr/local/bin/golem${NC}"
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Configuration: ~/.golem/golem.yaml"
echo "Start: golem"
echo "Stop: Ctrl+C"
```

### 4.2 Installation Directory Structure

After installation:

```
~/.golem/
├── golem                        # 可执行文件
├── golem.yaml                   # 配置文件
├── golem.log                    # 日志文件（运行时生成）
└── sessions/                    # 会话数据（运行时生成）
```

## 5. Usage Documentation

### 5.1 Quick Start

```bash
# 1. 解压安装包
tar -xzf golem-darwin-arm64.tar.gz
cd release

# 2. 运行安装脚本
./install.sh

# 3. 配置大模型
vim ~/.golem/golem.yaml
# 设置 api_key

# 4. 启动服务
golem
```

### 5.2 Configure LLM Provider

**Option 1: Edit config file**
```yaml
# ~/.golem/golem.yaml
llm:
  providers:
    openai:
      api_key: "sk-your-api-key"
```

**Option 2: Use environment variable**
```bash
export OPENAI_API_KEY="sk-your-api-key"
golem
```

### 5.3 Configure Feishu Bot

**Step 1: Create Feishu App**
1. Visit https://open.feishu.cn
2. Create enterprise app
3. Get App ID and App Secret

**Step 2: Configure Permissions**
Add permissions:
- im:message
- im:message.group_at_msg
- im:message.p2p_msg

**Step 3: Enable Event Subscription**
Add event: im.message.receive_v1

**Step 4: Configure Golem**
```yaml
# ~/.golem/golem.yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"
  app_secret: "xxxx"
  verification_token: "xxxx"
```

**Step 5: Restart**
```bash
golem
```

### 5.4 Start/Stop Commands

**Start:**
```bash
golem
# or
golem -config /path/to/golem.yaml
```

**Stop:**
```bash
# Press Ctrl+C
# or
ps aux | grep golem
kill <PID>
```

### 5.5 View Logs

```bash
tail -f ~/.golem/golem.log
```

### 5.6 Switch Model

Edit ~/.golem/golem.yaml:
```yaml
llm:
  default_provider: "claude"  # Switch to Claude
```

## 6. Future Expansion

### 6.1 Platform Support

Current: macOS arm64

Future:
- macOS x86 (amd64)
- Linux (amd64, arm64)
- Windows (amd64)

### 6.2 Build Script Enhancement

For multi-platform support, modify release.sh:

```bash
# Build for multiple platforms
PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64" "windows/amd64")

for platform in "${PLATFORMS[@]}"; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    output="release/golem-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output="${output}.exe"
    fi
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -o $output ./cmd/golem
done
```

## 7. Testing

### 7.1 Test Release Script

```bash
# Run release script
./release.sh

# Verify output
ls -la release/
./release/golem --help
```

### 7.2 Test Installation

```bash
# Run install script
cd release
./install.sh

# Verify installation
ls -la ~/.golem/
golem --help
```

### 7.3 Test Configuration

```bash
# Test with config file
golem -config ~/.golem/golem.yaml

# Test with environment variable
OPENAI_API_KEY=test golem
```

## Appendix A: File Checklist

- [ ] `release.sh` - Release script
- [ ] `configs/golem.example.yaml` - Example configuration
- [ ] `release/README.md` - Generated documentation
- [ ] `release/install.sh` - Generated install script
- [ ] `.gitignore` - Update to exclude release artifacts

## Appendix B: References

- [cli-box release.sh](https://github.com/Shadow-Azure/cli-box/blob/main/release.sh)
- [openclaw configuration](https://github.com/Shadow-Azure/openclaw-main)
- [Golem AI Agent Design](docs/superpowers/specs/2026-06-07-golem-ai-agent-design.md)
