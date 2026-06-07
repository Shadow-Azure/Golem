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
    echo "Please install Go from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}Found Go: ${GO_VERSION}${NC}"

# Step 2: Clean up old processes
echo -e "${YELLOW}Step 2: Cleaning up old processes...${NC}"
if pgrep -x "golem" > /dev/null; then
    pkill -x "golem" 2>/dev/null || true
    sleep 1
    echo -e "${GREEN}Stopped running golem process${NC}"
else
    echo -e "${GREEN}No running golem process found${NC}"
fi

# Step 3: Build Go binary
echo -e "${YELLOW}Step 3: Building Go binary...${NC}"
rm -rf release
mkdir -p release

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "arm64" ]; then
    GOARCH="arm64"
elif [ "$ARCH" = "x86_64" ]; then
    GOARCH="amd64"
else
    echo -e "${RED}Unsupported architecture: $ARCH${NC}"
    exit 1
fi

if [ "$OS" = "darwin" ]; then
    GOOS="darwin"
elif [ "$OS" = "linux" ]; then
    GOOS="linux"
else
    echo -e "${RED}Unsupported OS: $OS${NC}"
    exit 1
fi

echo -e "${GREEN}Building for ${GOOS}/${GOARCH}...${NC}"

CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.Version=${VERSION}" -o release/golem ./cmd/golem
chmod +x release/golem

BINARY_SIZE=$(du -h release/golem | cut -f1)
echo -e "${GREEN}Built binary: release/golem (${BINARY_SIZE})${NC}"

# Step 4: Copy config file
echo -e "${YELLOW}Step 4: Copying config file...${NC}"
cp configs/golem.example.yaml release/golem.example.yaml
echo -e "${GREEN}Copied: release/golem.example.yaml${NC}"

# Step 5: Generate README
echo -e "${YELLOW}Step 5: Generating README...${NC}"
cat > release/README.md << 'READMEEOF'
# Golem AI Agent

本地 AI 助手框架，支持飞书机器人集成。

## 快速开始

### 1. 安装

解压安装包：

```bash
tar -xzf golem-darwin-arm64.tar.gz
cd release
```

运行安装脚本：

```bash
./install.sh
```

### 2. 配置大模型

编辑配置文件：

```bash
vim ~/.golem/golem.yaml
```

配置 OpenAI：

```yaml
llm:
  providers:
    openai:
      api_key: "sk-your-api-key"
```

或使用环境变量：

```bash
export OPENAI_API_KEY="sk-your-api-key"
```

### 3. 启动服务

```bash
golem
```

或指定配置文件：

```bash
golem -config /path/to/golem.yaml
```

### 4. 停止服务

按 `Ctrl+C` 停止

或查找进程并终止：

```bash
ps aux | grep golem
kill <PID>
```

---

## 飞书机器人配置

### 1. 创建飞书应用

1. 访问 https://open.feishu.cn
2. 创建企业自建应用
3. 获取 App ID 和 App Secret

### 2. 配置权限

在飞书开放平台，为应用添加权限：

- `im:message`
- `im:message.group_at_msg`
- `im:message.p2p_msg`

### 3. 启用事件订阅

1. 在"事件订阅"中，添加事件：
   - `im.message.receive_v1`
2. 配置请求地址（如果使用 Webhook 模式）

### 4. 配置 Golem

编辑 `~/.golem/golem.yaml`：

```yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"
  app_secret: "xxxx"
  verification_token: "xxxx"
```

或使用环境变量：

```bash
export FEISHU_APP_ID="cli_xxxx"
export FEISHU_APP_SECRET="xxxx"
export FEISHU_VERIFICATION_TOKEN="xxxx"
```

### 5. 重启服务

```bash
golem
```

---

## 常见问题

### Q: 如何切换模型？

修改 `~/.golem/golem.yaml` 中的 `default_provider`：

```yaml
llm:
  default_provider: "claude"  # 切换到 Claude
```

### Q: 如何查看日志？

```bash
tail -f ~/.golem/golem.log
```

### Q: 如何更新版本？

1. 下载新版本
2. 运行 `./install.sh` 覆盖安装

### Q: 支持哪些模型？

- OpenAI: gpt-4o, gpt-4, gpt-3.5-turbo
- Claude: claude-3-opus, claude-3-sonnet, claude-3-haiku

---

## 配置参考

### 环境变量

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `OPENAI_API_KEY` | OpenAI API 密钥 | 是（如果使用 OpenAI） |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 是（如果使用 Claude） |
| `FEISHU_APP_ID` | 飞书应用 ID | 是（如果启用飞书） |
| `FEISHU_APP_SECRET` | 飞书应用密钥 | 是（如果启用飞书） |
| `FEISHU_VERIFICATION_TOKEN` | 飞书验证令牌 | 是（如果启用飞书） |
| `FEISHU_ENCRYPT_KEY` | 飞书加密密钥 | 否 |

### 配置优先级

```
环境变量 > ~/.golem/golem.yaml > 默认值
```
READMEEOF

echo -e "${GREEN}Generated: release/README.md${NC}"

# Step 6: Generate install script
echo -e "${YELLOW}Step 6: Generating install script...${NC}"
cat > release/install.sh << 'INSTALLEOF'
#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Installing Golem AI Agent...${NC}"

# Check if running from release directory
if [ ! -f "./golem" ]; then
    echo -e "${RED}Error: golem binary not found${NC}"
    echo "Please run this script from the release directory"
    exit 1
fi

# Create directory
mkdir -p ~/.golem
echo -e "${GREEN}Created directory: ~/.golem${NC}"

# Copy binary
cp golem ~/.golem/golem
chmod +x ~/.golem/golem
echo -e "${GREEN}Installed binary: ~/.golem/golem${NC}"

# Copy config (only if not exists)
if [ ! -f ~/.golem/golem.yaml ]; then
    cp golem.example.yaml ~/.golem/golem.yaml
    echo -e "${YELLOW}Created config file: ~/.golem/golem.yaml${NC}"
    echo -e "${YELLOW}Please edit ~/.golem/golem.yaml to configure your API keys${NC}"
else
    echo -e "${YELLOW}Config file already exists, skipping...${NC}"
fi

# Create symlink (optional, requires write permission)
if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    ln -sf ~/.golem/golem /usr/local/bin/golem
    echo -e "${GREEN}Created symlink: /usr/local/bin/golem${NC}"
else
    echo -e "${YELLOW}Note: ~/.golem/golem is installed. Add ~/.golem to PATH or run with full path.${NC}"
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Edit config: vim ~/.golem/golem.yaml"
echo "  2. Set API key: export OPENAI_API_KEY=\"sk-your-key\""
echo "  3. Start: golem"
echo ""
echo "For Feishu integration, see README.md"
INSTALLEOF

chmod +x release/install.sh
echo -e "${GREEN}Generated: release/install.sh${NC}"

# Step 7: Package
echo -e "${YELLOW}Step 7: Packaging...${NC}"
PLATFORM="${GOOS}-${GOARCH}"
tar -czf "release/golem-${PLATFORM}.tar.gz" -C release golem golem.example.yaml README.md install.sh

PACKAGE_SIZE=$(du -h "release/golem-${PLATFORM}.tar.gz" | cut -f1)
echo -e "${GREEN}Created package: release/golem-${PLATFORM}.tar.gz (${PACKAGE_SIZE})${NC}"

# Summary
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Build complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Output files:"
echo "  release/golem                    - Binary (${BINARY_SIZE})"
echo "  release/golem.example.yaml       - Example config"
echo "  release/README.md                - Documentation"
echo "  release/install.sh               - Install script"
echo "  release/golem-${PLATFORM}.tar.gz - Package (${PACKAGE_SIZE})"
echo ""
echo "To install:"
echo "  cd release && ./install.sh"
echo ""
echo "To distribute:"
echo "  share release/golem-${PLATFORM}.tar.gz"
