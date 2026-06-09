# Golem Local Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add TUI chat, WebChat (Vue 3), and Feishu documentation for local testing.

**Architecture:** TUI uses Go stdin/stdout for terminal chat. WebChat uses Go HTTP server + Vue 3 frontend. Both share the same LLM provider layer.

**Tech Stack:** Go 1.21+, Vue 3, Vite, pnpm

---

## File Structure

```
golem/
├── cmd/golem/
│   ├── chat.go                # TUI chat command (create)
│   └── web.go                 # WebChat server command (create)
├── web/
│   ├── server.go              # HTTP server (create)
│   ├── api.go                 # REST API handlers (create)
│   ├── types.go               # Request/Response types (create)
│   └── static/
│       └── index.html         # Placeholder (create)
├── ui/
│   ├── package.json           # Vue project config (create)
│   ├── vite.config.js         # Vite config (create)
│   ├── index.html             # HTML template (create)
│   └── src/
│       ├── main.js            # Vue entry (create)
│       ├── App.vue            # Main app component (create)
│       └── components/
│           └── Chat.vue       # Chat component (create)
├── docs/
│   └── feishu.md              # Feishu guide (create)
└── release.sh                 # Update to include docs
```

---

## Task 1: Create WebChat Backend Types

**Files:**
- Create: `web/types.go`
- Create: `web/types_test.go`

- [ ] **Step 1: Create web directory**

```bash
mkdir -p web/static
```

- [ ] **Step 2: Create types.go**

```go
// web/types.go
package web

// ChatRequest represents a chat request from the client
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents a chat response to the client
type ChatResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HistoryResponse represents the session history
type HistoryResponse struct {
	Messages []MessageItem `json:"messages"`
}

// MessageItem represents a single message in history
type MessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
```

- [ ] **Step 3: Create types_test.go**

```go
// web/types_test.go
package web

import (
	"encoding/json"
	"testing"
)

func TestChatRequest_JSON(t *testing.T) {
	req := ChatRequest{Message: "hello"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Message != "hello" {
		t.Errorf("expected 'hello', got '%s'", decoded.Message)
	}
}

func TestChatResponse_JSON(t *testing.T) {
	resp := ChatResponse{Content: "hi there", Error: ""}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Content != "hi there" {
		t.Errorf("expected 'hi there', got '%s'", decoded.Content)
	}
	if decoded.Error != "" {
		t.Errorf("expected empty error, got '%s'", decoded.Error)
	}
}

func TestHistoryResponse_JSON(t *testing.T) {
	resp := HistoryResponse{
		Messages: []MessageItem{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded HistoryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(decoded.Messages))
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/zn-ice/2026/Golem
go test ./web/ -v
```

- [ ] **Step 5: Commit**

```bash
git add web/types.go web/types_test.go
git commit -m "feat(web): add WebChat types"
```

---

## Task 2: Create WebChat HTTP Server

**Files:**
- Create: `web/server.go`
- Create: `web/api.go`
- Create: `web/server_test.go`

- [ ] **Step 1: Create server.go**

```go
// web/server.go
package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// Server represents the WebChat HTTP server
type Server struct {
	engine   *core.Engine
	provider core.ProviderPlugin
	session  *core.Session
	addr     string
	logger   *slog.Logger
	mu       sync.RWMutex
}

// NewServer creates a new WebChat server
func NewServer(engine *core.Engine, provider core.ProviderPlugin, addr string) *Server {
	return &Server{
		engine:   engine,
		provider: provider,
		addr:     addr,
		logger:   slog.Default().With("component", "webchat"),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	staticDir := filepath.Join(".", "web", "static")
	if _, err := os.Stat(staticDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(staticDir)))
		s.logger.Info("serving static files", "dir", staticDir)
	}

	// API endpoints
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/clear", s.handleClear)

	s.logger.Info("starting webchat server", "addr", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return s.addr
}
```

- [ ] **Step 2: Create api.go**

```go
// web/api.go
package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// handleChat handles POST /api/chat
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Message == "" {
		s.writeError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	// Get or create session
	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get session")
		return
	}

	// Add user message
	s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: req.Message,
	})

	// Get history
	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Call LLM
	response, err := s.provider.Chat(r.Context(), history, core.ChatConfig{})
	if err != nil {
		s.logger.Error("LLM error", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to get response")
		return
	}

	// Add assistant message
	s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: response.Content,
	})

	// Return response
	s.writeJSON(w, ChatResponse{Content: response.Content})
}

// handleHistory handles GET /api/history
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get session")
		return
	}

	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 100)

	messages := make([]MessageItem, len(history))
	for i, msg := range history {
		messages[i] = MessageItem{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	s.writeJSON(w, HistoryResponse{Messages: messages})
}

// handleClear handles POST /api/clear
func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.session = nil
	s.mu.Unlock()

	s.writeJSON(w, map[string]string{"status": "ok"})
}

// getOrCreateSession gets or creates a session
func (s *Server) getOrCreateSession() (*core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return s.session, nil
	}

	session, err := s.engine.GetSessionManager().CreateSession("webchat", "web")
	if err != nil {
		return nil, err
	}

	s.session = session
	return session, nil
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
```

- [ ] **Step 3: Create server_test.go**

```go
// web/server_test.go
package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
)

// MockProvider is a mock LLM provider for testing
type MockProvider struct{}

func (p *MockProvider) Name() string                   { return "mock" }
func (p *MockProvider) Version() string                { return "1.0.0" }
func (p *MockProvider) Initialize(config map[string]interface{}) error { return nil }
func (p *MockProvider) Start() error                   { return nil }
func (p *MockProvider) Stop() error                    { return nil }
func (p *MockProvider) HealthCheck() plugin.HealthStatus { return plugin.HealthStatus{Healthy: true} }
func (p *MockProvider) GetProviderType() string         { return "mock" }
func (p *MockProvider) SupportsStreaming() bool         { return false }
func (p *MockProvider) Chat(ctx interface{}, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
	return &core.ChatResponse{Content: "mock response"}, nil
}
func (p *MockProvider) ChatStream(ctx interface{}, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func setupTestServer() *Server {
	cfg := config.DefaultConfig()
	engine, _ := core.NewEngine(cfg)
	provider := &MockProvider{}
	return NewServer(engine, provider, ":0")
}

func TestHandleChat(t *testing.T) {
	srv := setupTestServer()

	body, _ := json.Marshal(ChatRequest{Message: "hello"})
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp ChatResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Content != "mock response" {
		t.Errorf("expected 'mock response', got '%s'", resp.Content)
	}
}

func TestHandleChat_EmptyMessage(t *testing.T) {
	srv := setupTestServer()

	body, _ := json.Marshal(ChatRequest{Message: ""})
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleHistory(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest("GET", "/api/history", nil)
	w := httptest.NewRecorder()

	srv.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp HistoryResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Messages == nil {
		t.Error("messages should not be nil")
	}
}

func TestHandleClear(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest("POST", "/api/clear", nil)
	w := httptest.NewRecorder()

	srv.handleClear(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/zn-ice/2026/Golem
go test ./web/ -v
```

- [ ] **Step 5: Commit**

```bash
git add web/
git commit -m "feat(web): implement WebChat HTTP server with API"
```

---

## Task 3: Create Web CLI Command

**Files:**
- Create: `cmd/golem/web.go`

- [ ] **Step 1: Create web.go**

```go
// cmd/golem/web.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	"github.com/Shadow-Azure/Golem/web"
	"github.com/spf13/cobra"
)

var webPort int

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 WebChat 服务",
	Long:  "启动 Web 界面，在浏览器中与 AI 助手对话。",
	Run:   runWeb,
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8080, "WebChat 服务端口")
}

func runWeb(cmd *cobra.Command, args []string) {
	// Load config
	configPath := cfgFile
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".golem", "golem.yaml")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// Create engine
	engine, err := core.NewEngine(cfg)
	if err != nil {
		fmt.Printf("创建引擎失败: %v\n", err)
		os.Exit(1)
	}

	// Create plugin manager
	pm := plugin.NewManager()

	// Register providers
	var provider core.ProviderPlugin

	if openaiConfig, ok := cfg.LLM.Providers["openai"]; ok {
		openaiProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      openaiConfig.APIKey,
			BaseURL:     openaiConfig.BaseURL,
			Model:       openaiConfig.Model,
			Temperature: openaiConfig.Temperature,
			MaxTokens:   openaiConfig.MaxTokens,
		})
		pm.LoadPlugin("openai", openaiProv)
		if cfg.LLM.DefaultProvider == "openai" {
			provider = openaiProv
		}
	}

	if claudeConfig, ok := cfg.LLM.Providers["claude"]; ok {
		claudeProv := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
			APIKey:    claudeConfig.APIKey,
			BaseURL:   claudeConfig.BaseURL,
			Model:     claudeConfig.Model,
			MaxTokens: claudeConfig.MaxTokens,
		})
		pm.LoadPlugin("claude", claudeProv)
		if cfg.LLM.DefaultProvider == "claude" {
			provider = claudeProv
		}
	}

	if provider == nil {
		fmt.Println("未找到 LLM 提供商，请检查配置")
		os.Exit(1)
	}

	// Start engine
	engine.Start()

	// Create and start web server
	addr := fmt.Sprintf(":%d", webPort)
	srv := web.NewServer(engine, provider, addr)

	fmt.Printf("🤖 Golem WebChat 已启动\n")
	fmt.Printf("打开浏览器访问: http://localhost:%d\n", webPort)
	fmt.Println("按 Ctrl+C 停止")

	if err := srv.Start(); err != nil {
		fmt.Printf("启动服务失败: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Add webCmd to root.go**

Add to the `init()` function in `cmd/golem/root.go`:

```go
rootCmd.AddCommand(webCmd)
```

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 4: Commit**

```bash
git add cmd/golem/web.go cmd/golem/root.go
git commit -m "feat(cli): add web command for WebChat"
```

---

## Task 4: Create TUI Chat Command

**Files:**
- Create: `cmd/golem/chat.go`

- [ ] **Step 1: Create chat.go**

```go
// cmd/golem/chat.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Shadow-Azure/Golem/internal/config"
	"github.com/Shadow-Azure/Golem/internal/core"
	"github.com/Shadow-Azure/Golem/internal/plugin"
	openaiPlugin "github.com/Shadow-Azure/Golem/plugins/providers/openai"
	claudePlugin "github.com/Shadow-Azure/Golem/plugins/providers/claude"
	"github.com/spf13/cobra"
)

var chatProvider string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "启动终端聊天",
	Long:  "在终端中直接与 AI 助手对话。",
	Run:   runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatProvider, "provider", "", "LLM 提供商 (openai, claude)")
}

func runChat(cmd *cobra.Command, args []string) {
	// Load config
	configPath := cfgFile
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".golem", "golem.yaml")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// Create engine
	engine, err := core.NewEngine(cfg)
	if err != nil {
		fmt.Printf("创建引擎失败: %v\n", err)
		os.Exit(1)
	}

	// Create plugin manager
	pm := plugin.NewManager()

	// Determine provider
	providerName := cfg.LLM.DefaultProvider
	if chatProvider != "" {
		providerName = chatProvider
	}

	// Register providers
	var provider core.ProviderPlugin

	if openaiConfig, ok := cfg.LLM.Providers["openai"]; ok {
		openaiProv := openaiPlugin.NewProvider(openaiPlugin.ProviderConfig{
			APIKey:      openaiConfig.APIKey,
			BaseURL:     openaiConfig.BaseURL,
			Model:       openaiConfig.Model,
			Temperature: openaiConfig.Temperature,
			MaxTokens:   openaiConfig.MaxTokens,
		})
		pm.LoadPlugin("openai", openaiProv)
		if providerName == "openai" {
			provider = openaiProv
		}
	}

	if claudeConfig, ok := cfg.LLM.Providers["claude"]; ok {
		claudeProv := claudePlugin.NewProvider(claudePlugin.ProviderConfig{
			APIKey:    claudeConfig.APIKey,
			BaseURL:   claudeConfig.BaseURL,
			Model:     claudeConfig.Model,
			MaxTokens: claudeConfig.MaxTokens,
		})
		pm.LoadPlugin("claude", claudeProv)
		if providerName == "claude" {
			provider = claudeProv
		}
	}

	if provider == nil {
		fmt.Println("未找到 LLM 提供商，请检查配置")
		os.Exit(1)
	}

	// Start engine
	engine.Start()

	// Create session
	session, _ := engine.GetSessionManager().CreateSession("tui", "terminal")

	fmt.Println("🤖 Golem AI Chat")
	fmt.Printf("提供商: %s\n", providerName)
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
			if handleChatCommand(input, engine, session) {
				continue
			}
			break
		}

		// Add user message
		engine.GetSessionManager().AddMessage(session.ID, core.Message{
			Role:    "user",
			Content: input,
		})

		// Get history
		history, _ := engine.GetSessionManager().GetHistory(session.ID, 50)

		// Call LLM
		fmt.Print("AI: ")
		response, err := provider.Chat(context.Background(), history, core.ChatConfig{})
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			continue
		}

		fmt.Println(response.Content)
		fmt.Println()

		// Add assistant message
		engine.GetSessionManager().AddMessage(session.ID, core.Message{
			Role:    "assistant",
			Content: response.Content,
		})
	}
}

// handleChatCommand handles chat commands, returns true to continue, false to exit
func handleChatCommand(input string, engine *core.Engine, session *core.Session) bool {
	switch input {
	case "/help":
		fmt.Println("可用命令:")
		fmt.Println("  /help     - 显示帮助")
		fmt.Println("  /clear    - 清空历史")
		fmt.Println("  /history  - 显示历史")
		fmt.Println("  /quit     - 退出")
		return true
	case "/clear":
		// Create new session
		engine.GetSessionManager().DeleteSession(session.ID)
		fmt.Println("历史已清空")
		return true
	case "/history":
		history, _ := engine.GetSessionManager().GetHistory(session.ID, 20)
		if len(history) == 0 {
			fmt.Println("暂无历史记录")
		} else {
			for _, msg := range history {
				role := "You"
				if msg.Role == "assistant" {
					role = "AI"
				}
				fmt.Printf("%s: %s\n", role, msg.Content)
			}
		}
		return true
	case "/quit":
		fmt.Println("再见！")
		return false
	default:
		fmt.Printf("未知命令: %s\n", input)
		return true
	}
}
```

- [ ] **Step 2: Add chatCmd to root.go**

Add to the `init()` function in `cmd/golem/root.go`:

```go
rootCmd.AddCommand(chatCmd)
```

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 4: Commit**

```bash
git add cmd/golem/chat.go cmd/golem/root.go
git commit -m "feat(cli): add chat command for TUI"
```

---

## Task 5: Create Vue 3 Frontend

**Files:**
- Create: `ui/package.json`
- Create: `ui/vite.config.js`
- Create: `ui/index.html`
- Create: `ui/src/main.js`
- Create: `ui/src/App.vue`
- Create: `ui/src/components/Chat.vue`

- [ ] **Step 1: Create ui directory and package.json**

```bash
mkdir -p ui/src/components ui/public
```

```json
{
  "name": "golem-webchat",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "vite": "^5.0.0"
  }
}
```

- [ ] **Step 2: Create vite.config.js**

```javascript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: '../web/static',
    emptyOutDir: true
  }
})
```

- [ ] **Step 3: Create index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Golem AI Chat</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.js"></script>
</body>
</html>
```

- [ ] **Step 4: Create main.js**

```javascript
import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
```

- [ ] **Step 5: Create App.vue**

```vue
<template>
  <Chat />
</template>

<script setup>
import Chat from './components/Chat.vue'
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  background: #f0f2f5;
}
</style>
```

- [ ] **Step 6: Create Chat.vue**

```vue
<template>
  <div class="chat-container">
    <div class="header">
      <h1>🤖 Golem AI Chat</h1>
    </div>

    <div class="messages" ref="messagesRef">
      <div
        v-for="(msg, index) in messages"
        :key="index"
        :class="['message', msg.role]"
      >
        <div class="avatar">{{ msg.role === 'user' ? '👤' : '🤖' }}</div>
        <div class="content">{{ msg.content }}</div>
      </div>
      <div v-if="loading" class="message assistant">
        <div class="avatar">🤖</div>
        <div class="content loading">思考中...</div>
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
        发送
      </button>
      <button @click="clearHistory" class="clear-btn">清空</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'

const messages = ref([])
const input = ref('')
const loading = ref(false)
const messagesRef = ref(null)

const scrollToBottom = () => {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

const fetchHistory = async () => {
  try {
    const response = await fetch('/api/history')
    const data = await response.json()
    messages.value = data.messages || []
    nextTick(scrollToBottom)
  } catch (error) {
    console.error('Failed to fetch history:', error)
  }
}

const sendMessage = async () => {
  if (!input.value.trim() || loading.value) return

  const userMessage = input.value.trim()
  input.value = ''
  loading.value = true

  messages.value.push({ role: 'user', content: userMessage })
  nextTick(scrollToBottom)

  try {
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: userMessage })
    })

    const data = await response.json()
    if (data.error) {
      messages.value.push({ role: 'assistant', content: '错误: ' + data.error })
    } else {
      messages.value.push({ role: 'assistant', content: data.content })
    }
  } catch (error) {
    messages.value.push({ role: 'assistant', content: '错误: ' + error.message })
  } finally {
    loading.value = false
    nextTick(scrollToBottom)
  }
}

const clearHistory = async () => {
  try {
    await fetch('/api/clear', { method: 'POST' })
    messages.value = []
  } catch (error) {
    console.error('Failed to clear history:', error)
  }
}

onMounted(() => {
  fetchHistory()
})
</script>

<style scoped>
.chat-container {
  max-width: 800px;
  margin: 0 auto;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: white;
  box-shadow: 0 0 20px rgba(0,0,0,0.1);
}

.header {
  padding: 20px;
  background: #007bff;
  color: white;
  text-align: center;
}

.header h1 {
  font-size: 1.5rem;
  font-weight: 600;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #f5f5f5;
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
  flex-shrink: 0;
}

.content {
  max-width: 70%;
  padding: 12px 16px;
  border-radius: 12px;
  line-height: 1.5;
  word-break: break-word;
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

.loading {
  color: #999;
  font-style: italic;
}

.input-area {
  display: flex;
  gap: 12px;
  padding: 20px;
  background: white;
  border-top: 1px solid #eee;
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

.clear-btn {
  background: #6c757d !important;
}

.clear-btn:hover:not(:disabled) {
  background: #545b62 !important;
}
</style>
```

- [ ] **Step 7: Commit**

```bash
git add ui/
git commit -m "feat(ui): add Vue 3 WebChat frontend"
```

---

## Task 6: Create Feishu Documentation

**Files:**
- Create: `docs/feishu.md`

- [ ] **Step 1: Create docs directory and feishu.md**

```bash
mkdir -p docs
```

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

### 方式一：交互式配置

```bash
golem onboard
```

按照提示选择飞书配置，输入 App ID、App Secret 和 Verification Token。

### 方式二：手动配置

编辑 `~/.golem/golem.yaml`：

```yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"           # 替换为你的 App ID
  app_secret: "xxxx"           # 替换为你的 App Secret
  verification_token: "xxxx"   # 替换为你的 Verification Token
```

### 方式三：环境变量

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

### Q: 如何停止服务？

```bash
golem stop
```
```

- [ ] **Step 2: Commit**

```bash
git add docs/feishu.md
git commit -m "docs: add Feishu bot connection guide"
```

---

## Task 7: Update Release Script

**Files:**
- Modify: `release.sh`

- [ ] **Step 1: Update release.sh to include docs and web static**

Read the current release.sh and add after Step 7 (Package):

```bash
# Step 8: Copy docs
echo -e "${YELLOW}Step 8: Copying docs...${NC}"
mkdir -p release/docs
cp docs/feishu.md release/docs/
echo -e "${GREEN}Copied: release/docs/feishu.md${NC}"

# Step 9: Build WebChat UI (if pnpm available)
echo -e "${YELLOW}Step 9: Building WebChat UI...${NC}"
if command -v pnpm &> /dev/null && [ -d "ui" ]; then
    cd ui
    pnpm install
    pnpm build
    cd ..
    mkdir -p release/web/static
    cp -r ui/dist/* release/web/static/
    echo -e "${GREEN}Built WebChat UI${NC}"
else
    echo -e "${YELLOW}Skipping WebChat UI (pnpm not found or ui/ not exists)${NC}"
fi
```

- [ ] **Step 2: Commit**

```bash
git add release.sh
git commit -m "build(release): add docs and WebChat UI to release"
```

---

## Task 8: Final Testing

- [ ] **Step 1: Run all Go tests**

```bash
cd /Users/zn-ice/2026/Golem
go test ./...
```

- [ ] **Step 2: Build and test TUI**

```bash
go build -o bin/golem ./cmd/golem
echo "/quit" | ./bin/golem chat
```

- [ ] **Step 3: Run release script**

```bash
./release.sh
```

- [ ] **Step 4: Verify release output**

```bash
ls -la release/
ls -la release/docs/
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete local testing with TUI, WebChat, and Feishu docs"
```

---

## Self-Review Checklist

✅ **Spec coverage:** All requirements covered - TUI, WebChat, Feishu docs
✅ **Placeholder scan:** No TBD/TODO found
✅ **Type consistency:** All types consistent across tasks

---

## Summary

This plan implements:

1. **WebChat Backend** - Go HTTP server with REST API
2. **WebChat Frontend** - Vue 3 chat interface
3. **TUI Chat** - Terminal-based chat command
4. **Feishu Documentation** - Connection guide
5. **Release Script Update** - Include docs and UI

**Total Tasks:** 8
**Estimated Time:** 2-3 hours
