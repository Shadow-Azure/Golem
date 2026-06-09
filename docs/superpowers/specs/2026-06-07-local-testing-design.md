# Golem Local Testing Design Specification

**Date**: 2026-06-07
**Status**: Draft
**Author**: Claude Code (Brainstorming)

## 1. Overview

### 1.1 Project Goal

Add local testing capabilities to Golem AI Agent:
- TUI (Terminal UI) for quick chat testing
- WebChat (Vue 3) for browser-based chat testing
- Feishu connection documentation in release package

### 1.2 Key Design Principles

1. **Quick Testing**: Easy to start and test locally
2. **Multiple Interfaces**: TUI and WebChat for different use cases
3. **Developer Friendly**: Clear documentation and setup guides
4. **Consistent API**: Same backend API for TUI and WebChat

### 1.3 Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| TUI | Go stdin/stdout | - |
| WebChat Backend | Go net/http | - |
| WebChat Frontend | Vue 3 + Vite | 3.x |
| Build | pnpm | 8+ |

## 2. TUI Design

### 2.1 Command Structure

```bash
golem chat                    # Start TUI chat
golem chat --config path      # Use custom config
golem chat --provider openai  # Use specific provider
```

### 2.2 TUI Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    golem chat 流程                           │
├─────────────────────────────────────────────────────────────┤
│  1. 加载配置                                                │
│  2. 初始化 LLM 提供商                                       │
│  3. 显示欢迎信息                                            │
│  4. 进入聊天循环                                            │
│     ├─ 显示提示符 "You: "                                   │
│     ├─ 读取用户输入                                         │
│     ├─ 调用 LLM API                                        │
│     ├─ 流式输出响应                                         │
│     └─ 循环                                                 │
│  5. 支持命令                                                │
│     ├─ /help     显示帮助                                   │
│     ├─ /clear    清空历史                                   │
│     ├─ /model    切换模型                                   │
│     ├─ /history  显示历史                                   │
│     └─ /quit     退出                                       │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 TUI Implementation

```go
// cmd/golem/chat.go
package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"

    "github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
    Use:   "chat",
    Short: "启动终端聊天",
    Long:  "在终端中直接与 AI 助手对话。",
    Run:   runChat,
}

func runChat(cmd *cobra.Command, args []string) {
    // Load config
    cfg := loadConfig()

    // Initialize LLM provider
    provider := initProvider(cfg)

    // Create session
    session := createSession()

    fmt.Println("🤖 Golem AI Chat")
    fmt.Println("输入消息开始对话，输入 /help 查看命令")
    fmt.Println()

    reader := bufio.NewReader(os.Stdin)

    for {
        fmt.Print("You: ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)

        if input == "" {
            continue
        }

        // Handle commands
        if strings.HasPrefix(input, "/") {
            handleCommand(input, session)
            continue
        }

        // Add user message to session
        addMessage(session, "user", input)

        // Get response from LLM
        fmt.Print("AI: ")
        response := streamResponse(provider, session)
        fmt.Println()

        // Add assistant message to session
        addMessage(session, "assistant", response)
    }
}

func handleCommand(input string, session *Session) {
    switch input {
    case "/help":
        fmt.Println("可用命令:")
        fmt.Println("  /help     - 显示帮助")
        fmt.Println("  /clear    - 清空历史")
        fmt.Println("  /model    - 切换模型")
        fmt.Println("  /history  - 显示历史")
        fmt.Println("  /quit     - 退出")
    case "/clear":
        session.Messages = nil
        fmt.Println("历史已清空")
    case "/quit":
        fmt.Println("再见！")
        os.Exit(0)
    default:
        fmt.Printf("未知命令: %s\n", input)
    }
}
```

## 3. WebChat Design

### 3.1 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    WebChat 架构                             │
├─────────────────────────────────────────────────────────────┤
│  Browser (Vue 3)                                            │
│  ├─ Chat.vue        # 聊天界面组件                          │
│  ├─ Message.vue     # 消息组件                              │
│  └─ Input.vue       # 输入组件                              │
│         │                                                   │
│         │ HTTP API                                          │
│         ▼                                                   │
│  Go HTTP Server                                             │
│  ├─ GET /           # 静态文件服务                          │
│  ├─ POST /api/chat  # 发送消息                              │
│  ├─ GET /api/stream # SSE 流式响应                          │
│  └─ GET /api/history # 获取历史                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 API Design

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | 静态文件服务 |
| POST | `/api/chat` | 发送消息 |
| GET | `/api/stream` | SSE 流式响应 |
| GET | `/api/history` | 获取会话历史 |
| POST | `/api/clear` | 清空会话 |

### 3.3 API Implementation

```go
// web/server.go
package web

import (
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
)

type Server struct {
    engine  *core.Engine
    session *core.Session
    addr    string
}

func NewServer(engine *core.Engine, addr string) *Server {
    return &Server{
        engine: engine,
        addr:   addr,
    }
}

func (s *Server) Start() error {
    mux := http.NewServeMux()

    // Static files
    staticDir := filepath.Join(".", "web", "static")
    if _, err := os.Stat(staticDir); err == nil {
        mux.Handle("/", http.FileServer(http.Dir(staticDir)))
    }

    // API endpoints
    mux.HandleFunc("/api/chat", s.handleChat)
    mux.HandleFunc("/api/stream", s.handleStream)
    mux.HandleFunc("/api/history", s.handleHistory)
    mux.HandleFunc("/api/clear", s.handleClear)

    return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Add user message to session
    s.engine.GetSessionManager().AddMessage(s.session.ID, core.Message{
        Role:    "user",
        Content: req.Message,
    })

    // Get LLM response
    provider, _ := s.engine.GetPluginManager().GetProvider("openai")
    history, _ := s.engine.GetSessionManager().GetHistory(s.session.ID, 50)

    response, err := provider.Chat(r.Context(), history, core.ChatConfig{})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Add assistant message to session
    s.engine.GetSessionManager().AddMessage(s.session.ID, core.Message{
        Role:    "assistant",
        Content: response.Content,
    })

    // Return response
    json.NewEncoder(w).Encode(ChatResponse{
        Content: response.Content,
    })
}
```

### 3.4 Vue 3 Frontend

```vue
<!-- ui/src/components/Chat.vue -->
<template>
  <div class="chat-container">
    <div class="messages" ref="messagesRef">
      <div
        v-for="(msg, index) in messages"
        :key="index"
        :class="['message', msg.role]"
      >
        <div class="avatar">{{ msg.role === 'user' ? '👤' : '🤖' }}</div>
        <div class="content">{{ msg.content }}</div>
      </div>
    </div>

    <div class="input-area">
      <input
        v-model="input"
        @keyup.enter="sendMessage"
        placeholder="输入消息..."
        :disabled="loading"
      />
      <button @click="sendMessage" :disabled="loading || !input.trim()">
        {{ loading ? '发送中...' : '发送' }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'

const messages = ref([])
const input = ref('')
const loading = ref(false)
const messagesRef = ref(null)

const sendMessage = async () => {
  if (!input.value.trim() || loading.value) return

  const userMessage = input.value.trim()
  input.value = ''
  loading.value = true

  // Add user message
  messages.value.push({ role: 'user', content: userMessage })

  try {
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: userMessage })
    })

    const data = await response.json()
    messages.value.push({ role: 'assistant', content: data.content })
  } catch (error) {
    messages.value.push({ role: 'assistant', content: 'Error: ' + error.message })
  } finally {
    loading.value = false
    nextTick(() => scrollToBottom())
  }
}

const scrollToBottom = () => {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

onMounted(() => {
  fetchHistory()
})

const fetchHistory = async () => {
  try {
    const response = await fetch('/api/history')
    const data = await response.json()
    messages.value = data.messages || []
  } catch (error) {
    console.error('Failed to fetch history:', error)
  }
}
</script>

<style scoped>
.chat-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #f5f5f5;
  border-radius: 8px;
  margin-bottom: 20px;
}

.message {
  display: flex;
  margin-bottom: 16px;
  gap: 12px;
}

.message.user {
  flex-direction: row-reverse;
}

.avatar {
  font-size: 24px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: white;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.content {
  max-width: 70%;
  padding: 12px 16px;
  border-radius: 12px;
  line-height: 1.5;
}

.message.user .content {
  background: #007bff;
  color: white;
  border-bottom-right-radius: 4px;
}

.message.assistant .content {
  background: white;
  color: #333;
  border-bottom-left-radius: 4px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.input-area {
  display: flex;
  gap: 12px;
}

.input-area input {
  flex: 1;
  padding: 12px 16px;
  border: 2px solid #ddd;
  border-radius: 8px;
  font-size: 16px;
  outline: none;
  transition: border-color 0.2s;
}

.input-area input:focus {
  border-color: #007bff;
}

.input-area button {
  padding: 12px 24px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.2s;
}

.input-area button:hover:not(:disabled) {
  background: #0056b3;
}

.input-area button:disabled {
  background: #ccc;
  cursor: not-allowed;
}
</style>
```

## 4. Feishu Documentation

### 4.1 Documentation Structure

```
release/
├── golem
├── golem.example.yaml
├── README.md
├── install.sh
└── docs/
    └── feishu.md          # 飞书连接指南
```

### 4.2 Feishu Documentation Content

```markdown
# 飞书机器人连接指南

## 1. 创建飞书应用

1. 访问 [飞书开放平台](https://open.feishu.cn)
2. 点击"创建企业自建应用"
3. 填写应用信息：
   - 应用名称：Golem AI Assistant
   - 应用描述：AI 助手机器人
4. 创建完成后，获取：
   - App ID
   - App Secret

## 2. 配置应用权限

在"权限管理"中添加以下权限：

| 权限 | 说明 |
|------|------|
| im:message | 获取与发送单聊、群组消息 |
| im:message.group_at_msg | 接收群聊中@机器人消息 |
| im:message.p2p_msg | 接收机器人单聊消息 |

## 3. 启用事件订阅

1. 进入"事件订阅"页面
2. 选择"使用长连接接收事件"
3. 添加事件：
   - im.message.receive_v1（接收消息）

## 4. 配置 Golem

编辑 `~/.golem/golem.yaml`：

```yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"           # 替换为你的 App ID
  app_secret: "xxxx"           # 替换为你的 App Secret
  verification_token: "xxxx"   # 替换为你的 Verification Token
```

或使用环境变量：

```bash
export FEISHU_APP_ID="cli_xxxx"
export FEISHU_APP_SECRET="xxxx"
export FEISHU_VERIFICATION_TOKEN="xxxx"
```

## 5. 启动服务

```bash
# 启动后台服务
golem start

# 查看状态
golem status

# 查看日志
tail -f ~/.golem/golem.log
```

## 6. 测试机器人

1. 在飞书中搜索你的应用名称
2. 发送消息给机器人
3. 机器人应该会回复 AI 生成的内容

## 7. 常见问题

### Q: 机器人没有回复？

检查：
1. 服务是否正在运行：`golem status`
2. 配置是否正确：`cat ~/.golem/golem.yaml`
3. 日志是否有错误：`tail -f ~/.golem/golem.log`

### Q: 如何切换 LLM 提供商？

编辑 `~/.golem/golem.yaml`：

```yaml
llm:
  default_provider: "claude"  # 切换到 Claude
```

然后重启服务：

```bash
golem restart
```

### Q: 如何查看机器人日志？

```bash
tail -f ~/.golem/golem.log
```
```

## 5. CLI Commands

### 5.1 Chat Command

```bash
# 启动 TUI 聊天
golem chat

# 使用自定义配置
golem chat --config ~/.golem/golem.yaml

# 使用特定提供商
golem chat --provider claude
```

### 5.2 Web Command

```bash
# 启动 WebChat 服务（默认端口 8080）
golem web

# 指定端口
golem web --port 9090

# 指定配置
golem web --config ~/.golem/golem.yaml
```

## 6. File Structure

### 6.1 Go Backend

```
golem/
├── cmd/golem/
│   ├── chat.go            # TUI 聊天命令
│   └── web.go             # WebChat 服务命令
├── web/
│   ├── server.go          # HTTP 服务器
│   ├── api.go             # REST API
│   ├── types.go           # 请求/响应类型
│   └── static/            # Vue 3 构建产物
│       └── index.html
└── docs/
    └── feishu.md          # 飞书连接文档
```

### 6.2 Vue 3 Frontend

```
ui/
├── src/
│   ├── App.vue            # 主应用组件
│   ├── main.js            # 入口文件
│   ├── components/
│   │   ├── Chat.vue       # 聊天组件
│   │   ├── Message.vue    # 消息组件
│   │   └── Input.vue      # 输入组件
│   └── styles/
│       └── main.css       # 样式文件
├── public/
│   └── index.html         # HTML 模板
├── package.json
├── vite.config.js
└── README.md
```

## 7. Build Process

### 7.1 Development

```bash
# 启动 Vue 开发服务器
cd ui
pnpm install
pnpm dev

# 启动 Go 后端
go run ./cmd/golem web --port 8080
```

### 7.2 Production

```bash
# 构建 Vue 前端
cd ui
pnpm build

# 复制构建产物
cp -r dist/* ../web/static/

# 构建 Go 二进制
cd ..
go build -o golem ./cmd/golem
```

### 7.3 Release Script Update

在 `release.sh` 中添加：

```bash
# Step 8: Build WebChat UI
echo -e "${YELLOW}Step 8: Building WebChat UI...${NC}"
if [ -d "ui" ]; then
    cd ui
    pnpm install
    pnpm build
    cd ..
    mkdir -p release/web/static
    cp -r ui/dist/* release/web/static/
fi

# Step 9: Copy docs
echo -e "${YELLOW}Step 9: Copying docs...${NC}"
mkdir -p release/docs
cp docs/feishu.md release/docs/
```

## 8. Testing

### 8.1 TUI Testing

```bash
# 启动 TUI
golem chat

# 测试消息
You: Hello
AI: Hello! How can I help you?

# 测试命令
You: /help
You: /clear
You: /quit
```

### 8.2 WebChat Testing

```bash
# 启动 WebChat
golem web --port 8080

# 打开浏览器
open http://localhost:8080
```

### 8.3 Feishu Testing

```bash
# 配置飞书
golem onboard

# 启动服务
golem start

# 在飞书中测试
```

## Appendix A: References

- [openclaw Control UI](https://github.com/Shadow-Azure/openclaw-main/tree/main/ui)
- [openclaw TUI](https://github.com/Shadow-Azure/openclaw-main/tree/main/src/tui)
- [Vue 3 Documentation](https://vuejs.org/)
- [Vite Documentation](https://vitejs.dev/)
